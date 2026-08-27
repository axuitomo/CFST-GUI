# DDD 设计文档：MICS 抽样引擎（对齐 PRD v0.7）

- 依据：`docs/prd-mics-engine-upgrade.md`（v0.7，2026-08-27，已定稿可开工）
- 范围：`internal/mcis`（抽样上下文）+ `internal/appcore/source_mcis.go`（输入源接入）+ 与之共享配置的 `internal/probecore` / `internal/task`
- 目标：把 PRD 的"问题清单 → 需求 → 方案"显式化为领域驱动设计模型，使每一项 PRD 变更都能落到具体的 DDD 构件（聚合/实体/值对象/领域服务/事件/应用服务）上，并与仓库现有分层（`docs/architecture-constraints.md`、`docs/agent-code-architecture.md`）保持一致。
- 约定：**DDD 术语与代码事实一一对应**；不引入 DDD 框架/依赖（保持现有 Go 包结构与"薄入口 + 共享核心"分层）。
- 实施状态：**阶段 1-4 代码与自动化测试已完成**；公网真实抽样 smoke 尚未执行，保留为交付前人工验收项。

---

## 1. 战略设计（Strategic Design）

### 1.1 领域定位

- **领域**：Cloudflare CDN IP 优选与测速（CFST-GUI）。本 PRD 聚焦其中的**候选筛选**子域。
- **核心问题**：在 Cloudflare CIDR 空间内，以有限探测预算高效探索，产出对"目标测速域名"链路质量最优的 Top-N 候选 IP，交给主流程终验（预筛 → 终验）。
- **价值主张**：用分层 Thompson Sampling 在"探索新区域"与"利用已知优区"间自适应平衡，用更少的探测换取更高的候选命中率，从而**节省 CFST 主流程的测速配额/墙钟**。

### 1.2 子域划分

| 子域 | 类型 | 职责 | 代码承载 |
|---|---|---|---|
| **抽样搜索**（bandit 引擎） | **核心子域** | 前缀树动态分裂、多 SearchHead 多样性、exploit/explore 平衡、Top-N 收集——差异化能力所在 | `internal/mcis/bandit`、`internal/mcis/engine` |
| **探测** | 支撑子域 | 对候选 IP 发起 HTTP trace 探测（多轮、跳过首轮）、记录 connect/TLS/TTFB 延迟、解析 colo、判定 OK | `internal/mcis/probe` |
| **输入源接入** | 支撑子域 | 将配置与 CIDR/IP 输入装配为 MICS 请求，并把 Top-N 转为主流程候选 | `internal/appcore/source_mcis.go` |
| **CIDR 处理** | 通用子域 | 前缀解析、去重、分裂、随机取样 | `internal/mcis/cidr` |
| **配置归一化** | 通用子域 | ProbeConfig 默认值、净化、TraceURL 派生 | `internal/probecore` |
| **HTTP 基础设施** | 通用子域 | 请求 profile（SNI/Host/UA/证书）、TLS/HTTP 客户端 | `internal/httpcfg`、`internal/httpclient` |

### 1.3 限界上下文（Bounded Context）

| 限界上下文 | 代码承载 | 职责 | 上下文类型 |
|---|---|---|---|
| **配置上下文** ConfigContext | `internal/probecore`、`internal/httpcfg` | 配置 schema、默认值、净化、派生（`TraceURL` 优先于 `URL`）；跨端契约 `cfst-gui-config-v2` | 通用 |
| **抽样上下文** SamplingContext | `internal/mcis` | bandit 引擎 + 探测 + 调度 + TopN——**本 PRD 的主战场** | 核心 |
| **输入源上下文** SourceContext | `internal/appcore/source_*` | 输入源解析、MICS 候选接入（`RunMCISSearchContext`）、候选去重 | 支撑 |
| **测速上下文** SpeedTestContext | `internal/task` | CFST 主流程（tcping/httping/download/colo filter）——抽样结果的下游终验 | 支撑 |
| **编排上下文** OrchestrationContext | `internal/app`、`mobileapi`、`frontend` | CLI/GUI/WebUI/Android 传输、生命周期、事件发布 | 通用 |

### 1.4 上下文映射（Context Map）

```
               共享内核（共享层配置：证书校验/目标域名/超时/UA/COLO）
   ┌─────────────┐  ────────────────────────────────────────────►  ┌──────────────┐
   │ 配置上下文   │                                                    │ 抽样上下文    │
   │ Config      │                                                    │ Sampling     │
   └─────────────┘                                                    └──────┬───────┘
                                                                               │ 候选 IP 列表
                                                                               ▼ (Conformist)
   ┌─────────────┐  ▶ RunMCISSearchContext  ──────────────────────────►  ┌──────────────┐
   │ 输入源上下文 │                                                    │ 测速上下文    │
   │ Source      │                                                    │ SpeedTest    │
   └─────────────┘                                                    └──────────────┘
```

- **配置上下文 ↔ 抽样上下文：共享内核（Shared Kernel）**。`ProbeConfig` 的共享层字段（证书校验 `verify_tls_certificate`、目标域名 `TraceURL` 优先、超时 `maxDelayMS`、UA、COLO 过滤）是单一事实来源，抽样上下文继承而非整包覆盖主 strategy。P0-1/P0-2 即是在**加固这条共享内核**（证书校验接线、COLO 过滤接线）。
- **抽样上下文 → 测速上下文：上游提供者 / 下游消费者（Conformist）**。抽样上下文是"预筛"，测速上下文是"终验"。对外契约固定为 `RunMCISSearch` 签名与返回值不变（PRD §2.3 非目标），抽样上下文遵循下游可消费的候选 IP 列表契约。
- **输入源上下文 → 抽样上下文：调用方（Caller）**。通过 Go 包依赖调用，无需防腐层；`RunMCISSearchContext` 承担装配（构建 engine 配置 + probe 配置 + 执行 + 转换 TopN）。
- **边界纪律**：抽样上下文不反向依赖测速上下文内部（不 import `internal/task`）；`strategy` 仅在配置上下文归一化为 fast/full，不驱动抽样流水线（PRD §4.1 独立层不受 strategy 影响）。

---

## 2. 通用语言（Ubiquitous Language）

| 术语 | 定义 | 代码映射 |
|---|---|---|
| 抽样（MICS） | `ip_mode="mcis"` 的候选筛选模式：独立引擎先筛候选，再交主流程终验 | `source_mcis.go` |
| 前缀（Prefix） | 一个 CIDR 网络段，是树的节点标识 | `netip.Prefix`、`bandit.ArmNode.Prefix` |
| 分裂（Split） | 父前缀按步长细分出子前缀（IPv4 +2 位至 /24，IPv6 +4 位至 /56） | `cidr.SplitPrefix`、`ArmTree.SplitNode` |
| 样本（Sample） | 一次对该前缀内 IP 的探测结果更新 | `ArmNode.Update`、`ArmStats.Samples` |
| 探测（Probe） | 对 `https://<ip>/cdn-cgi/trace` 的 HTTP 请求（**GET**，PRD P0-0），多轮、跳过首轮 | `Prober.ProbeHTTPTraceMulti` |
| 延迟分段 | connect / TLS / TTFB / Total 四段毫秒延迟 | `probe.Result` |
| OK | 探测返回 2xx；所有结果都更新 bandit，只有 `OK=true` 且通过 colo 过滤的结果可进入 Top-N | `probe.Result.OK`、`engine.processOneResult` |
| colo | Cloudflare 节点三字码（trace body 返回**大写**，如 `colo=SJC`） | `Result.Trace["colo"]`、`task.ExtractColo` |
| 地区过滤 | ColoAllow 白名单 / ColoBlock 黑名单，决定结果是否进 TopN | `engine.passColoFilter` |
| 搜索头（SearchHead） | 独立采样器的探索单元，保多样性 | `bandit.SearchHead`、`HeadManager` |
| 探索/利用（explore/exploit） | 探索未知区 vs 利用已知优区，动态平衡 | `submitOneTask` 的 `exploitRate` |
| 束宽（Beam） | 每个头在一次选择中考察的候选前缀数 | `engine.Config.Beam` |
| 预算（Budget） | 本轮探测总次数上限 | `engine.Config.Budget` |
| Top-N | 按分数保留的最优 IP 集合 | `TopNCollector` |
| 候选（Candidate） | 抽样产出、交给主流程终验的 IP；必须先满足 `OK=true` 与地区过滤资格 | `RunMCISSearchContext` 返回的 entries |
| 证书校验 | 跟随 `verify_tls_certificate` 对目标域名做 TLS 校验证书 | `InsecureSkipVerify`（P0-1） |
| 目标域名 | trace/SNI/Host 解析的域名来源，`TraceURL` 优先于 `URL` | P0-1、`request_profile.go` |

---

## 3. 战术设计（Tactical Design）

### 3.1 聚合（Aggregate）

**聚合 1：ArmTree（前缀树，聚合根）**
- 根：`ArmNode`（实体，标识 = `Prefix`）。
- 一致性边界：分裂、统计更新、节点查找都经 `ArmTree` 暴露的方法（`Update` / `SplitNode` / `GetSplitCandidates` / `LeafNodes` / `Size`），不直接暴露内部 `nodeMap` 的可变引用。
- **聚合不变式（Invariant）**：
  1. `CanSplit` 前置：未分裂 + `Samples >= MinSamplesSplit` + 位长未达 `MaxBitsV4/V6`；
  2. 分裂后父节点 `IsSplit=true`、子节点挂到父；
  3. 节点总数 ≤ `MaxTreeNodes`（**P1-5 新增**，默认 4096）；根前缀去重后若已超限，`Run` 在建树前返回明确错误；
  4. 分裂候选须满足 `InformationGain() >= MinSplitInformationGain`（**P1-5 叠加**，默认 0.10）；一次分裂只有在全部子节点都容得下时才原子执行。
- **PRD 变更触点**：P1-5 修改不变式 1/3/4（`MinSamplesSplit` 5→8、节点上限、信息量门槛）。

**聚合 2：SearchHead（搜索头，聚合根）**
- 根：`SearchHead`（实体，标识 = `HeadID`）；包含 `ThompsonSampler`（领域服务）与聚焦前缀 + 探索历史。
- 一致性边界：每个头独立采样器、独立焦点与历史；`HeadManager` 负责跨头多样性（斥力计算、再平衡）。
- **PRD 变更触点**：无直接改动（P0-3/F3 涉及的是 exploit 选择对采样器随机流的消费顺序——领域服务确定性，见 §3.3/§6）。

**聚合 3：TopNCollector（Top-N 收集器）**
- 一致性边界：`Consider` 内部去重（IP 维）、容量上限、按 `ScoreMS` 淘汰最差；`Snapshot` 返回排序快照。
- 语义上更接近"内存结果仓库"，因当前无持久化，按聚合管理。
- **PRD 变更触点**：P0-1/P0-2 决定“是否进入 TopN”，发生在聚合边界之外：`processOneResult` 仍用全部成功/失败结果更新 `ArmTree`，但只有 `OK=true` 且通过 colo 过滤的结果可调用 `Consider`。实施前失败结果会按 `2×timeout` 罚分进入未满的 TopN；现已增加资格门禁，保证严格证书校验后的候选均为成功探测。

### 3.2 实体与值对象

**实体（Entity，有标识）**
| 实体 | 标识 | 职责 | 代码 |
|---|---|---|---|
| `ArmNode` | `Prefix` | 维护 Beta 成功率先验 + Normal-Gamma 延迟后验、Welford 方差、分裂状态 | `bandit/arm.go` |
| `SearchHead` | `HeadID` | 聚焦前缀、探索历史、绑定采样器 | `bandit/head.go` |
| 候选 IP | `IP` | 抽样产出的候选（无独立对象，以 `netip.Addr` 表示） | `engine.TopResult.IP` |

**值对象（Value Object，无标识、不可变）**
| 值对象 | 构成 | 代码 |
|---|---|---|
| `Prefix` | 网络段（IPv4/IPv6） | `netip.Prefix` |
| `ProbeResult` | OK + Status + Error + ConnectMS/TLSMS/TTFBMS/TotalMS + Trace + When | `probe.Result` |
| `ArmStats` | Samples/Successes/Failures/MeanLatency/VarLatency/SuccessRate/IsSplit | `bandit.ArmStats` |
| `Colo` | 三字码（**归一化大写**，P0-2/F2） | `task.normalizeColoCode` 语义 |
| `TreeConfig` | SplitStep/MaxBits/MinSamples/MaxNodes/MinInformationGain | `bandit.TreeConfig` |
| `EngineConfig` | Budget/TopN/Concurrency/Heads/Beam/Split*/MaxTreeNodes/MinSplitInformationGain/ColoAllow/Block/Seed | `engine.Config` |
| `Request` / `Response` | 引擎入参（CIDRs+Probe）/ 出参（Top） | `engine.Request/Response` |
| `TopResult` | 候选 + 分数 + 前缀统计 + Trace | `engine.TopResult` |

### 3.3 领域服务（Domain Service）

| 领域服务 | 输入 → 输出 | 职责/算法 | PRD 触点 |
|---|---|---|---|
| `ThompsonSampler` | 候选节点 → 采样分数/最佳节点 | 后验采样：Beta（成功率）+ Normal-Gamma（延迟）；<3 样本乐观初始化；<10 样本探索加成；失败按 2×timeout 罚 | 不改奖励公式（PRD §2.3） |
| `SplitStrategy`（分裂决策） | 叶子节点 → 候选/实际分裂 | 优先级 = 平均延迟 − 成功率加成 − 信息量加成；排序取 Top；受聚合不变式约束 | **P1-5**（MinSamples/节点上限/信息量门槛） |
| `ExploitSelector`（利用选择） | TopN 快照 → 加权前缀列表 | tier1（≤1.2×best）3 倍权、tier2（≤1.5×best）1 倍权；**按 best 升序生成列表（P0-3）** | **P0-3 / F3** |
| `ColoFilter`（地区过滤） | colo + Allow/Block → 是否放行 | 白名单/黑名单；**两侧大小写归一化（P0-2/F2）**，与主流程口径一致 | **P0-2 / F2** |
| `Prober`（探测） | IP → ProbeResult | 多轮、跳过首轮、平均；**GET `/cdn-cgi/trace`（P0-0）**；解析 trace body 写 `Trace`（P0-2） | **P0-0 / P0-2** |

> 设计说明：`ThompsonSampler` 在代码里是 `SearchHead` 的成员，但语义上是无状态算法（除 RNG 与罚参）的领域服务，挂到头上以保持每头独立随机流。`RNG` 消费顺序受 `LeafNodes()` map 迭代序影响——这是 F3 可复现性问题的领域根因（见 §6）。

### 3.4 领域事件（Domain Event）

当前实现采用**通道驱动的事件循环**（`engine.go` 的 `tasks`/`done` channel），是隐式领域事件，未显式建模为事件对象。按"最小改动、不引入事件总线"原则，以既有通道作为事件载体，语义对齐如下：

| 领域事件 | 语义 | 当前载体 | 消费者 |
|---|---|---|---|
| `ProbeCompleted(prefix, ip, result)` | 一次探测结束 | `done chan probeDone` | 调度循环：更新树、判 colo、进 TopN、提交下一个 |
| `PrefixSplit(parent → children)` | 前缀分裂 | `trySplit()` 返回值 | 树状态；后续选择可及子节点 |
| `TopNChanged(candidate, score)` | TopN 更新 | `TopNCollector.Consider` | `Snapshot`/`Best` 读取方 |
| `CandidateRejected(result, reason)` | 因探测失败或地区过滤被拒 | `processOneResult` 早退 | （隐式；`reason=probe/colo`，可选调试事件） |

> 建议：**本轮不把事件对象化**（收益低、触碰调度核心），保持 channel 语义；仅在未来需要跨上下文订阅（如抽样进度进前端事件流 `probe:event`）时再显式化。

### 3.5 仓储（Repository）

- **现状：纯内存、无持久化仓储**。聚合由 `Engine` 在 `Run` 内创建并持有，生命周期与单次运行绑定。
- **未来（P2-8，backlog）**：引入 `CandidateRepository`——TopN 落盘 → 下次运行加载为先验。届时 `ArmTree` 初始化时可注入历史先验（A.3 方案 C 已定延后）。
- 本版不新增持久化接口。

### 3.6 应用服务（Application Service）

| 应用服务 | 职责 | 组合/编排 | PRD 触点 |
|---|---|---|---|
| `Engine.Run`（`engine/engine.go`） | 一次抽样搜索的事务边界：加载前缀 → 重建运行态 → 建树/建头/建 TopN → 事件循环调度（提交→探测→奖励→分裂→收集）→ 收尾 | `ArmTree` + `HeadManager` + `TopNCollector` + 并发 worker | **P1-6**：每次运行重置 `seenIPs/submitted/completed` 并重建运行组件，仅保证顺序复用，不新增并发 `Run` 能力；**P0-1/P0-2**：先更新 bandit，再以 `OK + colo` 资格门禁决定是否进入 TopN |
| `RunMCISSearchContext`（`source_mcis.go`） | 输入源侧装配：构建 engine/probe 配置 → 执行 → TopN 去重转候选列表 → 附带 warning | `BuildMCISEngineConfig` + `BuildMCISProbeConfig` + `Engine.Run` | 对外契约 `RunMCISSearch` 签名不变（PRD §2.3） |

### 3.7 工厂（Factory）

| 工厂 | 产出 | 说明 |
|---|---|---|
| `NewProber` | `Prober` | 配置默认值、dial/HTTP client profile 组装 |
| `NewArmTree` | `ArmTree` | 前缀去重、初始化节点 |
| `NewHeadManager` | `HeadManager` | 按 `BaseSeed + i*9973` 播种各头 |
| `NewTopNCollector` | `TopNCollector` | 容量初始化 |
| `BuildMCISEngineConfig` | `engine.Config` | 由 `limit`+`Routines` 派生搜索性能参数（独立层）；**P0-2 已实现**：复用 `task.ParseColoAllowList` / `NormalizeColoFilterMode` 将 `HttpingCFColo` 映射到互斥的 `ColoAllow` 或 `ColoBlock` |
| `BuildMCISProbeConfig` | `probe.Config` | 共享层继承：证书校验（**P0-1**）、目标域名 `TraceURL` 优先（**P0-1**）、超时/UA；不承载 COLO 过滤配置 |

---

## 4. PRD 变更 → DDD 模型变更映射

| PRD 项 | DDD 构件 | 变更类型 | 领域约束变化 |
|---|---|---|---|
| **P0-0** 探测 HEAD→GET | `Prober`（探测领域服务） | 协议行为 | OK/延迟/colo 信号真实化；bandit 奖励不再全灭，退化为随机探索的根因消除 |
| **P0-1** 证书校验 | `BuildMCISProbeConfig`（工厂）+ `InsecureSkipVerify`（探测配置值对象）+ `processOneResult`（候选资格门禁） | 共享内核接线 + 结果资格收紧 | 抽样探测按目标域名做 TLS 校验；失败仍更新 bandit，但不得进入 TopN；目标域名来源统一为 `TraceURL` 优先 |
| **P0-2** colo 过滤 | `ProbeResult.Trace`（值对象）+ `ColoFilter`（领域服务）+ `BuildMCISEngineConfig`（工厂） | 值对象扩展 + 领域服务接线 | 不变式：配置 Allow/Block 后，TopN 只含/排除指定 colo；**F2**：两侧大小写归一化，与主流程一致 |
| **P0-3** exploit 排序 | `ExploitSelector`（领域服务） | 确定性 | 不变式：exploit 候选按 `best ScoreMS` 升序，分数相同时按 prefix 排序；tier1 3x/tier2 1x；**F3**：仅承诺列表级可复现 |
| **P1-5** 分裂优化 | `ArmTree`（聚合不变式）+ `SplitStrategy`（领域服务） | 聚合规则 | `MinSamplesSplit` 5→8；`MaxTreeNodes=4096`；`MinSplitInformationGain=0.10`；根节点去重后已超上限则明确报错，分裂不得部分创建子节点 |
| **P1-6** 引擎复用 | `Engine.Run`（应用服务） | 生命周期 | 不变式：同 Engine 二次 `Run` 与新建一致；实施前 `seenIPs/submitted/completed` 未重置，导致二次运行跳过主调度并破坏预算/统计语义；现已在有效运行前重置 |
| **P2-7** 单元测试 | 各领域服务/聚合/应用服务 | 领域验证 | 与阶段 1-4 同批补齐：GET/trace、TLS 与目标域名、候选资格（F9）、colo 归一化（F2）、exploit 列表确定性（F3）、分裂不变式、二次 Run 复用（F4） |

---

## 5. 与现有分层架构的落位（architecture-constraints.md）

| DDD 构件 | 代码落位 | 现有分层 |
|---|---|---|
| 抽样上下文（聚合/领域服务/应用服务） | `internal/mcis/bandit`、`internal/mcis/engine`、`internal/mcis/probe`、`internal/mcis/cidr` | 领域核心（`internal/*core`） |
| 配置上下文（共享内核） | `internal/probecore`、`internal/httpcfg`、`internal/httpclient` | 领域核心 |
| 输入源上下文（装配/接入） | `internal/appcore/source_mcis.go` | 跨端应用核心（`internal/appcore`） |
| 测速上下文（下游终验） | `internal/task` | 底层探测与工具 |
| 编排上下文 | `internal/app`、`mobileapi`、`frontend` | 应用与平台适配 / 前端 |

**依赖方向合规**：抽样上下文只依赖 `internal/*core` 级能力；`internal/appcore`/`internal/task` 不导入 `internal/app`、`mobileapi`（受 `dependency_boundary_test.go` 约束）。PRD 改动（P0-0…P1-6）全部落在领域核心与 appcore 装配层，不触碰编排/平台层。

---

## 6. 领域级风险与设计备注（对齐评审并补充实施核查）

| 主题 | 领域影响 | 结论 |
|---|---|---|
| **F1 默认 URL 证书**（已撤销） | `VerifyTLSCertificate` 默认 true，严格校验下若目标域名证书不可用则抽样为空 | 按用户确认证书可用，PRD 移除运营依赖表述；保留预期行为说明（自定义不可用 URL 时为空） |
| **F2 colo 归一化** | `ColoFilter` 精确匹配 vs trace body 大写 colo → 大小写敏感静默失效 | 已并入 PRD v0.6，映射与匹配两侧统一归一化 |
| **F3 可复现范围** | `ThompsonSampler` RNG 消费顺序受 `LeafNodes()` map 迭代及并发完成顺序影响；此外，`Run` 虽计算了 `Seed=0` 的时间种子，但当前未把该局部变量传给 `HeadManager` | 本次仅承诺 exploit 候选列表级确定性，并以 `ScoreMS + Prefix` 建立全序；不顺带修复 seed 语义或全引擎可复现性，复用测试显式使用非零 seed 和可控本地探测 |
| **F4 二次 Run 状态污染** | 实施前计数与去重状态未重置，二次运行仍提交初始批次，但主事件循环因旧 `completed` 跳过，预算、统计和结果处理语义均不正确 | 已实现运行前重置，并以本地 TLS、固定 `/32` 和非零 seed 对照新 Engine 验证 |
| **F9 失败结果进入 TopN** | 实施前 `processOneResult` 对失败探测仅施加 `2×timeout` 罚分，TopN 未满时仍会收录，导致 P0-1 的“候选均通过 TLS”验收不成立 | 已随 P0-1 增加候选资格门禁：失败结果保留 bandit 负反馈，但不进入 TopN |
| **F5/F8 工作量标注** | 配置装配层（工厂）改动面比"1 行"略大 | 已并入 PRD v0.7（A.4 修正） |
| **F6 仓库清理** | `.tmp-probe/`、`*Zone.Identifier` 未跟踪残留 | 已并入 PRD v0.7（§6 清理项） |
| **F7 SNI 兜底硬编码** | `BuildMCISProbeConfig` 兜底 `cf.xiu2.xyz` 为第三方域名 | **用户决定不处理**，维持现状 |

---

## 7. 实施计划与里程碑

### 7.1 交付策略

PRD 决定 M1+M2 一次合入；实施时仍按依赖拆成 4 个可验证步骤，每一步同时提交对应单元测试。P2-7 不延后到代码合入之后，M3 仅用于扩大覆盖、真实数据回测和可选参数调优。

```text
阶段 0 基线
   │
   ▼
阶段 1 Probe：GET + 有界读取 + trace 解析（P0-0）
   │
   ▼
阶段 2 装配与资格：TLS/TraceURL + COLO + OK 门禁（P0-1/P0-2/F9）
   │
   ▼
阶段 3 搜索策略：exploit 全序 + 分裂约束（P0-3/P1-5）
   │
   ▼
阶段 4 生命周期：Engine 顺序复用（P1-6）
   │
   ▼
阶段 5 全量验证 + 真实抽样 smoke
```

实施边界：不改 `RunMCISSearch` 签名、配置 schema、GUI 字段、主流程 `internal/task` 的测速逻辑和 bandit 奖励公式；不引入依赖；不承诺并发调用同一 `Engine.Run`，也不顺带实现全引擎可复现。

### 7.2 开工前冻结的实现决策

| 决策点 | 实施口径 | 原因/约束 |
|---|---|---|
| trace body | GET 后最多读取前 64 KiB 用于 `parseTrace`，其余响应体丢弃并关闭；`OK` 仍只由 2xx 决定 | 防止非预期大响应造成无界内存，同时保持连接复用与 PRD 的 2xx 语义 |
| 候选资格 | 所有结果先更新 `ArmTree`；仅 `result.OK && passColoFilter(colo)` 时进入 TopN | TLS/HTTP 失败必须成为 bandit 负反馈，但不能成为下游候选（F9） |
| 目标域名 | `TraceURL` → `URL` → 默认配置；显式 `SNI` 优先，其次 `HostHeader`，再其次目标域名 | 与主流程 trace 口径一致，保留既有兜底域名决策 |
| COLO 映射 | 在 `appcore` 复用 `task.ParseColoAllowList` 与 `task.NormalizeColoFilterMode`；allow/deny 只设置一侧；engine 对配置值与 trace 值再次做 trim + 大小写归一化 | 复用主流程的分隔符、模式和代码归一化口径，并在领域边界防御直接调用 |
| 空 COLO | allow 模式拒绝，deny 模式放行；未配置过滤时全部放行 | 与 `task.Engine.configuredColoAllowed` 保持一致 |
| exploit 全序 | 先按每个 prefix 的最佳 `ScoreMS` 升序，分数相同时按 `Prefix.Addr()`、`Prefix.Bits()` 排序，再展开 3x/1x 权重 | 仅按分数排序无法消除同分时的 map 顺序 |
| 分裂参数 | `MinSamplesSplit=8`、`MaxTreeNodes=4096`、`MinSplitInformationGain=0.10`，均为内部 engine/tree 配置，不新增用户配置 | 给 PRD 的“信息量门槛”确定可编码默认值；后续只通过 M3 回测调整 |
| 节点上限 | 根前缀去重后若已超过上限，`Run` 返回明确错误；一次分裂若容不下全部子节点则不分裂，不允许部分创建 | 保证 `Size() <= MaxTreeNodes` 是真实不变式 |
| Engine 复用 | 每次有效运行前重置 `seenIPs`、`submitted`、`completed`，并重建 tree/head/topN/channel；只支持前一次 Run 完成后的顺序复用 | 满足 P1-6，避免为非目标的并发复用引入锁与生命周期复杂度 |
| Seed 范围 | 复用测试使用非零固定 seed；本次不修复 `Seed=0` 的局部时间种子未传入 `HeadManager` 问题 | 该问题不阻塞 P0-3 的列表级确定性，避免扩大行为变更 |

### 7.3 分阶段改动

#### 阶段 0：建立基线

- 运行 `go test ./internal/mcis/... ./internal/appcore/...`，记录现有通过/失败状态。
- 确认工作区中 `.tmp-probe/`、`*Zone.Identifier` 等未跟踪文件不进入本次 diff；不擅自删除用户文件。
- 为后续真实 smoke 固定输入 CIDR、`limit`、`Routines`、TLS 开关和 COLO 条件，避免用不同输入比较质量。

**退出条件**：基线结果已记录；本任务的文档变更仅涉及本 DDD 文件，仓库既有未提交内容保持不动。

#### 阶段 1：修复探测协议（P0-0）

| 文件 | 改动 |
|---|---|
| `internal/mcis/probe/trace.go` | `HEAD` 改 `GET`；有界读取 body；调用 `parseTrace` 写入 `Result.Trace`；保持 2xx、延迟分段、多轮平均和首轮跳过语义 |
| `internal/mcis/probe/trace_test.go`（新增） | 本地 TLS fixture 验证 GET 方法、trace 解析、非 2xx、body 上限、多轮结果继承最后一次成功 trace |

测试不得依赖公网；使用本地 TLS server 与 `DialAddress`，覆盖 `colo=SJC`、CRLF、空行、无效行等输入。

**退出条件**：单次与多轮探测均能得到 `OK=true`、状态码和 `Trace["colo"]`；失败响应不伪装为成功。

#### 阶段 2：接通共享配置与候选资格（P0-1/P0-2/F9）

| 文件 | 改动 |
|---|---|
| `internal/appcore/source_mcis.go` | `InsecureSkipVerify = !cfg.VerifyTLSCertificate`；目标域名改为 `TraceURL` 优先；把 `HttpingCFColo` + mode 映射到互斥的 `ColoAllow/ColoBlock` |
| `internal/appcore/source_mcis_test.go`（新增） | 覆盖 TLS 开关、TraceURL/URL/SNI/Host 优先级、allow/deny、多分隔符和混合大小写 |
| `internal/mcis/engine/engine.go` | COLO 两侧归一化；`processOneResult` 在更新树后增加 `OK + colo` TopN 资格门禁 |
| `internal/mcis/engine/engine_test.go`（新增） | 覆盖 allow/deny/空 COLO 语义，以及失败结果更新 arm 但不进入 TopN |

**退出条件**：严格 TLS 下的握手/HTTP 失败不会进入 TopN；关闭校验保留旧行为；COLO allow/deny 与主流程对同一输入给出一致结论。

#### 阶段 3：收紧搜索与分裂不变式（P0-3/P1-5）

| 文件 | 改动 |
|---|---|
| `internal/mcis/engine/config.go` | 默认 `MinSamplesSplit` 改 8；新增并校验 `MaxTreeNodes`、`MinSplitInformationGain`；完整接入 `ApplyDefaults`/`ToTreeConfig` |
| `internal/mcis/engine/engine.go` | exploit 候选建立确定性全序；建树前检查根节点数；`trySplit` 只处理达到信息量门槛的候选 |
| `internal/mcis/bandit/tree.go` | `TreeConfig` 承载节点上限与信息量门槛；`GetSplitCandidates` 过滤低信息量节点；`SplitNode` 在写锁内执行容量检查并原子创建全部子节点 |
| `internal/mcis/bandit/tree_test.go`（新增） | 覆盖 7/8 样本边界、信息量门槛、IPv4/IPv6 子节点容量、上限内成功与临界值不分裂 |
| `internal/mcis/engine/engine_test.go` | 覆盖 exploit 的 3x/1x 权重、分数排序、同分 prefix tie-break |

`internal/mcis/bandit/arm.go` 的 `InformationGain()` 已可复用，除非测试证明其契约需要修正，否则不改该文件。

**退出条件**：相同 TopN 快照生成完全相同的 exploit 列表；任何输入与分裂路径都不突破 4096 节点；低于 0.10 信息量的节点不分裂。

#### 阶段 4：修复 Engine 顺序复用（P1-6）

| 文件 | 改动 |
|---|---|
| `internal/mcis/engine/engine.go` | 在通过配置和输入校验后、启动 worker 前重置原子计数与 `sync.Map`，随后重建本轮组件和 channel |
| `internal/mcis/engine/engine_test.go` | 以本地 TLS fixture、单 `/32`、`Concurrency=1`、小预算和非零固定 seed 连续运行同一 Engine，再与新 Engine 对照 |

测试断言第二次运行非空、提交/完成数重新从 0 累计、旧 `seenIPs` 不影响本轮；不以公网时延或并发完成顺序作为断言依据。

**退出条件**：同一 Engine 顺序运行两次均完成预算并产出合格候选；`go test -race ./internal/mcis/...` 不出现本次新增竞态。

### 7.4 测试矩阵

| 需求/风险 | 自动化证据 | 关键断言 |
|---|---|---|
| P0-0 GET | `probe/trace_test.go` | 请求方法为 GET；2xx 为 OK；body 写入 Trace；非 2xx 为失败 |
| P0-1 TLS | `source_mcis_test.go` + 本地 TLS fixture | 配置映射正确；严格模式拒绝不可信证书；失败不进 TopN |
| P0-2 COLO | `source_mcis_test.go` + `engine_test.go` | `sjc,SJC` 等价；allow/deny/空值语义与主流程一致 |
| P0-3 exploit | `engine_test.go` | tier1 重复 3 次、tier2 1 次；同分时仍有稳定顺序 |
| P1-5 分裂 | `tree_test.go` + `engine/config` 测试 | 最少 8 样本；信息量 ≥ 0.10；节点数不越界；无部分分裂 |
| P1-6 复用 | `engine_test.go` | 二次 Run 不为空；计数与去重状态不串扰；新/旧 Engine 在可控 fixture 下等价 |
| 边界兼容 | `appcore` 与依赖边界测试 | 对外签名、配置 schema、平台依赖方向不变 |

### 7.5 验证与验收

先运行窄范围验证，再运行仓库级过滤测试；命令均从仓库根目录在 PowerShell 执行：

```powershell
go test ./internal/mcis/... ./internal/appcore/...
go test -race ./internal/mcis/...

$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' })
go test $goPackages

rg -n "\[[^]]+\]\([^)]+\)" docs/prd-mics-engine-upgrade-ddd.md docs/prd-mics-engine-upgrade.md
```

真实抽样 smoke 不进入默认单测，需在网络可用时使用阶段 0 固定输入执行，记录：

1. GET trace 出现 2xx，成功率不再全零，返回候选全部 `OK=true`。
2. `verify_tls_certificate=true/false` 行为符合预期，严格模式下无证书失败候选。
3. allow 使用小写、多值配置时 TopN 只含指定 COLO；deny 不含被屏蔽 COLO。
4. 相同预算下树节点不超过 4096；最终交给 CFST 后的有效候选率不低于基线。

若环境无法完成公网 smoke，自动化测试仍须全部通过，并在交付说明中明确未验证的网络与候选质量风险。

### 7.6 合入、回退与完成定义

- **合入方式**：M1+M2 保持一次交付，但提交历史按“probe → 配置/资格 → 搜索策略 → 生命周期”组织，便于定位回归。
- **回退优先级**：若真实候选质量只在 P1-5 后下降，优先单独回退分裂门槛/上限参数；P0-0/P0-1/P0-2 正确性修复不得因参数回归一并撤销。
- **兼容性**：无配置迁移、无 API/bridge/schema 变化；旧配置继续由 `probecore` 归一化。
- **完成定义**：阶段 1-4 的代码与测试全部进入同一合入批次；窄范围、race、仓库级测试通过；真实 smoke 已通过或明确记录无法执行的原因与风险；最终 diff 不含 `.tmp-probe/`、`*Zone.Identifier` 或其他无关文件。

### 7.7 里程碑映射

| 里程碑 | 范围 | 交付物 |
|---|---|---|
| **M1+M2（一次合入）** | P0-0/P0-1/P0-2/P0-3/P1-5/P1-6/P2-7 对应回归 | 阶段 1-4 全部代码、单元测试、验证记录 |
| **M3（后续）** | 扩展覆盖 + P2-9 可选参数回测 | 多输入真实数据集、候选质量对比、必要时调整 0.10/4096 等内部默认值 |
| **Backlog** | P1-4（带宽感知） | 仅在端点与 Range/降级路径明确后采用 TopN 外层 gate，不改 bandit 奖励 |
| **Backlog** | P2-8（历史先验） | `CandidateRepository` 落盘/加载与 `ArmTree` 先验注入，另立设计 |

### 7.8 实施结果

- **阶段 1**：Probe 已改为 GET，响应体解析窗口限制为 64 KiB，trace 字段进入 `Result.Trace`；本地 TLS 测试覆盖 2xx、非 2xx、证书校验和多轮继承。
- **阶段 2**：`VerifyTLSCertificate`、`TraceURL`、COLO allow/deny 已接入；失败探测只更新 bandit，不进入 TopN。
- **阶段 3**：exploit 使用 `ScoreMS + Prefix` 全序；分裂默认参数为 8/4096/0.10，IPv4/IPv6 容量检查均为原子操作。
- **阶段 4**：`Engine.Run` 在有效运行前重置计数和 `seenIPs`，顺序复用已与新 Engine 对照验证。
- **自动化验证**：窄范围测试、`go test -race ./internal/mcis/...`、相关 `go vet`、仓库级过滤测试以及 10 次重复 MICS 测试均通过。
- **未执行**：公网真实抽样 smoke；网络成功率、真实 COLO 分布和最终 CFST 候选质量仍需人工验收。

---

## 8. 结论

本 DDD 文档将 PRD v0.7 的抽样引擎需求显式化为：**1 个核心子域（抽样搜索）+ 2 个支撑子域（探测、输入源接入）+ 3 个通用子域（CIDR 处理、配置归一化、HTTP 基础设施）**，收敛于 **抽样上下文**（`internal/mcis`），通过**共享内核**（配置共享层）与 **Conformist**（预筛→终验）两条上下文关系与配置/测速上下文协作。

战术层共有 **3 个聚合**（`ArmTree`、`SearchHead`、`TopNCollector`）、**2 个实体** + 若干值对象、**5 个领域服务**（`ThompsonSampler`/`SplitStrategy`/`ExploitSelector`/`ColoFilter`/`Prober`）、**4 类领域事件**（当前以 channel 载体隐式存在）、**2 个应用服务**（`Engine.Run`、`RunMCISSearchContext`）、**6 个工厂**。

PRD 本版变更（P0-0…P1-6）已落到上述构件，未改变现有分层、对外签名或配置 schema，也未引入新依赖。自动化验收结果见 §7.8；需求范围与最终人工验收目标仍以 PRD v0.7 为准。
