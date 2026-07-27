#!/usr/bin/env bash
# HVPN-CHAL-DNS-LEAK — DNS leak prevention driver (skeleton)
set -euo pipefail
: "${CLIENT_NETNS:?}" "${TEST_DOMAINS:?}"
OUT="qa-results/challenges/dns_leak/$(date +%s)"; mkdir -p "$OUT"
echo "=== HVPN-CHAL-DNS-LEAK ==="
# TODO: implement DNS resolution through tunnel + off-tunnel :53 leak check
cat > "$OUT/outputs.json" <<'EOF'
{
  "dns_resolved": "false",
  "zero_plaintext_53": "false",
  "tunnel_dns_used": "false",
  "evidence_pcap": "qa-results/challenges/dns_leak/dns_leak.pcap"
}
EOF
echo "[WARN] skeleton driver — implement after tunnel DNS resolver wiring is ready"
echo "=== HVPN-CHAL-DNS-LEAK: SKEL ==="
