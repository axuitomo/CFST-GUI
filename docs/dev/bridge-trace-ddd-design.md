# 桥接追踪埋点 · DDD 设计方案（方案 B）

> 状态：已实现并通过 Go 侧验证（build / vet / test + 行为测试）；Kotlin 侧待 Android 构建验证；前端类型检查见文末。
> 范围：仅诊断用埋点，不改桥接协议、命令契约、配置结构或任何平台行为。

## 1. 背景与问题域

**症状**：Android（小米澎湃）薄平台适配 / 桥接不可靠；ADB 与 logcat 受限，运行态是黑盒；此前多次"让 AI 修改"无效的根因是缺少数值证据——每次调用到底传了什么、走到哪一层、在哪一层失败。

**目标**：为三条缝（Vue 前端 / Kotlin 插件 / Go 服务）各加一处观测触点，把每次桥接调用的入参、出参、失败信息汇聚为**单一文件落盘**（`bridge-debug.log`），可通过现有 `debug.export` 导出取证。

**非目标**：不引入外部遥测、不上报网络、不改 gomobile 绑定 API、不改变任何命令/事件契约。

## 2. 统一语言（Ubiquitous Language）

| 术语 | 含义 |
| --- | --- |
| BridgeTraceEvent | 一次桥接观测的最小记录，JSONL 一行 |
| TraceSink | 唯一落盘者（Go 侧持有文件与互斥） |
| 生产者 | Vue 前端 / Kotlin 插件 / Go 服务，三个观察视角 |
| bridge-debug.log | 汇聚后的追踪文件，位于 `<runtimeDir>/logs/` |
| 弱失败 | trace 的任何失败都不得影响主命令/事件链路 |
| 门控 | 仅当 Go `Init` 成功（baseDir 非空）后追踪才启用 |

## 3. 限界上下文与落点映射

| 限界上下文 | 代码位置 | 触点 | 事件名 |
| --- | --- | --- | --- |
| 前端适配层 | `frontend/src/lib/bridge.ts` | `invokeCore`（native 分支）、`ensureNativeBridge` | `invoke.in` / `invoke.out` / `bridge.init_failed` |
| 原生插件层 | `CfstPlugin.kt` | `Invoke` 入口、`rejectWithLog` | `invoke.in` / `invoke.reject` |
| 核心服务层 | `mobileapi/invoke.go`、`probe.go`、`service.go` | `Service.Invoke`、`deliverProbeEvent`、`Init` | `invoke.in` / `invoke.out` / `event.out` / `init` |
| 观测域（横切支撑） | `mobileapi/trace.go` | `Service.trace` → 文件 | 汇聚全部上游事件 |

成功路径不重复埋点：Kotlin 成功时由 Go 的 `invoke.out` 与前端 `invoke.out` 覆盖；Kotlin 只补**入口**（证明调用到达原生层）与**拒绝**（失败证据）。

## 4. 领域模型

- **实体 BridgeTraceEvent**：`{event, level, ts, command, payload(截断), ok, code, message, error, task_id, base_dir, ...}`，一次调用一条。
- **值对象 TracePolicy**：`enabled`、`payload_limit`（Go 2000 / Kotlin 与前端 500）、`max_file_size`（4MB）、`max_archives`（3）。
- **聚合 TraceLog**：单文件 JSONL，由 TraceSink 维护「追加 + 轮转 + 互斥」这一组不变量。
- **领域服务 TraceRecorder**：`record(event, fields)`，屏蔽生产者差异，统一归一化、脱敏、截断。
- **基础设施 FileTraceSink**（Go，唯一写文件者）；前端不再把原始追踪数据输出到 `console.debug`，避免开发者工具泄露敏感字段。

## 5. 关键不变量（领域规则）

1. **弱失败**：所有写日志失败被吞掉；Kotlin `try/catch`、前端 `.catch(() => {})`、Go 吞写错误，主链路不受影响。
2. **单通道汇聚**：只有 Go 写文件；Kotlin 与前端经 `bridge.trace` 命令汇入，避免多写者竞争和三套轮转/脱敏实现。
3. **门控**：Go 侧仅 `Init` 成功后落盘（contract/单测不调 Init 则零文件，测试机无副作用）；Kotlin 侧要求 `service` 已初始化。
4. **脱敏**：整行经 `utils.RedactSensitiveText`（Bearer Token、Telegram Bot Token、URL query 敏感键）。
5. **有界资源**：4MB × 3 轮转、payload 截断，探测事件高频时磁盘与 IO 压力有上限。
6. **强制落盘**：文件写入先刷新用户态缓冲，再调用 `File.Sync()`；关闭和轮转前同样同步，尽量保证诊断记录在进程异常退出前已提交到存储设备。
7. **成对可还原**：`invoke.in` / `invoke.out` 成对出现，事件带 `ts`，前端事件流可核对 seq。

## 6. 一次调用的追踪时间线（示例：probe.start）

```text
Vue    invoke.in   {command:"probe.start", payload:"{...}"}
Kotlin invoke.in   {command:"probe.start", payload:"{..."}
Go     invoke.in   {command:"probe.start", payload:"{...}"}
Go     invoke.out  {command:"probe.start", result:"{code:PROBE_STARTED,ok:true...}"}
Go     event.out   {event:"probe.started", task_id:"...", payload:"{...}"}   (若干)
Vue    invoke.out  {command:"probe.start", ok:true, code:"PROBE_STARTED"}
```

失败时额外出现：`Kotlin invoke.reject {action:"Invoke(probe.start)", error:"..."}` 或前端 `bridge.init_failed`。据此可立即区分：**调用是否到达原生层 / 到达后是否进入 Go / Go 是否返回 / 返回后前端是否解析成功**。

## 7. 决策记录（ADR）

| 决策 | 选择 | 理由 |
| --- | --- | --- |
| 落盘者 | Go 单通道，而非三层各自写文件 | 共享复用：一套轮转/脱敏/互斥；避免 JNI/JS 多写者竞争 |
| 跨层透传 | 新增 `bridge.trace` 命令，而非新 JNI 导出方法 | 不改 gomobile 绑定 API，无需重生成绑定，契约零变化 |
| 埋点位置 | `mobileapi`（平台适配层）而非 `appcore` | 观测属平台诊断关注点，保持架构边界；appcore 逻辑零改动 |
| 门控方式 | `Init` 成功后启用（baseDir 非空） | 天然隔离测试与生产；Android 端 Init 恒被调用，功能完整 |

## 8. 验证与回滚

**已完成**：

- `go build ./mobileapi` / `go vet ./mobileapi` ✓
- `go test ./mobileapi ./internal/contracttest` ✓（含新增 `trace_test.go` 行为测试：Init 后落盘、未 Init 零文件）
- 前端 `vue-tsc --noEmit`（结果见交付说明）

**待验证**：Kotlin 改动需 Android SDK / gradle 构建 APK 后真机导出确认；当前改动镜像既有 `service.invoke` 调用模式，风险低但未编译。

**回滚**：删除 `mobileapi/trace.go`、`mobileapi/trace_test.go`，还原 5 处小改动（invoke.go / service.go / probe.go / CfstPlugin.kt / bridge.ts）即可；无协议与契约变化，可随时热回滚。三层各留一个 `BRIDGE_TRACE_ENABLED` 开关（Go 由 Init 门控），置 false 即静默。
