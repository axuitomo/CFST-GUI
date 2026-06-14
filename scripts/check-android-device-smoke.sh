#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

PACKAGE_NAME="io.github.axuitomo.cfstgui"
device_serial="${ANDROID_SERIAL:-}"

usage() {
  cat <<'EOF'
usage: scripts/check-android-device-smoke.sh [--device SERIAL] <apk>

Installs the APK on a connected device/emulator and checks Android runtime
package signals that cannot be proven from the built artifact alone.
EOF
}

apk_path=""
while (($# > 0)); do
  case "$1" in
    --device)
      if [[ $# -lt 2 ]]; then
        printf 'missing value for --device\n' >&2
        usage >&2
        exit 2
      fi
      device_serial="$2"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      printf 'unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if [[ -n "$apk_path" ]]; then
        printf 'unexpected extra argument: %s\n' "$1" >&2
        usage >&2
        exit 2
      fi
      apk_path="$1"
      ;;
  esac
  shift
done

if [[ -z "$apk_path" ]]; then
  usage >&2
  exit 2
fi
if [[ ! -f "$apk_path" ]]; then
  printf 'Android APK not found: %s\n' "$apk_path" >&2
  exit 1
fi

select_adb_binary() {
  if command -v adb >/dev/null 2>&1; then
    command -v adb
    return
  fi
  local sdk_dir="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$ROOT_DIR/.android-toolchain/android-sdk}}"
  local adb_path="$sdk_dir/platform-tools/adb"
  if [[ -x "$adb_path" ]]; then
    printf '%s\n' "$adb_path"
    return
  fi
  printf 'required command not found: adb\n' >&2
  exit 1
}

ADB_BIN="$(select_adb_binary)"

select_device_serial() {
  local selected="$1"
  if [[ -n "$selected" ]]; then
    printf '%s\n' "$selected"
    return
  fi

  local devices
  devices="$("$ADB_BIN" devices | awk 'NR > 1 && $2 == "device" {print $1}')"
  local count
  count="$(grep -c . <<<"$devices" || true)"
  if [[ "$count" == "0" ]]; then
    printf 'no connected Android device/emulator in adb "device" state\n' >&2
    exit 1
  fi
  if [[ "$count" != "1" ]]; then
    printf 'multiple Android devices detected; set ANDROID_SERIAL or pass --device\n' >&2
    printf '%s\n' "$devices" >&2
    exit 1
  fi
  printf '%s\n' "$devices"
}

device_serial="$(select_device_serial "$device_serial")"
adb_args=("$ADB_BIN" "-s" "$device_serial")

run_adb() {
  "${adb_args[@]}" "$@"
}

require_output_contains() {
  local output="$1"
  local pattern="$2"
  local label="$3"
  if grep -Fq "$pattern" <<<"$output"; then
    printf 'ok      %s\n' "$label"
    return
  fi
  printf 'Android device smoke check failed: %s missing pattern: %s\n' "$label" "$pattern" >&2
  exit 1
}

cfst_log "Installing Android APK on $device_serial"
run_adb install -r -d "$apk_path" >/dev/null
printf 'ok      APK installed: %s\n' "$apk_path"

cfst_log "Checking installed package manifest"
package_dump="$(run_adb shell dumpsys package "$PACKAGE_NAME" | tr -d '\r')"
require_output_contains "$package_dump" "$PACKAGE_NAME" "package registered"
require_output_contains "$package_dump" "targetSdk=37" "target SDK 37 on device"
require_output_contains "$package_dump" "android.permission.FOREGROUND_SERVICE" "foreground service permission on device"
require_output_contains "$package_dump" "android.permission.FOREGROUND_SERVICE_DATA_SYNC" "dataSync foreground service permission on device"
require_output_contains "$package_dump" "android.permission.POST_NOTIFICATIONS" "notification permission on device"
require_output_contains "$package_dump" "android.permission.REQUEST_INSTALL_PACKAGES" "APK install permission on device"
require_output_contains "$package_dump" "android.permission.WAKE_LOCK" "wake lock permission on device"
require_output_contains "$package_dump" "android.permission.RECEIVE_BOOT_COMPLETED" "WorkManager boot permission on device"
require_output_contains "$package_dump" "MainActivity" "MainActivity on device"
require_output_contains "$package_dump" "ProbeForegroundService" "CFST foreground service on device"
require_output_contains "$package_dump" "AndroidKeepAliveForegroundService" "CFST keep-alive foreground service on device"
require_output_contains "$package_dump" "UpdatePackageCleanupReceiver" "update cleanup receiver on device"
require_output_contains "$package_dump" "$PACKAGE_NAME.fileprovider" "FileProvider authority on device"
require_output_contains "$package_dump" "androidx.work.impl.background.systemjob.SystemJobService" "WorkManager JobService on device"
require_output_contains "$package_dump" "androidx.work.impl.foreground.SystemForegroundService" "WorkManager foreground service on device"
require_output_contains "$package_dump" "androidx.work.impl.background.systemalarm.RescheduleReceiver" "WorkManager reschedule receiver on device"

sdk_version="$(run_adb shell getprop ro.build.version.sdk | tr -d '\r')"
if [[ "$sdk_version" =~ ^[0-9]+$ && "$sdk_version" -ge 33 ]]; then
  cfst_log "Checking Android 13+ notification permission grant path"
  if run_adb shell pm grant "$PACKAGE_NAME" android.permission.POST_NOTIFICATIONS >/dev/null 2>&1; then
    package_dump="$(run_adb shell dumpsys package "$PACKAGE_NAME" | tr -d '\r')"
    require_output_contains "$package_dump" "android.permission.POST_NOTIFICATIONS: granted=true" "notification permission grant on device"
  else
    cfst_warn "adb could not grant POST_NOTIFICATIONS; verify the permission prompt manually on this device"
  fi
fi

cfst_log "Launching Android app"
run_adb shell am force-stop "$PACKAGE_NAME" >/dev/null
run_adb shell monkey -p "$PACKAGE_NAME" -c android.intent.category.LAUNCHER 1 >/dev/null
sleep 3
app_pid="$(run_adb shell pidof "$PACKAGE_NAME" 2>/dev/null | tr -d '\r' || true)"
if [[ -z "$app_pid" ]]; then
  printf 'Android device smoke check failed: app process did not stay alive after launcher start\n' >&2
  exit 1
fi
printf 'ok      app launched with pid %s\n' "$app_pid"

cfst_log "Android device smoke check completed"
