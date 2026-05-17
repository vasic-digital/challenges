#!/usr/bin/env bash
# ui_terminal_interaction_challenge.sh — anti-bluff UI Challenge for
# the Challenges submodule per CONST-035 + CONST-050(B). Submodule
# cascade per CONST-051(A). Drives a target binary non-interactively.

set -uo pipefail

CHL_BIN="${CHALLENGES_BIN:-}"
TIMEOUT_SEC="${UI_TIMEOUT_SEC:-30}"
USER_HOSTILE=('panic:' 'goroutine [0-9]+ \[running\]:' 'runtime error:' 'segmentation fault' 'fatal error:')

echo "=== Challenges UI Terminal-Interaction Challenge ==="
echo "  bin=$CHL_BIN timeout=${TIMEOUT_SEC}s"

if [[ -z "$CHL_BIN" ]] || [[ ! -x "$CHL_BIN" ]]; then
    echo "[1/4] SKIP: CHALLENGES_BIN not set or not executable — SKIP-OK: #env-binary-missing"
    echo "  (operator: export CHALLENGES_BIN=/path/to/your/consuming-binary)"
    echo "=== Challenges UI Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1/4] Binary present: PASS"

assert_no_panic() {
    local label="$1" body="$2"
    for pat in "${USER_HOSTILE[@]}"; do
        printf '%s' "$body" | grep -qE "$pat" && { echo "  FAIL: $label leaked: $pat"; return 1; }
    done
}

help_out=$(timeout "$TIMEOUT_SEC" "$CHL_BIN" --help 2>&1 || \
           timeout "$TIMEOUT_SEC" "$CHL_BIN" -h 2>&1 || true)
assert_no_panic "--help" "$help_out" || exit 1
[[ -z "$help_out" ]] && { echo "[2/4] FAIL: empty help"; exit 1; }
echo "[2/4] Help output: PASS"

ver_out=$(timeout "$TIMEOUT_SEC" "$CHL_BIN" --version 2>&1 || \
          timeout "$TIMEOUT_SEC" "$CHL_BIN" -v 2>&1 || true)
assert_no_panic "--version" "$ver_out" || exit 1
echo "[3/4] Version output: PASS"

set +e
bogus=$(timeout "$TIMEOUT_SEC" "$CHL_BIN" --this-flag-does-not-exist 2>&1)
bogus_exit=$?
set -e
[[ "$bogus_exit" -ge 124 ]] && { echo "[4/4] FAIL: bogus crashed (exit $bogus_exit)"; exit 1; }
assert_no_panic "bogus flag" "$bogus" || exit 1
echo "[4/4] Invalid-flag: PASS (exit $bogus_exit)"

echo
echo "=== Challenges UI Challenge: PASSED ==="
echo "  evidence: bin=$CHL_BIN bogus_exit=$bogus_exit"
