#!/usr/bin/env bash
# HVPN-CHAL-CONTROL-PLANE-HA — Control-plane fail-static driver (skeleton)
set -euo pipefail
: "${COORDINATOR_PID:?}" "${TEST_LAN_HOST:?}"
OUT="qa-results/challenges/control_plane_ha/$(date +%s)"; mkdir -p "$OUT"
echo "=== HVPN-CHAL-CONTROL-PLANE-HA ==="
# TODO: implement CP stop/start while tunnel carries traffic + delta resume check
cat > "$OUT/outputs.json" <<'EOF'
{
  "tunnel_survived": "false",
  "zero_tunnel_drop": "false",
  "delta_resumed": "false",
  "evidence_pcap": "qa-results/challenges/control_plane_ha/ha_failstatic.pcap"
}
EOF
echo "[WARN] skeleton driver — implement after coordinator graceful-stop hooks are ready"
echo "=== HVPN-CHAL-CONTROL-PLANE-HA: SKEL ==="
