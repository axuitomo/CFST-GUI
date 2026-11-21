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
| `source-profiles.json` | 固定应用数据目录 | 输入组，包含 `active_profile_id` 和 `items[].sources`。 |
| `cfip-log.txt` | 固定应用数据目录 | 开启 debug 后写入的调试日志。 |
| `exports/` | 导出目录或固定应用数据目录下的默认导出目录 | CSV、测速文件和调试日志导出结果。 |
| `imports/` | 固定应用数据目录 | 建议存放导入文件。 |
| `backups/` | 固定应用数据目录 | 本地配置备份归档目录。 |
| `tasks/` | 固定应用数据目录 | 任务快照和持久化结果 JSON。结果文件按 32MiB 上限流式读取。 |
| `webview2/` | 本机缓存目录 | Windows WebView2 profile（localStorage、缓存）。固定为 `%LOCALAPPDATA%\CFST-GUI\webview2`，不跟随漫游目录，也不跟随自定义数据目录。首次启动会依次尝试从上一版的 `%APPDATA%\CFST-GUI\webview2` 和 Wails 默认的 `%APPDATA%\<exe 名>` 一次性搬迁。 |

检测到旧版自定义 `storage_dir` 时，程序会尝试迁移 `desktop-config.json`、`desktop-draft.json`、`config.json`、`cfip-log.txt`、`result.csv`、`source-profiles.json`、`exports/`、`imports/`、`backups/` 和地区数据文件。迁移不会删除旧目录。

## `desktop-config.json` 结构

写入磁盘时，配置外层包含元数据：

```json
{
  "config_snapshot": {},
  "saved_at": "2026-05-08T00:00:00+08:00",
  "schema_version": "cfst-gui-config-v2"
}
```

`config_snapshot` 内部包含这些顶层字段：

| 字段 | 说明 |
| --- | --- |
| `dns_providers` | DNS 推送平台列表，每条可独立选择 Cloudflare、DNSHE、阿里云 DNS 或 DNSPod（腾讯云）。 |
| `cloudflare` | 兼容字段：DNS 读取页和旧调用链使用的 Cloudflare 配置，保存时由第一条 Cloudflare 平台派生。 |
| `github` | GitHub 结果导出配置。 |
| `upload` | 统一上传筛选与目标 Top N 配置。 |
| `export` | 本地 CSV 导出目标、文件名和覆盖策略；旧 `export.github` 仍兼容读取。 |
| `post_probe_push` | 手动测速完成后的自动推送勾选项。 |
| `backup.webdav` | WebDAV 配置备份和恢复。 |
| `maintenance` | 运行时维护策略，例如终态任务快照保留天数。 |
| `probe` | 探测策略、并发、阈值、超时、调试等核心参数。 |
| `source_ip_mode` | 当前输入组的 IP 模式，统一应用到本组所有来源。 |
| `sources` | 输入源列表。 |
| `scheduler` | 自动任务调度偏好。 |
| `ui` | UI 行为偏好。 |

`desktop-draft.json` 使用相同外层结构。前端会防抖调用 `draft.save` 写入草稿；`config.load` 会返回 `draft_status`，当草稿 `saved_at` 新于正式配置时，界面会提示恢复或丢弃。正式执行 `config.save` 成功后会清理草稿，避免下次启动重复恢复。

## 旧配置兼容与字段净化

读取 `desktop-config.json`、移动端 `mobile-config.json` 和配置压缩包中的 `config_snapshot` 时，程序会先按当前 schema 生成兼容快照：

- 缺少的新字段会补当前默认值，例如 `backup.webdav`、`probe.retry_policy`、`scheduler` 等旧配置中不存在的字段。
- 已废弃或未知字段会被忽略，不会导致读取失败，也不会出现在返回给前端的 `config_snapshot` 中。
- 常见旧字段别名会迁移到当前字段名，例如 `apiToken` → `api_token`、`remotePath` → `remote_path`、`timeoutSeconds` → `timeout_seconds`、`stageLimits` → `stage_limits`、`cooldownPolicy` → `cooldown_policy`、`retryPolicy` → `retry_policy`、`dailyTimes` → `daily_times`、输入源 `type` → `kind`。
- 旧 `export.github` 会同步到顶层 `github`；旧 `upload.cloudflare.routing_*` 和 `upload.cloudflare.top_n` 会同步到顶层 `cloudflare`。保存时仍会写回兼容结构，避免旧调用链读取不到。
- 旧版 `sourceText` 或 `probe.ipText` 会在没有 `sources` 字段时转换为一个 `inline` 输入源。
- 探测字段合并：旧 `probe.download_count` / `testCount` 只作为 `stage_limits.stage3` 的兼容来源；旧 `timeouts.stage3_ms` 只在缺少 `download_time_seconds` 时按秒迁移；旧 `download_speed_sample_interval_seconds` 只在缺少 `_ms` 时按毫秒迁移。
- 追踪字段改名：`httping_status_code` → `trace_status_code`、`httping_cf_colo` → `trace_cf_colo`、`httping_cf_colo_mode` → `trace_cf_colo_mode`，旧键仍可读取，保存时只写新键。HTTPing 延迟模式与 `httping` 字段已移除，旧配置中的该键会被忽略。
- 已移除字段：`thresholds.max_http_latency_ms`（运行时本来就固定不限制）、`custom_host_sni` 总开关（四个 Host/SNI 字段非空即生效）。旧配置显式写 `custom_host_sni=false` 时，迁移会清空这四个字段，保持"关闭即完全不生效"的行为。

兼容读取不会立刻改写磁盘文件。只有保存配置、导入配置归档、WebDAV 备份/还原时间写回、保存或切换输入组/策略档案等写入路径会把净化后的当前格式落盘。JSON 语法错误仍会按解析失败处理，兼容逻辑只处理字段缺失、旧别名和未知字段。

## `dns_providers`

DNS 推送平台列表。测速后自动推送、定时任务和结果页手动推送都会写入所有 `enabled=true` 且凭据完整的平台；列表为空时程序回退读取旧顶层 `cloudflare` 段，因此旧配置无需迁移。

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `id` | 自动生成 | 平台条目标识，结果页手动推送用它指定单个平台。 |
| `provider` | 必填 | `cloudflare`、`dnshe`、`aliyun` 或 `dnspod`；`alidns`/`dndps`/`tencentcloud` 作为别名兼容读取。 |
| `name` | 空 | 平台备注名，用于通知和结果页选择；空值显示为「平台名 (记录名)」。 |
| `enabled` | `true` | 是否启用该平台；凭据或目标不完整时不会推送。 |
| `record_name` | 空 | Cloudflare 填记录名；DNSHE、阿里云 DNS、DNSPod 填完整域名（例如 `api.example.com`），子域下的 `www.api.example.com` 也可直接填。 |
| `record_type` | `A` | `A`、`AAAA`；Cloudflare 额外支持 `ALL`（A + AAAA）。 |
| `ttl` | Cloudflare `300` / 其余 `600` | DNS TTL。Cloudflare 只支持 60、300、600 秒；阿里云与 DNSPod 小于 60 时回退到 `600`。 |
| `top_n` | `5` | 该平台的目标上传数量；`0` 表示不限，共享上传筛选仍先执行。 |
| `api_token` | 空 | Cloudflare API Token（敏感）。 |
| `zone_id` | 空 | Cloudflare Zone ID。 |
| `routing_enabled` | `false` | 仅 Cloudflare：是否启用分流规则。 |
| `routing_rules` | `[]` | 仅 Cloudflare：分流规则数组，字段与旧 `cloudflare.routing_rules` 相同。 |
| `api_key` | 空 | DNSHE API Key、阿里云 AccessKey ID、DNSPod SecretId（敏感）。 |
| `api_secret` | 空 | DNSHE API Secret、阿里云 AccessKey Secret、DNSPod SecretKey（敏感）。 |
| `subdomain` | 空 | DNSHE 子域名。可填账号下已注册的子域名标签或完整域名；裸标签在账号里没有同名子域时按 `<标签>.<record_name>` 解释（例如 `test` + `axuitomo.l.cd` → `test.axuitomo.l.cd`），账号里存在同名子域时仍按该子域解析。 |
| `subdomain_id` | `0` | DNSHE 子域 ID；`0` 表示按 `subdomain` 或 `record_name` 自动匹配。 |
| `allow_cname` | `false` | 是否允许覆盖同名 CNAME。默认关闭：目标记录名已有 CNAME 时拒绝本次推送并返回 `DNS_CNAME_PROTECTED`，不删除别名；开启后才先删除 CNAME 再写入 A/AAAA。四个平台通用。 |

DNSHE 使用免费域名 API V2.0：鉴权头为 `X-API-Key` / `X-API-Secret`，端点 `https://api005.dnshe.com/index.php?m=domain_hub`。推送前程序会读取子域名列表并按最长后缀匹配出 `subdomain_id`，随后按记录名同步 A/AAAA 记录（创建、更新、删除）；`subdomain` 填裸标签且账号下无同名子域时，会先拼成 `<标签>.<record_name>` 再匹配，写入名同步改为该完整域名。同一记录名上的 CNAME 仅在 `allow_cname=true` 时先删除再写入，写入失败时尝试恢复。

阿里云 DNS 走云解析 RPC 接口（`https://alidns.aliyuncs.com/`，`SignatureVersion 1.0` + HMAC-SHA1）：`DescribeDomains` 按最长后缀解析所属域名，再用 `DescribeDomainRecords` / `AddDomainRecord` / `UpdateDomainRecord` / `DeleteDomainRecord` 同步记录。AccessKey 需要该域名下的解析权限。

DNSPod 走腾讯云 API 3.0（`https://dnspod.tencentcloudapi.com/`，`TC3-HMAC-SHA256` 签名，`X-TC-Version: 2021-03-23`）：`DescribeDomainList` 解析所属域名，再用 `DescribeRecordList` / `CreateRecord` / `ModifyRecord` / `DeleteRecord` 同步记录，记录线路固定使用「默认」。SecretId/SecretKey 需要 DNSPod 的域名解析权限。

三类「完整域名」平台在推送前都会先读取账号下的域名列表做最长后缀匹配，域名不在账号下时直接报错而不写入；同一记录名上的 CNAME 与 Cloudflare 一样，默认拒绝覆盖，只有 `allow_cname=true` 时才会被替换。

## `cloudflare`

`cloudflare` 现在是兼容段：有 `dns_providers` 时后端只读取平台列表，本段仅用于 DNS 读取页和历史调用链；前端保存时会把第一条 Cloudflare 平台同步写回本段。

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `api_token` | 空 | Cloudflare API Token，属于敏感信息；只读取 DNS 可授予 DNS Read，需要推送时授予目标 Zone 的 DNS Edit 权限，详见 [Cloudflare API Token 权限设置教程](../integration/cloudflare-api-token.md)。 |
| `zone_id` | 空 | Cloudflare Zone ID。 |
| `enabled` | `false` | 是否启用 Cloudflare 配置；测速后自动推送需要该项开启。 |
| `record_name` | 空 | 当前配置记录读取和默认推送目标使用的 DNS 记录名。读取时优先级最高：填写后按它读，留空则退回第一条启用的分流域名。 |
| `record_type` | `A` | 默认记录类型；推送逻辑会按 IP 类型处理，分流规则可单独覆盖。 |
| `proxied` | `false` | 是否开启 Cloudflare proxy。 |
| `ttl` | `300` | DNS TTL，默认 300 秒。 |
| `allow_cname` | `false` | 兼容字段；是否允许覆盖同名 CNAME。默认关闭，语义与 `dns_providers[].allow_cname` 一致。 |
| `comment` | 空 | 写入 DNS 记录的备注。 |
| `top_n` | `5` | Cloudflare 目标上传数量；`0` 表示不限。 |
| `routing_enabled` | `false` | 是否启用 Cloudflare 分流规则。 |
| `routing_rules` | `[]` | Cloudflare 分流规则数组；每条规则可按国家/COLO 筛选并推送到指定记录名。 |

Cloudflare 分流规则按列表顺序排列，可折叠单条规则以便集中编辑；新增规则按钮位于列表末尾，记录类型和白名单/黑名单选择框在窄屏上限制宽度。

DNS 读取区可按所选平台读取 Cloudflare、DNSHE、阿里云 DNS 或 DNSPod 记录，不修改线上 DNS；同页的 DNS 平台自检会在 `cfst-selftest.<推送记录名>` 上临时写入并清理，用于验证四个平台的读取、上传与记录更替，DNSHE 则固定在主域名（子域 zone 顶点）下的 `cfst-selftest.<主域名>`。定时任务和测速后自动推送中的 Cloudflare 平台会复用第一条 Cloudflare 平台条目（本段同步内容）、共享上传策略、`top_n` 和分流规则。`api_token` 会随配置快照保存；导出配置或备份归档前，需要确认文件不会被提交到仓库或公开分享。

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
| `owner` | 空 | GitHub 仓库 owner；留空时不预填仓库。 |
| `repo` | 空 | GitHub 仓库名；留空时不预填仓库。 |
| `branch` | `main` | 目标分支。 |
| `path_template` | `cfst-results/{date}/{time}-{task_id}.csv` | 目标文件路径模板。 |
| `commit_message_template` | `CFST results {date} {time}` | 提交信息模板。 |
| `format` | `csv` | GitHub 导出格式。 |
| `csv_header_template` | 空 | CSV 头模板。 |
| `csv_row_template` | 空 | CSV 行模板。 |
| `txt_row_template` | `{ip}` | TXT 行模板。 |
| `token` | 空 | GitHub PAT，属于敏感信息；推荐使用 fine-grained PAT，并仅授予目标仓库 Contents Read and write，详见 [GitHub PAT 权限设置教程](../integration/github-pat.md)。 |
| `top_n` | `20` | GitHub 目标上传数量；`0` 表示不限。 |
| `last_export_at` | 空 | 最近 GitHub 导出时间。 |

旧 `export.github` 继续兼容读取和保存，但当前 UI 的 GitHub 配置卡片以顶层 `github` 为主。

## `post_probe_push`

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `cloudflare_enabled` | `false` | 手动单任务测速完成后，自动执行 DNS 推送（`dns_providers` 中的全部启用平台）。字段名保留为兼容旧配置。 |
| `github_enabled` | `false` | 手动单任务测速完成后，自动执行 GitHub 导出。 |

`post_probe_push` 不叠加到定时任务；scheduler 会显式禁用这条自动推送入口，避免和调度推送重复。

测速后自动推送会先应用统一上传筛选，再分别使用各 DNS 平台和 GitHub 的目标 Top N。筛选失败时会返回 warning 并停止后续 provider；某个 DNS 平台鉴权失败只影响该平台，其余平台继续推送并返回 `partial`；GitHub 筛选后没有结果时会跳过导出，不提交空文件，也不会回退到未筛选的完整测速结果。

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

- 定时任务中的 DNS 推送（全部启用平台）
- 定时任务中的 GitHub 导出
- 手动 GitHub 结果导出
- 测速后自动推送列表中的 DNS 平台 / GitHub

各平台的 Top N 在 `dns_providers[].top_n`；GitHub 的 Top N 在顶层 `github.top_n`。旧 `upload.cloudflare.top_n`、`upload.github.top_n` 仍兼容读取。

DNS 推送会按平台汇总三类结论：筛选后没有可推送 IP 时为跳过，所有平台失败的时为失败，部分平台成功时为 `partial`。定时任务状态、上传通知、Webhook 和 Telegram 通知都按平台列表输出（`providers`），Android 常驻通知同样展示每个平台的状态和条数。

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
| `strategy` | `fast` | `fast` 跳过文件测速；`full` 执行文件测速。兼容值 `latency` 会归一化为 `fast`，`speed`、`exhaustive` 会归一化为 `full`。 |
| `disable_download` | `true` | 默认禁用下载测速；`strategy=full` 时会关闭该项。 |
| `stage_limits.stage2` | `0` | 阶段 1 通过后按 TCP RTT 升序选取并进入阶段 2；`0` 不限制。 |
| `stage_limits.stage3` | `100` | 阶段 2 通过后按追踪延迟（`HeadDelay`）升序选取并进入阶段 3 文件测速。 |
| `source_pool_limit` | `65536` | 单个输入源候选池上限，范围 1 到 1000000；输入源不再单独设 IP 上限，一律展开到该上限为止（超过上限时按上限执行并给出提示）。配置了输入源 COLO 预筛时，解析预算为该上限的 8 倍，超出部分会被截断并告警。所有输入源聚合后的候选池另有 1000000 条的绝对上限。 |
| `doh_url` | `https://dns.alidns.com/dns-query` | 用 DoH（RFC 8484）解析输入源域名和来源 URL 主机名，必须是 `https://` 地址。设置页「网络与任务 → DNS 与 ECH」提供阿里 DoH、`https://sm2.doh.pub/dns-query`、`https://cloudflare-dns.com/dns-query` 三个预设和自定义输入，并可一键测试端点解析与 ECH 获取情况（`doh.test`）。留空或非法值都按默认阿里 DoH 处理（非法值额外给出一条警告）；DoH 端点连接失败、返回非 200 或响应无效时自动回退系统 DNS，有效的 NXDOMAIN 不会回退。查询 A/AAAA 记录，单次查询超时 4 秒。DoH 请求只跟随显式配置的 `HTTPS_PROXY`/`https_proxy` 与 `NO_PROXY`（启动进程时已设置的环境变量），不跟随 Windows 系统代理。同一端点还会读取 HTTPS RR（类型 65）以启用 ECH；ECH 被服务端拒绝或因普通 TLS 证书不符而失败时自动改用普通 TLS 重试一次，同一主机后续不再尝试。 |
| `ech` | `true` | 对来源 URL 请求启用 ECH（RFC 9849）。关闭后不再读取 HTTPS RR、不再尝试 ECH。仅作用于远程输入源抓取，不影响追踪探测和文件测速。 |
| `ech_domain` | `cloudflare-ech.com` | ECH 配置来源域名，也是 ECH 外层 SNI。自动获取时先用来源主机自己的 HTTPS RR，主机没有发布 `ech` 参数时改用该域名的记录（一次任务只查一次）。只接受裸域名，带协议、端口或路径时回退默认值并告警。 |
| `ech_config` | 空 | 手填 base64 `ECHConfigList`，填入后覆盖 `ech_domain` 和来源主机的 HTTPS RR 自动获取，因此没有配置 DoH 也能启用 ECH（域名改由系统 DNS 解析）。不是合法 base64 时清空并给出警告，回退到自动获取。 |

### 并发与采样

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `concurrency.stage1` | 桌面 `200`，Android `64` | TCP 延迟测速并发，最大 `1000`。参考区间：Windows `128-384`、Linux `128-768`、macOS `96-256`、Android `32-128`（蜂窝或省电模式 `16-64`）。 |
| `concurrency.stage2` | `30` | 追踪探测并发，最大 `30`。 |
| `concurrency.stage3` | `1` | 文件测速阶段并发，可设 `1-16`。默认 `1` 保证单个 IP 独占带宽的测量口径；调大可缩短大列表的总耗时，但并发下载会互相抢带宽，测得速度偏低。超过 `16` 会归一化为 `16`。 |
| `mcis.budget` | `0`（自动） | MICS 抽样预算；自动按输入源候选池上限（`source_pool_limit`）×3 计算，正数可手动指定且不设固定最大值。候选数少于预算时按去重后的唯一候选网络收敛，IPv4 按地址、IPv6 按 `/64` 去重。 |
| `mcis.concurrency` | `0`（自动） | MICS 抽样并发；自动与 TCP 并发线程数同步，正数可手动指定，不按平台设置上限。 |
| `ping_times` | `4` | 单个 IP TCP 发包次数，最少 `2`。 |
| `skip_first_latency_sample` | `true` | 是否跳过首个延迟样本。 |
| `event_throttle_ms` | `100` | 进度事件推送节流，作用在 `probe.progress` 事件与阶段日志节流上；运行前下发。 |
| `download_speed_sample_interval_ms` | `1000` | 下载测速实时速率的刷新间隔，最短 1 秒（更小的值按 1 秒执行）；实时速率按最近 1 秒滑动窗口计算，间隔长于 1 秒时按该间隔计算。 |

### 下载与网络

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `download_success_limit` | `0`（关闭） | 仅用于文件下载测速；达到指定的合格 IP 数后停止提交剩余下载测速，进入结果整理。`0` 不限制。 |
| `download_get_concurrency` | `4` | 单 IP 下载 GET 并发，范围 `1` 到 `32`。 |
| `download_buffer_kb` | `256` | 下载缓冲区，范围 `64` 到 `4096` KB。 |
| `download_http_protocol` | `auto` | 下载 HTTP 协议，可用 `auto`、`tcp`、`h1`、`h2`、`h3`。`auto` 在 Linux ARM 和 Android 上会回退到 `tcp`，避免 H3/UDP 异常；其他平台仍先尝试 HTTP/3。 |
| `download_speed_metric` | `average` | 下载速率依据，可用 `average` 或 `max`；仅影响最低下载速度阈值和结果显示数量 Top N 评分。 |
| `download_time_seconds` | `4` | 单 IP 下载测速时长。 |
| `download_warmup_seconds` | `1` | 下载测速预热时长。 |
| `tcp_port` | `443` | TCP 延迟和下载测速端口。 |
| `port_policy` | `fixed_global` | 默认固定使用 `tcp_port` 作为第一阶段测速端口；可显式设置为 `source_override_global`，让输入源声明的端口优先。 |
| `url` | `https://speed.cloudflare.com/__down?bytes=99999999` | 文件测速 URL；默认值取 Cloudflare 官方端点允许的最大 `bytes`，≥ `100000000` 会被 403 拒绝。 |
| `trace_url` | 空 | 追踪探测 URL；空时可从文件测速 URL 推导 `/cdn-cgi/trace`。1034 是 zone 级响应，Cloudflare 官方 zone（默认推导出的 `speed.cloudflare.com`）不会返回它，要按自己的站点拦截 1034，就把它填成自己站点的 `/cdn-cgi/trace`。也可指向自建探针的 `/trace`（见 [`tools/cfst-probe-worker`](../../tools/cfst-probe-worker/README.md)），配合 `trace_colo_mode=trace_url` 使用。 |
| `user_agent` | 内置 Firefox UA | 请求 User-Agent。 |
| `host_header` | 空 | 覆盖追踪请求的 Host 头；非空即生效，留空时跟随追踪 URL。 |
| `sni` | 空 | 覆盖追踪请求的 TLS SNI；非空即生效，留空时跟随追踪 URL。注意它与 Host 是两个独立字段，不会互相继承。 |
| `download_host_header` | 空 | 覆盖文件测速的 Host 头；非空即生效，留空时，同主机端口继承追踪 Host，否则跟随文件测速 URL。 |
| `download_sni` | 空 | 覆盖文件测速的 TLS SNI；非空即生效，留空时，同主机端口继承追踪 SNI，否则跟随文件测速 URL。 |
| `verify_tls_certificate` | `false` | 是否严格校验服务端证书链及证书域名；按追踪和下载阶段各自实际使用的 SNI 校验，默认关闭。远端输入源抓取、更新包、COLO 词典和 Cloudflare DNS API 始终校验证书，不受该字段影响。 |
| `request_headers` | 空 | 作用于追踪探测和文件测速的多行请求头，每行 `Header-Name: value`；`Host`、`User-Agent`、`Range` 等保留头会被忽略。 |

文件测速只跟随同源重定向，并拒绝 `text/html`、`application/xhtml+xml` 以及内容嗅探识别出的 HTML 响应，避免将反向代理默认页、nginx 管理面板或登录页误计为测速流量。Android 的明文策略只作用于 Java/WebView 网络栈，原生测速引擎仍可访问明文 `http://`，但明文容易被中间网络劫持或改写，建议改用 `https://`；桌面端和 WebUI 一贯允许明文 HTTP。

403 与 1034 没有开关：无论出现在阶段 2 追踪还是阶段 3 文件测速，都直接淘汰该 IP，不重试、不计入连续失败冷却。日志用 `reason=forbidden` / `edge_ip_restricted` 加 `note=403` / `note=1034` 区分；阶段 2 读到 403 正文里的 1034 标记（`error code: 1034`、`Edge IP Restricted`、`errorCode: 1034`，大小写不敏感）会提前淘汰，不必等到文件测速。注意 1034 只在请求带有真实站点身份时出现：默认追踪 URL 落在 Cloudflare 官方 zone，该 zone 不返回 1034，因此默认配置下阶段 2 筛不出 1034，要按自己的站点拦截就把 `trace_url` 填成自己站点的 `/cdn-cgi/trace`；阶段 3 的下载请求打的是 `url`，同样要指向真实站点才可能看到 1034。

### 追踪、阈值与输出

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `trace_status_code` | `200` | 追踪有效状态码；默认只接受 `200`，显式设置 `0` 可关闭状态码筛选。旧键 `httping_status_code` 仍兼容读取。 |
| `trace_cf_colo` | 空 | 追踪阶段的地区码过滤，白名单/黑名单由 `trace_cf_colo_mode` 决定。旧键 `httping_cf_colo` 仍兼容读取。 |
| `trace_cf_colo_mode` | `allow` | 地区码过滤模式，可用 `allow`、`deny`。旧键 `httping_cf_colo_mode` 仍兼容读取。 |
| `thresholds.max_tcp_latency_ms` | `null` | TCP 延迟上限；空时使用内部默认 `9999` ms。 |
| `thresholds.min_download_mbps` | `0` | 下载速度下限，单位 MB/s；按 `download_speed_metric` 选择平均速率或最高速率判定。 |
| `min_delay_ms` | `0` | TCP 延迟下限。 |
| `max_loss_rate` | `0.15` | 丢包率上限，最大 `1.00`。 |
| `print_num` | `0` | 结果显示数量；`0` 表示不限制，正数按 30% 延迟 + 70% 下载速率的归一化加权评分筛选最终 Top N，速率依据同 `download_speed_metric`。 |
| `test_all` | `false` | 保留兼容字段。IP/CIDR 由 IP 池阶段统一展开，GUI/WebUI/Android 保存配置时仍写 `false`，旧配置中的该字段已不影响正常流程。 |

### 超时、重试和调试

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `timeouts.stage1_ms` | `1000` | TCP 阶段超时。追踪和文件测速的 TLS 握手另有独立预算：取该值的两倍且不低于 3 秒，不会随这个值一起被压到 1 秒。 |
| `timeouts.stage2_ms` | `1000` | 追踪阶段超时。 |
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

当前输入组的运行配置存放在顶层 `source_ip_mode` 和输入组的 `items[]` 中。`items[].ip_mode` 保存 IP 处理模式，`items[].colo_filter` 与 `items[].colo_filter_mode` 保存整组地区筛选。界面只提供组级设置，新增来源沿用组配置；空组也保留已选设置。来源内的 `ip_mode`、`colo_filter` 与 `colo_filter_mode` 仅用于兼容旧来源配置，显式组设置优先。

旧配置未设置组模式时，统一的来源模式会被识别为组模式；如果旧来源混用了两种模式，会保留原行为并提示选择统一模式，不会在读取时静默覆盖。

每个输入源支持以下字段：

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `id` | `source-1` | 输入源 ID。 |
| `name` | `输入源 1` | 输入源名称。 |
| `kind` | `url` | 输入源类型，常见为 URL、本地文件或手动内容。 |
| `url` | 空 | 远程输入源 URL。 |
| `path` | 空 | 本地输入源路径。 |
| `content` | 空 | 手动输入内容。 |
| `enabled` | `true` | 是否参与探测；缺省视为启用，只有显式 `false` 才会停用该输入源。 |
| `ip_limit` | 已移除 | 界面不再提供该字段，读取配置时一律归一为 `-1`（不限量），实际候选量由 `probe.source_pool_limit`（默认 65536）与聚合绝对上限收敛。 |
| `ip_mode` | `traverse` | 兼容字段；由组级 `source_ip_mode` 统一下发。旧组未统一时保留原来源设置。 |
| `colo_filter` | 空 | 兼容字段；新配置的节点筛选保存在输入组 `items[].colo_filter`。 |
| `colo_filter_mode` | `allow` | 兼容字段；新配置的白名单/黑名单模式保存在输入组 `items[].colo_filter_mode`。 |
| `last_fetched_at` | 空 | 最近一次成功抓取时间；读取失败不会覆盖该字段。 |
| `last_fetched_count` | `0` | 最近一次成功抓取的候选数量；读取失败保留旧值。 |
| `status_text` | 空 | 输入源状态文本。 |

输入源内容支持在单条候选中携带端口：`1.1.1.1:2053`、`example.com:8443`、`[2606:4700::1]:443`。端口会记录到输入源端口上下文，并在预览、任务看板和结果页展示。`CIDR+port` 暂不支持，解析时会保留 CIDR 候选但忽略端口，并回退全局 `probe.tcp_port`。

远程输入源在预览、抓取和测速任务准备时都会读取：这是 IP 池阶段，跟随系统代理（Windows Internet 设置中的手动代理与绕过列表，未配置时回退 `HTTP_PROXY`/`HTTPS_PROXY` 等环境变量）。TCP 测延迟、追踪探测和文件测速等其他阶段一律直连，不跟随系统代理。

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
| `config_source` | `draft_preferred` | 定时任务配置来源；草稿存在且新于正式配置时优先使用草稿，否则使用正式配置。 |
| `post_run_source_profile_action` | `update_recent_run_source_profile` | 定时任务完成后更新固定 ID `source-profile-recent-run` 的最近运行输入组。 |

补充说明：

- `auto_dns_push = false` 会跳过本次定时任务的 DNS 推送。
- `auto_github_export = false` 会跳过本次定时任务的 GitHub 导出。
- Android 使用系统 WorkManager 注册下一次自动调度；`auto_dns_push` 和 `auto_github_export` 控制测速完成后的 DNS 推送和 GitHub 导出。触发时间可能受系统省电和厂商后台策略影响，建议在“异常保护”中申请电池优化豁免并加入厂商后台白名单。

## 风险与建议

`desktop-config.json`、配置归档和 WebDAV 备份可能包含 Cloudflare API Token、WebDAV 密码、历史导出路径和本地文件路径，不应提交到公开仓库。

手动编辑配置时建议先备份原文件，再保持 JSON 字段类型不变。程序会对并发、丢包率、下载缓冲区、HTTP 协议和调试日志模式做归一化，但错误类型仍可能导致配置被回退到默认值。
