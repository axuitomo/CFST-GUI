#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cfst_log "Checking Go formatting"
mapfile -t go_files < <(
  find "$ROOT_DIR" -type f -name '*.go' \
    -not -path "$ROOT_DIR/build/*" \
    -not -path "$ROOT_DIR/node_modules/*" \
    -not -path "$ROOT_DIR/.tmp/*" \
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

# 全量扫描：检查范围随改动集变化，会让格式漂移只在某个工作流里才暴露——发布就因此红过一次
# （v2.1.4 之前的 DnsSelfTestPanel.vue）。前端全部文件 --check 只要几十毫秒，不再按改动集裁剪。
# 用 git 列文件而不是遍历目录，让 .gitignore（node_modules、dist、wailsjs、.vite 缓存）自动生效。
mapfile -t frontend_files < <(
  git -C "$ROOT_DIR" ls-files --cached --others --exclude-standard -- frontend |
    sed 's#^frontend/##' |
    grep -E '\.(ts|vue|css|json)$'
)

if ((${#frontend_files[@]} == 0)); then
  cfst_log "No frontend files selected for oxfmt check"
else
  cfst_log "Checking frontend formatting (whole tree)"
  (cd "$FRONTEND_DIR" && pnpm exec oxfmt --check "${frontend_files[@]}")
fi

cfst_log "Formatting checks completed"
