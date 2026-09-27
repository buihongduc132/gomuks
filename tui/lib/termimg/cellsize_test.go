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
)

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
	}
}

func TestCellAspectRatio_DefaultConstant(t *testing.T) {
	if DefaultCellAspectRatio != 0.5 {
		t.Fatalf("DefaultCellAspectRatio must be 0.5, got %f", DefaultCellAspectRatio)
	}
}
