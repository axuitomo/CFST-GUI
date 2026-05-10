# 配置详解

本文档说明 CFST-GUI 的配置目录、配置文件、主要字段、默认值和敏感信息风险。字段名以当前 `desktop-config.json` 快照格式为准。

## 配置目录

默认配置根目录由 Go 的 `os.UserConfigDir()` 决定，并追加 `CFST-GUI` 子目录。不同系统的实际路径由操作系统决定。

便携模式有两种触发方式：

| 方式 | 行为 |
| --- | --- |
| 设置 `CFST_GUI_PORTABLE_ROOT=/some/root` | 数据目录解析为 `/some/root/data` |
| 可执行文件同目录存在 `portable.json` | 数据目录解析为 `<exe-dir>/data` |

如果用户在界面中选择自定义储存目录，程序会把选择结果写入默认配置目录下的 `storage.json`。`storage.json` 是储存位置引导文件，不一定和实际数据目录在同一个目录。

## 文件清单

| 文件或目录 | 默认位置 | 说明 |
| --- | --- | --- |
| `storage.json` | 默认 `CFST-GUI` 配置目录 | 储存目录 bootstrap，记录 `storage_dir`、`storage_uri`、`setup_completed` 等字段。 |
| `desktop-config.json` | 当前 `storageRoot()` | 桌面 GUI 和 WebUI 主要配置快照。 |
| `config.json` | 当前 `storageRoot()` | 兼容旧桥接结构的配置文件。 |
| `profiles.json` | 当前 `storageRoot()` | 探测配置档案，包含 `active_profile_id` 和 `items`。 |
| `source-profiles.json` | 当前 `storageRoot()` | 输入源档案，包含 `active_profile_id` 和 `items[].sources`。 |
| `cfip-log.txt` | 当前 `storageRoot()` | 开启 debug 后写入的调试日志。 |
| `exports/` | 当前 `storageRoot()` | 建议存放 CSV 导出结果。 |
| `imports/` | 当前 `storageRoot()` | 建议存放导入文件。 |
| `backups/` | 当前 `storageRoot()` | 本地配置备份归档目录。 |

切换储存目录时，程序会尝试迁移 `desktop-config.json`、`config.json`、`cfip-log.txt`、`result.csv`、`profiles.json`、`source-profiles.json`、`exports/`、`imports/`、`backups/` 和地区数据文件。

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
| `cloudflare` | Cloudflare DNS 推送配置。 |
| `export` | CSV 导出目标、文件名和覆盖策略。 |
| `backup.webdav` | WebDAV 配置备份和恢复。 |
| `probe` | 探测策略、并发、阈值、超时、调试等核心参数。 |
| `sources` | 输入源列表。 |
| `ui` | UI 行为偏好。 |

## `cloudflare`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `api_token` | 空 | Cloudflare API Token，属于敏感信息。 |
| `zone_id` | 空 | Cloudflare Zone ID。 |
| `record_name` | 空 | 要覆盖推送的 DNS 记录名。 |
| `record_type` | `A` | 记录类型，当前默认 A；推送逻辑会按 IP 类型处理。 |
| `proxied` | `false` | 是否开启 Cloudflare proxy。 |
| `ttl` | `300` | DNS TTL，默认 300 秒。 |
| `comment` | 空 | 写入 DNS 记录的备注。 |

`api_token` 会随配置快照保存。导出配置或备份归档前，需要确认文件不会被提交到仓库或公开分享。

## `export`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `file_name` | `result.csv` | 默认导出文件名。 |
| `file_name_template` | 空 | 文件名模板；支持 `{date}`、`{time}`、`{profile}`、`{task_id}`。 |
| `format` | `csv` | 当前导出格式。 |
| `overwrite` | `replace_on_start` | 覆盖策略；值为 `append` 时追加写入。 |
| `target_dir` | 空 | 桌面/WebUI 文件系统导出目录；空时使用 `storageRoot()`。 |
| `target_uri` | 空 | Android SAF 导出 URI。 |

文件名会经过路径非法字符清理，避免把 `/`、`\`、`:`、`*`、`?`、`"`、`<`、`>`、`|` 写入文件名。

## `backup.webdav`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 是否启用 WebDAV 备份。 |
| `server_url` | 空 | WebDAV 服务地址。 |
| `username` | 空 | WebDAV 用户名。 |
| `password` | 空 | WebDAV 密码或 Token，属于敏感信息。 |
| `remote_path` | `cfst-gui-config.zip` | 远端备份文件路径。 |
| `timeout_seconds` | `30` | WebDAV 请求超时。 |
| `last_backup_at` | 空 | 最近备份时间，由界面维护。 |
| `last_restore_at` | 空 | 最近恢复时间，由界面维护。 |

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
| `download_time_seconds` | `10` | 单 IP 下载测速时长。 |
| `download_warmup_seconds` | `5` | 下载测速预热时长。 |
| `tcp_port` | `443` | TCP 延迟和下载测速端口。 |
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

## `ui`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `auto_detect_source_name` | `true` | 是否根据输入源自动识别名称。 |

## 风险与建议

`desktop-config.json`、配置归档和 WebDAV 备份可能包含 Cloudflare API Token、WebDAV 密码、导出路径和本地文件路径，不应提交到公开仓库。

手动编辑配置时建议先备份原文件，再保持 JSON 字段类型不变。程序会对并发、丢包率、下载缓冲区、HTTP 协议和调试日志模式做归一化，但错误类型仍可能导致配置被回退到默认值。
