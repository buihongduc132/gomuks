// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package messages

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/lib/termimg"
)

func createTestPNGBytes(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func createCraftedPNGHeaderBytes(width, height uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], width)
	binary.BigEndian.PutUint32(ihdrData[4:8], height)
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

func newSimulationScreen(w, h int) mauview.Screen {
	s := tcell.NewSimulationScreen("")
	_ = s.Init()
	s.SetSize(w, h)
	return s
}

func TestFileMessage_ThreadSafety(t *testing.T) {
	pngData := createTestPNGBytes(64, 48)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "photo.png",
		URL:     "mxc://matrix.example.com/media123",
	}

	meta := &database.Room{ID: "!room:example.com"}
	roomStore := store.NewRoomStore(nil, meta)
	evt := &database.Event{ID: "$event1:example.com"}

	uiMsg := NewFileMessage(roomStore, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		// Simulate network latency
		time.Sleep(5 * time.Millisecond)
		return pngData, nil
	})

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Reader goroutines: repeatedly calling CalculateBuffer, Height, Draw, PlainText, Clone, ImageData
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			prefs := config.UserPreferences{}
			screen := newSimulationScreen(80, 24)
			for !stop.Load() {
				uiMsg.CalculateBuffer(prefs, 80)
				_ = uiMsg.Height()
				fileMsg.CalculateBuffer(prefs, 80, uiMsg)
				_ = fileMsg.Height()
				fileMsg.Draw(screen, uiMsg)
				_ = fileMsg.PlainText()
				_ = fileMsg.Clone()
				_ = fileMsg.ImageData()
				_ = fileMsg.ImageError()
				_ = fileMsg.IsDownloading()
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Writer goroutines: triggering DownloadPreview and SetImageData
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for !stop.Load() {
				if writerID%2 == 0 {
					fileMsg.DownloadPreview()
				} else {
					fileMsg.SetImageData(pngData)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(200 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
	fileMsg.WaitDownload()

	if len(fileMsg.ImageData()) == 0 {
		t.Errorf("expected image data to be stored after downloads completed")
	}
}

func TestFileMessage_DecompressionBombSafety(t *testing.T) {
	// 20000x20000 = 400 Megapixels (> 16MP limit)
	bombHeader := createCraftedPNGHeaderBytes(20000, 20000)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "pixel_bomb.png",
		URL:     "mxc://matrix.example.com/bomb",
	}

	evt := &database.Event{ID: "$bomb:example.com"}
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return bombHeader, nil
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	// 1. Assert decompression bomb was rejected and never stored
	if data := fileMsg.ImageData(); data != nil {
		t.Fatalf("expected bomb payload to NOT be stored in imageData, got %d bytes", len(data))
	}

	// 2. Assert ImageError is recorded with ErrImageTooLarge
	imgErr := fileMsg.ImageError()
	if imgErr == nil {
		t.Fatalf("expected ImageError to be recorded, got nil")
	}
	if !errors.Is(imgErr, termimg.ErrImageTooLarge) {
		t.Errorf("expected ErrImageTooLarge, got: %v", imgErr)
	}

	// 3. Assert buffer invalidation occurred (bufferedWidth reset to 0)
	if uiMsg.BufferedWidth() != 0 {
		t.Errorf("expected bufferedWidth to be 0 after safety rejection, got %d", uiMsg.BufferedWidth())
	}

	// 4. Assert CalculateBuffer handles error gracefully without panic or OOM
	prefs := config.UserPreferences{}
	uiMsg.CalculateBuffer(prefs, 80)
	if uiMsg.Height() < 1 {
		t.Errorf("expected height >= 1 for fallback error text, got %d", uiMsg.Height())
	}
}

func TestFileMessage_ThumbnailPrioritization(t *testing.T) {
	thumbPNG := createTestPNGBytes(32, 24)
	fullPNG := createTestPNGBytes(640, 480)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "landscape.png",
		URL:     "mxc://matrix.example.com/full_image",
		Info: &event.FileInfo{
			ThumbnailURL: "mxc://matrix.example.com/thumb_image",
		},
	}

	evt := &database.Event{ID: "$thumb_test:example.com"}
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	var requestedURIs []string
	var mu sync.Mutex

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		mu.Lock()
		requestedURIs = append(requestedURIs, uri.String())
		mu.Unlock()

		if uri.FileID == "thumb_image" {
			return thumbPNG, nil
		} else if uri.FileID == "full_image" {
			return fullPNG, nil
		}
		return nil, errors.New("unexpected URI")
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	mu.Lock()
	defer mu.Unlock()

	if len(requestedURIs) != 1 {
		t.Fatalf("expected exactly 1 download request, got %d: %v", len(requestedURIs), requestedURIs)
	}
	if requestedURIs[0] != "mxc://matrix.example.com/thumb_image" {
		t.Errorf("expected thumbnail to be downloaded first, got: %s", requestedURIs[0])
	}
	if !bytes.Equal(fileMsg.ImageData(), thumbPNG) {
		t.Errorf("expected stored imageData to match thumbnail bytes")
	}
}

func TestFileMessage_ThumbnailFallbackToFullMedia(t *testing.T) {
	fullPNG := createTestPNGBytes(320, 240)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "fallback.png",
		URL:     "mxc://matrix.example.com/full_image",
		Info: &event.FileInfo{
			ThumbnailURL: "mxc://matrix.example.com/failing_thumb",
		},
	}

	evt := &database.Event{ID: "$fallback:example.com"}
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	var requestedURIs []string
	var mu sync.Mutex

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		mu.Lock()
		requestedURIs = append(requestedURIs, uri.String())
		mu.Unlock()

		if uri.FileID == "failing_thumb" {
			return nil, errors.New("thumbnail 404 not found")
		}
		if uri.FileID == "full_image" {
			return fullPNG, nil
		}
		return nil, errors.New("unknown URI")
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	mu.Lock()
	defer mu.Unlock()

	if len(requestedURIs) != 2 {
		t.Fatalf("expected 2 download requests (thumb then full), got %d: %v", len(requestedURIs), requestedURIs)
	}
	if requestedURIs[0] != "mxc://matrix.example.com/failing_thumb" {
		t.Errorf("expected first attempt to be thumbnail, got: %s", requestedURIs[0])
	}
	if requestedURIs[1] != "mxc://matrix.example.com/full_image" {
		t.Errorf("expected second attempt to be full media, got: %s", requestedURIs[1])
	}
	if !bytes.Equal(fileMsg.ImageData(), fullPNG) {
		t.Errorf("expected stored imageData to match full image bytes")
	}
	if fileMsg.ImageError() != nil {
		t.Errorf("expected nil ImageError after successful fallback, got: %v", fileMsg.ImageError())
	}
}

func TestFileMessage_ThumbnailSafetyValidationFallback(t *testing.T) {
	corruptThumb := []byte("corrupt-not-a-png")
	validFull := createTestPNGBytes(100, 100)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "thumb_corrupt.png",
		URL:     "mxc://matrix.example.com/good_full",
		Info: &event.FileInfo{
			ThumbnailURL: "mxc://matrix.example.com/corrupt_thumb",
		},
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$t1:ex"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		if uri.FileID == "corrupt_thumb" {
			return corruptThumb, nil
		}
		return validFull, nil
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	if !bytes.Equal(fileMsg.ImageData(), validFull) {
		t.Errorf("expected fallback to good full image when thumbnail is corrupt")
	}
	if fileMsg.ImageError() != nil {
		t.Errorf("expected no final error after fallback, got %v", fileMsg.ImageError())
	}
}

func TestFileMessage_ActiveRoomRedrawGating(t *testing.T) {
	pngData := createTestPNGBytes(64, 64)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "active_room_test.png",
		URL:     "mxc://matrix.example.com/img",
	}

	activeRoomID := id.RoomID("!active:example.com")
	inactiveRoomID := id.RoomID("!inactive:example.com")

	t.Run("redraw triggered when room is active", func(t *testing.T) {
		meta := &database.Room{ID: activeRoomID}
		roomStore := store.NewRoomStore(nil, meta)
		uiMsg := NewFileMessage(roomStore, nil, &database.Event{ID: "$1"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var redrawCount atomic.Int32
		fileMsg.Redraw = func() {
			redrawCount.Add(1)
		}
		fileMsg.ActiveRoomChecker = func() bool {
			return true
		}
		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			return pngData, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		if redrawCount.Load() != 1 {
			t.Errorf("expected redraw to be triggered once for active room, got %d", redrawCount.Load())
		}
	})

	t.Run("redraw suppressed when room is inactive", func(t *testing.T) {
		meta := &database.Room{ID: inactiveRoomID}
		roomStore := store.NewRoomStore(nil, meta)
		uiMsg := NewFileMessage(roomStore, nil, &database.Event{ID: "$2"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var redrawCount atomic.Int32
		fileMsg.Redraw = func() {
			redrawCount.Add(1)
		}
		fileMsg.ActiveRoomChecker = func() bool {
			return false
		}
		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			return pngData, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		if redrawCount.Load() != 0 {
			t.Errorf("expected redraw to be suppressed for inactive room, got %d", redrawCount.Load())
		}

		// Verify data is still populated and buffer invalidated
		if len(fileMsg.ImageData()) == 0 {
			t.Errorf("expected imageData to be populated even if room was inactive")
		}
		if uiMsg.BufferedWidth() != 0 {
			t.Errorf("expected bufferedWidth to be reset to 0 even if room was inactive")
		}
	})

	t.Run("package-level ActiveRoomChecker and RequestRedraw", func(t *testing.T) {
		meta := &database.Room{ID: activeRoomID}
		roomStore := store.NewRoomStore(nil, meta)
		uiMsg := NewFileMessage(roomStore, nil, &database.Event{ID: "$3"}, content)
		fileMsg := uiMsg.Renderer.(*FileMessage)

		var globalRedrawRoom id.RoomID
		var mu sync.Mutex

		origChecker := ActiveRoomChecker
		origRedraw := RequestRedraw
		defer func() {
			ActiveRoomChecker = origChecker
			RequestRedraw = origRedraw
		}()

		ActiveRoomChecker = func(roomID id.RoomID) bool {
			return roomID == activeRoomID
		}
		RequestRedraw = func(roomID id.RoomID) {
			mu.Lock()
			globalRedrawRoom = roomID
			mu.Unlock()
		}

		fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
			return pngData, nil
		})

		fileMsg.DownloadPreview()
		fileMsg.WaitDownload()

		mu.Lock()
		defer mu.Unlock()
		if globalRedrawRoom != activeRoomID {
			t.Errorf("expected RequestRedraw to be called for %s, got %s", activeRoomID, globalRedrawRoom)
		}
	})
}

func TestFileMessage_BufferInvalidation(t *testing.T) {
	pngData := createTestPNGBytes(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "buffer_test.png",
		URL:     "mxc://matrix.example.com/img",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$buf1"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	prefs := config.UserPreferences{}
	// First calculation with empty data
	uiMsg.CalculateBuffer(prefs, 80)
	if uiMsg.BufferedWidth() != 80 {
		t.Fatalf("expected bufferedWidth=80, got %d", uiMsg.BufferedWidth())
	}
	initialHeight := uiMsg.Height()
	if initialHeight != 1 {
		t.Fatalf("expected initial plaintext height 1, got %d", initialHeight)
	}

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return pngData, nil
	})

	fileMsg.DownloadPreview()
	fileMsg.WaitDownload()

	// Assert buffer invalidation occurred
	if uiMsg.BufferedWidth() != 0 {
		t.Fatalf("expected bufferedWidth=0 after download completion, got %d", uiMsg.BufferedWidth())
	}

	// Recalculate buffer after image arrived
	uiMsg.CalculateBuffer(prefs, 80)
	if uiMsg.BufferedWidth() != 80 {
		t.Fatalf("expected bufferedWidth=80 after re-calculation, got %d", uiMsg.BufferedWidth())
	}
	newHeight := uiMsg.Height()
	if newHeight <= initialHeight {
		t.Errorf("expected new height > %d after image rendered, got %d", initialHeight, newHeight)
	}
}

func TestFileMessage_ConcurrentDownloadDeduplication(t *testing.T) {
	pngData := createTestPNGBytes(64, 48)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "dedup.png",
		URL:     "mxc://matrix.example.com/img",
	}

	uiMsg := NewFileMessage(nil, nil, &database.Event{ID: "$dedup"}, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	var downloadCount atomic.Int32
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		downloadCount.Add(1)
		time.Sleep(20 * time.Millisecond)
		return pngData, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fileMsg.DownloadPreview()
		}()
	}
	wg.Wait()
	fileMsg.WaitDownload()

	if count := downloadCount.Load(); count != 1 {
		t.Errorf("expected downloadFn to be executed exactly once, got %d", count)
	}
}

func TestParser_ImageDownloadWiring(t *testing.T) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "parser_test.png",
		URL:     "mxc://matrix.example.com/img",
		Info: &event.FileInfo{
			ThumbnailURL: "mxc://matrix.example.com/thumb",
		},
	}
	contentBytes, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal content: %v", err)
	}
	evt := &database.Event{
		ID:      "$parse1",
		Type:    event.EventMessage.Type,
		Content: contentBytes,
	}

	t.Run("downloads triggered when enabled", func(t *testing.T) {
		prefs := &config.UserPreferences{
			DisableDownloads: false,
			DisableImages:    false,
		}
		parsedMsg := ParseEvent(nil, prefs, nil, evt)
		if parsedMsg == nil {
			t.Fatalf("expected non-nil parsed message")
		}
		fileMsg, ok := parsedMsg.Renderer.(*FileMessage)
		if !ok {
			t.Fatalf("expected *FileMessage renderer")
		}
		// Thumbnail metadata should be captured
		if fileMsg.ThumbnailURL.String() != "mxc://matrix.example.com/thumb" {
			t.Errorf("expected thumbnail URL to be captured, got %s", fileMsg.ThumbnailURL.String())
		}
	})

	t.Run("downloads NOT triggered when DisableDownloads=true", func(t *testing.T) {
		prefs := &config.UserPreferences{
			DisableDownloads: true,
			DisableImages:    false,
		}
		parsedMsg := ParseEvent(nil, prefs, nil, evt)
		if parsedMsg == nil {
			t.Fatalf("expected non-nil parsed message")
		}
		fileMsg := parsedMsg.Renderer.(*FileMessage)
		if fileMsg.IsDownloading() {
			t.Errorf("expected download to NOT be triggered when DisableDownloads=true")
		}
	})

	t.Run("downloads NOT triggered when DisableImages=true", func(t *testing.T) {
		prefs := &config.UserPreferences{
			DisableDownloads: false,
			DisableImages:    true,
		}
		parsedMsg := ParseEvent(nil, prefs, nil, evt)
		if parsedMsg == nil {
			t.Fatalf("expected non-nil parsed message")
		}
		fileMsg := parsedMsg.Renderer.(*FileMessage)
		if fileMsg.IsDownloading() {
			t.Errorf("expected download to NOT be triggered when DisableImages=true")
		}
	})
}

func TestFileMessage_CloneIntegrity(t *testing.T) {
	pngData := createTestPNGBytes(32, 32)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "clone_test.png",
		URL:     "mxc://matrix.example.com/clone",
	}

	meta := &database.Room{ID: "!room:example.com"}
	roomStore := store.NewRoomStore(nil, meta)
	evt := &database.Event{ID: "$clone1:example.com"}

	uiMsg := NewFileMessage(roomStore, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{}
	uiMsg.CalculateBuffer(prefs, 80)

	// Clone the UIMessage
	clonedUIMsg := uiMsg.Clone()
	clonedFileMsg, ok := clonedUIMsg.Renderer.(*FileMessage)
	if !ok {
		t.Fatalf("expected cloned renderer to be *FileMessage")
	}

	// Verify bidirectional link
	if clonedFileMsg.uiMsg != clonedUIMsg {
		t.Errorf("expected cloned FileMessage.uiMsg to point to cloned UIMessage (%p), got %p", clonedUIMsg, clonedFileMsg.uiMsg)
	}

	// Verify buffer deep copy
	if len(clonedFileMsg.buffer) != len(fileMsg.buffer) {
		t.Fatalf("expected clone buffer length %d, got %d", len(fileMsg.buffer), len(clonedFileMsg.buffer))
	}
	if clonedFileMsg.cachedWidth != fileMsg.cachedWidth {
		t.Errorf("expected cloned cachedWidth %d, got %d", fileMsg.cachedWidth, clonedFileMsg.cachedWidth)
	}

	// Mutate original buffer cells, assert clone buffer cells are unmodified
	if len(fileMsg.buffer) > 0 && len(fileMsg.buffer[0]) > 0 {
		origChar := fileMsg.buffer[0][0].Char
		fileMsg.buffer[0][0].Char = 'X'
		if clonedFileMsg.buffer[0][0].Char == 'X' {
			t.Errorf("cloned buffer cells are shallowly shared with original")
		}
		fileMsg.buffer[0][0].Char = origChar
	}

	// Invalidate clone buffer, original buffer must remain valid
	clonedUIMsg.InvalidateBuffer()
	if clonedUIMsg.BufferedWidth() != 0 {
		t.Errorf("expected cloned BufferedWidth=0 after invalidation, got %d", clonedUIMsg.BufferedWidth())
	}
	if uiMsg.BufferedWidth() != 80 {
		t.Errorf("original BufferedWidth should remain 80 after clone invalidation, got %d", uiMsg.BufferedWidth())
	}

	// Verify clone has independent download state
	if clonedFileMsg.IsDownloading() {
		t.Errorf("clone should not report downloading")
	}
	// WaitDownload should return immediately without blocking
	waitCh := make(chan struct{})
	go func() {
		clonedFileMsg.WaitDownload()
		close(waitCh)
	}()
	select {
	case <-waitCh:
	case <-time.After(500 * time.Millisecond):
		t.Errorf("cloned WaitDownload blocked unexpectedly")
	}
}

// ---------------------------------------------------------------------------
// Milestone M3 Tests: Safe Rendering Engine, Cell-Masking, Protocols, Clamping
// ---------------------------------------------------------------------------

func createTestUIMessage(content *event.MessageEventContent) (*UIMessage, *FileMessage) {
	meta := &database.Room{ID: "!room:example.com"}
	roomStore := store.NewRoomStore(nil, meta)
	evt := &database.Event{ID: "$event1:example.com"}
	uiMsg := NewFileMessage(roomStore, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)
	return uiMsg, fileMsg
}

func TestFileMessage_M3_ProtocolBranching_ITerm2(t *testing.T) {
	pngData := createTestPNGBytes(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "flower.png",
		URL:     "mxc://matrix.example.com/flower100",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "iterm2",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	uiMsg.CalculateBuffer(prefs, 80)

	if fileMsg.CachedProto() != termimg.ProtocolITerm2 {
		t.Fatalf("expected cachedProto %s, got %s", termimg.ProtocolITerm2, fileMsg.CachedProto())
	}
	osc := fileMsg.CachedOSC()
	if len(osc) == 0 {
		t.Fatalf("expected non-empty cachedOSC")
	}
	if !strings.Contains(string(osc), "]1337;File=inline=1;width=") {
		t.Errorf("cachedOSC missing standard OSC 1337 signature: %s", string(osc))
	}

	// Verify buffer is placeholder filled with spaces
	if len(fileMsg.buffer) == 0 {
		t.Fatalf("expected non-empty placeholder buffer")
	}
	for y, line := range fileMsg.buffer {
		for x, cell := range line {
			if cell.Char != ' ' {
				t.Fatalf("expected placeholder cell at (%d,%d) to be ' ', got %c", x, y, cell.Char)
			}
		}
	}

	// Test Draw() with cell-masking and OSC sequence emission
	simScreen := newSimulationScreen(100, 40)
	// Create a proxy screen representing the message within the content viewport
	// screenY = 2, screenX = 2 (within bounds: row 0 is topic bar, rows 1..39 is content)
	proxy := mauview.NewProxyScreen(simScreen, 2, 2, 70, fileMsg.Height())

	var oscBuf bytes.Buffer
	SetOSCOutputWriter(&oscBuf)
	defer SetOSCOutputWriter(nil)

	fileMsg.Draw(proxy, uiMsg)

	// 1. Assert cell mask applied
	mask := fileMsg.CurrentMask()
	if mask == nil {
		t.Fatalf("expected active ImageMask after Draw in iterm2 mode")
	}
	if !mask.Active {
		t.Errorf("expected ImageMask to be Active")
	}
	if mask.ScreenX != 2 || mask.ScreenY != 2 {
		t.Errorf("expected mask screen coordinates (2,2), got (%d,%d)", mask.ScreenX, mask.ScreenY)
	}

	// 2. Assert escape sequence emitted with CUP positioning
	emitted := oscBuf.String()
	expectedCUP := "\x1b[3;3H" // y+1, x+1 => 2+1, 2+1
	if !strings.HasPrefix(emitted, expectedCUP) {
		t.Errorf("expected emitted sequence to start with CUP %q, got %q", expectedCUP, emitted[:min(len(emitted), 10)])
	}
	if !strings.Contains(emitted, "]1337;File=inline=1") {
		t.Errorf("expected emitted sequence to contain OSC 1337 escape, got %q", emitted)
	}

	// 3. Assert deduplication across identical frames
	oscBuf.Reset()
	fileMsg.Draw(proxy, uiMsg)
	if oscBuf.Len() != 0 {
		t.Errorf("expected 0 bytes emitted on identical redraw frame (deduplication), got %d bytes", oscBuf.Len())
	}

	// 4. Position changed: should re-emit with new coordinates
	proxy2 := mauview.NewProxyScreen(simScreen, 5, 4, 70, fileMsg.Height())
	fileMsg.Draw(proxy2, uiMsg)
	emitted2 := oscBuf.String()
	expectedCUP2 := "\x1b[5;6H" // y+1, x+1 => 4+1, 5+1
	if !strings.HasPrefix(emitted2, expectedCUP2) {
		t.Errorf("expected CUP %q on position change, got %q", expectedCUP2, emitted2[:min(len(emitted2), 10)])
	}
}

func TestFileMessage_M3_ProtocolBranching_HalfBlocks(t *testing.T) {
	pngData := createTestPNGBytes(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "halfblocks.png",
		URL:     "mxc://matrix.example.com/halfblocks100",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "halfblocks",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	uiMsg.CalculateBuffer(prefs, 80)

	if fileMsg.CachedProto() != termimg.ProtocolHalfBlocks {
		t.Fatalf("expected cachedProto %s, got %s", termimg.ProtocolHalfBlocks, fileMsg.CachedProto())
	}
	if fileMsg.CachedOSC() != nil {
		t.Errorf("expected nil cachedOSC in halfblocks mode")
	}

	// Half-blocks should be rendered into buffer with '▄' runes
	if fileMsg.Height() <= 0 || fileMsg.Height() > 16 {
		t.Fatalf("expected height in [1, 16], got %d", fileMsg.Height())
	}
	hasHalfBlockRune := false
	for _, line := range fileMsg.buffer {
		for _, cell := range line {
			if cell.Char == '▄' {
				hasHalfBlockRune = true
				break
			}
		}
	}
	if !hasHalfBlockRune {
		t.Errorf("expected half-block runes '▄' in buffer")
	}

	// In Draw(), halfblocks must not apply cell mask
	simScreen := newSimulationScreen(100, 40)
	proxy := mauview.NewProxyScreen(simScreen, 2, 2, 70, fileMsg.Height())
	fileMsg.Draw(proxy, uiMsg)

	if fileMsg.CurrentMask() != nil {
		t.Errorf("expected CurrentMask to be nil in halfblocks mode")
	}
}

func TestFileMessage_M3_ProtocolBranching_Disabled(t *testing.T) {
	pngData := createTestPNGBytes(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "disabled.png",
		URL:     "mxc://matrix.example.com/disabled100",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{
		ImagePreviewProtocol: "disabled",
	}

	uiMsg.CalculateBuffer(prefs, 80)

	if fileMsg.Height() != 1 {
		t.Fatalf("expected 1 row for disabled image preview (text fallback), got %d", fileMsg.Height())
	}
	text := fileMsg.buffer[0].String()
	if !strings.Contains(text, "disabled.png") {
		t.Errorf("expected text fallback to contain filename, got %q", text)
	}
}

func TestFileMessage_M3_ClampingParity_ExtremeAspectRatios(t *testing.T) {
	// Tall vertical image: 50 width x 500 height
	tallPNG := createTestPNGBytes(50, 500)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "tall.png",
		URL:     "mxc://matrix.example.com/tall",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(tallPNG)

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "halfblocks",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	uiMsg.CalculateBuffer(prefs, 80)

	// CRITICAL: height must never explode past MaxHeight
	if fileMsg.Height() > 16 {
		t.Fatalf("tall image height exploded to %d rows (max allowed: 16)", fileMsg.Height())
	}
	if fileMsg.Height() < 3 {
		t.Errorf("tall image height too small: %d", fileMsg.Height())
	}

	// Panoramic horizontal image: 500 width x 20 height
	panoPNG := createTestPNGBytes(500, 20)
	contentPano := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "pano.png",
		URL:     "mxc://matrix.example.com/pano",
	}
	uiMsgPano, fileMsgPano := createTestUIMessage(contentPano)
	fileMsgPano.SetImageData(panoPNG)

	uiMsgPano.CalculateBuffer(prefs, 80)

	if fileMsgPano.Height() < 3 {
		t.Fatalf("panoramic image height %d violated MinHeight (3)", fileMsgPano.Height())
	}
	if fileMsgPano.Height() > 16 {
		t.Fatalf("panoramic image height %d exceeded max 16", fileMsgPano.Height())
	}
	if len(fileMsgPano.buffer) > 0 && len(fileMsgPano.buffer[0]) > 66 {
		t.Fatalf("panoramic image width %d exceeded maxCols 66", len(fileMsgPano.buffer[0]))
	}
}

func TestFileMessage_M3_DecompressionSafetyAndCorruptData(t *testing.T) {
	// Decompression bomb: header claims 20000x20000 (400 Megapixels)
	bombHeader := createCraftedPNGHeaderBytes(20000, 20000)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "bomb.png",
		URL:     "mxc://matrix.example.com/bomb",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	// Bypass SetImageData safety check to simulate an adversarial raw payload arriving
	fileMsg.mu.Lock()
	fileMsg.imageData = bombHeader
	fileMsg.mu.Unlock()

	prefs := config.UserPreferences{
		ImagePreviewProtocol: "halfblocks",
	}

	// Must not panic, must gracefully fall back to text with error
	uiMsg.CalculateBuffer(prefs, 80)

	if fileMsg.ImageError() == nil {
		t.Errorf("expected ImageError for >16MP bomb")
	}
	if fileMsg.Height() < 1 {
		t.Errorf("expected at least 1-row text fallback for decompression bomb, got %d", fileMsg.Height())
	}
	text := fileMsg.buffer[0].String()
	if !strings.Contains(text, "bomb.png") {
		t.Errorf("expected text fallback to contain filename, got %q", text)
	}

	// Corrupt garbage data
	corruptContent := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "corrupt.png",
		URL:     "mxc://matrix.example.com/corrupt",
	}
	uiMsgCorrupt, fileMsgCorrupt := createTestUIMessage(corruptContent)
	fileMsgCorrupt.mu.Lock()
	fileMsgCorrupt.imageData = []byte("this is corrupt garbage not an image")
	fileMsgCorrupt.mu.Unlock()

	uiMsgCorrupt.CalculateBuffer(prefs, 80)
	if fileMsgCorrupt.ImageError() == nil {
		t.Errorf("expected ImageError for corrupt image payload")
	}
	if fileMsgCorrupt.Height() < 1 {
		t.Errorf("expected at least 1-row text fallback for corrupt payload, got %d", fileMsgCorrupt.Height())
	}
}

func TestFileMessage_M3_ViewportClipping_BoundarySafety(t *testing.T) {
	pngData := createTestPNGBytes(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "viewport.png",
		URL:     "mxc://matrix.example.com/viewport",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "iterm2",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	uiMsg.CalculateBuffer(prefs, 80)
	imgRows := fileMsg.Height()

	simScreen := newSimulationScreen(100, 40)
	var oscBuf bytes.Buffer
	SetOSCOutputWriter(&oscBuf)
	defer SetOSCOutputWriter(nil)

	// Case A: Partially clipped at top (screenY = 0 < 1, overlapping Topic Bar)
	// Parent viewport size is 35 rows.
	parentProxy := mauview.NewProxyScreen(simScreen, 0, 0, 100, 35)
	topClippedProxy := mauview.NewProxyScreen(parentProxy, 2, 0, 70, imgRows)

	fileMsg.Draw(topClippedProxy, uiMsg)

	if oscBuf.Len() != 0 {
		t.Errorf("expected OSC 1337 to be suppressed when screenY < 1, but emitted %d bytes", oscBuf.Len())
	}
	if fileMsg.CurrentMask() != nil {
		t.Errorf("expected mask to be cleared when partially clipped at top")
	}

	// Case B: Fully visible within viewport bounds (screenY = 2, screenY+imgRows <= 1+35)
	oscBuf.Reset()
	visibleProxy := mauview.NewProxyScreen(parentProxy, 2, 2, 70, imgRows)
	fileMsg.Draw(visibleProxy, uiMsg)

	if oscBuf.Len() == 0 {
		t.Errorf("expected OSC 1337 to be emitted when message is fully within viewport")
	}
	if fileMsg.CurrentMask() == nil || !fileMsg.CurrentMask().Active {
		t.Errorf("expected cell mask to be active when fully within viewport")
	}

	// Case C: Partially clipped at bottom (screenY = 25, imgRows = 16 => screenY+rows = 41 > 36)
	oscBuf.Reset()
	bottomClippedProxy := mauview.NewProxyScreen(parentProxy, 2, 25, 70, imgRows)
	fileMsg.Draw(bottomClippedProxy, uiMsg)

	if oscBuf.Len() != 0 {
		t.Errorf("expected OSC 1337 to be suppressed when bottom overflow occurs, but emitted %d bytes", oscBuf.Len())
	}
	if fileMsg.CurrentMask() != nil {
		t.Errorf("expected mask to be cleared when partially clipped at bottom")
	}

	// Case D: Horizontally clipped off-screen (screenX = -5)
	oscBuf.Reset()
	leftClippedProxy := mauview.NewProxyScreen(parentProxy, -5, 2, 70, imgRows)
	fileMsg.Draw(leftClippedProxy, uiMsg)

	if oscBuf.Len() != 0 {
		t.Errorf("expected OSC 1337 to be suppressed when screenX < 0, but emitted %d bytes", oscBuf.Len())
	}
	if fileMsg.CurrentMask() != nil {
		t.Errorf("expected mask to be cleared when screenX < 0")
	}
}

func TestFileMessage_M3_ResizeCaching(t *testing.T) {
	pngData := createTestPNGBytes(100, 100)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "resize.png",
		URL:     "mxc://matrix.example.com/resize",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "halfblocks",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	bbox := termimg.CalculateClampedDimensions(100, 100, 100, 66, 16)
	targetCols := bbox.Cols

	// Calculate at width 100
	uiMsg.CalculateBuffer(prefs, 100)
	cache1 := fileMsg.RenderCache()
	if cache1 == nil || len(cache1[targetCols]) == 0 {
		t.Fatalf("expected renderCache[%d] to be populated", targetCols)
	}
	firstRendered := cache1[targetCols]

	// Invalidate buffer to simulate window resize tick, but same clamped column boundary
	uiMsg.InvalidateBuffer()

	// Calculate at width 120 (also clamps to same targetCols)
	uiMsg.CalculateBuffer(prefs, 120)
	cache2 := fileMsg.RenderCache()
	secondRendered := cache2[targetCols]

	if len(firstRendered) != len(secondRendered) {
		t.Fatalf("cache mismatch: lengths %d vs %d", len(firstRendered), len(secondRendered))
	}
}

func TestFileMessage_M3_ConcurrencyStress(t *testing.T) {
	pngData := createTestPNGBytes(80, 60)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "stress.png",
		URL:     "mxc://matrix.example.com/stress",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	simScreen := newSimulationScreen(120, 50)
	stop := make(chan struct{})

	var wg sync.WaitGroup
	// Goroutine 1: Rapid buffer recalculation with varying widths
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := 70
		for {
			select {
			case <-stop:
				return
			default:
				prefs := config.UserPreferences{
					ImagePreviewProtocol:  "iterm2",
					ImagePreviewMaxWidth:  66,
					ImagePreviewMaxHeight: 16,
				}
				uiMsg.CalculateBuffer(prefs, w)
				w++
				if w > 100 {
					w = 70
				}
			}
		}
	}()

	// Goroutine 2: Rapid drawing with varying coordinates
	wg.Add(1)
	go func() {
		defer wg.Done()
		y := 2
		for {
			select {
			case <-stop:
				return
			default:
				proxy := mauview.NewProxyScreen(simScreen, 2, y, 70, max(1, fileMsg.Height()))
				fileMsg.Draw(proxy, uiMsg)
				y++
				if y > 10 {
					y = 2
				}
			}
		}
	}()

	// Goroutine 3: Cloning
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = uiMsg.Clone()
			}
		}
	}()

	// Goroutine 4: ClearMask and CurrentMask
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = fileMsg.CurrentMask()
				fileMsg.ClearMask()
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
