# P2-F23 — Cline Browser Tool

**Phase:** Phase 2 — CLI Agent Porting
**Spec:** `docs/superpowers/specs/2026-05-07-p2-f23-cline-browser-tool-design.md` (commit `83d401d`)
**Plan:** `docs/superpowers/plans/2026-05-07-p2-f23-cline-browser-tool.md`

## Summary

Real, end-to-end 6-tool browser-automation suite modelled on cline's
Puppeteer surface: `browser_navigate` / `browser_snapshot` /
`browser_click` / `browser_type` / `browser_screenshot` /
`browser_close`. Atomic-pointer `BrowserManager`; lazy-create on first
navigate; `sync.Once` close. Headless default;
`HELIXCODE_BROWSER_HEADED=true` opt-in. Screenshots written to
`$XDG_DATA_HOME/helixcode/browser/screenshots/<session>/<n>.png` with
PNG-magic + DecodeConfig + size>1024 verification.

Per-tool RequiresApproval (P2-F21 integration): navigate / click / type /
close → LevelEdit; snapshot / screenshot → LevelReadOnly. Coexists with
the legacy `browser_legacy_*` multi-browser tools.

## Phases (seven A-G; gated on chromium availability)

1. **PHASE-A — NAVIGATE-AND-SNAPSHOT** (4 checks): navigate + snapshot.html
   contains `FIXTURE_LOADED_42`, len > 100, title == `F23-FIXTURE`.
2. **PHASE-B — SNAPSHOT-MODE-TEXT** (2 checks): text mode contains the
   sentinel and lacks raw HTML tags.
3. **PHASE-C — CLICK-MUTATES-DOM** (3 checks): pre=UNCLICKED → click(#b)
   → post=CLICKED_42 (positive byte differential).
4. **PHASE-D — TYPE-INTO-INPUT** (2 checks): type(#in, "HELIX_42") +
   chromedp.Value reads back `HELIX_42`.
5. **PHASE-E — SCREENSHOT-PNG-MAGIC** (4 checks): file exists, size > 1024,
   first 8 bytes == `89 50 4E 47 0D 0A 1A 0A`, `image/png.DecodeConfig`
   succeeds.
6. **PHASE-F — CLOSE-TEARS-DOWN** (2 checks): close + subsequent snapshot
   returns `errors.Is(err, ErrNoActiveSession)`.
7. **PHASE-G — CONCURRENT-SESSION-SHARING** (2 checks): 5 goroutines call
   `EnsureSession`; all return identical `*BrowserSession` pointer.

## Anti-bluff invariants

- Fixture sentinel string equality on every snapshot (positive evidence
  the page actually loaded; not a `chromedp.Navigate` no-op).
- DOM-mutation byte differential (PHASE-C): UNCLICKED → CLICKED_42 with
  the post-snapshot lacking the pre-state.
- Input-value byte readback (PHASE-D): chromedp.Value reads `.value` of
  the input, asserting the bytes reached the DOM.
- PNG-magic bytes match BEFORE writing (T07 in-memory verification) AND
  size > 1024 AFTER writing (positive disk evidence; not a chromedp-empty-
  buf bluff).
- ErrNoActiveSession post-close (positive teardown evidence).
- Pointer equality across N=5 concurrent EnsureSession calls.

## Run

```bash
cd challenges/p2-f23-cline-browser-tool
./run.sh
```

If chromium is unavailable on the host, the harness emits
`SKIP-OK: #P2-F23 chromium not available` and exits 0. Any byte-evidence
mismatch is a hard failure (exit 1).

## Composition with previous features

- **F21 (approval modes):** the four LevelEdit tools (navigate, click,
  type, close) gate behind ModeAutoEdit or higher; snapshot + screenshot
  bypass the approval gate (LevelReadOnly).
- **F22 (autocommit):** browser tools have no path parameter →
  per-tool path-derivation table returns nil → committer skips with
  reason "no changes" → no spurious commits.
- **F04 (worktree):** browser sessions are per-CLI-instance; worktree
  isolation irrelevant.

## Permitted skip

`SKIP-OK: #P2-F23 chromium not available` is the ONLY permitted skip and
applies only when `chromium / chromium-browser / google-chrome / chrome`
is absent from `$PATH`. Any other failure path is a hard challenge
failure.
