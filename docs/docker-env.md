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
| `CFST_RUNTIME_DIAGNOSTICS` | 空 | `internal/runtimecleanup` | 运行时诊断开关；设为 `1`/`true` 后可读取内存、goroutine 和最近清理摘要。 |
| `CFST_RUNTIME_DIAGNOSTICS_REMOTE` | 空 | `internal/runtimecleanup` | 远程运行时诊断开关；仅在同时配置 `CFST_WEBUI_TOKEN` 时允许非本机回环请求读取诊断。 |

WebUI 文件访问根目录默认包含 `/data` 和当前 `storageRoot()`。如果设置 `CFST_WEBUI_ALLOWED_ROOTS`，其路径会追加到允许列表，而不是替换默认值。

桌面端、Linux WebUI、Docker Compose 和 Android mobileapi 都会启动同一套运行时清理器。默认每 8 小时执行一次周期清理，并按 8 小时限频执行 Go 重回收；任务运行中只做轻量清理，避免影响测速。任务完成后的 30 秒延迟清理只释放 idle 连接、过期缓存和内存快照引用，不删除任务结果 JSON 或 CSV 文件，因此“当前结果”仍可从持久化结果或 CSV 回填。

## Docker Compose

Linux WebUI 发行包内的 `docker-compose.yml` 默认使用：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CFST_WEBUI_PORT` | `34115` | 宿主机端口映射，映射到容器 `34115`。 |
| `CFST_WEBUI_TOKEN` | `change-me` | Compose 默认令牌，部署前必须修改。 |
| `CFST_VERSION` | `latest` 或脚本版本 | Compose 镜像标签变量；`.env.example` 中由构建脚本写入当前版本。 |
| `CFST_DATA_VOLUME` | `cfst-webui-data` | Docker named volume 的实际名称，用于迁移、备份或多实例隔离。 |
| `TZ` | `Asia/Shanghai` | 容器时区；影响 WebUI 内每日定时任务的本地时间计算。 |

生成的 Compose 服务会在容器内设置：

```yaml
environment:
  TZ: ${TZ:-Asia/Shanghai}
  CFST_WEBUI_ADDR: 0.0.0.0:34115
  CFST_WEBUI_TOKEN: ${CFST_WEBUI_TOKEN:-change-me}
  CFST_GUI_PORTABLE_ROOT: /data
  CFST_WEBUI_ALLOWED_ROOTS: /data
```

数据 volume 默认挂载到 `/data`。因为 `CFST_GUI_PORTABLE_ROOT=/data` 会让应用数据目录解析为 `/data/data`，所以备份 volume 时需要保留整个 `/data` 挂载内容。定时任务、Cloudflare DNS 自动推送、GitHub 自动导出和上传筛选策略均通过 WebUI 保存到该数据目录；Docker 环境变量只负责运行时端口、鉴权、时区和数据挂载。

如需在 Docker 中查看运行时诊断，给 Compose 服务额外设置 `CFST_RUNTIME_DIAGNOSTICS=1`，然后从容器内部或本机回环访问 WebUI 诊断接口。通过宿主机浏览器访问容器映射端口通常会被服务端视为非回环请求；此时必须同时设置 `CFST_RUNTIME_DIAGNOSTICS_REMOTE=1` 和 `CFST_WEBUI_TOKEN`。诊断接口不会默认对公网开放；清理器本身不依赖诊断开关，未开启诊断时也会正常执行。

生成的镜像和 Compose 服务都使用内置健康检查：

```yaml
healthcheck:
  test: ["CMD", "/app/cfst-webui", "--healthcheck"]
```

`--healthcheck` 会从容器内请求 `http://127.0.0.1:34115/api/health`，不依赖 `curl` 或 `wget`，因此适配 `scratch` 镜像。

默认网络模式为 Docker bridge，并通过 `CFST_WEBUI_PORT` 发布端口。需要 host 网络时使用发行包内的 override：

```bash
docker compose -f docker-compose.yml -f docker-compose.host.yml up -d --build
```

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
| `CFST_VERSION` | `1.8.2` | `scripts/build-release.sh`、Android Gradle | 发行版本号；脚本会写入 Go `github.com/axuitomo/CFST-GUI/internal/app.version`。 |
| `GOMOBILE_BIN` | `$(go env GOPATH)/bin/gomobile` | Android 构建脚本 | gomobile 可执行文件路径。 |
| `ANDROID_HOME` | 自动推导 | Android 构建脚本 | Android SDK 目录。 |
| `ANDROID_SDK_ROOT` | 自动推导 | Android 构建脚本 | Android SDK 目录，优先级与 `ANDROID_HOME` 互相兼容。 |
| `ANDROID_NDK_HOME` | `<sdk>/ndk/26.3.11579264` | Android 构建脚本 | Android NDK 目录。 |
| `CFST_ANDROID_TOOLCHAIN_DIR` | `$XDG_CACHE_HOME/cfst-gui/android-toolchain` | `scripts/build-android-mobile.sh` | Debug 构建时自动推导 SDK/NDK 的工具链根目录。 |

更新下载代理策略：桌面端与 Android 端下载更新包时会并发尝试 `ghproxy.vip`、`gh.3w.pm`、`gh.ddlc.top` 和原始 GitHub Release 下载地址，优先使用最先完整下载且通过 SHA256 校验的结果。

`scripts/build-release.sh linux` 会一次生成 `amd64` 和 `arm64` 两种 Linux WebUI bundle；`linux-amd64` 与 `linux-arm64` 可按架构单独构建。脚本会用 `CFST_VERSION` 写入每个 bundle 的 `.env.example`，并生成 Docker context 与 `run-local.sh`。

## Android 签名

Release APK 签名只从环境变量读取，不把 keystore 或密码写入仓库：

| 变量 | 是否必需 | 说明 |
| --- | --- | --- |
| `CFST_ANDROID_KEYSTORE` | Release 必需 | release keystore 文件路径。 |
| `CFST_ANDROID_KEYSTORE_PASSWORD` | Release 必需 | keystore 密码。 |
| `CFST_ANDROID_KEY_ALIAS` | Release 必需 | key alias。 |
| `CFST_ANDROID_KEY_PASSWORD` | Release 必需 | key 密码。 |
| `CFST_ANDROID_VERSION_CODE` | 可选 | Android `versionCode`；默认 `10802`。 |
| `CFST_VERSION` | 可选 | Android `versionName`；默认 `1.8.2`，前缀 `v` 会被去掉。 |

本地 Release 构建示例：

```bash
export CFST_ANDROID_KEYSTORE=/absolute/path/release.jks
export CFST_ANDROID_KEYSTORE_PASSWORD=...
export CFST_ANDROID_KEY_ALIAS=...
export CFST_ANDROID_KEY_PASSWORD=...
export CFST_VERSION=1.8.2
bash scripts/build-release.sh android
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
ghcr.io/axuitomo/cfst-gui:latest
```

该工作流只有手动触发入口，输入 `version` 默认 `1.8.2`。它会先分别运行 `scripts/build-release.sh linux-amd64` 与 `scripts/build-release.sh linux-arm64` 生成 Docker context，再用 Docker Buildx 合并发布单一多架构 tag，覆盖 `linux/amd64` 与 `linux/arm64`。版本 tag 是固定引用，`latest` 是便捷滚动标签。
