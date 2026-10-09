#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cfst_log "Verifying Go module checksums"
(cd "$ROOT_DIR" && go mod verify)

if command -v govulncheck >/dev/null 2>&1; then
  cfst_log "Running govulncheck"
  mapfile -t go_packages < <(cfst_go_packages)
  (cd "$ROOT_DIR" && govulncheck "${go_packages[@]}")
else
  cfst_warn "govulncheck not found; skipping Go vulnerability scan"
fi

cfst_log "Listing available Go module updates"
(cd "$ROOT_DIR" && go list -m -u all)

cfst_prepare_frontend

cfst_log "Running pnpm audit"
# 显式指定审计端点：npmmirror 等镜像不实现 /-/npm/v1/security/advisories/bulk，
# 否则安全门禁会在镜像环境下直接报 ERR_PNPM_AUDIT_ENDPOINT_NOT_EXISTS 而不是真正执行审计。
# 需要换端点时设置 CFST_NPM_AUDIT_REGISTRY。
audit_registry="${CFST_NPM_AUDIT_REGISTRY:-https://registry.npmjs.org}"
(cd "$FRONTEND_DIR" && pnpm audit --audit-level="${CFST_PNPM_AUDIT_LEVEL:-moderate}" --registry="$audit_registry")

cfst_log "Listing available pnpm package updates"
(cd "$FRONTEND_DIR" && pnpm outdated) || true

cfst_log "Dependency audit completed"
