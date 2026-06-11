#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_log "Verifying Go module checksums"
(cd "$ROOT_DIR" && go mod verify)

cfst_log "Listing available Go module updates"
(cd "$ROOT_DIR" && go list -m -u all)

cfst_prepare_frontend

cfst_log "Running pnpm audit"
(cd "$FRONTEND_DIR" && pnpm audit --audit-level="${CFST_PNPM_AUDIT_LEVEL:-moderate}")

cfst_log "Listing available pnpm package updates"
(cd "$FRONTEND_DIR" && pnpm outdated) || true

cfst_log "Dependency audit completed"
