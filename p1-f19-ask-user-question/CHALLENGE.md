# Challenge: P1-F19 — ask_user Multiple-Choice Tool

## Purpose

Prove HelixCode's Phase 1 / Feature 19 `ask_user` tool actually works
end-to-end against a real `*bytes.Buffer` reader and writer pair. Per
Article XI §11.9, every PASS must carry positive runtime evidence
captured during execution. The harness emits **byte-level captures**
of rendered prompts, **byte-offset positive evidence** of preview
text appearing in the writer, and **reader-position invariants** that
prove (rather than imply) whether the prompter consumed input.

The harness wires together the F19 surface area:

- `askuser.Question` / `askuser.Choice` / `askuser.Result` (T02) —
  validated input + output types.
- `askuser.NewStdinPrompter` (T03) — production Prompter with
  non-TTY short-circuit, retry loop, timeout, and F18 renderer.
- `askuser.NewAskUserTool` (T04) — Tool wrapper that exposes the
  Prompter to the agent registry.
- `askuser.ErrInteractiveTerminalRequired` (T02 sentinel) — surfaced
  via `errors.Is` through the tool's `%w` wrapping.

Every always-runs phase is hermetic: `bytes.Buffer` is the source and
sink, `IsTTY` is a deterministic closure, the renderer is forced to
plain mode so substring assertions are not confounded by ANSI escape
sequences. There is no gated phase — all five phases run in every
environment and produce evidence.

## Procedure

1. Build the F19 challenge harness from
   `helix_code/tests/integration/cmd/p1f19_challenge`.
2. Run the harness; it executes five phases:

   a. **Phase A — TTY-WITH-INPUT-RETURNS-CHOICE (always runs).**
      Construct a `stdinPrompter` with `Reader = bytes.NewBufferString("2\n")`,
      a fresh `bytes.Buffer` writer, and `IsTTY = func() bool { return true }`.
      Wrap in `AskUserTool`. Execute with three choices labelled
      Apply/Backout/Cancel mapped to values a/b/c. Assert the result
      map contains `value=="b"`, `index==1`, `used_default==false`.
      Assert the writer captures contain `"Pick:"` and the numbered
      list `1. Apply`, `2. Backout`, `3. Cancel`. Assert
      `reader.Len()==0` (the buffer was fully consumed up to and
      including the `\n`). Print evidence: input + value + index +
      writer byte count + reader remaining.
   b. **Phase B — NON-TTY-WITH-DEFAULT-RETURNS-DEFAULT (always runs,
      load-bearing).** Construct with `IsTTY = func() bool { return false }`,
      reader pre-loaded with `"2\n"` (must NOT be read), Default `"b"`.
      Execute. Assert `value=="b"`, `index==1`, `used_default==true`.
      **Reader-untouched invariant**: `reader.Len()` MUST equal the
      initial length — a non-TTY short-circuit that secretly read the
      buffer before returning the default would drop Len. **Writer-
      empty invariant**: `writer.Len()==0` because no rendering should
      reach a non-TTY destination. Print evidence: value + used_default
      + reader remaining (with untouched verdict) + writer byte count
      (with empty verdict).
   c. **Phase C — NON-TTY-NO-DEFAULT-ERRORS (always runs).** IsTTY
      false, no Default. Execute. Assert the returned error chain
      satisfies `errors.Is(err, askuser.ErrInteractiveTerminalRequired)`.
      Assert writer empty. Print evidence: error string + sentinel
      verdict + writer byte count.
   d. **Phase D — PREVIEW-VISIBLE-IN-OUTPUT (always runs).** TTY;
      reader has `"1\n"`. Choices carry inline preview strings such
      as `"applies the change to disk"`. Execute. Assert
      `value=="a"`. Use `strings.Index` against the writer bytes to
      find the **byte offset** at which each preview substring
      appears; assert the offsets are non-negative. Print evidence:
      preview text plus the exact byte offset where it begins (this
      is the load-bearing positive evidence — a metadata-only
      regression that "set" the preview field but never rendered it
      would produce offset -1 and trip the assertion).
   e. **Phase E — INVALID-INPUT-RETRY (always runs).** TTY; reader
      has `"9\n2\n"` — out-of-range followed by valid. Execute.
      Assert `value=="b"`, `index==1`. Assert the writer rendered the
      question text at least twice (one render per attempt). Assert
      the writer contains the `"1-3"` range hint. Assert reader fully
      consumed. Print evidence: prompt count + writer byte count +
      reader remaining.
3. Anti-bluff smoke clean over the harness file + this CHALLENGE.md
   + run.sh (the smoke regex is built from string fragments so the
   script does not match itself).
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F19 challenge harness PASS` final line.
- **Phase A**: result `value=="b"` `index==1` `used_default==false`;
  writer contains `"Pick:"` plus the numbered choice list; reader
  fully consumed (`reader.Len()==0`).
- **Phase B**: result `value=="b"` `index==1` `used_default==true`;
  reader length unchanged from initial (load-bearing reader-untouched
  invariant); writer length zero.
- **Phase C**: returned error satisfies `errors.Is(err, ErrInteractiveTerminalRequired)`;
  writer length zero.
- **Phase D**: result `value=="a"`; both preview substrings present
  in writer (`strings.Index >= 0`); harness prints the exact byte
  offsets.
- **Phase E**: result `value=="b"` `index==1`; question text count in
  writer ≥ 2; writer contains the `1-3` range hint; reader fully
  consumed.
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh (regex built from fragments).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A — reader consumption**. A regression that fabricated a
  Result without reading the input would leave `reader.Len()` equal
  to the initial length, tripping the consumption assertion. The
  rendered prompt assertions guard against the inverse failure mode
  (reader consumed but writer never written).
- **Phase B — load-bearing reader-untouched + writer-empty**. The
  non-TTY short-circuit is the entire reason `IsTTY` exists. A
  regression that wrapped the reader in `bufio.NewReader` and called
  `ReadString` before checking the TTY flag would drop `reader.Len()`
  even when returning the default; a regression that rendered the
  prompt before the TTY check would push `writer.Len()` above zero.
  Either failure mode immediately trips this phase, and no metadata
  inspection can substitute — the byte counts are the contract.
- **Phase C — sentinel propagation**. The error must travel through
  `AskUserTool.Execute`'s `%w` wrap and remain reachable via
  `errors.Is`. A regression that replaced `%w` with `%v` (string
  concatenation) would lose the sentinel and trip the assertion.
- **Phase D — byte-offset positive evidence**. The preview text MUST
  appear at a positive byte offset within the writer captures. A
  regression that "set" preview metadata but never piped it through
  `FormatQuestion` (or that rendered it to a separate sink) would
  return offset -1 and fail. This is the F19 equivalent of F18's
  byte-count evidence: a real byte at a real offset, not "the
  preview field was non-empty".
- **Phase E — retry plus hint**. A regression that hard-failed on the
  first invalid input would either return an error or silently accept
  the wrong index; either failure mode trips the value/index assertion.
  A regression that retried but did not redraw would produce
  `promptCount==1`, tripping the count assertion. A regression that
  redrew but lost the range hint would trip the substring assertion.
