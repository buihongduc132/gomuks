# Gotcha Coverage — tui_improvements

> Source: flow/findings/2026-09-06-tui_improvements/
> Mode: findings
> Sub-agent: self (Gotcha Reviewer)
> Units reviewed: OT1, Turn 1 & 2 Conclusion (Reply Snippet), Turn 1 & 2 Conclusion (User Tag Modal)

## Findings (ranked)

### Rank 5 (Sophisticated)
- **Panic on Nil Pointer Dereference (`AsMessage()` / Non-Message Events)**
  - What: Calling `.Body` directly on `view.replying.GetMautrixContent().AsMessage()` crashes the TUI for state events, redactions, or decryption failures.
  - Why missed: Assumed replied-to events are always standard text messages.
  - Severity: Critical (App crash)
  - Mitigation: Add defensive nil guards before accessing `AsMessage()` and `.Body`.

- **Unicode / Multi-Byte Slicing Panics & Terminal Display Width Mismatch**
  - What: Slicing `body[:30]` on strings < 30 bytes panics. Slicing in the middle of a UTF-8 rune causes corrupt characters or layout blowouts (30 CJK characters = 60 terminal columns).
  - Why missed: Assumed ASCII character byte length equals terminal width.
  - Severity: Critical (App crash / UI corruption)
  - Mitigation: Use `runewidth.Truncate(body, 30, "…")` which handles bounds safety, valid UTF-8 boundaries, and column widths.

- **Destructive Buffer Replacement via `SetText(...)`**
  - What: `view.input.SetText(...)` wipes out any existing draft text the user has typed.
  - Why missed: Assumed the composer was empty when tagging.
  - Severity: Critical (Data loss of typed draft)
  - Mitigation: Splice tags into the input buffer at `cursorOffset` and update cursor position.

- **Fuzzy Filter Mutation Desynchronizes / Blinds Selection State**
  - What: Storing selections by UI index breaks when the search filter updates and the list re-indexes.
  - Why missed: Assuming static list behavior for a dynamic fuzzy search input.
  - Severity: High (Logic failure)
  - Mitigation: Track selections by persistent identifier (e.g. `UserID`) and display a persistent summary.

- **Lack of Visual Checkbox Affordance**
  - What: Without explicitly rendering `[x]`, the user has zero visual feedback on which items are toggled.
  - Why missed: Assuming standard highlighting implies multi-selection state.
  - Severity: High (UX failure)
  - Mitigation: Prefix each line with `[x]` or `[ ]` and trigger re-render on each toggle.

### Rank 4 (Significant)
- **Space Key Conflict with Search Input**
  - What: `Space` cannot be used to toggle selections if the user needs to type spaces in their search query.
  - Why missed: Overloading input keys in a search box.
  - Severity: High
  - Mitigation: Use `Tab` or `Ctrl+Space` to toggle selection.

- **Unescaped Newlines Corrupting Single-Line Status Bar**
  - What: Newlines in the snippet break the 1-row status bar layout.
  - Why missed: Assumed plain text meant single-line.
  - Severity: High
  - Mitigation: Replace whitespace/newlines with single spaces and strip ANSI.

- **Matrix Spec Reply Fallback Quote Prefix Bloat**
  - What: Replying to a reply includes the prior quote fallback, making the snippet useless.
  - Why missed: Ignored Matrix fallback semantics.
  - Severity: High
  - Mitigation: Strip Matrix quote fallback blocks before extracting the snippet.

- **Main-Thread Freeze in Large Rooms**
  - What: `GetMembers()` and synchronous fuzzy matching on 50,000+ members freezes the TUI.
  - Why missed: Scalability assumption.
  - Severity: High
  - Mitigation: Limit candidate pool or execute fuzzy matching asynchronously.

- **Mentions Parsing (MSC3952) / Notification Failure**
  - What: Tagging `@John Doe` without configuring `m.mentions` in the payload will not trigger notifications for the recipient.
  - Why missed: Assuming text injection equals protocol-level mention.
  - Severity: High
  - Mitigation: Decide exact format (MXID) or ensure gomuks message dispatch pipeline parses display names and populates `m.mentions.user_ids`.

- **Focus Loss on Modal Close**
  - What: When modal closes, focus drops into void.
  - Why missed: Missing explicit focus re-acquisition.
  - Severity: High
  - Mitigation: Explicitly call `view.input.Focus()` on confirm/cancel.

### Rank 3 (Moderate)
- **Enter Key Semantic Ambiguity**
  - What: `Enter` semantics undefined for empty vs non-empty toggle sets.
  - Why missed: Edge case of user hitting Enter without space.
  - Severity: Medium
  - Mitigation: `Enter` submits explicitly toggled items; if empty, submits highlighted item.

- **Double `@` Symbol**
  - What: Inline trigger creates `@@user`.
  - Why missed: Overlap between trigger character and tag prefix.
  - Severity: Medium
  - Mitigation: Detect and replace preceding `@`.

- **Redacted Events and Media Attachments**
  - What: Empty `Body` for redacted or media messages leaves blank snippet.
  - Why missed: Assuming all messages have text bodies.
  - Severity: Medium
  - Mitigation: Use `MsgType` fallbacks (e.g. `[Image: filename]`).

### Rank 2 (Minor)
- **Go Map Iteration Randomization**
  - What: Tags inserted in random order.
  - Severity: Medium
  - Mitigation: Sort by UserID or track insertion order.

- **Inclusion of Left/Banned Users**
  - What: Tagging inactive users.
  - Severity: Low
  - Mitigation: Filter `membership == join`.

- **Delimiter Formatting**
  - What: Concatenated tags (`@a@b`).
  - Severity: Low
  - Mitigation: Join with trailing spaces.

## Cross-references
- Also relevant: OT1
