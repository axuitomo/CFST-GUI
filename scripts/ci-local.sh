#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_prepare_frontend
export CFST_SKIP_NPM_CI=1

bash "$ROOT_DIR/scripts/format-check.sh"
bash "$ROOT_DIR/scripts/lint.sh"
bash "$ROOT_DIR/scripts/check.sh"
bash "$ROOT_DIR/scripts/verify-generated.sh"

if [[ "${CFST_SKIP_AUDIT:-0}" != "1" ]]; then
  bash "$ROOT_DIR/scripts/audit.sh"
fi

cfst_log "Local CI completed"
