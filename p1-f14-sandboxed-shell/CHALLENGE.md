# Challenge: P1-F14 — Sandboxed Shell Execution

## Purpose

Prove HelixCode's Phase 1 / Feature 14 Sandboxed Shell Execution actually
works end-to-end against the real Detector, real bubblewrap subprocesses,
and real on-disk YAML round-trips. Per Article XI §11.9, every PASS must
carry positive runtime evidence captured during execution.

The harness wires together the F14 surface area:

- `sandbox.NewDetector().Detect()` (T03) — real probes of `bwrap` on PATH,
  `/proc/sys/kernel/unprivileged_userns_clone`, and
  `/sys/fs/cgroup/cgroup.controllers`.
- `sandbox.SandboxManager` (T06) — single chokepoint that adjudicates
  CONST-033 deny BEFORE any backend dispatch and fails-closed when no
  backend was selected.
- `sandbox.BubblewrapBackend` (T04) — real `bwrap(1)` subprocess, deterministic
  argv, network share/unshare honoured.
- `sandbox.NativeBackend` (T05) — Linux user-namespace fallback that re-execs
  the host binary into the helper-mode dispatcher.
- `sandbox.WriteSandboxConfig` / `LoadSandboxConfig` (T08) — secret-safe
  YAML round-trip with `O_EXCL` + mode `0600` enforcement.

Phases 0, A, B, and E MUST always run and pass; Phases C and D run only
when the host exposes the matching primitive (bubblewrap on PATH /
unprivileged userns enabled). Skips are honest and counted as PASS, per
the F11/F12/F13 precedent.

## Procedure

1. Build the F14 challenge harness from
   `helix_code/tests/integration/cmd/p1f14_challenge`.
2. Run the harness — the first statement of `main()` is the native-helper
   dispatch (`if sandbox.IsHelperInvocation() { os.Exit(sandbox.RunAsHelper()) }`)
   so Phase D's re-exec into the new namespaces does not recurse into the
   harness itself.
3. The harness executes six phases:
   a. **Phase 0 — Detector (informational).** Run `sandbox.NewDetector().Detect()`
      and print the full `SandboxCapabilities` JSON plus the resolved
      backend kind.
   b. **Phase A — CONST-033 rejected before spawn.** Construct a manager
      with `caps.SelectedBackend = BackendBubblewrap` but NO backend
      slots wired (`bubblewrap=nil, native=nil`). Call `Execute` with
      four power-management variants (`systemctl suspend`, nested
      `bash -c 'systemctl poweroff'`, chained `ls; pm-suspend`, and
      `loginctl terminate-user $USER`). Each MUST surface `*DenyError`,
      NOT `*FailClosedError`. The DenyError must reference CONST-033.
      The load-bearing observable is the type discrimination: if the
      deny check ran AFTER the fail-closed check, we would see a
      FailClosedError because the backend slot is nil.
   c. **Phase B — Fail-closed when no backend.** Construct a manager
      with `caps.SelectedBackend = BackendNone` and a populated
      `UnavailableReason`. Call `Execute("echo hi", DefaultSandboxPolicy())`
      and assert the result is `*FailClosedError` whose `Reason` contains
      the verbatim text we set.
   d. **Phase C (gated) — Bubblewrap end-to-end.** Skipped when the
      detector did not pick bubblewrap. Otherwise: real `bwrap`
      subprocess runs `echo hello-from-sandbox-challenge` with
      `DefaultSandboxPolicy()`; runs `echo network-allowed-test` with
      `NetworkAllowed=true`; runs `curl -m 3 https://example.com … || echo NETDENIED`
      with default policy and asserts stdout contains `NETDENIED`
      (proving the default DENY is honoured by real netns isolation).
   e. **Phase D (gated) — Native (userns) end-to-end.** Skipped when
      `caps.UnprivilegedUserNS == false`. Otherwise: force-construct a
      `NativeBackend` and a manager wired only to it; run
      `echo hello-from-native-sandbox` and assert stdout matches. The
      backend re-execs THIS binary inside the new namespaces; the
      helper-mode dispatch at the top of `main()` short-circuits the
      child to `RunAsHelper`. Hosts where userns is restricted by
      AppArmor/seccomp despite the sysctl gate fall back to a noted
      skip.
   f. **Phase E — Sandbox config YAML round-trip on disk.** Tempdir,
      `WriteSandboxConfig` with a non-default policy + `UserDenyList`,
      `os.Stat` to verify mode `0600` and non-zero size,
      `LoadSandboxConfig` to read back, `reflect.DeepEqual` on the
      `UserDenyList` and field-by-field equality on the policy scalars.
4. Anti-bluff smoke clean over harness + challenge dir (the smoke regex
   is built from string fragments so the script does not match itself).
5. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F14 challenge harness PASS` final line.
- Phase 0: `SandboxCapabilities` JSON printed; `SelectedBackend` is one
  of `bubblewrap`, `native`, or `none`.
- Phase A: every deny variant produces `*DenyError` with `MatchedRule`
  containing `CONST-033`; none produce `*FailClosedError`. The
  type-discrimination evidence is what proves the deny check fires
  before the fail-closed branch.
- Phase B: `*FailClosedError` whose `Reason` carries the verbatim
  `UnavailableReason` we set on the manager's caps.
- Phase C: when bubblewrap is selected, three sub-checks pass — echo
  stdout round-trip, NetworkAllowed=true echo, default-policy curl
  probe outputs `NETDENIED`. Otherwise prints the gated-skip line.
- Phase D: when userns is enabled, the native-backend echo succeeds via
  the re-exec helper. Hosts that block userns at the LSM layer print a
  noted skip; hosts that lack userns entirely print the gated-skip line.
- Phase E: `WriteSandboxConfig` produces a file with mode exactly `0600`
  and non-zero size; `LoadSandboxConfig` returns equal scalar fields
  and a `reflect.DeepEqual`-equal `UserDenyList`.
- Anti-bluff smoke clean over harness file + this CHALLENGE.md + run.sh.
- Cross-compile linux/amd64 clean.
