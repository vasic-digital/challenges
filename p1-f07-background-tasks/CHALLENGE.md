# Challenge: P1-F07 — Background Task System

## Purpose

Prove that HelixCode's background task system actually streams mid-execution
output and successfully cancels long-running shell processes. Per Article XI
§11.9, every PASS must carry positive runtime evidence.

## Procedure

1. Build the F07 challenge harness.
2. Run the harness — it:
   a. Starts a Bash command that emits 3 lines with 0.3s gaps.
   b. Polls TaskOutput every 200ms; logs each new state/line count to stdout.
   c. Asserts the polling timeline shows growing line counts (not just final).
   d. Starts `sleep 30`, cancels it after 200ms, asserts cancel within 3s.
   e. Logs `pgrep -x sleep` output as supporting evidence.
3. Anti-bluff smoke clean across F07 files.
4. Cross-compile linux clean.

## Pass criteria

- Harness exits 0 with `==> P1-F07 challenge harness PASS` as final line.
- Polling timeline shows >=2 distinct line counts during execution (proves streaming).
- Sleep task transitions to Cancelled or Failed within 3s of StopTask.
- Anti-bluff smoke clean.
- Cross-compile linux clean.
