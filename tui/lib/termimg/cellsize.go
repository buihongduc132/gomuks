// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package termimg

// DefaultCellAspectRatio is the standard fallback ratio (width / height = 0.5, i.e. 1:2).
const DefaultCellAspectRatio = 0.5

// CellAspectRatio returns the aspect ratio (width / height) of a terminal cell.
// If CellSize() succeeds, it returns float64(wPx) / float64(hPx).
// Otherwise, it falls back to DefaultCellAspectRatio (0.5).
func CellAspectRatio() float64 {
	if wPx, hPx, ok := CellSize(); ok && wPx > 0 && hPx > 0 {
		return float64(wPx) / float64(hPx)
	}
	return DefaultCellAspectRatio
}
