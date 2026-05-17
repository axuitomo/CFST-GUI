#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
RELEASE_DIR="$ROOT_DIR/build/release"
DESKTOP_DIR="$RELEASE_DIR/desktop"
ANDROID_RELEASE_DIR="$RELEASE_DIR/android"
VERSION="${CFST_VERSION:-1.7}"
GOMOBILE_BIN="${GOMOBILE_BIN:-$(go env GOPATH)/bin/gomobile}"
LD_FLAGS="-X github.com/axuitomo/CFST-GUI/internal/app.version=$VERSION"
TARGET="${1:-all}"
CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
DEFAULT_ANDROID_SDK_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$CACHE_HOME/cfst-gui/android-toolchain/android-sdk}}"
DEFAULT_ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-$DEFAULT_ANDROID_SDK_HOME/ndk/26.3.11579264}"
ANDROID_16K_LDFLAGS='-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'

require_file() {
  local path="$1"
  local message="$2"
  if [[ ! -f "$path" ]]; then
    echo "$message: $path" >&2
    exit 1
  fi
}

require_android_signing() {
  local missing=0
  for name in CFST_ANDROID_KEYSTORE CFST_ANDROID_KEYSTORE_PASSWORD CFST_ANDROID_KEY_ALIAS CFST_ANDROID_KEY_PASSWORD; do
    if [[ -z "${!name:-}" ]]; then
      echo "missing required environment variable: $name" >&2
      missing=1
    fi
  done
  if [[ "$missing" -ne 0 ]]; then
    echo "Android Release signing requires CFST_ANDROID_KEYSTORE, CFST_ANDROID_KEYSTORE_PASSWORD, CFST_ANDROID_KEY_ALIAS, CFST_ANDROID_KEY_PASSWORD." >&2
    exit 1
  fi
  require_file "$CFST_ANDROID_KEYSTORE" "Android Release keystore not found"
}

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  shasum -a 256 "$1" | awk '{print $1}'
}

release_asset_download_url() {
  local asset_name="$1"
  local repository="${GITHUB_REPOSITORY:-axuitomo/CFST-GUI}"
  local release_tag="v${VERSION#v}"
  printf 'https://xget.xi-xu.me/gh/%s/releases/download/%s/%s' "$repository" "$release_tag" "$asset_name"
}

build_frontend() {
  cd "$ROOT_DIR"
  wails generate module
  cd "$FRONTEND_DIR"
  npm ci
  npm run build
}

build_windows() {
  cd "$ROOT_DIR"
  wails build -platform windows/amd64 -tags tray -ldflags "$LD_FLAGS"
  require_file "$ROOT_DIR/build/bin/cfst-gui.exe" "Windows build output not found"
  cp "$ROOT_DIR/build/bin/cfst-gui.exe" "$DESKTOP_DIR/cfst-gui-windows-amd64.exe"
}

linux_bundle_dir() {
  local arch="$1"
  printf '%s/build/cfst-webui-linux-%s' "$ROOT_DIR" "$arch"
}

linux_bundle_archive() {
  local arch="$1"
  printf '%s/cfst-gui-linux-%s.tar.gz' "$DESKTOP_DIR" "$arch"
}

write_linux_bundle_files() {
  local bundle_dir="$1"
  cat > "$bundle_dir/Dockerfile" <<'EOF'
FROM scratch

WORKDIR /app
COPY cfst-webui /app/cfst-webui
COPY ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 34115
ENTRYPOINT ["/app/cfst-webui"]
EOF
  cat > "$bundle_dir/docker-compose.yml" <<'EOF'
services:
  cfst-webui:
    build: .
    image: cfst-webui:${CFST_VERSION:-latest}
    container_name: cfst-webui
    restart: unless-stopped
    environment:
      CFST_WEBUI_ADDR: 0.0.0.0:34115
      CFST_WEBUI_TOKEN: ${CFST_WEBUI_TOKEN:-change-me}
      CFST_GUI_PORTABLE_ROOT: /data
      CFST_WEBUI_ALLOWED_ROOTS: /data
    ports:
      - "${CFST_WEBUI_PORT:-34115}:34115"
    volumes:
      - cfst-webui-data:/data

volumes:
  cfst-webui-data:
EOF
  cat > "$bundle_dir/.env.example" <<EOF
CFST_WEBUI_PORT=34115
CFST_WEBUI_TOKEN=change-me
CFST_VERSION=$VERSION
EOF
  cat > "$bundle_dir/run-local.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORTABLE_ROOT="${CFST_GUI_PORTABLE_ROOT:-$SCRIPT_DIR/portable}"
mkdir -p "$PORTABLE_ROOT"

export CFST_GUI_PORTABLE_ROOT="$PORTABLE_ROOT"
export CFST_WEBUI_ADDR="${CFST_WEBUI_ADDR:-127.0.0.1:34115}"

exec "$SCRIPT_DIR/cfst-webui"
EOF
  chmod +x "$bundle_dir/run-local.sh"
  cat > "$bundle_dir/README.md" <<'EOF'
# CFST WebUI Bundle

## Docker Compose

1. Copy `.env.example` to `.env`.
2. Change `CFST_WEBUI_TOKEN` before exposing the service.
3. Run `docker compose up -d --build`.
4. Open `http://localhost:34115` and enter the token.

Data is persisted in the `cfst-webui-data` Docker volume mounted at `/data`.

## Local Linux

1. Run `./run-local.sh`.
2. Open `http://127.0.0.1:34115`.

`run-local.sh` keeps portable data under `./portable/data` by default. Override `CFST_WEBUI_ADDR` or `CFST_GUI_PORTABLE_ROOT` if you need a different bind address or storage path.
EOF
}

build_linux_arch() {
  cd "$ROOT_DIR"
  local arch="$1"
  local bundle_dir
  local archive_path
  case "$arch" in
    amd64|arm64)
      ;;
    *)
      echo "unsupported Linux arch: $arch" >&2
      exit 2
      ;;
  esac
  bundle_dir="$(linux_bundle_dir "$arch")"
  archive_path="$(linux_bundle_archive "$arch")"
  rm -rf "$bundle_dir"
  mkdir -p "$bundle_dir"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -tags webui -ldflags "$LD_FLAGS" -o "$bundle_dir/cfst-webui" .
  require_file "$bundle_dir/cfst-webui" "Linux WebUI build output not found"
  if [[ -f /etc/ssl/certs/ca-certificates.crt ]]; then
    cp /etc/ssl/certs/ca-certificates.crt "$bundle_dir/ca-certificates.crt"
  fi
  require_file "$bundle_dir/ca-certificates.crt" "CA certificates bundle not found"
  write_linux_bundle_files "$bundle_dir"
  tar -C "$(dirname "$bundle_dir")" -czf "$archive_path" "$(basename "$bundle_dir")"
}

build_linux() {
  build_linux_arch amd64
  build_linux_arch arm64
}

build_macos() {
  cd "$ROOT_DIR"
  local arch="$1"
  wails build -platform "darwin/$arch" -tags tray -ldflags "$LD_FLAGS"
  local app="$ROOT_DIR/build/bin/CFST-GUI.app"
  require_file "$app/Contents/MacOS/cfst-gui" "macOS build output not found"
  (cd "$ROOT_DIR/build/bin" && zip -qry "$DESKTOP_DIR/cfst-gui-darwin-$arch.app.zip" "CFST-GUI.app")
  rm -rf "$app"
}

build_android() {
  require_android_signing
  export ANDROID_NDK_HOME="$DEFAULT_ANDROID_NDK_HOME"
  if [[ ! -x "$GOMOBILE_BIN" ]]; then
    echo "gomobile not found at $GOMOBILE_BIN; run: go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b" >&2
    exit 1
  fi
  cd "$FRONTEND_DIR"
  npx cap sync android
  mkdir -p "$ANDROID_DIR/app/libs"
  "$GOMOBILE_BIN" bind \
    -androidapi 21 \
    -target=android/arm64,android/arm \
    -ldflags "$ANDROID_16K_LDFLAGS" \
    -o "$ANDROID_DIR/app/libs/mobileapi.aar" \
    github.com/axuitomo/CFST-GUI/mobileapi
  cd "$ANDROID_DIR"
  bash ./gradlew assembleRelease
  local apk="$ANDROID_DIR/app/build/outputs/apk/release/app-universal-release.apk"
  require_file "$apk" "Android release APK not found"
  bash "$ROOT_DIR/scripts/check-android-page-alignment.sh" "$ANDROID_DIR/app/libs/mobileapi.aar" "$apk"
  cp "$apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-release.apk"
}

write_manifest() {
  local windows="$DESKTOP_DIR/cfst-gui-windows-amd64.exe"
  local linux_amd64="$DESKTOP_DIR/cfst-gui-linux-amd64.tar.gz"
  local linux_arm64="$DESKTOP_DIR/cfst-gui-linux-arm64.tar.gz"
  local darwin_amd="$DESKTOP_DIR/cfst-gui-darwin-amd64.app.zip"
  local darwin_arm="$DESKTOP_DIR/cfst-gui-darwin-arm64.app.zip"
  local android="$ANDROID_RELEASE_DIR/cfst-gui-android-release.apk"
  require_file "$windows" "Windows asset missing"
  require_file "$linux_amd64" "Linux amd64 asset missing"
  require_file "$linux_arm64" "Linux arm64 asset missing"
  require_file "$darwin_amd" "macOS amd64 asset missing"
  require_file "$darwin_arm" "macOS arm64 asset missing"
  require_file "$android" "Android asset missing"
  cat > "$RELEASE_DIR/cfst-gui-update-manifest.json" <<EOF
{
  "version": "$VERSION",
  "assets": [
    {"goos":"windows","goarch":"amd64","platform":"windows/amd64","name":"cfst-gui-windows-amd64.exe","download_url":"$(release_asset_download_url "cfst-gui-windows-amd64.exe")","sha256":"$(hash_file "$windows")","install_mode":"replace_exe"},
    {"goos":"linux","goarch":"amd64","platform":"linux/amd64","name":"cfst-gui-linux-amd64.tar.gz","download_url":"$(release_asset_download_url "cfst-gui-linux-amd64.tar.gz")","sha256":"$(hash_file "$linux_amd64")","install_mode":"docker_compose"},
    {"goos":"linux","goarch":"arm64","platform":"linux/arm64","name":"cfst-gui-linux-arm64.tar.gz","download_url":"$(release_asset_download_url "cfst-gui-linux-arm64.tar.gz")","sha256":"$(hash_file "$linux_arm64")","install_mode":"docker_compose"},
    {"goos":"darwin","goarch":"amd64","platform":"darwin/amd64","name":"cfst-gui-darwin-amd64.app.zip","download_url":"$(release_asset_download_url "cfst-gui-darwin-amd64.app.zip")","sha256":"$(hash_file "$darwin_amd")","install_mode":"replace_app"},
    {"goos":"darwin","goarch":"arm64","platform":"darwin/arm64","name":"cfst-gui-darwin-arm64.app.zip","download_url":"$(release_asset_download_url "cfst-gui-darwin-arm64.app.zip")","sha256":"$(hash_file "$darwin_arm")","install_mode":"replace_app"},
    {"goos":"android","goarch":"arm64","platform":"android","name":"cfst-gui-android-release.apk","download_url":"$(release_asset_download_url "cfst-gui-android-release.apk")","sha256":"$(hash_file "$android")","install_mode":"android_apk"}
  ]
}
EOF
}

mkdir -p "$DESKTOP_DIR" "$ANDROID_RELEASE_DIR"

case "$TARGET" in
  all)
    rm -rf "$RELEASE_DIR" "$ROOT_DIR/Releases"
    mkdir -p "$DESKTOP_DIR" "$ANDROID_RELEASE_DIR"
    build_frontend
    build_windows
    build_linux
    build_macos amd64
    build_macos arm64
    build_android
    write_manifest
    ;;
  windows)
    build_frontend
    build_windows
    ;;
  linux)
    build_frontend
    build_linux
    ;;
  linux-amd64)
    build_frontend
    build_linux_arch amd64
    ;;
  linux-arm64)
    build_frontend
    build_linux_arch arm64
    ;;
  darwin-amd64)
    build_frontend
    build_macos amd64
    ;;
  darwin-arm64)
    build_frontend
    build_macos arm64
    ;;
  android)
    build_frontend
    build_android
    ;;
  manifest)
    write_manifest
    ;;
  *)
    echo "usage: $0 [all|windows|linux|linux-amd64|linux-arm64|darwin-amd64|darwin-arm64|android|manifest]" >&2
    exit 2
    ;;
esac

find "$RELEASE_DIR" -type f | sort
