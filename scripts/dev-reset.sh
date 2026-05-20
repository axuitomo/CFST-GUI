#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

apply=0
include_frontend_dist=0
run_check=1

usage() {
  cat <<'EOF'
usage: scripts/dev-reset.sh [--apply] [--dry-run] [--frontend-dist] [--skip-check]

Safely rebuilds the local development environment. Without --apply, it only
prints the clean plan.

Options:
  --apply          Actually clean ignored outputs and reinstall dependencies.
  --dry-run        Print the clean plan. This is the default.
  --frontend-dist  Also remove untracked frontend/dist files.
  --skip-check     Skip scripts/check.sh after rebuilding the environment.
EOF
}

while (($# > 0)); do
  case "$1" in
    --apply)
      apply=1
      ;;
    --dry-run)
      apply=0
      ;;
    --frontend-dist)
      include_frontend_dist=1
      ;;
    --skip-check)
      run_check=0
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

clean_args=(--dry-run --deps)
if ((include_frontend_dist)); then
  clean_args+=(--frontend-dist)
fi

if ((apply == 0)); then
  cfst_warn "dry-run only; rerun with --apply to reset the dev environment"
  bash "$ROOT_DIR/scripts/clean.sh" "${clean_args[@]}"
  exit 0
fi

clean_args=(--apply --deps)
if ((include_frontend_dist)); then
  clean_args+=(--frontend-dist)
fi

bash "$ROOT_DIR/scripts/clean.sh" "${clean_args[@]}"
cfst_prepare_frontend
cfst_generate_wails_module_if_possible

if ((run_check)); then
  bash "$ROOT_DIR/scripts/check.sh"
fi

cfst_log "Development environment reset completed"
