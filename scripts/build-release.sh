#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
RELEASE_DIR="$ROOT_DIR/build/release"
DESKTOP_DIR="$RELEASE_DIR/desktop"
ANDROID_RELEASE_DIR="$RELEASE_DIR/android"
WINDOWS_RELEASE_ASSET="$DESKTOP_DIR/cfst-gui-windows-amd64.exe"
VERSION="${CFST_VERSION:-1.7.4}"
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

require_windows_signing() {
  if [[ -n "${CFST_WINDOWS_SIGNING_CERT:-}" ]]; then
    require_file "$CFST_WINDOWS_SIGNING_CERT" "Windows signing certificate not found"
    return
  fi

  if [[ -z "${CFST_WINDOWS_SIGNING_CERT_SUBJECT:-}" && -z "${CFST_WINDOWS_SIGNING_CERT_THUMBPRINT:-}" ]]; then
    echo "missing required environment variable: CFST_WINDOWS_SIGNING_CERT" >&2
    echo "Windows installer signing requires CFST_WINDOWS_SIGNING_CERT, or CFST_WINDOWS_SIGNING_CERT_SUBJECT / CFST_WINDOWS_SIGNING_CERT_THUMBPRINT with CFST_WINDOWS_SIGNING_PASSWORD." >&2
    exit 1
  fi

  if [[ -z "${CFST_WINDOWS_SIGNING_PASSWORD:-}" ]]; then
    echo "missing required environment variable: CFST_WINDOWS_SIGNING_PASSWORD" >&2
    echo "Local Windows certificate export requires CFST_WINDOWS_SIGNING_PASSWORD." >&2
    exit 1
  fi

  require_tool "powershell.exe" "Windows certificate store export requires powershell.exe (WSL interop)."

  local cert_cache_dir="$CACHE_HOME/cfst-gui/windows-signing"
  local cert_cache_path="$cert_cache_dir/cfst-gui-local-signing.pfx"
  local cert_cache_native="$cert_cache_path"
  local ps_password="${CFST_WINDOWS_SIGNING_PASSWORD//\'/\'\'}"
  local ps_subject="${CFST_WINDOWS_SIGNING_CERT_SUBJECT:-}"
  local ps_thumbprint="${CFST_WINDOWS_SIGNING_CERT_THUMBPRINT:-}"
  mkdir -p "$cert_cache_dir"

  ps_subject="${ps_subject//\'/\'\'}"
  ps_thumbprint="${ps_thumbprint//\'/\'\'}"

  if command -v wslpath >/dev/null 2>&1; then
    cert_cache_native="$(wslpath -w "$cert_cache_path")"
  elif command -v cygpath >/dev/null 2>&1; then
    cert_cache_native="$(cygpath -w "$cert_cache_path")"
  fi

  cert_cache_native="${cert_cache_native//\'/\'\'}"

  powershell.exe -NoProfile -Command "\$password = ConvertTo-SecureString -String '$ps_password' -AsPlainText -Force; \$certs = Get-ChildItem Cert:\CurrentUser\My,Cert:\LocalMachine\My -CodeSigningCert; if ('$ps_thumbprint') { \$cert = \$certs | Where-Object { \$_.Thumbprint -eq '$ps_thumbprint' } | Select-Object -First 1 } else { \$cert = \$certs | Where-Object { \$_.Subject -eq '$ps_subject' } | Select-Object -First 1 }; if (-not \$cert) { throw 'Windows code signing certificate not found'; }; Export-PfxCertificate -Cert \$cert.PSPath -FilePath '$cert_cache_native' -Password \$password -Force | Out-Null" >/dev/null

  CFST_WINDOWS_SIGNING_CERT="$cert_cache_path"
  export CFST_WINDOWS_SIGNING_CERT
  require_file "$CFST_WINDOWS_SIGNING_CERT" "Windows signing certificate export not found"
}

discover_windows_signing_tool() {
  if [[ -n "${CFST_WINDOWS_SIGNING_TOOL:-}" ]]; then
    printf '%s\n' "$CFST_WINDOWS_SIGNING_TOOL"
    return 0
  fi

  if command -v SignTool.exe >/dev/null 2>&1; then
    command -v SignTool.exe
    return 0
  fi

  local sdk_root
  local candidate
  for sdk_root in \
    "/mnt/c/Program Files (x86)/Windows Kits/10/bin" \
    "/mnt/c/Program Files/Windows Kits/10/bin"; do
    [[ -d "$sdk_root" ]] || continue
    candidate="$(find "$sdk_root" -type f -iname 'signtool.exe' | grep '/x64/' | sort -V | tail -n 1 || true)"
    if [[ -n "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  return 1
}

require_tool() {
  local name="$1"
  local hint="$2"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "$name not found. $hint" >&2
    exit 1
  fi
}

windows_native_path() {
  local path="$1"
  if command -v wslpath >/dev/null 2>&1; then
    wslpath -w "$path"
    return
  fi
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$path"
    return
  fi
  printf '%s\n' "$path"
}

sign_windows_installer() {
  local target_path="$1"
  local target_native
  local cert_native
  local signing_tool
  local ps_password

  target_native="$(windows_native_path "$target_path")"
  cert_native="${CFST_WINDOWS_SIGNING_CERT_NATIVE:-$(windows_native_path "$CFST_WINDOWS_SIGNING_CERT")}"
  signing_tool="${CFST_WINDOWS_SIGNING_TOOL:-$(discover_windows_signing_tool)}"

  target_native="${target_native//\'/\'\'}"
  cert_native="${cert_native//\'/\'\'}"
  signing_tool="${signing_tool//\'/\'\'}"
  ps_password="${CFST_WINDOWS_SIGNING_PASSWORD:-}"
  ps_password="${ps_password//\'/\'\'}"

  if [[ -n "$ps_password" ]]; then
    powershell.exe -NoProfile -Command "& '$signing_tool' sign /fd SHA256 /f '$cert_native' /p '$ps_password' '$target_native'" >/dev/null
    return
  fi

  powershell.exe -NoProfile -Command "& '$signing_tool' sign /fd SHA256 /f '$cert_native' '$target_native'" >/dev/null
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
  printf 'https://github.com/%s/releases/latest/download/%s' "$repository" "$asset_name"
}

build_frontend() {
  cd "$ROOT_DIR"
  wails generate module
  cd "$FRONTEND_DIR"
  npm ci
  npm run build
}

build_windows() {
  require_windows_signing
  require_tool "makensis" "Install NSIS and add makensis to PATH."
  local signing_tool
  signing_tool="$(discover_windows_signing_tool)" || {
    echo "SignTool.exe not found. Install Windows SDK and add SignTool.exe to PATH, or set CFST_WINDOWS_SIGNING_TOOL." >&2
    exit 1
  }
  cd "$ROOT_DIR"
  if command -v wslpath >/dev/null 2>&1; then
    export CFST_WINDOWS_SIGNING_CERT_NATIVE="$(wslpath -w "$CFST_WINDOWS_SIGNING_CERT")"
    if [[ "$signing_tool" = /* ]]; then
      export CFST_WINDOWS_SIGNING_TOOL="$(wslpath -w "$signing_tool")"
    else
      export CFST_WINDOWS_SIGNING_TOOL="$signing_tool"
    fi
  elif command -v cygpath >/dev/null 2>&1; then
    export CFST_WINDOWS_SIGNING_CERT_NATIVE="$(cygpath -w "$CFST_WINDOWS_SIGNING_CERT")"
    if [[ "$signing_tool" = /* ]]; then
      export CFST_WINDOWS_SIGNING_TOOL="$(cygpath -w "$signing_tool")"
    else
      export CFST_WINDOWS_SIGNING_TOOL="$signing_tool"
    fi
  else
    export CFST_WINDOWS_SIGNING_CERT_NATIVE="$CFST_WINDOWS_SIGNING_CERT"
    export CFST_WINDOWS_SIGNING_TOOL="$signing_tool"
  fi
  rm -f "$WINDOWS_RELEASE_ASSET"
  wails build -platform windows/amd64 -nsis -tags tray -ldflags "$LD_FLAGS"
  require_file "$WINDOWS_RELEASE_ASSET" "Windows installer output not found"
  sign_windows_installer "$WINDOWS_RELEASE_ASSET"
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
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/app/cfst-webui", "--healthcheck"]
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
      TZ: ${TZ:-Asia/Shanghai}
      CFST_WEBUI_ADDR: 0.0.0.0:34115
      CFST_WEBUI_TOKEN: ${CFST_WEBUI_TOKEN:-change-me}
      CFST_GUI_PORTABLE_ROOT: /data
      CFST_WEBUI_ALLOWED_ROOTS: /data
    ports:
      - "${CFST_WEBUI_PORT:-34115}:34115"
    volumes:
      - cfst-webui-data:/data
    healthcheck:
      test: ["CMD", "/app/cfst-webui", "--healthcheck"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3

volumes:
  cfst-webui-data:
    name: ${CFST_DATA_VOLUME:-cfst-webui-data}
EOF
  cat > "$bundle_dir/docker-compose.host.yml" <<'EOF'
services:
  cfst-webui:
    network_mode: host
    ports: !reset []
EOF
  cat > "$bundle_dir/.env.example" <<EOF
CFST_WEBUI_PORT=34115
CFST_WEBUI_TOKEN=change-me
CFST_VERSION=$VERSION
CFST_DATA_VOLUME=cfst-webui-data
TZ=Asia/Shanghai
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
Application settings, scheduler rules, Cloudflare DNS push settings, GitHub export settings, exports, and backups are stored under `/data/data`.

Common commands:

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f
docker compose restart
docker compose down
```

Host network mode is optional. It removes port publishing and lets the service listen directly on the host network:

```bash
docker compose -f docker-compose.yml -f docker-compose.host.yml up -d --build
```

Back up the named volume:

```bash
docker run --rm \
  -v cfst-webui-data:/data:ro \
  -v "$PWD:/backup" \
  busybox tar -czf /backup/cfst-webui-data.tar.gz -C /data .
```

Restore the named volume:

```bash
docker run --rm \
  -v cfst-webui-data:/data \
  -v "$PWD:/backup" \
  busybox sh -c 'cd /data && tar -xzf /backup/cfst-webui-data.tar.gz'
```

Run the published GHCR image instead of building locally:

```bash
docker run -d \
  --name cfst-webui \
  --restart unless-stopped \
  -p 34115:34115 \
  -e TZ=Asia/Shanghai \
  -e CFST_WEBUI_ADDR=0.0.0.0:34115 \
  -e CFST_WEBUI_TOKEN=change-me \
  -e CFST_GUI_PORTABLE_ROOT=/data \
  -e CFST_WEBUI_ALLOWED_ROOTS=/data \
  -v cfst-webui-data:/data \
  ghcr.io/axuitomo/cfst-gui:latest
```

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
  bash "$ROOT_DIR/scripts/patch-android-gradle-warnings.sh"
  mkdir -p "$ANDROID_DIR/app/libs"
  "$GOMOBILE_BIN" bind \
    -androidapi 21 \
    -target=android/arm64,android/arm \
    -ldflags "$ANDROID_16K_LDFLAGS" \
    -o "$ANDROID_DIR/app/libs/mobileapi.aar" \
    github.com/axuitomo/CFST-GUI/mobileapi
  cd "$ANDROID_DIR"
  bash ./gradlew assembleRelease
  local arm64_apk="$ANDROID_DIR/app/build/outputs/apk/release/app-arm64-v8a-release.apk"
  local armv7_apk="$ANDROID_DIR/app/build/outputs/apk/release/app-armeabi-v7a-release.apk"
  local universal_apk="$ANDROID_DIR/app/build/outputs/apk/release/app-universal-release.apk"
  require_file "$arm64_apk" "Android arm64 release APK not found"
  require_file "$armv7_apk" "Android armeabi-v7a release APK not found"
  require_file "$universal_apk" "Android universal release APK not found"
  bash "$ROOT_DIR/scripts/check-android-page-alignment.sh" "$ANDROID_DIR/app/libs/mobileapi.aar" "$universal_apk"
  cp "$arm64_apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-arm64-v8a-release.apk"
  cp "$armv7_apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-armeabi-v7a-release.apk"
  cp "$universal_apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-release.apk"
}

write_manifest() {
  local windows="$WINDOWS_RELEASE_ASSET"
  local linux_amd64="$DESKTOP_DIR/cfst-gui-linux-amd64.tar.gz"
  local linux_arm64="$DESKTOP_DIR/cfst-gui-linux-arm64.tar.gz"
  local darwin_amd="$DESKTOP_DIR/cfst-gui-darwin-amd64.app.zip"
  local darwin_arm="$DESKTOP_DIR/cfst-gui-darwin-arm64.app.zip"
  local android_arm64="$ANDROID_RELEASE_DIR/cfst-gui-android-arm64-v8a-release.apk"
  local android_armv7="$ANDROID_RELEASE_DIR/cfst-gui-android-armeabi-v7a-release.apk"
  local android_universal="$ANDROID_RELEASE_DIR/cfst-gui-android-release.apk"
  require_file "$windows" "Windows asset missing"
  require_file "$linux_amd64" "Linux amd64 asset missing"
  require_file "$linux_arm64" "Linux arm64 asset missing"
  require_file "$darwin_amd" "macOS amd64 asset missing"
  require_file "$darwin_arm" "macOS arm64 asset missing"
  require_file "$android_arm64" "Android arm64 asset missing"
  require_file "$android_armv7" "Android armeabi-v7a asset missing"
  require_file "$android_universal" "Android universal asset missing"
  cat > "$RELEASE_DIR/cfst-gui-update-manifest.json" <<EOF
{
  "docker_image": "ghcr.io/axuitomo/cfst-gui:$VERSION",
  "version": "$VERSION",
  "assets": [
    {"goos":"windows","goarch":"amd64","platform":"windows/amd64","name":"cfst-gui-windows-amd64.exe","download_url":"$(release_asset_download_url "cfst-gui-windows-amd64.exe")","sha256":"$(hash_file "$windows")","install_mode":"windows_exe"},
    {"goos":"linux","goarch":"amd64","platform":"linux/amd64","name":"cfst-gui-linux-amd64.tar.gz","download_url":"$(release_asset_download_url "cfst-gui-linux-amd64.tar.gz")","sha256":"$(hash_file "$linux_amd64")","install_mode":"docker_compose"},
    {"goos":"linux","goarch":"arm64","platform":"linux/arm64","name":"cfst-gui-linux-arm64.tar.gz","download_url":"$(release_asset_download_url "cfst-gui-linux-arm64.tar.gz")","sha256":"$(hash_file "$linux_arm64")","install_mode":"docker_compose"},
    {"goos":"darwin","goarch":"amd64","platform":"darwin/amd64","name":"cfst-gui-darwin-amd64.app.zip","download_url":"$(release_asset_download_url "cfst-gui-darwin-amd64.app.zip")","sha256":"$(hash_file "$darwin_amd")","install_mode":"replace_app"},
    {"goos":"darwin","goarch":"arm64","platform":"darwin/arm64","name":"cfst-gui-darwin-arm64.app.zip","download_url":"$(release_asset_download_url "cfst-gui-darwin-arm64.app.zip")","sha256":"$(hash_file "$darwin_arm")","install_mode":"replace_app"},
    {"goos":"android","goarch":"universal","platform":"android","abi":"universal","name":"cfst-gui-android-release.apk","download_url":"$(release_asset_download_url "cfst-gui-android-release.apk")","sha256":"$(hash_file "$android_universal")","install_mode":"android_apk"},
    {"goos":"android","goarch":"arm64","platform":"android","abi":"arm64-v8a","name":"cfst-gui-android-arm64-v8a-release.apk","download_url":"$(release_asset_download_url "cfst-gui-android-arm64-v8a-release.apk")","sha256":"$(hash_file "$android_arm64")","install_mode":"android_apk"},
    {"goos":"android","goarch":"arm","platform":"android","abi":"armeabi-v7a","name":"cfst-gui-android-armeabi-v7a-release.apk","download_url":"$(release_asset_download_url "cfst-gui-android-armeabi-v7a-release.apk")","sha256":"$(hash_file "$android_armv7")","install_mode":"android_apk"}
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
