# 工作流节点卡片功能连续性修复状态

> 更新时间：2026-06-02T16:35:00+08:00  
> 依据文档：[工作流节点卡片功能设计清单](./pipeline-node-card-design.md)  
> 目标：记录节点卡片连续性修复的当前状态、验收口径和防偏移护栏，避免修复建议与代码实现再次脱节。

## 1. 结论

节点卡片连续性缺口已完成一轮闭环修复。默认流程保持轻量闭环：输入源组 -> 测速 -> 结果检查与输出 -> 结束。高级上传回退流程已作为内置模板补齐：输入源组 -> 测速 -> 结果筛选 -> 结果检查，然后按有结果/无结果分别进入投递或人工复核路径。

当前防偏移策略是“文档记录状态 + 自动化校验目录”。前后端节点目录一致性由 `scripts/check-pipeline-catalog.mjs` 校验，并已接入 `scripts/lint.sh`。

## 2. 修复状态总览

| 优先级 | 项目 | 当前状态 | 关键验收 |
| --- | --- | --- | --- |
| P0 | 修复 `deliver_dns.node_type` fallback 错误 | 已完成 | 前端 fallback 中 `deliver_dns` 是 `deliver` |
| P1 | GitHub 导出卡片补齐 `source` 与 `top_n` | 已完成 | UI 可配置来源和导出前 N 条 |
| P1 | 明确 `top_n` 作用层级 | 已完成 | 筛选节点影响下游，投递节点只影响自身 |
| P1 | 增加前后端目录一致性检查 | 已完成 | `node scripts/check-pipeline-catalog.mjs` 通过 |
| P2 | 强化空结果路径推荐模板 | 已完成 | 内置高级上传回退流程包含 `false -> manual_review` 路径 |
| P2 | 稳定多筛选节点的数据来源 | 已完成 | 运行时使用 `LastUploadSelection`，不再遍历 map |
| P3 | 补充节点输入/输出摘要 | 已完成 | 折叠摘要优先展示数据流、`source` 和 `top_n` |

## 3. 关键实现

### 3.1 节点目录连续性

后端目录仍由 `DefaultPipelineNodeCatalog` 提供，前端保留 `defaultPipelineNodeCatalog` 作为桥接失败时的 fallback。两份目录必须保持 action、node_type 和表单字段 key 一致。

已完成项：

1. `deliver_dns` 前端 fallback 类型修正为 `deliver`。
2. `deliver_github` 后端和前端目录都暴露 `source`、`top_n`。
3. `top_n` 文案区分筛选层和投递层。
4. 目录一致性脚本覆盖 9 个 action，并接入 lint。

参考：[internal/appcore/pipeline.go](../internal/appcore/pipeline.go:345)、[frontend/src/lib/bridge.ts](../frontend/src/lib/bridge.ts:1380)、[scripts/check-pipeline-catalog.mjs](../scripts/check-pipeline-catalog.mjs:137)、[scripts/lint.sh](../scripts/lint.sh:11)。

### 3.2 高级上传回退流程

新增内置模板 `pipeline-template-advanced-upload`，用于承载空结果回退路径：

```text
输入源组 -> 测速 -> 结果筛选 -> 结果检查
  有结果 -> DNS 推送 -> GitHub 导出 -> 结束(completed)
  无结果 -> 人工复核标记 -> 结束(manual_review)
```

该模板会在工作区初始化和归一化时自动补齐，不会替换用户自定义模板，也不会抢占当前激活模板。

参考：[internal/appcore/pipeline.go](../internal/appcore/pipeline.go:17)、[internal/appcore/pipeline.go](../internal/appcore/pipeline.go:977)、[internal/appcore/pipeline.go](../internal/appcore/pipeline.go:1156)。

### 3.3 数据来源确定性

运行时通过 `pipelineRuntimeContext.LastUploadSelection` 记录最近一次上传筛选结果。`pipelineRowsForNodeSource` 会优先使用显式 `source`，在需要最近筛选结果时读取该字段，不再遍历 `NodeOutputs` map。

参考：[internal/app/pipeline.go](../internal/app/pipeline.go:21)、[internal/app/pipeline.go](../internal/app/pipeline.go:1108)、[internal/app/pipeline.go](../internal/app/pipeline.go:1167)。

### 3.4 前端连续性提示

前端校验会对分支 outcome 覆盖不完整给出 warning，不阻止保存。折叠摘要增加轻量数据流说明，并优先展示 `source`、`top_n`，降低用户不展开卡片时的理解成本。

参考：[frontend/src/lib/pipelineStudio.ts](../frontend/src/lib/pipelineStudio.ts:135)、[frontend/src/lib/pipelineStudio.ts](../frontend/src/lib/pipelineStudio.ts:427)。

## 4. 防偏移护栏

每次修改 pipeline 节点目录、节点类型、字段 key 或内置模板时，必须同步跑以下检查：

| 检查 | 命令 | 目的 |
| --- | --- | --- |
| 节点目录一致性 | `node scripts/check-pipeline-catalog.mjs` | 防止前后端 fallback 漂移 |
| 后端模板/校验 | `go test ./internal/appcore` | 确认默认模板和高级模板仍可保存 |
| pipeline 运行时 | `go test ./internal/app` | 确认分支、投递、回退路径行为稳定 |
| 前端 lint | `npm --prefix frontend run lint` | 确认摘要、校验和表单类型无静态问题 |
| 全量本地 lint | `bash scripts/lint.sh` | 汇总 go vet、目录一致性、shellcheck、ESLint |

如果新增节点或字段，先改后端目录，再改前端 fallback，最后运行目录一致性脚本。不要只更新其中一侧。

## 5. 验收判断

当前节点卡片连续性可以按以下条件验收：

1. 前后端目录中的 9 个 action、node_type 和主要字段保持一致。
2. 每个运行时支持的关键配置都能在卡片 UI 中看到，或有明确默认说明。
3. 默认模板和高级上传回退模板都能通过 `ValidatePipelineTemplate`。
4. 空结果、DNS 跳过、GitHub 空导出、CSV 缺失等非 happy path 都有明确状态。
5. 多筛选节点场景下，下游读取的数据来源稳定且可解释。

## 6. 后续维护建议

短期继续保留 `scripts/check-pipeline-catalog.mjs` 作为轻量护栏。中期如果节点目录继续扩张，可以考虑从后端目录导出 JSON，再生成前端 fallback，进一步减少双写维护成本。
