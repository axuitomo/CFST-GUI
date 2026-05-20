#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

if (($# > 0)); then
  cfst_log "Checking Android 16KB page alignment for supplied artifacts"
  bash "$ROOT_DIR/scripts/check-android-page-alignment.sh" "$@"
else
  cfst_log "Building Android debug artifacts and checking 16KB page alignment"
  bash "$ROOT_DIR/scripts/build-android-mobile.sh"
fi
