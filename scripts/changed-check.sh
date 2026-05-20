#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

base=""
run_build=0

usage() {
  cat <<'EOF'
usage: scripts/changed-check.sh [--base <ref>] [--build]

Runs checks only for areas touched by the current change set.

Options:
  --base <ref>  Compare against a specific base ref.
  --build       Run frontend production build when frontend files changed.
EOF
}

while (($# > 0)); do
  case "$1" in
    --base)
      base="${2:-}"
      shift
      ;;
    --build)
      run_build=1
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

if [[ -z "$base" ]]; then
  if [[ -n "${GITHUB_BASE_REF:-}" ]] && git -C "$ROOT_DIR" rev-parse --verify "origin/$GITHUB_BASE_REF" >/dev/null 2>&1; then
    base="origin/$GITHUB_BASE_REF"
  elif git -C "$ROOT_DIR" rev-parse --verify origin/main >/dev/null 2>&1; then
    base="origin/main"
  elif git -C "$ROOT_DIR" rev-parse --verify origin/master >/dev/null 2>&1; then
    base="origin/master"
  else
    base="HEAD"
  fi
fi

cfst_log "Collecting changed files against $base"
if [[ "$base" == "HEAD" ]]; then
  mapfile -t changed_files < <(
    {
      git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR HEAD
      git -C "$ROOT_DIR" ls-files --others --exclude-standard
    } | sort -u
  )
else
  mapfile -t changed_files < <(
    {
      git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR "$base"...HEAD
      git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR HEAD
      git -C "$ROOT_DIR" ls-files --others --exclude-standard
    } | sort -u
  )
fi

if ((${#changed_files[@]} == 0)); then
  cfst_log "No changed files detected"
  exit 0
fi

printf '%s\n' "${changed_files[@]}"

has_go=0
has_frontend=0
has_scripts=0
has_docs=0

for file in "${changed_files[@]}"; do
  case "$file" in
    *.go|go.mod|go.sum)
      has_go=1
      ;;
    frontend/*)
      has_frontend=1
      ;;
    scripts/*.sh|scripts/lib/*.sh)
      has_scripts=1
      ;;
    README.md|docs/*.md|docs/**/*.md)
      has_docs=1
      ;;
  esac
done

if ((has_scripts)); then
  cfst_log "Checking changed shell scripts"
  bash -n "$ROOT_DIR"/scripts/*.sh "$ROOT_DIR"/scripts/lib/*.sh
  if command -v shellcheck >/dev/null 2>&1; then
    shellcheck "$ROOT_DIR"/scripts/*.sh "$ROOT_DIR"/scripts/lib/*.sh
  else
    cfst_warn "shellcheck not found; skipping shell lint"
  fi
fi

if ((has_go)); then
  cfst_log "Running Go checks for changed Go area"
  mapfile -t go_packages < <(cfst_go_packages)
  (cd "$ROOT_DIR" && go test "${go_packages[@]}")
fi

if ((has_frontend)); then
  cfst_prepare_frontend
  cfst_log "Running frontend lint and typecheck"
  (cd "$FRONTEND_DIR" && npm run lint && npm run typecheck)
  if ((run_build)); then
    (cd "$FRONTEND_DIR" && npm run build)
  fi
fi

if ((has_docs)); then
  bash "$ROOT_DIR/scripts/docs-check.sh"
fi

bash "$ROOT_DIR/scripts/format-check.sh"

cfst_log "Changed-file checks completed"
