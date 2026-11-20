# DNS / GitHub 上传链路

本文记录 CFST-GUI 当前的结果上传实现。DNS 推送目标由顶层 `dns_providers` 数组描述，每条可独立选择平台；顶层 `cloudflare`、旧 `upload.cloudflare.*`、`upload.github.*` 和旧 `export.github` 仍作为兼容读取路径保留。

## 能力边界

- 支持多个 DNS 平台并列推送：Cloudflare、DNSHE、阿里云 DNS 与 DNSPod（腾讯云）可同时启用，同一平台也可以添加多条（多 Zone、多子域）。
- 手动结果页可以按平台推送 DNS 记录，或导出到 GitHub。
- 手动单任务测速完成后，可以按 `post_probe_push` 勾选项自动上传。
- 定时任务完成后，可以按 scheduler 配置自动执行 DNS 推送和 GitHub 导出。
- DNS 页面可读取所选 DNS 平台的线上记录（Cloudflare、DNSHE、阿里云 DNS、DNSPod），仍为只读，不执行创建、更新或删除。

## 共享筛选

DNS 平台和 GitHub 上传共用 `upload.shared_filter`：

- 可按探测状态、IP 版本、COLO 白名单或黑名单筛选。
- 可限制最大丢包率、TCP 延迟、追踪延迟和最低下载速度。
- 每个 DNS 平台使用自己的 `top_n`，GitHub 使用自己的 Top N。
- 所有 DNS 平台都可以在共享筛选后继续按记录类型、COLO 和记录名拆分（分流规则）；主目标为空时只推送分流目标。

共享实现位于：

- `internal/dnspush/`：与平台无关的记录同步引擎（`Backend` 接口 + `Reconcile`）
- `internal/cloudflarecore/`：Cloudflare API 客户端与 `Backend` 适配
- `internal/dnshecore/`：DNSHE 免费域名 API V2.0 客户端与 `Backend` 适配
- `internal/alidnscore/`：阿里云云解析 RPC 客户端（`SignatureVersion 1.0` + HMAC-SHA1）与 `Backend` 适配
- `internal/dnspodcore/`：DNSPod（腾讯云）API 3.0 客户端（`TC3-HMAC-SHA256`）与 `Backend` 适配
- `internal/appcore/dns_providers.go`：`dns_providers` 解析、旧 `cloudflare` 段回退
- `internal/appcore/dns_provider_service.go`：逐平台推送编排与汇总
- `internal/appcore/upload_selection.go`
- `internal/appcore/post_probe_push.go`
- `internal/appcore/upload_notification.go`

`internal/app`、`mobileapi` 和 Kotlin 不实现上传规则，只通过统一 `Invoke` 转发命令或展示平台通知状态。

## 记录同步

`internal/dnspush` 负责一次推送里的记录对账：

1. 列出目标记录名下的全部记录。
2. 存在阻断类型（同名的 CNAME 等）时，`ReconcileWithOptions` 按 `AllowCNAME` 决定：默认 `false` 直接返回 `ErrCNAMEProtected`，不删除任何记录；只有显式允许才先删除再写入目标记录，写入失败会尝试恢复被删除的记录。
3. 否则按记录类型和内容对账：缺失的创建、内容或 TTL 变化的原地更新、多余的删除。

各平台适配器只提供列表、创建、更新、删除和「两条记录是否等价」的判断。Cloudflare 的 `comment`/`proxied` 通过 `dnspush.Record.Attrs` 传递，DNSHE 使用模块 `id` 定位记录。

Cloudflare 上传前会归一化 IP 并按地址族分组：IPv4 写入 A，IPv6 写入 AAAA。匹配记录采用覆盖同步，因此可能创建、更新或删除线上记录。执行前必须确认 API Token、Zone ID、记录名、Top N 和分流规则指向预期环境。

三个「完整域名」平台共用 `dnspush` 的 `MatchZone` / `RelativeName` / `AbsoluteName` 做域名与相对记录名换算，并把完整记录名写成 `@` 或相对前缀：

- DNSHE：读取子域名列表；优先按 `subdomain`（标签或完整域名）定位，其次用兼容字段 `subdomain_id`，最后按 `record_name` 最长后缀匹配。记录名必须属于所选子域名，否则拒绝写入；`record_name` 为空时写入该子域顶点。
- 阿里云 DNS：用 `DescribeDomains` 列出账号域名并匹配所属域名，再按 `RR` + `Value` + `TTL` 同步。
- DNSPod：用 `DescribeDomainList` 匹配所属域名，记录线路固定「默认」，再按 `SubDomain` + `Value` + `TTL` 同步。

账号下找不到匹配域名时直接报错，不会写入其它域名；CNAME 冲突由共享的 `ReconcileWithOptions` 统一处理，默认拒绝覆盖，平台配置 `allow_cname` 为 `true` 时才替换。

入口包括：

| 入口 | 行为 |
| --- | --- |
| 结果页手动推送 | 使用当前结果和结果页选择平台的参数执行，`provider_id` 指定单个平台。 |
| 测速后自动推送 | 单任务完成后按 `post_probe_push.cloudflare_enabled` 执行全部启用平台。 |
| 定时任务 DNS 推送 | 单任务调度完成后按 `scheduler.auto_dns_push` 执行全部启用平台。 |

## GitHub 导出

GitHub 导出复用仓库、分支、路径模板、文件格式、行模板和共享筛选配置。失败只记录上传状态，不回滚测速或已经完成的 DNS 推送。

入口包括：

| 入口 | 行为 |
| --- | --- |
| 结果页手动导出 | 使用当前结果和 GitHub Top N 执行。 |
| 测速后自动推送 | 单任务完成后按 `post_probe_push.github_enabled` 执行。 |
| 定时任务 GitHub 导出 | 单任务调度完成后按 `scheduler.auto_github_export` 执行。 |

## 通知与去重

- 手动上传、测速后自动上传和定时任务上传都会生成统一上传通知。
- 通知按平台列表输出：`providers[]` 里每条含 `id`、`label`、`provider`、`status`、`upload_count`；`cloudflare_status` 等旧字段仍会为旧客户端保留。
- Telegram 可以发送上传结论和可选的 Top N 列表；Webhook 直接推送包含 `providers` 的 JSON。
- Android 常驻通知按平台逐条展示状态与条数，旧负载仍回退到 Cloudflare/GitHub 两行。
- 定时任务会禁用 `post_probe_push`，避免同一次测速重复上传。
- 没有可上传结果时记为 `skipped`，不视为执行错误；单个平台失败时整体为 `partial`。

修改上传行为后，应同时验证桌面/WebUI 和 Android，并更新 `docs/guide/configuration.md`、`docs/reference/README.md` 及对应测试。
