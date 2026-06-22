# Challenge: P1-F08 — Plan Mode

## Purpose

Prove that the project's plan-mode gating actually blocks unauthorised destructive
tool calls, allows them once a matching plan action is approved, key-arg
mismatches still block, and ExitPlanMode restores normal execution. Per
Article XI §11.9, every PASS must carry positive runtime evidence.

## Procedure

1. Build the F08 challenge harness.
2. Run the harness — it:
   a. Transitions to ModePlan.
   b. Calls shell (echo hi) with NO approved plan — expects ErrPlanModeGated.
   c. Submits + approves a plan with action shell command=echo hi.
   d. Calls shell (echo hi) — expects success, captures "hi".
   e. Calls shell (echo bye, different command) — expects ErrPlanModeGated (key-arg mismatch).
   f. Calls ExitPlanMode, then shell (echo bye) — expects success.
3. Anti-bluff smoke clean.
4. Cross-compile linux clean.

## Pass criteria

- Harness exits 0 with `==> P1-F08 challenge harness PASS` final line.
- All five gating outcomes captured (blocked -> allowed -> blocked-on-mismatch -> exit -> allowed-after-exit).
- Anti-bluff smoke clean.
- Cross-compile linux clean.
