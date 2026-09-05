# Gotcha Coverage — Terminal Image Support in Gomuks

> Source: `flow/findings/2026-09-05-terminal_image_support/`
> Mode: findings
> Sub-agent: Gotcha Reviewer (research)
> Units reviewed: `[LD1]`, `[LD2]`, `[OT1]`, `[OT2]`, `[OT3]`
> Reference Blueprint: `iamb` (`ratatui-image` architecture)

---

## Findings (Ranked)

### Rank 5 (Sophisticated — Fundamental Gaps)

#### 1. Terminal Multiplexer (tmux/screen) Passthrough Barrier
- **Target Unit**: `[LD1]`, `[OT1]`
- **What**: When Gomuks runs inside `tmux`, unescaped OSC 1337 sequences are stripped or printed as thousands of raw base64 garbage characters across the pane.
- **Why missed**: Analyzed native terminal emulators directly without accounting for multiplexer translation layers.
- **Severity**: **Rank 5** (Breaks WezTerm/iTerm2 image display for all tmux users).
- **Mitigation (iamb approach)**: Detect `$TMUX`. If inside tmux, wrap in DCS passthrough `\x1bPtmux;\x1b\x1b]1337;...\x07\x1b\`. If `allow-passthrough` is unsupported, automatically fall back to ANSI half-blocks.

#### 2. `tcell` Cursor Position Desync & Double-Buffering Overwrites
- **Target Unit**: `[OT1]`
- **What**: Emitting inline OSC 1337 moves the terminal's hardware cursor. `tcell`'s screen diff matrix has no awareness of this shift. On the next draw cycle, `tcell` emits spaces or text over the image area, causing screen tearing.
- **Why missed**: Treated terminal stdout as an append-only stream rather than an immediate-mode 2D diff matrix.
- **Severity**: **Rank 5** (Invalidates naive in-timeline OSC 1337 bursts).
- **Mitigation (iamb approach)**: Replicate `ratatui-image`'s cell-masking pattern: fill the TUI screen buffer rectangle with transparent placeholder cells so `tcell`'s diff engine never writes characters over the image region.

#### 3. Memory Exhaustion via Unbounded Bitmap Decompression ("Pixel Bombs")
- **Target Unit**: `[LD2]`, `[OT2]`
- **What**: `imaging.Resize` and `image.Decode` allocate the full uncompressed RGBA bitmap before downsampling. A 50MP photo allocates ~200MB RAM; decompression bombs cause immediate OOM crash.
- **Why missed**: Assumed downloaded images are already small.
- **Severity**: **Rank 5** (Crash vector on public Matrix rooms).
- **Mitigation (iamb approach)**: Inspect dimensions with `image.DecodeConfig` first. Reject or clamp images over 16MP. Fetch homeserver-generated thumbnails rather than raw original files.

---

### Rank 4 (Significant — Correctness & Stability)

#### 4. Data Race on `FileMessage` State Between Worker and UI Thread
- **Target Unit**: `[OT2]`
- **What**: Background download goroutine modifies `msg.imageData` while main UI thread reads it during scrolling/resizing without synchronization.
- **Why missed**: Thread boundary not formally defined in prototype sketch.
- **Severity**: **Rank 4** (Triggers Go race detector; causes panic under concurrent load).
- **Mitigation**: Synchronize via `sync.RWMutex` or pass downloaded image payloads through the main TUI event channel.

#### 5. SSH Environment Scrubbing Breaks Terminal Auto-Detection
- **Target Unit**: `[LD1]`
- **What**: `sshd` strips custom env vars (`$TERM_PROGRAM`, `$WEZTERM_PANE`) by default, causing Gomuks over SSH to fail to detect WezTerm/iTerm2.
- **Why missed**: Tested in local desktop environment where env vars are populated.
- **Severity**: **Rank 4** (Silently disables WezTerm/iTerm2 protocol over SSH).
- **Mitigation (iamb approach)**: Provide explicit configuration override `image_preview.protocol = "iterm2"` in config, paired with DA1 terminal query fallback.

#### 6. Unsupported Modern Media Formats in Matrix Feeds (WebP/AVIF/HEIC)
- **Target Unit**: `[LD2]`
- **What**: Matrix mobile clients frequently upload animated WebP, AVIF, and HEIC. Go standard image packages fail on extended WebP and have zero AVIF/HEIC support.
- **Why missed**: Assumed PNG/JPEG dominance.
- **Severity**: **Rank 4** (Photos from mobile clients fail to display).
- **Mitigation**: Request Matrix homeserver thumbnails with `format=jpeg` to force server-side transcoding.

#### 7. Terminal Window Resize CPU Throttling and UI Freeze
- **Target Unit**: `[OT3]`, `[LD2]`
- **What**: Rapid `SIGWINCH` stream during terminal dragging re-runs Lanczos downscaling on every visible image, causing 100% CPU freeze.
- **Why missed**: Assumed resize occurs once.
- **Severity**: **Rank 4** (UI thread freezes during window adjustments).
- **Mitigation**: Cache rendered cell buffers keyed by width; debounce resize calculations by 100ms.

#### 8. Redraw Storms and Leaked Workers on Rapid Room Switching
- **Target Unit**: `[OT2]`
- **What**: Rapidly switching rooms launches download goroutines that emit global redraw events even after the room is abandoned.
- **Why missed**: Considered single-room lifecycle in isolation.
- **Severity**: **Rank 4** (Wasted CPU, battery, and buffer churn).
- **Mitigation**: Tie download goroutines to room view `context.Context`; discard redraw events if room is no longer active.

#### 9. Ghosting on Modal Dismissal (No OSC 1337 Erasure)
- **Target Unit**: `[OT1]`
- **What**: OSC 1337 has no protocol-level delete command. Dismissing a modal can leave orphan pixel fragments.
- **Why missed**: Assumed closing a TUI modal resets emulator graphics.
- **Severity**: **Rank 4** (Visual artifacts after image preview).
- **Mitigation**: Trigger a full `screen.Sync()` to force an absolute terminal redraw upon dismissing image views.

---

### Rank 3 (Moderate — Design & Adaptations)

#### 10. Full-Resolution Media Ingestion Instead of Homeserver Thumbnails
- **Target Unit**: `[OT2]`, `[OT3]`
- **What**: Fetching full raw photos over network for a small preview wastes megabytes of bandwidth.
- **Mitigation (iamb approach)**: Query `/_matrix/client/v3/media/thumbnail?width=320&height=240&method=scale`.

#### 11. Extreme Aspect Ratio Clamping Failure (Panoramas vs Tall Screenshots)
- **Target Unit**: `[OT3]`
- **What**: Panoramic images scale to 1 row (illegible); tall screenshots dominate timeline.
- **Mitigation**: Implement 2D bounding box `(maxCols, maxRows)` maintaining aspect ratio with minimum height constraint (≥3 rows).

#### 12. Color Banding on 256-Color Terminals
- **Target Unit**: `[LD1]`, `[LD2]`
- **What**: Non-TrueColor terminals map 24-bit half-blocks with blotchy color quantization.
- **Mitigation**: Enable Floyd-Steinberg dithering in `ansimage` when `screen.Colors() < 16777216`.

#### 13. PTY Buffer Exhaustion on Large OSC Payloads
- **Target Unit**: `[OT1]`, `[LD1]`
- **What**: Pumping multi-megabyte base64 escape bursts causes terminal UI lag.
- **Mitigation**: Pre-downscale images to max 1920×1080 before base64 encoding.

#### 14. Font Aspect Ratio & Line-Height Distortion
- **Target Unit**: `[LD2]`, `[OT3]`
- **What**: Custom terminal line-spacing settings (`line_height != 1.0`) distort 1:2 half-block pixel aspect ratios.
- **Mitigation**: Provide cell aspect ratio configuration setting (default 0.5).

---

### Rank 2 / 1 (Minor / YAGNI)

#### 15. Black Background Bleed on Transparent Images in Light Themes (Rank 2)
- **Mitigation**: Use default theme background color instead of hardcoded `color.Black`.

#### 16. Terminal Text Selection & Clipboard Copy Artifacts (Rank 2)
- **Mitigation**: Mouse drag over half-blocks copies `'▄'` runes; expected TUI behavior.

#### 17. Animated GIF CPU Churn on Inactive Timelines (Rank 1 — YAGNI)
- **Mitigation**: YAGNI: Keep previews static (decode first frame only).

---

## Synthesis: Adopting the `iamb` Architecture

In response to user requirement *"Note that iamb is working flawlessly; follow it approach for me"*:

`iamb` solves these gotchas in its `ratatui-image` implementation via:
1. **Picker Pattern**: Auto-probes protocol (`iterm2`, `kitty`, `sixel`, `halfblocks`), overridable via config.
2. **Fixed Bounding Box**: Configurable preview size (`width = 66, height = 10`), proportional scaling.
3. **Cell Masking**: Reserves TUI cells in the screen buffer to prevent text redraw collisions.
4. **Thumbnail Fetching**: Requests homeserver-scaled thumbnails rather than full originals.
5. **Universal Fallback**: Degrades cleanly to half-blocks when protocols are unsupported or blocked by tmux.
