# P2-F22 — Aider Git Auto-Commit Per Change

**Phase:** Phase 2 — CLI Agent Porting
**Spec:** `docs/superpowers/specs/2026-05-06-p2-f22-aider-git-auto-commit-design.md` (commit `8be7fba`)
**Plan:** `docs/superpowers/plans/2026-05-06-p2-f22-aider-git-auto-commit.md`

## Summary

Real, end-to-end per-edit git auto-commit modelled on aider. One commit per
accepted edit (LevelEdit / LevelAll tools). LLM-summarised commit subject
(50–72 chars, imperative voice) with deterministic fallback on LLM
unavailability. `Co-Authored-By: HelixCode <noreply@helixcode.dev>` trailer on
every auto-commit. Default-on; opt-out via `HELIXCODE_GIT_AUTO_COMMIT=off` env,
runtime `/git_auto_commit off` slash, or per-edit `_helix_skip_git_commit:true`
parameter.

## Phases (all run unconditionally)

1. **PHASE-A — DEFAULT-ON-COMMITS-EDIT** (7 checks)
2. **PHASE-B — LLM-SUMMARY-ACCURATE** (3 checks; pins LLM-call-then-use)
3. **PHASE-C — NON-EDIT-NO-OP** (2 checks; level filter)
4. **PHASE-D — ENV-OFF-NO-COMMIT** (3 checks; opt-out)
5. **PHASE-E — RUNTIME-TOGGLE** (4 checks; SetEnabled visible on next call)
6. **PHASE-F — PER-EDIT-SKIP** (3 checks; SkipParamKey honoured)
7. **PHASE-G — SECRET-FILTER** (3 checks; CONST-042 belt-and-suspenders)

## Anti-bluff invariants

- HEAD SHA differential observable per phase (positive evidence).
- `git status --porcelain` empty-vs-dirty pinned per phase.
- `Co-Authored-By: HelixCode <noreply@helixcode.dev>` trailer present
  byte-for-byte (PHASE-A, PHASE-B).
- LLM sentinel string equals subject in PHASE-B → proves the LLM response
  reaches the commit message (not a hardcoded fallback).
- Subject length ≤ 72 chars (git convention).
- `[REDACTED]` token replaces fake AKIA key in PHASE-G; original key absent.

## Run

```bash
./run.sh
```

Exit 0 = PASS. Exit 1 with diagnostic = FAIL.
