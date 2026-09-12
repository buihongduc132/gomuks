# Gomuks TUI E2E Test Infrastructure Specification

## 1. Overview & Architecture

This document specifies the End-to-End (E2E) testing infrastructure for the Gomuks TUI client enhancements:
- **R1: Truncated Reply Snippet in Status Bar** (Features F1–F5)
- **R2: Multi-Select User Tagging Modal** (Features F6–F12)
- **R3: Non-Destructive Composer Tag Injection** (Features F13–F16)
- **M4: E2E Pass & Adversarial Hardening** (Feature F17)

The infrastructure follows the **4-Tier Testing Methodology** ensuring exhaustive validation from atomic feature behaviors up to multi-step real-world interactive user flows:
1. **Tier 1: Feature Coverage** (>=5 test cases per feature, total >=80 cases)
2. **Tier 2: Boundary & Corner Cases** (>=5 test cases per feature, total >=80 cases)
3. **Tier 3: Cross-Feature Combinations** (Pairwise interaction across R1, R2, and R3)
4. **Tier 4: Real-World Application Scenarios** (Multi-step end-to-end user workflows)

---

## 2. Test Execution Harness Architecture

### 2.1 Headless TUI Terminal Simulation
The test infrastructure simulates a full terminal screen using `github.com/gdamore/tcell/v2` simulation screens (`tcell.NewSimulationScreen("")`).
- Standard simulation dimensions: **80×24** and **120×30**.
- Enables complete rendering of `MainView`, `RoomView`, `mauview.Flex`, `mauview.Center`, `UserTagModal`, and `mauview.InputArea`.
- Verifies screen buffer contents by scraping runes and styles cell-by-cell without requiring a physical terminal emulator or PTY process.

### 2.2 Component Hierarchy & Wiring
```
MainView (mauview.Flex)
├── RoomList (Left pane, width 25)
├── RoomView (Main pane)
│   ├── TopicBar (Row 0, height 1)
│   ├── MessageView / Content (Rows 1..21)
│   ├── StatusBar (Row 22, height 1) -> Displays "Replying to <sender>: \"<snippet>\""
│   └── InputArea (Row 23, height 1) -> Message Composer with cursor runewidth tracking
└── Modal Layer (mauview.Center overlay)
    └── UserTagModal (Flex Box)
        ├── Search InputArea (Query input, Space pass-through)
        └── Results TextView (Region tags, [x] vs [ ] indicators)
```

### 2.3 Opaque-Box Event Injection
Tests dispatch real `tcell.EventKey` events through `mauview.Component.OnKeyEvent()`:
- `tcell.KeyRune`: Printable characters (including `' '` for search queries, `'@'` for inline triggers).
- `tcell.KeyTab`: Multi-select checkbox toggling and candidate advancing.
- `tcell.KeyDown` / `tcell.KeyUp`: Candidate highlight cycling without toggling.
- `tcell.KeyEnter`: Dual-confirmation (toggled set if non-empty; highlighted candidate fallback if empty).
- `tcell.KeyEscape`: Modal cancellation and composer focus restoration.

### 2.4 Assertion Engines
- **Screen Buffer Scraping**: Inspects cells on Row 22 (status bar) and modal regions to verify exact visual rendering.
- **Runewidth & Layout Validation**: Asserts that status bar content never exceeds 1 row (`StatusBarHeight = 1`) and strings are properly truncated with `…` respecting double-width CJK/Emoji characters.
- **Buffer & Cursor Integrity**: Asserts that `InputArea.GetText()` preserves existing drafts and `InputArea.GetCursorOffset()` correctly tracks runewidth offsets (`\n` width = 1).

---

## 3. 4-Tier Test Matrix

### Tier 1: Feature Coverage (>=5 test cases per feature)

#### Feature F1: Reply Snippet Formatting
- `T1-F1.1`: Standard plain text message (`"Hello world"`) formats into snippet `"Hello world"`.
- `T1-F1.2`: Nil `database.Event` pointer safely returns empty snippet `""` without crashing.
- `T1-F1.3`: State event (`evt.StateKey != nil`, e.g. `m.room.member`) returns empty or safe fallback without dereferencing `.Body`.
- `T1-F1.4`: Redacted event (`evt.RedactedBy != ""`) returns `"[Redacted]"` or safe fallback.
- `T1-F1.5`: Decryption failure event (`evt.DecryptionError != ""`) returns `"[Decryption error]"` or safe fallback.
- `T1-F1.6`: Media attachment events (`m.image`, `m.video`, `m.audio`, `m.file`) return contextual fallback (e.g. `"[Image]"`).

#### Feature F2: Matrix Quote Stripping
- `T1-F2.1`: Single quote block (`"> <@user:domain> original text\n\nmy reply"`) strips quote to `"my reply"`.
- `T1-F2.2`: Multi-line quote (`"> line 1\n> line 2\n\nreply text"`) strips all quoted lines to `"reply text"`.
- `T1-F2.3`: Empty quote line (`">\nreply text"`) strips quote line to `"reply text"`.
- `T1-F2.4`: Nested quote (`"> > nested\n> reply\n\nactual message"`) strips nested quotes to `"actual message"`.
- `T1-F2.5`: Message without quote (`"Direct message without quotes"`) returns text unchanged.

#### Feature F3: Unicode Truncation & Runewidth
- `T1-F3.1`: Short ASCII text (<=30 cols) retains full text without ellipsis.
- `T1-F3.2`: Long ASCII text (>30 cols) truncates to exactly 30 columns ending in `…`.
- `T1-F3.3`: Double-width CJK characters (15 chars = 30 cols) fit exactly without truncation.
- `T1-F3.4`: Double-width CJK characters (16 chars = 32 cols) truncate safely to 14 chars + `…` (29 or 30 cols) without cutting a rune.
- `T1-F3.5`: Emoji sequences (e.g. `👋🎉`) with multi-byte UTF-8 representations truncate without byte splitting.

#### Feature F4: Newline Normalization
- `T1-F4.1`: Single newline (`"line1\nline2"`) normalizes to `"line1 line2"`.
- `T1-F4.2`: Windows CRLF (`"line1\r\nline2"`) normalizes to `"line1 line2"`.
- `T1-F4.3`: Multiple consecutive newlines (`"line1\n\n\nline2"`) collapse to single space `"line1 line2"`.
- `T1-F4.4`: Mixed tabs, spaces, and newlines (`"line1 \t \n  line2"`) collapse into single space.
- `T1-F4.5`: Leading and trailing whitespace and newlines are stripped cleanly.

#### Feature F5: Status Bar Display Integration
- `T1-F5.1`: When `view.replying != nil`, `GetStatus()` appends `: "<snippet>"` to `"Replying to <sender>"`.
- `T1-F5.2`: When `view.replying == nil`, status bar displays standard status without reply prefix.
- `T1-F5.3`: When `view.Room == nil` or `view.Room.Typing == nil`, `GetStatus()` does not panic.
- `T1-F5.4`: Status bar text drawn into `view.status` (`mauview.TextField`) fits on 1 terminal row.
- `T1-F5.5`: When snippet is empty, status bar displays `Replying to <sender> - ` without empty quotes `""`.

#### Feature F6: User Tagging Modal Structure
- `T1-F6.1`: `NewUserTagModal` instantiates with parent `MainView`, member list, and callback.
- `T1-F6.2`: Modal contains search `InputArea` and results `TextView`.
- `T1-F6.3`: Modal is centered on screen wrapped in `mauview.Center`.
- `T1-F6.4`: `parent.ShowModal(modal)` places modal into `mainView.modal` and gives it focus.
- `T1-F6.5`: `parent.HideModal()` removes modal from `mainView.modal`.

#### Feature F7: Room Member Retrieval & Escaping
- `T1-F7.1`: Candidate members retrieved from `store.RoomStore.GetMembers()`.
- `T1-F7.2`: Member display name parsed, falling back to User ID localpart if empty.
- `T1-F7.3`: Display names with square brackets (e.g. `Alice [Mod]`) are escaped via `mauview.Escape`.
- `T1-F7.4`: Only joined/invited members (`MembershipJoin`, `MembershipInvite`) are populated.
- `T1-F7.5`: Candidates with duplicate display names render with disambiguating User ID `(@user:domain)`.

#### Feature F8: Multi-Select User ID Tracking
- `T1-F8.1`: Toggle user toggles presence in `map[id.UserID]struct{}`.
- `T1-F8.2`: Multiple users toggled preserves all IDs in `selectedOrder []id.UserID`.
- `T1-F8.3`: User types search filter: pre-selected users remain selected even when filtered out.
- `T1-F8.4`: Search filter cleared: pre-selected users reappear with `[x]` indicator checked.
- `T1-F8.5`: Repeated toggling of the same user adds and removes ID from set cleanly.

#### Feature F9: Visual Selection Indicators
- `T1-F9.1`: Unselected candidates display prefix `[ ] `.
- `T1-F9.2`: Selected candidates display prefix `[x] `.
- `T1-F9.3`: Toggling candidate updates `[ ] ` to `[x] ` immediately on screen re-render.
- `T1-F9.4`: Active highlighted row is visually differentiated from non-highlighted rows.
- `T1-F9.5`: Result list rendering capped at 100 items to prevent UI lag.

#### Feature F10: Keyboard Routing & Space Handling
- `T1-F10.1`: Printable character keys are appended to search `InputArea` query.
- `T1-F10.2`: Pressing `Space` (`tcell.KeyRune ' '`) types a space into search query without toggling selection.
- `T1-F10.3`: Pressing `Tab` (`tcell.KeyTab`) toggles selection checkbox on highlighted candidate.
- `T1-F10.4`: Pressing `Tab` advances highlight index to next candidate.
- `T1-F10.5`: Pressing `Down` / `Up` arrow keys navigates highlight without altering selection states.

#### Feature F11: Dual Confirmation Semantics
- `T1-F11.1`: Pressing `Enter` when >=1 items toggled confirms all toggled items in selection order.
- `T1-F11.2`: Pressing `Enter` when 0 items toggled confirms currently highlighted candidate.
- `T1-F11.3`: Pressing `Enter` when 0 candidates match search query acts as no-op or closes safely.
- `T1-F11.4`: Pressing `Escape` cancels modal without invoking confirmation callback.
- `T1-F11.5`: Confirmation callback receives formatted display tags or IDs as `[]string`.

#### Feature F12: Focus Restoration
- `T1-F12.1`: Confirming modal via `Enter` restores focus to `RoomView.input`.
- `T1-F12.2`: Canceling modal via `Escape` restores focus to `RoomView.input`.
- `T1-F12.3`: While modal is open, keystrokes are routed to modal, not `RoomView.input`.
- `T1-F12.4`: `RoomView.Focus()` delegates to `view.input.Focus()`.
- `T1-F12.5`: Subsequent keyboard typing immediately goes into the composer buffer.

#### Feature F13: Runewidth-Aware Buffer Partitioning
- `T1-F13.1`: Partition ASCII text at cursor offset into `textBefore` and `textAfter`.
- `T1-F13.2`: Partition buffer at cursor offset 0 (start of buffer).
- `T1-F13.3`: Partition buffer at end of buffer (cursor offset = string width).
- `T1-F13.4`: Partition buffer containing double-width CJK characters without splitting runes.
- `T1-F13.5`: Partition multi-line buffer where newline `\n` has runewidth 1 matching `mauview.InputArea`.

#### Feature F14: Non-Destructive Tag Splicing
- `T1-F14.1`: Inject tag into empty buffer produces `@user `.
- `T1-F14.2`: Inject tag at end of buffer `"Hello "` produces `"Hello @user "`.
- `T1-F14.3`: Inject tag in middle of buffer `"Start end"` at cursor 6 produces `"Start @user end"`.
- `T1-F14.4`: Inject multiple tags produces joined string with trailing space (e.g. `"@alice @bob "`).
- `T1-F14.5`: Text before and text after cursor are completely preserved without data loss.

#### Feature F15: Inline `@` Trigger Replacement
- `T1-F15.1`: Splicing with cursor immediately following `@` (`"Hello @"`) replaces `@` -> `"Hello @user "`.
- `T1-F15.2`: Splicing with cursor at lone `@` (`"@"`) replaces `@` -> `"@user "`.
- `T1-F15.3`: Splicing with cursor in middle following `@` (`"Contact @ now"`) replaces `@` -> `"Contact @user now"`.
- `T1-F15.4`: Splicing with cursor not following `@` (`"Hello"`) prepends `@` -> `"Hello @user "`.
- `T1-F15.5`: Inline `@` replacement prevents duplicate `@@user`.

#### Feature F16: Keybinding & Trigger Integration
- `T1-F16.1`: `keybindings.yaml` config contains `tag_user` keybinding under `room:`.
- `T1-F16.2`: `RoomView.OnKeyEvent` triggers modal opening on `tag_user` key event.
- `T1-F16.3`: `RoomView.OpenUserTagModal()` opens modal with current room members.
- `T1-F16.4`: Room view with nil room store handles `OpenUserTagModal()` gracefully without crashing.
- `T1-F16.5`: Confirming modal invokes `InjectTagsIntoComposer` and updates cursor runewidth.

---

### Tier 2: Boundary & Corner Cases (>=5 test cases per feature)

#### Boundary B1: Reply Snippet Formatting
- `T2-B1.1`: `database.Event` with `Content = nil` and `Decrypted = nil`.
- `T2-B1.2`: Event with empty string body `""` returns empty snippet.
- `T2-B1.3`: Event with only non-printable / control characters (ASCII 0x01..0x1F).
- `T2-B1.4`: Event with massive text body (1,000,000 characters) does not exhaust memory or CPU.
- `T2-B1.5`: Notice event (`m.notice`) formats cleanly with body snippet.

#### Boundary B2: Matrix Quote Stripping
- `T2-B2.1`: Quote-only message (`"> quote with no reply"`) strips `>` prefix rather than returning empty string.
- `T2-B2.2`: Quote with spaces before `>` (e.g. `"   > indented quote"`).
- `T2-B2.3`: Multiple distinct quote blocks separated by text (`"> q1\n\nt1\n\n> q2\n\nt2"`).
- `T2-B2.4`: Message containing math operators or arrows (e.g. `"x > y and a -> b"`).
- `T2-B2.5`: Message consisting only of `>` characters (`">>>\n>>\n>"`).

#### Boundary B3: Unicode Truncation & Runewidth
- `T2-B3.1`: Truncating string where 30th column falls in the middle of a 2-column CJK rune.
- `T2-B3.2`: Truncating string where 30th column falls on a 4-byte UTF-8 emoji codepoint.
- `T2-B3.3`: String with Zero Width Joiner (ZWJ) family emoji sequences.
- `T2-B3.4`: String with Right-to-Left (RTL) Arabic/Hebrew script mixed with LTR.
- `T2-B3.5`: Max columns set to boundary values: `maxCols = 0`, `maxCols = 1`, `maxCols = 2`.

#### Boundary B4: Newline Normalization
- `T2-B4.1`: String consisting entirely of newlines (`"\n\n\n\n"`).
- `T2-B4.2`: Mixed newline representations (`"\r\n\n\r\n"`).
- `T2-B4.3`: Unicode line break characters (U+2028 LINE SEPARATOR, U+2029 PARAGRAPH SEPARATOR).
- `T2-B4.4`: Zero-width spaces (`\u200b`) mixed with newlines.
- `T2-B4.5`: Trailing and leading newlines with internal whitespace preserved as single spaces.

#### Boundary B5: Status Bar Display Integration
- `T2-B5.1`: Rapid reply selection toggling (reply -> clear -> reply -> send).
- `T2-B5.2`: Replying to a sender with a 100-character Matrix ID (`@verylonguser...:longdomain.com`).
- `T2-B5.3`: Status bar display when room is in message selection mode (`view.selecting = true`).
- `T2-B5.4`: Status bar display with simultaneous typing notification and reply snippet.
- `T2-B5.5`: Screen width narrowed to 20 columns: status bar truncates gracefully without panic.

#### Boundary B6: User Tagging Modal Structure
- `T2-B6.1`: Initializing modal with `nil` member slice (0 candidates).
- `T2-B6.2`: Initializing modal with 50,000 members (performance stress test).
- `T2-B6.3`: Terminal window resized while modal is active.
- `T2-B6.4`: Closing modal when modal was never shown.
- `T2-B6.5`: Opening modal while another modal is already active on `MainView`.

#### Boundary B7: Room Member Retrieval & Escaping
- `T2-B7.1`: Member with empty string display name and empty localpart.
- `T2-B7.2`: Member whose display name contains raw markup/color codes (e.g. `[red]Alice[white]`).
- `T2-B7.3`: Member whose display name contains terminal escape sequences (ANSI CSI).
- `T2-B7.4`: Member with all whitespace display name (`"   "`).
- `T2-B7.5`: Member with extreme length display name (500 characters).

#### Boundary B8: Multi-Select User ID Tracking
- `T2-B8.1`: User rapidly selected, deselected, and reselected across 10 filter queries.
- `T2-B8.2`: All room members selected simultaneously.
- `T2-B8.3`: Selection map checked for memory leaks on modal reset.
- `T2-B8.4`: Non-existent User ID toggle attempt.
- `T2-B8.5`: Duplicate User IDs in member list handled idempotently.

#### Boundary B9: Visual Selection Indicators
- `T2-B9.1`: Member name starting with `[x]` (e.g. `[x] Fake Selected`) rendered as `[ ] [x] Fake Selected`.
- `T2-B9.2`: Selection indicator at list view boundary (item 0, item 99, item 100).
- `T2-B9.3`: Display in high-contrast or inverted color mode.
- `T2-B9.4`: Visual indicator with zero matches rendered.
- `T2-B9.5`: Rapid navigation causing fast redraw cycles without tearing.

#### Boundary B10: Keyboard Routing & Space Handling
- `T2-B10.1`: Typing multiple consecutive spaces into search query (`"alice  bob"`).
- `T2-B10.2`: Pressing `Tab` on empty search result list.
- `T2-B10.3`: Pressing `Backtab` (Shift+Tab) behavior.
- `T2-B10.4`: Pressing `Ctrl+C` or `Ctrl+D` inside search input.
- `T2-B10.5`: Pressing non-ASCII / dead keys in search input.

#### Boundary B11: Dual Confirmation Semantics
- `T2-B11.1`: Pressing `Enter` when search filter yields 0 matches and 0 items toggled.
- `T2-B11.2`: Pressing `Enter` when items were toggled, but current search filter hides all of them.
- `T2-B11.3`: Double pressing `Enter` within 1ms.
- `T2-B11.4`: Pressing `Enter` after unchecking all items.
- `T2-B11.5`: Calling confirmation callback when callback is `nil`.

#### Boundary B12: Focus Restoration
- `T2-B12.1`: Modal closed when parent `MainView` has nil `roomView`.
- `T2-B12.2`: Modal closed when `RoomView.input` was disabled.
- `T2-B12.3`: Double `HideModal()` invocation without panic.
- `T2-B12.4`: Modal closed while user list on right pane was focused.
- `T2-B12.5`: Focus restored properly after 100 consecutive modal open/close cycles.

#### Boundary B13: Runewidth-Aware Buffer Partitioning
- `T2-B13.1`: Partitioning string at negative cursor offset (`cursor = -5`).
- `T2-B13.2`: Partitioning string at cursor offset greater than total width (`cursor = 99999`).
- `T2-B13.3`: Partitioning multi-line string containing only `\n` characters.
- `T2-B13.4`: Partitioning string containing emoji with skin tone modifiers.
- `T2-B13.5`: Partitioning string where cursor falls between `\r` and `\n`.

#### Boundary B14: Non-Destructive Tag Splicing
- `T2-B14.1`: Splicing into draft of 100,000 characters without truncation.
- `T2-B14.2`: Splicing 100 user tags simultaneously into a single line.
- `T2-B14.3`: Splicing tags containing special punctuation (`@user.name-123`).
- `T2-B14.4`: Splicing into middle of a URL in draft text.
- `T2-B14.5`: Splicing when cursor is positioned at start of line 2 in multi-line draft.

#### Boundary B15: Inline `@` Trigger Replacement
- `T2-B15.1`: Draft `"user@@"` with cursor after second `@` replaces only the second `@`.
- `T2-B15.2`: Draft with cursor after `@` at start of line 2 (`"line1\n@"`).
- `T2-B15.3`: Draft where `@` is preceded by non-space punctuation (`"(@"`).
- `T2-B15.4`: Draft where cursor is before `@` (`cursor = 0`, text = `"@"`): does not replace.
- `T2-B15.5`: Draft with full-width CJK `@` (U+FF20 ＠) behavior.

#### Boundary B16: Keybinding & Trigger Integration
- `T2-B16.1`: `tag_user` key event received while composer already contains full draft.
- `T2-B16.2`: Triggering modal in room with 0 messages and 1 member (self).
- `T2-B16.3`: Triggering modal while client is syncing (`view.syncingModal != nil`).
- `T2-B16.4`: Custom keybinding with modifiers (e.g. `Ctrl+Alt+T`).
- `T2-B16.5`: Key event with unknown modifier ignored safely.

---

### Tier 3: Cross-Feature Combinations (Pairwise Coverage)

| Combo ID | Features | Description |
|----------|----------|-------------|
| `C1` | F1 + F6 | Setting reply on message then opening tagging modal: status bar snippet is preserved underneath modal overlay. |
| `C2` | F1 + F14 | Setting reply on message then injecting user tags: status bar maintains reply state while composer buffer updates. |
| `C3` | F6 + F14 | Opening modal, selecting 3 users via Tab, pressing Enter: all 3 tags spliced with trailing space into composer. |
| `C4` | F8 + F10 + F14 | Multi-select across search queries: filter -> Tab -> filter -> Tab -> Enter: both candidates injected in order. |
| `C5` | F15 + F6 + F14 | Inline `@` typed in composer -> modal opened -> user selected -> Enter: preceding `@` replaced without `@@`. |
| `C6` | F3 + F13 + F14 | Replying to CJK message (truncated with `…`) and drafting multi-line CJK text: cursor correctly positioned using runewidth. |
| `C7` | F4 + F5 + F1 | Replying to multi-line message: newlines flattened into single spaces on status bar without disrupting composer layout. |
| `C8` | F11 + F12 + F14 | Modal canceled with Escape: no tags injected, draft text unchanged, composer immediately refocused. |

---

### Tier 4: Real-World Application Scenarios

#### Scenario S1: Quick Single Mention in Ongoing Discussion
1. User enters active room.
2. User types draft `"Can you look at this, @"`.
3. User triggers User Tagging Modal.
4. User types `"bob"` in search query.
5. User presses `Enter` without pressing `Tab` (triggering highlighted candidate fallback).
6. Modal closes, preceding `@` is replaced, composer text becomes `"Can you look at this, @bob "`.
7. Composer receives focus, cursor is placed after `@bob `.
8. User hits `Enter` to send message.

#### Scenario S2: Team Announcement Multi-Tagging Across Filters
1. User drafts announcement `"Team sync today: "`.
2. User opens tagging modal via hotkey (`Ctrl+t`).
3. User types `"eng"` in search filter -> results show `Alice (eng)`, `Bob (eng)`.
4. User presses `Tab` on Alice -> `[x] Alice (eng)`.
5. User presses `Tab` on Bob -> `[x] Bob (eng)`.
6. User clears search filter and types `"lead"` -> results show `Carol (lead)`.
7. User presses `Tab` on Carol -> `[x] Carol (lead)`.
8. User presses `Enter`.
9. Modal closes and composer text becomes `"Team sync today: @Alice @Bob @Carol "`.
10. Cursor placed at end of buffer, composer refocused.

#### Scenario S3: Contextual Reply with Quote Fallback Stripping & Tagging
1. Message in timeline contains Matrix quote fallback: `"> <@dave:server> Hello\n\nI agree with dave"`.
2. User selects message to reply.
3. Status bar renders: `Replying to @sender: "I agree with dave" - `.
4. User types `"cc @"`.
5. User opens tagging modal, selects `Dave`.
6. Tag injected -> `"cc @Dave "`.
7. Status bar still displays `Replying to @sender: "I agree with dave" - `.
8. User sends message -> reply is dispatched, status bar clears, composer clears.

#### Scenario S4: Interrupted Modal Search and Draft Recovery
1. User types draft `"Deploying to @"`.
2. User opens tagging modal.
3. User searches `"charlie"` and realizes Charlie is unavailable.
4. User presses `Escape` to dismiss modal.
5. Composer text remains `"Deploying to @"` with cursor at offset 14.
6. Composer has focus immediately; user presses `Backspace` and continues drafting.

#### Scenario S5: High-Density Room Stress & Selection Persistence
1. Room contains 1,000 members.
2. User opens tagging modal.
3. Results list is verified to render in <16ms and capped at 100 visible items.
4. User selects member #1 via `Tab`.
5. User searches for member #500 and selects via `Tab`.
6. User clears search and selects member #999 via `Tab`.
7. User confirms with `Enter`.
8. All 3 members are injected in exact selection order with valid delimiters.

---

## 4. Progressive Testability & Harness Decoupling

During milestone progression:
- **Milestone 1**: `tui/reply-snippet.go` and `RoomView.GetStatus()` are validated directly.
- **Milestone 2**: `tui/user-tag-modal.go` is validated through modal lifecycle and keyboard routing tests.
- **Milestone 3**: `tui/composer-injection.go` is validated through cursor runewidth and non-destructive splicing tests.
- **Milestone 4**: All 4 tiers execute end-to-end against integrated production components.

The E2E test suite in `tui/e2e_test.go` uses an integrated test harness providing:
1. Reference specification oracles for mathematical output derivation.
2. Seamless binding to package-level implementations as they become available.
3. Full compatibility with standard `go test ./tui/...`.
