#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
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
  if grep -Fq "$pattern" "$path"; then
    ok "$label"
  else
    fail "$label missing pattern: $pattern"
  fi
}

check_contains "$ROOT_DIR/scripts/build-release.sh" "VERSION=\"\${CFST_VERSION:-$version}\"" "build-release default version"
check_contains "$ROOT_DIR/.github/workflows/release.yml" "default: \"$version\"" "release workflow input default"
check_contains "$ROOT_DIR/.github/workflows/android-release-resubmit.yml" "default: \"$version\"" "Android resubmit workflow input default"
check_contains "$ROOT_DIR/.github/workflows/container.yml" "default: \"$version\"" "container workflow input default"
check_contains "$ANDROID_DIR/app/build.gradle" "? \"$version\"" "Android default versionName"
check_contains "$ROOT_DIR/internal/app/run.go" "var version = \"$version\"" "runtime default version"
check_contains "$ANDROID_DIR/build.gradle" "JavaVersion.VERSION_24" "Android Java 24 requirement"
check_contains "$ANDROID_DIR/build.gradle" "def androidJavaBytecodeVersion = JavaVersion.VERSION_24" "Android Java 24 bytecode target"
check_contains "$ANDROID_DIR/variables.gradle" "compileSdkVersion = 36" "Android compile SDK 36"
check_contains "$ANDROID_DIR/variables.gradle" "targetSdkVersion = 36" "Android target SDK 36"
check_contains "$ANDROID_DIR/variables.gradle" "cordovaAndroidVersion = '15.0.0'" "Cordova Android 15 baseline"
check_contains "$ROOT_DIR/frontend/package.json" "\"@capacitor/core\": \"^8.4.0\"" "Capacitor core 8.4.0"
check_contains "$ROOT_DIR/frontend/package.json" "\"@capacitor/android\": \"^8.4.0\"" "Capacitor Android 8.4.0"
check_contains "$ROOT_DIR/frontend/package.json" "\"@capacitor/cli\": \"^8.4.0\"" "Capacitor CLI 8.4.0"
check_contains "$ROOT_DIR/frontend/package-lock.json" "@capacitor/android/-/android-8.4.0.tgz" "Capacitor Android 8.4.0 lock entry"
check_contains "$ROOT_DIR/frontend/package-lock.json" "@capacitor/cli/-/cli-8.4.0.tgz" "Capacitor CLI 8.4.0 lock entry"
check_contains "$ROOT_DIR/frontend/package-lock.json" "@capacitor/core/-/core-8.4.0.tgz" "Capacitor core 8.4.0 lock entry"

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
for cmd in git go npm wails; do
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
