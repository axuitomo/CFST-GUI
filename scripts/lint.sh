#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_log "Running go vet"
mapfile -t go_packages < <(cfst_go_packages)
(cd "$ROOT_DIR" && go vet "${go_packages[@]}")

if command -v golangci-lint >/dev/null 2>&1; then
  cfst_log "Running golangci-lint (errcheck/staticcheck/ineffassign/unused/revive/goimports)"
  (cd "$ROOT_DIR" && golangci-lint run)
else
  if [[ "${CFST_REQUIRE_GOLANGCI:-0}" == "1" ]]; then
    printf 'golangci-lint is required because CFST_REQUIRE_GOLANGCI=1\n' >&2
    exit 1
  fi
  cfst_warn "golangci-lint not found; skipping Go lint"
fi

if command -v shellcheck >/dev/null 2>&1; then
  cfst_log "Running shellcheck"
  mapfile -t shell_files < <(find "$ROOT_DIR/scripts" -type f -name '*.sh' | sort)
  if ((${#shell_files[@]} > 0)); then
    shellcheck "${shell_files[@]}"
  fi
else
  if [[ "${CFST_REQUIRE_SHELLCHECK:-0}" == "1" ]]; then
    printf 'shellcheck is required because CFST_REQUIRE_SHELLCHECK=1\n' >&2
    exit 1
  fi
  cfst_warn "shellcheck not found; skipping shell lint"
fi

if command -v actionlint >/dev/null 2>&1; then
  cfst_log "Running actionlint (GitHub Actions workflows)"
  (cd "$ROOT_DIR" && actionlint -color)
else
  if [[ "${CFST_REQUIRE_ACTIONLINT:-0}" == "1" ]]; then
    printf 'actionlint is required because CFST_REQUIRE_ACTIONLINT=1\n' >&2
    exit 1
  fi
  cfst_warn "actionlint not found; skipping workflow lint"
fi

cfst_prepare_frontend

cfst_log "Running frontend ESLint"
(cd "$FRONTEND_DIR" && pnpm run lint)

cfst_log "Running frontend stylelint"
(cd "$FRONTEND_DIR" && pnpm exec stylelint "src/**/*.css")

cfst_log "Running markdownlint"
(cd "$ROOT_DIR" && pnpm exec markdownlint-cli2)

cfst_log "Running root ESLint (Playwright config and E2E tests)"
(cd "$ROOT_DIR" && pnpm exec eslint playwright.config.ts "tests/**/*.ts")

cfst_log "Running Android ktlint (main source set) and detekt"
android_sdk_home=""
for candidate in "${ANDROID_HOME:-}" "${ANDROID_SDK_ROOT:-}" "$HOME/Library/Android/sdk" "${LOCALAPPDATA:-}/Android/Sdk"; do
  if [[ -n "$candidate" && -d "$candidate" ]]; then
    android_sdk_home="$candidate"
    break
  fi
done
if [[ -x "$ROOT_DIR/mobile/android/gradlew" || -f "$ROOT_DIR/mobile/android/gradlew.bat" ]] && [[ -n "$android_sdk_home" ]]; then
  if [[ ! -d "$ROOT_DIR/mobile/android/capacitor-cordova-android-plugins" ]]; then
    cfst_log "Generating Capacitor Android plugins"
    (cd "$FRONTEND_DIR" && pnpm run build && pnpm exec cap sync android)
  fi
  (
    cd "$ROOT_DIR/mobile/android"
    bash ./gradlew ktlintMainSourceSetCheck detekt --console=plain
  )
elif [[ -x "$ROOT_DIR/mobile/android/gradlew" || -f "$ROOT_DIR/mobile/android/gradlew.bat" ]]; then
  if [[ "${CFST_REQUIRE_ANDROID_LINT:-0}" == "1" ]]; then
    printf 'Android lint is required because CFST_REQUIRE_ANDROID_LINT=1, but the Android SDK was not found\n' >&2
    exit 1
  fi
  cfst_warn "Android SDK not found; skipping Android lint"
else
  cfst_warn "Android Gradle wrapper not found; skipping Android lint"
fi

cfst_log "Lint checks completed"
