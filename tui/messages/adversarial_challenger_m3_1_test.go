// gomuks - A terminal Matrix client written in Go.
// Adversarial Challenge Test Suite for Milestone M3: Safe Rendering Engine & TUI Cell-Masking
// Challenger 1: Empirical verification of geometry clamping, image safety, and render cache hits.

package messages

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/lib/termimg"
	"go.mau.fi/gomuks/tui/messages/tstring"
)

// Helper to create an image with distinct dimensions
func createSolidColorPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: 200, G: 50, B: 50, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// 1. Stress test extreme aspect ratio images and assert height NEVER exceeds 16 rows and width NEVER exceeds 66 columns.
func TestChallenger1_M3_ExtremeAspectRatio_GeometryClamping_ExhaustiveMatrix(t *testing.T) {
	aspectRatios := []struct {
		w, h int
		desc string
	}{
		{1, 1000, "1x1000 ultra-tall needle"},
		{1000, 1, "1000x1 ultra-wide panoramic line"},
		{10000, 10000, "10000x10000 massive square (100MP)"},
		{0, 0, "0x0 zero dimensions"},
		{1, 1, "1x1 single pixel"},
		{1, 2, "1x2 vertical couple"},
		{2, 1, "2x1 horizontal couple"},
		{1, 16, "1x16 needle matching max rows"},
		{16, 1, "16x1 strip"},
		{1, 32, "1x32 needle"},
		{32, 1, "32x1 strip"},
		{50000, 1, "50000x1 extreme horizontal line"},
		{1, 50000, "1x50000 extreme vertical line"},
		{4000, 4000, "4000x4000 16MP limit square"},
		{3999, 4000, "3999x4000 near 16MP limit"},
		{1, 1000000, "1x1000000 million-row needle"},
		{1000000, 1, "1000000x1 million-col line"},
		{0, 100, "0x100 zero width"},
		{100, 0, "100x0 zero height"},
		{-10, 20, "-10x20 negative width"},
		{20, -10, "20x-10 negative height"},
		{-1, -1, "-1x-1 negative dimensions"},
	}

	availableColsList := []int{-10, 0, 1, 2, 3, 5, 10, 20, 40, 66, 80, 100, 120, 200, 500, 10000}

	configs := []struct {
		maxCols int
		maxRows int
		desc    string
	}{
		{0, 0, "default limits (0 -> 66, 16)"},
		{66, 16, "standard iamb parity (66, 16)"},
		{30, 8, "restricted bounds (30, 8)"},
		{1, 1, "minimum bounds (1, 1)"},
		{100, 20, "expanded bounds (100, 20)"},
	}

	for _, cfg := range configs {
		effectiveMaxCols := cfg.maxCols
		if effectiveMaxCols <= 0 {
			effectiveMaxCols = termimg.DefaultMaxWidth // 66
		}
		effectiveMaxRows := cfg.maxRows
		if effectiveMaxRows <= 0 {
			effectiveMaxRows = termimg.DefaultMaxHeight // 16
		}

		for _, ar := range aspectRatios {
			for _, avail := range availableColsList {
				bbox := termimg.CalculateClampedDimensions(ar.w, ar.h, avail, cfg.maxCols, cfg.maxRows)

				// Oracle assertions
				if ar.w <= 0 || ar.h <= 0 {
					if bbox.Cols != 0 || bbox.Rows != 0 {
						t.Errorf("[%s] non-positive input (%d,%d) must return (0,0), got (%d,%d)", ar.desc, ar.w, ar.h, bbox.Cols, bbox.Rows)
					}
					continue
				}

				if bbox.Rows > effectiveMaxRows {
					t.Fatalf("[%s, %s, avail=%d] Rows %d exceeded max allowed %d",
						cfg.desc, ar.desc, avail, bbox.Rows, effectiveMaxRows)
				}

				limitCols := avail
				if cfg.maxCols > 0 && cfg.maxCols < limitCols {
					limitCols = cfg.maxCols
				}
				if limitCols < 1 {
					limitCols = 1
				}
				if limitCols > effectiveMaxCols {
					limitCols = effectiveMaxCols
				}

				if bbox.Cols > effectiveMaxCols {
					t.Fatalf("[%s, %s, avail=%d] Cols %d exceeded max allowed %d",
						cfg.desc, ar.desc, avail, bbox.Cols, effectiveMaxCols)
				}

				if bbox.Cols < 1 || bbox.Rows < 1 {
					t.Fatalf("[%s, %s, avail=%d] Cols %d or Rows %d is non-positive",
						cfg.desc, ar.desc, avail, bbox.Cols, bbox.Rows)
				}
			}
		}
	}
}

// 1b. End-to-end rendering clamping in UIMessage.CalculateBuffer and FileMessage.Draw
func TestChallenger1_M3_ExtremeAspectRatio_EndToEndRendering(t *testing.T) {
	testCases := []struct {
		w, h int
		desc string
	}{
		{1, 1000, "1x1000 ultra-tall needle"},
		{1000, 1, "1000x1 ultra-wide panoramic line"},
		{1, 1, "1x1 single pixel"},
		{1, 2, "1x2 vertical couple"},
		{2, 1, "2x1 horizontal couple"},
		{66, 16, "66x16 exact boundary"},
		{100, 100, "100x100 standard image"},
	}

	protocols := []string{"iterm2", "halfblocks"}

	for _, proto := range protocols {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s_%s", proto, tc.desc), func(t *testing.T) {
				pngBytes := createSolidColorPNG(tc.w, tc.h)

				content := &event.MessageEventContent{
					MsgType: event.MsgImage,
					Body:    tc.desc + ".png",
					URL:     "mxc://matrix.example.com/test",
				}
				uiMsg, fileMsg := createTestUIMessage(content)
				fileMsg.SetImageData(pngBytes)

				prefs := config.UserPreferences{
					ImagePreviewProtocol:  proto,
					ImagePreviewMaxWidth:  66,
					ImagePreviewMaxHeight: 16,
				}

				widths := []int{10, 30, 50, 66, 80, 120}
				for _, w := range widths {
					uiMsg.InvalidateBuffer()
					uiMsg.CalculateBuffer(prefs, w)

					h := fileMsg.Height()
					if h > 16 {
						t.Fatalf("[%s, w=%d] height %d exceeded maximum 16 rows", tc.desc, w, h)
					}
					if h < 1 {
						t.Fatalf("[%s, w=%d] height %d is less than 1 row", tc.desc, w, h)
					}

					// Verify line widths
					for rowIdx, line := range fileMsg.buffer {
						if len(line) > 66 {
							t.Fatalf("[%s, w=%d, row=%d] line width %d exceeded 66 columns", tc.desc, w, rowIdx, len(line))
						}
					}

					// Test Draw() on simulation screen to verify zero panics
					simScreen := newSimulationScreen(100, 40)
					proxy := mauview.NewProxyScreen(simScreen, 2, 2, w, max(1, h))
					fileMsg.Draw(proxy, uiMsg)
				}
			})
		}
	}
}

// 2. Stress test decompression bombs (>16MP images) and corrupt data streams.
// Assert zero panics, zero memory spikes, and clean fallback to red error annotations.
func TestChallenger1_M3_DecompressionBombsAndCorruptStreams(t *testing.T) {
	type payloadCase struct {
		name    string
		data    []byte
		isBomb  bool
		errWord string
	}

	cases := []payloadCase{
		{
			name:    "100MP bomb (10000x10000)",
			data:    createCraftedPNGHeaderBytes(10000, 10000),
			isBomb:  true,
			errWord: "exceeds maximum allowed size of 16 megapixels",
		},
		{
			name:    "400MP bomb (20000x20000)",
			data:    createCraftedPNGHeaderBytes(20000, 20000),
			isBomb:  true,
			errWord: "exceeds maximum allowed size of 16 megapixels",
		},
		{
			name:    "2.5GP bomb (50000x50000)",
			data:    createCraftedPNGHeaderBytes(50000, 50000),
			isBomb:  true,
			errWord: "exceeds maximum allowed size of 16 megapixels",
		},
		{
			name:    "16.004MP boundary bomb (4001x4000)",
			data:    createCraftedPNGHeaderBytes(4001, 4000),
			isBomb:  true,
			errWord: "exceeds maximum allowed size of 16 megapixels",
		},
		{
			name:    "10^12 pixels integer overflow bomb (1000000x1000000)",
			data:    createCraftedPNGHeaderBytes(1000000, 1000000),
			isBomb:  true,
			errWord: "exceeds maximum allowed size of 16 megapixels",
		},
		{
			name:    "int32 max boundary bomb",
			data:    createCraftedPNGHeaderBytes(1<<31-1, 1<<31-1),
			isBomb:  true,
			errWord: "exceeds maximum allowed size of 16 megapixels",
		},
		{
			name:    "0x0 zero dimensions",
			data:    createCraftedPNGHeaderBytes(0, 0),
			isBomb:  false,
			errWord: "non-positive dimensions",
		},
		{
			name:    "0x100 zero width",
			data:    createCraftedPNGHeaderBytes(0, 100),
			isBomb:  false,
			errWord: "non-positive dimensions",
		},
		{
			name:    "100x0 zero height",
			data:    createCraftedPNGHeaderBytes(100, 0),
			isBomb:  false,
			errWord: "non-positive dimensions",
		},
		{
			name:    "empty byte stream",
			data:    []byte{},
			isBomb:  false,
			errWord: "Download media",
		},
		{
			name:    "1-byte corrupt stream",
			data:    []byte{0x89},
			isBomb:  false,
			errWord: "failed to decode image config",
		},
		{
			name:    "truncated 8-byte PNG signature only",
			data:    []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
			isBomb:  false,
			errWord: "failed to decode image config",
		},
		{
			name:    "corrupt random garbage stream",
			data:    []byte("CORRUPT_NOT_AN_IMAGE_STREAM_0123456789_!@#$%^&*()"),
			isBomb:  false,
			errWord: "failed to decode image config",
		},
		{
			name:    "valid PNG header followed by truncated IDAT",
			data:    append(createCraftedPNGHeaderBytes(100, 100), []byte("RANDOM_CORRUPT_IDAT_BYTES")...),
			isBomb:  false,
			errWord: "Failed to display image",
		},
	}

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "halfblocks",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Measure memory before test
			runtime.GC()
			var memBefore runtime.MemStats
			runtime.ReadMemStats(&memBefore)

			content := &event.MessageEventContent{
				MsgType: event.MsgImage,
				Body:    "payload.png",
				URL:     "mxc://matrix.example.com/payload",
			}
			uiMsg, fileMsg := createTestUIMessage(content)

			// Bypass SetImageData to directly inject payload into msg.imageData (simulating hostile download)
			fileMsg.mu.Lock()
			fileMsg.imageData = tc.data
			fileMsg.mu.Unlock()

			// Must not panic
			uiMsg.CalculateBuffer(prefs, 80)

			// Measure memory after test
			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			// Assert zero memory spike (heap alloc delta < 5MB)
			heapDelta := int64(memAfter.Alloc) - int64(memBefore.Alloc)
			if heapDelta > 5*1024*1024 {
				t.Errorf("[%s] memory spike detected: %d bytes allocated during validation", tc.name, heapDelta)
			}

			// Height must be safe
			if fileMsg.Height() > 16 {
				t.Errorf("[%s] height exploded to %d rows", tc.name, fileMsg.Height())
			}
			if fileMsg.Height() < 1 {
				t.Errorf("[%s] height is 0, expected fallback buffer", tc.name)
			}

			// Verify fallback buffer has RED error annotation
			if len(tc.data) > 0 {
				hasRedCell := false
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
				if !hasRedCell {
					t.Errorf("[%s] expected RED error annotation in buffer", tc.name)
				}
			}

			// Verify Draw does not panic on the error fallback buffer
			simScreen := newSimulationScreen(100, 40)
			proxy := mauview.NewProxyScreen(simScreen, 2, 2, 80, fileMsg.Height())
			fileMsg.Draw(proxy, uiMsg)
		})
	}
}

// 3. Stress test rapid resize: simulate rapid window width oscillations and assert cache hits in renderCache.
func TestChallenger1_M3_RapidResize_CacheHitsVerification(t *testing.T) {
	pngBytes := createSolidColorPNG(200, 200)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "resize_test.png",
		URL:     "mxc://matrix.example.com/resize",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngBytes)

	prefs := config.UserPreferences{
		ImagePreviewProtocol:  "halfblocks",
		ImagePreviewMaxWidth:  66,
		ImagePreviewMaxHeight: 16,
	}

	// Oscillation sequence
	oscillatingWidths := []int{20, 80, 20, 80, 30, 100, 30, 100, 15, 66, 15, 66}

	// First pass: populate cache and record buffer addresses
	firstAddresses := make(map[int]*tstring.Cell)
	for _, w := range oscillatingWidths {
		bbox := termimg.CalculateClampedDimensions(200, 200, w, 66, 16)
		targetCols := bbox.Cols

		uiMsg.InvalidateBuffer()
		uiMsg.CalculateBuffer(prefs, w)

		fileMsg.mu.RLock()
		buf := fileMsg.buffer
		if len(buf) > 0 && len(buf[0]) > 0 {
			firstAddresses[targetCols] = &buf[0][0]
		}
		fileMsg.mu.RUnlock()
	}

	cacheAfterWarmup := fileMsg.RenderCache()
	if len(cacheAfterWarmup) == 0 {
		t.Fatalf("renderCache was not populated after warm-up pass")
	}

	// Second pass: assert exact cache hit (identical slice pointer reuse) without re-allocation
	for _, w := range oscillatingWidths {
		bbox := termimg.CalculateClampedDimensions(200, 200, w, 66, 16)
		targetCols := bbox.Cols

		expectedAddr, ok := firstAddresses[targetCols]
		if !ok || expectedAddr == nil {
			t.Fatalf("expected recorded address for cols=%d", targetCols)
		}

		uiMsg.InvalidateBuffer()
		uiMsg.CalculateBuffer(prefs, w)

		// Assert buffer in fileMsg was populated directly from cache
		fileMsg.mu.RLock()
		buf := fileMsg.buffer
		fileMsg.mu.RUnlock()

		if len(buf) == 0 || len(buf[0]) == 0 {
			t.Fatalf("empty buffer for width %d", w)
		}

		currentAddr := &buf[0][0]
		if currentAddr != expectedAddr {
			t.Errorf("width %d (cols=%d): cache MISS! Buffer re-allocated at %p instead of reusing %p",
				w, targetCols, currentAddr, expectedAddr)
		}
	}
}

// 3b. High-concurrency rapid resize stress test under -race
func TestChallenger1_M3_RapidResize_ConcurrentStress(t *testing.T) {
	pngBytes := createSolidColorPNG(150, 100)

	content := &event.MessageEventContent{
		MsgType: event.MsgImage,
		Body:    "concurrent_stress.png",
		URL:     "mxc://matrix.example.com/concurrent",
	}
	uiMsg, fileMsg := createTestUIMessage(content)
	fileMsg.SetImageData(pngBytes)

	simScreen := newSimulationScreen(120, 50)
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Goroutines 1-3: Rapid buffer calculations with oscillating widths and alternating protocols
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			widths := []int{10, 20, 30, 40, 50, 66, 80, 100, 120}
			protos := []string{"iterm2", "halfblocks"}

			for {
				select {
				case <-stop:
					return
				default:
					w := widths[rnd.Intn(len(widths))]
					p := protos[rnd.Intn(len(protos))]
					prefs := config.UserPreferences{
						ImagePreviewProtocol:  p,
						ImagePreviewMaxWidth:  66,
						ImagePreviewMaxHeight: 16,
					}
					uiMsg.CalculateBuffer(prefs, w)
					time.Sleep(time.Duration(rnd.Intn(500)) * time.Microsecond)
				}
			}
		}(g)
	}

	// Goroutine 4: Rapid buffer invalidation
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				uiMsg.InvalidateBuffer()
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Goroutines 5-6: Rapid Draw() with oscillating screen positions
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id+10)))
			for {
				select {
				case <-stop:
					return
				default:
					x := rnd.Intn(20) - 5
					y := rnd.Intn(40) - 5
					h := max(1, fileMsg.Height())
					proxy := mauview.NewProxyScreen(simScreen, x, y, 70, h)
					fileMsg.Draw(proxy, uiMsg)
					time.Sleep(time.Duration(rnd.Intn(500)) * time.Microsecond)
				}
			}
		}(g)
	}

	// Goroutine 7: Rapid RenderCache queries
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c := fileMsg.RenderCache()
				_ = len(c)
				time.Sleep(500 * time.Microsecond)
			}
		}
	}()

	// Goroutine 8: Cloning
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = uiMsg.Clone()
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Goroutine 9: Mask operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = fileMsg.CurrentMask()
				fileMsg.ClearMask()
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Run stress test for 400ms under -race
	time.Sleep(400 * time.Millisecond)
	close(stop)
	wg.Wait()
}
