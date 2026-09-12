# TUI Reply Snippet and User Tagging

> Plan ID: `tui-reply-and-user-tags`
> Created: 2026-09-06 · Last reconciled: 2026-09-06
> Working Status: todo
> Deployment: local:todo
> Branch: 
> Location: flow/plans/tui-reply-and-user-tags.md

## References
- Findings: `flow/findings/2026-09-06-tui_improvements/`
- Source: resolved from current context (exploration findings + gotcha coverage)

## Requirement (verbatim)
"3. whenever reply , make it to have preview of the message will be reply instead of just the message id; 4. find out how to be able to tag user as well (could implement it as a modal popup like the search room / search spaces , could tag multiple user as well"

## DOD (Definition of Done)
Plan done when ALL below true:
- [ ] TUI status bar displays a truncated snippet of the message being replied to.
- [ ] User can open a modal popup in TUI to select and tag one or multiple users.
- [ ] Selected tags are injected into the message composer without destroying existing draft text.

## Tasks

### Reply Snippet
- [ ] reply-nil-guards: `view.replying.GetMautrixContent()` and `.AsMessage()` calls are wrapped in nil checks to prevent crashes on non-message events.
- [ ] reply-fallback: Redacted events, images, and audio messages render contextual fallbacks (`[Image]`, `[Redacted]`) instead of empty snippets.
- [ ] reply-strip-quote: Nested Matrix reply fallback blocks (`> `) are stripped from the snippet so the actual message content is shown.
- [ ] reply-truncate: Snippet is safely truncated using `runewidth.Truncate` to 30 display columns to prevent UTF-8 boundary panics and layout blowouts.
- [ ] reply-normalize: Newlines and extra whitespace in the snippet are replaced with single spaces to maintain the 1-row status bar constraint.

### User Tagging Modal
- [ ] tag-modal-ui: A new `UserTagModal` exists, wrapping `FuzzyPickerModal` semantics, and is bound to a hotkey or triggered by `@` in the input area.
- [ ] tag-multi-select: Modal supports multi-selection using `Tab` (or similar non-space key) to toggle items, tracking selections by `UserID`.
- [ ] tag-visuals: Modal list visually distinguishes selected vs unselected items (e.g. prefixing with `[x]` / `[ ]`).
- [ ] tag-limit-pool: Modal restricts the candidate member pool (e.g. active members only) to prevent main-thread freezing on 50k+ member rooms.
- [ ] tag-submit-semantics: Pressing `Enter` confirms explicitly toggled items; if none are toggled, it confirms the currently highlighted item.
- [ ] tag-focus-restore: Closing or confirming the modal explicitly calls `view.input.Focus()` to prevent input freezing.

### Composer Injection
- [ ] tag-splice: Confirmed tags are spliced into `view.input` at the current cursor offset, preserving the user's existing draft text.
- [ ] tag-double-at: Injection detects if the character immediately preceding the cursor is `@` and replaces it to avoid `@@username`.
- [ ] tag-delimiter: Multiple injected tags are joined with single spaces and end with a trailing space (`@alice @bob `).
- [ ] tag-mentions-payload: The message dispatch pipeline correctly parses injected display names / MXIDs to populate `m.mentions.user_ids` (MSC3952) so users receive notifications.

## Idempotency
Re-running `/10-plan-declarative` on same requirement reconciles to THIS plan.
Implemented items auto-marked `- [x]`. Pending items surface as work-remaining.
DO NOT rewrite item prose on re-run (status flips only).

## Open Threads
_(populated by /20-plan-verify-gotcha + re-runs)_
