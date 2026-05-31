#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

new_version=""
android_code=""
apply=0
create_notes=1

usage() {
  cat <<'EOF'
usage: scripts/version-bump.sh <version> [--android-code <code>] [--apply] [--no-release-notes]

Updates release defaults in scripts, workflows, Android Gradle, and docs.
Without --apply, prints the planned changes only.

Examples:
  scripts/version-bump.sh 1.7.4
  scripts/version-bump.sh 1.8.0 --android-code 10800 --apply
EOF
}

derive_android_code() {
  local version="$1"
  local major minor patch
  IFS=. read -r major minor patch <<<"$version"
  patch="${patch:-0}"
  printf '%d\n' "$((10#$major * 10000 + 10#$minor * 100 + 10#$patch))"
}

while (($# > 0)); do
  case "$1" in
    --android-code)
      android_code="${2:-}"
      shift
      ;;
    --apply)
      apply=1
      ;;
    --no-release-notes)
      create_notes=0
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
      new_version="${1#v}"
      ;;
  esac
  shift
done

if [[ -z "$new_version" ]]; then
  usage >&2
  exit 2
fi

if [[ ! "$new_version" =~ ^[0-9]+(\.[0-9]+){1,2}$ ]]; then
  printf 'invalid version: %s\n' "$new_version" >&2
  exit 2
fi

old_version="$(cfst_default_version)"
old_code="$(cfst_android_default_version_code)"
if [[ -z "$old_version" ]]; then
  printf 'failed to parse current default version from scripts/build-release.sh\n' >&2
  exit 1
fi
if [[ -z "$old_code" ]]; then
  printf 'failed to parse current Android versionCode from mobile/android/app/build.gradle\n' >&2
  exit 1
fi
android_code="${android_code:-$(derive_android_code "$new_version")}"
if [[ ! "$android_code" =~ ^[0-9]+$ ]]; then
  printf 'invalid Android versionCode: %s\n' "$android_code" >&2
  exit 2
fi

targets=(
  scripts/build-release.sh
  .github/workflows/release.yml
  .github/workflows/android-release-resubmit.yml
  .github/workflows/container.yml
  wails.json
  mobile/android/app/build.gradle
  docs/docker-env.md
  docs/deployment.md
)

cfst_log "Version bump plan"
printf 'old version: %s\nnew version: %s\nold Android code: %s\nnew Android code: %s\n' "$old_version" "$new_version" "$old_code" "$android_code"
printf 'target files:\n'
printf '  %s\n' "${targets[@]}"

if ((apply == 0)); then
  cfst_warn "dry-run only; rerun with --apply to modify files"
  exit 0
fi

for rel in "${targets[@]}"; do
  path="$ROOT_DIR/$rel"
  [[ -f "$path" ]] || continue
  perl -0pi -e "s/\\Q$old_version\\E/$new_version/g; s/\\Q$old_code\\E/$android_code/g" "$path"
done

notes="$ROOT_DIR/docs/release-notes/v$new_version.md"
if ((create_notes)) && [[ ! -f "$notes" ]]; then
  cat >"$notes" <<EOF
# CFST-GUI v$new_version

## 变更摘要

- 待补充。

## 验证

- 待补充。
EOF
  printf 'created %s\n' "$notes"
fi

cfst_log "Version defaults updated"
git -C "$ROOT_DIR" diff --stat -- "${targets[@]}" "docs/release-notes/v$new_version.md"
