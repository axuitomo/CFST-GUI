#!/usr/bin/env bash
set -euo pipefail

# Frontend platform-convergence boundary.
#
# Platform runtime references (window.wails / wailsjs / @capacitor / Capacitor)
# are allowed only under frontend/src/lib/, per docs/dev/architecture-constraints.md.
# Invoked by .githooks/pre-commit and scripts/checks/check.sh.

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cd "$ROOT_DIR"

pattern="window\.wails|window\[['\"]wails['\"]\]|wailsjs/|@capacitor/|Capacitor"

violations="$(grep -rEn "$pattern" frontend/src \
  --include='*.ts' --include='*.vue' --include='*.js' 2>/dev/null \
  | grep -v '^frontend/src/lib/' || true)"

if [ -n "$violations" ]; then
  printf 'Frontend boundary check failed: platform runtime references (window.wails / wailsjs / @capacitor / Capacitor) are only allowed under frontend/src/lib/.\n' >&2
  printf '%s\n' "$violations" >&2
  printf 'See docs/dev/architecture-constraints.md.\n' >&2
  exit 1
fi

printf 'Frontend boundary check passed.\n'
