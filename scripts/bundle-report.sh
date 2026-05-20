#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

run_build=0
limit_kb="${CFST_BUNDLE_WARN_KB:-0}"

usage() {
  cat <<'EOF'
usage: scripts/bundle-report.sh [--build] [--limit-kb <size>]

Reports frontend dist asset sizes and gzip sizes.

Options:
  --build           Run npm run build before reporting.
  --limit-kb <n>    Warn when a JS/CSS asset is larger than n KiB. Default: disabled.
EOF
}

while (($# > 0)); do
  case "$1" in
    --build)
      run_build=1
      ;;
    --limit-kb)
      limit_kb="${2:-}"
      shift
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

if ((run_build)); then
  cfst_prepare_frontend
  (cd "$FRONTEND_DIR" && npm run build)
fi

dist_dir="$FRONTEND_DIR/dist"
if [[ ! -d "$dist_dir" ]]; then
  printf 'frontend/dist does not exist; run scripts/bundle-report.sh --build first\n' >&2
  exit 1
fi

cfst_log "Frontend bundle report"
printf '%-70s %12s %12s\n' "asset" "size" "gzip"

warn=0
while IFS= read -r -d '' file; do
  rel="${file#$ROOT_DIR/}"
  size_bytes="$(stat -c '%s' "$file")"
  gzip_bytes="$(gzip -c "$file" | wc -c | awk '{print $1}')"
  printf '%-70s %12s %12s\n' "$rel" "$(cfst_human_size "$file")" "$(printf '%s' "$gzip_bytes" | numfmt --to=iec --suffix=B 2>/dev/null || printf '%s' "$gzip_bytes")"

  if ((limit_kb > 0)); then
    case "$file" in
      *.js|*.css)
        if ((size_bytes > limit_kb * 1024)); then
          warn=1
        fi
        ;;
    esac
  fi
done < <(find "$dist_dir" -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' \) -print0 | sort -z)

if ((limit_kb > 0 && warn)); then
  printf '\nwarning: one or more JS/CSS assets exceed %s KiB\n' "$limit_kb" >&2
fi
