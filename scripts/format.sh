#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_log "Formatting Go files"
mapfile -t go_files < <(
  find "$ROOT_DIR" -type f -name '*.go' \
    -not -path "$ROOT_DIR/build/*" \
    -not -path "$ROOT_DIR/frontend/node_modules/*" \
    -not -path "$ROOT_DIR/mobile/android/.gradle/*" \
    -not -path "$ROOT_DIR/mobile/android/app/build/*" \
    -not -path "$ROOT_DIR/mobile/android/build/*"
)

if ((${#go_files[@]} > 0)); then
  gofmt -w "${go_files[@]}"
fi

cfst_prepare_frontend

cfst_log "Formatting frontend files"
(cd "$FRONTEND_DIR" && npm run format)

cfst_log "Formatting completed"
