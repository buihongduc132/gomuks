# References

> Sources consulted during this explore session.

## Source files
- `tui/messages/filemessage.go` — Gomuks TUI message renderer for media/files; contains dormant image rendering pipeline and commented-out `DownloadPreview()`.
- `tui/lib/ansimage/ansimage.go` — Gomuks built-in ANSI half-block (`▄`) TrueColor renderer converting images to character cells.
- `pkg/rpc/client/client.go` — Gomuks RPC client containing `Download(mxc, encrypted)` method connecting to the daemon media download endpoint.
- `pkg/rpc/client.go` — Gomuks RPC implementation containing `DownloadMedia` HTTP handler.
- `tui/messages/parser.go` — Message event parser that instantiates `FileMessage` and calls `DownloadPreview()`.
- `tui/tui.go` — Core Gomuks TUI application lifecycle, redrawing, and `RunExternal` subprocess runner.
- `tui/config/config.go` — User preferences and terminal capability heuristics (`InlineURLsProbablySupported`).

## Documents
- `https://github.com/tulir/gomuks/issues/103` — Upstream Gomuks issue discussing in-band image rendering on supported terminals (Kitty, Sixel, `ansimage`).
- `https://iterm2.com/documentation-images.html` — iTerm2 Inline Images Protocol specification (OSC 1337).
- `https://sw.kovidgoyal.net/kitty/graphics-protocol/` — Kitty Graphics Protocol specification (APC `\x1b_G...`).
- `https://wezfurlong.org/wezterm/imgcat.html` — WezTerm image display capabilities and protocol support (iTerm2, Kitty, Sixel).

- `https://github.com/ulyssa/iamb` — Matrix terminal client implementing image previews via `ratatui-image`.
- `https://github.com/ext0l/ratatui-image` — Terminal image rendering crate providing Picker protocol auto-detection, cell-masking, and halfblock fallbacks.

## Code patterns
- `tstring.Cell{ Char: '▄', Style: tcell.StyleDefault.Background(topColor).Foreground(bottomColor) }` — Two vertical pixels per character cell, native to `tcell` screen buffer (`tui/lib/ansimage/ansimage.go`).
- `msg.matrix.Download(msg.URL, msg.IsEncrypted)` — Decrypted media retrieval from Gomuks daemon over local RPC (`pkg/rpc/client/client.go`).
- `ui.RunExternal(executablePath, args...)` — Screen suspension pattern for external viewers (`tui/tui.go`).
- `ratatui-image` cell-masking pattern — Fills TUI buffer character grid with transparent/empty cells over the image bounding box to prevent double-buffering redraw overwrites.
