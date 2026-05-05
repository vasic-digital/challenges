#!/usr/bin/env bash
# Installs the anti-bluff pre-commit hook into .git/hooks/pre-commit.
# Idempotent.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Resolve the actual git directory (may be a file pointing elsewhere if this
# repo is a submodule of a larger project).
GIT_DIR="$(git -C "${ROOT_DIR}" rev-parse --git-dir)"
if [[ "${GIT_DIR}" != /* ]]; then
  GIT_DIR="${ROOT_DIR}/${GIT_DIR}"
fi

mkdir -p "${GIT_DIR}/hooks"
HOOK_TARGET="${GIT_DIR}/hooks/pre-commit"
HOOK_SOURCE="${SCRIPT_DIR}/pre-commit-hook.sh"

if [[ -e "${HOOK_TARGET}" && ! -L "${HOOK_TARGET}" ]]; then
  echo "Existing non-symlink pre-commit hook at ${HOOK_TARGET}; refusing to overwrite." >&2
  echo "Move it aside, then re-run." >&2
  exit 1
fi

ln -sf "${HOOK_SOURCE}" "${HOOK_TARGET}"
chmod +x "${HOOK_SOURCE}"
echo "Installed ${HOOK_TARGET} -> ${HOOK_SOURCE}"
