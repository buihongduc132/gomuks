# Explore Turn 2 — WezTerm & iTerm Priority Architecture

- Date: 2026-09-05
- Phase: explore
- Status: complete

---

## Conversation

> **[user]** (verbatim — trust chain L1)
>
> persist all of these into flow/findings/ for me ; we are aiming for the approach that could supported wezterm first ; and iterm ; for the rest , the more the better;

> **[assistant]**

### Terminal Target Priority Analysis

The user established explicit platform targeting priorities:
1. **Primary Target**: **WezTerm**
2. **Secondary Target**: **iTerm2**
3. **Tertiary / Fallback**: **The rest (the more the better)**

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                        Terminal Capability & Protocol Matrix                           │
├─────────────────┬──────────────────┬──────────────────┬─────────────────┬──────────────┤
│ Terminal        │ iTerm2 OSC 1337  │ Kitty Protocol   │ Sixel (DCS)     │ Half-Blocks  │
├─────────────────┼──────────────────┼──────────────────┼─────────────────┼──────────────┤
│ WezTerm (#1)    │ YES (Native)     │ YES (Native)     │ YES (Native)    │ YES (Native) │
│ iTerm2  (#2)    │ YES (Canonical)  │ NO               │ YES (Modern)    │ YES (Native) │
│ Ghostty         │ NO               │ YES (Native)     │ NO              │ YES (Native) │
│ Kitty           │ NO               │ YES (Canonical)  │ NO              │ YES (Native) │
│ Foot            │ NO               │ Experimental     │ YES (Native)    │ YES (Native) │
│ Alacritty       │ NO               │ NO (without fork)│ NO              │ YES (Native) │
│ Tmux / SSH      │ Passthrough only │ Passthrough only │ Passthrough only│ YES (Direct) │
└─────────────────┴──────────────────┴──────────────────┴─────────────────┴──────────────┘
```

---

### Architectural Strategy: Two-Tier Protocol Hierarchy

To achieve **WezTerm first** and **iTerm2 second**, with **universal support for the rest**:

```
                              Terminal Check
                                    │
           ┌────────────────────────┴────────────────────────┐
           ▼                                                 ▼
   WezTerm or iTerm2                                   Other Terminals
   (OSC 1337 or Kitty)                             (Alacritty, Tmux, SSH...)
           │                                                 │
           ▼                                                 ▼
High-Res Pixel Graphics                          Universal ANSI Half-Blocks
(OSC 1337 inline / modal)                        (▄ TrueColor in-grid cells)
```

#### Tier 1 (WezTerm + iTerm2): The Shared Protocol = iTerm2 Inline Images (OSC 1337)
- **Protocol**: `\x1b]1337;File=inline=1;width=...;height=...:<base64-data>\x07`
- **Why it fits**: Both WezTerm and iTerm2 natively support OSC 1337 out-of-the-box.
- **Terminal Detection**:
  - `os.Getenv("TERM_PROGRAM") == "WezTerm" || os.Getenv("WEZTERM_PANE") != ""`
  - `os.Getenv("TERM_PROGRAM") == "iTerm.app" || os.Getenv("LC_TERMINAL") == "iTerm2"`

#### Tier 2 (The Rest): Universal ANSI Half-Block Engine (`tui/lib/ansimage`)
- **Protocol**: Unicode `▄` cells with TrueColor foreground & background.
- **Why it fits**: Requires zero terminal graphics protocol support. Works across every terminal that supports TrueColor (Kitty, Ghostty, Alacritty, GNOME Terminal, Linux console, tmux, SSH sessions).

---

### TUI Integration Architecture within `mauview` / `tcell`

Full-screen TUI clients built on `tcell` manage a character grid. For inline images:
1. **In-Timeline Rendering**:
   - `ansimage` renders half-block rows directly into `msg.buffer []tstring.TString`.
   - `mauview` calculates height and scrolls naturally without graphic artifacts or redraw flickering.
2. **High-Res Protocol Burst (WezTerm / iTerm2)**:
   - For users desiring high-res pixel rendering on WezTerm/iTerm2, gomuks can either:
     a. Emit OSC 1337 inside a dedicated preview overlay / popup modal (where `tcell` suspends or locks the bounding box).
     b. Provide an interactive keybind (e.g. `o` or `Enter`) to trigger an in-terminal high-res popover via OSC 1337 or launch an external viewer.

---

## Status at end of turn 2
- Target priority locked: WezTerm first, iTerm2 second, universal fallback for the rest.
- Convergence protocol identified: iTerm2 OSC 1337 (covers both WezTerm and iTerm2) + ANSI Half-blocks (covers the rest).
- Decision recorded in `2026-09-05-locked-decisions.yaml`.
