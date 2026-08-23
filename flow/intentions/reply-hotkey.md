# Intention: Hotkey to select previous message to reply to

**Date**: 2026-08-22
**Status**: implemented (2026-08-23)
**Depends on**: configurable-hotkeys.md

## User verbatim

a. be able to config hotkey instead of the hardcode list
b. WEB having hotkey to SELECT the previous message to reply to.
I need that functionalities INSIDE the terminal one as well

## Why

Users need a keyboard-driven way to select a message for replying in both
the web client and the terminal client.

## Requirement (DOD)

1. Terminal: `Ctrl+r` in room scope enters select mode with reason `reply to`
2. Web: `Ctrl+ArrowUp` / `Ctrl+ArrowDown` (preference-gated `ctrl_arrow_reply`)
   remain the default reply-select keys and are rebindable via Settings → Keybindings
3. Configurable: terminal via `~/.config/gomuks/terminal-keybindings.yaml` (room scope);
   web via localStorage + settings panel
4. Tests: web keyconfig + settings panel; terminal `findMessage` unit tests; TUI builds

## Implementation

### Terminal
- `'Ctrl+r': reply` in `tui/config/keybindings.yaml` room scope
- `case "reply":` in `tui/room-view.go` calls `StartSelecting(SelectReply, "")`
- Flow: Ctrl+r → selecting mode → Up/Down → Enter → "Replying to <sender>"

### Web
- Composer uses `keyconfig` `reply_prev` / `reply_next` (defaults Ctrl+ArrowUp/Down)
- Settings → Keybindings records a new shortcut and persists it

## Non-goals

- No chord sequences for reply
- No Matrix account-data sync of keybindings
