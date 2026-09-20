#!/usr/bin/env bash
set -euo pipefail

# Frontend platform-convergence boundary.
#
# 平台分流只允许出现在 frontend/src/lib/（桥接层），per
# docs/dev/architecture-constraints.md：
#
#   1) 平台运行时引用：window.wails / _wails（宿主运行时标记）/ wailsjs/ / @capacitor/ /
#      Capacitor
#   2) 平台通道引用：'/api/...' 字面量或模板、Wails 宿主地址（wails.localhost、
#      protocol === "wails:"）
#
# 视图/组件/组合式函数里的注释也不要写出这些名字（grep 不区分注释与代码）：提到宿主
# 运行时标记时改用 lib/wailsRuntime.ts 的函数名。
#
# 视图与组件不得自己判断通道：桌面端误走 WebUI 会打到只有 webui 构建才有的
# /api/command/{command}（表现为右下角 404 提示），WebUI 误走桌面 IPC 会打到不存在的
# /wails/runtime。分流统一走 lib/bridge.ts 的 resolveBridgeMode。
# Invoked by .githooks/pre-commit and scripts/checks/check.sh.

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cd "$ROOT_DIR"

runtime_pattern="window\.wails|window\[['\"]wails['\"]\]|_wails|wailsjs/|@capacitor/|Capacitor"
channel_pattern="['\"\`]/api/|wails\.localhost|protocol[[:space:]]*===[[:space:]]*['\"]wails:"

violations="$(
  {
    grep -rEn "$runtime_pattern" frontend/src --include='*.ts' --include='*.vue' --include='*.js' 2>/dev/null || true
    grep -rEn "$channel_pattern" frontend/src --include='*.ts' --include='*.vue' --include='*.js' 2>/dev/null || true
  } | grep -v '^frontend/src/lib/' | sort -u || true
)"

if [ -n "$violations" ]; then
  printf 'Frontend boundary check failed: platform switching must stay inside frontend/src/lib/.\n' >&2
  printf '  - runtime references: window.wails / _wails / wailsjs/ / @capacitor / Capacitor\n' >&2
  printf '  - channel references: /api/... literals or templates / wails.localhost / protocol === "wails:"\n' >&2
  printf '%s\n' "$violations" >&2
  printf 'Route channel decisions through lib/bridge.ts resolveBridgeMode. See docs/dev/architecture-constraints.md.\n' >&2
  exit 1
fi

printf 'Frontend boundary check passed.\n'
