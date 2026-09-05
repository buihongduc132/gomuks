# Declarative Plan: Terminal Image Support in Gomuks

> Plan ID: `terminal-image-preview`  
> Created: 2026-09-06 · Last reconciled: 2026-09-06  
> Working Status: todo  
> Deployment: dev:todo - prod:todo  
> Branch: feat/terminal-image-preview  
> Worktree: /home/bhd/Documents/Projects/bhd/gomuks-wt-terminal-images  
> Location: flow/plans/terminal-image-preview.md (committed ad922195)  
> Items: 15 total (0 implemented, 15 pending)

## References
- Intention: `flow/intentions/reply-hotkey.md` (established TUI extension conventions)
- Findings: `flow/findings/2026-09-05-terminal_image_support/`
- Locked Decisions:
  - `[LD1]`: WezTerm #1, iTerm2 #2 via OSC 1337, universal ANSI half-block fallback.
  - `[LD2]`: Inherit Gomuks built-in `ansimage` TrueColor half-block engine.
  - `[LD3]`: Replicate `iamb` / `ratatui-image` architecture (Picker, cell-masking, 2D clamp, thumbnail API).
- Source: Resolved from explicit user command (`/10-plan-declarative remember to make worktree for this ;`) + conversation substance (terminal image support explore, cite-strip audit, and gotcha coverage).

## Requirement (verbatim)
> ```text
> │
> Nguyễn Linh Bảo          │13:02:00 Trần Thanh Thảo ║ zalo-photo.jpg: http://100.114.135.99:48311/_gomuks/media/bhd-main2.tail05fddd.ts.net/KmHzmEfrKedRgZpdtemInYOE?encrypted=false&image_auth=eyJ1c2VybmFtZSI6ImJoZCIsImV4cGlyeSI6MTc4ODU5NjUyNCwiaW1hZ2Vfb25seSI6dHJ1ZX0.v_                   │
> Nguyễn Lê Ngọc Vũ        │                         ║ 8UNSn2agLH7RxKf62Nx_T4ASk5BnVvzPnXzN7FK6Q                                                                                                                                                                                                  │
> Koala                    │13:02:02 Trần Thanh Thảo ╨ Nay em ăn này                                                                                                                                                                                                                              │
> --- 1 search remote on how do we be able to shows the img in the terminal if it is supported ; 
> 
> How can we implement it in gomuks itself ;
> 
> persist all of these into flow/findings/ for me ; we are aiming for the approach that could supported wezterm first ; and iterm ; for the rest , the more the better;
> 
> Note that iamb is working flawlessly; follow it approach for me ;
> 
> /10-plan-declarative remember to make worktree for this ;
> ```
> — Source: User explicit instructions & multi-turn explore transcript (2026-09-05 / 2026-09-06)

## DOD (Definition of Done)
Plan done when ALL below true:
- [ ] Image messages render visual previews inline in `gomuks-terminal` instead of falling back to plaintext URL links.
- [ ] WezTerm and iTerm2 render high-resolution images via the iTerm2 OSC 1337 protocol without screen tearing during message scrolling.
- [ ] All other terminals (and unconfigured tmux environments) cleanly fall back to 24-bit TrueColor ANSI half-blocks (`▄`).
- [ ] Media download and decoding occur asynchronously on background workers without freezing the main TUI event loop.
- [ ] Bitmap decompression bombs are rejected by validating dimensions with `image.DecodeConfig` before buffer allocation (capped at 16MP).
- [ ] Previews adhere to a bounded 2D aspect-ratio clamped box (default max width 66 cols, max height 16 rows, min height 3 rows).
- [ ] Terminal image protocol is user-configurable via `config.yaml` (`auto`, `iterm2`, `halfblocks`, `disabled`) to support SSH overrides.
- [ ] Automated tests verify terminal picker detection, aspect ratio clamping, and thread-safe download lifecycles.

## Tasks

### Media Download & Lifecycle (`pkg/rpc/` & `tui/messages/`)
- [ ] `dl-async-nonblocking`: `FileMessage.DownloadPreview` retrieves image data asynchronously via `gc.Download` without blocking the TUI event loop.
- [ ] `dl-thread-safety`: `FileMessage` access to `imageData` and `buffer` is protected by mutex synchronization against concurrent reads during timeline rendering.
- [ ] `dl-redraw-dispatch`: Download completion dispatches a UI redraw event to the active room view only if the room remains active.
- [ ] `dl-thumbnail-api`: Media retrieval queries Matrix homeserver thumbnail endpoint (`format=jpeg&method=scale`) for preview generation when available.

### Image Safety & Geometry Clamping (`tui/messages/` & `tui/lib/ansimage/`)
- [ ] `img-decode-safety`: Image header dimensions are validated via `image.DecodeConfig` prior to full buffer allocation, rejecting payloads exceeding 16MP.
- [ ] `img-box-clamping`: `CalculateBuffer` calculates dimensions within a proportional 2D bounding box (max width 66 cols, max height 16 rows, min height 3 rows).
- [ ] `img-resize-debounce`: Rapid `SIGWINCH` resize events debounce image recalculations and cache rendered buffers by column width to prevent CPU lockup.

### Protocol Picker & Terminal Detection (`tui/config/` & `tui/lib/`)
- [ ] `proto-picker-detect`: Terminal graphics picker auto-detects WezTerm (`$TERM_PROGRAM == "WezTerm"` or `$WEZTERM_PANE != ""`) and iTerm2 (`$TERM_PROGRAM == "iTerm.app"` or `$LC_TERMINAL == "iTerm2"`).
- [ ] `proto-picker-config`: `UserPreferences` supports `ImagePreviewProtocol` (`"auto"`, `"iterm2"`, `"halfblocks"`, `"disabled"`) and `ImagePreviewSize` with defaults.
- [ ] `proto-tmux-passthrough`: When `$TMUX` is present, OSC 1337 escape sequences are wrapped in DCS tmux passthrough escapes (`\x1bPtmux;...`) or degraded to half-blocks if passthrough is unverified.

### Rendering & Screen Buffer Integration (`tui/messages/` & `tui/lib/`)
- [ ] `render-halfblock-fallback`: `ansimage` TrueColor half-block cells (`▄`) render directly into `tstring.TString` buffers for universal terminal support.
- [ ] `render-osc1337-cellmask`: In WezTerm/iTerm2 mode, inline OSC 1337 rendering masks underlying cells in `tcell.Screen` with blank placeholder cells to prevent redraw overwrites.
- [ ] `render-modal-sync`: Dismissing modal or overlay previews triggers `screen.Sync()` to prevent ghosted graphic artifacts.

### Verification & Automated Tests (`tui/messages/` & `tui/`)
- [ ] `test-proto-picker`: Unit tests assert terminal detection correctly identifies WezTerm, iTerm2, tmux, and SSH fallback.
- [ ] `test-box-clamping`: Unit tests assert extreme aspect ratios (panoramas and vertical strips) scale within bounded dimensions.

## Idempotency
Re-running `/10-plan-declarative` on same requirement reconciles to THIS plan.
Implemented items auto-marked `- [x]`. Pending items surface as work-remaining.
DO NOT rewrite item prose on re-run (status flips only).

## Open Threads
_(populated from `flow/findings/2026-09-05-terminal_image_support/2026-09-05-open-threads.yaml`)_
- `OT1`: `osc1337-vs-halfblock-timeline-rendering` (addressed via `render-osc1337-cellmask` and `render-halfblock-fallback`)
- `OT2`: `async-download-ui-redraw-signal` (addressed via `dl-async-nonblocking` and `dl-redraw-dispatch`)
- `OT3`: `timeline-dimension-clamping` (addressed via `img-box-clamping`)
- `OT4`: `tmux-passthrough-barrier` (addressed via `proto-tmux-passthrough`)
- `OT5`: `unbounded-bitmap-decompression-oom` (addressed via `img-decode-safety`)
- `OT6`: `filemessage-concurrency-data-race` (addressed via `dl-thread-safety`)
- `OT7`: `ssh-env-scrubbing-config-override` (addressed via `proto-picker-config`)
- `OT8`: `homeserver-thumbnail-fetch` (addressed via `dl-thumbnail-api`)
