#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
TOOLCHAIN_DIR="${CFST_ANDROID_TOOLCHAIN_DIR:-$CACHE_HOME/cfst-gui/android-toolchain}"
SDK_DIR="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$TOOLCHAIN_DIR/android-sdk}}"
NDK_DIR="${ANDROID_NDK_HOME:-$SDK_DIR/ndk/29.0.14206865}"
GOMOBILE_BIN="${GOMOBILE_BIN:-$(go env GOPATH)/bin/gomobile}"
GOMOBILE_TIMEOUT_SECONDS="${CFST_GOMOBILE_TIMEOUT_SECONDS:-1800}"
GOMOBILE_CGO_ENABLED="${CFST_GOMOBILE_CGO_ENABLED:-0}"
ANDROID_16K_LDFLAGS='-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'

require_file() {
  local path="$1"
  local message="$2"
  if [[ ! -f "$path" ]]; then
    echo "$message: $path" >&2
    exit 1
  fi
}

clean_gomobile_temp() {
  local -a temp_roots=()
  temp_roots+=("${TMPDIR:-}")
  temp_roots+=("${TEMP:-}")
  temp_roots+=("${TMP:-}")
  temp_roots+=("${LOCALAPPDATA:-}/Temp")
  temp_roots+=("${USERPROFILE:-}/AppData/Local/Temp")
  temp_roots+=("/tmp")
  local temp_root
  for temp_root in "${temp_roots[@]}"; do
    if [[ -n "$temp_root" && -d "$temp_root" ]]; then
      find "$temp_root" -maxdepth 1 -type d -name 'gomobile-*' -exec rm -rf {} + 2>/dev/null || true
    fi
  done
}

run_gomobile_bind() {
  clean_gomobile_temp
  export CGO_ENABLED="$GOMOBILE_CGO_ENABLED"
  local -a command=("$GOMOBILE_BIN" bind -androidapi 21 -target=android/arm64 -ldflags "$ANDROID_16K_LDFLAGS" -o "$ANDROID_DIR/app/libs/mobileapi.aar" github.com/axuitomo/CFST-GUI/mobileapi)
  if command -v timeout >/dev/null 2>&1; then
    timeout --signal=TERM "${GOMOBILE_TIMEOUT_SECONDS}s" "${command[@]}"
  else
    "${command[@]}"
  fi
  clean_gomobile_temp
}

export ANDROID_HOME="$SDK_DIR"
export ANDROID_SDK_ROOT="$SDK_DIR"
export ANDROID_NDK_HOME="$NDK_DIR"

if [[ ! -x "$GOMOBILE_BIN" ]]; then
  echo "gomobile not found at $GOMOBILE_BIN; run: go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260821190718-4776eadac327" >&2
  exit 1
fi

cfst_generate_wails_module_if_possible

cd "$FRONTEND_DIR"
pnpm run build
pnpm exec cap sync android
bash "$ROOT_DIR/scripts/check-android-fileprovider-resources.sh"
bash "$ROOT_DIR/scripts/patch-android-gradle-warnings.sh"

mkdir -p "$ANDROID_DIR/app/libs"
run_gomobile_bind

cd "$ANDROID_DIR"
bash ./gradlew assembleDebug

DEBUG_ARM64_APK="$ANDROID_DIR/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk"
require_file "$ANDROID_DIR/app/libs/mobileapi.aar" "Android debug AAR not found"
require_file "$DEBUG_ARM64_APK" "Android arm64 debug APK not found"

bash "$ROOT_DIR/scripts/check-android-page-alignment.sh" \
  "$ANDROID_DIR/app/libs/mobileapi.aar" \
  "$DEBUG_ARM64_APK"

bash "$ROOT_DIR/scripts/check-android-apk-manifest.sh" \
  "$DEBUG_ARM64_APK"
