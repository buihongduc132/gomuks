// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Empirical Challenger 2 Test Suite for Milestone M3:
// Viewport Boundary Clipping, Cell-Masking, and OSC 1337 Suppression Safety.

package messages

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/gomuks/tui/config"
)

// makeChallengerM3PNG creates a valid colored PNG image of given dimensions.
func makeChallengerM3PNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 50, G: 150, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// 1. Top Boundary Clipping: line < 0 and screenY < 1
// ---------------------------------------------------------------------------
// Tests that when an image is partially scrolled off the top of the viewport
// (screenY < 1, e.g. line = -10, -5, -2, -1 in contentProxy where screenY = 1 + line <= 0,
// or screenY = 0 directly on root screen):
// - OSC 1337 is strictly suppressed (0 bytes emitted).
// - Cell mask is strictly cleared (CurrentMask() == nil).
// - Topic Bar (Row 0) is completely untouched (no graphic bleed).
// - Half-blocks ('▄') cleanly draw into the visible slice.
func TestChallenger2_M3_TopBoundaryClipping_SafetyAndHalfBlocks(t *testing.T) {
	pngData := makeChallengerM3PNG(80, 80)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "top_clip.png",
		URL:     "mxc://matrix.example.com/top_clip",
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
	if imgRows < 3 {
		t.Fatalf("expected imgRows >= 3, got %d", imgRows)
	}

	// Simulation screen: 100 cols x 40 rows.
	// Row 0 is Topic Bar (filled with 'T').
	// Rows 1..36 is Content Area (height 36).
	// Row 37 is Status Bar (filled with 'S').
	// Rows 38..39 are Input Area.
	simScreen := newSimulationScreen(100, 40)
	for x := 0; x < 100; x++ {
		simScreen.SetCell(x, 0, tcell.StyleDefault.Foreground(tcell.ColorGreen), 'T')
		simScreen.SetCell(x, 37, tcell.StyleDefault.Foreground(tcell.ColorYellow), 'S')
	}

	contentProxy := mauview.NewProxyScreen(simScreen, 0, 1, 100, 36)

	var oscBuf bytes.Buffer
	SetOSCOutputWriter(&oscBuf)
	defer SetOSCOutputWriter(nil)

	// In contentProxy (OffsetY=1), line < 0 corresponds to:
	// line = -10 => screenY = -9 < 1
	// line = -5  => screenY = -4 < 1
	// line = -2  => screenY = -1 < 1
	// line = -1  => screenY = 0 < 1
	testLines := []int{-10, -5, -2, -1}
	for _, line := range testLines {
		oscBuf.Reset()

		// Clear content area to spaces before each subtest
		for y := 0; y < 36; y++ {
			for x := 0; x < 100; x++ {
				contentProxy.SetCell(x, y, tcell.StyleDefault, ' ')
			}
		}

		// Message proxy at Y = line relative to contentProxy
		msgProxy := mauview.NewProxyScreen(contentProxy, 2, line, 70, imgRows)
		fileMsg.Draw(msgProxy, uiMsg)

		// 1. Assert OSC 1337 strictly suppressed
		if oscBuf.Len() != 0 {
			t.Errorf("[line=%d] expected OSC 1337 strictly suppressed, got %d bytes", line, oscBuf.Len())
		}

		// 2. Assert Cell Mask cleared
		if mask := fileMsg.CurrentMask(); mask != nil {
			t.Errorf("[line=%d] expected CurrentMask() to be nil when clipped, got %+v", line, mask)
		}

		// 3. Assert Topic Bar (Row 0) has ZERO bleed
		for x := 0; x < 100; x++ {
			ch, _, _, _ := simScreen.GetContent(x, 0)
			if ch != 'T' {
				t.Fatalf("[line=%d] Topic Bar at (x=%d, y=0) corrupted: expected 'T', got %c", line, x, ch)
			}
		}

		// 4. Assert Status Bar (Row 37) has ZERO bleed
		for x := 0; x < 100; x++ {
			ch, _, _, _ := simScreen.GetContent(x, 37)
			if ch != 'S' {
				t.Fatalf("[line=%d] Status Bar at (x=%d, y=37) corrupted: expected 'S', got %c", line, x, ch)
			}
		}

		// 5. Assert half-blocks cleanly draw in the visible slice
		startScreenY := 1 + line
		if startScreenY < 1 {
			startScreenY = 1
		}
		endScreenY := 1 + line + imgRows - 1
		if endScreenY > 36 {
			endScreenY = 36
		}

		if startScreenY <= endScreenY {
			hasHalfBlock := false
			for sy := startScreenY; sy <= endScreenY; sy++ {
				ch, _, _, _ := simScreen.GetContent(5, sy)
				if ch == '▄' {
					hasHalfBlock = true
					break
				}
			}
			if !hasHalfBlock {
				t.Errorf("[line=%d] expected half-block '▄' rendered in visible slice [%d..%d], none found",
					line, startScreenY, endScreenY)
			}
		}
	}

	// Subtest: Direct screenY = 0 on rootScreen (overlapping Topic Bar at y=0)
	oscBuf.Reset()
	rootDirectProxy := mauview.NewProxyScreen(simScreen, 2, 0, 70, imgRows)
	fileMsg.Draw(rootDirectProxy, uiMsg)
	if oscBuf.Len() != 0 {
		t.Errorf("[direct screenY=0] expected OSC 1337 suppressed, got %d bytes", oscBuf.Len())
	}
	if mask := fileMsg.CurrentMask(); mask != nil {
		t.Errorf("[direct screenY=0] expected mask nil, got %+v", mask)
	}
}

// ---------------------------------------------------------------------------
// 2. Bottom Boundary Clipping: screenY + rows > maxAllowedY
// ---------------------------------------------------------------------------
// Tests that when an image extends past the bottom of the viewport
// (screenY + rows > maxAllowedY, e.g. overlapping Status Bar):
// - OSC 1337 is strictly suppressed (0 bytes emitted).
// - Cell mask is strictly cleared.
// - Status Bar (Row 37) is completely untouched.
// - Half-blocks cleanly draw into the visible upper portion of the message.
func TestChallenger2_M3_BottomBoundaryClipping_SafetyAndHalfBlocks(t *testing.T) {
	pngData := makeChallengerM3PNG(80, 80)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "bottom_clip.png",
		URL:     "mxc://matrix.example.com/bottom_clip",
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
	for x := 0; x < 100; x++ {
		simScreen.SetCell(x, 0, tcell.StyleDefault.Foreground(tcell.ColorGreen), 'T')
		simScreen.SetCell(x, 37, tcell.StyleDefault.Foreground(tcell.ColorYellow), 'S')
	}

	contentProxy := mauview.NewProxyScreen(simScreen, 0, 1, 100, 36)

	var oscBuf bytes.Buffer
	SetOSCOutputWriter(&oscBuf)
	defer SetOSCOutputWriter(nil)

	// In contentProxy (Height=36, OffsetY=1), maxAllowedY = 37 (Status bar is row 37).
	// Image rows = 16.
	// When line = 21: screenY = 22, 22+16 = 38 > 37 (clipped by 1 row).
	// When line = 30: screenY = 31, 31+16 = 47 > 37 (clipped by 10 rows).
	// When line = 35: screenY = 36, only 1 row visible in content area.
	// When line = 36: screenY = 37, starts directly on Status Bar (0 rows visible).
	testLines := []int{21, 25, 30, 35, 36, 40}
	for _, line := range testLines {
		oscBuf.Reset()

		for y := 0; y < 36; y++ {
			for x := 0; x < 100; x++ {
				contentProxy.SetCell(x, y, tcell.StyleDefault, ' ')
			}
		}

		msgProxy := mauview.NewProxyScreen(contentProxy, 2, line, 70, imgRows)
		fileMsg.Draw(msgProxy, uiMsg)

		// 1. Assert OSC 1337 strictly suppressed
		if oscBuf.Len() != 0 {
			t.Errorf("[line=%d] expected OSC 1337 strictly suppressed, got %d bytes", line, oscBuf.Len())
		}

		// 2. Assert Cell Mask cleared
		if mask := fileMsg.CurrentMask(); mask != nil {
			t.Errorf("[line=%d] expected CurrentMask() to be nil when bottom clipped, got %+v", line, mask)
		}

		// 3. Assert Topic Bar (Row 0) untouched
		for x := 0; x < 100; x++ {
			ch, _, _, _ := simScreen.GetContent(x, 0)
			if ch != 'T' {
				t.Fatalf("[line=%d] Topic Bar corrupted at x=%d: expected 'T', got %c", line, x, ch)
			}
		}

		// 4. Assert Status Bar (Row 37) untouched
		for x := 0; x < 100; x++ {
			ch, _, _, _ := simScreen.GetContent(x, 37)
			if ch != 'S' {
				t.Fatalf("[line=%d] Status Bar corrupted at x=%d: expected 'S', got %c", line, x, ch)
			}
		}

		// 5. Assert half-blocks cleanly draw in the visible slice (if any)
		if line < 36 {
			startScreenY := 1 + line
			endScreenY := 36
			hasHalfBlock := false
			for sy := startScreenY; sy <= endScreenY; sy++ {
				ch, _, _, _ := simScreen.GetContent(5, sy)
				if ch == '▄' {
					hasHalfBlock = true
					break
				}
			}
			if !hasHalfBlock {
				t.Errorf("[line=%d] expected half-blocks in visible slice [%d..%d]", line, startScreenY, endScreenY)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Boundary Crossing Transitions: Clipped <-> Fully Visible
// ---------------------------------------------------------------------------
// Verifies that state transitions (e.g. scrolling a message into view then out):
// - Seamlessly emits OSC 1337 and applies cell mask when fully visible.
// - Immediately clears cell mask and suppresses OSC 1337 when clipped.
func TestChallenger2_M3_BoundaryTransition_StateToggling(t *testing.T) {
	pngData := makeChallengerM3PNG(80, 80)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "transition.png",
		URL:     "mxc://matrix.example.com/transition",
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
	contentProxy := mauview.NewProxyScreen(simScreen, 0, 1, 100, 36)

	var oscBuf bytes.Buffer
	SetOSCOutputWriter(&oscBuf)
	defer SetOSCOutputWriter(nil)

	// Step 1: Scrolled top-clipped at line = -2
	oscBuf.Reset()
	p1 := mauview.NewProxyScreen(contentProxy, 2, -2, 70, imgRows)
	fileMsg.Draw(p1, uiMsg)
	if oscBuf.Len() != 0 {
		t.Errorf("Step 1: expected OSC suppressed when clipped")
	}
	if fileMsg.CurrentMask() != nil {
		t.Errorf("Step 1: expected mask nil when clipped")
	}

	// Step 2: Scrolled into full visibility at line = 2 (screenY = 3, 3+16=19 <= 37)
	oscBuf.Reset()
	p2 := mauview.NewProxyScreen(contentProxy, 2, 2, 70, imgRows)
	fileMsg.Draw(p2, uiMsg)
	if oscBuf.Len() == 0 {
		t.Errorf("Step 2: expected OSC emitted when fully visible")
	}
	if mask := fileMsg.CurrentMask(); mask == nil || !mask.Active {
		t.Errorf("Step 2: expected active cell mask when fully visible")
	}

	// Step 3: Scrolled bottom-clipped at line = 25 (screenY = 26, 26+16=42 > 37)
	oscBuf.Reset()
	p3 := mauview.NewProxyScreen(contentProxy, 2, 25, 70, imgRows)
	fileMsg.Draw(p3, uiMsg)
	if oscBuf.Len() != 0 {
		t.Errorf("Step 3: expected OSC suppressed when bottom clipped")
	}
	if fileMsg.CurrentMask() != nil {
		t.Errorf("Step 3: expected mask cleared when bottom clipped")
	}

	// Step 4: Scrolled back into full visibility at line = 5
	oscBuf.Reset()
	p4 := mauview.NewProxyScreen(contentProxy, 2, 5, 70, imgRows)
	fileMsg.Draw(p4, uiMsg)
	if oscBuf.Len() == 0 {
		t.Errorf("Step 4: expected OSC re-emitted when returned to full visibility")
	}
	if mask := fileMsg.CurrentMask(); mask == nil || !mask.Active {
		t.Errorf("Step 4: expected active cell mask restored")
	}
}

// ---------------------------------------------------------------------------
// 4. Concurrency Stress on Boundary Clipping Under Race Detector
// ---------------------------------------------------------------------------
func TestChallenger2_M3_ConcurrentClippingStress(t *testing.T) {
	pngData := makeChallengerM3PNG(80, 80)
	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "concurrent_clip.png",
		URL:     "mxc://matrix.example.com/concurrent_clip",
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

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			sim := newSimulationScreen(100, 40)
			content := mauview.NewProxyScreen(sim, 0, 1, 100, 36)

			for iter := 0; iter < iterations; iter++ {
				// Vary offsets across clipped top, visible, and clipped bottom
				offset := (iter%60) - 20 // ranges from -20 to +39
				proxy := mauview.NewProxyScreen(content, 2, offset, 70, imgRows)

				if iter%3 == 0 {
					fileMsg.CalculateBuffer(prefs, 80+iter%10, uiMsg)
				}
				fileMsg.Draw(proxy, uiMsg)

				if iter%5 == 0 {
					_ = fileMsg.CurrentMask()
					_ = fileMsg.RenderCache()
					_ = fileMsg.CachedOSC()
				}
			}
		}(i)
	}

	wg.Wait()
}
