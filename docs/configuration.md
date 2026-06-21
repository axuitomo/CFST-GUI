# 配置详解

本文档说明 CFST-GUI 的配置目录、配置文件、主要字段、默认值和敏感信息风险。字段名以当前 `desktop-config.json` 快照格式为准。

## 配置目录

默认配置根目录由 Go 的 `os.UserConfigDir()` 决定，并追加 `CFST-GUI` 子目录。不同系统的实际路径由操作系统决定。

便携模式有两种触发方式：

| 方式 | 行为 |
| --- | --- |
| 设置 `CFST_GUI_PORTABLE_ROOT=/some/root` | 数据目录解析为 `/some/root/data` |
| 可执行文件同目录存在 `portable.json` | 数据目录解析为 `<exe-dir>/data` |

当前版本不再支持在界面中自定义储存目录。桌面端固定使用默认应用数据目录；Android 固定使用 app 私有数据目录。旧版 `storage.json` 中的 `storage_dir` 只用于一次性迁移旧数据到固定目录，之后不再作为运行时路径。

## 文件清单

| 文件或目录 | 默认位置 | 说明 |
| --- | --- | --- |
| `storage.json` | 默认 `CFST-GUI` 配置目录 | 储存 bootstrap；旧 `storage_dir` 只读兼容，用于一次性迁移。 |
| `desktop-config.json` | 固定应用数据目录 | 桌面 GUI 和 WebUI 主要配置快照。 |
| `desktop-draft.json` | 固定应用数据目录 | 桌面 GUI 自动保存草稿，用于恢复未正式保存的设置。 |
| `mobile-config.json` | Android app 私有数据目录 | Android 当前配置快照，结构与桌面配置快照同构。 |
| `config.json` | 固定应用数据目录 | 兼容旧桥接结构的配置文件。 |
| `source-profiles.json` | 固定应用数据目录 | 输入源档案，包含 `active_profile_id` 和 `items[].sources`。 |
| `cfip-log.txt` | 固定应用数据目录 | 开启 debug 后写入的调试日志。 |
| `exports/` | 导出目录或固定应用数据目录下的默认导出目录 | CSV、测速文件和调试日志导出结果。 |
| `imports/` | 固定应用数据目录 | 建议存放导入文件。 |
| `backups/` | 固定应用数据目录 | 本地配置备份归档目录。 |

检测到旧版自定义 `storage_dir` 时，程序会尝试迁移 `desktop-config.json`、`desktop-draft.json`、`config.json`、`cfip-log.txt`、`result.csv`、`source-profiles.json`、`exports/`、`imports/`、`backups/` 和地区数据文件。迁移不会删除旧目录。

## `desktop-config.json` 结构

写入磁盘时，配置外层包含元数据：

```json
{
  "config_snapshot": {},
  "saved_at": "2026-05-08T00:00:00+08:00",
  "schema_version": "cfst-gui-wails-v1"
}
```

`config_snapshot` 内部包含这些顶层字段：

| 字段 | 说明 |
| --- | --- |
| `cloudflare` | Cloudflare DNS 读取、推送和分流配置。 |
| `github` | GitHub 结果导出配置。 |
| `upload` | 统一上传筛选与目标 Top N 配置。 |
| `export` | 本地 CSV 导出目标、文件名和覆盖策略；旧 `export.github` 仍兼容读取。 |
| `post_probe_push` | 手动测速完成后的自动推送勾选项。 |
| `backup.webdav` | WebDAV 配置备份和恢复。 |
| `maintenance` | 运行时维护策略，例如终态任务快照保留天数。 |
| `probe` | 探测策略、并发、阈值、超时、调试等核心参数。 |
| `sources` | 输入源列表。 |
| `scheduler` | 自动任务调度偏好。 |
| `ui` | UI 行为偏好。 |

`desktop-draft.json` 使用相同外层结构。前端会防抖写入草稿；`LoadDesktopConfig` 会返回 `draft_status`，当草稿 `saved_at` 新于正式配置时，界面会提示恢复或丢弃。正式执行 `SaveDesktopConfig` 成功后会清理草稿，避免下次启动重复恢复。

## 旧配置兼容与字段净化

读取 `desktop-config.json`、移动端 `mobile-config.json` 和配置压缩包中的 `config_snapshot` 时，程序会先按当前 schema 生成兼容快照：

- 缺少的新字段会补当前默认值，例如 `backup.webdav`、`probe.retry_policy`、`scheduler` 等旧配置中不存在的字段。
- 已废弃或未知字段会被忽略，不会导致读取失败，也不会出现在返回给前端的 `config_snapshot` 中。
- 常见旧字段别名会迁移到当前字段名，例如 `apiToken` → `api_token`、`remotePath` → `remote_path`、`timeoutSeconds` → `timeout_seconds`、`stageLimits` → `stage_limits`、`cooldownPolicy` → `cooldown_policy`、`retryPolicy` → `retry_policy`、`dailyTimes` → `daily_times`、输入源 `type` → `kind`。
- 旧 `export.github` 会同步到顶层 `github`；旧 `upload.cloudflare.routing_*` 和 `upload.cloudflare.top_n` 会同步到顶层 `cloudflare`。保存时仍会写回兼容结构，避免旧调用链读取不到。
- 旧版 `sourceText` 或 `probe.ipText` 会在没有 `sources` 字段时转换为一个 `inline` 输入源。

兼容读取不会立刻改写磁盘文件。只有保存配置、导入配置归档、WebDAV 备份/还原时间写回、保存或切换输入源档案/策略档案等写入路径会把净化后的当前格式落盘。JSON 语法错误仍会按解析失败处理，兼容逻辑只处理字段缺失、旧别名和未知字段。

## `cloudflare`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `api_token` | 空 | Cloudflare API Token，属于敏感信息；只读取 DNS 可授予 DNS Read，需要推送时授予目标 Zone 的 DNS Edit 权限，详见 [Cloudflare API Token 权限设置教程](./cloudflare-api-token.md)。 |
| `zone_id` | 空 | Cloudflare Zone ID。 |
| `enabled` | `false` | 是否启用 Cloudflare 配置；测速后自动推送需要该项开启。 |
| `record_name` | 空 | 当前配置记录读取和默认推送目标使用的 DNS 记录名。 |
| `record_type` | `A` | 默认记录类型；推送逻辑会按 IP 类型处理，分流规则可单独覆盖。 |
| `proxied` | `false` | 是否开启 Cloudflare proxy。 |
| `ttl` | `300` | DNS TTL，默认 300 秒。 |
| `comment` | 空 | 写入 DNS 记录的备注。 |
| `top_n` | `5` | Cloudflare 目标上传数量；`0` 表示不限。 |
| `routing_enabled` | `false` | 是否启用 Cloudflare 分流规则。 |
| `routing_rules` | `[]` | Cloudflare 分流规则数组；每条规则可按国家/COLO 筛选并推送到指定记录名。 |

DNS 读取页只读取 Cloudflare 记录，不修改线上 DNS。工作流、定时任务和测速后自动推送中的 Cloudflare 推送会复用本配置、共享上传策略、`top_n` 和分流规则。`api_token` 会随配置快照保存；导出配置或备份归档前，需要确认文件不会被提交到仓库或公开分享。

`routing_rules` 中常见字段：

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 是否启用该分流规则。 |
| `name` | 空 | 规则显示名称。 |
| `record_name` | 空 | 该规则要覆盖推送的完整 DNS 记录名。 |
| `record_type` | `A` | 记录类型，支持 `A`、`AAAA`、`ALL`。 |
| `filter_mode` | `allow` | 筛选模式，支持 `allow`、`deny`。 |
| `filter_tokens` | 空 | 国家/COLO 筛选词，逗号分隔。 |
| `top_n` | `5` | 该规则上传数量；`0` 表示不限。 |

## `github`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 是否启用 GitHub 结果导出；测速后自动推送需要该项开启。 |
| `owner` | 当前仓库 origin owner 或 `axuitomo` | GitHub 仓库 owner。 |
| `repo` | 当前仓库 origin repo 或 `CFST-GUI` | GitHub 仓库名。 |
| `branch` | `main` | 目标分支。 |
| `path_template` | `cfst-results/{date}/{time}-{task_id}.csv` | 目标文件路径模板。 |
| `commit_message_template` | `CFST results {date} {time}` | 提交信息模板。 |
| `format` | `csv` | GitHub 导出格式。 |
| `csv_header_template` | 空 | CSV 头模板。 |
| `csv_row_template` | 空 | CSV 行模板。 |
| `txt_row_template` | `{ip}` | TXT 行模板。 |
| `token` | 空 | GitHub PAT，属于敏感信息；推荐使用 fine-grained PAT，并仅授予目标仓库 Contents Read and write，详见 [GitHub PAT 权限设置教程](./github-pat.md)。 |
| `top_n` | `20` | GitHub 目标上传数量；`0` 表示不限。 |
| `last_export_at` | 空 | 最近 GitHub 导出时间。 |

旧 `export.github` 继续兼容读取和保存，但当前 UI 的 GitHub 配置卡片以顶层 `github` 为主。

## `post_probe_push`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `cloudflare_enabled` | `false` | 手动单任务或手动工作流测速完成后，自动执行 Cloudflare 推送。 |
| `github_enabled` | `false` | 手动单任务或手动工作流测速完成后，自动执行 GitHub 导出。 |

`post_probe_push` 不叠加到定时任务；scheduler 和定时工作流会显式禁用这条自动推送入口，避免和原有调度推送重复。手动工作流如果已经包含对应 `deliver_dns` 或 `deliver_github` 节点，本次自动推送会跳过对应 provider。

测速后自动推送会先应用统一上传筛选，再分别使用 Cloudflare / GitHub 的目标 Top N。筛选失败时会返回 warning 并停止后续 provider；GitHub 筛选后没有结果时会跳过导出，不提交空文件，也不会回退到未筛选的完整测速结果。

## `upload`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `shared_filter.enabled` | `false` | 是否启用共享上传筛选。 |
| `shared_filter.status` | `passed` | 上传状态筛选，当前可用 `passed`、`all`。 |
| `shared_filter.ip_version` | `any` | IP 版本筛选，支持 `any`、`ipv4`、`ipv6`。 |
| `shared_filter.colo_allow` | 空 | COLO 白名单，逗号分隔。 |
| `shared_filter.colo_deny` | 空 | COLO 黑名单，逗号分隔。 |
| `shared_filter.max_tcp_latency_ms` | `null` | 最大 TCP 延迟；空表示不限制。 |
| `shared_filter.max_trace_latency_ms` | `null` | 最大追踪延迟；空表示不限制。 |
| `shared_filter.min_download_mbps` | `0` | 最低下载速度；单位 MB/s。 |
| `shared_filter.max_loss_rate` | `null` | 最大丢包率；空表示不限制。 |

统一上传筛选会影响：

- 工作流和定时任务中的 Cloudflare 推送
- 工作流和定时任务中的 GitHub 导出
- 手动 GitHub 结果导出
- 测速后自动推送列表中的 Cloudflare / GitHub

各 provider 的 Top N 已迁移到顶层 `cloudflare.top_n` 和 `github.top_n`；旧 `upload.cloudflare.top_n`、`upload.github.top_n` 仍兼容读取。

Cloudflare 分流或组合推送会区分三类结论：筛选后没有可推送 IP 时为跳过，所有目标写入失败时为失败，部分目标成功时为 `partial`。定时任务状态、上传通知和 Telegram 通知会保留这些 provider 结果。

DNS 读取页不执行推送，因此不走该筛选器。

## `export`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `file_name` | `result.csv` | 默认导出文件名。 |
| `file_name_template` | 空 | 文件名模板；支持 `{date}`、`{time}`、`{profile}`、`{task_id}`。 |
| `format` | `csv` | 当前导出格式。 |
| `overwrite` | `replace_on_start` | 覆盖策略；值为 `append` 时追加写入。 |
| `target_dir` | 空 | 桌面/WebUI 文件系统导出目录；空时使用默认导出目录。 |
| `target_uri` | 空 | Android SAF 导出目录 URI；Android 导出 CSV、测速文件和调试日志前需要选择该目录。 |

文件名会经过路径非法字符清理，避免把 `/`、`\`、`:`、`*`、`?`、`"`、`<`、`>`、`|` 写入文件名。

`target_dir` 和 `target_uri` 属于当前设备的本地导出目标，不参与跨设备同步。导入配置压缩包或从 WebDAV 还原时会保留当前设备已有的导出目标；如果当前设备尚未设置，则保持为空，由桌面/WebUI 回退默认导出目录，Android 则要求重新选择 SAF 导出目录。

GitHub 结果导出配置已经独立到顶层 `github`；`export.github` 作为兼容镜像保留。

## `backup.webdav`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 是否启用 WebDAV 备份。 |
| `server_url` | 空 | WebDAV 服务地址。 |
| `username` | 空 | WebDAV 用户名。 |
| `password` | 空 | WebDAV 密码或 Token，属于敏感信息。 |
| `remote_path` | `cfst-gui-config.zip` | 远端备份文件路径。 |
| `timeout_seconds` | `30` | WebDAV 请求超时。 |
| `last_backup_at` | 空 | 最近 WebDAV 备份时间，由后端写回。 |
| `last_restore_at` | 空 | 最近 WebDAV 还原时间，由后端写回。 |

## `maintenance`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `completed_task_retention_days` | `7` | 终态任务快照和结果文件保留天数；适用于 `completed`、`failed` 和 `no_results`。设为 `0` 时关闭自动清理。 |

## `probe`

### 策略与阶段

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `strategy` | `fast` | `fast` 跳过文件测速；`full` 执行文件测速。兼容值 `latency`、`http-colo` 会归一化为 `fast`，`speed`、`exhaustive` 会归一化为 `full`。 |
| `disable_download` | `true` | 默认禁用下载测速；`strategy=full` 时会关闭该项。 |
| `stage_limits.stage1` | 兼容旧值 | 旧配置兼容字段；新保存配置不主动写入，后端不再按该字段截断阶段 1 TCP 候选。 |
| `stage_limits.stage2` | 兼容旧值 | 旧配置兼容字段；新保存配置不主动写入，后端不再按该字段截断阶段 2 追踪候选。 |
| `stage_limits.stage3` | `10` | 阶段 3 文件测速候选上限。 |

### 并发与采样

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `concurrency.stage1` | `200` | TCP 延迟测速并发，最大 `1000`。 |
| `concurrency.stage2` | `30` | 追踪探测并发，最大 `30`。 |
| `concurrency.stage3` | `1` | 文件测速阶段并发，当前最大 `1`。 |
| `ping_times` | `4` | 单个 IP TCP 发包次数，最少 `2`。 |
| `skip_first_latency_sample` | `true` | 是否跳过首个延迟样本。 |
| `event_throttle_ms` | `100` | 进度事件推送节流。 |
| `download_speed_sample_interval_ms` | `500` | 下载测速速度采样间隔。 |

### 下载与网络

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `download_count` | `10` | 兼容旧字段；当前主要作为阶段 3 上限来源之一。 |
| `download_get_concurrency` | `4` | 单 IP 下载 GET 并发，范围 `1` 到 `32`。 |
| `download_buffer_kb` | `256` | 下载缓冲区，范围 `64` 到 `4096` KB。 |
| `download_http_protocol` | `auto` | 下载 HTTP 协议，可用 `auto`、`tcp`、`h1`、`h2`、`h3`。 |
| `download_speed_metric` | `average` | 下载速率依据，可用 `average` 或 `max`；仅影响最低下载速度阈值和结果显示数量 Top N 评分。 |
| `download_time_seconds` | `4` | 单 IP 下载测速时长。 |
| `download_warmup_seconds` | `1` | 下载测速预热时长。 |
| `tcp_port` | `443` | TCP 延迟和下载测速端口。 |
| `port_policy` | `source_override_global` | 输入源端口优先策略；当输入源行包含单一端口时，本次任务使用该端口，否则回退 `tcp_port` 并输出 warning。 |
| `url` | `https://speed.cloudflare.com/__down?bytes=10000000` | 文件测速 URL。 |
| `trace_url` | 空 | 追踪探测 URL；空时可从文件测速 URL 推导 `/cdn-cgi/trace`。 |
| `user_agent` | 内置 Firefox UA | 请求 User-Agent。 |
| `host_header` | 空 | 强制覆盖 Host 头。 |
| `sni` | 空 | 强制覆盖 TLS SNI。 |
| `request_headers` | 空 | 作用于追踪探测和文件测速的多行请求头，每行 `Header-Name: value`；`Host`、`User-Agent`、`Range` 等保留头会被忽略。 |

### HTTPing、阈值与输出

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `httping` | `false` | 是否使用 HTTPing 延迟模式。 |
| `httping_status_code` | `0` | 追踪有效状态码；`0` 表示不按状态码筛选，设置 `100-599` 才启用精确状态码过滤。 |
| `httping_cf_colo` | 空 | 按地区码过滤。 |
| `thresholds.max_tcp_latency_ms` | `null` | TCP 延迟上限；空时使用内部默认 `9999` ms。 |
| `thresholds.max_http_latency_ms` | `null` | HTTP 追踪延迟上限；空时不限制。 |
| `thresholds.min_download_mbps` | `0` | 下载速度下限，单位 MB/s；按 `download_speed_metric` 选择平均速率或最高速率判定。 |
| `min_delay_ms` | `0` | TCP 延迟下限。 |
| `max_loss_rate` | `0.15` | 丢包率上限，最大 `1.00`。 |
| `print_num` | `0` | 结果显示数量；`0` 表示不限制，正数按 30% 延迟 + 70% 下载速率的归一化加权评分筛选最终 Top N，速率依据同 `download_speed_metric`。 |
| `test_all` | `false` | 桌面配置中固定归一化为 `false`。 |

### 超时、重试和调试

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `timeouts.stage1_ms` | `1000` | TCP 阶段超时。 |
| `timeouts.stage2_ms` | `1000` | 追踪阶段超时。 |
| `timeouts.stage3_ms` | `10000` | 文件测速阶段超时。 |
| `retry_policy.max_attempts` | `0` | 重试次数；`0` 表示不开启额外重试。 |
| `retry_policy.backoff_ms` | `0` | 重试退避时间。 |
| `cooldown_policy.consecutive_failures` | `3` | 连续失败冷却阈值。 |
| `cooldown_policy.cooldown_ms` | `250` | 冷却时间。 |
| `debug` | `false` | 是否开启调试日志。 |
| `debug_capture_enabled` | `false` | 是否启用调试抓包目标。 |
| `debug_capture_address` | 空 | 调试抓包地址。 |
| `debug_log_mode` | `structured` | 日志模式，可用 `structured`、`freeform`。 |
| `debug_log_format` | 空 | `freeform` 模式模板；空时使用默认 `{ts} [{level}] {event} task={task_id} stage={stage} {message}`。 |
| `debug_log_verbosity` | `detailed` | 日志记录粒度，可用 `simple`、`detailed`；`simple` 只保留任务启动、阶段完成、导出和最终状态。 |

`cfip-log.txt` 只会在 `probe.debug=true` 时持续写入。Android 原生插件层的 bridge、储存同步和事件 fallback 异常会额外写入系统 `Logcat`（tag: `CfstPlugin`），这部分不依赖 `probe.debug`。

## `sources`

每个输入源支持以下字段：

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `id` | `source-1` | 输入源 ID。 |
| `name` | `输入源 1` | 输入源名称。 |
| `kind` | `url` | 输入源类型，常见为 URL、本地文件或手动内容。 |
| `url` | 空 | 远程输入源 URL。 |
| `path` | 空 | 本地输入源路径。 |
| `content` | 空 | 手动输入内容。 |
| `enabled` | `true` | 是否参与探测。 |
| `ip_limit` | `500` | 单个输入源候选 IP 上限。 |
| `ip_mode` | `traverse` | `traverse` 顺序展开；`mcis` 使用 MICS 抽样。 |
| `colo_filter` | 空 | 地区过滤条件。 |
| `last_fetched_at` | 空 | 最近抓取时间。 |
| `last_fetched_count` | `0` | 最近抓取候选数量。 |
| `status_text` | 空 | 输入源状态文本。 |

输入源内容支持在单条候选中携带端口：`1.1.1.1:2053`、`example.com:8443`、`[2606:4700::1]:443`。端口会记录到输入源端口上下文，并在预览、任务看板和结果页展示。`CIDR+port` 暂不支持，解析时会保留 CIDR 候选但忽略端口，并回退全局 `probe.tcp_port`。

远程输入源在预览、抓取和测速任务准备时都会直连读取，不使用环境代理变量。

## `ui`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `auto_detect_source_name` | `true` | 是否根据输入源自动识别名称。 |
| `theme_mode` | `auto_system_time` | UI 主题模式，可用 `light`、`dark`、`auto_system_time`、`auto_time`。`auto_system_time` 优先跟随系统深浅色，失败时按时间兜底；`auto_time` 始终按本地时间切换。 |
| `theme_light_start` | `07:00` | 时间兜底模式下浅色主题开始时间。 |
| `theme_dark_start` | `19:00` | 时间兜底模式下深色主题开始时间。 |

Android 端固定从 app 私有运行时目录读取配置、档案、任务快照和调试日志。SAF 持久化授权只用于导出目录；如果 `target_uri` 权限失效，CSV、测速文件或调试日志导出会要求重新选择导出目录。

## `scheduler`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 是否启用自动调度。 |
| `interval_minutes` | `0` | 间隔调度分钟数，`0` 表示不按间隔触发。 |
| `daily_times` | `[]` | 每日固定触发时间列表，兼容旧字段 `dailyTimes`。 |
| `skip_if_active` | `true` | 当前已有任务运行时是否跳过本次调度。 |
| `auto_dns_push` | `true` | 调度任务完成后是否自动执行 DNS 推送。 |
| `auto_github_export` | `true` | 调度任务完成后是否自动执行 GitHub 结果导出。 |
| `run_mode` | `probe` | 调度运行模式；`probe` 表示按当前配置执行单任务模式，`pipeline` 表示按 `pipeline-profiles.json` 中已启用策略串行执行 Multi-Profile Pipeline。 |
| `config_source` | `draft_preferred` | 定时任务配置来源；草稿存在且新于正式配置时优先使用草稿，否则使用正式配置。 |
| `post_run_source_profile_action` | `update_recent_run_source_profile` | 定时任务完成后更新固定 ID `source-profile-recent-run` 的最近运行输入源档案。 |

补充说明：

- `run_mode = pipeline` 时，调度器不会读取当前草稿/正式配置，而是直接使用策略页保存的每个 `config_snapshot`。
- `auto_dns_push = false` 会统一覆盖为“跳过 DNS 推送”，但不会改写各策略原本保存的 `dns_push_policy`。
- `auto_github_export` 当前仅作用于 `probe` 模式；`pipeline` 模式下会在状态里标记为未接入并跳过。
- Android 使用系统 WorkManager 注册下一次自动调度，当前固定执行 `probe` 模式，并会把 `pipeline_template_id` 归一为空值；`auto_dns_push` 和 `auto_github_export` 仍控制测速完成后的 DNS 推送和 GitHub 导出。触发时间可能受系统省电和厂商后台策略影响，建议在“异常保护”中申请电池优化豁免并加入厂商后台白名单。

## 风险与建议

`desktop-config.json`、配置归档和 WebDAV 备份可能包含 Cloudflare API Token、WebDAV 密码、历史导出路径和本地文件路径，不应提交到公开仓库。

手动编辑配置时建议先备份原文件，再保持 JSON 字段类型不变。程序会对并发、丢包率、下载缓冲区、HTTP 协议和调试日志模式做归一化，但错误类型仍可能导致配置被回退到默认值。
