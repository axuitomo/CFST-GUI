#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

# frontend/dist 的构建产物按 .gitignore 约定不入库（只保留 .gitkeep 占位），因此这里不再
# 对 dist 做 git 漂移检测——真正的「旧前端」防线是：每次都重新生成 bindings、重新构建前端，
# 并断言构建产物确实产出且可被 //go:embed。只有静态的 frontend_assets.go（embed 指令）需要
# 保证不会在重新生成过程中被意外改动。
tracked_paths=(frontend_assets.go)

snapshot_tracked_state() {
  {
    git -C "$ROOT_DIR" status --porcelain -- "${tracked_paths[@]}"
    git -C "$ROOT_DIR" diff --binary -- "${tracked_paths[@]}"
    git -C "$ROOT_DIR" diff --cached --binary -- "${tracked_paths[@]}"
  }
}

assert_frontend_build_output() {
  # Vite 构建后必须有入口与至少一个内容哈希的 JS chunk，否则说明前端没真正构建出来，
  # 后续 go build 只会嵌入占位 .gitkeep，运行时就是空白/旧界面。
  if [[ ! -f "$FRONTEND_DIR/dist/index.html" ]]; then
    printf 'frontend build produced no dist/index.html; frontend/dist must be built before embedding\n' >&2
    exit 1
  fi
  if ! compgen -G "$FRONTEND_DIR/dist/assets/*.js" >/dev/null; then
    printf 'frontend build produced no dist/assets/*.js chunk; check the Vite build output\n' >&2
    exit 1
  fi
}

before_snapshot="$(snapshot_tracked_state)"

cfst_log "Regenerating Wails frontend bridge"
(cd "$ROOT_DIR" && wails3 generate bindings)
cfst_require_wails_bindings

if [[ "${CFST_SKIP_FRONTEND_BUILD:-0}" != "1" ]]; then
  cfst_prepare_frontend
  cfst_log "Rebuilding embedded frontend assets"
  (cd "$FRONTEND_DIR" && pnpm run build)
  assert_frontend_build_output
fi

cfst_log "Checking tracked embed wiring for regeneration drift"
after_snapshot="$(snapshot_tracked_state)"

if [[ "$after_snapshot" != "$before_snapshot" ]]; then
  printf 'Tracked embed wiring changed during regeneration:\n%s\n\n' \
    "$(git -C "$ROOT_DIR" status --porcelain -- "${tracked_paths[@]}")" >&2
  git -C "$ROOT_DIR" diff --stat -- "${tracked_paths[@]}" >&2
  exit 1
fi

cfst_log "Generated artifacts are present and embed wiring is stable"
