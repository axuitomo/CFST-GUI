#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORTABLE_ROOT="${CFST_GUI_PORTABLE_ROOT:-$SCRIPT_DIR/portable}"
mkdir -p "$PORTABLE_ROOT"

export CFST_GUI_PORTABLE_ROOT="$PORTABLE_ROOT"
export CFST_WEBUI_ADDR="${CFST_WEBUI_ADDR:-127.0.0.1:34115}"

exec "$SCRIPT_DIR/cfst-webui"
