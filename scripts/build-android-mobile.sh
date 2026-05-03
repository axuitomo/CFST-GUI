#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
TOOLCHAIN_DIR="${CFST_ANDROID_TOOLCHAIN_DIR:-$CACHE_HOME/cfst-gui/android-toolchain}"
SDK_DIR="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$TOOLCHAIN_DIR/android-sdk}}"
NDK_DIR="${ANDROID_NDK_HOME:-$SDK_DIR/ndk/26.3.11579264}"
GOMOBILE_BIN="${GOMOBILE_BIN:-$(go env GOPATH)/bin/gomobile}"

export ANDROID_HOME="$SDK_DIR"
export ANDROID_SDK_ROOT="$SDK_DIR"
export ANDROID_NDK_HOME="$NDK_DIR"

if [[ ! -x "$GOMOBILE_BIN" ]]; then
  echo "gomobile not found at $GOMOBILE_BIN; run: go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b" >&2
  exit 1
fi

cd "$FRONTEND_DIR"
npm run build
npx cap sync android

mkdir -p "$ANDROID_DIR/app/libs"
"$GOMOBILE_BIN" bind \
  -androidapi 21 \
  -target=android/arm64,android/arm \
  -o "$ANDROID_DIR/app/libs/mobileapi.aar" \
  github.com/XIU2/CloudflareSpeedTest/mobileapi

cd "$ANDROID_DIR"
./gradlew assembleDebug
