#!/bin/bash
# recording_pipeline_challenge.sh — Phase 34.K / Bug #14.G initial cut.
#
# End-to-end Challenge for the Bug #14 recording infrastructure (live
# dual-display screen recording + Tesseract/Whisper background analysis
# + Claude<->HelixQA bridge). Verifies that the bridge HTTP API, the
# recording wrapper, and the analyzer pipeline are wired together and
# producing structured findings.
#
# Per Issues.md G9 sub-G + Constitution §11.4: recording infrastructure
# is the §11.4 enforcement mechanism for the entire project. If the
# pipeline silently breaks, every test that depends on it for positive
# evidence becomes a bluff. This Challenge meta-tests the meta-test.
#
# Flow:
#   1. Bridge health-check at http://127.0.0.1:7842/v1/health.
#      503 / connection refused → SKIP-with-reason "dev infra not running".
#   2. POST /v1/recording/start with a 1 s sampling interval.
#   3. Wait 10 s.
#   4. POST /v1/recording/stop.
#   5. POST /v1/analyze/start.
#   6. GET /v1/findings/stream (line-delimited JSON, NDJSON-style).
#      Read at least 1 line within 30 s.
#   7. Assert: line count >= 1, JSON parses, has fields ts + display +
#      frame_idx + decision.
#
# Exit codes:
#   0 = bridge up + recording + analyzer + findings all green
#   1 = at least one assertion FAILed (real defect — escalate)
#   2 = bridge unreachable (infra not running — SKIP, not a failure)
#
# Tracked: Issues.md G9 sub-G / Bug #14 / TaskCreate #166.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Anti-bluff library — same pattern as no_suspend_calls_challenge.sh.
LIB_AB="$SCRIPT_DIR/../../lib/anti_bluff.sh"
[ -f "$LIB_AB" ] || LIB_AB="$SCRIPT_DIR/../../../challenges/lib/anti_bluff.sh"
if [ ! -f "$LIB_AB" ]; then
    echo "FATAL: anti_bluff.sh missing (looked in $LIB_AB)" >&2
    exit 2
fi
. "$LIB_AB"

ab_init "recording_pipeline_challenge" "/tmp/recording_pipeline_challenge.results"
ab_send_action "Bug #14.G — meta-test the recording-pipeline (bridge + recording wrapper + analyzer + findings stream)"

# ───────────────────────────────────────────────────────────────────────
# 1. Trap on EXIT (Bug #10 audit pattern).
# ───────────────────────────────────────────────────────────────────────
TEST_COMPLETED=0
RECORDING_STARTED=0
TEST_NAME_TOKEN=""

on_exit() {
    rc=$?
    if [ "$TEST_COMPLETED" = "0" ]; then
        ab_fail "Challenge exited at line ${LINENO:-?} rc=$rc BEFORE reaching summary — possible script-internal crash (§11.4.1 FAIL-bluff guard)"
        ab_summary 2>/dev/null || true
    fi
    # Best-effort: stop any recording we may have left running.
    if [ "$RECORDING_STARTED" = "1" ] && [ -n "$TEST_NAME_TOKEN" ]; then
        curl -s -X POST -m 3 \
            -H 'Content-Type: application/json' \
            -d "{\"test_name\":\"$TEST_NAME_TOKEN\"}" \
            "$BRIDGE_URL/v1/recording/stop" >/dev/null 2>&1 || true
    fi
    exit $rc
}
trap on_exit EXIT

# ───────────────────────────────────────────────────────────────────────
# 2. Configuration.
# ───────────────────────────────────────────────────────────────────────
BRIDGE_URL="${HELIXQA_BRIDGE_URL:-http://127.0.0.1:7842}"
TEST_NAME_TOKEN="recording_pipeline_challenge_$(date +%s)"
INTERVAL_MS="${HELIXQA_RECORDING_INTERVAL_MS:-1000}"
RECORD_SECONDS="${HELIXQA_RECORD_SECONDS:-10}"
FINDINGS_TIMEOUT_S="${HELIXQA_FINDINGS_TIMEOUT_S:-30}"

# Pick first connected device serial (best-effort). Bridge accepts an
# empty string and falls back to its default device.
DEVICE_SERIAL="${HELIXQA_DEVICE_SERIAL:-}"
if [ -z "$DEVICE_SERIAL" ] && command -v adb >/dev/null 2>&1; then
    DEVICE_SERIAL=$(adb devices 2>/dev/null \
        | awk 'NR>1 && $2=="device" {print $1; exit}')
fi
[ -z "$DEVICE_SERIAL" ] && DEVICE_SERIAL="any"

echo "=== recording_pipeline_challenge ==="
echo "Bridge URL:    $BRIDGE_URL"
echo "Test token:    $TEST_NAME_TOKEN"
echo "Device serial: $DEVICE_SERIAL"
echo "Interval (ms): $INTERVAL_MS"
echo "Record (s):    $RECORD_SECONDS"
echo

# ───────────────────────────────────────────────────────────────────────
# 3. Step 1 — bridge health-check.
# ───────────────────────────────────────────────────────────────────────
echo "--- Step 1: bridge health-check ---"

if ! command -v curl >/dev/null 2>&1; then
    ab_skip "curl not installed on host — cannot probe bridge"
    TEST_COMPLETED=1
    ab_summary
    exit 2
fi

HEALTH_RESP=$(curl -s -m 5 -o /tmp/__rpc_health.body -w '%{http_code}' \
    "$BRIDGE_URL/v1/health" 2>/dev/null || echo "000")
echo "  HTTP $HEALTH_RESP"
case "$HEALTH_RESP" in
    200)
        ab_pass "bridge /v1/health responded HTTP 200 — dev infra up"
        ;;
    000|503|502|504)
        ab_skip "helixqa-bridge not running at $BRIDGE_URL (HTTP=$HEALTH_RESP), Bug #14 dev infrastructure required — set HELIXQA_BRIDGE_URL or start scripts/dev/helixqa-bridge.sh"
        TEST_COMPLETED=1
        ab_summary
        exit 2
        ;;
    *)
        ab_fail "bridge /v1/health returned unexpected HTTP $HEALTH_RESP (body: $(cat /tmp/__rpc_health.body 2>/dev/null | head -c 200))"
        TEST_COMPLETED=1
        ab_summary
        exit 1
        ;;
esac

# ───────────────────────────────────────────────────────────────────────
# 4. Step 2 — start recording.
# ───────────────────────────────────────────────────────────────────────
echo
echo "--- Step 2: start recording ---"

START_BODY=$(cat <<EOF
{"test_name":"$TEST_NAME_TOKEN","device_serial":"$DEVICE_SERIAL","interval_ms":$INTERVAL_MS}
EOF
)
START_RESP=$(curl -s -m 10 -o /tmp/__rpc_start.body -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' \
    -d "$START_BODY" \
    "$BRIDGE_URL/v1/recording/start" 2>/dev/null || echo "000")
echo "  HTTP $START_RESP"
echo "  body: $(cat /tmp/__rpc_start.body 2>/dev/null | head -c 300)"
if [ "$START_RESP" = "200" ] || [ "$START_RESP" = "201" ] || [ "$START_RESP" = "202" ]; then
    ab_pass "/v1/recording/start accepted (HTTP $START_RESP)"
    RECORDING_STARTED=1
else
    ab_fail "/v1/recording/start returned HTTP $START_RESP — recording wrapper rejected the request"
    TEST_COMPLETED=1
    ab_summary
    exit 1
fi

# ───────────────────────────────────────────────────────────────────────
# 5. Step 3 — wait for the recording window to fill.
# ───────────────────────────────────────────────────────────────────────
echo
echo "--- Step 3: wait $RECORD_SECONDS s for recording window ---"
sleep "$RECORD_SECONDS"

# ───────────────────────────────────────────────────────────────────────
# 6. Step 4 — stop recording.
# ───────────────────────────────────────────────────────────────────────
echo
echo "--- Step 4: stop recording ---"

STOP_BODY=$(cat <<EOF
{"test_name":"$TEST_NAME_TOKEN","device_serial":"$DEVICE_SERIAL"}
EOF
)
STOP_RESP=$(curl -s -m 10 -o /tmp/__rpc_stop.body -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' \
    -d "$STOP_BODY" \
    "$BRIDGE_URL/v1/recording/stop" 2>/dev/null || echo "000")
echo "  HTTP $STOP_RESP"
if [ "$STOP_RESP" = "200" ] || [ "$STOP_RESP" = "204" ]; then
    ab_pass "/v1/recording/stop accepted (HTTP $STOP_RESP)"
    RECORDING_STARTED=0
else
    ab_fail "/v1/recording/stop returned HTTP $STOP_RESP — recording wrapper failed to finalise"
fi

# ───────────────────────────────────────────────────────────────────────
# 7. Step 5 — trigger analyzer.
# ───────────────────────────────────────────────────────────────────────
echo
echo "--- Step 5: trigger analyzer ---"

ANALYZE_BODY=$(cat <<EOF
{"test_name":"$TEST_NAME_TOKEN","interval_ms":$INTERVAL_MS}
EOF
)
ANALYZE_RESP=$(curl -s -m 15 -o /tmp/__rpc_analyze.body -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' \
    -d "$ANALYZE_BODY" \
    "$BRIDGE_URL/v1/analyze/start" 2>/dev/null || echo "000")
echo "  HTTP $ANALYZE_RESP"
if [ "$ANALYZE_RESP" = "200" ] || [ "$ANALYZE_RESP" = "201" ] || [ "$ANALYZE_RESP" = "202" ]; then
    ab_pass "/v1/analyze/start accepted (HTTP $ANALYZE_RESP)"
else
    ab_fail "/v1/analyze/start returned HTTP $ANALYZE_RESP — analyzer pipeline rejected the request"
fi

# ───────────────────────────────────────────────────────────────────────
# 8. Step 6 — read findings stream (NDJSON, one finding per line).
# ───────────────────────────────────────────────────────────────────────
echo
echo "--- Step 6: poll /v1/findings/stream (timeout ${FINDINGS_TIMEOUT_S}s) ---"

FINDINGS_FILE="/tmp/__rpc_findings.ndjson"
: > "$FINDINGS_FILE"

# Read with a short connection timeout per attempt; loop until we have
# at least one line OR we hit the overall timeout.
elapsed=0
while [ "$elapsed" -lt "$FINDINGS_TIMEOUT_S" ]; do
    # --max-time bounds each attempt so a hung server doesn't trap us.
    curl -s -m 5 -N \
        -G --data-urlencode "test_name=$TEST_NAME_TOKEN" \
        "$BRIDGE_URL/v1/findings/stream" 2>/dev/null \
        | head -5 >> "$FINDINGS_FILE" || true
    if [ -s "$FINDINGS_FILE" ]; then
        break
    fi
    sleep 2
    elapsed=$((elapsed + 7))
done

LINE_COUNT=$(wc -l < "$FINDINGS_FILE" 2>/dev/null || echo 0)
LINE_COUNT=$(echo "$LINE_COUNT" | tr -dc '0-9')
[ -z "$LINE_COUNT" ] && LINE_COUNT=0
echo "  findings_lines=$LINE_COUNT"
echo "  first line:    $(head -1 "$FINDINGS_FILE" 2>/dev/null | head -c 300)"

# ───────────────────────────────────────────────────────────────────────
# 9. Step 7 — assert findings shape.
# ───────────────────────────────────────────────────────────────────────
echo
echo "--- Step 7: assert findings shape ---"

if [ "$LINE_COUNT" -lt 1 ]; then
    ab_fail "findings stream returned 0 lines in ${FINDINGS_TIMEOUT_S} s — analyzer is silently broken (§11.4 escape: tests using positive-evidence assertions cannot get evidence)"
    TEST_COMPLETED=1
    ab_summary
    exit 1
fi
ab_pass "findings stream returned $LINE_COUNT line(s) within ${FINDINGS_TIMEOUT_S}s"

FIRST_LINE=$(head -1 "$FINDINGS_FILE")

# JSON-parse check: prefer jq if available, fall back to python3, then
# to a crude grep that at least confirms it looks like a JSON object.
PARSED_OK=0
if command -v jq >/dev/null 2>&1; then
    if echo "$FIRST_LINE" | jq -e . >/dev/null 2>&1; then
        PARSED_OK=1
    fi
elif command -v python3 >/dev/null 2>&1; then
    if echo "$FIRST_LINE" | python3 -c 'import sys,json; json.loads(sys.stdin.read())' 2>/dev/null; then
        PARSED_OK=1
    fi
elif echo "$FIRST_LINE" | grep -qE '^\{.*\}$'; then
    PARSED_OK=1
fi

if [ "$PARSED_OK" = "1" ]; then
    ab_pass "first finding line parses as JSON"
else
    ab_fail "first finding line does NOT parse as JSON: $(echo "$FIRST_LINE" | head -c 200)"
fi

# Field presence check: ts + display + frame_idx + decision.
MISSING_FIELDS=""
for field in ts display frame_idx decision; do
    if ! echo "$FIRST_LINE" | grep -qE "\"$field\"[[:space:]]*:"; then
        MISSING_FIELDS="$MISSING_FIELDS $field"
    fi
done
if [ -z "$MISSING_FIELDS" ]; then
    ab_pass "first finding line has all required fields (ts, display, frame_idx, decision)"
else
    ab_fail "first finding line missing required fields:$MISSING_FIELDS"
fi

# ───────────────────────────────────────────────────────────────────────
# 10. Summary.
# ───────────────────────────────────────────────────────────────────────
echo
echo "=== recording_pipeline_challenge done ==="
TEST_COMPLETED=1
ab_summary
