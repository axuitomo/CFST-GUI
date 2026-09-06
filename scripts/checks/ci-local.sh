#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cfst_prepare_frontend
export CFST_SKIP_PNPM_INSTALL=1

bash "$ROOT_DIR/scripts/checks/format-check.sh"
bash "$ROOT_DIR/scripts/checks/lint.sh"
bash "$ROOT_DIR/scripts/checks/check.sh"
bash "$ROOT_DIR/scripts/checks/verify-generated.sh"

if [[ "${CFST_SKIP_AUDIT:-0}" != "1" ]]; then
  bash "$ROOT_DIR/scripts/dev/audit.sh"
fi

cfst_log "Local CI completed"
