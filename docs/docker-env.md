# Docker 与环境变量

本文档集中说明 CFST-GUI 在 WebUI、Docker Compose、构建脚本、Android 签名和 GitHub Actions 中使用的环境变量。

## WebUI 运行时

| 变量 | 默认值 | 使用位置 | 说明 |
| --- | --- | --- | --- |
| `CFST_WEBUI_ADDR` | `0.0.0.0:34115` | `internal/app/webui.go` | WebUI HTTP Server 监听地址。 |
| `CFST_WEBUI_TOKEN` | 空 | `internal/app/webui.go` | WebUI 访问令牌；为空时不启用鉴权。 |
| `CFST_GUI_PORTABLE_ROOT` | 空 | `internal/app/storage.go` | 便携数据根目录；实际数据目录为 `${CFST_GUI_PORTABLE_ROOT}/data`。 |
| `CFST_WEBUI_ALLOWED_ROOTS` | 空 | `internal/app/webui.go` | WebUI 文件列表和下载允许访问的根目录，支持逗号或冒号分隔。 |
| `CFST_HTTP_PROTOCOL` | `auto` | `internal/httpclient/client.go` | 默认 HTTP 协议，可用 `auto`、`tcp`、`h1`、`h2`、`h3`。 |

WebUI 文件访问根目录默认包含 `/data` 和当前 `storageRoot()`。如果设置 `CFST_WEBUI_ALLOWED_ROOTS`，其路径会追加到允许列表，而不是替换默认值。

## Docker Compose

Linux WebUI 发行包内的 `docker-compose.yml` 默认使用：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CFST_WEBUI_PORT` | `34115` | 宿主机端口映射，映射到容器 `34115`。 |
| `CFST_WEBUI_TOKEN` | `change-me` | Compose 默认令牌，部署前必须修改。 |
| `CFST_VERSION` | `latest` 或脚本版本 | Compose 镜像标签变量；`.env.example` 中由构建脚本写入当前版本。 |

生成的 Compose 服务会在容器内设置：

```yaml
environment:
  CFST_WEBUI_ADDR: 0.0.0.0:34115
  CFST_WEBUI_TOKEN: ${CFST_WEBUI_TOKEN:-change-me}
  CFST_GUI_PORTABLE_ROOT: /data
  CFST_WEBUI_ALLOWED_ROOTS: /data
```

数据 volume 默认挂载到 `/data`。因为 `CFST_GUI_PORTABLE_ROOT=/data` 会让应用数据目录解析为 `/data/data`，所以备份 volume 时需要保留整个 `/data` 挂载内容。

## Linux 本地运行脚本

Linux bundle 内新增 `run-local.sh`，默认会设置：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CFST_GUI_PORTABLE_ROOT` | `<bundle>/portable` | 让本地运行时的数据跟随 bundle 落盘到 `portable/data`。 |
| `CFST_WEBUI_ADDR` | `127.0.0.1:34115` | 默认只监听本机回环地址；如需局域网访问可自行覆盖。 |

`run-local.sh` 不会默认注入 `CFST_WEBUI_TOKEN`。如果要把服务暴露到非本机地址，建议同时设置访问令牌。

## Release 构建

| 变量 | 默认值 | 使用位置 | 说明 |
| --- | --- | --- | --- |
| `CFST_VERSION` | `1.7.2` | `scripts/build-release.sh`、Android Gradle | 发行版本号；脚本会写入 Go `github.com/axuitomo/CFST-GUI/internal/app.version`。 |
| `GOMOBILE_BIN` | `$(go env GOPATH)/bin/gomobile` | Android 构建脚本 | gomobile 可执行文件路径。 |
| `ANDROID_HOME` | 自动推导 | Android 构建脚本 | Android SDK 目录。 |
| `ANDROID_SDK_ROOT` | 自动推导 | Android 构建脚本 | Android SDK 目录，优先级与 `ANDROID_HOME` 互相兼容。 |
| `ANDROID_NDK_HOME` | `<sdk>/ndk/26.3.11579264` | Android 构建脚本 | Android NDK 目录。 |
| `CFST_ANDROID_TOOLCHAIN_DIR` | `$XDG_CACHE_HOME/cfst-gui/android-toolchain` | `scripts/build-android-mobile.sh` | Debug 构建时自动推导 SDK/NDK 的工具链根目录。 |

更新下载代理优先级：桌面端与 Android 端在线更新会优先尝试 `ghproxy.com`，失败后回退到 `kkgithub.com`，最后再尝试原始 GitHub Release 下载地址。

`scripts/build-release.sh linux` 会一次生成 `amd64` 和 `arm64` 两种 Linux WebUI bundle；`linux-amd64` 与 `linux-arm64` 可按架构单独构建。脚本会用 `CFST_VERSION` 写入每个 bundle 的 `.env.example`，并生成 Docker context 与 `run-local.sh`。

## Android 签名

Release APK 签名只从环境变量读取，不把 keystore 或密码写入仓库：

| 变量 | 是否必需 | 说明 |
| --- | --- | --- |
| `CFST_ANDROID_KEYSTORE` | Release 必需 | release keystore 文件路径。 |
| `CFST_ANDROID_KEYSTORE_PASSWORD` | Release 必需 | keystore 密码。 |
| `CFST_ANDROID_KEY_ALIAS` | Release 必需 | key alias。 |
| `CFST_ANDROID_KEY_PASSWORD` | Release 必需 | key 密码。 |
| `CFST_ANDROID_VERSION_CODE` | 可选 | Android `versionCode`；默认 `10702`。 |
| `CFST_VERSION` | 可选 | Android `versionName`；默认 `1.7.2`，前缀 `v` 会被去掉。 |

本地 Release 构建示例：

```bash
export CFST_ANDROID_KEYSTORE=/absolute/path/release.jks
export CFST_ANDROID_KEYSTORE_PASSWORD=...
export CFST_ANDROID_KEY_ALIAS=...
export CFST_ANDROID_KEY_PASSWORD=...
export CFST_VERSION=1.7.2
./scripts/build-release.sh android
```

## GitHub Actions Secret

`.github/workflows/release.yml` 需要这些 Secrets 才能构建 Android Release：

| Secret | 说明 |
| --- | --- |
| `CFST_ANDROID_KEYSTORE_BASE64` | Base64 编码后的 release keystore。 |
| `CFST_ANDROID_KEYSTORE_PASSWORD` | keystore 密码。 |
| `CFST_ANDROID_KEY_ALIAS` | key alias。 |
| `CFST_ANDROID_KEY_PASSWORD` | key 密码。 |

工作流会把 `CFST_ANDROID_KEYSTORE_BASE64` 解码到 runner 临时目录，再通过 `CFST_ANDROID_KEYSTORE` 传给 Gradle。

## GHCR 镜像发布

`.github/workflows/container.yml` 使用 GitHub `GITHUB_TOKEN` 登录 GHCR，并发布：

```text
ghcr.io/axuitomo/cfst-gui:<version>
ghcr.io/axuitomo/cfst-gui:v<version>
```

该工作流只有手动触发入口，输入 `version` 默认 `1.7.2`。它会先分别运行 `scripts/build-release.sh linux-amd64` 与 `scripts/build-release.sh linux-arm64` 生成 Docker context，再用 Docker Buildx 合并发布单一多架构 tag，覆盖 `linux/amd64` 与 `linux/arm64`。
