# 外部接口与输入输出

> 本页属于 [参考文档](./README.md) 的一部分，说明主程序对外部 HTTP 与第三方 API 的调用，以及整体输入、输出形态。WebUI HTTP API 见 [接口契约](./api-contracts.md#2-webui-http-api)。

## 1. 外部 HTTP 与第三方 API

### 1.1 主程序外部 HTTP 请求

| 用途 | 方法 | 地址 | 调用位置 |
| --- | --- | --- | --- |
| 版本检查 | `GET` | `https://api.github.com/repos/axuitomo/CFST-GUI/releases/latest`，直连 GitHub API，不使用环境代理 | `internal/app/run.go`、`internal/app/update.go` |
| 更新 manifest / 更新包下载 | `GET` | GitHub Release manifest 与 asset 下载地址；直连并发尝试 GitHub 加速候选链，不使用环境代理 | `internal/app/update.go` |
| 输入源 URL 读取 | `GET` | 用户配置的输入源 URL；GitHub Raw 可兜底到 `https://cdn.jsdelivr.net/gh/...`；预览、抓取和测速任务准备阶段都会直连读取，不使用环境代理 | `internal/appcore/source_content.go`、`probe_service.go` |
| 阶段 2 追踪探测 | `GET` | 默认从文件测速URL派生 `/cdn-cgi/trace`，可由 `trace_url` 覆盖 | `internal/task/head.go`、`internal/task/colo.go` |
| 下载测速 | `GET` | 默认 `https://speedtest.xyz9923.dpdns.org/500m`，可由桌面配置覆盖，阶段 3 不再访问 `/cdn-cgi/trace` | `internal/task/download.go` |
| MICS抽样探测 | `HEAD` | `https://<target-host>/cdn-cgi/trace` 或解析后的路径 | `internal/mcis/probe/trace.go` |
| Cloudflare DNS | `GET`/`POST`/`PUT`/`DELETE` | `https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records` | `internal/appcore/cloudflare_service.go`、`internal/cloudflarecore/` |
| WebDAV 备份 | `HEAD`/`PUT`/`GET` | 用户配置的 WebDAV URL + `remote_path` | `internal/appcore/archive_service.go`、`internal/archivecore/` |
| COLO 字典更新 | `GET` | 默认 geofeed URL 或用户传入 `source_url`；GitHub 辅助映射源复用在线更新的直连加速候选链 | `internal/app/desktop_colo_dictionary.go`、`mobileapi/colo_dictionary.go` |

说明：

- 桌面 Wails 和 CLI 模式不提供本地 HTTP 端口；Linux WebUI build tag 会启动 `/api/*` HTTP API。
- Wails bridge 不是 HTTP REST API，而是桌面运行时内的 JS 到 Go 调用；WebUI API 见 [接口契约](./api-contracts.md#2-webui-http-api)。
- 追踪探测、下载测速和 MICS抽样会通过自定义 Dial 目标让请求命中指定 IP。
- 后端 HTTP 出口统一走 `internal/httpclient`；默认优先尝试 HTTP/3，失败后回退到 TCP 上的 HTTP/1.1/2，非安全写操作不做跨协议重试。Linux ARM 和 Android 上 `auto` 会直接使用 TCP，避免首个 IP 先付一次 QUIC 握手超时。
- 阶段 3 测速 GET 会追加随机测速参数，并设置 `Cache-Control: no-store` / `Pragma: no-cache`；同时校验响应长度、Range 与可用的 Digest/MD5/SHA256 头。文件测速仅跟随同源重定向，并通过 Content-Type 与内容嗅探拒绝 HTML 默认页、nginx 管理面板或登录页。Cloudflare DNS、GitHub Release、更新包下载和签名 URL 不追加随机参数。
- Host Header、SNI、User-Agent、TLS 校验和调试抓包目标由 `internal/httpcfg` 统一处理；抓包目标仅在调试开启且 `debug_capture_enabled=true` 时覆盖拨号地址。

### 1.2 DNS API

当前产品通过 GUI/WebUI/Android 的 DNS 读取页直接调用 Cloudflare DNS 列表 API，不再维护外置示例脚本入口。DNS 读取页只执行查询；创建、更新和删除由手动结果推送、定时任务 DNS 推送和测速后自动推送中的 Cloudflare 操作触发。

| 用途 | 方法 | 地址 | 调用位置 |
| --- | --- | --- | --- |
| 查询 DNS 记录 | `GET` | `https://api.cloudflare.com/client/v4/zones/{ZONE_ID}/dns_records` | `internal/appcore/cloudflare_service.go`、`internal/cloudflarecore/` |
| 创建 DNS 记录 | `POST` | `https://api.cloudflare.com/client/v4/zones/{ZONE_ID}/dns_records` | `internal/appcore/cloudflare_service.go`、`internal/cloudflarecore/` |
| 更新 DNS 记录 | `PUT` | `https://api.cloudflare.com/client/v4/zones/{ZONE_ID}/dns_records/{DNS_RECORD_ID}` | `internal/appcore/cloudflare_service.go`、`internal/cloudflarecore/` |
| 删除 DNS 记录 | `DELETE` | `https://api.cloudflare.com/client/v4/zones/{ZONE_ID}/dns_records/{DNS_RECORD_ID}` | `internal/appcore/cloudflare_service.go`、`internal/cloudflarecore/` |

## 2. 输入与输出

### 2.1 输入

| 输入类型 | 说明 |
| --- | --- |
| 桌面输入源 | URL、本地文件、手动输入，支持 `traverse` 和 `mcis` |
| 桌面/WebUI 配置 | `desktop-config.json` 中的 `probe`、`export`、`backup`、`cloudflare`、`sources`、`ui` |
| Android 配置 | `mobile-config.json`，结构与桌面配置快照保持同构 |
| 配置归档 | JSON 或 ZIP 配置包，包含配置快照和 source profiles |
| IP 列表文件 | 默认 `ip.txt`，也支持 `ipv6.txt` 或自定义文件 |
| WebUI 请求 | `/api/command/{command}` JSON payload、`/api/platform/{command}`、SSE 订阅和受限文件路径 |

### 2.2 输出

| 输出类型 | 说明 |
| --- | --- |
| GUI 任务看板 | 展示任务状态、进度、警告、过程追踪和导出历史 |
| 当前结果页 | 展示本次测速结果、排序过滤、单条重测和导出位置 |
| Wails/SSE/Capacitor 事件 | `probe:event` 推送预处理、进度、导出、完成、终止、失败和暂停等待事件 |
| CSV 文件 | 默认 `result.csv`，可通过桌面配置修改 |
| 配置文件和归档 | `desktop-config.json`、`source-profiles.json`、JSON 导出和 ZIP 归档 |
| 调试日志 | 开启调试时写入 `cfip-log.txt`，默认 JSONL，可选自由格式文本 |
| Cloudflare DNS | DNS 页可读取线上记录；手动结果推送、定时任务和测速后自动推送可覆盖匹配记录 |
| 发行版安装包 | GitHub Release 输出 Windows、Linux、Android 资产和 `cfst-gui-update-manifest.json`，不发布 macOS 或 iOS 资产 |

结构化调试日志每行是一条 JSON 事件，固定包含 `ts`、`level`、`event`，并在探测任务中携带 `task_id` 和阶段字段。自由格式日志使用 `debug_log_format` 中的 `{field}` 占位符从同一份脱敏后的顶层事件字段渲染文本行，默认模板为 `{ts} [{level}] {event} task={task_id} stage={stage} {message}`。`debug_log_verbosity=detailed` 保持现有完整事件，核心事件为 `probe.start`、`stage.start`、`stage.complete`、`probe.export`、`probe.complete`、`probe.failed`，并可包含 `stage.detail` 中间细节；`debug_log_verbosity=simple` 只记录 `probe.start`、`stage.complete`、`probe.export`、`probe.complete`、`probe.failed`。

常见字段包括 `stage`、`message`、`duration_ms`、`counts`、`config`、`source`、`ip`、`colo`、`tcp`、`head`、`get`、`reason`、`error`。日志会脱敏 API Token、Telegram Bot Token、Authorization、Cookie、密码、Secret 以及 URL query 中疑似 token/signature/auth 的参数；调试日志和诊断包导出也会再次清理历史日志中嵌入 URL 路径的 Telegram Bot Token，诊断配置摘要不会包含 Telegram Bot Token 或聊天 ID。

当前产品不会生成外置示例脚本文件；导出结果、配置归档和调试日志均通过 GUI/WebUI/Android 流程管理。
