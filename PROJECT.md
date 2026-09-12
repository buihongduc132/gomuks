# Project: Gomuks TUI Truncated Reply Snippet & Multi-Select User Tagging Modal

## Architecture
Gomuks TUI is built in Go on top of `go.mau.fi/mauview` and `github.com/gdamore/tcell/v2`.
- **TUI Core**: `cmd/gomuks-terminal/`, `tui/` (`MainView`, `RoomView`, `MessageView`).
- **Data Models**: `pkg/hicli/database/event.go` (`database.Event`), `pkg/rpc/store/room.go` (`store.AutocompleteMemberEntry`).
- **Widgets**:
  - Status Bar: `RoomView.status` (`mauview.TextField`, 1-row status bar `StatusBarHeight = 1`).
  - Modals: `UserTagModal` wrapping `mauview.InputArea` and `mauview.TextView` inside `mauview.Flex` and `mauview.Center`, displayed via `MainView.ShowModal()`.
  - Composer: `RoomView.input` (`mauview.InputArea`, cursor tracked in runewidth units where `\n` has width 1).

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | Reply Snippet Formatting | Format plain-text snippet from `database.Event` with nil guards for state events, redactions, decryption failures, media | M1 | ORIGINAL_REQUEST §R1 |
| F2 | Matrix Quote Stripping | Strip reply fallbacks (`> <@user:domain> ...\n\n`), empty quotes `>\n`, and nested quotes | M1 | ORIGINAL_REQUEST §R1 |
| F3 | Unicode Truncation & Runewidth | Safely truncate with `…` respecting terminal column width (CJK, emojis, UTF-8 boundaries) via `go-runewidth` | M1 | ORIGINAL_REQUEST §R1 |
| F4 | Newline Normalization | Flatten all newlines and multiple whitespace into single spaces for 1-row status bar | M1 | ORIGINAL_REQUEST §R1 |
| F5 | Status Bar Display | Integrate snippet into `RoomView.GetStatus()` with defensive nil guards | M1 | ORIGINAL_REQUEST §R1 |
| F6 | User Tagging Modal Structure | `UserTagModal` widget with `mauview.InputArea` (search) and `mauview.TextView` (region tags) | M2 | ORIGINAL_REQUEST §R2 |
| F7 | Room Member Retrieval | Retrieve cached room members via `view.Room.GetMembers()` with display name escaping | M2 | ORIGINAL_REQUEST §R2 |
| F8 | Multi-Select User ID Tracking | Track selections in `map[id.UserID]struct{}` + ordered slice so selections survive search filtering | M2 | ORIGINAL_REQUEST §R2 |
| F9 | Visual Selection Indicators | Prefix candidate rows with `[x] ` vs `[ ] ` and cap rendered list to avoid UI lag | M2 | ORIGINAL_REQUEST §R2 |
| F10 | Keyboard Routing & Space Handling | Allow `Space` in search query without toggling; `Tab` toggles checkbox & advances highlight | M2 | ORIGINAL_REQUEST §R2 |
| F11 | Dual Confirmation Semantics | `Enter` confirms toggled items (or highlighted fallback if none toggled); `Escape` cancels | M2 | ORIGINAL_REQUEST §R2 |
| F12 | Focus Restoration | Explicitly refocus `view.input` on modal dismissal to prevent composer lockup | M2 | ORIGINAL_REQUEST §R2 |
| F13 | Runewidth-Aware Buffer Partitioning | Split composer text at cursor runewidth offset using `substringBefore` congruent with `mauview` | M3 | ORIGINAL_REQUEST §R3 |
| F14 | Non-Destructive Tag Splicing | Splice formatted user tags (`@alice @bob `) into composer at cursor without wiping draft | M3 | ORIGINAL_REQUEST §R3 |
| F15 | Inline `@` Trigger Replacement | Replace preceding `@` trigger at cursor position rather than producing `@@` | M3 | ORIGINAL_REQUEST §R3 |
| F16 | Keybinding & Trigger Integration | Wire `tag_user` keybind in `keybindings.yaml` and `RoomView.OnKeyEvent` to launch modal | M3 | ORIGINAL_REQUEST §R3 |
| F17 | Comprehensive Test Suite & Hardening | Complete E2E tests (Tiers 1-4) and Tier 5 adversarial hardening | M4 | ORIGINAL_REQUEST Acceptance Criteria |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Reply Snippet Safe Truncation | F1, F2, F3, F4, F5 | none | DONE |
| M2 | Multi-Select User Tagging Modal | F6, F7, F8, F9, F10, F11, F12 | none | DONE |
| M3 | Composer Tag Injection & Wiring | F13, F14, F15, F16 | M2 | DONE |
| M4 | E2E Pass & Adversarial Hardening | F17 (Tiers 1-4 100% pass + Tier 5 Hardening) | M1, M2, M3 | DONE |

## Interface Contracts
### M1: Reply Snippet Helper (`tui/reply-snippet.go` or `tui/room-view.go`)
- `FormatReplySnippet(evt *database.Event, maxCols int) string`
  - Input: `evt *database.Event` (may be nil, state event, redacted, undecrypted, media, or message), `maxCols int` (e.g. 30).
  - Output: Safely truncated single-line plain text snippet respecting `maxCols` terminal columns, ending in `…` if truncated.
- `StripReplyFallback(text string) string`
  - Input: Raw message body with potential Matrix quote lines.
  - Output: Plain text message content with quote fallback block removed.
- Status Bar Integration:
  - In `RoomView.GetStatus()`: when `view.replying != nil`, append `: "<snippet>"` to the reply status line.

### M2: User Tagging Modal (`tui/user-tag-modal.go`)
- `NewUserTagModal(parent *MainView, members []*store.AutocompleteMemberEntry, onConfirm func(selectedUsers []string)) *UserTagModal`
  - Tracks selections by `id.UserID` in `map[id.UserID]struct{}`.
  - Maintains `selectedOrder []id.UserID`.
  - On confirm, calls `onConfirm` with formatted display tags (or usernames/MXIDs).
  - On dismiss, calls `parent.HideModal()` and ensures composer `Focus()`.

### M3: Composer Injection (`tui/composer-injection.go`)
- `InjectTagsIntoComposer(input *mauview.InputArea, tags []string)`
  - Slices buffer at `input.GetCursorOffset()`.
  - Replaces preceding `@` if present.
  - Injects formatted tags (`@user1 @user2 `).
  - Updates `input.SetText(...)`, updates `input.SetCursorOffset(...)`, and calls `input.Focus()`.
- Room Keybind:
  - `tag_user` in `keybindings.yaml` triggers `RoomView.OpenUserTagModal()`.

## Code Layout
- `tui/reply-snippet.go`: Reply snippet formatting, quote stripping, rune truncation, and tests.
- `tui/user-tag-modal.go`: Multi-select user tagging modal component and unit tests.
- `tui/composer-injection.go`: Composer text splicing, cursor placement, trigger replacement.
- `tui/room-view.go`: Integration of status bar reply snippet, modal invocation, and composer focus.
- `tui/config/keybindings.yaml`: Default keybinding for `tag_user`.
- `tui/room_view_test.go`: Tests for reply snippet formatting and status bar rendering.
- `tui/user_tag_modal_test.go`: Tests for modal search, multi-selection, space key routing, and selection persistence.
- `tui/composer_injection_test.go`: Tests for composer injection, splicing, inline `@` replacement, and multi-line cursor positioning.
- `tui/tier5_adversarial_test.go`: Permanent Tier 5 adversarial regression tests for invalid UTF-8 splicing, Ctrl+A routing, concurrent focus/blur, and Unicode scale.
- `tui/e2e_test.go`: Comprehensive E2E test suite covering Tiers 1-4 across all features.

