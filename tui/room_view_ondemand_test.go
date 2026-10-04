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
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
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
		ID:        id.EventID("$img_evt_1"),
		Sender:    id.UserID("@alice:matrix.org"),
		Type:      event.EventMessage.Type,
		Content:   raw,
		Timestamp: jsontime.UM(time.Now()),
	}
	uiMsg := messages.NewFileMessage(nil, nil, evt, content)
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

	// 2. Calling OnSelect with SelectImage must trigger RequestPreview()
	rv.OnSelect(uiMsg)

	if !fm.IsPreviewRequested() {
		t.Fatalf("expected fm.IsPreviewRequested() to be true after rv.OnSelect(SelectImage)")
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
	kb := config.Keybind{
		Key: tcell.KeyCtrlI,
		Mod: tcell.ModCtrl,
		Ch:  9,
	}
	action, ok := cfg.Keybindings.Room[kb]
	if !ok {
		// Try without ModCtrl or Ch if normalized differently
		found := false
		for k, act := range cfg.Keybindings.Room {
			if act == "load_image" {
				found = true
				if k.Key != tcell.KeyCtrlI {
					t.Errorf("expected KeyCtrlI for load_image, got %v", k.Key)
				}
				break
			}
		}
		if !found {
			t.Fatalf("expected Ctrl+i keybinding mapping to 'load_image' in room keybindings")
		}
		return
	}
	if action != "load_image" {
		t.Fatalf("expected Ctrl+i to map to 'load_image', got %q", action)
	}
}

func TestRoomView_OnDemand_SelectReasonConstant(t *testing.T) {
	if SelectImage != SelectReason("load image") {
		t.Fatalf("expected SelectImage to equal 'load image', got %q", SelectImage)
	}
}
