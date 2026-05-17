# Challenge: P1-F06 — MCP Full Lifecycle

## Purpose

Prove that HelixCode's client-side MCP support actually connects to a real
external MCP server, performs the JSON-RPC handshake (initialize →
notifications/initialized → tools/list), and successfully invokes a tool.
Per Article XI §11.9, every PASS must carry positive runtime evidence.

## Procedure

1. Build `bin/helixcode`.
2. Build the test echo MCP server (a small Go program that speaks MCP over
   stdio and replies to every request with empty result, except tools/list
   which returns one real tool).
3. Write `.helixcode/mcp.yml` declaring the echo server with
   `transport: stdio` and `alwaysLoad: true`.
4. Run `helixcode mcp test echo` — assert exit code 0 and stdout contains
   "ready".
5. Run `helixcode mcp list` — assert table includes "echo" with transport
   "stdio".
6. Anti-bluff smoke: `grep -rn "simulated\|for now\|TODO implement\|placeholder"
   helix_code/internal/mcp/` returns empty.
7. Cross-compile (Linux only — Windows has pre-existing CGO failures unrelated
   to F06: `cd HelixCode && go build ./cmd/cli/... ./internal/mcp/...`).

## Pass criteria

- Step 4 stdout contains "ready" (no "simulated", no "for now").
- Step 5 stdout contains the server name and transport.
- Step 6 returns clean.
- Step 7 produces both binaries.
