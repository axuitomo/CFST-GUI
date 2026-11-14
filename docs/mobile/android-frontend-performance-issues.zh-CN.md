# Android 前端性能排查结果

## 范围

- 构建：当前工作区的 Android ARM64 Debug 构建，包名 `io.github.axuitomo.cfstgui.debug`，版本 `2.0.6`。
- 设备：`23113RKC6C`，Android 16 / API 36，WebView `153.0.8010.36`，CSS 视口 `411 × 914`；屏幕支持 60/90/120 Hz，采样后上报为 120 Hz。
- 测试数据规模：2 个输入配置（input profile）和 4 行数据源（source row）。名称和数据源内容有意省略。
- 输入方式：通过 CDP 向真实设备的 WebView 派发鼠标事件，而不是模拟器点击。这样可以覆盖浏览器的事件与渲染路径，但不属于手指触摸的延迟测量。
- 交互耗时采用 `PerformanceObserver` 的 Event Timing 条目，阈值为 16 ms；下文中的“INP 候选值”指本次样本中观测到的最大交互条目，并非正式的 field INP 百分位数值。帧率来自短窗口的 `requestAnimationFrame` 采样。未加入任何常驻埋点。

## 发现

### P1：移动端页面切换会保留上一页的滚动偏移

**复现步骤**：在设置页把 `.mobile-main.scrollTop` 设为 `600`，然后导航到“数据源”页。设置页的 `scrollHeight=1727`、`clientHeight=411`。在“数据源”页，共用的滚动容器保留并把偏移量钳制到 `307.7`（`719 - 411`）；此时“展开数据源组管理器”控件位于视口 Y 坐标 `-170`，不可见。继续导航到“仪表盘”页时仍保留 `scrollTop=307.7`，页面内容起始于 Y `-183.7`。

这会使新选中的页面一打开就停在半途，其顶层控件可能被挤出视口。该现象可稳定复现，且与性能观测器无关。

**代码位置**：[MobileShell.vue](../../frontend/src/components/layout/MobileShell.vue#L44) 在变化的 slot 外围维护着同一个 `.mobile-main` 滚动容器。[App.vue](../../frontend/src/App.vue#L2527) 只切换当前选中视图，既没有重置、也没有恢复该容器的滚动位置。

**置信度**：已确认。

### P2：反复展开/收起输入配置文件管理器时帧率掉到约 60 FPS

测试在存在 2 个配置的情况下把该管理器开关了 5 次。最初几个循环约为 118–120 FPS；第 3 至第 5 个循环反复降到约 60 FPS。整轮运行中，INP 候选值的 P50 为 `24 ms`、最大 `56 ms`；单独重复一次则达到 `64 ms`。这次重复运行记录到 16.7 ms 的帧间隔，且没有超过 50 ms 的 `longtask` 条目。之后对 4 个数据源编辑器做“全部展开再全部收起”的操作，则停留在约 118–120 FPS，INP 候选值 P50 为 `16 ms`、最大 `24 ms`。

配置文件管理器会先一起创建配置列表及其控件，然后对容器高度做动画。该 CSS transition 在 260 ms 内改变 `height`、margin 和 padding。在反复开关的情况下，这可以合理地构成布局/绘制开销；没有 long task 说明存在大规模同步 JavaScript 任务的可能性较低，但这并不能证明渲染层就是根因。

**代码位置**：[SourcesView.vue](../../frontend/src/views/SourcesView.vue#L633) 负责开关该管理器；配置列表通过 `v-for` 在 [SourcesView.vue](../../frontend/src/views/SourcesView.vue#L655) 渲染。[styles.css](../../frontend/src/styles.css#L869) 对 `height`、margin 和 padding 做动画。

**置信度**：症状可重复复现已确认；布局/绘制的归因仍属假设，待浏览器/Perfetto trace 验证。由于设备上只有 2 个配置可用，更大的生产环境配置尚未测试。

### P2：页面切换存在孤立的掉帧，但没有 JS long task

在“仪表盘”“结果”“设置”和“数据源”四个视图间共切换 17 次，交互候选值 P50 为 `48 ms`、最大 `96 ms`。短窗口帧率采样 P50 为 `110.6 FPS`，最低 `99.2 FPS`；在动作窗口内共有 25 个采样帧间隔超过 `16.7 ms`。未记录到超过 50 ms 的 `longtask` 条目。屏幕支持 120 Hz，运行结束后 Android 上报为激活的 120 Hz 模式；但刷新模式并未固定，也没有针对每一次交互单独采样。

路由路径在受支持时使用 `document.startViewTransition()`，切换由 `v-if` 选中的页面子树，并对整个 `.page-transition-content` 快照播放 60–80 ms 的动画。这些操作是孤立延迟的主要嫌疑对象，但现有数据无法区分究竟是快照捕获、布局、绘制、合成，还是 Android 的刷新率策略所致。

**代码位置**：[App.vue](../../frontend/src/App.vue#L2527) 启动视图过渡；[MobileShell.vue](../../frontend/src/components/layout/MobileShell.vue#L46) 将其应用于整个页面 slot；[styles.css](../../frontend/src/styles.css#L813) 定义了快照动画的时长。

**置信度**：本轮运行中掉帧症状已确认；具体所处的渲染阶段尚未确认。

### 设置页与数据源组相关控件

测试覆盖了“基础/高级”设置切换，以及在线更新、探测策略、应用数据目录这几个区块的展开/收起。10 个被测动作的 INP 候选值 P50 为 `24 ms`、最大 `40 ms`，帧率 P50 为 `120.1 FPS`，无 long task。这些动作窗口内共有 3 个帧间隔超过 `16.7 ms`。

输入组设置面板被打开、关闭了三次。6 个动作的 INP 候选值 P50 为 `24 ms`、最大 `40 ms`；帧率 P50 为 `109.8 FPS`，无 long task。这些动作并未出现配置文件管理器列表那种反复掉到 60 FPS 的表现。

**代码位置**：设置模式与条件渲染的高级内容位于 [SettingsView.vue](../../frontend/src/views/SettingsView.vue#L493) 和 [SettingsView.vue](../../frontend/src/views/SettingsView.vue#L824)；输入组控件的过渡位于 [SourcesView.vue](../../frontend/src/views/SourcesView.vue#L641)。

## 测量汇总

| 场景 | 动作数 | INP 候选值 P50 / 最大 | RAF 帧率 P50 / 最低 | long task |
| --- | ---: | ---: | ---: | ---: |
| 设置模式与各区块 | 10 | 24 / 40 ms | 120.1 / 116.3 | 0 |
| 配置文件管理器展开/收起 | 10 | 24 / 56 ms；重复测试最大 64 ms | 118.1 / 59.9 | 0 |
| 输入组设置面板 | 6 | 24 / 40 ms | 109.8 / 104.3 | 0 |
| 4 个数据源编辑器，全部打开后全部关闭 | 8 | 16 / 24 ms | 120.2 / 118.3 | 0 |
| 页面切换 | 17 | 48 / 96 ms | 110.6 / 99.2 | 0 |

FPS 是观测到的短窗口帧率，并非固定的屏幕刷新率保证值。帧间隔计数只作为样本报告，不代表设备整体的卡顿百分比。

## 下一步验证
原有的初始验证条目已被下面 2026-11-13 的 Perfetto 与 INP 复测取代。

## Perfetto 与 INP 复测（2026-11-13）

### 采集

- 设备与构建保持不变：`23113RKC6C`，Android 16 / API 36，WebView `153.0.8010.36`，Debug `2.0.6`；数据源测试数据为 2 个配置、4 行数据源。
- 使用 `linux.ftrace`（应用范围内的 `gfx`、`view`、`input`、`wm`、`am`、`webview`、`sched`、CPU 频率/空闲）、`android.surfaceflinger.frametimeline`、`android.surfaceflinger.frame` 和 `android.surfaceflinger.layers` 采集了一段 90 秒的 Perfetto trace。该 trace 覆盖了 88.81 秒的 FrameTimeline 数据。
- Chrome tracing 并行运行，分三个阶段：设置、数据源、以及重复的页面导航。Event Timing 条目按 `interactionId` 分组；每次点击的事件耗时取其中最大者作为 INP 候选值。耗时被拆分为输入延迟、事件处理与呈现延迟三部分。这仍然是通过 CDP 派发的输入，不是手指触摸测试，也不是 field INP。Chrome tracing 本身有开销，可能使时间数值偏高。
- 本地原始采集文件（未提交入库）：`cfst-focused.perfetto-trace`、`cfst-chrome-settings.json`、`cfst-chrome-sources.json`、`cfst-chrome-navigation.json` 和 `cfst-inp-results.json`。这些是临时产物，不是仓库里的长期夹具。

### INP 结果

| 场景 | 动作数 | 候选值 P50 | P95 | 最大 | 渲染进程 `RunTask` >50 ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| 设置页导航、各区块与模式切换 | 9 | 24 ms | 168 ms | 168 ms | 4 |
| 输入配置文件管理器展开/收起 | 12 | 32 ms | 48 ms | 48 ms | 0 |
| 输入组设置面板展开/收起 | 6 | 24 ms | 40 ms | 40 ms | 0 |
| 4 个数据源编辑器，打开与关闭 | 8 | 16 ms | 32 ms | 32 ms | 0 |
| 页面导航 | 13 个有效动作 | 64 ms | 112 ms | 112 ms | 6 |

“设置”的最大值出现在进入页面时：总计 `168 ms`，拆分为 `15.9 ms` 输入延迟、`2.5 ms` 处理、`149.6 ms` 呈现。进入“数据源”页测得 `152 ms`（`0.7 ms` 输入延迟、`0.2 ms` 处理、`151.1 ms` 呈现）。这说明瓶颈在输入之后的渲染/呈现阶段，而非事件处理函数慢。

输入配置文件管理器方面，12 次开关全部在 `48 ms` 及以下完成；渲染进程主线程没有超过 50 ms 的 `RunTask`。这些窗口内最长的 `UpdateLayoutTree` / 样式-布局切片为 `15.4 ms`，最长的 `Paint` 切片不足 `1 ms`。本次运行未能复现早前持续 60 FPS 的观测结果，同样也没有建立在设备上存在两个以上配置时的扩展性行为。

高级/基础设置模式切换测得 `64 ms` 和 `56 ms`；二者各自对应一次约 `77 ms` 的渲染进程 `RunTask`。设置页进入时有一次 `136.3 ms` 的渲染进程主线程 `RunTask`；其嵌套 trace 中包含一次 `20.5 ms` 的样式/布局更新。页面导航有 6 个超过 50 ms 的渲染进程主线程任务，最大 `86.8 ms`，最大样式/布局切片 `34.1 ms`。路由过渡期间出现了 `ViewTransition::ProcessCurrentState` 事件。这些都是实测到的相关性；trace 本身并不能证明延迟单由 View Transition 造成。

### Perfetto FrameTimeline 的局限

对 `actual_frame_timeline_slice` 的 88.81 秒 Perfetto 查询，只返回了 `/system/bin/surfaceflinger` 与 `com.android.systemui` 的行，**没有任何 `io.github.axuitomo.cfstgui.debug` 或其 `MainActivity` 图层的行**。更早的一次宽泛采集只有 67 行 CFST 图层数据、仅跨越 0.55 秒，同样不足以计算应用卡顿率。因此这次 Perfetto 采集无法支撑一个有效的 CFST FPS 或应用卡顿百分比。上面的 Chrome 渲染进程 trace 提供了主线程/布局/绘制的证据，但不能替代应用的 FrameTimeline 呈现数据。

### 更新后的评估

1. 设置页和路由进入的 INP 离群值以呈现阶段为主。事件处理函数本身通常都很短；页面挂载、样式/布局以及过渡的呈现才是可能的开销重心。
2. 设置模式切换是最明确的同步主线程热点：两次模式切换都出现约 77 ms 的 `RunTask`。
3. 在当前数据规模下，数据源配置文件管理器的开关没有出现超过 50 ms 的渲染任务。此前的帧率下降仍未被确认，也不应在缺少对应应用帧 trace 的情况下归因于 CSS 高度动画。
4. 上文记录的滚动偏移缺陷是独立问题，且仍然成立。

### 仍需验证

1. 修复或以其他方式验证针对 WebView/MainActivity 图层的 Android FrameTimeline 采集，然后在交互序列开始前立即启动一段短 trace 重跑。在应用行出现之前，FPS 与卡顿结论均应保持开放。
2. 在关闭 DevTools tracing、并使用真实手指点击的情况下，重复设置页进入/模式切换与页面导航，以估算 tracing 和 CDP 输入带来的额外开销。
3. 用 10、25、50 个配置重复配置文件管理器测试。

本次复测未改动任何应用代码。设置模式与数据源面板均已还原；应用停留在“仪表盘”页，滚动位置已重置。
