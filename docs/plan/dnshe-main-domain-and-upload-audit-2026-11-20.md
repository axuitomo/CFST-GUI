# DNSHE 审计：主域名记录识别与上传

对象：`internal/dnshecore`、`internal/appcore/dns_records_list.go`、`internal/dnspush`。
方式：代码逐层比对 + DNSHE API V2.0 文档 + 本机账号只读实测。**未执行任何写请求，未改动配置。**

## 修复状态（2026-11-20）

- **问题 1 已修复**：`dnspush.AbsoluteName` 增加「记录名恰好等于 zone」分支，顶点返回原域名；DNS 页读取与 `RelativeName` 同步覆盖 `@`。回归测试见 `internal/dnspush/dnspush_test.go`。
- **子域名入口已补齐**：DNSHE 配置新增 `subdomain`（标签或完整域名），优先级 `subdomain` > `subdomain_id` > `record_name`；设置页可读取账号子域名并可视化选择，`subdomain_id` 保留为兼容字段。
- **分流已泛化**：Cloudflare、DNSHE、阿里云 DNS、DNSPod 都支持分流规则；DNSHE 只填子域名、不填记录名时仍会推送子域顶点。
- **未在本轮改动**：问题 2 的顶点 CNAME 语义冲突仍保留，推送前仍需人工确认目标不是 Cloudflare for SaaS 别名；CNAME 模式未实现。

## 0. 结论

1. **主域名记录识别：确认缺陷（P0，用户可见）**
   `dnspush.AbsoluteName` 缺少「记录名恰好等于 zone」的分支。DNSHE 返回的顶点记录名是完整域名（实测 `axuitomo.l.cd`），会被拼成 `axuitomo.l.cd.axuitomo.l.cd`；DNS 页再按配置名精确过滤，主域名的 A/CNAME 记录就全部消失。表现是「上传成功了，但读取主域名一条都没有」。
2. **上传：确认高危语义冲突（P0，会改坏现网别名）**
   本机 DNSHE 顶点不是 A 记录，而是一条 `CNAME → test.xyz9923.qzz.io`，同域还有 `_cf-custom-hostname`、`_acme-challenge` 两条 TXT，属于 Cloudflare for SaaS 自定义主机名的标准布置。共享 `Reconcile` 把同名 CNAME 当阻塞记录，推送会**先删 CNAME 再写 A/AAAA**，摘要仍显示成功。Cloudflare 路径碰不到「目标名本身就是别名」这种形态，所以两边行为不一样。
3. **其余问题**：`SubdomainID` 与记录名不校验、读取用配置名而不是查询名、写接口 `name` 口径未按文档、30 请求/分钟无退避、`rootdomain` 解析后从未使用。

## 1. 实测证据（只读）

| 项 | 实测值 |
| --- | --- |
| `subdomains/list` | `id=1668828352`、`subdomain=axuitomo`、`rootdomain=l.cd`、`full_domain=axuitomo.l.cd`、`status=Registered` |
| `dns_records/list` | 3 条：`axuitomo.l.cd` CNAME → `test.xyz9923.qzz.io`；`_cf-custom-hostname.axuitomo.l.cd` TXT；`_acme-challenge.axuitomo.l.cd` TXT |
| `dnspush.AbsoluteName("axuitomo.l.cd","axuitomo.l.cd")` | 返回 `axuitomo.l.cd.axuitomo.l.cd`（临时单测确认，测试文件已删除） |
| `dnspush.AbsoluteName` 其余输入 | `@`/空 → `axuitomo.l.cd`；`www` → `www.axuitomo.l.cd`；`www.axuitomo.l.cd` → 原样 |

DNSHE 官方文档（V2.0）关键口径：

- `dns_records/list` 返回的 `name` 是**完整域名**（示例 `test.example.com`、`www.test.example.com`）。
- `dns_records/create` 的 `name` 明确支持 `@`、留空（=当前子域本身）与完整域名；相对前缀未在文档中承诺。
- 认证只允许 `X-API-Key` / `X-API-Secret` 头；默认速率限制 30 请求/分钟。

## 2. 主域名记录识别问题

### 2.1 P0：顶点记录被拼成双重域名

`internal/dnspush/dnspush.go:106-119`：

```go
case normalized == "" || normalized == "@": return zone
case zone != "" && strings.HasSuffix(normalized, "."+zone): return normalized
case zone == "": return normalized
default: return normalized + "." + zone
```

缺少 `normalized == zone`。DNSHE 的顶点记录名正好等于 zone，于是走 default。调用点在 `internal/appcore/dns_records_list.go:211`，只有 DNSHE 分支使用该函数；Cloudflare 直接用返回的全名，阿里云/DNSPod 返回的是相对 label，因此只有 DNSHE 顶点中招。

影响面：

| 读取范围 | 结果 |
| --- | --- |
| `configured`（默认） | `filterDNSRecordSnapshots` 按 `axuitomo.l.cd` 精确匹配，顶点记录被丢弃 → 主域名显示 0 条 |
| `custom` 输入 `axuitomo.l.cd` | 同上，0 条 |
| `zone`（全部记录） | 不按名过滤，但顶点记录显示成 `axuitomo.l.cd.axuitomo.l.cd`，看起来像多出一层子域 |

现有测试 `internal/dnspush/dnspush_test.go:30-49` 只覆盖 `@`、`api`、`a.b`、相对全名，没有覆盖 `name == zone`，所以一直没暴露。

修法：加一个分支 `case normalized == zone: return zone`，并补三条断言（`zone`、`@`、`""` 都回到 zone；`a.b` 仍展开；外部全名仍原样）。

### 2.2 P1：`rootdomain` 没有参与识别

`internal/dnshecore/dnshe.go:45,113,197,230` 解析了 `RootDomain`，但全仓库再无引用。`ResolveSubdomain`（`:238-264`）只匹配 `full_domain`：

- 用户填的是主域名（如 `l.cd`）时，报「找不到匹配的子域名，请先在自由域名管理中注册」。`l.cd` 是 DNSHE 的公共根域名，用户根本注册不了，提示误导。
- 若某次响应缺 `full_domain`，也没有用 `subdomain + "." + rootdomain` 兜底。
- `status` 也没校验（实测值是 `Registered`，不是文档过滤项里的 `active`），悬挂/过期域会直接进入写流程后失败。

修法：`full_domain` 为空时用 `subdomain.rootdomain` 组合；命中 `rootdomain` 时给出明确错误（「这是主域名，只能管理你注册的子域名」）；写前检查状态，异常只读提示不写。

## 3. 上传问题

### 3.1 P0：同名 CNAME 被当成阻塞记录删除（与 Cloudflare 的核心差异）

链路：

1. `dnshecore.Push` → `pushBackend.ListRecords`（`dnshego.go:441-457`）按完整名过滤，顶点 CNAME 被收进集合。
2. `dnspush.Reconcile`（`dnspush.go:182-245`）调用 `blockingRecords`，把所有 CNAME 视为阻塞记录：**先 DELETE，再逐条 CREATE A/AAAA**；失败才回滚重建 CNAME。
3. 成功后摘要为「创建 N、删除 1」，看不出别名已被改写。

本机现网的含义：`axuitomo.l.cd CNAME test.xyz9923.qzz.io` 是 Cloudflare for SaaS 的正常接入形态（`_cf-custom-hostname` 负责主机名预验证，`_acme-challenge` 负责证书）。一旦推送：

- DNSHE 允许删除 → CNAME 消失，顶点变成直连 A 记录；Cloudflare 侧的自定义主机名失去 CNAME 链，后续验证/续期/摘除都按「直连 A」处理，和原设计不一样。
- DNSHE 拒绝删除 → 推送在 `delete` 阶段失败，回滚重建 CNAME；用户看到「推送失败」。

两种结果都不是用户要的。且应用内自检的 `cname` 阶段（`dns_selftest.go:1168-1189`）正在把「删 CNAME 再写 A」钉成正确行为，无法发现这个场景。

建议二选一（需要产品决策）：

- **保守**：DNSHE 目标名下发现 CNAME 时直接报错，不自动替换；提示「这是别名记录，请改用 CNAME 模式或停用该平台」。
- **完整**：给 DNSHE 增加 `cname` 托管模式：desired 是一条 `CNAME → <Cloudflare 记录名>`，`EquivalentRecord` 比较 content，永不写 A/AAAA，也不把该 CNAME 当阻塞记录。

在决策前，**建议先停用当前 DNSHE 平台**：它现在是启用状态，下一次手动/自动/定时推送就会改写这条别名。

### 3.2 P1：`SubdomainID` 与记录名不校验，会静默写到别的顶点

`resolveTarget`（`dnshego.go:416-433`）在 `SubdomainID > 0` 时直接 `GetSubdomain`，不再验证 `cfg.RecordName` 是否属于该子域；`relativeRecordName`（`:512-525`）对「不属于该子域的名字」直接返回 `@`：

```go
if subdomainFullDomain == "" || recordName == "" || recordName == subdomainFullDomain { return "@" }
if HasSuffix(recordName, "."+subdomainFullDomain) { ... }
return "@"   // ← 不匹配也落到顶点
```

配置里粘错子域 ID、或改了记录名没清 ID，就会把 A/AAAA 写到另一个子域的顶点，且摘要正常。阿里云/DNSPod 的口径是「域名不在账号下直接报错」，这里应保持一致：解析后强制校验 `RecordName == FullDomain` 或是其后缀，否则报配置错误；`relativeRecordName` 不匹配时返回错误而不是 `@`。

### 3.3 P1：写接口的 `name` 口径比 Cloudflare 窄

Cloudflare 写的是完整记录名；DNSHE 当前把完整名转成相对前缀（顶点转 `@`）。官方文档只明确承诺 `@`、留空、完整域名三种写法，相对前缀没有文本保证。顶点（当前目标）没问题；一旦目标改成 `www.axuitomo.l.cd`，请求体就是 `name:"www"`，属于未验证口径。

建议：非顶点一律直接发完整域名（文档已承诺），顶点用 `@`；删除 `relativeRecordName` 的 `@` 兜底，改成「不匹配即报错」。这样也与 Cloudflare 的 `name` 语义对齐。

### 3.4 P1：30 请求/分钟 + 无退避 + 全量列子域

- `ListSubdomains`（`:180-207`）固定 `per_page=500`，按 `has_more` 最多翻 100 页。账号子域多时一次推送就可能打出上百个请求，直接超过 30/分钟的默认限制。
- `Push` 一次典型请求：列子域 1 + 列记录 1 + 删/建 N + 回读 1。TopN=5、A only 约 9 次，安全；TopN>25 或双族就会压线；自动推送与定时任务同分钟叠加更容易 429。
- `Client.do`（`:565-610`）对 429 不重试、不退避，直接把错误抛给上层；`apiError` 只认 `message`，限流响应里的 `error` 字段会变成「DNSHE API 返回失败」，看不出是限流。

建议：`ResolveSubdomain` 用 `search` / `rootdomain` / `fields=id,subdomain,rootdomain,full_domain,status` 精确查询；优先保存 `subdomain_id`；写循环串行并做 1 次/秒级节流；429 退避重试；解析 `error` 字段。

### 3.5 P2：ID 与返回结构

- `apiRecord.ID int`（`:141-151`）实测值 `578665182055606` 已超过 2^31。桌面/Android arm64 的 `int` 是 64 位，暂时安全；32 位构建会在 JSON 解码时溢出。建议统一 `int64`。
- `dnshecore.Record` 没有 JSON tag，`records_after` 序列化后是 `Content/ID/Name/TTL/Type` 大写键；Cloudflare 的 `CloudflareDNSRecord` 是 snake_case。目前前端未消费该字段，但接口形状已经分叉。
- `RecordsAfter` 返回整个子域、全类型（Cloudflare 只返回目标名的 A/AAAA），跨平台拿它做校验会多算；已在 `dns-compat-check-2026-11-20.md` P1-1 记录。

## 4. 与 Cloudflare 的差异对照

| 维度 | Cloudflare | DNSHE |
| --- | --- | --- |
| Zone 身份 | `zone_id` 固定，记录名是 zone 内任意主机 | 注册的二级域名就是 zone，`record_name` 必须落在其下 |
| 顶点记录 | 直接返回完整名，无换算 | 返回完整名，但读取时用 `AbsoluteName`，顶点被双重拼接 |
| 同名 CNAME | 按阻塞记录删除重建（目标名通常是独立主机） | 目标名本身就是 Cloudflare 自定义主机名的 CNAME，删除即改坏别名 |
| 记录名写法 | 写完整名 | 当前写相对前缀，文档只承诺 `@`/空/完整名 |
| 读取范围 | `RecordsAfter` 只含目标名 A/AAAA | `RecordsAfter` 含整个子域、全类型 |
| 速率 | 宽松 | 30 请求/分钟，无退避 |
| 类型 | A/AAAA/CNAME，UI 支持 ALL | 仅 A/AAAA 托管；CNAME 只会被删 |
| TTL | 60/300/600 白名单 | 默认 600，仅下限约束 |

## 5. 建议落地顺序

1. `AbsoluteName` 补 `name == zone` 分支 + 三条测试（零风险，先修「主域名读不到」）。
2. `resolveTarget` / `relativeRecordName` 改为「不匹配即错误」，禁止 `@` 兜底写错顶点。
3. DNSHE CNAME 策略二选一（保守报错 / CNAME 模式）；在此之前停用 DNSHE 平台，避免下一次推送改写别名。
4. 读取按查询名解析子域（`custom` 跨子域），并把「不属于当前子域」改成明确错误而不是空列表。
5. 写接口改发完整名；`ResolveSubdomain` 收窄查询；429 退避与节流。
6. `int64`、`records_after` JSON tag、`rootdomain/status` 兜底与提示。

## 6. 验证

```powershell
go test ./internal/dnspush/... ./internal/dnshecore/... ./internal/appcore/...
```

手工验收：

1. DNS 页选 DNSHE、`configured` 范围读取：顶点记录显示为 `axuitomo.l.cd`，不是 `axuitomo.l.cd.axuitomo.l.cd`，且条数包含顶点。
2. 只在 `cfst-selftest.axuitomo.l.cd` 上做上传自检，不动顶点；确认 CNAME 阶段的语义不会碰生产别名。
3. 顶点仍为 CNAME 时，DNSHE 推送应明确拒绝或走 CNAME 模式，不得出现「创建 N、删除 1」的成功摘要。
