#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_generate_wails_module_if_possible

cfst_log "Running Go tests"
mapfile -t go_packages < <(cfst_go_packages)
(cd "$ROOT_DIR" && go test "${go_packages[@]}")

cfst_prepare_frontend

cfst_log "Running frontend typecheck"
(cd "$FRONTEND_DIR" && npm run typecheck)

cfst_log "Running frontend production build"
(cd "$FRONTEND_DIR" && npm run build)

cfst_log "Project checks completed"
