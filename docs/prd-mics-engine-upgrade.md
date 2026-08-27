---
title: MICS 抽样引擎升级 PRD
mode: prd
module: internal/mcis + internal/appcore/source_mcis.go
version: v0.7（双向钢人验证评审修订）
created_at: 2026-08-27
updated_at: 2026-08-27
status: 已定稿，可开工
---

# PRD：MICS 抽样引擎升级

## 1. 背景与问题

### 1.1 现状（基于代码事实）

- MICS（输入源 `ip_mode = "mcis"`，界面显示"抽样"）是输入源候选筛选模式：先用独立抽样搜索引擎在 Cloudflare CIDR 内探索候选 IP，再交给 CFST 主流程做最终测速（`internal/appcore/source_mcis.go:18-65`）。
- 引擎核心为分层 Thompson Sampling（`internal/mcis/bandit`）：前缀树按探测结果动态分裂（IPv4 每次 +2 位至 /24，IPv6 每次 +4 位至 /56），多 SearchHead 保多样性，exploit/explore 动态平衡，Top-N 收集。
- 探测为对 `https://<ip>/cdn-cgi/trace` 的 **HEAD** 请求（`probe/trace.go:136`），多轮、跳过首轮握手，记录 connect / TLS / TTFB 三段延迟；以 2xx 判定 OK。
- `internal/mcis` 包当前无任何单元测试。

### 1.2 问题清单（含可行性评审新增/修正项）

| # | 严重度 | 位置（代码事实） | 问题描述 | 影响 |
|---|--------|------------------|----------|------|
| **P0-0** | **致命** | `probe/trace.go:136` | **探测方法用 HEAD，但 Cloudflare 对 `/cdn-cgi/trace` 的 HEAD 返回 404（GET 才 200，实测）**，而 OK 判定要求 2xx | **所有探测均判失败 → 成功率信号全灭、延迟信号退化，bandit 退化为随机探索**；且 HEAD 无 body，coloe 无法解析 |
| P0-1 | 高 | `source_mcis.go:86` | MICS 探测硬编码 `InsecureSkipVerify: true`，不理会用户 `verify_tls_certificate` 配置 | 抽样 IP 未校验目标域名证书，候选里混入对指定域名 TLS 不合法/握手失败的节点 → 后续下载阶段出现 SSL 错误 |
| P0-2 | 高 | `probe/trace.go:272` | `parseTrace()` 从未被调用；探测用 HEAD 无 body 且被丢弃；`Result.Trace` 恒为空 | `ColoAllow / ColoBlock` 地区过滤完全不生效（前置依赖 P0-0 的 GET 改造） |
| P0-3 | 低 | `engine/engine.go:357-395` | `getExploitationPrefixes()` doc 注释声称"按最佳排序"，实现为 map 迭代 + 按 3x/1x 重复加权 | **加权本身生效（加权随机选取不依赖顺序），仅注释与实现不符**；为可复现性补排序，属低风险一致性修正 |
| P1-5 | 中 | `bandit/tree.go`、`engine/engine.go:337` | `MinSamplesSplit=5` 即允许分裂；`trySplit` 每 20 样本拆 `Heads*2` 个；树无大小上限 | 分裂碎片化，弱区段被过度细分 |
| P1-6 | 低 | `engine/engine.go:60` | `Run()` 不重置 `seenIPs / submitted / completed` | 同一 Engine 二次运行 `completed` 未重置 → **直接返回空 TopN**（当前每次调用新建 engine，属隐患） |
| P1-4 | 中（待决策） | `probe/trace.go`、`engine/result.go` | 探测仅延迟无带宽信号 | 详见 §3 P1-4 与附录 B：可行性评审后建议降级/延后 |

## 2. 目标与非目标

### 2.1 目标

- **修复 P0-0：探测方法 HEAD→GET**，让成功率/延迟/colo 信号全部真实生效（最高优先）。
- 抽样候选对指定测速域名 **SSL 可靠**：证书校验跟随用户配置。
- **COLO 地区过滤真实生效**。
- exploit 阶段加权可复现（P0-3 一致性修正）。
- 引擎**可重复运行、状态干净**。
- 在**可维护性与性能加权**前提下评估带宽维度（详见 §3 P1-4）。

### 2.2 决策原则

本版所有 P1+ 优化项按 **总分 = 0.5 × 可维护性 + 0.5 × 性能**（各 5 分制）选型：
- **可维护性**：代码量、配置面、失败模式、回归风险（是否触碰 bandit 核心奖励/调度）。
- **性能**：墙钟耗时、探测负载、候选命中率/CFST 配额节省。
- 对触碰 bandit 核心逻辑的改动从严，能放到外层"gate/后处理"的优先放外层。
- **一切需求须通过可行性评审（附录 B 双向验证）后方可进入里程碑**。

### 2.3 非目标

- 不做独立于 GUI 的抽样服务（若需要另立项目）。
- 不改 CFST 主流程测速逻辑与既有对外契约（`RunMCISSearch` 签名与返回值保持不变）。
- 不引入新依赖/不改变现有探测协议族。
- 本迭代不修改 bandit 奖励公式（见附录 A 方案对比结论）。

## 3. 需求描述

### P0-0 探测方法 HEAD→GET（新增，最高优先）

**现状**：`probe/trace.go:136` 用 `http.MethodHead` 请求 `/cdn-cgi/trace`。

**实测证据**：`HEAD /cdn-cgi/trace` 返回 404、`GET /cdn-cgi/trace` 返回 200 且带 body（`colo=…`）；HEAD 按 RFC 无 body。当前所有探测 OK=false。

**目标行为**：探测改用 GET；读取并（在 P0-2 中）解析响应体；OK 判定保持 2xx。

**验收标准**：单 IP 探测返回 200 且 `OK=true`；多轮探测平均后的 `OK/TotalMS` 反映真实链路质量；budget 内成功率分布非全零。

**额外收益**：GET 的 body 即 P0-2 colo 解析的数据源；`/cdn-cgi/trace` body 约 300B，多轮开销可忽略。

### P0-1 证书校验接入（阻塞项）

**现状**：`source_mcis.go:86` 硬编码 `InsecureSkipVerify: true`。

**目标行为**：`probeCfg.InsecureSkipVerify = !cfg.VerifyTLSCertificate`，跟随界面"严格校验证书"开关；**共享"目标域名"来源改为 `cfg.TraceURL` 优先（空则回退 `cfg.URL`）**，与主流程 trace 阶段判定口径一致（详见 §4.1 分层设计与附录 B.4）；SNI 解析顺序保持 `cfg.SNI` → `cfg.HostHeader` → 目标域名 → 兜底 host。

**前置条件（运营依赖）**：测速 URL 域名证书必须可用。默认 `https://speedtest.xyz9923.dpdns.org/500m` 证书可用性已确认（2026-08-27 复核），无阻塞项。

**验收标准**：
- `verify_tls_certificate=true` 时，抽样候选仅包含对目标域名 TLS 握手成功且证书合法的 IP。
- `verify_tls_certificate=false` 时行为与现状一致。

### P0-2 COLO 地区过滤修复（前置依赖 P0-0）

**现状**：`parseTrace()` 死代码；探测无 body 可读；`Result.Trace` 恒为空。

**目标行为**：P0-0 改 GET 后读取 `/cdn-cgi/trace` body，解析 `colo` 等字段写入 `Result.Trace`；`engine.processOneResult` 走既有 `passColoFilter`；**并将主配置的 `HttpingCFColo` + 模式映射到引擎 `ColoAllow/ColoBlock`**（现 `BuildMCISEngineConfig` 硬编码 `ColoAllow=nil`，过滤实际关闭）。**colo 匹配两侧做大小写归一化**（F2）：实测 trace body 返回大写 colo（如 `colo=SJC`），engine `passColoFilter` 现为精确匹配，映射时统一转大写并与主流程 `normalizeColoCode` 口径一致，避免大小写差异导致过滤静默失效。

**验收标准**：
- 配置 `HttpingCFColo`/`ColoAllow`/`ColoBlock` 后，TopN 只包含/排除指定 colo 的 IP。
- **大小写混合配置（如 `sjc`、`sjc,hkg`）过滤同样生效，与主流程 `HttpingCFColo` 行为一致。**
- 未配置过滤时行为与现状一致。

### P0-3 exploit 加权一致性修正（低严重度）

**现状**：`getExploitationPrefixes()` doc 注释与实际不符（未排序，但加权随机选取功能正确）。

**目标行为**：按前缀最佳 `ScoreMS` 升序生成加权列表（tier1 3x / tier2 1x），保证 exploit **候选列表**的确定性与加权语义稳定。

**验收标准**：同一随机种子下，exploit **候选列表内容与排序**可复现；加权语义不变（3x/1x）。
> **范围界定（F3）**：本项仅承诺"候选列表级"确定性，不承诺引擎端到端可复现——`LeafNodes()` 遍历 `nodeMap`（map 迭代序）驱动 `SelectNextPrefix/SelectBeam/RebalanceHeads`，`ThompsonSampler` 随机流消费顺序受其影响；且 GUI 路径 `Seed=0` 使用时间种子（`engine.go:76-77`）。如需全引擎可复现，应另立事项（确定性遍历排序 + Seed 接线）。

### P1-5 分裂策略优化（方案 A 必做 + 方案 B 叠加）

**目标行为**：
- `MinSamplesSplit` 5 → 8（子节点至少 ~2 样本再拆，减少碎片化）。
- 新增树节点总数上限（默认 4096，超出后停止分裂）。
- 叠加：`trySplit` 对分裂候选增加信息量门槛（复用现有 `InformationGain()`）。

**验收标准**：同等预算下节点总数可控（≤ 上限）；命中 TopN 的准确率不下降。

### P1-6 引擎可复用

**目标行为**：`Run()` 开头重置 `seenIPs / submitted / completed` 等全部可变状态。

**验收标准**：同一 Engine 连续 `Run` 两次，第二次输出与新建 Engine 一致；补回归测试（固定随机种子下断言）。

### P1-4 带宽感知（**已决策：延后到 backlog**）

**可行性结论（附录 B）**：官方端点 `speed.cloudflare.com/__down` 对 Range/裸请求返回 403；用户自有 `/500m` 实测 404；Range 依赖用户自有文件且需 Range 支持与 403/无 Range 降级路径。按"可维护×性能"权重，收益有限、维护面大。

**决策（2026-08-27）**：**延后到 backlog，本版不实现**。主流程 CFST 下载阶段已按 `minSpeedMB` 过滤慢 IP，MICS 侧带宽 gate 非正确性必需。若未来纳入，沿用附录 A 结论（方案 B 门槛 gate、不改 bandit 奖励公式）。

### P2-7 单元测试（随里程碑补齐）

随各里程碑为改动点补单测：bandit（Thompson 采样、分裂、exploit 加权）、probe（GET trace 解析、多轮平均）、engine（调度、TopN 收集、Run 复用）、source_mcis（集成契约）；**另补 colo 大小写归一化用例（P0-2）与 exploit 候选列表确定性用例（P0-3）。**

### P2-8 历史结果先验（延后，backlog）

维持附录 A.3 结论：不纳入本版。

### P2-9 参数调优（可选）

按真实抽样数据回测调整默认参数，作为 M3 之后的可选项。

## 4. 技术方案概览

### 4.1 探测配置分层设计（架构约束）

MICS 与主流程是"预筛 → 终验"上下游，配置按两层管理，**不整包覆盖主 strategy，也不完全独立**：

| 层 | 配置项 | 处理 | 落点 |
|---|---|---|---|
| **共享层（正确性，单一来源）** | 证书校验 `verify_tls_certificate` | MICS 继承主配置 | `InsecureSkipVerify = !cfg.VerifyTLSCertificate`（P0-1） |
| | 目标域名（SNI/Host） | **`TraceURL` 优先，回退 `URL`**，与主流程 trace 一致 | SNI/Host 解析 |
| | 超时 `maxDelayMS` | 继承 | `Timeout`（clamp 1000-3000） |
| | UA `userAgent` | 继承 | `UserAgent` |
| | COLO 过滤 `httpingCFColo`+mode | 映射到引擎 | `ColoAllow/ColoBlock`（P0-2 补充） |
| **独立层（搜索性能，代码派生）** | Rounds/Budget/Heads/Beam/Concurrency/Split | MICS 自持，由 `limit`+`Routines` 派生，**不受 strategy 影响** | `BuildMCISEngineConfig`、`mcisprobe.Config` |
| **探测方法** | HEAD/GET | 与 strategy 无关，统一 GET | P0-0 |

**边界**：`Rounds` 耦合自 `PingTimes`（用户直觉的"发包次数"），作为共享性能代理保留，不新增配置面（附录 B.4）。

### 4.2 改动文件清单

| 文件 | 改动内容 |
|------|----------|
| `internal/mcis/probe/trace.go` | P0-0：HEAD→GET；P0-2：读取并解析 trace body |
| `internal/appcore/source_mcis.go` | P0-1：`InsecureSkipVerify` 跟随配置 + 目标域名 `TraceURL` 优先；P0-2：`HttpingCFColo`→`ColoAllow/Block` 映射 + colo 大小写归一化 |
| `internal/mcis/engine/engine.go` | P0-2：`passColoFilter` 大小写归一化；P0-3：exploit 排序确定性；P1-5：分裂接线；P1-6：Run 状态重置 |
| `internal/mcis/bandit/tree.go`、`arm.go` | P1-5：`MinSamplesSplit`、节点上限、信息量门槛 |
| 新增测试文件 | P2-7 |

**升级后探测数据流**：IP → TCP connect → TLS 握手（可选证书校验）→ **GET** `/cdn-cgi/trace`（解析 colo）→ 更新 bandit 奖励（成功率 + 延迟 + colo）。

## 5. 整体验收

- `go build ./... && go test ./internal/mcis/... ./internal/appcore/...` 全部通过。
- 真实抽样验证一轮：确认 (a) 探测返回 200/OK 正常、成功率分布非全零；(b) `verify_tls_certificate` 开/关行为符合预期；(c) colo 过滤生效；(d) TopN 质量（交由 CFST 测速后命中率）相比基线不下降。

## 6. 风险与注意事项

- **P0-0 是其余探测相关修复的前置**：HEAD→GET 改动小，但需回归验证探测耗时（GET 带 body，多轮平均成本略增，约 300B，可忽略）。
- **P0-1 前置依赖**：测速 URL 域名证书可用性已复核确认（默认 URL 无阻塞）；若用户日后自定义到证书不可用的 URL 且开启校验，抽样结果将为空，属预期行为。
- **P1-5 行为变化**：提高分裂门槛后深钻变慢但更稳；需对比基准确认不回归。
- **仓库清理（F6）**：`.tmp-probe/`（实测脚本残留）与多个 `*Zone.Identifier` 为未跟踪文件，合入前删除或加入 `.gitignore`，避免污染仓库。

## 7. 里程碑建议（已决策：M1+M2 一次合入）

- **合入批次（M1+M2 合并，一次交付）**：P0-0（HEAD→GET）+ P0-1（证书校验）+ P0-2（colo）+ P0-3（一致性）+ P1-5（分裂优化）+ P1-6（引擎复用）。一次合入后统一回归验证。
- **M3（后续）**：单元测试补齐 + （可选）参数回测；P1-4/P2-8 已在 backlog。

## 8. 待确认事项（已决策，2026-08-27）

| # | 事项 | 决策 |
|---|---|---|
| 1 | P1-4 带宽感知去留 | **延后到 backlog**（§3 P1-4） |
| 2 | 测速 URL 域名处理 | **保持默认** `https://speedtest.xyz9923.dpdns.org/500m`，不切换 `*.qzz.io`（证书可用性已复核确认） |
| 3 | P0-3 是否纳入本次 | **纳入**（M1+M2 合并批次，按 F3 范围界定验收） |
| 4 | 合入节奏 | **M1+M2 一次合入**（§7） |

**无遗留运营依赖**：默认 URL 证书可用性已确认（2026-08-27 复核），P0-1 开启证书校验后可正常产出候选。

---

## 附录 A：算法增益可行性分析与方案对比

> 打分：可维护性 5 分制（代码量、配置面、失败模式、回归风险）；性能 5 分制（墙钟、探测负载、候选命中率/配额节省）。总分 = 0.5×维护 + 0.5×性能。触碰 bandit 核心（奖励/调度）从严。

### A.1 P1-4 带宽感知

| 方案 | 描述 | 维护 | 性能 | 总分 | 量化代价/收益 |
|---|---|---|---|---|---|
| A | 带宽并入 bandit 奖励公式 | 2 | 4 | 3.0 | 每个延迟过关候选多 1 次 GET；需重调 exploit/explore，回归风险大 |
| B | 仅对 Top-N 池做吞吐门槛 gate（不改奖励公式） | 4 | 4 | 4.0 | 池内约 3×limit 次 256KB Range GET；**可行性受限（见附录 B）** |
| C | 维持现状 | 5 | 2 | 3.5 | 慢 IP 全量进 CFST |

### A.2 P1-5 分裂策略

| 方案 | 描述 | 维护 | 性能 | 总分 |
|---|---|---|---|---|
| **A（必做）** | MinSamplesSplit 5→8 + 树节点上限 4096 | 5 | 4 | 4.5 |
| **B（叠加）** | 分裂候选信息量门槛（复用 InformationGain） | 3 | 4 | 3.5 |
| C | 维持现状 | 5 | 2 | 3.5 |

### A.3 P2-8 历史先验

| 方案 | 描述 | 维护 | 性能 | 总分 |
|---|---|---|---|---|
| A | 落盘 TopN → 加载为先验 | 2 | 3 | 2.5 |
| B | 先验文件可配置关闭 | 3 | 3 | 3.0 |
| **C（入选：延后）** | 不纳入本版，backlog | 5 | 3 | 4.0 |

### A.4 P0 项

| 项 | 维护 | 性能 | 说明 |
|---|---|---|---|
| P0-0 探测 HEAD→GET | 4 | 5 | 1 处方法改动，激活全部探测信号（新增，附录 B 发现） |
| P0-1 证书校验接入 | 5 | 5 | 1 处接线 + 目标域名来源调整，直接消除 SSL 问题 |
| P0-2 COLO 过滤 | 4 | 4 | GET 改造后 20-30 行（含 `HttpingCFColo`→`ColoAllow/Block` 映射 + colo 归一化） |
| P0-3 exploit 排序 | 5 | 4 | 一致性修正（非功能 bug，见附录 B） |

---

## 附录 B：可行性评审记录（双向验证，2026-08-27）

> 方法：**正向**（需求→代码，逐项核对能否按现有结构实现）+ **反向**（真实运行行为→需求，用实测验证 PRD 假设）。实测环境：本机 curl 直连 Cloudflare。

### B.1 反向验证（实测证据）

| 实测项 | 结果 | 结论 |
|---|---|---|
| `HEAD https://speed.cloudflare.com/cdn-cgi/trace` | **404** | 探测方法缺陷（P0-0）：OK 判定失败 |
| `GET  https://speed.cloudflare.com/cdn-cgi/trace` | **200**，body 含 `colo=SJC` 等 | GET 可行且带 colo |
| `GET .../cdn-cgi/trace` body 中 `colo` 大小写 | **大写**（实测 `colo=SJC`） | P0-2 过滤需大小写归一化（F2） |
| `HEAD https://test.xyz9923.qzz.io/cdn-cgi/trace` | **404** | 与 host 无关，Cloudflare 对 trace 的 HEAD 统一 404 |
| `Range GET https://speed.cloudflare.com/__down?bytes=500000000` | **403** | 官方端点拦截，P1-4 不能依赖 |
| `GET https://speed.cloudflare.com/__down?bytes=100000000`（无 Range） | **403** | 官方端点对裸请求也拦截 |
| `Range GET https://test.xyz9923.qzz.io/500m` | **404** | 用户自有测速文件路径需确认 |

### B.2 正向验证（需求→代码）

| 需求 | 判定 | 依据 |
|---|---|---|
| P0-0 HEAD→GET | ✅ 可行，1 处改动 | `probeOnce` 内 `http.MethodHead`→`http.MethodGet`；OK 判定不变 |
| P0-1 证书校验 | ✅ 可行 | `InsecureSkipVerify` 经 `httpcfg.Resolve` 流入 TLS 配置，`!cfg.VerifyTLSCertificate` 接线成立；`cfg.VerifyTLSCertificate` 字段已存在 |
| P0-2 colo 过滤 | ✅ 可行（依赖 P0-0） | `processOneResult` 已读 `Trace["colo"]` 并走 `passColoFilter`，链路存在，只缺 body |
| P0-3 exploit 加权 | ✅ 低风险一致性 | 加权随机选取不依赖顺序，功能正确；补排序仅为可复现 |
| P1-5 分裂优化 | ✅ 可行 | `MinSamples=5`（`DefaultTreeConfig`）、无上限（`SplitNode`）确认 |
| P1-6 引擎复用 | ✅ 可行 | `Run()` 不重置 `seenIPs/submitted/completed`，确认 |
| P1-4 带宽 gate | ⚠️ 受限 | Range 依赖用户自有文件 + 支持情况；官方端点 403；需降级路径 |
| P0-2 colo 归一化（F2，v0.6 新增） | ✅ 需补 | `engine.passColoFilter` 精确匹配（`engine.go:243`），映射层需两侧归一化并与主流程 `normalizeColoCode` 口径一致 |
| P0-3 可复现范围（F3，v0.6 新增） | ⚠️ 列表级 | 全引擎可复现受 `LeafNodes()` map 迭代序 + `Seed=0` 时间种子限制（`engine.go:76-77`），本项仅承诺 exploit 候选列表确定性 |

### B.3 评审结论对 PRD 的修订

1. **新增 P0-0**（最高优先）：探测 HEAD→GET。影响面最大，是 P0-2 的前置，且修复成功率信号。
2. **P0-2 修订**：明确"先 GET 才有 body 可读"，不再以 HEAD 为前提。
3. **P0-3 降级**：由"exploit 加权失效"修正为"注释与实现不符、功能正确"，严重度中→低。
4. **P1-4 建议降级/延后**：官方端点 403 + 自有文件需确认，按权重原则不进入 M2 必做。
5. **新增 F2（P0-2，v0.6）**：colo 过滤两侧大小写归一化——trace body 返回大写 colo（实测 `colo=SJC`），engine `passColoFilter` 精确匹配会静默失效；映射层统一归一化并与主流程口径一致。
6. **新增 F3（P0-3，v0.6）**：可复现性承诺收敛为"exploit 候选列表级"；全引擎端到端可复现（map 迭代序、Seed 接线）另立事项。
7. **撤销 F1（v0.6）**：默认 URL 证书可用性已复核确认，删除原"遗留运营依赖"表述与附录 C 修复步骤。
8. **并入 F4（v0.7）**：P1-6 影响修正为"二次 Run 直接返回空 TopN"，验收补固定种子回归。
9. **并入 F5/F8（v0.7）**：附录 A.4 工作量标注修正（P0-1"1 处 + 域名来源调整"；P0-2"20-30 行 + 测试"）。
10. **并入 F6（v0.7）**：新增仓库清理项（`.tmp-probe/`、`*Zone.Identifier` 残留，见 §6）。
11. **不采纳 F7（v0.7）**：兜底 SNI `cf.xiu2.xyz` 硬编码维持现状（决策：不处理）。

### B.4 探测配置分层设计双向验证（2026-08-27）

**正向（分层需求 → 代码承载）**：共享层各字段落点链路均已存在——证书校验经 `mcisprobe.Config.InsecureSkipVerify → httpcfg.Resolve → TLSConfig`；超时/UA 已继承；独立层参数由 `limit`+`Routines` 派生、无 GUI 面、不受 `strategy` 影响（`strategy` 在 `probe_config.go` 仅归一化为 fast/full，未驱动流水线）。分层可承载，10 项中 8 项已实现。

**反向（代码事实 → 设计自洽）**，发现 3 缺口：

| # | 缺口 | 代码事实 | 修正 |
|---|---|---|---|
| 1 | 域名来源不一致 | 主流程 trace 用 `cfg.TraceURL`（空则 `DeriveTraceURL(URL)`，`probe_config.go:316-328`）；MICS 直接用 `cfg.URL` host（`source_mcis.go:98-104`）。TraceURL≠URL 时判定口径漂移 | 共享"目标域名"改为 `TraceURL` 优先 |
| 2 | COLO 过滤未接线 | `BuildMCISEngineConfig` 硬编码 `ColoAllow=nil`（`source_mcis.go:74`）；即便解析出 colo，引擎过滤仍关闭 | `HttpingCFColo`+mode → `ColoAllow/Block`（并入 P0-2） |
| 3 | 证书校验硬编码 | `source_mcis.go:86` `InsecureSkipVerify=true` | P0-1 接线 |

**确认项**：`Rounds` 耦合自 `PingTimes`（`source_mcis.go:82`）——`PingTimes` 是用户直觉的"发包次数"，作为共享性能代理保留，不新增配置面。

---

## 附录 C：默认测速 URL 证书说明（2026-08-27 复核）

默认测速 URL `https://speedtest.xyz9923.dpdns.org/500m` 的 TLS 证书可用性已复核确认，P0-1 开启证书校验无阻塞项，无需证书修复动作，亦无需切换 `*.qzz.io`。
