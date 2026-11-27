#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

force=0

usage() {
  cat <<'EOF'
usage: scripts/dev/hooks-install.sh [--force]

Activates the versioned repository hooks in .githooks/ via core.hooksPath.

Options:
  --force  Overwrite an existing non-.githooks core.hooksPath and remove legacy .git/hooks/{pre-commit,pre-push}.
EOF
}

while (($# > 0)); do
  case "$1" in
    --force)
      force=1
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

current_hooks_path="$(git -C "$ROOT_DIR" config --local --get core.hooksPath || true)"
if [[ -n "$current_hooks_path" && "$current_hooks_path" != ".githooks" && "$force" != "1" ]]; then
  printf 'core.hooksPath is already set to %s (use --force to overwrite)\n' "$current_hooks_path" >&2
  exit 1
fi

git -C "$ROOT_DIR" config --local core.hooksPath .githooks
if [[ -f "$ROOT_DIR/.githooks/pre-commit" ]]; then
  chmod +x "$ROOT_DIR/.githooks/pre-commit"
fi

git_dir="$(git -C "$ROOT_DIR" rev-parse --git-dir)"
case "$git_dir" in
  /*|[A-Za-z]:*) hooks_dir="$git_dir/hooks" ;;
  *) hooks_dir="$ROOT_DIR/$git_dir/hooks" ;;
esac

if ((force)); then
  for legacy in pre-commit pre-push; do
    if [[ -e "$hooks_dir/$legacy" ]]; then
      rm -f "$hooks_dir/$legacy"
      printf 'removed legacy %s\n' "$hooks_dir/$legacy"
    fi
  done
fi

cfst_log "Git hooks activated (core.hooksPath=.githooks)"
