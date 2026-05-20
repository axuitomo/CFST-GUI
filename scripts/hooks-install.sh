#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

force=0
with_pre_push=1

usage() {
  cat <<'EOF'
usage: scripts/hooks-install.sh [--force] [--no-pre-push]

Installs local Git hooks for this repository.

Options:
  --force        Overwrite existing hooks.
  --no-pre-push  Install only the pre-commit hook.
EOF
}

while (($# > 0)); do
  case "$1" in
    --force)
      force=1
      ;;
    --no-pre-push)
      with_pre_push=0
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

git_dir="$(git -C "$ROOT_DIR" rev-parse --git-dir)"
hooks_dir="$ROOT_DIR/$git_dir/hooks"
mkdir -p "$hooks_dir"

install_hook() {
  local name="$1"
  local path="$hooks_dir/$name"

  if [[ -e "$path" && "$force" != "1" ]]; then
    printf 'hook already exists: %s (use --force to overwrite)\n' "$path" >&2
    exit 1
  fi

  cat >"$path"
  chmod +x "$path"
  printf 'installed %s\n' "$path"
}

install_hook pre-commit <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
export CFST_SKIP_NPM_CI=1
bash "$ROOT_DIR/scripts/format-check.sh"
bash "$ROOT_DIR/scripts/lint.sh"
bash "$ROOT_DIR/scripts/secrets-scan.sh"
EOF

if ((with_pre_push)); then
  install_hook pre-push <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
export CFST_SKIP_NPM_CI=1
export CFST_SKIP_WAILS_GENERATE=1
bash "$ROOT_DIR/scripts/check.sh"
EOF
fi

cfst_log "Git hooks installed"
