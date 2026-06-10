# CFST-GUI

[![Go Version](https://img.shields.io/github/go-mod/go-version/axuitomo/CFST-GUI?style=flat-square&label=Go&color=00ADD8&logo=go)](go.mod)
[![Wails](https://img.shields.io/badge/Wails-v2.12.0-f36f45?style=flat-square)](https://wails.io/)
[![License](https://img.shields.io/github/license/axuitomo/CFST-GUI?style=flat-square&label=License)](LICENSE)

CFST-GUI 是一个基于 Wails + Vue + Capacitor 的 Cloudflare/CDN IP 测速工具，提供可视化任务面板、输入源管理、结果导出、配置同步、DNS 记录读取和自动推送能力。

当前产品形态覆盖 Wails 桌面 GUI、Linux WebUI 和 Android 应用，功能入口统一收敛到 Vue 前端界面。

## 当前状态

- 桌面端框架：Wails v2.12.0，默认启动原生桌面 GUI
- 后端：Go 1.26.2，保留 CFST 核心测速、过滤和 CSV 导出逻辑
- 前端：Vue 3 + Vite + Tailwind CSS + Phosphor Icons
- Linux WebUI：`webui` build tag 构建 HTTP 服务，提供 `/api/app/{method}`、SSE 和受限文件 API
- Android 架构：Vue + Capacitor WebView + Kotlin Plugin + gomobile AAR + `mobileapi` Go 服务
- Kotlin 作用：`CfstPlugin.kt` 负责把 Capacitor 调用转发到 gomobile 生成的 Go 服务，并处理 SAF 文件选择、导出 URI、安装更新和 probe 事件回传
- Android 发布基线：JDK 24、AGP 9.2.1、Gradle 9.5.1、KGP 2.4.0、SDK/target 37、Build Tools 37.0.0、NDK 29.0.14206865
- 发行产物：Windows、macOS、Linux WebUI、Android，统一输出到 `build/release/`
- 在线更新：设置页检查 GitHub Releases，按 `cfst-gui-update-manifest.json` 匹配平台资产；下载更新包时会并发尝试 `ghproxy.vip`、`gh.3w.pm`、`gh.ddlc.top` 和原始 GitHub Release 地址，并使用 SHA256 校验结果

## 功能概览

### 桌面测速

GUI 提供任务仪表盘和当前结果页，用于启动、跟踪和查看测速结果：

- 工作流默认从左到右执行：输入源组、测速、结果检查与输出、结束；结果检查与输出后仍可继续衔接导出、推送或其他处理
- 输入源组可从当前绑定配置中勾选单个输入源，测速节点内固定展示 TCP延迟测速、追踪测试、下载测速三个阶段
- 固定 4 阶段探测：IP池、TCP测延迟、追踪探测、文件测速
- 极速模式执行 IP池/TCP/追踪，完整模式额外执行文件测速
- TCP 平均延迟、丢包率（默认 15%，最高 100%）、可选追踪状态码、地区码、下载速度阈值过滤
- 地区码识别与结果排序
- 任务进度、活动日志、警告信息和当前测速结果页，结果页支持分页读取、排序、状态筛选和 IPv4/IPv6 筛选
- CSV 导出，默认文件名为 `result.csv`；桌面/WebUI 使用导出目录，Android 使用持久化授权的 SAF 导出目录

### 输入源管理

输入源可以来自远程 URL、本地文件或手动输入，并跟随桌面配置一起保存。
桌面端使用系统文件对话框选择本地输入文件和导出目录；Android 端使用系统 SAF 文件选择器，导入文件会复制到 app 私有目录供 Go 侧读取，SAF 持久授权仅用于导出目录。

输入源会按行清洗，自动跳过空行和 `#` 注释，并从复杂行中提取 IP/CIDR 或域名；域名会使用系统本地 DNS 解析为 IP 后参与测速。

支持两种候选处理方式：

- `traverse`：按顺序展开和整理输入源中的 IP/CIDR/域名
- `mcis`：界面显示为 MICS抽样，先使用内置抽样搜索引擎探索候选，再交给 CFST 做最终测速

每个输入源都可以独立设置启用状态、IP 上限和处理模式。

### 探测策略

内置两个主要预设：

- 极速模式：执行阶段 0/1/2，即 IP池、TCP测延迟和追踪探测，跳过文件测速
- 完整模式：执行阶段 0/1/2/3，在追踪通过后追加文件测速

阶段 1 TCP 默认发包 4 次并跳过首包统计，默认只有丢包率不超过 15% 的 IP 才会进入后续阶段，最高可配置到 100%；当前结果页和 CSV 中的延迟均为 TCP 平均延迟。追踪探测默认并发为 30，最高可设为 30；文件测速固定串行执行，单 IP 内部默认使用 4 个 HTTP Range GET 分片聚合测速，服务端不支持 Range 时回退完整流式 GET。文件测速遇到短文件完成、EOF 或临时断流时会在该时长内自动续连同一 IP，并累计预热后的有效测量窗口。

高级参数包括 TCP并发线程、测速上限、单 IP 下载测速时间、下载预热时间、GET 分片并发、下载协议、下载缓冲、端口、文件测速URL、追踪 URL、User-Agent、Host Header、SNI、通用请求 Headers、追踪有效状态码、地区码过滤、调试抓包开关和目标等。旧配置中的阶段 1、追踪候选上限和下载数量字段仍兼容读取，但不再截断阶段 1 或追踪候选；测速上限由 `stage_limits.stage3` 控制。

### DNS 记录读取与推送

界面提供独立的 Cloudflare 配置卡片和 DNS 读取页。DNS 页只负责通过 Cloudflare 官方 API 读取记录，不再承担手动推送入口；它可以读取当前 Zone 下全部记录、Cloudflare 配置中的记录名，或指定子域名/记录名，并支持按 A/AAAA 类型筛选。

Cloudflare DNS 推送能力仍然保留在后台链路：工作流 `deliver_dns` 节点、定时任务 DNS 推送，以及“测速后自动推送列表”中的 Cloudflare 勾选项会真实创建、更新或删除 DNS 记录。推送会复用 Cloudflare 配置、共享上传策略、Cloudflare Top N 和分流规则；IPv4 写入 A，IPv6 写入 AAAA。执行前请确认 API Token、Zone ID、记录名称和分流规则正确，避免覆盖生产记录。

### 配置、档案与同步

配置以当前 `config_snapshot` schema 为准，桌面/WebUI 使用 `desktop-config.json`，Android 使用同构的 `mobile-config.json`。

- 配置包导入/导出使用 ZIP，归档内固定包含 `cfst-gui-config.json`
- WebDAV 支持测试、备份和还原远端配置包
- `source-profiles.json` 管理输入组（兼容沿用旧文件名和字段）
- GitHub 导出可把结果 CSV 推送到指定仓库路径
- 旧配置读取时会补齐新字段默认值、迁移常见旧字段别名并忽略未知字段；读取本身不会静默改写磁盘，保存、导入、WebDAV 写回和档案切换时会落盘为当前规范格式

配置导出、ZIP 归档、WebDAV 备份和本地备份文件可能包含 Cloudflare Token、WebDAV 凭据、导出路径和输入源路径，请只保存到可信位置。

## 文档入口

更完整的使用、部署和接口说明在 `docs/` 目录：

| 场景 | 文档 |
| --- | --- |
| 面向普通用户了解产品定位、发行资产和安装建议 | [介绍产品.md](介绍产品.md) |
| CLI 参数、运行模式和验证命令 | [docs/cli.md](docs/cli.md) |
| 桌面、WebUI、Android 和 Release 构建 | [docs/deployment.md](docs/deployment.md) |
| 配置目录、字段默认值和旧配置兼容 | [docs/configuration.md](docs/configuration.md) |
| Cloudflare DNS 读取/推送 Token 最小权限 | [docs/cloudflare-api-token.md](docs/cloudflare-api-token.md) |
| GitHub 结果导出 PAT 最小权限 | [docs/github-pat.md](docs/github-pat.md) |
| Cloudflare/GitHub 上传筛选和自动推送口径 | [docs/upload-workflow-design.md](docs/upload-workflow-design.md) |
| WebUI、Docker、Android 和 Actions 环境变量 | [docs/docker-env.md](docs/docker-env.md) |
| Android 架构、SAF 文件访问和移动端桥接 | [docs/android-mobile.md](docs/android-mobile.md) |
| Wails/WebUI/Android API、事件和源码定位 | [docs/功能与相关接口文档.md](docs/功能与相关接口文档.md) |
| v1.8.2 发布说明与资产清单 | [docs/release-notes/v1.8.2.md](docs/release-notes/v1.8.2.md) |

## 运行方式

### 准备环境

需要安装：

- Go 1.26.2
- Node.js / npm
- Wails v2 开发工具

安装 Wails 开发工具：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

安装前端依赖：

```bash
cd frontend
npm install
```

### 启动桌面 GUI

在仓库根目录运行：

```bash
wails dev
```

或构建当前嵌入式前端后直接运行 Go 程序：

```bash
go run .
```

### 构建发行版

```bash
export CFST_ANDROID_KEYSTORE=/absolute/path/release.jks
export CFST_ANDROID_KEYSTORE_PASSWORD=...
export CFST_ANDROID_KEY_ALIAS=...
export CFST_ANDROID_KEY_PASSWORD=...
bash scripts/build-release.sh
```

也可以按目标单独构建：`bash scripts/build-release.sh <target>`。可用 target 包括 `windows`、`linux`、`linux-amd64`、`linux-arm64`、`darwin-amd64`、`darwin-arm64`、`android` 和 `manifest`。其中 `linux` 会一次生成 `amd64` 和 `arm64` 两个 WebUI bundle；macOS 产物需要在 macOS runner/主机上构建，GitHub Actions 会按平台拆分后再统一生成 manifest 与 Release。

发行版会生成以下最终产物：

- `build/release/desktop/cfst-gui-windows-amd64.exe`
- `build/release/desktop/cfst-gui-linux-amd64.tar.gz`
- `build/release/desktop/cfst-gui-linux-arm64.tar.gz`
- `build/release/desktop/cfst-gui-darwin-amd64.app.zip`
- `build/release/desktop/cfst-gui-darwin-arm64.app.zip`
- `build/release/android/cfst-gui-android-release.apk`
- `build/release/android/cfst-gui-android-arm64-v8a-release.apk`
- `build/release/android/cfst-gui-android-armeabi-v7a-release.apk`
- `build/release/cfst-gui-update-manifest.json`

Windows 和 macOS 桌面端默认使用自适应窗口尺寸：启动时最大化到当前屏幕可用区域，设置页可切换固定验收尺寸并随时恢复“自适应”。Linux 发行包提供 `amd64` / `arm64` 两种 WebUI bundle，既支持 `docker compose up -d --build`，也支持直接执行 bundle 内的 `./run-local.sh` 在本机运行；界面随浏览器 viewport 响应式自适应，固定验收尺寸仅 Wails 桌面支持。Docker 部署默认端口为 `34115`，数据通过 Docker volume 持久化，Compose 默认带 `Asia/Shanghai` 时区、健康检查和可选 host 网络 override；本地运行默认监听 `127.0.0.1:34115`，并把便携数据放在 bundle 内 `portable/data`。Android 使用移动壳响应式布局。Windows 桌面构建会启用托盘后台能力；关闭窗口时隐藏到系统托盘，托盘菜单提供“打开主界面”和“关闭软件”。如果目标环境无法初始化托盘，关闭窗口会直接退出，避免隐藏后无法找回。macOS 发行包暂不启用托盘，以避免与 Wails 原生 AppDelegate 链接冲突。

Android 构建默认会把 `gomobile` 生成的 `libgojni.so` 链接为 16KB 页对齐，同时保持对 4KB 页设备的兼容，以满足新设备页大小要求。Debug 和 Release 构建会检查 split APK 的 16KB ELF/zipalign 状态与最终 manifest，覆盖 SDK 37、Android 13 通知权限、Android 14 dataSync 前台服务、WorkManager、FileProvider 和更新清理 receiver。Android 在线更新 APK 只通过 app 私有 `files/updates/` 暴露给 `FileProvider` 安装确认。

GitHub Actions 的发行流水线位于 `.github/workflows/release.yml`，由 `v*` tag 或手动触发。Android Release 签名需要配置这些 Secrets：`CFST_ANDROID_KEYSTORE_BASE64`、`CFST_ANDROID_KEYSTORE_PASSWORD`、`CFST_ANDROID_KEY_ALIAS`、`CFST_ANDROID_KEY_PASSWORD`。Windows `exe` 安装器签名需要 `CFST_WINDOWS_SIGNING_CERT_BASE64` 和 `CFST_WINDOWS_SIGNING_PASSWORD`，workflow 会在 Windows runner 上安装 NSIS 并生成经典安装包。

## 常用开发命令

```bash
# 首次开发建议先让 Wails 生成前端桥接代码
wails dev

# 一键本地质量门禁
bash scripts/ci-local.sh

# 快速功能检查：Go 测试 + 前端 typecheck/build
bash scripts/check.sh

# Lint：go vet + shellcheck + ESLint
bash scripts/lint.sh

# 格式化或格式检查
bash scripts/format.sh
bash scripts/format-check.sh

# 依赖校验与安全审计
bash scripts/audit.sh

# Wails/前端生成物一致性检查
bash scripts/verify-generated.sh

# Android debug 构建、16KB 页对齐和 APK manifest 检查
bash scripts/check-android.sh

# 清理忽略的构建产物，先用 dry-run 确认影响范围
bash scripts/clean.sh --dry-run

# 诊断当前开发环境；Android 可在连接设备后追加 --device-smoke
bash scripts/doctor.sh
bash scripts/android-doctor.sh
bash scripts/android-doctor.sh --device-smoke --device-smoke-apk mobile/android/app/build/outputs/apk/debug/app-universal-debug.apk

# 新机器初始化或重建开发环境
bash scripts/bootstrap.sh --install-tools
bash scripts/dev-reset.sh --dry-run

# 只检查当前变更，或安装本地 Git hooks
bash scripts/changed-check.sh
bash scripts/hooks-install.sh

# 发版前检查、版本号同步、产物检查
bash scripts/release-preflight.sh 1.8.2 --allow-dirty
bash scripts/version-bump.sh 1.8.2
bash scripts/artifact-inspect.sh --allow-missing

# 前端 bundle、依赖、文档、结果文件和密钥扫描
bash scripts/bundle-report.sh
bash scripts/update-deps-report.sh
bash scripts/docs-check.sh
bash scripts/validate-results.sh --dir cfst-results
bash scripts/secrets-scan.sh

# 启动开发模式
bash scripts/open-dev.sh desktop

# 发行版构建
bash scripts/build-release.sh
```

如果单独执行前端命令时提示缺少 `frontend/wailsjs`，先回到仓库根目录运行一次 `wails dev`、`wails generate module` 或 `bash scripts/check.sh` 生成 Wails 桥接代码。

`scripts/format-check.sh` 默认只检查当前变更涉及的前端文件，避免在未建立 Prettier 全量基线前阻塞无关文件；需要全量检查时运行 `CFST_FORMAT_SCOPE=all bash scripts/format-check.sh`。GitHub Actions 的 PR 质量门禁位于 `.github/workflows/quality.yml`，会调用 `bash scripts/ci-local.sh`。

帮助脚本默认以只读诊断或 dry-run 为主；会修改文件或本地环境的脚本会要求显式参数，例如 `bash scripts/dev-reset.sh --apply`、`bash scripts/version-bump.sh <version> --apply`、`bash scripts/hooks-install.sh --force`。如果只想快速验证当前改动，优先运行 `bash scripts/changed-check.sh`；发版前运行 `bash scripts/release-preflight.sh <version>` 和 `bash scripts/artifact-inspect.sh`。

## 配置与数据

默认配置根目录由 Go 的 `os.UserConfigDir()` 决定，并追加 `CFST-GUI` 子目录。当前版本不再支持通过界面选择自定义储存目录；桌面端固定使用应用数据目录，Android 固定使用 app 私有目录。仍可使用 `CFST_GUI_PORTABLE_ROOT` / `portable.json` 启用便携数据目录。

主要文件和目录：

- `storage.json`：储存 bootstrap；旧版自定义 `storage_dir` 只用于一次性迁移
- `desktop-config.json`：桌面 GUI / WebUI 当前配置快照
- `mobile-config.json`：Android 当前配置快照，结构与桌面快照同构
- `config.json`：兼容旧桥接结构的配置文件
- `source-profiles.json`：输入组，包含 `items[].sources`，兼容沿用旧文件名和字段
- `cfip-log.txt`：调试日志，默认 JSONL，也可在设置页选择自由格式文本和记录粒度
- `exports/`、`imports/`、`backups/`：建议用于结果导出、导入文件和导入前备份

旧版配置缺少的新字段会在读取时补当前默认值；未知字段不会导致读取失败，但保存、导入、WebDAV 写回或切换档案后会被清理为当前规范格式。更完整字段说明见 [配置详解](docs/configuration.md)。

## 项目结构

```text
.
├── main.go                         # 薄启动入口，注入资源后调用 internal/app.Run
├── resources.go                    # 根目录资源桥接，向 internal/app 注入前端 FS 和托盘图标
├── frontend_assets.go              # frontend/dist 嵌入资源，保持 go:embed 路径稳定
├── tray_icon*.go                   # tray build tag 下嵌入 build/ 图标，非 tray 使用 stub
├── frontend/                       # Vue 前端，桌面、WebUI 和 Android 共用
│   ├── src/App.vue                 # UI 状态编排、任务流和页面事件入口
│   ├── src/views/                  # 仪表盘、结果、输入源、配置、DNS 页面
│   ├── src/lib/bridge.ts           # Wails/WebUI/Capacitor 三端桥接适配层
│   ├── dist/                       # 生产静态资源，供桌面/WebUI/Android 打包
│   └── capacitor.config.ts         # Android Capacitor 配置
├── mobileapi/                      # gomobile 暴露给 Android Kotlin 层的 Go 服务
│   ├── config_compat.go            # Android 配置 schema 兼容和字段净化
│   ├── probe.go / storage.go       # 移动端测速、配置和档案持久化
│   └── archive.go / github_export.go / dns.go
├── mobile/android/                 # Android 原生工程、Kotlin Plugin、资源和 Gradle 配置
├── internal/
│   ├── app/                        # 桌面/WebUI 应用实现、CLI 分发、配置和更新能力
│   │   ├── run.go                  # 模式判定、CLI/GUI 分发和版本信息
│   │   ├── app.go / app_archive.go # 后端 App 方法、配置归档和 WebDAV
│   │   ├── gui.go / app_wails.go   # Wails 窗口、后端绑定和前端资源注入
│   │   ├── webui.go / app_webui.go # Linux WebUI HTTP API、静态资源和文件访问
│   │   ├── storage.go / config_compat.go
│   │   ├── desktop_sources.go / desktop_colo_dictionary.go / desktop_probe_events.go
│   │   └── scheduler.go / cloudflare_dns.go / github_export.go / update*.go
│   ├── colodict/                   # COLO 字典处理
│   ├── httpcfg/ / httpclient/      # HTTP 配置与客户端
│   ├── mcis/                       # MICS 抽样搜索
│   ├── sourceparse/                # 输入源解析
│   ├── task/                       # CFST TCP、追踪、HTTPing、下载测速和重试策略
│   └── utils/                      # CSV、精度、调试日志、输出辅助
├── docs/                           # 使用、部署、配置、接口和 release notes 文档
├── scripts/                        # Android、桌面和统一 Release 构建脚本
├── .github/                        # Issue 模板、Release 和 GHCR Actions 工作流
├── build/                          # 应用图标、平台资源和本地构建/发行输出
├── tools/                          # 开发辅助工具
└── wails.json                      # Wails 项目配置
```

## 模块路径

当前 Go module 路径为：

```text
github.com/axuitomo/CFST-GUI
```

## 注意事项

- 默认文件测速URL为 `https://speed.cloudflare.com/__down?bytes=10000000`，生产使用可按需换成自建测试地址。
- 后端 HTTP 出口统一使用共享客户端，默认优先尝试 HTTP/3，失败后回退到 TCP 上的 HTTP/1.1/2；测速 GET 会带 `Cache-Control: no-store` 和 `Pragma: no-cache`，并校验长度、Range 与可用的 Digest/MD5/SHA256 响应头。
- 网络测速结果会受代理、运营商、路由器策略和本地网络状态影响。
- 追踪探测、文件测速与大规模扫描可能触发远端或网络侧限制；GUI 会把追踪并发线程限制在当前后端允许范围内。
- DNS 读取页不会修改线上记录；工作流、定时任务和测速后自动推送中的 Cloudflare 推送会真实修改 Cloudflare 线上记录，建议先读取记录并确认配置后再启用。
- 配置和归档文件可能包含敏感凭据，不要提交到公开仓库或公开分享。

## 致谢与参考

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest)：感谢其提供最初的核心测速逻辑基础
- [Leo-Mu/montecarlo-ip-searcher](https://github.com/Leo-Mu/montecarlo-ip-searcher)：MICS 抽样搜索思路
- [cmliu/CF-Workers-SpeedTestURL](https://github.com/cmliu/CF-Workers-SpeedTestURL)：测速 URL 相关参考
- [Netrvin/cloudflare-colo-list](https://github.com/Netrvin/cloudflare-colo-list)：Cloudflare COLO 数据来源参考
- [Cloudflare local IP ranges](https://api.cloudflare.com/local-ip-ranges.csv)：Cloudflare 本地 IP 段数据来源
- [xiaolin-007/CloudFlareScan](https://github.com/xiaolin-007/CloudFlareScan)：Cloudflare 扫描工具相关参考

## License

本项目沿用 GPL-3.0 License，详见 [LICENSE](LICENSE)。
