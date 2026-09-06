// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Empirical Challenger 2 Test Suite for Milestone M3:
// Timeline Expansion Under Active Scroll & InvalidateTimeline() Scroll Offset Retention.

package tui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
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

// makeTestPNG creates a valid colored PNG image of given dimensions.
func makeTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 200, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func setupTestMessageView(t *testing.T, count int, imageIndex int) (*MessageView, *store.RoomStore, []*database.Event, *messages.FileMessage) {
	cfg := config.NewConfig()
	cfg.Preferences.ImagePreviewProtocol = "iterm2"
	cfg.Preferences.ImagePreviewMaxWidth = 66
	cfg.Preferences.ImagePreviewMaxHeight = 16

	roomID := id.RoomID("!challenger_m3:example.org")
	meta := &database.Room{ID: roomID}
	room := store.NewRoomStore(nil, meta)

	var events []*database.Event
	var targetFileMsg *messages.FileMessage

	now := jsontime.UnixMilliNow()

	for i := 1; i <= count; i++ {
		rowID := database.EventRowID(i)
		evtID := id.EventID(fmt.Sprintf("$evt_%d", i))
		var evt *database.Event

		if i == imageIndex {
			content := &event.MessageEventContent{
				MsgType: event.MsgImage,
				Body:    fmt.Sprintf("photo_%d.png", i),
				URL:     "mxc://example.org/photo",
			}
			evt = &database.Event{
				RowID:     rowID,
				ID:        evtID,
				Timestamp: now,
			}
			uiMsg := messages.NewFileMessage(room, nil, evt, content)
			targetFileMsg = uiMsg.Renderer.(*messages.FileMessage)
			evt.RenderMeta = uiMsg
		} else {
			evt = &database.Event{
				RowID:     rowID,
				ID:        evtID,
				Timestamp: now,
			}
			text := fmt.Sprintf("Message number %d in timeline", i)
			uiMsg := messages.NewExpandedTextMessage(evt, room, tstring.NewTString(text))
			evt.RenderMeta = uiMsg
		}
		events = append(events, evt)
	}

	room.TimelineCache.SetCurrent(&events)

	mainView := &MainView{config: cfg}
	roomView := &RoomView{
		parent: mainView,
		Room:   room,
		config: cfg,
	}
	mv := NewMessageView(roomView)
	return mv, room, events, targetFileMsg
}

// ---------------------------------------------------------------------------
// 1. Timeline Expansion Under Active Scroll: scrollOffset > 0 Retention
// ---------------------------------------------------------------------------
// Simulates user scrolling up in timeline (scrollOffset = 15).
// An image message in the middle of the timeline completes async download,
// calls InvalidateTimeline(), and update(width) recalculates buffer heights.
// Asserts that scrollOffset is STRICTLY NOT reset to 0.
func TestChallenger2_M3_TimelineExpansion_ActiveScrollOffsetRetention(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 20, 10)

	// Step 1: Initial layout update (width = 80)
	mv.update(80)
	initialHeight := mv.TotalHeight()
	if initialHeight == 0 {
		t.Fatalf("expected initial TotalHeight > 0, got %d", initialHeight)
	}

	// Step 2: User scrolls up by 15 rows
	mv.ScrollOffset.Store(15)
	if offset := mv.GetScrollOffset(); offset != 15 {
		t.Fatalf("expected scrollOffset 15, got %d", offset)
	}

	// Step 3: Simulate async image download completion
	pngData := makeTestPNG(120, 120)
	fileMsg.SetImageData(pngData)

	// Step 4: Invalidate timeline and trigger recalculation
	mv.InvalidateTimeline()
	mv.update(80)

	// Step 5: Assert scrollOffset is NOT reset to 0
	newOffset := mv.GetScrollOffset()
	if newOffset == 0 {
		t.Fatalf("VIOLATION: InvalidateTimeline() reset active scrollOffset to 0! (expected >= 15)")
	}
	if newOffset < 15 {
		t.Errorf("expected scrollOffset >= 15, got %d", newOffset)
	}

	// Step 6: Assert TotalHeight expanded due to the 16-row image
	newHeight := mv.TotalHeight()
	if newHeight <= initialHeight {
		t.Errorf("expected TotalHeight to expand: initial=%d, new=%d", initialHeight, newHeight)
	}
}

// ---------------------------------------------------------------------------
// 2. Timeline Expansion when Image is the Last Event in Timeline
// ---------------------------------------------------------------------------
// Verifies that when the expanding image is the last event in the timeline
// and the user is scrolled up, scrollOffset is still preserved and not wiped to 0.
func TestChallenger2_M3_TimelineExpansion_LastEventImage_ActiveScroll(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 15, 15) // Event 15 is the image

	mv.update(80)
	mv.ScrollOffset.Store(10)
	if offset := mv.GetScrollOffset(); offset != 10 {
		t.Fatalf("expected scrollOffset 10, got %d", offset)
	}

	pngData := makeTestPNG(120, 120)
	fileMsg.SetImageData(pngData)

	mv.InvalidateTimeline()
	mv.update(80)

	newOffset := mv.GetScrollOffset()
	if newOffset == 0 {
		t.Fatalf("VIOLATION: scrollOffset was reset to 0 when last event image expanded!")
	}
	if newOffset < 10 {
		t.Errorf("expected scrollOffset >= 10, got %d", newOffset)
	}
}

// ---------------------------------------------------------------------------
// 3. Repeated InvalidateTimeline() Calls Under Scroll
// ---------------------------------------------------------------------------
// Rapid succession of image downloads completing while user is reading history.
// Asserts scrollOffset remains positive across all invalidations.
func TestChallenger2_M3_TimelineExpansion_RepeatedInvalidationsUnderScroll(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 25, 12)

	mv.update(80)
	mv.ScrollOffset.Store(25)

	pngData := makeTestPNG(100, 100)

	for iter := 1; iter <= 10; iter++ {
		fileMsg.SetImageData(pngData)
		mv.InvalidateTimeline()
		mv.update(80)

		offset := mv.GetScrollOffset()
		if offset == 0 {
			t.Fatalf("iteration %d: scrollOffset prematurely reset to 0", iter)
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Concurrent Scroll, Invalidation, Update, and Draw Stress Under Race Detector
// ---------------------------------------------------------------------------
func TestChallenger2_M3_TimelineExpansion_ConcurrentStress(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 30, 15)
	mv.update(80)
	mv.ScrollOffset.Store(10)

	pngData := makeTestPNG(80, 80)

	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	// Goroutine 1: User scrolling
	wg.Add(1)
	go func() {
		defer wg.Done()
		scrollVal := int32(5)
		for {
			select {
			case <-stopCh:
				return
			default:
				mv.ScrollOffset.Store(scrollVal)
				_ = mv.GetScrollOffset()
				scrollVal = (scrollVal % 40) + 1
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	// Goroutine 2: Async download completions calling InvalidateTimeline
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			default:
				fileMsg.SetImageData(pngData)
				mv.InvalidateTimeline()
				time.Sleep(3 * time.Millisecond)
			}
		}
	}()

	// Goroutine 3: Render loop calling update()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			default:
				mv.update(80)
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	// Goroutine 4: TUI drawing
	wg.Add(1)
	go func() {
		defer wg.Done()
		sim := tcell.NewSimulationScreen("")
		_ = sim.Init()
		sim.SetSize(80, 24)
		for {
			select {
			case <-stopCh:
				return
			default:
				mv.Draw(sim)
				time.Sleep(3 * time.Millisecond)
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stopCh)
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 5. Adversarial Reproduction: Infinite Loop Hang on Zero-Height Message in Draw()
// ---------------------------------------------------------------------------
// When async download completes (or SetImageData is called), filemessage.go
// clears msg.buffer = nil (lines 438, 531).
// If Draw() executes before CalculateBuffer() rebuilds the buffer, msg.Height() returns 0.
// In MessageView.Draw() line 382:
//     line += msg.Height()
// Because msg.Height() is 0, line never increments, causing an infinite loop.
func TestChallenger2_M3_Draw_InfiniteLoop_OnZeroHeightMessage(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 5, 3)
	mv.update(80)

	// Simulate DownloadPreview() clearing buffer to nil
	fileMsg.SetImageData(makeTestPNG(60, 60))
	// At this point, fileMsg.buffer is nil, so fileMsg.Height() == 0:
	if h := fileMsg.Height(); h != 0 {
		t.Fatalf("expected fileMsg.Height() to be 0 after SetImageData before CalculateBuffer, got %d", h)
	}

	sim := tcell.NewSimulationScreen("")
	_ = sim.Init()
	sim.SetSize(80, 24)

	drawDone := make(chan struct{})
	go func() {
		// Acquire lock and call the draw loop (which uses msgBuffer from previous update)
		// without calling update() first, simulating concurrent Draw loop execution
		mv.Draw(sim)
		close(drawDone)
	}()

	select {
	case <-drawDone:
		t.Log("Draw() returned successfully without hanging")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("CRITICAL BUG CONFIRMED: MessageView.Draw() entered an INFINITE LOOP because msg.Height() == 0")
	}
}
