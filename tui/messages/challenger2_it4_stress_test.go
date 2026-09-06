// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Empirical Challenger 2 (Iteration 4) Deep Stress Harness

package messages

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
)

// 1. Stress: High contention multi-waiter, multi-callback, chained re-entrancy
// Waiters start AFTER download is actively in flight or callbacks are executing.
func TestChallenger2_It4_ChainedReentrancyAndExternalWaiters_Stress(t *testing.T) {
	const iterations = 10
	pngData := makeChallengerPNG(16, 16)

	for i := 0; i < iterations; i++ {
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    fmt.Sprintf("stress_%d.png", i),
			URL:     id.ContentURIString(fmt.Sprintf("mxc://example.com/stress_%d", i)),
		}
		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: id.EventID(fmt.Sprintf("$stress_%d", i))}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		downloadStarted := make(chan struct{})
		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			close(downloadStarted)
			time.Sleep(10 * time.Millisecond)
			return pngData, nil
		})

		var cb1Finished, cb2Finished, cb3Finished atomic.Bool
		const numExternalWaiters = 20
		var externalWaitersDone sync.WaitGroup
		var prematureReturns atomic.Int32

		externalWaitersDone.Add(numExternalWaiters)

		// Start download with chained re-entrancy
		downloadCompleted := make(chan struct{})
		fileMsg.DownloadPreview(func() {
			// Callback 1
			time.Sleep(10 * time.Millisecond)
			fileMsg.WaitDownload() // Re-entrant call from callback 1 directly

			// Register Callback 2 inside Callback 1
			fileMsg.DownloadPreview(func() {
				time.Sleep(10 * time.Millisecond)
				// Re-entrant call from child goroutine inside Callback 2
				childDone := make(chan struct{})
				go func() {
					fileMsg.WaitDownload()
					close(childDone)
				}()
				<-childDone

				// Register Callback 3 inside Callback 2
				fileMsg.DownloadPreview(func() {
					time.Sleep(10 * time.Millisecond)
					fileMsg.WaitDownload() // Re-entrant call from callback 3
					cb3Finished.Store(true)
				})

				cb2Finished.Store(true)
			})

			cb1Finished.Store(true)
			close(downloadCompleted)
		})

		// Wait until download is actively in-flight
		<-downloadStarted

		// Now launch external waiters while download/callbacks are in progress
		for w := 0; w < numExternalWaiters; w++ {
			go func() {
				fileMsg.WaitDownload()
				// When WaitDownload unblocks, all callbacks MUST be complete!
				if !cb1Finished.Load() || !cb2Finished.Load() || !cb3Finished.Load() {
					prematureReturns.Add(1)
				}
				externalWaitersDone.Done()
			}()
		}

		<-downloadCompleted
		externalWaitersDone.Wait()

		if prematureReturns.Load() > 0 {
			t.Fatalf("Iteration %d: %d external waiters returned before all chained callbacks finished!", i, prematureReturns.Load())
		}
		if !cb1Finished.Load() || !cb2Finished.Load() || !cb3Finished.Load() {
			t.Fatalf("Iteration %d: not all callbacks finished! cb1=%v cb2=%v cb3=%v", i, cb1Finished.Load(), cb2Finished.Load(), cb3Finished.Load())
		}
	}
}

// 2. Stress: Registrars initiate download first, then concurrent WaitDownloads
func TestChallenger2_It4_MassiveConcurrentRegistrationsAndWaiters(t *testing.T) {
	pngData := makeChallengerPNG(16, 16)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "massive_concurrency.png",
		URL:     "mxc://example.com/massive",
	}
	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$massive"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadStarted := make(chan struct{})
	var startOnce sync.Once
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		startOnce.Do(func() {
			close(downloadStarted)
		})
		time.Sleep(20 * time.Millisecond)
		return pngData, nil
	})

	const numRegistrars = 50
	const numWaiters = 50
	var executedCallbacks atomic.Int32
	var prematureWaiters atomic.Int32
	var totalCallbacksExpected atomic.Int32

	// Launch initial download to ensure active state
	totalCallbacksExpected.Add(1)
	fileMsg.DownloadPreview(func() {
		executedCallbacks.Add(1)
	})

	<-downloadStarted

	var doneWg sync.WaitGroup
	doneWg.Add(numRegistrars + numWaiters)

	for i := 0; i < numRegistrars; i++ {
		go func(idx int) {
			totalCallbacksExpected.Add(1)
			fileMsg.DownloadPreview(func() {
				executedCallbacks.Add(1)
			})
			doneWg.Done()
		}(i)
	}

	for i := 0; i < numWaiters; i++ {
		go func(idx int) {
			fileMsg.WaitDownload()
			// Check if download actually produced image data
			if len(fileMsg.ImageData()) == 0 {
				prematureWaiters.Add(1)
			}
			doneWg.Done()
		}(i)
	}

	doneWg.Wait()
	// Final wait to ensure everything is drained
	fileMsg.WaitDownload()

	if prematureWaiters.Load() > 0 {
		t.Errorf("FAIL: %d waiters returned with empty ImageData", prematureWaiters.Load())
	}
	if executedCallbacks.Load() != totalCallbacksExpected.Load() {
		t.Errorf("FAIL: expected %d callbacks executed, got %d", totalCallbacksExpected.Load(), executedCallbacks.Load())
	}
}

// 3. Stress: Error handling with concurrent WaitDownload and SetOnDownloadComplete
func TestChallenger2_It4_DownloadErrorWithConcurrentWaiters(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "error_waiters.png",
		URL:     "mxc://example.com/err_wait",
	}
	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$err_wait"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadStarted := make(chan struct{})
	expectedErr := errors.New("simulated network 500 error")
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		close(downloadStarted)
		time.Sleep(15 * time.Millisecond)
		return nil, expectedErr
	})

	const numWaiters = 30
	var wg sync.WaitGroup
	wg.Add(numWaiters)
	var onCompleteFired atomic.Bool

	fileMsg.SetOnDownloadComplete(func() {
		onCompleteFired.Store(true)
	})

	fileMsg.DownloadPreview()
	<-downloadStarted

	for i := 0; i < numWaiters; i++ {
		go func() {
			defer wg.Done()
			fileMsg.WaitDownload()
			if !onCompleteFired.Load() {
				t.Errorf("WaitDownload returned before SetOnDownloadComplete fired!")
			}
		}()
	}

	wg.Wait()
	fileMsg.WaitDownload()

	if !onCompleteFired.Load() {
		t.Errorf("SetOnDownloadComplete was never executed on error!")
	}
	if fileMsg.ImageData() != nil {
		t.Errorf("ImageData() should be nil on error")
	}
}
