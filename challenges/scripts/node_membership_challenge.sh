#!/usr/bin/env bash
# node_membership_challenge.sh — Helix Cluster OS node-membership +
# lease-renewal Challenge (CONST-035 / CLAUDE-1 end-user usability).
#
# Asserts, against the LIVE etcd that backs the cluster's node registry:
#   1. >= MIN_NODES node records exist under /clusteros/nodes/,
#   2. EVERY discovered node record decodes to JSON with "Healthy": true,
#   3. each node record is bound to an etcd lease (membership is
#      lease-backed, not a stale orphaned key),
#   4. LEASE RENEWAL guard (encodes the D14 regression directly): after
#      sleeping LEASE_RESAMPLE_SEC (default 35s, > the node TTL window),
#      the SAME node keys are STILL present with Healthy:true — i.e. the
#      membership leases were renewed and the nodes did not silently
#      expire out of the registry.
#
# This consumes the real registry an operator/scheduler reads, decoding
# the actual stored Node JSON — real sink-side evidence, not a mock.
#
# Endpoints DERIVED from env (no hardcoded IPs):
#   ETCD_ENDPOINT     base URL of an etcd member (default http://localhost:2379)
#                     (first of $ETCD_ENDPOINTS comma list is used if set)
#   ETCD_ENDPOINTS    optional comma list; first reachable member chosen
#   NODES_PREFIX      key prefix             (default /clusteros/nodes/)
#   MIN_NODES         minimum healthy nodes  (default 2)
#   LEASE_RESAMPLE_SEC second-sample delay   (default 35)
#   NODE_TIMEOUT_SEC  per-request timeout    (default 5)
#
# SKIP-OK only when etcd is genuinely unreachable.
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
ab_init "node_membership_challenge" "/tmp/node_membership_challenge.results"

command -v curl    >/dev/null 2>&1 || { echo "FAIL: curl not available"    >&2; exit 2; }
command -v jq      >/dev/null 2>&1 || { echo "FAIL: jq not available"      >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 not available" >&2; exit 2; }

NODES_PREFIX="${NODES_PREFIX:-/clusteros/nodes/}"
MIN_NODES="${MIN_NODES:-2}"
LEASE_RESAMPLE_SEC="${LEASE_RESAMPLE_SEC:-35}"
TIMEOUT_SEC="${NODE_TIMEOUT_SEC:-5}"

# Resolve a reachable etcd endpoint.
ENDPOINT="${ETCD_ENDPOINT:-}"
if [[ -z "$ENDPOINT" ]]; then
  IFS=',' read -ra CANDS <<< "${ETCD_ENDPOINTS:-http://localhost:2379}"
  for c in "${CANDS[@]}"; do
    c="$(printf '%s' "$c" | xargs)"
    [[ -z "$c" ]] && continue
    if curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null "${c}/health" 2>/dev/null; then
      ENDPOINT="$c"; break
    fi
  done
  ENDPOINT="${ENDPOINT:-http://localhost:2379}"
fi

echo "=== node_membership_challenge ==="
echo "  endpoint=$ENDPOINT prefix=$NODES_PREFIX min_nodes=$MIN_NODES resample=${LEASE_RESAMPLE_SEC}s"
echo

# b64 helper (portable across macOS/Linux; no -w flag dependence).
b64() { printf '%s' "$1" | python3 -c 'import sys,base64; sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())'; }
b64d() { printf '%s' "$1" | python3 -c 'import sys,base64; sys.stdout.write(base64.b64decode(sys.stdin.read()).decode("utf-8","replace"))'; }

# range_end for a prefix = prefix with last byte incremented.
range_end_for() {
  python3 - "$1" <<'PY'
import sys
p=sys.argv[1].encode()
b=bytearray(p); b[-1]+=1
import base64
sys.stdout.write(base64.b64encode(bytes(b)).decode())
PY
}

# Returns raw etcd v3 range JSON, or empty on failure.
range_query() {
  local key_b64 end_b64
  key_b64=$(b64 "$NODES_PREFIX")
  end_b64=$(range_end_for "$NODES_PREFIX")
  curl -sS --max-time "$TIMEOUT_SEC" -X POST "${ENDPOINT}/v3/kv/range" \
    -d "{\"key\":\"${key_b64}\",\"range_end\":\"${end_b64}\"}" 2>/dev/null
}

# Reachability probe -> honest SKIP-OK if absent.
if ! curl -sS --max-time "$TIMEOUT_SEC" -o /dev/null "${ENDPOINT}/health" 2>/dev/null; then
  ab_skip "etcd unreachable at $ENDPOINT" "#env-target-down (honest SKIP-OK per §11.4.3)"
  echo
  echo "=== summary: SKIP-OK (etcd absent) ==="
  ab_summary || true
  exit 0
fi

ab_send_action "etcd v3 range over $NODES_PREFIX (sample 1)"
RESP1=$(range_query)
if [[ -z "$RESP1" ]] || ! printf '%s' "$RESP1" | jq -e . >/dev/null 2>&1; then
  ab_fail "etcd range query returned no/invalid JSON"
  ab_summary || true
  echo "=== summary: FAIL ==="
  exit 1
fi

# Collect keys (decoded) + per-key Healthy + lease for sample 1.
mapfile -t KEYS1 < <(printf '%s' "$RESP1" | jq -r '.kvs[]?.key' | while read -r k; do b64d "$k"; echo; done)
count1=$(printf '%s' "$RESP1" | jq -r '(.kvs // []) | length')
echo "  sample1: $count1 node key(s) under $NODES_PREFIX"

if [[ "${count1:-0}" -lt "$MIN_NODES" ]]; then
  ab_fail "only ${count1:-0} node(s) registered (< MIN_NODES=$MIN_NODES)"
else
  ab_pass "found ${count1} registered node(s) (>= MIN_NODES=$MIN_NODES)"
fi

# Validate each record: Healthy:true + lease attached.
healthy_ok=1
lease_ok=1
declare -A SAMPLE1_HEALTHY
n=$(printf '%s' "$RESP1" | jq -r '(.kvs // []) | length')
for (( i=0; i<n; i++ )); do
  key_b64=$(printf '%s' "$RESP1" | jq -r ".kvs[$i].key")
  val_b64=$(printf '%s' "$RESP1" | jq -r ".kvs[$i].value")
  lease=$(printf '%s'  "$RESP1" | jq -r ".kvs[$i].lease // \"0\"")
  key=$(b64d "$key_b64")
  val=$(b64d "$val_b64")
  id=$(printf '%s' "$val" | jq -r '.ID // empty' 2>/dev/null)
  healthy=$(printf '%s' "$val" | jq -r '.Healthy // false' 2>/dev/null)
  echo "    node '$id' (key=$key) Healthy=$healthy lease=$lease"
  [[ "$healthy" == "true" ]] && SAMPLE1_HEALTHY["$key"]=1 || healthy_ok=0
  [[ "$lease" == "0" || -z "$lease" ]] && lease_ok=0
done

if [[ "$healthy_ok" -eq 1 && "${count1:-0}" -ge "$MIN_NODES" ]]; then
  ab_pass "every node record decodes to JSON with Healthy:true"
else
  ab_fail "one or more node records are not Healthy:true (or too few nodes)"
fi
if [[ "$lease_ok" -eq 1 && "${count1:-0}" -ge "$MIN_NODES" ]]; then
  ab_pass "every node record is bound to an etcd lease (membership is lease-backed)"
else
  ab_fail "one or more node records have no lease — membership not lease-backed"
fi

# ---- Lease-renewal guard (D14 regression encoded as a Challenge) ----
echo
echo "  waiting ${LEASE_RESAMPLE_SEC}s to verify lease renewal (D14 guard)..."
sleep "$LEASE_RESAMPLE_SEC"

ab_send_action "etcd v3 range over $NODES_PREFIX (sample 2, +${LEASE_RESAMPLE_SEC}s)"
RESP2=$(range_query)
if [[ -z "$RESP2" ]] || ! printf '%s' "$RESP2" | jq -e . >/dev/null 2>&1; then
  ab_fail "second etcd range query returned no/invalid JSON"
  ab_summary || true
  echo "=== summary: FAIL ==="
  exit 1
fi

count2=$(printf '%s' "$RESP2" | jq -r '(.kvs // []) | length')
echo "  sample2: $count2 node key(s) under $NODES_PREFIX"

# Each key healthy in sample1 MUST still be present + Healthy in sample2.
survived=1
n2=$(printf '%s' "$RESP2" | jq -r '(.kvs // []) | length')
declare -A SAMPLE2_HEALTHY
for (( i=0; i<n2; i++ )); do
  key_b64=$(printf '%s' "$RESP2" | jq -r ".kvs[$i].key")
  val_b64=$(printf '%s' "$RESP2" | jq -r ".kvs[$i].value")
  key=$(b64d "$key_b64"); val=$(b64d "$val_b64")
  healthy=$(printf '%s' "$val" | jq -r '.Healthy // false' 2>/dev/null)
  [[ "$healthy" == "true" ]] && SAMPLE2_HEALTHY["$key"]=1
done
for key in "${!SAMPLE1_HEALTHY[@]}"; do
  if [[ -z "${SAMPLE2_HEALTHY[$key]:-}" ]]; then
    echo "    REGRESSION: node key '$key' healthy in sample1 but gone/unhealthy in sample2"
    survived=0
  fi
done

if [[ "$survived" -eq 1 && "$count2" -ge "$MIN_NODES" ]]; then
  ab_pass "all sample1 healthy nodes persisted across ${LEASE_RESAMPLE_SEC}s — leases renewed (D14 guard PASS)"
else
  ab_fail "membership did NOT persist across ${LEASE_RESAMPLE_SEC}s — lease renewal regression (D14)"
fi

echo
if ab_summary; then
  echo "=== summary: PASS ==="
  exit 0
else
  echo "=== summary: FAIL ==="
  exit 1
fi
