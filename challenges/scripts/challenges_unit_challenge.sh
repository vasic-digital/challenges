#!/usr/bin/env bash
# challenges_unit_challenge.sh - Validates Challenges module unit tests
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Challenges"

# Phase 23.0 — anti-bluff compliance promotion (Constitution §11.2.5 + §11.4).
SCRIPT_DIR_AB="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_AB="$SCRIPT_DIR_AB/../../lib/anti_bluff.sh"
[ -f "$LIB_AB" ] || LIB_AB="$SCRIPT_DIR_AB/../../../challenges/lib/anti_bluff.sh"
. "$LIB_AB"
ab_init "challenges_unit_challenge" "/tmp/challenges_unit_challenge.results"
ab_send_action "challenges_unit_challenge.sh - Validates Challenges module unit tests"
PASS=0
FAIL=0
TOTAL=0

pass() { ab_pass "$1"; PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { ab_fail "$1"; FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Unit Test Challenge ==="
echo ""

# Test 1: Test files exist
echo "Test: Test files exist"
test_count=$(find "${MODULE_DIR}" -name "*_test.go" | wc -l)
if [ "${test_count}" -gt 0 ]; then
    pass "Found ${test_count} test files"
else
    fail "No test files found"
fi

# Test 2: Tests exist in each package
echo "Test: Test coverage across packages"
pkgs_with_tests=0
for pkg_dir in "${MODULE_DIR}"/pkg/*/; do
    pkg_name=$(basename "$pkg_dir")
    pkg_tests=$(find "$pkg_dir" -name "*_test.go" | wc -l)
    if [ "$pkg_tests" -gt 0 ]; then
        pkgs_with_tests=$((pkgs_with_tests + 1))
    fi
done
if [ "$pkgs_with_tests" -ge 5 ]; then
    pass "At least 5 packages have tests (found ${pkgs_with_tests})"
else
    fail "Only ${pkgs_with_tests} packages have tests (expected at least 5)"
fi

# Test 3: Unit tests pass
echo "Test: Unit tests pass"
if (cd "${MODULE_DIR}" && GOMAXPROCS=2 nice -n 19 go test -short -count=1 -p 1 ./... 2>&1); then
    pass "Unit tests pass"
else
    fail "Unit tests failed"
fi

# Test 4: No race conditions (short mode)
echo "Test: Race detector clean"
if (cd "${MODULE_DIR}" && GOMAXPROCS=2 nice -n 19 go test -short -race -count=1 -p 1 ./... 2>&1); then
    pass "No race conditions detected"
else
    fail "Race conditions detected"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
