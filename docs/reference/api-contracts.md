# 接口契约（后端命令、WebUI API、Bridge 与事件）

> 本页属于 [参考文档](./README.md) 的一部分，集中说明三端共享的命令契约、WebUI HTTP API、前端 Bridge 和 `probe:event` 事件接口。功能与运行链路见 [功能与运行链路](./capabilities-and-flows.md)。

## 1. 后端能力接口

桌面、WebUI 和 Android 共享稳定命令 ID 与 JSON 契约。Wails 使用 `App.Invoke(command, payloadJSON)`，WebUI 使用 `POST /api/command/{command}`，gomobile 使用 `mobileapi.Service.Invoke(command, payloadJSON)`。窗口、托盘、SAF、WorkManager、电池、通知权限和更新安装由对应平台适配器处理，不进入业务核心。

### 1.1 统一返回结构

三端业务命令统一返回 `appcore.CommandResult`，Android native 层返回其 JSON 编码，前端 bridge 只做传输解析与类型归一化：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `code` | `string` | 业务状态码 |
| `data` | `any` | 返回数据 |
| `message` | `string` | 用户可读消息 |
| `ok` | `boolean` | 是否成功 |
| `schema_version` | `string` | 固定为 `cfst-gui-command-v2` |
| `task_id` | `string \| null` | 关联任务 ID |
| `warnings` | `string[]` | 非致命警告 |

配置文件 envelope 使用 `cfst-gui-config-v2`，probe 事件 envelope 使用 `cfst-gui-event-v2`。命令和事件字段统一使用 snake_case。

### 1.2 三端能力矩阵

| 能力 | 前端 bridge | 共享命令 / 平台入口 | 三端行为 | 说明 |
| --- | --- | --- | --- | --- |
| 应用信息与更新 | `getAppInfo()`、更新相关函数 | Wails/WebUI platform API；Android 原生分流 | 平台适配 | 读取版本、检查并安装匹配资产或打开 Release 页 |
| 配置 | `loadConfig()`、`saveConfig()` 和草稿函数 | `config.load`、`config.save`、`draft.load/save/discard` | 配置读写共享；草稿仅桌面/WebUI | 保留 `desktop-config.json`、`mobile-config.json` 文件名，内容统一为 v2 |
| 应用数据目录 | `setStorageDirectory()`、`checkStorageHealth()` | `storage.set`、`storage.health` | 平台适配 | Android 私有目录固定；SAF 不作为运行时存储镜像 |
| 配置导出、归档与 WebDAV | 对应 bridge 函数 | `config.export`、`config.backup`、`archive.export/import`、`webdav.test/backup/restore` | 共享业务命令 | 归档固定包含 `cfst-gui-config.json`，导入前备份当前配置 |
| GitHub 结果导出 | `testGitHubExport()`、`exportResultsToGitHub()` | `github.test`、`github.export` | 共享业务命令 | 测试仓库写入配置并推送结果 |
| 自动调度 | `loadSchedulerStatus()` | `scheduler.status`；Android 另由系统触发 `scheduler.refresh/run` | 核心产生状态与执行结果，平台只驱动触发 | 支持 saved、draft、draft_preferred、payload 配置来源和 profile 更新动作 |
| 输入源档案 | 输入源档案 bridge 函数 | `source_profiles.load/save/update_current/save_store/switch/delete` | 共享业务命令 | 管理 `source-profiles.json` |
| 输入源预览 | `previewSource()` | `source.preview`、`source.fetch` | 共享业务命令 | 读取 URL、文件或手动输入，返回候选预览和状态；UI 只暴露 `source.preview` |
| COLO 字典 | COLO bridge 函数 | `colo.status`、`colo.update`、`colo.process` | 共享业务命令 | 管理远程与本地 COLO 字典 |
| 探测任务 | `startProbe()`、`stopProbe()`、`resumeProbe()` | `probe.start/run/pause/cancel/resume` | 桌面/WebUI 异步 start，Android 前台服务同步 run | 暂停与终止是独立命令，事件统一推送到 `probe:event` |
| 任务历史与结果 | 任务 bridge 函数 | `task.get`、`task.list`、`task.results` | 共享业务命令 | 任务快照和结果持久化在 `tasks/`；`task.results` 对流式读取的持久化 JSON 或 CSV 做筛选分页，支持进程重建恢复 |
| Cloudflare DNS | `listDnsRecords()`、`pushDnsRecords()` | `cloudflare.list`、`cloudflare.push` | 共享业务命令 | 读取记录或从手动、调度、探测后流程推送结果 |
| 路径选择/打开 | `selectPath()`、`openPath()` | Wails/WebUI platform API 或 Android SAF/Intent | 平台适配 | 不进入 `appcore` 业务命令路由 |

### 1.3 平台绑定方法

| Wails `App` 方法 | 当前用途 |
| --- | --- |
| `GetHealth()` | Wails 侧健康信息，返回服务名、版本、配置路径、schema、transport |
| `Invoke(command, payloadJSON)` | 唯一共享业务入口 |
| `GetAppInfo()`、更新相关方法 | 平台版本与更新能力 |
| `ShowMainWindow()`、`HideMainWindow()`、`QuitApplication()` | 桌面窗口/托盘生命周期辅助；WebUI build tag 下返回不可用提示 |
| `OpenPath()`、`OpenLogDirectory()`、`SelectPath()` | 桌面或 WebUI 平台文件交互 |

gomobile 的 `mobileapi.Service` 只公开 `Init`、`SetEventSink` 和 `Invoke`。旧逐方法业务桥接不再保留 API 别名。

### 1.4 `SourcePreviewRequest`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `config` | `object` | 当前配置快照 |
| `persist_state` | `boolean` | 是否持久化来源读取状态 |
| `preview_limit` | `number` | 返回预览条数上限，默认 16 |
| `source` | `Source` | 单个输入源配置 |

返回 `data` 结构：

| 字段 | 说明 |
| --- | --- |
| `preview_entries` | 预览候选 IP 列表 |
| `source_status` | 更新后的来源状态 |
| `port_summary` | 端口上下文，包含 `global_tcp_port`、`source_port_values`、`current_test_port`、`port_policy` |
| `summary.action` | `预览` 或 `抓取` |
| `summary.invalid_count` | 非法 IP/CIDR/域名数量 |
| `summary.mode` | `traverse` 或 `mcis` |
| `summary.name` | 输入源名称 |
| `summary.total_count` | 完整候选数量 |

### 1.5 `ProbePayload`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `config` | `object` | 配置快照 |
| `sources` | `Source[]` | 输入源数组 |
| `task_id` | `string` | 前端生成的任务 ID，缺失时后端生成 |
| `android_export_uri` | `string` | Android SAF 导出 URI，桌面/WebUI 通常为空 |

### 1.6 `ProbeRunResult`

| 字段 | 说明 |
| --- | --- |
| `config` | 本次执行归一化后的 `ProbeConfig` |
| `durationMs` | 总耗时，毫秒 |
| `outputFile` | CSV 输出路径，可能为空 |
| `results` | `ProbeRow[]` |
| `source` | 输入源解析统计 |
| `sourceStatuses` | 输入源最新状态 |
| `startedAt` | 任务开始时间 |
| `summary` | 汇总统计 |
| `task_context` / `taskContext` | 本次任务上下文，包含配置来源、全局端口、源端口列表、当前测试端口和端口策略 |
| `warnings` | 非致命警告 |
| `schemaVersion` | Go 后端 schema |

`ProbeRow` 字段：

| 字段 | 说明 |
| --- | --- |
| `ip` | IP 地址 |
| `sended` | 发送次数 |
| `received` | 成功次数 |
| `lossRate` | 丢包率，范围 0 到 1 |
| `delayMs` | TCP 平均延迟，毫秒 |
| `traceDelayMs` | 追踪延迟，毫秒 |
| `downloadSpeedMb` | 平均下载速率，MB/s |
| `maxDownloadSpeedMb` | 最高下载速率，MB/s |
| `colo` | 地区码，缺失时前端展示为 `N/A` |
| `test_port` | 当前行实际测试端口；无行级端口时为本次任务端口 |

## 2. WebUI HTTP API

Linux WebUI 通过 `go build -tags webui` 或 `scripts/build/build-release.sh linux|linux-amd64|linux-arm64` 构建。Release bundle 提供 `linux/amd64` 与 `linux/arm64` 两种产物；Docker Compose 默认监听 `0.0.0.0:34115`，`run-local.sh` 默认监听 `127.0.0.1:34115`。鉴权由 `CFST_WEBUI_TOKEN` 控制，详见 [Docker 与环境变量](../guide/docker-env.md)。

| 路由 | 方法 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `/api/health` | `GET` | 不需要 | 返回 `ok`、`service`、`version`、`auth_required` |
| `/api/command/{command}` | `POST` | 需要 | 读取 JSON payload，并调用 `App.Invoke(command, payloadJSON)` |
| `/api/platform/{command}` | `POST` | 需要 | 分发 WebUI 平台能力，例如应用信息、更新和打开日志目录 |
| `/api/events/probe` | `GET` | 需要 | SSE 流，逐条发送 `probe:event` envelope JSON |
| `/api/files/list?path=...` | `GET` | 需要 | 列出允许根目录内的文件，返回 `entries`、`path`、`roots` |
| `/api/files/download?path=...` | `GET` | 需要 | 下载允许根目录内的单个文件 |
| `/` | `GET`/`HEAD` | 不需要 | 返回嵌入的 `frontend/dist` SPA；找不到静态文件时回退到首页 |

WebUI 鉴权支持 `Authorization: Bearer <token>`，SSE 和下载场景也兼容 `?token=<token>`。如果未设置 `CFST_WEBUI_TOKEN`，受保护 API 会跳过鉴权。

`/api/command/{command}` 当前按稳定分域 ID 分发这些能力：

| 能力 | command |
| --- | --- |
| 配置与储存 | `config.load/save/export/backup`、`draft.load/save/discard`、`storage.set/health` |
| 配置归档与 WebDAV | `archive.export/import`、`webdav.test/backup/restore` |
| 输入源档案 | `source_profiles.load/save/update_current/save_store/switch/delete` |
| 输入源与 COLO 字典 | `source.preview/fetch`、`colo.status/update/process` |
| 探测任务与结果 | `probe.start/run/pause/cancel/resume`、`task.get/list/results` |
| Cloudflare 和 GitHub | `cloudflare.list/push`、`github.test/export` |
| 导出、通知和诊断 | `results.export_csv`、`telegram.test`、`diagnostics.export`、`debug.export`、`runtime.status` |
| 调度状态 | `scheduler.status` |

平台能力通过 `/api/platform/{command}` 单独处理。未知业务命令返回 HTTP 200 的 `COMMAND_UNKNOWN`；未知平台命令返回 HTTP 404 的 `PLATFORM_COMMAND_UNKNOWN`。

文件 API 的允许根目录来自 `/data`、当前 `storageRoot()` 和 `CFST_WEBUI_ALLOWED_ROOTS`。路径会转成绝对路径并校验必须位于允许根内，避免浏览器任意读取宿主文件。

## 3. 前端 Bridge 接口

前端 bridge 文件是 `frontend/src/lib/bridge.ts`。它负责：

- 在 Wails、WebUI、Android native 三种运行时之间选择正确后端：挂载前由 `resolveBridgeMode()` 定好通道（宿主身份优先：Capacitor 原生壳 → Wails 运行时/宿主地址（`wails.localhost`、`wails:`）→ 其余按 WebUI 处理），视图与组件不得自行判断。
- 校验并归一化 Wails/WebUI/Capacitor 返回值。
- 将 Go 结构转换为 UI 更容易消费的数据结构。
- 维护当前任务的前端缓存，并通过持久化任务 API 在启动时恢复最新快照和结果。
- 监听 `probe:event` Wails 事件、WebUI SSE 或 Capacitor listener 并分发给页面。

### 3.1 统一 `CommandResult<T>`

```ts
interface CommandResult<T = Record<string, unknown> | null> {
  code: string;
  data: T | null;
  message: string;
  ok: boolean;
  schema_version: string;
  task_id: string | null;
  warnings: string[];
}
```

### 3.2 Bridge 函数

| 函数 | 后端依赖 | 说明 |
| --- | --- | --- |
| `loadConfig()`、`saveConfig(payload)`、`saveDraft(payload)`、`discardDraft()` | 三端配置读写方法，草稿接口以桌面/WebUI 为主 | 读取/保存当前配置快照，并管理未正式保存的桌面草稿 |
| `getAppInfo()`、`checkForUpdates()`、`downloadAndInstallUpdate()`、`openReleasePage()` | 应用信息和更新方法 | 应用元信息、在线更新和 Release 页 |
| `setStorageDirectory()`、`checkStorageHealth()` | 储存目录方法 | 选择、迁移和检查储存目录 |
| `exportConfig()`、`exportConfigArchive()`、`importConfigArchive()`、`backupCurrentConfig()` | 配置导出/归档方法 | JSON/ZIP 配置导入导出；导入 ZIP 前会先本地备份当前配置 |
| `testWebDAV()`、`backupConfigToWebDAV()`、`restoreConfigFromWebDAV()` | WebDAV 方法 | 测试、备份和还原远端配置包 |
| `testGitHubExport()`、`exportResultsToGitHub()` | GitHub 导出方法 | 测试仓库写入配置，并把结果 CSV 推送到配置路径 |
| `loadSchedulerStatus()` | 调度状态方法 | 读取调度器当前状态、下一次触发和最近执行信息 |
| `loadSourceProfiles()`、`saveSourceProfile()`、`updateCurrentSourceProfile()`、`saveSourceProfileStore()`、`switchSourceProfile()`、`deleteSourceProfile()` | Source Profile 方法 | 输入源档案管理；`updateCurrentSourceProfile()` 会更新 active，缺失时新建 |
| `previewSource()` | 输入源方法 | 预览单个输入源（`source.fetch` 命令仍可用，当前 UI 未暴露） |
| `loadColoDictionaryStatus()`、`updateColoDictionary()`、`processColoDictionary()` | COLO 字典方法 | 字典状态、更新和本地处理 |
| `startProbe()`、`stopProbe()`、`resumeProbe()`、`listenToProbeEvents()` | 探测任务方法和事件通道 | 启动、暂停、继续任务并监听进度 |
| `listTaskSnapshots()`、`getTaskSnapshot()`、`listTaskResults()` | 三端任务快照与结果方法 | 读取持久化任务历史、恢复最新任务，并对当前结果排序和过滤 |
| `listDnsRecords()`、`pushDnsRecords()` | Cloudflare DNS 方法 | 读取 DNS 记录；后台推送链路覆盖推送 A/AAAA 记录 |
| `selectPath()`、`openPath()` | 路径选择/打开方法 | 桌面系统选择器、WebUI 文件 API 或 Android SAF |

### 3.3 结果列表字段

前端将后端 `ProbeRow` 归一化为 `ProbeResult`：

| 字段 | 说明 |
| --- | --- |
| `address` | IP 地址 |
| `colo` | 地区码 |
| `download_mbps` | 平均下载速率，数值单位为 MB/s（字段名保留旧拼写以兼容前端缓存） |
| `max_download_mbps` | 最高下载速率，数值单位为 MB/s |
| `export_status` | 当前固定为 `exported` |
| `stage_status` | 当前固定为 `completed` |
| `tcp_latency_ms` | 来自后端 `delayMs`，含义为 TCP 平均延迟 |
| `trace_latency_ms` | 来自后端 `traceDelayMs`，含义为阶段 2 追踪延迟 |
| `test_port` | 当前结果行实际测试端口 |
| `last_error_code` | 当前为 `null` |

支持的排序字段如下，排序只影响当前结果展示顺序，不改变探测结果文件内容。

| 排序字段 | UI 含义 |
| --- | --- |
| `address` | IP 地址 |
| `stage` | 阶段状态 |
| `tcp` | TCP 延迟 |
| `trace` | 追踪延迟 |
| `download` | 平均速率 |
| `max_download` | 最高速率 |
| `export_status` | 导出状态 |

支持的状态过滤项：`all`、`exported`、`pending`、`failed`。

支持的 IP 版本过滤项：`all`、`ipv4`、`ipv6`。IP 版本过滤只影响当前结果展示顺序和可见行，不改变探测结果文件内容。

## 4. 探测事件接口

后端事件通道固定为 `probe:event`。桌面端通过 Wails `EventsEmit` 推送，WebUI 通过 `/api/events/probe` SSE 传递，Android 通过 Capacitor `addListener("probe:event")` 回传。事件 envelope 的 `schema_version` 固定为 `cfst-gui-event-v2`。

### 4.1 事件 Envelope

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `event` | `string` | 事件类型 |
| `payload` | `object` | 事件负载 |
| `schema_version` | `string` | Go 后端 schema |
| `seq` | `number` | 单任务内递增序号 |
| `task_id` | `string` | 任务 ID |
| `ts` | `string` | RFC3339 时间 |

WebUI SSE 为每个 frame 写入稳定的 SSE `id`，服务端保留最近 512 条事件。订阅通道容量为 256；慢客户端积压时会丢弃最旧事件并保留最新事件，避免 `probe.speed` 样本把通道堵死。浏览器断线后由原生 `EventSource` 自动重连，并通过 `Last-Event-ID` 请求断点后的事件；重放窗口不足或客户端检测到 `seq` 跳号时，前端会重新读取任务快照和结果进行对账。跨任务事件、重复事件和无法解析的 frame 会被忽略，不会覆盖当前任务状态。

### 4.2 事件类型

| 事件 | payload | 说明 |
| --- | --- | --- |
| `probe.preprocessed` | `accepted`、`filtered`、`invalid`、`source_statuses`、`stage`、`total` | 输入源预处理完成，`stage` 为 `stage0_pool` |
| `probe.mcis.progress` | `stage`、`source_id`、`source_name`、`completed`、`total`、`succeeded`、`failed`、`candidate_count`、`concurrency`、`elapsed_ms`、`last_ip`、`last_colo`、`last_ok` | MICS 输入源抽样实时进度，`stage` 为 `stage0_mcis`；任务快照通过独立的 `mcis_progress` 字段保存，不覆盖主测速 `progress` |
| `probe.progress` | `stage`、`processed`、`passed`、`failed`、`total` | TCP、追踪或文件测速阶段进度 |
| `probe.speed` | `stage`、`ip`、`current_speed_mb_s`、`current_ready`、`average_speed_mb_s`、`average_ready`、`bytes_read`、`elapsed_ms`、`colo` | 阶段 3 文件测速实时速率样本；ready 为 false 时前端显示 `-`，`average_ready=true` 表示已有预热后的有效测量窗口 |
| `stage.detail` | `stage`、`ip`、`reason`、`get.segment_id`、`get.range_start`、`get.range_end`、`get.reconnect_reason` | 调试日志事件；阶段 3 会记录 Range 分片、协议、GET 并发和续连原因 |
| `probe.partial_export` | `target_path`、`written` | CSV 已写出 |
| `probe.completed` | `exported`、`failed`、`passed`、`result_count`、`target_path`、`failure_summary`、`task_context` | 任务完成 |
| `probe.cancelled` | `message`、`stage`、可选 `debug_log_path` | 用户主动终止后的独立终态；任务快照 `status` 为 `cancelled`，不会再发送完成或失败事件 |
| `probe.failed` | `message`、`recoverable` | 任务失败 |
| `probe.cooling` | `reason`、`recoverable`、`stage`、`ip` | 暂停或终止请求的过渡事件；`recoverable=true` 表示可继续，`false` 表示正在退出且不可恢复 |

`probe.progress.stage` 当前可能为：

| 阶段 | 说明 |
| --- | --- |
| `stage0_pool` | IP池完成，随 `probe.preprocessed` 推送 |
| `stage0_mcis` | MICS 候选抽样；进度按已完成探测数 / 抽样预算计算 |
| `stage1_tcp` | TCP测延迟 |
| `stage2_trace` | 追踪探测 |
| `stage3_get` | 文件测速 |
