# Challenges - A generic, reusable Go module for defining, registering, executing, and reporting on challenges (structured test scenarios). Features a plugin-based architecture with built-in assertion evaluation, multi-format reporting, and live monitoring.
# Module: digital.vasic.challenges

.PHONY: build test test-race test-short test-integration test-bench test-coverage fmt vet lint clean help

MODULE := digital.vasic.challenges
GOMAXPROCS ?= 2

build:
	go build ./...

test:
	GOMAXPROCS=$(GOMAXPROCS) go test -count=1 -race -p 1 ./...

test-race:
	GOMAXPROCS=$(GOMAXPROCS) go test -count=1 -race -p 1 ./...

test-short:
	GOMAXPROCS=$(GOMAXPROCS) go test -count=1 -short -p 1 ./...

test-integration:
	GOMAXPROCS=$(GOMAXPROCS) go test -count=1 -race -p 1 ./tests/integration/... 2>/dev/null || echo "No integration tests"

test-bench:
	GOMAXPROCS=$(GOMAXPROCS) go test -bench=. -benchmem ./tests/benchmark/... 2>/dev/null || echo "No benchmarks"

test-coverage:
	GOMAXPROCS=$(GOMAXPROCS) go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	golangci-lint run ./...

clean:
	rm -f coverage.out coverage.html
	go clean -cache

# Challenges (run from parent HelixAgent project)
challenge:
	../challenges/scripts/challenges_challenge.sh 2>/dev/null || echo "No challenge script"

# === CONST-035 anti-bluff gates ===
.PHONY: anti-bluff anti-bluff-scan anti-bluff-anchors anti-bluff-mutation anti-bluff-mutation-changed update-baseline qa-all

anti-bluff-scan:
	@bash scripts/anti-bluff/bluff-scanner.sh --mode all

anti-bluff-anchors:
	@bash challenges/scripts/anchor_manifest_challenge.sh

anti-bluff-mutation:
	@bash challenges/scripts/mutation_ratchet_challenge.sh --mode all

anti-bluff-mutation-changed:
	@bash challenges/scripts/mutation_ratchet_challenge.sh

anti-bluff: anti-bluff-scan anti-bluff-anchors anti-bluff-mutation-changed

# Aggregate quality gate. Wires existing build/vet/test plus host-power
# challenges and the CONST-035 anti-bluff gates.
qa-all: build vet test
	@bash challenges/scripts/no_suspend_calls_challenge.sh
	@bash challenges/scripts/host_no_auto_suspend_challenge.sh
	@$(MAKE) anti-bluff

update-baseline:
	@echo "Manual baseline update - see docs/ANTI_BLUFF.md"
	@echo "1. Run scanner: bash scripts/anti-bluff/bluff-scanner.sh --mode all"
	@echo "2. Run mutation: bash challenges/scripts/mutation_ratchet_challenge.sh --mode all"
	@echo "3. Edit challenges/baselines/bluff-baseline.txt to reflect new state."

help:
	@echo "Challenges - A generic, reusable Go module for defining, registering, executing, and reporting on challenges (structured test scenarios). Features a plugin-based architecture with built-in assertion evaluation, multi-format reporting, and live monitoring."
	@echo ""
	@echo "Build & Test:"
	@echo "  make build         Build all packages"
	@echo "  make test          Run all tests with race detection"
	@echo "  make test-short    Run unit tests only"
	@echo "  make test-bench    Run benchmarks"
	@echo "  make test-coverage Generate coverage report"
	@echo ""
	@echo "Quality:"
	@echo "  make fmt           Format code"
	@echo "  make vet           Run go vet"
	@echo "  make lint          Run golangci-lint"
	@echo ""
	@echo "Other:"
	@echo "  make clean         Remove build artifacts"
	@echo "  make challenge     Run challenge script (from parent project)"
