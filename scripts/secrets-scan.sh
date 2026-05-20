#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_require_cmd rg

pattern='-----BEGIN ([A-Z]+ )?PRIVATE KEY-----|github_pat_[A-Za-z0-9_]+|gh[pousr]_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16}|(?i)(password|passwd|secret|api[_-]?key|access[_-]?key)\s*[:=]\s*["'\''][^"'\''[:space:]]{8,}["'\'']'

cfst_log "Scanning for likely committed secrets"

set +e
matches="$(
  rg -n -I --hidden --no-heading --with-filename \
    -g '!.git/**' \
    -g '!frontend/node_modules/**' \
    -g '!frontend/dist/**' \
    -g '!build/**' \
    -g '!mobile/android/.gradle/**' \
    -g '!mobile/android/app/build/**' \
    -g '!mobile/android/build/**' \
    -g '!frontend/package-lock.json' \
    -g '!go.sum' \
    -g '!scripts/secrets-scan.sh' \
    -e "$pattern" "$ROOT_DIR"
)"
status=$?
set -e

if [[ "$status" -eq 1 ]]; then
  cfst_log "No likely secrets found"
  exit 0
fi

if [[ "$status" -ne 0 ]]; then
  printf 'secret scan failed with rg exit code %s\n' "$status" >&2
  exit "$status"
fi

printf '%s\n' "$matches" | awk -F: '!seen[$1 ":" $2]++ {print $1 ":" $2 ": possible secret pattern"}' >&2
printf '\nPotential secrets found. Review the file/line locations above; values were intentionally not printed.\n' >&2
exit 1
