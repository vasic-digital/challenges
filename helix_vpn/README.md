# HelixVPN Challenge Suite

**Module:** `digital.vasic.challenges`  
**Project:** HelixVPN  
**Purpose:** Anti-bluff Challenge bank for the HelixVPN MVP. Every challenge in this directory scores a PASS only on captured runtime evidence — a pcap, a throughput sample, a counter delta, or a rendered frame. Exit codes and config-only assertions are not sufficient.

---

## What this suite proves

This bank covers the six critical paths of the HelixVPN MVP:

1. **Auth + tunnel establishment** — a real device enrolls, authenticates with mTLS, and establishes a WireGuard tunnel.
2. **Reconnect / roaming** — a transient carrier drop or interface change re-establishes the tunnel without user action and without leaking plaintext.
3. **Kill-switch** — when the tunnel drops, the OS firewall blocks all off-tunnel traffic; a pcap proves zero plaintext egress.
4. **DNS leak prevention** — DNS queries are forced through the tunnel; no plaintext `:53` packet leaves the host.
5. **Control-plane HA / fail-static** — when the control plane is stopped, existing tunnels keep forwarding.
6. **Client UI visual proof** — the Connect button, status transitions, and one-button connect flow are verified by recorded evidence + vision verdict.

Each challenge is a `challenges` bank entry that can be loaded by `pkg/bank` and executed by `pkg/runner`. The drivers are intended to be implemented under `rig/` in the HelixVPN project; this bank defines the contract those drivers must satisfy.

---

## File layout

```
submodules/challenges/helix_vpn/
├── README.md
├── helix_vpn_challenges.json   # Full bank in JSON (pkg/bank compatible)
├── helix_vpn_challenges.yaml   # Full bank in YAML (pkg/bank compatible)
└── drivers/                    # Bash driver skeletons (project-specific)
    ├── auth_tunnel_establish.sh
    ├── reconnect_roaming.sh
    ├── kill_switch.sh
    ├── dns_leak.sh
    ├── control_plane_ha.sh
    └── client_ui_visual.sh
```

---

## Challenge IDs

| ID | Critical path | Evidence artifact | Bound DoD / Gate |
|---|---|---|---|
| `HVPN-CHAL-AUTH-TUNNEL` | Auth + tunnel establishment | `auth_tunnel.pcap`, `iperf3.json` | DoD-2 |
| `HVPN-CHAL-RECONNECT-ROAMING` | Reconnect / roaming | `roaming_reconnect.pcap`, `status_log.json` | — |
| `HVPN-CHAL-KILL-SWITCH` | Kill-switch no leak | `killswitch_gap.pcap` | DoD-7 |
| `HVPN-CHAL-DNS-LEAK` | DNS leak prevention | `dns_leak.pcap`, `dns_resolvers.json` | DoD-7 |
| `HVPN-CHAL-CONTROL-PLANE-HA` | Control-plane HA / fail-static | `ha_failstatic.pcap`, `cp_down.log` | — |
| `HVPN-CHAL-CLIENT-UI-VISUAL` | Client UI visual proof | `ui_connect.mp4`, `vision_verdict.json` | DoD-8, §11.4.170 |

---

## Anti-bluff rules

Every challenge enforces the following, derived from `docs/research/mvp/final/10-testing-acceptance-and-qa.md` §0 / §3:

1. **Captured evidence only.** The PASS cites a file path to a real artifact produced while the feature ran.
2. **Liveness over a window.** Throughput is sampled over ≥ 10 s; a one-shot `ping` is a pre-filter, never the proof.
3. **Independent counter-advance.** `iperf3` goodput and the kernel WG counter (`wg show`) must both advance.
4. **Self-validated analyzers.** Every pcap classifier / leak detector ships with a golden-good + golden-bad fixture pair.
5. **Paired §1.1 mutation.** For every gate, a mutation that disables the protection must make the challenge FAIL.

---

## Running a challenge

```bash
# From the HelixVPN project root
make challenges-helix_vpn
# Or directly via the challenges CLI
go run submodules/challenges/cmd/challenges/main.go \
  --bank submodules/challenges/helix_vpn/helix_vpn_challenges.yaml \
  --challenge HVPN-CHAL-KILL-SWITCH
```

---

## References

- `docs/research/mvp/final/10-testing-acceptance-and-qa.md` — canonical QA contract.
- `docs/research/mvp/final/implementation/09-testing-qa/coverage-ledger.md` — full FR/NFR × test-type mapping.
- `submodules/helix_qa/banks/helix_vpn/` — paired HelixQA autonomous-session bank.
- Constitution: §11.4.27, §11.4.69, §11.4.107, §11.4.115, §11.4.169.
