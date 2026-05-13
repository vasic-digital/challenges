# P2-F24 Challenge — Codex Project Memory

**Spec**: `docs/superpowers/specs/2026-05-07-p2-f24-codex-project-memory-design.md`
**Plan**: `docs/superpowers/plans/2026-05-07-p2-f24-codex-project-memory.md`
**Date**: 2026-05-07

## Goal

Prove the F24 codex-style project-memory subsystem works end-to-end against
real tempdirs and real fsnotify (no mocks). Five phases A-E exercise the five
anti-bluff vectors enumerated in spec §5.2:

1. **PHASE-A — PROJECT-ONLY**: tempdir with `helixcode.md` is loaded; sentinel
   `MEMORY_FIXTURE_24` (built from non-contiguous fragments) is present in
   `Memory.Project`; `Memory.User` is empty; `ProjectPath` resolves to the
   discovered file.
2. **PHASE-B — MISSING-FILE-GRACEFUL**: empty tempdir produces empty
   `Memory{}` with NIL error; `Render()` returns empty string. Missing files
   are NOT errors — anti-bluff Bluff #5.
3. **PHASE-C — HOT-RELOAD**: file rewrite triggers fsnotify-driven Reload
   within 1500 ms; registry Snapshot returns NEW content AND no longer
   contains the OLD sentinel (positive byte differential — anti-bluff Bluff
   #2).
4. **PHASE-D — PROJECT-PLUS-USER**: project memory + user overlay both loaded;
   `Render()` contains both sentinels with project-before-user ordering and
   the canonical delimiter (anti-bluff: render order is a security invariant).
5. **PHASE-E — TRUNCATION**: 100 KB file is truncated to exactly
   `MaxMemoryBytes` (64 KB); `TruncatedProject == true`; first 64 KB of the
   loaded content is byte-equal to the input (anti-bluff Bluff #4 — silent
   truncation forbidden).

## Run

```bash
./run.sh
```

Exits 0 on PASS; exits 1 on any byte-evidence mismatch. Absence-of-error is
NEVER acceptable as PASS — every phase asserts a positive runtime fact.

## Anti-bluff hot zone

The harness sentinels (`MEMORY_FIXTURE_24`, `MEM_INITIAL_24`, `MEM_UPDATED_24`,
`PROJ_BODY_24`, `USER_BODY_24`) are constructed from non-contiguous fragments
in main.go so the harness's own source code does NOT match the anti-bluff
smoke pattern.

The `run.sh` wrapper builds its bluff regex from concatenated string fragments
(P1/P2/P3/P4) so the script's own bytes never trigger the very scan it runs.
Verify by running `./run.sh` — it greps the F24 surface for the four bluff
markers and exits non-zero on any hit.
