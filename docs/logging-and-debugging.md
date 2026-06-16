# 日志与调试机制说明

本文档基于当前源码整理 CFST-GUI 的日志、调试、诊断包、运行时诊断和 UI 对接机制。目标读者是维护者、排障人员和需要对接 WebUI / Android / Wails bridge 的开发者。

## 结论摘要

- 日志目录统一位于应用数据目录下的 `logs/`。桌面端和 WebUI 使用 `storageRoot()/logs`，Android 使用 `basePath()/logs`。
- 基础运行日志写入 `app-YYYY-MM-DD.jsonl`；运行监控写入 `monitor-YYYY-MM-DD.jsonl` 和 `main-heartbeat.json`；探测调试日志写入 `cfip-log.txt`；兼容错误日志写入 `error-log.txt`。
- `cfip-log.txt` 只在探测配置 `probe.debug=true` 时生成，并且每次开启调试运行会重新创建并截断该文件。
- 结构化日志统一使用 JSON Lines，schema 为 `cfst-log-v1`。调试日志会对 token、password、secret、cookie、authorization 等字段和敏感 URL query 做脱敏。
- 诊断包只包含日志、心跳和 `manifest.json`，不包含配置文件、Cloudflare Token、GitHub PAT 或 WebDAV 凭据。
- WebUI 不直接打开服务端文件管理器；`OpenLogDirectory` 只返回服务端日志路径。Android 私有日志目录也不直接打开，推荐通过导出调试日志或诊断包共享。

## 主要源码位置

| 模块 | 文件 | 职责 |
| --- | --- | --- |
| 调试日志底层 | `internal/utils/debuglog.go` | `ConfigureDebugLog`、`DebugEvent`、脱敏、调试日志行渲染。 |
| 运行日志底层 | `internal/utils/runtime_log.go` | `app-*.jsonl` / `monitor-*.jsonl` 写入、等级过滤、保留期清理。 |
| 心跳文件 | `internal/utils/runtime_monitor.go` | `main-heartbeat.json` 读写、stale 判断。 |
| 运行时清理/诊断 | `internal/runtimecleanup/cleanup.go` | 定期清理、内存/goroutine 统计、诊断开关。 |
| 诊断包 | `internal/appcore/diagnostic_bundle.go` | ZIP 打包日志文件和 manifest。 |
| Trace 诊断 | `internal/appcore/trace_diagnostics.go` | 追踪阶段失败原因统计、样本和摘要。 |
| 桌面探测链路 | `internal/app/app.go` | 探测事件、debug log 生命周期、失败/error log、运行日志配置。 |
| 桌面监控进程 | `internal/app/process_monitor.go`、`internal/app/log_monitor.go` | 主进程心跳和外部 monitor 进程。 |
| 桌面导出接口 | `internal/app/debug_export.go` | `ExportDebugLog`、`ExportDiagnosticBundle`。 |
| WebUI 接口 | `internal/app/webui.go`、`internal/app/app_webui.go` | `/api/app/*` 映射、WebUI 日志目录行为、诊断访问保护。 |
| Android 实现 | `mobileapi/debug_export.go`、`mobileapi/probe.go`、`mobileapi/runtime_cleanup.go`、`mobileapi/runtime_monitor.go` | 移动端调试日志、诊断包、事件、心跳和运行时诊断。 |
| 前端桥接 | `frontend/src/lib/bridge.ts` | Wails / WebUI / Capacitor 三端调用适配。 |
| 前端配置类型 | `frontend/src/lib/bridge/types.ts`、`frontend/src/lib/bridge/config.ts` | 日志与调试字段类型和归一化。 |
| 前端 UI | `frontend/src/views/SettingsView.vue`、`frontend/src/views/DashboardView.vue`、`frontend/src/App.vue` | 设置项、导出按钮、活动/导出历史、前端错误上报。 |

## 数据目录和文件

桌面端默认数据目录来自 Go `os.UserConfigDir()` 加 `CFST-GUI`。如果启用便携模式：

| 触发方式 | 实际数据目录 |
| --- | --- |
| `CFST_GUI_PORTABLE_ROOT=/some/root` | `/some/root/data` |
| 可执行文件同目录存在 `portable.json` | `<exe-dir>/data` |

WebUI Docker 默认设置 `CFST_GUI_PORTABLE_ROOT=/data`，因此容器内应用数据目录是 `/data/data`，日志目录是 `/data/data/logs`。

Android 由 Capacitor / native 层调用 `mobileapi.Service.Init(baseDir)` 初始化；未传入时默认使用 `os.UserConfigDir()/CFST-GUI/mobile`。

日志文件清单：

| 文件 | 写入方 | 内容 | 清理策略 |
| --- | --- | --- | --- |
| `logs/cfip-log.txt` | `utils.ConfigureDebugLog` / `utils.DebugEvent` | 探测调试事件 JSONL。 | 运行日志清理会删除超过保留期且非当前打开的旧文件。 |
| `logs/error-log.txt` | `utils.AppendErrorLog` | panic、前端运行时错误、持久化失败、监控启动失败等错误 JSONL。 | 当前不按日期轮转，也不由保留期清理删除。 |
| `logs/app-YYYY-MM-DD.jsonl` | `utils.AppendRuntimeLog*` | 运行日志，包含 runtime/debug/error 频道事件。 | 按 `logging.retention_days` 清理。 |
| `logs/monitor-YYYY-MM-DD.jsonl` | `utils.AppendMonitorLogWithRetention` | monitor 进程事件、主进程卡死/恢复/退出。 | 按 `logging.retention_days` 清理。 |
| `logs/main-heartbeat.json` | 桌面/Android 心跳循环 | 主进程 PID、状态、启动时间、最后更新时间、日志目录。 | 诊断包会收集；不按日期清理。 |

## 配置字段

配置快照中日志相关字段位于 `logging`：

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 是否写入基础运行日志 `app-YYYY-MM-DD.jsonl`。 |
| `level` | `error` | 运行日志等级，支持 `error`、`warn`、`info`、`debug`。等级越高写入越多。未知值会降级为 `error`。 |
| `monitor_enabled` | `true` | 是否启用运行监控。桌面端会启动 monitor 进程；Android 只写主进程心跳。 |
| `retention_days` | `7` | `app-*`、`monitor-*` 和旧 `cfip-log.txt` 的保留天数，范围 1 到 365。 |
| `durability` | `split` | 当前唯一有效策略。`error` / `warn` 等级运行日志会同步刷盘，`info` / `debug` 不强制同步。 |

探测调试相关字段位于 `probe`：

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `debug` | `false` | 是否开启探测调试日志。关闭时不创建 `cfip-log.txt`。 |
| `debug_log_mode` | `structured` | 当前固定为结构化 JSONL。旧 `freeform` 或未知值会被归一化为 `structured`。 |
| `debug_log_format` | 空 | 当前结构化模式下忽略。 |
| `debug_log_verbosity` | `detailed` | `detailed` 写入全部调试事件；`simple` 只保留关键事件。 |
| `debug_capture_enabled` | `false` | 是否启用请求捕获地址。只有 `debug=true` 且地址非空时生效。 |
| `debug_capture_address` | 空 | 调试捕获监听地址，例如 `127.0.0.1:8080` 或 `8080`。生效时后端拨号目标会被覆盖。 |

`simple` 调试粒度只写入这些事件：

- `probe.start`
- `stage.complete`
- `probe.export`
- `probe.complete`
- `probe.failed`

## 调试日志机制

`ConfigureDebugLog(enabled, path, mode, format, verbosity)` 是调试日志入口。

开启时的行为：

1. 关闭旧调试日志文件。
2. 创建日志目录。
3. 以 `O_CREATE|O_WRONLY|O_TRUNC` 打开 `cfip-log.txt`，权限 `0600`。
4. 将 Go 标准库 `log` 输出切到调试日志控制台输出。
5. `DebugEvent` 每次写一行 JSON，并同步追加到运行日志系统，是否落到 `app-*.jsonl` 取决于 `logging.enabled` 和 `logging.level`。

关闭或任务结束时：

1. `CloseDebugLog` 关闭文件。
2. 清空当前 task id。
3. 调试输出回到 `io.Discard`。

每条调试日志的基础结构：

```json
{
  "schema_version": "cfst-log-v1",
  "channel": "debug",
  "level": "info",
  "event": "probe.start",
  "ts": "2026-06-16T08:00:00.000000000+08:00",
  "task_id": "cfst-...",
  "message": "探测任务启动。",
  "data": {}
}
```

顶层保留字段包括 `schema_version`、`channel`、`level`、`event`、`ts`、`task_id`、`stage`、`message`。其他字段会放入 `data`。

敏感信息脱敏规则：

- 字段名包含 `api_token`、`authorization`、`cookie`、`password`、`secret`、`set_cookie` 会写成 `<redacted>`。
- 字段名为 `token`、以 `_token` 结尾或以 `token_` 开头会写成 `<redacted>`。
- 字符串中形如 bearer/token 的长值会被替换。
- URL query 中包含 `token`、`secret`、`password`、`authorization`、`auth`、`signature`、`api_key`、`apikey` 会被替换。

## 运行日志机制

`ConfigureRuntimeLog(enabled, directory, level, retentionDays, durability)` 在加载或保存配置时调用。

运行日志写入规则：

- `AppendRuntimeLog` 使用当前配置，按等级过滤写入 `app-YYYY-MM-DD.jsonl`。
- `AppendRuntimeLogAlways` 只检查 `enabled` 和目录，常用于 shutdown 这类必须记录的 runtime 事件。
- `AppendErrorLog` 写入 `error-log.txt`，并尝试把同一事件追加到运行日志系统。
- `AppendMonitorLogWithRetention` 写入 `monitor-YYYY-MM-DD.jsonl`，并在写入前做保留期清理。

等级过滤从高优先级到低优先级为：

| 配置等级 | 会写入的事件等级 |
| --- | --- |
| `error` | `error` |
| `warn` | `error`、`warn` |
| `info` | `error`、`warn`、`info` |
| `debug` | `error`、`warn`、`info`、`debug` |

保留期清理覆盖：

- `app-*.jsonl`
- `monitor-*.jsonl`
- 超过保留期且当前未打开的 `cfip-log.txt`

当前不清理 `error-log.txt` 和 `main-heartbeat.json`。

## 运行监控和心跳

桌面端启用 `logging.monitor_enabled=true` 后，会启动两部分机制：

1. 主进程每 2 秒写一次 `main-heartbeat.json`。
2. 启动同一可执行文件的子进程并传入 `--log-monitor` 参数，作为外部 monitor。

monitor 参数包括：

```text
--log-monitor
--parent-pid <pid>
--log-dir <logs>
--heartbeat <logs/main-heartbeat.json>
--retention-days <days>
--stale-after 10s
```

monitor 事件：

| 事件 | 触发条件 |
| --- | --- |
| `monitor.started` | monitor 启动。 |
| `main.heartbeat_unavailable` | 读取心跳失败。 |
| `main.hung` | 主进程仍存在，但心跳超过 `stale-after` 未更新。 |
| `main.recovered` | 之前判定 hung，随后心跳恢复。 |
| `main.exited` | 主进程退出且心跳状态为 `shutdown`。 |
| `main.crashed_or_killed` | 主进程不存在，且没有正常 shutdown 心跳。 |
| `main.shutdown` | 桌面主进程关闭时写入运行日志。 |

可用 `CFST_DISABLE_PROCESS_MONITOR=1` 禁用桌面 monitor 进程。测试二进制默认不会启动 monitor。

Android 当前只写 `main-heartbeat.json`，不启动外部 monitor 进程。

## 运行时清理和诊断

桌面端和 Android 都会启动 `runtimecleanup.Cleaner`。

默认行为：

- 周期：每 8 小时运行一次。
- 任务结束后：延迟 30 秒触发轻量清理。
- 重清理：每 8 小时限频执行一次，运行中任务或 scheduler 活跃时跳过。
- 轻量清理：关闭 idle 连接、清理 H3 failure cache、清理过期日志、裁剪内存中的终态 task snapshot。
- 重清理：执行 Go GC 和 `debug.FreeOSMemory()`。

运行时诊断默认关闭。读取诊断需要设置：

| 变量 | 说明 |
| --- | --- |
| `CFST_RUNTIME_DIAGNOSTICS=1` | 允许读取 goroutine、内存、清理次数、task snapshot 数等状态。 |
| `CFST_RUNTIME_DIAGNOSTICS_REMOTE=1` | WebUI 允许非本机访问运行时诊断。还必须配置 `CFST_WEBUI_TOKEN`。 |

WebUI 的 `GetRuntimeStatus` 有访问保护：

- 本机回环请求可以读取。
- 远程请求需要 `CFST_RUNTIME_DIAGNOSTICS_REMOTE=1` 且 `CFST_WEBUI_TOKEN` 非空。
- 不满足时返回 `RUNTIME_DIAGNOSTICS_LOCAL_ONLY`。

## Trace 诊断

追踪阶段会通过 `TraceDiagnosticsCollector` 记录失败原因、HTTP 状态码和最多 3 条样本。事件 payload 字段为 `trace_diagnostics`。

结构：

```json
{
  "reason_counts": {
    "rate_limited": 2
  },
  "status_counts": {
    "429": 2
  },
  "samples": [
    {
      "ip": "203.0.113.1",
      "reason": "rate_limited",
      "status_code": 429,
      "url": "https://example.com/cdn-cgi/trace"
    }
  ],
  "trace_colo_mode": "trace_url",
  "trace_url": "https://example.com/cdn-cgi/trace"
}
```

常见 reason 显示名：

| reason | UI/摘要含义 |
| --- | --- |
| `colo_filter` | 地区码不匹配 |
| `rate_limited` | 服务端限流 |
| `request_create_failed` | 追踪请求创建失败 |
| `source_colo_filter` | 输入源 COLO 过滤未通过 |
| `status_mismatch` | 状态码不匹配 |
| `trace_error` | 追踪请求失败 |
| `trace_latency_limit` | 追踪延迟超阈值 |
| `trace_read_error` | 追踪响应读取失败 |

如果追踪阶段失败且没有后续下载结果，后端会把失败阶段标为 `stage2_trace`，并生成“追踪阶段失败：...”摘要。前端会在任务失败和导出历史中展示该摘要。

## 诊断包

`BuildDiagnosticBundle(logDir, platform, now, requestedName)` 会生成 ZIP。

包含文件：

- `manifest.json`
- `logs/cfip-log.txt`
- `logs/error-log.txt`
- `logs/main-heartbeat.json`
- `logs/app-*.jsonl`
- `logs/monitor-*.jsonl`

`manifest.json` 字段：

| 字段 | 说明 |
| --- | --- |
| `generated_at` | 生成时间。 |
| `included` | 成功打包的文件列表。 |
| `missing` | 期望但不存在的文件或 pattern。 |
| `log_dir` | 来源日志目录。 |
| `platform` | 平台，如 `windows/amd64`、`linux/amd64`、`darwin/arm64`、`android`。 |

没有任何可读日志时返回 `DIAGNOSTIC_BUNDLE_EMPTY`。这通常意味着还没运行任务、没开启日志或日志目录为空。

## 桥接和接口

### Wails 桌面接口

前端通过 `window.go.app.App` 调用 Go 方法。

| 方法 | 说明 | 典型返回 code |
| --- | --- | --- |
| `ExportDebugLog(payload)` | 读取 `cfip-log.txt` 并导出到目标路径，或返回 base64。 | `DEBUG_LOG_EXPORT_OK`、`DEBUG_LOG_EXPORT_NOT_FOUND` |
| `ExportDiagnosticBundle(payload)` | 构建并导出诊断 ZIP，或返回 base64。 | `DIAGNOSTIC_BUNDLE_EXPORT_OK`、`DIAGNOSTIC_BUNDLE_EMPTY` |
| `OpenLogDirectory(payload)` | 创建并打开日志目录。 | `LOG_DIRECTORY_OPENED` |
| `RecordFrontendRuntimeError(payload)` | 将前端错误写入 `error-log.txt` 和运行日志。 | `FRONTEND_RUNTIME_ERROR_LOGGED` |
| `GetRuntimeStatus()` | 读取运行时诊断状态。 | `RUNTIME_STATUS_READY`、`RUNTIME_DIAGNOSTICS_DISABLED` |

`ExportDebugLog` / `ExportDiagnosticBundle` 支持的常见 payload：

| 字段 | 说明 |
| --- | --- |
| `target_path` / `targetPath` / `path` | 直接写入本地文件。 |
| `target_dir` / `targetDir` | 未传目标文件时，拼接文件名写入该目录。 |
| `target_uri` / `targetUri` / `uri` | 不写本地文件，返回 `content_base64` 给前端或移动端保存。 |
| `file_name` / `fileName` / `default_file_name` / `defaultFileName` | 建议文件名。 |
| `config` / `config_snapshot` / `configSnapshot` | 用于读取导出目录配置。 |

### WebUI HTTP 接口

WebUI 入口在 `internal/app/webui.go`：

| HTTP 路径 | 鉴权 | 说明 |
| --- | --- | --- |
| `GET /api/health` | 否 | 健康检查，返回 `ok`、`service`、`version`、`auth_required`。 |
| `POST /api/app/{method}` | 是 | 调用桌面同名能力，例如 `ExportDebugLog`、`ExportDiagnosticBundle`、`OpenLogDirectory`、`GetRuntimeStatus`。 |
| `GET /api/events/probe` | 是 | 探测事件流。 |
| `GET /api/files/list` | 是 | 服务端文件列表。 |
| `GET /api/files/download` | 是 | 服务端文件下载。 |

`CFST_WEBUI_TOKEN` 非空时，`/api/app/*`、事件和文件接口要求 `Authorization: Bearer <token>` 或 `?token=<token>`。

WebUI 差异：

- `OpenLogDirectory` 返回 `LOG_DIRECTORY_WEBUI`，只告诉前端服务端日志目录，不尝试打开文件管理器。
- `ExportDebugLog` 如果 payload 有 `target_uri`，后端返回 `content_base64`；前端用浏览器下载。
- `ExportDiagnosticBundle` 在 WebUI 下前端会自动传 `target_uri="__cfst_browser_download__"`，后端返回 base64 后由浏览器下载。

### Android / Capacitor 接口

Android 前端通过 Capacitor plugin 调用 `mobileapi.Service`。

| 方法 | 说明 | 差异 |
| --- | --- | --- |
| `ExportDebugLog(payloadJSON)` | 导出移动端 `logs/cfip-log.txt`。 | 需要 `target_uri` 或可写目标路径/目录；推荐 SAF。 |
| `ExportDiagnosticBundle(payloadJSON)` | 导出移动端诊断 ZIP。 | 返回中包含 `content_base64`，便于 native/前端写入 SAF。 |
| `OpenLogDirectory(payloadJSON)` | 返回日志目录路径。 | code 为 `LOG_DIRECTORY_ANDROID_UNAVAILABLE`，提示使用导出日志。 |
| `RuntimeStatus()` | 读取运行时诊断。 | 仍受 `CFST_RUNTIME_DIAGNOSTICS` 控制。 |

Android UI 在导出调试日志或诊断包前，如果没有 `settings.exportTargetUri`，会提示先选择 Android SAF 导出目录。

### CLI 参数

CLI 兼容调试参数：

| 参数 | 说明 |
| --- | --- |
| `--debug` | 开启调试输出，写入 `logs/cfip-log.txt`。 |
| `--debug-capture <addr>` | 调试模式下将实际拨号目标改为本地监听地址/端口。 |

CLI 当前调用 `utils.ConfigureDebugLog(utils.Debug, debugLogFilePath())`，使用默认结构化、详细粒度。

## 探测事件和 UI 对接

后端探测事件会通过 Wails event、WebUI SSE 或 Android native event 进入前端，前端统一归一化为 `desktop:probe` 事件。

与日志/调试有关的 payload 字段：

| 字段 | 说明 |
| --- | --- |
| `debug_log_path` / `debugLogPath` | 调试日志路径。只有开启 `probe.debug` 时通常存在。 |
| `log_uri` / `logUri` | 可打开或可导出的日志目标。 |
| `trace_diagnostics` / `traceDiagnostics` | 追踪阶段诊断。 |
| `failure_stage` | 失败阶段，例如 `stage2_trace`。 |
| `target_path` | CSV 或系统导出目标。 |
| `warnings` | 配置、日志初始化、导出等警告。 |

前端处理点：

- `App.vue` 的 `eventDebugLogDisplayPath` 从事件 payload 提取日志路径。
- `eventDebugLogOpenTarget` 优先使用 `log_uri`，否则使用 `debug_log_path`。
- `eventTraceFailureSummary` 调用 `summarizeTraceDiagnostics` 生成失败摘要。
- `probe.partial_export`、`probe.export_completed`、`probe.export_failed`、`probe.completed`、`probe.failed` 会更新导出历史中的 `debugLogPath` / `debugLogTarget`。
- `DashboardView.vue` 在“最近导出”中显示 `LOG <path>`，并在有日志路径时显示“打开日志”按钮。

前端还有两类非落盘状态：

| 状态 | 位置 | 说明 |
| --- | --- | --- |
| `logs` | `App.vue` | 内存数组，只保留最近 160 条 bridge/UI 事件，用于开发和状态追踪，不会写入磁盘。 |
| `activityFeed` | `App.vue` / `DashboardView.vue` | UI 活动流，只保留最近 10 条关键状态变化。 |

## 设置页对接

`SettingsView.vue` 的“调试”折叠区包含日志和调试两组能力。

日志区域：

- “导出诊断包”触发 `export-diagnostic-bundle`。
- “导出调试日志”触发 `export-debug-log`。
- “打开日志目录”触发 `open-log-directory`。
- “基础运行日志”绑定 `settings.loggingEnabled`。
- “日志等级”绑定 `settings.loggingLevel`。
- “保留天数”绑定 `settings.loggingRetentionDays`。
- “运行监控”绑定 `settings.loggingMonitorEnabled`。
- “分级同步”展示 `settings.loggingDurability`，当前总是 `split`。

调试区域：

- “临时开启调试”绑定 `settings.probeDebug`。
- “记录粒度”绑定 `settings.probeDebugLogVerbosity`，可选 `simple` / `detailed`。
- “高级：抓包监听地址”绑定 `settings.probeDebugCaptureEnabled`。
- 地址输入框绑定 `settings.probeDebugCaptureAddress`。

`App.vue` 中的配置映射：

- `applyConfigSnapshot` 从 `normalized.logging` 和 `normalized.probe` 回填 UI 状态。
- `buildConfigSnapshot` 将 UI 状态写回 `logging` 和 `probe`。
- `debug_log_mode` 保存时固定为 `structured`。
- `debug_log_format` 保存时固定为空字符串。

## 前端运行时错误上报

前端在挂载时注册：

- `window.error`
- `window.unhandledrejection`

错误会进入 `reportFrontendRuntimeError`，构造 payload：

- `source`
- `message`
- `stack`
- `filename`、`lineno`、`colno` 等上下文

桌面端通过 `RecordFrontendRuntimeError` 写入 `error-log.txt`，并返回：

- `log_path`: `logs/error-log.txt`
- `app_log_path`: 当天 `logs/app-YYYY-MM-DD.jsonl`

桥接事件监听器自身抛错时，`bridge.ts` 也会调用 `recordFrontendRuntimeError`，source 为 `probe-event-listener`。

当前 `recordFrontendRuntimeError` 只走 Wails bridge；WebUI 或 Android 没有可用 Wails bridge 时会返回 `FRONTEND_RUNTIME_ERROR_LOG_SKIPPED`。

## 常见排障说明

| 现象 | 原因或处理 |
| --- | --- |
| 导出调试日志提示不存在 | 需要先开启“临时开启调试”并运行一次任务；`cfip-log.txt` 不会在普通运行中生成。 |
| `cfip-log.txt` 只有最近一次任务 | 调试日志打开方式是截断写入；这是当前实现。需要长期留存时应及时导出诊断包或调试日志。 |
| WebUI 点“打开日志目录”没有打开文件管理器 | WebUI 运行在服务端，只返回服务端路径；需要通过 Docker volume、SSH 或导出功能读取。 |
| Android 点“打开日志目录”只显示提示 | Android 私有目录不能直接打开；需要先选择 SAF 导出目录，再导出调试日志或诊断包。 |
| 运行时诊断显示未启用 | 需要设置 `CFST_RUNTIME_DIAGNOSTICS=1` 后重启对应运行形态。 |
| 远程 WebUI 读取运行时诊断失败 | 除 `CFST_RUNTIME_DIAGNOSTICS=1` 外，还需要 `CFST_RUNTIME_DIAGNOSTICS_REMOTE=1` 和非空 `CFST_WEBUI_TOKEN`。 |
| 设置 `debug_log_mode=freeform` 没有效果 | freeform 已停用，前后端都会归一化为 `structured`。 |
| 日志等级设为 `error` 时 app 日志很少 | 等级过滤只写 error；调到 `warn`、`info` 或 `debug` 才会写更多运行事件。 |
| `error-log.txt` 持续增长 | 当前保留期清理不处理 `error-log.txt`；需要人工导出后清理或后续增加轮转策略。 |

## 维护注意事项

- 不要把配置归档和诊断包混淆。配置归档包含敏感凭据，诊断包当前只包含日志和 manifest。
- 新增日志字段时优先使用结构化字段，让 `runtimeLogEntry` 放入 `data`；只有 `message`、`task_id`、`stage` 这类高频检索字段应放顶层。
- 新增可能包含凭据的字段名时，应同步检查 `isSensitiveDebugKey` 和 `isSensitiveDebugQueryKey`。
- WebUI 新增诊断类接口时要复用本机/远程访问保护，不要绕过 `CFST_RUNTIME_DIAGNOSTICS_REMOTE` 和 token 约束。
- Android 导出路径要优先考虑 SAF URI，不要假设 app 私有路径能被系统文件管理器直接打开。
