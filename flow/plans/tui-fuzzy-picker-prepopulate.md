# TUI Fuzzy Picker Pre-population on Initial Open

> Plan ID: `tui-fuzzy-picker-prepopulate`
> Created: 2026-09-03 · Last reconciled: 2026-09-03
> Status: done
> Branch: main
> Location: flow/plans/tui-fuzzy-picker-prepopulate.md
> Items: 6 total (6 implemented, 0 pending)

## Requirement (verbatim)
> "why the fuck do you keep not hearing my words . I told that the SEARCH do not show ANY space initially on it opening :
> ╔═════════════Space Switcher═════════════╗
> ║                                        ║
> ╚════════════════════════════════════════╝
> this is a FUCKING non-ux friendly; imagine facebook , on pressing 'search for friends' , do it fucking showing a BLANK list in the drop down? put this problem into flow/findings/ ; you are to create the plan and write the red test to prove that bug for me ; then commit x;"
> — Source: User explicit request & conversation context (2026-09-03)

## DOD (Definition of Done)
Plan done when ALL below true:
- [x] Opening Space Switcher (`Ctrl+s` / `Alt+s`) immediately displays all available spaces with `"All Rooms"` at top
- [x] Opening Room Switcher (`Ctrl+k` / `Alt+k`) immediately displays all joined rooms
- [x] Clearing the search text box restores the full list of entries
- [x] First item is highlighted by default on modal open so pressing `Enter` immediately selects it
- [x] Automated unit tests in Go verify initial population, filter reduction, and restoration upon clearing query
- [x] Root cause analysis and findings documented under `flow/findings/`

## Tasks

### Modal Component Core (`tui/fuzzy-picker-modal.go`)
- [x] `fp-init-populate`: `NewFuzzyPickerModal` calls `fp.changeHandler("")` on construction to populate items before initial draw
- [x] `fp-empty-query-all-items`: `changeHandler` generates full match list from `fp.titles` when `len(str) == 0` instead of clearing results
- [x] `fp-default-highlight`: Top entry (`index 0`) is highlighted and scrolled to visible view on empty query

### Test Suite (`tui/fuzzy_picker_test.go`)
- [x] `test-initial-populate`: `TestFuzzyPickerModal_InitialPopulate` asserts all items exist on modal open and `Enter` selects top item
- [x] `test-filter-and-restore`: `TestFuzzyPickerModal_FilterAndRestoreAll` asserts filtering narrows list and empty query restores all items

### Documentation & Findings (`flow/findings/`)
- [x] `finding-doc`: `flow/findings/tui-fuzzy-picker-empty-on-open.md` records symptom, root cause, and expected UX behavior

## Idempotency
Re-running `/10-plan-declarative` on same requirement reconciles to THIS plan.
Implemented items auto-marked `- [x]`. Pending items surface as work-remaining.
DO NOT rewrite item prose on re-run (status flips only).

## Open Threads
_(none)_
