#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
TOOLCHAIN_DIR="${CFST_ANDROID_TOOLCHAIN_DIR:-$CACHE_HOME/cfst-gui/android-toolchain}"
SDK_DIR="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$TOOLCHAIN_DIR/android-sdk}}"
NDK_DIR="${ANDROID_NDK_HOME:-$SDK_DIR/ndk/29.0.14206865}"
GOMOBILE_BIN="${GOMOBILE_BIN:-$(go env GOPATH)/bin/gomobile}"
ANDROID_16K_LDFLAGS='-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'

require_file() {
  local path="$1"
  local message="$2"
  if [[ ! -f "$path" ]]; then
    echo "$message: $path" >&2
    exit 1
  fi
}

export ANDROID_HOME="$SDK_DIR"
export ANDROID_SDK_ROOT="$SDK_DIR"
export ANDROID_NDK_HOME="$NDK_DIR"

if [[ ! -x "$GOMOBILE_BIN" ]]; then
  echo "gomobile not found at $GOMOBILE_BIN; run: go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b" >&2
  exit 1
fi

cd "$FRONTEND_DIR"
pnpm run build
pnpm exec cap sync android
bash "$ROOT_DIR/scripts/patch-android-gradle-warnings.sh"

mkdir -p "$ANDROID_DIR/app/libs"
"$GOMOBILE_BIN" bind \
  -androidapi 21 \
  -target=android/arm64,android/arm \
  -ldflags "$ANDROID_16K_LDFLAGS" \
  -o "$ANDROID_DIR/app/libs/mobileapi.aar" \
  github.com/axuitomo/CFST-GUI/mobileapi

cd "$ANDROID_DIR"
./gradlew assembleDebug

DEBUG_ARM64_APK="$ANDROID_DIR/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk"
DEBUG_ARM_APK="$ANDROID_DIR/app/build/outputs/apk/debug/app-armeabi-v7a-debug.apk"
DEBUG_UNIVERSAL_APK="$ANDROID_DIR/app/build/outputs/apk/debug/app-universal-debug.apk"

require_file "$ANDROID_DIR/app/libs/mobileapi.aar" "Android debug AAR not found"
require_file "$DEBUG_ARM64_APK" "Android arm64 debug APK not found"
require_file "$DEBUG_ARM_APK" "Android armv7 debug APK not found"
require_file "$DEBUG_UNIVERSAL_APK" "Android universal debug APK not found"

bash "$ROOT_DIR/scripts/check-android-page-alignment.sh" \
  "$ANDROID_DIR/app/libs/mobileapi.aar" \
  "$DEBUG_ARM64_APK" \
  "$DEBUG_ARM_APK" \
  "$DEBUG_UNIVERSAL_APK"

bash "$ROOT_DIR/scripts/check-android-apk-manifest.sh" \
  "$DEBUG_ARM64_APK" \
  "$DEBUG_ARM_APK" \
  "$DEBUG_UNIVERSAL_APK"
