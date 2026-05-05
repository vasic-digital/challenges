#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT/HelixCode"

echo "==> build F08 challenge harness"
HARNESS_BIN="$(mktemp -d)/p1f08_challenge"
go build -o "$HARNESS_BIN" ./tests/integration/cmd/p1f08_challenge

echo "==> run harness"
"$HARNESS_BIN"

echo "==> anti-bluff smoke on F08-affected code"
if grep -rn "simulated\|for now\|TODO implement\|placeholder" \
    internal/workflow/planmode/gating.go \
    internal/tools/types_planmode.go internal/tools/plan_tools.go \
    internal/commands/plan_command.go; then
    echo "BLUFF FOUND" >&2
    exit 1
fi
echo "clean"

echo "==> cross-compile linux"
go build ./cmd/cli/... ./internal/workflow/planmode/... ./internal/tools/

echo "==> P1-F08 challenge PASS"
