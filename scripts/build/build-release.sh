#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
ANDROID_DIR="$ROOT_DIR/mobile/android"
RELEASE_DIR="$ROOT_DIR/build/artifacts/release"
DESKTOP_DIR="$RELEASE_DIR/desktop"
ANDROID_RELEASE_DIR="$RELEASE_DIR/android"
WINDOWS_RELEASE_ASSET="$DESKTOP_DIR/cfst-gui-windows-amd64.exe"
WINDOWS_PORTABLE_ASSET="$DESKTOP_DIR/cfst-gui-windows-amd64-portable.exe"
# 桌面版：windowsgui 子系统，双击不附带控制台窗口。
WINDOWS_APP_BINARY="$ROOT_DIR/build/bin/cfst-gui.exe"
WINDOWS_SYSO="$ROOT_DIR/wails_windows_amd64.syso"
# 安装器是独立的 Go 程序，主程序作为载荷嵌进去（cmd/installer/payload/）。
WINDOWS_INSTALLER_PKG="$ROOT_DIR/cmd/installer"
WINDOWS_INSTALLER_PAYLOAD="$WINDOWS_INSTALLER_PKG/payload/cfst-gui.exe"
WINDOWS_INSTALLER_LICENSE="$WINDOWS_INSTALLER_PKG/payload/LICENSE"
WINDOWS_INSTALLER_NOTICES="$WINDOWS_INSTALLER_PKG/payload/THIRD-PARTY-NOTICES.md"
WINDOWS_INSTALLER_SYSO="$WINDOWS_INSTALLER_PKG/installer_windows_amd64.syso"
WINDOWS_INSTALLER_MANIFEST="$ROOT_DIR/build/windows/installer.exe.manifest"
VERSION="${CFST_VERSION:-2.1.4}"
GOMOBILE_BIN="${GOMOBILE_BIN:-$(go env GOPATH)/bin/gomobile}"
# -s -w 去掉符号表与 DWARF（27.5MB → 19.8MB），-trimpath 让二进制里不再带本机绝对路径，
# 顺带让构建可复现。Go 的行号来自 pclntab 而不是 DWARF，所以 strip 后 panic 栈与
# `go tool pprof -lines` 仍有 file:line（已实测），丢掉的只有 Delve 调试与变量/类型信息。
LD_FLAGS="-s -w -X github.com/axuitomo/CFST-GUI/internal/app.version=$VERSION"
# -H windowsgui 把桌面版切成 GUI 子系统，避免双击时额外弹出控制台窗口。
LD_FLAGS_WINDOWS_GUI="$LD_FLAGS -H windowsgui"
TARGET="${1:-all}"
RELEASE_TARGETS="${CFST_RELEASE_TARGETS:-windows,linux,android}"
CACHE_HOME="${XDG_CACHE_HOME:-${HOME:-/tmp}/.cache}"
DEFAULT_ANDROID_SDK_HOME="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$CACHE_HOME/cfst-gui/android-toolchain/android-sdk}}"
DEFAULT_ANDROID_NDK_HOME="${ANDROID_NDK_HOME:-$DEFAULT_ANDROID_SDK_HOME/ndk/30.0.16248370}"
ANDROID_16K_LDFLAGS='-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'
GOMOBILE_TIMEOUT_SECONDS="${CFST_GOMOBILE_TIMEOUT_SECONDS:-1800}"
GOMOBILE_CGO_ENABLED="${CFST_GOMOBILE_CGO_ENABLED:-0}"

require_file() {
  local path="$1"
  local message="$2"
  if [[ ! -f "$path" ]]; then
    echo "$message: $path" >&2
    exit 1
  fi
}

clean_gomobile_temp() {
  local -a temp_roots=()
  temp_roots+=("${TMPDIR:-}")
  temp_roots+=("${TEMP:-}")
  temp_roots+=("${TMP:-}")
  temp_roots+=("${LOCALAPPDATA:-}/Temp")
  temp_roots+=("${USERPROFILE:-}/AppData/Local/Temp")
  temp_roots+=("/tmp")
  local temp_root
  for temp_root in "${temp_roots[@]}"; do
    if [[ -n "$temp_root" && -d "$temp_root" ]]; then
      find "$temp_root" -maxdepth 1 -type d -name 'gomobile-*' -exec rm -rf {} + 2>/dev/null || true
    fi
  done
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

windows_allow_self_signed() {
  [[ "${CFST_WINDOWS_ALLOW_SELF_SIGNED:-1}" != "0" ]]
}

# 生成临时自签代码签名证书：PFX 留在缓存目录，证书本身从证书存储移除。
# 仅在允许自签且没有配置任何签名证书时启用。
generate_windows_self_signed_cert() {
  require_tool "powershell.exe" "Windows self-signed certificate generation requires powershell.exe."

  local cert_cache_dir="$CACHE_HOME/cfst-gui/windows-signing"
  local cert_cache_path="$cert_cache_dir/cfst-gui-self-signed.pfx"
  local cert_cache_native="$cert_cache_path"
  local cert_password
  cert_password="$(openssl rand -hex 16 2>/dev/null || true)"
  if [[ -z "$cert_password" ]]; then
    cert_password="cfst$(date +%s%N)$$"
  fi
  mkdir -p "$cert_cache_dir"

  if command -v wslpath >/dev/null 2>&1; then
    cert_cache_native="$(wslpath -w "$cert_cache_path")"
  elif command -v cygpath >/dev/null 2>&1; then
    cert_cache_native="$(cygpath -w "$cert_cache_path")"
  fi

  cert_cache_native="${cert_cache_native//\'/\'\'}"
  cert_password="${cert_password//\'/\'\'}"

  powershell.exe -NoProfile -NonInteractive -Command "\$credential = ConvertTo-SecureString -String '$cert_password' -AsPlainText -Force; \$cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject 'CN=CFST-GUI Self-Signed' -CertStoreLocation Cert:\CurrentUser\My -KeyExportPolicy Exportable -NotAfter (Get-Date).AddYears(2); try { Export-PfxCertificate -Cert \$cert.PSPath -FilePath '$cert_cache_native' -Password \$credential -Force | Out-Null } finally { Remove-Item -LiteralPath \$cert.PSPath -Force -ErrorAction SilentlyContinue }" >/dev/null

  CFST_WINDOWS_SIGNING_CERT="$cert_cache_path"
  CFST_WINDOWS_SIGNING_PASSWORD="$cert_password"
  export CFST_WINDOWS_SIGNING_CERT CFST_WINDOWS_SIGNING_PASSWORD
  echo "using generated self-signed Windows signing certificate; set CFST_WINDOWS_ALLOW_SELF_SIGNED=0 to require a trusted certificate." >&2
  require_file "$CFST_WINDOWS_SIGNING_CERT" "Windows signing certificate export not found"
}

require_windows_signing() {
  if [[ -n "${CFST_WINDOWS_SIGNING_CERT:-}" ]]; then
    require_file "$CFST_WINDOWS_SIGNING_CERT" "Windows signing certificate not found"
    return
  fi

  if [[ -n "${CFST_WINDOWS_SIGNING_CERT_SUBJECT:-}" || -n "${CFST_WINDOWS_SIGNING_CERT_THUMBPRINT:-}" ]]; then
    if [[ -z "${CFST_WINDOWS_SIGNING_PASSWORD:-}" ]]; then
      echo "missing required environment variable: CFST_WINDOWS_SIGNING_PASSWORD" >&2
      echo "Local Windows certificate export requires CFST_WINDOWS_SIGNING_PASSWORD." >&2
      exit 1
    fi

    require_tool "powershell.exe" "Windows certificate store export requires powershell.exe."

    local cert_cache_dir="$CACHE_HOME/cfst-gui/windows-signing"
    local cert_cache_path="$cert_cache_dir/cfst-gui-local-signing.pfx"
    local cert_cache_native="$cert_cache_path"
    local ps_credential="${CFST_WINDOWS_SIGNING_PASSWORD//\'/\'\'}"
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

    powershell.exe -NoProfile -Command "\$credential = ConvertTo-SecureString -String '$ps_credential' -AsPlainText -Force; \$certs = Get-ChildItem Cert:\CurrentUser\My,Cert:\LocalMachine\My -CodeSigningCert; if ('$ps_thumbprint') { \$cert = \$certs | Where-Object { \$_.Thumbprint -eq '$ps_thumbprint' } | Select-Object -First 1 } else { \$cert = \$certs | Where-Object { \$_.Subject -eq '$ps_subject' } | Select-Object -First 1 }; if (-not \$cert) { throw 'Windows code signing certificate not found'; }; Export-PfxCertificate -Cert \$cert.PSPath -FilePath '$cert_cache_native' -Password \$credential -Force | Out-Null" >/dev/null

    CFST_WINDOWS_SIGNING_CERT="$cert_cache_path"
    export CFST_WINDOWS_SIGNING_CERT
    require_file "$CFST_WINDOWS_SIGNING_CERT" "Windows signing certificate export not found"
    return
  fi

  if windows_allow_self_signed; then
    generate_windows_self_signed_cert
    return
  fi

  echo "missing required environment variable: CFST_WINDOWS_SIGNING_CERT" >&2
  echo "Windows installer signing requires CFST_WINDOWS_SIGNING_CERT, or CFST_WINDOWS_SIGNING_CERT_SUBJECT / CFST_WINDOWS_SIGNING_CERT_THUMBPRINT with CFST_WINDOWS_SIGNING_PASSWORD. Set CFST_WINDOWS_ALLOW_SELF_SIGNED=1 to allow a generated self-signed certificate." >&2
  exit 1
}

require_macos_signing() {
  local missing=0
  for name in CFST_MACOS_SIGNING_IDENTITY CFST_APPLE_ID CFST_APPLE_APP_PASSWORD CFST_APPLE_TEAM_ID; do
    if [[ -z "${!name:-}" ]]; then
      echo "missing required environment variable: $name" >&2
      missing=1
    fi
  done
  if [[ "$missing" -ne 0 ]]; then
    echo "macOS Release signing and notarization require a Developer ID identity plus Apple notary credentials." >&2
    exit 1
  fi
  require_tool "codesign" "Install the Xcode command line tools on a macOS host."
  require_tool "xcrun" "Install Xcode or the Xcode command line tools on a macOS host."
  require_tool "ditto" "macOS notarization requires the system ditto tool."
}

sign_and_notarize_macos() {
  local app="$1"
  if [[ "${CFST_REQUIRE_MACOS_SIGNING:-0}" != "1" && -z "${CFST_MACOS_SIGNING_IDENTITY:-}" ]]; then
    return
  fi

  require_macos_signing
  codesign --force --deep --options runtime --timestamp --sign "$CFST_MACOS_SIGNING_IDENTITY" "$app"
  codesign --verify --deep --strict --verbose=2 "$app"

  local notary_dir
  local notary_archive
  notary_dir="$(mktemp -d "${TMPDIR:-/tmp}/cfst-gui-notary.XXXXXX")"
  notary_archive="$notary_dir/CFST-GUI.app.zip"
  ditto -c -k --keepParent "$app" "$notary_archive"
  xcrun notarytool submit "$notary_archive" \
    --apple-id "$CFST_APPLE_ID" \
    --password "$CFST_APPLE_APP_PASSWORD" \
    --team-id "$CFST_APPLE_TEAM_ID" \
    --wait
  rm -rf "$notary_dir"

  xcrun stapler staple "$app"
  xcrun stapler validate "$app"
  codesign --verify --deep --strict --verbose=2 "$app"
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
    "/mnt/c/Program Files/Windows Kits/10/bin" \
    "/c/Program Files (x86)/Windows Kits/10/bin" \
    "/c/Program Files/Windows Kits/10/bin"; do
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

# SignTool 对自签证书、私有链和时间戳链会打印警告。警告保留在日志里，
# 只有真正的 Error 才中断发布构建。
invoke_signtool() {
  local output
  local status
  if output="$("$@" 2>&1)"; then
    status=0
  else
    status=$?
  fi
  if [[ -n "$output" ]]; then
    printf '%s\n' "$output" >&2
  fi
  if ((status == 0)); then
    return 0
  fi
  if grep -qiE 'signtool error|^error:' <<<"$output"; then
    return "$status"
  fi
  if grep -qi 'warning' <<<"$output"; then
    printf 'signtool exited %d with warnings only; continuing.\n' "$status" >&2
    return 0
  fi
  return "$status"
}

sign_windows_installer() {
  local target_path="$1"
  local target_native
  local cert_native
  local signing_tool
  local ps_credential

  target_native="$(windows_native_path "$target_path")"
  cert_native="${CFST_WINDOWS_SIGNING_CERT_NATIVE:-$(windows_native_path "$CFST_WINDOWS_SIGNING_CERT")}"
  signing_tool="${CFST_WINDOWS_SIGNING_TOOL:-$(discover_windows_signing_tool)}"

  target_native="${target_native//\'/\'\'}"
  cert_native="${cert_native//\'/\'\'}"
  signing_tool="${signing_tool//\'/\'\'}"
  ps_credential="${CFST_WINDOWS_SIGNING_PASSWORD:-}"
  ps_credential="${ps_credential//\'/\'\'}"

  if [[ -n "$ps_credential" ]]; then
    invoke_signtool powershell.exe -NoProfile -NonInteractive -Command "& '$signing_tool' sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /f '$cert_native' /p '$ps_credential' '$target_native'"
    return
  fi

  invoke_signtool powershell.exe -NoProfile -NonInteractive -Command "& '$signing_tool' sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /f '$cert_native' '$target_native'"
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
  printf 'https://github.com/%s/releases/download/v%s/%s' "$repository" "$VERSION" "$asset_name"
}

build_frontend() {
  cd "$ROOT_DIR"
  wails3 generate bindings
  require_file "$FRONTEND_DIR/bindings/github.com/axuitomo/CFST-GUI/internal/app/app.js" "Wails bindings were not generated"
  cd "$FRONTEND_DIR"
  pnpm install --frozen-lockfile
  require_file "$FRONTEND_DIR/node_modules/@wailsio/runtime/package.json" "Wails frontend runtime is not installed"
  pnpm --dir "$FRONTEND_DIR" run build
  node "$ROOT_DIR/scripts/build/generate-third-party-notices.mjs" --check
  node --test "$ROOT_DIR/scripts/build/inline-license-notices.test.mjs" "$ROOT_DIR/scripts/build/legal-assets.test.mjs"
}

# 安装器自己的版本资源：图标 + requireAdministrator 清单 + PE 版本信息。
# 与主程序共用同一套 info.json 字段，只是 FileDescription 不同。
generate_windows_installer_syso() {
  require_tool "wails3" "Install the Wails v3 CLI to generate the installer's Windows resources."
  require_file "$WINDOWS_INSTALLER_MANIFEST" "Installer manifest is missing"
  local numeric_version
  numeric_version="$(windows_numeric_version)"
  local work_dir
  work_dir="$(mktemp -d "${TMPDIR:-/tmp}/cfst-gui-installer-syso.XXXXXX")"
  {
    printf '{
'
    printf '  "fixed": {"file_version": "%s", "product_version": "%s"},
' "$numeric_version" "$numeric_version"
    printf '  "info": {
    "0000": {
'
    printf '      "Comments": "CFST-GUI 安装程序",
'
    printf '      "CompanyName": "axuitomo",
'
    printf '      "FileDescription": "CFST-GUI 安装程序",
'
    printf '      "FileVersion": "%s",
' "$VERSION"
    printf '      "LegalCopyright": "Copyright (C) 2026 axuitomo. Licensed under GPL-3.0-only.",
'
    printf '      "ProductName": "CFST-GUI",
'
    printf '      "ProductVersion": "%s"
' "$VERSION"
    printf '    }
  }
}
'
  } > "$work_dir/info.json"
  rm -f "$WINDOWS_INSTALLER_SYSO"
  wails3 generate syso -arch amd64     -icon "$(windows_native_path "$ROOT_DIR/build/windows/icon.ico")"     -manifest "$(windows_native_path "$WINDOWS_INSTALLER_MANIFEST")"     -info "$(windows_native_path "$work_dir/info.json")"     -out "$(windows_native_path "$WINDOWS_INSTALLER_SYSO")"
  require_file "$WINDOWS_INSTALLER_SYSO" "Installer version resource (.syso) was not generated"
  rm -rf "$work_dir"
}

windows_numeric_version() {
  local numeric_version="${VERSION%%-*}"
  local major minor patch
  major="${numeric_version%%.*}"
  minor="${numeric_version#*.}"; minor="${minor%%.*}"
  # 两段式版本号（如 1.9）没有 patch 字段，PE 固定版本补 0。
  case "$numeric_version" in
    *.*.*) patch="${numeric_version#*.*.}"; patch="${patch%%.*}" ;;
    *) patch="0" ;;
  esac
  printf '%s.%s.%s.0' "$major" "$minor" "$patch"
}

# 生成带 VS_VERSIONINFO 的 .syso，使 Windows 产物在资源管理器里能看到版本号。
# 仓库里的 build/windows/info.json 仍保留 {{.Info.*}} 模板占位，wails3 generate syso
# 需要已渲染的实体文件，因此这里按当前 VERSION 现场渲染后再生成。
generate_windows_syso() {
  require_tool "wails3" "Install the Wails v3 CLI to generate the Windows version resource."
  local numeric_version
  numeric_version="$(windows_numeric_version)"
  local work_dir
  work_dir="$(mktemp -d "${TMPDIR:-/tmp}/cfst-gui-syso.XXXXXX")"
  {
    printf '{\n'
    printf '  "fixed": {"file_version": "%s", "product_version": "%s"},\n' "$numeric_version" "$numeric_version"
    printf '  "info": {\n    "0000": {\n'
    printf '      "Comments": "Cloudflare/CDN IP 测速工具",\n'
    printf '      "CompanyName": "axuitomo",\n'
    printf '      "FileDescription": "CFST-GUI",\n'
    printf '      "FileVersion": "%s",\n' "$VERSION"
    printf '      "LegalCopyright": "Copyright (C) 2026 axuitomo. Licensed under GPL-3.0-only.",\n'
    printf '      "ProductName": "CFST-GUI",\n'
    printf '      "ProductVersion": "%s"\n' "$VERSION"
    printf '    }\n  }\n}\n'
  } > "$work_dir/info.json"
  # wails3 是原生 Windows 程序，不认 Git Bash 的 /c/... 路径，四个路径都要转换。
  wails3 generate syso -arch amd64 \
    -icon "$(windows_native_path "$ROOT_DIR/build/windows/icon.ico")" \
    -manifest "$(windows_native_path "$ROOT_DIR/build/windows/wails.exe.manifest")" \
    -info "$(windows_native_path "$work_dir/info.json")" \
    -out "$(windows_native_path "$WINDOWS_SYSO")"
  require_file "$WINDOWS_SYSO" "Windows version resource (.syso) was not generated"
  rm -rf "$work_dir"
}

# 安装器 = 内嵌主程序载荷的 Go 程序。
# 载荷是已经签过名的主程序二进制，安装出来的应用因此仍带签名。
build_windows_installer() {
  cp "$WINDOWS_APP_BINARY" "$WINDOWS_INSTALLER_PAYLOAD"
  generate_windows_installer_syso
  cp "$ROOT_DIR/LICENSE" "$WINDOWS_INSTALLER_LICENSE"
  cp "$ROOT_DIR/THIRD-PARTY-NOTICES.md" "$WINDOWS_INSTALLER_NOTICES"
  # 主程序载荷、许可证副本与 .syso 仅供 go build 使用，成功或失败后都会清理，
  # 避免 30MB 载荷和法律文件副本残留在工作区。
  if ! go build -trimpath \
      -ldflags "$LD_FLAGS_WINDOWS_GUI -X main.version=$VERSION" \
      -o "$WINDOWS_RELEASE_ASSET" ./cmd/installer; then
    rm -f "$WINDOWS_INSTALLER_PAYLOAD" "$WINDOWS_INSTALLER_LICENSE" "$WINDOWS_INSTALLER_NOTICES" "$WINDOWS_INSTALLER_SYSO"
    return 1
  fi
  rm -f "$WINDOWS_INSTALLER_PAYLOAD" "$WINDOWS_INSTALLER_LICENSE" "$WINDOWS_INSTALLER_NOTICES" "$WINDOWS_INSTALLER_SYSO"
  require_file "$WINDOWS_RELEASE_ASSET" "Windows installer output not found"
  sign_windows_installer "$WINDOWS_RELEASE_ASSET"
}

build_windows() {
  require_windows_signing
  local signing_tool
  signing_tool="$(discover_windows_signing_tool)" || {
    echo "SignTool.exe not found. Install Windows SDK and add SignTool.exe to PATH, or set CFST_WINDOWS_SIGNING_TOOL." >&2
    exit 1
  }
  cd "$ROOT_DIR"
  if command -v wslpath >/dev/null 2>&1; then
    CFST_WINDOWS_SIGNING_CERT_NATIVE="$(wslpath -w "$CFST_WINDOWS_SIGNING_CERT")"
    export CFST_WINDOWS_SIGNING_CERT_NATIVE
    if [[ "$signing_tool" = /* ]]; then
      CFST_WINDOWS_SIGNING_TOOL="$(wslpath -w "$signing_tool")"
      export CFST_WINDOWS_SIGNING_TOOL
    else
      CFST_WINDOWS_SIGNING_TOOL="$signing_tool"
      export CFST_WINDOWS_SIGNING_TOOL
    fi
  elif command -v cygpath >/dev/null 2>&1; then
    CFST_WINDOWS_SIGNING_CERT_NATIVE="$(cygpath -w "$CFST_WINDOWS_SIGNING_CERT")"
    export CFST_WINDOWS_SIGNING_CERT_NATIVE
    if [[ "$signing_tool" = /* ]]; then
      CFST_WINDOWS_SIGNING_TOOL="$(cygpath -w "$signing_tool")"
      export CFST_WINDOWS_SIGNING_TOOL
    else
      CFST_WINDOWS_SIGNING_TOOL="$signing_tool"
      export CFST_WINDOWS_SIGNING_TOOL
    fi
  else
    CFST_WINDOWS_SIGNING_CERT_NATIVE="$CFST_WINDOWS_SIGNING_CERT"
    CFST_WINDOWS_SIGNING_TOOL="$signing_tool"
    export CFST_WINDOWS_SIGNING_CERT_NATIVE CFST_WINDOWS_SIGNING_TOOL
  fi
  mkdir -p "$DESKTOP_DIR" "$(dirname "$WINDOWS_APP_BINARY")"
  rm -f "$WINDOWS_RELEASE_ASSET" "$WINDOWS_PORTABLE_ASSET" \
    "$WINDOWS_APP_BINARY" "$WINDOWS_SYSO"
  # .syso 必须落在 main 包目录（仓库根）才会被 go build 链接，构建结束后立即清理。
  generate_windows_syso
  go build -trimpath -tags production,tray -ldflags "$LD_FLAGS_WINDOWS_GUI" -o "$WINDOWS_APP_BINARY" .
  require_file "$WINDOWS_APP_BINARY" "Windows desktop binary not found"
  rm -f "$WINDOWS_SYSO"
  sign_windows_installer "$WINDOWS_APP_BINARY"
  cp "$WINDOWS_APP_BINARY" "$WINDOWS_PORTABLE_ASSET"
  build_windows_installer
}

linux_bundle_dir() {
  local arch="$1"
  printf '%s/build/artifacts/webui-linux-%s' "$ROOT_DIR" "$arch"
}

linux_bundle_archive() {
  local arch="$1"
  printf '%s/cfst-gui-linux-%s.tar.gz' "$DESKTOP_DIR" "$arch"
}

write_linux_bundle_files() {
  local bundle_dir="$1"
  cp "$ROOT_DIR/LICENSE" "$bundle_dir/LICENSE"
  cp "$ROOT_DIR/THIRD-PARTY-NOTICES.md" "$bundle_dir/THIRD-PARTY-NOTICES.md"
  local source_commit
  source_commit="$(git -C "$ROOT_DIR" rev-parse HEAD)"
  printf 'Corresponding Source: https://github.com/axuitomo/CFST-GUI/archive/%s.tar.gz\nBuild instructions: https://github.com/axuitomo/CFST-GUI/blob/%s/docs/guide/deployment.md\n' "$source_commit" "$source_commit" > "$bundle_dir/SOURCE.txt"
  # S5: 配置模板外置到 scripts/build/templates/webui-bundle/（真实文件，不再是
  # heredoc 字符串）；.env.example 的 @VERSION@ 由 sed 工具侧组装，其余纯静态 cp。
  local template_dir="$ROOT_DIR/scripts/build/templates/webui-bundle"
  cp "$template_dir/Dockerfile" "$bundle_dir/Dockerfile"
  cp "$template_dir/docker-compose.yml" "$bundle_dir/docker-compose.yml"
  cp "$template_dir/docker-compose.host.yml" "$bundle_dir/docker-compose.host.yml"
  local version_escaped
  version_escaped="$(printf '%s' "$VERSION" | sed 's/[&|\\]/\\&/g')"
  sed "s|@VERSION@|$version_escaped|g" "$template_dir/.env.example" > "$bundle_dir/.env.example"
  cp "$template_dir/run-local.sh" "$bundle_dir/run-local.sh"
  chmod +x "$bundle_dir/run-local.sh"
  cp "$template_dir/README.md" "$bundle_dir/README.md"
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
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -tags webui -ldflags "$LD_FLAGS" -o "$bundle_dir/cfst-webui" .
  require_file "$bundle_dir/cfst-webui" "Linux WebUI build output not found"
  if [[ -f /etc/ssl/certs/ca-certificates.crt ]]; then
    cp /etc/ssl/certs/ca-certificates.crt "$bundle_dir/ca-certificates.crt"
  fi
  require_file "$bundle_dir/ca-certificates.crt" "CA certificates bundle not found"
  if [[ -f /usr/share/doc/ca-certificates/copyright ]]; then
    cp /usr/share/doc/ca-certificates/copyright "$bundle_dir/CA-CERTIFICATES-COPYRIGHT"
  elif [[ -f /usr/share/licenses/ca-certificates/LICENSE ]]; then
    cp /usr/share/licenses/ca-certificates/LICENSE "$bundle_dir/CA-CERTIFICATES-COPYRIGHT"
  else
    echo "CA certificates copyright notice not found" >&2
    exit 1
  fi
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
  GOARCH="$arch" wails3 build -tags tray
  local app="$ROOT_DIR/build/bin/CFST-GUI.app"
  require_file "$app/Contents/MacOS/cfst-gui" "macOS build output not found"
  mkdir -p "$app/Contents/Resources"
  cp "$ROOT_DIR/LICENSE" "$app/Contents/Resources/LICENSE"
  cp "$ROOT_DIR/THIRD-PARTY-NOTICES.md" "$app/Contents/Resources/THIRD-PARTY-NOTICES.md"
  sign_and_notarize_macos "$app"
  (cd "$ROOT_DIR/build/bin" && zip -qry "$DESKTOP_DIR/cfst-gui-darwin-$arch.app.zip" "CFST-GUI.app")
  rm -rf "$app"
}
build_android() {
  require_android_signing
  export ANDROID_NDK_HOME="$DEFAULT_ANDROID_NDK_HOME"
  if [[ ! -x "$GOMOBILE_BIN" ]]; then
    echo "gomobile not found at $GOMOBILE_BIN; run: go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20261011034528-197770444867" >&2
    exit 1
  fi
  clean_gomobile_temp
  cd "$FRONTEND_DIR"
  pnpm exec cap sync android
  bash "$ROOT_DIR/scripts/checks/check-android-fileprovider-resources.sh"
  bash "$ROOT_DIR/scripts/build/patch-android-gradle-warnings.sh"
  mkdir -p "$ANDROID_DIR/app/libs"
  export CGO_ENABLED="$GOMOBILE_CGO_ENABLED"
  local -a bind_command=("$GOMOBILE_BIN" bind -androidapi 21 -target=android/arm64 -ldflags "$ANDROID_16K_LDFLAGS" -o "$ANDROID_DIR/app/libs/mobileapi.aar" github.com/axuitomo/CFST-GUI/mobileapi)
  if command -v timeout >/dev/null 2>&1; then
    timeout --signal=TERM "${GOMOBILE_TIMEOUT_SECONDS}s" "${bind_command[@]}"
  else
    "${bind_command[@]}"
  fi
  clean_gomobile_temp
  cd "$ANDROID_DIR"
  bash ./gradlew -I "$ROOT_DIR/scripts/build/android-license-report.init.gradle" --no-configuration-cache :app:generateLicenseReport
  node "$ROOT_DIR/scripts/build/check-android-license-report.mjs"
  bash ./gradlew assembleRelease assembleDebug
  local arm64_apk="$ANDROID_DIR/app/build/outputs/apk/release/app-arm64-v8a-release.apk"
  local debug_apk="$ANDROID_DIR/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk"
  require_file "$arm64_apk" "Android arm64 release APK not found"
  require_file "$debug_apk" "Android arm64 debug APK not found"
  bash "$ROOT_DIR/scripts/checks/check-android-page-alignment.sh" "$ANDROID_DIR/app/libs/mobileapi.aar" "$arm64_apk" "$debug_apk"
  bash "$ROOT_DIR/scripts/checks/check-android-apk-manifest.sh" "$arm64_apk" "$debug_apk"
  node "$ROOT_DIR/scripts/checks/verify-android-release.mjs" "$arm64_apk"
  node --test "$ROOT_DIR/scripts/checks/verify-android-release.test.mjs"
  local aapt="$DEFAULT_ANDROID_SDK_HOME/build-tools/37.0.0/aapt"
  [[ -f "$aapt" ]] || aapt="$DEFAULT_ANDROID_SDK_HOME/build-tools/37.0.0/aapt.exe"
  require_file "$aapt" "Android aapt tool not found"
  local apk badging apk_files
  for apk in "$arm64_apk" "$debug_apk"; do
    badging="$("$aapt" dump badging "$apk")"
    apk_files="$("$aapt" list "$apk")"
    for legal_file in assets/public/LICENSE assets/public/THIRD-PARTY-NOTICES.md; do
      grep -Fxq "$legal_file" <<<"$apk_files" || {
        echo "Android APK is missing $legal_file: $apk" >&2
        exit 1
      }
    done
    grep -F "versionName='$VERSION'" <<<"$badging" >/dev/null || {
      echo "Android APK version mismatch: expected $VERSION in $apk" >&2
      exit 1
    }
    grep -F "native-code: 'arm64-v8a'" <<<"$badging" >/dev/null || {
      echo "Android APK ABI mismatch: $apk" >&2
      exit 1
    }
    if [[ "$apk" == "$debug_apk" ]]; then
      grep -F "application-debuggable" <<<"$badging" >/dev/null || {
        echo "Android debug APK is not debuggable" >&2
        exit 1
      }
    fi
  done
  cp "$arm64_apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-arm64-v8a-release.apk"
  cp "$debug_apk" "$ANDROID_RELEASE_DIR/cfst-gui-android-arm64-v8a-debug.apk"
}

write_manifest() {
  local windows="$WINDOWS_RELEASE_ASSET"
  local linux_amd64="$DESKTOP_DIR/cfst-gui-linux-amd64.tar.gz"
  local linux_arm64="$DESKTOP_DIR/cfst-gui-linux-arm64.tar.gz"
  local android_arm64="$ANDROID_RELEASE_DIR/cfst-gui-android-arm64-v8a-release.apk"
  local -a assets=()
  local asset

  require_file "$windows" "Windows asset missing"
  require_file "$android_arm64" "Android arm64 asset missing"
  # 只登记桌面安装器：updatecore.MatchManifestAsset 按 goos/goarch 取第一个匹配项，
  # 不区分 install_mode。若把 CLI 产物也写进清单，桌面端在线更新可能选中 CLI 二进制。
  # cfst-gui-windows-amd64-cli.exe 仅作为本地构建产物，不发布到 GitHub Release，也不参与自动更新。
  assets+=("    {\"goos\":\"windows\",\"goarch\":\"amd64\",\"platform\":\"windows/amd64\",\"name\":\"cfst-gui-windows-amd64.exe\",\"download_url\":\"$(release_asset_download_url "cfst-gui-windows-amd64.exe")\",\"sha256\":\"$(hash_file "$windows")\",\"install_mode\":\"windows_exe\"}")
  if [[ -f "$linux_amd64" ]]; then
    assets+=("    {\"goos\":\"linux\",\"goarch\":\"amd64\",\"platform\":\"linux/amd64\",\"name\":\"cfst-gui-linux-amd64.tar.gz\",\"download_url\":\"$(release_asset_download_url "cfst-gui-linux-amd64.tar.gz")\",\"sha256\":\"$(hash_file "$linux_amd64")\",\"install_mode\":\"docker_compose\"}")
  fi
  if [[ -f "$linux_arm64" ]]; then
    assets+=("    {\"goos\":\"linux\",\"goarch\":\"arm64\",\"platform\":\"linux/arm64\",\"name\":\"cfst-gui-linux-arm64.tar.gz\",\"download_url\":\"$(release_asset_download_url "cfst-gui-linux-arm64.tar.gz")\",\"sha256\":\"$(hash_file "$linux_arm64")\",\"install_mode\":\"docker_compose\"}")
  fi
  assets+=("    {\"goos\":\"android\",\"goarch\":\"arm64\",\"platform\":\"android\",\"abi\":\"arm64-v8a\",\"name\":\"cfst-gui-android-arm64-v8a-release.apk\",\"download_url\":\"$(release_asset_download_url "cfst-gui-android-arm64-v8a-release.apk")\",\"sha256\":\"$(hash_file "$android_arm64")\",\"install_mode\":\"android_apk\"}")
  {
    printf '{\n  \"docker_image\": \"ghcr.io/axuitomo/cfst-gui:%s\",\n  \"version\": \"%s\",\n  \"assets\": [\n' "$VERSION" "$VERSION"
    for ((i = 0; i < ${#assets[@]}; i++)); do
      asset="${assets[$i]}"
      if ((i > 0)); then printf ',\n'; fi
      printf '%s' "$asset"
    done
    printf '\n  ]\n}\n'
  } > "$RELEASE_DIR/cfst-gui-update-manifest.json"
}

build_release_targets() {
  local -a targets=()
  local target
  IFS=',' read -r -a targets <<< "$RELEASE_TARGETS"
  build_frontend
  for target in "${targets[@]}"; do
    case "$target" in
      windows) build_windows ;;
      linux) build_linux ;;
      android) build_android ;;
      macos) build_macos amd64; build_macos arm64 ;;
      '') ;;
      *) echo "unsupported CFST_RELEASE_TARGETS entry: $target" >&2; exit 2 ;;
    esac
  done
}

require_release_targets() {
  local -a targets=()
  local target
  IFS=',' read -r -a targets <<< "$RELEASE_TARGETS"
  for target in "${targets[@]}"; do
    case "$target" in
      windows) require_file "$WINDOWS_RELEASE_ASSET" "Windows release asset missing" ;;
      linux) require_file "$DESKTOP_DIR/cfst-gui-linux-amd64.tar.gz" "Linux amd64 release asset missing"; require_file "$DESKTOP_DIR/cfst-gui-linux-arm64.tar.gz" "Linux arm64 release asset missing" ;;
      android) require_file "$ANDROID_RELEASE_DIR/cfst-gui-android-arm64-v8a-release.apk" "Android arm64 release asset missing" ;;
      macos) require_file "$DESKTOP_DIR/cfst-gui-darwin-amd64.app.zip" "macOS amd64 release asset missing"; require_file "$DESKTOP_DIR/cfst-gui-darwin-arm64.app.zip" "macOS arm64 release asset missing" ;;
    esac
  done
}
mkdir -p "$DESKTOP_DIR" "$ANDROID_RELEASE_DIR"

case "$TARGET" in
  all)
    rm -rf "$RELEASE_DIR" "$ROOT_DIR/Releases"
    mkdir -p "$DESKTOP_DIR" "$ANDROID_RELEASE_DIR"
    build_release_targets
    require_release_targets
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
