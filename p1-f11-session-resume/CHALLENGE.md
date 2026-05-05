# Challenge: P1-F11 — Session Transcript Resume

## Purpose

Prove HelixCode's session transcript resume actually persists conversation
messages across a real process boundary, restores them byte-exact in a brand
new process, and locates the most-recent session globally across multiple
project paths. Per Article XI §11.9, every PASS must carry positive runtime
evidence.

The harness uses a **real fork-exec subprocess** (not just a fresh struct in
the same Go process) so the read-side process has a different PID and zero
shared in-memory state with the writer. The transcript MUST come from disk.

## Procedure

1. Build the F11 challenge harness.
2. Run the harness — orchestrator forks itself twice:
   a. **Phase A** (child PID #1, `phase=write`): construct a real
      `TranscriptStore` + `SessionManager` rooted at a tempdir, seed
      ProjectPath/Name, append 3 messages, assert
      `meta.MessageCount == 3`.
   b. **Orchestrator** stats `transcript.jsonl` and `metadata.json` on disk
      and asserts both are non-empty BEFORE the read child runs (proves the
      bytes are really on disk, not just in writer-process memory).
   c. **Phase B** (child PID #2, `phase=read`): brand-new process, no
      in-memory state from phase A. Construct a fresh `SessionManager`
      bound to the SAME `baseDir`, call `Resume(sessionID)`, assert all 3
      messages round-trip byte-exact (role + content for each).
   d. **Phase C** (in-orchestrator): write a SECOND session with a different
      `ProjectPath` (`/tmp/projB-f11`) and a more recent `LastActivity`,
      call `ResumeFinder.FindResumeTarget(ctx, ResumeGlobal, "")`, assert
      it returns the more recent session regardless of project. Then call
      `FindResumeTarget(ctx, ResumeProject, "/tmp/projA-f11")` and assert
      it filters back to the original session.
3. Anti-bluff smoke clean
4. Cross-compile linux clean

## Pass criteria

- Harness exits 0 with `==> P1-F11 challenge harness PASS` final line
- Two real subprocesses run with distinct PIDs (orchestrator prints both)
- On-disk `transcript.jsonl` and `metadata.json` are stat-verified
  non-empty between phase A and phase B
- All 3 messages round-trip byte-exact across the process boundary
- `ResumeGlobal` returns the most-recent session across project paths
- `ResumeProject` filters by ProjectPath
- Anti-bluff smoke clean
- Cross-compile linux clean
