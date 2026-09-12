#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cfst_generate_wails_module_if_possible

cfst_log "Running Go tests"
mapfile -t go_packages < <(cfst_go_packages)
(cd "$ROOT_DIR" && go test "${go_packages[@]}")

# WebUI 构建标签不在默认构建中（发布流程用它构建 Linux WebUI 二进制），这里单独跑一次
# internal/app 的标签测试，避免 webui 路径只在发布时才被编译。
cfst_log "Running Go tests (webui build tag)"
(cd "$ROOT_DIR" && go test -tags webui ./internal/app/)

cfst_prepare_frontend

cfst_log "Running frontend unit tests"
(cd "$FRONTEND_DIR" && pnpm run test)

cfst_log "Running frontend typecheck"
(cd "$FRONTEND_DIR" && pnpm run typecheck)

cfst_log "Running frontend production build"
(cd "$FRONTEND_DIR" && pnpm run build)

cfst_log "Project checks completed"
