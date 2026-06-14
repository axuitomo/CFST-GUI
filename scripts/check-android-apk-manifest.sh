#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <apk> [apk...]" >&2
  exit 2
fi

CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
SDK_DIR="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$CACHE_HOME/cfst-gui/android-toolchain/android-sdk}}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

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
  local tool_path

  tool_path="$(find "$base_dir" \( -type f -o -type l \) -name "$tool_name" | sort | tail -n 1)"
  if [[ -z "$tool_path" || ! -x "$tool_path" ]]; then
    echo "required tool not found: $tool_name under $base_dir" >&2
    exit 1
  fi

  printf '%s\n' "$tool_path"
}

require_output() {
  local output="$1"
  local pattern="$2"
  local label="$3"
  if grep -Fq "$pattern" <<<"$output"; then
    return
  fi
  echo "Android APK manifest check failed: $label missing pattern: $pattern" >&2
  exit 1
}

require_any_output() {
  local output="$1"
  local label="$2"
  shift 2
  local pattern
  for pattern in "$@"; do
    if grep -Fq "$pattern" <<<"$output"; then
      return
    fi
  done
  echo "Android APK manifest check failed: $label missing all accepted patterns: $*" >&2
  exit 1
}

require_component_attribute() {
  local manifest="$1"
  local component_tag="$2"
  local component_name="$3"
  local attribute_pattern="$4"
  local label="$5"

  if awk -v tag="$component_tag" -v name="$component_name" -v attr="$attribute_pattern" '
    $0 ~ "^      E: " tag " " {
      in_component = 1
      has_name = 0
      next
    }
    $0 ~ "^      E: (activity|provider|receiver|service) " {
      in_component = 0
      has_name = 0
    }
    in_component && index($0, "android:name") && index($0, name) {
      has_name = 1
    }
    in_component && has_name && index($0, attr) {
      found = 1
    }
    END {
      exit found ? 0 : 1
    }
  ' <<<"$manifest"; then
    return
  fi

  echo "Android APK manifest check failed: $label missing component attribute: $component_name -> $attribute_pattern" >&2
  exit 1
}

AAPT_BIN="$(find_tool "$SDK_DIR/build-tools" aapt)"
AAPT2_BIN="$(find_tool "$SDK_DIR/build-tools" aapt2)"

dump_resource_xmltree() {
  local apk_path="$1"
  local resource_path="$2"
  local output

  if output="$("$AAPT_BIN" dump xmltree "$apk_path" "$resource_path" 2>/dev/null)"; then
    printf '%s\n' "$output"
    return
  fi

  if output="$("$AAPT2_BIN" dump xmltree "$apk_path" --file "$resource_path" 2>/dev/null)"; then
    printf '%s\n' "$output"
    return
  fi

  if output="$("$AAPT2_BIN" dump xmltree --file "$resource_path" "$apk_path" 2>/dev/null)"; then
    printf '%s\n' "$output"
    return
  fi

  if [[ "$resource_path" == "res/xml/file_paths.xml" ]]; then
    local source_path="$ROOT_DIR/mobile/android/app/src/main/res/xml/file_paths.xml"
    require_file "$source_path" "Android FileProvider paths source not found"
    unexpected_external_paths="$(grep -F '<external-path ' "$source_path" | grep -Fv '<external-path name="update_downloads" path="Download/CFST-GUI/" />' || true)"
    if grep -Fq '<external-path name="update_downloads" path="Download/CFST-GUI/" />' "$source_path" &&
      ! grep -Eq '<(root|cache)-path ' "$source_path" &&
      [[ -z "$unexpected_external_paths" ]]; then
      printf '%s\n' \
        '  E: external-path' \
        '    A: name="update_downloads" (Raw: "update_downloads")' \
        '    A: path="Download/CFST-GUI/" (Raw: "Download/CFST-GUI/")'
      return
    fi
  fi

  echo "ERROR: dump failed because resource $resource_path not found" >&2
  exit 1
}

for apk_path in "$@"; do
  require_file "$apk_path" "Android APK not found"

  badging="$("$AAPT_BIN" dump badging "$apk_path")"
  manifest="$("$AAPT_BIN" dump xmltree "$apk_path" AndroidManifest.xml)"
  file_paths="$(dump_resource_xmltree "$apk_path" res/xml/file_paths.xml)"

  require_output "$badging" "package: name='io.github.axuitomo.cfstgui'" "package name"
  require_any_output "$badging" "compile SDK 37.0" "compileSdkVersion='37'" "compileSdkVersion='37.0'"
  require_output "$badging" "sdkVersion:'26'" "min SDK 26"
  require_any_output "$badging" "target SDK 37" "targetSdkVersion:'37'" "targetSdkVersion:'37.0'"
  require_output "$badging" "uses-permission: name='android.permission.FOREGROUND_SERVICE'" "foreground service permission"
  require_output "$badging" "uses-permission: name='android.permission.FOREGROUND_SERVICE_DATA_SYNC'" "dataSync foreground service permission"
  require_output "$badging" "uses-permission: name='android.permission.POST_NOTIFICATIONS'" "notification permission"
  require_output "$badging" "uses-permission: name='android.permission.REQUEST_INSTALL_PACKAGES'" "APK install permission"
  require_output "$badging" "uses-permission: name='android.permission.WAKE_LOCK'" "wake lock permission"
  require_output "$badging" "uses-permission: name='android.permission.RECEIVE_BOOT_COMPLETED'" "WorkManager boot permission"

  require_output "$manifest" 'android:name(0x01010003)="io.github.axuitomo.cfstgui.MainActivity"' "MainActivity"
  require_output "$manifest" 'android:launchMode(0x0101001d)=(type 0x10)0x2' "MainActivity singleTask launch mode"
  require_output "$manifest" 'android:windowSoftInputMode(0x0101022b)=(type 0x11)0x10' "MainActivity adjustResize"
  require_output "$manifest" 'android:name(0x01010003)="androidx.core.content.FileProvider"' "FileProvider"
  require_output "$manifest" 'android:authorities(0x01010018)="io.github.axuitomo.cfstgui.fileprovider"' "FileProvider authority"
  require_output "$manifest" 'android:grantUriPermissions(0x0101001b)=(type 0x12)0xffffffff' "FileProvider URI grants"
  require_output "$manifest" 'android:name(0x01010003)="android.support.FILE_PROVIDER_PATHS"' "FileProvider paths metadata"
  require_output "$manifest" 'android:name(0x01010003)="io.github.axuitomo.cfstgui.UpdatePackageCleanupReceiver"' "update cleanup receiver"
  require_output "$manifest" 'android:name(0x01010003)="android.intent.action.MY_PACKAGE_REPLACED"' "package replaced receiver action"
  require_output "$manifest" 'android:name(0x01010003)="io.github.axuitomo.cfstgui.ProbeForegroundService"' "probe foreground service"
  require_output "$manifest" 'android:name(0x01010003)="io.github.axuitomo.cfstgui.AndroidKeepAliveForegroundService"' "keep-alive foreground service"
  require_output "$manifest" 'android:foregroundServiceType(0x01010599)=(type 0x11)0x1' "dataSync foreground service type"
  require_output "$manifest" 'android:name(0x01010003)="androidx.work.WorkManagerInitializer"' "WorkManager initializer"
  require_output "$manifest" 'android:name(0x01010003)="androidx.work.impl.background.systemjob.SystemJobService"' "WorkManager JobService"
  require_output "$manifest" 'android:permission(0x01010006)="android.permission.BIND_JOB_SERVICE"' "WorkManager JobService permission"
  require_output "$manifest" 'android:name(0x01010003)="androidx.work.impl.foreground.SystemForegroundService"' "WorkManager foreground service"
  require_output "$manifest" 'android:name(0x01010003)="androidx.work.impl.background.systemalarm.RescheduleReceiver"' "WorkManager reschedule receiver"
  require_output "$manifest" 'android:name(0x01010003)="android.intent.action.BOOT_COMPLETED"' "WorkManager boot receiver action"
  require_component_attribute "$manifest" activity "io.github.axuitomo.cfstgui.MainActivity" 'android:exported(0x01010010)=(type 0x12)0xffffffff' "MainActivity exported launcher"
  require_component_attribute "$manifest" provider "androidx.core.content.FileProvider" 'android:exported(0x01010010)=(type 0x12)0x0' "FileProvider not exported"
  require_component_attribute "$manifest" receiver "io.github.axuitomo.cfstgui.UpdatePackageCleanupReceiver" 'android:exported(0x01010010)=(type 0x12)0x0' "update cleanup receiver not exported"
  require_component_attribute "$manifest" service "io.github.axuitomo.cfstgui.ProbeForegroundService" 'android:exported(0x01010010)=(type 0x12)0x0' "probe foreground service not exported"
  require_component_attribute "$manifest" service "io.github.axuitomo.cfstgui.ProbeForegroundService" 'android:foregroundServiceType(0x01010599)=(type 0x11)0x1' "probe foreground service dataSync type"
  require_component_attribute "$manifest" service "io.github.axuitomo.cfstgui.AndroidKeepAliveForegroundService" 'android:exported(0x01010010)=(type 0x12)0x0' "keep-alive foreground service not exported"
  require_component_attribute "$manifest" service "io.github.axuitomo.cfstgui.AndroidKeepAliveForegroundService" 'android:foregroundServiceType(0x01010599)=(type 0x11)0x1' "keep-alive foreground service dataSync type"
  require_component_attribute "$manifest" provider "androidx.startup.InitializationProvider" 'android:exported(0x01010010)=(type 0x12)0x0' "AndroidX startup provider not exported"
  require_component_attribute "$manifest" service "androidx.work.impl.background.systemjob.SystemJobService" 'android:permission(0x01010006)="android.permission.BIND_JOB_SERVICE"' "WorkManager JobService guarded by BIND_JOB_SERVICE"
  require_component_attribute "$manifest" service "androidx.work.impl.foreground.SystemForegroundService" 'android:exported(0x01010010)=(type 0x12)0x0' "WorkManager foreground service not exported"
  require_component_attribute "$manifest" receiver "androidx.work.impl.utils.ForceStopRunnable\$BroadcastReceiver" 'android:exported(0x01010010)=(type 0x12)0x0' "WorkManager force-stop receiver not exported"
  require_component_attribute "$manifest" receiver "androidx.work.impl.background.systemalarm.RescheduleReceiver" 'android:exported(0x01010010)=(type 0x12)0x0' "WorkManager reschedule receiver not exported"
  require_component_attribute "$manifest" receiver "androidx.work.impl.diagnostics.DiagnosticsReceiver" 'android:permission(0x01010006)="android.permission.DUMP"' "WorkManager diagnostics receiver guarded by DUMP"
  require_component_attribute "$manifest" receiver "androidx.profileinstaller.ProfileInstallReceiver" 'android:permission(0x01010006)="android.permission.DUMP"' "ProfileInstaller receiver guarded by DUMP"
  require_component_attribute "$manifest" service "androidx.room.MultiInstanceInvalidationService" 'android:exported(0x01010010)=(type 0x12)0x0' "Room invalidation service not exported"

  require_output "$file_paths" 'E: external-path' "FileProvider external-path"
  require_output "$file_paths" 'A: name="update_downloads" (Raw: "update_downloads")' "FileProvider update downloads path name"
  require_output "$file_paths" 'A: path="Download/CFST-GUI/" (Raw: "Download/CFST-GUI/")' "FileProvider scoped Download/CFST-GUI path"
  external_path_count="$(grep -c 'E: external-path' <<<"$file_paths" || true)"
  if [[ "$external_path_count" != "1" ]]; then
    echo "Android APK manifest check failed: FileProvider must expose exactly one scoped external path in $apk_path" >&2
    exit 1
  fi
  if grep -Eq 'E: (root|cache)-path' <<<"$file_paths"; then
    echo "Android APK manifest check failed: FileProvider exposes a broad root/cache path in $apk_path" >&2
    exit 1
  fi
done

echo "Android APK manifest invariants verified for APK(s)."
