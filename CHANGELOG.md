# 更新日志

本文件记录 CFST-GUI 的用户可见变更，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循语义化版本。每个版本更完整的验证命令和发行资产见 [`docs/release-notes/`](docs/release-notes/README.md)。

## [2.0.4] - 2026-10-29

2.0.3 的 GitHub Release 已撤回（原 Release 转为 draft），代码与 `v2.0.3` 标签保留在仓库中。撤回原因是文件测速实时速率读数异常（0 与尖峰交替、收尾显示 `-`），修复见下文「修复」一节，随后以 2.0.4 重新发布。

### 新增

- DNS 设置从「系统配置 → 高级 → 网络与任务 → 第一阶段」的普通字段抽成独立卡片「网络与任务 → DNS 与 ECH」，集中放 DoH 地址（含三个预设与自定义）、ECH 开关、ECH 域名和 ECH 配置；折叠标题显示当前 DoH 预设，修改沿用设置页的自动保存。
- 新增 `doh.test` 命令与「测试 DoH」按钮：用当前配置的端点解析测试主机并报告 IPv4/IPv6 数量，再按手填配置、来源主机 HTTPS RR、ECH 域名的顺序报告 ECHConfigList 来源与字节数。测试路径不回退系统 DNS，端点故障会明确报错；合法 NXDOMAIN 视为端点可用；非法 ECH 域名或 base64 配置降级为自动获取并给出警告。
- 文件测速 URL 从「探测策略 → 第三阶段」的高级专属区域提到「探测策略」顶部，普通设置里也能直接改；字段只有一份，高级设置看到的是同一处。追踪 URL 留空时仍由该地址派生 `/cdn-cgi/trace`。
- 403 与 1034 拆成两个独立开关：「屏蔽 403」（`probe.ignore_403` / `-ignore-403`，默认开启）与「屏蔽 1034」（`probe.ignore_1034` / `-ignore-1034`）分别决定两类响应的处理。屏蔽即跳过：不重试、不计入连续失败冷却；不屏蔽则按普通失败处理：按 `retry_policy` 重试并计入冷却。1034 判定优先于 403，屏蔽 403 不会连带屏蔽 1034。阶段 1 HTTPing 遇到 403 会补一次小范围 GET 读正文来区分 1034 和普通 403。
- 两种开关设置下，没测到速率的 IP 都不会进入结果：403 / 1034 只是被拒绝的原因，不是保留理由。日志里用 `reason=edge_ip_restricted` / `forbidden` 加 `note` 字段标出是哪一类，`cfip-log.txt` 打开 debug 后可按 reason 筛出这些 IP。
- 新增 `probe.edge_rejected` 事件：403 / 1034 淘汰会同时发一条不受调试开关约束的界面事件，进程面板直接显示「边缘 IP 受限 1034」或「HTTP 403 拒绝」。此前只能打开调试日志写 `cfip-log.txt` 才能在 `stage.reject` 里看到 1034 标识。

### 变更

- `probe.ignore_1034` 默认值从 `true` 改为 `false`：边缘 IP 受限不再默认跳过，而是按普通失败处理（重试并计入冷却）。旧配置里显式写入的值继续保留；升级后如需维持「不重试」的旧行为，请在设置页勾选「屏蔽 1034」或传 `-ignore-1034`。
- 结果 CSV 统一为 10 列：`IP 地址、已发送、已接收、丢包率、TCP延迟(ms)、平均速率(MB/s)、最高速率(MB/s)、地区码、追踪延迟(ms)、备注`。任务自写的 `result.csv` 与结果页「导出 CSV」从此是同一个形状（此前前者 8 列、后者 9 列且末列不同）。旧文件仍可按表头读取；追加模式下新写入的行会比旧表头多列。
- 阶段 1 HTTPing 的 403 淘汰改用 `reason=forbidden`（此前与其它状态码不匹配共用 `status_mismatch`），这样两类边缘拒绝在 `stage.reject`、`probe.edge_rejected` 和文档里用同一套名词；非 403 的状态码不匹配仍是 `status_mismatch`。
- 自定义 Host / SNI 改为默认关闭：新增 `probe.custom_host_sni` 开关，关闭时 `host_header`、`sni`、`download_host_header`、`download_sni` 一律不生效，Host 与 SNI 自动跟随 URL（追踪阶段跟随 `trace_url`），行为与只填 URL 完全一致；设置页四个输入框在开关关闭时置灰。旧配置没有该字段时，只要填过任一字段就按开启处理，升级前后请求身份不变；CLI 的 `-host` / `-sni` 传入即自动开启。

### 修复

- 设置页第一次展开或收起卡片会整页抖一下：展开动效首次开启时给所有 `.settings-mode-enter` 元素（包含全部折叠卡片）挂了 `block-size: 0 → auto` 动画，卡片会一起塌到 0 再长回来；现在这张动画只作用于切换普通/高级时新出现的分区，卡片本身只用 `::details-content` 的高度过渡，并且该过渡常开，第一次 toggle 也能平滑推开而不再硬跳。内容淡入也改为只跟随被点击的那张卡片。
- 文件测速实时速率改为最近 1 秒滑动窗口，刷新节奏与窗口对齐（`download_speed_sample_interval_ms` 默认改为 `1000`，更小的值按 1 秒执行）；读数不再在 0 与尖峰之间跳动，收尾采样也不再显示 `-`，最高速率同样取窗口速率且不会低于平均速率。

## [2.0.3] - 2026-10-29（已撤回）

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
- 桌面端上传结论的系统通知恢复发送：前端此前调用 Wails v2 的 `window.runtime` 接口，在 v3 宿主下必然抛错并只留下 `notification.desktop_failed` 日志；现在改由 Go 侧在 `upload.notification` 事件上调用 Wails v3 通知服务，主窗口隐藏在托盘时同样能收到。通知初始化失败只写 `error-log.txt`，不影响启动与测速。
- Windows 发行构建与 `task build` 补齐 Wails v3 的 `production` 标签（macOS 的 `wails3 build` 原本就会自动追加）。缺少该标签时发行包会编入 Vite 反代路径与调试开关。

## 更早版本

| 版本 | 日期 | 摘要 |
| --- | --- | --- |
| [v2.0.2](docs/release-notes/v2.0.2.md) | 2026-10-09 | 多渠道上传通知（Telegram、Webhook、钉钉、企业微信、Email）、MICS 抽样预算与下载成功数上限、Windows 系统代理、HTTP/2 超时修复、oxlint + oxfmt。 |
| [v2.0.1](docs/release-notes/v2.0.1.md) | 2026-09-20 | 修复 Android WebView 渲染进程被回收导致的进程终止，消除冷启动白屏，清理 CodeQL 告警。 |
| [v2.0.0-fix](docs/release-notes/v2.0.0-fix.md) | 2026-09-19 | 修复 bridge 通道判定、Windows WebView2 数据目录与移动端帮助提示。 |
| [v2.0.0](docs/release-notes/v2.0.0.md) | 2026-09-19 | 设置页分区帮助、前端缓存策略、启动诊断、桌面/CLI 启动模式拆分、质量门禁与文档拆分。 |

全部历史版本见 [`docs/release-notes/README.md`](docs/release-notes/README.md)。
