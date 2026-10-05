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

package tui

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/messages"
)

func makeTestFileMessage() (*messages.UIMessage, *messages.FileMessage) {
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "sample.png",
		URL:     id.ContentURIString("mxc://example.com/sample"),
	}
	raw, _ := json.Marshal(content)
	evt := &database.Event{
		RowID:     1,
		ID:        id.EventID("$img_evt_1"),
		Sender:    id.UserID("@alice:matrix.org"),
		Type:      event.EventMessage.Type,
		Content:   raw,
		Timestamp: jsontime.UM(time.Now()),
	}
	uiMsg := messages.NewFileMessage(nil, nil, evt, content)
	evt.RenderMeta = uiMsg
	fm := uiMsg.Renderer.(*messages.FileMessage)
	return uiMsg, fm
}

func TestRoomView_OnDemand_SelectImageCallsRequestPreview(t *testing.T) {
	uiMsg, fm := makeTestFileMessage()

	// 1. Initially preview is not requested
	if fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to be false initially")
	}

	rv := &RoomView{
		selecting:    true,
		selectReason: SelectImage,
	}

	// 2. Calling OnSelect with SelectImage must trigger RequestPreview() and set status notification
	rv.OnSelect(uiMsg)

	if !fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to be true after rv.OnSelect(SelectImage) (mutant detected: case SelectImage was no-oped)")
	}
	if rv.tempStatus != "Loading image preview..." {
		t.Fatalf("expected status notification 'Loading image preview...', got %q", rv.tempStatus)
	}

	// 3. Selection must remain active for SelectImage (so multiple images can be loaded)
	if !rv.selecting {
		t.Fatalf("expected rv.selecting to remain true after SelectImage")
	}
}

func TestRoomView_OnDemand_SelectImageIdempotency(t *testing.T) {
	uiMsg, fm := makeTestFileMessage()

	rv := &RoomView{
		selecting:    true,
		selectReason: SelectImage,
	}

	rv.OnSelect(uiMsg)
	if !fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to be true")
	}

	// Calling OnSelect second time must be safe and idempotent
	rv.OnSelect(uiMsg)
	if !fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to remain true")
	}
}

func TestRoomView_OnDemand_KeybindingLoadImageMapping(t *testing.T) {
	cfg := config.NewConfig()
	cfg.LoadKeybindings()

	// Test Ctrl+o mapping
	kbCtrlO := config.Keybind{
		Key: tcell.KeyCtrlO,
		Mod: tcell.ModCtrl,
		Ch:  15,
	}
	actionCtrlO, okCtrlO := cfg.Keybindings.Room[kbCtrlO]
	if !okCtrlO {
		for k, act := range cfg.Keybindings.Room {
			if act == "load_image" && k.Key == tcell.KeyCtrlO {
				okCtrlO = true
				actionCtrlO = act
				break
			}
		}
	}
	if !okCtrlO || actionCtrlO != "load_image" {
		t.Fatalf("expected Ctrl+o to map to 'load_image', got %q (ok=%v)", actionCtrlO, okCtrlO)
	}

	// Test Alt+i mapping
	kbAltI := config.Keybind{
		Key: tcell.KeyRune,
		Mod: tcell.ModAlt,
		Ch:  'i',
	}
	actionAltI, okAltI := cfg.Keybindings.Room[kbAltI]
	if !okAltI {
		for k, act := range cfg.Keybindings.Room {
			if act == "load_image" && k.Key == tcell.KeyRune && k.Ch == 'i' && (k.Mod&tcell.ModAlt != 0) {
				okAltI = true
				actionAltI = act
				break
			}
		}
	}
	if !okAltI || actionAltI != "load_image" {
		t.Fatalf("expected Alt+i to map to 'load_image', got %q (ok=%v)", actionAltI, okAltI)
	}
}

func TestRoomView_OnDemand_OnKeyEvent_NewEventKey(t *testing.T) {
	cfg := config.NewConfig()
	cfg.LoadKeybindings()

	rv := &RoomView{
		config: cfg,
		input:  mauview.NewInputArea(),
	}

	// Test 1: Real tcell event for Ctrl+o triggers SelectImage mode
	evtCtrlO := tcell.NewEventKey(tcell.KeyCtrlO, 15, tcell.ModCtrl)
	handled := rv.OnKeyEvent(evtCtrlO)
	if !handled {
		t.Fatalf("expected OnKeyEvent(Ctrl+o) to be handled")
	}
	if !rv.selecting {
		t.Fatalf("expected rv.selecting to be true after Ctrl+o")
	}
	if rv.selectReason != SelectImage {
		t.Fatalf("expected selectReason to be SelectImage, got %q", rv.selectReason)
	}

	// OnSelect with targeted image triggers preview request
	uiMsg, fm := makeTestFileMessage()
	rv.OnSelect(uiMsg)
	if !fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to be true after OnSelect with Ctrl+o")
	}

	// Stop selecting resets state cleanly
	rv.StopSelecting()
	if rv.selecting {
		t.Fatalf("expected selecting=false after StopSelecting()")
	}
	if rv.selectReason != "" {
		t.Fatalf("expected selectReason=\"\" after StopSelecting(), got %q", rv.selectReason)
	}

	// Test 2: Real tcell event for Alt+i triggers SelectImage mode
	evtAltI := tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt)
	handled = rv.OnKeyEvent(evtAltI)
	if !handled {
		t.Fatalf("expected OnKeyEvent(Alt+i) to be handled")
	}
	if !rv.selecting {
		t.Fatalf("expected rv.selecting to be true after Alt+i")
	}
	if rv.selectReason != SelectImage {
		t.Fatalf("expected selectReason to be SelectImage, got %q", rv.selectReason)
	}

	uiMsg, fm = makeTestFileMessage()
	rv.OnSelect(uiMsg)
	if !fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to be true after OnSelect with Alt+i")
	}

	// Test 3: Tab (byte 0x09 decoded by tcell as KeyTAB with ModNone) does NOT trigger load_image
	rv.StopSelecting()
	evtTab := tcell.NewEventKey(tcell.KeyTAB, '\t', tcell.ModNone)
	_ = rv.OnKeyEvent(evtTab)
	if rv.selecting {
		t.Fatalf("expected Tab (ModNone) not to trigger selecting mode")
	}
}

func TestRoomView_OnDemand_PreferencesGating(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Preferences.DisableDownloads = true

	rv := &RoomView{
		config:       cfg,
		selecting:    true,
		selectReason: SelectImage,
	}

	uiMsg, fm := makeTestFileMessage()
	rv.OnSelect(uiMsg)

	if fm.IsPreviewRequested() {
		t.Fatalf("preview must NOT be requested when DisableDownloads=true")
	}
	if rv.tempStatus != "Downloads are disabled in settings" {
		t.Fatalf("expected status 'Downloads are disabled in settings', got %q", rv.tempStatus)
	}

	// DisableImages test
	cfg.Preferences.DisableDownloads = false
	cfg.Preferences.DisableImages = true
	rv.tempStatus = ""

	rv.OnSelect(uiMsg)
	if fm.IsPreviewRequested() {
		t.Fatalf("preview must NOT be requested when DisableImages=true")
	}
	if rv.tempStatus != "Images are disabled in settings" {
		t.Fatalf("expected status 'Images are disabled in settings', got %q", rv.tempStatus)
	}
}

func TestRoomView_OnDemand_RetryAfterFailure(t *testing.T) {
	uiMsg, fm := makeTestFileMessage()

	// Simulate previous failure
	fm.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return nil, fmt.Errorf("network error")
	})
	fm.RequestPreview()
	fm.WaitDownload()

	if !fm.HasImageError() {
		t.Fatalf("expected fm.HasImageError() to be true after failed download")
	}

	// Now fix network and retry via OnSelect
	fm.SetDownloadFunc(func(uri id.ContentURI, encrypted bool) ([]byte, error) {
		return []byte("fake-png"), nil
	})

	rv := &RoomView{
		selecting:    true,
		selectReason: SelectImage,
	}
	rv.OnSelect(uiMsg)

	if fm.HasImageError() {
		t.Fatalf("expected image error to be cleared on retry")
	}
	if rv.tempStatus != "Loading image preview..." {
		t.Fatalf("expected status 'Loading image preview...', got %q", rv.tempStatus)
	}
}

func TestRoomView_OnDemand_SelectReasonConstant(t *testing.T) {
	if SelectImage != SelectReason("load image") {
		t.Fatalf("expected SelectImage to equal 'load image', got %q", SelectImage)
	}
}
