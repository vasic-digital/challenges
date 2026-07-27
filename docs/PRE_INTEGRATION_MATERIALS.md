# Challenges — Pre-Integration Materials

**Revision:** 1
**Last modified:** 2026-07-15T11:19:17Z
**Purpose:** Consolidated pre-integration materials (gate before any integration/deployment work).

> Scope note: this document consolidates and cross-references material that
> already lives in this repository. It does **not** restate `ARCHITECTURE.md`
> or the `docs/` set — it points to them and adds the deploy/ports/health/boot
> facts an integrator needs before wiring this module into a consumer. Every
> statement below is grounded in a real file in this repository (paths cited).
> Undeterminable items are marked `UNKNOWN:` rather than invented.

---

## 1. Purpose / What it is

`digital.vasic.challenges` is a **generic, reusable Go module** for defining,
registering, executing, and reporting on **challenges** — structured test
scenarios. It is a plugin-based test-challenge framework with built-in
assertion evaluation, multi-format reporting, and live monitoring.

- Module path: `digital.vasic.challenges` (`go.mod:1`).
- Self-description: "A generic, reusable Go module for defining, registering,
  executing, and reporting on challenges (structured test scenarios)."
  (`README.md:3`).
- It is **project-agnostic test infrastructure** (§11.4.27 / §11.4.28): it is
  consumed as a dependency by other modules and carries no project-specific
  context. Per `CLAUDE.md` "Integration Seams", downstream consumers are
  `HelixLLM`, `HelixQA`, `LLMsVerifier` (and, per `README.md:174-178`, the
  wider consuming-project family); its single upstream dependency is
  `Containers`.
- Because it is itself test infrastructure, its anti-bluff posture is **meta**
  — "the bank validates the banks" (`README.md:172-221`,
  `docs/ANTI_BLUFF.md`).

Full architectural narrative: **`ARCHITECTURE.md`** and **`docs/ARCHITECTURE.md`**
(do not duplicate — read those).

---

## 2. Architecture overview

Authoritative source: **`ARCHITECTURE.md`** (`## Structure`, `## Key
Components`, `## Data Flow`), mirrored at **`docs/ARCHITECTURE.md`**, plus
diagrams under `docs/diagrams/` (`architecture.mmd`, `challenge_lifecycle.mmd`,
`monitoring_flow.mmd`, `runner_sequence.mmd`). Summary only below.

### Real top-level layout (grounded via `find`)

| Dir | Role (cited) |
|-----|--------------|
| `pkg/` | Go library packages (see table below) |
| `cmd/userflow-runner/` | CLI entrypoint (`cmd/userflow-runner/main.go`) |
| `banks/` | Challenge-bank definitions: `banks/examples/` (28 generic JSON example banks), `banks/yole/` (feature-coverage YAML banks + fixtures) |
| `challenges/` | Shell-script challenge banks + fixtures: `challenges/scripts/*.sh`, `challenges/fixtures/`, `challenges/baselines/` |
| `docs/` | Full documentation set (see §6/§8 and the `docs/` index) |
| `scripts/` | Anti-bluff + host-power-management helper scripts |
| `tests/` | `tests/chaos/`, `tests/stress/` |
| `lib/`, `commit/`, `upstreams/` | Support tooling |

### `pkg/` packages (cited: `ARCHITECTURE.md:9-27`, `CLAUDE.md` "Package Structure")

`challenge` (core types: `Challenge` interface, `Config`, `Result`,
`BaseChallenge`, `ProgressReporter`), `registry` (registration + Kahn's
topological dependency ordering), `runner` (sequential/parallel/pipeline
execution + liveness monitoring), `assertion` (engine + 16 built-in
evaluators), `report` (Markdown/JSON/HTML), `logging`, `env` (redaction),
`httpclient`, `bank` (load definitions from JSON/YAML), `monitor` (live
WebSocket/SSE dashboard), `metrics` (Prometheus-compatible), `plugin`, `infra`
(bridge to `digital.vasic.containers`), `userflow` (multi-platform automation:
8 adapter interfaces, 21 implementations, 19 challenge templates, 12
evaluators). Additional packages present in-tree: `pkg/container`,
`pkg/panoptic`.

### The "challenge bank" concept (cited: `README.md:90,174-201`)

A **challenge bank** is a set of challenge definitions loaded from data files
(`pkg/bank` loads JSON/YAML) or from shell scripts (`ShellChallenge` wraps a
bash script as a challenge). Real banks in-tree: `banks/examples/*.json` (28),
`banks/yole/*.yaml`, and `challenges/scripts/*.sh` (baseline shell-script
challenges). The meta-runner `challenges_describe_challenge.sh` inventories and
validates every bank (`README.md:203-211`).

### Stack

- Language/runtime: **Go 1.25.0** (`go.mod:3`). (`CLAUDE.md`/`README.md` prose
  say "Go 1.24+"; `go.mod` is authoritative at `go 1.25.0`.)
- Design patterns: Template Method, Strategy, Registry, Adapter, Decorator,
  Observer, Functional Options (`CLAUDE.md` "Design Patterns").

---

## 3. Dependencies

### Go module dependencies (cited: `go.mod`)

Direct:

| Dependency | Version | Note |
|------------|---------|------|
| `digital.vasic.containers` | `v0.0.0-…` | `replace => ../containers` (`go.mod:12`) — local sibling |
| `github.com/gorilla/websocket` | `v1.5.3` | live monitoring + WebSocket flow testing |
| `github.com/stretchr/testify` | `v1.11.1` | test assertions |
| `gopkg.in/yaml.v3` | `v3.0.1` | YAML bank loading |

Indirect (`go.mod:14-20`): `davecgh/go-spew`, `kr/pretty`,
`pmezard/go-difflib`, `stretchr/objx`, `gopkg.in/check.v1`.

### Submodule / dependency manifest (cited: `helix-deps.yaml`, `.gitmodules`)

- `helix-deps.yaml` (§11.4.31 manifest) declares two own-org deps, both
  `layout: flat`:
  - `containers` — `git@github.com:vasic-digital/Containers.git` (`main`);
    consumed at the parent root via a flat sibling + the `../containers` go.mod
    replace directive.
  - `Panoptic` — `git@github.com:vasic-digital/Panoptic.git` (`main`);
    documented as **currently nested** at `Submodules/Challenges/Panoptic`, a
    tracked §11.4.28 violation whose flat-layout migration is owed
    (`helix-deps.yaml:5-17,29-34`).
- `.gitmodules` is present but **empty (0 bytes)** — no active git-submodule
  wiring in this checkout (`.gitmodules` = 0 bytes). The dependency graph is
  expressed via `helix-deps.yaml` + the go.mod `replace`, not `.gitmodules`.

### Infrastructure dependency

- `digital.vasic.containers` — Podman container orchestration bridge for test
  infrastructure (`pkg/infra`, `pkg/userflow/container_infra.go`; imports
  `digital.vasic.containers/pkg/{compose,event,health,logging,runtime,serviceregistry}`
  per `helix-deps.yaml:28`). `TestEnvironment` manages container lifecycle per
  platform group within a 4 CPU / 8 GB resource budget (`CLAUDE.md` "User Flow
  Automation" → Container Integration).
- External CLI tools are runtime-optional per adapter (e.g. `grpcurl`,
  Playwright, `adb`, Gradle) — invoked only when the corresponding userflow
  adapter is used; adapters detect availability rather than hard-requiring it
  (`docs/userflow/*-adapter.md`).

---

## 4. Deploy / Distribution design

This module is **distributed as a Go library dependency, not a long-running
deployed service.**

- **Primary distribution = library.** `go get digital.vasic.challenges`, then
  import `pkg/*` (`README.md:5-9`). Downstream consumers `HelixLLM` / `HelixQA`
  / `LLMsVerifier` link it as a module dependency (`CLAUDE.md` "Integration
  Seams"). There is no server artifact to deploy — the consumer embeds it.
- **Secondary distribution = CLI binary.** `cmd/userflow-runner` builds to a
  standalone runner (`go build ./...`, `CLAUDE.md` "Build & Test"; flags in §7).
- **Tertiary distribution = data + shell banks.** Challenge banks
  (`banks/**/*.{json,yaml}`) and shell-script challenges
  (`challenges/scripts/*.sh`) are consumer-owned data / executables the library
  loads or wraps; they travel with the repo and are validated by
  `challenges_describe_challenge.sh`.
- **Distribution slice** (what an integrator pulls in): the `pkg/` library
  packages + (optionally) the `cmd/userflow-runner` binary + the challenge-bank
  data files the consumer chooses to load. The `digital.vasic.containers`
  sibling must be resolvable (flat layout + go.mod replace) for `pkg/infra` /
  `pkg/userflow` container features to compile and run.
- **Rootless container runtime**: container-backed test environments use Podman
  (rootless) per the Containers bridge (`CLAUDE.md`; §11.4.161).

---

## 5. Ports

`UNKNOWN:` — **library/test-framework: no fixed own listen port.** As a Go
library + CLI, the module opens no server port by default.

Grounded nuances (cited):

- The **only** server it can start is the **optional** live-monitoring
  WebSocket/SSE endpoint in `pkg/monitor/websocket.go`. Its listen address is
  **caller-supplied** — `NewWebSocketServer(addr string, …)` stores `addr`
  (`pkg/monitor/websocket.go:28-30`) and the `http.Server` binds `Addr: s.addr`
  (`pkg/monitor/websocket.go:47-48`). There is **no hardcoded/default port
  constant** in the module for it; the integrator chooses the bind address, and
  the monitor is only started when the consumer opts in.
- `pkg/userflow/tauri_cli_adapter.go:544` opens an ephemeral loopback listener
  (`net.Listen("tcp", "127.0.0.1:0")` — OS-assigned port) internal to the Tauri
  desktop adapter; not a service port.
- Port references such as `http://localhost:8090/health`
  (`pkg/container/verifier.go:52`) and the gRPC `serverAddr`
  (`pkg/userflow/adapter_grpc.go:125`) are **targets under test**, not ports
  this module listens on.

---

## 6. Health

`UNKNOWN:` — **no mandatory always-on health endpoint** (consistent with §5:
the module is not a standing service).

Grounded facts (cited):

- The optional monitor server registers a `/health` handler
  (`pkg/monitor/websocket.go:42`) — available only when the consumer starts the
  monitor with an `addr` (see §5).
- Health-check surfaces the module **calls on other services** (consumer-side,
  not self):
  - `pkg/infra/provider.go:12-13` + `pkg/infra/containers_adapter.go:65` —
    `HealthCheck(ctx, serviceName)` bridge to the Containers module.
  - `pkg/userflow/http_api_adapter.go:179-183` — `GET /health` against the
    system-under-test (status < 500 = healthy).
  - `pkg/userflow/adapter_grpc.go:39-41,207-209` — gRPC standard health check
    against a target service.
  - `pkg/container/verifier.go:52` — verifies a target container at
    `http://localhost:8090/health`.
- Framework-level liveness for long-running challenges is **progress-based**,
  not port-based: `ProgressReporter` (`pkg/challenge/progress.go`) + the
  liveness monitor (`pkg/runner/liveness.go`) declare a challenge `stuck` if no
  progress within `StaleThreshold` (`CLAUDE.md` "Progress-Based Liveness
  Detection").

---

## 7. How it boots

`Consumed as a library` is the primary path; a CLI and shell banks are also
provided.

- **As a library** (primary): the consumer constructs a `registry.Registry`,
  registers challenges, builds a `runner.Runner` with options, and calls
  `RunAll(ctx, config)`. Worked example: `README.md:52-76`. Data flow:
  `ARCHITECTURE.md:39-54`.
- **As a CLI**: `cmd/userflow-runner/main.go` — `main()` at line 118 delegates
  to `run() int` (line 122). Boot = parse flags (`flag.Parse()`,
  `cmd/userflow-runner/main.go:172`) then execute. Documented flags
  (`CLAUDE.md` "User Flow Automation" → CLI; `README.md:126`):
  `--platform`, `--report`, `--compose`, `--root`, `--timeout`, `--output`,
  `--verbose`. Additional flags present in `main.go`: a platform-groups file
  flag and a strict-anti-bluff flag (`cmd/userflow-runner/main.go:152,165`).
- **As shell challenge banks**: `bash challenges_describe_challenge.sh` (clean
  PASS = exit 0; `--anti-bluff-mutate` = planted-mutation FAIL, exit 99)
  (`README.md:203-211`); individual banks under `challenges/scripts/*.sh`.
- **Build / test entrypoints** (`CLAUDE.md` "Build & Test", `Makefile`):
  `go build ./...`, `go test ./... -count=1 -race`, and `Makefile` targets
  (`build`, `test`, `test-race`, `test-short`, `test-integration`,
  `test-bench`, `anti-bluff`, `qa-all`, `challenge`, …).
- Acceptance demo (`CLAUDE.md` "Acceptance demo"):
  `GOMAXPROCS=2 nice -n 19 go test -count=1 -race -v -run 'TestRunner_Execute' ./...`.

---

## 8. Materials status (verification pass)

Verification method: each row grounded against a real file in this repository
(paths in §1–§7). "HAS-VERIFIED" = the material already existed in-tree and was
confirmed against source; "CONSOLIDATED-FROM-SOURCE" = the fact existed in the
code but was not previously stated in a single pre-integration doc, so it was
extracted from source here (still a verified fact, not gap-filled/invented).

| # | Material | Source of truth | Status |
|---|----------|-----------------|--------|
| 1 | Purpose / what it is | `README.md:3`, `ARCHITECTURE.md:3-5`, `CLAUDE.md` Overview | HAS-VERIFIED |
| 2 | Architecture / structure | `ARCHITECTURE.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/*.mmd` | HAS-VERIFIED |
| 3 | Package inventory (`pkg/*`) | `ARCHITECTURE.md:9-27`, `CLAUDE.md` Package Structure | HAS-VERIFIED |
| 4 | Challenge-bank concept + inventory | `README.md:174-201`, `banks/`, `challenges/scripts/` (find) | HAS-VERIFIED |
| 5 | Go dependencies | `go.mod` | HAS-VERIFIED |
| 6 | Submodule/dep manifest | `helix-deps.yaml`; `.gitmodules` (empty, 0 bytes) | HAS-VERIFIED |
| 7 | Distribution = library + CLI + data | `README.md:5-9`, `CLAUDE.md` Integration Seams, `cmd/` | HAS-VERIFIED |
| 8 | Ports (no fixed own port) | `pkg/monitor/websocket.go:28-48` (caller-supplied addr) | CONSOLIDATED-FROM-SOURCE |
| 9 | Health (optional monitor `/health`; SUT-side checks) | `pkg/monitor/websocket.go:42`, `pkg/infra/*`, `pkg/userflow/*` | CONSOLIDATED-FROM-SOURCE |
| 10 | Boot / entrypoint | `README.md:52-76`, `cmd/userflow-runner/main.go:118-172`, `CLAUDE.md` Acceptance demo | HAS-VERIFIED |
| 11 | Anti-bluff / test-coverage posture | `docs/ANTI_BLUFF.md`, `docs/test-coverage.md`, `README.md:172-221` | HAS-VERIFIED |
| 12 | Full doc set (API/user/plugin/userflow) | `docs/API_REFERENCE.md`, `docs/USER_GUIDE.md`, `docs/PLUGIN_GUIDE.md`, `docs/userflow/*` | HAS-VERIFIED |

### Overall verdict: **HAS-VERIFIED**

The pre-integration materials already exist in this repository (README,
`ARCHITECTURE.md`, and a full `docs/` set including `ANTI_BLUFF`,
`API_REFERENCE`, `USER_GUIDE`, `PLUGIN_GUIDE`, `test-coverage`, and the
`docs/userflow/` adapter/framework docs). This file consolidates them and adds
the ports/health/boot facts extracted directly from source. No material was
missing or fabricated. Two rows (ports, health) are marked
CONSOLIDATED-FROM-SOURCE because those facts, while true and code-grounded, had
not previously been captured in a single pre-integration document.

### Recorded `UNKNOWN:` items

- `UNKNOWN:` **Ports** — library/test-framework, no fixed own listen port; the
  optional monitor server's bind address is caller-supplied (§5).
- `UNKNOWN:` **Health** — no mandatory always-on health endpoint; the monitor's
  `/health` is opt-in and the other health checks target external
  systems-under-test (§6).
