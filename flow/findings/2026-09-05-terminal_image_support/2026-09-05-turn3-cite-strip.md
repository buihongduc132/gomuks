# Explore Turn 3 — Citation & YAGNI Strip Audit

- Date: 2026-09-05
- Phase: audit
- Status: complete

---

## Conversation

> **[user]** (verbatim — trust chain L1)
>
> /40-findings-cite-strip

> **[assistant]**

### Verbatim Citation & YAGNI Mapping

| ID | User Verbatim (Exact Quote) | Proposed Solution Element | Evidence | YAGNI Verdict |
|:---|:---|:---|:---|:---:|
| **V1** | `Nguyễn Linh Bảo │13:02:00 Trần Thanh Thảo ║ zalo-photo.jpg: http://100.114.135.99:48311/...` | Diagnose missing media rendering in Gomuks TUI timeline; identify empty `imageData`. | `tui/messages/filemessage.go:L142` | ✅ exact |
| **V2** | `--- 1 search remote on how do we be able to shows the img in the terminal if it is supported ;` | Survey remote terminal image protocols (Kitty, Sixel, OSC 1337, Half-blocks). | `turn1: Part 1` | ✅ exact |
| **V3** | `How can we implement it in gomuks itself ;` | Reactivate dormant `ansimage` rendering in `filemessage.go` via async RPC download. | `tui/lib/ansimage`, `turn1: Part 2` | ✅ exact |
| **V4** | `persist all of these into flow/findings/ for me ;` | Persist multi-turn transcript and decisions under `flow/findings/2026-09-05-terminal_image_support/`. | `flow/findings/2026-09-05-terminal_image_support/` | ✅ exact |
| **V5** | `we are aiming for the approach that could supported wezterm first ; and iterm ; for the rest , the more the better;` | Target iTerm2 OSC 1337 inline protocol (shared by WezTerm & iTerm2) + ANSI Half-blocks fallback for all other terminals. | `turn2`, `locked-decisions.yaml:LD1` | ⚠️ tension (OSC 1337 vs `tcell` redraw) |

---

### Strip List (Over-Engineering Removed)

| # | Stripped Element | Why Stripped (Zero User Demand / Redundant) | Surviving Minimal Counterpart |
|---|---|---|---|
| **S1** | **Kitty Graphics Protocol** | User explicitly bounded priority to **WezTerm first**, **iTerm second**. iTerm2 does NOT support Kitty. WezTerm already speaks OSC 1337. Kitty adds an unnecessary second protocol stack. | **iTerm2 OSC 1337** natively covers both WezTerm and iTerm. |
| **S2** | **Sixel Graphics Protocol** | Neither WezTerm nor iTerm requires Sixel. Sixel is 256-color palette-limited and destructive to terminal text cells. | **ANSI TrueColor Half-blocks (`▄`)** handles all non-OSC terminals with full 24-bit color. |
| **S3** | **External Viewer Subprocess (`ui.RunExternal`)** | User asked to "shows the img in the terminal", not launch external desktop windows (`xdg-open` / `feh` / `chafa`). | In-terminal display directly in the Gomuks TUI. |
| **S4** | **Complex Metadata Cache Daemon** | Gomuks daemon already caches HTTP media. Client-side SQLite indexing is unneeded. | In-memory `[]byte` slice in `FileMessage`. |

---

### Conflict Pairs & Resolution

* **Tension**: Native in-band escape sequences (OSC 1337) vs `tcell` full-screen redraw diffing.
  * *Conflict*: In `tcell`, printing raw escape sequences directly into a scrollable message list causes `tcell`'s screen diff to overwrite or tear the image during room scrolling.
  * *Resolution*: Use the two-tier architecture:
    1. **Timeline**: ANSI TrueColor half-blocks (`▄`) provide universal, flicker-free, natively scrollable image previews in the chat stream.
    2. **High-Res OSC 1337**: Emitted when the user focuses/toggles high-res view on a message or inside a dedicated view overlay, bypassing scrolling desync.

---

### Nearest-to-N Convergence Check

* **User Bounds**: 2 named terminal platforms (**WezTerm #1**, **iTerm #2**) + 1 universal fallback tier ("the rest").
* **Proposed Stack Before Strip**: 5 items (Kitty APC + Sixel DCS + iTerm2 OSC 1337 + ANSI half-blocks + External viewer launcher).
* **Converged Stack After Strip**: Exactly **2 mechanisms**:
  1. **Tier 1 (High-Res)**: **iTerm2 OSC 1337** (satisfies WezTerm #1 and iTerm2 #2).
  2. **Tier 2 (Universal Fallback)**: **ANSI TrueColor Half-Blocks** (satisfies "the rest" with 100% terminal compatibility).

---

## Status at end of turn 3
- Full YAGNI audit complete.
- Over-engineered protocols (Kitty, Sixel, External Subprocess) stripped.
- Converged down to exact 2-tier architecture (OSC 1337 + ANSI half-blocks).
- `open-threads.yaml` updated to remove stripped items.
