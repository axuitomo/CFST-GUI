# 桥接追踪埋点 · DDD 设计方案（方案 B）

> 状态：已完成 Go 回归和 Kotlin 编译、单元测试；Android 真机 JNI 端到端与耗时尚未验证。
> 范围：诊断元数据与追踪去重，不改变业务命令返回值或事件正文。

## 1. 背景与问题域

**症状**：Android 桥接跨越 Vue、Kotlin 和 Go，重复记录入口、返回值和追踪命令本身会放大同步写盘；原始配置、URL 和错误消息也可能泄露凭据。

**目标**：把调用结果及必要失败元数据汇聚到单一 `bridge-debug.log`，保留诊断可用性与同步持久化，避免记录业务正文。文件可通过 adb 或 Android Studio Device Explorer 读取，见 [Android 调试](../mobile/android-debug.md)。

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
| 前端适配层 | `packages/frontend/src/lib/adapters/android/adapter.ts` | `ensureNativeBridge` 失败 | `bridge.init_failed` |
| 原生插件层 | `CfstPlugin.kt` | `rejectWithLog` | `invoke.reject` |
| 核心服务层 | `mobileapi/invoke.go`、`probe.go`、`service.go` | `Service.Invoke`、`deliverProbeEvent`、`Init` | `invoke.out` / `event.out` / `init` |
| 观测域（横切支撑） | `mobileapi/trace.go` | `Service.trace` → 文件 | 汇聚全部上游事件 |

普通 Go 调用只写一条 `invoke.out`，前端与 Kotlin 不再重复记录成功路径。`bridge.trace` 不追踪自身；Kotlin 也不对该命令的失败再次发起追踪。

## 4. 领域模型

- **实体 BridgeTraceEvent**：固定 `event`、`level`、`ts`，附加允许的元数据，不记录 payload、result、message、error、配置或任意自定义字段。
- **值对象 TracePolicy**：字符串字段仅允许 `source`、`command`、`code`、`task_id`、`source_event`、`action`、`error_type`；值和事件名限制为 128 字节内的 ASCII 字母、数字与 `_.:-()`。另允许布尔 `ok` 和数值 `seq`、`duration_ms`。
- **聚合 TraceLog**：单文件 JSONL，由 TraceSink 维护追加、轮转与互斥；单文件 4MB，最多保留 3 个归档。
- **领域服务 TraceRecorder**：`record(event, fields)`，在唯一 Go sink 校验允许字段，再做整行脱敏。
- **基础设施 FileTraceSink**（Go，唯一写文件者）；前端不再把原始追踪数据输出到 `console.debug`，避免开发者工具泄露敏感字段。

## 5. 关键不变量（领域规则）

1. **弱失败**：所有写日志失败被吞掉；Kotlin `try/catch`、前端 `.catch(() => {})`、Go 吞写错误，主链路不受影响。
2. **单通道汇聚**：只有 Go 写文件；Kotlin 与前端经 `bridge.trace` 命令汇入，避免多写者竞争和三套轮转/脱敏实现。
3. **门控**：Go 侧仅 `Init` 成功后落盘（contract/单测不调 Init 则零文件，测试机无副作用）；Kotlin 侧要求 `service` 已初始化。
4. **脱敏**：Go sink 先限制元数据字段，再整行经 `utils.RedactSensitiveText`（Bearer Token、Telegram Bot Token、URL query 敏感键）。
5. **有界资源**：单文件 4MB、3 个轮转归档，字符串元数据有界；普通调用去重，但每条保留日志仍同步写盘，高频事件仍有 IO 成本。
6. **强制落盘**：文件写入先刷新用户态缓冲，再调用 `File.Sync()`；关闭和轮转前同样同步，尽量保证诊断记录在进程异常退出前已提交到存储设备。
7. **结果可还原**：`invoke.out` 带 `command`、`code`、`ok`、`duration_ms` 和可用的 `task_id`；`event.out` 带 `source_event`、`task_id`、`seq`。不再用重复入口日志推断未完成调用。

## 6. 追踪示例（普通查询与独立探测事件）

```text
Go     invoke.out {command:"task.get", code:"TASK_SNAPSHOT", ok:true, duration_ms:1, task_id:"cfst-123"}
Go     event.out  {source_event:"probe.progress", task_id:"cfst-123", seq:42}
```

失败时可能额外出现 `Kotlin invoke.reject {action:"Invoke(probe.start)", error_type:"IllegalStateException"}` 或前端 `bridge.init_failed`。失败详情通过业务返回值传递，日志仅保留定位所需的标识符与结果。初始化必须明确返回布尔 `ok:true`，失败不能缓存为已初始化。

## 7. 决策记录（ADR）

| 决策 | 选择 | 理由 |
| --- | --- | --- |
| 落盘者 | Go 单通道，而非三层各自写文件 | 共享复用：一套轮转/脱敏/互斥；避免 JNI/JS 多写者竞争 |
| 跨层透传 | 新增 `bridge.trace` 命令，而非新 JNI 导出方法 | 不改 gomobile 绑定 API，无需重生成绑定，契约零变化 |
| 埋点位置 | `mobileapi`（平台适配层）而非 `appcore` | 观测属平台诊断关注点，保持架构边界；appcore 逻辑零改动 |
| 门控方式 | `Init` 成功后启用（baseDir 非空） | 天然隔离测试与生产；Android 端 Init 恒被调用，功能完整 |

## 8. 验证与回滚

**已完成**：

- `mobileapi` 回归覆盖：Init 后落盘、未 Init 不写文件、普通调用一条结果日志、追踪命令无自追踪、假凭据与 URL 不落盘、事件正文不变。
- Kotlin 编译与初始化、命令归一化、存储迁移单测共 15 项通过。
- 前端 `bridgeAudit.test.ts` 覆盖初始化失败重试、并发共享、查询字段投影和令牌刷新后的 SSE 生命周期。

**待验证**：重新生成 AAR、构建 APK 后的真机 JNI 链路、诊断读取及实际耗时。日志条数减少不能直接等同于耗时改善。

**回滚**：恢复对应平台适配层与测试的既有实现即可，无持久化数据迁移。前端与 Kotlin 保留 `BRIDGE_TRACE_ENABLED`；Go 由 Init 门控，默认诊断与同步持久化保持启用。
