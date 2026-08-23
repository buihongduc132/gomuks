# gomuks

**Fork**: buihongduc132/gomuks (upstream: gomuks/gomuks)
**Purpose**: Add user-configurable hotkeys to web client

Matrix client (Go backend + TypeScript/React web frontend). Web client = most mature, daily-use ready. Terminal client experimental. This fork adds configurable keybindings.

## Stack

- Backend: Go + mautrix-go SDK (can run as daemon/bouncer OR embedded)
- Frontend: TypeScript/React (Vite bundler)
- Build: `./build.sh` (generates web assets, compiles daemon)

## Commands

```bash
# Build web + daemon (requires Go 1.21+, Node.js)
./build.sh

# Build daemon only (no web regen)
./build-noweb.sh

# Build terminal client
./build-terminal.sh

# Test
go test ./...
```

## Architecture

```
cmd/
├── gomuks/         # daemon entry (web backend server)
├── gomuks-terminal/# TUI client
└── ...
pkg/
├── gomuks/         # core backend logic
└── hicli/          # high-level client API
web/
├── src/ui/keybindings.ts  # HOTKEY CODE (keyDownMap, keyUpMap)
└── dist/           # built assets (served by daemon)
```

## Current hotkeys (defaults, user-configurable)

Web: `web/src/ui/keyconfig.ts` defaults, loaded via localStorage `gomuks-keybindings`, edited in Settings → Keybindings.
Terminal: `tui/config/keybindings.yaml` embed + `~/.config/gomuks/terminal-keybindings.yaml` override. Room `Ctrl+r` = reply-select.

| Key | Action |
|-----|--------|
| Escape | Close right panel OR clear active room |
| Ctrl+k | Focus room search |
| Alt+↑ | Next room in list |
| Alt+↓ | Previous room in list |
| Ctrl+f | Open search panel |
| Ctrl+ArrowUp / Ctrl+ArrowDown | Web: select previous/next message to reply (preference-gated) |
| Ctrl+r | Terminal: enter select-to-reply mode |

## Testing

- `cd web && npx vitest --run` — 354+ unit tests (jsdom, v8 coverage)
- `./scripts/cov-gaps.sh` — per-file coverage gap list
- Coverage target: >=80%/file × 4 categories (lines/statements/functions/branches)
- Config: `web/vitest.config.ts` (plain object — see comment re vite8 rolldown bug)

## Fork intention

**Goal**: Make hotkeys user-configurable (config file + UI settings panel) + reply-select hotkey in web and terminal.

See `flow/intentions/configurable-hotkeys.md` and `flow/intentions/reply-hotkey.md`.

## Deployment (local bin via mise)

Mirrors `../pi-plugins` deploy pattern (lockfile guard, atomic install, manifest, smoke test).

```bash
mise run deploy          # SERVER: build (noweb) → ~/.local/bin/gomuks-dev   ← DEFAULT
mise run deploy-client   # CLIENT: build terminal → ~/.local/bin/gomuks-terminal-dev
cd web && npm ci         # only needed for deploy-full
mise run deploy-full     # full build (web assets) → gomuks-dev
mise run deploy-prod     # PROD bin gomuks-daemon-patched (use with care)
mise run deploy-status   # show manifests (mode/commit/branch/time)
```

- **server** = daemon (`cmd/gomuks`) — singleton: lock + `.bak` rollback + manifest.
- **client** = terminal (`cmd/gomuks-terminal`) — multiple installs: default `gomuks-terminal-dev`; extra copies via `MODE=client BIN_NAME=gomuks-terminal-dev2 bash ops/deploy/deploy-local.sh` (each gets own manifest). `FORCE_BAK=1` to back up on same-name overwrite.

- Config parity: `gomuks-dev` reads prod config `~/.config/gomuks` + data `~/.local/share/gomuks` unchanged.
- Isolated run: `GOMUKS_ROOT=~/.local/share/gomuks-dev gomuks-dev`
- Previous version kept as `<bin>.bak`; manifest `<bin>.manifest.json` beside bin.
- ⚠ Don't run `gomuks-dev` daemon while prod daemon runs — same sqlite DB (`_txlock=immediate`) conflicts.

## Gotchas

- Web keyDownMap is rebuilt from `keyconfig` defaults + localStorage overrides (`reload()` / `gomuks-keybindings-changed`)
- Any key not in keyDownMap + not modifier-only + target=body → auto-focuses composer
- Enter/Tab stay reserved; reply keys stay composer-scoped (need composer state)
- Upstream updates: rebase fork on upstream/main (gomuks/gomuks active, ~weekly commits)

## Boundaries

**Never**:
- Break upstream parity (always rebase, not diverge)
- Commit built assets (`web/dist/`) — gitignored, rebuild locally

**Ask first**:
- New features beyond configurable hotkeys (upstream might accept PR → ask them first)

## Pointers

| Area | Read |
|------|------|
| Hotkey implementation plan | `flow/intentions/configurable-hotkeys.md` |
| Reply-select hotkey | `flow/intentions/reply-hotkey.md` |
| Upstream docs | https://docs.mau.fi/gomuks/ |
| Matrix room | #gomuks:gomuks.app |
