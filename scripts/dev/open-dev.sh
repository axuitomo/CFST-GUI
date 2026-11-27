#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

mode="${1:-desktop}"

usage() {
  cat <<'EOF'
usage: scripts/dev/open-dev.sh [desktop|frontend|webui|go]

Starts a development server/process.

  desktop   Run wails3 dev -config build/config.yml.
  frontend  Run Vite dev server in packages/frontend/.
  webui     Run Go WebUI entrypoint ./apps/webui (embedded frontend; builds it first).
  go        Run go run ./apps/desktop with embedded assets (builds frontend first if missing).
EOF
}

# 内嵌模式（go/webui）直接把 packages/frontend/dist //go:embed 进二进制；若只有占位 .gitkeep
# （全新克隆或被清理过），先构建前端，避免跑起来是空白/旧界面。已构建则复用上次产物。
ensure_embedded_frontend() {
  if [[ ! -f "$FRONTEND_DIR/dist/index.html" ]]; then
    cfst_log "Embedded frontend missing; building packages/frontend/dist once"
    cfst_prepare_frontend
    (cd "$FRONTEND_DIR" && pnpm run build)
  else
    cfst_log "Reusing existing packages/frontend/dist; run 'pnpm --dir packages/frontend build' to refresh embedded assets"
  fi
}

case "$mode" in
  desktop)
    cfst_require_cmd wails3
    cd "$ROOT_DIR"
    exec wails3 dev -config build/config.yml
    ;;
  frontend)
    cd "$FRONTEND_DIR"
    exec pnpm run dev
    ;;
  webui)
    ensure_embedded_frontend
    cd "$ROOT_DIR"
    exec go run ./apps/webui
    ;;
  go)
    ensure_embedded_frontend
    cd "$ROOT_DIR"
    exec go run ./apps/desktop
    ;;
  -h|--help)
    usage
    ;;
  *)
    printf 'unknown dev mode: %s\n' "$mode" >&2
    usage >&2
    exit 2
    ;;
esac
