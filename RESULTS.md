# Results — gomuks image preview + reply campaign

BASE REF: `ae64189e07ab212e19d38980febf74944ec5255a`
Branch with the work: `gated/c1-image-ondemand` (worktree `.worktrees/lane-c1-img`)

## Gate verdicts — observed by the ORCHESTRATOR'S OWN re-run

Delegate self-reports carry zero evidentiary weight here. Every verdict below was
produced by re-running the gate from this shell.

| gate | verdict | evidence | diff reviewed |
|---|---|---|---|
| **G1** lazy parse | ✅ **GREEN** | my re-run, exit 0 | yes — minimal, correct |
| **G2** on-demand select | ✅ **GREEN** | my re-run, exit 0 | yes — plus wiring checked beyond the gate |
| **G3** cell size | ✅ **GREEN** | clean-checkout re-run, exit 0 | yes — `cellsize.go` read in full |
| **G4** reply fallback | ⛔ **GATE INVALID → RETIRED** | see `G4-reply-fallback/README.md` | change reverted |

P2P (195 named tests) verified **GREEN** on the C1 lane by my own run: no test that
passed at base fails now. Compared as SETS against `base-known-red.txt`, per V5.

## Commits verified

- `70ae8af0` — G1: fetch removed from `ParseMessage`. This is the eager-load bug.
- `a48d56d2` — G2: `SelectImage` reason, live `OnSelect` arm, `Ctrl+i` → `load_image`,
  `RequestPreview()` on `FileMessage`, conditional selection reset so several images
  can be loaded in a row.

G2 verified beyond what its gate could see: `case "load_image":
view.StartSelecting(SelectImage, "")` sits directly beside the `reply` dispatch. A
green gate proves the detector fired, not that the diff is sound (L-GW6) — so the diff
was read.

## Final verified state

Clean checkout of `5a51d80d` (fresh worktree, fresh `go mod download`): builds, all
three gates exit 0, tracked-source guard passes, P2P green (0 failures, 100% pass). `FINAL_EXIT=0`.

Six commits on `gated/c1-image-ondemand`:

- `70ae8af0` G1 — eager fetch removed from `ParseMessage`
- `a48d56d2` G2 — on-demand select flow
- `4df6d4aa` G3 — queried cell geometry
- `83cfeeb1` G3 — track `cellsize.go` + `cellsize_test.go` (see harness defect 3)
- `37084f55` R-01 — fix(tui): restore image buffer rendering and viewport clipping parity (User Decision: Reading A)
- `5a51d80d` G2 verification — consume previewRequested in room view and add comprehensive unit tests (`filemessage_ondemand_test.go`, `room_view_ondemand_test.go`)

`cellsize.go` reviewed in full: real `TIOCGWINSZ` across stdout/stderr/stdin then
`/dev/tty`, explicit `ok=false` when the terminal reports no pixel metrics, no silent
10x20 fallback, duplicate `const n` collapsed into `DefaultCellAspectRatio`.
`TestCellSize_FallbackPath` and `TestCellAspectRatio_DefaultConstant` pass.

Convergence note: G3 took three attempts. Attempts 1-2 (`model=flash`) made **zero
edits to `filemessage.go` across ~29 iterations**, despite having edited that same file
for G2 in that same worktree — so the task was reachable and the loop simply did not
converge. Attempt 3 (`model=pro`, `--sub 0`) closed it in 7 iterations. Model choice was
the decisive variable, not the prompt.

## Harness defect 3 — untracked source passed every gate

A clean checkout of `4df6d4aa` did **not compile**:
`geometry.go:39:17: undefined: DefaultCellAspectRatio`.

Cause: `~/.gitignore_global:69` contains a bare `lib/` pattern, which matches
`tui/lib/`. Both new files (`cellsize.go`, `cellsize_test.go`) were silently ignored;
`git add -A` skipped them without a word. Every gate still passed, because gates read
the WORKING TREE, where the files were present. `git status --porcelain` showed nothing,
since ignored files need `--ignored` to appear.

Fixed in two places:
1. Repo `.gitignore` gained `!tui/lib/` — negating the DIRECTORY is required, because
   git cannot re-include a file whose parent directory is excluded. The user's global
   gitignore was deliberately left untouched.
2. `check-lane.sh` gained a **tracked-source guard**: any `.go` file on disk but not in
   git is now `exit 2` (gate-invalid — broken measurement environment, not a red
   verdict). Confirmed to flip the previously-false-green lane to exit 2.

This is the defect the "verify PUSHED state, not local worktree" rule exists to catch.
Caught by the rule, exactly as written.

## RESOLVED per user decision

**R-01** — User decision: **Reading A ruled**. Dispatched by user via `/goal finish… — make go test -count=1 ./tui + ./tui/lib/termimg/... 0-fail + go build ./cmd/gomuks-terminal green`.
Tests are right: total rendered height must stay ≤ `DefaultMaxHeight` (16). Commit `43b541b2` was invalid to prepend text into image buffer causing height overflow, placeholder corruptions, and boundary clipping bypass.
Resolved in commit `37084f55` (`fix(tui): restore image buffer rendering and viewport clipping parity`).
Result: 0 failures across all 69 messages tests, all 195 P2P tests passing, all gates green.

## BLOCKED on user decision

**G4** — retire permanently, or renegotiate 8 tests to get spec behaviour?

## Harness defects found and fixed during the campaign

1. **My gate scripts could not run in a worktree.** `[ -d "$REPO/.git" ]` is false
   inside a worktree (`.git` is a gitdir *file*). Every gate would have exited 2 in
   every lane. Replaced with `git -C "$REPO" rev-parse --git-dir`. Caught by a
   green-side probe before launch, not by the red proof — which is precisely why the
   contract demands proving the green side too (L-SW1, L-V5).
2. **Circuit breaker vs F2P gates.** agyralph auto-disables a check after 5
   consecutive failures. An F2P gate is red *by construction* at launch, so the trip
   was guaranteed; lane C2 then ran iterations 6-12 with no gate feedback while
   `state.json` still read `running`. Mitigation: pre-seed the gate's current failure
   output into the launch prompt, and grep the run log for `CIRCUIT BREAKER` before
   believing any lane outcome.

## Parallelism, stated honestly

Ceiling was **2 lanes**, not more. G1/G2/G3 all touch `tui/messages/filemessage.go`, so
per `parallelism.md` they are ONE ordered component (C1), not three lanes. Only G4 was
independent (C2). Not inflated to look parallel.

## Deferred (reviewed, deliberately not gated this pass)

Kitty image IDs never deleted (no `a=d`) · `Draw` holds the write mutex across tty I/O ·
`Clone()` drops `cachedKitty`/`imageID`/`mask` → dead placeholders · triple
`ValidateImageSafety` per image · eager ansimage render in the iTerm2 arm · unbounded
`renderCache` · `idExtra` always 0 · no disk cache + `context.TODO()` in `client.go` ·
hardcoded `DefaultReplySnippetMaxCols`. All touch `filemessage.go` → would serialize
into C1 and blow the batch budget.
