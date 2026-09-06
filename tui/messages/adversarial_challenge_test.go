// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Adversarial challenge test suite for Milestone M2 (Async Media Lifecycle & Thread Safety)

package messages

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
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
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/lib/termimg"
)

// Helper to generate minimal valid PNG
func makeAdvPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 64, B: 32, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Helper to create PNG with specific header width & height
func makeAdvPNGHeader(w, h uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], w)
	binary.BigEndian.PutUint32(ihdrData[4:8], h)
	ihdrData[8] = 8
	ihdrData[9] = 2
	ihdrData[10] = 0
	ihdrData[11] = 0
	ihdrData[12] = 0

	var lenBytes [4]byte
	binary.BigEndian.PutUint32(lenBytes[:], 13)
	buf.Write(lenBytes[:])

	chunkType := []byte("IHDR")
	buf.Write(chunkType)
	buf.Write(ihdrData)

	crc := crc32.NewIEEE()
	crc.Write(chunkType)
	crc.Write(ihdrData)
	var crcBytes [4]byte
	binary.BigEndian.PutUint32(crcBytes[:], crc.Sum32())
	buf.Write(crcBytes[:])

	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// 1. Concurrency Race: UIMessage.CalculateBuffer vs DownloadPreview / SetImageData
// ---------------------------------------------------------------------------
// In real Gomuks operation, the UI loop calls uiMsg.CalculateBuffer(...) on UIMessage,
// while background goroutines mutate uiMsg.bufferedWidth = 0.
func TestAdversarial_UIMessageCalculateBuffer_DataRace(t *testing.T) {
	pngData := makeAdvPNG(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "race_test.png",
		URL:     "mxc://example.com/race",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$race1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(2 * time.Millisecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	// UI thread simulation calling uiMsg.CalculateBuffer
	wg.Add(1)
	go func() {
		defer wg.Done()
		prefs := config.UserPreferences{}
		for !stop.Load() {
			uiMsg.CalculateBuffer(prefs, 80)
			_ = uiMsg.Height()
			time.Sleep(100 * time.Microsecond)
		}
	}()

	// Background download / setter simulation
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			fileMsg.SetImageData(pngData)
			time.Sleep(500 * time.Microsecond)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 2. Concurrency Race on SetDownloadFunc & SetOnDownloadComplete vs Worker
// ---------------------------------------------------------------------------
func TestAdversarial_CallbackSetters_DataRace(t *testing.T) {
	pngData := makeAdvPNG(16, 16)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "callback_race.png",
		URL:     "mxc://example.com/cb",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$cb1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Goroutine mutating downloadFn and onDownloadComplete
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
				return pngData, nil
			})
			fileMsg.SetOnDownloadComplete(func() {})
			time.Sleep(500 * time.Microsecond)
		}
	}()

	// Goroutine calling DownloadPreview repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			fileMsg.DownloadPreview()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// 3. Callback Lifecycle & IsDownloading() State Contract
// ---------------------------------------------------------------------------
// When onDone() or Redraw() runs, is the download actually considered finished?
func TestAdversarial_IsDownloadingStateInCallback(t *testing.T) {
	pngData := makeAdvPNG(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "lifecycle.png",
		URL:     "mxc://example.com/life",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$life1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return pngData, nil
	})

	var isDownloadingDuringCallback bool
	doneCh := make(chan struct{})

	fileMsg.DownloadPreview(func() {
		isDownloadingDuringCallback = fileMsg.IsDownloading()
		close(doneCh)
	})

	select {
	case <-doneCh:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for callback")
	}

	if isDownloadingDuringCallback {
		t.Errorf("FAIL: FileMessage.IsDownloading() returned true inside onDone callback; expected false because download completed")
	}
}

// ---------------------------------------------------------------------------
// 4. Deadlock Challenge: WaitDownload called within onDone
// ---------------------------------------------------------------------------
func TestAdversarial_WaitDownloadDeadlock(t *testing.T) {
	pngData := makeAdvPNG(16, 16)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "deadlock.png",
		URL:     "mxc://example.com/deadlock",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$dl1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return pngData, nil
	})

	doneCh := make(chan struct{})
	go func() {
		fileMsg.DownloadPreview(func() {
			// If onDone is executed before wg.Done(), calling WaitDownload on another goroutine or
			// expecting WaitDownload to finish promptly should be checked.
			// Specifically, within onDone goroutine, WaitDownload will deadlock if it waits on its own goroutine's defer!
			ch := make(chan struct{})
			go func() {
				fileMsg.WaitDownload()
				close(ch)
			}()
			select {
			case <-ch:
				// successfully waited
			case <-time.After(500 * time.Millisecond):
				t.Errorf("FAIL: WaitDownload deadlocked or stalled during onDone execution")
			}
			close(doneCh)
		})
	}()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("test timed out")
	}
}

// ---------------------------------------------------------------------------
// 4b. WaitGroup Concurrent Add & Wait Race on Retry
// ---------------------------------------------------------------------------
func TestAdversarial_WaitGroupConcurrentAddWait(t *testing.T) {
	pngData := makeAdvPNG(16, 16)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "wg_race.png",
		URL:     "mxc://example.com/wgrace",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$wgrace"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		time.Sleep(1 * time.Millisecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Goroutine calling WaitDownload
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fileMsg.WaitDownload()
				time.Sleep(500 * time.Microsecond)
			}
		}()
	}

	// Goroutine calling DownloadPreview
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				// Reset imageData to allow retry
				fileMsg.SetImageData(nil)
				fileMsg.DownloadPreview()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// 5. Reentrant / Duplicate DownloadPreview in onDone
// ---------------------------------------------------------------------------
func TestAdversarial_DownloadPreviewReentrancy(t *testing.T) {
	pngData := makeAdvPNG(16, 16)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "reenter.png",
		URL:     "mxc://example.com/reenter",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$re1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	var downloadAttempts atomic.Int32
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		downloadAttempts.Add(1)
		return pngData, nil
	})

	doneCh := make(chan struct{})
	// Initial download
	fileMsg.DownloadPreview(func() {
		// When initial download completes, call DownloadPreview again
		fileMsg.DownloadPreview(func() {
			close(doneCh)
		})
	})

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("reentrant DownloadPreview timed out or deadlocked")
	}

	if downloadAttempts.Load() != 1 {
		t.Errorf("expected exactly 1 download attempt, got %d", downloadAttempts.Load())
	}
}

// ---------------------------------------------------------------------------
// 6. CalculateBuffer Boundary & Extreme Layout Conditions
// ---------------------------------------------------------------------------
func TestAdversarial_CalculateBuffer_NarrowWidth(t *testing.T) {
	pngData := makeAdvPNG(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "narrow.png",
		URL:     "mxc://example.com/narrow",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$nar1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{}

	// Test boundary widths: 0, 1, 2, 3, 4, 5, 6
	for w := 0; w <= 10; w++ {
		uiMsg.CalculateBuffer(prefs, w)
		h := uiMsg.Height()
		// None of these should panic
		if w >= 2 && h < 0 {
			t.Errorf("unexpected negative height for width %d: %d", w, h)
		}
	}
}

// ---------------------------------------------------------------------------
// 7. Decompression Bomb Boundary: Exactly 16MP vs 16MP + 1 pixel
// ---------------------------------------------------------------------------
func TestAdversarial_DecompressionBombExactBoundaries(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "boundary.png",
		URL:     "mxc://example.com/boundary",
	}

	t.Run("16,000,000 pixels (4000x4000) allowed", func(t *testing.T) {
		header := makeAdvPNGHeader(4000, 4000)
		_, err := termimg.ValidateImageSafety(bytes.NewReader(header))
		if err != nil {
			t.Errorf("expected 16MP (4000x4000 = 16,000,000) to be allowed, got error: %v", err)
		}
	})

	t.Run("16,000,001 pixels rejected", func(t *testing.T) {
		// 4000 * 4001 = 16,004,000 (> 16,000,000)
		header := makeAdvPNGHeader(4000, 4001)
		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$b1"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)
		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			return header, nil
		})
		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		if fileMsg.ImageData() != nil {
			t.Errorf("expected payload exceeding 16MP to be rejected")
		}
		if !errors.Is(fileMsg.ImageError(), termimg.ErrImageTooLarge) {
			t.Errorf("expected ErrImageTooLarge, got %v", fileMsg.ImageError())
		}
	})

	t.Run("65536 x 65536 integer overflow check", func(t *testing.T) {
		header := makeAdvPNGHeader(65536, 65536)
		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$b2"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)
		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			return header, nil
		})
		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		if fileMsg.ImageData() != nil {
			t.Errorf("expected payload with overflow dimensions to be rejected")
		}
		if !errors.Is(fileMsg.ImageError(), termimg.ErrImageTooLarge) {
			t.Errorf("expected ErrImageTooLarge, got %v", fileMsg.ImageError())
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Goroutine Leak Check on Parser & Download
// ---------------------------------------------------------------------------
func TestAdversarial_ParserDownloadLeak(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "leak_test.png",
		URL:     "mxc://example.com/leak",
	}

	meta := &database.Room{ID: "!leak:example.com"}
	roomStore := store.NewRoomStore(nil, meta)
	evt := &database.Event{ID: "$leak:example.com"}

	uiMsg := NewFileMessage(roomStore, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	// Custom download function that succeeds
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return makeAdvPNG(10, 10), nil
	})

	fileMsg.DownloadPreview()
	// Must be able to cleanly wait without hanging
	fileMsg.WaitDownload()

	if len(fileMsg.ImageData()) == 0 {
		t.Errorf("expected image data to be populated")
	}
}
