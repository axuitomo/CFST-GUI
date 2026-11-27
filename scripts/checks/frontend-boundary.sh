#!/usr/bin/env bash
set -euo pipefail

# Frontend platform-convergence boundary.
#
# 平台分流只允许出现在 frontend/src/lib/adapters/（三端适配器层），per
# docs/dev/architecture-constraints.md：
#
#   1) 平台运行时引用：@wailsio / window.wails / _wails / wailsjs/ / @capacitor/ / Capacitor
#   2) 平台通道引用：'/api/...' 字面量或模板、Wails 宿主地址（wails.localhost、protocol === "wails:"）
#
# 适配器层之外（含共享 bridge 核心、composables、components、views，测试文件除外）都不得
# 出现这些引用；分流与平台能力统一经 lib/bridge.ts 路由到 lib/adapters/。
# Invoked by .githooks/pre-commit and scripts/checks/check.sh.

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cd "$ROOT_DIR"

runtime_pattern="@wailsio|window\.wails|window\[['\"]wails['\"]\]|_wails|wailsjs/|@capacitor/|Capacitor"
channel_pattern="['\"\`]/api/|wails\.localhost|protocol[[:space:]]*===[[:space:]]*['\"]wails:"

violations="$(
  {
    grep -rEn "$runtime_pattern" frontend/src --include='*.ts' --include='*.vue' --include='*.js' 2>/dev/null || true
    grep -rEn "$channel_pattern" frontend/src --include='*.ts' --include='*.vue' --include='*.js' 2>/dev/null || true
  } | grep -vE '^frontend/src/lib/adapters/|^frontend/src/.*\.(test|browser\.test)\.ts:' | sort -u || true
)"

if [ -n "$violations" ]; then
  printf 'Frontend boundary check failed: platform switching must stay inside frontend/src/lib/adapters/.\n' >&2
  printf '  - runtime references: @wailsio / window.wails / _wails / wailsjs/ / @capacitor / Capacitor\n' >&2
  printf '  - channel references: /api/... literals or templates / wails.localhost / protocol === "wails:"\n' >&2
  printf '%s\n' "$violations" >&2
  printf 'Route platform capabilities through lib/bridge.ts and lib/adapters/. See docs/dev/architecture-constraints.md.\n' >&2
  exit 1
fi

printf 'Frontend boundary check passed.\n'
