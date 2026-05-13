# Challenge: P2-F21 — Codex Approval Modes

## Purpose

Prove HelixCode's Phase 2 / Feature 21 codex-approval-modes gate actually
gates real tool execution end-to-end against the real `tools.ToolRegistry`,
the real `approval.ApprovalManager`, the real `CheckApproval` 4x4 matrix,
the real `PromptForApproval` delegation seam, the real `SetMode` atomic
swap, and the real F02-equivalent inner final-deny contract.

Per Article XI 11.9, every PASS must carry positive runtime evidence
captured during execution. The harness emits **wrapped-error checks**
(`errors.Is(err, approval.ErrApprovalDenied)`) for the deny phases,
**Tool.Execute counter assertions** for the side-effect-must-not-happen
invariant, **recorded prompt question text** for the prompter delegation
phase, **byte-equal sentinel injection** for the sandbox-marker phase
(`_helix_sandbox_required=true`, `_helix_sandbox_network_allowed=false`,
spec 7128289 §11), and **mid-session source/mode transition assertions**
(`SourceDefault` -> `SourceRuntime`) for the runtime-change phase.

The harness wires together the F21 surface area:

- `approval.ApprovalMode` / `approval.ApprovalLevel` / `approval.Decision`
  / `approval.Action` / sentinels (T02) — validated input/output types.
- `approval.NewApprovalManager` / `CheckApproval` / `PromptForApproval` /
  `SetMode` / `SandboxRequired` / `NetworkAllowed` (T04) — the gate.
- `tools.ToolRegistry.SetApprovalManager` / `applyApprovalGate` (T05+T07)
  — the registry hook that enforces the gate before each Tool.Execute.

Every always-runs phase is hermetic: no real OS env, no real filesystem,
no real shell, no real LLM provider — the only "real" components are the
F21 production code paths under test plus a `recordingResponder` that
satisfies `approval.PromptResponder` deterministically.

## Procedure

1. Build the F21 challenge harness from
   `HelixCode/tests/integration/cmd/p2f21_challenge`.
2. Run the harness; it executes five phases:

   a. **Phase A — SUGGEST-DENY (always runs).** Construct
      `tools.NewToolRegistry(DefaultRegistryConfig())` and
      `approval.NewApprovalManager{InitialMode=ModeSuggest,
      SandboxAvailable=true}`. Register an in-process stub Tool whose
      `RequiresApproval()==LevelEdit`. Wire the manager onto the
      registry via `SetApprovalManager`. Invoke `registry.Execute` for
      the stub. Assert `errors.Is(err, approval.ErrApprovalDenied)`,
      stub's executed counter == 0, and prompter calls == 0.
   b. **Phase B — AUTO-EDIT-PROMPT (always runs).** Same shape but
      `InitialMode=ModeAutoEdit` and a stub with `LevelRun`. Run twice
      with two responders: `Allow=true` -> Execute returns `"ok"` with
      executed counter == 1 and prompter calls == 1; `Allow=false` ->
      `errors.Is ErrApprovalDenied` with executed counter == 0 and
      prompter calls == 1. The prompter records the question text and
      the harness asserts it contains the tool name.
   c. **Phase C — FULL-AUTO-SANDBOX (always runs, load-bearing).**
      `InitialMode=ModeFullAuto`. The stub records the args map it
      received. Assert `args["_helix_sandbox_required"] == true` AND
      `args["_helix_sandbox_network_allowed"] == false` (per spec
      7128289 §11) AND prompter calls == 0 (full-auto never prompts).
   d. **Phase D — RUNTIME-CHANGE (always runs).** Construct in
      `ModeSuggest`. First Execute against a `LevelRun` stub:
      `errors.Is ErrApprovalDenied`, executed == 0,
      `mgr.Source() == SourceDefault`. Then `mgr.SetMode(ModeFullAuto)`.
      Assert `mgr.Mode() == ModeFullAuto` and
      `mgr.Source() == SourceRuntime`. Re-execute the same call:
      now `nil` error, executed == 1, sandbox markers injected.
   e. **Phase E — F02-FINAL-DENY (always runs).** Construct in
      `ModeDangerous`. Register a `LevelRun` stub whose Execute
      enforces an inner deny-rule mimicking F02's path-aware final
      deny: refuse `args["path"]` starting with `/etc/`. Sanity:
      `path="/tmp/ok"` -> ALLOW (proves the rule is path-aware, not
      blanket). Forbidden: `path="/etc/foo"` -> non-nil error
      containing "final-deny" AND the executed counter does NOT
      increment past the benign baseline. Assert prompter calls == 0
      (dangerously-bypass never prompts the F21 gate).
3. Anti-bluff smoke clean over the harness file + this CHALLENGE.md
   + run.sh (the smoke regex is built from string fragments so the
   script does not match itself).
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P2-F21 challenge harness PASS` final line.
- **Phase A**: deny error wraps `approval.ErrApprovalDenied`; inner
  Tool.Execute counter == 0; prompter calls == 0.
- **Phase B**: YES sub-case ALLOWs (`"ok"`, executed==1, calls==1,
  question contains tool name); NO sub-case DENYs (errors.Is
  `ErrApprovalDenied`, executed==0, calls==1).
- **Phase C**: `_helix_sandbox_required==true` and
  `_helix_sandbox_network_allowed==false` injected into the args map
  the inner Tool.Execute received; prompter calls == 0.
- **Phase D**: pre-swap DENY (errors.Is `ErrApprovalDenied`,
  executed==0, `Source==SourceDefault`); post-swap ALLOW (executed==1,
  `_helix_sandbox_required==true`, `Mode==ModeFullAuto`,
  `Source==SourceRuntime`).
- **Phase E**: benign `/tmp/ok` ALLOW (executed==1); forbidden
  `/etc/foo` final-deny (non-nil error containing "final-deny",
  executed counter unchanged from benign baseline); prompter
  calls == 0.
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh (regex built from fragments).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A — wrapped sentinel + counter.** A regression that "denied"
  with a generic error (e.g., `errors.New("nope")`) would still fail
  the test because it would not satisfy
  `errors.Is(err, approval.ErrApprovalDenied)`. Likewise, a regression
  that "denied" cosmetically but still invoked the inner Execute (e.g.,
  by logging-and-continuing) would fail the executed counter check.
  Two independent anti-bluff invariants must hold simultaneously.
- **Phase B — recorded question + polarity flip.** The
  `recordingResponder.LastQuestion` capture is positive runtime
  evidence that the prompter was actually consulted. A regression that
  short-circuited the prompter and returned ALLOW directly would leave
  `LastQuestion == ""`. Running the same path with both `Allow=true`
  and `Allow=false` and observing the counter flip catches a
  regression that ignored the responder's answer (always-yes or
  always-no bug).
- **Phase C — byte-equal sentinel keys.** The sentinel keys
  `_helix_sandbox_required` and `_helix_sandbox_network_allowed` are
  pinned in spec 7128289 §11. A regression that spelled them
  differently (e.g., `_sandbox_required`) or emitted them as strings
  (`"true"` vs `true`) would fail the type-asserted equality check.
- **Phase D — atomic transition + Source==SourceRuntime.** A
  regression that "set" mode but did not actually swap the atomic
  pointer (e.g., a value-receiver bug) would leave the second Execute
  in `ModeSuggest` and fail the post-swap ALLOW assertion. The
  `Source` transition catches a regression that swapped mode but did
  not update the resolved-source bookkeeping (a bug that would silently
  make `/approval status` lie to the user).
- **Phase E — F02 contract proxy.** F02 is currently not directly
  wired into the registry; the contract that "approval modes never
  override inner content-aware permission rules" is therefore pinned
  via a test-fixture deny-rule embedded in the Tool's own `Execute`.
  This is exactly the seam any future F02 integration MUST preserve:
  a deny rule layered after `applyApprovalGate` MUST still be reached
  even under `ModeDangerous`. The benign sanity-call (`/tmp/ok`
  ALLOW) before the forbidden case (`/etc/foo` DENY) catches a
  regression that turned the rule into a blanket reject -- the
  benign path would fail and surface that the rule is content-blind.

## §11 — F02 wiring note (for the next agent)

Today the F21 `applyApprovalGate` is the only registry-level policy
gate. F02 (permission rules) lives in `internal/tools/permissions/`
and is consulted per-tool by tools that opt in (e.g.,
`shell_sandboxed`); it is NOT a registry-level pre-execute hook. Phase
E's stub-tool inner deny-rule mirrors F02's contract and is the
explicit pin: the harness asserts that inner final-deny refuses the
call EVEN when the F21 gate (`ModeDangerous`) returns ALLOW. When F02
gains a registry-level seam, the same invariant — inner deny over
gate-allow — MUST still hold; the seam must be wired AFTER
`applyApprovalGate`, not before, so the pre-execute order is
plan-mode -> validate -> approval-gate -> F02 -> hooks -> Execute.
