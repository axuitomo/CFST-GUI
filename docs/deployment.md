# 部署与构建

本文档按场景整理 CFST-GUI 的开发、构建、部署、升级和发布流程。

## 环境要求

| 组件 | 当前要求 |
| --- | --- |
| Go | `go.mod` 固定 `go 1.26.2` |
| Wails | `github.com/wailsapp/wails/v2/cmd/wails@v2.12.0` |
| Node.js | GitHub Actions 使用 Node.js `22` |
| 前端 | Vue 3、Vite 6、Tailwind CSS 3，脚本在 `frontend/package.json` |
| Android | Capacitor 7、gomobile、Android SDK 35、NDK `26.3.11579264` |
| JDK | Android 构建要求 JDK 24 |

## 本地开发

安装 Wails CLI：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

安装前端依赖：

```bash
cd frontend
npm install
cd ..
```

启动桌面开发模式：

```bash
wails dev
```

常用检查命令：

```bash
cd frontend
npm run typecheck
npm run build
cd ..
bash -lc 'source scripts/lib/common.sh; go test $(cfst_go_packages)'
```

## 桌面构建

Windows、macOS 和 Linux WebUI 发行资产由统一脚本生成：

```bash
./scripts/build-release.sh windows
./scripts/build-release.sh darwin-amd64
./scripts/build-release.sh darwin-arm64
./scripts/build-release.sh linux
./scripts/build-release.sh linux-amd64
./scripts/build-release.sh linux-arm64
```

输出目录：

| 目标 | 产物 |
| --- | --- |
| Windows amd64 | `build/release/desktop/cfst-gui-windows-amd64.exe` |
| macOS amd64 | `build/release/desktop/cfst-gui-darwin-amd64.app.zip` |
| macOS arm64 | `build/release/desktop/cfst-gui-darwin-arm64.app.zip` |
| Linux WebUI amd64 | `build/release/desktop/cfst-gui-linux-amd64.tar.gz` |
| Linux WebUI arm64 | `build/release/desktop/cfst-gui-linux-arm64.tar.gz` |

Windows 产物改为经典 `exe` 安装包，统一通过 Wails `-nsis` 生成，需要 NSIS `makensis`、Windows SDK `SignTool.exe` 和签名证书。macOS 是原生 Wails 桌面 GUI，默认启动时会自适应最大化到当前屏幕可用区域，并可在设置页切换固定验收尺寸后恢复“自适应”。Linux 目标不是 Wails 桌面包，而是带 `webui` build tag 的 HTTP WebUI 服务 bundle；统一脚本里的 `linux` 目标会一次构建 `amd64` 和 `arm64` 两种 bundle，单独 target 则只生成指定架构。它随浏览器 viewport 响应式自适应，设置页仅允许刷新“自适应”状态，固定验收尺寸仅 Wails 桌面支持。macOS 产物应在对应 macOS runner 或主机上构建，并验证 darwin-amd64、darwin-arm64 两种架构。

## Linux WebUI

WebUI 服务由 `internal/app/webui.go` 提供，构建时需要 `webui` build tag。统一脚本会执行等价构建，并生成 Docker 上下文：

```bash
./scripts/build-release.sh linux
./scripts/build-release.sh linux-amd64
./scripts/build-release.sh linux-arm64
```

脚本会创建：

| 路径 | 说明 |
| --- | --- |
| `build/cfst-webui-linux-amd64/cfst-webui` | Linux amd64 WebUI 可执行文件 |
| `build/cfst-webui-linux-amd64/Dockerfile` | `scratch` 镜像构建文件 |
| `build/cfst-webui-linux-amd64/docker-compose.yml` | Compose 部署文件 |
| `build/cfst-webui-linux-amd64/docker-compose.host.yml` | host 网络模式 Compose override |
| `build/cfst-webui-linux-amd64/.env.example` | Compose 环境变量示例 |
| `build/cfst-webui-linux-amd64/run-local.sh` | Linux amd64 本地运行入口 |
| `build/cfst-webui-linux-arm64/cfst-webui` | Linux arm64 WebUI 可执行文件 |
| `build/cfst-webui-linux-arm64/Dockerfile` | `scratch` 镜像构建文件 |
| `build/cfst-webui-linux-arm64/docker-compose.yml` | Compose 部署文件 |
| `build/cfst-webui-linux-arm64/docker-compose.host.yml` | host 网络模式 Compose override |
| `build/cfst-webui-linux-arm64/.env.example` | Compose 环境变量示例 |
| `build/cfst-webui-linux-arm64/run-local.sh` | Linux arm64 本地运行入口 |
| `build/release/desktop/cfst-gui-linux-amd64.tar.gz` | 可分发压缩包 |
| `build/release/desktop/cfst-gui-linux-arm64.tar.gz` | 可分发压缩包 |

手动构建 WebUI 可执行文件时使用：

```bash
mkdir -p build/cfst-webui-linux-amd64 build/cfst-webui-linux-arm64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags webui -ldflags "-X github.com/axuitomo/CFST-GUI/internal/app.version=1.7.6" -o build/cfst-webui-linux-amd64/cfst-webui .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags webui -ldflags "-X github.com/axuitomo/CFST-GUI/internal/app.version=1.7.6" -o build/cfst-webui-linux-arm64/cfst-webui .
```

## Docker Compose 部署

解压 Linux WebUI 发行包后进入其中的 Compose 目录：

```bash
tar -xzf build/release/desktop/cfst-gui-linux-<arch>.tar.gz -C /opt/cfst-gui
cd /opt/cfst-gui/cfst-webui-linux-<arch>
cp .env.example .env
```

部署前修改 `.env` 中的 `CFST_WEBUI_TOKEN`，不要使用默认 `change-me` 暴露服务。

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

## Linux 本地运行

如果不使用 Docker，选择与主机架构匹配的发行包后直接运行 bundle 内脚本：

```bash
tar -xzf build/release/desktop/cfst-gui-linux-<arch>.tar.gz -C /opt/cfst-gui
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
| `source-profiles.json` | 输入源档案 |
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

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260410095206-2cfb76559b7b
gomobile init
```

执行 Debug 构建：

```bash
./scripts/build-android-mobile.sh
```

脚本流程：

1. 执行 `frontend` 生产构建。
2. 执行 `npx cap sync android` 同步 Web assets。
3. 执行 `gomobile bind -target=android/arm64,android/arm -ldflags '-linkmode external -extldflags "-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"'` 生成 `mobileapi.aar`。
4. 执行 `mobile/android/gradlew assembleDebug` 输出 Debug APK。

Debug APK 输出位置见 [Android Mobile Architecture](./android-mobile.md)。

## Android Release 构建

Release 构建通过统一脚本完成：

```bash
export CFST_ANDROID_KEYSTORE=/absolute/path/release.jks
export CFST_ANDROID_KEYSTORE_PASSWORD=...
export CFST_ANDROID_KEY_ALIAS=...
export CFST_ANDROID_KEY_PASSWORD=...
./scripts/build-release.sh android
```

最终产物：

```text
build/release/android/cfst-gui-android-release.apk
build/release/android/cfst-gui-android-arm64-v8a-release.apk
build/release/android/cfst-gui-android-armeabi-v7a-release.apk
```

`mobile/android/app/build.gradle` 从环境变量读取 `CFST_VERSION` 和 `CFST_ANDROID_VERSION_CODE`，默认值分别是 `1.7.6` 和 `10706`。新旧 APK 在线更新要求使用同一签名证书。

Android 原生库发布要求 `libgojni.so` 使用 16KB ELF 段对齐，同时保持对 4KB 设备的向后兼容。当前脚本通过 `gomobile bind` 的 linker flags 固化该行为；验收时至少检查一次：

```bash
bash scripts/check-android-page-alignment.sh \
  mobile/android/app/libs/mobileapi.aar \
  mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk
```

## GitHub Release

`.github/workflows/release.yml` 由 `v*` tag 或手动触发。流水线会分平台构建 Windows、Linux WebUI amd64、Linux WebUI arm64、macOS amd64、macOS arm64 和 Android 资产，然后集中生成 `cfst-gui-update-manifest.json` 并发布 GitHub Release。

Android Release 需要配置这些 GitHub Secrets：

| Secret | 说明 |
| --- | --- |
| `CFST_ANDROID_KEYSTORE_BASE64` | Base64 编码后的 release keystore |
| `CFST_ANDROID_KEYSTORE_PASSWORD` | keystore 密码 |
| `CFST_ANDROID_KEY_ALIAS` | key alias |
| `CFST_ANDROID_KEY_PASSWORD` | key 密码 |

## GHCR 镜像

`.github/workflows/container.yml` 支持手动发布 WebUI 镜像到 GHCR：

```text
ghcr.io/axuitomo/cfst-gui:<version>
ghcr.io/axuitomo/cfst-gui:v<version>
ghcr.io/axuitomo/cfst-gui:latest
```

该 workflow 会分别运行 `scripts/build-release.sh linux-amd64` 与 `scripts/build-release.sh linux-arm64` 生成 Docker context，再把两个 digest 合并为同一个多架构 GHCR tag，最终同时覆盖 `linux/amd64` 与 `linux/arm64`。版本 tag 用于可复现部署，`latest` 用于跟随最新发布。
