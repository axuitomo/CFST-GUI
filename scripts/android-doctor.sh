#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_log "Checking Android development environment"
bash "$ROOT_DIR/scripts/doctor.sh" --android "$@"

cfst_log "Checking Android project files"
for path in \
  "$ANDROID_DIR/settings.gradle" \
  "$ANDROID_DIR/build.gradle" \
  "$ANDROID_DIR/app/build.gradle" \
  "$ANDROID_DIR/gradle/wrapper/gradle-wrapper.properties"; do
  if [[ -f "$path" ]]; then
    printf 'ok      %s\n' "$path"
  else
    printf 'missing %s\n' "$path" >&2
    exit 1
  fi
done

sdk_dir="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$ROOT_DIR/.android-toolchain/android-sdk}}"
ndk_dir="${ANDROID_NDK_HOME:-$sdk_dir/ndk/26.3.11579264}"
if [[ ! -d "$ndk_dir" && -d "$ROOT_DIR/.android-toolchain/android-ndk-r26c" ]]; then
  ndk_dir="$ROOT_DIR/.android-toolchain/android-ndk-r26c"
fi

if [[ -d "$sdk_dir" ]]; then
  if [[ -d "$sdk_dir/platforms/android-36" ]]; then
    printf 'ok      Android platform android-36 under %s\n' "$sdk_dir"
  else
    printf 'missing Android platform android-36 under Android SDK: %s\n' "$sdk_dir" >&2
    exit 1
  fi
  if [[ -d "$sdk_dir/build-tools/36.0.0" ]]; then
    printf 'ok      Android build-tools 36.0.0 under %s\n' "$sdk_dir"
  else
    printf 'missing Android build-tools 36.0.0 under Android SDK: %s\n' "$sdk_dir" >&2
    exit 1
  fi
  if find "$sdk_dir/cmdline-tools" -type f -name sdkmanager -print -quit >/dev/null 2>&1; then
    printf 'ok      sdkmanager under %s\n' "$sdk_dir"
  fi
  if find "$sdk_dir/platform-tools" -type f -name adb -print -quit >/dev/null 2>&1; then
    printf 'ok      adb under %s\n' "$sdk_dir"
  fi
  if find "$sdk_dir/build-tools" -type f -name zipalign -print -quit >/dev/null 2>&1; then
    printf 'ok      zipalign under %s\n' "$sdk_dir"
  else
    printf 'missing zipalign under Android SDK: %s\n' "$sdk_dir" >&2
    exit 1
  fi
fi

if [[ -d "$ndk_dir" ]]; then
  if find "$ndk_dir/toolchains/llvm/prebuilt" -type f -name llvm-readelf -print -quit >/dev/null 2>&1; then
    printf 'ok      llvm-readelf under %s\n' "$ndk_dir"
  elif find "$ndk_dir/toolchains/llvm/prebuilt" -type f -name llvm-readobj -print -quit >/dev/null 2>&1; then
    printf 'ok      llvm-readobj under %s\n' "$ndk_dir"
  else
    printf 'missing llvm-readelf/llvm-readobj under Android NDK: %s\n' "$ndk_dir" >&2
    exit 1
  fi
fi

cfst_log "Android doctor completed"
