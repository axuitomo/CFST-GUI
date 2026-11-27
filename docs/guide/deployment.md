# 部署与构建

本文档按场景整理 CFST-GUI 的开发、构建、部署、升级和发布流程。

## 环境要求

| 组件 | 当前基线（最低版本） |
| --- | --- |
| Go | `1.27.4`（`go.mod` 声明的语言与工具链基线） |
| 本地 Shell | Windows PowerShell 7 或更高版本；脚本兼容 PowerShell 5.1 的场景除外 |
| Wails | `v3.2.0-nightly.20261103`（`wails3` CLI 与 Go 模块版本必须匹配） |
| Node.js / pnpm | Node.js `26.12.1`、pnpm `12.16.1` |
| 前端 | Vue 3、Vite 8、Tailwind CSS 4、TypeScript 6，脚本在 `frontend/package.json` |
| Android | Capacitor `8.6.2`、Cordova Android `15.0.0`、gomobile、AGP `9.5.1`、Gradle `9.9.1`、AGP 9 内置 Kotlin（顶层 KGP classpath `2.4.21`）、Android SDK platform `android-37.0`、Build Tools `37.0.0`、cmdline-tools `20.0`、NDK `30.0.16248370` |
| JDK | Android 构建要求 JDK 25（当前验证环境为 `25.0.4.1`）；Gradle JVM 和 Android 子项目 compile options 都以 Java 25 bytecode 为发布基线 |

除非本表或对应构建文件明确要求精确版本，开发机和 CI 使用的环境版本不得低于上述基线；升级版本还必须满足仓库锁文件、插件和目标平台的兼容性要求。Wails 当前用 `3.2.0` 线的夜版（`v3.2.0-nightly.<日期>`）：夜版之间可能带 API 或 CLI 参数变化，必须按仓库钉住的这一个版本判定，不能凭语义版本号认为“更高的数字就更安全”，也不要跨日期混用 Go 模块、`wails3` CLI 和 `@wailsio/runtime` 三端版本。

版本基线的来源是仓库实际配置：`go.mod`、`frontend/package.json`、Android Gradle 配置、Gradle wrapper 和 GitHub Actions 工作流。修改这些基线时，必须同步更新构建脚本、doctor/preflight 检查、CI 配置和本节文档。

本机工具链基准检查：
`bash scripts/checks/doctor.sh --strict` 用于复现当前 CI 的固定基线，会对 Go、Node.js、pnpm 和 Wails 执行严格版本检查；它不是对“更高版本可用”的一般兼容性承诺。使用高于基线的版本时，应额外运行相关构建和测试，并处理由工具链升级带来的兼容性问题。

本地开发和常规验证统一使用 Windows PowerShell，并从真实 Windows 驱动器路径进入仓库。执行前端命令前先运行 `$PSVersionTable.PSVersion`、`node --version`、`pnpm --version` 和 `go version`；Windows 安装器构建、签名以及 WebView2/SignTool 检查也使用同一原生环境。仓库不要求安装 WSL；现有 `.sh` 发布脚本可从 PowerShell 通过 Git for Windows 的 `bash.exe` 调用。

## 本地开发

安装 Wails CLI：

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.2.0-nightly.20261103
```

安装前端依赖：

```powershell
pnpm --dir frontend install
```

前端样式由 Tailwind CSS 4 通过 `@tailwindcss/vite` 接入 Vite 8；`frontend/src/styles.css` 使用 `@import "tailwindcss"` 和 `@config "../tailwind.config.cjs"`，`frontend/postcss.config.cjs` 仅保留 Autoprefixer。`pnpm build` 会刷新 `frontend/dist` 中的 hashed JS/CSS 资产，随后由桌面、WebUI 和 Android 构建复用。`frontend/dist` 是构建产物、不入库（仅保留 `.gitkeep` 占位以保证 `//go:embed` 可编译），因此任何桌面/WebUI/Android 出货构建都必须先跑前端构建；漏构建会在启动时直接报错提示先 `pnpm --dir frontend build`，不会回退到旧界面。静态资源对 `index.html` 下发 `no-cache`、对带 hash 的 `assets/*` 下发 `immutable`，避免覆盖安装后读到旧缓存；WebUI 还会对文本类资源（`.js`/`.css`/`.html`/`.svg`/`.json` 等）做 gzip，主 bundle 从约 789 KB 压到约 216 KB，手机与局域网访问走的正是这条路径。Range 请求与非文本资源不压缩，桌面端的资源由 Wails 自己的资源服务提供、不受影响。

启动桌面开发模式：

```powershell
wails3 dev
```

该命令启动 Vite 开发服务器并打开 Wails 原生桌面窗口，前端改动经 `FRONTEND_DEVSERVER_URL`
热更新；Vite 端口跟随 `wails3 dev` 导出的 `WAILS_VITE_PORT`，单独运行
`pnpm --dir frontend dev` 时回退到 34117。默认配置路径为 `build/config.yml`，与规范配置
`build/config/wails.yml` 内容一致，改任一份请同步另一份。需要调试浏览器形态的 WebUI 时单独执行
`bash scripts/dev/open-dev.sh webui`，它使用 `-tags webui` 构建并监听 `127.0.0.1:34115`。

`wails3 dev` 的第一步是 `node scripts/dev/free-dev-port.mjs`。开发会话被强制中断（关终端、
IDE 停止任务）时，`background` 类型的 Vite 进程可能变成孤儿并继续监听 `WAILS_VITE_PORT`，
而 `wails3 dev` 启动前会先探测该端口，被占用就直接报错退出，表现为“没反应、窗口还是旧的”。
该脚本只结束监听目标端口且命令行含 `vite` 的 node 进程，其余占用只打印提示，不会误杀。

桌面窗口基于 WebView2。Wails 默认把 WebView2 用户数据目录设为 `%APPDATA%\<exe 名>`，进程
环境缺少 `APPDATA` 时（部分启动器、计划任务、CI 会剥掉该变量）拼接结果会退化成 exe 自身
路径，WebView2 弹出“无法创建数据目录”且窗口无法创建。程序因此显式指定本机缓存目录
`%LOCALAPPDATA%\CFST-GUI\webview2`：域账号或开启配置漫游的部署不会在登录/注销时同步一份
浏览器缓存；`os.UserCacheDir()` 不可用时退回应用数据目录下的同名子目录。

常用检查命令：

```powershell
& .\scripts\checks\check.ps1
& .\scripts\checks\lint.ps1
& .\scripts\checks\ci-local.ps1
pnpm test
pnpm lint
pnpm typecheck
pnpm build
$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' })
go test $goPackages
```

`check.ps1` 会运行 Go 测试、Vitest、前端类型检查和生产构建；`ci-local.ps1` 进一步执行格式检查、lint、Wails 生成物一致性和依赖审计。没有安装 Wails 时，`check.ps1` 可复用已有 `frontend/bindings`；生成物一致性检查仍要求 Wails CLI 和 Git 元数据。

## 桌面构建

Windows、macOS 和 Linux WebUI 构建产物可由统一脚本生成：

```powershell
bash scripts/build/build-release.sh windows
bash scripts/build/build-release.sh darwin-amd64
bash scripts/build/build-release.sh darwin-arm64
bash scripts/build/build-release.sh linux
bash scripts/build/build-release.sh linux-amd64
bash scripts/build/build-release.sh linux-arm64
```

输出目录：

| 目标 | 产物 |
| --- | --- |
| Windows amd64 安装器 | `build/artifacts/release/desktop/cfst-gui-windows-amd64.exe` |
| Windows amd64 便携版 | `build/artifacts/release/desktop/cfst-gui-windows-amd64-portable.exe` |
| macOS amd64 | `build/artifacts/release/desktop/cfst-gui-darwin-amd64.app.zip` |
| macOS arm64 | `build/artifacts/release/desktop/cfst-gui-darwin-arm64.app.zip` |
| Linux WebUI amd64 | `build/artifacts/release/desktop/cfst-gui-linux-amd64.tar.gz` |
| Linux WebUI arm64 | `build/artifacts/release/desktop/cfst-gui-linux-arm64.tar.gz` |

Windows amd64 安装器由 `scripts/build/build-release.sh` 生成：先编译并签名桌面主程序 `build/bin/cfst-gui.exe`，同时另存为便携版 `cfst-gui-windows-amd64-portable.exe`；再把已签名的主程序作为载荷放进 `cmd/installer/payload/`，用 `wails3 generate syso` 为安装器生成带图标、`requireAdministrator` 清单与 PE 版本信息的版本资源，最后用 `go build -ldflags "-X main.version=$VERSION"` 编译 `./cmd/installer` 并签名，输出 `build/artifacts/release/desktop/cfst-gui-windows-amd64.exe`；安装器未注入版本时默认 `2.2.0`。桌面程序以 `-H windowsgui` 链接为 GUI 子系统，双击不弹出控制台；构建前脚本会用 `wails3 generate syso` 按当前 `CFST_VERSION` 现场渲染版本信息生成 `.syso`，让桌面程序在文件属性中带 `ProductVersion`。本地与 CI 都走这条路径，需要 Wails v3 CLI、Go 工具链、Windows SDK `SignTool.exe` 和代码签名证书（缺省时按自签设置生成临时证书），不再需要 NSIS。

安装器本身是一个 Wails v3 应用：560×380 无边框窗口，界面是内嵌 HTML/CSS/JS（`cmd/installer/ui/` 的 `index.html` + `installer.css` + `installer.js`，深色单色阶 + 1px 网格 + 等宽字体），界面与安装日志全部为简体中文，通过同源 HTTP 与 SSE 与安装引擎通信。安装器底部提供“查看许可证”，安装前即可读取内嵌的 `LICENSE` 与 `THIRD-PARTY-NOTICES.md`；安装时同一份文本会写入安装目录。安装或卸载前会检查目标目录里的主程序是否正在运行，界面先询问用户是否关闭应用并继续，静默安装则直接提示先退出应用。因为界面依赖 WebView2，运行时检测放在创建窗口之前用原生 MessageBox 完成，缺失时打开微软下载页并以状态码 66 退出。静默安装（`/S`、`--silent`）走完全无窗口路径，支持 `/D=` 指定目录、`--no-desktop`、`--no-launch`。卸载器是安装器自身的副本，装在 `$INSTDIR\uninstall.exe`，带 `--uninstall` 启动即执行卸载（`--uninstall --silent` 供 `QuietUninstallString` 使用），因此卸载界面与安装器完全一致。
macOS 是原生 Wails 桌面 GUI，默认启动时会自适应最大化到当前屏幕可用区域，并可在设置页切换固定验收尺寸后恢复“自适应”。Linux 目标不是 Wails 桌面包，而是带 `webui` build tag 的 HTTP WebUI 服务 bundle；统一脚本里的 `linux` 目标会一次构建 `amd64` 和 `arm64` 两种 bundle，单独 target 则只生成指定架构。它随浏览器 viewport 响应式自适应，设置页仅允许刷新“自适应”状态，固定验收尺寸仅 Wails 桌面支持。macOS 产物应在对应 macOS runner 或主机上构建，并验证 darwin-amd64、darwin-arm64 两种架构。
本地执行 `windows` 目标时，脚本从 PATH 与 Windows SDK 标准目录查找 `SignTool.exe`；如需显式指定，可设置 `CFST_WINDOWS_SIGNING_TOOL`。签名材料由 `CFST_WINDOWS_SIGNING_CERT`（PFX 路径）提供，或改用 `CFST_WINDOWS_SIGNING_CERT_SUBJECT` / `CFST_WINDOWS_SIGNING_CERT_THUMBPRINT` 配合 `CFST_WINDOWS_SIGNING_PASSWORD` 让脚本从本机证书存储导出到缓存目录。未提供任何证书时，默认的 `CFST_WINDOWS_ALLOW_SELF_SIGNED=1` 会让脚本生成临时自签代码签名证书，导出 PFX 后立即从证书存储移除；设置 `CFST_WINDOWS_ALLOW_SELF_SIGNED=0` 可恢复「缺少可用证书即失败」的严格行为。`SignTool` 的警告（例如自签链或时间戳链不受信）只写入日志，不终止构建；真正的签名错误仍然失败。

Windows 验收默认允许自签名与私有 PKI 证书：`scripts/checks/windows-signature.ps1` 校验文件哈希与 Authenticode 签名匹配、安装器与便携版及安装后文件使用同一签名身份、且带时间戳；证书链只允许在 `UntrustedRoot` / `PartialChain` 处终止。需要强制系统受信链时设置 `CFST_WINDOWS_ALLOW_SELF_SIGNED=0`。CI 未配置 `CFST_WINDOWS_SIGNING_CERT_BASE64` 时，会按 `windows_self_signed` 工作流输入（默认允许）在 runner 上生成临时自签证书；自签产物不会被 Windows SmartScreen 信任，只适合内部或预览发布。证书更新应通过 GitHub Secrets 安全替换 `CFST_WINDOWS_SIGNING_CERT_BASE64` 与 `CFST_WINDOWS_SIGNING_PASSWORD`，不要将私钥、PFX 或密码提交到仓库或聊天中。

安装器以管理员权限装到 `$PROGRAMFILES64\axuitomo\CFST-GUI`，同时写入 `LICENSE` 与 `THIRD-PARTY-NOTICES.md`，并创建开始菜单与桌面快捷方式（`--no-desktop` 可只保留开始菜单项）；卸载信息写入 `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\axuitomoCFST-GUI`。无法使用管理员权限或希望随身携带时改用便携版 `cfst-gui-windows-amd64-portable.exe`，直接双击运行；安装版与便携版共用 `%AppData%\CFST-GUI` 下的应用数据目录。CLI 模式与 CFST 兼容参数已移除；安装版与便携版收到旧命令行参数会以状态码 2 退出；把输入文件拖到 exe 上会作为本地输入源加入列表，保存配置后生效。

安装目录必须是非根目录的绝对路径；首次安装拒绝覆盖无关的非空目录。程序和卸载器先完整写入临时文件再替换，安装失败或提交前取消会恢复原文件。卸载只允许从注册安装位置的 `uninstall.exe` 启动，只删除应用自有文件，保留目录中的用户文件；程序仍被占用时保留系统卸载入口供重试。开始菜单与可选桌面快捷方式统一使用 Windows 公共已知文件夹。

需要单独分发 macOS 构建时，可使用 Developer ID Application 身份启用 hardened runtime 签名，再通过 Apple `notarytool` 公证并把票据 stapling 到 `.app`。`CFST_REQUIRE_MACOS_SIGNING=1` 时，`scripts/build/build-release.sh` 会要求 `CFST_MACOS_SIGNING_IDENTITY`、`CFST_APPLE_ID`、`CFST_APPLE_APP_PASSWORD` 和 `CFST_APPLE_TEAM_ID` 全部存在；签名、公证、stapling 或最终 `codesign --verify` 任一步失败都会终止构建。GitHub Release 不发布 macOS 或 iOS 资产。

## Linux WebUI

WebUI 服务由 `internal/app/webui.go` 提供，构建时需要 `webui` build tag。统一脚本会执行等价构建，并生成 Docker 上下文：

```bash
bash scripts/build/build-release.sh linux
bash scripts/build/build-release.sh linux-amd64
bash scripts/build/build-release.sh linux-arm64
```

脚本会创建：

| 路径 | 说明 |
| --- | --- |
| `build/artifacts/webui-linux-amd64/cfst-webui` | Linux amd64 WebUI 可执行文件 |
| `build/artifacts/webui-linux-amd64/Dockerfile` | `scratch` 镜像构建文件 |
| `build/artifacts/webui-linux-amd64/docker-compose.yml` | Compose 部署文件 |
| `build/artifacts/webui-linux-amd64/docker-compose.host.yml` | host 网络模式 Compose override |
| `build/artifacts/webui-linux-amd64/.env.example` | Compose 环境变量示例 |
| `build/artifacts/webui-linux-amd64/run-local.sh` | Linux amd64 本地运行入口 |
| `build/artifacts/webui-linux-arm64/cfst-webui` | Linux arm64 WebUI 可执行文件 |
| `build/artifacts/webui-linux-arm64/Dockerfile` | `scratch` 镜像构建文件 |
| `build/artifacts/webui-linux-arm64/docker-compose.yml` | Compose 部署文件 |
| `build/artifacts/webui-linux-arm64/docker-compose.host.yml` | host 网络模式 Compose override |
| `build/artifacts/webui-linux-arm64/.env.example` | Compose 环境变量示例 |
| `build/artifacts/webui-linux-arm64/run-local.sh` | Linux arm64 本地运行入口 |
| `build/artifacts/release/desktop/cfst-gui-linux-amd64.tar.gz` | 可分发压缩包 |
| `build/artifacts/release/desktop/cfst-gui-linux-arm64.tar.gz` | 可分发压缩包 |

每个 Linux 发行压缩包还附带根目录 `LICENSE`、`THIRD-PARTY-NOTICES.md`、`SOURCE.txt`（构建提交源码与构建说明链接）和 `CA-CERTIFICATES-COPYRIGHT`（随包 CA 证书的版权文本）；scratch 镜像也复制这些文件。

手动构建 WebUI 可执行文件时使用：

```bash
mkdir -p build/artifacts/webui-linux-amd64 build/artifacts/webui-linux-arm64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags webui -ldflags "-X github.com/axuitomo/CFST-GUI/internal/app.version=2.2.0" -o build/artifacts/webui-linux-amd64/cfst-webui .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags webui -ldflags "-X github.com/axuitomo/CFST-GUI/internal/app.version=2.2.0" -o build/artifacts/webui-linux-arm64/cfst-webui .
```

## Docker Compose 部署

解压 Linux WebUI 发行包后进入其中的 Compose 目录：

```bash
tar -xzf build/artifacts/release/desktop/cfst-gui-linux-<arch>.tar.gz -C /opt/cfst-gui
cd /opt/cfst-gui/cfst-webui-linux-<arch>
cp .env.example .env
```

部署前必须生成随机令牌并写入 `.env` 的 `CFST_WEBUI_TOKEN`，例如 `openssl rand -hex 24`。发行包不再提供默认令牌；该变量为空时 `docker compose` 会直接报错拒绝启动，避免用公开示例值暴露服务。

```bash
docker compose up -d --build
```

默认访问地址：

```text
http://localhost:34115
```

Compose 默认把 Docker volume `cfst-webui-data` 挂载到容器 `/data`，并设置 `CFST_GUI_PORTABLE_ROOT=/data`。由于程序会把便携根目录解析为 `${CFST_GUI_PORTABLE_ROOT}/data`，WebUI 的应用数据会落在容器内 `/data/data`，同时文件列表允许访问 `/data`。定时任务、Cloudflare DNS 自动推送、GitHub 自动导出和上传筛选策略都通过 WebUI 保存到该目录；Docker 命令只管理服务生命周期、端口、时区和数据卷。

常用管理命令：

```bash
docker compose ps
docker compose logs -f
docker compose restart
docker compose down
```

默认网络模式为 bridge，通过 `.env` 中的 `CFST_WEBUI_PORT` 发布端口。如果需要使用宿主机网络，可叠加发行包内的 override：

```bash
docker compose -f docker-compose.yml -f docker-compose.host.yml up -d --build
```

容器默认设置 `TZ=Asia/Shanghai`，可在 `.env` 覆盖。WebUI 二进制内置 IANA 时区数据，`scratch` 镜像中也能按该时区计算每日定时任务。Compose 和镜像都会使用 `/app/cfst-webui --healthcheck` 请求 `/api/health`，因此不需要在镜像里额外放置 `curl` 或 `wget`。

如果要直接使用 GHCR 镜像：

```bash
TOKEN="$(openssl rand -hex 24)"; echo "CFST_WEBUI_TOKEN=$TOKEN"
docker run -d \
  --name cfst-webui \
  --restart unless-stopped \
  -p 34115:34115 \
  -e TZ=Asia/Shanghai \
  -e CFST_WEBUI_ADDR=0.0.0.0:34115 \
  -e CFST_WEBUI_TOKEN="$TOKEN" \
  -e CFST_GUI_PORTABLE_ROOT=/data \
  -e CFST_WEBUI_ALLOWED_ROOTS=/data \
  -v cfst-webui-data:/data \
  ghcr.io/axuitomo/cfst-gui:latest
```

## Linux 本地运行

如果不使用 Docker，选择与主机架构匹配的发行包后直接运行 bundle 内脚本：

```bash
tar -xzf build/artifacts/release/desktop/cfst-gui-linux-<arch>.tar.gz -C /opt/cfst-gui
cd /opt/cfst-gui/cfst-webui-linux-<arch>
./run-local.sh
```

默认访问地址：

```text
http://127.0.0.1:34115
```

`run-local.sh` 默认会把 `CFST_GUI_PORTABLE_ROOT` 设为当前 bundle 下的 `portable/`，因此配置、导出和备份会落在 `portable/data`。如需对局域网开放，可显式设置 `CFST_WEBUI_ADDR=0.0.0.0:34115`，并配合 `CFST_WEBUI_TOKEN` 启用鉴权。

## 升级、备份与回滚

升级 WebUI 时保留 `cfst-webui-data` volume，只替换发行包或镜像，然后重新执行：

```bash
docker compose up -d --build
```

备份建议同时保留：

| 内容 | 说明 |
| --- | --- |
| Docker volume `cfst-webui-data` | WebUI 数据、配置、导出和备份文件 |
| `desktop-config.json` | 当前 GUI/WebUI 主要配置快照 |
| `source-profiles.json` | 输入组 |
| `exports/` 和 `backups/` | CSV 导出和本地配置归档 |

本地运行部署则保留 bundle 内的 `portable/` 目录，只替换程序文件并重新执行 `./run-local.sh`。

Docker volume 备份示例：

```bash
docker run --rm \
  -v cfst-webui-data:/data:ro \
  -v "$PWD:/backup" \
  busybox tar -czf /backup/cfst-webui-data.tar.gz -C /data .
```

Docker volume 恢复示例：

```bash
docker run --rm \
  -v cfst-webui-data:/data \
  -v "$PWD:/backup" \
  busybox sh -c 'cd /data && tar -xzf /backup/cfst-webui-data.tar.gz'
```

回滚时恢复上一个发行包或镜像版本，保留同一个 volume（或本地 `portable/` 数据目录），再执行 `docker compose up -d` 或 `./run-local.sh`。如果配置 schema 已被新版本写入，回滚前建议先导出配置归档。

## Android Debug 构建

安装 gomobile：

```powershell
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20261011034528-197770444867
gomobile init
```

执行 Debug 构建：

```powershell
bash scripts/build/build-android-mobile.sh
```

脚本流程：

1. 执行 `frontend` 生产构建。
2. Vite 构建会自动将根目录的 `LICENSE` 和 `THIRD-PARTY-NOTICES.md` 写入前端产物，然后执行 `pnpm exec cap sync android` 同步 Web assets 和 Capacitor 生成文件。
3. 执行 `gomobile bind -target=android/arm64,android/arm -ldflags '-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'` 生成 `mobileapi.aar`。
4. 执行 `mobile/android/gradlew assembleDebug` 输出 Debug APK。

Debug APK 输出位置见 [Android Mobile Architecture](../mobile/android-mobile.md)。

## Android Release 构建

Release 构建通过统一脚本完成：

```powershell
$env:CFST_ANDROID_KEYSTORE = 'C:\path\to\release.jks'
$env:CFST_ANDROID_KEYSTORE_PASSWORD = '...'
$env:CFST_ANDROID_KEY_ALIAS = '...'
$env:CFST_ANDROID_KEY_PASSWORD = '...'
bash scripts/build/build-release.sh android
```

最终产物：

```text
build/artifacts/release/android/cfst-gui-android-arm64-v8a-release.apk
build/artifacts/release/android/cfst-gui-android-arm64-v8a-debug.apk
```

`mobile/android/app/build.gradle` 从环境变量读取 `CFST_VERSION` 和 `CFST_ANDROID_VERSION_CODE`，默认值分别是 `2.2.0` 和 `20200`。新旧 APK 在线更新要求使用同一签名证书。Release 和 Debug APK 均包含 `LICENSE` 与 `THIRD-PARTY-NOTICES.md` 资产。

Release 构建在复制发布资产前运行 `scripts/checks/verify-android-release.mjs`：使用 `apksigner` 验证签名完整性，将 APK 签名证书 SHA-256 与配置的 Release keystore 公钥证书比较，拒绝 Debug 证书、可调试清单及错误包名，并逐字比较 APK 中的两份法律文件与根目录原文。密码通过环境变量提供给 `keytool`，不写入命令行参数。

Android 新配置默认开启探测调试日志，已有配置中明确关闭日志的设置会保留。主 Release 和 Android Release Resubmit 工作流将 Debug APK 单独上传到 `android-debug` CI artifact；GitHub Release 和自动更新使用 Release APK。Debug APK 可附加调试，但使用 debug 签名，不能覆盖正式签名的同包名应用。下载及安装说明见 [Android 构建产物](../mobile/android-mobile.md#outputs)。

Android 发布基线固定为 Capacitor `8.6.2`、Cordova Android `15.0.0`、AGP `9.5.1`、Gradle `9.9.1`、AGP 9 内置 Kotlin（顶层 KGP classpath 固定 `2.4.21`）、SDK platform `android-37.0`、Build Tools `37.0.0`、cmdline-tools `20.0` 和 NDK `30.0.16248370`。`mobile/android/build.gradle` 会强制校验当前 Gradle JVM 是 JDK 25，并通过顶层 `subprojects` 配置把 Android 子项目 compile options 统一覆盖为 Java 25 bytecode；`app/build.gradle` 不再显式应用 `org.jetbrains.kotlin.android`。`app/capacitor.build.gradle` 等带有 “DO NOT EDIT” 注释的文件由 `pnpm exec cap sync android` 生成，如果模板默认值写 Java 21，不手工编辑生成文件，以顶层覆盖保持一致。

AndroidX 依赖按最新稳定更新；`androidx.core` 升到 `1.19.0`，因此 compile SDK 同步升到 `android-37.0`。本地 Android SDK 需要安装 `cmdline-tools;latest`、`platforms;android-37.0`、`build-tools;37.0.0` 和 `ndk;30.0.16248370` 后再运行 Android 构建。

Android 原生代码位于 `mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/`，当前以 Kotlin 为唯一主语言；`CfstPlugin.kt` 保持 Capacitor plugin 入口，具体能力拆分到 `Android*` Kotlin 文件并由 `src/test` 下的 Kotlin 单元测试覆盖。

Android Activity 保持 edge-to-edge WebView 布局，但不隐藏状态栏或导航栏；状态栏、导航栏和刘海屏/打孔屏等异形屏由 `viewport-fit=cover`、safe-area padding、`adjustResize` 和 Android P+ display cutout 短边布局共同适配。前端 Android viewport 状态只在键盘 inset 或键盘开闭状态真实变化时写入 DOM，不再驱动 app 根容器高度，也不在输入框聚焦时强制居中滚动，避免安装确认页返回后点击输入框触发重复重排抖动。

Android 原生层关闭 theme force dark、WebView `FORCE_DARK_OFF` 和 Android 13+ algorithmic darkening，避免 WebView 或系统深色策略把按钮背景自动变淡、把按钮文字改成低对比颜色。

Android 原生库发布要求 `libgojni.so` 使用 16KB ELF 段对齐，同时保持对 4KB 设备的向后兼容。当前脚本通过 `gomobile bind` 的 linker flags 固化该行为；验收时至少检查一次所有 split APK 的 alignment、最终 manifest 和敏感组件导出状态：

```powershell
bash scripts/checks/check-android.sh `
  mobile/android/app/libs/mobileapi.aar `
  mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

连接真机或 AVD 后，再运行设备 smoke 检查安装后的系统可见状态：

```powershell
bash scripts/checks/android-doctor.sh --device-smoke `
  --device-smoke-apk mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk
```

设备 smoke 会验证设备侧 package、权限、FileProvider、更新清理 receiver、WorkManager 组件和 launcher 启动。SAF 授权、导入导出、通知权限弹窗、前台任务、定时任务触发、在线更新下载、APK 安装确认、安装确认页返回后的输入框聚焦稳定性，以及状态栏在刘海屏/异形屏设备上保持可见仍需人工手测。

## 许可证与第三方声明

项目采用 `GPL-3.0-only`，项目版权声明为 `Copyright (C) 2026 axuitomo`。上游衍生代码的作者信息另见根目录 `THIRD-PARTY-NOTICES.md`；该声明不替代第三方各自的许可证。

根目录 `LICENSE` 只放 [GNU GPL v3 原文](https://www.gnu.org/licenses/gpl-3.0.txt) 的逐字副本（35,149 字节，LF 行尾），不写项目名、版权行、SPDX 标识或注释，不追加版权附加文字与第三方声明，也不合并其它协议。GitHub 等平台的许可证识别按整段文本比对，以上任何附加内容都会让结果变成“无法识别”。项目版权与 SPDX 标识放在 `README.md` 和源码文件头，第三方归属与许可原文放在 `THIRD-PARTY-NOTICES.md`。`pnpm check:license` 以整文件哈希守住这条约束，quality 工作流会执行。

普通 `pnpm build`、直接 Vite 生产构建及发布脚本都会将根法律文件放入 `frontend/dist/`，经 Go embed 进入桌面与 WebUI 二进制，经 Capacitor sync 进入 APK 的 `assets/public/`。安装版另将两份文件写入安装目录，macOS 包放在 `Contents/Resources/`，便携版同时提供 Release 法律附件。

第三方清单收集前端生产 npm 包的所有已安装版本及包内法律文件、随产物分发的 Tailwind CSS、Capacitor Android、按发行目标解析的 Go 依赖、Go 标准库及其 vendor 许可、源码注释中的独立授权，以及 Android Release Maven 依赖和 AAR/JAR（含嵌套 JAR）的法律文本。修改依赖或工具链后，在仓库根目录执行：

```powershell
Push-Location mobile/android
./gradlew.bat -I ../../scripts/build/android-license-report.init.gradle --no-configuration-cache :app:generateLicenseReport
Pop-Location
node scripts/build/check-android-license-report.mjs --update
node scripts/build/generate-third-party-notices.mjs --write
pnpm build
node scripts/build/check-canonical-license.mjs
node --test scripts/build/inline-license-notices.test.mjs scripts/build/legal-assets.test.mjs
```

发布脚本以 `--check` 重新生成并比较清单，Android 构建脚本另校验实时 Gradle 报告，清单过期即失败。Go 扫描器固定为 `github.com/google/go-licenses/v2@v2.0.1`；生成和检查需要相应 Go、npm、Gradle 依赖可用。系统提供、未随应用分发的库不纳入清单；Linux 额外复制的 CA 证书使用构建系统的版权文件，缺失时中止打包。

## GitHub Release

`.github/workflows/release.yml` 由 `v*` tag、推送 `test` 分支或手动操作触发。`test` 分支会生成 `2.2.0-preview.<run_number>` 形式的唯一版本；任何包含 `-` 后缀的版本均发布为 GitHub Pre-release，纯数字版本发布为正式 Release。Release 正文会附实际构建提交的 Corresponding Source 源码归档、构建说明和当前 tag 的源码链接；资产包含 Windows、Android 安装包、`LICENSE`、`THIRD-PARTY-NOTICES.md` 和 `cfst-gui-update-manifest.json`，不包含 Linux、Docker、macOS 或 iOS。

发布前的质量门禁与 `Quality` 工作流共用 `.github/actions/quality-gate`（同一套工具链版本、同一套检查范围，都基于 `scripts/checks/ci-local.sh`），所以 tag 推送不会重新引入「只在发布时才跑」的检查；要调整门禁只改这个 composite action。`desktop` 与 `android` 任务都 `needs: quality`，门禁红即整次发布中止。

手动触发时设置布尔输入 `verify_only=true`，会复用正式签名 Secrets 构建 Windows 与 Android，上传验收产物，但跳过 GitHub Release 和 GHCR 发布。例如在包含待验收修改的分支上执行：

```powershell
gh workflow run release.yml --ref signing-verification -f version=2.2.0 -F verify_only=true
```

Windows 签名默认允许自签证书：未配置 `CFST_WINDOWS_SIGNING_CERT_BASE64` 时，CI 会生成临时自签证书并在结束后清理。设置 `windows_self_signed=false` 可恢复「必须使用 Secret 中的受信任证书，否则失败」的严格行为。

Windows 验收在临时 GitHub-hosted 管理员 runner 中执行 `scripts/checks/windows-signed-install.ps1`，要求安装器、便携版及安装后的主程序和卸载器具有有效且同一身份的时间戳签名（默认允许自签证书，`CFST_WINDOWS_ALLOW_SELF_SIGNED=0` 时要求受信链）；实际静默安装到含空格的独立目录，核对法律文本、注册表、快捷方式和覆盖安装，随后卸载并确认用户文件保留。脚本默认拒绝本地机器，需要在可丢弃且没有现有安装的环境复现时显式传入 `-AllowLocal`（仍需管理员权限）。Android 同时完成上述 Release 签名、身份和法律资产检查。任何验收失败都会阻止后续正式发布。

Android Release 需要配置这些 GitHub Secrets：

| Secret | 说明 |
| --- | --- |
| `CFST_ANDROID_KEYSTORE_BASE64` | Base64 编码后的 release keystore |
| `CFST_ANDROID_KEYSTORE_PASSWORD` | keystore 密码 |
| `CFST_ANDROID_KEY_ALIAS` | key alias |
| `CFST_ANDROID_KEY_PASSWORD` | key 密码 |

Quality workflow 还会在 Ubuntu 主门禁之外运行 Windows 原生前端/Go 检查、Android AAR 与 Debug APK 构建验收，以及 Chromium WebUI 本地冒烟测试。平台专属构建失败会直接阻塞合并。

## GHCR 镜像

`.github/workflows/container.yml` 会由主 Release workflow 在 GitHub Release 成功后调用，也支持需要时手动发布 WebUI 镜像到 GHCR：

```text
ghcr.io/axuitomo/cfst-gui:<version>
ghcr.io/axuitomo/cfst-gui:v<version>
ghcr.io/axuitomo/cfst-gui:latest
```

该 workflow 会分别运行 `scripts/build/build-release.sh linux-amd64` 与 `scripts/build/build-release.sh linux-arm64` 生成 Docker context，再把两个 digest 合并为同一个多架构 GHCR tag，最终同时覆盖 `linux/amd64` 与 `linux/arm64`。版本 tag 用于可复现部署；预览版本不会更新 `latest`，只有正式版本会更新 `latest`。主 Release 的 GitHub 资产不包含 Docker 镜像。
