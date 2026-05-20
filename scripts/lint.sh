#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_log "Running go vet"
mapfile -t go_packages < <(cfst_go_packages)
(cd "$ROOT_DIR" && go vet "${go_packages[@]}")

if command -v shellcheck >/dev/null 2>&1; then
  cfst_log "Running shellcheck"
  mapfile -t shell_files < <(find "$ROOT_DIR/scripts" -type f -name '*.sh' | sort)
  if ((${#shell_files[@]} > 0)); then
    shellcheck "${shell_files[@]}"
  fi
else
  if [[ "${CFST_REQUIRE_SHELLCHECK:-0}" == "1" ]]; then
    printf 'shellcheck is required because CFST_REQUIRE_SHELLCHECK=1\n' >&2
    exit 1
  fi
  cfst_warn "shellcheck not found; skipping shell lint"
fi

cfst_prepare_frontend

cfst_log "Running frontend ESLint"
(cd "$FRONTEND_DIR" && npm run lint)

cfst_log "Lint checks completed"
