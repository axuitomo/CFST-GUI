#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

mode="${1:-desktop}"

usage() {
  cat <<'EOF'
usage: scripts/open-dev.sh [desktop|frontend|webui|go]

Starts a development server/process.

Modes:
  desktop   Run wails dev.
  frontend  Run Vite dev server in frontend/.
  webui     Run Go WebUI mode with -tags webui.
  go        Run go run . with embedded assets.
EOF
}

case "$mode" in
  desktop)
    cfst_require_cmd wails
    cd "$ROOT_DIR"
    exec wails dev
    ;;
  frontend)
    cd "$FRONTEND_DIR"
    exec pnpm run dev
    ;;
  webui)
    cd "$ROOT_DIR"
    exec go run -tags webui .
    ;;
  go)
    cd "$ROOT_DIR"
    exec go run .
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
