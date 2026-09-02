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
  desktop/cfst-gui-windows-amd64.exe
  desktop/cfst-gui-linux-amd64.tar.gz
  desktop/cfst-gui-linux-arm64.tar.gz
  android/cfst-gui-android-arm64-v8a-release.apk
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
  node - "$release_dir" "$manifest" <<'NODE'
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");
const releaseDir = process.argv[2];
const manifestPath = process.argv[3];
const data = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
if (!data.version) throw new Error("manifest.version missing");
if (!data.docker_image) throw new Error("manifest.docker_image missing");
if (!Array.isArray(data.assets) || data.assets.length === 0) {
  throw new Error("manifest.assets missing");
}
for (const asset of data.assets) {
  if (!asset.name) throw new Error("manifest asset name missing");
  const expectedSuffix = `/releases/download/v${data.version}/${asset.name}`;
  if (!asset.download_url || !asset.download_url.endsWith(expectedSuffix)) {
    throw new Error(`manifest asset ${asset.name} must use versioned URL ${expectedSuffix}`);
  }
  if (!/^[a-f0-9]{64}$/.test(asset.sha256 || "")) {
    throw new Error(`manifest asset ${asset.name} has invalid sha256`);
  }
  if (asset.goos === "linux" && asset.install_mode !== "docker_compose") {
    throw new Error(`linux asset ${asset.name} must use install_mode=docker_compose`);
  }

  const localPath = ["desktop", "android", ""].map((dir) => path.join(releaseDir, dir, asset.name)).find(fs.existsSync);
  if (localPath) {
    const actual = crypto.createHash("sha256").update(fs.readFileSync(localPath)).digest("hex");
    if (actual !== asset.sha256) {
      throw new Error(`manifest sha256 mismatch for ${asset.name}: expected ${asset.sha256}, got ${actual}`);
    }
  }
}
console.log(`manifest version: ${data.version}`);
console.log(`docker image: ${data.docker_image}`);
NODE
fi

if ((missing > 0 && allow_missing == 0)); then
  printf '\nMissing %d expected artifact(s). Use --allow-missing for partial inspections.\n' "$missing" >&2
  exit 1
fi
