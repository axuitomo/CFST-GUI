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
| 输入组 | 输入组 bridge 函数 | `source_profiles.load/save/update_current/save_store/switch/delete` | 共享业务命令 | 管理 `source-profiles.json` |
| 输入源预览 | `previewSource()` | `source.preview`、`source.fetch` | 共享业务命令 | 读取 URL、文件或手动输入，返回候选预览和状态；UI 只暴露 `source.preview` |
| COLO 字典 | COLO bridge 函数 | `colo.status`、`colo.update`、`colo.process` | 共享业务命令 | 管理远程与本地 COLO 字典 |
| 探测任务 | `startProbe()`、`stopProbe()`、`resumeProbe()` | `probe.start/run/pause/cancel/resume` | 桌面/WebUI 异步 start，Android 前台服务同步 run | 暂停与终止是独立命令，事件统一推送到 `probe:event` |
| 任务历史与结果 | 任务 bridge 函数 | `task.get`、`task.list`、`task.results` | 共享业务命令 | 任务快照和结果持久化在 `tasks/`；`task.results` 对流式读取的持久化 JSON 或 CSV 做筛选分页，支持进程重建恢复 |
| 结果历史 | `mergeResultHistoryBatch()`、`queryResultHistory()`、`importResultHistory()` | `history.merge_batch`、`history.import`、`history.query` | 共享业务命令 | 跨批次历史持久化在 `result-history.json`(近 30 天、最多 12000 条,超出按测速时间保留最新),合并键为「批次 + IP + 端口 + 阶段」;旧版浏览器 `localStorage` 历史在首启时一次性导入后删键 |
| DNS 读取与推送 | `listDnsRecords()`、`pushDnsRecords()` | `cloudflare.list`、`cloudflare.push`、`dns.push` | 共享业务命令 | 读取所选平台（Cloudflare、DNSHE、阿里云 DNS、DNSPod）的记录，或按所选平台推送结果；所有平台均支持分流规则；读取时 `provider_id` 可省略以沿用旧 Cloudflare 路径；`scope` 支持 `zone`/`configured`/`routes`/`custom`，返回 `scope`、`records`，能确定时附带 `zone_name`；`cloudflare.*` 为兼容入口 |
| DNSHE 子域名 | `listDnsSubdomains()` | `dns.subdomains`、`dnshe.subdomains` | 共享业务命令 | 读取指定 DNSHE 平台的子域名（`id/subdomain/rootdomain/full_domain/status`），供设置页可视化选择；凭据不完整时返回 `DNS_CONFIG_INVALID` |
| DNS 平台自检 | `runDnsSelfTest()` | `dns.selftest` | 共享业务命令 | 只在 `cfst-selftest.<推送记录名>` 上验证读取、上传、幂等、换血、TTL 更新与 CNAME 阻塞替换，结束清理并复核非托管记录 |
| DNS 解析与 ECH | `testDoH()` | `doh.test` | 共享业务命令 | 用当前 DoH 端点解析测试主机，并按手填、来源主机、ECH 域名的顺序取 ECHConfigList；端点失败不回退系统 DNS，避免把坏端点测成可用 |
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

输入组使用 `SourceProfileItem.ip_mode` 保存组模式（`traverse` / `mcis`），当前配置对应顶层 `source_ip_mode`。`source_profiles.save` / `update_current` 接受可选 `ip_mode`；更新已有档案时省略该字段会继承原组模式。切换或删除当前组时，来源列表与组模式一起更新。`source.preview` / `source.fetch` 及测速入口均以 `config.source_ip_mode` 为准，显式组模式覆盖来源内的兼容字段 `ip_mode`。旧混合模式可保留为空，直到用户明确统一。

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
| `note` | 结果备注列内容，来自结果 CSV 的「备注」列。探测流程不写这一列（没测到速率的 IP 一律丢弃），只有读回历史或手改的 CSV 时才可能有值 |
| `test_port` | 当前行实际测试端口；无行级端口时为本次任务端口 |

## 2. WebUI HTTP API

Linux WebUI 通过 `go build -tags webui` 或 `scripts/build/build-release.sh linux|linux-amd64|linux-arm64` 构建。Release bundle 提供 `linux/amd64` 与 `linux/arm64` 两种产物；Docker Compose 默认监听 `0.0.0.0:34115`，`run-local.sh` 默认监听 `127.0.0.1:34115`。鉴权由 `CFST_WEBUI_TOKEN` 控制，详见 [Docker 与环境变量](../guide/docker-env.md)。

`POST /api/command/{command}` 与 `POST /api/platform/{method}` 必须携带 `Content-Type: application/json`（可带 charset）；缺失或使用表单类型返回 HTTP 415，跨来源浏览器写请求返回 HTTP 403。无令牌模式还要求 API 请求的 Host 为回环地址，避免域名重绑定；远程或反向代理部署必须配置令牌，代理需保留正确 Host。携带令牌的自动化 JSON 请求可以不发送 Origin，正常同源 SPA 无需额外配置。

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

前端 HTTP 收到 401 后会刷新令牌，并用新令牌重建现有 SSE；无订阅者时不重建连接。

`/api/command/{command}` 当前按稳定分域 ID 分发这些能力：

| 能力 | command |
| --- | --- |
| 配置与储存 | `config.load/save/export/backup`、`draft.load/save/discard`、`storage.set/health` |
| 配置归档与 WebDAV | `archive.export/import`、`webdav.test/backup/restore` |
| 输入组 | `source_profiles.load/save/update_current/save_store/switch/delete` |
| 输入源与 COLO 字典 | `source.preview/fetch`、`colo.status/update/process` |
| 桌面拖放导入 | `sources.dropped.consume`（仅桌面 App 层；返回待导入路径并清空，WebUI 返回空列表） |
| 探测任务与结果 | `probe.start/run/pause/cancel/resume`、`task.get/list/results` |
| 结果历史 | `history.merge_batch`、`history.import`、`history.query` |
| DNS、GitHub 与 DNS 解析 | `cloudflare.list/push`、`dns.push`、`dns.subdomains`、`github.test/export`、`doh.test` |
| 导出、通知和诊断 | `results.export_csv`、`telegram.test`、`diagnostics.export`、`debug.export`、`runtime.status` |
| 调度状态 | `scheduler.status` |

`task.results` 的真实任务 ID 以该任务的持久化 JSON 为准：文件存在时，即使结果为空、筛选后为空或分页越界，也不回退到 CSV。只有持久化结果缺失时，才按该任务快照的 `export_record.source_path`、`target_dir/file_name` 顺序读取 CSV；请求中的任意 `path` 不会覆盖任务来源。空任务 ID 或 `result-file` 表示显式文件模式，兼容原路径别名；相对路径以 exports 根目录解析。前端查询仅发送 `task_id`、单一 `path`、排序、IP 筛选和分页参数，不附带配置或重复查询。

平台能力通过 `/api/platform/{command}` 单独处理。未知业务命令返回 HTTP 200 的 `COMMAND_UNKNOWN`；未知平台命令返回 HTTP 404 的 `PLATFORM_COMMAND_UNKNOWN`。

文件 API 的允许根目录来自 `/data`、当前 `storageRoot()` 和 `CFST_WEBUI_ALLOWED_ROOTS`。路径转为绝对路径后，通过 Go `os.OpenInRoot` 在允许根内打开；文件列表和下载均使用该受限句柄，拒绝软链接、junction 等指向根外的路径。

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
| `loadSourceProfiles()`、`saveSourceProfile()`、`updateCurrentSourceProfile()`、`saveSourceProfileStore()`、`switchSourceProfile()`、`deleteSourceProfile()` | Source Profile 方法 | 输入组管理；`updateCurrentSourceProfile()` 会更新 active，缺失时新建 |
| `previewSource()` | 输入源方法 | 预览单个输入源（`source.fetch` 命令仍可用，当前 UI 未暴露） |
| `loadColoDictionaryStatus()`、`updateColoDictionary()`、`processColoDictionary()` | COLO 字典方法 | 字典状态、更新和本地处理 |
| `startProbe()`、`stopProbe()`、`resumeProbe()`、`listenToProbeEvents()` | 探测任务方法和事件通道 | 启动、暂停、继续任务并监听进度 |
| `listTaskSnapshots()`、`getTaskSnapshot()`、`listTaskResults()` | 三端任务快照与结果方法 | 读取持久化任务历史、恢复最新任务，并对当前结果排序和过滤 |
| `mergeResultHistoryBatch()`、`queryResultHistory()`、`importResultHistory()` | 结果历史方法 | 结果落地后写入历史批次,按需读取展示用的历史镜像,并在启动时一次性导入旧版浏览器 `localStorage` 历史 |
| `listDnsRecords()`、`pushDnsRecords()` | DNS 读取/推送方法 | 按所选平台读取 DNS 记录；后台推送链路覆盖推送 A/AAAA 记录 |
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
| `note` | 结果备注列内容，来自 CSV 的「备注」列；探测流程不写这一列（没测到速率的 IP 一律丢弃），只有读回历史或手改的 CSV 时才可能有值 |
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
| `probe.progress` | `stage`、`processed`、`passed`、`failed`、`total`，可选 `qualified_target` | TCP、追踪或文件测速阶段进度。`processed` 是已处理数、`total` 是本阶段待处理总数；文件测速设了「达到成功 IP 数后停止」时额外带 `qualified_target`（该配额），此时候选总数跑不满，界面进度按 `passed / qualified_target` 计算，`total` 仍是候选总数 |
| `probe.stage_summary` | `stage`、`counts`（`input`、`total`、`passed`、`failed`）、`duration_ms`、`reasons`（`reason`、`label`、`count`） | 每个探测阶段结束时下发一条汇总。`reasons` 是该阶段按淘汰原因聚合的条数（已翻译成中文 `label`，按条数降序），不受 debug 开关影响；看板的「阶段汇总」条目用它把漏斗和淘汰原因一次说清，并用它对齐该阶段的最终计数 |
| `probe.speed` | `stage`、`ip`、`current_speed_mb_s`、`current_ready`、`average_speed_mb_s`、`average_ready`、`bytes_read`、`elapsed_ms`、`colo` | 阶段 3 文件测速实时速率样本；`current_speed_mb_s` 是最近 1 秒滑动窗口的速率（采样间隔长于 1 秒时按采样间隔），`average_speed_mb_s` 是预热后累计速率；ready 为 false 时前端显示 `-`，`average_ready=true` 表示已有预热后的有效测量窗口 |
| `stage.detail` | `stage`、`ip`、`reason`、`get.segment_id`、`get.range_start`、`get.range_end`、`get.reconnect_reason` | 调试日志事件；阶段 3 会记录 Range 分片、协议、GET 并发和续连原因 |
| `probe.partial_export` | `target_path`、`written` | CSV 已写出 |
| `probe.completed` | `exported`、`failed`、`passed`、`result_count`、`target_path`、`failure_summary`、`task_context` | 任务完成 |
| `probe.cancelled` | `message`、`stage`、可选 `debug_log_path` | 用户主动终止后的独立终态；任务快照 `status` 为 `cancelled`，不会再发送完成或失败事件 |
| `probe.failed` | `message`、`recoverable` | 任务失败 |
| `probe.cooling` | `reason`、`recoverable`、`stage`、`ip` | 暂停或终止请求的过渡事件；`recoverable=true` 表示可继续，`false` 表示正在退出且不可恢复 |
| `probe.edge_rejected` | `items`、`count`、`batched`，以及照最后一条填的同名单条字段 `stage`、`ip`、`note`、`reason`、`ignored`、`status_code`、`message` | 403 / 1034 淘汰事件，按 250 ms 合并窗口批量下发：`items` 是本窗口内的全部明细（每条字段同单条形态），`count` 为条数，`batched=true` 表示本条不止一颗 IP。顶层单条字段照最后一条填，只读旧形态的消费方仍能拿到一个值，但按条数统计必须用 `count`。`note` 为 `403` / `1034`，`reason` 为 `forbidden` / `edge_ip_restricted`，`ignored` 恒为 `true`（403 / 1034 都直接跳过，不重试、不计冷却）。`stage` 是 `stage2_trace`（追踪阶段就认出 1034，提前淘汰）或 `stage3_get`（文件测速）。与 `stage.reject` 同源但不受 debug 开关约束，界面进程面板与 SSE 都能收到 |

`probe.progress.stage` 当前可能为：

| 阶段 | 说明 |
| --- | --- |
| `stage0_pool` | IP池完成，随 `probe.preprocessed` 推送 |
| `stage0_mcis` | MICS 候选抽样；进度按已完成探测数 / 抽样预算计算 |
| `stage1_tcp` | TCP测延迟 |
| `stage2_trace` | 追踪探测 |
| `stage3_get` | 文件测速；设了成功 IP 配额时按 `passed / qualified_target` 算进度 |
