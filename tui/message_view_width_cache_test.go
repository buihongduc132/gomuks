// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132

package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestMessageView_WidthCaching_PreservesOuterWidth verifies that update() preserves the outer screen
// width in prevWidth across non-bare mode, and successfully hits the early return cache.
func TestMessageView_WidthCaching_PreservesOuterWidth(t *testing.T) {
	mv, _, _, _ := setupTestMessageView(t, 5, 0)

	// Step 1: Default non-bare mode with screen width 100
	mv.update(100)

	if mv.prevWidth != 100 {
		t.Fatalf("expected prevWidth to store outer screen width 100, got %d", mv.prevWidth)
	}

	// Capture initial buffer pointer
	origBuffer := mv.msgBuffer

	// Step 2: Call update(100) again without invalidation; must hit early return
	mv.update(100)

	if len(mv.msgBuffer) == 0 || &mv.msgBuffer[0] != &origBuffer[0] {
		t.Fatalf("expected early return to reuse buffer slice, but buffer was reallocated")
	}

	// Step 3: Terminal resize to 120; must recalculate and update prevWidth
	mv.update(120)

	if mv.prevWidth != 120 {
		t.Fatalf("expected prevWidth to update to 120, got %d", mv.prevWidth)
	}
}

// TestMessageView_WidthCaching_BareMode verifies that update() also preserves prevWidth in bare mode.
func TestMessageView_WidthCaching_BareMode(t *testing.T) {
	mv, _, _, _ := setupTestMessageView(t, 5, 0)
	mv.config.Preferences.BareMessageView = true

	mv.update(90)
	if mv.prevWidth != 90 {
		t.Fatalf("expected prevWidth to store 90 in bare mode, got %d", mv.prevWidth)
	}

	origBuffer := mv.msgBuffer
	mv.update(90)
	if len(mv.msgBuffer) == 0 || &mv.msgBuffer[0] != &origBuffer[0] {
		t.Fatalf("expected early return to reuse buffer slice in bare mode")
	}
}

// TestMessageView_Draw_ZeroHeightMessage_DoesNotHang asserts that Draw() completes cleanly
// and does not enter an infinite loop when encountering a message whose Height() is 0.
func TestMessageView_Draw_ZeroHeightMessage_DoesNotHang(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 5, 3)
	mv.update(80)

	// Clear image buffer to force fileMsg.Height() == 0
	fileMsg.SetImageData(makeTestPNG(60, 60))
	if h := fileMsg.Height(); h != 0 {
		t.Fatalf("expected fileMsg.Height() == 0, got %d", h)
	}

	sim := tcell.NewSimulationScreen("")
	_ = sim.Init()
	sim.SetSize(80, 24)

	done := make(chan struct{})
	go func() {
		mv.Draw(sim)
		close(done)
	}()

	select {
	case <-done:
		t.Log("Draw() returned cleanly without hanging")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("DEADLOCK / INFINITE LOOP: MessageView.Draw() hung on zero-height message")
	}
}
