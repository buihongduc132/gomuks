# Explore Turn 1 — Terminal Image Support in Gomuks

- Date: 2026-09-05
- Phase: explore
- Status: complete

---

## Conversation

> **[user]** (verbatim — trust chain L1)
>
> /10-ospx-explore ```│
> Nguyễn Linh Bảo          │13:02:00 Trần Thanh Thảo ║ zalo-photo.jpg: http://100.114.135.99:48311/_gomuks/media/bhd-main2.tail05fddd.ts.net/KmHzmEfrKedRgZpdtemInYOE?encrypted=false&image_auth=eyJ1c2VybmFtZSI6ImJoZCIsImV4cGlyeSI6MTc4ODU5NjUyNCwiaW1hZ2Vfb25seSI6dHJ1ZX0.v_                   │
> Nguyễn Lê Ngọc Vũ        │                         ║ 8UNSn2agLH7RxKf62Nx_T4ASk5BnVvzPnXzN7FK6Q                                                                                                                                                                                                  │
> Koala                    │13:02:02 Trần Thanh Thảo ╨ Nay em ăn này                                                                                                                                                                                                                              │``` --- 1 search remote on how do we be able to shows the img in the terminal if it is supported ; 
> 
> How can we implement it in gomuks itself ;

> **[assistant]**

### Verbatim Quote
> ```text
> │
> Nguyễn Linh Bảo          │13:02:00 Trần Thanh Thảo ║ zalo-photo.jpg: http://100.114.135.99:48311/_gomuks/media/bhd-main2.tail05fddd.ts.net/KmHzmEfrKedRgZpdtemInYOE?encrypted=false&image_auth=eyJ1c2VybmFtZSI6ImJoZCIsImV4cGlyeSI6MTc4ODU5NjUyNCwiaW1hZ2Vfb25seSI6dHJ1ZX0.v_                   │
> Nguyễn Lê Ngọc Vũ        │                         ║ 8UNSn2agLH7RxKf62Nx_T4ASk5BnVvzPnXzN7FK6Q                                                                                                                                                                                                  │
> Koala                    │13:02:02 Trần Thanh Thảo ╨ Nay em ăn này                                                                                                                                                                                                                              │
> --- 1 search remote on how do we be able to shows the img in the terminal if it is supported ; 
> 
> How can we implement it in gomuks itself ;
> ```

---

## Executive Summary

1. **Terminal Image Support**: Modern terminal emulators render images using 4 primary mechanisms: **Kitty Graphics Protocol**, **Sixel**, **iTerm2 Inline Images (OSC 1337)**, and **Unicode Half-Blocks (ANSI 24-bit TrueColor)**.
2. **In Gomuks Specifically**: Gomuks **already has a complete ANSI TrueColor half-block renderer** in [`tui/lib/ansimage`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/lib/ansimage/ansimage.go) and the rendering pipeline in [`tui/messages/filemessage.go`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/messages/filemessage.go#L137-L174)!
3. **The Root Cause**: The reason your terminal showed the raw URL (`zalo-photo.jpg: http://...`) is that during the RPC backend refactor (commit `ae769a59`), [`msg.DownloadPreview()`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/messages/filemessage.go#L109-L130) was **commented out**, leaving `msg.imageData` empty.
4. **The Minimal Path**: Hooking `DownloadPreview()` to the existing daemon RPC download method [`gc.Download(mxc, encrypted)`](file:///home/bhd/Documents/Projects/bhd/gomuks/pkg/rpc/client/client.go#L214-L225) in an asynchronous goroutine and triggering a view redraw brings native inline image display to Gomuks immediately across **all terminals** with zero external dependencies.

---

## Part 1: Remote Search — How Terminals Display Images

Modern terminal emulators display images through in-band escape sequences or character cell fallbacks:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Terminal Image Technologies                           │
├──────────────────────┬──────────────────────┬───────────────────────────────────┤
│ Protocol / Method    │ Control Sequence     │ Supporting Terminals              │
├──────────────────────┼──────────────────────┼───────────────────────────────────┤
│ Kitty Graphics       │ APC: \x1b_G...;payload\x1b\│ Kitty, Ghostty, WezTerm, Foot     │
│ Sixel                │ DCS: \x1bPq... \x1b\ │ XTerm, Foot, WezTerm, mlterm      │
│ iTerm2 Inline        │ OSC: \x1b]1337;...   │ iTerm2, WezTerm, Mintty, VS Code  │
│ Half-Blocks (ANSI)   │ Unicode: '▄' + 24bit │ UNIVERSAL (all TrueColor terms)   │
└──────────────────────┴──────────────────────┴───────────────────────────────────┘
```

### 1. Protocol Breakdown & Mechanics

#### A. Kitty Graphics Protocol (`\x1b_G...;data\x1b\`)
* **How it works**: Uses Application Program Command (APC) escapes. Supports chunked Base64 PNG/JPEG/RGB transmission, z-indexing, placement IDs, and local shared-memory/file transmission (`t=f`).
* **Strengths**: Highest performance, full 32-bit RGBA with alpha blending, pixel-precise coordinates, hardware accelerated.
* **TUI Catch**: In full-screen TUI apps like `tcell`/`mauview`, images are virtual overlays. When the TUI redraws cells beneath the image or scrolls the message viewport, the Kitty image must be explicitly moved, clipped, or deleted by ID to prevent desync.

#### B. Sixel Graphics (`\x1bPq...;\x1b\`)
* **How it works**: DEC VT300-era standard (DCS sequence). Encodes 6 vertical pixels per byte with a 256-color palette.
* **Strengths**: Broad legacy terminal support; supported in modern terminal multiplexers like tmux 3.3a+ (with `allow-passthrough`) and Zellij.
* **TUI Catch**: Destructive stream rendering. Writing characters over Sixel-drawn rows erases the graphics.

#### C. iTerm2 Inline Images (`\x1b]1337;File=inline=1:...;data\x07`)
* **How it works**: Uses Operating System Command (OSC 1337). Sends standard Base64-encoded file payloads.
* **Strengths**: Simple one-shot stream; supported by iTerm2, WezTerm, and VS Code terminal.

#### D. Half-Block Characters (`▄` / `▀` with 24-bit TrueColor)
* **How it works**: Instead of raw pixel protocols, each character cell represents **two vertical pixels** using the Unicode lower half block `▄` (U+2584).
  * Cell **Background** Color = Top Pixel RGB (`\x1b[48;2;R;G;Bm`)
  * Cell **Foreground** Color = Bottom Pixel RGB (`\x1b[38;2;R;G;Bm`)
* **Strengths**: **100% native to character-grid TUIs**.
  * Handled like standard runes by [`tcell`](https://github.com/gdamore/tcell).
  * Seamlessly scrolls, wraps, clips, and redraws with zero flickering or protocol desynchronization.
  * Works through SSH, `tmux`, `screen`, and in every modern terminal (Ghostty, Alacritty, Kitty, WezTerm, Linux VT, Windows Terminal).

---

### 2. Search Results (Open Source Ecosystem)

#### What Exists (>100 Stars)
* **[`blacktop/go-termimg`](https://github.com/blacktop/go-termimg)**: Go terminal image library supporting Kitty, Sixel, iTerm2, and Halfblocks fallback with auto-detection.
* **[`BourgeoisBear/rasterm`](https://github.com/BourgeoisBear/rasterm)**: Go encoder for terminal graphics protocols.
* **[`chafa`](https://github.com/hpjansson/chafa)** (>4.5k stars): Standard C utility for terminal graphics conversion (symbols, sixel, kitty).
* **[`iamb`](https://github.com/ulyssa/iamb)** (>1.8k stars): Matrix TUI client in Rust. Implements Kitty, Sixel, and half-block fallback.
* **[`yazi`](https://github.com/sxyazi/yazi)** (>20k stars): Terminal file manager. Employs Kitty protocol with explicit placement IDs + sixel + chafa fallback.
* **`gomuks` Issue #103** ([`tulir/gomuks#103`](https://github.com/tulir/gomuks/issues/103)): *"Render images on supported terminals"*. Maintainer Tulir discussed `tcell` limitations with in-band escape sequences and opted for the half-block ANSI renderer approach.

#### What to Cherry-Pick
* **Native Cell-Grid Rendering**: The half-block approach (`▄` with TrueColor fg/bg) is the only method that integrates cleanly into a scrollable `tcell` view without rewriting the entire terminal rendering engine.
* **Async Fetch & Decode**: Never block the TUI event loop on media downloads or JPEG/PNG decodes; load asynchronously in a goroutine and emit a redraw event.

#### What to Avoid
* **Bypassing `tcell` with raw stdout escape sequences**: In `mauview`/`tcell`, `tcell.Screen` owns stdout. Emitting raw Kitty/Sixel escapes to `os.Stdout` corrupts the screen diff buffer, resulting in graphics disappearing or flickering when switching rooms or scrolling.
* **Synchronous Downloads in Message Parser**: Downloading images on the sync loop freezes the UI on slow networks.

---

## Part 2: How to Implement in Gomuks Itself

### Architecture & Current State in Codebase

```
                      Gomuks TUI Architecture
                      ───────────────────────
[Matrix Daemon / RPC]
        │
        │ HTTP GET /media/{server}/{media_id} (DownloadMedia)
        ▼
[pkg/rpc/client.GomuksClient]
        │  Download(mxc, encrypted) -> []byte
        ▼
[tui/messages.FileMessage]
   ┌────────────────────────────────────────────────────────┐
   │ imageData []byte                                       │
   │                                                        │
   │ CalculateBuffer():                                     │
   │   if len(imageData) == 0:                              │
   │       fallback to text URL  <─── [CURRENT STATE]       │
   │   else:                                                │
   │       ansimage.NewScaledFromReader(..., imgWidth)      │
   │       buffer = ansFile.Render() ['▄' TrueColor cells]  │
   └────────────────────────────────────────────────────────┘
        │
        ▼
[mauview.Screen / tcell.Screen]
   Standard Draw() into character grid -> Scrolls & Resizes Natively!
```

### Forensic Code Inspection

1. **The Renderer already exists**:
   Look at [`tui/lib/ansimage/ansimage.go:L146-L167`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/lib/ansimage/ansimage.go#L146-L167):
   ```go
   str[x] = tstring.Cell{
       Char:  '▄',
       Style: tcell.StyleDefault.Background(topColor).Foreground(bottomColor),
   }
   ```
2. **The Message View already handles it**:
   Look at [`tui/messages/filemessage.go:L157-L174`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/messages/filemessage.go#L157-L174):
   ```go
   img, _, err := image.DecodeConfig(bytes.NewReader(msg.imageData))
   imgWidth := img.Width
   if img.Width > width {
       imgWidth = width / 3
   }
   ansFile, err := ansimage.NewScaledFromReader(bytes.NewReader(msg.imageData), 0, imgWidth, color.Black)
   msg.buffer = ansFile.Render()
   ```
3. **The Download API already exists**:
   Look at [`pkg/rpc/client/client.go:L214-L225`](file:///home/bhd/Documents/Projects/bhd/gomuks/pkg/rpc/client/client.go#L214-L225):
   ```go
   func (gc *GomuksClient) Download(mxc id.ContentURI, encrypted bool) ([]byte, error) {
       resp, err := gc.GomuksAPI.(*rpc.GomuksRPC).DownloadMedia(...)
       ...
       return io.ReadAll(resp.Body)
   }
   ```

### Implementation Blueprint

To enable inline image display in Gomuks:

#### Step 1: Implement `DownloadPreview` asynchronously in `FileMessage`
Uncomment and adapt [`DownloadPreview()`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/messages/filemessage.go#L109-L130) to run in a background goroutine and trigger a view redraw once bytes arrive:

```go
func (msg *FileMessage) DownloadPreview(onLoaded func()) {
    if msg.matrix == nil || msg.URL.IsEmpty() || msg.Type != event.MsgImage {
        return
    }
    go func() {
        data, err := msg.matrix.Download(msg.URL, msg.IsEncrypted)
        if err != nil {
            debug.Printf("Failed to download image %s: %v", msg.URL, err)
            return
        }
        msg.imageData = data
        if onLoaded != nil {
            onLoaded()
        }
    }()
}
```

#### Step 2: Trigger Redraw from `parser.go` / `RoomView`
In [`tui/messages/parser.go`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/messages/parser.go#L204-L210), pass the redraw callback to `DownloadPreview`:
```go
case event.MsgImage:
    msg := NewFileMessage(room, matrix, evt, content)
    if !prefs.DisableDownloads && !prefs.DisableImages {
        renderer := msg.Renderer.(*FileMessage)
        renderer.DownloadPreview(func() {
            // Signal main loop to recalculate message buffer and redraw
            matrix.SendRedraw() 
        })
    }
    return msg
```

#### Step 3: Enforce Dimension Clamping
In [`FileMessage.CalculateBuffer`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/messages/filemessage.go#L161-L166), ensure large photos are constrained by both width and height (e.g., max 15 lines tall and max 35 columns wide) so a single image doesn't drown the room timeline:
```go
maxHeight := 16 // rows (which equals 32 vertical pixels with half-blocks)
maxWidth := min(width / 2, 40)
ansFile, err := ansimage.NewScaledFromReader(bytes.NewReader(msg.imageData), maxHeight * 2, maxWidth, color.Black)
```

#### Step 4: External High-Resolution Viewer Keybind (Progressive Enhancement)
Gomuks already provides [`ui.RunExternal`](file:///home/bhd/Documents/Projects/bhd/gomuks/tui/tui.go#L177-L188). In the room view key handler, bind a key (e.g., `o` or `Enter` on a selected image message) to execute the system image viewer (`xdg-open`, `feh`, `imv`, or `kitty icat` / `chafa` if on a supported terminal).

---

## Minimal Approach & Anti-Over-Engineering

> **Core Question**: *"What is the MINIMAL approach that could be implemented to serve the USER INTENTION?"*
>
> **User Intention**: The user sees raw text URLs for photos in `gomuks-terminal` and wants images displayed directly in the terminal if supported.
>
> **Minimal Answer**: Reactivate the dormant half-block TrueColor renderer already present in `tui/lib/ansimage` by connecting `DownloadPreview()` to `matrix.Download` in an async goroutine.

### Over-Engineered vs. Minimal Counterparts

| # | Over-Engineered Approach (REJECTED) | Minimal Counterpart (SHIP) |
|---|---|---|
| **1** | **Full Protocol Engine**: Writing custom terminal graphics drivers for Kitty APC, Sixel DCS, and iTerm2 OSC escape codes with cell-to-pixel matrix tracking (`TIOCGWINSZ`). | **Built-in Half-Blocks**: Use the existing `tui/lib/ansimage` TrueColor half-blocks (`▄`). Fits 100% into `tcell.Cell` grid, works everywhere with zero protocol desync. |
| **2** | **Complex Media Cache Daemon**: Adding SQLite metadata tables, LRU disk eviction policies, and retry queues for cached thumbnails. | **Direct Memory Buffer**: Store downloaded bytes directly in `msg.imageData`. Let the existing daemon HTTP media cache handle disk caching automatically. |
| **3** | **Embedded WebKit/WASM Overlay**: Spawning auxiliary graphical windows or floating canvas overlays over the terminal window. | **Native Subprocess Launch**: Re-use the existing `ui.RunExternal("xdg-open", url)` hook on keypress for users who want full-resolution inspection. |
| **4** | **Synchronous Blocking Downloads**: Pausing the message parser or UI loop until the network image payload arrives. | **Background Goroutine**: One `go func()` in `DownloadPreview()` that populates `msg.imageData` and flags `ui.NeedsRender = true`. |

---

## Decisions & Callouts

- **Inherit Existing Code**: Do not introduce new graphics libraries. The repository already vendor-ships `ansimage` and `imaging`. We inherit Tulir's intended architecture.
- **Callout on Kitty/Sixel Protocols in `tcell`**: While Kitty graphics offer pixel-perfect rendering, implementing Kitty protocol inside `tcell` requires modifying or bypassing `tcell`'s screen buffer, which causes scroll tearing and redraw collisions. The half-block approach avoids this entirely while satisfying terminal image display across all environments (including SSH and `tmux`).

---

## Status at end of turn 1
- Explored remote terminal graphics protocols (Kitty, Sixel, iTerm2, Half-blocks).
- Uncovered dormant `ansimage` half-block implementation inside gomuks codebase.
- User intention mapped to minimal implementation blueprint.
- Pointer to next turn: `2026-09-05-turn2-wezterm-iterm-priority.md`.
