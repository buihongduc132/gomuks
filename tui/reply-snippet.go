// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2025 Tulir Asokan
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
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/gomuks/pkg/hicli/database"
)

// DefaultReplySnippetMaxCols defines the default maximum display columns for a reply snippet.
const DefaultReplySnippetMaxCols = 30

// StripReplyFallback removes Matrix quote fallback lines from the beginning of a message.
// It handles standard quotes ("> "), empty quote lines (">\n"), blank lines within quotes,
// and quote-only messages (stripping the ">" prefix from quote lines).
func StripReplyFallback(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	trimmed := strings.TrimLeft(text, "\n")
	if !strings.HasPrefix(trimmed, ">") {
		return text
	}

	lines := strings.Split(trimmed, "\n")
	var quoteLines []string
	quoteEndIdx := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, ">") {
			quoteLines = append(quoteLines, line)
			quoteEndIdx = i + 1
		} else if strings.TrimSpace(line) == "" && i+1 < len(lines) && strings.HasPrefix(lines[i+1], ">") {
			// Blank line between quote lines inside the fallback block
			quoteLines = append(quoteLines, line)
			quoteEndIdx = i + 1
		} else {
			// Quote block ended
			break
		}
	}

	remaining := strings.TrimSpace(strings.Join(lines[quoteEndIdx:], "\n"))
	if remaining != "" {
		return remaining
	}

	// Quote-only message: strip ">" and optional leading space from each quote line
	var strippedQuoteLines []string
	for _, ql := range quoteLines {
		ql = strings.TrimPrefix(ql, ">")
		ql = strings.TrimPrefix(ql, " ")
		strippedQuoteLines = append(strippedQuoteLines, ql)
	}
	return strings.TrimSpace(strings.Join(strippedQuoteLines, "\n"))
}

func formatMediaFallback(mediaType, body string) string {
	cleanBody := strings.TrimSpace(body)
	if cleanBody != "" {
		return fmt.Sprintf("[%s: %s]", mediaType, cleanBody)
	}
	return fmt.Sprintf("[%s]", mediaType)
}

// FormatReplySnippet extracts a safely truncated, single-line plain text snippet
// from a replied-to event, respecting maxCols terminal display columns.
func FormatReplySnippet(evt *database.Event, maxCols int) string {
	if evt == nil || maxCols <= 0 {
		return ""
	}

	if evt.RedactedBy != "" {
		return runewidth.Truncate("[Redacted]", maxCols, "…")
	}

	if evt.DecryptionError != "" {
		return runewidth.Truncate("[Decryption error]", maxCols, "…")
	}

	if evt.StateKey != nil || evt.GetType().Class == event.StateEventType {
		return runewidth.Truncate("[State event]", maxCols, "…")
	}

	if evt.Type == event.EventEncrypted.Type && evt.Decrypted == nil {
		return runewidth.Truncate("[Encrypted message]", maxCols, "…")
	}

	if evt.Type == event.EventSticker.Type {
		return runewidth.Truncate("[Sticker]", maxCols, "…")
	}

	mautrixContent := evt.GetMautrixContent()
	if mautrixContent == nil {
		return ""
	}
	msgContent := mautrixContent.AsMessage()
	if msgContent == nil {
		return ""
	}

	var text string
	switch msgContent.MsgType {
	case event.MsgImage:
		text = formatMediaFallback("Image", msgContent.Body)
	case event.MsgVideo:
		text = formatMediaFallback("Video", msgContent.Body)
	case event.MsgAudio:
		text = formatMediaFallback("Audio", msgContent.Body)
	case event.MsgFile:
		text = formatMediaFallback("File", msgContent.Body)
	case event.MsgLocation:
		text = formatMediaFallback("Location", msgContent.Body)
	case event.MsgEmote:
		text = msgContent.Body
		if text != "" {
			text = "* " + text
		}
	case event.MsgText, event.MsgNotice:
		text = msgContent.Body
	default:
		if msgContent.Body != "" {
			text = msgContent.Body
		} else if msgContent.MsgType != "" {
			text = fmt.Sprintf("[%s]", msgContent.MsgType)
		} else {
			return ""
		}
	}

	text = StripReplyFallback(text)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	normalized := strings.Join(fields, " ")
	return runewidth.Truncate(normalized, maxCols, "…")
}
