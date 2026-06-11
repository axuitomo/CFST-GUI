#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

strict=0
android=0

usage() {
  cat <<'EOF'
usage: scripts/doctor.sh [--strict] [--android]

Checks the local development toolchain without modifying files.

Options:
  --strict   Treat optional toolchain gaps as failures.
  --android  Include Android-specific tools and paths.
EOF
}

while (($# > 0)); do
  case "$1" in
    --strict)
      strict=1
      ;;
    --android)
      android=1
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

required_missing=0
optional_missing=0

check_cmd() {
  local label="$1"
  local cmd="$2"
  local required="${3:-1}"
  local version_cmd="${4:-}"

  if command -v "$cmd" >/dev/null 2>&1; then
    if [[ -n "$version_cmd" ]]; then
      printf 'ok      %-18s %s\n' "$label" "$(eval "$version_cmd" 2>/dev/null | head -n 1)"
    else
      printf 'ok      %-18s %s\n' "$label" "$(command -v "$cmd")"
    fi
    return
  fi

  if [[ "$required" == "1" ]]; then
    printf 'missing %-18s required command: %s\n' "$label" "$cmd"
    required_missing=$((required_missing + 1))
  else
    printf 'missing %-18s optional command: %s\n' "$label" "$cmd"
    optional_missing=$((optional_missing + 1))
  fi
}

check_path() {
  local label="$1"
  local path="$2"
  local required="${3:-1}"

  if [[ -e "$path" ]]; then
    printf 'ok      %-18s %s\n' "$label" "$path"
    return
  fi

  if [[ "$required" == "1" ]]; then
    printf 'missing %-18s required path: %s\n' "$label" "$path"
    required_missing=$((required_missing + 1))
  else
    printf 'missing %-18s optional path: %s\n' "$label" "$path"
    optional_missing=$((optional_missing + 1))
  fi
}

cfst_log "Checking core toolchain"
check_cmd "git" git 1 "git --version"
check_cmd "go" go 1 "go version"
check_cmd "node" node 1 "node --version"
check_cmd "pnpm" pnpm 1 "pnpm --version"
check_cmd "wails" wails 1 "wails version"
check_cmd "shellcheck" shellcheck 0 "shellcheck --version"
check_path "go.mod" "$ROOT_DIR/go.mod" 1
check_path "pnpm lock" "$ROOT_DIR/pnpm-lock.yaml" 1

default_version="$(cfst_default_version)"
android_code="$(cfst_android_default_version_code)"
printf 'info    %-18s %s\n' "release version" "${default_version:-unknown}"
printf 'info    %-18s %s\n' "android code" "${android_code:-unknown}"

if ((android)); then
  cfst_log "Checking Android toolchain"
  check_cmd "java" java 1 "java -version 2>&1"
  check_cmd "gomobile" gomobile 1 "gomobile version"
  check_cmd "sdkmanager" sdkmanager 0 "sdkmanager --version"
  check_cmd "adb" adb 0 "adb version"
  check_path "gradlew" "$ANDROID_DIR/gradlew" 1

  sdk_dir="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$ROOT_DIR/.android-toolchain/android-sdk}}"
  ndk_dir="${ANDROID_NDK_HOME:-$sdk_dir/ndk/29.0.14206865}"
  if [[ ! -d "$ndk_dir" && -d "$ROOT_DIR/.android-toolchain/android-ndk-r26c" ]]; then
    ndk_dir="$ROOT_DIR/.android-toolchain/android-ndk-r26c"
  fi
  if [[ -n "$sdk_dir" ]]; then
    check_path "android sdk" "$sdk_dir" 1
  else
    printf 'missing %-18s ANDROID_HOME or ANDROID_SDK_ROOT is not set\n' "android sdk"
    required_missing=$((required_missing + 1))
  fi

  if [[ -n "$ndk_dir" ]]; then
    check_path "android ndk" "$ndk_dir" 1
  else
    printf 'missing %-18s ANDROID_NDK_HOME is not set\n' "android ndk"
    required_missing=$((required_missing + 1))
  fi
fi

if ((required_missing > 0)); then
  printf '\nRequired checks failed: %d\n' "$required_missing" >&2
  exit 1
fi

if ((strict && optional_missing > 0)); then
  printf '\nOptional checks failed under --strict: %d\n' "$optional_missing" >&2
  exit 1
fi

cfst_log "Doctor checks completed"
