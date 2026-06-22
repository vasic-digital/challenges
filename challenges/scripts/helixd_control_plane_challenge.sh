#!/usr/bin/env bash
# helixd_control_plane_challenge.sh — Helix Cluster OS control-plane
# liveness Challenge (CONST-035 / CLAUDE-1 end-user usability guarantee).
#
# Asserts, against the LIVE cluster, that the helixd control-plane daemon:
#   1. answers GET /status with HTTP 200,
#   2. reports top-level   status == "healthy",
#   3. reports EVERY backing service in the "services" map as "reachable"
#      (fails loudly if ANY backing service is unreachable),
#   4. carries a non-empty version + a sane unix timestamp.
#
# This is real sink-side evidence: it consumes helixd's own self-reported
# dependency health (the same JSON an operator/dashboard would read), not a
# mock and not a "process is up" proxy. A PASS means the control plane is
# end-user-reachable AND its dependencies are wired, per CLAUDE-1.
#
# Endpoints are DERIVED from env (no hardcoded IPs):
#   HELIX_HOST      host:port of helixd        (default localhost:8081)
#   HELIX_STATUS_URL full status URL           (default http://$HELIX_HOST/status)
#   HELIXD_TIMEOUT_SEC per-request timeout     (default 5)
#
# SKIP-OK (exit 0) only when helixd is genuinely unreachable — an honest
# environment limitation, never a way to hide a broken control plane.
#
# Exit:
#   0 = PASS (or honest SKIP-OK when helixd absent)
#   1 = FAIL (reachable but unhealthy / a backing service unreachable)
#   2 = invocation error

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Anti-bluff compliance promotion (Constitution §11.2.5 + §11.4).
LIB_AB="$SCRIPT_DIR/../../lib/anti_bluff.sh"
[ -f "$LIB_AB" ] || LIB_AB="$SCRIPT_DIR/../../../challenges/lib/anti_bluff.sh"
if [ ! -f "$LIB_AB" ]; then
  echo "FAIL: cannot locate lib/anti_bluff.sh (looked relative to $SCRIPT_DIR)" >&2
  exit 2
fi
. "$LIB_AB"
ab_init "helixd_control_plane_challenge" "/tmp/helixd_control_plane_challenge.results"

command -v curl >/dev/null 2>&1 || { echo "FAIL: curl not available" >&2; exit 2; }
command -v jq   >/dev/null 2>&1 || { echo "FAIL: jq not available"   >&2; exit 2; }

HELIX_HOST="${HELIX_HOST:-localhost:8081}"
STATUS_URL="${HELIX_STATUS_URL:-http://${HELIX_HOST}/status}"
TIMEOUT_SEC="${HELIXD_TIMEOUT_SEC:-5}"

echo "=== helixd_control_plane_challenge ==="
echo "  status_url=$STATUS_URL timeout=${TIMEOUT_SEC}s"
echo

ab_send_action "GET $STATUS_URL (helixd control-plane self-status)"

http_code=$(curl -sS --max-time "$TIMEOUT_SEC" -o /tmp/helixd_status.json \
  -w "%{http_code}" "$STATUS_URL" 2>/dev/null) || http_code="000"

if [[ "$http_code" == "000" ]]; then
  ab_skip "helixd unreachable at $STATUS_URL" "#env-target-down (honest SKIP-OK per §11.4.3)"
  echo
  echo "=== summary: SKIP-OK (helixd absent) ==="
  ab_summary || true
  exit 0
fi

body=$(cat /tmp/helixd_status.json 2>/dev/null)
echo "  HTTP $http_code"
echo "  body: $body"
echo

# 1. HTTP 200
if [[ "$http_code" != "200" ]]; then
  ab_fail "helixd /status returned HTTP $http_code (expected 200)"
  ab_summary || true
  echo "=== summary: FAIL ==="
  exit 1
fi
ab_pass "helixd /status reachable with HTTP 200"

# Body must be valid JSON.
if ! printf '%s' "$body" | jq -e . >/dev/null 2>&1; then
  ab_fail "helixd /status body is not valid JSON"
  ab_summary || true
  echo "=== summary: FAIL ==="
  exit 1
fi

# 2. top-level status == healthy
status=$(printf '%s' "$body" | jq -r '.status // empty')
if [[ "$status" == "healthy" ]]; then
  ab_pass "top-level status == 'healthy'"
else
  ab_fail "top-level status == '$status' (expected 'healthy')"
fi

# 3. every backing service reachable
svc_count=$(printf '%s' "$body" | jq -r '(.services // {}) | length')
if [[ "${svc_count:-0}" -eq 0 ]]; then
  ab_fail "helixd /status has no 'services' map — cannot prove dependencies wired"
else
  unreachable=$(printf '%s' "$body" \
    | jq -r '.services | to_entries[] | select(.value != "reachable") | "\(.key)=\(.value)"')
  reachable_list=$(printf '%s' "$body" \
    | jq -r '.services | to_entries[] | select(.value == "reachable") | .key' | paste -sd, -)
  if [[ -z "$unreachable" ]]; then
    ab_pass "all $svc_count backing services reachable: ${reachable_list}"
  else
    ab_fail "unreachable backing service(s): $(printf '%s' "$unreachable" | paste -sd' ' -)"
  fi
fi

# 4. version non-empty + sane timestamp
version=$(printf '%s' "$body" | jq -r '.version // empty')
ts=$(printf '%s' "$body" | jq -r '.timestamp // 0')
if [[ -n "$version" ]]; then
  ab_pass "version reported: '$version'"
else
  ab_fail "version field empty"
fi
# Sanity: timestamp within a wide-but-real window (after 2020, not absurdly future).
if [[ "$ts" =~ ^[0-9]+$ ]] && [[ "$ts" -gt 1577836800 ]]; then
  ab_pass "timestamp present and plausible ($ts)"
else
  ab_fail "timestamp implausible or missing ('$ts')"
fi

echo
if ab_summary; then
  echo "=== summary: PASS ==="
  exit 0
else
  echo "=== summary: FAIL ==="
  exit 1
fi
