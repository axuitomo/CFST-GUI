# CFST-GUI

[![Go Version](https://img.shields.io/github/go-mod/go-version/axuitomo/CFST-GUI?style=flat-square&label=Go&color=00ADD8&logo=go)](go.mod)
[![Wails](https://img.shields.io/badge/Wails-v2.10.2-f36f45?style=flat-square)](https://wails.io/)
[![License](https://img.shields.io/github/license/axuitomo/CFST-GUI?style=flat-square&label=License)](LICENSE)

CFST-GUI 是一个基于 Wails + Vue 的桌面端 Cloudflare/CDN IP 测速工具。它复用 `XIU2/CloudflareSpeedTest` 的 Go 核心测速逻辑，并在其上补了一层可视化任务面板、输入源管理、结果导出和 DNS 推送工作台。

当前仓库已经不是上游命令行项目的原始 README 所描述的状态：默认启动桌面 GUI；传入 CLI 参数时仍可按原 CFST 命令行方式运行。

## 当前状态

- 桌面端框架：Wails v2
- 后端：Go 1.22，保留 CFST 核心测速、过滤和 CSV 导出逻辑
- 前端：Vue 3 + Vite + Tailwind CSS + Phosphor Icons
- 默认模式：无参数运行时启动 GUI；带参数运行时进入 CLI
- 发行产物：本仓库不提交 `Releases/`、`build/bin/` 等本地构建产物

## 功能概览

### 桌面测速

GUI 提供任务仪表盘，用于启动、跟踪和查看测速结果：

- TCPing / HTTPing 延迟测速
- 可选下载测速
- 平均延迟、丢包率、下载速度阈值过滤
- 地区码识别与结果排序
- 任务进度、活动日志、警告信息和结果表格
- CSV 导出，默认文件名为 `result.csv`

### 输入源管理

输入源可以来自远程 URL、本地文件或手动输入，并跟随桌面配置一起保存。

支持两种候选 IP 处理方式：

- `traverse`：按顺序展开和整理输入源中的 IP/CIDR
- `mcis`：先使用内置 MCIS 搜索引擎探索候选，再交给 CFST 做最终测速

每个输入源都可以独立设置启用状态、IP 上限和处理模式。

### 探测策略

内置两个主要预设：

- 极速模式：跳过下载测速，只执行 TCP/HTTP 响应测速，适合日常快速更新候选 IP
- 完整模式：在低延迟筛选后追加真实下载测速，适合带宽优先场景

高级参数包括并发数、测速次数、端口、测试 URL、User-Agent、Host Header、SNI、HTTPing 状态码、地区码过滤、调试抓包目标等。

### DNS 工作台

界面提供 Cloudflare 配置表单和 DNS 推送面板，用于整理测速结果或手动 IP 列表。

注意：当前前端桥接实现会把 DNS 推送映射为本地记录预览，并不会真正写入 Cloudflare DNS。Cloudflare API Token、Zone ID、记录名等字段已经在配置结构中预留，真实写入逻辑需要后续接入。

## 运行方式

### 准备环境

需要安装：

- Go 1.22+
- Node.js / npm
- Wails CLI v2

安装 Wails CLI：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
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

### 使用 CLI 模式

只要传入 CFST 参数，程序会进入命令行模式：

```bash
go run . --cli -f ip.txt -n 200 -t 4 -o result.csv
```

也可以直接传原 CFST 参数：

```bash
go run . -f ip.txt -httping -tl 200
```

如果需要显式启动 GUI：

```bash
go run . --gui
```

### 构建桌面应用

```bash
wails build
```

构建输出会进入 Wails 的本地产物目录，仓库默认忽略 `build/bin/` 和 `Releases/`，避免把二进制或压缩包提交到 Git。

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
```

如果单独执行前端命令时提示缺少 `frontend/wailsjs`，先回到仓库根目录运行一次 `wails dev` 或 `wails build` 生成 Wails 桥接代码。

## 配置与数据

桌面配置默认写入系统用户配置目录：

- `CFST-GUI/desktop-config.json`：桌面端配置快照
- `CFST-GUI/config.json`：兼容配置
- `CFST-GUI/cfip-log.txt`：调试日志

具体根目录由 Go 的 `os.UserConfigDir()` 决定。Windows、macOS、Linux 的实际位置会有所不同。

## 项目结构

```text
.
├── app.go                    # Wails 后端桥接、配置、任务执行
├── gui.go                    # Wails 桌面窗口入口
├── main.go                   # GUI/CLI 双入口
├── desktop_sources.go        # 桌面输入源读取、预览和 MCIS 处理
├── desktop_probe_events.go   # Wails 事件推送
├── frontend/                 # Vue 桌面界面
│   ├── src/views/            # 仪表盘、输入源、配置、DNS 页面
│   └── src/lib/bridge.ts     # 前端到 Wails 后端的桥接层
├── internal/                 # HTTP 配置、MCIS 搜索引擎
├── task/                     # CFST 延迟/下载测速逻辑
├── utils/                    # CSV、输出、调试辅助
├── docs/                     # 功能与接口说明
├── script/                   # 上游 CFST 脚本示例
└── wails.json                # Wails 项目配置
```

## 与上游 CFST 的关系

本项目保留并扩展了 `XIU2/CloudflareSpeedTest` 的核心命令行能力，Go module 路径目前仍是：

```text
github.com/XIU2/CloudflareSpeedTest
```

GUI 侧新增了桌面配置、输入源预处理、任务事件、前端展示和本地 DNS 预览等能力。原 CLI 参数仍可作为兼容入口使用。

## 注意事项

- 默认测试 URL 为 `https://cf.xiu2.xyz/url`，稳定性不由本项目保证，生产使用建议换成自建测试地址。
- 网络测速结果会受代理、运营商、路由器策略和本地网络状态影响。
- HTTPing 与大规模扫描可能触发远端或网络侧限制，建议合理控制并发。
- 当前 DNS 推送功能仍是本地预览，不应视为已经完成 Cloudflare DNS 写入。

## License

本项目沿用 GPL-3.0 License，详见 [LICENSE](LICENSE)。
