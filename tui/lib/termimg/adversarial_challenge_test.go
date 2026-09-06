// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Adversarial challenge test suite for Milestone M1

package termimg

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
)

// ---------------------------------------------------------------------------
// 1. Conflicting Environment Flags & Terminal Graphics Detection
// ---------------------------------------------------------------------------

func TestAdversarial_ConflictingEnvironmentFlags(t *testing.T) {
	tests := []struct {
		name             string
		env              map[string]string
		mockPassthrough  bool
		expectedProto    Protocol
		expectedWez      bool
		expectedITerm    bool
		expectedTmux     bool
		expectedScreen   bool
		expectedPassFlag bool
	}{
		{
			name: "TMUX + WezTerm with passthrough enabled -> iterm2",
			env: map[string]string{
				"TMUX":         "/tmp/tmux-1000/default,1,0",
				"TERM_PROGRAM": "WezTerm",
			},
			mockPassthrough:  true,
			expectedProto:    ProtocolKitty,
			expectedWez:      true,
			expectedITerm:    false,
			expectedTmux:     true,
			expectedScreen:   false,
			expectedPassFlag: true,
		},
		{
			name: "TMUX + WezTerm with passthrough DISABLED -> degrades to halfblocks",
			env: map[string]string{
				"TMUX":         "/tmp/tmux-1000/default,1,0",
				"TERM_PROGRAM": "WezTerm",
			},
			mockPassthrough:  false,
			expectedProto:    ProtocolHalfBlocks,
			expectedWez:      true,
			expectedITerm:    false,
			expectedTmux:     true,
			expectedScreen:   false,
			expectedPassFlag: false,
		},
		{
			name: "TMUX + iTerm2 (LC_TERMINAL) with passthrough -> iterm2",
			env: map[string]string{
				"TMUX":        "/tmp/tmux-1000/default,1,0",
				"LC_TERMINAL": "iTerm2",
			},
			mockPassthrough:  true,
			expectedProto:    ProtocolITerm2,
			expectedWez:      false,
			expectedITerm:    true,
			expectedTmux:     true,
			expectedScreen:   false,
			expectedPassFlag: true,
		},
		{
			name: "TMUX + generic terminal (e.g. foot/alacritty) with passthrough -> halfblocks",
			env: map[string]string{
				"TMUX":         "/tmp/tmux-1000/default,1,0",
				"TERM_PROGRAM": "alacritty",
			},
			mockPassthrough:  true,
			expectedProto:    ProtocolHalfBlocks,
			expectedWez:      false,
			expectedITerm:    false,
			expectedTmux:     true,
			expectedScreen:   false,
			expectedPassFlag: true,
		},
		{
			name: "STY (GNU Screen) + WezTerm -> forces halfblocks regardless of terminal",
			env: map[string]string{
				"STY":          "12345.pts-1.box",
				"TERM_PROGRAM": "WezTerm",
			},
			mockPassthrough:  true,
			expectedProto:    ProtocolHalfBlocks,
			expectedWez:      true,
			expectedITerm:    false,
			expectedTmux:     false,
			expectedScreen:   true,
			expectedPassFlag: false,
		},
		{
			name: "STY + LC_TERMINAL=iTerm2 -> forces halfblocks",
			env: map[string]string{
				"STY":         "12345.pts-1.box",
				"LC_TERMINAL": "iTerm2",
			},
			mockPassthrough:  true,
			expectedProto:    ProtocolHalfBlocks,
			expectedWez:      false,
			expectedITerm:    true,
			expectedTmux:     false,
			expectedScreen:   true,
			expectedPassFlag: false,
		},
		{
			name: "Both TMUX AND STY set simultaneously + WezTerm -> Screen wins, degrades to halfblocks",
			env: map[string]string{
				"TMUX":         "/tmp/tmux-1000/default,1,0",
				"STY":          "12345.pts-1.box",
				"TERM_PROGRAM": "WezTerm",
			},
			mockPassthrough:  true,
			expectedProto:    ProtocolHalfBlocks,
			expectedWez:      true,
			expectedITerm:    false,
			expectedTmux:     true,
			expectedScreen:   true,
			expectedPassFlag: true,
		},
		{
			name: "Both WezTerm and iTerm indicators set (WEZTERM_PANE + LC_TERMINAL) -> iterm2",
			env: map[string]string{
				"WEZTERM_PANE": "1",
				"LC_TERMINAL":  "iTerm2",
			},
			mockPassthrough:  false,
			expectedProto:    ProtocolKitty,
			expectedWez:      true,
			expectedITerm:    true,
			expectedTmux:     false,
			expectedScreen:   false,
			expectedPassFlag: false,
		},
		{
			name: "Empty strings for all env vars -> default halfblocks",
			env: map[string]string{
				"TMUX":             "",
				"STY":              "",
				"TERM_PROGRAM":     "",
				"WEZTERM_PANE":     "",
				"LC_TERMINAL":      "",
				"ITERM_SESSION_ID": "",
			},
			mockPassthrough:  false,
			expectedProto:    ProtocolHalfBlocks,
			expectedWez:      false,
			expectedITerm:    false,
			expectedTmux:     false,
			expectedScreen:   false,
			expectedPassFlag: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnvs(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			oldChecker := tmuxPassthroughChecker
			defer func() { tmuxPassthroughChecker = oldChecker }()
			tmuxPassthroughChecker = func() bool { return tc.mockPassthrough }

			cap := DetectTerminal()

			if cap.Protocol != tc.expectedProto {
				t.Errorf("Protocol: expected %v, got %v", tc.expectedProto, cap.Protocol)
			}
			if cap.IsWezTerm != tc.expectedWez {
				t.Errorf("IsWezTerm: expected %v, got %v", tc.expectedWez, cap.IsWezTerm)
			}
			if cap.IsITerm2 != tc.expectedITerm {
				t.Errorf("IsITerm2: expected %v, got %v", tc.expectedITerm, cap.IsITerm2)
			}
			if cap.IsTmux != tc.expectedTmux {
				t.Errorf("IsTmux: expected %v, got %v", tc.expectedTmux, cap.IsTmux)
			}
			if cap.IsScreen != tc.expectedScreen {
				t.Errorf("IsScreen: expected %v, got %v", tc.expectedScreen, cap.IsScreen)
			}
			if cap.TmuxPassthrough != tc.expectedPassFlag {
				t.Errorf("TmuxPassthrough: expected %v, got %v", tc.expectedPassFlag, cap.TmuxPassthrough)
			}
		})
	}
}

func TestAdversarial_ResolveProtocol_FuzzAndEdge(t *testing.T) {
	clearEnvs(t)
	// Base terminal without WezTerm/iTerm2
	cases := []struct {
		input    string
		expected Protocol
	}{
		{"", ProtocolHalfBlocks},
		{"   ", ProtocolHalfBlocks},
		{"auto", ProtocolHalfBlocks},
		{"AUTO", ProtocolHalfBlocks},
		{"  Auto  ", ProtocolHalfBlocks},
		{"iterm2", ProtocolITerm2},
		{"ITERM2", ProtocolITerm2},
		{"  Iterm2\t", ProtocolITerm2},
		{"halfblocks", ProtocolHalfBlocks},
		{"HALFBLOCKS", ProtocolHalfBlocks},
		{"disabled", ProtocolDisabled},
		{"DISABLED", ProtocolDisabled},
		{"none", ProtocolHalfBlocks},  // unknown string falls back to detected
		{"off", ProtocolHalfBlocks},   // unknown string falls back to detected
		{"false", ProtocolHalfBlocks}, // unknown string falls back to detected
		{"0", ProtocolHalfBlocks},     // unknown string falls back to detected
		{"../../../etc", ProtocolHalfBlocks},
		{"; DROP TABLE;", ProtocolHalfBlocks},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("pref_%q", tc.input), func(t *testing.T) {
			got := ResolveProtocol(tc.input)
			if got != tc.expected {
				t.Errorf("ResolveProtocol(%q) = %v, expected %v", tc.input, got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Tmux DCS Passthrough Escape Sequence Formatting & Roundtrip
// ---------------------------------------------------------------------------

func TestAdversarial_FormatOSC1337_BinaryPreservation(t *testing.T) {
	// Create payload containing ALL 256 possible byte values (0x00 to 0xFF)
	// repeated multiple times to ensure binary transparency across all byte boundaries.
	var allBytes []byte
	for rep := 0; rep < 5; rep++ {
		for b := 0; b < 256; b++ {
			allBytes = append(allBytes, byte(b))
		}
	}

	t.Run("Standard OSC 1337 binary preservation", func(t *testing.T) {
		seq := FormatOSC1337(allBytes, 66, 16, false)
		str := string(seq)

		if !strings.HasPrefix(str, "\x1b]1337;File=inline=1;width=66;height=16;preserveAspectRatio=1:") {
			t.Fatalf("missing standard OSC 1337 header")
		}
		if !strings.HasSuffix(str, "\x07") {
			t.Fatalf("missing standard OSC 1337 BEL terminator")
		}

		payload := str[len("\x1b]1337;File=inline=1;width=66;height=16;preserveAspectRatio=1:") : len(str)-1]
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			t.Fatalf("failed to decode base64: %v", err)
		}
		if !bytes.Equal(decoded, allBytes) {
			t.Fatalf("decoded bytes differ from original binary payload")
		}
	})

	t.Run("Tmux DCS passthrough format and unwrap simulation", func(t *testing.T) {
		seq := FormatOSC1337(allBytes, 50, 12, true)
		str := string(seq)

		// Verify DCS header: \x1bPtmux;\x1b\x1b]1337;...
		dcsPrefix := "\x1bPtmux;\x1b\x1b]1337;File=inline=1;width=50;height=12;preserveAspectRatio=1:"
		if !strings.HasPrefix(str, dcsPrefix) {
			t.Fatalf("missing tmux DCS prefix: got %q", str[:min(len(str), len(dcsPrefix)+10)])
		}

		// Verify DCS footer: \x07\x1b\
		dcsSuffix := "\x07\x1b\\"
		if !strings.HasSuffix(str, dcsSuffix) {
			t.Fatalf("missing tmux DCS suffix: got %q", str[max(0, len(str)-10):])
		}

		// Simulate tmux unwrapper:
		// Tmux strips "\x1bPtmux;\x1b" and trailing "\x1b\" and converts "\x1b\x1b" back to "\x1b"
		innerContent := str[len("\x1bPtmux;\x1b") : len(str)-len("\x1b\\")]
		unwrapped := strings.ReplaceAll(innerContent, "\x1b\x1b", "\x1b")

		expectedUnwrappedPrefix := "\x1b]1337;File=inline=1;width=50;height=12;preserveAspectRatio=1:"
		if !strings.HasPrefix(unwrapped, expectedUnwrappedPrefix) {
			t.Fatalf("unwrapped sequence does not match expected OSC 1337 format")
		}
		if !strings.HasSuffix(unwrapped, "\x07") {
			t.Fatalf("unwrapped sequence missing BEL terminator")
		}

		b64Payload := unwrapped[len(expectedUnwrappedPrefix) : len(unwrapped)-1]
		decoded, err := base64.StdEncoding.DecodeString(b64Payload)
		if err != nil {
			t.Fatalf("failed to decode base64 from unwrapped sequence: %v", err)
		}
		if !bytes.Equal(decoded, allBytes) {
			t.Fatalf("unwrapped payload corrupted! Length %d != %d", len(decoded), len(allBytes))
		}
	})

	t.Run("Empty data handling", func(t *testing.T) {
		seqStd := FormatOSC1337(nil, 10, 5, false)
		if !strings.Contains(string(seqStd), ":\x07") {
			t.Errorf("expected empty payload to end with ':\x07', got %q", string(seqStd))
		}

		seqTmux := FormatOSC1337([]byte{}, 10, 5, true)
		if !strings.Contains(string(seqTmux), ":\x07\x1b\\") {
			t.Errorf("expected empty payload in tmux to end with ':\x07\x1b\\', got %q", string(seqTmux))
		}
	})
}

// ---------------------------------------------------------------------------
// 3. Geometry Clamping Under Extreme Inputs
// ---------------------------------------------------------------------------

func TestAdversarial_CalculateClampedDimensions_Stress(t *testing.T) {
	stressCases := []struct {
		name       string
		imgW, imgH int
		availCols  int
		maxCols    int
		maxRows    int
		assertFunc func(t *testing.T, box BoundingBox)
	}{
		{
			name:      "1x10,000 ultra-tall vertical line",
			imgW:      1,
			imgH:      10000,
			availCols: 80,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols < 1 || box.Cols > 66 {
					t.Errorf("invalid cols: %d", box.Cols)
				}
				if box.Rows != 16 {
					t.Errorf("expected maxRows 16, got %d", box.Rows)
				}
			},
		},
		{
			name:      "10,000x1 ultra-wide horizontal line",
			imgW:      10000,
			imgH:      1,
			availCols: 80,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols != 66 {
					t.Errorf("expected maxCols 66, got %d", box.Cols)
				}
				// imgH is 1, which is < MinHeight (3), so minH is clamped to 1
				if box.Rows != 1 {
					t.Errorf("expected 1 row for 1px height image, got %d", box.Rows)
				}
			},
		},
		{
			name:      "10,000x10 panorama with imgH > MinHeight",
			imgW:      10000,
			imgH:      10,
			availCols: 80,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols != 66 {
					t.Errorf("expected cols 66, got %d", box.Cols)
				}
				if box.Rows < MinHeight {
					t.Errorf("expected at least MinHeight rows (%d), got %d", MinHeight, box.Rows)
				}
			},
		},
		{
			name:      "availableCols = 1 tight screen",
			imgW:      1920,
			imgH:      1080,
			availCols: 1,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols != 1 {
					t.Errorf("expected cols clamped to availableCols 1, got %d", box.Cols)
				}
				if box.Rows < 1 || box.Rows > 16 {
					t.Errorf("rows out of range: %d", box.Rows)
				}
			},
		},
		{
			name:      "maxRows = 1 very constrained height",
			imgW:      800,
			imgH:      600,
			availCols: 80,
			maxCols:   66,
			maxRows:   1,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Rows != 1 {
					t.Errorf("expected rows clamped to maxRows 1, got %d", box.Rows)
				}
				if box.Cols < 1 || box.Cols > 66 {
					t.Errorf("cols out of bounds: %d", box.Cols)
				}
			},
		},
		{
			name:      "negative dimensions",
			imgW:      -100,
			imgH:      -50,
			availCols: 80,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols != 0 || box.Rows != 0 {
					t.Errorf("expected 0x0 for negative inputs, got %dx%d", box.Cols, box.Rows)
				}
			},
		},
		{
			name:      "zero image dimensions",
			imgW:      0,
			imgH:      500,
			availCols: 80,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols != 0 || box.Rows != 0 {
					t.Errorf("expected 0x0 for zero width, got %dx%d", box.Cols, box.Rows)
				}
			},
		},
		{
			name:      "huge dimensions near math.MaxInt32",
			imgW:      math.MaxInt32,
			imgH:      math.MaxInt32,
			availCols: 80,
			maxCols:   66,
			maxRows:   16,
			assertFunc: func(t *testing.T, box BoundingBox) {
				if box.Cols <= 0 || box.Cols > 66 {
					t.Errorf("cols out of bounds: %d", box.Cols)
				}
				if box.Rows <= 0 || box.Rows > 16 {
					t.Errorf("rows out of bounds: %d", box.Rows)
				}
			},
		},
	}

	for _, tc := range stressCases {
		t.Run(tc.name, func(t *testing.T) {
			box := CalculateClampedDimensions(tc.imgW, tc.imgH, tc.availCols, tc.maxCols, tc.maxRows)
			tc.assertFunc(t, box)
		})
	}
}

// ---------------------------------------------------------------------------
// 4. Image Safety: Non-square & Multi-format Bomb Stress
// ---------------------------------------------------------------------------

func TestAdversarial_ValidateImageSafety_Stress(t *testing.T) {
	t.Run("Asymmetric 100,000x200 decompression bomb", func(t *testing.T) {
		// 100,000 x 200 = 20,000,000 pixels (> 16MP)
		header := createCraftedPNGHeader(100000, 200)
		_, err := ValidateImageSafety(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected 20MP asymmetric bomb to be rejected")
		}
		if !errors.Is(err, ErrImageTooLarge) {
			t.Errorf("expected ErrImageTooLarge, got %v", err)
		}
	})

	t.Run("Boundary test: exactly 16,000,000 x 1 pixels", func(t *testing.T) {
		header := createCraftedPNGHeader(16000000, 1)
		cfg, err := ValidateImageSafety(bytes.NewReader(header))
		if err != nil {
			t.Fatalf("expected 16,000,000x1 to be allowed at boundary, got: %v", err)
		}
		if cfg.Width != 16000000 || cfg.Height != 1 {
			t.Errorf("expected 16000000x1, got %dx%d", cfg.Width, cfg.Height)
		}
	})

	t.Run("Boundary test: 16,000,001 x 1 pixels (one over boundary)", func(t *testing.T) {
		header := createCraftedPNGHeader(16000001, 1)
		_, err := ValidateImageSafety(bytes.NewReader(header))
		if err == nil {
			t.Fatalf("expected 16,000,001x1 to be rejected")
		}
		if !errors.Is(err, ErrImageTooLarge) {
			t.Errorf("expected ErrImageTooLarge, got %v", err)
		}
	})

	t.Run("Endless stream of zeros", func(t *testing.T) {
		zeroReader := bytes.NewReader(make([]byte, 1024))
		_, err := ValidateImageSafety(zeroReader)
		if err == nil {
			t.Fatalf("expected zero stream to fail image validation")
		}
		if !errors.Is(err, ErrInvalidImage) {
			t.Errorf("expected ErrInvalidImage, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. Cell Masking: Deep Nested ProxyScreens and Stress
// ---------------------------------------------------------------------------

func TestAdversarial_CellMask_DeepNesting(t *testing.T) {
	mockScreen := newMockLockScreen()

	// Wrap in 10 nested proxy screens
	var current mauview.Screen = mockScreen
	expectedX, expectedY := 0, 0
	for i := 1; i <= 10; i++ {
		current = mauview.NewProxyScreen(current, i, i*2, 100, 100)
		expectedX += i
		expectedY += i * 2
	}

	root, offX, offY := GetRootScreenAndOffset(current)
	if root != mockScreen {
		t.Fatalf("expected root to be mockScreen after 10 levels of nesting")
	}
	if offX != expectedX || offY != expectedY {
		t.Fatalf("expected offset (%d, %d), got (%d, %d)", expectedX, expectedY, offX, offY)
	}

	// Create ImageMask at (5, 5) relative to innermost screen
	mask := NewImageMask(current, 5, 5, 20, 10)
	if mask.ScreenX != expectedX+5 || mask.ScreenY != expectedY+5 {
		t.Errorf("expected Screen coordinates (%d, %d), got (%d, %d)", expectedX+5, expectedY+5, mask.ScreenX, mask.ScreenY)
	}

	mask.Apply()
	if !mask.Active {
		t.Errorf("mask should be active")
	}
	if mockScreen.lockCallCount != 0 {
		t.Errorf("expected 0 lock calls to prevent scrolling freeze, got %d", mockScreen.lockCallCount)
	}

	mask.Clear()
	if mask.Active {
		t.Errorf("mask should be inactive")
	}
	if mockScreen.lockCallCount != 0 {
		t.Errorf("expected 0 lock calls, got %d", mockScreen.lockCallCount)
	}
}

func TestAdversarial_PlaceholderBuffer_Bounds(t *testing.T) {
	// Zero and negative cases
	if buf := CreatePlaceholderBuffer(-1, 10); buf != nil {
		t.Errorf("expected nil for negative cols")
	}
	if buf := CreatePlaceholderBuffer(10, -1); buf != nil {
		t.Errorf("expected nil for negative rows")
	}
	if buf := CreatePlaceholderBuffer(0, 0); buf != nil {
		t.Errorf("expected nil for 0x0")
	}

	// Very large placeholder buffer
	buf := CreatePlaceholderBuffer(200, 100)
	if len(buf) != 100 {
		t.Fatalf("expected 100 rows, got %d", len(buf))
	}
	if len(buf[0]) != 200 {
		t.Fatalf("expected 200 cols, got %d", len(buf[0]))
	}
	for _, row := range buf {
		for _, cell := range row {
			if cell.Char != ' ' || cell.Style != tcell.StyleDefault {
				t.Fatalf("cell not default space")
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
