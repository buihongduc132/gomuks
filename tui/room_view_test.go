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
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	"go.mau.fi/util/jsontime"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
)

func makeTestMessageEvent(eventID string, sender string, msgType event.MessageType, body string) *database.Event {
	content := event.MessageEventContent{
		MsgType: msgType,
		Body:    body,
	}
	raw, _ := json.Marshal(content)
	return &database.Event{
		ID:        id.EventID(eventID),
		Sender:    id.UserID(sender),
		Type:      event.EventMessage.Type,
		Content:   raw,
		Timestamp: jsontime.UM(time.Now()),
	}
}

func TestFormatReplySnippet_NilEvent(t *testing.T) {
	if got := FormatReplySnippet(nil, 30); got != "" {
		t.Fatalf("expected empty string for nil event, got %q", got)
	}
}

func TestFormatReplySnippet_InvalidMaxCols(t *testing.T) {
	evt := makeTestMessageEvent("$1", "@alice:matrix.org", event.MsgText, "Hello")
	if got := FormatReplySnippet(evt, 0); got != "" {
		t.Fatalf("expected empty string for maxCols=0, got %q", got)
	}
	if got := FormatReplySnippet(evt, -5); got != "" {
		t.Fatalf("expected empty string for negative maxCols, got %q", got)
	}
}

func TestFormatReplySnippet_Redacted(t *testing.T) {
	evt := &database.Event{
		ID:         "$redacted",
		Sender:     "@alice:matrix.org",
		Type:       event.EventMessage.Type,
		RedactedBy: "$redaction_event_id",
	}
	got := FormatReplySnippet(evt, 30)
	if got != "[Redacted]" {
		t.Fatalf("expected %q, got %q", "[Redacted]", got)
	}

	// Truncated redacted
	gotTrunc := FormatReplySnippet(evt, 5)
	if runewidth.StringWidth(gotTrunc) > 5 {
		t.Fatalf("expected width <= 5, got %q (width %d)", gotTrunc, runewidth.StringWidth(gotTrunc))
	}
	if !strings.HasSuffix(gotTrunc, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", gotTrunc)
	}
}

func TestFormatReplySnippet_DecryptionError(t *testing.T) {
	evt := &database.Event{
		ID:              "$dec_err",
		Sender:          "@alice:matrix.org",
		Type:            event.EventEncrypted.Type,
		DecryptionError: "Megolm session not found",
	}
	got := FormatReplySnippet(evt, 30)
	if got != "[Decryption error]" {
		t.Fatalf("expected %q, got %q", "[Decryption error]", got)
	}
}

func TestFormatReplySnippet_EncryptedUndecrypted(t *testing.T) {
	evt := &database.Event{
		ID:     "$enc",
		Sender: "@alice:matrix.org",
		Type:   event.EventEncrypted.Type,
	}
	got := FormatReplySnippet(evt, 30)
	if got != "[Encrypted message]" {
		t.Fatalf("expected %q, got %q", "[Encrypted message]", got)
	}
}

func TestFormatReplySnippet_StateEvent(t *testing.T) {
	evt := &database.Event{
		ID:       "$state",
		Sender:   "@alice:matrix.org",
		Type:     event.StateMember.Type,
		StateKey: ptr.Ptr("@bob:matrix.org"),
		Content:  []byte(`{"membership": "join"}`),
	}
	// Must not panic on AsMessage() and must return clean fallback
	got := FormatReplySnippet(evt, 30)
	if got != "[State event]" {
		t.Fatalf("expected %q, got %q", "[State event]", got)
	}
}

func TestFormatReplySnippet_StickerEvent(t *testing.T) {
	evt := &database.Event{
		ID:      "$sticker",
		Sender:  "@alice:matrix.org",
		Type:    event.EventSticker.Type,
		Content: []byte(`{"body": "sticker.png"}`),
	}
	got := FormatReplySnippet(evt, 30)
	if got != "[Sticker]" {
		t.Fatalf("expected %q, got %q", "[Sticker]", got)
	}
}

func TestFormatReplySnippet_MediaTypes(t *testing.T) {
	cases := []struct {
		name     string
		msgType  event.MessageType
		body     string
		expected string
	}{
		{"image with filename", event.MsgImage, "cat.png", "[Image: cat.png]"},
		{"image without filename", event.MsgImage, "", "[Image]"},
		{"video with filename", event.MsgVideo, "recording.mp4", "[Video: recording.mp4]"},
		{"video without filename", event.MsgVideo, "   ", "[Video]"},
		{"audio with filename", event.MsgAudio, "voice.ogg", "[Audio: voice.ogg]"},
		{"audio without filename", event.MsgAudio, "", "[Audio]"},
		{"file with filename", event.MsgFile, "notes.pdf", "[File: notes.pdf]"},
		{"file without filename", event.MsgFile, "", "[File]"},
		{"location with description", event.MsgLocation, "Berlin, Germany", "[Location: Berlin, Germany]"},
		{"location without description", event.MsgLocation, "", "[Location]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := makeTestMessageEvent("$media", "@alice:matrix.org", tc.msgType, tc.body)
			got := FormatReplySnippet(evt, 30)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestFormatReplySnippet_Emote(t *testing.T) {
	evt := makeTestMessageEvent("$emote", "@alice:matrix.org", event.MsgEmote, "waves happily")
	got := FormatReplySnippet(evt, 30)
	if got != "* waves happily" {
		t.Fatalf("expected %q, got %q", "* waves happily", got)
	}
}

func TestFormatReplySnippet_PlainTextAndEmpty(t *testing.T) {
	evt := makeTestMessageEvent("$text", "@alice:matrix.org", event.MsgText, "Hello world!")
	if got := FormatReplySnippet(evt, 30); got != "Hello world!" {
		t.Fatalf("expected %q, got %q", "Hello world!", got)
	}

	emptyEvt := makeTestMessageEvent("$empty", "@alice:matrix.org", event.MsgText, "")
	if got := FormatReplySnippet(emptyEvt, 30); got != "" {
		t.Fatalf("expected empty string for empty body, got %q", got)
	}

	whitespaceEvt := makeTestMessageEvent("$ws", "@alice:matrix.org", event.MsgText, "   \n\t  \r\n ")
	if got := FormatReplySnippet(whitespaceEvt, 30); got != "" {
		t.Fatalf("expected empty string for whitespace body, got %q", got)
	}
}

func TestFormatReplySnippet_WhitespaceAndNewlineNormalization(t *testing.T) {
	input := "Line 1\n\nLine 2\r\nLine 3\t\tLine 4   end"
	evt := makeTestMessageEvent("$norm", "@alice:matrix.org", event.MsgText, input)
	got := FormatReplySnippet(evt, 50)
	expected := "Line 1 Line 2 Line 3 Line 4 end"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
	if strings.ContainsAny(got, "\r\n\t") {
		t.Fatalf("snippet contains raw whitespace/newlines: %q", got)
	}
}

func TestFormatReplySnippet_QuoteFallbackStripping(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "standard matrix reply fallback",
			body:     "> <@alice:matrix.org> Hello world\n\nI agree completely!",
			expected: "I agree completely!",
		},
		{
			name:     "quote fallback with empty quote line >\\n",
			body:     "> <@alice:matrix.org> Line 1\n>\n> Line 2\n\nUnderstood.",
			expected: "Understood.",
		},
		{
			name:     "quote fallback with space empty quote line > \\n",
			body:     "> <@alice:matrix.org> Line 1\n> \n> Line 2\n\nUnderstood.",
			expected: "Understood.",
		},
		{
			name:     "quote fallback with blank line between quotes",
			body:     "> <@alice:matrix.org> Line 1\n\n> Line 2\n\nUnderstood.",
			expected: "Understood.",
		},
		{
			name:     "nested quote fallback",
			body:     "> > nested\n> line\n\nMy top level reply",
			expected: "My top level reply",
		},
		{
			name:     "quote only single line",
			body:     "> A quoted line with no reply",
			expected: "A quoted line with no reply",
		},
		{
			name:     "quote only multi line",
			body:     "> Line 1\n>\n> Line 2",
			expected: "Line 1 Line 2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := makeTestMessageEvent("$quote", "@alice:matrix.org", event.MsgText, tc.body)
			got := FormatReplySnippet(evt, 50)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestFormatReplySnippet_UnicodeAndRunewidthTruncation(t *testing.T) {
	// ASCII truncation
	asciiEvt := makeTestMessageEvent("$ascii", "@alice:matrix.org", event.MsgText, "The quick brown fox jumps over the lazy dog")
	gotAscii := FormatReplySnippet(asciiEvt, 20)
	if width := runewidth.StringWidth(gotAscii); width > 20 {
		t.Fatalf("expected width <= 20, got %d for %q", width, gotAscii)
	}
	if !strings.HasSuffix(gotAscii, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", gotAscii)
	}

	// CJK characters (display width 2 per character)
	cjkEvt := makeTestMessageEvent("$cjk", "@alice:matrix.org", event.MsgText, "你好世界这是一个非常长的中文回复消息测试")
	gotCJK := FormatReplySnippet(cjkEvt, 10)
	if width := runewidth.StringWidth(gotCJK); width > 10 {
		t.Fatalf("expected CJK width <= 10, got %d for %q", width, gotCJK)
	}
	if !strings.HasSuffix(gotCJK, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", gotCJK)
	}
	// Verify no broken UTF-8 runes
	for _, r := range gotCJK {
		if r == '\uFFFD' {
			t.Fatalf("found Unicode replacement char in %q (UTF-8 split)", gotCJK)
		}
	}

	// Emojis (display width 2 per emoji)
	emojiEvt := makeTestMessageEvent("$emoji", "@alice:matrix.org", event.MsgText, "🎉🚀👋🔥✨🎈🐱🐶")
	gotEmoji := FormatReplySnippet(emojiEvt, 8)
	if width := runewidth.StringWidth(gotEmoji); width > 8 {
		t.Fatalf("expected emoji width <= 8, got %d for %q", width, gotEmoji)
	}
	if !strings.HasSuffix(gotEmoji, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", gotEmoji)
	}

	// Exact length boundary
	exactEvt := makeTestMessageEvent("$exact", "@alice:matrix.org", event.MsgText, strings.Repeat("A", 30))
	gotExact := FormatReplySnippet(exactEvt, 30)
	if gotExact != strings.Repeat("A", 30) {
		t.Fatalf("exact boundary should not append ellipsis, got %q", gotExact)
	}

	// 1 character over limit
	overEvt := makeTestMessageEvent("$over", "@alice:matrix.org", event.MsgText, strings.Repeat("A", 31))
	gotOver := FormatReplySnippet(overEvt, 30)
	if width := runewidth.StringWidth(gotOver); width != 30 {
		t.Fatalf("expected width 30, got %d for %q", width, gotOver)
	}
	if !strings.HasSuffix(gotOver, "…") {
		t.Fatalf("expected ellipsis suffix for over-length, got %q", gotOver)
	}
}

func TestStripReplyFallback(t *testing.T) {
	if StripReplyFallback("") != "" {
		t.Fatalf("expected empty string for empty input")
	}

	plain := "Hello world"
	if got := StripReplyFallback(plain); got != plain {
		t.Fatalf("expected %q, got %q", plain, got)
	}

	crlf := "> <@alice:matrix.org> quote\r\n\r\nreply with CRLF"
	if got := StripReplyFallback(crlf); got != "reply with CRLF" {
		t.Fatalf("expected %q, got %q", "reply with CRLF", got)
	}

	// Later line contains '>' but is not part of quote fallback
	bodyWithGT := "My answer is:\n> this line is blockquoted by author\nConclusion."
	if got := StripReplyFallback(bodyWithGT); got != bodyWithGT {
		t.Fatalf("expected text with internal '>' unchanged, got %q", got)
	}

	// Only '>' characters
	if got := StripReplyFallback(">\n> "); got != "" {
		t.Fatalf("expected empty string for empty quotes, got %q", got)
	}
}

func TestRoomView_GetStatus_Replying(t *testing.T) {
	// 1. Replying to a normal text message
	msgEvt := makeTestMessageEvent("$1", "@alice:matrix.org", event.MsgText, "Can we meet tomorrow?")
	rv := &RoomView{
		replying: msgEvt,
	}
	status := rv.GetStatus()
	expected := `Replying to @alice:matrix.org: "Can we meet tomorrow?"`
	if status != expected {
		t.Fatalf("expected status %q, got %q", expected, status)
	}

	// 2. Replying to an empty message (no snippet appended)
	emptyEvt := makeTestMessageEvent("$2", "@alice:matrix.org", event.MsgText, "")
	rv.replying = emptyEvt
	statusEmpty := rv.GetStatus()
	expectedEmpty := "Replying to @alice:matrix.org"
	if statusEmpty != expectedEmpty {
		t.Fatalf("expected status %q, got %q", expectedEmpty, statusEmpty)
	}

	// 3. Replying to redacted event
	redactedEvt := &database.Event{
		ID:         "$3",
		Sender:     "@bob:matrix.org",
		Type:       event.EventMessage.Type,
		RedactedBy: "$redaction",
	}
	rv.replying = redactedEvt
	statusRedacted := rv.GetStatus()
	expectedRedacted := `Replying to @bob:matrix.org: "[Redacted]"`
	if statusRedacted != expectedRedacted {
		t.Fatalf("expected status %q, got %q", expectedRedacted, statusRedacted)
	}

	// 4. Replying to decryption failure
	decErrEvt := &database.Event{
		ID:              "$4",
		Sender:          "@charlie:matrix.org",
		Type:            event.EventEncrypted.Type,
		DecryptionError: "Session unknown",
	}
	rv.replying = decErrEvt
	statusDecErr := rv.GetStatus()
	expectedDecErr := `Replying to @charlie:matrix.org: "[Decryption error]"`
	if statusDecErr != expectedDecErr {
		t.Fatalf("expected status %q, got %q", expectedDecErr, statusDecErr)
	}

	// 5. Replying to state event
	stateEvt := &database.Event{
		ID:       "$5",
		Sender:   "@david:matrix.org",
		Type:     event.StateMember.Type,
		StateKey: ptr.Ptr("@david:matrix.org"),
		Content:  []byte(`{"membership": "leave"}`),
	}
	rv.replying = stateEvt
	statusState := rv.GetStatus()
	expectedState := `Replying to @david:matrix.org: "[State event]"`
	if statusState != expectedState {
		t.Fatalf("expected status %q, got %q", expectedState, statusState)
	}

	// 6. Replying to media event (image)
	imgEvt := makeTestMessageEvent("$6", "@eve:matrix.org", event.MsgImage, "sunset.jpg")
	rv.replying = imgEvt
	statusImg := rv.GetStatus()
	expectedImg := `Replying to @eve:matrix.org: "[Image: sunset.jpg]"`
	if statusImg != expectedImg {
		t.Fatalf("expected status %q, got %q", expectedImg, statusImg)
	}

	// 7. Replying with empty sender fallback
	noSenderEvt := makeTestMessageEvent("$7", "", event.MsgText, "Ghost message")
	rv.replying = noSenderEvt
	statusNoSender := rv.GetStatus()
	expectedNoSender := `Replying to message: "Ghost message"`
	if statusNoSender != expectedNoSender {
		t.Fatalf("expected status %q, got %q", expectedNoSender, statusNoSender)
	}

	// 8. Multi-line message reply must stay single-line in status bar
	multiLineEvt := makeTestMessageEvent("$8", "@frank:matrix.org", event.MsgText, "Line 1\nLine 2\nLine 3")
	rv.replying = multiLineEvt
	statusMulti := rv.GetStatus()
	if strings.ContainsAny(statusMulti, "\r\n") {
		t.Fatalf("status line contains newlines: %q", statusMulti)
	}
}

func TestRoomView_GetStatus_NilSafety(t *testing.T) {
	// Nil receiver must return empty string and not panic
	var nilRv *RoomView
	if got := nilRv.GetStatus(); got != "" {
		t.Fatalf("expected empty status for nil RoomView, got %q", got)
	}

	// RoomView with all nil fields must not panic
	rv := &RoomView{}
	if got := rv.GetStatus(); got != "" {
		t.Fatalf("expected empty status for uninitialized RoomView, got %q", got)
	}

	// Editing state
	rv.editing = &database.Event{ID: "$edit"}
	if got := rv.GetStatus(); got != "Editing message" {
		t.Fatalf("expected %q, got %q", "Editing message", got)
	}
	rv.editing = nil

	// Selecting state
	rv.selecting = true
	rv.selectReason = "reply"
	if got := rv.GetStatus(); got != "Selecting message to reply" {
		t.Fatalf("expected %q, got %q", "Selecting message to reply", got)
	}
}

func TestRoomView_ClearAllContext_NilInput(t *testing.T) {
	rv := &RoomView{
		content:  &MessageView{},
		replying: &database.Event{ID: "$reply"},
		input:    nil,
	}
	rv.ClearAllContext()
	if rv.replying != nil {
		t.Fatalf("expected rv.replying to be nil after ClearAllContext()")
	}
}

func TestRoomView_GetStatus_ConcurrentReplyingTOCTOURace(t *testing.T) {
	msgEvt := makeTestMessageEvent("$race_msg", "@alice:matrix.org", event.MsgText, "Concurrent reply test message")
	rv := &RoomView{}

	const iterations = 25000
	const numReaders = 4
	const numWriters = 2

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					rv.replying = msgEvt
					rv.replying = nil
				}
			}
		}()
	}

	var readersWg sync.WaitGroup
	for r := 0; r < numReaders; r++ {
		readersWg.Add(1)
		go func() {
			defer readersWg.Done()
			for i := 0; i < iterations; i++ {
				status := rv.GetStatus()
				if status != "" && !strings.HasPrefix(status, "Replying to @alice:matrix.org") {
					t.Errorf("unexpected status format during concurrent race: %q", status)
					return
				}
			}
		}()
	}

	readersWg.Wait()
	close(stop)
	wg.Wait()
}
