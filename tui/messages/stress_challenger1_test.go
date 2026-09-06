// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Empirical Stress Harness by Challenger 1 for Milestone M2 Recheck (Iteration 2).
// Specifically targets:
// 1. Simultaneous CalculateBuffer() on UI thread and InvalidateBuffer() on background threads under race detector.
// 2. High-contention concurrent SetOnDownloadComplete() vs download completion lifecycle.
// 3. Callback self-mutation, re-entry, and Clone under high concurrency.

package messages

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/tui/config"
)

// Helper to generate a test PNG image for Challenger 1 stress tests.
func makeChallenger1PNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8((x * 7) % 256), G: uint8((y * 11) % 256), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}



// ---------------------------------------------------------------------------
// 1. Stress: CalculateBuffer on UI thread vs InvalidateBuffer on background threads
// ---------------------------------------------------------------------------
func TestChallenger1_CalculateBuffer_vs_InvalidateBuffer_Stress(t *testing.T) {
	pngData := makeChallenger1PNG(40, 40)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "stress_calc_inv.png",
		URL:     "mxc://example.com/stress1",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$stress1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)
	fileMsg.SetImageData(pngData)

	var stop atomic.Bool
	var wg sync.WaitGroup

	prefs := config.UserPreferences{}

	// UI Thread: repeatedly calls CalculateBuffer with alternating widths, plus Height and Draw
	wg.Add(1)
	go func() {
		defer wg.Done()
		widths := []int{40, 60, 80, 100, 120}
		idx := 0
		screen := newSimulationScreen(120, 40)
		for !stop.Load() {
			w := widths[idx%len(widths)]
			idx++
			uiMsg.CalculateBuffer(prefs, w)
			_ = uiMsg.Height()
			_ = uiMsg.BufferedWidth()
			uiMsg.Draw(screen)
		}
	}()

	// Background Thread 1 & 2: Direct InvalidateBuffer() calls
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				uiMsg.InvalidateBuffer()
				time.Sleep(50 * time.Microsecond)
			}
		}()
	}

	// Background Thread 3 & 4: SetImageData calls (which internally call InvalidateBuffer())
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.SetImageData(pngData)
				time.Sleep(100 * time.Microsecond)
			}
		}()
	}

	// Background Thread 5: Reading BufferedWidth
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			_ = uiMsg.BufferedWidth()
			time.Sleep(20 * time.Microsecond)
		}
	}()

	// Background Thread 6: Reading ImageData copy
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			_ = fileMsg.ImageData()
			time.Sleep(100 * time.Microsecond)
		}
	}()

	// Run under race detector for 500ms
	time.Sleep(500 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 2. Stress: Concurrent SetOnDownloadComplete vs Download Completion
// ---------------------------------------------------------------------------
func TestChallenger1_SetOnDownloadComplete_vs_DownloadCompletion_Stress(t *testing.T) {
	pngData := makeChallenger1PNG(30, 30)

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Run against multiple distinct messages to stress independent lifecycle instances
	numMessages := 5
	messages := make([]*FileMessage, numMessages)
	uiMessages := make([]*UIMessage, numMessages)

	for i := 0; i < numMessages; i++ {
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    "multi_stress.png",
			URL:     "mxc://example.com/multistress",
		}
		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: id.EventID(fmt.Sprintf("$event%d", i))}, content)
		uiMessages[i] = uiMsg
		messages[i] = uiMsg.Renderer.(*FileMessage)
		// Setup fast download func
		messages[i].SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			time.Sleep(100 * time.Microsecond)
			return pngData, nil
		})
	}

	var callbackCount atomic.Int64

	// Workers setting SetOnDownloadComplete continuously
	for i := 0; i < numMessages; i++ {
		msg := messages[i]
		wg.Add(1)
		go func(m *FileMessage) {
			defer wg.Done()
			for !stop.Load() {
				m.SetOnDownloadComplete(func() {
					callbackCount.Add(1)
					// Verify safe operations inside callback
					_ = m.IsDownloading()
					_ = m.ImageData()
				})
				time.Sleep(200 * time.Microsecond)
			}
		}(msg)
	}

	// Workers triggering DownloadPreview continuously with dynamic onDone
	for i := 0; i < numMessages; i++ {
		msg := messages[i]
		wg.Add(1)
		go func(m *FileMessage) {
			defer wg.Done()
			for !stop.Load() {
				m.DownloadPreview(func() {
					callbackCount.Add(1)
				})
				time.Sleep(300 * time.Microsecond)
			}
		}(msg)
	}

	// Workers waiting on download
	for i := 0; i < numMessages; i++ {
		msg := messages[i]
		wg.Add(1)
		go func(m *FileMessage) {
			defer wg.Done()
			for !stop.Load() {
				m.WaitDownload()
				time.Sleep(250 * time.Microsecond)
			}
		}(msg)
	}

	time.Sleep(600 * time.Millisecond)
	stop.Store(true)
	wg.Wait()

	// Ensure all downloads are completed
	for _, m := range messages {
		m.WaitDownload()
	}

	if callbackCount.Load() == 0 {
		t.Fatalf("Expected callbacks to be fired, got 0")
	}
}

// ---------------------------------------------------------------------------
// 3. Stress: Callback Self-Mutation & Re-entry inside onDownloadComplete
// ---------------------------------------------------------------------------
func TestChallenger1_CallbackSelfMutation_Stress(t *testing.T) {
	pngData := makeChallenger1PNG(20, 20)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "self_mutate.png",
		URL:     "mxc://example.com/self_mutate",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$self1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(50 * time.Microsecond)
		return pngData, nil
	})

	var completionInvoked atomic.Int32
	var innerCompleted atomic.Bool

	// Set a callback that mutates SetOnDownloadComplete and invokes WaitDownload & CalculateBuffer
	fileMsg.SetOnDownloadComplete(func() {
		completionInvoked.Add(1)
		// 1. Mutate SetOnDownloadComplete from inside the callback
		fileMsg.SetOnDownloadComplete(func() {
			innerCompleted.Store(true)
		})
		// 2. WaitDownload should return immediately without deadlock
		fileMsg.WaitDownload()
		// 3. IsDownloading should be false
		if fileMsg.IsDownloading() {
			t.Errorf("IsDownloading() was true inside onDownloadComplete callback")
		}
		// 4. CalculateBuffer on UI message
		uiMsg.CalculateBuffer(config.UserPreferences{}, 60)
		// 5. InvalidateBuffer
		uiMsg.InvalidateBuffer()
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	if completionInvoked.Load() != 1 {
		t.Fatalf("Expected initial completion to run once, ran %d times", completionInvoked.Load())
	}

	// Trigger second download or completion to test mutated callback
	// Clear imageData to allow second download
	fileMsg.mu.Lock()
	fileMsg.imageData = nil
	fileMsg.mu.Unlock()

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	if !innerCompleted.Load() {
		t.Fatalf("Expected mutated inner completion callback to have run")
	}
}

// ---------------------------------------------------------------------------
// 4. Stress: High Contention Mixed Operations (CalculateBuffer, Invalidate,
//    Download, SetImageData) WITHOUT Clone across 15 Goroutines
// ---------------------------------------------------------------------------
func TestChallenger1_HighContentionNoClone_Stress(t *testing.T) {
	pngData := makeChallenger1PNG(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "mixed_stress.png",
		URL:     "mxc://example.com/mixed",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$mixed_noclone"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(50 * time.Microsecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	prefs := config.UserPreferences{}

	// Goroutine group 1: UI Thread operations (CalculateBuffer, Height, BufferedWidth)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w := 50 + workerID*10
			for !stop.Load() {
				uiMsg.CalculateBuffer(prefs, w)
				_ = uiMsg.Height()
				_ = uiMsg.BufferedWidth()
				time.Sleep(40 * time.Microsecond)
			}
		}(i)
	}

	// Goroutine group 2: InvalidateBuffer on background threads
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				uiMsg.InvalidateBuffer()
				time.Sleep(50 * time.Microsecond)
			}
		}()
	}

	// Goroutine group 3: DownloadPreview and WaitDownload
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.DownloadPreview(func() {})
				fileMsg.WaitDownload()
				time.Sleep(80 * time.Microsecond)
			}
		}()
	}

	// Goroutine group 4: SetImageData and ImageError
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.SetImageData(pngData)
				_ = fileMsg.ImageError()
				time.Sleep(60 * time.Microsecond)
			}
		}()
	}

	// Goroutine group 5: SetOnDownloadComplete
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.SetOnDownloadComplete(func() {})
				time.Sleep(70 * time.Microsecond)
			}
		}()
	}

	time.Sleep(500 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// 4b. Stress: High Contention Mixed Operations WITH Continuous Clone() Calls
//     Simultaneous CalculateBuffer on UI thread, InvalidateBuffer on background threads,
//     continuous Clone calls + rendering of clones, active downloads, and SetImageData.
// ---------------------------------------------------------------------------
func TestChallenger1_HighContentionMixedOperations_Stress(t *testing.T) {
	pngData := makeChallenger1PNG(48, 48)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "mixed_clone_stress.png",
		URL:     "mxc://example.com/mixedclone",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$mixed_clone_stress"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(50 * time.Microsecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	prefs := config.UserPreferences{}

	// Goroutine group 1: UI Thread operations on original uiMsg (CalculateBuffer, Height, BufferedWidth, Draw)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w := 50 + workerID*10
			screen := newSimulationScreen(100, 30)
			for !stop.Load() {
				uiMsg.CalculateBuffer(prefs, w)
				_ = uiMsg.Height()
				_ = uiMsg.BufferedWidth()
				uiMsg.Draw(screen)
				time.Sleep(40 * time.Microsecond)
			}
		}(i)
	}

	// Goroutine group 2: InvalidateBuffer on background threads
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				uiMsg.InvalidateBuffer()
				time.Sleep(50 * time.Microsecond)
			}
		}()
	}

	// Goroutine group 3: Continuous Clone() calls + operations on cloned messages
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w := 60 + workerID*8
			screen := newSimulationScreen(100, 30)
			for !stop.Load() {
				clone := uiMsg.Clone()
				if clone == nil {
					t.Errorf("FAIL: Clone returned nil")
					return
				}
				cloneRenderer, ok := clone.Renderer.(*FileMessage)
				if !ok || cloneRenderer.uiMsg != clone {
					t.Errorf("FAIL: Cloned uiMsg backreference misaligned or nil")
					return
				}
				clone.CalculateBuffer(prefs, w)
				_ = clone.Height()
				_ = clone.BufferedWidth()
				clone.Draw(screen)
				clone.InvalidateBuffer()
				time.Sleep(60 * time.Microsecond)
			}
		}(i)
	}

	// Goroutine group 4: DownloadPreview and WaitDownload
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.DownloadPreview(func() {})
				fileMsg.WaitDownload()
				time.Sleep(80 * time.Microsecond)
			}
		}()
	}

	// Goroutine group 5: SetImageData and ImageError
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.SetImageData(pngData)
				_ = fileMsg.ImageError()
				time.Sleep(60 * time.Microsecond)
			}
		}()
	}

	// Goroutine group 6: SetOnDownloadComplete
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.SetOnDownloadComplete(func() {})
				time.Sleep(70 * time.Microsecond)
			}
		}()
	}

	time.Sleep(600 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// 5. Minimal Isolation: UIMessage.Clone() vs InvalidateBuffer() Data Race
// ---------------------------------------------------------------------------
// Demonstrates that `clone := *msg` in UIMessage.Clone() performs an unsynchronized
// non-atomic read of msg.bufferedWidth while background goroutines perform atomic.StoreInt32.
func TestChallenger1_UIMessageClone_vs_InvalidateBuffer_DataRace(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "clone_race.png",
		URL:     "mxc://example.com/clonerace",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$clonerace"}, content)

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Goroutine 1: UI or background goroutine cloning uiMsg
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			_ = uiMsg.Clone()
		}
	}()

	// Goroutine 2: InvalidateBuffer() writing atomically to msg.bufferedWidth
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
// 6. Stress: Failing Downloads under Concurrency
// ---------------------------------------------------------------------------
func TestChallenger1_ConcurrentFailingDownloads_Stress(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "failing_stress.png",
		URL:     "mxc://example.com/fail",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$fail"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadErr := errors.New("network failure")
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(100 * time.Microsecond)
		return nil, downloadErr
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.DownloadPreview(func() {
					_ = fileMsg.ImageError()
					_ = fileMsg.IsDownloading()
				})
				time.Sleep(200 * time.Microsecond)
			}
		}()
	}

	time.Sleep(400 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()

	if fileMsg.ImageError() == nil {
		t.Fatalf("Expected error to be recorded, got nil")
	}
}

// ---------------------------------------------------------------------------
// 7. Stress: Simultaneous CalculateBuffer on UI thread, InvalidateBuffer on
//    background threads, and continuous Clone() calls under Go race detector.
// ---------------------------------------------------------------------------
func TestChallenger1_Simultaneous_Calculate_Invalidate_Clone_Stress(t *testing.T) {
	pngData := makeChallenger1PNG(60, 60)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "trio_stress.png",
		URL:     "mxc://example.com/triostress",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$trio_stress"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)
	fileMsg.SetImageData(pngData)

	var stop atomic.Bool
	var wg sync.WaitGroup

	prefs := config.UserPreferences{}

	// UI Thread: repeatedly calls CalculateBuffer with varying widths, calls Height, BufferedWidth, Draw
	wg.Add(1)
	go func() {
		defer wg.Done()
		widths := []int{40, 60, 80, 100, 120, 140}
		idx := 0
		screen := newSimulationScreen(140, 50)
		for !stop.Load() {
			w := widths[idx%len(widths)]
			idx++
			uiMsg.CalculateBuffer(prefs, w)
			_ = uiMsg.Height()
			_ = uiMsg.BufferedWidth()
			uiMsg.Draw(screen)
		}
	}()

	// Background Threads 1-3: Tight loop InvalidateBuffer() calls
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				uiMsg.InvalidateBuffer()
			}
		}()
	}

	// Background Threads 4-7: Tight loop Clone() calls + independent clone mutations & rendering
	var totalClones atomic.Int64
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			w := 50 + workerID*15
			screen := newSimulationScreen(120, 40)
			for !stop.Load() {
				clone := uiMsg.Clone()
				if clone == nil {
					t.Errorf("FAIL: Clone returned nil")
					return
				}
				cloneRenderer, ok := clone.Renderer.(*FileMessage)
				if !ok || cloneRenderer.uiMsg != clone {
					t.Errorf("FAIL: Clone Renderer backreference mismatch")
					return
				}
				clone.CalculateBuffer(prefs, w)
				_ = clone.Height()
				_ = clone.BufferedWidth()
				clone.Draw(screen)
				clone.InvalidateBuffer()
				totalClones.Add(1)
			}
		}(i)
	}

	// Background Threads 8-9: Periodic SetImageData calls (forcing buffer invalidation under lock)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.SetImageData(pngData)
				time.Sleep(50 * time.Microsecond)
			}
		}()
	}

	// Run under race detector for 1.0 second
	time.Sleep(1000 * time.Millisecond)
	stop.Store(true)
	wg.Wait()

	if totalClones.Load() < 100 {
		t.Fatalf("Expected at least 100 clones during stress run, got %d", totalClones.Load())
	}
}
