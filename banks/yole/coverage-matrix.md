# Yole HelixQA Feature × Iteration Coverage Matrix

<!-- SPDX-FileCopyrightText: 2026 Milos Vasic -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

## Rules (CONST-039)

1. **Every prior-iteration scenario is RE-EXECUTED** on every new iteration's release candidate.
   The `make helixqa` target recursively scans `Challenges/banks/yole/` — so every scenario
   under `feature-coverage/` is automatically included without manual wiring.

2. **New iteration MUST add ≥ 1 scenario per new user-facing feature**, OR document
   why the feature is N/A (e.g. "iOS only, Xcode not configured yet — tracker #X").

3. **New iteration MUST NOT delete prior scenarios.** If a feature is removed, mark the
   scenario `status: retired` and add a `retired_reason` field; do not delete the YAML.

4. **HelixQA run gate**: all non-retired scenarios must PASS for a build to be ship-ready.
   SKIPPED status (emulator absent) counts as "deferred" — must be resolved before tagging.

5. **New scenarios live under `Challenges/banks/yole/feature-coverage/`** (Yole-specific).
   Cross-project scenarios belong in `HelixQA/banks/` (shared, Yole-agnostic).

---

## Feature Registry

| ID | Feature | First Iteration | Scenario File |
|----|---------|----------------|---------------|
| F1 | Syntax Highlighting (Tree-Sitter) | iter-57 | feature-1-syntax-highlighting.yaml |
| F2 | Source Code Support / Outline | iter-58 | feature-2-source-code-support.yaml |
| F3 | Auto-Complete (non-LSP) | iter-60 | feature-3-autocomplete.yaml |
| F4a | LSP Completion (rust-analyzer) | iter-61 | feature-4a-lsp-completion.yaml |
| F4b | LSP Diagnostics + Hover + Go-to-Def | iter-62 | feature-4b-diagnostics-hover-gotodef.yaml |
| F4c | LSP Refactoring / Rename | iter-63 | feature-4c-refactoring.yaml |
| F5 | Import From (.docx → Markdown) | iter-64 | feature-5-import.yaml |

---

## Iteration × Feature Matrix

> Legend: `PASS` = scenario passed in that iteration's HelixQA run.
> `SKIP(reason)` = prerequisite absent (emulator, Xcode, fixture).
> `DEFERRED` = not yet run; must be resolved before shipping that iteration.
> `N/A` = feature did not exist yet in that iteration.
> `RETIRED` = feature removed; scenario kept but marked retired.

| Iteration | F1 | F2 | F3 | F4a | F4b | F4c | F5 | Notes |
|-----------|----|----|----|----|-----|-----|-----|-------|
| iter-57 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | Syntax-highlighting feature landed; no HelixQA scenarios yet |
| iter-58 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | Source-code-support landed; no HelixQA scenarios yet |
| iter-60 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | Auto-complete landed; no HelixQA scenarios yet |
| iter-61 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | LSP completion landed; no HelixQA scenarios yet |
| iter-62 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | LSP diag/hover/gotodef landed; no HelixQA scenarios yet |
| iter-63 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | LSP refactoring landed; no HelixQA scenarios yet |
| iter-64 | N/A | N/A | N/A | N/A | N/A | N/A | N/A | Import-From landed; no HelixQA scenarios yet |
| iter-76 | DEFERRED | DEFERRED | DEFERRED | DEFERRED | DEFERRED | DEFERRED | DEFERRED(fixture) | **Scenarios authored; emulator required to run** |

> **iter-76 deferred reason**: Android emulator-5554 not present on this macOS host during
> authoring. Scenarios are committed and ready. Run `make helixqa` when emulator is up.
> `feature-5-import` additionally requires `test-import.docx` fixture (tracker
> #iter-76-import-fixture-docx-committed — generate via python-docx before running).

---

## iOS Tracker

**Tracker: #iter-76-ios-scenarios-pending-xcode**

All 7 scenarios are written as platform-agnostic with `platforms: [android, desktop, web]`.
iOS is listed as a comment deferral. When Xcode automation is configured (operator announced
"later"), add `ios` to the `platforms` list in each YAML — no structural changes needed.

---

## Adding a New Feature (per CONST-039)

1. Add a row to the Feature Registry table above.
2. Author a scenario YAML at `Challenges/banks/yole/feature-coverage/feature-N-<name>.yaml`.
3. Add `DEFERRED` cells to the Iteration × Feature Matrix for the current iteration.
4. Update cells to `PASS` / `SKIP(reason)` after the HelixQA run.
5. Confirm `helixqa_scenario_coverage_challenge.sh` count still passes (auto-checks ≥ 7 YAMLs;
   update the threshold in the script if the count grows).
