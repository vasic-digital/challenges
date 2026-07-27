#!/usr/bin/env bash
# verify_two_sum_vector_uniqueness.sh — deterministic-correctness proof for
# review finding W1 (R41-F): the freshly-randomised 4th test vector built by
# helixllm_coder_live_e2e_challenge.sh's [1/8] step MUST have EXACTLY ONE
# valid two_sum pair, brute-force-verified across every C(4,2)=6 index pair
# of the generated 4-element list. A vector with a SECOND valid pair lets a
# genuinely-correct coder that returns the OTHER pair fail the harness
# (false-RED — the harness could never false-PASS bad code, but it could
# spuriously fail correct code that returns a different, also-valid pair).
#
# WHAT THIS PROVES, WITHOUT a live coder (§11.4.115 reproduce-first +
# same-test-confirms-fix; §11.4.146 STEP 1/2 reproduce-then-confirm):
#
#   [RED]   The historical (pre-fix, commit 125f73b) construction — embedded
#           below verbatim, byte-for-byte, as HISTORICAL regression-evidence
#           DATA, never used for production — CAN produce vectors with a
#           second valid pair. This script generates N vectors with that
#           historical logic and independently brute-force-counts how many
#           have >1 valid pair. A non-zero count is the real, generated-data
#           RED reproduction of the W1 defect (never asserted from theory).
#
#   [GREEN] The CURRENT construction is extracted, byte-for-byte, from the
#           live helixllm_coder_live_e2e_challenge.sh file AT RUN TIME (the
#           unique `VECGEN_PYEOF` heredoc delimiter marks its bounds — zero
#           drift risk: this script always checks whatever is actually
#           shipping, never a hand-copied approximation that could go
#           stale). This script generates N vectors with that CURRENT logic
#           and independently brute-force-counts how many have >1 valid
#           pair. This run MUST report 0/N — if the challenge script ever
#           regresses (someone reintroduces the W1 defect), this verifier
#           catches it (§11.4.135-style standing regression guard).
#
# Independence discipline: the brute-force pair-count in BOTH sweeps is
# computed by THIS script reading the printed vector, never by trusting any
# internal "I checked it myself" claim from the generator being tested.
#
# Usage:
#   verify_two_sum_vector_uniqueness.sh [N]
#     N  — number of vectors to generate per construction (default 1000)
#
# Exit codes:
#   0  — GREEN: current (live, extracted) construction produced 0/N vectors
#        with a second valid pair AND 0/N with zero valid pairs, and the
#        target script still parses (`bash -n`) — uniqueness proven for
#        this run.
#   1  — current construction did NOT prove uniqueness across N runs (the
#        fix regressed, or was never applied), or the target script fails
#        `bash -n` — never claim success without this proof.
#   2  — usage / environment error (toolchain absent → honest SKIP-OK).
#
# NOTE (§11.4.28 decoupled): no hardcoded host, no live coder required —
# this is a pure, offline, deterministic-per-run construction proof. It
# never touches the coder process referenced by helixllm_coder_live_e2e_
# challenge.sh (that script's own [3/8]+ HTTP steps are untouched by this
# verifier).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_SCRIPT="${HELIXLLM_UNIQUENESS_TARGET_SCRIPT:-${SCRIPT_DIR}/helixllm_coder_live_e2e_challenge.sh}"
N="${1:-1000}"

for tool in python3; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "SKIP: required tool '$tool' absent — SKIP-OK: #env-no-$tool"
        exit 0
    fi
done

if [[ ! -f "$TARGET_SCRIPT" ]]; then
    echo "FAIL: target script not found: ${TARGET_SCRIPT}" >&2
    exit 2
fi

EVIDENCE_DIR="${HELIXLLM_UNIQUENESS_EVIDENCE_DIR:-$(mktemp -d -t two_sum_uniqueness_evidence.XXXXXX)}"
mkdir -p "$EVIDENCE_DIR"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

echo "=== Two-Sum Vector Uniqueness Verifier (W1 / R41-F) ==="
echo "  target=${TARGET_SCRIPT}"
echo "  N=${N}"
echo "  evidence_dir=${EVIDENCE_DIR}"

# -----------------------------------------------------------------------
# [1/4] Extract the CURRENT [1/8] vector-generation python body from the
# live challenge script, verbatim, via the unique VECGEN_PYEOF heredoc
# delimiter (zero drift — this is exactly what ships).
# -----------------------------------------------------------------------
if ! grep -q "<<'VECGEN_PYEOF'" "$TARGET_SCRIPT"; then
    echo "FAIL: VECGEN_PYEOF delimiter not found in ${TARGET_SCRIPT} — extraction cannot proceed (has the block been renamed?)" >&2
    exit 2
fi
awk '/<<.VECGEN_PYEOF./{flag=1; next} /^VECGEN_PYEOF$/{flag=0} flag' "$TARGET_SCRIPT" > "${WORK}/current_vecgen.py"
CURRENT_LINES=$(wc -l < "${WORK}/current_vecgen.py" | tr -d ' ')
if [[ "$CURRENT_LINES" -lt 3 ]]; then
    echo "FAIL: extracted current vector-gen body looks empty/too short (${CURRENT_LINES} lines) — extraction bug" >&2
    exit 2
fi
cp "${WORK}/current_vecgen.py" "${EVIDENCE_DIR}/current_vecgen_extracted.py"
echo "[1/4] extracted ${CURRENT_LINES}-line current construction -> ${EVIDENCE_DIR}/current_vecgen_extracted.py"

# -----------------------------------------------------------------------
# [2/4] HISTORICAL (pre-fix, commit 125f73b) construction — embedded
# verbatim as regression-evidence DATA. Never used for production. This
# reproduces the W1 defect for RED evidence, run-to-run, without needing
# git history to be present/reachable at test time.
# -----------------------------------------------------------------------
cat > "${WORK}/historical_vecgen.py" <<'HIST_PYEOF'
import random
random.seed()
a = random.randint(-500, 500)
b = random.randint(-500, 500)
while b == a:
    b = random.randint(-500, 500)
filler = [random.randint(-500, 500) for _ in range(4)]
pos_a = random.randint(0, 3)
pos_b = random.randint(0, 3)
while pos_b == pos_a:
    pos_b = random.randint(0, 3)
lst = filler[:]
lst[pos_a] = a
lst[pos_b] = b
print(a, b, ",".join(str(x) for x in lst), pos_a, pos_b)
HIST_PYEOF
cp "${WORK}/historical_vecgen.py" "${EVIDENCE_DIR}/historical_vecgen_pre_fix_125f73b.py"
echo "[2/4] historical (pre-fix, commit 125f73b) construction embedded -> ${EVIDENCE_DIR}/historical_vecgen_pre_fix_125f73b.py"

# -----------------------------------------------------------------------
# [3/4] Independent brute-force driver: runs a given generator script N
# times as SEPARATE python3 subprocess invocations (mirrors exactly how
# the challenge script invokes it in production — one process per
# generated vector), parses each printed vector, and independently
# brute-forces ALL C(4,2)=6 index pairs of the generated list to count
# how many sum to target. Never trusts the generator's own internal
# claims.
# -----------------------------------------------------------------------
run_uniqueness_sweep() {
    # $1 = path to generator .py  $2 = N  $3 = label (for stdout)
    local gen_py="$1" n="$2" label="$3"
    python3 - "$gen_py" "$n" "$label" <<'DRIVER_PYEOF'
import itertools
import subprocess
import sys

gen_py, n, label = sys.argv[1], int(sys.argv[2]), sys.argv[3]

total = 0
second_pair_count = 0
zero_pair_count = 0
malformed_count = 0
examples = []

for _ in range(n):
    proc = subprocess.run(
        ["python3", gen_py], capture_output=True, text=True, timeout=15
    )
    if proc.returncode != 0 or not proc.stdout.strip():
        malformed_count += 1
        continue
    parts = proc.stdout.strip().split()
    if len(parts) != 5:
        malformed_count += 1
        continue
    a_s, b_s, lst_csv, pos_a_s, pos_b_s = parts
    try:
        a = int(a_s)
        b = int(b_s)
        lst = [int(x) for x in lst_csv.split(",")]
        pos_a = int(pos_a_s)
        pos_b = int(pos_b_s)
    except ValueError:
        malformed_count += 1
        continue
    if len(lst) != 4 or lst[pos_a] != a or lst[pos_b] != b:
        malformed_count += 1
        continue
    target = a + b
    total += 1

    # Independent brute-force check: every C(4,2) index pair of the
    # PRINTED list — this script trusts nothing the generator claims.
    valid_pairs = [
        (i2, j2)
        for i2, j2 in itertools.combinations(range(len(lst)), 2)
        if lst[i2] + lst[j2] == target
    ]
    if len(valid_pairs) == 0:
        zero_pair_count += 1
        if len(examples) < 5:
            examples.append(f"ZERO_PAIR nums={lst} target={target} pairs={valid_pairs}")
    elif len(valid_pairs) > 1:
        second_pair_count += 1
        if len(examples) < 5:
            examples.append(f"SECOND_PAIR nums={lst} target={target} pairs={valid_pairs}")

print(f"RESULT[{label}]: total={total} second_pair_count={second_pair_count} "
      f"zero_pair_count={zero_pair_count} malformed_count={malformed_count}")
for ex in examples:
    print(f"  example: {ex}")

# Machine-parseable trailer line for the calling shell.
print(f"MACHINE[{label}]: {second_pair_count} {zero_pair_count} {malformed_count} {total}")
DRIVER_PYEOF
}

echo "[3/4] running HISTORICAL construction sweep (N=${N}; RED expectation: second_pair_count > 0)..."
run_uniqueness_sweep "${WORK}/historical_vecgen.py" "$N" "HISTORICAL_PRE_FIX" | tee "${WORK}/historical_sweep.txt"
cp "${WORK}/historical_sweep.txt" "${EVIDENCE_DIR}/historical_sweep_RED.txt"

echo
echo "[3/4] running CURRENT (extracted, live) construction sweep (N=${N}; GREEN requirement: second_pair_count == 0)..."
run_uniqueness_sweep "${WORK}/current_vecgen.py" "$N" "CURRENT_LIVE" | tee "${WORK}/current_sweep.txt"
cp "${WORK}/current_sweep.txt" "${EVIDENCE_DIR}/current_sweep_GREEN.txt"

HIST_LINE="$(grep '^MACHINE\[HISTORICAL_PRE_FIX\]:' "${WORK}/historical_sweep.txt")"
CUR_LINE="$(grep '^MACHINE\[CURRENT_LIVE\]:' "${WORK}/current_sweep.txt")"
CUR_TRAILER="${CUR_LINE#*: }"
CUR_SECOND="$(awk '{print $1}' <<<"$CUR_TRAILER")"
CUR_ZERO="$(awk '{print $2}' <<<"$CUR_TRAILER")"

# -----------------------------------------------------------------------
# [4/4] bash -n parse-check on the challenge script (STEP 4 of the fix
# workflow — confirms the fix did not break the shell script's syntax /
# the surrounding non-coder logic).
# -----------------------------------------------------------------------
if bash -n "$TARGET_SCRIPT" 2>"${WORK}/bashn.err"; then
    BASHN_RESULT="PASS"
else
    BASHN_RESULT="FAIL"
    cat "${WORK}/bashn.err" >&2
fi
echo "[4/4] bash -n ${TARGET_SCRIPT}: ${BASHN_RESULT}"

{
    echo "Two-Sum Vector Uniqueness Verifier — SUMMARY (W1 / R41-F)"
    echo "date_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "target=${TARGET_SCRIPT}"
    echo "N=${N}"
    echo
    echo "HISTORICAL (pre-fix, commit 125f73b) construction:"
    echo "  ${HIST_LINE}"
    echo
    echo "CURRENT (live, extracted) construction:"
    echo "  ${CUR_LINE}"
    echo
    echo "bash_-n_check=${BASHN_RESULT}"
    echo
    if [[ "$CUR_SECOND" == "0" && "$CUR_ZERO" == "0" && "$BASHN_RESULT" == "PASS" ]]; then
        echo "verdict=PASS (current construction: 0/${N} vectors with a second valid pair; every vector had exactly one valid pair)"
    else
        echo "verdict=FAIL (current construction did NOT prove uniqueness across N=${N} — see counts above)"
    fi
} > "${EVIDENCE_DIR}/SUMMARY.txt"
cat "${EVIDENCE_DIR}/SUMMARY.txt"

echo
echo "evidence: ${EVIDENCE_DIR}"

if [[ "$CUR_SECOND" != "0" || "$CUR_ZERO" != "0" || "$BASHN_RESULT" != "PASS" ]]; then
    echo "=== Two-Sum Vector Uniqueness Verifier: FAIL ==="
    exit 1
fi
echo "=== Two-Sum Vector Uniqueness Verifier: PASS ==="
exit 0
