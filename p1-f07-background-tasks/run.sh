#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT/HelixCode"

echo "==> build F07 challenge harness"
HARNESS_BIN="$(mktemp -d)/p1f07_challenge"
go build -o "$HARNESS_BIN" ./tests/integration/cmd/p1f07_challenge

echo "==> run harness"
"$HARNESS_BIN"

echo "==> anti-bluff smoke on F07-affected code"
if grep -rn "simulated\|for now\|TODO implement\|placeholder" \
    internal/workflow/background.go \
    internal/workflow/background_test.go \
    internal/tools/types_background.go \
    internal/tools/task_tools.go \
    internal/tools/shell/background.go \
    internal/commands/tasks_command.go; then
    echo "BLUFF FOUND" >&2
    exit 1
fi
echo "clean"

echo "==> cross-compile linux"
go build ./cmd/cli/... ./internal/workflow/... ./internal/tools/

echo "==> P1-F07 challenge PASS"
