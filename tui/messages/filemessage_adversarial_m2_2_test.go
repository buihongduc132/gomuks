// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Adversarial Empirical Challenge Suite by Challenger 2 for Milestone M2.

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

// Helper to create valid PNG of given dimension
func makeChallengerPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Helper to craft PNG header with arbitrary width/height without allocating pixel buffers
func makeChallengerPNGHeader(w, h uint32) []byte {
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
// Challenge 1: Encrypted vs Unencrypted Thumbnail Routing
// ---------------------------------------------------------------------------
func TestChallenger2_EncryptedThumbnailFlagRouting(t *testing.T) {
	pngData := makeChallengerPNG(32, 24)

	t.Run("Encrypted thumbnail routes encrypted=true", func(t *testing.T) {
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    "enc_thumb.png",
			URL:     "mxc://example.com/enc_full",
			Info: &event.FileInfo{
				ThumbnailFile: &event.EncryptedFileInfo{
					URL: "mxc://example.com/enc_thumb",
				},
			},
		}

		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$enc1"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var capturedURI id.ContentURI
		var capturedEnc bool
		var mu sync.Mutex

		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			mu.Lock()
			capturedURI = uri
			capturedEnc = encrypted
			mu.Unlock()
			return pngData, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		mu.Lock()
		defer mu.Unlock()

		if capturedURI.String() != "mxc://example.com/enc_thumb" {
			t.Errorf("expected URI mxc://example.com/enc_thumb, got %s", capturedURI.String())
		}
		if !capturedEnc {
			t.Errorf("expected encrypted=true for ThumbnailFile, got false")
		}
	})

	t.Run("Unencrypted thumbnail routes encrypted=false", func(t *testing.T) {
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    "plain_thumb.png",
			URL:     "mxc://example.com/plain_full",
			Info: &event.FileInfo{
				ThumbnailURL: "mxc://example.com/plain_thumb",
			},
		}

		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$plain1"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var capturedURI id.ContentURI
		var capturedEnc bool
		var mu sync.Mutex

		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			mu.Lock()
			capturedURI = uri
			capturedEnc = encrypted
			mu.Unlock()
			return pngData, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		mu.Lock()
		defer mu.Unlock()

		if capturedURI.String() != "mxc://example.com/plain_thumb" {
			t.Errorf("expected URI mxc://example.com/plain_thumb, got %s", capturedURI.String())
		}
		if capturedEnc {
			t.Errorf("expected encrypted=false for ThumbnailURL, got true")
		}
	})
}

// ---------------------------------------------------------------------------
// Challenge 2: Non-Image Media Ingestion & Fallback Guard
// ---------------------------------------------------------------------------
func TestChallenger2_NonImageMediaThumbnailHandling(t *testing.T) {
	thumbPNG := makeChallengerPNG(64, 48)

	t.Run("Video with thumbnail downloads thumbnail only", func(t *testing.T) {
		content := &event.MessageEventContent{
			MsgType: event.MsgVideo,
			Body:    "video.mp4",
			URL:     "mxc://example.com/heavy_video.mp4",
			Info: &event.FileInfo{
				ThumbnailURL: "mxc://example.com/video_thumb.png",
			},
		}

		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$v1"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var requestedURIs []string
		var mu sync.Mutex

		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			mu.Lock()
			requestedURIs = append(requestedURIs, uri.String())
			mu.Unlock()
			return thumbPNG, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		mu.Lock()
		defer mu.Unlock()

		if len(requestedURIs) != 1 || requestedURIs[0] != "mxc://example.com/video_thumb.png" {
			t.Fatalf("expected only thumbnail to be requested, got: %v", requestedURIs)
		}
		if !bytes.Equal(fileMsg.ImageData(), thumbPNG) {
			t.Errorf("expected stored imageData to match thumbnail bytes")
		}
	})

	t.Run("Video with failing thumbnail does NOT download heavy full video", func(t *testing.T) {
		content := &event.MessageEventContent{
			MsgType: event.MsgVideo,
			Body:    "movie.mp4",
			URL:     "mxc://example.com/1gb_movie.mp4",
			Info: &event.FileInfo{
				ThumbnailURL: "mxc://example.com/failing_thumb.png",
			},
		}

		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$v2"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var requestedURIs []string
		var mu sync.Mutex

		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			mu.Lock()
			requestedURIs = append(requestedURIs, uri.String())
			mu.Unlock()
			return nil, errors.New("thumbnail 404")
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		mu.Lock()
		defer mu.Unlock()

		if len(requestedURIs) != 1 {
			t.Fatalf("CRITICAL: Non-image media attempted fallback to full file! Requested: %v", requestedURIs)
		}
		if requestedURIs[0] != "mxc://example.com/failing_thumb.png" {
			t.Errorf("expected request to be thumbnail, got: %s", requestedURIs[0])
		}
		if fileMsg.ImageData() != nil {
			t.Errorf("expected imageData to be nil")
		}
	})

	t.Run("Video without thumbnail skips download entirely", func(t *testing.T) {
		content := &event.MessageEventContent{
			MsgType: event.MsgVideo,
			Body:    "nothumb.mp4",
			URL:     "mxc://example.com/nothumb.mp4",
		}

		uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$v3"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var downloaded atomic.Bool
		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			downloaded.Store(true)
			return nil, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		if downloaded.Load() {
			t.Errorf("expected video without thumbnail to never trigger downloadFn")
		}
	})
}

// ---------------------------------------------------------------------------
// Challenge 3: Fallback when Thumbnail is Decompression Bomb
// ---------------------------------------------------------------------------
func TestChallenger2_ThumbnailBombFallbackToValidFullMedia(t *testing.T) {
	// 50,000 x 50,000 = 2,500 Megapixels bomb
	bombThumb := makeChallengerPNGHeader(50000, 50000)
	validFull := makeChallengerPNG(200, 150)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "photo.png",
		URL:     "mxc://example.com/good_full.png",
		Info: &event.FileInfo{
			ThumbnailURL: "mxc://example.com/bomb_thumb.png",
		},
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$bombfb1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		if uri.FileID == "bomb_thumb.png" {
			return bombThumb, nil
		}
		if uri.FileID == "good_full.png" {
			return validFull, nil
		}
		return nil, errors.New("unknown URI")
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	if !bytes.Equal(fileMsg.ImageData(), validFull) {
		t.Fatalf("expected fallback to good full image when thumbnail is a decompression bomb")
	}
	if fileMsg.ImageError() != nil {
		t.Errorf("expected ImageError to be cleared after successful fallback, got: %v", fileMsg.ImageError())
	}
}

// ---------------------------------------------------------------------------
// Challenge 4: Both Thumbnail and Full Media are Decompression Bombs
// ---------------------------------------------------------------------------
func TestChallenger2_BothThumbnailAndFullAreBombs(t *testing.T) {
	bombThumb := makeChallengerPNGHeader(20000, 20000) // 400MP
	bombFull := makeChallengerPNGHeader(30000, 30000)  // 900MP

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "double_bomb.png",
		URL:     "mxc://example.com/bomb_full.png",
		Info: &event.FileInfo{
			ThumbnailURL: "mxc://example.com/bomb_thumb.png",
		},
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$double1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		if uri.FileID == "bomb_thumb.png" {
			return bombThumb, nil
		}
		return bombFull, nil
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	// Neither payload should be accepted into imageData
	if fileMsg.ImageData() != nil {
		t.Fatalf("CRITICAL: Decompression bomb was accepted into imageData!")
	}
	if fileMsg.ImageError() == nil {
		t.Fatalf("expected ImageError to be set when both are bombs")
	}
	if !errors.Is(fileMsg.ImageError(), termimg.ErrImageTooLarge) {
		t.Errorf("expected ErrImageTooLarge, got: %v", fileMsg.ImageError())
	}

	// Rendering buffer must handle without panic
	prefs := config.UserPreferences{}
	uiMsg.CalculateBuffer(prefs, 80)
	if uiMsg.Height() < 1 {
		t.Errorf("expected height >= 1 for fallback display, got %d", uiMsg.Height())
	}
}

// ---------------------------------------------------------------------------
// Challenge 5: Inactive Room Image Storage & Deferred Buffer Invalidation
// ---------------------------------------------------------------------------
func TestChallenger2_InactiveRoomDeferredRendering(t *testing.T) {
	pngData := makeChallengerPNG(60, 40)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "deferred.png",
		URL:     "mxc://example.com/def.png",
	}

	activeRoomID := id.RoomID("!room_A:example.com")
	currentRoomID := id.RoomID("!room_B:example.com") // inactive

	meta := &database.Room{ID: currentRoomID}
	roomStore := store.NewRoomStore(nil, meta)

	origChecker := ActiveRoomChecker
	origRedraw := RequestRedraw
	defer func() {
		ActiveRoomChecker = origChecker
		RequestRedraw = origRedraw
	}()

	ActiveRoomChecker = func(roomID id.RoomID) bool {
		return roomID == activeRoomID
	}

	var redrawCalled atomic.Bool
	RequestRedraw = func(roomID id.RoomID) {
		redrawCalled.Store(true)
	}

	uiMsg := NewFileMessage(roomStore, nil, &database.Event{ID: "$def1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return pngData, nil
	})

	// Pre-calculate buffer as plaintext before download
	prefs := config.UserPreferences{}
	uiMsg.CalculateBuffer(prefs, 80)
	if uiMsg.Height() != 1 {
		t.Fatalf("expected initial height 1, got %d", uiMsg.Height())
	}

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	// Redraw should NOT have been called because room_B is inactive
	if redrawCalled.Load() {
		t.Errorf("expected RequestRedraw to be suppressed for inactive room_B")
	}

	// But image data MUST be safely stored
	if !bytes.Equal(fileMsg.ImageData(), pngData) {
		t.Errorf("expected imageData to be stored even when room was inactive")
	}

	// User switches to room_B: CalculateBuffer is called
	uiMsg.CalculateBuffer(prefs, 80)
	if uiMsg.Height() <= 1 {
		t.Errorf("expected height > 1 after switching to room with downloaded media, got %d", uiMsg.Height())
	}
}

// ---------------------------------------------------------------------------
// Challenge 6: Dimension Clamping & Bounding Box Scaling (R4 Verification)
// ---------------------------------------------------------------------------
// Requirement R4: "Enforce 2D bounding box scaling adhering to iamb defaults
// (default width 66 columns, max height 16 rows, min height 3 rows)."
// Let's test what CalculateBuffer actually produces for huge images (e.g. 1920x1080).
func TestChallenger2_DimensionClampingIambParity(t *testing.T) {
	// 400x200 image in an 80-column terminal
	imgData := makeChallengerPNG(400, 200)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "large.png",
		URL:     "mxc://example.com/large.png",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$dim1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)
	fileMsg.SetImageData(imgData)

	prefs := config.UserPreferences{}
	uiMsg.CalculateBuffer(prefs, 80)

	h := uiMsg.Height()
	t.Logf("Calculated image height for 400x200 in 80 cols: %d rows", h)

	// Check if height exceeds iamb default max height (16 rows)
	if h > termimg.DefaultMaxHeight {
		t.Errorf("R4 VIOLATION: Image height %d exceeds DefaultMaxHeight %d", h, termimg.DefaultMaxHeight)
	}

	// Check if height respects min height (3 rows)
	if h < termimg.MinHeight {
		t.Errorf("R4 VIOLATION: Image height %d is below MinHeight %d", h, termimg.MinHeight)
	}
}

// ---------------------------------------------------------------------------
// Challenge 7: Second caller to DownloadPreview gets premature onDone callback
// ---------------------------------------------------------------------------
func TestChallenger2_PrematureCallbackOnInFlightDownload(t *testing.T) {
	pngData := makeChallengerPNG(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "inflight.png",
		URL:     "mxc://example.com/inflight.png",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$inf1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadStarted := make(chan struct{})
	allowDownloadFinish := make(chan struct{})

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		close(downloadStarted)
		<-allowDownloadFinish
		return pngData, nil
	})

	// Start initial download
	fileMsg.DownloadPreview()
	<-downloadStarted

	// Second caller calls DownloadPreview with a completion callback
	var cbCalled atomic.Bool
	var dataLenWhenCbCalled int
	var isDownloadingWhenCbCalled bool
	cbDone := make(chan struct{})

	fileMsg.DownloadPreview(func() {
		cbCalled.Store(true)
		dataLenWhenCbCalled = len(fileMsg.ImageData())
		isDownloadingWhenCbCalled = fileMsg.IsDownloading()
		close(cbDone)
	})

	// Does the second caller's callback get executed BEFORE the download even finished?
	select {
	case <-cbDone:
		t.Logf("Premature callback fired! dataLen=%d, isDownloading=%v", dataLenWhenCbCalled, isDownloadingWhenCbCalled)
		if dataLenWhenCbCalled == 0 && isDownloadingWhenCbCalled {
			t.Errorf("CRITICAL DEFECT: DownloadPreview invoked onDone callback while media download was still in-flight! ImageData is empty and IsDownloading is true!")
		}
	case <-time.After(100 * time.Millisecond):
		t.Logf("Callback was not prematurely fired.")
	}

	close(allowDownloadFinish)
	fileMsg.WaitDownload()
}

// ---------------------------------------------------------------------------
// Challenge 8: SetImageData allows decompression bombs without safety check
// ---------------------------------------------------------------------------
func TestChallenger2_SetImageDataDecompressionBombSafety(t *testing.T) {
	bombHeader := makeChallengerPNGHeader(20000, 20000) // 400MP

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "direct_bomb.png",
		URL:     "mxc://example.com/direct_bomb.png",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$dirbomb1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	// Direct injection of payload via SetImageData
	fileMsg.SetImageData(bombHeader)

	if len(fileMsg.ImageData()) > 0 {
		t.Logf("SetImageData accepted %d bytes of 400MP bomb without validation", len(bombHeader))
	}

	// Now UI thread calls CalculateBuffer
	prefs := config.UserPreferences{}
	uiMsg.CalculateBuffer(prefs, 80)

	// Since CalculateBuffer does image.DecodeConfig but doesn't validate MaxMegaPixels,
	// what happened to msg.buffer?
	bufLen := fileMsg.Height()
	t.Logf("Height after CalculateBuffer with 400MP payload: %d", bufLen)
	if fileMsg.ImageError() == nil && bufLen > 0 {
		t.Logf("CalculateBuffer did not record ErrImageTooLarge for 400MP image injected via SetImageData")
	}
}
