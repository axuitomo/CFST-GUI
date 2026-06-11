#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_log "Checking Go formatting"
mapfile -t go_files < <(
  find "$ROOT_DIR" -type f -name '*.go' \
    -not -path "$ROOT_DIR/build/*" \
    -not -path "$ROOT_DIR/frontend/node_modules/*" \
    -not -path "$ROOT_DIR/mobile/android/.gradle/*" \
    -not -path "$ROOT_DIR/mobile/android/app/build/*" \
    -not -path "$ROOT_DIR/mobile/android/build/*"
)

if ((${#go_files[@]} > 0)); then
  gofmt_output="$(gofmt -l "${go_files[@]}")"
  if [[ -n "$gofmt_output" ]]; then
    printf 'Go files require gofmt:\n%s\n' "$gofmt_output" >&2
    exit 1
  fi
fi

cfst_prepare_frontend

collect_frontend_format_files() {
  if [[ "${CFST_FORMAT_SCOPE:-changed}" == "all" ]]; then
    find "$FRONTEND_DIR" \
      \( -path "$FRONTEND_DIR/node_modules" -o -path "$FRONTEND_DIR/dist" -o -path "$FRONTEND_DIR/wailsjs" \) -prune \
      -o -type f \( -name '*.ts' -o -name '*.vue' -o -name '*.css' -o -name '*.json' \) -print |
      sed "s#^$ROOT_DIR/##"
    return
  fi

  if [[ -n "${GITHUB_BASE_REF:-}" ]] && git -C "$ROOT_DIR" rev-parse --verify "origin/$GITHUB_BASE_REF" >/dev/null 2>&1; then
    git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR "origin/$GITHUB_BASE_REF...HEAD" -- frontend
    return
  fi

  {
    git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR HEAD -- frontend
    git -C "$ROOT_DIR" ls-files --others --exclude-standard -- frontend
  } | sort -u
}

mapfile -t frontend_files < <(
  collect_frontend_format_files |
    grep -E '^frontend/.*\.(ts|vue|css|json)$' |
    grep -Ev '^frontend/(node_modules|dist|wailsjs)/' || true
)

if ((${#frontend_files[@]} == 0)); then
  cfst_log "No frontend files selected for Prettier check"
else
  cfst_log "Checking frontend formatting (${CFST_FORMAT_SCOPE:-changed} scope)"
  frontend_prettier_files=()
  for file in "${frontend_files[@]}"; do
    frontend_prettier_files+=("${file#frontend/}")
  done
  (cd "$FRONTEND_DIR" && pnpm exec prettier --check "${frontend_prettier_files[@]}")
fi

cfst_log "Formatting checks completed"
