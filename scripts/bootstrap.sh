#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

install_tools=0
android=0
run_check=0
skip_wails_generate=0

usage() {
  cat <<'EOF'
usage: scripts/bootstrap.sh [--install-tools] [--android] [--check] [--skip-wails-generate]

Initializes a fresh checkout for local development.

Options:
  --install-tools         Install Wails and, with --android, gomobile.
  --android               Also prepare Android/Capacitor bridge pieces.
  --check                 Run scripts/check.sh after bootstrap.
  --skip-wails-generate   Skip Wails bridge generation.
EOF
}

while (($# > 0)); do
  case "$1" in
    --install-tools)
      install_tools=1
      ;;
    --android)
      android=1
      ;;
    --check)
      run_check=1
      ;;
    --skip-wails-generate)
      skip_wails_generate=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

cfst_require_cmd go
cfst_require_cmd npm

if ((install_tools)); then
  cfst_log "Installing Wails CLI"
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
elif ! command -v wails >/dev/null 2>&1; then
  cfst_warn "wails is missing; rerun with --install-tools or install it manually"
fi

cfst_prepare_frontend

if ((skip_wails_generate == 0)); then
  cfst_generate_wails_module_if_possible
fi

if ((android)); then
  cfst_log "Preparing Capacitor Android project"
  (cd "$FRONTEND_DIR" && npx cap sync android)
  bash "$ROOT_DIR/scripts/patch-android-gradle-warnings.sh"

  if ((install_tools)); then
    cfst_log "Installing gomobile"
    go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b
    "$(go env GOPATH)/bin/gomobile" init
  elif ! command -v gomobile >/dev/null 2>&1 && [[ ! -x "$(go env GOPATH)/bin/gomobile" ]]; then
    cfst_warn "gomobile is missing; Android Go bridge builds will fail until it is installed"
  fi
fi

if ((run_check)); then
  bash "$ROOT_DIR/scripts/check.sh"
fi

cfst_log "Bootstrap completed"
