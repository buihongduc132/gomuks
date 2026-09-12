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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gdamore/tcell/v2"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/rpc/store"
)

func makeTestMembers() []*store.AutocompleteMemberEntry {
	return []*store.AutocompleteMemberEntry{
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
		{
			UserID:       "@charlie:example.com",
			Displayname:  "Charlie Clark",
			SearchString: "charlie clark",
		},
	}
}

func TestUserTagModal_InitialPopulate(t *testing.T) {
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, nil)

	if len(modal.matches) != 3 {
		t.Fatalf("expected 3 initial matches, got %d", len(modal.matches))
	}
	if modal.selected != 0 {
		t.Errorf("expected selected index 0, got %d", modal.selected)
	}
	if modal.GetSelectedIndex() != 0 {
		t.Errorf("expected GetSelectedIndex() 0, got %d", modal.GetSelectedIndex())
	}
	if len(modal.selectedUsers) != 0 {
		t.Errorf("expected 0 selected users initially, got %d", len(modal.selectedUsers))
	}
	if len(modal.selectedOrder) != 0 {
		t.Errorf("expected empty selectedOrder initially, got %d", len(modal.selectedOrder))
	}

	rendered := modal.GetRenderedText()
	if !strings.Contains(rendered, "[ ] Alice Adams (@alice:example.com)") {
		t.Errorf("rendered text missing Alice Adams unselected: %q", rendered)
	}
	if !strings.Contains(rendered, "[ ] Bob Brown (@bob:example.com)") {
		t.Errorf("rendered text missing Bob Brown unselected: %q", rendered)
	}
	if !strings.Contains(rendered, "[ ] Charlie Clark (@charlie:example.com)") {
		t.Errorf("rendered text missing Charlie Clark unselected: %q", rendered)
	}

	highlights := modal.results.GetHighlights()
	if len(highlights) != 1 || highlights[0] != "0" {
		t.Errorf("expected highlights ['0'], got %v", highlights)
	}
}

func TestUserTagModal_SearchFilterAndRestore(t *testing.T) {
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, nil)

	// Filter down to Bob
	modal.filter("bob")
	if len(modal.matches) != 1 {
		t.Fatalf("expected 1 match for 'bob', got %d", len(modal.matches))
	}
	if modal.matches[0].UserID != "@bob:example.com" {
		t.Errorf("expected match @bob:example.com, got %s", modal.matches[0].UserID)
	}

	rendered := modal.GetRenderedText()
	if !strings.Contains(rendered, "Bob Brown") {
		t.Errorf("expected rendered text to contain Bob Brown, got: %q", rendered)
	}
	if strings.Contains(rendered, "Alice Adams") {
		t.Errorf("expected rendered text NOT to contain Alice Adams when filtered, got: %q", rendered)
	}

	// Restore all via empty filter
	modal.filter("")
	if len(modal.matches) != 3 {
		t.Fatalf("expected 3 matches after restore, got %d", len(modal.matches))
	}
	renderedRestored := modal.GetRenderedText()
	if !strings.Contains(renderedRestored, "Alice Adams") ||
		!strings.Contains(renderedRestored, "Bob Brown") ||
		!strings.Contains(renderedRestored, "Charlie Clark") {
		t.Errorf("expected all 3 members restored, got: %q", renderedRestored)
	}

	// Also test ChangeHandler wrapper
	modal.ChangeHandler("charlie")
	if len(modal.matches) != 1 || modal.matches[0].UserID != "@charlie:example.com" {
		t.Errorf("ChangeHandler('charlie') failed to filter correctly")
	}
}

func TestUserTagModal_MultiSelect_TabToggle(t *testing.T) {
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, nil)

	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)

	// Tab 1: Select Alice (index 0) and advance highlight to index 1 (Bob)
	handled := modal.OnKeyEvent(tabEvt)
	if !handled {
		t.Error("expected Tab key to be handled")
	}
	if _, ok := modal.selectedUsers["@alice:example.com"]; !ok {
		t.Errorf("expected @alice:example.com to be selected")
	}
	if modal.selected != 1 {
		t.Errorf("expected highlight to advance to index 1, got %d", modal.selected)
	}
	rendered1 := modal.GetRenderedText()
	if !strings.Contains(rendered1, "[x] Alice Adams") {
		t.Errorf("expected [x] Alice Adams in rendered text, got: %q", rendered1)
	}
	if !strings.Contains(rendered1, "[ ] Bob Brown") {
		t.Errorf("expected [ ] Bob Brown in rendered text, got: %q", rendered1)
	}

	// Tab 2: Select Bob (index 1) and advance highlight to index 2 (Charlie)
	modal.OnKeyEvent(tabEvt)
	if _, ok := modal.selectedUsers["@bob:example.com"]; !ok {
		t.Errorf("expected @bob:example.com to be selected")
	}
	if modal.selected != 2 {
		t.Errorf("expected highlight to advance to index 2, got %d", modal.selected)
	}
	if len(modal.selectedOrder) != 2 {
		t.Fatalf("expected 2 items in selectedOrder, got %d", len(modal.selectedOrder))
	}
	if modal.selectedOrder[0] != "@alice:example.com" || modal.selectedOrder[1] != "@bob:example.com" {
		t.Errorf("unexpected selectedOrder: %v", modal.selectedOrder)
	}

	// Navigate up to Bob without modifying checkboxes
	upEvt := tcell.NewEventKey(tcell.KeyUp, 0, 0)
	modal.OnKeyEvent(upEvt)
	if modal.selected != 1 {
		t.Errorf("expected highlight at index 1 after Up arrow, got %d", modal.selected)
	}
	if len(modal.selectedUsers) != 2 {
		t.Errorf("Up arrow must not modify selections, got %d selected", len(modal.selectedUsers))
	}

	// Tab on Bob again to untoggle
	modal.OnKeyEvent(tabEvt)
	if _, ok := modal.selectedUsers["@bob:example.com"]; ok {
		t.Errorf("expected @bob:example.com to be untoggled")
	}
	if len(modal.selectedOrder) != 1 || modal.selectedOrder[0] != "@alice:example.com" {
		t.Errorf("expected only Alice in selectedOrder after untoggle, got %v", modal.selectedOrder)
	}
	renderedUntoggled := modal.GetRenderedText()
	if !strings.Contains(renderedUntoggled, "[ ] Bob Brown") {
		t.Errorf("expected [ ] Bob Brown after untoggle, got: %q", renderedUntoggled)
	}
	if !strings.Contains(renderedUntoggled, "[x] Alice Adams") {
		t.Errorf("expected [x] Alice Adams to remain selected, got: %q", renderedUntoggled)
	}
}

func TestUserTagModal_SelectionSurvivesFilter(t *testing.T) {
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, nil)

	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)

	// Select Alice
	modal.OnKeyEvent(tabEvt)
	if _, ok := modal.selectedUsers["@alice:example.com"]; !ok {
		t.Fatalf("failed to select Alice")
	}

	// Filter down to Charlie (hides Alice)
	modal.filter("charlie")
	if len(modal.matches) != 1 || modal.matches[0].UserID != "@charlie:example.com" {
		t.Fatalf("expected 1 match for charlie, got %d", len(modal.matches))
	}

	// Alice MUST survive the filter in selectedUsers and selectedOrder
	if _, ok := modal.selectedUsers["@alice:example.com"]; !ok {
		t.Errorf("selection for Alice was lost during search filtering")
	}
	if len(modal.selectedOrder) != 1 || modal.selectedOrder[0] != "@alice:example.com" {
		t.Errorf("selectedOrder was corrupted during search filtering: %v", modal.selectedOrder)
	}

	// Select Charlie while filtered
	modal.OnKeyEvent(tabEvt)
	if _, ok := modal.selectedUsers["@charlie:example.com"]; !ok {
		t.Errorf("failed to select Charlie while filtered")
	}

	// Clear filter and verify both Alice and Charlie remain selected in order
	modal.filter("")
	if len(modal.matches) != 3 {
		t.Fatalf("expected 3 matches after filter reset, got %d", len(modal.matches))
	}
	if len(modal.selectedUsers) != 2 {
		t.Errorf("expected 2 selected users, got %d", len(modal.selectedUsers))
	}
	order := modal.GetSelectedOrder()
	if len(order) != 2 || order[0] != "@alice:example.com" || order[1] != "@charlie:example.com" {
		t.Errorf("expected order [@alice, @charlie], got %v", order)
	}

	rendered := modal.GetRenderedText()
	if !strings.Contains(rendered, "[x] Alice Adams") ||
		!strings.Contains(rendered, "[ ] Bob Brown") ||
		!strings.Contains(rendered, "[x] Charlie Clark") {
		t.Errorf("rendered text does not show surviving selections: %q", rendered)
	}
}

func TestUserTagModal_SpaceInQueryDoesNotToggle(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{
			UserID:       "@alice:example.com",
			Displayname:  "Alice Wonder",
			SearchString: "alice wonder",
		},
		{
			UserID:       "@bob:example.com",
			Displayname:  "Bob Builder",
			SearchString: "bob builder",
		},
	}
	modal := NewUserTagModal(nil, members, nil)

	// Send Space key
	spaceEvt := tcell.NewEventKey(tcell.KeyRune, ' ', 0)
	handled := modal.OnKeyEvent(spaceEvt)
	if !handled {
		t.Error("expected Space key event to be handled by search InputArea")
	}

	if len(modal.selectedUsers) != 0 {
		t.Fatalf("typing Space must NOT toggle selections, got %d selected", len(modal.selectedUsers))
	}
	if modal.search.GetText() != " " {
		t.Errorf("expected search input to contain ' ', got %q", modal.search.GetText())
	}

	// Type "Alice Wonder" with spaces
	modal.search.SetText("")
	for _, r := range "Alice Wonder" {
		evt := tcell.NewEventKey(tcell.KeyRune, r, 0)
		modal.OnKeyEvent(evt)
	}

	if len(modal.selectedUsers) != 0 {
		t.Errorf("typing query with spaces must not toggle selections, got %d", len(modal.selectedUsers))
	}
	if modal.search.GetText() != "Alice Wonder" {
		t.Errorf("expected search text 'Alice Wonder', got %q", modal.search.GetText())
	}
	if len(modal.matches) != 1 || modal.matches[0].UserID != "@alice:example.com" {
		t.Errorf("expected 1 match for Alice Wonder, got %d", len(modal.matches))
	}
}

func TestUserTagModal_DualConfirmation_Toggled(t *testing.T) {
	members := makeTestMembers()
	var confirmed []string
	modal := NewUserTagModal(nil, members, func(selectedUsers []string) {
		confirmed = selectedUsers
	})

	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
	// Toggle Alice and Bob
	modal.OnKeyEvent(tabEvt) // selects Alice
	modal.OnKeyEvent(tabEvt) // selects Bob

	enterEvt := tcell.NewEventKey(tcell.KeyEnter, 13, 0)
	handled := modal.OnKeyEvent(enterEvt)
	if !handled {
		t.Error("expected Enter key to be handled")
	}

	if !modal.IsDismissed() {
		t.Error("modal should be marked as dismissed on Enter")
	}
	if len(confirmed) != 2 {
		t.Fatalf("expected 2 confirmed users, got %d", len(confirmed))
	}
	if confirmed[0] != "Alice Adams" || confirmed[1] != "Bob Brown" {
		t.Errorf("expected ['Alice Adams', 'Bob Brown'], got %v", confirmed)
	}
}

func TestUserTagModal_DualConfirmation_HighlightedFallback(t *testing.T) {
	members := makeTestMembers()
	var confirmed []string
	modal := NewUserTagModal(nil, members, func(selectedUsers []string) {
		confirmed = selectedUsers
	})

	// 0 items toggled. Down arrow to highlight Bob (index 1).
	downEvt := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	modal.OnKeyEvent(downEvt)
	if modal.selected != 1 {
		t.Fatalf("expected highlight on Bob (index 1), got %d", modal.selected)
	}

	enterEvt := tcell.NewEventKey(tcell.KeyEnter, 13, 0)
	modal.OnKeyEvent(enterEvt)

	if !modal.IsDismissed() {
		t.Error("modal should be dismissed on Enter")
	}
	if len(confirmed) != 1 {
		t.Fatalf("expected 1 confirmed user via fallback, got %d", len(confirmed))
	}
	if confirmed[0] != "Bob Brown" {
		t.Errorf("expected fallback confirmation for 'Bob Brown', got %q", confirmed[0])
	}
}

func TestUserTagModal_Cancel_Escape(t *testing.T) {
	members := makeTestMembers()
	callCount := 0
	modal := NewUserTagModal(nil, members, func(selectedUsers []string) {
		callCount++
	})

	// Toggle Alice
	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
	modal.OnKeyEvent(tabEvt)

	// Press Escape
	escEvt := tcell.NewEventKey(tcell.KeyEscape, 27, 0)
	handled := modal.OnKeyEvent(escEvt)
	if !handled {
		t.Error("expected Escape key to be handled")
	}

	if !modal.IsDismissed() {
		t.Error("modal should be marked as dismissed on Escape")
	}
	if callCount != 0 {
		t.Errorf("onConfirm must NOT be called on Escape, was called %d times", callCount)
	}
}

func TestUserTagModal_LargeMemberList_Capped(t *testing.T) {
	const total = 250
	largeMembers := make([]*store.AutocompleteMemberEntry, total)
	for i := 0; i < total; i++ {
		largeMembers[i] = &store.AutocompleteMemberEntry{
			UserID:       id.UserID(fmt.Sprintf("@user%d:example.com", i)),
			Displayname:  fmt.Sprintf("User Number %d", i),
			SearchString: fmt.Sprintf("user number %d", i),
		}
	}

	modal := NewUserTagModal(nil, largeMembers, nil)
	if len(modal.matches) != total {
		t.Fatalf("expected %d matches, got %d", total, len(modal.matches))
	}

	// Verify rendered text has at most 100 rows
	lines := strings.Split(strings.TrimRight(modal.GetRenderedText(), "\n"), "\n")
	if len(lines) != 100 {
		t.Errorf("expected rendered text to be capped at 100 rows, got %d", len(lines))
	}

	// Verify Down arrow cycles properly within [0, 100)
	downEvt := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	for i := 0; i < 150; i++ {
		modal.OnKeyEvent(downEvt)
		if modal.selected < 0 || modal.selected >= 100 {
			t.Fatalf("selected index %d out of bounds [0, 100) at step %d", modal.selected, i)
		}
	}
}

func TestUserTagModal_NilAndEmptyMembersSafety(t *testing.T) {
	// 1. Nil members slice
	modalNil := NewUserTagModal(nil, nil, nil)
	if modalNil == nil {
		t.Fatal("expected non-nil modal for nil members")
	}
	if len(modalNil.matches) != 0 {
		t.Errorf("expected 0 matches for nil members, got %d", len(modalNil.matches))
	}

	// Keys on nil modal must not panic
	events := []tcell.Event{
		tcell.NewEventKey(tcell.KeyTab, 9, 0),
		tcell.NewEventKey(tcell.KeyDown, 0, 0),
		tcell.NewEventKey(tcell.KeyUp, 0, 0),
		tcell.NewEventKey(tcell.KeyBacktab, 0, 0),
		tcell.NewEventKey(tcell.KeyRune, ' ', 0),
		tcell.NewEventKey(tcell.KeyEnter, 13, 0),
		tcell.NewEventKey(tcell.KeyEscape, 27, 0),
	}
	for _, evt := range events {
		modalNil.OnKeyEvent(evt.(mauview.KeyEvent))
	}

	// 2. Empty members slice
	modalEmpty := NewUserTagModal(nil, []*store.AutocompleteMemberEntry{}, nil)
	if len(modalEmpty.matches) != 0 {
		t.Errorf("expected 0 matches for empty members, got %d", len(modalEmpty.matches))
	}
	modalEmpty.filter("test")
	modalEmpty.toggleCurrent()
	modalEmpty.confirm()

	// 3. Slice containing nil entries
	dirtyMembers := []*store.AutocompleteMemberEntry{
		nil,
		{UserID: "@valid:example.com", Displayname: "Valid User"},
		nil,
	}
	modalDirty := NewUserTagModal(nil, dirtyMembers, nil)
	if len(modalDirty.matches) != 1 {
		t.Errorf("expected 1 valid match ignoring nil entries, got %d", len(modalDirty.matches))
	}
	renderedDirty := modalDirty.GetRenderedText()
	if !strings.Contains(renderedDirty, "Valid User") {
		t.Errorf("expected Valid User in rendered text, got: %q", renderedDirty)
	}

	// 4. Convenience helper NewUserTagModalFromRoom with nil room
	modalFromNilRoom := NewUserTagModalFromRoom(nil, nil, nil)
	if modalFromNilRoom == nil {
		t.Fatal("expected non-nil modal from nil room")
	}
	if len(modalFromNilRoom.matches) != 0 {
		t.Errorf("expected 0 matches from nil room, got %d", len(modalFromNilRoom.matches))
	}

	// 5. Focus and Blur safety
	modalDirty.Focus()
	modalDirty.Blur()
}

func TestUserTagModal_FocusRestoration(t *testing.T) {
	rv := &RoomView{
		input: mauview.NewInputArea(),
	}
	mainView := &MainView{
		roomView:    mauview.NewBox(rv),
		currentRoom: rv,
	}

	members := makeTestMembers()
	modal := NewUserTagModal(mainView, members, nil)
	mainView.ShowModal(modal)

	if mainView.modal == nil {
		t.Fatalf("modal should be shown")
	}

	// Confirm closes modal and refocuses roomView
	modal.confirm()
	if mainView.modal != nil {
		t.Errorf("modal should be dismissed after confirm")
	}
	if mainView.focused != mainView.roomView {
		t.Errorf("expected focused to be roomView, got %v", mainView.focused)
	}

	// Verify composer can receive keystroke immediately
	rv.input.TypeRune('h')
	if rv.input.GetText() != "h" {
		t.Errorf("expected composer to receive 'h', got %q", rv.input.GetText())
	}
}

func TestUserTagModal_DisplayNameEscaping_Brackets(t *testing.T) {
	bracketMembers := []*store.AutocompleteMemberEntry{
		{
			UserID:      "@admin:example.com",
			Displayname: "Alice [Admin] [VIP]",
		},
	}
	modal := NewUserTagModal(nil, bracketMembers, nil)
	rendered := modal.GetRenderedText()
	escapedName := mauview.Escape("Alice [Admin] [VIP]")
	if !strings.Contains(rendered, escapedName) {
		t.Errorf("expected escaped display name %q in rendered text, got %q", escapedName, rendered)
	}
	// Region text retrieval should be safe
	regionText := modal.results.GetRegionText("0")
	if !strings.Contains(regionText, escapedName) {
		t.Errorf("expected region text to contain escaped bracket name %q, got %q", escapedName, regionText)
	}
}

func TestUserTagModal_NilFocusRestoration(t *testing.T) {
	t.Run("Confirm_NilInput", func(t *testing.T) {
		rv := &RoomView{input: nil}
		mainView := &MainView{
			roomView:    mauview.NewBox(rv),
			currentRoom: rv,
		}
		called := false
		modal := NewUserTagModal(mainView, makeTestMembers(), func(tags []string) {
			called = true
		})
		mainView.ShowModal(modal)

		// Must not panic on Confirm
		modal.Confirm()
		if !called {
			t.Errorf("expected onConfirm to be called")
		}
		if !modal.IsDismissed() {
			t.Errorf("expected modal to be dismissed")
		}
		if mainView.modal != nil {
			t.Errorf("expected mainView.modal to be nil")
		}
	})

	t.Run("Cancel_NilInput", func(t *testing.T) {
		rv := &RoomView{input: nil}
		mainView := &MainView{
			roomView:    mauview.NewBox(rv),
			currentRoom: rv,
		}
		called := false
		modal := NewUserTagModal(mainView, makeTestMembers(), func(tags []string) {
			called = true
		})
		mainView.ShowModal(modal)

		// Must not panic on Cancel
		modal.Cancel()
		if called {
			t.Errorf("expected onConfirm NOT to be called on Cancel")
		}
		if !modal.IsDismissed() {
			t.Errorf("expected modal to be dismissed")
		}
		if mainView.modal != nil {
			t.Errorf("expected mainView.modal to be nil")
		}
	})

	t.Run("Confirm_NilCurrentRoom", func(t *testing.T) {
		mainView := &MainView{
			currentRoom: nil,
		}
		called := false
		modal := NewUserTagModal(mainView, makeTestMembers(), func(tags []string) {
			called = true
		})
		mainView.ShowModal(modal)

		modal.Confirm()
		if !called {
			t.Errorf("expected onConfirm to be called")
		}
		if !modal.IsDismissed() {
			t.Errorf("expected modal to be dismissed")
		}
	})

	t.Run("Cancel_NilCurrentRoom", func(t *testing.T) {
		mainView := &MainView{
			currentRoom: nil,
		}
		modal := NewUserTagModal(mainView, makeTestMembers(), nil)
		mainView.ShowModal(modal)

		modal.Cancel()
		if !modal.IsDismissed() {
			t.Errorf("expected modal to be dismissed")
		}
	})

	t.Run("Confirm_NilParent", func(t *testing.T) {
		called := false
		modal := NewUserTagModal(nil, makeTestMembers(), func(tags []string) {
			called = true
		})
		modal.Confirm()
		if !called {
			t.Errorf("expected onConfirm to be called")
		}
		if !modal.IsDismissed() {
			t.Errorf("expected modal to be dismissed")
		}
	})

	t.Run("Cancel_NilParent", func(t *testing.T) {
		modal := NewUserTagModal(nil, makeTestMembers(), nil)
		modal.Cancel()
		if !modal.IsDismissed() {
			t.Errorf("expected modal to be dismissed")
		}
	})
}

func TestUserTagModal_Idempotent_DoubleConfirm(t *testing.T) {
	calls := 0
	var receivedTags []string
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, func(tags []string) {
		calls++
		receivedTags = tags
	})

	modal.ToggleCurrent() // select Alice

	modal.Confirm()
	if calls != 1 {
		t.Fatalf("expected 1 call after first Confirm(), got %d", calls)
	}
	if len(receivedTags) != 1 || receivedTags[0] != "Alice Adams" {
		t.Fatalf("unexpected tags: %v", receivedTags)
	}

	// Repeated programmatic calls
	modal.Confirm()
	modal.Confirm()
	if calls != 1 {
		t.Errorf("expected onConfirm to be called exactly once across multiple Confirm() calls, got %d", calls)
	}

	// Repeated key events after dismissal
	enterEvt := tcell.NewEventKey(tcell.KeyEnter, 13, 0)
	handled := modal.OnKeyEvent(enterEvt)
	if handled {
		t.Errorf("expected OnKeyEvent to return false after dismissal, got true")
	}
	if calls != 1 {
		t.Errorf("expected onConfirm count to remain 1 after key events on dismissed modal, got %d", calls)
	}

	// Other key events also fall through
	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
	if modal.OnKeyEvent(tabEvt) {
		t.Errorf("expected Tab to return false after modal dismissal")
	}
}

func TestUserTagModal_Idempotent_ConfirmAfterCancel(t *testing.T) {
	calls := 0
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, func(tags []string) {
		calls++
	})

	modal.Cancel()
	if !modal.IsDismissed() {
		t.Fatalf("expected modal to be dismissed after Cancel()")
	}

	modal.Confirm()
	if calls != 0 {
		t.Errorf("expected onConfirm NOT to be called after Cancel(), got %d calls", calls)
	}

	// Also test Cancel after Confirm
	calls2 := 0
	modal2 := NewUserTagModal(nil, members, func(tags []string) {
		calls2++
	})
	modal2.Confirm()
	if calls2 != 1 {
		t.Fatalf("expected 1 call after Confirm(), got %d", calls2)
	}
	modal2.Cancel()
	if calls2 != 1 {
		t.Errorf("expected call count to remain 1 after Cancel() following Confirm(), got %d", calls2)
	}
}

func TestUserTagModal_UserIDEscaping_Brackets(t *testing.T) {
	bracketMembers := []*store.AutocompleteMemberEntry{
		{
			UserID:       "@alice:[2001:db8::1]:8448",
			Displayname:  "Alice IPv6",
			SearchString: "alice",
		},
		{
			UserID:       `@bob:["0"]:matrix.org`,
			Displayname:  "Bob Quoted",
			SearchString: "bob",
		},
		{
			UserID:       "@charlie:[red:blue:b]",
			Displayname:  "Charlie ColorTag",
			SearchString: "charlie",
		},
		{
			UserID:       "@[2001:db8::2]:8448",
			Displayname:  "",
			SearchString: "bracketed",
		},
	}

	modal := NewUserTagModal(nil, bracketMembers, nil)
	rendered := modal.GetRenderedText()

	// 1. Verify rendered text contains properly escaped user IDs
	escapedAlice := mauview.Escape("@alice:[2001:db8::1]:8448")
	if !strings.Contains(rendered, escapedAlice) {
		t.Errorf("expected rendered text to contain escaped Alice UserID %q, got: %q", escapedAlice, rendered)
	}

	escapedBob := mauview.Escape(`@bob:["0"]:matrix.org`)
	if !strings.Contains(rendered, escapedBob) {
		t.Errorf("expected rendered text to contain escaped Bob UserID %q, got: %q", escapedBob, rendered)
	}

	escapedCharlie := mauview.Escape("@charlie:[red:blue:b]")
	if !strings.Contains(rendered, escapedCharlie) {
		t.Errorf("expected rendered text to contain escaped Charlie UserID %q, got: %q", escapedCharlie, rendered)
	}

	// 2. Verify TextView region parsing is not corrupted by bracketed user IDs
	region0 := modal.results.GetRegionText("0")
	if !strings.Contains(region0, "Alice IPv6") || !strings.Contains(region0, escapedAlice) {
		t.Errorf("expected region 0 to contain Alice and escaped UserID, got: %q", region0)
	}

	region1 := modal.results.GetRegionText("1")
	if !strings.Contains(region1, "Bob Quoted") || !strings.Contains(region1, escapedBob) {
		t.Errorf("expected region 1 to contain Bob and escaped UserID, got: %q", region1)
	}

	region2 := modal.results.GetRegionText("2")
	if !strings.Contains(region2, "Charlie ColorTag") || !strings.Contains(region2, escapedCharlie) {
		t.Errorf("expected region 2 to contain Charlie and escaped UserID, got: %q", region2)
	}

	region3 := modal.results.GetRegionText("3")
	if !strings.Contains(region3, mauview.Escape("@[2001:db8::2]:8448")) {
		t.Errorf("expected region 3 to contain escaped fallback display name, got: %q", region3)
	}

	// 3. Confirm region highlight works
	highlights := modal.results.GetHighlights()
	if len(highlights) != 1 || highlights[0] != "0" {
		t.Errorf("expected highlight on '0', got %v", highlights)
	}
}

func TestUserTagModal_ConcurrentFilteringAndNavigation(t *testing.T) {
	numMembers := 50
	members := make([]*store.AutocompleteMemberEntry, numMembers)
	for i := 0; i < numMembers; i++ {
		members[i] = &store.AutocompleteMemberEntry{
			UserID:       id.UserID(fmt.Sprintf("@user%d:example.com", i)),
			Displayname:  fmt.Sprintf("User Display %d", i),
			SearchString: fmt.Sprintf("user %d", i),
		}
	}

	modal := NewUserTagModal(nil, members, nil)

	var wg sync.WaitGroup
	iterations := 200

	// Worker 1 & 2: Rapid Filter calls
	queries := []string{"", "user", "1", "display", "nonexistent", "999", "user display 1"}
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				q := queries[(i+workerID)%len(queries)]
				modal.Filter(q)
			}
		}(g)
	}

	// Worker 3 & 4: Keyboard navigation and concurrent rune typing (Down, Up, Backtab, Runes, Space, Backspace)
	downEvt := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	upEvt := tcell.NewEventKey(tcell.KeyUp, 0, 0)
	backtabEvt := tcell.NewEventKey(tcell.KeyBacktab, 0, 0)
	spaceEvt := tcell.NewEventKey(tcell.KeyRune, ' ', 0)
	bsEvt := tcell.NewEventKey(tcell.KeyBackspace2, 0, 0)
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				switch (i + workerID) % 6 {
				case 0:
					modal.OnKeyEvent(downEvt)
				case 1:
					modal.OnKeyEvent(upEvt)
				case 2:
					modal.OnKeyEvent(backtabEvt)
				case 3:
					runeEvt := tcell.NewEventKey(tcell.KeyRune, rune('a'+(i%26)), 0)
					modal.OnKeyEvent(runeEvt)
				case 4:
					modal.OnKeyEvent(spaceEvt)
				case 5:
					modal.OnKeyEvent(bsEvt)
				}
			}
		}(g)
	}

	// Worker 5: Toggling selections
	wg.Add(1)
	go func() {
		defer wg.Done()
		tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				modal.ToggleCurrent()
			} else {
				modal.OnKeyEvent(tabEvt)
			}
		}
	}()

	// Worker 6: Concurrent readers of state
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = modal.GetSelectedUsers()
			_ = modal.GetSelectedOrder()
			_ = modal.GetMatches()
			_ = modal.GetSelectedIndex()
			_ = modal.GetRenderedText()
			_ = modal.GetSearchText()
			_ = modal.IsDismissed()
		}
	}()

	// Worker 7: RenderList calls
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			modal.RenderList()
		}
	}()

	wg.Wait()

	// Verify state is consistent after concurrent access
	modal.Filter("")
	matches := modal.GetMatches()
	if len(matches) != numMembers {
		t.Errorf("expected %d matches after restore, got %d", numMembers, len(matches))
	}
}

func TestUserTagModal_ConcurrentConfirmation(t *testing.T) {
	var callCount int64
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, func(tags []string) {
		atomic.AddInt64(&callCount, 1)
	})

	var wg sync.WaitGroup
	goroutines := 20
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			modal.Confirm()
		}()
	}
	wg.Wait()

	if finalCount := atomic.LoadInt64(&callCount); finalCount != 1 {
		t.Errorf("expected onConfirm to be called exactly 1 time across %d concurrent Confirm() calls, got %d",
			goroutines, finalCount)
	}
}

func TestUserTagModal_ConcurrentRuneInput_Race(t *testing.T) {
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, nil)

	var wg sync.WaitGroup
	iterations := 150

	// Goroutine Group 1: Concurrent rune typers (Workers 0..2)
	// Rapidly generates character runes (lowercase, uppercase, digits), spaces, and backspaces.
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				var evt mauview.KeyEvent
				switch (i + workerID) % 5 {
				case 0:
					evt = tcell.NewEventKey(tcell.KeyRune, rune('a'+(i%26)), 0)
				case 1:
					evt = tcell.NewEventKey(tcell.KeyRune, rune('A'+(i%26)), 0)
				case 2:
					evt = tcell.NewEventKey(tcell.KeyRune, ' ', 0)
				case 3:
					evt = tcell.NewEventKey(tcell.KeyBackspace2, 0, 0)
				case 4:
					evt = tcell.NewEventKey(tcell.KeyRune, rune('0'+(i%10)), 0)
				}
				modal.OnKeyEvent(evt)
			}
		}(w)
	}

	// Goroutine Group 2: Concurrent search text and state readers (Workers 3..4)
	// Reads search text safely under inputLock via GetSearchText() and modal state under lock.RLock.
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = modal.GetSearchText()
				_ = modal.GetRenderedText()
				_ = modal.GetMatches()
				_ = modal.GetSelectedIndex()
				_ = modal.IsDismissed()
			}
		}()
	}

	wg.Wait()

	// Post-concurrency sanity verification:
	// Verify modal state has not deadlocked, corrupted memory, or left inconsistent counts.
	modal.Filter("")
	matches := modal.GetMatches()
	if len(matches) != len(members) {
		t.Errorf("expected %d matches after filter reset, got %d", len(members), len(matches))
	}
}

type mockPasteEvent struct {
	*tcell.EventPaste
	text string
}

func (m mockPasteEvent) Text() string {
	return m.text
}

func TestUserTagModal_OnPasteEvent(t *testing.T) {
	members := makeTestMembers()
	modal := NewUserTagModal(nil, members, nil)

	pasteEvt := mockPasteEvent{
		EventPaste: tcell.NewEventPaste(true),
		text:       "alice",
	}
	handled := modal.OnPasteEvent(pasteEvt)
	if !handled {
		t.Errorf("expected OnPasteEvent to return true, got false")
	}
	if text := modal.GetSearchText(); !strings.Contains(text, "alice") {
		t.Errorf("expected search text to contain 'alice', got %q", text)
	}
	if matches := modal.GetMatches(); len(matches) != 1 {
		t.Errorf("expected 1 match for pasted 'alice', got %d", len(matches))
	}
}

func TestRoomView_Focus_NilSafety(t *testing.T) {
	t.Run("NilInputArea_Focus", func(t *testing.T) {
		rv := &RoomView{input: nil}
		// Must not panic
		rv.Focus()
	})

	t.Run("NilInputArea_Blur", func(t *testing.T) {
		rv := &RoomView{input: nil}
		// Must not panic
		rv.Blur()
	})

	t.Run("NilReceiver_Focus", func(t *testing.T) {
		var rv *RoomView
		// Must not panic
		rv.Focus()
	})

	t.Run("NilReceiver_Blur", func(t *testing.T) {
		var rv *RoomView
		// Must not panic
		rv.Blur()
	})

	t.Run("NormalInput_FocusAndBlur", func(t *testing.T) {
		input := mauview.NewInputArea()
		rv := &RoomView{input: input}
		// Must not panic and correctly delegate to input
		rv.Focus()
		input.TypeRune('x')
		if input.GetText() != "x" {
			t.Errorf("expected input text 'x', got %q", input.GetText())
		}
		rv.Blur()
	})
}
