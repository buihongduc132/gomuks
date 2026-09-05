# Terminal Image Support in Gomuks

> Date range: 2026-09-05 → 2026-09-05
> Status: explore-ongoing

## Topics

### terminal-image-support (2026-09-05)
Explored remote terminal graphics protocols (Kitty, Sixel, iTerm2, ANSI half-blocks). Discovered that Gomuks already contains a dormant ANSI TrueColor half-block renderer in `tui/lib/ansimage` and `CalculateBuffer`, but `DownloadPreview()` was commented out during the RPC refactor. Identified the minimal approach to reactivate inline image rendering via asynchronous RPC downloads.

### wezterm-iterm-priority (2026-09-05)
Analyzed targeting WezTerm as primary and iTerm2 as secondary, with universal fallback for other terminals. Identified the iTerm2 inline image protocol (OSC 1337) as the shared native high-resolution protocol supported by both WezTerm and iTerm2. Established a two-tier architecture: iTerm2 OSC 1337 for high-res rendering on target terminals, backed by ANSI half-block cells for universal, flicker-free timeline display.

### cite-strip-audit (2026-09-05)
Audited proposed solutions against origin user verbatims. Stripped Kitty Graphics Protocol, Sixel, external window spawning, and client SQLite caching. Converged down to exactly 2 mechanisms: iTerm2 OSC 1337 (serving WezTerm + iTerm) and ANSI TrueColor half-blocks (serving all remaining terminals). Applied strip to `open-threads.yaml`.

### gotcha-coverage (2026-09-05)
Delegated rigorous gotcha review across 5 review units. Surfaced 17 gotchas (3 Rank 5, 6 Rank 4, 5 Rank 3) including tmux escape stripping, `tcell` double-buffering cursor desync, bitmap decompression bombs, and download data races. Adopted `iamb`'s proven `ratatui-image` architecture (Picker protocol detection, TUI cell masking, homeserver thumbnail queries, and 2D bounding boxes) as the reference blueprint.

## Pick up next time
1. `2026-09-05-turn3a-gotcha-terminal-image-support.md` — Complete ranked gotcha findings and `iamb` architecture synthesis.
2. `2026-09-05-open-threads.yaml` — Updated open threads with Rank 3+ gotchas (OT1–OT8).
3. `2026-09-05-locked-decisions.yaml` — Locked decisions LD1 (WezTerm/iTerm priority), LD2 (ansimage fallback), LD3 (iamb architecture).
4. Implement `DownloadPreview()` with thumbnail API and cell-masking in `tui/messages/filemessage.go`.
