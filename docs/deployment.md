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
go test ./...
```

## 桌面构建

Windows、macOS 和 Linux WebUI 发行资产由统一脚本生成：

```bash
./scripts/build-release.sh windows
./scripts/build-release.sh darwin-amd64
./scripts/build-release.sh darwin-arm64
./scripts/build-release.sh linux
```

输出目录：

| 目标 | 产物 |
| --- | --- |
| Windows amd64 | `build/release/desktop/cfst-gui-windows-amd64.exe` |
| macOS amd64 | `build/release/desktop/cfst-gui-darwin-amd64.app.zip` |
| macOS arm64 | `build/release/desktop/cfst-gui-darwin-arm64.app.zip` |
| Linux WebUI | `build/release/desktop/cfst-gui-linux-amd64.tar.gz` |

Windows 和 macOS 是原生 Wails 桌面 GUI，默认启动时会自适应最大化到当前屏幕可用区域，并可在设置页切换固定验收尺寸后恢复“自适应”。Linux 目标不是 Wails 桌面包，而是带 `webui` build tag 的 HTTP WebUI 服务和 Docker Compose 包；它随浏览器 viewport 响应式自适应，设置页仅允许刷新“自适应”状态，固定验收尺寸仅 Wails 桌面支持。macOS 产物应在对应 macOS runner 或主机上构建，并验证 darwin-amd64、darwin-arm64 两种架构。

## Linux WebUI

WebUI 服务由 `webui.go` 提供，构建时需要 `webui` build tag。统一脚本会执行等价构建，并生成 Docker 上下文：

```bash
./scripts/build-release.sh linux
```

脚本会创建：

| 路径 | 说明 |
| --- | --- |
| `build/cfst-webui-linux-amd64/cfst-webui` | Linux amd64 WebUI 可执行文件 |
| `build/cfst-webui-linux-amd64/Dockerfile` | `scratch` 镜像构建文件 |
| `build/cfst-webui-linux-amd64/docker-compose.yml` | Compose 部署文件 |
| `build/cfst-webui-linux-amd64/.env.example` | Compose 环境变量示例 |
| `build/release/desktop/cfst-gui-linux-amd64.tar.gz` | 可分发压缩包 |

手动构建 WebUI 可执行文件时使用：

```bash
mkdir -p build/cfst-webui-linux-amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags webui -ldflags "-X main.version=1.4" -o build/cfst-webui-linux-amd64/cfst-webui .
```

## Docker Compose 部署

解压 Linux WebUI 发行包后进入其中的 Compose 目录：

```bash
tar -xzf build/release/desktop/cfst-gui-linux-amd64.tar.gz -C /opt/cfst-gui
cd /opt/cfst-gui/cfst-webui-linux-amd64
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

Compose 默认把 Docker volume `cfst-webui-data` 挂载到容器 `/data`，并设置 `CFST_GUI_PORTABLE_ROOT=/data`。由于程序会把便携根目录解析为 `${CFST_GUI_PORTABLE_ROOT}/data`，WebUI 的应用数据会落在容器内 `/data/data`，同时文件列表允许访问 `/data`。

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
| `profiles.json` | 探测配置档案 |
| `source-profiles.json` | 输入源档案 |
| `exports/` 和 `backups/` | CSV 导出和本地配置归档 |

回滚时恢复上一个发行包或镜像版本，保留同一个 volume，再执行 `docker compose up -d`。如果配置 schema 已被新版本写入，回滚前建议先导出配置归档。

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
3. 执行 `gomobile bind -target=android/arm64,android/arm` 生成 `mobileapi.aar`。
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
```

`mobile/android/app/build.gradle` 从环境变量读取 `CFST_VERSION` 和 `CFST_ANDROID_VERSION_CODE`。新旧 APK 在线更新要求使用同一签名证书。

## GitHub Release

`.github/workflows/release.yml` 由 `v*` tag 或手动触发。流水线会分平台构建 Windows、Linux WebUI、macOS amd64、macOS arm64 和 Android 资产，然后集中生成 `cfst-gui-update-manifest.json` 并发布 GitHub Release。

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
```

该 workflow 会先运行 `scripts/build-release.sh linux` 生成 Docker context，再用 Docker Buildx 推送 `linux/amd64` 镜像。
