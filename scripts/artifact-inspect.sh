#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

release_dir="$ROOT_DIR/build/release"
allow_missing=0

usage() {
  cat <<'EOF'
usage: scripts/artifact-inspect.sh [--dir <release-dir>] [--allow-missing]

Checks release artifact presence, size, and sha256 hashes.
EOF
}

while (($# > 0)); do
  case "$1" in
    --dir)
      release_dir="${2:-}"
      shift
      ;;
    --allow-missing)
      allow_missing=1
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

expected=(
  desktop/cfst-gui-windows-amd64.msix
  desktop/cfst-gui-linux-amd64.tar.gz
  desktop/cfst-gui-linux-arm64.tar.gz
  desktop/cfst-gui-darwin-amd64.app.zip
  desktop/cfst-gui-darwin-arm64.app.zip
  android/cfst-gui-android-arm64-v8a-release.apk
  android/cfst-gui-android-armeabi-v7a-release.apk
  android/cfst-gui-android-release.apk
  cfst-gui-update-manifest.json
)

missing=0

cfst_log "Inspecting release artifacts in $release_dir"
printf '%-52s %12s %s\n' "artifact" "size" "sha256"

for rel in "${expected[@]}"; do
  path="$release_dir/$rel"
  if [[ ! -f "$path" ]]; then
    printf '%-52s %12s %s\n' "$rel" "missing" "-"
    missing=$((missing + 1))
    continue
  fi
  printf '%-52s %12s %s\n' "$rel" "$(cfst_human_size "$path")" "$(cfst_sha256 "$path")"
done

manifest="$release_dir/cfst-gui-update-manifest.json"
if [[ -f "$manifest" ]]; then
  cfst_log "Validating update manifest JSON"
  # shellcheck disable=SC2016
  node -e 'const fs=require("fs"); const p=process.argv[1]; const data=JSON.parse(fs.readFileSync(p,"utf8")); if(!data.version) throw new Error("manifest.version missing"); if(!data.assets) throw new Error("manifest.assets missing"); console.log(`manifest version: ${data.version}`);' "$manifest"
fi

if ((missing > 0 && allow_missing == 0)); then
  printf '\nMissing %d expected artifact(s). Use --allow-missing for partial inspections.\n' "$missing" >&2
  exit 1
fi
