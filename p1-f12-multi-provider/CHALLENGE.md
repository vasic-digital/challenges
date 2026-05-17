# Challenge: P1-F12 — Multi-Provider Backend

## Purpose

Prove HelixCode's Phase 1 / Feature 12 multi-provider backend actually works
end-to-end against real disk I/O and the four cloud backends (Anthropic,
Bedrock, Vertex AI, Azure). Per Article XI §11.9, every PASS must carry
positive runtime evidence.

The harness wires together the F12 surface area:

- `llm.Select` (T07) — flag > env > config selector with
  `ErrNoProviderConfigured` sentinel.
- `llm.NewCloudProvider` (T07) — cloud-quartet factory.
- `llm.RunWizard` non-interactive path (T08) — same code path the cobra
  `helixcode wizard --provider --api-key` subcommand drives.
- `llm.WriteWizardConfig` / `llm.LoadWizardConfig` (T08) — secret-safe
  on-disk YAML persistence (mode 0600 enforced, verified via `os.Stat`).

Local Phases A–D MUST always run and pass; Phase E (real cloud round-trip)
runs only when `ANTHROPIC_API_KEY` is present in the environment.

## Procedure

1. Build the F12 challenge harness from
   `helix_code/tests/integration/cmd/p1f12_challenge`.
2. Run the harness — it executes five phases:
   a. **Phase A — Selector precedence.** Asserts that
      (env=anthropic) -> Anthropic; (flag=bedrock + env=anthropic) ->
      Bedrock (flag wins); empty input -> `errors.Is(err,
      ErrNoProviderConfigured)`; (config=vertex-ai) -> VertexAI.
   b. **Phase B — Factory.** Calls `NewCloudProvider` for each of the
      four cloud `ProviderType` values with synthetic config; asserts
      each returned `Provider` implements the interface (`GetType`,
      `GetName`, `GetModels` non-panicking).
   c. **Phase C — Wizard round-trip on REAL disk.** Sets
      `XDG_CONFIG_HOME` to a tempdir; drives `RunWizard` with a
      `NonInteractiveResult` for Anthropic + an api key; calls
      `WriteWizardConfig`; verifies on-disk file mode is exactly 0600
      via `os.Stat`; reads back via `LoadWizardConfig` and asserts the
      provider type + api key round-trip byte-exact.
   d. **Phase D — End-to-end after disk read.** Loads the wizard
      config from disk, drives `Select(SelectorInput{Config: ...})`,
      drives `NewCloudProvider`, asserts the provider's reported type +
      name match.
   e. **Phase E (gated) — Real cloud round-trip.** If
      `ANTHROPIC_API_KEY` is set, constructs an Anthropic provider with
      the real key and calls `GetHealth(ctx)`. Otherwise prints
      `[skipped: ANTHROPIC_API_KEY not set]`.
3. Anti-bluff smoke clean over harness + challenge dir.
4. Cross-compile linux/amd64 clean.

## Pass criteria

- Harness exits 0 with `==> P1-F12 challenge harness PASS` final line
- Phase A: all four selector cases assert as expected
- Phase B: at least one cloud backend constructs (synthetic creds may be
  rejected by some SDKs at construction; that is acceptable as long as
  the constructor returns an error rather than panicking, AND at least
  one backend builds)
- Phase C: on-disk file present, `os.Stat` reports `Mode().Perm() ==
  0600`, `LoadWizardConfig` round-trips the api key
- Phase D: `Select` resolves to anthropic; `NewCloudProvider` returns a
  non-nil Anthropic Provider
- Phase E: either runs and reports a real `GetHealth` result, or prints
  the gated-skip line
- Anti-bluff smoke clean
- Cross-compile linux/amd64 clean
