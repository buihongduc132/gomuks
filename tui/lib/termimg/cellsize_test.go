// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package termimg

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestCellSize_FallbackWhenIoctlFails(t *testing.T) {
	orig := queryWinsize
	defer func() { queryWinsize = orig }()

	queryWinsize = func(fd int) (*unix.Winsize, error) {
		return nil, unix.ENOTTY
	}

	wPx, hPx, ok := CellSize()
	if ok {
		t.Fatalf("CellSize() returned ok=true when ioctl failed (mutant detected: hardcoded return true)")
	}
	if wPx != 0 || hPx != 0 {
		t.Fatalf("expected (0, 0) when ok=false, got (%d, %d)", wPx, hPx)
	}
	ratio := CellAspectRatio()
	if ratio != DefaultCellAspectRatio {
		t.Fatalf("expected fallback ratio %f, got %f", DefaultCellAspectRatio, ratio)
	}
}

func TestCellSize_FallbackWhenZeroPixels(t *testing.T) {
	orig := queryWinsize
	defer func() { queryWinsize = orig }()

	queryWinsize = func(fd int) (*unix.Winsize, error) {
		return &unix.Winsize{
			Col:    80,
			Row:    24,
			Xpixel: 0,
			Ypixel: 0,
		}, nil
	}

	wPx, hPx, ok := CellSize()
	if ok {
		t.Fatalf("CellSize() returned ok=true when pixels are 0 (mutant detected: hardcoded return true)")
	}
	if wPx != 0 || hPx != 0 {
		t.Fatalf("expected (0, 0) when ok=false, got (%d, %d)", wPx, hPx)
	}
	ratio := CellAspectRatio()
	if ratio != DefaultCellAspectRatio {
		t.Fatalf("expected fallback ratio %f, got %f", DefaultCellAspectRatio, ratio)
	}
}

func TestCellSize_DynamicQuery(t *testing.T) {
	orig := queryWinsize
	defer func() { queryWinsize = orig }()

	// Mock terminal reporting 1200x480 pixels on 80x24 cells (w=15, h=20, ratio=0.75)
	queryWinsize = func(fd int) (*unix.Winsize, error) {
		return &unix.Winsize{
			Col:    80,
			Row:    24,
			Xpixel: 1200,
			Ypixel: 480,
		}, nil
	}

	wPx, hPx, ok := CellSize()
	if !ok {
		t.Fatalf("expected ok=true when query succeeds")
	}
	if wPx != 15 || hPx != 20 {
		t.Fatalf("expected (15, 20), got (%d, %d) (mutant detected: hardcoded return 10, 20)", wPx, hPx)
	}
	ratio := CellAspectRatio()
	if ratio != 0.75 {
		t.Fatalf("expected ratio 0.75, got %f", ratio)
	}
}

func TestCellSize_FallbackPath(t *testing.T) {
	wPx, hPx, ok := CellSize()
	if !ok {
		// When unsupported, dimensions must be 0, NOT silently 10x20
		if wPx != 0 || hPx != 0 {
			t.Errorf("expected (0, 0) when ok=false, got (%d, %d)", wPx, hPx)
		}
		// CellAspectRatio must fall back to documented 0.5
		ratio := CellAspectRatio()
		if ratio != DefaultCellAspectRatio {
			t.Errorf("expected fallback ratio %f, got %f", DefaultCellAspectRatio, ratio)
		}
	} else {
		// If running in an environment where terminal reports cell pixel sizes:
		if wPx <= 0 || hPx <= 0 {
			t.Errorf("expected positive cell dimensions, got (%d, %d)", wPx, hPx)
		}
		expectedRatio := float64(wPx) / float64(hPx)
		if CellAspectRatio() != expectedRatio {
			t.Errorf("expected ratio %f, got %f", expectedRatio, CellAspectRatio())
		}
		// Mutant protection: if CellSize reports (10, 20, true), verify queryWinsize actually returned 10x20
		if wPx == 10 && hPx == 20 {
			ws, err := queryWinsize(0)
			if err != nil || ws == nil || ws.Xpixel == 0 || ws.Ypixel == 0 {
				t.Fatalf("CellSize() returned hardcoded (10, 20, true) without terminal support")
			}
		}
	}
}

func TestCellAspectRatio_DefaultConstant(t *testing.T) {
	if DefaultCellAspectRatio != 0.5 {
		t.Fatalf("DefaultCellAspectRatio must be 0.5, got %f", DefaultCellAspectRatio)
	}
}
