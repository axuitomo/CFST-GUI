#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

run_device_smoke=0
device_smoke_apk="$ANDROID_DIR/app/build/outputs/apk/debug/app-universal-debug.apk"
doctor_args=()

while (($# > 0)); do
  case "$1" in
    --device-smoke)
      run_device_smoke=1
      ;;
    --device-smoke-apk)
      if [[ $# -lt 2 ]]; then
        printf 'missing value for --device-smoke-apk\n' >&2
        exit 2
      fi
      run_device_smoke=1
      device_smoke_apk="$2"
      shift
      ;;
    *)
      doctor_args+=("$1")
      ;;
  esac
  shift
done

cfst_log "Checking Android development environment"
bash "$ROOT_DIR/scripts/doctor.sh" --android "${doctor_args[@]}"

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

require_android_pattern() {
  local path="$1"
  local pattern="$2"
  local label="$3"
  if grep -Fq -- "$pattern" "$path"; then
    printf 'ok      %s\n' "$label"
  else
    printf 'missing %s in %s: %s\n' "$label" "$path" "$pattern" >&2
    exit 1
  fi
}

require_android_absent_pattern() {
  local path="$1"
  local pattern="$2"
  local label="$3"
  if grep -Fq -- "$pattern" "$path"; then
    printf 'unexpected %s in %s: %s\n' "$label" "$path" "$pattern" >&2
    exit 1
  fi
  printf 'ok      %s\n' "$label"
}

cfst_log "Checking Android toolchain baseline"
require_android_pattern "$ANDROID_DIR/build.gradle" 'com.android.tools.build:gradle:9.2.1' "Android Gradle plugin 9.2.1 baseline"
require_android_pattern "$ANDROID_DIR/build.gradle" 'org.jetbrains.kotlin:kotlin-gradle-plugin:2.4.0' "Kotlin Gradle plugin 2.4.0 baseline"
require_android_pattern "$ANDROID_DIR/build.gradle" 'JavaVersion.VERSION_24' "JDK 24 Gradle JVM baseline"
require_android_pattern "$ANDROID_DIR/build.gradle" 'ext.androidJavaBytecodeVersion = JavaVersion.VERSION_24' "Java 24 bytecode baseline"
require_android_absent_pattern "$ANDROID_DIR/app/build.gradle" "apply plugin: 'org.jetbrains.kotlin.android'" "AGP 9 built-in Kotlin without legacy module plugin"
require_android_absent_pattern "$ANDROID_DIR/app/build.gradle" 'JvmTarget.JVM_24' "AGP 9 built-in Kotlin uses Java compile target"
require_android_pattern "$ANDROID_DIR/gradle/wrapper/gradle-wrapper.properties" 'gradle-9.5.1-bin.zip' "Gradle wrapper 9.5.1 baseline"
require_android_absent_pattern "$ANDROID_DIR/gradle.properties" 'android.suppressUnsupportedCompileSdk' "compile SDK 37 no longer needs AGP warning suppression"

cfst_log "Checking Android Kotlin buildscript classpath"
kotlin_baseline_output="$(mktemp)"
trap 'rm -f "$kotlin_baseline_output"' EXIT
if (cd "$ANDROID_DIR" && ./gradlew buildEnvironment --quiet >"$kotlin_baseline_output"); then
  if grep -Fq 'org.jetbrains.kotlin:kotlin-gradle-plugin:2.4.0' "$kotlin_baseline_output"; then
    printf 'ok      AGP built-in Kotlin uses KGP 2.4.0 from buildscript classpath\n'
  else
    printf 'unexpected Kotlin buildscript classpath; expected kotlin-gradle-plugin:2.4.0\n' >&2
    cat "$kotlin_baseline_output" >&2
    exit 1
  fi
else
  cat "$kotlin_baseline_output" >&2
  exit 1
fi

main_package_dir="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui"
test_package_dir="$ANDROID_DIR/app/src/test/java/io/github/axuitomo/cfstgui"
cfst_log "Checking Android Kotlin migration invariants"
if [[ ! -d "$main_package_dir" ]]; then
  printf 'missing Android main package directory: %s\n' "$main_package_dir" >&2
  exit 1
fi
remaining_java_source="$(find "$main_package_dir" -type f -name '*.java' -print -quit)"
if [[ -n "$remaining_java_source" ]]; then
  printf 'unexpected Android Java main source after Kotlin migration: %s\n' "$remaining_java_source" >&2
  exit 1
fi
if [[ -f "$main_package_dir/CfstPlugin.kt" ]]; then
  printf 'ok      Android main sources migrated to Kotlin\n'
else
  printf 'missing migrated Capacitor plugin source: %s\n' "$main_package_dir/CfstPlugin.kt" >&2
  exit 1
fi
if [[ -d "$test_package_dir" ]]; then
  remaining_java_test_source="$(find "$test_package_dir" -type f -name '*.java' -print -quit)"
  if [[ -n "$remaining_java_test_source" ]]; then
    printf 'unexpected CFST Android Java test source after Kotlin migration: %s\n' "$remaining_java_test_source" >&2
    exit 1
  fi
  printf 'ok      CFST Android unit tests migrated to Kotlin\n'
fi

manifest_path="$ANDROID_DIR/app/src/main/AndroidManifest.xml"
file_paths_path="$ANDROID_DIR/app/src/main/res/xml/file_paths.xml"
probe_service_path="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/ProbeForegroundService.kt"
keep_alive_service_path="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/AndroidKeepAliveForegroundService.kt"
schedule_worker_path="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/SchedulerWorker.kt"
update_installer_path="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/AndroidUpdateInstaller.kt"
update_downloads_path="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/AndroidUpdateDownloads.kt"
main_activity_path="$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt"

cfst_log "Checking Android manifest invariants"
for path in "$manifest_path" "$file_paths_path" "$probe_service_path" "$keep_alive_service_path" "$schedule_worker_path" "$update_installer_path" "$update_downloads_path"; do
  if [[ -f "$path" ]]; then
    printf 'ok      %s\n' "$path"
  else
    printf 'missing %s\n' "$path" >&2
    exit 1
  fi
done

cfst_log "Checking Android window inset invariants"
require_android_pattern "$main_activity_path" 'WindowCompat.setDecorFitsSystemWindows(window, false)' "edge-to-edge WebView inset handling"
require_android_pattern "$main_activity_path" 'controller.show(WindowInsetsCompat.Type.systemBars())' "Android status and navigation bars remain visible"
require_android_pattern "$main_activity_path" 'LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES' "display cutout short-edge layout"
require_android_pattern "$main_activity_path" 'WebSettings.FORCE_DARK_OFF' "WebView force dark disabled"
require_android_pattern "$main_activity_path" 'isAlgorithmicDarkeningAllowed = false' "WebView algorithmic darkening disabled"
require_android_pattern "$ROOT_DIR/mobile/android/app/src/main/res/values-v29/styles.xml" 'android:forceDarkAllowed' "API 29+ theme force dark disabled"
require_android_absent_pattern "$main_activity_path" 'hide(WindowInsetsCompat.Type.statusBars())' "Android status bar is not hidden"
require_android_absent_pattern "$main_activity_path" 'hide(WindowInsetsCompat.Type.systemBars())' "Android system bars are not hidden"
require_android_absent_pattern "$ROOT_DIR/frontend/src/App.vue" 'scrollIntoView({ block: "center"' "Android input focus does not force centered scrolling"
require_android_absent_pattern "$ROOT_DIR/frontend/src/styles.css" '--cfst-visual-viewport-height' "Android app height does not follow visualViewport"

require_android_pattern "$manifest_path" 'android.permission.FOREGROUND_SERVICE"' "foreground service permission"
require_android_pattern "$manifest_path" 'android.permission.FOREGROUND_SERVICE_DATA_SYNC"' "data sync foreground service permission"
require_android_pattern "$manifest_path" 'android.permission.POST_NOTIFICATIONS"' "Android 13 notification permission"
require_android_pattern "$manifest_path" 'android.permission.REQUEST_INSTALL_PACKAGES"' "APK install permission"
require_android_pattern "$manifest_path" 'android.permission.WAKE_LOCK"' "WorkManager wake lock permission"
require_android_pattern "$manifest_path" 'android:name=".MainActivity"' "MainActivity declaration"
require_android_pattern "$manifest_path" 'android:launchMode="singleTask"' "MainActivity singleTask launch mode"
require_android_pattern "$manifest_path" 'android:windowSoftInputMode="adjustResize"' "MainActivity keyboard resize mode"
require_android_pattern "$manifest_path" 'android:name="androidx.core.content.FileProvider"' "FileProvider declaration"
# shellcheck disable=SC2016
require_android_pattern "$manifest_path" 'android:authorities="${applicationId}.fileprovider"' "FileProvider package authority"
require_android_pattern "$manifest_path" 'android:exported="false"' "non-exported Android component"
require_android_pattern "$manifest_path" 'android:grantUriPermissions="true"' "FileProvider URI grants"
require_android_pattern "$manifest_path" 'android:name="android.support.FILE_PROVIDER_PATHS"' "FileProvider paths metadata"
require_android_pattern "$manifest_path" 'android:resource="@xml/file_paths"' "FileProvider paths resource"
require_android_pattern "$manifest_path" 'android:name=".UpdatePackageCleanupReceiver"' "update cleanup receiver"
require_android_pattern "$manifest_path" 'android.intent.action.MY_PACKAGE_REPLACED' "package replacement cleanup action"
require_android_pattern "$manifest_path" 'android:name=".ProbeForegroundService"' "probe foreground service"
require_android_pattern "$manifest_path" 'android:name=".AndroidKeepAliveForegroundService"' "Android keep-alive foreground service"
require_android_pattern "$manifest_path" 'android:foregroundServiceType="dataSync"' "Android 14 dataSync foreground service type"
require_android_pattern "$file_paths_path" '<files-path name="update_downloads" path="update_downloads/" />' "FileProvider private update downloads path"
unexpected_files_paths="$(grep -F '<files-path ' "$file_paths_path" | grep -Fv '<files-path name="update_downloads" path="update_downloads/" />' || true)"
if grep -Eq '<(root|cache|external)-path ' "$file_paths_path"; then
  printf 'unexpected broad FileProvider path in %s\n' "$file_paths_path" >&2
  exit 1
elif [[ -n "$unexpected_files_paths" ]]; then
  printf 'unexpected FileProvider files-path outside update_downloads in %s\n' "$file_paths_path" >&2
  exit 1
else
  printf 'ok      FileProvider exposes only private update_downloads path\n'
fi
require_android_pattern "$probe_service_path" 'ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC' "runtime dataSync foreground service start"
require_android_pattern "$keep_alive_service_path" 'ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC' "keep-alive dataSync foreground service start"
require_android_pattern "$keep_alive_service_path" 'START_STICKY' "keep-alive foreground service is sticky"
require_android_pattern "$keep_alive_service_path" 'setOngoing(true)' "keep-alive notification is ongoing"
require_android_pattern "$schedule_worker_path" 'setForegroundAsync(createForegroundInfo())' "WorkManager foreground execution"
require_android_pattern "$schedule_worker_path" 'context.startForegroundService(serviceIntent)' "WorkManager starts foreground probe service"
require_android_pattern "$schedule_worker_path" 'enqueueUniqueWork(UNIQUE_WORK_NAME, ExistingWorkPolicy.REPLACE, request)' "WorkManager unique replace scheduling"
require_android_pattern "$schedule_worker_path" 'private const val UNIQUE_WORK_NAME = "cfst-android-scheduler"' "WorkManager unique scheduler name"
require_android_pattern "$update_installer_path" 'displayDownloadPath(fileName: String)' "APK update path is user-visible app-private path"
require_android_pattern "$update_installer_path" 'private const val UPDATE_DOWNLOAD_DIRECTORY_NAME = "update_downloads"' "APK updates scoped to app-private update_downloads"
require_android_pattern "$update_installer_path" 'context.packageName + ".fileprovider"' "APK install FileProvider authority"
require_android_pattern "$update_installer_path" 'FileProvider.getUriForFile' "APK install flow uses FileProvider private file URI"
require_android_absent_pattern "$update_downloads_path" 'DownloadManager' "APK update download does not depend on system DownloadManager"
require_android_absent_pattern "$update_downloads_path" 'setDestinationInExternalPublicDir' "APK update download does not write to public Downloads"
require_android_pattern "$update_downloads_path" 'Proxy.NO_PROXY' "APK update download bypasses environment proxies"
require_android_pattern "$update_downloads_path" 'ExecutorCompletionService' "APK update download races mirror candidates"

sdk_dir="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$ROOT_DIR/.android-toolchain/android-sdk}}"
ndk_dir="${ANDROID_NDK_HOME:-$sdk_dir/ndk/29.0.14206865}"
if [[ ! -d "$ndk_dir" && -d "$ROOT_DIR/.android-toolchain/android-ndk-r26c" ]]; then
  ndk_dir="$ROOT_DIR/.android-toolchain/android-ndk-r26c"
fi

local_properties_path="$ANDROID_DIR/local.properties"
if [[ -f "$local_properties_path" ]]; then
  gradle_sdk_dir="$(sed -n 's/^sdk\.dir=//p' "$local_properties_path" | head -n 1)"
  if [[ -n "$gradle_sdk_dir" && "$gradle_sdk_dir" != "$sdk_dir" ]]; then
    printf 'Android SDK mismatch: environment resolves to %s but Gradle local.properties uses %s\n' "$sdk_dir" "$gradle_sdk_dir" >&2
    printf 'Update ANDROID_HOME/ANDROID_SDK_ROOT or mobile/android/local.properties so gomobile, checks, and Gradle use the same SDK.\n' >&2
    exit 1
  fi
fi

if [[ -d "$sdk_dir" ]]; then
  if [[ -d "$sdk_dir/platforms/android-37.0" ]]; then
    printf 'ok      Android platform android-37.0 under %s\n' "$sdk_dir"
  else
    printf 'missing Android platform android-37.0 under Android SDK: %s\n' "$sdk_dir" >&2
    exit 1
  fi
  if [[ -d "$sdk_dir/build-tools/37.0.0" ]]; then
    printf 'ok      Android build-tools 37.0.0 under %s\n' "$sdk_dir"
  else
    printf 'missing Android build-tools 37.0.0 under Android SDK: %s\n' "$sdk_dir" >&2
    exit 1
  fi
  if find "$sdk_dir/cmdline-tools" -type f -name sdkmanager -print -quit >/dev/null 2>&1; then
    printf 'ok      sdkmanager under %s\n' "$sdk_dir"
  fi
  if command -v sdkmanager >/dev/null 2>&1; then
    sdkmanager_version_output="$(mktemp)"
    if sdkmanager --version >"$sdkmanager_version_output" 2>&1; then
      sdkmanager_version="$(grep -E '^[0-9]+([.][0-9]+)*$' "$sdkmanager_version_output" | tail -n 1 || true)"
    else
      sdkmanager_version=""
    fi
    rm -f "$sdkmanager_version_output"
    sdkmanager_major="${sdkmanager_version%%.*}"
    if [[ "$sdkmanager_major" =~ ^[0-9]+$ && "$sdkmanager_major" -lt 20 ]]; then
      cfst_warn "Android cmdline-tools latest is 20.0; current sdkmanager is $sdkmanager_version and may warn about SDK XML v4"
    fi
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

if ((run_device_smoke)); then
  cfst_log "Checking Android device smoke flow"
  bash "$ROOT_DIR/scripts/check-android-device-smoke.sh" "$device_smoke_apk"
else
  cfst_warn "Android device smoke flow skipped; run scripts/android-doctor.sh --device-smoke after connecting a device or AVD"
fi

cfst_log "Android doctor completed"
