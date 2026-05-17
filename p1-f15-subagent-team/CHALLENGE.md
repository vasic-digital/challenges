# Challenge: P1-F15 — Subagent Team

## Purpose

Prove HelixCode's Phase 1 / Feature 15 Subagent Team actually works
end-to-end against the real `SubagentManager`, real `InProcessSpawner`
with a real `FakeLLMProvider`, real fork-exec of the harness binary as a
subprocess subagent, real F04 git worktree creation (when git is on
PATH), and a real cloud LLM round-trip (when `ANTHROPIC_API_KEY` is
set). Per Article XI §11.9, every PASS must carry positive runtime
evidence captured during execution.

The harness wires together the F15 surface area:

- `subagent.SubagentManager` (T05) — single chokepoint that picks a
  spawner per `task.Isolation`, enforces `MaxConcurrency`, fans
  results through one aggregator channel, and supports `Kill(id)`.
- `subagent.InProcessSpawner` (T03) — invokes the supplied
  `llm.Provider` directly in a goroutine.
- `subagent.SubprocessSpawner` (T04) — re-execs the host binary with
  `HELIXCODE_SUBAGENT_HELPER=1` and decodes the child's stdout as a
  `SubagentResult` JSON.
- `subagent.IsSubagentInvocation` / `RunAsSubagent` (T08) — child-side
  helper-mode dispatch. The harness binary IS the host binary in this
  challenge, so the parent's re-exec lands in our own `main()`.
- `subagent.WorktreeIntegration` (T06) — adapter onto F04's
  `worktree.Manager.CreateWorktreeForSubagent` that creates an
  isolated worktree without mutating the parent's `currentWorktree`.
- `subagent.FakeLLMProvider` (T02 — TEST-ONLY) — the seam that lets
  the harness positively distinguish "real provider was called" from
  "an upstream layer fabricated a result".

Phases A, B, E, and F MUST always run and pass. Phases C (worktree)
and D (real LLM) run only when their precondition is met (git on PATH
/ `ANTHROPIC_API_KEY` set). Skips are honest and counted as PASS, per
the F11/F12/F13/F14 precedent.

## Procedure

1. Build the F15 challenge harness from
   `helix_code/tests/integration/cmd/p1f15_challenge`.
2. Run the harness — the FIRST statement of `main()` is the subagent
   helper-mode dispatch (`if subagent.IsSubagentInvocation() { os.Exit(subagent.RunAsSubagent(harnessLLMFactory)) }`).
   Without this short-circuit, Phase B's re-exec'd child would
   re-enter the harness and recurse forever (its own Phase B would
   fork another child, etc.).
3. The harness executes six phases:
   a. **Phase A — In-process + real FakeLLMProvider.** Construct a
      manager wired to a real `InProcessSpawner` and a real
      `FakeLLMProvider` seeded with `phase-a-prompt -> phase-a-output`.
      Dispatch a single task with `Isolation=IsolationNone`. Drain
      the aggregator. Assert `State=StateSucceeded`,
      `Output=="phase-a-output"`, AND
      `FakeLLMProvider.GenerateCallCount()==1`. The call-count assertion
      is the load-bearing anti-bluff anchor: it proves the spawner
      actually invoked the provider rather than the manager
      fabricating a result.
   b. **Phase B — Subprocess re-execs THIS binary.** Construct a
      manager whose `SubprocessSpawner` points at `os.Executable()`.
      Dispatch a task with `Isolation=IsolationWorktree` and
      `Prompt="phase-b-prompt"`. The child sees
      `HELIXCODE_SUBAGENT_HELPER=1`, runs `RunAsSubagent` which
      constructs ITS OWN `FakeLLMProvider` via `harnessLLMFactory`
      (no canned response for "phase-b-prompt"), so the fallback
      echo `"FAKE-LLM-ECHO: phase-b-prompt"` is what the child emits
      on stdout. Assert `State=StateSucceeded`,
      `Output` starts with `"FAKE-LLM-ECHO: "` and contains the
      original prompt. ALSO assert the parent's `FakeLLMProvider`
      was NOT invoked (`GenerateCallCount==0`) — the subprocess
      spawner ignores `llmProvider` by contract.
   c. **Phase C (gated) — Real F04 worktree creation.** Skipped when
      `git` is not on PATH. Otherwise: `git init -b main` in a tempdir,
      seed README, construct a real `worktree.Manager` and
      `subagent.WorktreeIntegration`, call `Setup(task)` to create the
      worktree, stat the returned path, stage a real file, capture the
      diff via `WorktreeIntegration.CaptureDiff`, assert the diff
      contains the staged content. Assert the parent
      `worktree.Manager` was NOT mutated (`IsIsolated()==false`,
      `GetCurrentDirectory()==repoRoot`). Cleanup the worktree.
   d. **Phase D (gated) — Real Anthropic LLM round-trip.** Skipped
      when `ANTHROPIC_API_KEY` is not set. Otherwise: construct a real
      `AnthropicProvider`, dispatch a tiny task ("respond with the
      literal string 'hello-from-real-llm'"), drain. Assert
      `State=StateSucceeded` and non-empty `Output`. Real cloud
      round-trip; runtime evidence (model, duration) printed.
   e. **Phase E — MaxConcurrency cap.** Construct a manager with
      `MaxConcurrency=2` and a `FakeLLMProvider` configured with a
      500ms `WithDelay`. Dispatch three slow tasks back-to-back.
      Assert the third `Dispatch` returns
      `errors.Is(err, ErrMaxConcurrency)`. Drain the two accepted
      tasks to release the slots cleanly.
   f. **Phase F — Kill cancels a running subagent.** Dispatch a slow
      task, give the goroutine a brief moment to enter Generate's
      blocking section, call `manager.Kill(id)`, drain the result.
      Assert `State=StateCanceled`. Proves the manager's per-task
      `context.CancelFunc` actually propagates through the spawner
      to the in-flight provider call.
4. Anti-bluff smoke clean over harness + challenge dir (the smoke
   regex is built from string fragments so the script does not match
   itself).
5. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F15 challenge harness PASS` final line.
- Phase A: result `State=StateSucceeded`, `Output=="phase-a-output"`,
  `FakeLLMProvider.GenerateCallCount()==1`, `LastPrompt()=="phase-a-prompt"`.
- Phase B: subprocess child exits 0 with valid `SubagentResult` JSON
  on stdout; parent decodes `State=StateSucceeded` and `Output` starts
  with `"FAKE-LLM-ECHO: "` and contains `"phase-b-prompt"`. Parent's
  `FakeLLMProvider.GenerateCallCount()==0` (proves the parent provider
  is bypassed when `Isolation=IsolationWorktree`).
- Phase C: when git is on PATH, real worktree dir exists; stat is a
  directory; the staged diff contains the new file's content; the
  parent `worktree.Manager` is `IsIsolated()==false` and
  `GetCurrentDirectory()==repoRoot`. When git is missing, prints the
  gated-skip line.
- Phase D: when `ANTHROPIC_API_KEY` is set, real Anthropic call
  succeeds with non-empty output. Otherwise prints the gated-skip
  line.
- Phase E: third `Dispatch` returns `ErrMaxConcurrency`; the two
  accepted tasks both end in `StateSucceeded`.
- Phase F: result is `State=StateCanceled` (proves `Kill(id)`
  propagates `context.Canceled` through the manager's per-task
  cancel func into the spawner's Generate call).
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh.
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- The Phase A `GenerateCallCount==1` assertion is the load-bearing
  anchor: a manager that fabricated results would still pass the
  `State`/`Output` checks but the provider's call count would be 0.
- The Phase B parent-side `GenerateCallCount==0` assertion proves the
  subprocess spawner does NOT secretly funnel through the parent's
  provider as a fallback.
- The Phase C parent-state assertions (`IsIsolated()==false`,
  `GetCurrentDirectory()==repoRoot`) prove subagent dispatch does
  NOT silently relocate the parent agent.
- The Phase F `StateCanceled` discrimination proves `Kill(id)` actually
  cancels — a manager that ignored Kill would surface `StateSucceeded`
  after the FakeLLMProvider's 5-second delay completed.
