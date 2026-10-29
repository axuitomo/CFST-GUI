# 更新日志

本文件记录 CFST-GUI 的用户可见变更，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循语义化版本。每个版本更完整的验证命令和发行资产见 [`docs/release-notes/`](docs/release-notes/README.md)。

## [未发布]

### 修复

- 桌面端上传结论的系统通知恢复发送。此前前端调用的是 Wails v2 的 `window.runtime` 接口，在 v3 宿主下必然抛错并只留下 `notification.desktop_failed` 日志；现在改由 Go 侧在 `upload.notification` 事件上调用 Wails v3 通知服务，主窗口隐藏在托盘时同样能收到。通知初始化失败只写 `error-log.txt`，不影响启动与测速。
- Windows 发行构建与 `task build` 补齐 Wails v3 的 `production` 标签（macOS 的 `wails3 build` 原本就会自动追加）。缺少该标签时发行包会编入 Vite 反代路径与调试开关。

## [2.0.3] - 2026-10-20

### 新增

- DoH 输入源解析：新增 `probe.doh_url`（RFC 8484），同时解析输入源域名与来源 URL 主机名，A/AAAA 并行查询、4 秒预算、IPv4 优先；端点失败回退系统 DNS，合法 NXDOMAIN 不回退。DoH 客户端直连并固定 TCP；端点必须是 `https://`，非法值清空并给出配置警告。设置页提供阿里 / DoH.pub / Cloudflare 预设，CLI 为 `-doh`（留空即默认阿里 DoH）。
- 来源请求 ECH：按主机维护 TCP 传输，从同一 DoH 端点查询 HTTPS RR 的 `ech` 参数并挂到 TLS 握手，来源主机自有记录优先；无记录或被拒时回退普通传输并仍校验真实证书。新增 `probe.ech`、`probe.ech_config`、`probe.ech_domain`，CLI 为 `-ech`、`-ech-config`、`-ech-domain`。
- 新增 `probe.source_pool_limit`（默认 `65536`，上限 `1000000`）与 CLI `-pool-limit`，负 `ip_limit` 改为“展开到上限为止”。
- 下载测速并发上限从 1 提升到 16（默认仍为 1），设置页新增“并发测速 IP 数”。

### 变更

- 任务快照写盘按任务合并为每秒至多一次；结果页缓存已解码行，文件未变时不重复读取与排序。
- COLO 词典改为索引查询；HTTP/3 失效路径在进程内暂停 30 秒，避免每次探测重付 QUIC 握手代价。
- 候选池有界展开并在 stage 0 直接汇总候选、逐个来源上报进度；域名解析按次数（`limit × 4`，钳制 `1024..20000`）和 30 秒累计时长设预算，超限告警并标出截断。
- 结果页精简并默认按平均速度排序；仪表盘启动回退限定在乐观窗口内；前端只挂载当前 shell，进程条目改用稳定 key；设置页分区展开状态持久化。

### 修复

- 远端来源只走 TCP，避免慢 QUIC 握手吃满 30 秒来源超时后用过期的请求上下文回退。
- 来源遇到 429/5xx 重试一次并继续切换候选 URL；失败只更新状态文案，不再清空上一次成功读取的时间与数量；全部来源失败时补充直连提示。
- 探测阶段在后置检查通过后才计入 `completed_stages`。
- 调度器 `skip_if_active` 窗口内的 `PROBE_ALREADY_RUNNING` 记为跳过，不再让历史与下游 DNS/GitHub 状态显示失败。
- 配置写入串行化；来源状态就地更新，未知字段不丢失，缺少 `enabled` 键的来源视为启用。
- 移除事件发射器遗留的调试输出；恢复展开或重新进入设置页不再触发整页淡入。

## 更早版本

| 版本 | 日期 | 摘要 |
| --- | --- | --- |
| [v2.0.2](docs/release-notes/v2.0.2.md) | 2026-10-09 | 多渠道上传通知（Telegram、Webhook、钉钉、企业微信、Email）、MICS 抽样预算与下载成功数上限、Windows 系统代理、HTTP/2 超时修复、oxlint + oxfmt。 |
| [v2.0.1](docs/release-notes/v2.0.1.md) | 2026-09-20 | 修复 Android WebView 渲染进程被回收导致的进程终止，消除冷启动白屏，清理 CodeQL 告警。 |
| [v2.0.0-fix](docs/release-notes/v2.0.0-fix.md) | 2026-09-19 | 修复 bridge 通道判定、Windows WebView2 数据目录与移动端帮助提示。 |
| [v2.0.0](docs/release-notes/v2.0.0.md) | 2026-09-19 | 设置页分区帮助、前端缓存策略、启动诊断、桌面/CLI 启动模式拆分、质量门禁与文档拆分。 |

全部历史版本见 [`docs/release-notes/README.md`](docs/release-notes/README.md)。
