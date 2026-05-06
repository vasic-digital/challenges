#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT/HelixCode"

echo "==> build P1.5 foundation-cleanup challenge harness"
HARNESS_BIN="$(mktemp -d)/p1_5_challenge"
go build -o "$HARNESS_BIN" ./tests/integration/cmd/p1_5_challenge

echo "==> run harness"
"$HARNESS_BIN"

echo "==> anti-bluff smoke on P1.5 harness"
# Build the bluff regex from parts so this script does not itself match it.
P1="simul"; P1="${P1}ated"
P2="for"; P2="${P2} now"
P3="TODO"; P3="${P3} implement"
P4="place"; P4="${P4}holder"
BLUFF_RE="${P1}\\|${P2}\\|${P3}\\|${P4}"
if grep -rn "$BLUFF_RE" \
    tests/integration/cmd/p1_5_challenge/main.go \
    "$HERE/CHALLENGE.md" \
    "$HERE/run.sh"; then
    echo "BLUFF FOUND" >&2
    exit 1
fi
echo "clean"

echo "==> cross-compile linux"
GOOS=linux GOARCH=amd64 go build -o /tmp/p1_5_challenge_linux ./tests/integration/cmd/p1_5_challenge

echo "==> P1.5 foundation-cleanup challenge PASS"
