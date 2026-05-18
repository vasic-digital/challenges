#!/usr/bin/env bash
# challenges_functionality_challenge.sh - Validates Challenges module core functionality and structure
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MODULE_NAME="Challenges"

# Phase 23.0 — anti-bluff compliance promotion (Constitution §11.2.5 + §11.4).
SCRIPT_DIR_AB="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_AB="$SCRIPT_DIR_AB/../../lib/anti_bluff.sh"
[ -f "$LIB_AB" ] || LIB_AB="$SCRIPT_DIR_AB/../../../challenges/lib/anti_bluff.sh"
. "$LIB_AB"
ab_init "challenges_functionality_challenge" "/tmp/challenges_functionality_challenge.results"
ab_send_action "challenges_functionality_challenge.sh - Validates Challenges module core functionality and structure"
PASS=0
FAIL=0
TOTAL=0

pass() { ab_pass "$1"; PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { ab_fail "$1"; FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

echo "=== ${MODULE_NAME} Functionality Challenge ==="
echo ""

# Test 1: Required packages exist
echo "Test: Required packages exist"
pkgs_ok=true
for pkg in assertion challenge registry runner report; do
    if [ ! -d "${MODULE_DIR}/pkg/${pkg}" ]; then
        fail "Missing package: pkg/${pkg}"
        pkgs_ok=false
    fi
done
if [ "$pkgs_ok" = true ]; then
    pass "All required packages present (assertion, challenge, registry, runner, report)"
fi

# Test 2: Engine interface is defined
echo "Test: Engine interface is defined"
if grep -rq "type Engine interface\|type AssertionEngine interface" "${MODULE_DIR}/pkg/assertion/"; then
    pass "Engine interface is defined in pkg/assertion"
else
    fail "Engine interface not found in pkg/assertion"
fi

# Test 3: Challenge interface or Definition exists
echo "Test: Challenge definition exists"
if grep -rq "type Challenge interface\|type Definition struct" "${MODULE_DIR}/pkg/challenge/"; then
    pass "Challenge definition found in pkg/challenge"
else
    fail "Challenge definition not found in pkg/challenge"
fi

# Test 4: Evaluator function type exists
echo "Test: Evaluator function type exists"
if grep -rq "type Evaluator\|Evaluator" "${MODULE_DIR}/pkg/assertion/"; then
    pass "Evaluator type found in pkg/assertion"
else
    fail "Evaluator type not found"
fi

# Test 5: Runner implementation exists
echo "Test: Runner implementation exists"
if grep -rq "type\s\+\w*Runner\w*\s\+struct\|Run\|Execute" "${MODULE_DIR}/pkg/runner/"; then
    pass "Runner implementation found in pkg/runner"
else
    fail "No runner implementation found"
fi

# Test 6: Registry implementation exists
echo "Test: Registry implementation exists"
if grep -rq "type\s\+\w*Registry\w*\s\+struct\|Register\|Lookup" "${MODULE_DIR}/pkg/registry/"; then
    pass "Registry implementation found in pkg/registry"
else
    fail "No registry implementation found"
fi

# Test 7: Report generation support
echo "Test: Report generation support exists"
if grep -rq "Report\|Generate\|Summary\|Result" "${MODULE_DIR}/pkg/report/"; then
    pass "Report generation support found in pkg/report"
else
    fail "No report generation support found"
fi

# Test 8: Metrics/monitoring support
echo "Test: Metrics package exists"
if [ -d "${MODULE_DIR}/pkg/metrics" ] || [ -d "${MODULE_DIR}/pkg/monitor" ]; then
    pass "Metrics/monitoring package found"
else
    fail "No metrics/monitoring package found"
fi

# Test 9: Plugin system support
echo "Test: Plugin system support exists"
if [ -d "${MODULE_DIR}/pkg/plugin" ] && [ "$(find "${MODULE_DIR}/pkg/plugin" -name "*.go" ! -name "*_test.go" | wc -l)" -gt 0 ]; then
    pass "Plugin system support found in pkg/plugin"
else
    fail "No plugin system support found"
fi

# Test 10: Userflow testing support
echo "Test: Userflow testing support exists"
if [ -d "${MODULE_DIR}/pkg/userflow" ] && [ "$(find "${MODULE_DIR}/pkg/userflow" -name "*.go" ! -name "*_test.go" | wc -l)" -gt 0 ]; then
    pass "Userflow testing support found in pkg/userflow"
else
    fail "No userflow testing support found"
fi

# Test 11: Bank (assertion bank) support
echo "Test: Assertion bank support exists"
if [ -d "${MODULE_DIR}/pkg/bank" ] && grep -rq "type Bank struct\|Bank" "${MODULE_DIR}/pkg/bank/"; then
    pass "Assertion bank support found"
else
    fail "No assertion bank support found"
fi

# Test 12: Panoptic support
echo "Test: Panoptic support exists"
if [ -d "${MODULE_DIR}/pkg/panoptic" ]; then
    pass "Panoptic package found"
else
    fail "Panoptic package not found"
fi

echo ""
echo "=== Results: ${PASS}/${TOTAL} passed, ${FAIL} failed ==="
[ "${FAIL}" -eq 0 ] && exit 0 || exit 1
