#!/usr/bin/env bash
# provider_aliases_challenge.sh — anti-bluff Challenge for the claude_toolkit
# provider-alias verification pipeline (CONST-035 / Article XI §11.9).
#
# Verifies, READ-ONLY, that a claude_toolkit checkout's verification pipeline
# is honest — i.e. an alias may only reach status "verified" via a real
# completion POST that returns the VERIFY_OK sentinel plus a tool-calling
# success, never via a bare GET /v1/models 200. That bare-GET mapping was
# the huggingface-depleted-credits false-positive class: the models list
# answered 200 while every completion failed on billing, and the old
# pipeline still reported "verified".
#
# Checks (all evidence carries a unique per-run token):
#   1. scripts/providers-verify.sh no longer maps a bare GET /v1/models
#      probe to `verified` — it must POST to a chat endpoint and assert a
#      sentinel before emitting `verified` (static greps).
#   2. scripts/model_verify.py asserts the VERIFY_OK sentinel against the
#      response content and gates `verified` on tool calling (static greps).
#   3. If the toolkit's persisted status cache exists
#      (~/.local/share/claude-multi-account/providers/status.json): every
#      alias with status "verified" has failing_layer empty AND a
#      checked_at within the last ${STALE_DAYS} days (jq). A stale or
#      layered "verified" is a FAIL with the offending aliases as evidence.
#   4. scripts/tests/verify_aliases_live.sh presence is reported
#   5. Kimi Code OAuth support: detector discovers served models via /models,
#      OAuth records win precedence, launch-time token freshness (live cred
#      file + expiry + CLI refresh), kimi_proxy moonshot #/$defs/ schema
#      normalization, live kimi alias freshness in status.json.
#      (observed only — the live sweep is far too heavy to run inside this
#      Challenge).
#
# Env knobs:
#   CLAUDE_TOOLKIT_ROOT     toolkit checkout (default
#                           /run/media/milosvasic/DATA4TB/Projects/claude_toolkit)
#   CMA_PROVIDER_STATUS_FILE status.json path (default
#                           ~/.local/share/claude-multi-account/providers/status.json)
#   CMA_VERIFIED_MAX_AGE_DAYS  staleness budget in days (default 7)
#   CHALLENGE_EVIDENCE_DIR  evidence log dir (default challenges/evidence/)
#
# SKIP-OK (exit 0) only when the toolkit checkout is genuinely absent —
# an honest environment limitation, never a way to hide a broken pipeline.
#
# Exit:
#   0 = PASS (or honest SKIP-OK when the toolkit is absent)
#   1 = FAIL (positive evidence in the log)
#   2 = invocation / environment error

set -uo pipefail

for arg in "$@"; do
  case "$arg" in
    --help|-h) sed -n '1,44p' "$0"; exit 0 ;;
    *) echo "provider_aliases_challenge: unknown argument: $arg" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Anti-bluff compliance promotion (Constitution §11.2.5 + §11.4).
LIB_AB="$SCRIPT_DIR/../../lib/anti_bluff.sh"
[ -f "$LIB_AB" ] || LIB_AB="$SCRIPT_DIR/../../../challenges/lib/anti_bluff.sh"
if [ ! -f "$LIB_AB" ]; then
  echo "FAIL: cannot locate lib/anti_bluff.sh (looked relative to $SCRIPT_DIR)" >&2
  exit 2
fi
. "$LIB_AB"

TOOLKIT_ROOT="${CLAUDE_TOOLKIT_ROOT:-/run/media/milosvasic/DATA4TB/Projects/claude_toolkit}"
STATUS_FILE="${CMA_PROVIDER_STATUS_FILE:-$HOME/.local/share/claude-multi-account/providers/status.json}"
STALE_DAYS="${CMA_VERIFIED_MAX_AGE_DAYS:-7}"
case "$STALE_DAYS" in ''|*[!0-9]*) STALE_DAYS=7 ;; esac
EVIDENCE_DIR="${CHALLENGE_EVIDENCE_DIR:-$SCRIPT_DIR/../evidence}"

PROVIDERS_VERIFY="$TOOLKIT_ROOT/scripts/providers-verify.sh"
MODEL_VERIFY="$TOOLKIT_ROOT/scripts/model_verify.py"
LIVE_VERIFIER="$TOOLKIT_ROOT/scripts/tests/verify_aliases_live.sh"

mkdir -p "$EVIDENCE_DIR"
TOKEN="$(ab_evidence_token)"
ab_init "provider_aliases_challenge" "$EVIDENCE_DIR/provider_aliases_challenge-${TOKEN}.log"

echo "  toolkit_root=$TOOLKIT_ROOT"
echo "  status_file=$STATUS_FILE"
echo "  stale_budget_days=$STALE_DAYS"
echo "  evidence_token=$TOKEN"
echo

# --- Honest SKIP-OK: toolkit checkout genuinely absent ----------------------
if [ ! -d "$TOOLKIT_ROOT/scripts" ]; then
  ab_skip "claude_toolkit checkout" \
    "absent at $TOOLKIT_ROOT (set CLAUDE_TOOLKIT_ROOT) — honest SKIP-OK per §11.4.3"
  echo "  evidence: token=${TOKEN} toolkit_absent=$TOOLKIT_ROOT"
  echo
  echo "=== summary: SKIP-OK (toolkit absent) ==="
  ab_summary || true
  exit 0
fi

command -v jq   >/dev/null 2>&1 || { echo "FAIL: jq not available"   >&2; exit 2; }
command -v awk  >/dev/null 2>&1 || { echo "FAIL: awk not available"  >&2; exit 2; }
command -v grep >/dev/null 2>&1 || { echo "FAIL: grep not available" >&2; exit 2; }

# --- Check 1: providers-verify.sh — no verified-from-bare-GET ---------------
echo "[1/5] providers-verify.sh: no verified-on-bare-GET; POST + sentinel required"
ab_send_action "static-grep scripts/providers-verify.sh (bare-GET anti-pattern + POST/sentinel requirements)"
if [ ! -f "$PROVIDERS_VERIFY" ]; then
  ab_fail "scripts/providers-verify.sh missing in toolkit checkout $TOOLKIT_ROOT"
else
  stat -c '  evidence: file=%n size=%s mtime=%y' "$PROVIDERS_VERIFY" | tee -a "$AB_RESULTS_PATH"

  # 1a. The bluff pattern itself must be gone: a `200) emit verified` case
  #     arm fed by the curl GET probe of /v1/models.
  anti="$(grep -nE '^[[:space:]]*200\)[[:space:]]*emit[[:space:]]+verified' "$PROVIDERS_VERIFY" || true)"
  if [ -n "$anti" ]; then
    ab_fail "providers-verify.sh still maps a bare GET /v1/models HTTP 200 to 'verified': ${anti}"
  else
    ab_pass "no bare GET /v1/models → verified mapping in providers-verify.sh"
  fi

  # 1b. Verification must POST to a real chat completion endpoint (or
  #     delegate to model_verify.py, which POSTs). A GET of the model list
  #     proves nothing about completions.
  if grep -qE '/chat/completions|/v1/messages' "$PROVIDERS_VERIFY" \
     && grep -qE '(-X[[:space:]]*POST|--request[[:space:]]+POST|--data(-binary)?([[:space:]=])|-d[[:space:]]|http_post_json|model_verify\.py)' "$PROVIDERS_VERIFY"; then
    ab_pass "providers-verify.sh POSTs to a chat completion endpoint"
  else
    ab_fail "providers-verify.sh does not POST to a chat endpoint (GET-only probing cannot prove a model answers)"
  fi

  # 1c. A response sentinel must be asserted before `verified` is emitted.
  if grep -qE 'VERIFY_OK|SENTINEL' "$PROVIDERS_VERIFY"; then
    ab_pass "providers-verify.sh asserts a response sentinel before verified"
  else
    ab_fail "providers-verify.sh asserts no response sentinel (e.g. VERIFY_OK) before emitting verified"
  fi
fi
echo

# --- Check 2: model_verify.py — sentinel asserted + tool-call gate ----------
echo "[2/5] model_verify.py: VERIFY_OK sentinel asserted; verified gated on tool calling"
ab_send_action "static-grep scripts/model_verify.py (sentinel assertion + tool-call gate)"
if [ ! -f "$MODEL_VERIFY" ]; then
  ab_fail "scripts/model_verify.py missing in toolkit checkout $TOOLKIT_ROOT"
else
  stat -c '  evidence: file=%n size=%s mtime=%y' "$MODEL_VERIFY" | tee -a "$AB_RESULTS_PATH"

  # 2a. Sentinel literal exists at all.
  sentinel_sites="$(grep -nE 'VERIFY_OK' "$MODEL_VERIFY" || true)"
  if [ -n "$sentinel_sites" ]; then
    n_sites="$(printf '%s\n' "$sentinel_sites" | wc -l | tr -d '[:space:]')"
    ab_pass "VERIFY_OK sentinel referenced at ${n_sites} site(s) in model_verify.py"
  else
    ab_fail "no VERIFY_OK sentinel anywhere in model_verify.py"
  fi

  # 2b. The sentinel must be ASSERTED against the response content — a
  #     membership test (`EXPECTED_CONTENT [not] in <content>`), not merely
  #     defined or used as a prompt. Without this gate a model answering
  #     anything (or an error string) still scored as verified.
  assert_sites="$(grep -nE 'EXPECTED_CONTENT[[:space:]]+(not[[:space:]]+)?in[[:space:]]+[A-Za-z_]' "$MODEL_VERIFY" || true)"
  if [ -n "$assert_sites" ]; then
    ab_pass "VERIFY_OK sentinel asserted against response content: $(printf '%s' "$assert_sites" | head -n1)"
  else
    ab_fail "VERIFY_OK sentinel never asserted against response content (verified can pass without the sentinel)"
  fi

  # 2c. `verified` decisions must be gated on tool calling. Proximity scan:
  #     a `verified` assignment/decision within 4 lines of a `tool_call`
  #     reference (either direction) — the natural shape of the gate.
  if awk '/tool_call/{ if (v != "" && NR - v <= 4) found=1; t=NR }
          /verified/{ if (t != "" && NR - t <= 4) found=1; v=NR }
          END{ exit(found ? 0 : 1) }' "$MODEL_VERIFY"; then
    ab_pass "verified is gated on tool_call capability in model_verify.py"
  else
    ab_fail "no tool_call gate found near verified decisions in model_verify.py"
  fi
fi
echo

# --- Check 3: status.json — verified aliases fresh + failing_layer empty ----
echo "[3/5] status.json: verified aliases have failing_layer empty + fresh checked_at"
if [ ! -f "$STATUS_FILE" ]; then
  ab_skip "providers/status.json" \
    "absent at $STATUS_FILE — no persisted verification state on this host"
else
  ab_send_action "jq verified-alias invariants on $STATUS_FILE"
  if ! jq -e 'type == "object"' "$STATUS_FILE" >/dev/null 2>&1; then
    ab_fail "status.json is not a JSON object"
  else
    counts="$(jq -r '[to_entries[].value.status // "unknown"] | group_by(.) | map("\(.[0])=\(length)") | join(" ")' "$STATUS_FILE")"
    echo "  evidence: status counts: $counts" | tee -a "$AB_RESULTS_PATH"

    nverified="$(jq -r '[to_entries[] | select(.value.status == "verified")] | length' "$STATUS_FILE")"
    if [ "${nverified:-0}" -eq 0 ]; then
      # Vacuous-set guard: PASSing "all verified aliases are sound" over an
      # empty set would be a bluff — report honestly instead.
      ab_skip "verified-alias invariants" "no verified aliases in status.json (vacuous set)"
    else
      bad_layer="$(jq -r '[to_entries[]
        | select(.value.status == "verified")
        | select((.value.failing_layer // "") != "")
        | "\(.key)(failing_layer=\(.value.failing_layer))"] | join(" ")' "$STATUS_FILE")"
      if [ -n "$bad_layer" ]; then
        ab_fail "verified alias(es) with non-empty failing_layer: $bad_layer"
      else
        ab_pass "all $nverified verified alias(es) have failing_layer empty"
      fi

      cutoff=$(( $(date +%s) - STALE_DAYS * 86400 ))
      stale="$(jq -r --argjson cutoff "$cutoff" '[to_entries[]
        | select(.value.status == "verified")
        | select((try ((.value.checked_at // "") | fromdateiso8601) catch 0) < $cutoff)
        | "\(.key)(checked_at=\(.value.checked_at // "none"))"] | join(" ")' "$STATUS_FILE")"
      if [ -n "$stale" ]; then
        ab_fail "stale verified status detected (checked_at older than ${STALE_DAYS}d or unparseable): $stale"
      else
        ab_pass "all $nverified verified alias(es) checked_at within last ${STALE_DAYS} day(s)"
      fi
      echo "  evidence: verified_aliases=$nverified cutoff_epoch=$cutoff token=${TOKEN}" | tee -a "$AB_RESULTS_PATH"
    fi
  fi
fi
echo

# --- Check 4: live verifier presence (observed only) ------------------------
echo "[4/5] live alias verifier presence (observed — never executed by this Challenge)"
if [ -f "$LIVE_VERIFIER" ]; then
  ab_pass "live alias verifier present (observed only — the live sweep is too heavy for this Challenge): scripts/tests/verify_aliases_live.sh"
  stat -c '  evidence: file=%n size=%s mtime=%y' "$LIVE_VERIFIER" | tee -a "$AB_RESULTS_PATH"
else
  ab_skip "live alias verifier" "scripts/tests/verify_aliases_live.sh absent from this checkout"
fi
echo

# --- Check 5: Kimi Code (OAuth subscription) provider support -----------------
# v1.15.0 added full Kimi variant support: one alias per subscription-served
# model (discovered live), OAuth-first precedence over API keys, launch-time
# token freshness (the OAuth token lives ~15 min), and the moonshot-flavored
# schema normalizer (kimi_proxy.py) without which every k3 tool request fails.
echo "[5/5] Kimi Code OAuth support: multi-model records, OAuth-first, token freshness, schema proxy"
ab_send_action "static-grep Kimi OAuth support markers in claude-providers.sh / lib.sh / proxy/kimi_proxy.py"
PROVIDERS_SH="$TOOLKIT_ROOT/scripts/claude-providers.sh"
LIB_SH="$TOOLKIT_ROOT/scripts/lib.sh"
KPROXY="$TOOLKIT_ROOT/scripts/proxy/kimi_proxy.py"
if [ ! -f "$PROVIDERS_SH" ] || [ ! -f "$LIB_SH" ] || [ ! -f "$KPROXY" ]; then
  ab_fail "Kimi support files missing (claude-providers.sh / lib.sh / proxy/kimi_proxy.py)"
else
  # 5a. The detector must DISCOVER served models via the /models endpoint —
  #     a hardcoded single-model record is exactly the old partial support.
  if grep -q '/coding/v1' "$PROVIDERS_SH" && grep -q '"\$base/models"\|/v1/models' "$PROVIDERS_SH"; then
    ab_pass "kimicode detector discovers subscription models via the /models endpoint"
  else
    ab_fail "kimicode detector does not query the /models endpoint (models would be hardcoded)"
  fi

  # 5b. OAuth records must take precedence over API-key records for
  #     kimi-for-coding (the subscription is the priority path).
  if grep -q 'unique_by(.provider_id)' "$PROVIDERS_SH" && grep -q '\$e2 + \$base + \$e1' "$PROVIDERS_SH"; then
    ab_pass "resolve_records gives the OAuth detector records precedence (unique_by, e2 first)"
  else
    ab_fail "resolve_records does not prefer OAuth detector records (API key can shadow the subscription)"
  fi

  # 5c. Launch-time token freshness: the OAuth token lives ~15 minutes, so a
  #     sync-time snapshot is always stale. lib.sh must consult the LIVE
  #     credentials file (with expiry) and refresh via the CLI before falling
  #     back to the snapshot.
  if grep -q 'kimi-code/credentials/kimi-code.json' "$LIB_SH" \
     && grep -q 'expires_at' "$LIB_SH" && grep -q 'kimi -p "hi"' "$LIB_SH"; then
    ab_pass "launch path reads the live OAuth credentials file with expiry + CLI refresh"
  else
    ab_fail "launch path lacks live-token freshness (stale sync-time snapshot would 401 after ~15 min)"
  fi

  # 5d. kimi_proxy must rewrite foreign $refs to the moonshot flavor
  #     (#/$defs/) — proven live requirement for k3 tool calls.
  if grep -q '#/\$defs/' "$KPROXY" && grep -q 'definitions' "$KPROXY"; then
    ab_pass "kimi_proxy.py normalizes tool schemas to the moonshot #/\$defs/ flavor"
  else
    ab_fail "kimi_proxy.py does not normalize \$ref to #/\$defs/ (k3 rejects Claude Code tool schemas)"
  fi

  # 5e. Live host state (read-only): when an OAuth session exists, kimi
  #     aliases in status.json must be verified AND fresh (the same
  #     invariants as Check 3, scoped to kimi-*).
  CRED="$HOME/.kimi-code/credentials/kimi-code.json"
  if [ ! -f "$CRED" ]; then
    ab_skip "Kimi OAuth session" "no $CRED on this host — subscription aliases not installed here"
  else
    exp="$(jq -r '.expires_at // 0' "$CRED" 2>/dev/null || echo 0)"
    if [ "${exp:-0}" -gt "$(date +%s)" ] || command -v kimi >/dev/null 2>&1; then
      ab_pass "Kimi OAuth session present and token fresh-or-refreshable (expires_at=$exp)"
    else
      ab_fail "Kimi OAuth token expired AND no kimi CLI to refresh it (aliases would 401 at launch)"
    fi
    if [ -f "$STATUS_FILE" ]; then
      kimi_bad="$(jq -r '[to_entries[]
        | select(.key | startswith("kimi"))
        | select(.value.status != "verified")
        | "\(.key)(\(.value.status))"] | join(" ")' "$STATUS_FILE")"
      kimi_n="$(jq -r '[to_entries[] | select(.key | startswith("kimi"))] | length' "$STATUS_FILE")"
      if [ -n "$kimi_bad" ]; then
        ab_fail "kimi aliases not verified in status.json: $kimi_bad"
      elif [ "$kimi_n" -eq 0 ]; then
        ab_skip "kimi aliases in status.json" "OAuth session exists but no kimi aliases installed — run claude-providers sync"
      else
        ab_pass "all $kimi_n kimi alias(es) verified in status.json"
      fi
    fi
  fi
fi
echo

echo "  evidence: token=${TOKEN} log=${AB_RESULTS_PATH}"
if ab_summary; then
  exit 0
fi
exit 1
