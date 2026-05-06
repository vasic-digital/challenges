#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT/HelixCode"

echo "==> build F23 challenge harness"
HARNESS_BIN="$(mktemp -d)/p2f23_challenge"
go build -o "$HARNESS_BIN" ./tests/integration/cmd/p2f23_challenge

echo "==> run harness"
"$HARNESS_BIN"

echo "==> anti-bluff smoke on F23-affected code"
P1="simul"; P1="${P1}ated"
P2="for"; P2="${P2} now"
P3="TODO"; P3="${P3} implement"
P4="place"; P4="${P4}holder"
BLUFF_RE="${P1}\\|${P2}\\|${P3}\\|${P4}"
if grep -rn "$BLUFF_RE" \
    internal/tools/browser \
    internal/tools/browser_navigate_v2.go \
    internal/tools/browser_snapshot_v2.go \
    internal/tools/browser_click_type_v2.go \
    internal/tools/browser_screenshot_v2.go \
    internal/tools/browser_close_v2.go \
    internal/tools/browser_register_v2.go \
    internal/commands/browser_command.go \
    tests/integration/cmd/p2f23_challenge/main.go \
    tests/integration/cmd/p2f23_challenge/eval.go \
    "$HERE/CHALLENGE.md" \
    "$HERE/run.sh"; then
    echo "BLUFF FOUND" >&2
    exit 1
fi
echo "clean"

echo "==> cross-compile linux"
GOOS=linux GOARCH=amd64 go build -o /tmp/p2f23_challenge_linux ./tests/integration/cmd/p2f23_challenge

echo "==> P2-F23 challenge PASS"
