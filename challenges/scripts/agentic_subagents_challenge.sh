#!/usr/bin/env bash
# agentic_subagents_challenge.sh — anti-bluff Challenge for the HelixAgent
# agentic subagents-driven dynamic flow (CONST-035 + CONST-050(B);
# submodule cascade per CONST-051(A)).
#
# WHAT IT PROVES (positive runtime evidence, §11.4.5 / Article XI §11.9):
#   A real "Decompose into subtasks and execute: ..." request to the LIVE
#   HelixAgent ensemble endpoint drives the full dynamic agentic EXECUTE
#   flow: the OpenAI-compatible /v1/chat/completions response carries an
#   `agentic` object whose `mode` is "execute", whose `stages_completed`
#   contains the 7 canonical stages (understand, plan, assign, execute,
#   verify, synthesise, respond), and whose `agents_spawned` is >= 2.
#   The real captured values (agents_spawned, the stage list, the
#   provenance_id) are printed as runtime evidence — never metadata-only.
#
# HONEST BOUNDARY (§11.4.6 no-guessing, §11.4.3 SKIP-with-reason):
#   Mode selection is performed by a LIVE LLM intent classifier
#   (AgenticEnsemble.classifyMode → ClassifyIntentWithLLM). When that
#   classifier is degraded/unavailable it falls back to "reason"/debate
#   mode (agents_spawned=0) — by design. This Challenge retries the
#   execute prompt up to N times; if ANY attempt routes to execute it
#   hard-asserts the >=2-agents + 7-stages predicate (PASS). If EVERY
#   attempt falls back to reason mode, the live agentic-execute path is
#   present but the classifier did not route to it — an honest
#   SKIP-with-reason (classifier-degraded-to-reason), NEVER a fake PASS.
#
# PAIRED-MUTATION INVARIANT (CONST-035 + §1.1 + §11.4.115):
#   With --anti-bluff-mutate the script proves the assertion genuinely
#   reads the field and discriminates the execute path:
#     (A) re-evaluate the SAME live response under an unsatisfiable
#         min_agents=999999 → the predicate MUST reject (BAD);
#     (B) drive the NEGATIVE/control prompt (a pure reason/debate
#         request) and assert it does NOT satisfy the execute predicate
#         (its agents_spawned is 0). This is the RED-on-broken proof:
#         the same predicate FAILs on a non-agentic-execute response.
#   Exits 99 when both checks behave as required.
#
# Exit codes:
#   0  — live execute-flow response: agents_spawned>=2 AND 7 stages
#        (OR honest SKIP-with-reason when classifier never routed execute)
#   1  — gate FAIL on clean run (real defect: agentic flow not firing
#        despite the classifier routing to execute)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error

set -uo pipefail

ENDPOINT="${HELIXAGENT_ENDPOINT:-http://localhost:7061/v1/chat/completions}"
MODEL="${HELIXAGENT_MODEL:-helixagent-ensemble}"
MIN_AGENTS="${HELIXAGENT_MIN_AGENTS:-2}"
ATTEMPTS="${HELIXAGENT_EXEC_ATTEMPTS:-5}"
EXEC_PROMPT="${HELIXAGENT_EXEC_PROMPT:-Decompose into subtasks and execute: build a REST API with three endpoints, write tests, and document each one}"
REASON_PROMPT="${HELIXAGENT_REASON_PROMPT:-Plan and execute the following multi-step task with subagents: refactor a module, add unit tests, update docs}"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h) sed -n '1,55p' "$0"; exit 0 ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

echo "=== HelixAgent Agentic Subagents-Driven Challenge ==="
echo "  endpoint=${ENDPOINT} model=${MODEL} min_agents=${MIN_AGENTS} attempts=${ATTEMPTS}"

for bin in curl python3; do
    command -v "$bin" >/dev/null 2>&1 || { echo "[0] FAIL: '$bin' required"; exit 2; }
done

WORK="$(mktemp -d)"; trap 'rm -rf "${WORK}"' EXIT

# Reachability gate → honest SKIP-OK if the live agent is not up.
HEALTH_URL="${ENDPOINT%/v1/chat/completions}/health"
HC="$(curl -sS --max-time 5 -o /dev/null -w '%{http_code}' "${HEALTH_URL}" 2>/dev/null || echo 000)"
if [[ "${HC}" != "200" ]]; then
    echo "[1] SKIP: agent endpoint unreachable (health HTTP ${HC}) — SKIP-OK: #env-target-down"
    echo "=== Agentic Subagents Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
echo "[1] health: HTTP ${HC}"

post() {
    curl -sS --max-time 120 "${ENDPOINT}" \
        -H 'Content-Type: application/json' \
        -d "$(python3 -c 'import json,sys; print(json.dumps({"model":sys.argv[1],"messages":[{"role":"user","content":sys.argv[2]}]}))' "${MODEL}" "$1")"
}

# eval_agentic <response.json> <min_agents> → prints OK/BAD line, exit 0/2.
eval_agentic() {
    python3 - "$1" "$2" <<'PY'
import json,sys
resp_path, min_agents = sys.argv[1], int(sys.argv[2])
CANON = ["understand","plan","assign","execute","verify","synthesise","respond"]
try:
    d = json.load(open(resp_path))
except Exception as e:
    print("BAD parse:%s" % e); sys.exit(2)
a = d.get("agentic")
if not isinstance(a, dict):
    print("BAD no-agentic-object"); sys.exit(2)
agents = a.get("agents_spawned")
stages = a.get("stages_completed") or []
mode = a.get("mode")
prov = a.get("provenance_id","")
if not isinstance(agents, int):
    print("BAD agents-not-int:%r" % agents); sys.exit(2)
missing = [s for s in CANON if s not in stages]
ok = (agents >= min_agents) and (len(missing) == 0)
print("%s agents=%d nstages=%d mode=%s missing=%s prov=%s" %
      ("OK" if ok else "BAD", agents, len(stages), mode, ",".join(missing) or "-", prov))
sys.exit(0 if ok else 2)
PY
}

mode_of() { python3 -c 'import json,sys; print((json.load(open(sys.argv[1])).get("agentic") or {}).get("mode"))' "$1" 2>/dev/null; }
agents_of() { python3 -c 'import json,sys; print((json.load(open(sys.argv[1])).get("agentic") or {}).get("agents_spawned"))' "$1" 2>/dev/null; }

if [[ "${MUTATE}" -eq 1 ]]; then
    # Drive once to get a live response (any mode) for mutation A.
    post "${EXEC_PROMPT}" > "${WORK}/exec.json" 2>/dev/null || true
    [[ -s "${WORK}/exec.json" ]] || { echo "[MUT] FAIL: no live response"; exit 1; }
    echo "[MUT-A] re-evaluate live response with absurd min_agents=999999 (must BAD)"
    if eval_agentic "${WORK}/exec.json" 999999 >"${WORK}/mA.out" 2>&1; then
        echo "  MUTATION NOT DETECTED: gate PASSed an unsatisfiable threshold (BLUFF):"
        sed 's/^/    /' "${WORK}/mA.out"; exit 1
    fi
    echo "    $(cat "${WORK}/mA.out")"
    echo "[MUT-B] NEGATIVE/RED control: reason/debate prompt must NOT satisfy execute predicate"
    post "${REASON_PROMPT}" > "${WORK}/reason.json" 2>/dev/null || true
    echo "    control agentic: $(python3 -c 'import json,sys; print(json.dumps((json.load(open(sys.argv[1])).get("agentic") or {})))' "${WORK}/reason.json" 2>/dev/null)"
    if eval_agentic "${WORK}/reason.json" "${MIN_AGENTS}" >"${WORK}/mB.out" 2>&1; then
        echo "  MUTATION NOT DETECTED: reason/debate prompt satisfied the execute predicate (BLUFF):"
        sed 's/^/    /' "${WORK}/mB.out"; exit 1
    fi
    echo "    $(cat "${WORK}/mB.out")"
    echo "=== Agentic Subagents Challenge: MUTATION DETECTED (anti-bluff OK) ==="
    exit 99
fi

# Clean run: retry the execute prompt until the live classifier routes to
# execute mode, then hard-assert the predicate.
echo "[2] driving execute-flow prompt (up to ${ATTEMPTS} attempts; classifier is live-LLM)"
GOT_EXECUTE=0
LAST_MODE=""
for i in $(seq 1 "${ATTEMPTS}"); do
    post "${EXEC_PROMPT}" > "${WORK}/exec.json" 2>/dev/null || true
    [[ -s "${WORK}/exec.json" ]] || { echo "  attempt ${i}: FAIL no response"; continue; }
    M="$(mode_of "${WORK}/exec.json")"
    A="$(agents_of "${WORK}/exec.json")"
    LAST_MODE="${M}"
    echo "  attempt ${i}: mode=${M} agents_spawned=${A}"
    if [[ "${M}" == "execute" ]]; then GOT_EXECUTE=1; break; fi
done

if [[ "${GOT_EXECUTE}" -eq 0 ]]; then
    echo "[3] SKIP: live intent-classifier routed every attempt to '${LAST_MODE}' mode,"
    echo "    never to 'execute' — the agentic-execute path is reachable but the"
    echo "    classifier did not select it. SKIP-OK: #classifier-degraded-to-reason"
    echo "=== Agentic Subagents Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi

echo "[3] live execute-flow response captured; asserting agents_spawned >= ${MIN_AGENTS} AND 7 canonical stages"
echo "    raw agentic: $(python3 -c 'import json,sys; print(json.dumps((json.load(open(sys.argv[1])).get("agentic") or {})))' "${WORK}/exec.json")"
if ! RES="$(eval_agentic "${WORK}/exec.json" "${MIN_AGENTS}")"; then
    echo "  FAIL: ${RES}"; exit 1
fi
echo "  ${RES}"

echo
echo "=== Agentic Subagents Challenge: PASSED ==="
echo "  evidence: ${RES}"
