#!/usr/bin/env bash
# Pre-commit hook — runs scanner + manifest check on staged files.
# Mutation gate is excluded (too slow for pre-commit).
set -euo pipefail
# Resolve symlink chain so SCRIPT_DIR points at the actual install location,
# not at the symlink path under .git/hooks/ or .git/modules/<name>/hooks/.
src="${BASH_SOURCE[0]}"
while [[ -L "$src" ]]; do
  target="$(readlink "$src")"
  if [[ "$target" == /* ]]; then src="$target"; else src="$(cd "$(dirname "$src")" && pwd)/$target"; fi
done
SCRIPT_DIR="$(cd "$(dirname "$src")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Run scanner in changed-mode against staged files only.
"${SCRIPT_DIR}/bluff-scanner.sh" --mode changed

# Run anchor manifest check (cheap, < 1s).
if [[ -f "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh" ]]; then
  bash "${ROOT_DIR}/challenges/scripts/anchor_manifest_challenge.sh"
fi
