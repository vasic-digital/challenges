#!/usr/bin/env bash
# HVPN-CHAL-RECONNECT-ROAMING — Reconnect / roaming driver (skeleton)
set -euo pipefail
: "${CLIENT_NETNS:?}" "${DROP_ACTION:?}" "${TEST_LAN_HOST:?}"
OUT="qa-results/challenges/reconnect_roaming/$(date +%s)"; mkdir -p "$OUT"
echo "=== HVPN-CHAL-RECONNECT-ROAMING ==="
# TODO: implement netns rig drop + recovery capture
cat > "$OUT/outputs.json" <<'EOF'
{
  "tunnel_reestablished": "false",
  "transport_preserved": "false",
  "zero_leak_in_gap": "false",
  "recover_time_ms": 9999,
  "evidence_pcap": "qa-results/challenges/reconnect_roaming/roaming_reconnect.pcap"
}
EOF
echo "[WARN] skeleton driver — implement after orchestrator drop/recovery hooks are ready"
echo "=== HVPN-CHAL-RECONNECT-ROAMING: SKEL ==="
