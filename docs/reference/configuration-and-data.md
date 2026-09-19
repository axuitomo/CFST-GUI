# 配置与数据结构

> 本页属于 [参考文档](./README.md) 的一部分，说明配置文件位置、配置快照字段、输入源结构和配置归档。更完整的字段默认值与旧配置兼容说明见 [配置详解](../guide/configuration.md)。

## 1. 配置文件位置

配置目录由 Go 的 `os.UserConfigDir()` 决定，并追加 `CFST-GUI` 子目录。若设置 `CFST_GUI_PORTABLE_ROOT`，实际数据目录为 `${CFST_GUI_PORTABLE_ROOT}/data`；若可执行文件同目录存在 `portable.json`，则使用 `<exe-dir>/data`。

| 文件或目录 | 说明 |
| --- | --- |
| `storage.json` | 储存 bootstrap，位于默认配置目录；旧 `storage_dir` 只用于一次性迁移 |
| `desktop-config.json` | 桌面 GUI / WebUI 主要配置快照，位于固定应用数据目录 |
| `mobile-config.json` | Android app 私有目录中的当前配置快照，结构与桌面快照同构 |
| `config.json` | 兼容旧版配置结构 |
| `source-profiles.json` | 输入源档案，包含 `active_profile_id` 和 `items[].sources` |
| `desktop-draft.json` | 桌面草稿，结构同 `desktop-config.json`，用于恢复未正式保存的设置 |
| `tasks/*.json`、`tasks/*-results.json` | 持久化任务快照和结果；`task.results` 对流式 JSON/CSV 做筛选分页，结果 JSON 超过 32MiB 会失败。三端启动时可读取最新快照，活动状态失去对应运行时后会归一化为 `failed/persisted_only`，不会伪装成可继续任务 |
| `cfip-log.txt` | 调试日志，开启 debug 后写入 |
| `exports/`、`imports/`、`backups/` | 导出、导入和本地备份目录 |

检测到旧版自定义 `storage_dir` 时，后端会尝试迁移 `desktop-config.json`、`config.json`、`cfip-log.txt`、地区数据、`source-profiles.json`、`exports/`、`imports/` 和 `backups/` 等已知文件或目录；旧目录不会自动删除。

## 2. 桌面配置快照

`desktop-config.json` 写入磁盘时外层包含 `config_snapshot`、`saved_at`、`schema_version`；`config_snapshot` 由前端维护，主要包含：

| 顶层字段 | 说明 |
| --- | --- |
| `cloudflare` | Cloudflare DNS 读取、推送、Top N 和分流规则配置 |
| `github` | GitHub 结果导出配置 |
| `export` | 本地 CSV 导出配置；旧 `export.github` 兼容镜像仍保留 |
| `post_probe_push` | 手动测速完成后的自动推送勾选项 |
| `backup.webdav` | WebDAV 配置备份和还原 |
| `probe` | 探测策略和高级参数 |
| `sources` | 输入源数组 |
| `scheduler` | 自动调度偏好 |
| `ui` | UI 行为偏好 |

`cloudflare` 字段：

| 字段 | 说明 |
| --- | --- |
| `api_token` | Cloudflare API Token，DNS 读取和推送会真实使用该 Token |
| `zone_id` | Cloudflare Zone ID |
| `enabled` | 是否启用 Cloudflare 配置；测速后自动推送需要开启 |
| `record_name` | 当前配置记录读取和默认推送目标使用的 DNS 记录名 |
| `record_type` | 默认记录类型；新版推送按 IP 自动识别 A/AAAA，分流规则可覆盖 |
| `ttl` | TTL，仅支持 60、300、600 秒，默认 300 秒 |
| `proxied` | 是否开启 Cloudflare 代理 |
| `comment` | 备注 |
| `top_n` | Cloudflare 推送 Top N，默认 5；`0` 表示不限 |
| `routing_enabled`、`routing_rules` | Cloudflare 分流规则开关和规则数组 |

`export` 字段：

| 字段 | 说明 |
| --- | --- |
| `file_name` | 导出文件名，默认 `result.csv` |
| `file_name_template` | 文件名模板，支持 `{date}`、`{time}`、`{profile}`、`{task_id}` |
| `target_dir` | 桌面/WebUI 导出目录，空值表示默认导出目录 |
| `target_uri` | Android SAF 导出目录 URI，桌面端通常为空 |
| `format` | 预留字段，当前按 CSV 输出 |
| `overwrite` | `append` 时追加写入；其他值按任务开始时替换目标文件处理 |
| `github` | GitHub 结果导出配置，包含 owner、repo、branch、path_template、token 等字段 |

`target_dir` 和 `target_uri` 是当前设备本地导出目标。配置压缩包导入和 WebDAV 还原会保留当前设备已有值，不用远端值覆盖，避免 Android SAF URI 与桌面文件路径跨平台互相污染。

`backup.webdav` 字段：

| 字段 | 说明 |
| --- | --- |
| `enabled` | 是否启用 WebDAV 备份入口 |
| `server_url` | WebDAV 服务地址，必须以 `http://` 或 `https://` 开头 |
| `username`、`password` | WebDAV Basic Auth 凭据 |
| `remote_path` | 远端配置包相对路径，默认 `cfst-gui-config.zip`，不能填写完整 URL |
| `timeout_seconds` | WebDAV 请求超时，默认 30 秒 |
| `last_backup_at`、`last_restore_at` | 最近 WebDAV 备份/还原时间，由后端写回配置 |

`scheduler` 字段：

| 字段 | 说明 |
| --- | --- |
| `enabled` | 是否启用自动调度 |
| `interval_minutes` | 间隔调度分钟数，`0` 表示不按间隔触发 |
| `daily_times` | 每日固定触发时间列表，兼容旧字段 `dailyTimes` |
| `skip_if_active` | 当前已有任务运行时是否跳过本次调度 |
| `auto_dns_push` | 调度任务完成后是否自动执行 DNS 推送 |
| `auto_github_export` | 调度任务完成后是否自动执行 GitHub 结果导出 |
| `config_source` | 定时任务配置来源，默认 `draft_preferred`，草稿新于正式配置时使用草稿 |
| `post_run_source_profile_action` | 定时任务完成后更新固定 ID `source-profile-recent-run` 的最近运行输入源档案 |

`probe` 字段中当前会实际映射到后端测速逻辑的配置：

| 字段 | 映射到 | 说明 |
| --- | --- | --- |
| `strategy` | `ProbeConfig.Strategy` | `fast` 或 `full` |
| `concurrency.stage1` | `Routines` | TCP并发线程 |
| `concurrency.stage2` | `HeadRoutines` | 追踪并发线程，归一化后最大为 30 |
| `ping_times` | `PingTimes` | 单 IP 延迟测速次数，最少为 2 |
| `timeouts.stage1_ms` | `Stage1TimeoutMS` | 阶段 1 单次 TCP 连接超时，单位毫秒 |
| `timeouts.stage2_ms` | `Stage2TimeoutMS` | 阶段 2 单次追踪请求超时，单位毫秒 |
| `retry_policy.max_attempts` | `RetryMaxAttempts` | TCP、追踪和下载探测失败后的额外重试次数；`0` 表示不额外重试 |
| `retry_policy.backoff_ms` | `RetryBackoffMS` | 普通重试间隔；服务端 `Retry-After` 仍优先按限流退避处理 |
| `cooldown_policy.consecutive_failures` | `CooldownFailures` | 同一阶段连续失败达到该次数后进入冷却；`0` 关闭冷却 |
| `cooldown_policy.cooldown_ms` | `CooldownMS` | 连续失败冷却时长；暂停和终止均可中断等待 |
| `stage_limits.stage2` | `HeadTestCount` | 阶段 1 结果按 TCP RTT 升序后进入阶段 2 的候选上限；`0` 不限制 |
| `download_count` 或 `stage_limits.stage3` | `TestCount` / `Stage3Limit` | `download_count` 仅兼容读取；阶段 2 结果按追踪延迟 `HeadDelay` 升序后，`stage_limits.stage3` 限制进入文件测速的候选数 |
| `download_time_seconds` | `DownloadTimeSeconds` | 单 IP 下载测速时长，单位秒，默认 4，不设置最大值 |
| `download_warmup_seconds` | `DownloadWarmupSeconds` | 下载预热时间，单位秒，默认 1，最低 0 |
| `download_get_concurrency` | `DownloadGetConcurrency` | 单 IP 内部 Range GET worker 数，默认 4，范围 1-32 |
| `download_buffer_kb` | `DownloadBufferKB` | 下载读取缓冲，默认 256 KiB，范围 64-4096 KiB |
| `download_http_protocol` | `DownloadHTTPProtocol` | 下载协议，支持 `auto`、`tcp`、`h1`、`h2`、`h3`；`auto` 在 Linux ARM 和 Android 上回退到 `tcp` |
| `tcp_port` | `TCPPort` | 测速端口 |
| `port_policy` | 任务上下文 | 默认 `fixed_global`，第一阶段固定使用全局测速端口；显式设置 `source_override_global` 时输入源端口优先 |
| `url` | `URL` | 文件测速URL |
| `trace_url` | `TraceURL` | 追踪 URL；留空时从 `url` 派生 `/cdn-cgi/trace` |
| `user_agent` | `UserAgent` | 请求 User-Agent |
| `host_header` | `HostHeader` | 覆盖追踪请求 Host |
| `sni` | `SNI` | 覆盖追踪请求 TLS SNI |
| `download_host_header` | `DownloadHostHeader` | 覆盖文件测速 Host；留空时，同主机端口继承追踪 Host，否则跟随下载 URL |
| `download_sni` | `DownloadSNI` | 覆盖文件测速 TLS SNI；留空时，同主机端口继承追踪 SNI，否则跟随下载 URL |
| `verify_tls_certificate` | `VerifyTLSCertificate` | 严格校验证书链以及证书域名是否与各阶段实际使用的 SNI 匹配，默认开启 |
| `request_headers` | `RequestHeaders` | 追踪探测和文件测速的多行请求头；保留头会被忽略 |
| `httping` | `Httping` | 兼容旧配置字段，GUI 主流程固定执行追踪阶段 |
| `httping_status_code` | `HttpingStatusCode` | 指定追踪有效 HTTP 状态码，默认 200；显式设置 0 可关闭状态码筛选 |
| `httping_cf_colo` | `HttpingCFColo` | 地区码过滤 |
| `thresholds.max_tcp_latency_ms` | `MaxDelayMS` | TCP 平均延迟上限 |
| `thresholds.max_http_latency_ms` | `HeadMaxDelayMS` | 兼容旧配置字段，当前运行时固定不限制 |
| `min_delay_ms` | `MinDelayMS` | 平均延迟下限 |
| `max_loss_rate` | `MaxLossRate` | 丢包率上限，默认 0.15，最大 1 |
| `thresholds.min_download_mbps` | `MinSpeedMB` | 下载速度下限 |
| `download_speed_metric` | `DownloadSpeedMetric` | 下载速率依据，`average` 或 `max`；影响最低下载速度阈值和最终 Top N 评分 |
| `print_num` | `PrintNum` | 最终结果显示数量；0 不限制，正数按 30% 延迟 + 70% 下载速率的归一化加权评分筛选 Top N，速率依据同 `download_speed_metric` |
| `debug` | `Debug` | 是否写调试日志 |
| `debug_capture_enabled` | `DebugCaptureEnabled` | 是否启用调试拨号目标；旧配置只填 `debug_capture_address` 时会自动兼容为启用 |
| `debug_capture_address` | `DebugCaptureAddress` | 调试拨号目标 |
| `debug_log_mode` | `DebugLogMode` | 调试日志模式，`structured` 默认写 JSONL，`freeform` 写自由格式文本 |
| `debug_log_format` | `DebugLogFormat` | 自由格式模板，支持 `{field}` 占位符，未知字段输出为空 |
| `debug_log_verbosity` | `DebugLogVerbosity` | 调试日志粒度，`detailed` 保持完整事件，`simple` 只保留阶段摘要 |

当前部分预留或弱映射字段：

| 字段 | 当前状态 |
| --- | --- |
| `concurrency.stage3` | 兼容旧配置字段，当前运行时固定为 1 |
| `stage_limits.stage1` | 兼容旧配置字段；新保存配置不主动写入，后端不再按该字段截断 TCP 候选 |
| `test_all` | 共享配置会按快照映射；桌面前端保存时仍写 `false`，CLI `-allip` 会设为 `true` |

## 3. 输入源结构

前端 `SourceConfig` / Go `appcore.Source` 字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | `string` | 来源 ID |
| `name` | `string` | 来源名称 |
| `enabled` | `boolean` | 是否启用 |
| `kind` | `inline \| file \| url` | 来源类型 |
| `content` | `string` | 手动输入内容 |
| `path` | `string` | 本地文件路径 |
| `url` | `string` | 远程 URL；省略协议时默认补 `https://`，GitHub Raw 可切换/兜底到 jsDelivr |
| `ip_limit` | `number` | 候选 IP 上限，默认 500 |
| `ip_mode` | `traverse \| mcis` | 候选处理模式 |
| `last_fetched_at` | `string` | 最近读取时间 |
| `last_fetched_count` | `number` | 最近读取候选数量 |
| `status_text` | `string` | UI 展示状态 |

## 4. 配置归档和敏感信息

配置 JSON 导出包含 `app_version`、`schema_version`、`exported_at`、`config_snapshot`、`source_profiles` 和 `storage`。ZIP 配置归档固定包含 `cfst-gui-config.json`，用于本地下载、WebDAV 备份、Android SAF 导出和跨端还原。

导入归档前，后端会先把当前配置写入 `backups/` 作为 `pre-import` 备份；导入包缺少 `source_profiles` 时，会从导入快照中的 `sources` 生成默认输入源档案。

配置导出、ZIP 归档和 WebDAV 备份可能包含完整 Cloudflare API Token、WebDAV 密码、导出路径和输入源路径，必须只保存到可信位置。
