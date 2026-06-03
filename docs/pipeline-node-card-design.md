# 工作流节点卡片功能设计清单

> 生成时间：2026-06-02T15:41:49+08:00  
> 范围：策略工作流画布中的节点卡片目录、节点配置、运行时行为、数据流与默认模板。

## 1. 设计对象

工作流节点卡片由后端节点目录驱动，前端画布统一渲染。后端目录定义 `action`、`node_type`、默认配置、表单字段和分支 outcome；前端根据目录生成新增节点菜单、节点卡片、配置表单、状态徽标和连线校验。

关键结构如下：

| 对象 | 作用 | 参考 |
| --- | --- | --- |
| `PipelineNodeCatalogItem` | 节点卡片目录项，定义动作、展示名、表单和 outcome | [internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:200) |
| `PipelineNode` | 画布节点实例，保存 action、config、id、name、node_type、ui | [internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:71) |
| `PipelineEdge` | 节点连线，分支节点通过 `outcome` 决定路径 | [internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:81) |
| `PipelineStudioNode` | 通用节点卡片组件，负责展示、编辑、折叠、状态和问题提示 | [frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:226) |
| `executePipelineNode` | 运行时动作分发入口 | [internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:752) |

## 2. 卡片通用能力

所有节点卡片共享同一套基础交互：

| 能力 | 设计说明 | 参考 |
| --- | --- | --- |
| 展示节点类型与动作名 | 卡片顶部展示节点类型标签、节点名、动作展示名 | [frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:240) |
| 运行状态提示 | 通过 `completed`、`running`、`manual_review`、`failed` 等状态映射徽标颜色 | [frontend/src/lib/pipelineStudio.ts](/home/axuitomo/code/CFST-GUI/frontend/src/lib/pipelineStudio.ts:91) |
| 起点设置 | 选中节点后可设为入口节点，测速阶段拆分卡片只允许第一个阶段设为起点 | [frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:272) |
| 折叠/展开 | 折叠时展示配置摘要，展开时根据目录字段渲染表单 | [frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:312) |
| 表单自动生成 | 支持 `text`、`textarea`、`select`、`checkbox`、`number`、`json` 字段类型 | [frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:347) |
| 问题提示 | 前端对未知动作、类型不匹配、缺少连线、循环、不可达等问题做卡片提示 | [frontend/src/lib/pipelineStudio.ts](/home/axuitomo/code/CFST-GUI/frontend/src/lib/pipelineStudio.ts:299) |
| 运行结果叠加 | 最近一次运行结果按 node id 映射回卡片，显示消息、状态和已走过边 | [frontend/src/lib/pipelineStudio.ts](/home/axuitomo/code/CFST-GUI/frontend/src/lib/pipelineStudio.ts:505) |

## 3. 节点卡片总览

当前节点目录共包含 12 张节点卡片：

| 卡片 | action | node_type | 主要输入 | 主要输出 | 运行时函数 |
| --- | --- | --- | --- | --- | --- |
| 输入源组 | `select_sources` | `source` | 当前绑定配置或 Source Profile、`source_selection`、`source_ids` | `SelectedSources`、节点输出中的输入源列表 | `executeSelectSourcesNode` |
| 输入源筛选 | `filter_sources` | `source` | 输入源组输出、筛选配置 | 更新后的输入源列表 | `executeFilterSourcesNode` |
| TCP 延迟测速 | `probe_tcp` | `probe` | 输入源筛选结果、测速配置 | TCP 候选集 | `executeProbeTCPNode` |
| 追踪测试 | `probe_trace` | `probe` | TCP 候选集、追踪配置 | 追踪候选集 | `executeProbeTraceNode` |
| 下载测速 | `probe_download` | `probe` | 追踪候选集、下载配置 | `ProbeResult`、`probe_results` | `executeProbeDownloadNode` |
| 结果筛选 | `filter_results` | `filter` | `probe_results` 或 `filtered_rows` | `UploadSelectionResult`、`filtered_rows` | `executeFilterResultsNode` |
| 结果检查 | `branch_has_results` | `branch` | `probe_results` 或 `filtered_rows` | outcome: `true` / `false` | `executeBranchHasResultsNode` |
| DNS 推送 | `deliver_dns` | `deliver` | 筛选结果或测速结果、Cloudflare 配置 | Cloudflare DNS 推送结果 | `executeDeliverDNSNode` |
| GitHub 导出 | `deliver_github` | `deliver` | 筛选结果或测速结果、GitHub 配置 | GitHub 导出结果 | `executeDeliverGitHubNode` |
| 人工复核标记 | `recovery_mark` | `recovery` | 人工说明、标记状态 | warning 与状态摘要 | `executeRecoveryMarkNode` |
| 结束 | `end` | `end` | 最终状态、结束说明 | 流程最终状态 | `executeEndNode` |
| 结果检查与输出 | `check_output` | `deliver` | `probe_results` 或 `filtered_rows`、CSV 写入要求 | CSV 检查/补写结果 | `executeCheckOutputNode` |

节点目录定义位置：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:345)。运行时分发表位置：[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:769)。

## 4. 各节点卡片功能设计

### 4.1 输入源组

输入源组用于从当前绑定配置或指定 Source Profile 中选择输入源，作为后续输入源筛选和测速阶段的候选输入集合。勾选/取消只影响当前工作流节点，不会修改 Source Profile 自身的 enabled 状态。

| 设计项 | 内容 |
| --- | --- |
| 默认配置 | `source_profile_id: ""`、`source_selection: "enabled"`、`source_ids: []`，表示使用当前绑定配置中的全部启用输入源 |
| 表单字段 | `source_profile_id`、`source_selection` 由目录保底；前端卡片和 Inspector 以输入组下拉、搜索、勾选列表呈现，并隐藏原始 `source_ids` JSON |
| 运行逻辑 | `source_profile_id` 为空时读取当前绑定配置；非空时读取对应 Source Profile 最新 sources。`source_selection=enabled` 使用 enabled sources；`custom` 只使用 `source_ids` 命中的输入源 |
| 输出 | `runtimeCtx.SelectedSources` 与当前节点输出，摘要为“X 个输入源” |
| 下游建议 | 通常连接到“输入源筛选”节点；简单流程也可直接连接到“TCP 延迟测速” |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:348)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:783)、[frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:328)。

### 4.2 测速阶段

测速现在拆成三个连续节点：`probe_tcp` 执行 TCP 延迟测速，`probe_trace` 复用 TCP 候选继续追踪测试，`probe_download` 对追踪候选执行下载测速并产出 `probe_result` 供后续节点消费。兼容层会把旧模板中的 `run_probe` 节点迁移为这组三阶段节点，节点目录不再暴露 `run_probe`。

| 配置组 | 字段 | 设计说明 |
| --- | --- | --- |
| 输入源 | `source_mode` | `inherit` 继承绑定配置或上游输入源组；`custom` 使用节点内自定义输入源 |
| 输入源 | `sources` | 自定义 `DesktopSourceConfig` 数组，仅 `source_mode=custom` 时显示 |
| 输入源 | `source_ip_limit` | 覆盖每个输入源的候选 IP 上限 |
| 输入源 | `source_ip_mode` | `traverse` 直接遍历；`mcis` 先做搜索 |
| 输入源 | `source_colo_filter` / `source_colo_filter_mode` | 对当前节点所有输入源统一附加 Colo allow/deny 过滤 |
| TCP 阶段 | `tcp_port`、`port_policy`、`concurrency_stage1`、`ping_times`、`max_tcp_latency_ms`、`min_delay_ms`、`max_loss_rate`、`timeout_stage1_ms` | 仅在 `probe_tcp` 配置 UI 展示 |
| 追踪阶段 | `trace_url`、`trace_colo_mode`、`source_colo_filter_phase`、`concurrency_stage2`、`timeout_stage2_ms`、`httping_status_code`、`max_trace_latency_ms`、`httping_cf_colo`、`httping_cf_colo_mode` | 仅在 `probe_trace` 配置 UI 展示 |
| 下载阶段 | `url`、`stage3_limit`、`download_count`、`print_num`、`concurrency_stage3`、`download_*`、`min_download_mbps` | 仅在 `probe_download` 配置 UI 展示 |
| 隐藏默认值 | `strategy: "full"`、`disable_download: false` 以及三阶段完整默认配置 | 保留在 default config 中，维持运行时兼容，但不在其他阶段 UI 暴露 |

运行时会先根据 TCP 节点配置生成有效配置快照并准备输入源，随后每个阶段按节点配置覆盖当前快照。下载阶段完成后写入 `runtimeCtx.ProbeResult`，同时清空旧的 `FilteredRows`，避免下游误用上一轮筛选结果。

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:368)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:808)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1174)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1227)。

### 4.3 结果筛选

结果筛选节点把测速结果转换为上传选择结果，产出可供投递节点复用的 `filtered_rows`、`cloudflare_rows` 和 `github_rows`。

| 配置项 | 设计说明 |
| --- | --- |
| `source` | 从 `probe_results` 或已有 `filtered_rows` 继续筛选 |
| `status` | `passed` 或 `all`；当前 `ProbeRow` 主要表示可导出的成功结果 |
| `ip_version` | `any`、`ipv4`、`ipv6` |
| `max_loss_rate` | 最大丢包率 |
| `max_tcp_latency_ms` | 最大 TCP 延迟 |
| `max_trace_latency_ms` | 最大追踪延迟 |
| `min_download_mbps` | 最小下载速度 |
| `colo_allow` / `colo_deny` | Colo 白名单/黑名单，支持逗号、空格、换行、分号分隔 |
| `top_n` | 大于 0 时限制排序后的前 N 条继续向下游传递 |

运行时通过 `pipelineEnsureUploadSelection` 读取数据源，生成上传筛选配置快照并调用 `BuildUploadSelection`。筛选结果缓存到 `runtimeCtx.NodeOutputs[node.ID]`，并同步到 `runtimeCtx.FilteredRows`。

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:537)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:840)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1107)、[internal/appcore/upload_selection.go](/home/axuitomo/code/CFST-GUI/internal/appcore/upload_selection.go:48)。

### 4.4 结果检查

结果检查是分支节点，用来判断当前结果集是否为空，并按 outcome 决定下一条边。

| 设计项 | 内容 |
| --- | --- |
| 默认输入 | `filtered_rows` |
| 可选输入 | `probe_results` |
| outcome | `true` 表示有结果；`false` 表示无结果 |
| 运行消息 | 有结果时继续投递；无结果时进入回退路径 |
| 连线要求 | 分支节点出边必须声明 outcome，且不能重复 |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:646)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:868)、[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:1427)、[frontend/src/lib/pipelineStudio.ts](/home/axuitomo/code/CFST-GUI/frontend/src/lib/pipelineStudio.ts:373)。

### 4.5 DNS 推送

DNS 推送节点把筛选后的 IP 推送到 Cloudflare DNS。它既受节点配置控制，也受目标 DNS 推送策略和调度器覆盖项约束。

| 配置组 | 字段 | 设计说明 |
| --- | --- | --- |
| 数据来源 | `source` | `filtered_rows` 或 `probe_results` |
| 推送行为 | `top_n` | 覆盖 Cloudflare 上传数量 |
| DNS 记录 | `record_name` | 留空时继承绑定配置 |
| DNS 记录 | `record_type` | `A` 仅 IPv4；`AAAA` 仅 IPv6 |
| DNS 记录 | `ttl` | 覆盖 TTL |
| DNS 记录 | `proxied` | 是否启用 Cloudflare 代理 |
| DNS 记录 | `comment` | 可选注释，留空则沿用绑定配置 |

运行时先检查调度器是否允许 DNS 推送，再检查目标 `DNSPushPolicy`。筛选后如果没有匹配记录类型的 IP，则节点以 `skipped` 完成，不删除或覆盖已有 DNS。

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:673)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:887)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1372)、[internal/appcore/upload_selection.go](/home/axuitomo/code/CFST-GUI/internal/appcore/upload_selection.go:79)。

### 4.6 GitHub 导出

GitHub 导出节点把当前上传选择结果导出到 GitHub。当前目录项没有额外表单字段，主要依赖绑定配置、上游筛选和全局 GitHub 导出配置。

| 设计项 | 内容 |
| --- | --- |
| 默认配置 | `{}` |
| 数据来源 | 通过 `pipelineEnsureUploadSelection` 默认选择可用的 `filtered_rows` 或 `probe_results` |
| 输出 | GitHub 导出命令结果，摘要为导出行数 |
| 空结果行为 | 筛选后没有可导出结果时返回 `skipped` |
| top_n 行为 | 运行时支持 `top_n` 覆盖 GitHub 上传数量，但目录表单暂未暴露 |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:746)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:945)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1301)。

### 4.7 人工复核标记

人工复核标记用于在回退路径中记录原因，并把说明追加到 workflow warnings。

| 配置项 | 设计说明 |
| --- | --- |
| `status` | `manual_review`、`skipped`、`failed` |
| `message` | 人工复核说明，默认“需要人工复核。” |
| 输出 | `{status}`，摘要为状态值 |
| 运行状态 | 节点自身返回 `completed`，最终状态通常交给下游结束节点或上游状态继承逻辑处理 |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:753)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1063)。

### 4.8 结束

结束节点声明当前路径的最终状态和说明。结束节点没有 source handle，不能继续连出下一步。

| 配置项 | 设计说明 |
| --- | --- |
| `status` | `completed`、`manual_review`、`skipped`、`failed`、`partial` |
| `message` | 展示给运行结果区的最终说明 |
| 输出 | `{status}`，摘要为状态值 |
| 连线限制 | 结束节点不能有出边 |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:787)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1075)、[frontend/src/components/pipeline/PipelineStudioNode.vue](/home/axuitomo/code/CFST-GUI/frontend/src/components/pipeline/PipelineStudioNode.vue:237)。

### 4.9 结果检查与输出

结果检查与输出用于检查测速结果和 CSV 写入状态，必要时补写 CSV。它是默认模板中的投递节点，不再承担结束节点职责。

| 配置项 | 设计说明 |
| --- | --- |
| `source` | `probe_results` 或 `filtered_rows` |
| `require_csv` | 是否要求 CSV 已写入 |
| `export_if_missing` | CSV 缺失且存在结果时，按绑定配置补写 |
| 无结果行为 | 返回 `manual_review`，提示需要人工复核 |
| CSV 缺失行为 | 若要求 CSV 且不允许补写，则返回 `manual_review` |
| 成功行为 | 返回 `completed`，输出 `output_file` 与 `result_count` |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:823)、[internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:986)、[internal/appcore/pipeline_test.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline_test.go:5)、[internal/app/pipeline_runtime_test.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline_runtime_test.go:198)。

## 5. 默认模板设计

默认模板 ID/name 保持 `pipeline-template-default` / “默认流程”，但链路采用高级上传回退流程：

```text
输入源组 -> 输入源筛选 -> TCP 延迟测速 -> 追踪测试 -> 下载测速 -> 结果筛选 -> 结果检查
  有结果 -> DNS 推送 -> GitHub 导出 -> 结束：完成
  无结果 -> 人工复核标记 -> 结束：人工复核
```

默认节点为：

| 节点 ID | 节点名 | action | node_type | 默认位置 |
| --- | --- | --- | --- | --- |
| `advanced-source-group` | 输入源组 | `select_sources` | `source` | x=60, y=160 |
| `advanced-source-filter` | 输入源筛选 | `filter_sources` | `source` | x=420, y=160 |
| `advanced-probe-tcp` | TCP 延迟测速 | `probe_tcp` | `probe` | x=780, y=160 |
| `advanced-probe-trace` | 追踪测试 | `probe_trace` | `probe` | x=1140, y=160 |
| `advanced-probe-download` | 下载测速 | `probe_download` | `probe` | x=1500, y=160 |
| `advanced-filter` | 结果筛选 | `filter_results` | `filter` | x=1860, y=160 |
| `advanced-branch-results` | 结果检查 | `branch_has_results` | `branch` | x=2220, y=160 |
| `advanced-deliver-dns` | DNS 推送 | `deliver_dns` | `deliver` | x=2580, y=60 |
| `advanced-deliver-github` | GitHub 导出 | `deliver_github` | `deliver` | x=2940, y=60 |
| `advanced-end-completed` | 结束：完成 | `end` | `end` | x=3300, y=60 |
| `advanced-recovery-empty` | 人工复核标记 | `recovery_mark` | `recovery` | x=2580, y=280 |
| `advanced-end-manual-review` | 结束：人工复核 | `end` | `end` | x=2940, y=280 |

参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:873)。

## 6. 数据流连续性

当前工作流中有三类核心数据在节点间传递：

| 数据 | 写入节点 | 读取节点 | 说明 |
| --- | --- | --- | --- |
| `SelectedSources` | 输入源组 | 测速 | 当测速节点 `source_mode=inherit` 时优先使用 |
| `ProbeResult.Results` | 测速 | 结果筛选、结果检查、DNS 推送、GitHub 导出、结果检查与输出 | 原始测速结果集 |
| `FilteredRows` / `UploadSelectionResult` | 结果筛选、投递前选择器 | 结果检查、DNS 推送、GitHub 导出、结果检查与输出 | 筛选后的投递结果集 |

数据源读取规则位于 [internal/app/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/app/pipeline.go:1137)。当 `source` 未指定时，运行时会优先使用 `FilteredRows`，再使用最近一次上传选择结果，最后回退到 `ProbeResult.Results`。

## 7. 校验与兼容设计

前端和后端都包含流程校验：

| 校验项 | 前端提示 | 后端保存校验 |
| --- | --- | --- |
| 节点 ID 为空或重复 | 有 | 有 |
| action 未识别 | 有 | 有 |
| action 与 node_type 不匹配 | 有 | 有 |
| 缺少 entry node | 有 | 有 |
| 缺少 end node | 有 | 有 |
| end 节点存在出边 | 有 | 有 |
| 非分支节点多出边 | 有 | 有 |
| 分支缺 outcome 或 outcome 重复 | 有 | 有 |
| 不可达节点 | warning | error |
| 循环 | error | error |

前端校验参考：[frontend/src/lib/pipelineStudio.ts](/home/axuitomo/code/CFST-GUI/frontend/src/lib/pipelineStudio.ts:299)。后端校验参考：[internal/appcore/pipeline.go](/home/axuitomo/code/CFST-GUI/internal/appcore/pipeline.go:1352)。

## 8. 主要设计结论

当前节点卡片体系已经形成“目录驱动 UI + DAG 校验 + 运行时动作分发 + 节点输出缓存”的基本闭环。默认模板已从旧的“检查即结束”升级为“检查输出后继续到结束”，这让工作流能表达更完整的最终状态。

后续完善重点不在新增大量节点，而在保证前后端目录一致、显式暴露运行时已有但 UI 未开放的配置项、强化分支和投递路径的连续性提示。具体修复建议见 [工作流节点卡片功能连续性完善修复建议](/home/axuitomo/code/CFST-GUI/docs/pipeline-node-continuity-repair-suggestions.md)。
