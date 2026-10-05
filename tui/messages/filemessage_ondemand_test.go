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
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/tui/config"
)

func makeTestImageEvent() (*database.Event, *event.MessageEventContent) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "photo.png",
		URL:     id.ContentURIString("mxc://example.com/testphoto"),
		Info: &event.FileInfo{
			Width:  100,
			Height: 100,
		},
	}
	raw, _ := json.Marshal(content)
	evt := &database.Event{
		ID:        id.EventID("$ondemand_evt_1"),
		Sender:    id.UserID("@bob:matrix.org"),
		Type:      event.EventMessage.Type,
		Content:   raw,
		Timestamp: jsontime.UM(time.Now()),
	}
	return evt, content
}

func TestFileMessage_OnDemand_InitialStateNotRequested(t *testing.T) {
	evt, content := makeTestImageEvent()
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	downloadCalled := false
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		downloadCalled = true
		return createTestPNGBytes(100, 100), nil
	})

	// 1. Initial on-demand state: previewRequested is false, download not called
	if fileMsg.IsPreviewRequested() {
		t.Fatalf("expected IsPreviewRequested() to be false initially")
	}
	if fileMsg.IsDownloading() {
		t.Fatalf("expected IsDownloading() to be false initially")
	}
	if len(fileMsg.ImageData()) != 0 {
		t.Fatalf("expected ImageData() to be empty initially")
	}
	if downloadCalled {
		t.Fatalf("expected download to NOT be called eager/initially")
	}

	// 2. Buffer calculation without preview request should show fallback text link
	prefs := config.UserPreferences{
		BareMessageView:      false,
		DisableImages:        false,
		ImagePreviewProtocol: "halfblocks",
	}

	fileMsg.CalculateBuffer(prefs, 80, uiMsg)
	buf := fileMsg.buffer
	if len(buf) == 0 {
		t.Fatalf("expected fallback text buffer to be generated")
	}
	plain := buf[0].String()
	if downloadCalled {
		t.Fatalf("CalculateBuffer without RequestPreview must not trigger download")
	}
	if !fileMsg.IsPreviewRequested() && len(plain) == 0 {
		t.Fatalf("expected non-empty fallback text")
	}
}

func TestFileMessage_OnDemand_RequestPreviewTriggersDownload(t *testing.T) {
	evt, content := makeTestImageEvent()
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	pngBytes := createTestPNGBytes(100, 100)
	var downloadCount atomic.Int32
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		downloadCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return pngBytes, nil
	})

	// Explicit preview request
	fileMsg.RequestPreview()

	if !fileMsg.IsPreviewRequested() {
		t.Fatalf("expected IsPreviewRequested() to be true after RequestPreview()")
	}

	fileMsg.WaitDownload()

	if downloadCount.Load() != 1 {
		t.Fatalf("expected download to be called exactly once, got %d", downloadCount.Load())
	}
	if len(fileMsg.ImageData()) == 0 {
		t.Fatalf("expected ImageData() to be populated after WaitDownload()")
	}

	// Now CalculateBuffer produces rendered image
	prefs := config.UserPreferences{
		BareMessageView:      false,
		DisableImages:        false,
		ImagePreviewProtocol: "halfblocks",
	}

	fileMsg.CalculateBuffer(prefs, 80, uiMsg)
	buf := fileMsg.buffer
	if len(buf) <= 1 {
		t.Fatalf("expected image buffer height > 1 after download, got %d", len(buf))
	}
}

func TestFileMessage_OnDemand_ClonePreservesRequestState(t *testing.T) {
	evt, content := makeTestImageEvent()
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return createTestPNGBytes(50, 50), nil
	})

	fileMsg.RequestPreview()
	fileMsg.WaitDownload()

	cloned := fileMsg.Clone().(*FileMessage)
	if !cloned.IsPreviewRequested() {
		t.Fatalf("expected cloned FileMessage to have IsPreviewRequested() == true")
	}
	if len(cloned.ImageData()) != len(fileMsg.ImageData()) {
		t.Fatalf("expected cloned FileMessage to have same image data")
	}
}

func TestFileMessage_OnDemand_ConcurrentRequests(t *testing.T) {
	evt, content := makeTestImageEvent()
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	pngBytes := createTestPNGBytes(60, 60)
	var downloadCount atomic.Int32
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		downloadCount.Add(1)
		time.Sleep(15 * time.Millisecond)
		return pngBytes, nil
	})

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			fileMsg.RequestPreview()
		}()
	}
	wg.Wait()
	fileMsg.WaitDownload()

	if !fileMsg.IsPreviewRequested() {
		t.Fatalf("expected IsPreviewRequested() to be true")
	}
	if downloadCount.Load() != 1 {
		t.Fatalf("expected exactly 1 download execution across %d concurrent callers, got %d", numGoroutines, downloadCount.Load())
	}
}

func TestParseEvent_DoesNotEagerlyDownloadMedia(t *testing.T) {
	evt, _ := makeTestImageEvent()
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

	// G1 assertion: ParseEvent MUST NOT request or trigger download eagerly
	if fileMsg.IsPreviewRequested() {
		t.Fatalf("ParseEvent must not eagerly set previewRequested=true")
	}
	if fileMsg.IsDownloading() {
		t.Fatalf("ParseEvent must not eagerly start downloading media")
	}
	if len(fileMsg.ImageData()) != 0 {
		t.Fatalf("ParseEvent must not populate imageData eagerly")
	}
}

func TestFileMessage_OnDemand_RetryAfterError(t *testing.T) {
	evt, content := makeTestImageEvent()
	uiMsg := NewFileMessage(nil, nil, evt, content)
	fileMsg := uiMsg.Renderer.(*FileMessage)

	// Step 1: Simulate network failure
	callCount := 0
	fileMsg.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("transient network failure")
		}
		return createTestPNGBytes(50, 50), nil
	})

	fileMsg.RequestPreview()
	fileMsg.WaitDownload()

	if !fileMsg.HasImageError() {
		t.Fatalf("expected HasImageError() to be true after failure")
	}

	// Step 2: Retry via RequestPreview()
	fileMsg.RequestPreview()
	fileMsg.WaitDownload()

	if fileMsg.HasImageError() {
		t.Fatalf("expected HasImageError() to be false after successful retry")
	}
	if len(fileMsg.ImageData()) == 0 {
		t.Fatalf("expected ImageData() to be populated after retry")
	}
}
