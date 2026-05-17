# Challenge: P1-F17 — Smart File Editing

## Purpose

Prove HelixCode's Phase 1 / Feature 17 smart file-editing pipeline
actually works end-to-end against real on-disk tempdirs and the real
F08 `*multiedit.MultiFileEditor` transactional committer. Per Article
XI §11.9, every PASS must carry positive runtime evidence captured
during execution. The harness emits **sha-256** content hashes BEFORE
and AFTER each phase's expected mutation; the relation between those
hashes is the load-bearing positive-evidence anchor for that phase.

The harness wires together the F17 surface area:

- `smartedit.Parse` (T03) — SEARCH/REPLACE prompt parser with
  path-stickiness and per-block line tracking.
- `smartedit.ApplyPlanToContent` (T04) — lenient re-search + ambiguity
  rejection + `IsBinary` refusal.
- `smartedit.Differ` (T05) — unified-diff wrapper re-using the F08
  `multiedit.DiffManager`.
- `smartedit.SmartEditTool` (T06) — agent-callable Tool wiring
  parse + apply + diff + transactional commit + post-write re-read.
- `smartedit.NewMultieditCommitter` (T06) — production adapter
  routing `CommitFiles` through `BeginEdit` → `AddEdit` → `Preview`
  → `Commit` on a real `*multiedit.MultiFileEditor`.
- `smartedit.SmartEditTool.Commit` (T07) — convenience wrapper used
  by `/edit commit`; identical Execute body to the agent path.

Every phase is **always-runs**: there is no real-LLM dependency, no
external service, no env-gated skip. The harness creates a fresh
tempdir per phase, builds a per-phase `MultiFileEditor` rooted at
that tempdir (so backups + workspace live entirely inside the temp
tree), and tears it down on completion.

## Procedure

1. Build the F17 challenge harness from
   `helix_code/tests/integration/cmd/p1f17_challenge`.
2. Run the harness; it executes seven phases:
   a. **Phase A — SINGLE-FILE edit applied (always runs).** Write
      `a.txt` with content `"hello\nold-line\nworld\n"`. Build a
      single-block prompt replacing `old-line` → `new-line`. Call
      `tool.Commit`. Assert `result.Atomic==true`, then re-read
      `a.txt` from disk and assert sha256(after) equals the canonical
      sha256 of `"hello\nnew-line\nworld\n"` AND differs from
      sha256(before).
   b. **Phase B — NOT-FOUND aborts (always runs).** Write `b.txt`
      with content `"hello\n"`. Build a prompt with SEARCH text that
      is absent from the file. Call `tool.Commit`. Assert
      `result.Atomic==false` and the per-block outcome reports
      `not-found`. Re-read `b.txt` from disk; sha256(after) MUST
      equal sha256(before) — the whole-prompt gate prevented any
      write.
   c. **Phase C — MULTI-FILE atomic commit (always runs).** Write
      `c1.txt`=`"alpha\n"` and `c2.txt`=`"beta\n"`. Build a
      two-block prompt: `alpha`→`gamma` on `c1.txt` and
      `beta`→`delta` on `c2.txt`. Call `tool.Commit`. Assert
      `Atomic==true` and per-file sha256(after) equals the canonical
      sha256 of the expected post-content for that file.
   d. **Phase D — ROLLBACK on partial failure (always runs, load-
      bearing).** Write `d1.txt`=`"applies-fine\n"` and
      `d2.txt`=`"hello\n"`. Build a two-block prompt where block 1
      replaces `applies-fine` → `changed` (would apply cleanly in
      memory) and block 2 SEARCHes for `absent-text` in `d2.txt`
      (will fail). Call `tool.Commit`. Assert `Atomic==false`. Then
      re-read **both** files from disk; sha256(after) MUST equal
      sha256(before) for **both** files. This is the critical
      atomicity proof: file 1's block applied in memory but the
      whole-prompt gate prevented its write because file 2's block
      failed.
   e. **Phase E — DIFF EXACTNESS (always runs).** Single-file edit
      `e.txt`: `old-line` → `new-line`. After `Commit`, assert
      `result.Diff` contains the substrings `-old-line` and
      `+new-line` (the unified-diff `-`/`+` lines describing the
      change). Print a 6-line diff excerpt for visual inspection.
   f. **Phase F — AMBIG (always runs).** Write `f.txt` with content
      `"duplicate\nfiller\nduplicate\n"` (SEARCH text appears twice).
      Build a prompt with SEARCH=`duplicate`. Call `tool.Commit`.
      Assert `Atomic==false` and the per-block outcome reports
      `ambiguous`. Re-read; sha256(after)==sha256(before).
   g. **Phase G — BINARY (always runs).** Write `g.bin` with content
      `"\x00abc\n"` (NUL byte → binary detection). Build a prompt
      with SEARCH=`abc`. Call `tool.Commit`. Assert `Atomic==false`
      and the atomic-error string mentions `binary file`. Re-read;
      sha256(after)==sha256(before).
3. Anti-bluff smoke clean over the harness file + this CHALLENGE.md
   + run.sh (the smoke regex is built from string fragments so the
   script does not match itself).
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F17 challenge harness PASS` final line.
- **Phase A**: `result.Atomic==true`; sha256(after) equals the
  canonical sha256 of the expected post-content; sha256(after) !=
  sha256(before).
- **Phase B**: `result.Atomic==false`; per-block outcome includes
  `not-found`; sha256(after)==sha256(before).
- **Phase C**: `result.Atomic==true`; per-file sha256(after) matches
  the canonical sha256 of each file's expected post-content.
- **Phase D**: `result.Atomic==false`; **both** files'
  sha256(after)==sha256(before). File 1 NOT written despite its
  block applying in memory.
- **Phase E**: `result.Diff` contains both `-old-line` and
  `+new-line` substrings.
- **Phase F**: `result.Atomic==false`; per-block outcome includes
  `ambiguous`; sha256(after)==sha256(before).
- **Phase G**: `result.Atomic==false`; atomic-error mentions
  `binary file`; sha256(after)==sha256(before).
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh (regex built from fragments).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A.** sha256(after) matching the canonical sha256 of the
  expected content is REAL on-disk evidence: a regression that
  succeeded without writing the disk would leave
  sha256(after)==sha256(before) and Phase A would fail. A
  regression that wrote partial or wrong content would change
  sha256(after) to a value different from the expected sha and
  Phase A would fail.
- **Phase B / Phase F / Phase G.** sha256(after)==sha256(before) is
  REAL on-disk evidence that the abort branch did NOT touch the
  file. A regression that committed partial state would change
  sha256(after).
- **Phase C.** Per-file sha256 matches the canonical hash of each
  file's expected post-content. A regression that wrote only one of
  the two files would fail the assertion for the other.
- **Phase D — load-bearing.** This is the whole-prompt-atomicity
  proof. The applier produces a valid in-memory replacement for
  block 1 (file 1 would have been writable in isolation), but block
  2 fails so the gate aborts the commit. Asserting that file 1's
  sha256(after)==sha256(before) — even though its block applied in
  memory — is the mechanical guarantee that the gate works. A
  regression that committed file 1 because "block 1 applied" would
  fail this phase immediately.
- **Phase E.** Diff exactness pins the T05 unified-diff wrapper to
  the actual textual change. A regression that produced an empty
  diff (e.g. a stale Differ that returned the old buffer twice)
  would fail because `-old-line` would be missing. A regression
  that produced a diff for unrelated content would fail because
  `+new-line` would be missing.
