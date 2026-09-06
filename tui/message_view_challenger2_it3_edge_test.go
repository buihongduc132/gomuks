// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132

package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/messages"
	"go.mau.fi/gomuks/tui/messages/tstring"
)

// Helper to create a single-message test room
func setupSingleMessageView(t *testing.T, isImage bool, multiLineCount int) (*MessageView, *messages.FileMessage, *messages.UIMessage) {
	cfg := config.NewConfig()
	cfg.Preferences.ImagePreviewProtocol = "iterm2"
	cfg.Preferences.ImagePreviewMaxWidth = 66
	cfg.Preferences.ImagePreviewMaxHeight = 16

	roomID := id.RoomID("!challenger_single:example.org")
	meta := &database.Room{ID: roomID}
	room := store.NewRoomStore(nil, meta)

	now := jsontime.UnixMilliNow()
	var evt *database.Event
	var fileMsg *messages.FileMessage
	var uiMsg *messages.UIMessage

	if isImage {
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    "single_image.png",
			URL:     "mxc://example.org/single",
		}
		evt = &database.Event{
			RowID:     1,
			ID:        "$evt_single_img",
			Timestamp: now,
		}
		uiMsg = messages.NewFileMessage(room, nil, evt, content)
		fileMsg = uiMsg.Renderer.(*messages.FileMessage)
		evt.RenderMeta = uiMsg
	} else {
		evt = &database.Event{
			RowID:     1,
			ID:        "$evt_single_txt",
			Timestamp: now,
		}
		var text string
		if multiLineCount <= 1 {
			text = "Single line message"
		} else {
			text = "Line 1"
			for i := 2; i <= multiLineCount; i++ {
				text += fmt.Sprintf("\nLine %d", i)
			}
		}
		uiMsg = messages.NewExpandedTextMessage(evt, room, tstring.NewTString(text))
		evt.RenderMeta = uiMsg
	}

	events := []*database.Event{evt}
	room.TimelineCache.SetCurrent(&events)

	mainView := &MainView{config: cfg}
	roomView := &RoomView{
		parent: mainView,
		Room:   room,
		config: cfg,
	}
	mv := NewMessageView(roomView)
	return mv, fileMsg, uiMsg
}

// ---------------------------------------------------------------------------
// 1. Edge Case: Empty Room (len(msgBuffer) == 0)
// ---------------------------------------------------------------------------
func TestChallenger2_M3_EdgeCase_EmptyRoom(t *testing.T) {
	mv, _, _, _ := setupTestMessageView(t, 0, -1)
	mv.update(80)

	if len(mv.msgBuffer) != 0 {
		t.Fatalf("expected len(msgBuffer) == 0, got %d", len(mv.msgBuffer))
	}

	screenSizes := [][2]int{
		{80, 24},
		{0, 0},
		{1, 1},
		{80, 1},
		{10, 2},
		{120, 60},
	}

	scrollOffsets := []int32{0, 10, -5, 100}

	for _, size := range screenSizes {
		w, h := size[0], size[1]
		sim := tcell.NewSimulationScreen("")
		_ = sim.Init()
		sim.SetSize(w, h)

		for _, scroll := range scrollOffsets {
			mv.ScrollOffset.Store(scroll)

			done := make(chan struct{})
			start := time.Now()
			go func() {
				mv.Draw(sim)
				close(done)
			}()

			select {
			case <-done:
				dur := time.Since(start)
				if dur > 50*time.Millisecond {
					t.Errorf("Draw() exceeded 50ms limit for size (%d, %d), scroll %d: took %v", w, h, scroll, dur)
				}
			case <-time.After(100 * time.Millisecond):
				t.Fatalf("DEADLOCK / HANG on empty room Draw() with size (%d, %d), scroll %d", w, h, scroll)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Edge Case: Single Message Buffer
// ---------------------------------------------------------------------------
func TestChallenger2_M3_EdgeCase_SingleMessageBuffer(t *testing.T) {
	testCases := []struct {
		name           string
		isImage        bool
		multiLine      int
		withImageData  bool
		shrinkToZero   bool
		shrinkToHeight int
	}{
		{
			name:      "Single single-line text message",
			isImage:   false,
			multiLine: 1,
		},
		{
			name:      "Single 10-line text message",
			isImage:   false,
			multiLine: 10,
		},
		{
			name:          "Single image message (rendered height 16)",
			isImage:       true,
			withImageData: true,
		},
		{
			name:          "Single image message with zero height (no image data)",
			isImage:       true,
			withImageData: false,
		},
		{
			name:          "Single image message shrunk to zero right before Draw",
			isImage:       true,
			withImageData: true,
			shrinkToZero:  true,
		},
		{
			name:           "Single image message shrunk to height 1 right before Draw",
			isImage:        true,
			withImageData:  true,
			shrinkToHeight: 1,
		},
	}

	screenSizes := [][2]int{
		{80, 24},
		{80, 1},
		{80, 2},
		{1, 1},
		{0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mv, fileMsg, _ := setupSingleMessageView(t, tc.isImage, tc.multiLine)

			if tc.isImage && tc.withImageData {
				fileMsg.SetImageData(makeTestPNG(80, 80))
				mv.InvalidateTimeline()
			}
			mv.update(80)

			totalH := mv.TotalHeight()

			if tc.shrinkToZero && fileMsg != nil {
				// Re-set image data to simulate buffer reset before CalculateBuffer
				fileMsg.SetImageData(makeTestPNG(80, 80))
				if h := fileMsg.Height(); h != 0 {
					t.Fatalf("expected height 0 after shrinkToZero, got %d", h)
				}
			} else if tc.shrinkToHeight > 0 && fileMsg != nil {
				prefs := mv.config.Preferences
				prefs.DisableImages = true
				fileMsg.CalculateBuffer(prefs, 80, mv.msgBuffer[0])
				if h := fileMsg.Height(); h != tc.shrinkToHeight {
					t.Fatalf("expected height %d, got %d", tc.shrinkToHeight, h)
				}
			}

			// Sweep scroll offsets: 0, 1, totalH-1, totalH, totalH+5, -5
			scrollOffsets := []int32{0, 1, int32(totalH - 1), int32(totalH), int32(totalH + 5), -5}

			for _, size := range screenSizes {
				w, h := size[0], size[1]
				sim := tcell.NewSimulationScreen("")
				_ = sim.Init()
				sim.SetSize(w, h)

				for _, scroll := range scrollOffsets {
					mv.ScrollOffset.Store(scroll)

					done := make(chan struct{})
					start := time.Now()
					go func() {
						mv.Draw(sim)
						close(done)
					}()

					select {
					case <-done:
						dur := time.Since(start)
						if dur > 50*time.Millisecond {
							t.Errorf("[%s] Draw() exceeded 50ms limit for size (%d, %d), scroll %d: took %v",
								tc.name, w, h, scroll, dur)
						}
					case <-time.After(100 * time.Millisecond):
						t.Fatalf("[%s] DEADLOCK / HANG on Draw() with size (%d, %d), scroll %d",
							tc.name, w, h, scroll)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Edge Case: Adversarial Shrinkage Across All Rewind Boundary Offsets
// ---------------------------------------------------------------------------
func TestChallenger2_M3_EdgeCase_ShrinkAtAllRewindOffsets(t *testing.T) {
	// Setup 5 messages, message 3 is an image of height 16
	mv, _, _, fileMsg := setupTestMessageView(t, 5, 3)
	pngData := makeTestPNG(80, 80)
	fileMsg.SetImageData(pngData)
	mv.InvalidateTimeline()
	mv.update(80)

	initialH := fileMsg.Height()
	if initialH <= 1 {
		t.Fatalf("expected initialH > 1, got %d", initialH)
	}

	fileMsgStartIdx := -1
	for idx, msg := range mv.msgBuffer {
		if msg.Renderer == fileMsg {
			fileMsgStartIdx = idx
			break
		}
	}
	if fileMsgStartIdx == -1 {
		t.Fatalf("fileMsg not found in msgBuffer")
	}

	// Test across every offset K from 0 to initialH (0 to 16)
	for offsetK := 0; offsetK <= initialH; offsetK++ {
		// Test target shrinks: height 0, height 1, height 2, height initialH-1
		shrinkTargets := []int{0, 1, 2, initialH - 1}

		for _, targetH := range shrinkTargets {
			t.Run(fmt.Sprintf("OffsetK_%d_TargetH_%d", offsetK, targetH), func(t *testing.T) {
				// Re-setup cleanly
				curMV, _, _, curFileMsg := setupTestMessageView(t, 5, 3)
				curFileMsg.SetImageData(pngData)
				curMV.InvalidateTimeline()
				curMV.update(80)

				totalH := curMV.TotalHeight()
				screenH := 24

				targetIndexOffset := fileMsgStartIdx + offsetK
				targetScrollOffset := totalH - screenH - targetIndexOffset
				if targetScrollOffset >= 0 {
					curMV.ScrollOffset.Store(int32(targetScrollOffset))
				} else {
					screenH = totalH - targetIndexOffset
					if screenH < 1 {
						screenH = 1
					}
					curMV.ScrollOffset.Store(0)
				}

				// Perform dynamic shrink without update()
				if targetH == 0 {
					curFileMsg.SetImageData(pngData) // resets buffer to nil
				} else {
					prefs := curMV.config.Preferences
					if targetH == 1 {
						prefs.DisableImages = true
					}
					// Calculate custom buffer
					curFileMsg.CalculateBuffer(prefs, 80, curMV.msgBuffer[fileMsgStartIdx])
				}

				sim := tcell.NewSimulationScreen("")
				_ = sim.Init()
				sim.SetSize(80, screenH)

				done := make(chan struct{})
				start := time.Now()
				go func() {
					curMV.Draw(sim)
					close(done)
				}()

				select {
				case <-done:
					dur := time.Since(start)
					if dur > 50*time.Millisecond {
						t.Errorf("Draw() exceeded 50ms: took %v", dur)
					}
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("DEADLOCK / HANG on Draw() with offsetK=%d, targetH=%d", offsetK, targetH)
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Edge Case: Multiple Adjacent Zero-Height Messages
// ---------------------------------------------------------------------------
func TestChallenger2_M3_EdgeCase_MultipleAdjacentZeroHeightMessages(t *testing.T) {
	cfg := config.NewConfig()
	roomID := id.RoomID("!challenger_zeros:example.org")
	meta := &database.Room{ID: roomID}
	room := store.NewRoomStore(nil, meta)
	now := jsontime.UnixMilliNow()

	// Create 5 image messages
	var events []*database.Event
	var fileMsgs []*messages.FileMessage
	for i := 1; i <= 5; i++ {
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    fmt.Sprintf("img_%d.png", i),
			URL:     "mxc://example.org/zero",
		}
		evt := &database.Event{
			RowID:     database.EventRowID(i),
			ID:        id.EventID(fmt.Sprintf("$evt_%d", i)),
			Timestamp: now,
		}
		uiMsg := messages.NewFileMessage(room, nil, evt, content)
		fileMsg := uiMsg.Renderer.(*messages.FileMessage)
		evt.RenderMeta = uiMsg
		events = append(events, evt)
		fileMsgs = append(fileMsgs, fileMsg)
	}

	room.TimelineCache.SetCurrent(&events)
	mainView := &MainView{config: cfg}
	roomView := &RoomView{parent: mainView, Room: room, config: cfg}
	mv := NewMessageView(roomView)

	// Populate initially with image data
	pngData := makeTestPNG(60, 60)
	for _, fm := range fileMsgs {
		fm.SetImageData(pngData)
	}
	mv.InvalidateTimeline()
	mv.update(80)

	// Now clear all image buffers to 0 height
	for _, fm := range fileMsgs {
		fm.SetImageData(pngData)
		if h := fm.Height(); h != 0 {
			t.Fatalf("expected fm height 0, got %d", h)
		}
	}

	sim := tcell.NewSimulationScreen("")
	_ = sim.Init()
	sim.SetSize(80, 24)

	// Test drawing across various scroll offsets
	for scroll := int32(0); scroll <= int32(mv.TotalHeight()+5); scroll++ {
		mv.ScrollOffset.Store(scroll)
		done := make(chan struct{})
		start := time.Now()
		go func() {
			mv.Draw(sim)
			close(done)
		}()

		select {
		case <-done:
			dur := time.Since(start)
			if dur > 50*time.Millisecond {
				t.Errorf("Draw() exceeded 50ms for scroll %d: took %v", scroll, dur)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("DEADLOCK / HANG on Draw() with multiple adjacent zero-height messages at scroll %d", scroll)
		}
	}
}
