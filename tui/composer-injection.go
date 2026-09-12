// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"go.mau.fi/mauview"
)

// ComposerStringWidth calculates the display runewidth of a string congruent with mauview.InputArea,
// where each character's width is given by runewidth.RuneWidth(ch) and newlines ('\n')
// are assigned a width of 1.
func ComposerStringWidth(s string) (w int) {
	w = runewidth.StringWidth(s)
	for _, ch := range s {
		if ch == '\n' {
			w++
		}
	}
	return
}

// ComposerSubstringBefore returns the prefix of string s up to visual runewidth limit w,
// counting '\n' as width 1, congruent with mauview.iaSubstringBefore.
// It guarantees slicing strictly on valid UTF-8 rune boundaries.
func ComposerSubstringBefore(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ComposerStringWidth(s) <= w {
		return s
	}
	r := []rune(s)
	width := 0
	for i := 0; i < len(r); i++ {
		cw := runewidth.RuneWidth(r[i])
		if r[i] == '\n' {
			cw = 1
		}
		if width+cw > w {
			return string(r[:i])
		}
		width += cw
	}
	return s
}

// SpliceTags partitions text at the cursor runewidth offset, replaces any preceding '@' trigger,
// formats and splices the given user tags, and returns the reassembled text and the new cursor offset.
// If tags is empty or contains only whitespace, the original text and cursor offset are returned unmodified.
func SpliceTags(text string, cursorOffset int, tags []string) (string, int) {
	if len(tags) == 0 {
		return text, cursorOffset
	}

	var cleanTags []string
	for _, tag := range tags {
		t := strings.TrimSpace(tag)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "@") {
			t = "@" + t
		}
		cleanTags = append(cleanTags, t)
	}
	if len(cleanTags) == 0 {
		return text, cursorOffset
	}

	formattedTags := strings.Join(cleanTags, " ") + " "

	totalWidth := ComposerStringWidth(text)
	if cursorOffset < 0 {
		cursorOffset = 0
	} else if cursorOffset > totalWidth {
		cursorOffset = totalWidth
	}

	before := ComposerSubstringBefore(text, cursorOffset)
	cutIndex := len(before)
	if cutIndex > len(text) {
		cutIndex = len(text)
	}
	after := text[cutIndex:]

	if strings.HasSuffix(before, "@") {
		before = strings.TrimSuffix(before, "@")
	}

	prefix := before + formattedTags
	newText := prefix + after
	newCursorOffset := ComposerStringWidth(prefix)

	return newText, newCursorOffset
}

// InjectTagsIntoComposer injects selected user tags into a mauview.InputArea at the current cursor offset.
// It safely handles nil pointers, replaces any inline '@' trigger, updates the text buffer and cursor offset,
// clears any active selection, and ensures the composer maintains focus.
func InjectTagsIntoComposer(input *mauview.InputArea, tags []string) {
	if input == nil || len(tags) == 0 {
		return
	}

	text := input.GetText()
	cursorOffset := input.GetCursorOffset()

	newText, newCursor := SpliceTags(text, cursorOffset, tags)
	if newText == text && newCursor == cursorOffset {
		return
	}

	input.SetText(newText)
	input.SetCursorOffset(newCursor)
	input.ClearSelection()
	input.Focus()
}

// stringWidth is an alias for ComposerStringWidth for internal package usage.
func stringWidth(s string) int {
	return ComposerStringWidth(s)
}

// substringBefore is an alias for ComposerSubstringBefore for internal package usage.
func substringBefore(s string, w int) string {
	return ComposerSubstringBefore(s, w)
}
