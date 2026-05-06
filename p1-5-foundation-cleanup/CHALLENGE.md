# Challenge: P1.5 — Foundation Cleanup

## Purpose

Prove HelixCode's Phase 1.5 foundation-cleanup work packages (WP1–WP10)
actually landed in the working tree and stay landed. Each phase below is a
mechanical invariant on the real repo state; together they form the
"foundation didn't drift" gate that WP12's push order depends on.

Per Article XI 11.9, every PASS must carry positive runtime evidence
captured during execution. The harness emits **per-canonical-submodule
location reports** (Phase A), **bash-subshell-captured env-var values**
under three distinct loader branches (Phase B), the **list of canonical
docs/ directories actually found** (Phase C), the **scanned/allowlisted
counts plus zero-violation invariant** for snake_case (Phase D), and
**verbatim stdout** of the cascade verifier (Phase E).

The harness wires together Phase 1.5's surface area:

- WP3 deduplication — canonical first-party submodules at one path each.
- WP4 API-key loader — bash version `scripts/load_api_keys.sh`.
- WP6 docs consolidation — `docs/` is canonical; `Documentation/` is gone.
- WP7 snake_case — first-party directories conform or are allowlisted.
- WP8 anti-bluff cascade — Article XI 11.9 anchor in every Helix* repo's
  CONSTITUTION.md / CLAUDE.md / AGENTS.md.

Every always-runs phase is hermetic: Phase B's loader test uses a
synthesised `HOME` and a tempdir-only `pwd`, so the developer's real
`~/api_keys.sh` and the meta-repo `.env` are not consulted.

## Procedure

1. Build the P1.5 challenge harness from
   `HelixCode/tests/integration/cmd/p1_5_challenge`.
2. Run the harness; it executes five phases:

   a. **Phase A — NO-DUPLICATE-SUBMODULES (always runs).** Use
      `git ls-files .gitmodules` to enumerate the `.gitmodules` file(s)
      the META-REPO directly tracks (typically just `./.gitmodules`).
      Submodule-internal `.gitmodules` files are owned by their
      respective submodule and are out of scope here. Build the
      URL → declaring-paths map for the meta-repo's tracked entries.
      Assert every canonical Helix-owned URL maps to a single path.
      Independently, assert each of the canonical first-party
      submodules (`LLMsVerifier`, `Containers`, `Security`, `HelixQA`,
      `MCP-Servers`) appears at exactly its expected root location.
      Print `phaseA: <NAME> at <PATH> (1 location, no duplicates)` per
      canonical name on PASS.
   b. **Phase B — API-KEYS-LOADER (always runs).** Three independent
      bash subshells, each with a synthesised `HOME` and a tempdir
      `pwd`:
      - Branch 1: write `$HOME/api_keys.sh` with `export
        TEST_PHASE_B_KEY=value_from_sh`. Source the loader. Assert the
        env-var is propagated to a `bash -c` echo.
      - Branch 2: no `api_keys.sh`; create `.gitmodules` (empty) and
        `.env` (with `TEST_PHASE_B_KEY=value_from_env`) in the
        tempdir. Source the loader. Assert the env-var is propagated.
      - Branch 3: neither file. Source the loader. Assert it returns
        without setting the env-var and without aborting the
        subshell.
   c. **Phase C — DOCS-UNDER-DOCS-DIR (always runs).** Walk the
      meta-repo tree (third-party content roots and submodule subtrees
      skipped — submodules own their own doc-tree validation). Flag
      any directory whose basename equals `Documentation`
      (case-insensitive — including `documentation`, `DOCUMENTATION`)
      but is NOT exactly `docs`. Assert zero hits. Verify the
      canonical `docs/` exists at meta-repo root and at `HelixCode/`.
   d. **Phase D — SNAKE_CASE (always runs).** Walk meta-repo
      directly-tracked directories at depth ≤ 4 (submodule subtrees
      skipped — WP7 normalised the meta-repo's tracked dirs and the
      inner `HelixCode/` tree only). Skip dotted dirs. Apply the WP7
      deferred allowlist (Go-application packages like
      `applications/aurora-os`, `Specification/CLI_Specs_*`,
      `cmd/<binary>` Go convention, repo names from `.gitmodules`,
      and well-known project artefact dirs like `pkg/`, `internal/`,
      `tests/`, `docs/`). Assert every remaining basename matches
      `^[a-z0-9][a-z0-9_]*$`. Print total scanned, allowlisted, and
      0 violations.
   e. **Phase E — ANTI-BLUFF-ANCHOR (always runs).** Shell out to
      `scripts/verify_anti_bluff_cascade.sh`. Print verbatim stdout +
      stderr surrounded by sentinel lines so the evidence is captured
      in run.sh's terminal capture. PASS iff exit code 0.

3. Anti-bluff smoke clean over the harness file + this CHALLENGE.md
   + run.sh (the smoke regex is built from string fragments so the
   script does not match itself).
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1.5 challenge harness PASS` final line.
- **Phase A**: 5/5 canonical submodules at their expected root paths;
  no canonical URL is declared at more than one path across all
  scanned `.gitmodules`.
- **Phase B**: `branch1=PASS branch2=PASS branch3=PASS` — `value_from_sh`,
  `value_from_env`, and empty respectively.
- **Phase C**: zero `Documentation/` (any non-`docs` casing) directories
  in first-party tree; canonical `docs/` exists at root + `HelixCode/`.
- **Phase D**: nonzero scanned count, zero violations.
- **Phase E**: cascade verifier exit code 0; "OK: anti-bluff anchor
  present in all 39 files across 13 repos" line in captured output.
- Anti-bluff smoke clean over harness + this CHALLENGE.md + run.sh
  (regex built from fragments).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A — URL-uniqueness check is real.** A regression that re-added
  `HelixAgent/Containers`, `HelixAgent/HelixLLM/submodules/HelixQA`, or
  any other canonical-name copy under a first-party path would re-introduce
  a duplicate URL declaration; the harness fails on the duplicate-URL
  count immediately. The "occurrence-count == 1" check independently
  catches the same regression even if the URL string drifted (e.g. SSH
  vs HTTPS rewrite without a `.gitmodules` cleanup).
- **Phase B — bash subshell isolation.** The harness builds a clean
  `env` (PATH + HOME only) and runs `bash -c 'set +e; . loader.sh; echo
  P15_VAL=...'`. The captured `P15_VAL=` line is positive runtime
  evidence the loader injected the value into THAT subshell, not into
  the harness's parent process. A regression that reverted the loader
  to a no-op would emit `P15_VAL=` (empty) for branches 1 and 2 and the
  branch-specific assertion fails.
- **Phase C — case-insensitive Documentation guard.** A regression that
  re-created `documentation/` or `DOCUMENTATION/` (case-flip on a
  case-sensitive filesystem) is caught by the `EqualFold` test against
  any base except exactly `docs`.
- **Phase D — strict regex + allowlist.** The snake_case regex
  `^[a-z][a-z0-9_]*$` is mechanical; a regression that re-added a
  mixed-case directory at depth ≤ 4 outside the allowlist will trip
  immediately. The allowlist is anchored on `.gitmodules`-derived repo
  names and well-known Go-project conventions; expanding it requires a
  source-controlled edit to the harness.
- **Phase E — real script invocation.** The cascade verifier is
  shelled out via `bash scripts/verify_anti_bluff_cascade.sh`; its
  exit code IS the phase verdict. A regression that removed the
  Article XI 11.9 anchor from any cascaded file is caught by the
  verifier's `grep -q` over each `CONSTITUTION.md`, `CLAUDE.md`, and
  `AGENTS.md`.
