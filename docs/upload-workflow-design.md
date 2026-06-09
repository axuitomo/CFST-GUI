# Cloudflare / GitHub 上传工作流

本文记录 CFST-GUI 当前的结果上传实现状态。上传工作流已经从早期设计稿进入实现阶段；当前公开配置以顶层 `cloudflare`、`github` 和 `post_probe_push` 为主，旧 `upload.cloudflare.*`、`upload.github.*` 和旧 `export.github` 仅作为兼容读取路径保留。

## 1. 当前目标

上传工作流围绕测速结果提供三类能力：

1. 按共享筛选规则挑选要上传的结果，而不是无条件上传全部结果。
2. Cloudflare 和 GitHub 分别使用独立 Top N，避免被结果展示数量 `print_num` 绑定。
3. 支持工作流节点、定时任务、手动结果导出和测速后自动推送等入口复用同一套上传选择口径。

DNS 页面不参与推送。它只读取 Cloudflare 线上记录；真正修改 DNS 的路径是工作流 `deliver_dns` 节点、定时任务 DNS 推送和测速后自动推送中的 Cloudflare 项。

## 2. 配置口径

当前配置结构中，上传相关字段分布在三个顶层区域：

| 区域 | 作用 |
| --- | --- |
| `cloudflare` | Cloudflare API Token、Zone、默认记录名、DNS Top N 和分流规则。 |
| `github` | GitHub 仓库、分支、路径模板、提交模板、Token 和 GitHub Top N。 |
| `upload.shared_filter` | Cloudflare 与 GitHub 共用的状态、IP 版本、COLO、延迟、下载速度和丢包率筛选。 |
| `post_probe_push` | 手动测速完成后的 Cloudflare / GitHub 自动推送勾选项。 |

兼容规则：

- 旧 `export.github` 会同步到顶层 `github`，保存时仍写回兼容镜像。
- 旧 `upload.cloudflare.top_n` 和 `upload.github.top_n` 会分别同步到顶层 `cloudflare.top_n` 和 `github.top_n`。
- 旧 `upload.cloudflare.routing_*` 会同步到顶层 `cloudflare.routing_*`。
- 新文档和 UI 应优先描述顶层字段；旧字段只在兼容说明中出现。

更完整的字段说明见 [配置详解](./configuration.md)。

## 3. 上传选择器

共享筛选和目标拆分由 `BuildUploadSelection` 负责。它接收配置快照、测速结果和下载测速指标，产出三组结果：

| 输出 | 说明 |
| --- | --- |
| `SharedFilteredRows` | 应用共享筛选后的结果。 |
| `CloudflareRows` | 供 Cloudflare DNS 推送使用，受 `cloudflare.top_n` 和 Cloudflare 分流规则影响。 |
| `GitHubRows` | 供 GitHub 导出使用，受 `github.top_n` 影响。 |

核心实现：

- 上传选择器：[internal/appcore/upload_selection.go](../internal/appcore/upload_selection.go)
- 配置兼容净化：[internal/probecore/config_snapshot.go](../internal/probecore/config_snapshot.go)
- 工作流投递入口：[internal/app/pipeline.go](../internal/app/pipeline.go)
- 调度器自动动作：[internal/app/scheduler.go](../internal/app/scheduler.go)

共享筛选当前覆盖：

- 上传状态：全部或仅通过
- IP 版本：任意、IPv4、IPv6
- COLO allow / deny
- 最大 TCP 延迟
- 最大追踪延迟
- 最低下载速度
- 最大丢包率

## 4. Cloudflare 路径

Cloudflare 上传使用顶层 `cloudflare` 配置。默认推送会读取 `record_name`、`record_type`、`ttl`、`proxied`、`comment` 和 `top_n`；启用分流规则后，每条规则可以按国家或 COLO 筛选并推送到独立记录名。

执行路径：

| 入口 | 行为 |
| --- | --- |
| 工作流 `deliver_dns` 节点 | 使用节点数据源和配置覆盖项执行 DNS 推送。 |
| 定时任务 DNS 推送 | 调度完成后按配置自动执行，显式禁用时跳过；Android 后台定时任务不执行工作流，但单次测速完成后仍会执行该入口。 |
| 测速后自动推送 | 手动测速完成后按 `post_probe_push.cloudflare_enabled` 执行；定时任务会禁用这条入口以避免重复。 |

安全边界：

- DNS 读取页只调用列表 API，不创建、更新或删除记录。
- 筛选后没有可推送结果时跳过，不清空线上记录。
- A 记录只使用 IPv4，AAAA 记录只使用 IPv6，ALL 会按地址族分别处理。
- 只读取 DNS 时 Cloudflare Token 可授予 DNS Read；需要推送时必须授予 DNS Edit。

Cloudflare 权限说明见 [Cloudflare API Token 权限设置教程](./cloudflare-api-token.md)。

## 5. GitHub 路径

GitHub 上传使用顶层 `github` 配置，包含 owner、repo、branch、path_template、commit_message_template、format、模板字段、token 和 `top_n`。

执行路径：

| 入口 | 行为 |
| --- | --- |
| 当前结果页手动导出 | 用户主动把当前结果导出到 GitHub。 |
| 工作流 `deliver_github` 节点 | 使用上游筛选结果或测速结果导出。 |
| 定时任务 GitHub 导出 | 调度完成后按配置自动导出；Android 后台定时任务不执行工作流，但单次测速完成后仍会执行该入口。 |
| 测速后自动推送 | 手动测速完成后按 `post_probe_push.github_enabled` 执行。 |

如果筛选后没有可导出结果，GitHub 导出会跳过并返回提示，不提交空结果。GitHub PAT 最小权限见 [GitHub PAT 权限设置教程](./github-pat.md)。

## 6. 工作流节点关系

高级上传模板使用以下链路：

```text
输入源组 -> 测速 -> 结果筛选 -> 结果检查
  有结果 -> DNS 推送 -> GitHub 导出 -> 结束(completed)
  无结果 -> 人工复核标记 -> 结束(manual_review)
```

节点语义：

- `filter_results` 负责共享筛选和下游数据准备。
- `deliver_dns` 负责 Cloudflare 推送，可覆盖记录名、记录类型、TTL、Top N 等。
- `deliver_github` 负责 GitHub 导出，可覆盖数据来源和 Top N。
- `branch_has_results` 让空结果进入人工复核路径，避免无声失败。

节点目录和运行时细节见 [工作流节点卡片功能设计清单](./pipeline-node-card-design.md)。

## 7. 维护口径

修改上传工作流时，优先保持这些约束：

- 新业务规则放在 `internal/appcore` 或领域核心，桌面和 Android 只做平台适配。
- 文档使用顶层 `cloudflare`、`github` 和 `post_probe_push` 作为主口径。
- 旧字段只在兼容读取说明中出现，不作为新配置示例。
- 同步运行 `node scripts/check-pipeline-catalog.mjs`，防止前后端节点目录漂移。
- 文档变更运行 `bash scripts/docs-check.sh`。

## 8. 后续方向

当前上传选择器已经覆盖 Cloudflare 和 GitHub 的主要场景。后续如果要扩展，优先考虑：

1. 在结果页补充更明确的上传预览，让用户在推送前看到 Cloudflare / GitHub 各会消费多少条结果。
2. 为结果仓库增加可选的 GitHub Actions 后处理模板，例如校验 `cfst-results/**/*.csv`、生成 `latest.json` 或构建 Pages 索引。
3. 当新增 WebDAV、对象存储或其他目标时，继续复用上传选择器，不为每个目标复制筛选逻辑。
