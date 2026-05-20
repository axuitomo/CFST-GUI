#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"

cfst_log() {
  printf '\n==> %s\n' "$*"
}

cfst_warn() {
  printf 'warning: %s\n' "$*" >&2
}

cfst_require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "$cmd" >&2
    exit 1
  fi
}

cfst_prepare_frontend() {
  if [[ "${CFST_SKIP_NPM_CI:-0}" == "1" ]]; then
    cfst_log "Skipping frontend npm ci because CFST_SKIP_NPM_CI=1"
    return
  fi

  cfst_log "Installing frontend dependencies with npm ci"
  (cd "$FRONTEND_DIR" && npm ci)
}

cfst_generate_wails_module_if_possible() {
  if [[ "${CFST_SKIP_WAILS_GENERATE:-0}" == "1" ]]; then
    cfst_log "Skipping Wails module generation because CFST_SKIP_WAILS_GENERATE=1"
    return
  fi

  if command -v wails >/dev/null 2>&1; then
    cfst_log "Generating Wails frontend bridge"
    (cd "$ROOT_DIR" && wails generate module)
    return
  fi

  if [[ -d "$FRONTEND_DIR/wailsjs" ]]; then
    cfst_warn "wails command not found; using existing frontend/wailsjs"
    return
  fi

  printf 'wails command not found and frontend/wailsjs is missing. Install Wails or run with CFST_SKIP_WAILS_GENERATE=1 only when bridge files already exist.\n' >&2
  exit 1
}

cfst_go_packages() {
  (cd "$ROOT_DIR" && go list ./... | awk '$0 !~ /\/frontend\/node_modules(\/|$)/')
}

cfst_default_version() {
  sed -n 's/^VERSION="${CFST_VERSION:-\([^}"]*\)}".*/\1/p' "$ROOT_DIR/scripts/build-release.sh" | head -n 1
}

cfst_android_default_version_code() {
  sed -n 's/.*cfstAndroidVersionCode = .* ? \([0-9][0-9]*\) : .*/\1/p' "$ANDROID_DIR/app/build.gradle" | head -n 1
}

cfst_sha256() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return
  fi
  shasum -a 256 "$path" | awk '{print $1}'
}

cfst_human_size() {
  local path="$1"
  if command -v numfmt >/dev/null 2>&1; then
    stat -c '%s' "$path" | numfmt --to=iec --suffix=B
    return
  fi
  stat -c '%s' "$path"
}
