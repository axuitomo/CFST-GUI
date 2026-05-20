#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

usage() {
  cat <<'EOF'
usage: scripts/clean.sh [--apply] [--dry-run] [--deps] [--frontend-dist]

Removes ignored build outputs and caches while leaving tracked files intact.
Defaults to dry-run; pass --apply to actually delete files.

Options:
  --apply          Actually remove ignored generated files.
  --dry-run        Show what would be removed.
  --deps           Also remove frontend/node_modules.
  --frontend-dist  Remove untracked files under frontend/dist; tracked dist files are kept.
EOF
}

dry_run=1
include_deps=0
include_frontend_dist=0

while (($# > 0)); do
  case "$1" in
    --apply)
      dry_run=0
      ;;
    --dry-run)
      dry_run=1
      ;;
    --deps)
      include_deps=1
      ;;
    --frontend-dist)
      include_frontend_dist=1
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

ignored_targets=(
  build/release
  build/bin
  build/android
  build/cfst-webui-linux-amd64
  build/cfst-webui-linux-arm64
  frontend/.vite
  mobile/android/.gradle
  mobile/android/app/build
  mobile/android/app/libs
  mobile/android/build
  mobile/android/capacitor-cordova-android-plugins/build
)

if ((include_deps)); then
  ignored_targets+=(frontend/node_modules)
fi

clean_args=(-d -X)
if ((dry_run)); then
  clean_args=(-n "${clean_args[@]}")
else
  clean_args=(-f "${clean_args[@]}")
fi

cfst_log "Cleaning ignored generated files"
if ((dry_run)); then
  cfst_warn "dry-run only; rerun with --apply to remove files"
fi
git -C "$ROOT_DIR" clean "${clean_args[@]}" -- "${ignored_targets[@]}"

if ((include_frontend_dist)); then
  cfst_log "Cleaning untracked frontend/dist files"
  dist_args=(-d)
  if ((dry_run)); then
    dist_args=(-n "${dist_args[@]}")
  else
    dist_args=(-f "${dist_args[@]}")
  fi
  git -C "$ROOT_DIR" clean "${dist_args[@]}" -- frontend/dist
fi

cfst_log "Clean completed"
