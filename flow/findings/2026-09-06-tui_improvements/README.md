# tui_improvements

> Date range: 2026-09-06 → 2026-09-06
> Status: explore-ongoing

## Topics

### 2026-09-06-explore (2026-09-06)
Explored implementing a user tagging modal and displaying a reply snippet in the TUI client. Concluded that the reply snippet is trivial to add by extracting the message body from the event in `room-view.go`. Concluded that the user tagging modal requires extending the existing `FuzzyPickerModal` to support multi-select and hooking it up to the room members list.

## Pick up next time
1. `2026-09-06-turn1-user-requests.md`
2. `2026-09-06-turn2-tui-focus.md`
3. Decide if the user wants to tweak the multi-select modal behavior (OT1).
