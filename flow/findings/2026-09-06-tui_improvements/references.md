# References

> Sources consulted during this explore session.

## Source files
- `tui/room-view.go` — Consulted to see how the status bar renders "Replying to @user" and how we can inject the message snippet.
- `pkg/hicli/database/event.go` — Consulted to find how to extract the message body (`GetMautrixContent().AsMessage().Body`) for the reply snippet.
- `tui/view-main.go` — Consulted for modal management (`ShowModal`).
- `tui/fuzzy-picker-modal.go` — Consulted to see how modals currently work, finding that it only supports single-selection (`selected int`).

## Documents
- None.

## Code patterns
- Gomuks TUI modals (`FuzzyPickerModal`) — Pattern for modal popups in the TUI, showing a search input and a list of results. We will mimic this for the user-tagging modal.
