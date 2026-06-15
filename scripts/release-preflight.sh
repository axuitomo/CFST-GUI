#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

version="${CFST_VERSION:-$(cfst_default_version)}"
allow_dirty=0
check_android_signing=0
run_checks=0

usage() {
  cat <<'EOF'
usage: scripts/release-preflight.sh [version] [--allow-dirty] [--android-signing] [--run-checks]

Validates release readiness without building release artifacts by default.

Options:
  --allow-dirty      Do not fail when the git worktree has changes.
  --android-signing  Require Android release signing environment variables.
  --run-checks       Run scripts/ci-local.sh as part of preflight.
EOF
}

while (($# > 0)); do
  case "$1" in
    --allow-dirty)
      allow_dirty=1
      ;;
    --android-signing)
      check_android_signing=1
      ;;
    --run-checks)
      run_checks=1
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
      version="$1"
      ;;
  esac
  shift
done

version="${version#v}"
errors=0

fail() {
  printf 'fail    %s\n' "$*" >&2
  errors=$((errors + 1))
}

ok() {
  printf 'ok      %s\n' "$*"
}

cfst_log "Checking release version $version"
if [[ "$version" =~ ^[0-9]+(\.[0-9]+){1,2}$ ]]; then
  ok "version format: $version"
else
  fail "version must look like 1.7 or 1.7.2"
fi

notes="$ROOT_DIR/docs/release-notes/v$version.md"
if [[ -f "$notes" ]]; then
  ok "release notes exist: docs/release-notes/v$version.md"
else
  fail "release notes missing: docs/release-notes/v$version.md"
fi

check_contains() {
  local path="$1"
  local pattern="$2"
  local label="$3"
  if grep -Fq -- "$pattern" "$path"; then
    ok "$label"
  else
    fail "$label missing pattern: $pattern"
  fi
}

check_contains "$ROOT_DIR/scripts/build-release.sh" "VERSION=\"\${CFST_VERSION:-$version}\"" "build-release default version"
check_contains "$ROOT_DIR/.github/workflows/release.yml" "default: \"$version\"" "release workflow input default"
check_contains "$ROOT_DIR/.github/workflows/android-release-resubmit.yml" "default: \"$version\"" "Android resubmit workflow input default"
check_contains "$ROOT_DIR/.github/workflows/container.yml" "default: \"$version\"" "container workflow input default"
check_contains "$ROOT_DIR/.github/workflows/container.yml" "workflow_call:" "container workflow reusable entrypoint"
check_contains "$ROOT_DIR/.github/workflows/container.yml" "RAW_VERSION: \${{ inputs.version || github.ref_name }}" "container workflow resolves called or tagged version"
check_contains "$ROOT_DIR/.github/workflows/release.yml" "uses: ./.github/workflows/container.yml" "release workflow publishes GHCR"
check_contains "$ROOT_DIR/.github/workflows/release.yml" "packages: write" "release workflow grants GHCR package write"
check_contains "$ROOT_DIR/scripts/build-release.sh" "sync_wails_product_version()" "Wails product version sync helper"
check_contains "$ROOT_DIR/scripts/build-release.sh" "trap restore_wails_config EXIT" "Wails config restore trap"
check_contains "$ROOT_DIR/scripts/build-release.sh" "wails build -platform windows/amd64 -nsis -s -webview2 error -tags tray" "Windows Wails build skips duplicate frontend and uses installer WebView2"
check_contains "$ROOT_DIR/scripts/build-release.sh" "CFST_WINDOWS_SIGNING_TIMESTAMP_URL" "Windows signing timestamp environment"
check_contains "$ROOT_DIR/build/windows/installer/sign-installer.cmd" '/tr "%TIMESTAMP_URL%" /td SHA256' "Windows installer signing timestamp"
if grep -Fq -- "sign_windows_installer \"\$WINDOWS_RELEASE_ASSET\"" "$ROOT_DIR/scripts/build-release.sh"; then
  fail "Windows installer must be signed by NSIS finalize hooks only"
else
  ok "Windows installer avoids post-NSIS duplicate signing"
fi
check_contains "$ROOT_DIR/.github/workflows/release.yml" "java-version: \"24\"" "release workflow Android JDK 24"
check_contains "$ROOT_DIR/.github/workflows/android-release-resubmit.yml" "java-version: \"24\"" "Android resubmit workflow JDK 24"
check_contains "$ROOT_DIR/.github/workflows/release.yml" "gradle/actions/setup-gradle@v4" "release workflow Gradle cache"
check_contains "$ROOT_DIR/.github/workflows/android-release-resubmit.yml" "gradle/actions/setup-gradle@v4" "Android resubmit workflow Gradle cache"
check_contains "$ANDROID_DIR/app/build.gradle" "? \"$version\"" "Android default versionName"
check_contains "$ROOT_DIR/internal/app/run.go" "var version = \"$version\"" "runtime default version"
check_contains "$ANDROID_DIR/build.gradle" "com.android.tools.build:gradle:9.2.1" "Android Gradle plugin 9.2.1"
check_contains "$ROOT_DIR/scripts/patch-android-gradle-warnings.sh" "CFST_ANDROID_GRADLE_PLUGIN_VERSION:-9.2.1" "Capacitor generated Gradle patch AGP 9.2.1"
check_contains "$ANDROID_DIR/build.gradle" "JavaVersion.VERSION_24" "Android JDK 24 requirement"
check_contains "$ANDROID_DIR/build.gradle" "ext.androidJavaBytecodeVersion = JavaVersion.VERSION_24" "Android Java 24 bytecode target"
check_contains "$ANDROID_DIR/build.gradle" "org.jetbrains.kotlin:kotlin-gradle-plugin:2.4.0" "Android Kotlin Gradle plugin 2.4.0"
if grep -Fq -- "apply plugin: 'org.jetbrains.kotlin.android'" "$ANDROID_DIR/app/build.gradle"; then
  fail "Android AGP 9 built-in Kotlin should not apply org.jetbrains.kotlin.android in app/build.gradle"
else
  ok "Android AGP 9 built-in Kotlin without legacy module plugin"
fi
if grep -Fq -- "JvmTarget.JVM_24" "$ANDROID_DIR/app/build.gradle"; then
  fail "Android AGP 9 built-in Kotlin should follow Java compile target instead of explicit KotlinCompile JvmTarget"
else
  ok "Android AGP 9 built-in Kotlin follows Java compile target"
fi
check_contains "$ANDROID_DIR/gradle/wrapper/gradle-wrapper.properties" "gradle-9.5.1-bin.zip" "Android Gradle wrapper 9.5.1"
check_contains "$ANDROID_DIR/gradle.properties" "org.gradle.parallel=true" "Android Gradle parallel builds"
check_contains "$ANDROID_DIR/gradle.properties" "org.gradle.caching=true" "Android Gradle build cache"
check_contains "$ANDROID_DIR/gradle.properties" "org.gradle.configuration-cache=true" "Android Gradle configuration cache"
check_contains "$ANDROID_DIR/gradle.properties" "org.gradle.configuration-cache.problems=warn" "Android Gradle configuration cache warning mode"
if grep -Fq -- "android.suppressUnsupportedCompileSdk" "$ANDROID_DIR/gradle.properties"; then
  fail "Android compile SDK 37 should not need AGP warning suppression under AGP 9.2.1"
else
  ok "Android compile SDK 37 without AGP warning suppression"
fi
check_contains "$ROOT_DIR/scripts/build-android-mobile.sh" "check-android-apk-manifest.sh" "Android debug APK manifest check"
check_contains "$ROOT_DIR/scripts/build-release.sh" "check-android-apk-manifest.sh" "Android release APK manifest check"
check_contains "$ROOT_DIR/scripts/check-android-apk-manifest.sh" "androidx.work.impl.background.systemjob.SystemJobService" "Android WorkManager APK manifest check"
check_contains "$ROOT_DIR/scripts/check-android-apk-manifest.sh" "require_component_attribute" "Android APK exported component manifest check"
check_contains "$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt" "controller.show(WindowInsetsCompat.Type.systemBars())" "Android status and navigation bars remain visible"
check_contains "$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt" "LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES" "Android display cutout short-edge layout"
check_contains "$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt" "WebSettings.FORCE_DARK_OFF" "Android WebView force dark disabled"
check_contains "$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt" "isAlgorithmicDarkeningAllowed = false" "Android WebView algorithmic darkening disabled"
check_contains "$ANDROID_DIR/app/src/main/res/values-v29/styles.xml" "android:forceDarkAllowed" "Android API 29+ theme force dark disabled"
if grep -Fq -- "hide(WindowInsetsCompat.Type.statusBars())" "$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt" ||
  grep -Fq -- "hide(WindowInsetsCompat.Type.systemBars())" "$ANDROID_DIR/app/src/main/java/io/github/axuitomo/cfstgui/MainActivity.kt"; then
  fail "Android release must not hide status/system bars"
else
  ok "Android release keeps status/system bars visible"
fi
if grep -Fq -- 'scrollIntoView({ block: "center"' "$ROOT_DIR/frontend/src/App.vue"; then
  fail "Android input focus must not force centered scrolling"
else
  ok "Android input focus avoids centered scroll jumps"
fi
if grep -Fq -- "--cfst-visual-viewport-height" "$ROOT_DIR/frontend/src/styles.css"; then
  fail "Android app height must not be driven by visualViewport"
else
  ok "Android app height stays stable during keyboard viewport changes"
fi
check_contains "$ROOT_DIR/scripts/android-doctor.sh" "check-android-device-smoke.sh" "Android device smoke check entrypoint"
check_contains "$ROOT_DIR/scripts/check-android-device-smoke.sh" "POST_NOTIFICATIONS" "Android 13 notification device smoke check"
check_contains "$ANDROID_DIR/variables.gradle" "compileSdkVersion = 37" "Android compile SDK 37"
check_contains "$ANDROID_DIR/variables.gradle" "targetSdkVersion = 37" "Android target SDK 37"
check_contains "$ANDROID_DIR/variables.gradle" "androidxActivityVersion = '1.13.0'" "AndroidX Activity 1.13.0"
check_contains "$ANDROID_DIR/variables.gradle" "androidxCoreVersion = '1.19.0'" "AndroidX Core 1.19.0"
check_contains "$ANDROID_DIR/variables.gradle" "androidxFragmentVersion = '1.8.9'" "AndroidX Fragment 1.8.9"
check_contains "$ANDROID_DIR/variables.gradle" "androidxWebkitVersion = '1.16.0'" "AndroidX WebKit 1.16.0"
check_contains "$ANDROID_DIR/variables.gradle" "cordovaAndroidVersion = '15.0.0'" "Cordova Android 15 baseline"
check_contains "$ROOT_DIR/frontend/package.json" "\"@capacitor/core\": \"^8.4.0\"" "Capacitor core 8.4.0"
check_contains "$ROOT_DIR/frontend/package.json" "\"@capacitor/android\": \"^8.4.0\"" "Capacitor Android 8.4.0"
check_contains "$ROOT_DIR/frontend/package.json" "\"@capacitor/cli\": \"^8.4.0\"" "Capacitor CLI 8.4.0"
check_contains "$ROOT_DIR/pnpm-lock.yaml" "@capacitor/android@8.4.0" "Capacitor Android 8.4.0 lock entry"
check_contains "$ROOT_DIR/pnpm-lock.yaml" "@capacitor/cli@8.4.0" "Capacitor CLI 8.4.0 lock entry"
check_contains "$ROOT_DIR/pnpm-lock.yaml" "@capacitor/core@8.4.0" "Capacitor core 8.4.0 lock entry"

if ((allow_dirty == 0)); then
  if [[ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]]; then
    fail "git worktree is dirty; use --allow-dirty for local preflight"
  else
    ok "git worktree clean"
  fi
else
  cfst_warn "git dirty check disabled by --allow-dirty"
fi

if ((check_android_signing)); then
  cfst_log "Checking Android signing environment"
  for name in CFST_ANDROID_KEYSTORE CFST_ANDROID_KEYSTORE_PASSWORD CFST_ANDROID_KEY_ALIAS CFST_ANDROID_KEY_PASSWORD; do
    if [[ -n "${!name:-}" ]]; then
      ok "$name is set"
    else
      fail "$name is not set"
    fi
  done
  if [[ -n "${CFST_ANDROID_KEYSTORE:-}" && -f "$CFST_ANDROID_KEYSTORE" ]]; then
    ok "Android keystore exists"
  elif [[ -n "${CFST_ANDROID_KEYSTORE:-}" ]]; then
    fail "Android keystore file not found: $CFST_ANDROID_KEYSTORE"
  fi
fi

cfst_log "Checking required release tools"
for cmd in git go pnpm wails; do
  if command -v "$cmd" >/dev/null 2>&1; then
    ok "$cmd available"
  else
    fail "$cmd missing"
  fi
done

if ((run_checks)); then
  bash "$ROOT_DIR/scripts/ci-local.sh"
fi

if ((errors > 0)); then
  printf '\nRelease preflight failed with %d issue(s).\n' "$errors" >&2
  exit 1
fi

cfst_log "Release preflight passed"
