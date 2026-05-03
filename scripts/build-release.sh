#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
RELEASE_DIR="$ROOT_DIR/build/release"
DESKTOP_DIR="$RELEASE_DIR/desktop"
ANDROID_RELEASE_DIR="$RELEASE_DIR/android"
VERSION="${CFST_VERSION:-1.1}"
GOMOBILE_BIN="${GOMOBILE_BIN:-$(go env GOPATH)/bin/gomobile}"
LD_FLAGS="-X main.version=$VERSION"
TARGET="${1:-all}"
CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
DEFAULT_ANDROID_SDK_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$CACHE_HOME/cfst-gui/android-toolchain/android-sdk}}"
DEFAULT_ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-$DEFAULT_ANDROID_SDK_HOME/ndk/26.3.11579264}"

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

build_linux() {
  cd "$ROOT_DIR"
  wails build -platform linux/amd64 -tags "tray webkit2_41" -ldflags "$LD_FLAGS"
  local binary="$ROOT_DIR/build/bin/cfst-gui"
  require_file "$binary" "Linux build output not found"
  tar -C "$(dirname "$binary")" -czf "$DESKTOP_DIR/cfst-gui-linux-amd64.tar.gz" "$(basename "$binary")"
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
    -o "$ANDROID_DIR/app/libs/mobileapi.aar" \
    github.com/XIU2/CloudflareSpeedTest/mobileapi
  cd "$ANDROID_DIR"
  bash ./gradlew assembleRelease
  local apk="$ANDROID_DIR/app/build/outputs/apk/release/app-universal-release.apk"
  require_file "$apk" "Android release APK not found"
  cp "$apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-release.apk"
}

write_manifest() {
  local windows="$DESKTOP_DIR/cfst-gui-windows-amd64.exe"
  local linux="$DESKTOP_DIR/cfst-gui-linux-amd64.tar.gz"
  local darwin_amd="$DESKTOP_DIR/cfst-gui-darwin-amd64.app.zip"
  local darwin_arm="$DESKTOP_DIR/cfst-gui-darwin-arm64.app.zip"
  local android="$ANDROID_RELEASE_DIR/cfst-gui-android-release.apk"
  require_file "$windows" "Windows asset missing"
  require_file "$linux" "Linux asset missing"
  require_file "$darwin_amd" "macOS amd64 asset missing"
  require_file "$darwin_arm" "macOS arm64 asset missing"
  require_file "$android" "Android asset missing"
  cat > "$RELEASE_DIR/cfst-gui-update-manifest.json" <<EOF
{
  "version": "$VERSION",
  "assets": [
    {"goos":"windows","goarch":"amd64","platform":"windows/amd64","name":"cfst-gui-windows-amd64.exe","download_url":"","sha256":"$(hash_file "$windows")","install_mode":"replace_exe"},
    {"goos":"linux","goarch":"amd64","platform":"linux/amd64","name":"cfst-gui-linux-amd64.tar.gz","download_url":"","sha256":"$(hash_file "$linux")","install_mode":"replace_binary"},
    {"goos":"darwin","goarch":"amd64","platform":"darwin/amd64","name":"cfst-gui-darwin-amd64.app.zip","download_url":"","sha256":"$(hash_file "$darwin_amd")","install_mode":"replace_app"},
    {"goos":"darwin","goarch":"arm64","platform":"darwin/arm64","name":"cfst-gui-darwin-arm64.app.zip","download_url":"","sha256":"$(hash_file "$darwin_arm")","install_mode":"replace_app"},
    {"goos":"android","goarch":"arm64","platform":"android","name":"cfst-gui-android-release.apk","download_url":"","sha256":"$(hash_file "$android")","install_mode":"android_apk"}
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
    echo "usage: $0 [all|windows|linux|darwin-amd64|darwin-arm64|android|manifest]" >&2
    exit 2
    ;;
esac

find "$RELEASE_DIR" -type f | sort
