#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

cfst_require_cmd wails

generated_paths=(frontend/dist frontend_assets.go)

snapshot_generated_state() {
  {
    git -C "$ROOT_DIR" status --porcelain -- "${generated_paths[@]}"
    git -C "$ROOT_DIR" diff --binary -- "${generated_paths[@]}"
    git -C "$ROOT_DIR" diff --cached --binary -- "${generated_paths[@]}"
    while IFS= read -r -d '' path; do
      printf 'UNTRACKED %s %s\n' "$(cfst_sha256 "$ROOT_DIR/$path")" "$path"
    done < <(git -C "$ROOT_DIR" ls-files --others --exclude-standard -z -- "${generated_paths[@]}")
  }
}

before_snapshot="$(snapshot_generated_state)"

cfst_log "Regenerating Wails frontend bridge"
(cd "$ROOT_DIR" && wails generate module)

if [[ "${CFST_SKIP_FRONTEND_BUILD:-0}" != "1" ]]; then
  cfst_prepare_frontend
  cfst_log "Rebuilding embedded frontend assets"
  (cd "$FRONTEND_DIR" && pnpm run build)
fi

cfst_log "Checking generated artifacts for regeneration drift"
after_snapshot="$(snapshot_generated_state)"

if [[ "$after_snapshot" != "$before_snapshot" ]]; then
  printf 'Generated artifacts changed during regeneration:\n%s\n\n' \
    "$(git -C "$ROOT_DIR" status --porcelain -- "${generated_paths[@]}")" >&2
  git -C "$ROOT_DIR" diff --stat -- "${generated_paths[@]}" >&2
  exit 1
fi

generated_status="$(git -C "$ROOT_DIR" status --porcelain -- "${generated_paths[@]}")"

if [[ -n "$generated_status" ]]; then
  if [[ "${CFST_VERIFY_GENERATED_STRICT:-0}" == "1" ]]; then
    printf 'Generated artifacts have uncommitted changes:\n%s\n\n' "$generated_status" >&2
    git -C "$ROOT_DIR" diff --stat -- "${generated_paths[@]}" >&2
    exit 1
  fi
  cfst_warn "generated artifacts differ from HEAD but are stable after regeneration"
fi

cfst_log "Generated artifacts are stable"
