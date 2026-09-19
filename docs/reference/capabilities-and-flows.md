# 功能与运行链路

> 本页属于 [参考文档](./README.md) 的一部分，说明 CFST-GUI 的功能总览和主要运行链路。接口契约、配置结构等见参考文档入口的文档地图。

## 1. 功能总览

### 1.1 桌面 GUI

无参数运行时，程序启动 Wails 桌面 GUI。前端由 Vue 3 + Vite + Tailwind CSS 构建，后端由 Go 绑定 `App` 实例给 Wails。

桌面端主要页面如下：

| 页面 | 功能 |
| --- | --- |
| 任务看板 | 启动探测任务、展示进度、事件过程、警告和导出历史 |
| 当前结果 | 展示当前测速任务的结果表格、移动端结果卡片、排序过滤和导出位置 |
| 输入源 | 管理 URL、本地文件、手动输入三类 IP 来源 |
| 系统配置 | 保存 Cloudflare、导出、探测策略、高级 HTTP 和调试参数 |
| DNS 读取 | 通过 Cloudflare 官方 API 读取当前 Zone、当前配置记录或指定子域名记录；页面本身不执行推送 |

### 1.2 桌面测速

桌面、WebUI、Android 和 CLI 均由 `internal/appcore.Service` 持有任务运行时，并通过 `internal/probecore.RunProbeStages` 执行阶段 1/2/3 的共享编排。输入源准备、事件、任务持久化、上传和普通文件生成均由共享核心负责；平台层只负责传输、桌面路径交互、CLI 控制台输出和 Android SAF 发布。

| 功能 | 当前行为 | 关键代码 |
| --- | --- | --- |
| 阶段 0 IP池 | 读取输入源、清洗并展开候选 IP/CIDR/域名，并推送 `probe.preprocessed` | `internal/appcore/source_service.go`、`source_entries.go` |
| 阶段 1 TCP测延迟 | 强制执行 TCP 建连测速，`delayMs` 定义为 TCP 平均延迟 | `internal/task/tcping.go`、`internal/probecore/stage_workflow.go` |
| 阶段 2 追踪探测 | 对 TCP 通过候选执行 GET `/cdn-cgi/trace`，GUI 默认并发 30，最大并发 30，用于筛选和地区码 | `internal/task/head.go`、`internal/task/colo.go` |
| 阶段 3 文件测速 | `full` 策略追加文件下载测速，可按配置使用受限并发；下载时间按单 IP 生效 | `internal/task/download.go`、`internal/probecore/stage_workflow.go` |
| 阈值过滤 | 支持 TCP 延迟上限/下限、丢包率上限（硬上限 100%，默认 15%）、下载速度下限 | `internal/probecore/probe_config.go`、`stage_workflow.go` |
| 地区码识别 | 追踪响应 body `colo=XXX` 优先，`CF-RAY` 正则备用，并兼容 CloudFront、Fastly、Gcore、CDN77、Bunny 响应头 | `internal/task/colo.go` |
| CSV 导出 | 默认导出 `result.csv`，可配置目标目录和文件名 | `internal/utils/csv.go`、`internal/appcore/probe_service.go` |
| 调试抓包 | 支持自定义 User-Agent、Host Header、SNI，并可单独启用/关闭调试拨号目标 | `internal/httpcfg/profile.go`、`internal/task/request_profile.go` |

### 1.3 输入源管理

输入源随桌面配置保存，每个来源可独立设置启用状态、类型、IP 上限和 IP 处理模式。内容会按行清洗，跳过空行和 `#` 注释；行内 `#` 后的备注会被丢弃，再从主体内容中提取 IP/CIDR 或域名。域名使用系统本地 DNS 解析为 A/AAAA 后参与测速。

| 输入源类型 | 字段 | 读取方式 |
| --- | --- | --- |
| `url` | `url` | 使用 HTTP GET 读取远程 IP 列表；可省略协议，默认按 `https://` 处理 |
| `file` | `path` | 使用本地文件路径读取 IP 列表 |
| `inline` | `content` | 使用文本框中的手动输入内容 |

候选 IP 处理模式：

| 模式 | 行为 |
| --- | --- |
| `traverse` | 解析 IP/CIDR/域名，按顺序展开 CIDR，并按 `ip_limit` 截断 |
| `mcis` | 界面显示为 MICS抽样，将 IP/CIDR/域名解析后的候选转换为 CIDR 输入，先通过内置抽样搜索引擎和可选 IATA COLO 白名单筛选候选，再交给 CFST 做最终测速 |

### 1.4 探测策略

桌面端配置中有两个主要策略：

| 策略 | 说明 |
| --- | --- |
| `fast` | 默认策略，执行阶段 0/1/2：IP池、TCP测延迟、追踪探测，跳过文件测速 |
| `full` | 执行阶段 0/1/2/3：在追踪通过后追加文件测速 |

阶段 1 TCP 默认发包 4 次，启用跳过首包时首包不计入平均延迟和丢包统计。当前结果页、前端 `tcp_latency_ms` 和 CSV 延迟列只展示 TCP 平均延迟；阶段 2 追踪延迟通过 `traceDelayMs` / `trace_latency_ms` 提供给 UI，CSV 保持旧列格式。

阶段 2 追踪探测使用独立并发闸门，默认并发为 30，最大并发为 30。旧配置中 `concurrency.stage2` 超过 30 时会归一化为 30。阶段 3 文件测速只在 `full` 策略运行，外层文件测速并发固定为 1；单 IP 内部默认使用 4 个 HTTP Range GET 分片聚合测速，服务端不支持 Range 时回退完整流式 GET。文件测速在单 IP 时长内遇到 EOF、短文件完成或临时断流时会自动对同一 IP 续连，并累计预热后的有效测量窗口。下载协议 `auto` 在 Linux ARM 和 Android 上会回退到 `tcp`；Android 明文 `http://` 测速 URL 会给出 warning，系统默认也会拦截该请求。

高级参数包括 TCP并发线程、测速上限、单 IP 下载测速时间、下载预热时间、GET 分片并发、下载协议、下载缓冲、端口、文件测速URL、追踪 URL、User-Agent、Host Header、SNI、通用请求 Headers、追踪有效状态码、地区码过滤、调试抓包开关和目标等。旧配置中的 `download_count` 仅作为兼容读取字段；`stage_limits.stage3` 仍作为完整模式进入文件测速的候选上限。

### 1.5 DNS 读取页与推送链路

系统配置中提供独立 Cloudflare 配置卡片，包含启用开关、API Token、Zone ID、记录名、TTL、代理开关、备注、Top N 和分流规则；旧配置中的记录类型字段继续兼容读取。

当前实现行为：

- 桌面和移动端都会通过后端调用 Cloudflare 官方 API，读取记录要求完整 API Token 和 Zone ID。
- `listDnsRecords()` 可读取当前 Zone 全部记录、当前配置记录名，或指定子域名/记录名，并可按 A/AAAA 类型筛选后返回 `DnsRecordSnapshot`。
- DNS 读取页只调用读取接口，不创建、更新或删除记录。
- `pushDnsRecords()` 用于手动结果推送、定时任务和测速后自动推送；它会将输入 IP 归一化并按地址族分组，IPv4 覆盖同步 A 记录，IPv6 覆盖同步 AAAA 记录。
- TTL 仅支持 60、300、600 秒三档，默认 300 秒；旧配置中的自动 TTL `1` 会归一化为 300 秒。
- 推送操作不是 dry run；启用前应确认记录名、TTL、代理开关、备注、分流规则和 Top N，避免覆盖生产记录。

### 1.6 配置、档案与更新

系统配置页同时承担应用信息、应用数据目录状态、导出目录、备份还原、在线更新和高级探测参数管理。

| 功能 | 当前行为 | 关键代码 |
| --- | --- | --- |
| 应用数据目录 | 默认使用 `os.UserConfigDir()/CFST-GUI`，旧 `storage_dir` 只用于一次性迁移；支持 `CFST_GUI_PORTABLE_ROOT` 便携模式 | `internal/app/storage.go` |
| 输入源档案 | `source-profiles.json` 保存输入源档案，切换档案会写回当前输入源配置 | `internal/appcore/source_profiles_service.go`、`profiles.go` |
| 配置导入导出 | JSON 导出包含配置快照、source profiles、storage 状态；ZIP 归档内固定包含 `cfst-gui-config.json` | `internal/appcore/config_export_service.go`、`archive_service.go` |
| WebDAV 备份 | 使用 `HEAD` 测试连接、`PUT` 覆盖远端配置包、`GET` 拉取并导入配置包 | `internal/appcore/archive_service.go`、`internal/archivecore/` |
| COLO 字典 | 支持查看字典状态、拉取 Cloudflare geofeed 原始数据和 GitHub 辅助映射源，再本地处理，供输入源 COLO 过滤和 MICS抽样使用；辅助映射源复用在线更新的直连加速候选链 | `internal/app/desktop_colo_dictionary.go`、`internal/colodict/` |
| 在线更新 | 直连检查 GitHub Releases latest，下载/安装前读取 `cfst-gui-update-manifest.json`，按平台资产下载和安装；读取 manifest 和更新包下载会直连并发尝试 GitHub 加速候选链（`ghproxy.vip`、`gh.3w.pm`、`gh.ddlc.top` 和原始 GitHub Release 地址），全程不读取环境代理，并使用 SHA256 校验结果 | `internal/app/update.go` |

## 2. 运行链路

### 2.1 GUI 启动链路

```text
main()
  -> app.Run(os.Args[1:], runtimeResources())
  -> runGUI()
  -> wails.Run(...)
  -> Bind: App
  -> frontend 通过 App.Invoke / WebUI command API / Capacitor Invoke 调用共享命令
```

关键点：

- 根目录 `main.go` 是薄入口，资源桥接保留在根目录以维持 `go:embed` 可用路径。
- `internal/app/run.go` 中没有参数时默认进入 `runGUI()`。
- `internal/app/gui.go` 接收根目录注入的 `frontend/dist` 资源，并把 `App` 绑定给 Wails。
- 前端通过 `frontend/src/lib/bridge.ts` 封装 Wails 调用和事件监听，优先使用 `window.go.app.App`，并兼容 `window.go.main.App`。

### 2.2 桌面探测链路

```text
App.vue launchProbe()
  -> bridge.startProbe(payload)
  -> App.Invoke("probe.start", payloadJSON)
  -> appcore.Service.StartProbe(...)
  -> appcore.Service 准备输入源
  -> emitter: probe.preprocessed(stage0_pool)
  -> appcore.Service.RunProbe()
  -> stage1_tcp: task.NewPing().Run().FilterDelay().FilterLossRate()，丢包率默认 <=15%，最高可配置到 <=100%
  -> stage2_trace: task.TestTraceAvailability()
  -> stage3_get: task.TestDownloadSpeed(...) 仅 full 策略运行
  -> appcore.Service 生成 CSV、结果与任务快照
  -> emitter: probe.partial_export
  -> 测速后 Cloudflare/GitHub 推送与通知（启用时）
  -> emitter: probe.completed
  -> bridge 缓存 taskSnapshots / taskResults
  -> App.vue 刷新看板、当前结果页和自动推送/导出状态
```

MICS 抽样使用 TCP 上的 HTTPS 探测，并保留 `TotalMS`、`ConnectMS`、`TLSMS`、`TTFBMS`、COLO 和前缀成功统计。全局测速端口为 443 时，主流程直接把这些结果作为阶段 1 测量，按 `TotalMS` 初排并跳过同一候选的重复 TCP 测速；自定义端口仍执行对应端口的 TCP 测速。普通 IP 池和 MICS 候选均对 IPv6 按 `/64` 去重，每个 `/64` 只保留一个候选。若 TCP 阶段没有可用候选，流程会在 `stage1_tcp` 明确结束；若追踪阶段没有候选，则在 `stage2_trace` 结束且不启动文件测速。

任务从输入源准备开始持有同一个可取消上下文。前端调用 `stopProbe({mode: "cancel"})` 后，URL/DNS/MICS 输入准备、TCP/追踪/下载请求、重试与冷却等待、CSV 导出、测速后推送都会观察终止信号；终止路径发出 `probe.cancelled`，不会继续发出 `probe.completed` 或按 `probe.failed` 发送失败通知。`mode=pause` 会在输入源、探测、导出、推送提供方、结果持久化和终态提交的安全点阻塞，重试与冷却的剩余等待时间在暂停期间不会继续消耗；Cloudflare 覆盖推送等不可回滚写操作会完成当前安全步骤后再进入暂停。

### 2.3 输入源预览和抓取链路

```text
App.vue inspectSource()
  -> previewSource()
  -> Invoke("source.preview")
  -> appcore.Service.inspectSource()
  -> appcore.LoadSourceContentContext()
  -> appcore.BuildSourceEntriesWithConfig()
  -> 可选 MICS抽样搜索
  -> 返回 preview_entries、source_status、summary
```

共享命令仍保留 `source.fetch`：它会持久化 `last_fetched_at`、`last_fetched_count` 和 `status_text`。当前前端只暴露“预览”入口（`previewSource()`），`source.fetch` 保留给外部调用与后续 UI；`preview` 请求里 `persist_state` 为真时同样会持久化这组状态。

远程 URL 输入源在预览、抓取和测速任务准备阶段都会直连读取，不使用 `HTTP_PROXY`、`HTTPS_PROXY` 或 `NO_PROXY` 等环境代理变量，并对网络错误、读取错误、`429` 或 `5xx` 做一次重试。GitHub Raw 地址可在输入源卡片中切换为 jsDelivr CDN；Raw 读取失败时也会自动尝试等价的 jsDelivr 地址。本地文件和远程 HTTP 输入源都按 32MiB 上限读取，超出后直接失败而不是截断。

预览和抓取的主要区别：

| 操作 | 是否持久化状态 | 典型用途 |
| --- | --- | --- |
| 预览 | 默认不持久化，除非 `persist_state` 为真 | 检查来源是否可读、候选是否正确 |
| 抓取 | 持久化 `last_fetched_at`、`last_fetched_count`、`status_text` | 更新输入源读取状态 |
