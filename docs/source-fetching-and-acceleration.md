# 输入源获取机制与加速计划

本文档说明 CFST-GUI 当前输入源获取机制、输入源在任务链路中的作用，以及后续加速获取的实施计划。目标是先把现状讲清楚，再把可落地的性能优化拆成低风险阶段。

## 输入源的作用

输入源是测速任务的候选池入口。用户在输入源页、配置文件或输入源档案中维护来源，任务启动时后端把这些来源统一准备成候选文本，再交给测速阶段处理。

当前输入源承担这些职责：

| 职责 | 说明 |
| --- | --- |
| 候选池构建 | 把手动内容、本地文件、远程 URL 解析为 IP、CIDR、域名和带端口候选。 |
| 预览和抓取 | 输入源页可单独预览或抓取来源，抓取会持久化最近读取状态。 |
| 任务准备 | 手动任务和定时任务都会在启动探测前读取 `sources` 并构建 stage0 候选池。 |
| 输入源档案 | `source-profiles.json` 保存多套来源，当前档案会同步到配置快照。 |
| 状态展示 | 每个来源维护 `last_fetched_at`、`last_fetched_count`、`status_text`。 |
| 端口上下文 | `1.1.1.1:2053`、`example.com:8443` 等来源端口会进入后续端口策略。 |
| COLO 过滤 | 来源级 COLO 条件可参与候选构建或 stage2 过滤映射。 |
| MICS 抽样 | `ip_mode=mcis` 的来源会先运行 MICS 抽样，再进入普通候选流程。 |

输入源不是独立任务队列。它的读取、解析和状态更新都服务于当前预览、抓取或测速任务。

## 当前获取链路

输入源核心逻辑收敛在 `internal/appcore`，桌面端和 Android 端通过各自适配层注入 HTTP 客户端、COLO 字典路径、默认上限和 MICS runner。

```text
UI / config / source profiles
  -> source payloads
  -> PrepareSources
  -> ProcessSource
  -> LoadSourceContent
  -> BuildSourceEntriesWithConfig
  -> PreparedSources.Text / SourceStatuses / SourcePorts / SourceColoFilters
  -> probe stage0 candidate pool
```

### 来源类型

| 类型 | 获取方式 | 失败特征 |
| --- | --- | --- |
| `inline` | 读取 `content` 并去除首尾空白。 | 空内容会被跳过，不触发读取。 |
| `file` | 读取本地 `path` 指向的文件。 | 路径为空或文件不可读会成为来源读取失败。 |
| `url` | 标准化 URL 后发起 HTTP GET。 | URL 非法、网络错误、非 2xx 状态或 body 读取失败会成为来源读取失败。 |

URL 会先补齐协议并校验为 `http` 或 `https`。远程输入源预览、抓取和任务准备阶段都使用直连 HTTP 客户端，不读取环境代理变量。

### 准备阶段

`PrepareSources` 负责把多个来源合并成单个候选文本，同时保留每个来源的状态和 warning。当前实现有几个关键语义：

- 停用来源不读取，并在没有旧状态文本时写入停用提示。
- 启用但没有实际输入的来源不读取，保留旧状态。
- 有输入的启用来源会进入处理队列。
- 处理结果按原输入源顺序合并，避免并发读取改变候选顺序。
- 单个来源读取失败会记录 warning 并继续处理其他来源。
- 缺失 COLO 文件属于 fatal error，会阻止后续无法正确执行的任务。
- 来源处理 panic 会转换为该来源失败，避免拖垮整轮准备。

当前工作树已经把有输入的启用来源改为最多 4 路并发处理，并保持稳定输出顺序。这可以减少多 URL 或慢文件场景下的总等待时间，但并发上限仍是固定值。

### 解析阶段

`BuildSourceEntriesWithConfig` 把原始文本交给 `probecore.BuildSourceEntries`。解析阶段会完成这些工作：

- 解析 IP、CIDR、域名、端口和无效行。
- 按来源级 `ip_limit` 控制候选数量。
- 按 `ip_mode` 选择顺序展开或 MICS 抽样。
- 解析来源级 COLO 条件，并在 stage2 模式下输出 `SourceColoFilterMap`。
- 返回 `SourcePorts`，让后续 TCP/trace/download 阶段知道来源端口覆盖关系。

因此，输入源加速不能只缓存最终 entries。解析结果受来源选项、全局配置、COLO 字典、域名解析器和 MICS 策略影响，必须把这些因素纳入缓存边界。

## 桌面端与 Android 差异

两端共享 `internal/appcore` 中的核心读取和解析逻辑，但适配层存在刻意差异。

| 项目 | 桌面端 | Android |
| --- | --- | --- |
| 预览接口 | `PreviewDesktopSource` / `FetchDesktopSource` | `PreviewSource` / `FetchSource` |
| 任务准备 | `prepareDesktopSources` | `Service.prepareSources` |
| HTTP 超时 | 30 秒 | 20 秒 |
| URL 重试 | 网络错误、读取错误、`429`、`5xx` 可重试 | 默认不重试 |
| GitHub Raw 兜底 | Raw 失败可尝试等价 jsDelivr URL | 默认不做 jsDelivr 兜底 |
| 文件来源 | 本地路径读取 | Android 私有路径或桥接后的路径读取 |

这个差异解释了同一套来源在桌面端和 Android 端表现不同的原因：桌面端对 GitHub Raw 源更宽容，Android 端更保守，失败会更快暴露给用户。

## 性能瓶颈

当前主要耗时点集中在四类场景：

| 瓶颈 | 原因 | 影响 |
| --- | --- | --- |
| 多个远程 URL | 每个 URL 都要发起 HTTP GET，慢源会拖慢任务准备。 | 启动任务前等待时间变长。 |
| 重复来源 | 相同 URL、相同文件或相同 inline 内容会重复读取。 | 浪费网络、磁盘和解析时间。 |
| MICS 来源 | MICS 会额外发起抽样探测。 | 单来源处理时间明显高于普通 traverse。 |
| 缺少阶段指标 | 目前状态更偏结果，不足以定位 fetch、build、MCIS 哪一段慢。 | 难判断应该优化网络、解析还是来源配置。 |

并发准备已经降低了多来源串行等待的成本，但它不能解决重复读取、跨任务重复下载、条件请求缺失和慢源定位问题。

## 六大维度加速计划

### 维度一：并发调度

桌面端保持最多 4 路并发读取有输入的启用来源；Android 端使用更低并发上限，避免移动网络、文件桥接和 gomobile 调用堆积。并发策略必须遵守这些不变量：

- 输出候选顺序继续按用户配置顺序稳定合并。
- 空来源和停用来源不进入处理队列。
- 单来源失败不阻断其他来源，缺失 COLO 文件等 fatal error 仍保留现有语义。
- 桌面端和 Android 端继续通过 `internal/appcore` 共享行为。

当前状态：桌面默认上限为 4，Android 默认上限为 2，并且不暴露为 UI 配置。

验收标准：现有 `internal/appcore` 输入源测试通过，race 测试不发现数据竞争。

### 维度二：去重缓存

同一次 `PrepareSources` 内只共享原始内容读取结果，不共享完整解析结果：

- URL key 使用标准化后的 URL。
- file key 使用清理后的路径。
- inline key 使用内容 hash，避免直接把大文本作为 map key。

跨轮缓存先支持 URL 内容缓存，缓存记录与配置文件分离，避免污染 `desktop-config.json` 和 `mobile-config.json`。

当前状态：已具备同轮内容去重和 URL 文件缓存接口；同轮缓存只缓存 raw 内容和实际命中的 URL。

验收标准：相同 URL 在同轮任务准备中只发起一次 HTTP GET；相同来源配置仍分别产生独立状态、warning 和候选上限效果。

### 维度三：网络获取

桌面端继续保留 GitHub Raw 到 jsDelivr 的兜底与 retry；Android 端保持单次直连，先通过诊断确认是否需要打开 jsDelivr 兜底。

URL 缓存可用时发送 `If-None-Match` / `If-Modified-Since`。收到 `304 Not Modified` 时复用缓存 body，并更新本次来源状态；缓存不可用、损坏或读取失败时回退直连。

当前状态：URL 获取支持持久化缓存、`ETag` / `Last-Modified` 条件请求和 `304` 复用。

验收标准：重复启动任务时未变化的 URL 源不重复下载 body；缓存不可用时自动回退直连读取。

### 维度四：解析与 MICS

内容去重后，每个 source 仍独立执行 `BuildSourceEntriesWithConfig`，保证这些来源级配置各自生效：

- `ip_limit`
- `ip_mode`
- `colo_filter`
- `colo_filter_mode`
- 默认端口和来源端口上下文
- MICS runner 和 COLO 字典

MICS 耗时单独计量，不把 MICS 结果和普通 traverse 结果混用。解析结果缓存不作为第一阶段实现；后续如需实现，key 必须包含来源选项、全局解析配置、COLO 字典版本、resolver 和 MICS 配置。

### 维度五：Android 生命周期

Android 加速逻辑优先复用 `internal/appcore`，平台差异留在 `mobileapi` 适配层：

- 准备阶段使用更低并发上限。
- 预览、抓取和任务准备使用移动端私有缓存路径。
- URL 读取仍保持不重试、不 jsDelivr 兜底的默认行为。

后续阶段需要补 URL/file/MICS 的取消或超时传播，确保手动取消、重复 start、系统调度重入时不会留下后台读取。

### 维度六：观测与验证

每个来源记录 fetch、build、mcis、total 耗时，并记录同轮缓存、跨轮缓存、条件请求、HTTP 状态码和实际使用 URL。诊断优先写入 debug log，不增加普通 UI 复杂度。

当前状态：任务 debug log 会写入 `source.prepare.detail` 事件，慢源诊断包含 source id/name/kind、阶段耗时和缓存命中信息。

验收标准：debug log 中能看到每个来源的阶段耗时；普通用户 UI 不因诊断字段增加而变复杂。

## 缓存边界

加速实现必须区分内容缓存和解析缓存。

| 层级 | 可共享条件 | 不能忽略的因素 |
| --- | --- | --- |
| 内容缓存 | 相同 URL、相同文件或相同 inline 内容。 | URL 标准化、文件路径和修改信息、HTTP 条件请求元数据。 |
| 解析缓存 | 相同 raw 内容且解析相关配置完全一致。 | `ip_limit`、`ip_mode`、COLO 条件、COLO 字典、resolver、MICS 配置、默认端口策略。 |

优先实现内容缓存，因为它收益明确且不容易改变候选语义。解析缓存收益更高，但必须更保守，否则容易让不同来源配置错误复用同一批 entries。

## 兼容性要求

后续实现加速时必须保持这些行为不变：

- `sources` 配置结构保持兼容，不强制用户迁移。
- 输入源顺序决定最终候选文本顺序。
- 预览默认不持久化状态，抓取和任务准备按现有规则更新状态。
- 单来源读取失败继续保留 warning 并跳过该来源。
- 缺失 COLO 文件继续作为 fatal error。
- 桌面端和 Android 端优先复用 `internal/appcore`，差异只放在适配层。
- 不把自动加速机制暴露成复杂 UI，除非已有诊断数据证明用户需要手动控制。

## 验证建议

文档阶段只需要运行 Markdown 检查：

```bash
bash scripts/docs-check.sh
git diff --check
```

后续代码实现阶段建议补充：

- `go test -count=1 ./internal/appcore`
- `go test -race -count=1 ./internal/appcore`
- `go test -count=1 ./internal/app ./mobileapi`
- 桌面端 URL 兜底、同轮去重和 debug log 诊断测试。
- Android 端低并发、active/cancel 场景下的来源读取取消测试。
- HTTP 条件请求的 `200`、`304`、缓存损坏和缓存过期测试。
