#!/usr/bin/env bash
# HVPN-CHAL-KILL-SWITCH — Kill-switch no-leak driver (skeleton)
set -euo pipefail
: "${CLIENT_NETNS:?}" "${KILL_TARGET:?}"
OUT="qa-results/challenges/killswitch/$(date +%s)"; mkdir -p "$OUT"
echo "=== HVPN-CHAL-KILL-SWITCH ==="
# TODO: implement pcap during forced drop + leak analyzer
cat > "$OUT/outputs.json" <<'EOF'
{
  "kill_switch_active": "false",
  "zero_plaintext": "false",
  "zero_non_loopback": "false",
  "evidence_pcap": "qa-results/challenges/killswitch/killswitch_gap.pcap"
}
EOF
echo "[WARN] skeleton driver — implement after per-OS kill-switch firewall hooks are ready"
echo "=== HVPN-CHAL-KILL-SWITCH: SKEL ==="
