// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package termimg

import (
	"os"

	"golang.org/x/sys/unix"
)

// DefaultCellAspectRatio is the standard fallback ratio (width / height = 0.5, i.e. 1:2).
const DefaultCellAspectRatio = 0.5

// CellSize queries the terminal via the TIOCGWINSZ ioctl for the pixel dimensions of a single cell.
// It checks stdout, stderr, stdin, and /dev/tty.
// If the terminal reports ws_xpixel or ws_ypixel as 0, or if an error occurs,
// ok is false and (0, 0, false) is returned.
// It does NOT silently return 10x20.
func CellSize() (wPx, hPx int, ok bool) {
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		if f == nil {
			continue
		}
		if ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ); err == nil && ws != nil {
			if ws.Col > 0 && ws.Row > 0 && ws.Xpixel > 0 && ws.Ypixel > 0 {
				w := int(ws.Xpixel / ws.Col)
				h := int(ws.Ypixel / ws.Row)
				if w > 0 && h > 0 {
					return w, h, true
				}
			}
		}
	}

	if tty, err := os.Open("/dev/tty"); err == nil {
		defer tty.Close()
		if ws, err := unix.IoctlGetWinsize(int(tty.Fd()), unix.TIOCGWINSZ); err == nil && ws != nil {
			if ws.Col > 0 && ws.Row > 0 && ws.Xpixel > 0 && ws.Ypixel > 0 {
				w := int(ws.Xpixel / ws.Col)
				h := int(ws.Ypixel / ws.Row)
				if w > 0 && h > 0 {
					return w, h, true
				}
			}
		}
	}

	return 0, 0, false
}

// CellAspectRatio returns the aspect ratio (width / height) of a terminal cell.
// If CellSize() succeeds, it returns float64(wPx) / float64(hPx).
// Otherwise, it falls back to DefaultCellAspectRatio (0.5).
func CellAspectRatio() float64 {
	if wPx, hPx, ok := CellSize(); ok && wPx > 0 && hPx > 0 {
		return float64(wPx) / float64(hPx)
	}
	return DefaultCellAspectRatio
}
