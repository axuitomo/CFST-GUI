# Cloudflare / GitHub 上传工作流设计

## 1. 目标

围绕当前 CFST-GUI 的测速结果，设计一套统一的“结果上传工作流”，满足这些能力：

1. 支持按结果规则筛选待上传数据，而不是无条件上传全部结果。
2. 支持自定义上传数量，例如“只上传前 1 条 / 前 3 条 / 前 10 条”。
3. 支持同一次测速结果同时输出到两个目标：
   - Cloudflare DNS 记录覆盖推送
   - GitHub 仓库结果 CSV 导出
4. 支持手动触发和定时触发两种模式。
5. 兼容现有 `print_num`、CSV 导出、定时任务、Cloudflare 和 GitHub 配置结构。

## 2. 当前现状

### 2.1 当前调度流程

当前定时任务完成测速后，会直接拿 `result.Results` 执行后续动作：

1. 运行测速工作流。
2. 若开启 `auto_dns_push`，把 `result.Results` 的 IP 列表推送到 Cloudflare。
3. 若开启 `auto_github_export`，把 `result.Results` 直接编码成 CSV 并导出到 GitHub。

关键代码：

- 调度器主流程：[internal/app/scheduler.go](/home/axuitomo/code/CFST-GUI/internal/app/scheduler.go:120)
- Cloudflare 自动推送调用：[internal/app/scheduler.go](/home/axuitomo/code/CFST-GUI/internal/app/scheduler.go:200)
- GitHub 自动导出调用：[internal/app/scheduler.go](/home/axuitomo/code/CFST-GUI/internal/app/scheduler.go:217)

### 2.2 当前结果裁剪方式

当前 `print_num` 会在测速工作流内部先裁掉最终结果，只保留加权评分后的 Top N：

- 结果裁剪位置：[internal/probecore/workflow.go](/home/axuitomo/code/CFST-GUI/internal/probecore/workflow.go:165)
- `PrintNum` 默认值与配置定义：[internal/probecore/probe_config.go](/home/axuitomo/code/CFST-GUI/internal/probecore/probe_config.go:133)

这意味着现在的“显示数量”和“上传数量”是耦合的。

### 2.3 当前 CSV 能力

项目已经具备两条和 CSV 相关的现成能力：

1. 将测速结果行导出为 CSV
2. 从已生成的结果 CSV 再读回结构化结果行

关键代码：

- 手动导出 CSV / GitHub：[internal/app/github_export.go](/home/axuitomo/code/CFST-GUI/internal/app/github_export.go:49)
- GitHub 导出 `ProbeRow`：[internal/app/github_export.go](/home/axuitomo/code/CFST-GUI/internal/app/github_export.go:106)
- 读取结果 CSV：[internal/app/app.go](/home/axuitomo/code/CFST-GUI/internal/app/app.go:581)

### 2.4 当前设置页入口

现有设置页已经具备以下基础入口：

- 定时后自动推送 Cloudflare DNS：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:1191)
- 定时后自动导出 GitHub：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:1198)
- 导出文件名模板与目录：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:1270)
- GitHub 导出配置：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:1306)
- 结果显示数量 `probePrintNum`：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:979)

## 3. 核心问题

当前实现已经能“测速后推送”，但还不够细：

1. `print_num` 同时影响显示结果和后续上传，缺少“上传专用数量”。
2. Cloudflare 和 GitHub 共用同一批 `result.Results`，但没有“上传前筛选层”。
3. 结果筛选逻辑虽然存在于测速阶段，但没有独立的“上传策略”配置。
4. 定时任务只能做“跑完就上传”，不能表达：
   - 只上传低延迟结果
   - 只上传指定 COLO
   - 只上传下载速度超过阈值的结果
   - 只把前 N 条上传到 Cloudflare，但把前 M 条导出到 GitHub

## 4. 目标工作流

建议把工作流拆成四层：

1. **测速层**：负责生成完整结果。
2. **结果展示层**：负责界面显示和本地 CSV 导出。
3. **上传筛选层**：负责从结果集中挑选“要上传的子集”。
4. **目标执行层**：把筛选后的结果分别发送到 Cloudflare / GitHub。

推荐流程如下：

```text
RunDesktopProbe
  -> 得到完整结果 rows/rawResults
  -> 生成本地 result.csv
  -> ApplyUploadSelection(config, results)
      -> uploadRowsForCloudflare
      -> uploadRowsForGitHub
  -> PushCloudflareDNSRecords(uploadRowsForCloudflare)
  -> ExportProbeRowsToGitHub(uploadRowsForGitHub)
```

## 5. 配置设计

建议新增一个独立配置块：`upload`。

```json
{
  "upload": {
    "enabled": true,
    "source": "memory_results",
    "selection_mode": "filtered_topn",
    "shared_filter": {
      "status": ["passed"],
      "ip_version": "any",
      "colo_allow": [],
      "colo_deny": [],
      "max_tcp_latency_ms": 0,
      "max_trace_latency_ms": 0,
      "min_download_mbps": 0,
      "max_loss_rate": 0
    },
    "cloudflare": {
      "enabled": true,
      "top_n": 2
    },
    "github": {
      "enabled": true,
      "top_n": 10
    }
  }
}
```

### 字段说明

**`upload.source`**

- `memory_results`：直接使用本轮测速完成后的内存结果
- `result_csv`：从 `result.csv` 重新读取后再筛选

建议第一阶段只实现 `memory_results`，因为当前调度器已经拿到了 `result.Results`，实现成本最低。`result_csv` 可以作为第二阶段兼容选项，便于“重传最近一次结果”。

**`upload.selection_mode`**

- `all_results`：不额外筛选，全部上传
- `filtered_only`：仅按上传筛选规则过滤
- `filtered_topn`：先过滤，再按评分取 Top N

**`upload.shared_filter`**

这部分定义跨上传目标共用的规则，避免 Cloudflare 和 GitHub 配两套相似条件。

建议第一版支持：

- `status`
- `ip_version`
- `colo_allow`
- `colo_deny`
- `max_tcp_latency_ms`
- `max_trace_latency_ms`
- `min_download_mbps`
- `max_loss_rate`

**目标专属 `top_n`**

这是本设计最关键的字段。它把“上传数量”从 `print_num` 中拆出来。

例如：

- `print_num = 20`：界面展示 20 条
- `upload.cloudflare.top_n = 2`：Cloudflare 只推前 2 条
- `upload.github.top_n = 10`：GitHub 只导出前 10 条

## 6. 评分与筛选策略

### 6.1 默认排序策略

建议复用现有评分口径，保持用户认知一致：

- 30% TCP/延迟得分
- 70% 下载速度得分

对应当前实现：

- `LimitFinalProbeResults` 已有现成逻辑：[internal/probecore/result.go](/home/axuitomo/code/CFST-GUI/internal/probecore/result.go:81)

建议新增一个纯 `ProbeRow` 版本的选择函数，例如：

```go
func SelectUploadRows(rows []ProbeRow, limit int, metric string) []ProbeRow
```

这样可以避免先回退到 `CloudflareIPData` 再做转换。

### 6.2 Cloudflare 特殊规则

Cloudflare DNS 推送与 GitHub CSV 不同，建议额外加两条保护规则：

1. 当筛选后结果为空时，默认跳过推送，不删除现有 DNS。
2. 当目标记录类型是 `A` 时，只取 IPv4；`AAAA` 时只取 IPv6。

这一层可在调用 `PushCloudflareDNSRecords` 前处理。

## 7. 后端落点建议

### 7.1 新增统一上传选择器

建议新增文件：

- `internal/app/upload_selection.go`

建议职责：

1. 解析 `upload` 配置
2. 对 `[]ProbeRow` 应用筛选规则
3. 产出两个独立结果集：
   - `RowsForCloudflare`
   - `RowsForGitHub`
4. 产出筛选日志 / warning，供调度器状态和前端提示使用

建议接口：

```go
type UploadSelectionResult struct {
    SharedFilteredRows []ProbeRow
    CloudflareRows     []ProbeRow
    GitHubRows         []ProbeRow
    Warnings           []string
}

func BuildUploadSelection(snapshot map[string]any, rows []ProbeRow) (UploadSelectionResult, error)
```

### 7.2 调度器改造点

当前调度器直接使用 `result.Results`。建议改为：

1. 先 `BuildUploadSelection(snapshot, result.Results)`
2. `Cloudflare` 使用 `selection.CloudflareRows`
3. `GitHub` 使用 `selection.GitHubRows`

改造位置：

- [internal/app/scheduler.go](/home/axuitomo/code/CFST-GUI/internal/app/scheduler.go:200)

### 7.3 手动导出改造点

当前“导出 GitHub”按钮也可以复用同一选择器，让手动行为和定时行为一致。

建议入口：

- [internal/app/github_export.go](/home/axuitomo/code/CFST-GUI/internal/app/github_export.go:86)

可增加一个布尔配置，例如：

- `export.github.use_upload_selection`

但更简洁的做法是：只要 payload 里传 `results`，就先按上传规则过滤，再导出。

### 7.4 CSV 回读兼容

如果未来要支持“按已生成的 CSV 重传”，可复用：

- [internal/app/app.go](/home/axuitomo/code/CFST-GUI/internal/app/app.go:581)

做法是先 `ListResultFile`，再把读回的 `rows` 丢给 `BuildUploadSelection`。

## 8. 前端设计

建议在“导出设置”下面新增一个二级区块：`上传策略`。

### 8.1 建议字段

**基础开关**

- 启用上传策略
- 上传源：本轮结果 / 最近结果 CSV

**共用筛选**

- 状态：全部 / 仅通过
- IP 版本：全部 / IPv4 / IPv6
- COLO 允许列表
- COLO 排除列表
- 最大 TCP 延迟
- 最大追踪延迟
- 最低下载速度
- 最大丢包率

**目标专属**

- Cloudflare 上传数量 Top N
- GitHub 上传数量 Top N

### 8.2 放置位置

最合适的位置仍然是设置页的导出区，因为它和以下配置强关联：

- GitHub 导出
- Cloudflare 自动推送
- 调度器自动上传

参考现有设置区域：

- 导出设置：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:1254)
- 调度器自动动作：[frontend/src/views/SettingsView.vue](/home/axuitomo/code/CFST-GUI/frontend/src/views/SettingsView.vue:1191)

## 9. Cloudflare 工作流设计

### 9.1 手动工作流

```text
用户运行测速
  -> 查看结果
  -> 点击“推送 Cloudflare”
  -> 应用上传筛选规则
  -> 取 Cloudflare Top N
  -> 转换为 ipsRaw
  -> 覆盖写入目标 DNS 记录
```

### 9.2 定时工作流

```text
定时任务触发
  -> 运行测速
  -> 更新最近运行档案
  -> 计算上传选择结果
  -> 若 cloudflare.enabled = true 且 AutoDNSPush = true
      -> 推送筛选后的 Top N 结果
  -> 写入 schedulerStatus
```

### 9.3 建议状态补充

建议在 `schedulerStatus` 里增加两类可观测信息：

- `upload_selection_summary`
- `cloudflare_uploaded_count`

这样前端能显示“本次测速 20 条，筛选后 4 条，Cloudflare 上传 2 条”。

## 10. GitHub 工作流设计

这里分成两种含义。

### 10.1 应用内 GitHub 结果上传工作流

```text
测速完成
  -> 应用上传筛选规则
  -> 取 GitHub Top N
  -> 编码成 CSV
  -> 按 path_template 提交到目标仓库
```

现有入口足够好，主要只需要把“上传前选择”插进去：

- [internal/app/github_export.go](/home/axuitomo/code/CFST-GUI/internal/app/github_export.go:86)

### 10.2 GitHub Actions 配套工作流

当前仓库已有两条 CI 工作流：

- 发布 Release：[.github/workflows/release.yml](/home/axuitomo/code/CFST-GUI/.github/workflows/release.yml:1)
- 发布 GHCR 容器：[.github/workflows/container.yml](/home/axuitomo/code/CFST-GUI/.github/workflows/container.yml:1)

建议新增第三条工作流，例如：

- `.github/workflows/upload-results-template.yml`

用途不是替代应用内上传，而是提供“结果仓库自动后处理”模板，例如：

1. 当 `cfst-results/**/*.csv` 有新提交时触发
2. 校验 CSV 表头和格式
3. 生成最近一次摘要 `latest.json`
4. 生成可供 Pages 展示的聚合文件
5. 可选同步到另一个分支或对象存储

建议触发条件：

```yaml
on:
  push:
    paths:
      - "cfst-results/**/*.csv"
  workflow_dispatch:
```

建议步骤：

1. Checkout
2. 校验 CSV 文件
3. 读取最新结果并生成摘要
4. 上传摘要 artifact 或提交到 `cfst-results-index/`

这条 Actions 工作流的价值在于：

- 把“上传结果”和“结果消费”解耦
- 方便后续接 Pages、看板或对外 API
- 不影响桌面端与移动端现有上传能力

## 11. 分阶段落地建议

### Phase 1

先做最小可用版本：

1. 新增 `upload` 配置
2. 新增 `BuildUploadSelection`
3. 调度器自动 Cloudflare / GitHub 改为走统一选择器
4. 前端增加“Cloudflare Top N / GitHub Top N”

### Phase 2

继续增强：

1. 增加共享筛选规则
2. 支持 `result_csv` 重传
3. 手动导出 GitHub 也走统一选择器
4. 结果页显示“本次上传预览”

### Phase 3

做生态配套：

1. 增加 `.github/workflows/upload-results-template.yml`
2. 自动生成聚合 JSON
3. 预留接入 Cloudflare Pages / GitHub Pages 展示页

## 12. 结论

最推荐的设计不是直接在 Cloudflare 和 GitHub 各自增加一套筛选逻辑，而是先抽一层统一的“上传选择器”。

这样做有三个直接好处：

1. Cloudflare 与 GitHub 上传行为一致，减少分叉。
2. `print_num` 可以继续负责“展示结果”，上传数量改由独立 `top_n` 控制。
3. 后续不管增加 WebDAV、对象存储还是 Pages，同样可以复用这一层。

如果按实现成本和收益排序，优先级建议是：

1. 先拆出 `upload.cloudflare.top_n` / `upload.github.top_n`
2. 再加共享筛选规则
3. 最后补 GitHub Actions 的结果后处理工作流
