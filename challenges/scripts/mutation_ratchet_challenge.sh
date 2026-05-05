#!/usr/bin/env bash
# CONST-035 mutation ratchet challenge (Go).
# Modes:
#   default (no --mode): run on changed files vs main.
#   --mode all: run full project (slow).
# Compares against challenges/baselines/bluff-baseline.txt Section 2.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MODE="changed"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 3 ;;
  esac
done

BASELINE="${ROOT_DIR}/challenges/baselines/bluff-baseline.txt"
CONFIG="${ROOT_DIR}/.go-mutesting.yml"

if ! command -v go-mutesting >/dev/null; then
  GOPATH_BIN="$(go env GOPATH)/bin"
  export PATH="${GOPATH_BIN}:${PATH}"
fi

if ! command -v go-mutesting >/dev/null; then
  echo "FAIL: go-mutesting not installed (run: go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest)" >&2
  exit 1
fi

# Detect default branch (main or master).
detect_default_branch() {
  local b
  for b in main master; do
    if git -C "${ROOT_DIR}" rev-parse --verify "$b" >/dev/null 2>&1; then
      echo "$b"; return 0
    fi
  done
  echo "main"
}
DEFAULT_BRANCH="$(detect_default_branch)"

OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

if [[ "$MODE" == "changed" ]]; then
  mapfile -t CHANGED < <(git -C "${ROOT_DIR}" diff --name-only "${DEFAULT_BRANCH}"...HEAD -- '*.go' 2>/dev/null || true)
  # Also include staged changes (for pre-commit / mid-PR runs).
  mapfile -t STAGED < <(git -C "${ROOT_DIR}" diff --name-only --cached -- '*.go' 2>/dev/null || true)
  CHANGED+=("${STAGED[@]}")
  # Filter out anti-bluff fixtures and removed files.
  FILTERED=()
  for f in "${CHANGED[@]}"; do
    [[ -z "$f" ]] && continue
    case "$f" in
      scripts/anti-bluff/tests/fixtures/*) continue ;;
    esac
    [[ -f "${ROOT_DIR}/$f" ]] || continue
    FILTERED+=("$f")
  done
  if (( ${#FILTERED[@]} == 0 )); then
    echo "OK: no Go changes vs ${DEFAULT_BRANCH}."
    exit 0
  fi
  PKGS=()
  for f in "${FILTERED[@]}"; do PKGS+=("./$(dirname "$f")"); done
  mapfile -t PKGS < <(printf '%s\n' "${PKGS[@]}" | sort -u)
  go-mutesting --config="${CONFIG}" "${PKGS[@]}" > "${OUT}" 2>&1 || true
else
  go-mutesting --config="${CONFIG}" ./... > "${OUT}" 2>&1 || true
fi

# Parse per-file kill rates.
# go-mutesting (avito-tech fork) emits lines like:
#   PASS "/tmp/.private/.../go-mutesting-NNN/<repo-path>/file.go.MM" with checksum ...
#   FAIL "/tmp/.private/.../go-mutesting-NNN/<repo-path>/file.go.MM" with checksum ...
# We extract the repo-relative path by stripping the temp-dir prefix and the .MM suffix.
python3 - "${OUT}" "${BASELINE}" "${MODE}" "${ROOT_DIR}" <<'PYEOF'
import collections, re, sys, os
out_path, baseline_path, mode, root = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]

# Pattern: PASS|FAIL "/tmp/.../go-mutesting-XXXX/REL/PATH/file.go.NN" ...
line_re = re.compile(r'^(PASS|FAIL)\s+"([^"]+\.go)\.\d+"')

killed = collections.Counter()
total = collections.Counter()

def normalize(path):
    # strip leading /tmp/.../go-mutesting-NNNN/ prefix to get repo-relative path
    m = re.search(r'/go-mutesting-[0-9]+/(.+)$', path)
    if m:
        return m.group(1)
    return path

with open(out_path) as f:
    for line in f:
        m = line_re.match(line)
        if not m: continue
        verdict, src = m.group(1), m.group(2)
        rel = normalize(src)
        total[rel] += 1
        if verdict == "FAIL":  # mutation killed (test caught it)
            killed[rel] += 1

baseline = {}
section = None
if os.path.exists(baseline_path):
    with open(baseline_path) as f:
        for line in f:
            line = line.rstrip("\n")
            if line.startswith("# === SECTION 2"):
                section = 2; continue
            if line.startswith("# === SECTION 3"):
                section = 3; continue
            if section == 2 and line and not line.startswith("#"):
                try:
                    parts = line.split(":")
                    if len(parts) >= 3:
                        p, r, n = parts[0], int(parts[1]), int(parts[2])
                        baseline[p] = r
                except (ValueError, IndexError):
                    continue

failed = False
overall_killed = 0
overall_total = 0
for fn in sorted(total.keys()):
    rate = 100 * killed[fn] // total[fn] if total[fn] else 0
    overall_killed += killed[fn]
    overall_total += total[fn]
    if mode == "changed" and rate < 90:
        print(f"FAIL: {fn} kill rate {rate}% < 90% (changed-code threshold)")
        failed = True
    if fn in baseline and rate < baseline[fn]:
        print(f"FAIL: {fn} kill rate {rate}% < baseline {baseline[fn]}% (ratchet)")
        failed = True

if mode == "all":
    overall_rate = 100 * overall_killed // overall_total if overall_total else 0
    print(f"INFO: overall kill rate {overall_rate}% ({overall_killed}/{overall_total})", file=sys.stderr)
    # Project-wide ratchet: 80%. But for sub-project 1 we just RECORD; the
    # baseline itself is what we ratchet against. We do not fail on
    # absolute < 80% if the baseline was captured at < 80% (sub-project
    # 4 is what reduces it). So this is informational only at this stage.

sys.exit(1 if failed else 0)
PYEOF

echo "OK: mutation ratchet (mode=${MODE})."
