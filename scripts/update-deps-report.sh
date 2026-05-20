#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

output="$ROOT_DIR/build/reports/dependency-updates.md"
stdout=0

usage() {
  cat <<'EOF'
usage: scripts/update-deps-report.sh [--output <path>] [--stdout]

Generates a dependency update report without changing dependency versions.
EOF
}

while (($# > 0)); do
  case "$1" in
    --output)
      output="${2:-}"
      shift
      ;;
    --stdout)
      stdout=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

{
  printf '# Dependency Update Report\n\n'
  printf -- '- Generated at: `%s`\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf -- '- Go version: `%s`\n' "$(go version)"
  printf -- '- Node version: `%s`\n' "$(node --version 2>/dev/null || printf 'missing')"
  printf -- '- npm version: `%s`\n\n' "$(npm --version 2>/dev/null || printf 'missing')"

  printf '## Go Modules\n\n'
  printf '```text\n'
  (cd "$ROOT_DIR" && go list -m -u all)
  printf '```\n\n'

  printf '## npm Packages\n\n'
  printf '```text\n'
  (cd "$FRONTEND_DIR" && npm outdated || true)
  printf '```\n'
} >"$tmp"

if ((stdout)); then
  cat "$tmp"
else
  mkdir -p "$(dirname "$output")"
  cp "$tmp" "$output"
  printf 'Wrote %s\n' "$output"
fi
