# 审计说明：`record_type` 约束要不要下沉到 core，会不会变语义

审计对象：兼容性报告里的 P0「类型约束只在 appcore 生效」。
审计方式：从 UI / 自动推送 / 定时任务 / WebUI 四条入口逐层追到 core，核对每一层是否已有类型过滤，并检查 core 测试对 `record_type` 的实际断言。

## 0. 结论

1. **产品里不可复现**：所有能触发真正写入的入口都先按 `record_type` 过滤了行，再交给 core。因此「配置 A-only、实际写了 AAAA」在当前 UI、自动推送、定时任务、WebUI 命令下都不会发生。
2. **原 P0 降级为 P2**：问题只剩内部契约不清——core 的 `Config.RecordType` 从来没参与写逻辑，直接调用 core 的代码不受配置约束。这是给未来调用者埋的坑，不是用户可见缺陷。
3. **「无 results 只给 ipsRaw」那条分支不可达**：`pushCloudflareDNSRecordsContext` 只有一个调用方 `pushDNSProviderTarget`，而后者在 `len(rows)==0` 时直接返回 `DNS_INPUT_EMPTY`，根本走不到 core。
4. **推荐不改写语义**：只做「死分支显式化 + 契约注释 + 测试锁定」，半天内完成、零行为变化。真要单点强制再加 core 过滤，但那是内部 API 语义变更，收益只是防未来误用。

## 1. 调用链审计

| 入口 | 传参 | 过滤点 | 结论 |
| --- | --- | --- | --- |
| 结果页手动推送 | `results` | `dns_provider_service.go:190,249`、`cloudflare_service.go:75` | 已过滤 |
| 测速后自动推送 | `results`（`post_probe_push.go:96`） | 同上 | 已过滤 |
| 定时任务 | `results`（`scheduler_service.go:234`） | 同上 | 已过滤 |
| WebUI `/api/command/dns.push` | 任意 JSON | 同 `dns.push` 链路 | 已过滤或直接拒绝 |

关键拦截点：

- `dns_provider_service.go:145-146`：`len(rows) == 0` 时直接 `DNS_INPUT_EMPTY`，**不进入任何平台推送**。
- `cloudflare_service.go:156-160`：`pushCloudflareDNSRecordsContext` 的唯一调用点，此时 rows 必然非空。
- `cloudflare_service.go:83-86`：只给 `ipsRaw`、不给 `results` 的分支，因此不可达。

core 侧证据：

- `cloudflarecore/dns.go:404-412` 的 `desiredRecords` 只按 IP 池分组，不读 `cfg.RecordType`。
- `alidnscore`、`dnspodcore`、`dnshecore` 的 `Push` 同样只按 IP 池生成 desired；三者的 `Config.RecordType` 字段只在解析/配置层存在，写逻辑从未读取。
- `cloudflarecore/dns_test.go:405-418` 的测试 helper 显式写 `"record_type": "A"`，却传入 IPv6 并断言创建了 AAAA。这不是疏忽，而是把「core 不读 record_type」固化成了既有契约。

## 2. 改法一（推荐）：不改写语义，只固化契约

### 改动点

1. `internal/appcore/cloudflare_service.go`
   在 `pushCloudflareDNSRecordsContext` 开头显式要求结果集：`firstNonNil(payload["results"], payload["rows"]) == nil` 时直接返回 `DNS_INPUT_EMPTY`。现在这一步由上层隐式保证，改成显式后，函数自身不再有「只给 ipsRaw 也能写」的歧义。
   同时删掉 `:83` 那行从 `payload["ipsRaw"]` 取值的兜底（或保留但注明「仅由 results 分支写入」）。
2. 四个 core 的 `Push` / `PushRecords` 注释补一句契约：
   > 本函数按输入 IP 池写 A/AAAA，不读取 `Config.RecordType`；类型筛选由调用方（appcore）负责。
3. 测试
   - 新增：`dns.push` 不传 `results` → `DNS_INPUT_EMPTY`，且 Cloudflare 假服务器**收到 0 次写请求**。
   - 新增：`record_type=A` + 混合行 → 只写 A、同名 AAAA 被清理（即 R9）；`record_type=ALL` → 两族都写；`record_type=AAAA` → 只写 AAAA。
   - 调整：`TestPushRecordsReconcilesAAndAAAARecordsWithoutDeletingFirst` 改名或加注释，说明它验证的是「core 按输入写两族」，与 `record_type` 无关；否则后来的人会误以为这是 A-only 用例。

### 语义变化

- 用户路径：**无**。
- core 行为：**无**（仍按输入写两族）。
- 唯一变化：一条当前不可达的内部分支从「会写」变成「报空输入」。没有调用方，影响为 0。

## 3. 改法二（可选）：在 core 单点强制

### 改动点

1. `internal/dnspush` 新增 `FilterIPGroups(groups IPGroups, recordType string) IPGroups`（`ALL` 保留两族，`A` 只留 v4，`AAAA` 只留 v6，未知值按 `A`）。
2. 四个 core 的 `Push` 在生成 desired 前调用该函数。
3. 同步两处配套：
   - `cloudflare_service.go:104` 的 `upload_count` 目前按原始 `ipsRaw` 计数，过滤后要改成按实际参与推送的 IP 计数，否则计数与创建条数不一致。
   - 被过滤掉的地址要进 `PushResult.Warnings`，避免调用方以为「给了就会写」。

### 语义变化

- 用户路径：**无**（rows 已过滤，双过滤幂等）。
- core API：**有**。`Push` 的托管集合从「输入里的全部 IP」变成「`record_type` 指定的一族」；配合类型收敛语义，未被选中的一族会被清理，而不是保留。
- 现有测试必须改：`TestPushRecordsReconcilesAAndAAAARecordsWithoutDeletingFirst`
  - 第一次推送：`Created` 3 → 2（AAAA 不再创建），`Deleted` 3 不变，`Ignored` 2 不变；`createdCount` 3 → 2。
  - 第二次推送：`Deleted` 3 → 2；`deletedCount` 3 → 2。
  - 断言里的 AAAA 内容从「换新」变成「被清空」，与 R9 一致。

## 4. 语义变化对照

| 场景 | 现状 | 改法一 | 改法二 |
| --- | --- | --- | --- |
| 结果页手动推送 | 按 record_type 过滤后写 | 不变 | 不变 |
| 自动推送 / 定时任务 | 同上 | 不变 | 不变 |
| WebUI 调 `dns.push` 且不传 results | 不写（`DNS_INPUT_EMPTY`） | 不写（显式） | 不写 |
| 直接调 core `Push`（非 apcore 路径） | 写两族 | 写两族 | 只写 record_type 一族，另一族被清理 |
| core 单测 | 通过 | 通过（改名/注释） | 2 处断言需改 |
| 配置语义一致性 | 仅 appcore 成立 | 同左（文档写明） | 全链路成立 |

## 5. 为什么不建议直接选改法二

1. 收益只是「防未来误用」：今天没有任何可达路径会误写，改动换来的用户价值为零。
2. 代价是内部 API 语义变更：core 的契约从「输入即托管集合」变成「配置即托管集合」，会让四个平台的 core 测试与 future 调用方都要重新对齐。
3. 两层都过滤后，规则一旦漂移（例如 appcore 以后新增 IP 合法性校验、或支持新记录类型），core 与 appcore 的结论可能不一致，排查成本比现在更高。
4. 与项目「最小改动、保持向后兼容」的约定不符。

## 6. 建议落地顺序

1. 改法一的三项（显式空输入 + 契约注释 + 三条测试），零风险，可立即做。
2. 若之后出现第二个直接调用 core 的入口，再把过滤下沉并同步 warning 与计数（改法二）。
3. 不要动 `Config.RecordType` 字段本身：它在前端表单、配置归一化、读取过滤（`cloudflare.list`）里都有用途，删字段属于无收益的结构改动。

## 7. 验证

```
go test ./internal/appcore/... ./internal/cloudflarecore/... ./internal/dnspush/...
```

真实 zone 手测（可选）：`record_type=ALL` 推混合 IP → 两族都在；切 `A` 再推 → A 更新、AAAA 清空（R9 的定义行为）。两者都符合「允许动 AAAA」的口径，不需要额外决策。

## 8. 落地记录（改法一已实施）

| 项 | 位置 | 内容 |
| --- | --- | --- |
| 显式拒绝裸 IP | `internal/appcore/cloudflare_service.go` | `pushCloudflareDNSRecordsContext` 在 `results` / `rows` 缺失或为空时直接返回 `DNS_INPUT_EMPTY`，并在注释说明「rows 承载类型过滤与上传筛选」 |
| 契约注释 | `cloudflarecore/dns.go`、`dnspush/dnspush.go`、`alidnscore/alidns.go`、`dnspodcore/dnspod.go`、`dnshecore/dnshe.go` | 写明按输入 IP 池管理 A/AAAA，不读 `Config.RecordType`，类型筛选由调用方负责 |
| 新增测试 | `internal/appcore/cloudflare_record_type_test.go` | ① 裸 IP 直调与经 `dns.push` 分发都被拒绝，Cloudflare 假服务器 0 次请求；② `record_type=A/AAAA/ALL` 三种过滤，断言创建类型序列与 `upload_count` |
| 测试改名 | `internal/cloudflarecore/dns_test.go` | `TestPushRecordsReconcilesAAndAAAARecordsWithoutDeletingFirst` → `TestPushRecordsManagesBothAddressFamiliesFromInputPool`，并注明「payload 设 A 但输入含 IPv6、仍写 AAAA」是契约而非疏忽 |
| 变更记录 | `CHANGELOG.md` | 在 `[Unreleased] → 修复` 增加一条，说明裸 IP 入口被拒绝且 core 契约不变 |

验证结果：`gofmt` 后 `go test` 六个相关包全部通过（appcore 3.6s，其余 <1s）。

语义核对：用户可达路径（结果页 / 自动推送 / 定时任务 / WebUI）行为完全不变；唯一变化是原本不可达的内部裸 IP 分支从「会写」变成「显式报空输入」。core 的写语义没有改动，改法二（core 强制过滤）仍然未做，留待出现第二个直接调用 core 的入口时再评估。

