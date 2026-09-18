# CLAUDE.md - Challenges Module

## INHERITED FROM constitution/CLAUDE.md

All rules in `constitution/CLAUDE.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `CLAUDE.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

## Acceptance demo for this module

```bash
# Full challenge execution engine: Configure → Execute → Validate → Cleanup + report
cd Challenges && GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v \
  -run 'TestRunner_Execute' ./...
```
Expect: PASS; a sample challenge runs to completion, assertion engine evaluates, and JSON/Markdown reports are emitted. The `cmd/userflow-runner` CLI (see `Challenges/README.md`) drives the same pipeline end-to-end.

## Module Overview

`digital.vasic.challenges` is a generic, reusable Go module for defining, registering, executing, and reporting on challenges (structured test scenarios). It provides a comprehensive framework for validation testing with built-in assertion evaluation, multiple reporting formats, live monitoring, and plugin extensibility.

**Module path**: `digital.vasic.challenges`
**Go version**: 1.24+
**Dependencies**: `digital.vasic.containers` (infrastructure bridge), standard Go libraries, testify (tests only)

## Package Responsibilities

| Package | Path | Responsibility |
|---------|------|----------------|
| `challenge` | `pkg/challenge/` | Core types and interfaces: `Challenge` interface defining lifecycle (Configure, Validate, Execute, Cleanup). `BaseChallenge` template implementation. `Config` and `Result` structures. Challenge status enumeration. |
| `registry` | `pkg/registry/` | Challenge registration and dependency management: Registry with topological sorting (Kahn's algorithm). Dependency validation (cycle detection). Challenge lookup by ID/tags. Ordered execution sequencing. |
| `runner` | `pkg/runner/` | Execution engine: Sequential, parallel, and pipeline execution modes. Concurrent execution with semaphore control. Timeout handling and graceful cancellation. Result aggregation and reporting. |
| `assertion` | `pkg/assertion/` | Assertion evaluation engine: 16 built-in evaluators (`not_empty`, `contains`, `min_length`, `quality_score`, etc.). Custom evaluator registration. Expression parser for complex assertions. Flexible comparison operators. |
| `report` | `pkg/report/` | Report generation: Multiple formats (Markdown, JSON, HTML). Summary statistics (pass/fail/skip counts). Detailed execution timelines. Export to file or string. |
| `logging` | `pkg/logging/` | Structured logging: JSON and console formatters. Multi-logger composition. Redacting logger for sensitive data. API request/response tracking. |
| `env` | `pkg/env/` | Environment variable management: Load from .env files. Variable interpolation and defaults. Redaction patterns for secrets (API keys, tokens). Validation and type conversion. |
| `bank` | `pkg/bank/` | Challenge bank (definition loading): Load challenge definitions from JSON/YAML. Template variable substitution. Bulk challenge instantiation. Definition validation. |
| `monitor` | `pkg/monitor/` | Live monitoring: WebSocket-based real-time dashboard. Event collection (challenge start/complete/fail). Progress tracking and ETA calculation. Metrics export. |
| `metrics` | `pkg/metrics/` | Prometheus-compatible metrics: Challenge execution counters. Duration histograms. Success/failure rates. Custom metric registration. |
| `plugin` | `pkg/plugin/` | Plugin system: Plugin interface for custom challenge types. Dynamic plugin loading. Lifecycle management (Init, Shutdown). Plugin registry and versioning. |
| `infra` | `pkg/infra/` | Infrastructure bridge: Adapter to `digital.vasic.containers` module. Service startup/shutdown coordination. Health check integration. Resource cleanup. |
| `userflow` | `pkg/userflow/` | Multi-platform user flow automation: adapters, templates, evaluators, container infra bridge (see "User Flow Automation" below). |

## Dependency Graph

```
runner  --->  challenge, registry, assertion, report, logging
registry  --->  challenge
assertion  --->  logging
report  --->  challenge, logging
monitor  --->  challenge, metrics
plugin  --->  challenge, registry
infra  --->  challenge, containers module
bank  --->  challenge, env
```

`challenge` is the foundational package. `runner` integrates most packages for orchestration.

## Key Files

| File | Purpose |
|------|---------|
| `pkg/challenge/challenge.go` | Challenge interface, BaseChallenge, Config, Result types |
| `pkg/challenge/shell.go` | ShellChallenge for executing bash scripts |
| `pkg/challenge/progress.go` | ProgressReporter for progress-based liveness detection |
| `pkg/registry/registry.go` | Registry implementation with dependency ordering |
| `pkg/runner/runner.go` | Runner implementation with execution modes |
| `pkg/runner/liveness.go` | Liveness monitor (stuck-vs-timed-out detection) |
| `pkg/assertion/engine.go` | Assertion engine with built-in evaluators |
| `pkg/assertion/evaluators.go` | All 16 built-in evaluator implementations |
| `pkg/report/markdown.go` | Markdown report generator |
| `pkg/report/json.go` | JSON report generator |
| `pkg/report/html.go` | HTML report generator |
| `pkg/logging/logger.go` | Logger interface and implementations |
| `pkg/env/loader.go` | Environment variable loader with redaction |
| `pkg/bank/bank.go` | Challenge bank with JSON/YAML loading |
| `pkg/monitor/monitor.go` | Live monitoring with WebSocket server |
| `pkg/metrics/metrics.go` | Prometheus metrics collector |
| `pkg/plugin/plugin.go` | Plugin interface and registry |
| `pkg/infra/adapter.go` | Containers module adapter |
| `pkg/userflow/` | User flow automation adapters, templates, evaluators |
| `cmd/userflow-runner` | CLI driving the userflow pipeline end-to-end |
| `go.mod` | Module definition and dependencies |
| `CLAUDE.md` | AI coding assistant instructions (this file) |
| `README.md` | User-facing documentation with quick start |

## Design Patterns

- **Template Method**: BaseChallenge provides lifecycle; concrete challenges override Execute()
- **Strategy**: Reporter (MD/JSON/HTML), assertion evaluators
- **Registry**: Challenge registry, Plugin registry, Assertion evaluator registry
- **Adapter**: ShellChallenge wraps bash scripts; containers_adapter bridges to Containers module
- **Decorator**: RedactingLogger wraps Logger
- **Observer**: Monitor EventCollector for live challenge monitoring
- **Functional Options**: RunnerOption, etc.

## Progress-Based Liveness Detection

Challenges may run for hours (e.g., scanning a 10TB NAS over SMB). Hard timeouts are **wrong** for long-running challenges. Instead, the framework uses progress-based liveness detection:

- **ProgressReporter** (`pkg/challenge/progress.go`): Buffered channel (64) for challenges to signal forward progress. Call `ReportProgress(msg, data)` periodically.
- **Liveness Monitor** (`pkg/runner/liveness.go`): Goroutine that watches the progress channel. If no progress is reported within `StaleThreshold`, the challenge is declared **stuck** and cancelled.
- **StatusStuck** (`"stuck"`): New terminal status distinct from `"timed_out"`. Stuck = no progress; timed out = hard timeout exceeded.

### Usage

```go
// In your challenge's Execute():
func (c *MyChallenge) Execute(ctx context.Context) (*Result, error) {
    for i := range files {
        c.ReportProgress("scanning", map[string]any{
            "files_processed": i,
        })
        // ... do work ...
    }
}
```

The runner automatically attaches a `ProgressReporter` to any challenge that embeds `BaseChallenge`. Configure thresholds:

```go
runner.NewRunner(
    runner.WithTimeout(72*time.Hour),         // Hard upper bound (generous)
    runner.WithStaleThreshold(5*time.Minute), // Kill if no progress for 5 min
)
```

Per-challenge override via `Config.StaleThreshold`.

## User Flow Automation (`pkg/userflow`)

Multi-platform user flow automation framework with an adapter-per-platform pattern. Generic — no project-specific references in `pkg/userflow/`.

**Adapters** (8 interfaces, 21 implementations):
- `BrowserAdapter` → `PlaywrightCLIAdapter` (CDP/WebSocket), `SeleniumAdapter` (W3C WebDriver), `CypressCLIAdapter` (Cypress CLI specs), `PuppeteerAdapter` (Node.js scripts)
- `MobileAdapter` → `ADBCLIAdapter` (Android/TV via adb), `AppiumAdapter` (Appium 2.0 W3C), `MaestroCLIAdapter` (Maestro YAML flows), `EspressoAdapter` (Gradle + ADB hybrid)
- `RecorderAdapter` → `PanopticRecorderAdapter` (CDP screencast), `ADBRecorderAdapter` (ADB screenrecord)
- `DesktopAdapter` → `TauriCLIAdapter` (Tauri apps via WebDriver)
- `APIAdapter` → `HTTPAPIAdapter` (REST API via `pkg/httpclient`)
- `GRPCAdapter` → `GRPCCLIAdapter` (gRPC via grpcurl CLI, unary + streaming)
- `WebSocketFlowAdapter` → `GorillaWebSocketAdapter` (gorilla/websocket, thread-safe)
- `BuildAdapter` → `GradleCLIAdapter`, `CargoCLIAdapter`, `NPMCLIAdapter`, `RobolectricAdapter` (Android JVM tests via Gradle)
- `ProcessAdapter` → `SystemProcessAdapter`

**Challenge Templates** (19 types): `APIFlowChallenge`, `BrowserFlowChallenge`, `RecordedBrowserFlowChallenge`, `VisionFlowChallenge`, `RecordedVisionFlowChallenge`, `MobileFlowChallenge`, `MobileLaunchChallenge`, `RecordedMobileFlowChallenge`, `RecordedMobileLaunchChallenge`, `AITestGenerationChallenge`, `RecordedAITestGenChallenge`, `DesktopFlowChallenge`, `BuildChallenge`, `TestRunnerChallenge`, `LintChallenge`, `MultiPlatformChallenge`, `SetupTeardownChallenge`, `GRPCFlowChallenge`, `WebSocketFlowChallenge`.

**Recorded Challenge Templates** wrap their non-recorded counterparts with `RecorderAdapter`, adding video recording with integrity verification (non-zero file size, duration, frame count). Use these for all UI challenges.

**Container Integration**: `TestEnvironment` bridges to `digital.vasic.containers` via `PlatformGroup` concept. Manages Podman container lifecycle (setup/teardown) per platform group within the 4 CPU / 8 GB resource budget.

**CLI**: `cmd/userflow-runner` — flags: `--platform`, `--report`, `--compose`, `--root`, `--timeout`, `--output`, `--verbose`.

**Evaluators** (12 userflow-specific): `http_status_ok`, `http_status_created`, `http_status_unauthorized`, `http_status_forbidden`, `http_status_not_found`, `http_json_valid`, `browser_element_visible`, `browser_url_matches`, `mobile_activity_visible`, `mobile_element_exists`, `build_success`, `test_pass_rate`.

## Agent Coordination Guide

### Division of Work

When multiple agents work on this module simultaneously, divide work by package boundary:

1. **Challenge Agent** -- Owns `pkg/challenge/`. Core types affect all other packages. Must coordinate before modifying `Challenge` interface or `Result` structure.
2. **Registry Agent** -- Owns `pkg/registry/`. Dependency ordering logic. Changes rarely affect other packages except runner.
3. **Runner Agent** -- Owns `pkg/runner/`. Integration layer. Requires testing against all execution modes.
4. **Assertion Agent** -- Owns `pkg/assertion/`. New evaluators can be added independently. Evaluator registry changes require runner updates.
5. **Report Agent** -- Owns `pkg/report/`. New report formats can be added independently. Reporter interface changes affect runner.
6. **Monitoring Agent** -- Owns `pkg/monitor/`. Real-time monitoring. Can work independently but coordinates with runner for event hooks.
7. **Plugin Agent** -- Owns `pkg/plugin/`. Plugin system. Must coordinate with registry for plugin registration.

### Coordination Rules

- **Challenge interface changes** require all agents to update. The `Challenge` interface is the shared contract.
- **Assertion evaluators** and **report formats** are independent and can be modified in parallel.
- **Runner package** integrates all packages. Any interface change in sub-packages requires corresponding runner updates.
- **Monitor and metrics** packages are loosely coupled. Coordinate on event schema.
- **Test isolation**: Each package has its own `_test.go` files. Integration tests in `runner` package.
- **No circular dependencies**: The dependency graph is strictly acyclic. Never import `runner` from sub-packages.

### Safe Parallel Changes

These changes can be made simultaneously without coordination:
- Adding a new assertion evaluator to `pkg/assertion/`
- Adding a new report format to `pkg/report/`
- Adding new monitoring events to `pkg/monitor/`
- Adding new plugins to `pkg/plugin/`
- Adding new tests to any package
- Updating documentation

### Changes Requiring Coordination

- Modifying the `Challenge` interface methods
- Changing `Result` structure fields
- Modifying assertion evaluator registry interface
- Adding new execution modes to runner
- Changing event schema in monitor
- Modifying plugin interface

## Build and Test Commands

```bash
# Build all packages
go build ./...

# Run all tests with race detection
go test ./... -count=1 -race

# Run unit tests only (short mode)
go test ./... -short

# Run integration tests (requires Containers module)
go test -tags=integration ./...

# Run benchmarks
go test -bench=. ./tests/benchmark/

# Run a specific test
go test -v -run TestRunner_RunAll ./pkg/runner/

# Format code
gofmt -w .

# Vet code
go vet ./...
```

## Commit Conventions

Follow Conventional Commits with package scope:

```
feat(assertion): add regex_match evaluator
feat(report): add PDF report generator
feat(plugin): implement plugin versioning
fix(runner): prevent race condition in parallel execution
fix(registry): detect circular dependencies correctly
test(assertion): add evaluator edge case tests
docs(challenges): update plugin development guide
refactor(monitor): extract WebSocket server to separate file
```

## Thread Safety Notes

- **Runner** executes challenges concurrently with semaphore control. Uses `sync.WaitGroup` for coordination.
- **Registry** is thread-safe for reads after initialization. Writes during registration use mutex.
- **Assertion engine** evaluators must be safe for concurrent invocation.
- **Monitor** uses channels for event collection and mutex for state access.
- **Metrics collector** uses atomic operations for counters.
- **Plugin registry** locks during plugin loading/unloading.

## Built-in Assertion Evaluators

| Evaluator | Description | Example |
|-----------|-------------|---------|
| `not_empty` | Value must not be empty string or nil | `"result": { "not_empty": true }` |
| `not_mock` | Response must not be a mock/placeholder | `"response": { "not_mock": true }` |
| `contains` | String contains substring | `"output": { "contains": "success" }` |
| `contains_any` | String contains any of the substrings | `"output": { "contains_any": ["ok", "success"] }` |
| `min_length` | String has minimum length | `"response": { "min_length": 10 }` |
| `quality_score` | LLM response quality score (0-1) | `"quality": { "quality_score": 0.8 }` |
| `reasoning_present` | Response contains reasoning/explanation | `"answer": { "reasoning_present": true }` |
| `code_valid` | Code block is syntactically valid | `"code": { "code_valid": "go" }` |
| `min_count` | Array/collection minimum count | `"items": { "min_count": 5 }` |
| `exact_count` | Array/collection exact count | `"items": { "exact_count": 10 }` |
| `max_latency` | Operation completed within time limit (ms) | `"latency": { "max_latency": 1000 }` |
| `all_valid` | All items in array pass validation | `"results": { "all_valid": true }` |
| `no_duplicates` | Array has no duplicate values | `"ids": { "no_duplicates": true }` |
| `all_pass` | All nested assertions pass | `"tests": { "all_pass": true }` |
| `no_mock_responses` | No mock/placeholder responses in collection | `"responses": { "no_mock_responses": true }` |
| `min_score` | Numeric score meets minimum threshold | `"score": { "min_score": 85.0 }` |

## Configuration Example

```go
package main

import (
    "context"
    "digital.vasic.challenges/pkg/challenge"
    "digital.vasic.challenges/pkg/registry"
    "digital.vasic.challenges/pkg/runner"
    "digital.vasic.challenges/pkg/report"
)

func main() {
    // Create registry
    reg := registry.New()

    // Register challenges
    reg.Register(&MyChallenge{
        id: "test-api",
        dependencies: []string{},
    })

    // Create runner
    run := runner.New(reg,
        runner.WithParallelism(5),
        runner.WithTimeout(30*time.Second),
    )

    // Execute all challenges
    results, _ := run.RunAll(context.Background())

    // Generate report
    reporter := report.NewMarkdownReporter()
    report := reporter.Generate(results)
    fmt.Println(report)
}
```

## Custom Challenge Example

```go
type APITestChallenge struct {
    challenge.BaseChallenge
    endpoint string
}

func (c *APITestChallenge) Execute(ctx context.Context) error {
    resp, err := http.Get(c.endpoint)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    // Evaluate assertions
    return c.AssertionEngine.Evaluate(ctx, map[string]interface{}{
        "status_code": resp.StatusCode,
        "response": map[string]interface{}{
            "not_empty": true,
            "contains": "success",
        },
    })
}
```

## Best Practices

### 1. Use Dependency Ordering
```go
// Good - declare dependencies
challenge := &MyChallenge{
    dependencies: []string{"setup-database", "start-api"},
}

// Registry will execute in correct order
```

### 2. Implement Cleanup
```go
func (c *MyChallenge) Cleanup(ctx context.Context) error {
    // Always clean up resources
    c.client.Close()
    c.db.Close()
    return nil
}
```

### 3. Use Assertion Engine
```go
// Good - use built-in evaluators
return c.AssertionEngine.Evaluate(ctx, map[string]interface{}{
    "result": {
        "not_empty": true,
        "min_length": 10,
    },
})

// Bad - manual validation
if result == "" || len(result) < 10 {
    return errors.New("validation failed")
}
```

### 4. Set Timeouts
```go
// Good - always set timeouts
runner := runner.New(reg,
    runner.WithTimeout(30*time.Second),
)

// Bad - no timeout (can hang indefinitely)
runner := runner.New(reg)
```

### 5. Use Structured Logging
```go
// Good - structured logging
logger.InfoWithFields("Challenge executed", map[string]interface{}{
    "challenge_id": c.ID(),
    "duration": elapsed,
    "status": "passed",
})

// Bad - unstructured logging
log.Printf("Challenge %s took %v and passed", c.ID(), elapsed)
```

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | Containers |
| Downstream (these import this module) | HelixLLM, HelixQA, LLMsVerifier |

*Siblings* means other project-owned modules at the project repo root. The root project app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See this repository's root-level agent-instruction carriers for the rules that keep these seams contract-tested.

---

**Last Updated**: February 10, 2026
**Version**: 1.0.0
**Status**: Production Ready
