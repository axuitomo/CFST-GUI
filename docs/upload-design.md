# Cloudflare / GitHub 上传链路

本文记录 CFST-GUI 当前的结果上传实现。公开配置以顶层 `cloudflare`、`github` 和 `post_probe_push` 为主；旧 `upload.cloudflare.*`、`upload.github.*` 和旧 `export.github` 仅作为兼容读取路径保留。

## 能力边界

- 手动结果页可以推送 Cloudflare DNS 或导出到 GitHub。
- 手动单任务测速完成后，可以按 `post_probe_push` 勾选项自动上传。
- 定时任务完成后，可以按 scheduler 配置自动执行 DNS 推送和 GitHub 导出。
- DNS 页面只读取 Cloudflare 线上记录，不执行创建、更新或删除。

## 共享筛选

Cloudflare 和 GitHub 上传共用 `upload.shared_filter`：

- 可按探测状态、IP 版本、COLO 白名单或黑名单筛选。
- 可限制最大丢包率、TCP 延迟、追踪延迟和最低下载速度。
- Cloudflare 与 GitHub 分别使用自己的 Top N 设置。
- Cloudflare 分流规则可以在共享筛选后继续按记录类型、COLO 和记录名拆分。

共享实现位于：

- `internal/appcore/upload_selection.go`
- `internal/appcore/cloudflare_service.go`
- `internal/appcore/github_service.go`
- `internal/appcore/post_probe_push.go`
- `internal/appcore/upload_notification.go`
- `internal/cloudflarecore/`
- `internal/githubcore/`

`internal/app`、`mobileapi` 和 Kotlin 不实现上传规则，只通过统一 `Invoke` 转发命令或展示平台通知状态。

## Cloudflare DNS

上传前会归一化 IP 并按地址族分组：IPv4 写入 A，IPv6 写入 AAAA。匹配记录采用覆盖同步，因此可能创建、更新或删除线上记录。执行前必须确认 API Token、Zone ID、记录名、Top N 和分流规则指向预期环境。

入口包括：

| 入口 | 行为 |
| --- | --- |
| 结果页手动推送 | 使用当前结果和结果页参数执行。 |
| 测速后自动推送 | 单任务完成后按 `post_probe_push.cloudflare_enabled` 执行。 |
| 定时任务 DNS 推送 | 单任务调度完成后按 `scheduler.auto_dns_push` 执行。 |

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
- Telegram 可以发送上传结论和可选的 Top N 列表。
- 定时任务会禁用 `post_probe_push`，避免同一次测速重复上传。
- 没有可上传结果时记为 `skipped`，不视为执行错误。

修改上传行为后，应同时验证桌面/WebUI 和 Android，并更新 `docs/configuration.md`、`docs/功能与相关接口文档.md` 及对应测试。
