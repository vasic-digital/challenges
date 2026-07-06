#!/usr/bin/env bash
# HVPN-CHAL-CLIENT-UI-VISUAL — Client UI visual proof driver (skeleton)
set -euo pipefail
: "${APP_FLAVOR:?}" "${PLATFORM:?}"
OUT="qa-results/challenges/client_ui/$(date +%s)"; mkdir -p "$OUT"
echo "=== HVPN-CHAL-CLIENT-UI-VISUAL ==="
# TODO: implement panoptic/vision_engine recording + OCR/FSM verdict
cat > "$OUT/outputs.json" <<'EOF'
{
  "recording_path": "qa-results/challenges/client_ui/ui_connect.mp4",
  "vision_verdict": "FAIL",
  "fsm_matches_ui": "false",
  "no_overlap_overlay": "false"
}
EOF
echo "[WARN] skeleton driver — implement after vision_engine / panoptic integration is ready"
echo "=== HVPN-CHAL-CLIENT-UI-VISUAL: SKEL ==="
