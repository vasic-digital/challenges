# Challenge: P1-F16 — OpenTelemetry Integration

## Purpose

Prove HelixCode's Phase 1 / Feature 16 OpenTelemetry integration actually
works end-to-end against the real OTel SDK exporters: a real stdout
exporter (Phase A + Phase C), a real OTLP/HTTP exporter pointed at an
in-process `httptest.Server` receiver (Phase B), the real noop fast path
(Phase D), and a real collector when the operator points
`OTEL_EXPORTER_OTLP_ENDPOINT` at one (Phase E). Per Article XI §11.9,
every PASS must carry positive runtime evidence captured during
execution.

The harness wires together the F16 surface area:

- `telemetry.LoadConfigFromEnv` (T04) — env-var precedence chain
  (`HELIXCODE_OTEL_EXPORTER` > `OTEL_TRACES_EXPORTER` >
  `OTEL_EXPORTER_OTLP_PROTOCOL`).
- `telemetry.NewTelemetryProvider` (T05) — wires the OTel SDK
  `TracerProvider` + `MeterProvider` against the configured exporter
  (`stdout`, `grpc`, `http/protobuf`, or `noop`).
- `telemetry.TracedLLMProvider` (T06) — decorator that adds a
  `llm.Generate` span + four metrics (`helixcode_llm_calls_total`,
  `helixcode_llm_latency_seconds`,
  `helixcode_llm_prompt_tokens_total`,
  `helixcode_llm_completion_tokens_total`) per Generate call. Every
  attribute is filtered through `FilterAttributes` against the
  effective deny-list (CONST-042).
- `telemetry.SetStdoutWriterForTest` (T07 build-tag seam) — exposes
  the package-private `stdoutWriter` to the harness so Phases A, C,
  and D can capture exporter output into a `bytes.Buffer` instead of
  redirecting `os.Stdout` itself. Gated by `-tags=testing_export`.
- `subagent.FakeLLMProvider` (T02 — TEST-ONLY) — a real `llm.Provider`
  with a canned-response map and call-count counter. The harness uses
  it as the inner provider so Phase A's "real Generate happened" claim
  can be verified by `GenerateCallCount==1`.

Phases A, B, C, and D MUST always run and pass. Phase E (real
collector) runs only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set AND a
TCP dial against the parsed host:port succeeds. Skips are honest and
counted as PASS, per the F11/F12/F13/F14/F15 precedent.

## Procedure

1. Build the F16 challenge harness from
   `helix_code/tests/integration/cmd/p1f16_challenge` with the
   `testing_export` build tag (so the harness can call
   `telemetry.SetStdoutWriterForTest`).
2. Run the harness; it executes five phases:
   a. **Phase A — STDOUT exporter end-to-end (always runs).**
      `LoadConfigFromEnv(map[HELIXCODE_OTEL_EXPORTER=stdout, ...])`,
      `NewTelemetryProvider(cfg)`, swap the package-private
      `stdoutWriter` for a captured buffer, wrap a
      `subagent.FakeLLMProvider` (canned `phase-A-prompt ->
      phase-A-output`) in `TracedLLMProvider`, call `Generate`,
      `ForceFlush`. Assert the captured buffer contains the
      `llm.Generate` span name, the `llm.model` attribute key, the
      configured model value, and the `helixcode_llm_calls_total`
      metric. Print a one-line evidence snippet of the captured span.
   b. **Phase B — Real OTLP/HTTP into fake in-process receiver
      (always runs).** Start a tiny `httptest.Server` whose handler
      records every POST it receives. Build a real `OTLPHTTP`
      `TelemetryProvider` against `server.URL`, wrap a fake LLM, run
      Generate, `ForceFlush`, `Shutdown`. Assert the receiver got at
      least one POST to `/v1/traces` with a non-empty body (REAL HTTP
      round-trip evidence — the OTel SDK actually serialised
      protobuf bytes and dispatched them over a TCP socket). Also
      report metric POSTs when present.
   c. **Phase C — Secret-attribute filter (always runs).** Stdout
      exporter again with a captured buffer. Generate with a prompt
      whose body contains the unique marker
      `API_KEY=sk-CHALLENGE-12345`. Assert the captured stdout
      contains the `llm.Generate` span name (so we know export
      actually happened) AND does NOT contain the marker. The marker
      absence is the load-bearing CONST-042 anti-leak proof.
   d. **Phase D — Noop zero-cost (always runs).**
      `LoadConfigFromEnv(empty)` resolves to `ExporterNoop`. Construct
      provider, swap stdoutWriter for a fresh buffer, wrap a fake
      LLM, call Generate 100 times. Assert
      `provider.Exporter()==ExporterNoop`, the inner provider's call
      count is 100 (real Generate ran), and the captured buffer is
      empty (no telemetry leaked). Print elapsed wall time.
   e. **Phase E — Real OTLP/HTTP collector (gated).** Skipped when
      `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, unparseable, or
      unreachable (TCP-dial probe with 2 s timeout). Otherwise:
      construct an `OTLPHTTP` provider, emit a span,
      `ForceFlush`, print `real collector phase: span dispatched to
      <endpoint>`. Honest skip is a PASS per the F11/F12/F13/F14/F15
      precedent.
3. Anti-bluff smoke clean over harness + challenge dir (the smoke
   regex is built from string fragments so the script does not match
   itself).
4. Cross-compile linux/amd64 clean (with `-tags=testing_export`).

## Pass criteria

- Harness exits 0 with `==> P1-F16 challenge harness PASS` final line.
- Phase A: `LoadConfigFromEnv` returns `cfg.Enabled=true` +
  `cfg.Exporter=ExporterStdout`; captured stdout contains the
  `llm.Generate` span name, the `llm.model` attribute key, the
  configured model value, and the `helixcode_llm_calls_total` metric;
  `FakeLLMProvider.GenerateCallCount==1`.
- Phase B: in-process `httptest.Server` receives at least one POST to
  `/v1/traces` with a non-empty body; metric POSTs to `/v1/metrics`
  reported when present (count is informational, not load-bearing
  because the periodic metric reader's tempo can race the harness's
  Shutdown call).
- Phase C: captured stdout contains the `llm.Generate` span name AND
  does NOT contain the secret marker. Failure of either assertion is
  a hard CONST-042 violation.
- Phase D: `provider.Exporter()==ExporterNoop`; 100 Generate calls
  succeed; the captured buffer length is exactly 0.
- Phase E: when reachable, span dispatched to the configured
  endpoint without error. Otherwise prints the gated-skip line with
  the reason (env unset, URL unparseable, TCP dial failed).
- Anti-bluff smoke clean over harness file + this CHALLENGE.md +
  run.sh (the smoke regex is built from string fragments so the
  script does not match itself).
- Cross-compile linux/amd64 clean.

## Anti-bluff anchors

- **Phase A.** The `llm.Generate` span name + `llm.model` attribute
  + `helixcode_llm_calls_total` metric in the captured buffer is real
  exporter output — a regression that turned `Generate` into a
  pass-through would emit zero bytes and the assertion would fail
  immediately. The `GenerateCallCount==1` check pinned by Phase A's
  inner-provider construction confirms the real fake-LLM was
  actually invoked.
- **Phase B.** `httptest.Server` recording POSTs is REAL HTTP
  round-trip evidence: the OTel SDK serialised a protobuf payload
  and dispatched it over a TCP socket to a handler that recorded
  exactly the bytes received. A regression that swapped the OTLP/HTTP
  exporter for a print would leave the receiver with zero POSTs.
- **Phase C.** Asserting the marker is absent is the load-bearing
  CONST-042 anti-leak proof. Crucially we ALSO assert the
  `llm.Generate` span name IS present — this rules out the silent-
  failure mode where filter "passes" simply because no export
  happened. Without this paired check the test would pass on a
  broken exporter.
- **Phase D.** The captured-buffer-length-zero assertion is the
  zero-cost claim made mechanical: a regression to "always-on
  stdout" would fill the buffer; a regression to "Generate noop"
  would drop the call count to 0. Both are caught.
