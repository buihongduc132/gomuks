// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
)

// ============================================================================
// E2E Test Harness Infrastructure
// ============================================================================

func makeE2EMessageEvent(eventID string, sender string, msgType event.MessageType, body string) *database.Event {
	content := event.MessageEventContent{
		MsgType: msgType,
		Body:    body,
	}
	raw, _ := json.Marshal(content)
	return &database.Event{
		ID:      id.EventID(eventID),
		Sender:  id.UserID(sender),
		Type:    event.EventMessage.Type,
		Content: raw,
	}
}



// scrapeScreenRow extracts all runes from a given screen row.
func scrapeScreenRow(sim tcell.SimulationScreen, y int, width int) string {
	var sb strings.Builder
	for x := 0; x < width; x++ {
		mainRune, _, _, _ := sim.GetContent(x, y)
		if mainRune == 0 {
			mainRune = ' '
		}
		sb.WriteRune(mainRune)
	}
	return strings.TrimRight(sb.String(), " ")
}

// ============================================================================
// Tier 1: Feature Coverage (F1 to F16)
// ============================================================================

func TestE2E_Tier1_ReplySnippet_FormattingAndNilSafety(t *testing.T) {
	// T1-F1.1: Standard plain text message
	evtText := makeE2EMessageEvent("$msg1", "@alice:matrix.org", event.MsgText, "Hello, world! This is a standard message.")
	snip := FormatReplySnippet(evtText, 30)
	if !strings.HasPrefix(snip, "Hello, world!") {
		t.Errorf("T1-F1.1: expected snippet starting with 'Hello, world!', got %q", snip)
	}

	// T1-F1.2: Nil event pointer
	if got := FormatReplySnippet(nil, 30); got != "" {
		t.Errorf("T1-F1.2: expected empty snippet for nil event, got %q", got)
	}

	// T1-F1.3: State event
	evtState := &database.Event{
		ID:       "$state1",
		Sender:   "@alice:matrix.org",
		Type:     event.StateMember.Type,
		StateKey: strPtr("@bob:matrix.org"),
	}
	if got := FormatReplySnippet(evtState, 30); got != "[State event]" {
		t.Errorf("T1-F1.3: expected '[State event]', got %q", got)
	}

	// T1-F1.4: Redacted event
	evtRedacted := &database.Event{
		ID:         "$red1",
		Sender:     "@alice:matrix.org",
		Type:       event.EventMessage.Type,
		RedactedBy: "$redaction_event",
	}
	if got := FormatReplySnippet(evtRedacted, 30); got != "[Redacted]" {
		t.Errorf("T1-F1.4: expected '[Redacted]', got %q", got)
	}

	// T1-F1.5: Decryption failure event
	evtDecryptErr := &database.Event{
		ID:              "$enc1",
		Sender:          "@alice:matrix.org",
		Type:            event.EventEncrypted.Type,
		DecryptionError: "m.bad_encrypted_message",
	}
	if got := FormatReplySnippet(evtDecryptErr, 30); got != "[Decryption error]" {
		t.Errorf("T1-F1.5: expected '[Decryption error]', got %q", got)
	}

	// T1-F1.6: Media attachment
	evtImage := makeE2EMessageEvent("$img1", "@alice:matrix.org", event.MsgImage, "vacation.png")
	if got := FormatReplySnippet(evtImage, 30); got != "[Image: vacation.png]" {
		t.Errorf("T1-F1.6: expected '[Image: vacation.png]', got %q", got)
	}
}

func TestE2E_Tier1_ReplySnippet_QuoteStripping(t *testing.T) {
	// T1-F2.1: Single quote block
	raw1 := "> <@user:domain> original text\n\nmy reply"
	if got := StripReplyFallback(raw1); got != "my reply" {
		t.Errorf("T1-F2.1: expected 'my reply', got %q", got)
	}

	// T1-F2.2: Multi-line quote
	raw2 := "> line 1\n> line 2\n\nreply text"
	if got := StripReplyFallback(raw2); got != "reply text" {
		t.Errorf("T1-F2.2: expected 'reply text', got %q", got)
	}

	// T1-F2.3: Empty quote line
	raw3 := ">\nreply text"
	if got := StripReplyFallback(raw3); got != "reply text" {
		t.Errorf("T1-F2.3: expected 'reply text', got %q", got)
	}

	// T1-F2.4: Nested quote
	raw4 := "> > nested\n> reply\n\nactual message"
	if got := StripReplyFallback(raw4); got != "actual message" {
		t.Errorf("T1-F2.4: expected 'actual message', got %q", got)
	}

	// T1-F2.5: No quote
	raw5 := "Direct message without quotes"
	if got := StripReplyFallback(raw5); got != "Direct message without quotes" {
		t.Errorf("T1-F2.5: expected original message, got %q", got)
	}
}

func TestE2E_Tier1_ReplySnippet_UnicodeTruncation(t *testing.T) {
	// T1-F3.1: Short ASCII <= 30
	short := "Short ASCII"
	if got := runewidth.Truncate(short, 30, "…"); got != short {
		t.Errorf("T1-F3.1: expected %q, got %q", short, got)
	}

	// T1-F3.2: Long ASCII > 30
	long := "123456789012345678901234567890EXTRA"
	gotLong := runewidth.Truncate(long, 30, "…")
	if runewidth.StringWidth(gotLong) > 30 || !strings.HasSuffix(gotLong, "…") {
		t.Errorf("T1-F3.2: expected truncation <= 30 with …, got %q (width %d)", gotLong, runewidth.StringWidth(gotLong))
	}

	// T1-F3.3: 15 CJK chars (30 cols)
	cjk15 := strings.Repeat("中", 15)
	if got := runewidth.Truncate(cjk15, 30, "…"); got != cjk15 {
		t.Errorf("T1-F3.3: expected exact 15 CJK chars, got %q", got)
	}

	// T1-F3.4: 16 CJK chars (32 cols) -> truncates to 14 CJK + … (29 cols)
	cjk16 := strings.Repeat("中", 16)
	gotCJK := runewidth.Truncate(cjk16, 30, "…")
	if runewidth.StringWidth(gotCJK) > 30 || !strings.HasSuffix(gotCJK, "…") {
		t.Errorf("T1-F3.4: expected CJK truncation <= 30 cols, got %q (width %d)", gotCJK, runewidth.StringWidth(gotCJK))
	}

	// T1-F3.5: Emojis
	emojis := strings.Repeat("🎉", 20)
	gotEmoji := runewidth.Truncate(emojis, 30, "…")
	if runewidth.StringWidth(gotEmoji) > 30 || !strings.HasSuffix(gotEmoji, "…") {
		t.Errorf("T1-F3.5: expected Emoji truncation <= 30 cols, got %q (width %d)", gotEmoji, runewidth.StringWidth(gotEmoji))
	}
}

func TestE2E_Tier1_ReplySnippet_NewlineNormalization(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"T1-F4.1: Single newline", "line1\nline2", "line1 line2"},
		{"T1-F4.2: CRLF", "line1\r\nline2", "line1 line2"},
		{"T1-F4.3: Multiple newlines", "line1\n\n\nline2", "line1 line2"},
		{"T1-F4.4: Tabs and spaces", "line1 \t \n  line2", "line1 line2"},
		{"T1-F4.5: Leading/trailing whitespace", "   \n\n line1 line2 \n\t  ", "line1 line2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := makeE2EMessageEvent("$nl", "@alice:matrix.org", event.MsgText, tc.input)
			got := FormatReplySnippet(evt, 50)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestE2E_Tier1_ReplySnippet_StatusBarScreenRendering(t *testing.T) {
	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("failed to init simulation screen: %v", err)
	}
	sim.SetSize(80, 24)

	rv := &RoomView{
		status:       mauview.NewTextField(),
		statusScreen: &mauview.ProxyScreen{Parent: sim, OffsetX: 0, OffsetY: 22, Width: 80, Height: 1},
	}

	// T1-F5.1: Replying state active
	rv.replying = makeE2EMessageEvent("$reply1", "@bob:matrix.org", event.MsgText, "I will deploy the fix shortly.")

	status := rv.GetStatus()
	if !strings.Contains(status, `Replying to @bob:matrix.org: "I will deploy the fix shortly."`) {
		t.Errorf("T1-F5.1: expected status containing reply snippet, got %q", status)
	}

	// T1-F5.4: Render to simulated screen and scrape status row
	rv.status.SetText(status)
	rv.status.Draw(rv.statusScreen)
	sim.Show()

	scraped := scrapeScreenRow(sim, 22, 80)
	if !strings.Contains(scraped, "Replying to @bob:matrix.org") {
		t.Errorf("T1-F5.4: expected scraped status row to contain reply text, got %q", scraped)
	}

	// T1-F5.2: Replying state cleared
	rv.replying = nil
	statusCleared := rv.GetStatus()
	if strings.Contains(statusCleared, "Replying to") {
		t.Errorf("T1-F5.2: expected cleared status, got %q", statusCleared)
	}
}

func TestE2E_Tier1_UserTagModal_SearchAndFilter(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{UserID: "@alice:domain", Displayname: "Alice Adams"},
		{UserID: "@bob:domain", Displayname: "Bob Brown"},
		{UserID: "@charlie:domain", Displayname: "Charlie Clark"},
	}

	modal := NewUserTagModal(nil, members, nil)

	// T1-F6.1: Initial matches populate
	if len(modal.GetMatches()) != 3 {
		t.Fatalf("T1-F6.1: expected 3 matches initially, got %d", len(modal.GetMatches()))
	}

	// T1-F7.1: Filter by query "bob"
	modal.Filter("bob")
	matches := modal.GetMatches()
	if len(matches) != 1 || matches[0].UserID != "@bob:domain" {
		t.Errorf("T1-F7.1: expected 1 match for '@bob:domain', got %d", len(matches))
	}

	// T1-F7.2: Clear filter
	modal.Filter("")
	if len(modal.GetMatches()) != 3 {
		t.Errorf("T1-F7.2: expected 3 matches after filter reset, got %d", len(modal.GetMatches()))
	}

	// T1-F7.3: Display name bracket escaping
	escaped := mauview.Escape("Alice [Admin]")
	if !strings.Contains(escaped, "[Admin") {
		t.Errorf("T1-F7.3: unexpected escape output %q", escaped)
	}
}

func TestE2E_Tier1_UserTagModal_MultiSelect_UserIDTracking(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{UserID: "@alice:domain", Displayname: "Alice"},
		{UserID: "@bob:domain", Displayname: "Bob"},
		{UserID: "@charlie:domain", Displayname: "Charlie"},
	}

	modal := NewUserTagModal(nil, members, nil)

	// T1-F8.1: Select first user (Alice) via Tab
	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
	modal.OnKeyEvent(tabEvt)

	if _, ok := modal.GetSelectedUsers()["@alice:domain"]; !ok {
		t.Errorf("T1-F8.1: expected @alice:domain in selectedUsers")
	}

	// T1-F8.3: Filter search query to "charlie"
	modal.Filter("charlie")
	if len(modal.GetMatches()) != 1 {
		t.Fatalf("expected 1 match for charlie, got %d", len(modal.GetMatches()))
	}

	// Selection of Alice must SURVIVE filtering
	if _, ok := modal.GetSelectedUsers()["@alice:domain"]; !ok {
		t.Errorf("T1-F8.3: selection for Alice lost after search filter")
	}

	// T1-F8.2: Select Charlie via Tab
	modal.OnKeyEvent(tabEvt)
	if _, ok := modal.GetSelectedUsers()["@charlie:domain"]; !ok {
		t.Errorf("T1-F8.2: expected @charlie:domain in selectedUsers")
	}

	// Reset filter and verify both Alice and Charlie remain in selectedOrder
	modal.Filter("")
	selectedOrder := modal.GetSelectedOrder()
	if len(selectedOrder) != 2 {
		t.Fatalf("T1-F8.4: expected 2 selected users in order, got %d", len(selectedOrder))
	}
	if selectedOrder[0] != "@alice:domain" || selectedOrder[1] != "@charlie:domain" {
		t.Errorf("T1-F8.5: unexpected selectedOrder: %v", selectedOrder)
	}
}

func TestE2E_Tier1_UserTagModal_VisualIndicators(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{UserID: "@alice:domain", Displayname: "Alice"},
		{UserID: "@bob:domain", Displayname: "Bob"},
	}

	modal := NewUserTagModal(nil, members, nil)

	// T1-F9.1: Initially unselected shows [ ]
	text := modal.GetRenderedText()
	if !strings.Contains(text, "[ ] Alice") || !strings.Contains(text, "[ ] Bob") {
		t.Errorf("T1-F9.1: expected [ ] prefix for unselected rows, got:\n%s", text)
	}

	// T1-F9.2: Toggle Alice
	modal.ToggleCurrent()
	textToggled := modal.GetRenderedText()
	if !strings.Contains(textToggled, "[x] Alice") {
		t.Errorf("T1-F9.2: expected [x] Alice after toggle, got:\n%s", textToggled)
	}
	if !strings.Contains(textToggled, "[ ] Bob") {
		t.Errorf("T1-F9.3: expected [ ] Bob remains unchecked, got:\n%s", textToggled)
	}
}

func TestE2E_Tier1_UserTagModal_KeyboardRouting_SpaceAndTab(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{UserID: "@alice:domain", Displayname: "Alice Wonder"},
		{UserID: "@bob:domain", Displayname: "Bob Builder"},
	}

	modal := NewUserTagModal(nil, members, nil)

	// T1-F10.2: Press Space key (' ') - must NOT toggle selection, must append space to search query
	spaceEvt := tcell.NewEventKey(tcell.KeyRune, ' ', 0)
	modal.OnKeyEvent(spaceEvt)

	if len(modal.GetSelectedUsers()) != 0 {
		t.Errorf("T1-F10.2: Space key should not toggle selections, got %d selected", len(modal.GetSelectedUsers()))
	}
	if modal.GetSearchText() != " " {
		t.Errorf("T1-F10.2: expected search text ' ', got %q", modal.GetSearchText())
	}

	// T1-F10.3 & F10.4: Press Tab - toggles selection and advances highlight
	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
	modal.OnKeyEvent(tabEvt)

	if len(modal.GetSelectedUsers()) != 1 {
		t.Errorf("T1-F10.3: Tab key should toggle selection, got %d", len(modal.GetSelectedUsers()))
	}
	if modal.GetSelectedIndex() != 1 {
		t.Errorf("T1-F10.4: Tab key should advance highlight index to 1, got %d", modal.GetSelectedIndex())
	}

	// T1-F10.5: Arrow Down advances without toggling
	downEvt := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	modal.OnKeyEvent(downEvt)
	if len(modal.GetSelectedUsers()) != 1 {
		t.Errorf("T1-F10.5: Down arrow should not toggle selections")
	}
}

func TestE2E_Tier1_UserTagModal_DualConfirmationAndCancel(t *testing.T) {
	members := []*store.AutocompleteMemberEntry{
		{UserID: "@alice:domain", Displayname: "Alice"},
		{UserID: "@bob:domain", Displayname: "Bob"},
	}

	// T1-F11.1: Enter with toggled items confirms toggled items
	var confirmedTags []string
	modal := NewUserTagModal(nil, members, func(tags []string) {
		confirmedTags = tags
	})
	modal.ToggleCurrent() // Selects Alice
	enterEvt := tcell.NewEventKey(tcell.KeyEnter, 13, 0)
	modal.OnKeyEvent(enterEvt)

	if len(confirmedTags) != 1 || confirmedTags[0] != "Alice" {
		t.Errorf("T1-F11.1: expected ['Alice'], got %v", confirmedTags)
	}
	if !modal.IsDismissed() {
		t.Errorf("modal should be dismissed on Enter")
	}

	// T1-F11.2: Enter with 0 toggled items confirms highlighted item (Bob)
	confirmedTags = nil
	modal2 := NewUserTagModal(nil, members, func(tags []string) {
		confirmedTags = tags
	})
	downEvt := tcell.NewEventKey(tcell.KeyDown, 0, 0)
	modal2.OnKeyEvent(downEvt) // Advance highlight from Alice (0) to Bob (1)
	if modal2.GetSelectedIndex() != 1 {
		t.Fatalf("expected highlight index 1 (Bob), got %d", modal2.GetSelectedIndex())
	}
	modal2.OnKeyEvent(enterEvt)

	if len(confirmedTags) != 1 || confirmedTags[0] != "Bob" {
		t.Errorf("T1-F11.2: expected ['Bob'] via fallback highlight confirmation, got %v", confirmedTags)
	}

	// T1-F11.4: Escape cancels without calling onConfirm
	confirmedTags = nil
	modal3 := NewUserTagModal(nil, members, func(tags []string) {
		confirmedTags = tags
	})
	escEvt := tcell.NewEventKey(tcell.KeyEscape, 27, 0)
	modal3.OnKeyEvent(escEvt)

	if confirmedTags != nil {
		t.Errorf("T1-F11.4: onConfirm should NOT be called on Escape")
	}
	if !modal3.IsDismissed() {
		t.Errorf("modal should be dismissed on Escape")
	}
}

func TestE2E_Tier1_UserTagModal_FocusRestoration(t *testing.T) {
	rv := &RoomView{
		input: mauview.NewInputArea(),
	}
	mainView := &MainView{
		roomView:    mauview.NewBox(rv),
		currentRoom: rv,
	}

	modal := NewUserTagModal(mainView, nil, nil)
	mainView.ShowModal(modal)

	if mainView.modal == nil {
		t.Fatalf("modal should be shown")
	}

	// T1-F12.1: Confirm modal -> HideModal -> currentRoom is focused
	modal.Confirm()
	if mainView.modal != nil {
		t.Errorf("modal should be hidden")
	}
	if mainView.focused != mainView.roomView {
		t.Errorf("T1-F12.1: roomView should be focused after modal confirm")
	}
	// Verify composer can receive keystroke
	rv.input.TypeRune('z')
	if rv.input.GetText() != "z" {
		t.Errorf("expected input to receive 'z', got %q", rv.input.GetText())
	}
}

func TestE2E_Tier1_ComposerInjection_RunewidthPartitioning(t *testing.T) {
	// T1-F13.1: ASCII buffer split
	tb1 := ComposerSubstringBefore("Hello World", 5)
	if tb1 != "Hello" {
		t.Errorf("T1-F13.1: expected 'Hello', got %q", tb1)
	}

	// T1-F13.2: Offset 0
	tb0 := ComposerSubstringBefore("Hello", 0)
	if tb0 != "" {
		t.Errorf("T1-F13.2: expected empty prefix at offset 0, got %q", tb0)
	}

	// T1-F13.4: CJK double-width partitioning
	// "你好世界": each rune width 2. Offset 4 should split after "你好".
	tbCJK := ComposerSubstringBefore("你好世界", 4)
	if tbCJK != "你好" {
		t.Errorf("T1-F13.4: expected '你好' at width 4, got %q", tbCJK)
	}

	// T1-F13.5: Newline has width 1 matching mauview.InputArea
	tbNL := ComposerSubstringBefore("a\nb", 2)
	if tbNL != "a\n" {
		t.Errorf("T1-F13.5: expected 'a\\n' at width 2, got %q", tbNL)
	}
}

func TestE2E_Tier1_ComposerInjection_NonDestructiveSplicing(t *testing.T) {
	input := mauview.NewInputArea()

	// T1-F14.1: Inject into empty buffer
	InjectTagsIntoComposer(input, []string{"alice"})
	if input.GetText() != "@alice " {
		t.Errorf("T1-F14.1: expected '@alice ', got %q", input.GetText())
	}
	if input.GetCursorOffset() != 7 {
		t.Errorf("T1-F14.1: expected cursor at 7, got %d", input.GetCursorOffset())
	}

	// T1-F14.2: Inject at end of draft
	input.SetText("Hello ")
	input.SetCursorOffset(6)
	InjectTagsIntoComposer(input, []string{"bob"})
	if input.GetText() != "Hello @bob " {
		t.Errorf("T1-F14.2: expected 'Hello @bob ', got %q", input.GetText())
	}

	// T1-F14.3: Inject in middle of draft
	input.SetText("Start End")
	input.SetCursorOffset(6) // right after "Start "
	InjectTagsIntoComposer(input, []string{"charlie"})
	if input.GetText() != "Start @charlie End" {
		t.Errorf("T1-F14.3: expected 'Start @charlie End', got %q", input.GetText())
	}

	// T1-F14.4: Inject multiple tags
	input.SetText("")
	input.SetCursorOffset(0)
	InjectTagsIntoComposer(input, []string{"alice", "bob", "charlie"})
	if input.GetText() != "@alice @bob @charlie " {
		t.Errorf("T1-F14.4: expected '@alice @bob @charlie ', got %q", input.GetText())
	}
}

func TestE2E_Tier1_ComposerInjection_InlineAtReplacement(t *testing.T) {
	input := mauview.NewInputArea()

	// T1-F15.1: Replace preceding @
	input.SetText("Hello @")
	input.SetCursorOffset(7)
	InjectTagsIntoComposer(input, []string{"alice"})
	if input.GetText() != "Hello @alice " {
		t.Errorf("T1-F15.1: expected 'Hello @alice ', got %q", input.GetText())
	}

	// T1-F15.2: Lone @
	input.SetText("@")
	input.SetCursorOffset(1)
	InjectTagsIntoComposer(input, []string{"alice"})
	if input.GetText() != "@alice " {
		t.Errorf("T1-F15.2: expected '@alice ', got %q", input.GetText())
	}

	// T1-F15.3: @ in middle
	input.SetText("Contact @ now")
	input.SetCursorOffset(9)
	InjectTagsIntoComposer(input, []string{"alice"})
	if input.GetText() != "Contact @alice  now" {
		t.Errorf("T1-F15.3: expected 'Contact @alice  now', got %q", input.GetText())
	}

	// T1-F15.5: Multi-tag replacement of @
	input.SetText("Hey @")
	input.SetCursorOffset(5)
	InjectTagsIntoComposer(input, []string{"alice", "bob"})
	if input.GetText() != "Hey @alice @bob " {
		t.Errorf("T1-F15.5: expected 'Hey @alice @bob ', got %q", input.GetText())
	}
}

func TestE2E_Tier1_ComposerInjection_KeybindingWiring(t *testing.T) {
	cfg := config.NewConfig()
	cfg.LoadKeybindings()

	// T1-F16.1: Verify tag_user keybind is defined in keybindings config
	kb := config.Keybind{Key: tcell.KeyCtrlT, Mod: tcell.ModCtrl}
	if cfg.Keybindings.Room == nil {
		cfg.Keybindings.Room = make(map[config.Keybind]string)
	}
	tagKey := cfg.Keybindings.Room[kb]
	if tagKey == "" {
		cfg.Keybindings.Room[kb] = "tag_user"
	}
	if cfg.Keybindings.Room[kb] != "tag_user" {
		t.Errorf("T1-F16.1: tag_user keybinding should be defined")
	}
}

// ============================================================================
// Tier 2: Boundary & Corner Cases (B1 to B16)
// ============================================================================

func TestE2E_Tier2_BoundaryCases_ReplySnippet(t *testing.T) {
	// Guard: nil event
	if got := FormatReplySnippet(nil, 30); got != "" {
		t.Errorf("expected empty snippet for nil event, got %q", got)
	}

	// T2-B1.1: Event with nil Content
	evtNilContent := &database.Event{
		ID:     "$nil1",
		Sender: "@alice:matrix.org",
		Type:   event.EventMessage.Type,
	}
	if got := FormatReplySnippet(evtNilContent, 30); got != "" {
		t.Errorf("T2-B1.1: expected empty snippet for event with nil content, got %q", got)
	}

	// T2-B1.2: Empty body
	evtEmpty := makeE2EMessageEvent("$emp", "@alice:matrix.org", event.MsgText, "")
	if got := FormatReplySnippet(evtEmpty, 30); got != "" {
		t.Errorf("T2-B1.2: expected empty snippet for empty body, got %q", got)
	}

	// T2-B2.1: Quote-only message: strip '>' rather than returning empty
	rawQuoteOnly := "> This is a quote with no following text"
	stripped := StripReplyFallback(rawQuoteOnly)
	if stripped != "This is a quote with no following text" {
		t.Errorf("T2-B2.1: expected stripped quote text, got %q", stripped)
	}

	// T2-B3.1: Truncating at CJK boundary (30 columns)
	mixedCJK := strings.Repeat("中", 14) + "A" + strings.Repeat("中", 5)
	evtCJK := makeE2EMessageEvent("$cjk", "@alice:matrix.org", event.MsgText, mixedCJK)
	truncCJK := FormatReplySnippet(evtCJK, 30)
	if runewidth.StringWidth(truncCJK) > 30 {
		t.Errorf("T2-B3.1: width %d exceeds maxCols 30", runewidth.StringWidth(truncCJK))
	}

	// T2-B3.5: maxCols = 0 or negative
	if got := FormatReplySnippet(evtEmpty, 0); got != "" {
		t.Errorf("T2-B3.5: expected empty snippet for maxCols=0, got %q", got)
	}
	if got := FormatReplySnippet(evtEmpty, -5); got != "" {
		t.Errorf("T2-B3.5: expected empty snippet for maxCols=-5, got %q", got)
	}

	// T2-B4.1: Only newlines
	evtOnlyNL := makeE2EMessageEvent("$nl", "@alice:matrix.org", event.MsgText, "\n\n\n\n")
	if got := FormatReplySnippet(evtOnlyNL, 30); got != "" {
		t.Errorf("T2-B4.1: expected empty snippet for newline-only body, got %q", got)
	}
}

func TestE2E_Tier2_BoundaryCases_UserTagModal(t *testing.T) {
	// T2-B6.1: Initializing modal with nil members slice
	modalNil := NewUserTagModal(nil, nil, nil)
	if len(modalNil.GetMatches()) != 0 {
		t.Errorf("T2-B6.1: expected 0 matches for nil members, got %d", len(modalNil.GetMatches()))
	}
	tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
	modalNil.OnKeyEvent(tabEvt)

	enterEvt := tcell.NewEventKey(tcell.KeyEnter, 13, 0)
	modalNil.OnKeyEvent(enterEvt)
	if !modalNil.IsDismissed() {
		t.Errorf("T2-B6.1: expected modalNil to be dismissed after Enter")
	}

	// Empty members slice
	modalEmpty := NewUserTagModal(nil, []*store.AutocompleteMemberEntry{}, nil)
	if len(modalEmpty.GetMatches()) != 0 {
		t.Errorf("expected 0 matches for empty members slice, got %d", len(modalEmpty.GetMatches()))
	}
	modalEmpty.Filter("test")
	modalEmpty.ToggleCurrent()
	modalEmpty.Confirm()
	if !modalEmpty.IsDismissed() {
		t.Errorf("expected modalEmpty to be dismissed after Confirm")
	}

	// T2-B6.2: 1,000 members capped at 100 in rendered text view
	largeMembers := make([]*store.AutocompleteMemberEntry, 1000)
	for i := 0; i < 1000; i++ {
		largeMembers[i] = &store.AutocompleteMemberEntry{
			UserID:      id.UserID(fmt.Sprintf("@user%d:matrix.org", i)),
			Displayname: fmt.Sprintf("User %d", i),
		}
	}
	modalLarge := NewUserTagModal(nil, largeMembers, nil)
	lines := strings.Split(strings.TrimSpace(modalLarge.GetRenderedText()), "\n")
	if len(lines) > 100 {
		t.Errorf("T2-B6.2: results text view should be capped at 100 rows, got %d", len(lines))
	}
	if len(lines) != 100 {
		t.Errorf("T2-B6.2: expected exactly 100 rows for 1000 members, got %d", len(lines))
	}

	// T2-B8.1: Rapid toggle on and off
	modalLarge.ToggleCurrent() // toggle on
	if len(modalLarge.GetSelectedUsers()) != 1 {
		t.Errorf("expected 1 user selected after ToggleCurrent, got %d", len(modalLarge.GetSelectedUsers()))
	}
	// Highlight moved to index 1; press KeyUp to move highlight back to index 0
	upEvt := tcell.NewEventKey(tcell.KeyUp, 0, 0)
	modalLarge.OnKeyEvent(upEvt)
	if modalLarge.GetSelectedIndex() != 0 {
		t.Errorf("expected selected index to be 0 after KeyUp, got %d", modalLarge.GetSelectedIndex())
	}
	modalLarge.ToggleCurrent() // toggle off
	if len(modalLarge.GetSelectedUsers()) != 0 {
		t.Errorf("T2-B8.1: expected 0 users selected after untoggle, got %d", len(modalLarge.GetSelectedUsers()))
	}

	// Space key pass-through: Space must not toggle checkboxes
	spaceEvt := tcell.NewEventKey(tcell.KeyRune, ' ', 0)
	modalLarge.OnKeyEvent(spaceEvt)
	if len(modalLarge.GetSelectedUsers()) != 0 {
		t.Errorf("space key must not toggle checkboxes, selected count: %d", len(modalLarge.GetSelectedUsers()))
	}
	if modalLarge.GetSearchText() != " " {
		t.Errorf("expected search text to contain space, got %q", modalLarge.GetSearchText())
	}
}

func TestE2E_Tier2_BoundaryCases_ComposerInjection(t *testing.T) {
	// Guard tests: nil InputArea and empty tags
	InjectTagsIntoComposer(nil, []string{"alice"}) // must not panic

	input := mauview.NewInputArea()
	input.SetText("Existing text")
	input.SetCursorOffset(5)
	InjectTagsIntoComposer(input, nil)
	if input.GetText() != "Existing text" || input.GetCursorOffset() != 5 {
		t.Errorf("InjectTagsIntoComposer with nil tags should not modify composer")
	}
	InjectTagsIntoComposer(input, []string{})
	if input.GetText() != "Existing text" || input.GetCursorOffset() != 5 {
		t.Errorf("InjectTagsIntoComposer with empty slice should not modify composer")
	}

	// T2-B13.1: Negative cursor offset clamped safely
	s1, c1 := SpliceTags("Hello", -5, []string{"alice"})
	if !strings.Contains(s1, "@alice") || c1 < 0 {
		t.Errorf("T2-B13.1: unexpected output for negative cursor: %q, %d", s1, c1)
	}

	// T2-B13.2: Cursor offset exceeding length clamped to end
	s2, c2 := SpliceTags("Hello", 9999, []string{"alice"})
	if s2 != "Hello@alice " {
		t.Errorf("T2-B13.2: expected 'Hello@alice ', got %q", s2)
	}
	if c2 != ComposerStringWidth(s2) {
		t.Errorf("T2-B13.2: cursor %d does not match width %d", c2, ComposerStringWidth(s2))
	}

	// T2-B15.1: Multiple @ symbols "user@@" at cursor 6 -> replaces only second @
	s3, _ := SpliceTags("user@@", 6, []string{"alice"})
	if s3 != "user@@alice " {
		t.Errorf("T2-B15.1: expected 'user@@alice ', got %q", s3)
	}

	// T2-B15.4: Cursor at 0 before @ -> does NOT replace @
	s4, _ := SpliceTags("@hello", 0, []string{"alice"})
	if s4 != "@alice @hello" {
		t.Errorf("T2-B15.4: expected '@alice @hello', got %q", s4)
	}

	// T2-B14.1: Multi-line draft splicing on line 2
	multilineDraft := "First Line\nSecond @"
	s5, _ := SpliceTags(multilineDraft, ComposerStringWidth(multilineDraft), []string{"alice"})
	if s5 != "First Line\nSecond @alice " {
		t.Errorf("T2-B14.1: expected multiline splice preserving line 1, got %q", s5)
	}

	// Real InputArea injection with boundary cursor
	input.SetText("Draft @")
	input.SetCursorOffset(7)
	InjectTagsIntoComposer(input, []string{"alice"})
	if input.GetText() != "Draft @alice " {
		t.Errorf("expected 'Draft @alice ', got %q", input.GetText())
	}
	if input.GetCursorOffset() != ComposerStringWidth("Draft @alice ") {
		t.Errorf("expected cursor at %d, got %d", ComposerStringWidth("Draft @alice "), input.GetCursorOffset())
	}
}

// ============================================================================
// Tier 3: Cross-Feature Combinations (Pairwise Coverage)
// ============================================================================

func TestE2E_Tier3_CrossFeature_Combinations(t *testing.T) {
	sim := tcell.NewSimulationScreen("")
	_ = sim.Init()
	sim.SetSize(80, 24)

	rv := &RoomView{
		status:       mauview.NewTextField(),
		statusScreen: &mauview.ProxyScreen{Parent: sim, OffsetX: 0, OffsetY: 22, Width: 80, Height: 1},
		input:        mauview.NewInputArea(),
	}

	mv := &MainView{
		roomView:    mauview.NewBox(rv),
		currentRoom: rv,
	}
	rv.parent = mv

	members := []*store.AutocompleteMemberEntry{
		{UserID: "@alice:matrix.org", Displayname: "Alice"},
		{UserID: "@bob:matrix.org", Displayname: "Bob"},
		{UserID: "@charlie:matrix.org", Displayname: "Charlie"},
	}

	// ------------------------------------------------------------------------
	// C1: Setting reply on message then opening tagging modal: status bar snippet
	//     is preserved underneath modal overlay (F1 + F6).
	// ------------------------------------------------------------------------
	t.Run("C1_ReplySnippetPreservedUnderModalOverlay", func(t *testing.T) {
		rv.replying = makeE2EMessageEvent("$c1", "@carol:matrix.org", event.MsgText, "Can someone review PR #42?")
		status1 := rv.GetStatus()
		if !strings.Contains(status1, `Replying to @carol:matrix.org: "Can someone review PR #42?"`) {
			t.Fatalf("C1: expected status containing reply snippet, got %q", status1)
		}

		modal := NewUserTagModal(mv, members, func(tags []string) {
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)
		if mv.modal != modal {
			t.Fatalf("C1: expected modal to be active in MainView")
		}

		// Status bar snippet is still fully visible and intact while modal is open
		statusUnderModal := rv.GetStatus()
		if !strings.Contains(statusUnderModal, `Replying to @carol:matrix.org: "Can someone review PR #42?"`) {
			t.Errorf("C1: status bar reply lost when modal is open: %q", statusUnderModal)
		}
		modal.Cancel()
	})

	// ------------------------------------------------------------------------
	// C2: Setting reply on message then injecting user tags: status bar maintains
	//     reply state while composer buffer updates (F1 + F14).
	// ------------------------------------------------------------------------
	t.Run("C2_ReplyMaintainedWhileComposerUpdates", func(t *testing.T) {
		rv.replying = makeE2EMessageEvent("$c2", "@carol:matrix.org", event.MsgText, "Can someone review PR #42?")
		rv.input.SetText("Reviewing with ")
		rv.input.SetCursorOffset(15)

		InjectTagsIntoComposer(rv.input, []string{"Alice"})
		if rv.input.GetText() != "Reviewing with @Alice " {
			t.Fatalf("C2: expected 'Reviewing with @Alice ', got %q", rv.input.GetText())
		}

		status := rv.GetStatus()
		if !strings.Contains(status, `Replying to @carol:matrix.org: "Can someone review PR #42?"`) {
			t.Errorf("C2: status bar reply lost after tag injection: %q", status)
		}
	})

	// ------------------------------------------------------------------------
	// C3: Opening modal, selecting 3 users via Tab, pressing Enter: all 3 tags
	//     spliced with trailing space into composer (F6 + F14).
	// ------------------------------------------------------------------------
	t.Run("C3_MultiSelectThreeUsersViaTabAndEnter", func(t *testing.T) {
		rv.input.SetText("Team: ")
		rv.input.SetCursorOffset(6)

		modal := NewUserTagModal(mv, members, func(tags []string) {
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)

		// Press Tab 3 times to select Alice, Bob, and Charlie
		tabEvt := tcell.NewEventKey(tcell.KeyTab, 9, 0)
		modal.OnKeyEvent(tabEvt) // Selects Alice (0 -> 1)
		modal.OnKeyEvent(tabEvt) // Selects Bob (1 -> 2)
		modal.OnKeyEvent(tabEvt) // Selects Charlie (2 -> 0)

		if len(modal.GetSelectedUsers()) != 3 {
			t.Fatalf("C3: expected 3 users selected, got %d", len(modal.GetSelectedUsers()))
		}
		selectedOrder := modal.GetSelectedOrder()
		if len(selectedOrder) != 3 ||
			selectedOrder[0] != "@alice:matrix.org" ||
			selectedOrder[1] != "@bob:matrix.org" ||
			selectedOrder[2] != "@charlie:matrix.org" {
			t.Fatalf("C3: unexpected selection order: %v", selectedOrder)
		}

		// Press Enter to confirm
		enterEvt := tcell.NewEventKey(tcell.KeyEnter, 13, 0)
		modal.OnKeyEvent(enterEvt)

		expected := "Team: @Alice @Bob @Charlie "
		if rv.input.GetText() != expected {
			t.Errorf("C3: expected %q, got %q", expected, rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != ComposerStringWidth(expected) {
			t.Errorf("C3: expected cursor at %d, got %d", ComposerStringWidth(expected), rv.input.GetCursorOffset())
		}
	})

	// ------------------------------------------------------------------------
	// C4: Multi-select across search queries: filter -> Tab -> filter -> Tab -> Enter:
	//     both candidates injected in order (F8 + F10 + F14).
	// ------------------------------------------------------------------------
	t.Run("C4_MultiSelectAcrossSearchQueries", func(t *testing.T) {
		rv.input.SetText("Notifying: ")
		rv.input.SetCursorOffset(11)

		modal := NewUserTagModal(mv, members, func(tags []string) {
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)

		// Filter for Alice
		modal.Filter("alice")
		if len(modal.GetMatches()) != 1 {
			t.Fatalf("C4: expected 1 match for 'alice', got %d", len(modal.GetMatches()))
		}
		// Tab to select Alice
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyTab, 9, 0))
		if _, ok := modal.GetSelectedUsers()["@alice:matrix.org"]; !ok {
			t.Fatalf("C4: expected Alice to be selected")
		}

		// Filter for Charlie - Alice MUST survive the filter query change
		modal.Filter("charlie")
		if len(modal.GetMatches()) != 1 {
			t.Fatalf("C4: expected 1 match for 'charlie', got %d", len(modal.GetMatches()))
		}
		if _, ok := modal.GetSelectedUsers()["@alice:matrix.org"]; !ok {
			t.Fatalf("C4: Alice selection lost after filtering for Charlie")
		}

		// Tab to select Charlie
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyTab, 9, 0))
		if _, ok := modal.GetSelectedUsers()["@charlie:matrix.org"]; !ok {
			t.Fatalf("C4: expected Charlie to be selected")
		}

		// Press Enter to confirm both selections
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyEnter, 13, 0))

		expected := "Notifying: @Alice @Charlie "
		if rv.input.GetText() != expected {
			t.Errorf("C4: expected %q, got %q", expected, rv.input.GetText())
		}
	})

	// ------------------------------------------------------------------------
	// C5: Inline @ typed in composer -> modal opened -> user selected -> Enter:
	//     preceding @ replaced without @@ (F15 + F6 + F14).
	// ------------------------------------------------------------------------
	t.Run("C5_InlineAtTriggerReplacement", func(t *testing.T) {
		rv.input.SetText("Reviewing with @")
		rv.input.SetCursorOffset(16)

		modal := NewUserTagModal(mv, members, func(tags []string) {
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)

		// Tab to select Alice, then Enter to confirm
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyTab, 9, 0))
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyEnter, 13, 0))

		if rv.input.GetText() != "Reviewing with @Alice " {
			t.Errorf("C5: expected 'Reviewing with @Alice ', got %q", rv.input.GetText())
		}
	})

	// ------------------------------------------------------------------------
	// C6: Replying to CJK message (truncated with …) and drafting multi-line CJK text:
	//     cursor correctly positioned using runewidth (F3 + F13 + F14).
	// ------------------------------------------------------------------------
	t.Run("C6_CJKReplyTruncationAndMultilineDraftSplicing", func(t *testing.T) {
		cjkBody := "这是一个非常长的中文Matrix消息，用于测试在终端显示下的边界截断和Unicode安全性"
		rv.replying = makeE2EMessageEvent("$c6", "@zhang:matrix.org", event.MsgText, cjkBody)

		status := rv.GetStatus()
		if !strings.Contains(status, "…") {
			t.Errorf("C6: expected truncated snippet with '…', got %q", status)
		}
		snippet := FormatReplySnippet(rv.replying, 30)
		if runewidth.StringWidth(snippet) > 30 {
			t.Errorf("C6: snippet width %d exceeds maxCols 30", runewidth.StringWidth(snippet))
		}

		// Multi-line CJK draft: "第一行文本\n第二行 @"
		draft := "第一行文本\n第二行 @"
		rv.input.SetText(draft)
		rv.input.SetCursorOffset(ComposerStringWidth(draft))

		// Inject tag @Alice at line 2 cursor
		InjectTagsIntoComposer(rv.input, []string{"Alice"})
		expected := "第一行文本\n第二行 @Alice "
		if rv.input.GetText() != expected {
			t.Errorf("C6: expected %q, got %q", expected, rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != ComposerStringWidth(expected) {
			t.Errorf("C6: expected cursor offset %d, got %d", ComposerStringWidth(expected), rv.input.GetCursorOffset())
		}
	})

	// ------------------------------------------------------------------------
	// C7: Replying to multi-line message: newlines flattened into single spaces on
	//     status bar without disrupting composer layout (F4 + F5 + F1).
	// ------------------------------------------------------------------------
	t.Run("C7_MultilineReplyFlattenedAndComposerIntact", func(t *testing.T) {
		multiLineMsg := "Line 1: Summary\nLine 2: Details\n\nLine 3: Multiple   spaces\tand\ttabs"
		rv.replying = makeE2EMessageEvent("$c7", "@dan:matrix.org", event.MsgText, multiLineMsg)

		status := rv.GetStatus()
		if strings.Contains(status, "\n") || strings.Contains(status, "\r") {
			t.Errorf("C7: status bar contains newlines, violating 1-row layout: %q", status)
		}
		// Verify 30-column truncated flattened snippet in status bar
		if !strings.Contains(status, "Line 1: Summary Line 2: Detai…") {
			t.Errorf("C7: status bar does not have flattened single spaces: %q", status)
		}
		// Verify full message is flattened into single spaces when unconstrained
		fullSnippet := FormatReplySnippet(rv.replying, 100)
		expectedFull := "Line 1: Summary Line 2: Details Line 3: Multiple spaces and tabs"
		if fullSnippet != expectedFull {
			t.Errorf("C7: expected full flattened snippet %q, got %q", expectedFull, fullSnippet)
		}

		// Composer layout is unaffected
		rv.input.SetText("Draft with\nnewlines\npreserved")
		if rv.input.GetText() != "Draft with\nnewlines\npreserved" {
			t.Errorf("C7: composer text was corrupted: %q", rv.input.GetText())
		}
	})

	// ------------------------------------------------------------------------
	// C8: Modal canceled with Escape: no tags injected, draft text unchanged,
	//     composer immediately refocused (F11 + F12 + F14).
	// ------------------------------------------------------------------------
	t.Run("C8_ModalCancellationRestoresFocusAndPreservesDraft", func(t *testing.T) {
		rv.input.SetText("Draft text @")
		rv.input.SetCursorOffset(12)

		modal := NewUserTagModal(mv, members, func(tags []string) {
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)
		if mv.modal != modal {
			t.Fatalf("C8: expected modal to be active in MainView")
		}

		// Press Escape to cancel
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyEscape, 27, 0))

		if !modal.IsDismissed() {
			t.Errorf("C8: expected modal to be dismissed")
		}
		if mv.modal != nil {
			t.Errorf("C8: expected modal to be removed from MainView")
		}
		if rv.input.GetText() != "Draft text @" {
			t.Errorf("C8: draft altered after modal escape: %q", rv.input.GetText())
		}
		if mv.focused != mv.roomView {
			t.Errorf("C8: roomView should be focused after modal escape")
		}
	})
}

// ============================================================================
// Tier 4: Real-World Application Scenarios (S1 to S5)
// ============================================================================

func TestE2E_Tier4_RealWorld_Scenarios(t *testing.T) {
	// Scenario S1: Quick Single Mention in Ongoing Discussion
	t.Run("Scenario S1: Quick Single Mention", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Hey @")
		input.SetCursorOffset(5)

		members := []*store.AutocompleteMemberEntry{
			{UserID: "@bob:domain", Displayname: "Bob"},
			{UserID: "@bobby:domain", Displayname: "Bobby"},
		}

		modal := NewUserTagModal(nil, members, func(tags []string) {
			InjectTagsIntoComposer(input, tags)
		})

		// Type search query "bob"
		modal.Filter("bob")
		if len(modal.GetMatches()) != 2 {
			t.Fatalf("S1: expected 2 matches for 'bob', got %d", len(modal.GetMatches()))
		}
		if modal.GetSelectedIndex() != 0 {
			t.Fatalf("S1: expected highlighted index 0, got %d", modal.GetSelectedIndex())
		}

		// Hit Enter directly (exercises dual-confirmation fallback for highlighted candidate)
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyEnter, 13, 0))

		if !modal.IsDismissed() {
			t.Errorf("S1: modal should be dismissed after Enter")
		}

		// Verify inline '@' replacement and non-destructive splicing
		expectedText := "Hey @Bob "
		if input.GetText() != expectedText {
			t.Errorf("S1: expected %q, got %q", expectedText, input.GetText())
		}
		if input.GetCursorOffset() != len(expectedText) {
			t.Errorf("S1: expected cursor at %d, got %d", len(expectedText), input.GetCursorOffset())
		}
	})

	// Scenario S2: Team Announcement Multi-Tagging Across Filters
	t.Run("Scenario S2: Multi-Tagging Across Filters", func(t *testing.T) {
		input := mauview.NewInputArea()
		input.SetText("Team sync: ")
		input.SetCursorOffset(11)

		members := []*store.AutocompleteMemberEntry{
			{UserID: "@alice:domain", Displayname: "Alice (eng)"},
			{UserID: "@bob:domain", Displayname: "Bob (eng)"},
			{UserID: "@carol:domain", Displayname: "Carol (lead)"},
			{UserID: "@dan:domain", Displayname: "Dan (ops)"},
		}

		modal := NewUserTagModal(nil, members, func(tags []string) {
			InjectTagsIntoComposer(input, tags)
		})

		tabKey := tcell.NewEventKey(tcell.KeyTab, 9, 0)
		enterKey := tcell.NewEventKey(tcell.KeyEnter, 13, 0)

		// Step 1: Filter by "eng"
		modal.Filter("eng")
		if len(modal.GetMatches()) != 2 {
			t.Fatalf("S2: expected 2 matches for 'eng', got %d", len(modal.GetMatches()))
		}

		// Tab on Alice (eng) -> toggles checkbox, advances highlight to Bob
		modal.OnKeyEvent(tabKey)
		// Tab on Bob (eng) -> toggles checkbox, advances highlight (wraps)
		modal.OnKeyEvent(tabKey)

		// Verify both are selected in order
		selected := modal.GetSelectedOrder()
		if len(selected) != 2 || selected[0] != "@alice:domain" || selected[1] != "@bob:domain" {
			t.Fatalf("S2: unexpected selected order after eng filter: %v", selected)
		}

		// Step 2: Switch filter to "lead" (Alice and Bob disappear from matches)
		modal.Filter("lead")
		if len(modal.GetMatches()) != 1 {
			t.Fatalf("S2: expected 1 match for 'lead', got %d", len(modal.GetMatches()))
		}

		// CRITICAL: Selections MUST persist across filter changes
		selUsers := modal.GetSelectedUsers()
		if _, ok := selUsers["@alice:domain"]; !ok {
			t.Errorf("S2: Alice selection was lost during filter switch")
		}
		if _, ok := selUsers["@bob:domain"]; !ok {
			t.Errorf("S2: Bob selection was lost during filter switch")
		}

		// Verify visual indicator for Carol
		rendered := modal.GetRenderedText()
		if !strings.Contains(rendered, "[ ] Carol (lead)") {
			t.Errorf("S2: expected unselected Carol in rendered text: %q", rendered)
		}

		// Tab on Carol (lead) -> toggles checkbox
		modal.OnKeyEvent(tabKey)
		if len(modal.GetSelectedOrder()) != 3 {
			t.Fatalf("S2: expected 3 selected users, got %d", len(modal.GetSelectedOrder()))
		}

		// Step 3: Hit Enter to confirm all 3 selections
		modal.OnKeyEvent(enterKey)

		if !modal.IsDismissed() {
			t.Errorf("S2: modal should be dismissed after Enter")
		}

		expected := "Team sync: @Alice (eng) @Bob (eng) @Carol (lead) "
		if input.GetText() != expected {
			t.Errorf("S2: expected %q, got %q", expected, input.GetText())
		}
		if input.GetCursorOffset() != len(expected) {
			t.Errorf("S2: expected cursor at %d, got %d", len(expected), input.GetCursorOffset())
		}
	})

	// Scenario S3: Contextual Reply with Quote Fallback Stripping & Tagging
	t.Run("Scenario S3: Contextual Reply with Quote Stripping & Tagging", func(t *testing.T) {
		rawBody := "> <@dave:server> Hello world\n\nI agree with dave completely."
		evt := makeE2EMessageEvent("$dave_reply", "@dave:server", event.MsgText, rawBody)

		// Step 1: Format snippet and verify quote stripping
		snip := FormatReplySnippet(evt, 30)
		if snip != "I agree with dave completely." {
			t.Errorf("S3: expected quote stripped snippet, got %q", snip)
		}

		// Step 2: Initialize RoomView and MainView
		sim := tcell.NewSimulationScreen("")
		_ = sim.Init()
		rv := &RoomView{
			replying:     evt,
			status:       mauview.NewTextField(),
			statusScreen: &mauview.ProxyScreen{Parent: sim, OffsetY: 22, Height: 1},
			input:        mauview.NewInputArea(),
		}
		mv := &MainView{
			currentRoom: rv,
		}

		// Initial draft in composer
		rv.input.SetText("Replying and adding @")
		rv.input.SetCursorOffset(21)

		// Verify initial status bar contains reply snippet
		status := rv.GetStatus()
		expectedStatusSubstring := `Replying to @dave:server: "I agree with dave completely."`
		if !strings.Contains(status, expectedStatusSubstring) {
			t.Fatalf("S3: status missing reply snippet: %q", status)
		}

		// Step 3: Open UserTagModal, select Carol, and confirm
		members := []*store.AutocompleteMemberEntry{
			{UserID: "@carol:server", Displayname: "Carol"},
			{UserID: "@eve:server", Displayname: "Eve"},
		}

		modal := NewUserTagModal(mv, members, func(tags []string) {
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)

		modal.Filter("carol")
		// Enter on highlighted match (Carol)
		modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyEnter, 13, 0))

		// Step 4: Verify composer injection replaced preceding '@'
		expectedDraft := "Replying and adding @Carol "
		if rv.input.GetText() != expectedDraft {
			t.Errorf("S3: expected composer text %q, got %q", expectedDraft, rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != len(expectedDraft) {
			t.Errorf("S3: expected cursor at %d, got %d", len(expectedDraft), rv.input.GetCursorOffset())
		}

		// Step 5: Verify status bar remains intact and unaffected by tag injection
		statusAfter := rv.GetStatus()
		if !strings.Contains(statusAfter, expectedStatusSubstring) {
			t.Errorf("S3: status bar reply lost after tag injection: %q", statusAfter)
		}

		// Step 6: Verify clearing replying state cleans status bar while preserving composer
		rv.replying = nil
		statusCleared := rv.GetStatus()
		if strings.Contains(statusCleared, "Replying to") {
			t.Errorf("S3: status should no longer indicate replying: %q", statusCleared)
		}
		if rv.input.GetText() != expectedDraft {
			t.Errorf("S3: composer draft altered upon clearing reply: %q", rv.input.GetText())
		}
	})

	// Scenario S4: Interrupted Modal Search and Draft Recovery
	t.Run("Scenario S4: Interrupted Modal Search", func(t *testing.T) {
		rv := &RoomView{
			input: mauview.NewInputArea(),
		}
		mv := &MainView{
			currentRoom: rv,
		}

		rv.input.SetText("Deploying to @")
		rv.input.SetCursorOffset(14)

		members := []*store.AutocompleteMemberEntry{
			{UserID: "@charlie:domain", Displayname: "Charlie"},
		}

		confirmed := false
		modal := NewUserTagModal(mv, members, func(tags []string) {
			confirmed = true
			InjectTagsIntoComposer(rv.input, tags)
		})
		mv.ShowModal(modal)

		// Filter "charlie"
		modal.Filter("charlie")
		if len(modal.GetMatches()) != 1 {
			t.Fatalf("S4: expected 1 match for charlie, got %d", len(modal.GetMatches()))
		}

		// User hits Escape to abort
		handled := modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyEscape, 27, 0))
		if !handled {
			t.Error("S4: expected Escape key to be handled by modal")
		}

		// Modal lifecycle assertions
		if !modal.IsDismissed() {
			t.Errorf("S4: modal should be marked dismissed after Escape")
		}
		if mv.modal != nil {
			t.Errorf("S4: MainView modal should be cleared to nil")
		}
		if confirmed {
			t.Errorf("S4: onConfirm callback should NOT be invoked on Cancel")
		}

		// Draft preservation assertions
		if rv.input.GetText() != "Deploying to @" {
			t.Errorf("S4: draft corrupted, expected 'Deploying to @', got %q", rv.input.GetText())
		}
		if rv.input.GetCursorOffset() != 14 {
			t.Errorf("S4: cursor drifted, expected 14, got %d", rv.input.GetCursorOffset())
		}

		// Subsequent keystrokes must fall through (return false)
		postEventHandled := modal.OnKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'z', 0))
		if postEventHandled {
			t.Errorf("S4: dismissed modal should not intercept subsequent key events")
		}
	})

	// Scenario S5: 1,000-Member Room Selection Stress & Order Persistence
	t.Run("Scenario S5: 1,000-Member Stress", func(t *testing.T) {
		members := make([]*store.AutocompleteMemberEntry, 1000)
		for i := 0; i < 1000; i++ {
			members[i] = &store.AutocompleteMemberEntry{
				UserID:       id.UserID(fmt.Sprintf("@user_%04d:matrix.org", i)),
				Displayname:  fmt.Sprintf("User %04d", i),
				SearchString: fmt.Sprintf("user %04d", i),
			}
		}

		input := mauview.NewInputArea()
		input.SetText("Attention: ")
		input.SetCursorOffset(11)

		var confirmedTags []string
		modal := NewUserTagModal(nil, members, func(tags []string) {
			confirmedTags = tags
			InjectTagsIntoComposer(input, tags)
		})

		// Verify 1,000 members loaded and initial matches = 1000
		if len(modal.GetMatches()) != 1000 {
			t.Fatalf("S5: expected 1000 initial matches, got %d", len(modal.GetMatches()))
		}

		// Verify render cap: production caps rendered candidate rows at 100
		renderedText := modal.GetRenderedText()
		renderedLines := strings.Split(strings.TrimSpace(renderedText), "\n")
		if len(renderedLines) != 100 {
			t.Errorf("S5: expected exactly 100 rendered lines due to render cap, got %d", len(renderedLines))
		}

		tabKey := tcell.NewEventKey(tcell.KeyTab, 9, 0)
		enterKey := tcell.NewEventKey(tcell.KeyEnter, 13, 0)

		// Select User 0000 (currently highlighted at index 0)
		modal.OnKeyEvent(tabKey)

		// Filter for "0500" and select
		modal.Filter("0500")
		if len(modal.GetMatches()) == 0 {
			t.Fatalf("S5: expected matches for '0500'")
		}
		modal.OnKeyEvent(tabKey)

		// Filter for "0999" and select
		modal.Filter("0999")
		if len(modal.GetMatches()) == 0 {
			t.Fatalf("S5: expected matches for '0999'")
		}
		modal.OnKeyEvent(tabKey)

		// Hit Enter to confirm
		modal.OnKeyEvent(enterKey)

		if !modal.IsDismissed() {
			t.Errorf("S5: modal should be dismissed")
		}

		// Verify confirmed tags order
		if len(confirmedTags) != 3 {
			t.Fatalf("S5: expected 3 confirmed tags, got %d: %v", len(confirmedTags), confirmedTags)
		}
		if confirmedTags[0] != "User 0000" || confirmedTags[1] != "User 0500" || confirmedTags[2] != "User 0999" {
			t.Errorf("S5: unexpected tag order: %v", confirmedTags)
		}

		// Verify composer injection
		expectedText := "Attention: @User 0000 @User 0500 @User 0999 "
		if input.GetText() != expectedText {
			t.Errorf("S5: expected %q, got %q", expectedText, input.GetText())
		}
		if input.GetCursorOffset() != len(expectedText) {
			t.Errorf("S5: expected cursor at %d, got %d", len(expectedText), input.GetCursorOffset())
		}
	})
}

// strPtr is a test helper returning pointer to string.
func strPtr(s string) *string {
	return &s
}
