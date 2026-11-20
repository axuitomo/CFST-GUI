# DNS 平台功能测试流程：读取 / 上传更新 / 记录更替

> 基准平台：Cloudflare（`internal/cloudflarecore` + `internal/dnspush`）。本文只覆盖功能正确性与影响范围，不含测速、延迟、公网传播、ECH。
> 托管类型：**A、AAAA，以及作为阻塞记录的 CNAME**（CNAME 只会被删除替换，不会被创建）。

## 0. 测什么、不测什么

**测**

1. **读取**：按记录名过滤、A/AAAA 类型过滤、分页、空集合、字段完整性。
2. **上传**：首次写入多条 A 与 AAAA（POST 逐条创建）。
3. **更新**：同内容记录的 TTL / proxied / comment 变化走原地 PATCH，记录 id 不变。
4. **记录更替**：IP 集合换血（先建后删）、重复记录清理、同名 CNAME 被替换、A 与 AAAA 的整类收敛。

**不测**：接口延迟与限速表现、公网解析是否生效、多云平台差异、ECH、DoH。

**红线（每条用例都必须断言）**

- 写操作只允许落在：`name == 目标记录名` 且 `type ∈ {A, AAAA}`；CNAME 只允许作为阻塞记录被删除。
- 目标名下的 TXT / MX / NS 等非托管类型不得删除或改写；其 `id + content` 必须逐字节不变。
- 同 zone 其它记录名的任何记录不得变化（用全 zone 指纹比对，见 §6）。
- 输入可同时含 IPv4 与 IPv6；`record_type=ALL/A/AAAA` 决定参与推送的地址族。过滤发生在 appcore 层，所有可达入口都会经过；core 层不读该字段，因此 core 用例不能用 `record_type` 当期望依据，要按输入 IP 池算。

## 1. 基准语义（所有期望值都由它推出）

| 语义 | 依据 | 对测试的影响 |
| --- | --- | --- |
| 同一 name 可以有多条 A / AAAA | Cloudflare 多值记录 | 首次上传断言「创建条数 == 有效 IP 条数」 |
| 同一 name 不能并存 CNAME 与 A/AAAA | DNS 规范 + `blockingRecords` | 存在 CNAME 时先删 CNAME 再建地址记录，且必须走回滚路径 |
| A 与 AAAA 同属托管集合 | `Reconcile` 的 A+AAAA 遍历 | desired 中缺失的一类会被**整类清理**，不是「保持不变」 |
| 记录身份是 `id`；属性更新是 `PATCH /dns_records/{id}` | `UpdateRecord` | 属性更新的用例要断言 id 不变 |
| 集合匹配按 `Content`，不是按 id | `Reconcile` 的 `byContent` 索引 | **换 IP 等于「新建 + 删除」，不是更新**；最容易写错期望值的地方 |
| 等价判定包含 comment / proxied / ttl | `sameManagedRecord` | 只改这三项任意一项都会触发 PATCH |
| TTL 只允许 60 / 300 / 600 | `IsAllowedTTL` | 非法值静默改成 300 并给警告，需要单独断言 |
| 读取按 `name` + `type` 过滤，`per_page=100` 翻页 | `listRecords` | 超过 100 条要断言翻页取全 |
| `RecordsAfter` 只回读 A / AAAA | `PushRecords` 末尾 `ListRecords` | 推送后返回的记录里没有 CNAME，这是预期，不是缺陷 |
| 列表 `type` 为空时返回该 name 全类型 | `normalizeListRecordType` 返回空串 | 想读 CNAME 必须用「只按 name 过滤」，不能用 `ListRecords` |
| `record_type` 只在 appcore 过滤输入行 | `FilterRowsForCloudflareRecordType` | core 层直接调用时，IPv6 会照写 AAAA |

## 2. 环境与夹具

1. 专用测试 zone，专用主机名 `cfst-selftest.example.com`；不复用生产记录名。应用内自检强制要求测试记录名以 `cfst-selftest` 开头，并必须是当前推送记录名的子域名。
2. Token 权限：Zone.DNS 编辑 + Zone 读取；单独签发，不进生产配置。
3. 预置哨兵：
   - 同 zone 其它 name：1 条 A、1 条 AAAA、1 条 TXT、1 条 CNAME；
   - 目标 name：1 条 TXT、1 条 MX（用于证明非托管类型不被清理）。
4. 托管夹具按用例准备：A、AAAA、CNAME 由用例自己布置，测试前恢复到该用例起始状态。
5. 测试前保存 zone 全量快照：`{id, name, type, content, ttl, proxied, comment}`，排序后存 JSON。
6. 每个用例结束后按 id 恢复目标 name 的 A / AAAA / CNAME，不做整 zone 回滚。

## 3. 主流程（串行 8 步）

| 步 | 动作 | 关键断言 |
| --- | --- | --- |
| S0 | 读全 zone 快照，记录哨兵指纹 | 快照成功，目标 name 起始状态符合夹具 |
| S1 | 读取功能：按 name、按 A、按 AAAA、空 type 四种读法各读一次 | 只返回目标 name；类型过滤准确；空 type 能读到 CNAME/TXT；字段齐全 |
| S2 | 目标 name 无地址记录、无 CNAME，推送 `{1.1.1.1, 1.2.3.4, 2001:db8::1}` | `created=3 updated=0 deleted=0`；A/AAAA 的 name/ttl/proxied/comment 与配置一致 |
| S3 | 同集合再推一次 | `0/0/0`；两个地址族的记录条数与 id 集合完全不变（幂等） |
| S4 | 推送 `{2.2.2.2, 2001:db8::2}` | `created=2 updated=0 deleted=3`；终态两族各一条；旧 id 全部消失 |
| S5 | 对两条记录同时改 TTL 300→600、proxied 取反 | `created=0 updated=2 deleted=0`；**两条 id 各自不变** |
| S6 | 人为在同名补一条 CNAME（先删地址记录），再推送 `{3.3.3.3, 2001:db8::3}` | `deleted=1（CNAME）created=2`；终态只有 A/AAAA；TXT/MX 仍在 |
| S7 | 清理：删除本流程创建的地址记录，按 S0 快照恢复 CNAME | 目标 name 回到 S0；哨兵指纹逐条相等 |
| S8 | 复核全 zone 指纹 | 与 S0 完全一致（除本流程明确改动的目标 A/AAAA/CNAME） |

S2 与 S3 之间不要等 TTL，功能测试不依赖传播。

## 4. 用例矩阵（可直接落成表驱动测试）

期望值列 `C/U/D` = `Summary.Created/Updated/Deleted`。

| 编号 | 起始状态（目标 name） | 输入 | 期望请求序列 | C/U/D | 终态断言 |
| --- | --- | --- | --- | --- | --- |
| R1 | 空 | A 2 条 + AAAA 1 条（TTL 300） | POST ×3 | 3/0/0 | 3 条新记录，地址族正确 |
| R2 | R1 终态 | 同一集合 | 无写请求 | 0/0/0 | id 集合不变 |
| R3 | R1 终态 | A 1 条 + AAAA 1 条（全部换新） | POST ×2 → DELETE ×3 | 2/0/3 | 仅新 2 条；旧 id 全消失 |
| R4 | A:1.1.1.1 TTL300 proxied=false | 同 content，TTL600 proxied=true | PATCH ×1 | 0/1/0 | id 不变，ttl=600，proxied=true |
| R5 | A:1.1.1.1 重复两条 | A:1.1.1.1 | DELETE ×1 | 0/0/1 | 保留第一条（按列表顺序），条数 1 |
| R6 | CNAME → other.example.com | A 1 条 + AAAA 1 条 | DELETE ×1（CNAME）→ POST ×2 | 2/0/1 | 仅 A/AAAA；CNAME 不存在 |
| R7 | CNAME + TXT + MX | A 1 条 | DELETE CNAME → POST A | 1/0/1 | TXT/MX 的 id 与内容不变 |
| R8 | CNAME 存在 | 空 IP 输入 | 无任何请求 | 0/0/0 | CNAME 保持原样（`HasInputIPs=false` 提前返回） |
| R9 | AAAA:2001:db8::1（A 空） | 仅 A:1.1.1.1 | POST ×1 → DELETE ×1（AAAA） | 1/0/1 | A 写入、AAAA 被整类清理；类型收敛的定义行为 |
| R10 | A:1.1.1.1（AAAA 空） | 仅 AAAA:2001:db8::2 | POST ×1 → DELETE ×1（A） | 1/0/1 | AAAA 写入、A 被整类清理 |
| R11 | A:1.1.1.1、A:1.1.1.2（TTL 300） | 同内容两条，TTL 600 | PATCH ×2 | 0/2/0 | 两条 id 各自不变 |
| R12 | 同名 120 条 A（跨两页） | 其中 20 条 | 分页读全 → DELETE ×100 | 0/0/100 | 终态 20 条，无跨页漏删 |
| R13 | 空 | 仅 AAAA:2001:db8::1 | POST ×1 | 1/0/0 | 只有 AAAA，无 A |
| R14 | AAAA:2001:db8::1 TTL300 | 同 content，TTL600 | PATCH ×1 | 0/1/0 | id 不变 |
| R15 | A:1.1.1.1、AAAA:2001:db8::1 | A:2.2.2.2、AAAA:2001:db8::2 | POST ×2 → DELETE ×2 | 2/0/2 | 两族各一条，旧 id 消失 |
| R16 | A:1.1.1.1、AAAA:2001:db8::1 | 仅 A:1.1.1.1（ALL→A 收敛） | 无 POST → DELETE ×1（AAAA） | 0/0/1 | AAAA 被清理，A 保持；需要 UI 提示这是预期 |

补充读取用例：

| 编号 | 场景 | 期望 |
| --- | --- | --- |
| Q1 | 按 name 读、不传 type | 返回该 name 的 A/AAAA/CNAME/TXT/MX；用于覆盖 CNAME |
| Q2 | 按 name + type=A | 只返回 A |
| Q3 | 按 name + type=AAAA | 只返回 AAAA |
| Q4 | 空集合 | `result=[]`、`success=true`，不报错 |
| Q5 | 字段完整性 | 每条记录 `id/name/type/content/ttl/proxied/comment` 均可读取 |
| Q6 | 分页边界 | `total_pages=1` 只请求 1 次；`total_pages=0` 视为结束，不无限翻页 |
| Q7 | 混合集合 | A 与 AAAA 各 1 条时，type=A/AAAA 的过滤结果互不混入 |

## 5. 失败注入（回滚与半更新）

| 编号 | 注入点 | 期望结果 | 要断言的现状 |
| --- | --- | --- | --- |
| F1 | 第 2 条 POST 返回 500 | `OperationError{create}`，**已建的第 1 条不回滚** | 非阻塞路径无回滚，记录残留；锁住现状，避免以后悄悄变 |
| F2 | DELETE CNAME 返回 500 | `OperationError{delete}`；无新增地址记录 | 阻塞路径在删除失败时立即回滚 |
| F3 | 删完 CNAME、POST 第 2 条失败 | 回滚：删除已建记录，重建 CNAME，summary 归零 | 验证 `rollback` 的重读逻辑 |
| F4 | PATCH 返回 500 | `OperationError{update}`；记录内容与属性不变 | 不能出现「返回失败但值已改」 |
| F5 | 列表返回 401/403 | `OperationError{list}`，不进入任何写操作 | 鉴权失败不得产生半写 |
| F6 | 429 | 原样返回错误码，不重试、不静默成功 | 与推送层「失败只记录不回滚」一致 |
| F7 | 服务端已写入但响应丢失 | 回滚前重新 List 一次，按实际状态算 created/deleted | 覆盖 CHANGELOG 里修过的「响应丢失后重读状态」路径 |
| F8 | 推送中途取消 ctx | 停止后续写；已完成的写如实计入 summary | 不能返回「全部成功」 |
| F9 | AAAA 的 POST 失败 | 与 F1 相同：不回滚已完成的 A | 两个地址族共用同一分支，不额外回滚 |

## 6. 影响面校验（每条用例后必跑）

1. **全 zone 指纹**：除目标 name 外所有记录，按 `id|name|type|content|ttl|proxied|comment` 排序后求 SHA-256；用例前后必须相等。
2. **目标 name 的非托管类型**：TXT / MX / NS 逐条 `id + content` 比对，任何变化直接判失败。
3. **托管集合**：对 A、AAAA、CNAME 求集合差；出现预期外的 A/AAAA 或多出来的 CNAME，判失败。
4. 指纹变化时立刻中止后续用例，先恢复现场再查原因。

## 7. 清场与报告

- 每条用例结束打印：起始快照、请求序列（方法 + 路径 + 记录 id）、summary、终态集合、指纹结论。
- 清理顺序：先删本流程新建的 A/AAAA，再按 id 恢复原 CNAME；只动目标 name。
- 报告里必须写清「本次测试改动了哪些 id」，便于事后审计。

## 8. 自动化落点

| 层级 | 位置 | 覆盖 |
| --- | --- | --- |
| 单元 | `internal/dnspush/*_test.go`（Reconcile 表驱动） | R1~R16、F1~F9；用假 Backend 记录调用序列 |
| 单元 | `internal/cloudflarecore/dns_test.go` | 读取过滤、分页、TTL 白名单、PATCH 方法、`RecordsAfter` 只含 A/AAAA |
| 契约 | `internal/contracttest` | 请求方法/路径/查询参数：`GET /dns_records?name=&type=&per_page=100&page=N`、`POST`、`PATCH /{id}`、`DELETE /{id}` |
| 真实环境 | 环境变量门控（如 `CFST_CF_TEST_ZONE_ID` / `_TOKEN` / `_RECORD`） | S0~S8；变量缺失自动 skip，禁止默认打真实 zone |

真实环境用例只允许在 `cfst-selftest` 记录名上跑；脚本启动先读一次快照，发现目标名不是预期值直接退出。

### 8.1 应用内自检（已落地）

DNS 页面新增「DNS 平台自检」面板，命令为 `dns.selftest`，四个平台共用同一套阶段断言。它不跑公网解析与测速，只验证读取、上传、更新和记录更替。

**作用域**

- 测试记录名默认 `cfst-selftest.<推送记录名>`，可改，但必须：不等于推送记录名、是推送记录名的子域名、并以 `cfst-selftest` 开头。因此不会误碰 `www` 等生产子域名。
- 只删除/写入测试记录名下的 A、AAAA、CNAME；同名的 TXT / MX / NS 等记录在基线时记指纹，终态逐条比对，必须不变。
- 不读也不改同 zone 其它记录名。

**阶段**（浏览器/桌面均可见每一步状态与耗时）

| 阶段 | 动作 | 断言 |
| --- | --- | --- |
| preflight | 校验平台配置与测试记录名 | 配置完整、命名合规 |
| baseline | 读取测试记录名全部类型 | 记录可控，保存非托管记录指纹 |
| reset | 删除测试记录名下的残留 A/AAAA/CNAME | 起始托管记录为 0 |
| upload | 推送 2 条 A + 1 条 AAAA（文档地址段） | `created=3 updated=0 deleted=0`；回读内容与 TTL 正确 |
| read | 全类型、A、AAAA 三种读法 | 过滤准确、字段齐全 |
| idempotent | 同集合再推一次 | `0/0/0`，记录 id 集合不变 |
| replace | 换成 1 条 A + 1 条 AAAA | `created=2 deleted=3`，旧 id 全部消失 |
| update | 同内容改 TTL | `updated=2`，id 不变，回读新 TTL |
| cname | 先建同名 CNAME，再推送地址记录 | `deleted=1 created=2`，终态无 CNAME；平台拒绝创建 CNAME 时标记「跳过」而不是失败 |
| cleanup | 删除测试记录名下的 A/AAAA/CNAME | 托管记录归零 |
| verify | 终态复核 | 托管记录 0，非托管记录指纹与基线一致 |

**TTL 选择**

- Cloudflare：配置值不是 600 时用「配置值 → 600」；配置值就是 600 时用「600 → 300」。
- 阿里云 / DNSPod / DNSHE：配置值低于 600 时先抬到 600，再按「≤600 → 1200，否则 600」选择；避免用低于平台最小 TTL 的值去触发属性更新。

**结果**

- 全部通过：`DNS_SELFTEST_COMPLETED`。
- CNAME 阶段被平台拒绝而跳过：`DNS_SELFTEST_PARTIAL`，其余阶段通过。
- 任一断言失败：`DNS_SELFTEST_FAILED`；清理阶段与终态复核仍会执行，失败详情会列出残留记录的 id，便于手动删除。
- 结果里的 `wrote_records` 区分「根本没进入写入阶段」与「写入后已清理」；前者不会给出“已清理”的假象。

## 9. 已知口径差异与待确认

1. **类型收敛是定义行为，但需要提示**：只推 A 会删除同名 AAAA，只推 AAAA 会删除同名 A（R9/R10/R16）。这是 `Reconcile` 的既定语义，允许动 AAAA 后不再算越界；但切换 `record_type` 会清掉另一族记录，UI 必须提前明说，否则用户会当成数据丢失。
2. **`record_type` 约束只在 appcore**：core 层不读该字段，但所有可达的写入入口都先按行过滤，因此产品路径不会误写另一族；core 的缺口只影响直接调用它的代码。无需为此改语义，见审计说明 `dns-record-type-fix-audit-2026-11-20.md`。
3. **非托管路径无回滚**：F1/F9 的残留是现状。若要求「要么全成要么全不成」，需要把回滚从阻塞路径推广到普通创建/更新/删除路径。
4. **`RecordsAfter` 不含 CNAME，且跨平台范围不同**：Cloudflare 只返回目标名的 A/AAAA；DNSHE 返回整个子域；阿里/DNSPod 返回整个 zone。推送后验证必须自己按 name 读，不能直接拿 `RecordsAfter` 当目标记录集。
5. **更新即 PATCH，换 IP 即新建+删除**：验收口径要按这个写，否则会出现「期望 updated=1，实际 created=1 deleted=1」的误判。
6. **TTL 非法值被静默改写**：60/300/600 之外的值会变 300 并给 warning，用例要断言 warning 存在，而不是断言写入原值。
