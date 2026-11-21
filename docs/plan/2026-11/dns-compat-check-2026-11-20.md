# 兼容性检查报告：DNS 读取 / 上传更新 / 记录更替

检查对象：`docs/plan/2026-11/dns-platform-test-flow-2026-11-20.md` 的测试口径，以及它依赖的四个平台实现（Cloudflare / DNSHE / 阿里云 / DNSPod）。
检查方式：逐包比对代码语义，并跑通现有测试。

```
go test ./internal/dnspush/... ./internal/cloudflarecore/... ./internal/dnspodcore/... ./internal/alidnscore/... ./internal/dnshecore/...
ok  （5 个包全部通过，Go 1.27.4 windows/amd64）
```

**结论**：口径改为「允许动 AAAA」后，原先的 P0（AAAA 越界删除）关闭，因为 A 与 AAAA 同属托管集合、整类收敛是既定语义。原第二条 P0（类型约束只在 appcore）经调用链复核后**降级为 P2**：产品可达的写路径都已先按 `record_type` 过滤，core 缺口只在直接调用 core 时成立。当前无 P0，另有 3 项 P1：`RecordsAfter` 范围不一致、DNSPod 线路重置、非 CNAME 路径无回滚。详细审计见 `dns-record-type-fix-audit-2026-11-20.md`。

## 1. 兼容矩阵

| 维度 | Cloudflare | DNSHE | 阿里云 DNS | DNSPod |
| --- | --- | --- | --- | --- |
| 读取过滤位置 | 服务端 `name` + `type` 过滤，`per_page=100` 翻页 | 拉整个 subdomain，客户端按完整域名过滤 | 拉整个 zone（500/页，最多 100 页），客户端按 name 过滤 | 拉整个 zone（100/页，offset < 10000），客户端按 name 过滤 |
| `RecordsAfter` 内容 | **仅目标 name 的 A/AAAA** | 整个 subdomain、全类型 | **整个 zone、全类型** | **整个 zone、全类型** |
| 更新方式 | `PATCH /dns_records/{id}` | `action=update`（全字段） | `UpdateDomainRecord`（全字段） | `ModifyRecord`（全字段） |
| 等价判定字段 | content + ttl + proxied + comment + name + type | type + name + content + ttl | type + name + content + ttl（忽略 line/status） | type + name + content + ttl（忽略 line/status） |
| 同名 CNAME | 删除后重建地址记录（带回滚） | 同左 | 同左 | 同左 |
| TXT / MX | 保留 | 保留 | 保留 | 保留 |
| A / AAAA 整类收敛 | 是 | 是 | 是 | 是 |
| 附加属性 | proxied / comment 参与写 | 无 | line / status 只读 | **line 被固定写回「默认」** |
| TTL 规则 | 60 / 300 / 600，其他静默变 300 | >0，默认 600 | ≥60，默认 600 | ≥60，默认 600 |
| 读取数量上限 | 按 `total_pages` 翻页，无硬上限 | 单次 list，无分页参数 | 100 × 500 = 50,000 条/zone | 10,000 条/zone |

结论：`created / updated / deleted` 四平台一致（同一个 `Reconcile`），summary 断言可以共用；`RecordsAfter`、TTL、附加属性必须按平台分开断言。

## 2. 口径变更后的结论

### 2.1 原 P0-1 关闭：AAAA 纳入托管

- 事实不变：`internal/dnspush/dnspush.go:246-275` 的过期清理固定遍历 A 与 AAAA，desired 缺失的一类会被整类删除；现有测试 `internal/cloudflarecore/dns_test.go:188-200` 已经用「第二次只推 IPv4 → AAAA 清空」钉住了该行为。
- 口径变更后这不再是越界，而是**类型收敛**：全量覆盖语义，A/AAAA 都归本次推送管理。
- 仍保留一条 UX 风险：切换 `record_type`（ALL→A 或 A→AAAA）会清掉另一族记录。这属于用户可感知的数据变化，需要在设置页与推送结果里明确提示，不能靠文档兜底。
- 若后续希望「保留另一族」，才需要引入 `managedTypes` 参数；当前口径不需要，也不建议顺手改，避免无谓的兼容面。

### 2.2 P2（原 P0，已降级）：类型约束只在 appcore 生效

- 证据：`desiredRecords`（`cloudflarecore/dns.go:404-412`）、`DesiredRecords`（`dnspush/dnspush.go:73-84`）只按 IP 池生成 A/AAAA，完全不读 `cfg.RecordType`；`FilterRowsForCloudflareRecordType` 只出现在 appcore（`dns_provider_service.go:190, 249`）。
- 可达性复核：四条写入入口（结果页、自动推送、定时任务、WebUI 命令）都先传 `results` 并完成行过滤；`pushDNSProviderTarget` 在 rows 为空时直接返回 `DNS_INPUT_EMPTY`，因此「只给 ipsRaw」的分支（`cloudflare_service.go:83-86`）不可达。结论：产品路径不会出现「配置 A-only 却写 AAAA」。
- 真实影响：只有直接调用 core 的代码（单测、脚本、未来新入口）不受 `record_type` 约束。属于内部契约缺口，不是用户可见缺陷。
- 处理建议：不改写语义，先把契约显式化——空 results 显式拒绝、core 注释写明「按输入写两族、类型筛选由调用方负责」、补三条 appcore 过滤测试。若未来出现第二个直接调用 core 的入口，再考虑下沉过滤。详见 `dns-record-type-fix-audit-2026-11-20.md`。
- 状态：上述契约显式化已落地，`go test` 六个相关包通过；用户路径无变化。

## 3. 降级项（P1）

### P1-1 `RecordsAfter` 语义不一致

- Cloudflare：`ListRecords` 只读目标 name 的 A/AAAA（`cloudflarecore/dns.go:196-206`）。
- DNSHE：读整个 subdomain（`dnshecore/dnshe.go:406-411`）。
- 阿里云 / DNSPod：读整个 zone（`alidnscore/alidns.go:297-302`、`dnspodcore/dnspod.go:320-325`）。
- 影响：拿 `records_after` 当「本次写入结果」的验证或 UI 展示，换平台就会把整域记录算进来；阿里/DNSPod 大 zone 还会让 bridge payload 膨胀。
- 处理建议：统一成「目标 name 的记录」——各 backend 回读后先按 name 过滤；或至少把差异写进接口文档与测试期望（当前流程按后者处理）。

### P1-2 DNSPod 更新会重置解析线路

- 证据：`dnspodcore/dnspod.go:255-275`，`ModifyRecord` 固定写 `RecordLine: DefaultLine`；`fromAPIRecord` 已带 `attrs["line"]`，但 `EquivalentRecord` 忽略 line，所以只有「因 content/ttl 变化触发更新」时才会无提示地把线路改回默认。
- 影响：分线路解析（电信/联通/境外）的记录被静默改写，功能回归且测试不易发现。
- 处理建议：更新时优先沿用 `record.Attrs["line"]`，缺省才用默认；`EquivalentRecord` 纳入 line 比较，避免多余改写。

### P1-3 非 CNAME 路径没有回滚

- 证据：`dnspush/dnspush.go:234-282` 的普通分支逐条 create/update/delete，任一失败直接返回；只有 `blockingRecords` 分支（`:181-244`）带 rollback 与「响应丢失后重读」。
- 影响：中途失败会留下半更新（例如三条记录只建成两条）。现有测试 `TestPushRecordsCreateFailureKeepsExistingRecords` 只证明「不删旧记录」，没证明「不残留新记录」。
- 处理建议：测试先按现状固化（流程 F1/F9），是否推广回滚另行决策。

## 4. 已知限制（P2）

1. **读取上限**：DNSPod 10,000 条/zone（`offset < 10000`），阿里云 50,000 条/zone（100 页 × 500），DNSHE 单次 list 无分页。超限时读取不完整，`Reconcile` 会把没读到的记录当不存在，可能重复创建同名记录，或漏判 CNAME 阻塞。建议至少加一次「结果被截断」警告。
2. **DNSHE 更新依赖 ID**：`id` 与 `provider_record_id` 都为空时直接报「缺少 DNSHE 记录 ID」；`mergeRecord` 会保留 ID，正常链路没问题，构造假记录时要带上。
3. **DNSHE 无分页参数**：返回条数由服务端决定，大子域下与 P2-1 同类。
4. **TTL 差异**：Cloudflare 白名单最严；阿里/DNSPod/DNSHE 只校验下限。跨平台不能共用同一组 TTL 期望值。
5. **阿里云不写线路**：`UpdateDomainRecord` 不传 line 参数，行为与 DNSPod 不同，不要把两者的「属性更新」期望写成一样。

## 5. 配置与旧数据兼容（均通过）

| 项 | 结论 | 依据 |
| --- | --- | --- |
| 旧 `cloudflare` 块（无 `dns_providers`） | 兼容，自动降级为单平台 | `dns_providers.go:47-66` |
| provider 别名 `alidns` / `dndps` / `tencentcloud` | 兼容，归一化为标准名 | `dns_providers.go:124-137` |
| 掩码凭据 | 兼容，判为未重新输入 | `IsMaskedSecret` + `dnsProviderConfigProblem` |
| 默认 TTL | 前后端一致（CF 300，其余 600） | `dns_providers.go:139-150`、`frontend/src/lib/bridge/config.ts:37-44` |
| snake_case / camelCase 双写 | 兼容 | `dns_providers.go:86-101`、`config.ts` 的 `normalizeDNSProviders` |
| `dns.push` / `cloudflare.list` 命令与返回码 | 不需要改，测试流程不改接口 | `invoke.go:50-52` |
| 旧配置缺 `zone_id` 的 CF 平台 | 不兼容（本来就会拒绝推送） | `dnsProviderConfigProblem` |

## 6. 测试流程的对应调整（已落到流程文档）

1. R9 从「冲突证据」改为「类型收敛的定义行为」：AAAA 存在、只推 A → `created=1, deleted=1`；新增 R10（反向）、R13/R14（AAAA 上传与属性更新）、R15（双族换血）、R16（ALL→A 收敛）。
2. 主流程 S2/S4/S5/S6 改为 A + AAAA 双族，summary 期望值随之改成 3/2/3 这类双族计数。
3. `RecordsAfter` 断言按平台分开：CF 断言「只有目标 A/AAAA」，DNSHE/阿里/DNSPod 断言「含整域记录」。
4. 「全 zone 指纹」在 DNSHE 上只能覆盖 subdomain 视野，报告要写明检查范围。
5. core 层用例要单独覆盖「不传 `record_type`、输入含 IPv6」的场景，用来证明 2.2 的现状；appcore 层用例覆盖 `record_type=A/AAAA/ALL` 的行过滤。

## 7. 建议处理顺序

1. 契约显式化：空 results 拒绝、core 注释写明「类型筛选由调用方负责」、补 appcore 过滤测试（2.2，零风险）。
2. 统一 `RecordsAfter`，或在接口文档里固化四平台差异（P1-1）。
3. 修 DNSPod 线路保留（P1-2）。
4. 回滚与读取上限按需排期（P1-3、P2）。
5. 类型收敛的 UI 提示随设置页文案一起补（2.1 的 UX 项）。

前 4 项都不改变用户可见语义；只有在未来出现「绕过 appcore 直接调 core」的新入口时，才需要把类型过滤真正下沉。
