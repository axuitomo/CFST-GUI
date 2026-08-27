# 评审报告：MICS 抽样引擎升级 PRD（v0.5）——双向钢人验证

- 评审对象：`docs/prd-mics-engine-upgrade.md`（v0.5，2026-08-27，状态"已定稿，可开工"）
- 评审方法：**双向钢人验证**——反向（代码事实 + 本机实测 → PRD 声明）逐条核对 PRD 有没有说谎；正向（需求 → 代码）站在反对面（钢人/抬杠）逐项挑战 PRD 的结论与验收标准，看"可开工"是否站得住。
- 评审人：MainAgent（独立复验，不采信 PRD 既有结论）
- 评审日期：2026-08-27
- 基线：`go build ./internal/mcis/... ./internal/appcore/... ./internal/probecore/... ./internal/httpcfg/... ./internal/httpclient/...` 通过；`go test ./internal/probecore/... ./internal/httpcfg/... ./internal/httpclient/...` 通过。

---

## 0. 评审结论（先给结论）

| 维度 | 结论 |
|---|---|
| 代码事实核验 | ✅ 问题清单（P0-0…P1-6）与附录 B 的代码事实**全部属实**，行号、字段、行为一一对应 |
| 实测证据核验 | ✅ 附录 B 的关键实测（HEAD 404 / GET 200、官方端点 403、qzz.io HEAD 404）**全部复现**；默认 URL 证书在本机环境表现为 HTTP 301/HTTPS 超时，**经用户确认实际可用，F1 撤销**（见 §2.2） |
| 技术方案可行性 | ✅ 各 P 项改动方向成立，改动面/文件清单与代码结构吻合 |
| 决策合理性 | ✅ 合理；**P1-4 延后、M1+M2 合入节奏**正确；§8 决策 #2"保持默认 URL"经用户确认无隐患（F1 撤销） |
| **"可开工"判定** | **通过（附 F2/F3 修订）**：事实与方案没问题，F1 按用户确认撤销；F2（colo 归一化）、F3（可复现性范围）已并入 PRD v0.6，其余为低severity 修正 |

**一句话**：PRD 没撒谎、方案能落地。评审发现的三项实质问题中，F1 经用户确认（默认 URL 证书实际可用）撤销；F2、F3 已修订进 PRD v0.6；其余 F4–F8 为低severity 描述/清理项，不阻塞合入。

---

## 1. 反向验证：代码事实 & 实测证据 → PRD 声明

### 1.1 问题清单代码事实逐项核验

| PRD 项 | 声称（代码事实） | 复核结果 | 判定 |
|---|---|---|---|
| P0-0 | `probe/trace.go:136` 用 HEAD；`parseTrace()`（:272）从未被调用；body 被丢弃；`Result.Trace` 恒空 | 三处全部属实：`http.MethodHead` 在 :136；body `io.Copy(io.Discard, ...)`（:166）；`parseTrace` 全仓仅 trace.go 一处定义、零调用；`probeOnce` 从不写 `res.Trace` | ✅ |
| P0-1 | `source_mcis.go:86` 硬编码 `InsecureSkipVerify: true` | 属实（:86） | ✅ |
| P0-2 | `BuildMCISEngineConfig` 硬编码 `ColoAllow=nil`（:74）；colo 过滤链路存在但缺 body | `ColoAllow=nil` 属实；`processOneResult` 读 `Trace["colo"]`（engine.go:277-278）+ `passColoFilter`（engine.go:243）链路真实存在 | ✅ |
| P0-3 | `getExploitationPrefixes()` doc 声称"按最佳排序"，实现为 map 迭代 + 3x/1x 加权；加权语义正确 | doc 注释（:359）与实现（:384-392 map 遍历）确实不符；`idx := int(r/exploitRate*len(...))` 按索引均匀取、tier1 重复 3 次，加权语义与顺序无关、功能正确 | ✅ |
| P1-5 | `MinSamplesSplit=5`（`DefaultTreeConfig`）、无树大小上限；`trySplit` 每 20 样本拆 `Heads*2` 个 | `DefaultTreeConfig.MinSamples=5`（tree.go:42）、`SplitNode` 无节点数上限、`trySplit` 内 `maxSplits=Heads*2`（engine.go:342）、调度每 `SplitInterval=20` 触发一次（engine.go:150, config.go:91） | ✅ |
| P1-6 | `Run()` 不重置 `seenIPs/submitted/completed` | 属实（engine.go:60 无任何重置） | ✅ |
| B.4 缺口1 | 主流程 trace 用 `cfg.TraceURL`（空则 `DeriveTraceURL`），MICS 直接用 `cfg.URL` host | `request_profile.go:13` 用 `TraceURL` 建 profile；`probe_config.go:316-328` 有派生逻辑；`source_mcis.go:98-104` 用 `cfg.URL` 解析 host | ✅ |
| B.4 缺口2 | COLO 过滤未接线（`ColoAllow=nil`） | ✅ 同 P0-2 | ✅ |
| B.4 缺口3 | 证书校验硬编码 | ✅ 同 P0-1 | ✅ |
| 1.1 | `internal/mcis` 包当前无任何单元测试 | 复核：`internal/mcis` 递归无任何 `_test.go` | ✅ |
| P0-1 前置 | `cfg.VerifyTLSCertificate` 字段已存在 | `probe_config.go:60` 存在，**默认值 `true`**（:131） | ✅ |
| 证书接线 | `InsecureSkipVerify` 经 `httpcfg.Resolve` 流入 TLS 配置 | `trace.go:76` → `httpcfg.Profile.InsecureSkipVerify` → `httpclient.tlsConfigForProfile`（client.go:325） | ✅ |
| 主流程口径 | 主流程已按 `!cfg.VerifyTLSCertificate` 派生 | `appcore/probe_engine.go:54` 属实 | ✅ |

### 1.2 附录 B 实测证据独立复验（本机 curl）

| PRD 实测项 | PRD 结论 | 本机复验 | 判定 |
|---|---|---|---|
| `HEAD https://speed.cloudflare.com/cdn-cgi/trace` | 404 | **404** | ✅ |
| `GET https://speed.cloudflare.com/cdn-cgi/trace` | 200，body 含 colo | **200**，body `colo=SJC`（**大写**，见 F2） | ✅ |
| `HEAD https://test.xyz9923.qzz.io/cdn-cgi/trace` | 404 | **404** | ✅ |
| `GET https://speed.cloudflare.com/__down?bytes=100000000`（无 Range） | 403 | **403** | ✅ |
| `Range GET https://speed.cloudflare.com/__down?bytes=500000000` | 403 | **403** | ✅ |
| `https://speedtest.xyz9923.dpdns.org/500m` TLS 不可用 | TLS 握手失败 | HTTP 明文 → **301** 跳 https；HTTPS → **超时无响应**（本网段表现为超时而非 fatal alert，结论一致：证书/HTTPS 不可用） | ✅ |

> 补充证据：`.tmp-probe/main.go`（作者遗留实测脚本，用 Range 打 500m 端点）印证 P1-4 排查痕迹；该文件为未跟踪残留，建议清理（见 F6）。

---

## 2. 正向验证（钢人视角）：需求 → 代码，逐项挑战

对每个 PRD 决策/验收标准，站在反对面找它站不住的地方。**核验通过的**（钢人未驳倒）直接带过，**站不住/有缺口**的列为 F1–F8。

### 2.1 已站住的决策

- **P0-0 HEAD→GET**：方案"1 处方法改动 + OK 判定不变"成立；GET 返回 200 且 body 即 colo 数据源，实测支撑。可行。
- **P0-1 接线方向**：`InsecureSkipVerify = !cfg.VerifyTLSCertificate` 与主流程口径（probe_engine.go:54）一致；SNI/Host 解析顺序与主流程一致。接线本身可行。
- **P0-2 链路**：GET 改造后 `parseTrace` 解析 `colo` → `Result.Trace` → `processOneResult` → `passColoFilter` 链路全部现成，只缺 body，判断成立（但见 F2）。
- **P0-3 降级定性**：由"exploit 加权失效"修正为"注释与实现不符、功能正确"，严重度中→低，判断准确。
- **P1-5**：`MinSamplesSplit` 5→8、节点上限 4096、信息量门槛——树结构与 `InformationGain()` 均已存在，可落地。
- **P1-6**：重置 `seenIPs/submitted/completed` 的修复面小（但严重度见 F4）。
- **P1-4 延后**：官方端点 403（实测复现）+ 自有文件路径存疑，延后合理。
- **M1+M2 一次合入**：依赖链（P0-2 依赖 P0-0）成立，一次合入后统一回归，合理。

### 2.2 新发现问题（PRD 未覆盖 / 覆盖不足）

#### F1（已撤销）——默认 URL 证书疑点

- 原始判断：`VerifyTLSCertificate` 默认 `true`（probe_config.go:131）→ P0-1 后 `InsecureSkipVerify=false`，若默认 URL `speedtest.xyz9923.dpdns.org` 证书不可用，则 MICS 严格校验下全部候选 TLS 失败 → 抽样为空。本机实测该域名表现为 HTTP 301 / HTTPS 超时。
- **撤销依据**：用户确认默认测速 URL 证书实际可用（2026-08-27）。本机超时疑为测试环境网络差异，不以实测否定用户结论。
- **处理**：从 PRD 移除"证书已坏/遗留运营依赖"表述（§3 P0-1 前置、§6 风险、§8、附录 C 均改为"证书可用性已复核确认"）。保留的合理关注点：`VerifyTLSCertificate` 默认 `true`，若用户日后自定义到证书不可用的 URL 且开启校验，抽样为空属预期行为（PRD §6 已注明）。

#### F2（中，建议改）——P0-2 colo 过滤大小写归一化缺失，会静默失效（**已并入 PRD v0.6**）

- 事实链：trace body 返回 **大写** colo（实测 `colo=SJC`）；engine 的 `passColoFilter`（engine.go:243-261）做**精确字符串匹配**、无归一化；而主流程 `configuredColoAllowed` 对两侧都做了 `normalizeColoCode`（httping.go:155）。
- 风险：用户若填小写（`sjc` / `sjc,hkg`）或映射层不归一化，过滤会**静默不生效**（该拒的没拒 / 该留的没留），与主流程行为不一致。
- 处理：已并入 PRD v0.6 P0-2（目标行为 + 验收标准 + 改动文件清单 + P2-7 测试），映射两侧统一走 `task.normalizeColoCode`（或等价归一化）。

#### F3（中，验收承诺过头）——P0-3"同一随机种子下 exploit 选择可复现"按当前方案**不成立**（**已并入 PRD v0.6**）

- `getExploitationPrefixes` 补排序只能让 **exploit 候选列表** 确定性；但引擎整体仍不可复现：
  1. GUI 路径 `Seed=0`（`BuildMCISEngineConfig` 未设 Seed）→ `Run()` 里 `seed=time.Now().UnixNano()`（engine.go:76-77），种子本身就是随机的；
  2. 即便固定种子，`LeafNodes()` 遍历 `nodeMap`（**map 迭代序**，tree.go:158）驱动 `SelectNextPrefix/SelectBeam/RebalanceHeads`（head.go:130,194,397），而 `ThompsonSampler` 的 RNG 流被探索/利用按序消费，**调用顺序依赖 map 序 → 随机流消费顺序不定 → exploit 分支选择也不可复现**。
- 建议（已并入 PRD v0.6）：P0-3 验收改为"exploit **候选列表**在固定种子下可复现（list 确定性）"，并把"全引擎可复现"（需要确定性遍历排序 + Seed 接线）作为独立事项，不用当前 P0-3 的小改动承诺端到端可复现。

#### F4（低，描述偏差，已并入 PRD v0.7）——P1-6 二次 Run 不是"状态串扰"，是**直接返回空 TopN**

- 事实：`Run()` 不重置 `completed`；二次 Run 时 `completed.Load()` 已 ≥ Budget，`schedule()` 主循环条件 `completed < Budget` 直接不满足 → **循环体一次不执行 → 返回空 Top**（engine.go:139）。
- 影响：当前每次调用新建 Engine，确属隐患；但修复（Run 开头重置）仍很小。
- 处理：已并入 PRD v0.7（§1.2 影响列修正 + §3 P1-6 验收补"固定随机种子下断言"）。

#### F5（低，工作量标注偏差，已并入 PRD v0.7）——A.4 表写 P0-1"1 行改动"，实际含两处

- §3 P0-1 目标含：(a) `InsecureSkipVerify` 接线（1 行）+ (b) 目标域名来源改 `TraceURL` 优先 + SNI/Host 解析顺序调整（`BuildMCISProbeConfig` 的 host 解析块，数行）。A.4 表按"1 行改动"计分，略微低估，不影响选型结论。
- 处理：已并入 PRD v0.7（附录 A.4 P0-1 说明改为"1 处接线 + 目标域名来源调整"）。

#### F6（低，清理项，已并入 PRD v0.7）——`.tmp-probe/` 为未跟踪残留

- `git status` 显示 `.tmp-probe/` 未跟踪（含实测脚本 `main.go`），另有多个 `*Zone.Identifier` 残留。
- 处理：已并入 PRD v0.7（§6 风险与注意事项新增"仓库清理"项），合入前删除或加入 `.gitignore`。

#### F7（用户决定不采纳）——兜底 SNI `cf.xiu2.xyz` 为硬编码第三方域名

- `source_mcis.go:115-118`：URL 解析失败时的兜底 SNI/Host 为 `cf.xiu2.xyz`。严格校验下该兜底必然证书失败（该路径仅在 URL 不可解析时触发，影响面小）。
- **决定**：用户明确不处理，维持现状，不移入 PRD。

#### F8（低，已并入 PRD v0.7）——P0-2 工作量"约 15 行"略低估

- 实际含映射层（`HttpingCFColo`+Mode → ColoAllow/Block）+ F2 的归一化，预计 20-30 行 + 测试。
- 处理：已并入 PRD v0.7（附录 A.4 P0-2 说明改为"20-30 行，含映射 + colo 归一化"）。

---

## 3. 分项评审汇总表

| 项 | 事实核验 | 方案可行性 | 评审意见 |
|---|---|---|---|
| P0-0 HEAD→GET | ✅ | ✅ | 最高优先正确；改动最小、收益最大 |
| P0-1 证书校验 | ✅ | ✅（接线） | 接线成立；F1 已撤销，默认 URL 无阻塞 |
| P0-2 colo 过滤 | ✅ | ✅ | F2 归一化已并入 PRD v0.6 |
| P0-3 exploit 排序 | ✅ | ✅（列表级） | F3 范围界定已并入 PRD v0.6 |
| P1-5 分裂优化 | ✅ | ✅ | 方案 A+B 合理 |
| P1-6 引擎复用 | ✅ | ✅ | 已并入 PRD v0.7（F4） |
| P1-4 带宽感知 | ✅（403 复现） | 延后正确 | 维持 backlog |
| P2-7 单元测试 | ✅（当前无测试属实） | ✅ | 补 F2/F3/F4 相关用例（F1 撤销） |
| §8 决策 #1 P1-4 延后 | — | ✅ | 同意 |
| §8 决策 #2 保持默认 URL | — | ✅ | 维持（F1 撤销，证书可用性已复核） |
| §8 决策 #3 P0-3 纳入 | — | ✅ | 同意（按 F3 范围界定验收） |
| §8 决策 #4 M1+M2 一次合入 | — | ✅ | 同意 |

---

## 4. 结论与合入建议

**结论：事实层无可指摘（PRD 的每条代码事实与实测证据均经复核属实），方案与里程碑设计合理，"已定稿、可开工"成立。** F1 经用户确认撤销；F2、F3、F4、F5、F6、F8 已并入 PRD v0.7；F7 按用户决定不采纳。

已随 PRD v0.7 落实：
1. **F2**：P0-2 映射两侧 colo 归一化，避免过滤静默失效（并入 PRD §3 P0-2 / §4.2 / §P2-7）。
2. **F3**：P0-3 验收收敛为"exploit 候选列表级"可复现，全引擎可复现另立事项（并入 PRD §3 P0-3 范围界定）。
3. **F1 撤销**：PRD 移除证书"已坏/遗留运营依赖"表述（§3 P0-1 前置、§6 风险、§8、附录 C），改为"证书可用性已复核确认"。
4. **F4**：P1-6 影响修正为"二次 Run 直接返回空 TopN"，验收补固定种子回归（并入 PRD §1.2 / §3 P1-6）。
5. **F5/F8**：附录 A.4 工作量标注修正（P0-1"1 处接线 + 目标域名来源调整"；P0-2"20-30 行，含映射 + 归一化"）。
6. **F6**：新增仓库清理项（`.tmp-probe/`、`*Zone.Identifier` 残留，并入 PRD §6）。
7. **F7 不采纳**：兜底 SNI `cf.xiu2.xyz` 硬编码按用户决定维持现状，不移入 PRD。

M3（测试补齐）应覆盖：F2 的大小写用例、F3 的候选列表确定性用例、F4 的二次 Run 复用（固定种子）。

> 说明：本评审仅核验 PRD 与现状代码/实测的一致性，不含对实现 PRD 后的新代码评审；建议 M1+M2 合入后对实际 diff 再做一次对应评审。
