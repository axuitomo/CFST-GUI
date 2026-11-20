# 审计报告：DNS 多平台提交（0b98662）

审计对象：`chore/deps-stable-2026-11` 上今天的两笔提交。

| 提交 | 内容 | 文件规模 |
| --- | --- | --- |
| `3f9171c` | 输入组抽样术语统一为 MICS 抽样 | 7 个文件 |
| `0b98662` | 多平台 DNS 读取 / 分流 / DNSHE 子域名 / 平台自检 | 46 个文件，+4297 / -118 |

审计方式：逐文件 diff、调用链核对、配置归一化核对、测试与构建验证。本次审计只读代码和本地测试，没有对任何真实 DNS 平台发起写请求。

## 0. 结论

1. 用户可达的推送 / 读取 / 配置链路没有发现新增 P0 缺陷；`record_type` 契约、DNSHE 顶点记录、跨子域写入拒绝都与前一份审计的落地记录一致。
2. 仍有一个**已知但未修复的 P0 风险**：DNSHE 子域顶点如果是 CNAME（Cloudflare for SaaS 自定义主机名），推送 A/AAAA 会先删除 CNAME。新的「只填子域名、不填记录名」路径让这个顶点成为默认主目标，暴露面比之前更大。
3. 新增自检存在 2 项 P1：DNSHE 只填子域名时自检必定失败；DNSHE 单次自检请求量约 45-50 次，超过 30 请求/分钟且没有 429 退避，失败后清理也可能被限流。
4. 自检没有单飞锁，两个窗口同时执行会互删测试记录。
5. 文档口径已在本轮审计中同步修正（见第 5 节）。
6. 上述 2.1、3.1、3.2 与第 4 节 P2 已在后续改动中修复，状态见第 8 节。

## 1. 验证结果

全部通过，未发现编译、类型、格式或现有测试回归。

```
gofmt -l internal/appcore internal/dnshecore internal/dnspush internal/cloudflarecore internal/alidnscore internal/dnspodcore
go test ./internal/appcore/... ./internal/dnshecore/... ./internal/dnspush/... ./internal/cloudflarecore/... ./internal/alidnscore/... ./internal/dnspodcore/...
go test -race ./internal/appcore/
pnpm typecheck
pnpm lint
pnpm test          # 18 个测试文件、87 个用例
pnpm check:vapor   # 15 个 SFC 全部 Vapor
pnpm build         # vue-tsc + vite 生产构建
```

未执行：真实平台写入手测、浏览器回归和视觉基线。原因是写入手测会改线上记录，浏览器/视觉基线需要 CI Windows runner 生成。

## 2. P0（遗留，非本次引入）

### 2.1 DNSHE 顶点 CNAME 会被 A/AAAA 推送覆盖

证据：

- `dnshecore.Push` 在 `cfg.RecordName` 为空时把目标改成 `subdomain.FullDomain`（`internal/dnshecore/dnshe.go:430-432`）。
- `dnsProviderHasPrimaryTarget` 对 DNSHE 只要填了 `subdomain` 或 `subdomain_id` 就认为有主目标（`internal/appcore/dns_provider_routing.go:159-164`）。
- `dnspush.Reconcile` 把同名 CNAME 当作阻塞记录，先删除再创建 A/AAAA；前一份审计已在真实账号确认顶点是 `CNAME → test.xyz9923.qzz.io`。

影响：用户只填子域名时，主目标就是该子域顶点；如果顶点是 Cloudflare for SaaS 的 CNAME，一次推送会删除别名并写入 A/AAAA，摘要仍显示成功。自检的 CNAME 阶段只覆盖 `cfst-selftest.*` 测试名，不能提前发现生产顶点的这种形态。

建议（按成本从低到高）：

1. DNSHE 推送前对主目标做一次只读检查，目标名存在 CNAME 且目标为 A/AAAA 推送时返回 `DNS_CONFIG_INVALID`，要求用户显式换目标或先删除别名。
2. 设置页在 DNSHE 主目标为子域顶点时给出持续提示，不靠文档兜底。
3. 实现 CNAME 模式（保留别名、不写 A/AAAA），作为后续独立需求。

## 3. P1

### 3.1 DNSHE 只填子域名时无法运行自检

证据链：

- 后端允许 `record_name` 为空、只填 `subdomain`（`internal/appcore/dns_providers.go:297-301`）。
- 自检记录名仍要求 `provider.RecordName` 非空（`internal/appcore/dns_selftest.go:204-207`），否则返回「当前平台没有填写推送记录名」。
- 前端把 `recordName || subdomain` 作为自检基准（`frontend/src/App.vue:2158-2170`），因此会显示一个可点击的 `cfst-selftest.edge`，但后端必定拒绝。
- `dnsheSelfTestAdapter` 解析子域时只看 `SubdomainID` / `RecordName`，忽略 `Subdomain`（`dns_selftest.go:1105-1123`），与推送侧 `subdomain > subdomain_id > record_name` 的优先级不一致。

影响：DNSHE 子域名选择是本次新增的主推能力，但这类配置的自检完全不可用；如果只修基准名而不修适配器，还可能测到与推送不同的子域，给出假通过。

建议：

1. 自检入口先按 `subdomain` 解析出完整域名，作为自检基准；解析失败按配置错误返回。
2. DNSHE 适配器复用与 `resolveTarget` 相同的优先级，并缓存解析结果。
3. 增加「只填子域名」的自检用例（假 DNSHE 服务端即可）。

### 3.2 DNSHE 自检请求量超过 30 请求/分钟，且没有 429 退避

按代码路径估算，单次 DNSHE 自检约 45-50 次 API 请求：baseline/reset/各阶段 read、每次 Push 内部的解析子域 + 列记录 + Reconcile 列记录 + 终态刷新、CNAME 创建与清理。DNSHE 默认限制 30 请求/分钟，`internal/dnshecore` 没有 429 识别、退避或节流（前一份审计 3.4 已记录）。

影响：自检很可能在换血或属性更新阶段被限流打断；清理阶段同样被限流时，测试记录会残留，正好落在自检最需要可靠的地方。阿里云、DNSPod、Cloudflare 的限制更宽松，不受此条影响。

建议：

1. 适配器缓存子域 ID，Push 时不再重复 `GetSubdomain`。
2. 为 DNSHE 客户端加 1 请求/秒级节流和 429 退避（可在 selftest 外层包一层）。
3. 或在自检入口限制 DNSHE 每分钟只允许启动一次，并在阶段之间留最小间隔。

## 4. P2 / P3

| 级别 | 问题 | 证据 | 影响 / 建议 |
| --- | --- | --- | --- |
| P2 | 自检没有单飞锁 | `dns.selftest` 直接进入 `RunDNSPlatformSelfTest`（`invoke.go:53-58`） | 桌面 + WebUI 或两个 WebUI 标签同时运行同一测试名时，reset/cleanup 会删掉对方刚写入的记录。建议按 provider + 测试名加互斥，重复请求返回 `DNS_SELFTEST_BUSY` |
| P2 | `wrote_records` 语义偏宽 | `reset` 删除残留、CNAME 前清理地址记录都会置 true（`dns_selftest.go:513-517,579-591`） | 未写入也会显示「已清理测试记录」，与「未写入测试记录」互斥文案不一致。建议增加 `touched_records`，或只在实际 create/update 后置 true |
| P3 | `dns.subdomains` 省略 `provider_id` 时选第一个启用平台 | `dns_subdomains.go:20-34` | 第一条不是 DNSHE 时返回「只有 DNSHE 支持」，对 WebUI 直接调用不友好。建议省略时选第一条 DNSHE，仍找不到再报错 |
| P3 | 已停用平台仍可读取 | `invokeDNSRecordsList` 不检查 `Enabled`，前端 `readButtonDisabled` 只看是否有选中项 | 页面显示「已停用」但读取按钮可用。只读操作本身无害，建议明确这是「允许只读、禁止写入」还是禁用按钮 |
| P3 | 自检结果不会随平台切换清空 | `DnsSelfTestPanel` 切换平台只重置测试名与确认框 | 旧平台的结果仍显示在新平台下方。建议切换时通知父组件清空 `dnsSelfTestResult` |

## 5. 文档同步（本轮已修正）

提交 `0b98662` 更新了大部分文档，但仍有 5 处把 DNS 读取写成 Cloudflare 专用，与多平台读取矛盾。本轮审计已修正：

| 文件 | 修正 |
| --- | --- |
| `docs/guide/configuration.md` | 「只读取 Cloudflare 记录」改为按所选平台读取四个平台 |
| `docs/reference/README.md` | 「已接入 Cloudflare 官方 API」改为四个平台的读取接口 |
| `docs/reference/external-interfaces.md` | 说明读取页调用所选平台 API，下表只列 Cloudflare 地址 |
| `docs/reference/capabilities-and-flows.md` | 能力表改为所选平台读取 |
| `docs/integration/cloudflare-api-token.md` | 把「DNS 读取页只调用 Cloudflare」限定为选择 Cloudflare 平台时 |

## 6. 测试缺口

1. DNSHE / 阿里云 / DNSPod 的自检适配器没有假服务端集成用例；目前只有 Cloudflare 全流程用例和 DNSHE 名称换算单测。
2. 没有「只填子域名」的自检用例，因此 3.1 没有被测试暴露。
3. 没有并发自检用例，4 节的互删风险没有回归保护。
4. `DnsSelfTestPanel` 的前端校验（前缀、子域关系、确认勾选）没有组件测试。
5. 没有 429 / 限流场景的用例。

## 8. 后续修复状态（2026-11-20）

用户确认产品口径：新增「允许修改 CNAME」开关，默认关闭，四个平台统一；关闭时拒绝覆盖且不删除别名。

| 原问题 | 状态 | 落地 |
| --- | --- | --- |
| 2.1 DNSHE 顶点 CNAME 被推送删除 | 已修复 | `dnspush.ReconcileWithOptions` + `ErrCNAMEProtected`；四平台 `Config.AllowCNAME` 默认 `false`；错误码 `DNS_CNAME_PROTECTED`；设置页复选框可显式开启 |
| 3.1 DNSHE 只填子域名时自检失败 | 已修复 | 自检入口与适配器统一按 `subdomain > subdomain_id > record_name` 解析；`record_name` 为空时用子域完整域名作为测试基准 |
| 3.2 DNSHE 自检请求量超限且无 429 退避 | 已修复 | `Client` 支持 `SetRequestInterval`；429 按 `Retry-After` 或 1s/2s/4s 指数退避，最多 3 次；自检客户端固定 2s 间隔并复用 `PushResolved` |
| P2 自检无单飞锁 | 已修复 | `RunDNSPlatformSelfTest` 使用 `TryLock`，重复执行返回 `DNS_SELFTEST_BUSY` |
| P2 `wrote_records` 语义偏宽 | 已修复 | 仅在本轮实际创建或更新记录后置 `true`；清理旧残留不再计入 |

新增回归测试覆盖：CNAME 默认拒绝与显式允许、DNSHE 429 退避/重试耗尽/请求间隔、子域名-only 自检命名、Busy 单飞、reset-only 不写 `wrote_records`、四平台 `allow_cname` 配置解析。

CNAME 托管模式（保留别名并写 CNAME）仍未实现，属于独立需求；当前默认行为是拒绝覆盖，不会改坏现网别名。

## 9. 建议落地顺序

1. 先修 3.1（功能不可用）和 4 节单飞锁，两者都只改自检入口与适配器，不动推送语义。
2. 再做 3.2 的 DNSHE 节流与 429 退避；这是自检能否在免费账号上稳定跑完的前提。
3. 2.1 的顶点 CNAME 防护需要产品口径（拒绝、提示或 CNAME 模式），建议单独开一轮，不要顺手改 `Reconcile`。
4. 其余 P3 可随下次 UI 调整一起处理。
