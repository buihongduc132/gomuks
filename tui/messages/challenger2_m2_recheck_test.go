// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Empirical Challenger 2 Test Suite for Milestone M2 Recheck (Iteration 2).
// Tests lifecycle state transitions, deadlock freedom, callback queuing,
// WaitGroup reuse elimination, and clone deep-copy isolation.

package messages

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
)

// ---------------------------------------------------------------------------
// 1. WaitDownload and IsDownloading inside onDone callback (No Deadlock)
// ---------------------------------------------------------------------------
func TestChallenger2_WaitDownloadAndIsDownloadingInsideCallback_Success(t *testing.T) {
	pngData := makeChallengerPNG(48, 48)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "callback_deadlock_test.png",
		URL:     "mxc://example.com/dl1",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$cb_dl_success"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		return pngData, nil
	})

	doneCh := make(chan struct{})
	var isDownloadingInCallback bool
	var waitCompletedInCallback bool
	var reentrantCompleted bool

	fileMsg.DownloadPreview(func() {
		// 1. Check IsDownloading inside callback
		isDownloadingInCallback = fileMsg.IsDownloading()

		// 2. Call WaitDownload inside callback synchronously
		waitDone := make(chan struct{})
		go func() {
			fileMsg.WaitDownload()
			close(waitDone)
		}()

		select {
		case <-waitDone:
			waitCompletedInCallback = true
		case <-time.After(1 * time.Second):
			t.Errorf("FAIL: WaitDownload deadlocked inside onDone callback")
		}

		// 3. Test reentrant DownloadPreview inside callback
		reentrantDone := make(chan struct{})
		fileMsg.DownloadPreview(func() {
			close(reentrantDone)
		})
		select {
		case <-reentrantDone:
			reentrantCompleted = true
		case <-time.After(500 * time.Millisecond):
			t.Errorf("FAIL: Reentrant DownloadPreview inside callback deadlocked")
		}

		close(doneCh)
	})

	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for outer callback completion")
	}

	if isDownloadingInCallback {
		t.Errorf("FAIL: IsDownloading() was true inside onDone callback; expected false")
	}
	if !waitCompletedInCallback {
		t.Errorf("FAIL: WaitDownload() failed to complete inside onDone callback")
	}
	if !reentrantCompleted {
		t.Errorf("FAIL: Reentrant DownloadPreview failed inside onDone callback")
	}
	if len(fileMsg.ImageData()) == 0 {
		t.Errorf("FAIL: ImageData is empty after download finished")
	}
}

func TestChallenger2_WaitDownloadAndIsDownloadingInsideCallback_Failure(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "failing_download.png",
		URL:     "mxc://example.com/fail_dl",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$cb_dl_fail"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadErr := errors.New("network timeout 504")
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(10 * time.Millisecond)
		return nil, downloadErr
	})

	doneCh := make(chan struct{})
	var isDownloadingInCallback bool
	var waitCompletedInCallback bool

	fileMsg.DownloadPreview(func() {
		isDownloadingInCallback = fileMsg.IsDownloading()

		waitDone := make(chan struct{})
		go func() {
			fileMsg.WaitDownload()
			close(waitDone)
		}()

		select {
		case <-waitDone:
			waitCompletedInCallback = true
		case <-time.After(1 * time.Second):
			t.Errorf("FAIL: WaitDownload deadlocked inside onDone failure callback")
		}

		close(doneCh)
	})

	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for failure callback")
	}

	if isDownloadingInCallback {
		t.Errorf("FAIL: IsDownloading() was true inside failure callback; expected false")
	}
	if !waitCompletedInCallback {
		t.Errorf("FAIL: WaitDownload() failed inside failure callback")
	}
	if fileMsg.ImageError() == nil {
		t.Errorf("FAIL: Expected ImageError to be recorded on failure")
	}
}

// ---------------------------------------------------------------------------
// 2. Concurrent DownloadPreview Callbacks Queuing & Zero Premature Execution
// ---------------------------------------------------------------------------
func TestChallenger2_ConcurrentDownloadPreview_CallbacksQueuing(t *testing.T) {
	pngData := makeChallengerPNG(40, 40)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "concurrent_queue.png",
		URL:     "mxc://example.com/concur_queue",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$queue_concur"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadStarted := make(chan struct{})
	releaseDownload := make(chan struct{})
	var downloadInvocations atomic.Int32

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		downloadInvocations.Add(1)
		close(downloadStarted)
		<-releaseDownload
		return pngData, nil
	})

	// 1. Initial caller triggers download
	var initialCbCalled atomic.Bool
	fileMsg.DownloadPreview(func() {
		initialCbCalled.Store(true)
	})

	// Wait until download function is actively running in background
	<-downloadStarted

	// 2. Concurrent callers while download is in-flight
	const numConcurrentCallers = 50
	var executedCallbacks atomic.Int32
	var prematureCallbacks atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < numConcurrentCallers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fileMsg.DownloadPreview(func() {
				// Assert on state when callback executes
				if len(fileMsg.ImageData()) == 0 {
					prematureCallbacks.Add(1)
				}
				if fileMsg.IsDownloading() {
					prematureCallbacks.Add(1)
				}
				executedCallbacks.Add(1)
			})
		}(i)
	}
	wg.Wait()

	// Verify that while download is paused, NO callbacks have executed yet
	if executedCallbacks.Load() != 0 {
		t.Fatalf("CRITICAL: %d callbacks executed before download was released!", executedCallbacks.Load())
	}
	if initialCbCalled.Load() {
		t.Fatalf("CRITICAL: Initial callback executed before download was released!")
	}

	// 3. Release download to finish
	close(releaseDownload)
	fileMsg.WaitDownload()

	// Give a brief moment for all dispatched callbacks to finish executing
	time.Sleep(50 * time.Millisecond)

	if downloadInvocations.Load() != 1 {
		t.Errorf("FAIL: Expected exactly 1 download invocation, got %d", downloadInvocations.Load())
	}
	if prematureCallbacks.Load() != 0 {
		t.Errorf("FAIL: %d callbacks observed premature state (empty data or downloading=true)", prematureCallbacks.Load())
	}
	if executedCallbacks.Load() != numConcurrentCallers {
		t.Errorf("FAIL: Expected %d concurrent callbacks executed, got %d", numConcurrentCallers, executedCallbacks.Load())
	}
	if !initialCbCalled.Load() {
		t.Errorf("FAIL: Initial callback was not executed")
	}

	// 4. Subsequent caller after completion gets immediate callback with full data
	var subsequentCbRan bool
	fileMsg.DownloadPreview(func() {
		subsequentCbRan = true
		if len(fileMsg.ImageData()) == 0 {
			t.Errorf("subsequent callback received empty image data")
		}
	})
	if !subsequentCbRan {
		t.Errorf("FAIL: Subsequent callback was not executed immediately")
	}
}

// ---------------------------------------------------------------------------
// 3. WaitGroup Reuse Panic Complete Elimination under High Churn
// ---------------------------------------------------------------------------
func TestChallenger2_WaitGroupReusePanic_HighChurnStress(t *testing.T) {
	pngData := makeChallengerPNG(24, 24)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "churn.png",
		URL:     "mxc://example.com/churn",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$churn1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(2 * time.Millisecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	// 10 goroutines continuously calling WaitDownload()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.WaitDownload()
				time.Sleep(100 * time.Microsecond)
			}
		}()
	}

	// 10 goroutines triggering DownloadPreview and resetting data (churn)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for !stop.Load() {
				if id%2 == 0 {
					// Reset image data to force a new download round
					fileMsg.SetImageData(nil)
				}
				fileMsg.DownloadPreview()
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// 5 goroutines calling UI render & buffer calculation concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prefs := config.UserPreferences{}
			for !stop.Load() {
				uiMsg.CalculateBuffer(prefs, 80)
				_ = uiMsg.Height()
				time.Sleep(500 * time.Microsecond)
			}
		}()
	}

	// Run under high contention for 250ms
	time.Sleep(250 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// 4. Clone Deep Copy and Pointer Isolation Verification
// ---------------------------------------------------------------------------
func TestChallenger2_CloneDeepCopyAndPointerIsolation(t *testing.T) {
	pngData := makeChallengerPNG(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "clone_isolation.png",
		URL:     "mxc://example.com/clone_iso",
	}

	meta := &database.Room{ID: "!iso_room:example.com"}
	roomStore := store.NewRoomStore(nil, meta)
	evt := &database.Event{ID: "$clone_iso_evt"}

	origUIMsg := NewFileMessage(roomStore, nil, evt, content)
	origFileMsg := origUIMsg.Renderer.(*FileMessage)
	origFileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{}
	origUIMsg.CalculateBuffer(prefs, 80)

	// Clone original
	clonedUIMsg := origUIMsg.Clone()
	clonedFileMsg, ok := clonedUIMsg.Renderer.(*FileMessage)
	if !ok {
		t.Fatalf("FAIL: Cloned renderer is not *FileMessage")
	}

	// 1. Assert bidirectional back-reference isolation
	if clonedFileMsg.uiMsg != clonedUIMsg {
		t.Errorf("FAIL: clonedFileMsg.uiMsg (%p) does not point to clonedUIMsg (%p)", clonedFileMsg.uiMsg, clonedUIMsg)
	}
	if origFileMsg.uiMsg != origUIMsg {
		t.Errorf("FAIL: origFileMsg.uiMsg (%p) was mutated to point to clonedUIMsg", origFileMsg.uiMsg)
	}

	// 2. Assert Buffer Line Independence & Deep Copy
	if len(clonedFileMsg.buffer) == 0 {
		t.Fatalf("FAIL: clonedFileMsg buffer is empty")
	}
	if len(clonedFileMsg.buffer) != len(origFileMsg.buffer) {
		t.Fatalf("FAIL: buffer line count mismatch: orig %d vs clone %d", len(origFileMsg.buffer), len(clonedFileMsg.buffer))
	}

	// Compare backing slice headers - they must NOT share backing memory
	origFirstLine := origFileMsg.buffer[0]
	clonedFirstLine := clonedFileMsg.buffer[0]
	if len(origFirstLine) > 0 && len(clonedFirstLine) > 0 {
		// Mutate original first cell
		origChar := origFirstLine[0].Char
		origFirstLine[0].Char = 'Z'

		// Clone must retain its original cell value
		if clonedFirstLine[0].Char == 'Z' {
			t.Errorf("CRITICAL FAIL: Mutating origFileMsg.buffer modified clonedFileMsg.buffer! Shallow copy detected.")
		}

		// Restore
		origFirstLine[0].Char = origChar
	}

	// 3. Assert Independent Invalidation via InvalidateBuffer
	origUIMsg.InvalidateBuffer()
	if origUIMsg.BufferedWidth() != 0 {
		t.Errorf("FAIL: origUIMsg.BufferedWidth() expected 0, got %d", origUIMsg.BufferedWidth())
	}
	if clonedUIMsg.BufferedWidth() != 80 {
		t.Errorf("FAIL: clonedUIMsg.BufferedWidth() was corrupted by orig invalidation; expected 80, got %d", clonedUIMsg.BufferedWidth())
	}

	clonedUIMsg.InvalidateBuffer()
	if clonedUIMsg.BufferedWidth() != 0 {
		t.Errorf("FAIL: clonedUIMsg.BufferedWidth() expected 0, got %d", clonedUIMsg.BufferedWidth())
	}

	// 4. Assert SetImageData on Clone only invalidates Clone
	origUIMsg.CalculateBuffer(prefs, 80)
	if origUIMsg.BufferedWidth() != 80 {
		t.Fatalf("FAIL: origUIMsg buffer recalculation failed")
	}

	newPNG := makeChallengerPNG(16, 16)
	clonedFileMsg.SetImageData(newPNG)

	// Cloned buffered width should be 0 (invalidated), original must remain 80
	if clonedUIMsg.BufferedWidth() != 0 {
		t.Errorf("FAIL: clonedUIMsg was not invalidated after SetImageData on clone")
	}
	if origUIMsg.BufferedWidth() != 80 {
		t.Errorf("FAIL: origUIMsg was invalidated when SetImageData was called on clone!")
	}

	// 5. Assert ImageData isolation
	if !bytes.Equal(clonedFileMsg.ImageData(), newPNG) {
		t.Errorf("FAIL: clonedFileMsg does not have new image data")
	}
	if !bytes.Equal(origFileMsg.ImageData(), pngData) {
		t.Errorf("FAIL: origFileMsg image data was overwritten by clone's SetImageData!")
	}
}

// ---------------------------------------------------------------------------
// 5. Concurrent Clone Stress Under Active Download, Render, and Draw
// ---------------------------------------------------------------------------
func TestChallenger2_ConcurrentCloneStress_UnderActiveDownloadAndRender(t *testing.T) {
	pngData := makeChallengerPNG(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "concurrent_clone.png",
		URL:     "mxc://example.com/concur_clone",
	}

	meta := &database.Room{ID: "!concur_room:example.com"}
	roomStore := store.NewRoomStore(nil, meta)
	evt := &database.Event{ID: "$concur_clone_evt"}

	uiMsg := NewFileMessage(roomStore, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(3 * time.Millisecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Clone workers: repeatedly clone uiMsg, verify back-reference and draw clone
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			prefs := config.UserPreferences{}
			screen := newSimulationScreen(80, 24)
			for !stop.Load() {
				clone := uiMsg.Clone()
				cloneRenderer, ok := clone.Renderer.(*FileMessage)
				if !ok || cloneRenderer.uiMsg != clone {
					t.Errorf("FAIL: Cloned uiMsg backreference misaligned")
					return
				}
				clone.CalculateBuffer(prefs, 80)
				_ = clone.Height()
				clone.Draw(screen)
				clone.InvalidateBuffer()
				time.Sleep(500 * time.Microsecond)
			}
		}(i)
	}

	// Downloader workers: repeatedly start downloads
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.DownloadPreview()
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	// Mutator/Invalidator workers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			prefs := config.UserPreferences{}
			screen := newSimulationScreen(80, 24)
			for !stop.Load() {
				if id%2 == 0 {
					fileMsg.SetImageData(pngData)
				}
				uiMsg.CalculateBuffer(prefs, 80)
				uiMsg.Draw(screen)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(250 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// 6. Minimal Isolation: UIMessage.Clone vs InvalidateBuffer Data Race
// ---------------------------------------------------------------------------
func TestChallenger2_CloneVsInvalidateBuffer_DataRace(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "race.png",
	}
	evt := &database.Event{ID: "$race", Sender: "@alice:example.com"}
	uiMsg := NewFileMessage(nil, nil, evt, content)

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Goroutine 1: continuously clones uiMsg
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			_ = uiMsg.Clone()
		}
	}()

	// Goroutine 2: continuously invalidates buffer (atomic.StoreInt32(&uiMsg.bufferedWidth, 0))
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			uiMsg.InvalidateBuffer()
		}
	}()

	time.Sleep(50 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 7. Empirical Test: Does WaitDownload() return BEFORE callbacks finish?
// ---------------------------------------------------------------------------
func TestChallenger2_WaitDownloadPrematureReturnBeforeCallbacksComplete(t *testing.T) {
	pngData := makeChallengerPNG(20, 20)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "premature_wait.png",
		URL:     "mxc://example.com/premature_wait",
	}
	evt := &database.Event{ID: "$premature", Sender: "@bob:example.com"}

	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return pngData, nil
	})

	var callbackFinished atomic.Bool
	fileMsg.DownloadPreview(func() {
		time.Sleep(50 * time.Millisecond)
		callbackFinished.Store(true)
	})

	// Caller waits for download to finish
	fileMsg.WaitDownload()

	// Has callback finished when WaitDownload returns?
	if !callbackFinished.Load() {
		t.Errorf("LIFECYCLE DEFECT: WaitDownload() returned BEFORE onDone callbacks finished executing!")
	}
}

// ---------------------------------------------------------------------------
// 8. Empirical Test: Deterministic Premature WaitDownload While Callback Is Active
// ---------------------------------------------------------------------------
func TestChallenger2_WaitDownloadWhileCallbacksRunning_ReturnsImmediatelyWithoutWaiting(t *testing.T) {
	pngData := makeChallengerPNG(20, 20)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "premature_deterministic.png",
		URL:     "mxc://example.com/premature_deterministic",
	}
	evt := &database.Event{ID: "$premature_det", Sender: "@bob:example.com"}

	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return pngData, nil
	})

	callbackStarted := make(chan struct{})
	var callbackFinished atomic.Bool

	fileMsg.DownloadPreview(func() {
		close(callbackStarted)
		time.Sleep(50 * time.Millisecond)
		callbackFinished.Store(true)
	})

	// Wait until the callback has actively started running
	<-callbackStarted

	// External caller now calls WaitDownload()
	waitReturned := make(chan struct{})
	go func() {
		fileMsg.WaitDownload()
		close(waitReturned)
	}()

	select {
	case <-waitReturned:
		// Check if WaitDownload returned before the callback finished
		if !callbackFinished.Load() {
			t.Errorf("LIFECYCLE FLAW: WaitDownload() returned prematurely while callback was actively executing in background!")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WaitDownload timed out")
	}
}



