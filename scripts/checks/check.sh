#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cfst_generate_wails_module_if_possible

cfst_log "Running Go tests"
mapfile -t go_packages < <(cfst_go_packages)
(cd "$ROOT_DIR" && go test "${go_packages[@]}")

# 确认 desktop 与 webui 两端适配层均能编译并通过测试。
cfst_log "Running Go adapter tests"
(cd "$ROOT_DIR" && go test ./internal/adapters/...)

cfst_prepare_frontend

cfst_log "Running frontend boundary check"
(cd "$ROOT_DIR" && bash scripts/checks/frontend-boundary.sh)

cfst_log "Running frontend Vapor mode check"
(cd "$ROOT_DIR" && node scripts/checks/vapor-mode.mjs)

cfst_log "Running no-string-code check"
(cd "$ROOT_DIR" && node scripts/checks/no-string-code.mjs)

cfst_log "Running DNS provider kind consistency check"
(cd "$ROOT_DIR" && node scripts/checks/dns-provider-kinds.mjs)

cfst_log "Running frontend unit tests"
(cd "$FRONTEND_DIR" && pnpm run test)

cfst_log "Running frontend typecheck"
(cd "$FRONTEND_DIR" && pnpm run typecheck)

cfst_log "Running frontend production build"
(cd "$FRONTEND_DIR" && pnpm run build)

cfst_log "Running Cloudflare probe worker tests"
(cd "$ROOT_DIR" && pnpm run test:worker)

cfst_log "Project checks completed"
