#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

if (($# > 0)); then
  aar_path="$1"
  shift
  cfst_log "Checking Android 16KB page alignment for supplied artifacts"
  bash "$ROOT_DIR/scripts/check-android-page-alignment.sh" "$aar_path" "$@"
  cfst_log "Checking Android APK manifest invariants for supplied artifacts"
  bash "$ROOT_DIR/scripts/check-android-apk-manifest.sh" "$@"
else
  cfst_log "Building Android debug artifacts and checking 16KB page alignment"
  bash "$ROOT_DIR/scripts/build-android-mobile.sh"
  cfst_log "Checking Android APK manifest invariants for built debug artifacts"
  bash "$ROOT_DIR/scripts/check-android-apk-manifest.sh" \
    "$ANDROID_DIR/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk" \
    "$ANDROID_DIR/app/build/outputs/apk/debug/app-armeabi-v7a-debug.apk" \
    "$ANDROID_DIR/app/build/outputs/apk/debug/app-universal-debug.apk"
fi
