# Challenge: P1-F18 — No-Flicker Rendering

## Purpose

Prove HelixCode's Phase 1 / Feature 18 rendering pipeline actually works
end-to-end against real `*bytes.Buffer` sinks (and the live `os.Stdout`
when it happens to be a TTY). Per Article XI §11.9, every PASS must
carry positive runtime evidence captured during execution. The harness
emits **byte-level captures** of the rendered output and the
load-bearing assertion in each phase compares those captures against
documented byte invariants — counts of ANSI control sequences for
fancy mode, **zero**-counts for plain mode, and a strict-smaller delta
relation for the dirty-region diff path.

The harness wires together the F18 surface area:

- `render.NewANSIRenderer` (T03) — fancy-mode Renderer with
  CR + clear-line in-place updates and per-line frame diff.
- `render.NewPlainRenderer` (T04) — plain-mode Renderer with the
  zero-ANSI / zero-CR invariant (\r is stripped, \x1b is never
  emitted by renderer logic).
- `render.NewViewport` (T05) — frame buffer + per-line dirty
  tracking, exercised transitively via `RenderFrame`.
- `render.NewRenderer` (T06) — factory with the
  `opts.Mode → HELIXCODE_RENDER → IsTTY → ModePlain` precedence
  ladder.
- `render.RenderTextBlock` (T08) — multi-line tool-output helper
  that turns a `\n`-joined string into a `Frame` and routes through
  `Renderer.RenderFrame`.

Every always-runs phase is hermetic: bytes.Buffer is the destination,
no stdin/stdout piping, no env var dependency, no LLM call. Phase E
is the gated exception — it asserts the factory's TTY-detect path
end-to-end against a real terminal but SKIP-OKs cleanly when the
harness is launched under a pipe / redirected stdout / non-interactive
runner.

## Procedure

1. Build the F18 challenge harness from
   `helix_code/tests/integration/cmd/p1f18_challenge`.
2. Run the harness; it executes five phases:
   a. **Phase A — STREAMING-FANCY (always runs).** Construct an
      `ansiRenderer` over a `bytes.Buffer`. Call `Begin("a")`, then
      `WriteToken` for each of 10 whitespace-bounded words (each
      word ending with a single space, no newline), then `Commit`,
      then `Close`. Capture the buffer's bytes and assert: at least
      one `\x1b[?25l` (hide-cursor) sequence, at least 10
      `\r\x1b[K` (CR + clear-line) sequences (one per token), at
      least one `\x1b[?25h` (show-cursor) sequence, and a final
      newline. Print evidence: byte count + per-sequence counts.
   b. **Phase B — STREAMING-PLAIN (always runs).** Construct a
      `plainRenderer` over a `bytes.Buffer`. Same 10 tokens.
      Capture the buffer's bytes and assert ZERO `\x1b` (ANSI) and
      ZERO `\x0d` (CR) bytes — the load-bearing zero-ANSI / zero-CR
      invariant. Assert all 10 words are present in the captured
      transcript (the renderer is not eating tokens). Print
      evidence: byte count + zero-counts.
   c. **Phase C — DIRTY-REGION-DIFF (always runs, load-bearing).**
      Construct an `ansiRenderer` over a `bytes.Buffer`. Call
      `RenderTextBlock(r, "blk", block1)` with a 3-line block.
      Snapshot `firstLen = buf.Len()`. Call
      `RenderTextBlock(r, "blk", block2)` where exactly one line
      differs. Compute `delta = buf.Len() - firstLen` and assert
      `delta < firstLen` (strict-smaller invariant: re-rendering
      one of three lines MUST emit fewer bytes than the initial
      full render). Apply a `\x1b\[\d+A` regex to the delta bytes
      and assert exactly one match (single in-place cursor-up
      rewrite, not a full re-paint). Print evidence: firstLen,
      delta, the delta<firstLen verdict, cursor-up count.
   d. **Phase D — TTY-FALLBACK (always runs).** Call
      `render.NewRenderer` with `Writer = &bytes.Buffer{}` and
      `EnvLookup = func(string) string { return "" }` (env unset).
      A bytes.Buffer is by definition not a TTY, so the factory
      MUST resolve the auto rung to `ModePlain`. Assert
      `r.Mode() == ModePlain`. Render a normal token stream and
      re-assert the zero-ANSI / zero-CR invariant on the output
      bytes. Print evidence: resolved mode.
   e. **Phase E — REAL-TTY (gated).** If
      `term.IsTerminal(int(os.Stdout.Fd()))` is true: construct a
      factory pointed at `os.Stdout` with env unset and assert
      `r.Mode() == ModeFancy`; render a small text block; print
      `phaseE: real-TTY rendered`. Otherwise print
      `[skipped: stdout is not a TTY]` plus the explicit SKIP-OK
      reason — the assertion only carries evidential weight against
      a real terminal and a forced PASS would be a bluff.
3. Anti-bluff smoke clean over the harness file + this CHALLENGE.md
   + run.sh (the smoke regex is built from string fragments so the
   script does not match itself).
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F18 challenge harness PASS` final line.
- **Phase A**: hide-cursor count ≥ 1; CR+clear-line count ≥ 10
  (one per token); show-cursor count ≥ 1; output contains ≥ 1 `\n`.
- **Phase B**: ANSI-byte count == 0; CR-byte count == 0; every word
  in `streamWords` is a substring of the captured output.
- **Phase C**: `delta < firstLen` (strict); cursor-up sequence count
  in delta is exactly 1.
- **Phase D**: factory-resolved `r.Mode() == ModePlain`; output
  ANSI-count == 0 and CR-count == 0.
- **Phase E**: when stdout is a real TTY, `r.Mode() == ModeFancy`
  and rendering succeeds; otherwise SKIP-OK with reason.
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh (regex built from fragments).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A.** A regression that lost the hide-cursor first-Begin
  emit, the per-token CR + clear-line emit, the show-cursor Close
  emit, or the terminal newline at Commit will trip one of the four
  byte-count assertions. The captures are real bytes from a real
  `*bytes.Buffer`; there is no metadata path through which a
  broken renderer could fake the counts.
- **Phase B.** The zero-ANSI / zero-CR invariant is the entire
  reason the plain renderer exists. A regression that leaked any
  `\x1b` from renderer logic (e.g. an accidental ANSI passthrough
  in line buffering) or that failed to strip `\r` from caller
  text would push either count above zero and trip the assertion.
- **Phase C — load-bearing.** This is the dirty-region diff proof.
  A regression that re-rendered the entire block on the second
  call (full re-paint) would emit roughly the same bytes again,
  violating `delta < firstLen`. A regression that emitted three
  cursor-up sequences (one per line, indicating per-line
  re-rewrite of unchanged lines) or zero cursor-up sequences (full
  re-paint without cursor moves) would violate the
  exactly-one-cursor-up assertion. Either failure mode immediately
  trips this phase.
- **Phase D.** A regression in the factory's auto-detect ladder
  (e.g. defaulting to fancy on unknown writer types, or honouring
  a stale HELIXCODE_RENDER value despite the lookup returning "")
  would land in fancy mode against a `bytes.Buffer` and the
  zero-ANSI re-assertion would catch the leak.
- **Phase E.** Gated SKIP-OK is acceptable: the assertion only
  carries evidential weight against a real terminal. A forced
  PASS in a non-TTY environment (CI runner, redirected output)
  would be the exact "test passed but feature broken" failure
  mode the constitutional anchor was written to prevent.
