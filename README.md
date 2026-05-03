# CFST-GUI

[![Go Version](https://img.shields.io/github/go-mod/go-version/axuitomo/CFST-GUI?style=flat-square&label=Go&color=00ADD8&logo=go)](go.mod)
[![Wails](https://img.shields.io/badge/Wails-v2.12.0-f36f45?style=flat-square)](https://wails.io/)
[![License](https://img.shields.io/github/license/axuitomo/CFST-GUI?style=flat-square&label=License)](LICENSE)

CFST-GUI 是一个基于 Wails + Vue 的桌面端 Cloudflare/CDN IP 测速工具。它复用 `XIU2/CloudflareSpeedTest` 的 Go 核心测速逻辑，并在其上补了一层可视化任务面板、输入源管理、结果导出和 DNS 推送工作台。

当前仓库已经不是上游项目原始 README 所描述的状态：产品形态以桌面 GUI 和 Android 应用为主，功能入口统一收敛到 Vue 前端界面。

## 当前状态

- 桌面端框架：Wails v2.12.0
- 后端：Go 1.26.2，保留 CFST 核心测速、过滤和 CSV 导出逻辑
- 前端：Vue 3 + Vite + Tailwind CSS + Phosphor Icons
- Android 架构：Vue + Capacitor WebView + Java Plugin + gomobile AAR + `mobileapi` Go 服务
- Java 作用：`CfstPlugin.java` 负责把 Capacitor 调用转发到 gomobile 生成的 Go 服务，并处理 SAF 文件选择、导出 URI、安装更新和 probe 事件回传
- 默认模式：桌面端启动 Wails GUI；Android 端启动 Capacitor WebView
- 发行产物：Windows、macOS、Linux、Android，统一输出到 `build/release/`
- 在线更新：设置页检查 GitHub Releases，按 `cfst-gui-update-manifest.json` 下载匹配平台资产

## 功能概览

### 桌面测速

GUI 提供任务仪表盘和当前结果页，用于启动、跟踪和查看测速结果：

- 固定 4 阶段探测：IP池、TCP测延迟、追踪探测、文件测速
- 极速模式执行 IP池/TCP/追踪，完整模式额外执行文件测速
- TCP 平均延迟、丢包率（最高 15%）、追踪状态码、地区码、下载速度阈值过滤
- 地区码识别与结果排序
- 任务进度、活动日志、警告信息和当前测速结果页，结果展示 TCP、追踪和文件测速指标
- CSV 导出，默认文件名为 `result.csv`；Android 可通过系统 SAF 保存到用户选择的文件

### 输入源管理

输入源可以来自远程 URL、本地文件或手动输入，并跟随桌面配置一起保存。
桌面端使用系统文件对话框选择本地输入文件和导出目录；Android 端使用系统 SAF 文件选择器，导入文件会复制到 app 私有目录供 Go 侧读取。

支持两种候选 IP 处理方式：

- `traverse`：按顺序展开和整理输入源中的 IP/CIDR
- `mcis`：界面显示为 MICS抽样，先使用内置抽样搜索引擎探索候选，再交给 CFST 做最终测速

每个输入源都可以独立设置启用状态、IP 上限和处理模式。

### 探测策略

内置两个主要预设：

- 极速模式：执行阶段 0/1/2，即 IP池、TCP测延迟和追踪探测，跳过文件测速
- 完整模式：执行阶段 0/1/2/3，在追踪通过后追加文件测速

阶段 1 TCP 默认发包 4 次并跳过首包统计，只有丢包率不超过 15% 的 IP 才会进入后续阶段；当前结果页和 CSV 中的延迟均为 TCP 平均延迟。追踪探测默认并发为 6，最高可设为 20；文件测速固定串行执行，所有追踪通过的 IP 都会进入文件测速，下载测速时间表示每个 IP 的单次测速时长。

高级参数包括 TCP并发线程、追踪候选上限、单 IP 下载测速时间、测速次数、端口、文件测速URL、追踪 URL、User-Agent、Host Header、SNI、追踪有效状态码、地区码过滤、调试抓包目标等。旧配置中的下载数量字段仍保留兼容，但不再限制文件测速数量。

### DNS 工作台

界面提供 Cloudflare 配置表单和 DNS 推送面板，用于把测速结果或手动 IP 列表覆盖推送到 Cloudflare DNS。

DNS 面板已经接入真实 Cloudflare API：读取记录会访问当前 Zone 下匹配记录名的 A/AAAA 线上记录，推送会按 IP 地址族自动识别记录类型，IPv4 写入 A、IPv6 写入 AAAA。TTL 固定支持 1 分钟、5 分钟、10 分钟三档，默认 5 分钟。执行前请确认 API Token、Zone ID 和记录名称正确，避免覆盖生产记录。

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
./scripts/build-release.sh
```

也可以按目标单独构建：`./scripts/build-release.sh windows|linux|darwin-amd64|darwin-arm64|android|manifest`。macOS 产物需要在 macOS runner/主机上构建，GitHub Actions 会按平台拆分后再统一生成 manifest 与 Release。

发行版会生成以下最终产物：

- `build/release/desktop/cfst-gui-windows-amd64.exe`
- `build/release/desktop/cfst-gui-linux-amd64.tar.gz`
- `build/release/desktop/cfst-gui-darwin-amd64.app.zip`
- `build/release/desktop/cfst-gui-darwin-arm64.app.zip`
- `build/release/android/cfst-gui-android-release.apk`
- `build/release/cfst-gui-update-manifest.json`

Windows/Linux 桌面构建会启用托盘后台能力；关闭窗口时隐藏到系统托盘，托盘菜单提供“打开主界面”和“关闭软件”。如果目标环境无法初始化托盘，关闭窗口会直接退出，避免隐藏后无法找回。macOS 发行包暂不启用托盘，以避免与 Wails 原生 AppDelegate 链接冲突。

GitHub Actions 的发行流水线位于 `.github/workflows/release.yml`，由 `v*` tag 或手动触发。Android Release 签名需要配置这些 Secrets：`CFST_ANDROID_KEYSTORE_BASE64`、`CFST_ANDROID_KEYSTORE_PASSWORD`、`CFST_ANDROID_KEY_ALIAS`、`CFST_ANDROID_KEY_PASSWORD`。

## 常用开发命令

```bash
# 首次开发建议先让 Wails 生成前端桥接代码
wails dev

# 前端类型检查与构建
cd frontend
npm run typecheck
npm run build

# Go 侧检查
cd ..
go test ./...

# 发行版构建
./scripts/build-release.sh
```

如果单独执行前端命令时提示缺少 `frontend/wailsjs`，先回到仓库根目录运行一次 `wails dev` 或 `wails build` 生成 Wails 桥接代码。

## 配置与数据

桌面配置默认写入系统用户配置目录：

- `CFST-GUI/desktop-config.json`：桌面端配置快照
- `CFST-GUI/config.json`：兼容配置
- `CFST-GUI/cfip-log.txt`：JSONL 调试日志

具体根目录由 Go 的 `os.UserConfigDir()` 决定。Windows、macOS、Linux 的实际位置会有所不同。

## 项目结构

```text
.
├── app.go                    # Wails 后端桥接、配置、任务执行
├── gui.go                    # Wails 桌面窗口入口
├── main.go                   # 桌面应用启动入口
├── desktop_sources.go        # 桌面输入源读取、预览和 MICS抽样处理
├── desktop_probe_events.go   # Wails 事件推送
├── tray.go                   # 桌面后台托盘生命周期
├── tray_systray.go           # Windows/Linux 托盘菜单与图标
├── frontend/                 # Vue 前端，桌面与 Android 共用
│   ├── src/views/            # 仪表盘、当前结果、输入源、配置、DNS 页面
│   ├── src/lib/bridge.ts     # Wails/Capacitor 双端桥接适配层
│   └── capacitor.config.ts   # Android Capacitor 配置
├── mobileapi/                # gomobile 暴露给 Android Java 层的 Go 服务
├── mobile/android/           # Android 原生工程、Java Plugin、资源和 Gradle 配置
├── internal/                 # HTTP 配置、MICS抽样搜索引擎
├── task/                     # CFST TCP/追踪/文件测速逻辑
├── utils/                    # CSV、输出、调试辅助
├── docs/                     # 功能与接口说明
├── scripts/                  # 桌面、Android、Release 构建脚本
├── script/                   # 上游 CFST 脚本示例
├── build/                    # 应用图标、平台资源和构建输出
└── wails.json                # Wails 项目配置
```

## 与上游 CFST 的关系

本项目保留并扩展了 `XIU2/CloudflareSpeedTest` 的核心测速能力，Go module 路径目前仍是：

```text
github.com/XIU2/CloudflareSpeedTest
```

GUI 侧新增了桌面配置、输入源预处理、任务事件、前端展示和 Cloudflare DNS 推送等能力。

## 注意事项

- 默认文件测速URL为 `https://speed.cloudflare.com/__down?bytes=10000000`，生产使用可按需换成自建测试地址。
- 网络测速结果会受代理、运营商、路由器策略和本地网络状态影响。
- 追踪探测、文件测速与大规模扫描可能触发远端或网络侧限制；GUI 会把追踪并发线程限制在 6。
- DNS 推送会真实修改 Cloudflare 线上记录，建议先读取记录并确认配置后再执行覆盖推送。

## 使用到的项目

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest)：核心测速逻辑和脚本基础
- [Leo-Mu/montecarlo-ip-searcher](https://github.com/Leo-Mu/montecarlo-ip-searcher)：MICS 抽样搜索思路
- [cmliu/CF-Workers-SpeedTestURL](https://github.com/cmliu/CF-Workers-SpeedTestURL)：测速 URL 相关参考
- [Netrvin/cloudflare-colo-list](https://github.com/Netrvin/cloudflare-colo-list)：Cloudflare COLO 数据来源参考
- [Cloudflare local IP ranges](https://api.cloudflare.com/local-ip-ranges.csv)：Cloudflare 本地 IP 段数据来源

## License

本项目沿用 GPL-3.0 License，详见 [LICENSE](LICENSE)。
