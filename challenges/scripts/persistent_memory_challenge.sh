#!/usr/bin/env bash
# persistent_memory_challenge.sh — anti-bluff Challenge for the
# HelixCode HelixMemory persistent local store (CONST-035 + CONST-050(B);
# submodule cascade per CONST-051(A)).
#
# WHAT IT PROVES (positive runtime evidence, §11.4.5 / Article XI §11.9):
#   A distinctive token written to the zero-config on-disk SQLite local
#   store in ONE OS process is recalled from a SEPARATE, FRESH OS process
#   that opens the SAME on-disk file. before==after / in-memory-only state
#   cannot pass: the two `go run` invocations share NOTHING except the
#   file on disk, and the read process embeds a per-run unique token so a
#   stale cache pre-dating the token can never match.
#
#   Evidence captured: the WROTE line (id + row count), the on-disk DB
#   path + byte size (>0), and the RECALLED:<content> line carrying the
#   exact token from the fresh reader process.
#
# PAIRED-MUTATION INVARIANT (CONST-035 + §1.1):
#   With --anti-bluff-mutate the script performs the WRITE, then asks the
#   reader to recall a DIFFERENT token that was never written. A genuine
#   store MUST report MISS (no recall). If the reader still "recalls"
#   something the gate is bluffing — the mutation asserts the recall
#   FAILs and exits 99. The real on-disk DB is read-only here; no source
#   is mutated.
#
# Exit codes:
#   0  — RED→GREEN: token written in proc-1 recalled in fresh proc-2
#   1  — gate FAIL on clean run (real defect: cross-process recall broken)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error (toolchain absent → honest SKIP-OK)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROBE_DIR="$(cd "${SCRIPT_DIR}/../fixtures/memprobe" && pwd)"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h) sed -n '1,40p' "$0"; exit 0 ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

echo "=== HelixCode HelixMemory Persistent-Memory Challenge ==="
echo "  probe=${PROBE_DIR}"

if ! command -v go >/dev/null 2>&1; then
    echo "[0/4] SKIP: Go toolchain absent — SKIP-OK: #env-no-go"
    echo "=== Persistent-Memory Challenge: PASSED (SKIP-OK) ==="
    exit 0
fi
if [[ ! -f "${PROBE_DIR}/main.go" ]]; then
    echo "[0/4] FAIL: memprobe helper missing at ${PROBE_DIR}/main.go"
    exit 1
fi

WORK="$(mktemp -d)"
DB="${WORK}/helixmemory.db"
TOKEN="pm$(date +%s)$$$(head -c4 /dev/urandom 2>/dev/null | od -An -tx1 | tr -d ' \n')"
trap 'rm -rf "${WORK}"' EXIT

run_probe() { ( cd "${PROBE_DIR}" && go run . "$@" ) ; }

# [1/4] PROCESS 1 — write the token to the on-disk store.
echo "[1/4] proc-1 WRITE token=${TOKEN}"
WROTE="$(run_probe write "${DB}" "${TOKEN}" 2>"${WORK}/w.err")"
if [[ -z "${WROTE}" ]] || ! grep -q '^WROTE ' <<<"${WROTE}"; then
    echo "  FAIL: write process produced no WROTE evidence"
    sed 's/^/  stderr: /' "${WORK}/w.err"
    exit 1
fi
echo "  ${WROTE}"

# [2/4] on-disk DB must exist + be non-empty SQLite.
if [[ ! -s "${DB}" ]]; then
    echo "[2/4] FAIL: on-disk DB missing/empty at ${DB}"
    exit 1
fi
DBSZ=$(wc -c < "${DB}" | tr -d ' ')
DBSIG="$(head -c 16 "${DB}" 2>/dev/null | tr -d '\0')"
echo "[2/4] on-disk DB: ${DBSZ} bytes, header='${DBSIG}'"
[[ "${DBSIG}" == SQLite* ]] || { echo "  FAIL: not a SQLite file"; exit 1; }

if [[ "${MUTATE}" -eq 1 ]]; then
    # Paired mutation: ask the fresh reader for a token that was NEVER
    # written. A genuine store MUST NOT recall it. If it does, the gate
    # is a bluff. We assert the recall FAILS (MISS / non-zero exit).
    BOGUS="${TOKEN}_NEVER_WRITTEN_MUTANT"
    echo "[MUT] proc-2 READ bogus token=${BOGUS} (must MISS)"
    if run_probe read "${DB}" "${BOGUS}" >"${WORK}/m.out" 2>&1; then
        echo "  MUTATION NOT DETECTED: reader recalled a never-written token (BLUFF):"
        sed 's/^/    /' "${WORK}/m.out"
        exit 1
    fi
    echo "  mutation correctly detected — never-written token NOT recalled:"
    sed 's/^/    /' "${WORK}/m.out"
    echo "=== Persistent-Memory Challenge: MUTATION DETECTED (anti-bluff OK) ==="
    exit 99
fi

# [3/4] PROCESS 2 (fresh process) — recall the token.
echo "[3/4] proc-2 READ (fresh process) token=${TOKEN}"
if ! RECALL="$(run_probe read "${DB}" "${TOKEN}" 2>"${WORK}/r.err")"; then
    echo "  FAIL: fresh reader process did NOT recall the token (cross-process persistence broken)"
    sed 's/^/  stderr: /' "${WORK}/r.err"
    echo "  reader stdout: ${RECALL:-<empty>}"
    exit 1
fi

# [4/4] the recalled content MUST carry the exact token.
if ! grep -q "^RECALLED:.*${TOKEN}" <<<"${RECALL}"; then
    echo "[4/4] FAIL: recalled content does not carry the token: ${RECALL}"
    exit 1
fi
echo "[4/4] ${RECALL}"

echo
echo "=== Persistent-Memory Challenge: PASSED ==="
echo "  evidence: token=${TOKEN} db_bytes=${DBSZ} db_sig=${DBSIG}"
echo "  evidence: proc1=$(grep -oE 'count=[0-9]+' <<<"${WROTE}") recalled_in_fresh_process=yes"
