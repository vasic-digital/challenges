# Challenge: P1-F13 — LSP Integration

## Purpose

Prove HelixCode's Phase 1 / Feature 13 LSP Integration actually works
end-to-end against a real LSP subprocess and the real ToolRegistry. Per
Article XI §11.9, every PASS must carry positive runtime evidence.

The harness wires together the F13 surface area:

- `tools.LSPManager` (T05) — lazy-spawn pool driving LSP servers via
  real subprocess + real LSP-framed JSON-RPC over stdio.
- `internal/tools/lsp_fakeserver` (T05) — real Go binary speaking real
  LSP frames, used in place of language-specific real servers so the
  harness runs on hosts with no toolchain installed.
- `tools.ToolRegistry.SetLSPManager` + post-Execute auto-trigger (T08)
  — fires `NotifyOpen` + `NotifyChange` against the manager after a
  successful Edit-class tool (`fs_write` / `fs_edit` /
  `multiedit_commit`) so subsequent `GetDiagnostics` calls see fresh
  diagnostics.

Local Phases 0 and A–E MUST always run and pass; Phase F (real gopls
round-trip) runs only when `gopls` is on `PATH`.

## Procedure

1. Build the F13 challenge harness from
   `helix_code/tests/integration/cmd/p1f13_challenge`.
2. Run the harness — it executes seven phases:
   a. **Phase 0 — Setup.** `go build` the in-tree fake LSP server into
      a tempdir; print path + on-disk size; construct an `LSPManager`
      with a single `.fake`-routed spec.
   b. **Phase A — Lazy spawn + diagnostics round-trip.** Write a
      `.fake` file containing `// @fake-error: phase-A-bad`; call
      `manager.NotifyOpen`; poll `GetDiagnostics` (50ms) up to 2s for
      ≥1 diagnostic; assert exactly one with severity error and message
      containing `phase-A-bad`; assert one managed server with PID > 0.
   c. **Phase B — DidChange round-trip.** Rewrite the file to
      `// @fake-error: phase-B-different`; call `NotifyChange`; assert
      a diagnostic with the new message appears.
   d. **Phase C — Restart cycles the process.** Capture the running
      PID; call `manager.Restart("fake")`; rewrite + `NotifyOpen`; wait
      for diagnostics; assert the new server PID is different from
      the old one.
   e. **Phase D — Stop tears down the server.** Call
      `manager.Stop("fake")`; assert the manager either drops the
      entry from `Servers()` entirely or reports it with status
      `stopped`.
   f. **Phase E — Auto-trigger via real ToolRegistry.** Construct a
      fresh `ToolRegistry` via `NewToolRegistry(DefaultRegistryConfig)`
      with `WorkspaceRoot` = tempdir; construct a fresh `LSPManager`
      bound to the same fake-server binary; call `r.SetLSPManager(m)`;
      call `r.Execute(ctx, "fs_write", {path, content})` with content
      `// @fake-error: phase-E-via-registry`; assert the file lands on
      disk and the post-Execute auto-trigger publishes a diagnostic
      mentioning `phase-E-via-registry`.
   g. **Phase F (gated) — Real gopls round-trip.** If
      `exec.LookPath("gopls")` succeeds: write a syntactically broken
      `.go` file in a tempdir with `go.mod`, `NotifyOpen`, wait up to
      20s for ≥1 diagnostic, assert at least one was published.
      Otherwise prints `[skipped: gopls not on PATH]` and continues.
3. Anti-bluff smoke clean over harness + challenge dir.
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F13 challenge harness PASS` final line
- Phase 0: real `go build` produces a non-zero-size binary
- Phase A: exactly 1 error diagnostic with message containing
  `phase-A-bad`; one `Servers()` entry with PID > 0
- Phase B: a diagnostic with message containing `phase-B-different`
  appears within 2s of `NotifyChange`
- Phase C: the post-restart PID is non-zero and different from the
  pre-restart PID — proof Restart cycles the OS process, not just an
  in-memory marker
- Phase D: `Servers()` is empty or reports status `stopped`
- Phase E: `fs_write` writes the file AND the auto-trigger publishes a
  diagnostic mentioning `phase-E-via-registry`; the auto-trigger
  spawned a managed server with PID > 0
- Phase F: either runs and reports a real gopls diagnostic, or prints
  the gated-skip line
- Anti-bluff smoke clean
- Cross-compile linux/amd64 clean
