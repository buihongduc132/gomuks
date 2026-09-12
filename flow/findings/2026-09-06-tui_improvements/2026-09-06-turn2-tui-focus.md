# Explore Turn 2 — tui-focus
> Date: 2026-09-06
> Phase: explore
> Status: completed

> **[user]** (verbatim — trust chain L1)
> 1. Img-tui ; it is already implemented and deployed  local ;
> 3. must shows snippet if possible ; 
> 4. I am talking about  TUI;

[assistant]
Got it. That completely changes the context—focusing entirely on the TUI makes these very clear.

Here is how we can implement the remaining two features in the TUI:

> "3. must shows snippet if possible ;"

**Finding & Implementation Path:** 
Yes, this is entirely possible and straightforward. 
In `tui/room-view.go`, the `GetStatus()` function currently renders `"Replying to " + string(view.replying.Sender) + " - "`. 
Since `view.replying` is a `*database.Event`, we can extract the plain text body by calling `view.replying.GetMautrixContent().AsMessage().Body`. 
We can truncate this body (e.g., to 30 characters) and append it to the status bar string so it reads something like: 
`Replying to @user: "This is a snippet of..." -`

> "4. I am talking about TUI;" (User tag modal, multi-select)

**Finding & Implementation Path:**
The TUI currently has a `FuzzyPickerModal` (used by `fuzzy-search-modal.go` for rooms and `space-switcher-modal.go` for spaces). However, `FuzzyPickerModal` only tracks a single selection (`selected int`). 

To build a user tagging modal that supports multi-select, we would:
1. **Extend or Duplicate the Picker:** Create a `FuzzyMultiPickerModal` that tracks a `map[int]bool` or slice of selected indices (e.g., using `Space` to toggle a selection, and `Enter` to confirm).
2. **Create the User Tag Modal:** Build a new `user-tag-modal.go` that queries the current room's members (via `view.Room.GetMembers()` or similar) and feeds their display names and IDs into the multi-picker.
3. **Inject the Tags:** When `Enter` is pressed, the modal will map the selected items back to Matrix user IDs/names, format them as `@username`, and insert them directly into the composer's text buffer (`view.input.SetText(...)`).
4. **Keybinding:** Bind this to a hotkey (like `Ctrl+T` or `@`) in `room-view.go`'s input handler.

If you are happy with these approaches, would you like me to fast-forward this into an implementation plan/change via `/30a-ospx-ff`, or do you want to tweak how the multi-select modal behaves first?

Status at end of turn 2: Awaiting user confirmation to move to implementation.
