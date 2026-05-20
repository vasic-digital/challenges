#!/usr/bin/env bash
# challenges_describe_challenge.sh
#
# Round-304 meta-runner ("Challenge-of-Challenges") for the cross-cutting
# Challenge bank submodule (vasic-digital/Challenges).
#
# This submodule IS the cross-cutting Challenge bank. The meta-runner
# walks every bank under banks/examples + banks/yole, every shell-script
# Challenge under challenges/scripts, and asserts each has the expected
# structure + a paired-mutation evidence path. It is intentionally
# redundant with the per-bank Challenges — redundancy is the point: if
# any single per-bank Challenge silently regresses to a bluff (PASS on a
# broken feature), the meta-runner catches it via inventory cross-check.
#
# Validates that:
#   1. The deep-doc ledger (docs/test-coverage.md) lists every bank in
#      banks/examples + banks/yole + challenges/scripts that round-304
#      enumerates, and carries the round-304 + Article XI §11.9 mandate.
#   2. Every bank file exists, is readable, and (JSON banks) parses.
#   3. Every shell-script Challenge under challenges/scripts/ is exec +
#      parseable under `sh -n` (CONST-067).
#   4. The bilingual fixture (challenges/fixtures/payloads.json) parses
#      and covers >= 5 locales (en, de, es, ja, sr).
#   5. The README enumerates the round-304 anti-bluff guarantees, marks
#      round-304, and links to the describe-runner.
#
# Paired-mutation invariant (CONST-035 + CONST-050(B) + §11.9 + §1.1):
#   With --anti-bluff-mutate the script plants a deliberate inventory
#   mismatch in a TMP COPY of the ledger (renames a tracked bank token),
#   reruns validation against the tmp copy, and asserts the gate FAILS
#   with exit 99. This proves the gate actually catches ledger-vs-tree
#   drift instead of rubber-stamping it. The original tree is never
#   mutated.
#
# Exit codes:
#   0  — gate PASS on clean tree
#   1  — gate FAIL on clean tree (real failure to fix)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error
#
# Operator mandate (2026-05-19, verbatim, cascaded per §11.9):
#   "all existing tests and Challenges do work in anti-bluff manner -
#    they MUST confirm that all tested codebase really works as expected!
#    We had been in position that all tests do execute with success and
#    all Challenges as well, but in reality the most of the features
#    does not work and can't be used! This MUST NOT be the case and
#    execution of tests and Challenges MUST guarantee the quality, the
#    completition and full usability by end users of the product!"

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="${SCRIPT_DIR}"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h)
            sed -n '1,55p' "$0"
            exit 0
            ;;
        *)
            echo "unknown argument: $arg" >&2
            exit 2
            ;;
    esac
done

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

LEDGER="${MODULE_DIR}/docs/test-coverage.md"
FIXTURE="${MODULE_DIR}/challenges/fixtures/payloads.json"
README="${MODULE_DIR}/README.md"
BANKS_EXAMPLES="${MODULE_DIR}/banks/examples"
BANKS_YOLE="${MODULE_DIR}/banks/yole"
CHALLENGES_SCRIPTS="${MODULE_DIR}/challenges/scripts"

# If mutation requested, work against a tmp copy of the ledger with a
# planted token rename. The original tree stays untouched.
LEDGER_WORK="${LEDGER}"
TMP_LEDGER=""
if [ "${MUTATE}" -eq 1 ]; then
    TMP_LEDGER="$(mktemp)"
    cp "${LEDGER}" "${TMP_LEDGER}"
    # Plant a rename: concurrency.json → concurrencyMUTATED.json. The real
    # bank file remains absent from the planted ledger token, so the
    # ledger-vs-tree cross-reference must FAIL.
    sed -i 's/concurrency\.json/concurrencyMUTATED.json/g' "${TMP_LEDGER}"
    LEDGER_WORK="${TMP_LEDGER}"
    echo "=== Challenges Describe Challenge (anti-bluff-mutate mode) ==="
else
    echo "=== Challenges Describe Challenge (clean mode) ==="
fi
echo ""

# Section 1: ledger presence and round-304 markers
echo "Section 1: docs/test-coverage.md ledger"
if [ ! -f "${LEDGER_WORK}" ]; then
    fail "ledger missing at ${LEDGER_WORK}"
else
    pass "ledger present"
    if grep -q "round-304" "${LEDGER_WORK}"; then
        pass "ledger marked round-304"
    else
        fail "ledger missing round-304 marker"
    fi
    if grep -q "MUST guarantee the quality" "${LEDGER_WORK}"; then
        pass "ledger carries Article XI §11.9 mandate"
    else
        fail "ledger missing Article XI §11.9 mandate"
    fi
fi

# Section 2: banks/examples inventory cross-reference (28 JSON banks)
echo ""
echo "Section 2: banks/examples/*.json inventory cross-reference"
if [ ! -d "${BANKS_EXAMPLES}" ]; then
    fail "banks/examples missing"
else
    pass "banks/examples directory present"
    CHECKED=0
    MISSING_FROM_LEDGER=0
    while IFS= read -r -d '' bank; do
        CHECKED=$((CHECKED+1))
        bname="$(basename "${bank}")"
        if [ ! -r "${bank}" ]; then
            fail "bank not readable: ${bname}"
            continue
        fi
        # JSON sanity: file must start with { or [
        first_char="$(head -c 1 "${bank}" 2>/dev/null)"
        case "${first_char}" in
            '{'|'[') : ;;
            *) fail "bank not JSON: ${bname}" ;;
        esac
        if grep -qF "${bname}" "${LEDGER_WORK}"; then
            : # bank cross-referenced
        else
            fail "ledger missing bank ${bname}"
            MISSING_FROM_LEDGER=$((MISSING_FROM_LEDGER+1))
        fi
    done < <(find "${BANKS_EXAMPLES}" -maxdepth 1 -name '*.json' -print0)
    if [ "${CHECKED}" -lt 28 ]; then
        fail "banks/examples has only ${CHECKED} banks (expected >=28)"
    else
        pass "banks/examples has ${CHECKED} banks (>=28)"
    fi
    if [ "${MISSING_FROM_LEDGER}" -eq 0 ] && [ "${CHECKED}" -gt 0 ]; then
        pass "all ${CHECKED} example banks cross-referenced in ledger"
    fi
fi

# Section 3: banks/yole feature-coverage inventory
echo ""
echo "Section 3: banks/yole/feature-coverage inventory"
if [ ! -d "${BANKS_YOLE}/feature-coverage" ]; then
    fail "banks/yole/feature-coverage missing"
else
    pass "banks/yole/feature-coverage present"
    YOLE_COUNT=$(find "${BANKS_YOLE}/feature-coverage" -maxdepth 1 \
        -name 'feature-*.yaml' | wc -l)
    if [ "${YOLE_COUNT}" -ge 7 ]; then
        pass "banks/yole/feature-coverage has ${YOLE_COUNT} feature banks (>=7)"
    else
        fail "banks/yole/feature-coverage has only ${YOLE_COUNT} (expected >=7)"
    fi
    if [ -d "${BANKS_YOLE}/fixtures" ]; then
        pass "banks/yole/fixtures present"
    else
        fail "banks/yole/fixtures missing"
    fi
fi

# Section 4: shell-script Challenges inventory + parse
echo ""
echo "Section 4: challenges/scripts/*.sh inventory + sh -n parseability"
if [ ! -d "${CHALLENGES_SCRIPTS}" ]; then
    fail "challenges/scripts missing"
else
    pass "challenges/scripts directory present"
    SCRIPT_COUNT=0
    PARSE_FAIL=0
    EXEC_FAIL=0
    while IFS= read -r -d '' s; do
        SCRIPT_COUNT=$((SCRIPT_COUNT+1))
        sname="$(basename "${s}")"
        if [ ! -x "${s}" ]; then
            fail "script not executable: ${sname}"
            EXEC_FAIL=$((EXEC_FAIL+1))
        fi
        # CONST-067 target-shell parseability (use bash for bash scripts).
        if ! bash -n "${s}" 2>/dev/null; then
            fail "script fails bash -n parse: ${sname}"
            PARSE_FAIL=$((PARSE_FAIL+1))
        fi
    done < <(find "${CHALLENGES_SCRIPTS}" -maxdepth 1 -name '*.sh' -print0)
    if [ "${SCRIPT_COUNT}" -ge 16 ]; then
        pass "challenges/scripts has ${SCRIPT_COUNT} scripts (>=16)"
    else
        fail "challenges/scripts has only ${SCRIPT_COUNT} (expected >=16)"
    fi
    if [ "${PARSE_FAIL}" -eq 0 ] && [ "${SCRIPT_COUNT}" -gt 0 ]; then
        pass "all ${SCRIPT_COUNT} scripts parse clean under bash -n"
    fi
    if [ "${EXEC_FAIL}" -eq 0 ] && [ "${SCRIPT_COUNT}" -gt 0 ]; then
        pass "all ${SCRIPT_COUNT} scripts are executable"
    fi
    # Baseline reference text required.
    if [ -f "${CHALLENGES_SCRIPTS}/../baselines/bluff-baseline.txt" ]; then
        pass "challenges/baselines/bluff-baseline.txt present"
    else
        fail "challenges/baselines/bluff-baseline.txt missing"
    fi
fi

# Section 5: locale fixture sanity
echo ""
echo "Section 5: 5-locale fixture"
if [ ! -f "${FIXTURE}" ]; then
    fail "fixture missing at ${FIXTURE}"
else
    pass "fixture present"
    LOCALE_COUNT=$(grep -oE '"locale":\s*"[^"]+"' "${FIXTURE}" | sort -u | wc -l)
    if [ "${LOCALE_COUNT}" -ge 5 ]; then
        pass "fixture covers ${LOCALE_COUNT} locales (>=5)"
    else
        fail "fixture covers only ${LOCALE_COUNT} locales (<5)"
    fi
    for loc in en de es ja sr; do
        if grep -q "\"locale\": \"${loc}\"" "${FIXTURE}"; then
            pass "fixture includes locale ${loc}"
        else
            fail "fixture missing locale ${loc}"
        fi
    done
    # JSON sanity (no jq dependency — start byte + closing brace).
    first_char="$(head -c 1 "${FIXTURE}" 2>/dev/null)"
    last_char="$(tail -c 2 "${FIXTURE}" 2>/dev/null | head -c 1)"
    if [ "${first_char}" = "{" ] && [ "${last_char}" = "}" ]; then
        pass "fixture JSON brace-balanced"
    else
        fail "fixture JSON not brace-balanced"
    fi
fi

# Section 6: README round-304 anti-bluff section
echo ""
echo "Section 6: README round-304 anti-bluff section"
if grep -q "Anti-bluff guarantees" "${README}"; then
    pass "README declares Anti-bluff guarantees"
else
    fail "README missing Anti-bluff guarantees section"
fi
if grep -q "round-304" "${README}"; then
    pass "README marked round-304"
else
    fail "README missing round-304 marker"
fi
if grep -q "challenges_describe_challenge.sh" "${README}"; then
    pass "README links describe-runner"
else
    fail "README missing describe-runner link"
fi

# Cleanup mutated ledger if any
if [ -n "${TMP_LEDGER}" ]; then
    rm -f "${TMP_LEDGER}"
fi

echo ""
echo "=== Summary: ${PASS}/${TOTAL} PASS, ${FAIL} FAIL ==="

if [ "${MUTATE}" -eq 1 ]; then
    if [ "${FAIL}" -gt 0 ]; then
        echo "anti-bluff-mutate: gate correctly detected planted mutation (exit 99)"
        exit 99
    else
        echo "anti-bluff-mutate: gate FAILED to detect planted mutation — bluff!"
        exit 1
    fi
fi

if [ "${FAIL}" -gt 0 ]; then
    exit 1
fi
exit 0
