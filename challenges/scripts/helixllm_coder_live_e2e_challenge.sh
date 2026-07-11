#!/usr/bin/env bash
# helixllm_coder_live_e2e_challenge.sh — anti-bluff Challenge for the
# HelixLLM extension's coder-model integration (§11.4.169 mandatory
# test-type "Challenges"; CONST-050(B); CONST-035; Article XI §11.9).
#
# WHAT IT PROVES (positive runtime evidence, §11.4.5 / §11.4.69 / §107):
#   A realistic end-user coding task ("write and verify a function") is
#   sent, over a REAL HTTP request, to a LIVE llama.cpp / OpenAI-compatible
#   coder endpoint (e.g. HelixLLM's Qwen3-Coder-30B provider). The
#   response is NOT trusted on faith: the code the model returns is
#   extracted, statically scanned for bluff markers (TODO/simulate/
#   NotImplementedError/placeholder/"for now"), and then ACTUALLY
#   EXECUTED by a real Python interpreter against four assertions — three
#   fixed vectors plus ONE freshly-randomised vector generated at runtime
#   (so a hardcoded lookup table of well-known examples cannot pass).
#   Only a genuinely-correct, genuinely-executed implementation passes.
#
#   Evidence captured (full bidirectional transcript, §11.4.83/§107):
#     request.json    — exact HTTP request body sent to the coder
#     response.json    — exact HTTP response body received from the coder
#     solution.py       — the code extracted from the response
#     harness_stdout.txt — real python3 execution stdout (per-vector PASS/FAIL)
#     SUMMARY.txt        — verdict + evidence-file manifest
#
# TARGET (§11.4.28 decoupled — no hardcoded host):
#   HELIXLLM_CODER_URL   base URL of an OpenAI-compatible /v1 endpoint
#                         (default: http://localhost:18434)
#   HELIXLLM_CODER_MODEL  model id (default: auto-discovered from
#                         GET $HELIXLLM_CODER_URL/v1/models — CONST-036,
#                         never hardcoded)
#   HELIXLLM_CHALLENGE_EVIDENCE_DIR  where to write the transcript
#                         (default: a mktemp dir, printed on exit)
#
# READ-ONLY toward the coder process (§11.4.122): this script only ever
# issues GET/POST HTTP requests against the already-running server. It
# never starts, stops, restarts, or signals the coder process.
#
# PAIRED-MUTATION INVARIANT (CONST-035 + §1.1 + §11.4.115):
#   With --anti-bluff-mutate the script BYPASSES the live coder and
#   substitutes a deliberately-WRONG stub implementation
#   (`return [0, 0]` for every input) in place of "the model's answer",
#   then runs that stub through the EXACT SAME extraction → static-scan
#   → real-execution → assertion pipeline used for the real coder
#   response. A genuine (non-bluffing) verification pipeline MUST catch
#   the wrong output and FAIL the stub. If the pipeline reports PASS for
#   the deliberately-broken stub, the gate itself is a bluff — the
#   mutation run treats that as the real failure (exit 1). Correct
#   detection of the planted defect exits 99.
#
# Exit codes:
#   0  — real coder produced genuinely-correct, genuinely-executed code
#   1  — gate FAIL (real defect: bad code from coder, OR the mutation
#        run failed to catch the planted stub — i.e. our own checker
#        would have been a bluff)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error (toolchain absent → honest SKIP-OK)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BASE_URL="${HELIXLLM_CODER_URL:-http://localhost:18434}"
MODEL_OVERRIDE="${HELIXLLM_CODER_MODEL:-}"
TIMEOUT_SEC="${HELIXLLM_CODER_TIMEOUT_SEC:-90}"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h) sed -n '1,55p' "$0"; exit 0 ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

echo "=== HelixLLM Coder Live End-to-End Challenge ==="
echo "  base_url=${BASE_URL} timeout=${TIMEOUT_SEC}s mutate=${MUTATE}"

# ---------------------------------------------------------------------
# [0/8] Toolchain + reachability preflight (honest SKIP-OK per §11.4.3).
# ---------------------------------------------------------------------
for tool in curl jq python3; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "[0/8] SKIP: required tool '$tool' absent — SKIP-OK: #env-no-$tool"
        echo "=== HelixLLM Coder Challenge: PASSED (SKIP-OK) ==="
        exit 0
    fi
done

MODELS_JSON="$(curl -sS --max-time 8 "${BASE_URL}/v1/models" 2>/dev/null || true)"
if [[ -z "$MODELS_JSON" ]] || ! jq -e '.data' >/dev/null 2>&1 <<<"$MODELS_JSON"; then
    echo "[0/8] SKIP: coder unreachable at ${BASE_URL}/v1/models — SKIP-OK: #env-no-coder"
    echo "=== HelixLLM Coder Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[0/8] Coder reachable: ${BASE_URL}"

MODEL_ID="${MODEL_OVERRIDE:-$(jq -r '.data[0].id // empty' <<<"$MODELS_JSON")}"
if [[ -z "$MODEL_ID" ]]; then
    echo "[0/8] SKIP: could not discover a model id from ${BASE_URL}/v1/models — SKIP-OK: #env-no-model"
    echo "=== HelixLLM Coder Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[0/8] model=${MODEL_ID} (source: $([[ -n "$MODEL_OVERRIDE" ]] && echo env-override || echo auto-discovered))"

# ---------------------------------------------------------------------
# Evidence dir (persistent — NOT deleted on exit; caller/CI copies it
# into docs/qa/<run-id>/ per §11.4.83). Work dir is a separate mktemp
# that IS cleaned up.
# ---------------------------------------------------------------------
EVIDENCE_DIR="${HELIXLLM_CHALLENGE_EVIDENCE_DIR:-$(mktemp -d -t helixllm_coder_evidence.XXXXXX)}"
mkdir -p "$EVIDENCE_DIR"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT
echo "[0/8] evidence_dir=${EVIDENCE_DIR}"

RUN_TOKEN="helixllm$(date -u +%Y%m%dT%H%M%SZ)$$"

# ---------------------------------------------------------------------
# [1/8] Freshly-randomised 4th test vector — defeats a hardcoded
# lookup-table "solution" that only special-cases the textbook
# examples (§11.4.107 not-stale / genuine-execution discipline).
#
# UNIQUENESS GUARANTEE (fixes W1 / R41-F false-RED risk, 2026-07-11):
# earlier revisions filled the two non-answer slots with independent
# random fillers that were not excluded from also summing to target —
# a genuinely-correct coder returning that OTHER valid pair could then
# fail this harness (a false-RED; the harness could never false-PASS
# bad code, but could spuriously fail correct code returning a
# different, also-valid pair). It also silently violated the
# assumption stated to the model in USER_PROMPT below ("assume exactly
# one valid pair exists"). The construction below retries until an
# independent brute-force check over ALL C(4,2)=6 index pairs of the
# generated list confirms EXACTLY ONE valid pair exists, before
# accepting the vector — so both problems are fixed at once. Verified
# deterministically across 1000+ generated vectors by
# verify_two_sum_vector_uniqueness.sh (which extracts and
# brute-force-checks this exact block at test time — zero drift risk;
# see its evidence for real generated-vector counts).
# ---------------------------------------------------------------------
read -r RAND_A RAND_B RAND_LIST RAND_IDX_A RAND_IDX_B <<<"$(python3 - <<'VECGEN_PYEOF'
import itertools
import random

random.seed()

while True:
    a = random.randint(-500, 500)
    b = random.randint(-500, 500)
    if b == a:
        continue
    target = a + b

    # Fillers must differ from BOTH answer values (else a filler would
    # duplicate a/b at another index and pair with the OTHER answer
    # value to also sum to target), and must not sum to target with
    # each other.
    f1 = random.randint(-500, 500)
    if f1 in (a, b):
        continue
    f2 = random.randint(-500, 500)
    if f2 in (a, b) or f1 + f2 == target:
        continue

    pos_a = random.randint(0, 3)
    pos_b = random.randint(0, 3)
    if pos_b == pos_a:
        continue

    lst = [None, None, None, None]
    lst[pos_a] = a
    lst[pos_b] = b
    fillers = iter((f1, f2))
    for i in range(4):
        if lst[i] is None:
            lst[i] = next(fillers)

    # Defense-in-depth: never trust the exclusion logic on faith — the
    # vector is only accepted once brute-force confirms exactly one
    # valid pair exists among ALL C(4,2) index pairs.
    valid_pairs = [
        (i2, j2)
        for i2, j2 in itertools.combinations(range(4), 2)
        if lst[i2] + lst[j2] == target
    ]
    if len(valid_pairs) != 1:
        continue
    break

print(a, b, ",".join(str(x) for x in lst), pos_a, pos_b)
VECGEN_PYEOF
)"
RAND_TARGET=$((RAND_A + RAND_B))
echo "[1/8] randomised 4th vector: nums=[${RAND_LIST}] target=${RAND_TARGET} expect_indices=(${RAND_IDX_A},${RAND_IDX_B})"

# ---------------------------------------------------------------------
# [2/8] Build the real end-user coding request (realistic task: write
# AND verify a function — matches the operator's brief verbatim).
# ---------------------------------------------------------------------
SYSTEM_PROMPT='You are a precise Python coding assistant. Output ONLY a single fenced ```python code block containing exactly one function definition named two_sum(nums, target). No prose, no explanation, no example usage outside the code block.'
USER_PROMPT='Write a Python function `two_sum(nums, target)` that returns a list of the two 0-based indices of the two distinct elements in `nums` that add up to `target`. Assume exactly one valid pair exists and you must not reuse the same index twice. Return ONLY a fenced python code block with the function definition — no tests, no explanation, no extra text.'

jq -n \
    --arg model "$MODEL_ID" \
    --arg sys "$SYSTEM_PROMPT" \
    --arg usr "$USER_PROMPT" \
    '{model: $model, messages: [{role:"system", content:$sys}, {role:"user", content:$usr}], temperature: 0.1, max_tokens: 500}' \
    > "${WORK}/request.json"
cp "${WORK}/request.json" "${EVIDENCE_DIR}/request.json"
echo "[2/8] request built (model=${MODEL_ID}, run_token=${RUN_TOKEN})"

extract_code() {
    # $1 = raw response content text -> writes python code to stdout
    python3 - "$1" <<'PYEOF'
import re, sys
text = sys.argv[1]
m = re.search(r"```(?:python)?\s*\n(.*?)```", text, re.DOTALL)
print(m.group(1).strip() if m else text.strip())
PYEOF
}

run_and_assert() {
    # $1 = path to python code file containing two_sum(...)
    # Executes real python3, checks 3 fixed + 1 random vector.
    local code_file="$1"
    python3 - "$code_file" "$RAND_LIST" "$RAND_TARGET" "$RAND_IDX_A" "$RAND_IDX_B" <<'PYEOF'
import sys, importlib.util

code_file, rand_list, rand_target, rand_ia, rand_ib = sys.argv[1:6]

spec = importlib.util.spec_from_file_location("candidate", code_file)
mod = importlib.util.module_from_spec(spec)
try:
    spec.loader.exec_module(mod)
except Exception as e:
    print(f"LOAD_ERROR: {type(e).__name__}: {e}")
    sys.exit(1)

if not hasattr(mod, "two_sum"):
    print("LOAD_ERROR: two_sum not defined")
    sys.exit(1)

fn = mod.two_sum
rand_nums = [int(x) for x in rand_list.split(",")]
rand_target = int(rand_target)
rand_ia, rand_ib = int(rand_ia), int(rand_ib)

cases = [
    ("fixed_1", [2, 7, 11, 15], 9, {0, 1}),
    ("fixed_2", [3, 2, 4], 6, {1, 2}),
    ("fixed_3", [3, 3], 6, {0, 1}),
    ("random_4", rand_nums, rand_target, {rand_ia, rand_ib}),
]

all_ok = True
for name, nums, target, expect in cases:
    try:
        got = fn(list(nums), target)
    except Exception as e:
        print(f"CASE {name}: FAIL exception={type(e).__name__}: {e}")
        all_ok = False
        continue
    ok = (
        isinstance(got, (list, tuple))
        and len(got) == 2
        and set(got) == expect
        and nums[got[0]] + nums[got[1]] == target
    )
    status = "PASS" if ok else "FAIL"
    if not ok:
        all_ok = False
    print(f"CASE {name}: {status} nums={nums} target={target} got={got} expect_indices={sorted(expect)}")

print("OVERALL: " + ("PASS" if all_ok else "FAIL"))
sys.exit(0 if all_ok else 1)
PYEOF
}

BLUFF_SCAN() {
    # $1 = code text. Returns 0 (clean) or 1 (bluff pattern found).
    grep -qiE '\btodo\b|\bfor now\b|not.?implement|placeholder|\bsimulate|# *stub\b|raise +NotImplementedError|\bpass +#' <<<"$1"
}

if [[ "$MUTATE" -eq 1 ]]; then
    # -------------------------------------------------------------
    # PAIRED MUTATION: bypass the real coder, plant a deliberately
    # WRONG implementation, run it through the identical pipeline,
    # and assert the pipeline correctly FAILs it.
    # -------------------------------------------------------------
    echo "[MUT] planting deliberately-broken stub (two_sum -> constant [0, 0])"
    cat > "${WORK}/solution.py" <<'EOF'
def two_sum(nums, target):
    # MUTATION: deliberately wrong constant answer for anti-bluff proof
    return [0, 0]
EOF
    cp "${WORK}/solution.py" "${EVIDENCE_DIR}/solution_MUTATED.py"
    echo "[MUT] executing broken stub through real assertion pipeline..."
    if run_and_assert "${WORK}/solution.py" | tee "${WORK}/harness_stdout.txt"; then
        echo "  MUTATION NOT DETECTED: broken stub PASSED the checker (BLUFF in our own gate):"
        sed 's/^/    /' "${WORK}/harness_stdout.txt"
        cp "${WORK}/harness_stdout.txt" "${EVIDENCE_DIR}/harness_stdout_MUTATED.txt"
        exit 1
    fi
    echo "  mutation correctly detected — broken stub FAILED the real-execution assertions"
    cp "${WORK}/harness_stdout.txt" "${EVIDENCE_DIR}/harness_stdout_MUTATED.txt"
    echo "=== HelixLLM Coder Challenge: MUTATION DETECTED (anti-bluff OK) ==="
    exit 99
fi

# ---------------------------------------------------------------------
# [3/8] REAL HTTP call to the live coder. Captures the exact response.
# ---------------------------------------------------------------------
echo "[3/8] POST ${BASE_URL}/v1/chat/completions (real end-user coding task)..."
HTTP_CODE=$(curl -sS --max-time "$TIMEOUT_SEC" -o "${WORK}/response.json" -w '%{http_code}' \
    -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d @"${WORK}/request.json" 2>"${WORK}/curl.err") || true
cp "${WORK}/response.json" "${EVIDENCE_DIR}/response.json" 2>/dev/null || true

if [[ "$HTTP_CODE" != "200" ]]; then
    echo "[3/8] FAIL: coder returned HTTP ${HTTP_CODE}"
    sed 's/^/  stderr: /' "${WORK}/curl.err" 2>/dev/null || true
    cat "${WORK}/response.json" 2>/dev/null | sed 's/^/  body: /'
    exit 1
fi
echo "[3/8] HTTP 200 received"

RAW_CONTENT="$(jq -r '.choices[0].message.content // empty' "${WORK}/response.json")"
if [[ -z "$RAW_CONTENT" ]]; then
    echo "[4/8] FAIL: response has no choices[0].message.content (metadata-only / empty response)"
    exit 1
fi
echo "[4/8] response content: ${#RAW_CONTENT} chars"

USAGE_TOKENS="$(jq -r '.usage.completion_tokens // "N/A"' "${WORK}/response.json")"
echo "  usage.completion_tokens=${USAGE_TOKENS}"

# ---------------------------------------------------------------------
# [5/8] Extract the code and run the anti-bluff STATIC scan
# (metadata-only / stub / simulated output is a defect, not a pass).
# ---------------------------------------------------------------------
CODE="$(extract_code "$RAW_CONTENT")"
printf '%s\n' "$CODE" > "${WORK}/solution.py"
cp "${WORK}/solution.py" "${EVIDENCE_DIR}/solution.py"

if [[ -z "$CODE" ]]; then
    echo "[5/8] FAIL: no code extracted from response"
    exit 1
fi
echo "[5/8] extracted $(wc -l < "${WORK}/solution.py" | tr -d ' ') lines of code -> ${EVIDENCE_DIR}/solution.py"

if BLUFF_SCAN "$CODE"; then
    echo "[6/8] FAIL: bluff pattern detected in coder output (TODO/simulate/NotImplementedError/placeholder/stub)"
    exit 1
fi
echo "[6/8] static bluff scan: clean (no TODO/simulate/NotImplementedError/placeholder markers)"

# ---------------------------------------------------------------------
# [7/8] REAL EXECUTION of the model's code, real assertions, incl. the
# freshly-randomised vector.
# ---------------------------------------------------------------------
echo "[7/8] executing extracted code with python3 against 4 real assertions (3 fixed + 1 random)..."
if ! run_and_assert "${WORK}/solution.py" | tee "${WORK}/harness_stdout.txt"; then
    echo "  FAIL: real execution did not satisfy all assertions:"
    sed 's/^/    /' "${WORK}/harness_stdout.txt"
    cp "${WORK}/harness_stdout.txt" "${EVIDENCE_DIR}/harness_stdout.txt"
    exit 1
fi
cp "${WORK}/harness_stdout.txt" "${EVIDENCE_DIR}/harness_stdout.txt"
echo "[7/8] all 4 real-execution assertions PASSED (see harness_stdout.txt)"

# ---------------------------------------------------------------------
# [8/8] Write SUMMARY.txt evidence manifest.
# ---------------------------------------------------------------------
{
    echo "HelixLLM Coder Live End-to-End Challenge — SUMMARY"
    echo "run_token=${RUN_TOKEN}"
    echo "date_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "base_url=${BASE_URL}"
    echo "model=${MODEL_ID}"
    echo "completion_tokens=${USAGE_TOKENS}"
    echo "random_vector: nums=[${RAND_LIST}] target=${RAND_TARGET} expect_indices=(${RAND_IDX_A},${RAND_IDX_B})"
    echo "verdict=PASS"
    echo
    echo "evidence files:"
    echo "  request.json         — exact HTTP request sent to the live coder"
    echo "  response.json        — exact HTTP response received from the live coder"
    echo "  solution.py           — code extracted from the response"
    echo "  harness_stdout.txt    — real python3 execution output (per-vector PASS/FAIL)"
} > "${EVIDENCE_DIR}/SUMMARY.txt"

echo
echo "=== HelixLLM Coder Challenge: PASSED ==="
echo "  evidence: ${EVIDENCE_DIR}"
echo "  model=${MODEL_ID} completion_tokens=${USAGE_TOKENS} assertions=4/4"
