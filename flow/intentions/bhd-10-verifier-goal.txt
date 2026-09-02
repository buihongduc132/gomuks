Verify BHD-10 hotkey configuration + reply-select is complete and correct in this gomuks fork.

Issue ask (verbatim intent):
a. be able to config hotkey instead of the hardcode list
b. WEB having hotkey to SELECT the previous message to reply to
c. that functionality INSIDE the terminal one as well

Required evidence, not claims:
1. Web defaults still match original hardcoded map: Escape, Ctrl+k, Alt+ArrowUp, Alt+ArrowDown, Ctrl+f.
2. Web keybindings are loaded from localStorage key gomuks-keybindings via web/src/ui/keyconfig.ts and rebuilt in Keybindings.buildKeymaps.
3. Settings UI exists: Settings → Keybindings tab (web/src/ui/settings/KeybindingsSettings.tsx wired from SettingsView.tsx). Recording a key persists overrides; Enter/Tab are reserved; duplicate keys across bindable actions are rejected including collisions with other action defaults.
4. Web reply-select remains composer-scoped: MessageComposer uses reply_prev / reply_next from the effective keymap (defaults Ctrl+ArrowUp / Ctrl+ArrowDown), still gated by ctrl_arrow_reply preference. Do not send a Matrix message during proof.
5. Terminal: tui/config/keybindings.yaml room scope maps Ctrl+r to reply; room-view OnKeyEvent starts StartSelecting(SelectReply). findMessage walks TimelineCache, skips local echoes and service notices, SelectPrevious/SelectNext work. Unit tests in tui/find-message_test.go.
6. Tests: `go test ./tui` and vitest keyconfig/keybindings/KeybindingsSettings must pass. No regression of composer auto-focus hatch.
7. AGENTS.md and flow/intentions/{configurable-hotkeys,reply-hotkey}.md document the feature. Do not commit AGENTS.md Multica runtime block. Do not deploy to prod bin `gomuks` / `gomuks-daemon-patched`.

Reject if: settings UI missing, reply hotkey still hardcoded-only in web composer, terminal Ctrl+r missing, findMessage still commented out, tests missing/failing, or reserved keys bindable.
