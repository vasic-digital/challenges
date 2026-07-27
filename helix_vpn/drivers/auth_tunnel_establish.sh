#!/usr/bin/env bash
# HVPN-CHAL-AUTH-TUNNEL — Auth + tunnel establishment driver (skeleton)
#
# WHAT IT PROVES:
#   A real device enrolls anonymously, receives a short-lived mTLS device cert,
#   authenticates the control channel, and establishes a WireGuard tunnel.
#   Captured evidence: auth+tunnel pcap + iperf3 JSON.
#
# Exit codes:
#   0  — PASS with evidence
#   1  — FAIL (real defect)
#   2  — usage / environment error
#   99 — paired mutation detected (anti-bluff OK)

set -euo pipefail

: "${GATEWAY_ENDPOINT:?}" "${ENROLLMENT_TOKEN:?}" "${TEST_LAN_HOST:?}"

OUT="qa-results/challenges/auth_tunnel/$(date +%s)"
mkdir -p "$OUT"
trap 'echo "[cleanup] preserving $OUT"' EXIT

echo "=== HVPN-CHAL-AUTH-TUNNEL ==="
echo "  gateway=$GATEWAY_ENDPOINT lan=$TEST_LAN_HOST out=$OUT"

# TODO: implement once helixvpnctl / helix-core CLI is available
# 1. Start pcap on client netns host iface.
# 2. helixvpnctl enroll --token "$ENROLLMENT_TOKEN" --out "$OUT/device.toml"
# 3. helixvpnctl connect --config "$OUT/device.toml"
# 4. Wait for WG handshake (wg show) and status Connected.
# 5. iperf3 -c "$TEST_LAN_HOST" -J > "$OUT/iperf3.json"
# 6. curl "$TEST_LAN_HOST" → assert HTTP 200.
# 7. Stop pcap → "$OUT/auth_tunnel.pcap".
# 8. Compose challenge outputs JSON.

cat > "$OUT/outputs.json" <<'EOF'
{
  "enroll_ok": "false",
  "handshake_ok": "false",
  "iperf_bps": 0,
  "reach_ok": "false",
  "evidence_pcap": "qa-results/challenges/auth_tunnel/auth_tunnel.pcap"
}
EOF

echo "[WARN] skeleton driver — outputs are placeholders; implement after CLI/rig is ready"
echo "=== HVPN-CHAL-AUTH-TUNNEL: SKEL ==="
