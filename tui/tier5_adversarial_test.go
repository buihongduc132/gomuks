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
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/rpc/store"
)

// -----------------------------------------------------------------------------
// Tier 5 Adversarial Test 1: SpliceTags Invalid UTF-8 Slicing Guard
// -----------------------------------------------------------------------------

func TestTier5_Adversarial_SpliceTags_InvalidUTF8(t *testing.T) {
	testCases := []struct {
		name         string
		text         string
		cursorOffset int
		tags         []string
	}{
		{
			name:         "Double_0xFF_Offset_1",
			text:         "\xff\xff",
			cursorOffset: 1,
			tags:         []string{"alice"},
		},
		{
			name:         "Double_0xFF_Offset_0",
			text:         "\xff\xff",
			cursorOffset: 0,
			tags:         []string{"alice"},
		},
		{
			name:         "Double_0xFF_Offset_2",
			text:         "\xff\xff",
			cursorOffset: 2,
			tags:         []string{"alice"},
		},
		{
			name:         "Double_0xFF_Offset_5_OutOfBounds",
			text:         "\xff\xff",
			cursorOffset: 5,
			tags:         []string{"alice"},
		},
		{
			name:         "Single_0x80_Offset_0",
			text:         "\x80",
			cursorOffset: 0,
			tags:         []string{"bob"},
		},
		{
			name:         "Single_0x80_Offset_1",
			text:         "\x80",
			cursorOffset: 1,
			tags:         []string{"bob"},
		},
		{
			name:         "Mixed_Invalid_Bytes_In_Middle",
			text:         "hello\xffworld\xfe",
			cursorOffset: 5,
			tags:         []string{"alice", "@bob"},
		},
		{
			name:         "Mixed_Invalid_Bytes_At_End",
			text:         "draft message \xff\xff",
			cursorOffset: 15,
			tags:         []string{"charlie"},
		},
		{
			name:         "Truncated_2Byte_UTF8",
			text:         "pre\xc3suf",
			cursorOffset: 3,
			tags:         []string{"alice"},
		},
		{
			name:         "Truncated_3Byte_UTF8",
			text:         "pre\xe2\x82suf",
			cursorOffset: 4,
			tags:         []string{"alice"},
		},
		{
			name:         "Truncated_4Byte_UTF8",
			text:         "pre\xf0\x9f\x98suf",
			cursorOffset: 5,
			tags:         []string{"alice"},
		},
		{
			name:         "Invalid_Byte_Followed_By_At_Trigger",
			text:         "\xff@",
			cursorOffset: 2,
			tags:         []string{"alice"},
		},
		{
			name:         "Lone_0xFF_Multiple_Tags",
			text:         "\xff",
			cursorOffset: 1,
			tags:         []string{"alice", "bob", "charlie"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("SpliceTags panicked on input text %q (len=%d) at offset %d: %v",
						tc.text, len(tc.text), tc.cursorOffset, r)
				}
			}()

			resText, resOffset := SpliceTags(tc.text, tc.cursorOffset, tc.tags)
			if resOffset < 0 {
				t.Errorf("expected non-negative cursor offset, got %d", resOffset)
			}
			for _, tag := range tc.tags {
				clean := strings.TrimPrefix(tag, "@")
				if !strings.Contains(resText, clean) {
					t.Errorf("expected result %q to contain tag %q", resText, clean)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Tier 5 Adversarial Test 2: UserTagModal Ctrl+A Keyboard Navigation
// -----------------------------------------------------------------------------

func TestTier5_Adversarial_UserTagModal_CtrlA(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{
			UserID:       "@alice:example.com",
			Displayname:  "Alice Adams",
			SearchString: "alice adams",
		},
		{
			UserID:       "@bob:example.com",
			Displayname:  "Bob Brown",
			SearchString: "bob brown",
		},
	}

	t.Run("CtrlA_With_Typed_Text", func(t *testing.T) {
		modal := NewUserTagModal(nil, members, nil)

		// Type characters 'a', 'l', 'i'
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModNone))

		if modal.search.GetCursorOffset() != 3 {
			t.Fatalf("expected cursor offset 3 after typing 'ali', got %d", modal.search.GetCursorOffset())
		}
		if modal.search.GetText() != "ali" {
			t.Fatalf("expected search text 'ali', got %q", modal.search.GetText())
		}

		// Dispatch Ctrl+A
		ctrlA := tcell.NewEventKey(tcell.KeyCtrlA, 1, tcell.ModCtrl)
		handled := modal.OnKeyEvent(ctrlA)

		if !handled {
			t.Errorf("expected OnKeyEvent to return true for KeyCtrlA")
		}
		if modal.search.GetCursorOffset() != 0 {
			t.Errorf("expected cursor offset to be reset to 0 after Ctrl+A, got %d", modal.search.GetCursorOffset())
		}
		if modal.search.GetText() != "ali" {
			t.Errorf("search text should remain 'ali' after Ctrl+A, got %q", modal.search.GetText())
		}
	})

	t.Run("CtrlA_On_Empty_Query", func(t *testing.T) {
		modal := NewUserTagModal(nil, members, nil)
		ctrlA := tcell.NewEventKey(tcell.KeyCtrlA, 1, tcell.ModCtrl)
		handled := modal.OnKeyEvent(ctrlA)

		if !handled {
			t.Errorf("expected OnKeyEvent to return true for KeyCtrlA on empty query")
		}
		if modal.search.GetCursorOffset() != 0 {
			t.Errorf("expected cursor offset 0, got %d", modal.search.GetCursorOffset())
		}
	})

	t.Run("CtrlA_Repeated_Invocations", func(t *testing.T) {
		modal := NewUserTagModal(nil, members, nil)
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))

		ctrlA := tcell.NewEventKey(tcell.KeyCtrlA, 1, tcell.ModCtrl)
		for i := 0; i < 10; i++ {
			if !modal.OnKeyEvent(ctrlA) {
				t.Fatalf("iteration %d: expected KeyCtrlA to be handled", i)
			}
			if modal.search.GetCursorOffset() != 0 {
				t.Fatalf("iteration %d: expected cursor offset 0, got %d", i, modal.search.GetCursorOffset())
			}
		}
	})
}

// -----------------------------------------------------------------------------
// Tier 5 Adversarial Test 3: UserTagModal Concurrent Focus and Blur
// -----------------------------------------------------------------------------

func TestTier5_Adversarial_UserTagModal_ConcurrentFocusBlur(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{
			UserID:       "@alice:example.com",
			Displayname:  "Alice Adams",
			SearchString: "alice adams",
		},
		{
			UserID:       "@bob:example.com",
			Displayname:  "Bob Brown",
			SearchString: "bob brown",
		},
	}

	modal := NewUserTagModal(nil, members, nil)

	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if (gid+i)%2 == 0 {
					modal.Focus()
				} else {
					modal.Blur()
				}
				// Also stress rapid back-to-back transitions
				modal.Focus()
				modal.Blur()
			}
		}(g)
	}

	wg.Wait()

	if modal.container == nil || modal.search == nil {
		t.Fatal("modal container or search widget unexpectedly nil after concurrency test")
	}
}

// -----------------------------------------------------------------------------
// Tier 5 Adversarial Test 4: Extreme Unicode and Indentation Splicing
// -----------------------------------------------------------------------------

func TestTier5_Adversarial_ExtremeUnicodeAndIndentation(t *testing.T) {
	t.Run("CJK_Width_And_Splicing", func(t *testing.T) {
		text := "こんにちは世界\nMatrixへようこそ"
		// "こんにちは世界" has 7 characters * 2 width = 14
		// '\n' has width 1
		// "Matrixへようこそ": "Matrix" (6) + "へようこそ" (5*2=10) = 16
		// Total width = 14 + 1 + 16 = 31
		w := ComposerStringWidth(text)
		if w != 31 {
			t.Errorf("expected ComposerStringWidth 31 for CJK multiline text, got %d", w)
		}

		// Splice tags right after "こんにちは世界\n" (width 15)
		spliced, newOffset := SpliceTags(text, 15, []string{"alice"})
		expectedPrefix := "こんにちは世界\n@alice "
		if !strings.HasPrefix(spliced, expectedPrefix) {
			t.Errorf("expected spliced prefix %q, got %q", expectedPrefix, spliced)
		}
		expectedSuffix := "Matrixへようこそ"
		if !strings.HasSuffix(spliced, expectedSuffix) {
			t.Errorf("expected spliced suffix %q, got %q", expectedSuffix, spliced)
		}
		if newOffset != ComposerStringWidth(expectedPrefix) {
			t.Errorf("expected new cursor offset %d, got %d", ComposerStringWidth(expectedPrefix), newOffset)
		}
		if !utf8.ValidString(spliced) {
			t.Errorf("spliced text is not valid UTF-8: %q", spliced)
		}
	})

	t.Run("ZWJ_Emoji_And_SkinTone", func(t *testing.T) {
		text := "Hello 👨‍👩‍👧‍👦 family 👍🏽 done"
		w := ComposerStringWidth(text)
		if w <= 0 {
			t.Errorf("expected positive string width, got %d", w)
		}

		spliced, newOffset := SpliceTags(text, w, []string{"bob"})
		if !strings.HasSuffix(spliced, "@bob ") {
			t.Errorf("expected spliced to end with '@bob ', got %q", spliced)
		}
		if !strings.Contains(spliced, "👨‍👩‍👧‍👦") || !strings.Contains(spliced, "👍🏽") {
			t.Errorf("spliced text corrupted emoji sequences: %q", spliced)
		}
		if newOffset != ComposerStringWidth(spliced) {
			t.Errorf("expected cursor offset at end of text %d, got %d", ComposerStringWidth(spliced), newOffset)
		}
		if !utf8.ValidString(spliced) {
			t.Errorf("spliced text is not valid UTF-8: %q", spliced)
		}
	})

	t.Run("MultiLine_Mixed_Tabs_Spaces_Indentation", func(t *testing.T) {
		draft := "\t\tdef my_function():\n    # 4 spaces comment\n\t  mixed indentation\n\t\t\treturn True"
		w := ComposerStringWidth(draft)
		if w <= 0 {
			t.Errorf("expected positive string width, got %d", w)
		}

		// Splice tags in the middle: right after "def my_function():\n"
		beforeTarget := "\t\tdef my_function():\n"
		offset := ComposerStringWidth(beforeTarget)

		spliced, newOffset := SpliceTags(draft, offset, []string{"charlie"})

		// Ensure prefix indentation and suffix indentation remain completely intact
		expectedPrefix := beforeTarget + "@charlie "
		if !strings.HasPrefix(spliced, expectedPrefix) {
			t.Errorf("expected spliced text to begin with %q, got %q", expectedPrefix, spliced)
		}

		expectedSuffix := "    # 4 spaces comment\n\t  mixed indentation\n\t\t\treturn True"
		if !strings.HasSuffix(spliced, expectedSuffix) {
			t.Errorf("expected spliced text to preserve suffix indentation %q, got %q", expectedSuffix, spliced)
		}

		if newOffset != ComposerStringWidth(expectedPrefix) {
			t.Errorf("expected cursor offset %d, got %d", ComposerStringWidth(expectedPrefix), newOffset)
		}
	})

	t.Run("Consecutive_Newlines_And_Whitespace", func(t *testing.T) {
		text := "\n\n\n   \t\t   \n\n"
		spliced, _ := SpliceTags(text, 3, []string{"alice"})
		if !strings.Contains(spliced, "@alice ") {
			t.Errorf("expected spliced text to contain '@alice ', got %q", spliced)
		}
	})

	t.Run("ComposerSubstringBefore_Rune_Boundary_Safety", func(t *testing.T) {
		cjk := "日本語"
		// Each character has width 2.
		// If limit is 1, it should return "" because '日' has width 2 > 1.
		if sub := ComposerSubstringBefore(cjk, 1); sub != "" {
			t.Errorf("expected empty string for width 1 on double-width rune, got %q", sub)
		}
		if sub := ComposerSubstringBefore(cjk, 2); sub != "日" {
			t.Errorf("expected '日' for width 2, got %q", sub)
		}
		if sub := ComposerSubstringBefore(cjk, 3); sub != "日" {
			t.Errorf("expected '日' for width 3, got %q", sub)
		}
		if sub := ComposerSubstringBefore(cjk, 4); sub != "日本" {
			t.Errorf("expected '日本' for width 4, got %q", sub)
		}
	})
}

// -----------------------------------------------------------------------------
// Tier 5 Adversarial Test 5: Modal Query Metacharacters
// -----------------------------------------------------------------------------

func TestTier5_Adversarial_ModalQueryMetachars(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{
			UserID:       "@alice(admin):example.com",
			Displayname:  "Alice (Admin)",
			SearchString: "alice (admin)",
		},
		{
			UserID:       "@bob[dev]:example.com",
			Displayname:  "Bob [Dev]",
			SearchString: "bob [dev]",
		},
		{
			UserID:       "@charlie*star:example.com",
			Displayname:  "Charlie *Star*",
			SearchString: "charlie *star*",
		},
		{
			UserID:       "@david+plus:example.com",
			Displayname:  "David +1",
			SearchString: "david +1",
		},
		{
			UserID:       "@eve?question:example.com",
			Displayname:  "Eve ?Query?",
			SearchString: "eve ?query?",
		},
		{
			UserID:       "@frank^top:example.com",
			Displayname:  "Frank ^Top^",
			SearchString: "frank ^top^",
		},
	}

	modal := NewUserTagModal(nil, members, nil)

	metacharQueries := []struct {
		query          string
		expectedMatch  id.UserID
		shouldHaveSome bool
	}{
		{query: "(", expectedMatch: "@alice(admin):example.com", shouldHaveSome: true},
		{query: ")", expectedMatch: "@alice(admin):example.com", shouldHaveSome: true},
		{query: "(admin)", expectedMatch: "@alice(admin):example.com", shouldHaveSome: true},
		{query: "[", expectedMatch: "@bob[dev]:example.com", shouldHaveSome: true},
		{query: "]", expectedMatch: "@bob[dev]:example.com", shouldHaveSome: true},
		{query: "[dev]", expectedMatch: "@bob[dev]:example.com", shouldHaveSome: true},
		{query: "*", expectedMatch: "@charlie*star:example.com", shouldHaveSome: true},
		{query: "*star*", expectedMatch: "@charlie*star:example.com", shouldHaveSome: true},
		{query: "+", expectedMatch: "@david+plus:example.com", shouldHaveSome: true},
		{query: "+1", expectedMatch: "@david+plus:example.com", shouldHaveSome: true},
		{query: "?", expectedMatch: "@eve?question:example.com", shouldHaveSome: true},
		{query: "^", expectedMatch: "@frank^top:example.com", shouldHaveSome: true},
		{query: ".*", expectedMatch: "", shouldHaveSome: false},
		{query: "(\\d+)", expectedMatch: "", shouldHaveSome: false},
		{query: "[a-z]+", expectedMatch: "", shouldHaveSome: false},
		{query: "{1,3}", expectedMatch: "", shouldHaveSome: false},
		{query: "\\", expectedMatch: "", shouldHaveSome: false},
		{query: "$", expectedMatch: "", shouldHaveSome: false},
		{query: "()[]{}*+?^$\\.", expectedMatch: "", shouldHaveSome: false},
	}

	for _, tc := range metacharQueries {
		t.Run("Query_"+tc.query, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("modal.Filter panicked on query %q: %v", tc.query, r)
				}
			}()

			modal.Filter(tc.query)

			modal.lock.RLock()
			defer modal.lock.RUnlock()

			if tc.shouldHaveSome {
				found := false
				for _, match := range modal.matches {
					if match.UserID == tc.expectedMatch {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected matches for query %q to contain %s, got %d matches",
						tc.query, tc.expectedMatch, len(modal.matches))
				}
			}
		})
	}
}
