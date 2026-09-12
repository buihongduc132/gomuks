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
	"testing"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
)

// -----------------------------------------------------------------------------
// TS-1: Helper Functions (stringWidth and substringBefore)
// -----------------------------------------------------------------------------

func TestComposerInjection_Helpers(t *testing.T) {
	t.Run("stringWidth_ASCII", func(t *testing.T) {
		if w := ComposerStringWidth("hello"); w != 5 {
			t.Errorf("expected width 5, got %d", w)
		}
		if w := stringWidth("hello"); w != 5 {
			t.Errorf("expected width 5 via alias, got %d", w)
		}
	})

	t.Run("stringWidth_NewlineAsOne", func(t *testing.T) {
		if w := ComposerStringWidth("a\nb"); w != 3 {
			t.Errorf("expected width 3 for 'a\\nb', got %d", w)
		}
		if w := ComposerStringWidth("\n\n\n"); w != 3 {
			t.Errorf("expected width 3 for 3 newlines, got %d", w)
		}
	})

	t.Run("stringWidth_CJKAndEmoji", func(t *testing.T) {
		if w := ComposerStringWidth("你好"); w != 4 {
			t.Errorf("expected width 4 for '你好', got %d", w)
		}
		if w := ComposerStringWidth("👋"); w != 2 {
			t.Errorf("expected width 2 for '👋', got %d", w)
		}
	})

	t.Run("substringBefore_ASCII", func(t *testing.T) {
		if s := ComposerSubstringBefore("hello", 3); s != "hel" {
			t.Errorf("expected 'hel', got %q", s)
		}
		if s := ComposerSubstringBefore("hello", 0); s != "" {
			t.Errorf("expected '', got %q", s)
		}
		if s := ComposerSubstringBefore("hello", 10); s != "hello" {
			t.Errorf("expected 'hello', got %q", s)
		}
	})

	t.Run("substringBefore_Newline", func(t *testing.T) {
		if s := ComposerSubstringBefore("a\nb", 2); s != "a\n" {
			t.Errorf("expected 'a\\n', got %q", s)
		}
	})

	t.Run("substringBefore_CJK_BoundaryAndClamp", func(t *testing.T) {
		if s := ComposerSubstringBefore("你好世界", 4); s != "你好" {
			t.Errorf("expected '你好', got %q", s)
		}
		// Mid-rune cursor (w=3 falls inside '好' at width 2-4): must clamp to '你'
		if s := ComposerSubstringBefore("你好世界", 3); s != "你" {
			t.Errorf("expected '你', got %q", s)
		}
	})
}

// -----------------------------------------------------------------------------
// TS-2: Basic Tag Splicing
// -----------------------------------------------------------------------------

func TestComposerInjection_BasicSplicing(t *testing.T) {
	t.Run("empty_composer", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "@alice " {
			t.Errorf("expected '@alice ', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 7 {
			t.Errorf("expected cursor offset 7, got %d", input.GetCursorOffset())
		}
	})

	t.Run("splice_at_end", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hello ")
		input.SetCursorOffset(6)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "Hello @alice " {
			t.Errorf("expected 'Hello @alice ', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 13 {
			t.Errorf("expected cursor offset 13, got %d", input.GetCursorOffset())
		}
	})

	t.Run("splice_middle_existing_draft", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hello world")
		input.SetCursorOffset(6) // after "Hello "

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "Hello @alice world" {
			t.Errorf("expected 'Hello @alice world', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 13 {
			t.Errorf("expected cursor offset 13, got %d", input.GetCursorOffset())
		}
	})

	t.Run("splice_at_start", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("world")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "@alice world" {
			t.Errorf("expected '@alice world', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 7 {
			t.Errorf("expected cursor offset 7, got %d", input.GetCursorOffset())
		}
	})
}

// -----------------------------------------------------------------------------
// TS-3: Inline Trigger '@' Replacement
// -----------------------------------------------------------------------------

func TestComposerInjection_TriggerAtReplacement(t *testing.T) {
	t.Run("replace_trailing_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hello @")
		input.SetCursorOffset(7)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "Hello @alice " {
			t.Errorf("expected 'Hello @alice ', got %q", input.GetText())
		}
		if strings.Contains(input.GetText(), "@@") {
			t.Errorf("found unwanted duplicate '@@': %q", input.GetText())
		}
		if input.GetCursorOffset() != 13 {
			t.Errorf("expected cursor offset 13, got %d", input.GetCursorOffset())
		}
	})

	t.Run("lone_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("@")
		input.SetCursorOffset(1)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "@alice " {
			t.Errorf("expected '@alice ', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 7 {
			t.Errorf("expected cursor offset 7, got %d", input.GetCursorOffset())
		}
	})

	t.Run("replace_at_in_middle", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Contact @ now")
		input.SetCursorOffset(9) // after "Contact @"

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "Contact @alice  now" {
			t.Errorf("expected 'Contact @alice  now', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 15 {
			t.Errorf("expected cursor offset 15, got %d", input.GetCursorOffset())
		}
	})

	t.Run("multiple_at_only_replaces_last", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("user@@")
		input.SetCursorOffset(6)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "user@@alice " {
			t.Errorf("expected 'user@@alice ', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 12 {
			t.Errorf("expected cursor offset 12, got %d", input.GetCursorOffset())
		}
	})

	t.Run("cursor_before_at_does_not_replace", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("@")
		input.SetCursorOffset(0) // cursor BEFORE '@'

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "@alice @" {
			t.Errorf("expected '@alice @', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 7 {
			t.Errorf("expected cursor offset 7, got %d", input.GetCursorOffset())
		}
	})

	t.Run("at_preceded_by_punctuation", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("(@")
		input.SetCursorOffset(2)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "(@alice " {
			t.Errorf("expected '(@alice ', got %q", input.GetText())
		}
		if input.GetCursorOffset() != 8 {
			t.Errorf("expected cursor offset 8, got %d", input.GetCursorOffset())
		}
	})
}

// -----------------------------------------------------------------------------
// TS-4: Tag Formatting Permutations
// -----------------------------------------------------------------------------

func TestComposerInjection_TagFormatting(t *testing.T) {
	t.Run("multiple_tags_without_prefix", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"alice", "bob", "charlie"})

		expected := "@alice @bob @charlie "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != len(expected) {
			t.Errorf("expected cursor offset %d, got %d", len(expected), input.GetCursorOffset())
		}
	})

	t.Run("tags_already_prefixed_with_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"@alice:example.com", "@bob:example.com"})

		expected := "@alice:example.com @bob:example.com "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if strings.Contains(input.GetText(), "@@") {
			t.Errorf("unwanted double @@ found in %q", input.GetText())
		}
	})

	t.Run("mixed_prefixes", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"Alice Adams", "@bob:matrix.org"})

		expected := "@Alice Adams @bob:matrix.org "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
	})

	t.Run("tags_with_surrounding_whitespace", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"  alice  ", " bob "})

		expected := "@alice @bob "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
	})

	t.Run("empty_tags_in_slice_filtered", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hello ")
		input.SetCursorOffset(6)

		InjectTagsIntoComposer(input, []string{"", "   ", "alice", ""})

		expected := "Hello @alice "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != 13 {
			t.Errorf("expected cursor offset 13, got %d", input.GetCursorOffset())
		}
	})
}

// -----------------------------------------------------------------------------
// TS-5: Multi-Line Drafts & Newlines
// -----------------------------------------------------------------------------

func TestComposerInjection_MultiLine(t *testing.T) {
	t.Run("end_of_second_line_with_trigger_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "first line" = 10, '\n' = 1, "second line @" = 13 -> total cursor = 24
		input.SetText("first line\nsecond line @")
		input.SetCursorOffset(24)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "first line\nsecond line @alice "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		// 10 ('first line') + 1 ('\n') + 12 ('second line ') + 7 ('@alice ') = 30
		if input.GetCursorOffset() != 30 {
			t.Errorf("expected cursor offset 30, got %d", input.GetCursorOffset())
		}
	})

	t.Run("start_of_second_line", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "first line\nsecond line", cursor right after '\n' (offset 11)
		input.SetText("first line\nsecond line")
		input.SetCursorOffset(11)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "first line\n@alice second line"
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != 18 {
			t.Errorf("expected cursor offset 18, got %d", input.GetCursorOffset())
		}
	})

	t.Run("consecutive_newlines", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "line1" = 5, '\n' = 1, '\n' = 1, "line2 @" = 7 -> cursor = 14
		input.SetText("line1\n\nline2 @")
		input.SetCursorOffset(14)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "line1\n\nline2 @alice "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		// 5 + 1 + 1 + 6 ('line2 ') + 7 ('@alice ') = 20
		if input.GetCursorOffset() != ComposerStringWidth(expected) {
			t.Errorf("expected cursor offset %d, got %d", ComposerStringWidth(expected), input.GetCursorOffset())
		}
	})

	t.Run("lone_newline_and_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("\n@")
		input.SetCursorOffset(2)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "\n@alice "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != 8 {
			t.Errorf("expected cursor offset 8, got %d", input.GetCursorOffset())
		}
	})
}

// -----------------------------------------------------------------------------
// TS-6: Unicode, CJK, and Emojis
// -----------------------------------------------------------------------------

func TestComposerInjection_UnicodeAndEmoji(t *testing.T) {
	t.Run("cjk_with_trigger_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "你" (2) + "好" (2) + " " (1) + "@" (1) = 6
		input.SetText("你好 @")
		input.SetCursorOffset(6)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "你好 @alice "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		// 2 + 2 + 1 + 7 = 12
		if input.GetCursorOffset() != 12 {
			t.Errorf("expected cursor offset 12, got %d", input.GetCursorOffset())
		}
	})

	t.Run("cjk_middle_splice", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "你好世界", cursor after "你好" (width 4)
		input.SetText("你好世界")
		input.SetCursorOffset(4)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "你好@alice 世界"
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != 11 {
			t.Errorf("expected cursor offset 11, got %d", input.GetCursorOffset())
		}
	})

	t.Run("cjk_mid_rune_clamp_safety", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "你好世界", cursor offset 3 falls inside '好' (spans 2..4)
		input.SetText("你好世界")
		input.SetCursorOffset(3)

		InjectTagsIntoComposer(input, []string{"alice"})

		// Clamps before '好' -> textBefore = "你", textAfter = "好世界"
		expected := "你@alice 好世界"
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != 9 {
			t.Errorf("expected cursor offset 9, got %d", input.GetCursorOffset())
		}
	})

	t.Run("emoji_with_trigger_at", func(t *testing.T) {
		input := mauview.NewInputArea()
		// "👋" (2) + " " (1) + "@" (1) = 4
		input.SetText("👋 @")
		input.SetCursorOffset(4)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := "👋 @alice "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != 10 {
			t.Errorf("expected cursor offset 10, got %d", input.GetCursorOffset())
		}
	})

	t.Run("unicode_display_names", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("")
		input.SetCursorOffset(0)

		InjectTagsIntoComposer(input, []string{"李四", "Éléonore"})

		expected := "@李四 @Éléonore "
		if input.GetText() != expected {
			t.Errorf("expected %q, got %q", expected, input.GetText())
		}
		expectedWidth := ComposerStringWidth(expected)
		if input.GetCursorOffset() != expectedWidth {
			t.Errorf("expected cursor offset %d, got %d", expectedWidth, input.GetCursorOffset())
		}
	})
}

// -----------------------------------------------------------------------------
// TS-7: Defensive Nil & Boundary Safety
// -----------------------------------------------------------------------------

func TestComposerInjection_DefensiveSafety(t *testing.T) {
	t.Run("nil_input_does_not_panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("InjectTagsIntoComposer panicked on nil input: %v", r)
			}
		}()
		InjectTagsIntoComposer(nil, []string{"alice"})
	})

	t.Run("nil_tags_slice_leaves_buffer_unchanged", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Draft text")
		input.SetCursorOffset(5)

		InjectTagsIntoComposer(input, nil)

		if input.GetText() != "Draft text" {
			t.Errorf("expected buffer unchanged, got %q", input.GetText())
		}
		if input.GetCursorOffset() != 5 {
			t.Errorf("expected cursor unchanged, got %d", input.GetCursorOffset())
		}
	})

	t.Run("empty_tags_slice_leaves_buffer_unchanged", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Draft text")
		input.SetCursorOffset(5)

		InjectTagsIntoComposer(input, []string{})

		if input.GetText() != "Draft text" {
			t.Errorf("expected buffer unchanged, got %q", input.GetText())
		}
		if input.GetCursorOffset() != 5 {
			t.Errorf("expected cursor unchanged, got %d", input.GetCursorOffset())
		}
	})

	t.Run("all_whitespace_tags_leaves_buffer_unchanged", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Draft text")
		input.SetCursorOffset(5)

		InjectTagsIntoComposer(input, []string{"", "   ", "\t"})

		if input.GetText() != "Draft text" {
			t.Errorf("expected buffer unchanged, got %q", input.GetText())
		}
		if input.GetCursorOffset() != 5 {
			t.Errorf("expected cursor unchanged, got %d", input.GetCursorOffset())
		}
	})

	t.Run("negative_cursor_offset_handled_safely", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hello")
		input.SetCursorOffset(-5)

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panicked on negative cursor offset: %v", r)
			}
		}()
		InjectTagsIntoComposer(input, []string{"alice"})
		if !strings.Contains(input.GetText(), "@alice") {
			t.Errorf("expected tag to be injected, got %q", input.GetText())
		}
	})

	t.Run("overflow_cursor_offset_clamps_to_end", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hello")
		input.SetCursorOffset(99999)

		InjectTagsIntoComposer(input, []string{"alice"})

		if input.GetText() != "Hello@alice " {
			t.Errorf("expected 'Hello@alice ', got %q", input.GetText())
		}
	})

	t.Run("massive_buffer_non_destructive", func(t *testing.T) {
		bigPrefix := strings.Repeat("A", 50000)
		bigSuffix := strings.Repeat("B", 50000)
		input := mauview.NewInputArea()
		input.SetText(bigPrefix + bigSuffix)
		input.SetCursorOffset(50000)

		InjectTagsIntoComposer(input, []string{"alice"})

		expected := bigPrefix + "@alice " + bigSuffix
		if input.GetText() != expected {
			t.Errorf("massive buffer injection resulted in unexpected content length: %d vs %d",
				len(input.GetText()), len(expected))
		}
	})
}

// -----------------------------------------------------------------------------
// TS-8: Integration Tests (RoomView.OpenUserTagModal & onConfirm)
// -----------------------------------------------------------------------------

func makeTestRoomStoreForInjection() *store.RoomStore {
	meta := &database.Room{
		ID: "!test:example.com",
	}
	rs := store.NewRoomStore(nil, meta)
	aliceKey := "@alice:example.com"
	bobKey := "@bob:example.com"
	charlieKey := "@charlie:example.com"
	events := []*database.Event{
		{
			RowID:    1,
			Type:     event.StateMember.Type,
			StateKey: &aliceKey,
			Content:  []byte(`{"membership":"join","displayname":"Alice Adams"}`),
		},
		{
			RowID:    2,
			Type:     event.StateMember.Type,
			StateKey: &bobKey,
			Content:  []byte(`{"membership":"join","displayname":"Bob Brown"}`),
		},
		{
			RowID:    3,
			Type:     event.StateMember.Type,
			StateKey: &charlieKey,
			Content:  []byte(`{"membership":"join","displayname":"Charlie Clark"}`),
		},
	}
	rs.ApplyFullState(events, false)
	return rs
}

func TestRoomView_OpenUserTagModal_Integration(t *testing.T) {
	setupRoomViewWithMembers := func() (*MainView, *RoomView) {
		rv := &RoomView{
			input: mauview.NewInputArea(),
		}
		cfg := config.NewConfig()
		cfg.LoadKeybindings()
		mainView := &MainView{
			roomView:    mauview.NewBox(rv),
			currentRoom: rv,
			config:      cfg,
		}
		rv.parent = mainView
		rv.config = cfg
		rv.Room = makeTestRoomStoreForInjection()
		return mainView, rv
	}

	t.Run("launches_modal_and_sets_focus", func(t *testing.T) {
		mainView, rv := setupRoomViewWithMembers()

		if mainView.modal != nil {
			t.Fatalf("expected modal to be nil before launch")
		}

		rv.OpenUserTagModal()

		if mainView.modal == nil {
			t.Fatalf("expected modal to be shown on MainView")
		}
		tagModal, ok := mainView.modal.(*UserTagModal)
		if !ok {
			t.Fatalf("expected modal of type *UserTagModal, got %T", mainView.modal)
		}
		if tagModal.GetMatches() == nil || len(tagModal.GetMatches()) != 3 {
			t.Errorf("expected 3 member matches, got %d", len(tagModal.GetMatches()))
		}
	})

	t.Run("confirm_injects_tags_and_refocuses_composer", func(t *testing.T) {
		mainView, rv := setupRoomViewWithMembers()
		rv.input.SetText("Hello @")
		rv.input.SetCursorOffset(7)

		rv.OpenUserTagModal()

		tagModal := mainView.modal.(*UserTagModal)
		matches := tagModal.GetMatches()
		if len(matches) == 0 {
			t.Fatalf("expected at least 1 match")
		}
		expectedMember := matches[0]
		// Highlighted fallback confirms first matched member dynamically
		tagModal.Confirm()

		if mainView.modal != nil {
			t.Errorf("expected modal to be dismissed after confirm")
		}
		expectedText := "Hello @" + expectedMember.Displayname + " "
		if rv.input.GetText() != expectedText {
			t.Errorf("expected composer text %q, got %q", expectedText, rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != ComposerStringWidth(expectedText) {
			t.Errorf("expected cursor offset %d, got %d", ComposerStringWidth(expectedText), rv.input.GetCursorOffset())
		}
	})

	t.Run("confirm_with_filter_injects_specific_user", func(t *testing.T) {
		// Test filtering for Alice Adams via "Adams" (avoids fuzzy collision on Charlie Clark)
		mainView, rv := setupRoomViewWithMembers()
		rv.input.SetText("Hello @")
		rv.input.SetCursorOffset(7)

		rv.OpenUserTagModal()

		tagModal := mainView.modal.(*UserTagModal)
		tagModal.Filter("Adams")
		matches := tagModal.GetMatches()
		if len(matches) != 1 {
			t.Fatalf("expected exactly 1 match for query 'Adams', got %d", len(matches))
		}
		expectedMember := matches[0]
		if expectedMember.Displayname != "Alice Adams" {
			t.Fatalf("expected matched member to be Alice Adams, got %q", expectedMember.Displayname)
		}
		tagModal.Confirm()

		if mainView.modal != nil {
			t.Errorf("expected modal to be dismissed after confirm")
		}
		expectedText := "Hello @" + expectedMember.Displayname + " "
		if rv.input.GetText() != expectedText {
			t.Errorf("expected composer text %q, got %q", expectedText, rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != ComposerStringWidth(expectedText) {
			t.Errorf("expected cursor offset %d, got %d", ComposerStringWidth(expectedText), rv.input.GetCursorOffset())
		}

		// Also test filtering for Bob Brown via "Bob"
		mainViewBob, rvBob := setupRoomViewWithMembers()
		rvBob.input.SetText("Hey @")
		rvBob.input.SetCursorOffset(5)

		rvBob.OpenUserTagModal()

		tagModalBob := mainViewBob.modal.(*UserTagModal)
		tagModalBob.Filter("Bob")
		matchesBob := tagModalBob.GetMatches()
		if len(matchesBob) != 1 {
			t.Fatalf("expected exactly 1 match for query 'Bob', got %d", len(matchesBob))
		}
		expectedMemberBob := matchesBob[0]
		if expectedMemberBob.Displayname != "Bob Brown" {
			t.Fatalf("expected matched member to be Bob Brown, got %q", expectedMemberBob.Displayname)
		}
		tagModalBob.Confirm()

		if mainViewBob.modal != nil {
			t.Errorf("expected modal to be dismissed after confirm")
		}
		expectedTextBob := "Hey @" + expectedMemberBob.Displayname + " "
		if rvBob.input.GetText() != expectedTextBob {
			t.Errorf("expected composer text %q, got %q", expectedTextBob, rvBob.input.GetText())
		}
		if rvBob.input.GetCursorOffset() != ComposerStringWidth(expectedTextBob) {
			t.Errorf("expected cursor offset %d, got %d", ComposerStringWidth(expectedTextBob), rvBob.input.GetCursorOffset())
		}
	})

	t.Run("filter_alice_matches_candidates", func(t *testing.T) {
		mainView, rv := setupRoomViewWithMembers()
		rv.OpenUserTagModal()

		tagModal := mainView.modal.(*UserTagModal)
		tagModal.Filter("Alice")
		matches := tagModal.GetMatches()
		if len(matches) == 0 {
			t.Fatalf("expected matches for query 'Alice'")
		}
		foundAlice := false
		for _, m := range matches {
			if m.Displayname == "Alice Adams" {
				foundAlice = true
				break
			}
		}
		if !foundAlice {
			t.Errorf("expected Alice Adams to be in matches for query 'Alice'")
		}
	})

	t.Run("cancel_preserves_draft_and_refocuses_composer", func(t *testing.T) {
		mainView, rv := setupRoomViewWithMembers()
		rv.input.SetText("Hello @")
		rv.input.SetCursorOffset(7)

		rv.OpenUserTagModal()

		tagModal := mainView.modal.(*UserTagModal)
		tagModal.Cancel()

		if mainView.modal != nil {
			t.Errorf("expected modal to be dismissed after cancel")
		}
		if rv.input.GetText() != "Hello @" {
			t.Errorf("expected composer text to remain 'Hello @', got %q", rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != 7 {
			t.Errorf("expected cursor offset to remain 7, got %d", rv.input.GetCursorOffset())
		}
	})

	t.Run("nil_room_store_returns_safely", func(t *testing.T) {
		rv := &RoomView{
			input: mauview.NewInputArea(),
			Room:  nil, // nil Room store
		}
		mainView := &MainView{
			roomView:    mauview.NewBox(rv),
			currentRoom: rv,
			config:      config.NewConfig(),
		}
		rv.parent = mainView
		rv.config = mainView.config

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("OpenUserTagModal panicked on nil Room store: %v", r)
			}
		}()
		rv.OpenUserTagModal()

		if mainView.modal != nil {
			t.Errorf("expected modal to remain nil when Room is nil")
		}
	})

	t.Run("nil_parent_returns_cleanly", func(t *testing.T) {
		rv := &RoomView{
			input:  mauview.NewInputArea(),
			parent: nil,
			Room:   makeTestRoomStoreForInjection(),
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("OpenUserTagModal panicked on nil parent: %v", r)
			}
		}()
		rv.OpenUserTagModal()
	})

	t.Run("nil_input_returns_cleanly", func(t *testing.T) {
		rv := &RoomView{
			input:  nil,
			parent: &MainView{},
			Room:   makeTestRoomStoreForInjection(),
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("OpenUserTagModal panicked on nil input: %v", r)
			}
		}()
		rv.OpenUserTagModal()
	})

	t.Run("keybinding_tag_user_dispatches_modal", func(t *testing.T) {
		mainView, rv := setupRoomViewWithMembers()

		keyEvt := tcell.NewEventKey(tcell.KeyCtrlT, 20, tcell.ModCtrl)
		handled := rv.OnKeyEvent(keyEvt)

		if !handled {
			t.Errorf("expected 'tag_user' key event to be handled")
		}
		if mainView.modal == nil {
			t.Errorf("expected modal to be opened via keybinding Ctrl+t with rune 20")
		}
	})

	t.Run("keybinding_tag_user_zero_rune_dispatches_modal", func(t *testing.T) {
		mainView, rv := setupRoomViewWithMembers()

		keyEvt := tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl)
		handled := rv.OnKeyEvent(keyEvt)

		if !handled {
			t.Errorf("expected 'tag_user' key event to be handled with rune 0")
		}
		if mainView.modal == nil {
			t.Errorf("expected modal to be opened via keybinding Ctrl+t with rune 0")
		}
	})

	t.Run("nil_view_and_nil_config_safe", func(t *testing.T) {
		keyEvt := tcell.NewEventKey(tcell.KeyCtrlT, 20, tcell.ModCtrl)
		if (*RoomView)(nil).OnKeyEvent(keyEvt) != false {
			t.Errorf("expected nil RoomView to return false")
		}
		rvNoInput := &RoomView{input: nil}
		if rvNoInput.OnKeyEvent(keyEvt) != false {
			t.Errorf("expected RoomView with nil input to return false")
		}
		rvNoConfig := &RoomView{input: mauview.NewInputArea(), config: nil}
		if rvNoConfig.OnKeyEvent(keyEvt) != false {
			t.Errorf("expected RoomView with nil config to delegate to input")
		}
	})
}
