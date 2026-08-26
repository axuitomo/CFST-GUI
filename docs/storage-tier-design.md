---
AIGC:
    Label: "1"
    ContentProducer: 001191440300708461136T1XGW3
    ProduceID: e38ce0dc5ac905b02932fdc2582ed1e7_e3838045a07811f1a238525400e6dd8f
    ReservedCode1: P65yloG4eHm80VcGv0qRyiJTLE/pdDZsU1yaeYD7wk+Q8bsq/9jp3I/Gb4dSDrpJ5TlCmm+W+ScND5P3mboW1iI0mufOsQQOyCnyF/wk3V7wiFe9XDcnbki4AkWOZutY5L/1TOjUuwd1FSn8nhFlCJQ+jNC7j8ZyKJ0KuOz8XyKmg2Mq06FkfVr5YkI=
    ContentPropagator: 001191440300708461136T1XGW3
    PropagateID: e38ce0dc5ac905b02932fdc2582ed1e7_e3838045a07811f1a238525400e6dd8f
    ReservedCode2: P65yloG4eHm80VcGv0qRyiJTLE/pdDZsU1yaeYD7wk+Q8bsq/9jp3I/Gb4dSDrpJ5TlCmm+W+ScND5P3mboW1iI0mufOsQQOyCnyF/wk3V7wiFe9XDcnbki4AkWOZutY5L/1TOjUuwd1FSn8nhFlCJQ+jNC7j8ZyKJ0KuOz8XyKmg2Mq06FkfVr5YkI=
---

# 分层存储设计方案（Storage Tier Design）

> 状态：设计方案（待评审）
> 适用范围：CFST-GUI 全部三端（桌面 / WebUI / Android / CLI）
> 关联约束：[architecture-constraints.md](./architecture-constraints.md)、[behavior-baseline.md](./behavior-baseline.md)
> 契约 schema：`cfst-gui-command-v2` / `cfst-gui-config-v2` / `cfst-gui-event-v2`

---

## 目录

1. [现状分析](#1-现状分析)
2. [分层存储目标架构](#2-分层存储目标架构)
3. [日志可被其他应用读取的机制设计](#3-日志可被其他应用读取的机制设计)
4. [维护性设计](#4-维护性设计)
5. [性能设计](#5-性能设计)
6. [分阶段实施计划](#6-分阶段实施计划)
7. [与现有命令契约的兼容策略](#7-与现有命令契约的兼容策略)
8. [风险与回退](#8-风险与回退)

---

## 1. 现状分析

### 1.1 现有目录布局

当前存储根目录由 `internal/appcore.StorageLayout` 与 `internal/app`（`storageRoot()` / `portableDataDir()` / `defaultStorageDir()`）共同定义。Windows 默认 `%APPDATA%\CFST-GUI`，便携模式回退到程序目录 `data`；`storage.set` 命令当前已固定目录、不再支持自定义。

| 目录 / 文件 | 定位函数 | 内容 | 写入方式与权限 |
| --- | --- | --- | --- |
| `Root/desktop-config.json`（GUI）/ `config.json`（CLI） | `ConfigPath()` | 配置快照 `config_snapshot` + `saved_at` + `schema_version` | `WriteFileAtomic` 0o600；首次保存写入 v2，旧版留 `.v1.bak` |
| `Root/desktop-draft.json` | `DraftPath()` | 草稿配置 | `WriteFileAtomic` 0o600 |
| `Root/scheduler-status.json` | `SchedulerPath()` | 调度状态 | 原子写 0o600 |
| `Root/<source-profiles-file>` | `SourceProfilesPath()` | 输入源档案 | 原子写 0o600 |
| `Root/tasks/` | `TasksRoot()` | 任务快照 `{id}.json` 与结果 `{id}-results.json`（平铺） | `TaskStore` 原子写 0o600；`MaxTaskResultsBytes` 限制单文件大小 |
| `Root/logs/` | `LogsRoot()` | `cfip-log.txt`、`error-log.txt` | `utils.DebugLogger` `O_CREATE\|O_WRONLY\|O_TRUNC` 0o600，JSONL |
| `Root/exports/` | `ExportsRoot()` | `results.export_csv` 输出 | `WriteFileAtomic` 0o644 |
| `Root/backups/` | `BackupsRoot()` | `cfst-gui-pre-import-*.zip`、`config-*.json` 本地备份 | `os.WriteFile` 0o600 / `WriteFileAtomic` 0o600 |
| 结果 CSV（探测） | `currentServiceOutputFile` | 默认 `result.csv`（当前工作目录）；可配 `outputFile` | `utils.CSVWriter` `ExportContext` 0o644，支持 Append/Trunc、UTF-8 BOM |
| 结果 CSV（GitHub） | `githubcore.DefaultPathTemplate` | `cfst-results/{date}/{time}-{task_id}.csv` | 同上 0o644 |

### 1.2 存储相关命令

命令路由集中在 `internal/appcore/invoke.go`，payload 为 snake_case，envelope 固定 `cfst-gui-command-v2`。

| 命令域 | 命令 | 现状 | 说明 |
| --- | --- | --- | --- |
| 存储 | `storage.set` | 已废弃（返回 `STORAGE_SET_DEPRECATED`，固定目录） | 保留命令名以兼容旧调用方 |
| 存储 | `storage.health` | 健康检查（目录可用性） | 返回 `health` + `storage` |
| 配置 | `config.export` | 导出完整配置 JSON | 0o600，含敏感 Token 告警 |
| 配置 | `config.backup` | 备份到 `Root/backups/config-{ts}.json` | 0o600 |
| 归档 | `archive.export` / `archive.import` | 配置压缩包导出/导入 | 导出 0o600；导入前写 `pre-import` 本地备份并支持回滚 |
| 归档 | `webdav.test` / `webdav.backup` / `webdav.restore` | WebDAV 远端归档 | 远端 PUT/GET |
| 日志 | `debug.export` | 导出 `logs/cfip-log.txt`（脱敏后）为 txt | 0o644 或 base64/uri |
| 日志 | `diagnostics.export` | 打包 zip：logs、status、config-summary、最近 20 个终态任务快照 | 0o644 或 base64/uri |
| 结果 | `results.export_csv` | 写 `exports/` 或指定 `target_path` | 0o644 |
| 结果 | `github.export` | 按模板写 `cfst-results/...` 或远端 | 0o644 |

### 1.3 日志导出现状

- **写入路径**：`logs/cfip-log.txt` 由 `DebugLogger.Configure()` 以 `O_CREATE|O_WRONLY|O_TRUNC` 打开（**每次配置都会截断覆盖**），权限 0o600；错误日志 `logs/error-log.txt` 独立文件。
- **格式**：`DebugLogModeStructured`（默认）下每行 `json.Marshal` 输出一个 JSON 对象（JSONL），字段含 `event`/`level`/`ts`/`task_id` 及自定义字段；另有 freeform 文本模式（`{ts} [{level}] {event} ...`）。
- **导出链路**：
  - `debug.export`：读 `cfip-log.txt` → `utils.RedactSensitiveText` 脱敏 → 写 txt（`target_path`/`target_dir`/`DefaultExportDir`）或 `content_base64`（`target_uri`）。
  - `diagnostics.export`：`BuildDiagnosticPackage()` 组装 zip，包含 `logs/cfip-log.txt`、`logs/error-log.txt`、`status/scheduler.json`、`status/runtime.json`、`config/config-summary.json`、`tasks/` 最近 20 个终态快照。
- **权限现状**：日志文件 0o600（Windows 上即仅当前用户进程可读写，**其他应用/服务读取会被拒绝**）；导出产物 0o644 仅解决"导出后"的可读，不解决"运行中可被读取"。

### 1.4 可维护性与性能问题点

| # | 问题 | 影响 | 归类 |
| --- | --- | --- | --- |
| P1 | `cfip-log.txt` 每次 `Configure` 以 `O_TRUNC` 打开，运行期被截断覆盖 | 历史日志丢失，无法审计 | 可维护性 |
| P2 | 日志无轮转（size/time），单文件无限增长 | 磁盘膨胀、导出/读取变慢 | 性能 / 维护性 |
| P3 | 日志权限 0o600，`cfip-log.txt` 固定 `.txt` 名但实为 JSONL | 其他应用无法读取，格式误导 | 可读性 |
| P4 | 探测结果默认写当前工作目录 `result.csv`，位置不稳定；`exports/`、`backups/`、`tasks/` 均无保留/清理策略 | 目录混乱、磁盘占用无上限 | 维护性 |
| P5 | `tasks/` 任务快照平铺存放，`task.list` 需全目录扫描；诊断 zip 遍历全部任务目录 | 任务量增大后列表与打包变慢 | 性能 |
| P6 | 权限不一致：config/tasks 0o600、CSV 0o644、诊断导出 0o644、配置备份 0o600 | 规则不统一，难解释难维护 | 维护性 |
| P7 | 无统一"写入 → 归档 → 导出"能力分层，各处直接 `os.WriteFile`/`MkdirAll` | 路径、权限、原子性逻辑散落 | 维护性 |
| P8 | GitHub 导出模板 `cfst-results/{date}/...` 只按日建目录，无清理 | 长期运行磁盘增长 | 性能 |
| P9 | 日志/结果写入未做批量刷盘，热点路径（探测循环内 `Debugf`）逐行同步写 | 高频写放大，IO 压力 | 性能 |
| P10 | 无配置化入口（`storage.set` 已废弃），保留策略、日志等级、导出默认目录不可调 | 无法按环境调优 | 维护性 |

---

## 2. 分层存储目标架构

### 2.1 数据分层（热 / 温 / 冷）

| 层 | 定义 | 典型数据 | 访问频率 | 存储介质 | 保留目标 |
| --- | --- | --- | --- | --- | --- |
| 热层（Hot） | 应用运行必需、须低延迟访问 | `config.json`、`draft.json`、`scheduler-status.json`、运行中任务状态、当前日志活动文件 | 高 | 本地主目录 | 不清理 / 按版本保留 |
| 温层（Warm） | 可被检索、导出、诊断使用 | `tasks/{id}.json`、`tasks/{id}-results.json`、`logs/app/*.jsonl`（近期）、`exports/` 结果 CSV | 中 | 本地主目录 | 按天数保留（默认 90d） |
| 冷层（Cold） | 归档 / 备份，极少直接访问 | `backups/`、`archive/` 压缩包、轮转后的 `.jsonl.gz`、GitHub 历史结果 | 低 | 本地 `archive/` + WebDAV 远端 | 按策略保留（默认 365d） |

分层原则：

- **热层内容最小化**：仅当前状态文件；历史一律下沉温/冷层，保证 `config.load`、`storage.health` 等高频只读操作只触碰固定少量文件。
- **温层结构索引化**：任务快照按日期子目录组织（见 2.2），保证 `task.list` / `task.results` 可用目录前缀裁剪，避免全目录扫描。
- **冷层一律压缩**：归档统一 `.zip`（沿用 `archivecore`），轮转日志统一 `.jsonl.gz`，降低磁盘占用。

### 2.2 目录分层

保持 `StorageLayout` 顶层结构（`Root` / `config` 文件名 / `tasks` / `logs` / `exports` / `backups`）不变以兼容现有调用方，仅在子目录内细化：

```
Root/
├─ desktop-config.json          # 热：当前配置
├─ desktop-draft.json           # 热：草稿
├─ scheduler-status.json        # 热：调度状态
├─ source-profiles.json         # 热：输入源档案
├─ tasks/                       # 温：任务快照与结果
│  ├─ 2026/08/                  # 按 yyyy/MM 分桶（新增）
│  │  ├─ {task_id}.json
│  │  └─ {task_id}-results.json
│  └─ index.jsonl               # 增量索引（新增，见 §5.3）
├─ logs/                        # 温：运行日志
│  ├─ app/                      # 应用级日志（新增）
│  │  ├─ cfip-log.jsonl         # 热文件（轮转中）
│  │  └─ cfip-log.20260825-120000.jsonl.gz
│  ├─ probe/                    # 每次探测的独立日志（新增，可选）
│  │  └─ {task_id}.jsonl
│  └─ error-log.txt             # 兼容保留：错误日志（现有）
├─ exports/                     # 温：结果导出（现有）
│  └─ 2026/08/                  # 按日期分桶（新增）
├─ backups/                     # 冷：本地备份（现有）
│  └─ config-20260825-120000.json
├─ archive/                     # 冷：归档与轮转压缩（新增）
│  ├─ diagnostics/              # diagnostics.export 落地
│  └─ logs/                     # 轮转后的日志压缩包
└─ tmp/                         # 原子写临时文件（内部，非用户可见）
```

> 迁移策略：`migrateStorageFiles` 已有迁移条目机制（含 `result.csv`、`cfip-log.txt`、`exports`、`backups`），新增目录分层通过追加迁移条目完成，见 §4.4 与 §6。

### 2.3 能力分层

在 `internal/appcore` 内收敛为三层接口，杜绝各处裸调 `os.WriteFile` / `os.MkdirAll`：

| 层 | 模块建议 | 职责 | 对应现有实现 |
| --- | --- | --- | --- |
| 写入接口层 | `appcore/storagewriter.go`（新增） | 统一原子写、目录创建、权限、路径归一化 | `WriteFileAtomic`、`CaptureFileStates`/`RestoreFileStates`、`CSVWriter` |
| 归档层 | `appcore/archive_service.go` | 压缩、轮转、保留策略、WebDAV 归档 | `archivecore`、`BuildConfigArchive`、`writeLocalArchiveBackup` |
| 导出层 | `appcore/diagnostic_service.go`、`config_export_service.go` | 脱敏、打包、命令响应组装 | `invokeDebugExport`、`invokeDiagnosticExport`、`invokeConfigExport` |

分层数据流（以一次探测任务为例）：

```
probe.start
  → DebugLogger.Event(...)        # 写入层：logs/app/cfip-log.jsonl（JSONL + 轮转）
  → CSVWriter.ExportContext(...)  # 写入层：exports/{date}/ 或用户指定路径
  → TaskStore 快照               # 写入层：tasks/2026/08/{id}.json（原子写 + 索引追加）
  → PublishProbeEvent(...)        # 事件通道 probe:event（只读不下沉）
  → 保留策略定时器               # 归档层：过期温/冷数据清理（task_history / archive）
  → diagnostics.export           # 导出层：zip 打包（脱敏）
```

### 2.4 分层模型与数据流

**写入模型**：所有持久化写入收敛到 `storagewriter`，保证：

1. 原子性（临时文件 + `fsync` + rename，复用 `WriteFileAtomic` 现有实现）。
2. 权限由**文件类型清单**统一决定（见 §3.3），不再由各调用点随意指定。
3. 路径只经 `StorageLayout` 派生方法产出，禁止字符串拼接。

**读取模型**：热层直接读文件；温层经 `task.list`/`task.results` 的索引+目录前缀裁剪；冷层仅归档/还原路径访问。

**数据流分层表**：

| 数据 | 热写入 | 温存储 | 冷归档 | 导出/读取 |
| --- | --- | --- | --- | --- |
| 配置 | `SaveConfig` | — | `config.backup` / WebDAV | `config.export` / `config.load` |
| 任务快照 | `TaskStore` | `tasks/yyyy/MM/` | 过期清理 | `task.get` / `task.list` / `diagnostics.export` |
| 探测结果 | `CSVWriter` | `exports/yyyy/MM/` | GitHub 远端 | `results.export_csv` / `github.export` |
| 运行日志 | `DebugLogger` | `logs/app/`（轮转） | `logs/*.jsonl.gz` | `debug.export` / `diagnostics.export` |

### 2.5 生命周期管理

引入配置化保留策略（`cfst-gui-config-v2` 新增 `retention` 分域，默认值见下表；旧配置缺省时按默认值处理，兼容不破坏）：

| 配置项 | 默认 | 作用对象 |
| --- | --- | --- |
| `retention.tasks_days` | 90 | `tasks/` 温层任务快照（终态且超期） |
| `retention.exports_days` | 30 | `exports/` 结果 CSV |
| `retention.logs_size` | 50 MiB | 单个日志热文件轮转阈值 |
| `retention.logs_files` | 7 | 轮转后 `.jsonl.gz` 保留份数 |
| `retention.archive_days` | 365 | `archive/` 冷层保留 |
| `retention.enabled` | true | 总开关；置 false 时跳过一切清理 |

**清理规则**：

- 清理只对**温/冷层**执行，**严禁触碰热层状态文件与当前活动日志**。
- 清理动作一律写 `probe:event` 审计事件（`storage.retention.run`），并记录到日志。
- 清理通过 `archive_service` 的受控通道执行（对应"只动不删 + 副作用验证"约束），不使用裸 `RemoveAll`。
- 保留策略运行时机：服务启动时 + `storage.health` 触发 + 每日一次定时器。

---

## 3. 日志可被其他应用读取的机制设计

> 目标：满足"APP 能成功导出日志且被其他应用读取"，核心是 **统一目录 + 统一格式 + 可读权限 + 原子写与轮转 + 稳定导出契约**。

### 3.1 统一目录约定

- 固定约定 `logs/app/cfip-log.jsonl` 为**对外可读日志**主文件；`logs/error-log.txt` 兼容保留。
- 各端（桌面/WebUI/Android/CLI）统一由 `StorageLayout.LogsRoot()` 派生，`internal/app` 的 `logDirectoryPath()` 与 `appcore` 保持一致，不另行发明路径。
- 目录创建统一 0o755（现有 `os.MkdirAll(dir, 0o755)` 已是该语义）。

### 3.2 统一格式（JSONL + schema）

- **主线格式**：`logs/app/*.jsonl`，每行一个 JSON 对象（复用 `DebugLogModeStructured` 的 `json.Marshal` 序列化），字段：

```json
{"event":"probe.complete","level":"info","ts":"2026-08-25T10:00:00.123456789+08:00","task_id":"...","counts":{...}}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `event` | string | 是 | snake_case 事件名（`probe.start`/`probe.complete`/...） |
| `level` | string | 是 | `debug`/`info`/`warn`/`error` |
| `ts` | string | 是 | RFC3339Nano 时间戳 |
| `task_id` | string | 否 | 关联任务（有则填） |
| 其余 | any | 否 | 事件自定义字段（`counts`/`target_path`/`duration_ms` 等） |

- **约定 schema 文档**：新增 `docs/logging-contract.md` 固化该 schema，并同步到 `README.md`（符合 architecture-constraints 的"变更跨端契约同步文档"要求）。
- 现有 `DefaultDebugLogFormat`（freeform 文本模式）保留为内部诊断模式，**对外可读日志一律走 JSONL**，避免消费者解析歧义。
- 脱敏：导出链路沿用 `utils.RedactSensitiveText`，但**落盘日志本身不预脱敏**（保持原始可诊断），脱敏发生在导出/打包阶段。

### 3.3 文件权限

- **统一权限策略表**（由 `storagewriter` 按文件类型决定，替代当前散落指定）：

| 文件类型 | 权限 | 理由 |
| --- | --- | --- |
| 配置/草稿/调度/来源档案/任务快照 | 0o600 | 含敏感配置，仅本进程 |
| **应用日志 `logs/**`** | **0o644** | 其他应用可读，满足对外读取诉求；目录 0o755 |
| 结果 CSV / 导出产物 | 0o644 | 只读共享，现有语义 |
| 配置备份/归档 | 0o600 | 含敏感 Token |
| 诊断导出 zip | 0o644 | 只读共享 |

- **跨平台语义**：Unix 直接按 mode；Windows 上 0o644 意味着放开同用户只读 ACL，为"其他应用读取"提供基础；对需要跨用户读取的场景，文档显式说明由部署方在目标机上授权目录 ACL（默认不做全系统开放）。
- 实现：在 `storagewriter` 增加"按类型取权限"的函数并全量替换调用点，避免再次出现 0o600/0o644 混用。

### 3.4 原子写与轮转

**原子写**：

- 新增日志统一走 `WriteFileAtomic`（临时文件 + `fsync` + rename），避免读者读到半行。
- `DebugLogger` 打开热文件改为追加模式（`O_APPEND`）而非 `O_TRUNC`，消除运行期截断丢日志问题（问题 P1）。

**轮转策略**（解决 P2）：

- **阈值**：`retention.logs_size`（默认 50 MiB）按大小轮转；同时支持按天轮转（跨天后新建当日文件）。
- **流程**：热文件达到阈值 → 关闭并重命名为 `cfip-log.{yyyyMMdd-HHmmss}.jsonl` → 立即 gzip 为 `.jsonl.gz` 移入 `logs/archive/` → 新建空热文件 → 按 `retention.logs_files`（默认 7）清理最旧压缩包。
- **读取一致性**：轮转期间读 `debug.export` 时，对热文件 + 全部 `.jsonl.gz` 做**按时间排序合并**后再脱敏导出，保证导出的日志是完整连续的。
- **并发安全**：`DebugLogger` 已有 `sync.Mutex` 串行化写入；轮转在同一锁内完成，写者与轮转不并发。

### 3.5 导出接口契约

保持现有命令名与 envelope（`cfst-gui-command-v2`）不变，扩展行为：

| 命令 | 现有契约 | 增强（向后兼容） |
| --- | --- | --- |
| `debug.export` | 读 `cfip-log.txt` → 脱敏 → txt / base64 | 读 `cfip-log.jsonl` 主文件；若缺省则回退 `cfip-log.txt`；`data` 增加 `"format":"jsonl"`、`"log_dir"`、`"rotated_files":N` |
| `diagnostics.export` | zip：logs/status/config/tasks | `logs/` 条目统一指向 `logs/app/`（含压缩日志按需解压合并）；其余不变 |
| `storage.health` | 目录可用性 | `data.health` 增加 `logs_writable`、`logs_readable`、`retention` 摘要（见 §4.1） |
| `results.export_csv` | 写 exports/ 或 target_path | 默认路径改 `exports/yyyy/MM/{ts}-{task_id}.csv`；保留 `target_path` 完全覆盖优先 |

---

## 4. 维护性设计

### 4.1 可诊断性

- **`storage.health` 扩展**：返回各层目录存在性、可写性、`logs` 可读性、轮转状态、保留策略配置摘要、各目录占用字节数。
- **布局自省**：`runtime.status` 返回完整 `StorageLayout`（各根目录绝对路径），前端可直接展示。
- **审计事件**：所有迁移、轮转、清理、归档动作发布 `probe:event`（如 `storage.retention.run`、`storage.rotation.done`、`storage.migration.done`）。
- **失败可观测**：迁移/清理失败不静默，写 `error-log.txt` 并计入 `storage.health` 的 `last_errors`。

### 4.2 配置化

- `cfst-gui-config-v2` 新增 `retention` 分域（字段见 §2.5），由 `sanitizeConfig` 归一化，缺省用默认值，非法值回退默认。
- 新增 `storage.layout` 只读命令（可选）返回分层目录清单与策略，供前端"存储设置"页使用。
- `storage.set` 保持废弃状态，但文档声明：如需自定义根目录，改为通过环境变量 `CFST_GUI_DATA_DIR` 或便携模式（部署期决定），**不改回运行时命令**，避免与 `migrateStorageFiles` 冲突。

### 4.3 兼容与迁移

- 顶层 `StorageLayout` 字段与目录名**不变**，仅新增子目录，现有代码（`ConfigPath`/`TasksRoot`/`LogsRoot`/`ExportsRoot`/`BackupsRoot`）全部继续可用。
- 旧数据路径兼容：
  - `logs/cfip-log.txt`：迁移后由迁移条目改写为 `logs/app/cfip-log.jsonl`；若旧文件存在，首轮先复制旧内容为 JSONL 前缀再开启轮转。
  - `tasks/` 平铺文件：迁移时按 `mtime` 归入 `tasks/yyyy/MM/`；读取路径提供**回退逻辑**（新路径不存在时回退旧平铺路径），保证旧快照仍可被 `task.get`/`task.results` 读取。
  - `Root/result.csv`：沿用现有迁移条目迁移到 `exports/`，`resolveResultFilePath` 的旧路径回退保留。
- 迁移通过追加 `migrateStorageFiles` 条目实现（复用现有机制与测试），**迁移幂等、可重入**。

### 4.4 回滚

- 写入层复用 `CaptureFileStates`/`RestoreFileStates`（已在 `archive.import` 使用）：对配置、来源档案、任务快照做迁移前状态快照，任一步失败即回滚。
- 轮转/清理动作在操作前记录到 `logs/` 审计行；若策略配置错误，通过 `retention.enabled=false` 全局关闭并停止清理，不产生数据损失。
- 保留 `archive.import` 的 `pre-import` 本地备份机制，作为配置级整体回滚手段。

---

## 5. 性能设计

### 5.1 写入缓存与批量刷盘

- **日志**：`DebugLogger` 改为**行缓冲**（`bufio.Writer` + 内部缓冲），默认每 256 KiB 或 1 秒刷盘一次，降低高频 `Debugf` 的系统调用数；关键终态事件（`probe.complete`/`probe.failed`）触发 `Flush`+`fsync`，保证终态不丢。
- **结果 CSV**：`CSVWriter.ExportContext` 已有按行写逻辑，确认启用 `bufio` 缓冲并控制每次 `Sync` 频率（默认整批结束时一次）。
- **任务快照**：沿用原子写；高频的中间进度不落盘，仅终态/暂停/恢复落盘（现状已是如此，方案固化该约定）。

### 5.2 日志轮转与索引

- 轮转阈值与保留份数配置化（§2.5），避免单文件无限增长（P2）与导出时读大文件（P1/P8）。
- `tasks/` 新增 `index.jsonl` 增量索引：每写一条快照追加一行 `{task_id, path, status, ts}`（与快照同原子段写入），`task.list`/`task.results` 优先读索引，`diagnostics.export` 按索引挑最近 N 条，避免全目录扫描（P5）。
- 索引文件超阈值（默认 100k 行）后重写为紧凑索引（仅保留终态条目 + 最近活跃），控制读取成本。

### 5.3 IO 与并发安全

- 写路径串行化：日志由 `DebugLogger.mu` 串行；任务/配置由 `Service.mu` 串行；`storagewriter` 对同路径写入用互斥锁，杜绝并发 rename 竞态。
- 读路径与写路径不共锁：读取（`debug.export`、`task.results`）通过快照/轮转合并策略拿到一致视图，不阻塞写者。
- 归档/清理与写入串行（同一归档锁），但可与读取并发。
- 目录分桶（`tasks/yyyy/MM/`、`exports/yyyy/MM/`）降低单目录条目数与目录项扫描成本。

---

## 6. 分阶段实施计划

| 阶段 | 目标 | 关键改动 | 验收标准 |
| --- | --- | --- | --- |
| **P0：兼容冻结** | 基线对齐 | 无功能改动，补 `storage.health` 目录清单输出 | `go test ./internal/appcore ./internal/contracttest` 全绿 |
| **P1：日志可读与轮转** | 解决 P1/P2/P3 | `DebugLogger` 改 `O_APPEND` + 行缓冲 + size/time 轮转 + gzip；日志文件 0o644；`logs/app/cfip-log.jsonl`；导出回退逻辑 | 外部进程可读日志；`debug.export` 输出完整合并日志；轮转后保留份数符合配置 |
| **P2：目录分层与保留策略** | 解决 P4/P6/P10 | `tasks`/`exports` 按日期分桶 + 旧路径回退；`retention` 配置分域 + 清理通道；`storagewriter` 按类型定权限 | 迁移后旧快照仍可读；`retention` 开关生效；`storage.health` 展示策略摘要 |
| **P3：索引与归档** | 解决 P5/P8 | `tasks/index.jsonl` 增量索引；`diagnostics.export` 走索引；`archive/` 冷层目录与 GitHub 结果清理 | `task.list` 大数据量下不全目录扫描；索引重写正确 |
| **P4：文档与全量验证** | 收口 | `docs/logging-contract.md`、`README.md` 更新；迁移/回滚用例补测；golden fixture 按需更新 | `pnpm typecheck` + Go 全量测试 + 三端构建通过 |

每阶段独立合并、独立发布，避免大爆炸式重构；阶段间保持命令契约向后兼容（见 §7）。

---

## 7. 与现有命令契约的兼容策略

- **envelope 不变**：所有命令仍返回 `cfst-gui-command-v2`（`code`/`data`/`message`/`ok`/`task_id`/`warnings`）。
- **命令名与 payload 字段不变**：`storage.set`/`storage.health`/`config.*`/`archive.*`/`debug.export`/`diagnostics.export`/`results.export_csv`/`github.export` 的名称与既有 snake_case 字段保持兼容。
- **新增字段只增不改**：`data` 内新增 `format`/`rotated_files`/`logs_writable` 等字段；配置新增 `retention` 分域；旧调用方忽略新字段，新调用方对缺失字段用默认值。
- **错误码兼容**：保留全部既有 code（`DEBUG_LOG_EXPORT_OK`、`STORAGE_HEALTH_READY` 等）；新增失败码仅在新增路径出现。
- **契约基线**：变更必须对照 `behavior-baseline.md` 判断"兼容修复 vs 有意变更"；只有确认新行为成为新契约时才更新 `internal/contracttest/testdata/shared_behavior.golden.json`。
- **存储契约**：存储目录、便携模式、结果文件、调试日志路径、结果/输入源读取大小上限属跨端契约；目录分层属内部布局调整，但**路径回退必须覆盖旧布局**，保证桌面/WebUI/Android/CLI 四端读取一致。

---

## 8. 风险与回退

| 风险 | 等级 | 缓解 | 回退 |
| --- | --- | --- | --- |
| 轮转/迁移导致旧日志或旧任务快照暂不可读 | 中 | 旧路径回退逻辑 + 迁移幂等 | 删除新布局标记，回退旧布局；迁移条目已幂等可重跑 |
| 0o644 放宽日志权限引发敏感信息暴露 | 中 | 日志本身不含密钥（配置类敏感量走脱敏导出）；文档声明 ACL 责任 | `retention.enabled` 之外增加 `logs.mode=0600` 配置开关 |
| 行缓冲刷盘延迟导致崩溃丢少量日志 | 低 | 终态事件强制 `Flush`+`fsync`；缓冲上限 256 KiB | 调低缓冲阈值 |
| 索引损坏导致 `task.list` 异常 | 中 | 索引损坏时自动重建（扫描 `tasks/` 重建），读取始终有兜底 | 删除 `index.jsonl` 触发重建 |
| 清理误删数据 | 高 | 只动温/冷层 + 审计事件 + `retention.enabled=false` 总开关；清理走受控通道 | 归档前先入 `archive/`，可手动恢复 |
| 跨端行为不一致 | 中 | 契约基线 + `internal/contracttest` 双端比对 | 单端先回退命令实现，保持契约一致 |
| `diagnostics.export` 打包体积失控 | 中 | 按索引取最近 N 条 + 日志合并上限（默认 50 MiB） | 降低 `retention.logs_size` 与 N |

**总体回退策略**：本方案所有阶段性改动均可独立回退；全局最低安全阀为 `retention.enabled=false` 与 `logs.mode=0600`，置后即恢复到"只写不清理、仅本进程可读"的现状语义。
*（内容由AI生成，仅供参考）*
