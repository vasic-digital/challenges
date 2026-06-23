#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT/HelixCode"

echo "==> build bin/helixcode (server)"
make build

echo "==> build bin/cli (CLI with mcp subcommand)"
go build -o bin/cli ./cmd/cli

echo "==> build echo MCP server"
ECHO_BIN="$(mktemp -d)/echo-mcp"
go build -o "$ECHO_BIN" ./internal/mcp/testhelper_echo_server

echo "==> write mcp.yml"
mkdir -p .helixcode
cat > .helixcode/mcp.yml <<EOF
servers:
  - name: echo
    transport: stdio
    command: ["$ECHO_BIN"]
    alwaysLoad: true
EOF

echo "==> helixcode mcp list"
./bin/cli mcp list | tee /tmp/p1f06-list.txt  # bluff-scan: ok (grep -q echo/stdio on next lines assert the captured output; set -e active)
grep -q "echo" /tmp/p1f06-list.txt
grep -q "stdio" /tmp/p1f06-list.txt

echo "==> helixcode mcp test echo"
./bin/cli mcp test echo | tee /tmp/p1f06-test.txt  # bluff-scan: ok (grep -q ready + ! grep -qi simulated on next lines assert the captured output; set -e active)
grep -q "ready" /tmp/p1f06-test.txt
! grep -qi "simulated\|for now" /tmp/p1f06-test.txt

echo "==> anti-bluff smoke on internal/mcp/"
if grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/mcp/; then
    echo "BLUFF FOUND" >&2
    exit 1
fi
echo "clean"

echo "==> cross-compile (linux native)"
go build ./cmd/cli/... ./internal/mcp/...

echo "==> P1-F06 challenge PASS"
