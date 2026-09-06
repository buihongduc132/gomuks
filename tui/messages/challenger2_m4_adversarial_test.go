// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Empirical Challenger 2 Adversarial Test Suite for Milestone M4:
// Final Integration, Verification & Audit.
// Stress-testing graphics protocol fallbacks, extreme geometries (1x1000, 1000x1, 0x0),
// decompression bombs (>16MP rejection), tmux DCS escaping, corrupt data fallbacks,
// and zero unhandled panics under the Go Race Detector.

package messages

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/lib/termimg"
)

// Helper to create solid color PNG
func makeSolidPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: 120, G: 180, B: 240, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// 1. Extreme Aspect Ratios (1x1000, 1000x1, 0x0) & Degenerate Geometries
// ---------------------------------------------------------------------------

func TestChallenger2_M4_ExtremeAspectRatios_GeometryAndRendering(t *testing.T) {
	// A. Geometry Clamping Oracle Verification
	t.Run("GeometryClamping_ExhaustiveOracle", func(t *testing.T) {
		type geomCase struct {
			name        string
			imgW, imgH  int
			availCols   int
			maxCols     int
			maxRows     int
			checkOracle func(t *testing.T, box termimg.BoundingBox)
		}

		cases := []geomCase{
			{
				name:      "1x1000 ultra-tall needle",
				imgW:      1,
				imgH:      1000,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols < 1 || box.Cols > 66 {
						t.Errorf("expected cols in [1, 66], got %d", box.Cols)
					}
					if box.Rows != 16 {
						t.Errorf("expected rows clamped to maxRows 16, got %d", box.Rows)
					}
				},
			},
			{
				name:      "1000x1 ultra-wide horizontal line",
				imgW:      1000,
				imgH:      1,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols != 66 {
						t.Errorf("expected cols clamped to maxCols 66, got %d", box.Cols)
					}
					if box.Rows != 1 {
						t.Errorf("expected rows 1 for 1px native height, got %d", box.Rows)
					}
				},
			},
			{
				name:      "0x0 zero dimensions",
				imgW:      0,
				imgH:      0,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols != 0 || box.Rows != 0 {
						t.Errorf("expected (0, 0) for 0x0, got (%d, %d)", box.Cols, box.Rows)
					}
				},
			},
			{
				name:      "0x1000 zero width",
				imgW:      0,
				imgH:      1000,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols != 0 || box.Rows != 0 {
						t.Errorf("expected (0, 0) for zero width, got (%d, %d)", box.Cols, box.Rows)
					}
				},
			},
			{
				name:      "1000x0 zero height",
				imgW:      1000,
				imgH:      0,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols != 0 || box.Rows != 0 {
						t.Errorf("expected (0, 0) for zero height, got (%d, %d)", box.Cols, box.Rows)
					}
				},
			},
			{
				name:      "negative dimensions (-100, -50)",
				imgW:      -100,
				imgH:      -50,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols != 0 || box.Rows != 0 {
						t.Errorf("expected (0, 0) for negative dimensions, got (%d, %d)", box.Cols, box.Rows)
					}
				},
			},
			{
				name:      "int32 max overflow dimensions",
				imgW:      math.MaxInt32,
				imgH:      math.MaxInt32,
				availCols: 80,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols <= 0 || box.Cols > 66 {
						t.Errorf("cols %d outside [1, 66]", box.Cols)
					}
					if box.Rows <= 0 || box.Rows > 16 {
						t.Errorf("rows %d outside [1, 16]", box.Rows)
					}
				},
			},
			{
				name:      "availableCols = 1 narrow terminal",
				imgW:      800,
				imgH:      600,
				availCols: 1,
				maxCols:   66,
				maxRows:   16,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Cols != 1 {
						t.Errorf("expected cols clamped to 1, got %d", box.Cols)
					}
					if box.Rows < 1 || box.Rows > 16 {
						t.Errorf("rows %d outside [1, 16]", box.Rows)
					}
				},
			},
			{
				name:      "maxRows = 1 very constrained row boundary",
				imgW:      800,
				imgH:      600,
				availCols: 80,
				maxCols:   66,
				maxRows:   1,
				checkOracle: func(t *testing.T, box termimg.BoundingBox) {
					if box.Rows != 1 {
						t.Errorf("expected rows clamped to 1, got %d", box.Rows)
					}
					if box.Cols < 1 || box.Cols > 66 {
						t.Errorf("cols %d outside [1, 66]", box.Cols)
					}
				},
			},
		}

		for _, tc := range cases {
			box := termimg.CalculateClampedDimensions(tc.imgW, tc.imgH, tc.availCols, tc.maxCols, tc.maxRows)
			tc.checkOracle(t, box)
		}
	})

	// B. End-to-End Rendering on UIMessage & FileMessage
	t.Run("EndToEndRendering_ExtremeAspectRatios", func(t *testing.T) {
		renderCases := []struct {
			w, h int
			desc string
		}{
			{1, 1000, "1x1000 ultra-tall needle"},
			{1000, 1, "1000x1 ultra-wide panoramic line"},
			{1, 1, "1x1 single pixel"},
			{1, 2, "1x2 vertical 2-pixel"},
			{2, 1, "2x1 horizontal 2-pixel"},
			{10, 500, "10x500 tall image"},
			{500, 10, "500x10 wide image"},
			{66, 16, "66x16 exact bounding box limits"},
		}

		protocols := []string{"iterm2", "halfblocks"}

		for _, proto := range protocols {
			for _, rc := range renderCases {
				t.Run(fmt.Sprintf("%s_%s", proto, rc.desc), func(t *testing.T) {
					pngData := makeSolidPNG(rc.w, rc.h)

					content := &event.MessageEventContent{
						MsgType: event.MsgImage,
						Body:    rc.desc + ".png",
						URL:     "mxc://matrix.example.com/ar_test",
					}
					uiMsg, fileMsg := createTestUIMessage(content)
					fileMsg.SetImageData(pngData)

					prefs := config.UserPreferences{
						ImagePreviewProtocol:  proto,
						ImagePreviewMaxWidth:  66,
						ImagePreviewMaxHeight: 16,
					}

					widths := []int{10, 30, 66, 80, 120}
					for _, width := range widths {
						uiMsg.InvalidateBuffer()
						uiMsg.CalculateBuffer(prefs, width)

						h := fileMsg.Height()
						if h > 16 {
							t.Fatalf("[%s, w=%d] height %d exceeded max 16 rows", rc.desc, width, h)
						}
						if h < 1 {
							t.Fatalf("[%s, w=%d] height %d is non-positive", rc.desc, width, h)
						}

						// Verify line bounds
						fileMsg.mu.RLock()
						for rowIdx, line := range fileMsg.buffer {
							if len(line) > 66 {
								t.Fatalf("[%s, w=%d, row=%d] line length %d exceeded maxCols 66",
									rc.desc, width, rowIdx, len(line))
							}
						}
						fileMsg.mu.RUnlock()

						// Draw to simulation screen: assert zero panics
						simScreen := newSimulationScreen(100, 30)
						proxy := mauview.NewProxyScreen(simScreen, 0, 0, width, h)
						fileMsg.Draw(proxy, uiMsg)
					}
				})
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Decompression Bombs (>16MP Rejection) & Memory Safety
// ---------------------------------------------------------------------------

func TestChallenger2_M4_DecompressionBombs_SafetyAndMemory(t *testing.T) {
	bombCases := []struct {
		name        string
		w, h        uint32
		expectError error
		isBomb      bool
	}{
		{
			name:        "16,000,000 x 1 pixels (exact boundary allowed)",
			w:           16000000,
			h:           1,
			expectError: nil,
			isBomb:      false,
		},
		{
			name:        "16,000,001 x 1 pixels (boundary + 1 rejected)",
			w:           16000001,
			h:           1,
			expectError: termimg.ErrImageTooLarge,
			isBomb:      true,
		},
		{
			name:        "4001 x 4000 pixels (16.004MP boundary rejected)",
			w:           4001,
			h:           4000,
			expectError: termimg.ErrImageTooLarge,
			isBomb:      true,
		},
		{
			name:        "100MP bomb (10000 x 10000)",
			w:           10000,
			h:           10000,
			expectError: termimg.ErrImageTooLarge,
			isBomb:      true,
		},
		{
			name:        "400MP bomb (20000 x 20000)",
			w:           20000,
			h:           20000,
			expectError: termimg.ErrImageTooLarge,
			isBomb:      true,
		},
		{
			name:        "2.5GP bomb (50000 x 50000)",
			w:           50000,
			h:           50000,
			expectError: termimg.ErrImageTooLarge,
			isBomb:      true,
		},
		{
			name:        "Integer overflow bomb (1000000 x 1000000)",
			w:           1000000,
			h:           1000000,
			expectError: termimg.ErrImageTooLarge,
			isBomb:      true,
		},
		{
			name:        "Int32 max boundary bomb",
			w:           1<<31 - 1,
			h:           1<<31 - 1,
			expectError: termimg.ErrInvalidImage,
			isBomb:      true,
		},
		{
			name:        "0x0 zero dimensions",
			w:           0,
			h:           0,
			expectError: termimg.ErrInvalidImage,
			isBomb:      false,
		},
	}

	for _, bc := range bombCases {
		t.Run(bc.name, func(t *testing.T) {
			headerData := createCraftedPNGHeaderBytes(bc.w, bc.h)

			// Step 1: Test ValidateImageSafety
			cfg, err := termimg.ValidateImageSafety(bytes.NewReader(headerData))
			if bc.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", bc.expectError)
				}
				if !errors.Is(err, bc.expectError) {
					t.Fatalf("expected error %v, got %v", bc.expectError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.Width != int(bc.w) || cfg.Height != int(bc.h) {
					t.Errorf("expected dimensions %dx%d, got %dx%d", bc.w, bc.h, cfg.Width, cfg.Height)
				}
			}

			// Step 2: Test FileMessage.SetImageData safety and memory spike check
			runtime.GC()
			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)

			content := &event.MessageEventContent{
				MsgType: event.MsgImage,
				Body:    bc.name,
				URL:     "mxc://matrix.example.com/bomb",
			}
			uiMsg, fileMsg := createTestUIMessage(content)
			fileMsg.SetImageData(headerData)

			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			heapDelta := int64(memAfter.Alloc) - int64(memBefore.Alloc)
			if heapDelta > 2*1024*1024 {
				t.Fatalf("memory spike detected: %d bytes allocated during bomb validation", heapDelta)
			}

			if bc.isBomb {
				if fileMsg.ImageData() != nil {
					t.Errorf("expected imageData to be nil for bomb payload")
				}
				if fileMsg.ImageError() == nil {
					t.Errorf("expected imageErr to be recorded for bomb payload")
				}
			}

			// Step 3: Test CalculateBuffer and Draw with fallback to red error link
			prefs := config.UserPreferences{
				ImagePreviewProtocol:  "halfblocks",
				ImagePreviewMaxWidth:  66,
				ImagePreviewMaxHeight: 16,
			}
			uiMsg.CalculateBuffer(prefs, 80)

			// Height must be clamped
			if fileMsg.Height() > 16 {
				t.Fatalf("height exploded to %d", fileMsg.Height())
			}
			if fileMsg.Height() < 1 {
				t.Fatalf("height is %d, expected fallback link", fileMsg.Height())
			}

			// If bomb or error, buffer must contain red error text
			if bc.expectError != nil {
				hasRedCell := false
				fileMsg.mu.RLock()
				for _, line := range fileMsg.buffer {
					for _, cell := range line {
						fg, _, _ := cell.Style.Decompose()
						if fg == tcell.ColorRed {
							hasRedCell = true
							break
						}
					}
					if hasRedCell {
						break
					}
				}
				fileMsg.mu.RUnlock()

				if !hasRedCell {
					t.Errorf("expected red error text in buffer for %s", bc.name)
				}
			}

			// Assert Draw() does not panic
			simScreen := newSimulationScreen(100, 30)
			proxy := mauview.NewProxyScreen(simScreen, 0, 0, 80, fileMsg.Height())
			fileMsg.Draw(proxy, uiMsg)
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Tmux DCS Passthrough Escaping & Roundtrip
// ---------------------------------------------------------------------------

func TestChallenger2_M4_TmuxDCS_EscapingAndRoundtrip(t *testing.T) {
	// Generate payload containing all byte values 0x00 to 0xFF multiple times
	var fullPayload []byte
	for rep := 0; rep < 4; rep++ {
		for b := 0; b < 256; b++ {
			fullPayload = append(fullPayload, byte(b))
		}
	}

	t.Run("TmuxFormatAndDoubledEscapeIntegrity", func(t *testing.T) {
		seq := termimg.FormatOSC1337(fullPayload, 60, 15, true)
		str := string(seq)

		// Must begin with \x1bPtmux;\x1b\x1b]1337;
		expectedPrefix := "\x1bPtmux;\x1b\x1b]1337;File=inline=1;width=60;height=15;preserveAspectRatio=1:"
		if !strings.HasPrefix(str, expectedPrefix) {
			t.Fatalf("missing tmux DCS prefix: got %q", str[:minInt(len(str), len(expectedPrefix)+10)])
		}

		// Must end with \x07\x1b\
		expectedSuffix := "\x07\x1b\\"
		if !strings.HasSuffix(str, expectedSuffix) {
			t.Fatalf("missing tmux DCS suffix: got %q", str[maxInt(0, len(str)-10):])
		}

		// Unpack DCS simulation:
		// Tmux unwraps "\x1bPtmux;\x1b" ... "\x1b\" and replaces "\x1b\x1b" with "\x1b"
		inner := str[len("\x1bPtmux;\x1b") : len(str)-len("\x1b\\")]
		unwrapped := strings.ReplaceAll(inner, "\x1b\x1b", "\x1b")

		oscHeader := "\x1b]1337;File=inline=1;width=60;height=15;preserveAspectRatio=1:"
		if !strings.HasPrefix(unwrapped, oscHeader) {
			t.Fatalf("unwrapped sequence does not match OSC 1337 header")
		}
		if !strings.HasSuffix(unwrapped, "\x07") {
			t.Fatalf("unwrapped sequence missing BEL terminator")
		}

		b64Part := unwrapped[len(oscHeader) : len(unwrapped)-1]
		decoded, err := base64.StdEncoding.DecodeString(b64Part)
		if err != nil {
			t.Fatalf("failed to decode base64 from unwrapped sequence: %v", err)
		}
		if !bytes.Equal(decoded, fullPayload) {
			t.Fatalf("roundtrip corruption: decoded length %d != original length %d", len(decoded), len(fullPayload))
		}
	})

	t.Run("StandardFormatWithoutTmux", func(t *testing.T) {
		seq := termimg.FormatOSC1337(fullPayload, 50, 12, false)
		str := string(seq)

		stdHeader := "\x1b]1337;File=inline=1;width=50;height=12;preserveAspectRatio=1:"
		if !strings.HasPrefix(str, stdHeader) {
			t.Fatalf("missing standard OSC header")
		}
		if !strings.HasSuffix(str, "\x07") {
			t.Fatalf("missing standard BEL terminator")
		}
		if strings.Contains(str, "Ptmux;") {
			t.Fatalf("standard format must not contain Ptmux")
		}
	})

	t.Run("EndToEnd_EmitOSC1337_WriterCapture", func(t *testing.T) {
		var buf bytes.Buffer
		SetOSCOutputWriter(&buf)
		defer SetOSCOutputWriter(nil)

		pngData := makeSolidPNG(50, 50)
		content := &event.MessageEventContent{
			MsgType: event.MsgImage,
			Body:    "tmux_test.png",
			URL:     "mxc://matrix.example.com/tmux_test",
		}
		uiMsg, fileMsg := createTestUIMessage(content)
		fileMsg.SetImageData(pngData)

		prefs := config.UserPreferences{
			ImagePreviewProtocol:  "iterm2",
			ImagePreviewMaxWidth:  66,
			ImagePreviewMaxHeight: 16,
		}

		uiMsg.CalculateBuffer(prefs, 80)

		simScreen := newSimulationScreen(100, 30)
		proxy := mauview.NewProxyScreen(simScreen, 2, 2, 80, fileMsg.Height())

		fileMsg.Draw(proxy, uiMsg)

		captured := buf.Bytes()
		if len(captured) == 0 {
			t.Fatalf("expected emitted OSC sequence in output writer")
		}
		if !bytes.Contains(captured, []byte("\x1b]1337;File=inline=1;")) {
			t.Fatalf("output writer did not capture valid OSC 1337 sequence")
		}
	})
}

// ---------------------------------------------------------------------------
// 4. Fallback to Red Error Link on Corrupt Data
// ---------------------------------------------------------------------------

func TestChallenger2_M4_CorruptData_RedErrorFallback(t *testing.T) {
	corruptPayloads := []struct {
		name string
		data []byte
	}{
		{"empty byte slice", []byte{}},
		{"single byte 0x89", []byte{0x89}},
		{"3-byte slice", []byte{0x89, 0x50, 0x4e}},
		{"incomplete 8-byte PNG header", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}},
		{"PNG header + corrupted IDAT chunk", append(createCraftedPNGHeaderBytes(50, 50), []byte("GARBAGE_CHUNK_CORRUPT")...)},
		{"arbitrary non-image text string", []byte("NOT_AN_IMAGE_FILE_PAYLOAD_1234567890")},
		{"corrupt JPEG marker (FF D8 FF E0 + garbage)", []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x00}},
		{"64KB random noise bytes", makeRandomBytes(64 * 1024)},
	}

	protocols := []string{"iterm2", "halfblocks"}

	for _, proto := range protocols {
		for _, cp := range corruptPayloads {
			t.Run(fmt.Sprintf("%s_%s", proto, cp.name), func(t *testing.T) {
				content := &event.MessageEventContent{
					MsgType: event.MsgImage,
					Body:    cp.name,
					URL:     "mxc://matrix.example.com/corrupt",
				}
				uiMsg, fileMsg := createTestUIMessage(content)

				// Directly set imageData
				fileMsg.mu.Lock()
				fileMsg.imageData = cp.data
				fileMsg.mu.Unlock()

				prefs := config.UserPreferences{
					ImagePreviewProtocol:  proto,
					ImagePreviewMaxWidth:  66,
					ImagePreviewMaxHeight: 16,
				}

				// CalculateBuffer must not panic
				uiMsg.CalculateBuffer(prefs, 80)

				h := fileMsg.Height()
				if h < 1 {
					t.Fatalf("height %d is non-positive, expected error link buffer", h)
				}
				if h > 16 {
					t.Fatalf("height %d exceeded max bounds", h)
				}

				// If data was non-empty, assert red error annotation is present in buffer
				// (Under iterm2, if header is valid PNG, OSC 1337 placeholder is built instead)
				if len(cp.data) > 0 {
					if proto == "halfblocks" || cp.name != "PNG header + corrupted IDAT chunk" {
						hasRedCell := false
						fileMsg.mu.RLock()
						for _, line := range fileMsg.buffer {
							for _, cell := range line {
								fg, _, _ := cell.Style.Decompose()
								if fg == tcell.ColorRed {
									hasRedCell = true
									break
								}
							}
							if hasRedCell {
								break
							}
						}
						fileMsg.mu.RUnlock()

						if !hasRedCell {
							t.Errorf("expected RED error cell in fallback buffer for %s under %s", cp.name, proto)
						}
					}
				}

				// Draw must not panic
				simScreen := newSimulationScreen(100, 30)
				proxy := mauview.NewProxyScreen(simScreen, 0, 0, 80, h)
				fileMsg.Draw(proxy, uiMsg)
			})
		}
	}
}

// ---------------------------------------------------------------------------
// 5. Zero Unhandled Panics Multi-Goroutine Adversarial Stress Harness
// ---------------------------------------------------------------------------

func TestChallenger2_M4_AdversarialStress_ZeroUnhandledPanics(t *testing.T) {
	pngData := makeSolidPNG(80, 60)
	bombData := createCraftedPNGHeaderBytes(25000, 25000)
	corruptData := []byte("HOSTILE_CORRUPT_PAYLOAD_0987654321")

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "stress.png",
		URL:     "mxc://matrix.example.com/stress",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngData)

	rawSim := tcell.NewSimulationScreen("")
	_ = rawSim.Init()
	rawSim.SetSize(120, 50)
	var simScreen mauview.Screen = rawSim
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Worker 1: Rapid ImageData injection (valid, bomb, corrupt, empty)
	wg.Add(1)
	go func() {
		defer wg.Done()
		payloads := [][]byte{pngData, bombData, corruptData, nil, {}}
		idx := 0
		for {
			select {
			case <-stop:
				return
			default:
				fileMsg.SetImageData(payloads[idx%len(payloads)])
				idx++
				time.Sleep(200 * time.Microsecond)
			}
		}
	}()

	// Worker 2: Rapid CalculateBuffer with varying widths
	wg.Add(1)
	go func() {
		defer wg.Done()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		widths := []int{1, 5, 15, 30, 50, 66, 80, 100, 150}
		for {
			select {
			case <-stop:
				return
			default:
				w := widths[rnd.Intn(len(widths))]
				prefs := config.UserPreferences{
					ImagePreviewProtocol:  "halfblocks",
					ImagePreviewMaxWidth:  66,
					ImagePreviewMaxHeight: 16,
				}
				uiMsg.CalculateBuffer(prefs, w)
				time.Sleep(300 * time.Microsecond)
			}
		}
	}()

	// Worker 3: Rapid CalculateBuffer alternating protocols
	wg.Add(1)
	go func() {
		defer wg.Done()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano() + 1))
		protos := []string{"iterm2", "halfblocks", "disabled", "auto", "invalid_proto"}
		for {
			select {
			case <-stop:
				return
			default:
				p := protos[rnd.Intn(len(protos))]
				prefs := config.UserPreferences{
					ImagePreviewProtocol:  p,
					ImagePreviewMaxWidth:  66,
					ImagePreviewMaxHeight: 16,
				}
				uiMsg.CalculateBuffer(prefs, 80)
				time.Sleep(300 * time.Microsecond)
			}
		}
	}()

	// Worker 4: Rapid Draw() with oscillating screen coordinates
	wg.Add(1)
	go func() {
		defer wg.Done()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano() + 2))
		for {
			select {
			case <-stop:
				return
			default:
				x := rnd.Intn(40) - 10
				y := rnd.Intn(40) - 10
				h := maxInt(1, fileMsg.Height())
				proxy := mauview.NewProxyScreen(simScreen, x, y, 70, h)
				fileMsg.Draw(proxy, uiMsg)
				time.Sleep(200 * time.Microsecond)
			}
		}
	}()

	// Worker 5: Rapid InvalidateBuffer and ClearMask
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				uiMsg.InvalidateBuffer()
				fileMsg.ClearMask()
				time.Sleep(500 * time.Microsecond)
			}
		}
	}()

	// Worker 6: Rapid Cloning
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = uiMsg.Clone()
				time.Sleep(500 * time.Microsecond)
			}
		}
	}()

	// Worker 7: Rapid Cache and State queries
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = fileMsg.Height()
				_ = fileMsg.RenderCache()
				_ = fileMsg.CachedOSC()
				_ = fileMsg.CachedProto()
				_ = fileMsg.CurrentMask()
				_ = fileMsg.ImageData()
				time.Sleep(200 * time.Microsecond)
			}
		}
	}()

	// Worker 8: Rapid Geometry Clamping Stress
	wg.Add(1)
	go func() {
		defer wg.Done()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano() + 3))
		for {
			select {
			case <-stop:
				return
			default:
				w := rnd.Intn(10000) - 100
				h := rnd.Intn(10000) - 100
				avail := rnd.Intn(200) - 20
				_ = termimg.CalculateClampedDimensions(w, h, avail, 66, 16)
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()

	// Worker 9: Rapid FormatOSC1337 & PreDownscale Stress
	wg.Add(1)
	go func() {
		defer wg.Done()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano() + 4))
		for {
			select {
			case <-stop:
				return
			default:
				isTmux := rnd.Intn(2) == 0
				cols := rnd.Intn(80) + 1
				rows := rnd.Intn(20) + 1
				_ = termimg.FormatOSC1337(pngData, cols, rows, isTmux)
				_, _ = termimg.PreDownscaleForPTY(pngData, 600, 400)
				time.Sleep(200 * time.Microsecond)
			}
		}
	}()

	// Worker 10: Screen resize and sync
	wg.Add(1)
	go func() {
		defer wg.Done()
		rnd := rand.New(rand.NewSource(time.Now().UnixNano() + 5))
		for {
			select {
			case <-stop:
				return
			default:
				sw := rnd.Intn(100) + 20
				sh := rnd.Intn(40) + 10
				rawSim.SetSize(sw, sh)
				rawSim.Sync()
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Run stress harness for 1.2 seconds under -race
	time.Sleep(1200 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func makeRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
