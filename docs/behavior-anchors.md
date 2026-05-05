---
schema_version: 1
constitution_rule: CONST-035
last_audit: 2026-05-01
---

# Behavior Anchor Manifest — Challenges

Every row is a user-facing capability and the single anchor test that
proves it works end-to-end. See CONST-035 in `CONSTITUTION.md`.

## Status legend

- `active` — anchor exists and is callable; capability is verified.
- `pending-anchor` — capability declared, anchor test does not yet
  exist. Listed in `challenges/baselines/bluff-baseline.txt` Section 3.
  Reducing this state is the work of campaign sub-project 4.
- `retired` — capability removed; row kept for history.

## Path format

For Go tests: `<path>.go::<TestFuncName>`. The challenge verifier
greps for `func <TestFuncName>\b` in the file.

## Capabilities

| id | layer | capability | anchor_test_path | verifies | status |
|----|-------|------------|------------------|----------|--------|
| CAP-001 | submodule:Challenges | Run a registered challenge end-to-end via DefaultRunner | pkg/runner/runner_test.go::TestDefaultRunner_Run_Success | Runner.Run() executes a registered challenge and returns a Result with status=passed | active |
| CAP-002 | submodule:Challenges | Register a challenge in DefaultRegistry without collisions | pkg/registry/registry_test.go::TestDefaultRegistry_Register_Success | Registry.Register() accepts a fresh challenge ID and returns no error | active |
| CAP-003 | submodule:Challenges | Evaluate built-in not_empty assertion | pkg/assertion/builtin_test.go::TestEvaluateNotEmpty | NotEmpty evaluator returns pass when given non-empty value, fail when empty | active |
| CAP-004 | submodule:Challenges | Generate Markdown report from challenge results | pkg/report/markdown_test.go::TestMarkdownReporter_GenerateReport_Content | MarkdownReporter.GenerateReport() produces valid Markdown with all results | active |
| CAP-005 | submodule:Challenges | Load challenge bank from JSON or YAML file | pkg/bank/bank_test.go::TestBank_LoadFile | Bank.LoadFile() parses challenge definitions and registers them | active |
| CAP-006 | submodule:Challenges | Construct ShellChallenge that wraps a bash script | pkg/challenge/shell_test.go::TestShellChallenge_NewShellChallenge | ShellChallenge constructor returns a configured Challenge with the script bound | active |
| CAP-007 | submodule:Challenges | Liveness monitor handles nil progress reporter without panicking | pkg/runner/liveness_test.go::TestLivenessMonitor_NilProgress_NoOp | LivenessMonitor with nil progress channel is a safe no-op | active |
| CAP-008 | submodule:Challenges | Construct API HTTP client with default config | pkg/httpclient/client_test.go::TestNewAPIClient_Defaults | NewAPIClient() returns a usable client with sensible defaults | active |
| CAP-009 | submodule:Challenges | Validate challenge result requires positive evidence (anti-bluff metatest) | pkg/challenge/antibluff_test.go::TestValidate_PassWithEvidence | Validate() accepts pass results with non-empty evidence; rejects metadata-only passes | active |
| CAP-010 | submodule:Challenges | userflow-runner CLI resolves "all" platform target to every registered platform key | cmd/userflow-runner/main_test.go::TestResolveGroups_AllPlatformExpandsEveryKey | CLI flag parsing produces the full platform set when --platform=all | active |
| CAP-011 | submodule:Challenges | Run challenges in parallel via DefaultRunner | pkg/runner/parallel_test.go::TestRunParallel_Success | Runner.RunParallel() executes multiple challenges concurrently and returns aggregated results | active |
| CAP-012 | submodule:Challenges | Execute challenge pipeline with stages | pkg/runner/pipeline_test.go::TestPipeline_Execute_Success | Pipeline.Execute() runs ordered stages and produces a successful end-to-end result | active |
| CAP-013 | submodule:Challenges | Anti-bluff runner downgrades a bluff PASS to FAIL in strict mode | pkg/runner/antibluff_runner_test.go::TestAntiBluff_BluffPassDowngraded | Bluff results in strict mode are downgraded — the meta-test that proves the meta-test works | active |
| CAP-014 | submodule:Challenges | Resolve dependency-free challenge ordering | pkg/registry/dependency_test.go::TestGetDependencyOrder_NoDeps | Topological-sort returns the input order when no challenges declare dependencies | active |
| CAP-015 | submodule:Challenges | Register a challenge plugin | pkg/plugin/plugin_test.go::TestRegistry_Register | Plugin.Registry.Register() accepts a fresh plugin and exposes it for lookup | active |
| CAP-016 | submodule:Challenges | Update monitor dashboard data from a lifecycle event | pkg/monitor/dashboard_test.go::TestDashboardData_UpdateFromEvent | DashboardData.UpdateFromEvent() reflects challenge state transitions | active |
| CAP-017 | submodule:Challenges | InfraProvider interface contract is satisfied by adapters | pkg/infra/provider_test.go::TestInfraProvider_Interface | InfraProvider interface methods compile against ContainersAdapter | active |
| CAP-018 | submodule:Challenges | ADB CLI adapter constructor accepts ADB binary path | pkg/userflow/adb_cli_adapter_test.go::TestADBCLIAdapter_Constructor | NewADBCLIAdapter returns a configured adapter with the supplied adb path | active |
| CAP-019 | submodule:Challenges | ProgressReporter constructor exposes a buffered channel | pkg/challenge/progress_test.go::TestProgressReporter_New | NewProgressReporter() returns a ProgressReporter with non-nil progress channel | active |
| CAP-020 | submodule:Challenges | Environment-variable redaction masks API-key values in log output | pkg/env/redact_test.go::TestRedactAPIKey | RedactAPIKey() replaces API key tokens with the redaction sentinel | active |
| CAP-021 | submodule:Challenges | JSON logger constructor binds to stdout | pkg/logging/json_logger_test.go::TestJSONLogger_NewJSONLogger_Stdout | NewJSONLogger(stdout) returns a logger that writes JSON to stdout | active |
| CAP-022 | submodule:Challenges | Prometheus metrics record challenge execution timing | pkg/metrics/metrics_test.go::TestPrometheusMetrics_RecordExecution | RecordExecution() increments the per-challenge counter and observes duration | active |
| CAP-023 | submodule:Challenges | Browser flow challenge constructor binds adapter and steps | pkg/userflow/challenge_browser_test.go::TestNewBrowserFlowChallenge | NewBrowserFlowChallenge() returns a Challenge configured with the supplied adapter | active |
| CAP-024 | submodule:Challenges | Mobile launch challenge constructor accepts mobile adapter and package | pkg/userflow/challenge_mobile_test.go::TestNewMobileLaunchChallenge | NewMobileLaunchChallenge() returns a Challenge bound to mobile adapter and target package | active |

(Manifest now covers core runner+registry+plugin+monitor+infra+userflow
capabilities — 24 active rows. Long-tail: per-protocol httpclient,
adapter-by-adapter userflow tests, recorder integrity checks.)
