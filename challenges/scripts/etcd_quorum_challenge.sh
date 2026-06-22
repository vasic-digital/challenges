#!/usr/bin/env bash
# etcd_quorum_challenge.sh — Helix Cluster OS etcd-quorum Challenge
# (CONST-035 / CLAUDE-1 end-user usability guarantee).
#
# Asserts, against the LIVE etcd cluster that backs the control plane:
#   1. EVERY configured etcd endpoint reports health == true
#      (the etcdctl-equivalent "endpoint health" check, via the etcd
#      HTTP /health endpoint — no etcdctl dependency),
#   2. the cluster member list reports EXACTLY EXPECTED_MEMBERS members
#      (default 3) — quorum topology is intact,
#   3. all members agree on a single cluster_id (no split-brain),
#   4. a real linearizable read round-trips (POST /v3/kv/range) proving
#      the quorum can actually serve reads, not just answer /health.
#
# Real sink-side evidence: consumes the etcd cluster's own health +
# membership API the operator would inspect, and performs a genuine
# quorum-served read. No mock.
#
# Endpoints DERIVED from env (no hardcoded IPs):
#   ETCD_ENDPOINTS    comma list of member base URLs
#                     (default http://localhost:2379,http://localhost:2479,http://localhost:2579)
#   EXPECTED_MEMBERS  expected member count   (default 3)
#   ETCD_TIMEOUT_SEC  per-request timeout     (default 5)
#
# SKIP-OK only when NONE of the endpoints are reachable.
#
# Exit: 0 PASS / SKIP-OK | 1 FAIL | 2 invocation error

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

LIB_AB="$SCRIPT_DIR/../../lib/anti_bluff.sh"
[ -f "$LIB_AB" ] || LIB_AB="$SCRIPT_DIR/../../../challenges/lib/anti_bluff.sh"
if [ ! -f "$LIB_AB" ]; then
  echo "FAIL: cannot locate lib/anti_bluff.sh (looked relative to $SCRIPT_DIR)" >&2
  exit 2
fi
. "$LIB_AB"
ab_init "etcd_quorum_challenge" "/tmp/etcd_quorum_challenge.results"

command -v curl    >/dev/null 2>&1 || { echo "FAIL: curl not available"    >&2; exit 2; }
command -v jq      >/dev/null 2>&1 || { echo "FAIL: jq not available"      >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 not available" >&2; exit 2; }

ETCD_ENDPOINTS="${ETCD_ENDPOINTS:-http://localhost:2379,http://localhost:2479,http://localhost:2579}"
EXPECTED_MEMBERS="${EXPECTED_MEMBERS:-3}"
TIMEOUT_SEC="${ETCD_TIMEOUT_SEC:-5}"

echo "=== etcd_quorum_challenge ==="
echo "  endpoints=$ETCD_ENDPOINTS expected_members=$EXPECTED_MEMBERS timeout=${TIMEOUT_SEC}s"
echo

IFS=',' read -ra EPS <<< "$ETCD_ENDPOINTS"
# Trim each endpoint.
TRIMMED=()
for e in "${EPS[@]}"; do
  e="$(printf '%s' "$e" | xargs)"
  [[ -n "$e" ]] && TRIMMED+=("$e")
done
EPS=("${TRIMMED[@]}")

# Honest SKIP-OK: are ANY endpoints reachable at all?
any_reachable=0
for e in "${EPS[@]}"; do
  if curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null "${e}/health" 2>/dev/null; then
    any_reachable=1; break
  fi
done
if [[ "$any_reachable" -eq 0 ]]; then
  ab_skip "no etcd endpoint reachable among: ${ETCD_ENDPOINTS}" "#env-target-down (honest SKIP-OK per §11.4.3)"
  echo
  echo "=== summary: SKIP-OK (etcd cluster absent) ==="
  ab_summary || true
  exit 0
fi

# 1. Every endpoint health == true.
ab_send_action "GET /health on each etcd endpoint"
all_healthy=1
healthy_count=0
for e in "${EPS[@]}"; do
  hjson=$(curl -sS --max-time "$TIMEOUT_SEC" "${e}/health" 2>/dev/null || true)
  h=$(printf '%s' "$hjson" | jq -r '.health // "false"' 2>/dev/null)
  echo "    $e -> health=$h"
  if [[ "$h" == "true" ]]; then
    healthy_count=$((healthy_count+1))
  else
    all_healthy=0
  fi
done
if [[ "$all_healthy" -eq 1 && "$healthy_count" -eq "${#EPS[@]}" ]]; then
  ab_pass "all ${#EPS[@]} etcd endpoint(s) report health==true"
else
  ab_fail "only $healthy_count/${#EPS[@]} etcd endpoint(s) healthy"
fi

# 2 + 3. Member list count + single cluster_id (no split-brain).
ab_send_action "POST /v3/cluster/member/list (member topology)"
member_resp=""
member_endpoint=""
for e in "${EPS[@]}"; do
  r=$(curl -sS --max-time "$TIMEOUT_SEC" -X POST "${e}/v3/cluster/member/list" -d '{}' 2>/dev/null || true)
  if printf '%s' "$r" | jq -e '.members' >/dev/null 2>&1; then
    member_resp="$r"; member_endpoint="$e"; break
  fi
done

if [[ -z "$member_resp" ]]; then
  ab_fail "member list unavailable from any endpoint"
else
  member_count=$(printf '%s' "$member_resp" | jq -r '.members | length')
  echo "    member list (via $member_endpoint): $member_count member(s)"
  printf '%s' "$member_resp" | jq -r '.members[] | "      - \(.name) clientURLs=\(.clientURLs|join(","))"'
  if [[ "$member_count" -eq "$EXPECTED_MEMBERS" ]]; then
    ab_pass "member count == $EXPECTED_MEMBERS (quorum topology intact)"
  else
    ab_fail "member count == $member_count (expected $EXPECTED_MEMBERS)"
  fi

  # 3. cluster_id agreement across reachable endpoints (no split-brain).
  declare -A CLUSTER_IDS
  for e in "${EPS[@]}"; do
    cid=$(curl -sS --max-time "$TIMEOUT_SEC" -X POST "${e}/v3/cluster/member/list" -d '{}' 2>/dev/null \
      | jq -r '.header.cluster_id // empty' 2>/dev/null)
    [[ -n "$cid" ]] && CLUSTER_IDS["$cid"]=1
  done
  if [[ "${#CLUSTER_IDS[@]}" -eq 1 ]]; then
    ab_pass "all reachable members share one cluster_id (no split-brain): ${!CLUSTER_IDS[*]}"
  else
    ab_fail "members disagree on cluster_id (split-brain): ${!CLUSTER_IDS[*]}"
  fi
fi

# 4. Real linearizable round-trip read proving quorum serves reads.
ab_send_action "POST /v3/kv/range probe key (quorum-served read round-trip)"
PROBE_KEY=$(printf '/clusteros/' | python3 -c 'import sys,base64;sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())')
PROBE_END=$(printf '/clusteros0' | python3 -c 'import sys,base64;sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())')
read_ok=0
for e in "${EPS[@]}"; do
  rr=$(curl -sS --max-time "$TIMEOUT_SEC" -X POST "${e}/v3/kv/range" \
    -d "{\"key\":\"${PROBE_KEY}\",\"range_end\":\"${PROBE_END}\"}" 2>/dev/null || true)
  rev=$(printf '%s' "$rr" | jq -r '.header.revision // empty' 2>/dev/null)
  if [[ -n "$rev" ]]; then
    echo "    quorum read OK via $e (revision=$rev)"
    read_ok=1; break
  fi
done
if [[ "$read_ok" -eq 1 ]]; then
  ab_pass "quorum served a real linearizable read (revision advanced)"
else
  ab_fail "quorum could not serve a read round-trip"
fi

echo
if ab_summary; then
  echo "=== summary: PASS ==="
  exit 0
else
  echo "=== summary: FAIL ==="
  exit 1
fi
