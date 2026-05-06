#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT/HelixCode"

echo "==> build F15 challenge harness"
HARNESS_BIN="$(mktemp -d)/p1f15_challenge"
go build -o "$HARNESS_BIN" ./tests/integration/cmd/p1f15_challenge

echo "==> run harness"
"$HARNESS_BIN"

echo "==> anti-bluff smoke on F15-affected code"
# Build the bluff regex from parts so this script does not itself match it.
P1="simul"; P1="${P1}ated"
P2="for"; P2="${P2} now"
P3="TODO"; P3="${P3} implement"
P4="place"; P4="${P4}holder"
BLUFF_RE="${P1}\\|${P2}\\|${P3}\\|${P4}"
if grep -rn "$BLUFF_RE" \
    tests/integration/cmd/p1f15_challenge/main.go \
    "$HERE/CHALLENGE.md" \
    "$HERE/run.sh"; then
    echo "BLUFF FOUND" >&2
    exit 1
fi
echo "clean"

echo "==> cross-compile linux"
GOOS=linux GOARCH=amd64 go build -o /tmp/p1f15_challenge_linux ./tests/integration/cmd/p1f15_challenge

echo "==> P1-F15 challenge PASS"
