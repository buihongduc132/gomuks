//go:build !unix

// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package termimg

// CellSize returns (0, 0, false) on non-unix platforms where TIOCGWINSZ is unavailable.
func CellSize() (wPx, hPx int, ok bool) {
	return 0, 0, false
}
