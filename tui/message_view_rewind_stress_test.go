// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132

package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestChallenger2_Draw_Rewind_InfiniteLoop_ZeroHeight tests if MessageView.Draw() hangs when a multi-line message
// has height 0, and viewStart lands on a non-first line of that message.
func TestChallenger2_Draw_Rewind_InfiniteLoop_ZeroHeight(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 5, 3)
	mv.update(80)

	pngData := makeTestPNG(80, 80)
	fileMsg.SetImageData(pngData)
	mv.InvalidateTimeline()
	mv.update(80)

	initialH := fileMsg.Height()
	t.Logf("fileMsg initial height: %d", initialH)
	if initialH <= 1 {
		t.Fatalf("expected fileMsg height > 1, got %d", initialH)
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

	// Simulate buffer being cleared so Height() returns 0
	fileMsg.SetImageData(pngData)

	screenH := 24
	totalH := mv.TotalHeight()
	targetIndexOffset := fileMsgStartIdx + 1
	targetScrollOffset := totalH - screenH - targetIndexOffset
	if targetScrollOffset >= 0 {
		mv.ScrollOffset.Store(int32(targetScrollOffset))
	} else {
		screenH = totalH - targetIndexOffset
		mv.ScrollOffset.Store(0)
	}

	sim := tcell.NewSimulationScreen("")
	_ = sim.Init()
	sim.SetSize(80, screenH)

	done := make(chan struct{})
	go func() {
		mv.Draw(sim)
		close(done)
	}()

	select {
	case <-done:
		t.Log("Draw() returned cleanly")
	case <-time.After(1 * time.Second):
		t.Fatal("DEADLOCK / INFINITE LOOP DETECTED: Draw() hung on zero-height message at rewind boundary")
	}
}

// TestChallenger2_Draw_Rewind_OscillatingLoop_HeightOne tests if MessageView.Draw() hangs in an oscillating loop
// when a 16-row message shrinks to height 1, and viewStart lands on line 2 (offset +2).
func TestChallenger2_Draw_Rewind_OscillatingLoop_HeightOne(t *testing.T) {
	mv, _, _, fileMsg := setupTestMessageView(t, 5, 3)
	mv.update(80)

	pngData := makeTestPNG(80, 80)
	fileMsg.SetImageData(pngData)
	mv.InvalidateTimeline()
	mv.update(80)

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

	// Shrink message buffer to height 1 (e.g. by setting DisableImages)
	prefs := mv.config.Preferences
	prefs.DisableImages = true
	fileMsg.CalculateBuffer(prefs, 80, mv.msgBuffer[fileMsgStartIdx])

	if h := fileMsg.Height(); h != 1 {
		t.Fatalf("expected fileMsg height 1, got %d", h)
	}

	screenH := 24
	totalH := mv.TotalHeight()
	targetIndexOffset := fileMsgStartIdx + 2 // offset by 2 so rewind moves line back by 2 to -2
	targetScrollOffset := totalH - screenH - targetIndexOffset
	if targetScrollOffset >= 0 {
		mv.ScrollOffset.Store(int32(targetScrollOffset))
	} else {
		screenH = totalH - targetIndexOffset
		mv.ScrollOffset.Store(0)
	}

	sim := tcell.NewSimulationScreen("")
	_ = sim.Init()
	sim.SetSize(80, screenH)

	done := make(chan struct{})
	go func() {
		mv.Draw(sim)
		close(done)
	}()

	select {
	case <-done:
		t.Log("Draw() returned cleanly")
	case <-time.After(1 * time.Second):
		t.Fatal("DEADLOCK / OSCILLATING INFINITE LOOP DETECTED: Draw() hung when height=1 and rewind=2")
	}
}
