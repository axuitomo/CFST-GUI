#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <mobileapi.aar> <apk> [apk...]" >&2
  exit 2
fi

AAR_PATH="$1"
shift

CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
SDK_DIR="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$CACHE_HOME/cfst-gui/android-toolchain/android-sdk}}"
NDK_DIR="${ANDROID_NDK_HOME:-$SDK_DIR/ndk/29.0.14206865}"

require_file() {
  local path="$1"
  local message="$2"
  if [[ ! -f "$path" ]]; then
    echo "$message: $path" >&2
    exit 1
  fi
}

find_tool() {
  local base_dir="$1"
  local tool_name="$2"
  local selection="$3"
  local tool_path

  case "$selection" in
    first)
      tool_path="$(find "$base_dir" \( -type f -o -type l \) -name "$tool_name" | sort | head -n 1)"
      ;;
    last)
      tool_path="$(find "$base_dir" \( -type f -o -type l \) -name "$tool_name" | sort | tail -n 1)"
      ;;
    *)
      echo "unsupported tool selection: $selection" >&2
      exit 1
      ;;
  esac

  if [[ -z "$tool_path" || ! -x "$tool_path" ]]; then
    echo "required tool not found: $tool_name under $base_dir" >&2
    exit 1
  fi

  printf '%s\n' "$tool_path"
}

require_file "$AAR_PATH" "Android AAR not found"
for apk_path in "$@"; do
  require_file "$apk_path" "Android APK not found"
done

READELF_BIN="$(find_tool "$NDK_DIR/toolchains/llvm/prebuilt" llvm-readelf first)"
ZIPALIGN_BIN="$(find_tool "$SDK_DIR/build-tools" zipalign last)"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

unzip -q "$AAR_PATH" "jni/arm64-v8a/libgojni.so" "jni/armeabi-v7a/libgojni.so" -d "$TMP_DIR"

for abi in arm64-v8a armeabi-v7a; do
  so_path="$TMP_DIR/jni/$abi/libgojni.so"
  require_file "$so_path" "Android JNI library missing from AAR"
  if ! "$READELF_BIN" -l "$so_path" | awk '/LOAD/ {print $NF}' | grep -qx '0x4000'; then
    echo "ELF LOAD segment is not 16KB aligned for $abi: $so_path" >&2
    "$READELF_BIN" -l "$so_path" | sed -n '1,40p' >&2
    exit 1
  fi
done

for apk_path in "$@"; do
  "$ZIPALIGN_BIN" -c -P 16 -v 4 "$apk_path" >/dev/null
done

echo "Android 16KB page alignment verified for AAR and APK(s)."
