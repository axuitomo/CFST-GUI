# CFST-GUI 分层存储系统 PRD

> **文档版本**：v1.1
> **日期**：2026-08-25
> **状态**：待评审 → 可开发
> **作者**：架构组
> **关联文档**：`docs/storage-layered-design.md`、`docs/storage-tiering-comparison.md`、`docs/PRD-storage-tiering-review.md`
> **基于方案**：方案 J（混合方案：SQLite 热层 + 目录温冷层 + 平台外部冷层）

---

## 版本变更记录

| 版本 | 日期 | 变更内容 |
|------|------|---------|
| v1.0 | 2026-08-25 | 初始版本，10 个功能需求 |
| v1.1 | 2026-08-25 | 基于双向钢人论证审核，6 项关键调整：<br>① FR-002 增加 SQLite 前置验证门禁，大字段拆分到文件系统<br>② FR-003 flush 间隔降为 2 秒，轮转异步化，error-log 整合分两步<br>③ FR-004 归档改为标记 archived=true 不删除行，单一数据源<br>④ FR-005 明确 Kotlin 侧创建临时文件并传路径给 Go 侧<br>⑤ FR-006 分享改为临时缓存文件，不保留永久副本<br>⑥ FR-010 日志改为启动时强制迁移，全量扫描后台迁移<br>⑦ FR-007/FR-008 移出 v1.1 范围，转到后续迭代<br>⑧ FR-009 改为异步缓存 + 简化统计 |

---

## 1. 背景与目标

### 1.1 背景

CFST-GUI 当前采用扁平文件系统存储，所有数据（配置、日志、任务快照、导出文件）混存在应用数据根目录下。存在以下问题：

1. **日志丢失**：`DebugLogger.Configure` 使用 `O_TRUNC`，每次启动探测任务清空历史日志
2. **无轮转机制**：日志文件无限增长，长期运行后单文件可达数百 MB
3. **导出性能差**：Android 端日志导出走「全量读取 → base64 → JS Bridge → 解码 → SAF」，大日志时内存峰值高，可能 OOM
4. **跨应用读取困难**：导出日志仅写入 SAF 目录，未通过 FileProvider 暴露 content URI，其他应用无法直接读取
5. **路径分散**：存储路径常量散落在多个文件中
6. **无生命周期管理**：任务快照、日志无自动清理策略

### 1.2 目标

| 目标 | 衡量指标 | 基线 | 目标值 |
|------|---------|------|--------|
| 日志可追溯 | 历史日志保留份数 | 0（每次清空） | ≥5 份轮转（桌面）/ ≥3 份（Android） |
| 日志写入性能 | 单条 Event 写入延迟（缓冲命中） | ~50μs | <5μs |
| 日志写入吞吐 | 持续写入吞吐 | ~2000 条/秒 | ≥10000 条/秒 |
| Android 大日志导出 | 50MB 日志导出内存峰值 | ~150MB | <10MB |
| Android 大日志导出 | 50MB 日志导出耗时 | 未测（常 OOM） | <3 秒 |
| 跨应用读取 | 导出后可通过系统分享发送到其他应用 | 不支持 | 支持，content URI 可读 |
| 热数据读写性能 | 任务快照读写延迟 | ~2ms（JSON 文件） | <1ms（SQLite，验证后） |
| 存储初始化 | 应用启动存储初始化耗时 | ~150ms | <50ms |
| 路径统一管理 | 业务逻辑层硬编码文件名次数 | >10 处 | 0 |

### 1.3 非目标（v1.1 不做）

- 不实现服务端化多用户存储
- 不引入网络存储/云同步
- 不改变现有业务逻辑（探测引擎、调度器、Cloudflare 集成等）
- 不实现数据加密（后续迭代）
- ~~FR-007 Android Logcat 采集~~ → 移到 `docs/PRD-diagnostic-enhancement.md`
- ~~FR-008 配置版本化备份~~ → 移到后续迭代，需重新设计备份策略

---

## 2. 术语定义

| 术语 | 定义 |
|------|------|
| 热层（Hot Tier） | 高频读写、生命周期短的数据存储层，使用 SQLite WAL + 日志文件 |
| 温层（Warm Tier） | 中频读写、生命周期中等的数据存储层，使用应用数据目录文件系统 |
| 冷层（Cold Tier） | 低频读写、长期保留的数据存储层，使用平台外部目录（Downloads/SAF） |
| WAL | Write-Ahead Logging，SQLite 预写日志模式，支持 1 写 N 读并发 |
| SAF | Storage Access Framework，Android 存储访问框架 |
| FileProvider | Android 支持库，将应用私有文件暴露为 content:// URI |
| Checkpoint | SQLite 将 WAL 文件合并到主数据库的操作 |
| Debounce | 防抖，多次触发合并为一次执行 |
| 钢人论证（Steelmanning） | 构建对方最强版本的论证再进行评估，而非攻击最弱版本 |

---

## 3. 总体架构

### 3.1 分层存储架构

```
┌─────────────────────────────────────────────────────────────┐
│                      业务层 (appcore.Service)                 │
│  probe_service / task_service / config_service / diagnostic  │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                    存储抽象层 (StorageManager)                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ StorageLayout│  │ StorageConfig│  │ CleanupScheduler │  │
│  │ 路径统一管理  │  │ 层级配置管理  │  │ 过期数据清理调度  │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
└──────┬─────────────────────┬─────────────────────┬──────────┘
       │                     │                     │
┌──────▼──────┐      ┌──────▼──────┐      ┌──────▼──────┐
│  热层 Hot    │      │  温层 Warm   │      │  冷层 Cold   │
│             │      │             │      │             │
│ SQLite WAL  │      │  文件系统    │      │ 平台外部目录 │
│ - tasks表   │      │ - config/   │      │ - exports/  │
│ - runtime表 │      │ - imports/  │      │ - backups/  │
│             │      │ - task-data/│      │ - diagnostics│
│ 日志文件     │      │             │      │             │
│ - 追加写     │      │             │      │ Android:SAF │
│ - 大小轮转   │      │             │      │ 分享:临时缓存│
└─────────────┘      └─────────────┘      └─────────────┘
```

### 3.2 目录结构

#### 桌面端（Windows/macOS/Linux）

```
<os.UserConfigDir()>/CFST-GUI/
├── storage.json              # 存储引导文件（迁移状态、版本，位置始终不变）
├── hot.db                    # SQLite WAL 热层数据库
├── hot.db-wal                # SQLite WAL 文件
├── hot.db-shm                # SQLite 共享内存文件
├── logs/                     # 热层：日志目录（文件系统）
│   ├── cfip-log.txt          # 当前活动日志
│   ├── cfip-log.20260824-143000.txt  # 轮转归档
│   ├── error-log.txt         # 错误日志
│   └── error-log.20260820-180000.txt
├── config/                   # 温层：配置文件
│   ├── config.json
│   ├── desktop-draft.json
│   ├── scheduler.json
│   └── source-profiles.json
├── imports/                  # 温层：输入源缓存
├── task-data/                # 温层：任务完整数据（大字段，SQLite 只存元数据引用）
│   └── <task_id>.json
├── exports/                  # 冷层：导出文件
├── backups/                  # 冷层：配置备份（v1.2 实现自动备份）
└── diagnostics/              # 冷层：诊断包归档
```

#### Android 端

```
context.filesDir/                    # 应用私有目录（温层+热层）
├── mobile-config.json               # 存储引导/配置（位置始终不变）
├── hot.db                           # SQLite WAL
├── hot.db-wal
├── hot.db-shm
├── logs/
├── config/
├── imports/
├── task-data/
└── update_downloads/                # 更新包（FileProvider 已配置）

context.cacheDir/
├── export/                          # 导出临时文件（Go 侧写入，Kotlin 侧复制到 SAF 后删除）
└── share/                           # 分享临时文件（分享时复制，分享后/1小时后清理）

SAF 授权目录/                         # 冷层：用户选择的外部目录
├── exports/
├── backups/
└── diagnostics/
```

---

## 4. 功能需求（v1.1 范围：8 个）

### FR-STORAGE-001：存储布局管理

**优先级**：P0 | **可行性**：A | **预估工时**：1 天

**核心交付**：`StorageLayout` 统一路径管理，业务逻辑层消除硬编码文件名。

#### 功能细节

1. 重构 `internal/appcore/storage_layout.go` 中的 `StorageLayout` 结构体，包含所有层级路径
2. 新增 `internal/appcore/storage_paths.go` 定义所有文件名常量（唯一来源）
3. `StorageLayout` 提供语义化路径方法，业务代码必须通过方法获取路径
4. 支持旧路径兼容读取（迁移期，`LegacyPath()` 方法标注 `@Deprecated`）
5. `StorageLayout` 职责边界：只负责路径解析，不负责文件读写、不负责迁移逻辑

#### 接口定义

```go
// internal/appcore/storage_paths.go — 唯一文件名常量源
const (
    FileConfig         = "config.json"
    FileDesktopDraft   = "desktop-draft.json"
    FileScheduler      = "scheduler.json"
    FileSourceProfiles = "source-profiles.json"
    FileDebugLog       = "cfip-log.txt"
    FileErrorLog       = "error-log.txt"
    FileBootstrap      = "storage.json"
    FileHotDB          = "hot.db"
    DirTasks           = "tasks"
    DirLogs            = "logs"
    DirConfig          = "config"
    DirImports         = "imports"
    DirTaskData        = "task-data"
    DirExports         = "exports"
    DirBackups         = "backups"
    DirDiagnostics     = "diagnostics"
)

// internal/appcore/storage_layout.go
type StorageLayout struct {
    Root               string
    HotDBPath          string  // hot.db
    HotLogsDir         string  // logs/
    WarmConfigDir      string  // config/
    WarmImportsDir     string  // imports/
    WarmTaskDataDir    string  // task-data/
    ColdExportsDir     string  // exports/
    ColdBackupsDir     string  // backups/
    ColdDiagnosticsDir string  // diagnostics/
}

func NewStorageLayout(root string) StorageLayout

// 路径方法（按类别分组）
// 配置类
func (l StorageLayout) ConfigPath() string
func (l StorageLayout) DraftPath() string
func (l StorageLayout) SchedulerPath() string
func (l StorageLayout) SourceProfilesPath() string
// 日志类
func (l StorageLayout) DebugLogPath() string
func (l StorageLayout) ErrorLogPath() string
// 任务类
func (l StorageLayout) TaskDataPath(taskID string) string
// 兼容
func (l StorageLayout) LegacyPath(name string) string // Deprecated: 迁移期使用
```

#### 验收标准

- [ ] 业务逻辑层（`internal/appcore/`、`internal/app/`）中无硬编码文件名（grep 验证，测试和构建脚本允许例外）
- [ ] `StorageLayout` 覆盖所有现有文件路径
- [ ] 旧路径文件可通过 `LegacyPath()` 读取
- [ ] 单元测试覆盖所有路径方法
- [ ] `storage_paths.go` 是唯一的文件名常量来源

---

### FR-STORAGE-002：SQLite 热层（条件保留，需前置验证）

**优先级**：P1 | **可行性**：C（需验证） | **预估工时**：3-5 天（含 1 天验证）

**核心交付**：任务快照元数据和运行时状态存入 SQLite WAL，任务完整数据（大字段）存文件系统。

#### ⚠️ 前置验证门禁（必须第一个执行）

在全面开发 SQLite 热层之前，必须完成以下验证，**验证不通过则整个需求回退到文件系统热层方案**：

| 验证项 | 验证方法 | 通过标准 | 不通过处理 |
|--------|---------|---------|-----------|
| gomobile AAR 兼容性 | 用 `modernc.org/sqlite` 写最小 demo，gomobile bind 生成 AAR，Android 真机运行 CRUD | AAR 构建成功，真机 CRUD 无崩溃 | 回退到文件系统热层，或尝试 `mattn/go-sqlite3`（需 CGO+NDK） |
| 性能基准对比 | 测量 CFST-GUI 实际任务快照大小（50-200KB）的 SQLite 读写延迟，与 JSON 文件对比 | SQLite 读写延迟 ≤ JSON 文件的 80%（即至少快 20%） | 回退到文件系统热层 |
| 并发延迟 | 并发 200 goroutine 写快照，测量 p99 延迟 | p99 <50ms | 优化 debounce/批量写，仍不达标则回退 |

**验证工时**：1 天。验证通过后继续开发剩余 2-4 天；验证不通过则启用回退方案，本需求标记为"已评估，不采用"。

#### 功能细节（验证通过后）

1. 使用 `modernc.org/sqlite`（纯 Go 实现，无 CGO）
2. 数据库文件：`<root>/hot.db`
3. WAL 模式 PRAGMA：
   - `journal_mode=WAL`
   - `synchronous=NORMAL`
   - `busy_timeout=5000`
   - `cache_size=-64000`（64MB 缓存）
   - `temp_store=MEMORY`
4. 定期 checkpoint：每 1000 次写操作或每 5 分钟
5. **大字段拆分**（v1.1 关键调整）：SQLite 只存元数据（<2KB），完整配置和结果列表存文件系统 `warm/task-data/<task_id>.json`，SQLite 中存文件路径

#### 数据模型

```sql
-- 任务快照表（只存元数据，大字段在 task-data/ 文件中）
CREATE TABLE IF NOT EXISTS task_snapshots (
    task_id       TEXT PRIMARY KEY,
    status        TEXT NOT NULL,           -- pending/running/paused/completed/failed/canceled
    task_type     TEXT NOT NULL,           -- probe/scheduled/manual
    progress_json TEXT NOT NULL,           -- 进度统计摘要（<2KB，用于列表展示）
    data_file     TEXT NOT NULL,           -- warm/task-data/<task_id>.json 的路径
    archived      INTEGER NOT NULL DEFAULT 0,  -- 0=活动, 1=已归档（v1.1调整：不删除行，只标记）
    created_at    INTEGER NOT NULL,        -- Unix nano
    updated_at    INTEGER NOT NULL,        -- Unix nano
    completed_at  INTEGER                   -- Unix nano
);

CREATE INDEX IF NOT EXISTS idx_task_status ON task_snapshots(status);
CREATE INDEX IF NOT EXISTS idx_task_archived ON task_snapshots(archived);
CREATE INDEX IF NOT EXISTS idx_task_updated ON task_snapshots(updated_at DESC);

-- 运行时状态表（单例，key='current'）
CREATE TABLE IF NOT EXISTS runtime_state (
    key          TEXT PRIMARY KEY,
    value_json   TEXT NOT NULL,
    updated_at   INTEGER NOT NULL
);

-- Schema 版本表
CREATE TABLE IF NOT EXISTS schema_version (
    version      INTEGER PRIMARY KEY,
    applied_at   INTEGER NOT NULL,
    description  TEXT
);
```

#### 写入策略（v1.1 调整）

- **状态变更**（pending→running→completed/failed/canceled）：立即写入 SQLite，同时更新内存缓存
- **进度更新**：走 debounce（50ms 窗口批量提交），同时更新内存缓存
- **前端读取**：优先读内存缓存（保证读一致性），不直接查 SQLite
- **内存缓存与 SQLite 同步**：debounce 提交后缓存不变；应用启动时从 SQLite 加载到缓存

#### 接口定义

```go
type HotStore interface {
    // 任务快照
    UpsertTaskSnapshot(snapshot TaskSnapshot) error
    GetTaskSnapshot(taskID string) (*TaskSnapshot, error)
    ListTaskSnapshots(statusFilter string, includeArchived bool) ([]TaskSnapshot, error)
    MarkArchived(taskID string) error                // v1.1: 标记归档，不删除
    DeleteArchivedBefore(before time.Time, limit int) (int, error) // 清理超期已归档
    ListRecentCompleted(limit int) ([]TaskSnapshot, error)

    // 运行时状态
    SetRuntimeState(key string, value any) error
    GetRuntimeState(key string) (map[string]any, error)

    // 内存缓存
    GetCachedSnapshot(taskID string) (*TaskSnapshot, bool)
    InvalidateCache(taskID string)

    // 管理
    Close() error
    Checkpoint() error
    Vacuum() error
}
```

#### 回退方案（验证不通过时）

热层使用 JSON 文件 + 内存缓存 + debounce 刷盘：
- 活动任务快照存 `hot/tasks/<task_id>.json`
- 内存缓存保证读性能
- debounce 50ms 批量刷盘
- 性能目标调整为任务快照读写 <2ms
- 其余接口（`HotStore`）保持不变，业务层无感知

#### 验收标准

- [ ] **前置验证通过**：gomobile AAR 构建成功，真机 CRUD 正常，性能基准达标
- [ ] SQLite 数据库在应用启动时自动创建并初始化 schema
- [ ] 任务快照元数据读写延迟 <1ms（本地基准，数据 <2KB）
- [ ] 任务完整数据（大字段）存 `warm/task-data/<task_id>.json`，SQLite 只存路径
- [ ] 并发 200 goroutine 写快照，p99 延迟 <50ms，无数据丢失
- [ ] 前端读取优先走内存缓存，debounce 期间不读到旧数据
- [ ] 任务归档使用 `MarkArchived` 标记，不删除行；查询默认过滤 `archived=0`
- [ ] WAL 文件大小不超过主数据库的 2 倍（定期 checkpoint）
- [ ] 应用崩溃后重启，数据完整可恢复（WAL 恢复）
- [ ] `go test -race` 通过，无数据竞争
- [ ] gomobile AAR 构建成功，Android 端可正常使用

---

### FR-STORAGE-003：日志系统优化

**优先级**：P0 | **可行性**：A | **预估工时**：2 天

**核心交付**：追加写 + 大小轮转 + 写缓冲，替代当前 O_TRUNC 模式。

#### 功能细节

1. **写入模式**：`O_CREATE|O_WRONLY|O_APPEND`，不再使用 `O_TRUNC`
2. **写缓冲**：`bufio.Writer`，缓冲区大小 64KiB
3. **flush 策略**（v1.1 调整）：
   - 缓冲区满自动 flush
   - **每 2 秒定时 flush**（原 5 秒，降低崩溃丢失窗口）
   - **关键节点强制 flush**：任务开始、任务完成/失败、应用退到后台
   - 应用退出时强制 flush
4. **轮转触发**（满足任一）：
   - 当前日志文件大小 ≥ 阈值（桌面 10MiB / Android 5MiB，可配置）
   - 应用启动时且当前日志非空（保留上一会话）
5. **轮转异步化**（v1.1 调整）：轮转时不持有全局锁阻塞写操作：
   - 获取锁，将当前 file 引用替换为新打开的文件
   - 释放锁，新写操作直接写到新文件
   - 异步 goroutine 关闭旧文件、rename 为归档名、删除过期归档
6. **轮转命名**：`cfip-log.<YYYYMMDD-HHMMSS>.txt`
7. **保留策略**（v1.1 差异化）：
   - 桌面：`cfip-log` 保留 5 份 + 当前活动；`error-log` 保留 10 份
   - Android：`cfip-log` 保留 3 份 + 当前活动；`error-log` 保留 5 份
   - 超过保留数量的最旧文件自动删除
8. **error-log 整合分两步**（v1.1 调整）：
   - P0（本需求）：保持 `AppendErrorLog` 独立函数，但提取共用的 `LogRotator` 组件，error-log 和 cfip-log 共用轮转逻辑
   - P3（后续迭代）：完全整合进 `DebugLogger`，统一接口

#### 接口变更

```go
type LogRotationConfig struct {
    MaxFileSize   int64         // 桌面默认 10MiB，Android 默认 5MiB
    MaxFileCount  int           // 桌面默认 5，Android 默认 3
    RotateOnStart bool          // 默认 true
    FlushInterval time.Duration // 默认 2s（v1.1 从 5s 调整）
    BufferSize    int           // 默认 64KiB
}

// 共用轮转组件（v1.1 新增，error-log 和 cfip-log 共用）
type LogRotator struct {
    config LogRotationConfig
    // ...
}
func (r *LogRotator) CheckRotate(currentPath string, currentSize int64) (archivedPath string, rotated bool)
func (r *LogRotator) CleanupExpired(logDir string, prefix string) error

type DebugLogger struct {
    // ... 现有字段
    rotation   LogRotationConfig
    buffer     *bufio.Writer
    fileSize   int64
    flushTimer *time.Timer
    rotator    *LogRotator
}

// 新增方法
func (logger *DebugLogger) Flush() error
func (logger *DebugLogger) Rotate() (string, error)
func (logger *DebugLogger) ListRotatedLogs() []LogFileInfo
```

#### 性能优化

- JSON 序列化在锁外完成，锁内仅缓冲写入
- 复用 `sync.Pool` 中的 `bytes.Buffer` 减少 GC
- 时间戳缓存秒级部分，仅 nanos 动态计算
- 增加缓冲命中率监控（`Event()` 调用中缓冲命中 vs 触发 flush 的比例）

#### 验收标准

- [ ] 启动探测任务不再清空历史日志
- [ ] 日志文件达到阈值（桌面 10MiB / Android 5MiB）自动轮转，命名正确
- [ ] 保留最近 N 份轮转文件（桌面 5 / Android 3），超过自动删除最旧
- [ ] 单条 Event 写入延迟 <5μs（缓冲命中时）
- [ ] 日志写入吞吐 ≥10000 条/秒（基准测试，并发 50 goroutine）
- [ ] flush 间隔 2 秒，任务开始/完成/失败/退后台时强制 flush
- [ ] 轮转操作不阻塞新写操作（异步关闭旧文件）
- [ ] 应用退出时缓冲区数据全部刷盘，无丢失
- [ ] error-log 独立轮转，与 cfip-log 共用 `LogRotator` 组件
- [ ] 崩溃时最多丢失 2 秒日志（关键节点不丢失）

---

### FR-STORAGE-004：任务归档与清理（简化版）

**优先级**：P1 | **可行性**：B | **预估工时**：2 天

**核心交付**：已完成任务标记归档（不删除行），定期清理超期已归档任务和旧诊断包。

#### 功能细节（v1.1 大幅简化）

1. **归档策略**（v1.1 关键调整）：
   - 任务完成/失败/取消后 60 秒（原 10 秒，给用户查看时间），调用 `MarkArchived(taskID)`
   - **不删除 SQLite 行**，只设置 `archived=1`
   - 任务完整数据文件 `warm/task-data/<task_id>.json` 保留（不移动）
   - 查询任务列表默认 `WHERE archived=0`（活动任务），提供"查看历史任务"入口（`archived=1`）
   - **单一数据源**：所有任务都在 SQLite，不需要双读路径

2. **清理策略**（全部可配置，提供"保留所有"选项）：
   - 已归档任务超过保留期（默认 30 天）：删除 SQLite 行 + 删除 `task-data/` 文件
   - 诊断包超过保留期（默认 7 天）：删除文件
   - 配置备份：v1.1 不自动备份，无清理（移到 v1.2）
   - 日志轮转：按 FR-003 策略

3. **清理触发**（v1.1 调整，保底机制）：
   - 应用启动时异步执行一次（不阻塞启动）
   - 应用空闲时（探测任务结束后 5 分钟无新任务）执行
   - 手动触发（设置页"清理存储"按钮，调用 `storage.cleanup`）

4. **清理操作**：异步执行，不阻塞用户操作，清理结果可查询

#### 接口定义

```go
type CleanupPolicy struct {
    TaskArchiveRetentionDays int  // 默认 30，0=保留所有
    DiagnosticRetentionDays  int  // 默认 7，0=保留所有
}

type StorageCleaner interface {
    Run(policy CleanupPolicy) CleanupResult
    MarkArchivedTask(taskID string) error          // 标记归档
    CleanExpiredArchivedTasks(retentionDays int) (int, error) // 清理超期已归档
    CleanExpiredDiagnostics(retentionDays int) (int, error)
}

type CleanupResult struct {
    ArchivedTasks      int
    DeletedTasks       int
    DeletedDiagnostics int
    FreedBytes         int64
    Errors             []string
}
```

#### 验收标准

- [ ] 任务完成后 60 秒内自动标记 `archived=1`，不删除行
- [ ] 任务列表查询默认只返回 `archived=0` 的活动任务
- [ ] "查看历史任务"可查询 `archived=1` 的已归档任务
- [ ] 超过 30 天的已归档任务自动删除（SQLite 行 + task-data 文件）
- [ ] 超过 7 天的诊断包自动删除
- [ ] 清理策略可配置，"保留所有"选项有效（retentionDays=0 时不清理）
- [ ] 应用启动时异步执行一次清理，不阻塞启动
- [ ] 清理操作异步执行，不阻塞用户操作
- [ ] 清理结果可查询（删除数量、释放空间、错误列表）

---

### FR-STORAGE-005：Android 日志导出优化

**优先级**：P0 | **可行性**：B | **预估工时**：2 天

**核心交付**：Kotlin 侧创建临时文件，Go 侧写入，Kotlin 侧流式复制到 SAF，替代 base64 JS Bridge 传递。

#### 功能细节

1. **路径传递机制**（v1.1 关键调整，明确设计）：
   - Kotlin 侧在调用 `debug.export` / `diagnostics.export` 前，先在 `context.cacheDir/export/` 创建临时文件
   - 将临时文件绝对路径通过 payload 的 `temp_file_path` 字段传给 Go 侧
   - Go 侧检测到 `temp_file_path` 字段时，直接将日志/诊断包写入该路径，**不返回 base64**
   - 不需要修改 gomobile 接口，Go 侧不需要知道 Android 目录结构

2. **Kotlin 侧流式复制**：
   - 从 Go 侧响应中读取 `temp_file_path`
   - 使用 `FileInputStream` → `SAF OutputStream` 流式复制，缓冲区 **64KiB**（原 8KiB，减少系统调用）
   - 复制完成后删除临时文件
   - 无 `temp_file_path` 时回退到 base64 模式（兼容桌面端和旧版本）

3. **临时文件清理**（v1.1 新增）：
   - 导出成功后立即删除
   - 导出失败/取消时在 `finally` 块中删除
   - `MainActivity.onCreate` 时清理 `cacheDir/export/` 下所有文件（防崩溃残留）

4. **桌面端统一**（v1.1 调整）：桌面端也改用临时文件方案（写到 `os.TempDir()`），Wails 侧读取文件返回给前端。如果 Wails bridge 不支持文件传递，则桌面端保留 base64，PRD 中明确说明原因。

5. **诊断包导出同理**：Go 侧写 zip 到临时文件，Kotlin 侧流式复制到 SAF

#### 接口变更

```go
// Go 侧 debug.export 响应
type DebugExportResponse struct {
    FileName     string `json:"file_name"`
    TempFilePath string `json:"temp_file_path,omitempty"` // Android: Go 写入的临时文件路径
    ContentBase64 string `json:"content_base64,omitempty"` // 回退模式: base64 内容
    WrittenBytes int    `json:"written_bytes"`
    LogDir       string `json:"log_dir"`
}

// Go 侧检测 payload 中的 temp_file_path
// if tempFilePath := payload["temp_file_path"]; tempFilePath != "" {
//     os.WriteFile(tempFilePath, logContent, 0o600)
//     return {temp_file_path: tempFilePath}
// }
```

```kotlin
// Kotlin 侧调用前创建临时文件
val tempFile = File(context.cacheDir, "export/${System.currentTimeMillis()}-cfip-log.txt")
tempFile.parentFile?.mkdirs()
val payload = JSONObject().put("temp_file_path", tempFile.absolutePath)
val response = service.invoke("debug.export", payload.toString())
// 从 response 读取 temp_file_path，流式复制到 SAF
// finally { tempFile.delete() }
```

#### 验收标准

- [ ] Kotlin 侧在调用导出前创建 `cacheDir/export/` 临时文件，路径通过 payload 传递
- [ ] Go 侧检测到 `temp_file_path` 时直接写文件，不返回 base64
- [ ] Kotlin 侧使用 64KiB 缓冲区流式复制到 SAF
- [ ] Android 端导出 50MB 日志内存峰值 <10MB
- [ ] Android 端导出 50MB 日志耗时 <3 秒
- [ ] 导出成功/失败/取消后临时文件被删除
- [ ] `MainActivity.onCreate` 清理 `cacheDir/export/` 残留文件
- [ ] 导出文件内容与原始日志一致（SHA256 校验）
- [ ] 无 `temp_file_path` 时回退到 base64 模式（兼容）
- [ ] gomobile 接口无需修改

---

### FR-STORAGE-006：Android 跨应用读取

**优先级**：P0 | **可行性**：A | **预估工时**：1 天

**核心交付**：分享时复制到临时缓存目录，FileProvider 暴露 content URI，系统分享面板发送到其他应用。

#### 功能细节（v1.1 优化分享机制）

1. **不保留永久副本**（v1.1 关键调整）：
   - 导出到 SAF 的文件不保留 filesDir 副本
   - 用户点击"分享日志"时，Kotlin 侧从 SAF 导出目录复制最新日志到 `cacheDir/share/<timestamp>.txt`（临时文件）
   - 构建 FileProvider content URI 指向该临时文件
   - 调起系统分享面板
   - 分享完成后（或 1 小时后）清理临时文件

2. **FileProvider 配置**（v1.1 缩小暴露面）：
   ```xml
   <!-- res/xml/file_paths.xml -->
   <paths xmlns:android="http://schemas.android.com/apk/res/android">
       <files-path name="update_downloads" path="update_downloads/" />
       <cache-path name="share_files" path="share/" />
   </paths>
   ```
   - 只暴露 `cacheDir/share/` 下的临时分享文件
   - 不暴露整个 logs 目录

3. **分享语义**（v1.1 明确）：本需求实现的是"临时分享查看"——用户通过分享面板发送到其他应用，接收方立即处理。`FLAG_GRANT_READ_URI_PERMISSION` 授予临时读权限。如果用户需要持久化保存，可选择"保存到文件"应用。

4. **异常处理**（v1.1 新增）：`ShareFile` 和 `OpenLogFile` 都 `try-catch ActivityNotFoundException`，捕获后提示"未找到可打开此文件的应用，请安装文本阅读器"。

5. **支持多文件类型**：日志（.txt, text/plain）、诊断包（.zip, application/zip）、配置（.json, application/json）、CSV 结果（.csv, text/csv）。

#### 接口定义

```kotlin
@PluginMethod
fun ShareFile(call: PluginCall) {
    val sourcePath = call.getString("source_path", "")  // SAF 导出目录中的文件路径
    val fileName = call.getString("file_name", "")
    val mimeType = call.getString("mime_type", "text/plain")
    // 1. 从 SAF 复制到 cacheDir/share/<timestamp>-<fileName>
    // 2. 构建 FileProvider content URI
    // 3. ACTION_SEND intent + FLAG_GRANT_READ_URI_PERMISSION
    // 4. try-catch ActivityNotFoundException
    // 5. 返回 content_uri
}

@PluginMethod
fun OpenLogFile(call: PluginCall) {
    // 获取最新导出日志路径
    // 复制到 cacheDir/share/
    // ACTION_VIEW 打开
    // try-catch ActivityNotFoundException
}

@PluginMethod
fun GetExportedFileUri(call: PluginCall) {
    val fileName = call.getString("file_name", "")
    // 返回复制到 cacheDir/share/ 后的 content URI
}
```

#### 导出响应新增字段

```json
{
  "code": "DEBUG_LOG_EXPORT_OK",
  "data": {
    "file_name": "cfip-log-20260825-143000.txt",
    "saf_uri": "content://com.android.externalstorage.documents/tree/...",
    "mime_type": "text/plain",
    "size_bytes": 1048576,
    "can_share": true
  }
}
```

> 注：v1.1 移除了 `content_uri` 字段（因为不保留永久副本），分享时由 `ShareFile` 方法动态生成临时 content URI。

#### 验收标准

- [ ] `ShareFile` 方法可调起系统分享面板
- [ ] 分享时文件从 SAF 复制到 `cacheDir/share/` 临时文件
- [ ] FileProvider 只暴露 `cacheDir/share/`，不暴露 logs 目录
- [ ] 其他应用（邮件、文件管理器等）可通过 content URI 读取文件内容
- [ ] `FLAG_GRANT_READ_URI_PERMISSION` 临时授权有效
- [ ] 分享完成后或 1 小时后临时文件被清理
- [ ] `ActivityNotFoundException` 被捕获，提示用户安装阅读器
- [ ] 支持 .txt/.zip/.json/.csv 四种文件类型，MIME 类型正确
- [ ] FileProvider authority 与应用包名一致，无冲突

---

### FR-STORAGE-009：存储健康检查增强（简化版）

**优先级**：P2 | **可行性**：B | **预估工时**：1 天

**核心交付**：`storage.health` 异步执行 + 缓存结果，返回各层总字节数（简化统计）。

#### 功能细节（v1.1 简化）

1. **异步执行 + 缓存**（v1.1 关键调整）：
   - `storage.health` 调用时立即返回缓存结果（<10ms）
   - 后台异步执行实际统计，更新缓存
   - 缓存有效期 5 分钟，避免频繁遍历
   - `storage.health?refresh=true` 强制重新统计（忽略缓存）

2. **简化统计内容**（v1.1 调整）：
   - 只统计各层根目录的**总字节数**（限制遍历深度为 1 层，不递归子目录）
   - **不统计文件数**（遍历开销大，价值低）
   - "可清理空间"只做基于保留策略的**粗略估算**（如"任务归档超过 30 天的部分预估 X MB"），不精确计算
   - 不递归遍历子目录

3. **前端 UI 移到后续迭代**：v1.1 只实现后端接口，前端不展示用量条。接口数据供开发者调试使用。

#### 响应结构

```json
{
  "code": "STORAGE_HEALTH_READY",
  "data": {
    "root": "/path/to/storage",
    "total_used_bytes": 52428800,
    "estimated_reclaimable_bytes": 10485760,
    "cached": true,
    "cached_at": "2026-08-25T14:30:00Z",
    "tiers": [
      { "tier": "hot",  "path": "/path/to/hot.db", "used_bytes": 5242880 },
      { "tier": "warm", "path": "/path/to/config", "used_bytes": 1048576 },
      { "tier": "cold", "path": "/path/to/exports", "used_bytes": 46137344 }
    ]
  }
}
```

#### 验收标准

- [ ] `storage.health` 接口响应 <10ms（返回缓存）
- [ ] 后台异步统计各层总字节数，更新缓存
- [ ] 缓存有效期 5 分钟
- [ ] `refresh=true` 时强制重新统计
- [ ] 只统计总字节数，不统计文件数，不递归子目录
- [ ] 可清理空间为粗略估算，基于保留策略
- [ ] 健康检查不阻塞应用启动和用户操作

---

### FR-STORAGE-010：数据迁移

**优先级**：P0 | **可行性**：B | **预估工时**：2 天

**核心交付**：从旧版扁平存储迁移到新版分层存储，日志强制迁移，全量扫描后台迁移，无数据丢失。

#### 功能细节（v1.1 修正关键缺陷）

1. **`storage.json` 位置不变**（v1.1 明确）：始终在应用数据根目录，不随分层迁移改变位置。迁移状态记录在此文件中。

2. **日志强制迁移**（v1.1 关键修正）：
   - 应用启动时检测旧路径 `cfip-log.txt` 和 `error-log.txt`
   - 如果存在且新路径 `logs/cfip-log.txt` 不存在，**立即迁移**（move 到 `logs/` 目录）
   - **不能惰性迁移**（惰性会导致旧日志永久丢失）
   - 迁移完成后删除旧文件

3. **启动时全量扫描 + 后台异步迁移**（v1.1 调整）：
   - 启动时扫描旧目录，列出所有待迁移数据（配置、任务、导出、备份）
   - 日志立即迁移（阻塞，但很快，通常 <100ms）
   - 其他数据在后台异步迁移（不阻塞启动）
   - 迁移进度记录到 `storage.json` 的 `migration_steps`
   - 即使不"惰性访问"，数据也会被后台迁移，不会永久留在旧目录

4. **迁移映射表**：

| 旧路径（扁平） | 新路径（分层） | 迁移方式 |
|----------------|---------------|---------|
| `config.json` | `warm/config/config.json` | 后台异步 |
| `desktop-draft.json` | `warm/config/desktop-draft.json` | 后台异步 |
| `scheduler.json` | `warm/config/scheduler.json` | 后台异步 |
| `source-profiles.json` | `warm/config/source-profiles.json` | 后台异步 |
| `cfip-log.txt` | `hot/logs/cfip-log.txt` | **启动时立即** |
| `error-log.txt` | `hot/logs/error-log.txt` | **启动时立即** |
| `tasks/*.json`（活动） | SQLite `task_snapshots` 表 + `warm/task-data/<id>.json` | 后台异步导入 |
| `tasks/*.json`（已完成） | SQLite `task_snapshots`（archived=1）+ `warm/task-data/<id>.json` | 后台异步导入 |
| `cfst-results/` | `cold/exports/` | 后台异步移动 |
| `backups/` | `cold/backups/` | 后台异步移动 |
| `exports/` | `cold/exports/` | 后台异步移动 |

5. **Android 迁移触发点**（v1.1 明确）：
   - 主触发：`BroadcastReceiver` 监听 `Intent.ACTION_MY_PACKAGE_REPLACED`（应用更新后触发一次）
   - 兜底：`MainActivity.onCreate` 中检查迁移状态，如果未完成则执行
   - 由 `AndroidStorageMigration` 组件执行

6. **迁移安全**：
   - 迁移过程中**不删除**旧文件，仅复制/移动到新位置
   - 移动操作使用 `os.Rename`，失败时回退到复制+验证+删除
   - 迁移失败记录到 `storage.json` 的 `migration_errors`，下次启动重试
   - 所有数据迁移完成后，旧文件保留 7 天（防回滚），7 天后自动清理
   - 提供 `storage.force_clean_legacy` 命令手动清理旧文件

#### 迁移状态结构

```go
type MigrationState struct {
    LayeredMigrationCompleted bool              `json:"layered_migration_completed"`
    MigrationSteps             map[string]bool  `json:"migration_steps"`
    // config_migrated, tasks_migrated, logs_migrated, exports_migrated, backups_migrated
    MigrationErrors            map[string]string `json:"migration_errors,omitempty"`
    MigrationAttemptedAt       string             `json:"migration_attempted_at,omitempty"`
    LegacyCleanupScheduledAt   string             `json:"legacy_cleanup_scheduled_at,omitempty"` // 旧文件清理时间
}
```

#### 验收标准

- [ ] `storage.json` 位置在迁移前后保持不变（应用数据根目录）
- [ ] 应用启动时旧 `cfip-log.txt` / `error-log.txt` 被立即迁移到 `logs/`，不丢失
- [ ] 启动时全量扫描待迁移数据，后台异步执行配置/任务/导出/备份迁移
- [ ] 旧任务快照正确导入 SQLite（活动任务 archived=0，已完成 archived=1），大字段写入 `warm/task-data/`
- [ ] 迁移过程不删除旧文件，失败可重试
- [ ] 迁移过程不阻塞应用启动（日志迁移除外，<100ms）
- [ ] Android 迁移通过 `MY_PACKAGE_REPLACED` 触发，`MainActivity.onCreate` 兜底
- [ ] 所有数据迁移完成后，旧文件保留 7 天后自动清理
- [ ] `storage.force_clean_legacy` 可手动清理旧文件
- [ ] 旧版本用户升级后所有数据可正常访问，无丢失

---

## 5. 非功能需求

### 5.1 性能需求

| 指标 | 目标值 | 测试方法 |
|------|--------|---------|
| 存储初始化耗时 | <50ms | 应用启动计时，100 次取平均 |
| 任务快照元数据写入延迟 | <1ms（p99 <5ms，SQLite 验证后） | SQLite 基准测试，并发 100 |
| 任务快照元数据读取延迟 | <0.5ms（内存缓存命中） | 基准测试 |
| 并发 200 goroutine 写快照 p99 延迟 | <50ms | 并发基准测试 |
| 日志写入延迟（缓冲命中） | <5μs | DebugLogger 基准测试 |
| 日志写入吞吐 | ≥10000 条/秒 | 并发 50 goroutine 持续写 |
| 日志崩溃丢失窗口 | ≤2 秒（关键节点不丢失） | 崩溃测试 |
| Android 50MB 日志导出内存峰值 | <10MB | Android Profiler 监控 |
| Android 50MB 日志导出耗时 | <3 秒 | 真机测试 |
| 诊断包构建（10MB 日志）内存峰值 | <50MB | 内存监控 |
| 存储健康检查接口响应 | <10ms（缓存命中） | 计时测试 |

### 5.2 可维护性需求

| 指标 | 目标值 |
|------|--------|
| 业务逻辑层硬编码文件名 | 0（测试和构建脚本允许例外） |
| 文件名常量唯一来源 | `storage_paths.go` |
| 存储相关代码单元测试覆盖率 | ≥80% |
| 公共 API 文档注释覆盖率 | 100%（导出类型和方法） |
| SQLite schema 迁移 | 版本化管理，`schema_version` 表记录 |
| 日志格式 | 结构化 JSON，每行一个事件，可被 jq/ELK 解析 |

### 5.3 兼容性需求

| 指标 | 要求 |
|------|------|
| 旧版本数据兼容 | 支持从 v1.x 扁平存储迁移到分层存储，无数据丢失 |
| 配置文件格式 | 向后兼容，新增字段有默认值 |
| 桌面平台 | Windows 10+、macOS 11+、Ubuntu 20.04+ |
| Android 版本 | Android 8.0 (API 26)+ |
| Go 版本 | 与项目当前 go.mod 一致 |
| gomobile 兼容 | SQLite 纯 Go 实现可被 gomobile 绑定（需前置验证） |
| Wails bridge | 桌面端导出方案与 Wails bridge 兼容 |

### 5.4 可靠性需求

| 指标 | 要求 |
|------|------|
| 崩溃恢复 | 应用崩溃后重启，SQLite 数据完整（WAL 恢复），日志最多丢失 2 秒 |
| 写入原子性 | 配置写入使用临时文件+rename；SQLite 使用事务；日志追加写 |
| 磁盘满处理 | 磁盘空间不足时返回明确错误，不静默丢失数据 |
| 并发安全 | 所有存储操作支持多 goroutine 并发访问，`go test -race` 通过 |
| 迁移安全 | 迁移不删除旧文件，失败可重试，7 天回滚窗口 |

---

## 6. 接口清单

### 6.1 Go 侧命令接口（appcore.Service.Invoke）

| 命令 | 变更类型 | 说明 |
|------|---------|------|
| `storage.health` | 修改 | 异步缓存 + 简化统计，返回各层总字节数 |
| `storage.set` | 保持 | 已废弃，固定使用应用数据目录 |
| `storage.cleanup` | 新增 | 手动触发存储清理 |
| `storage.cleanup_status` | 新增 | 查询清理结果 |
| `storage.force_clean_legacy` | 新增 | 手动清理迁移后的旧文件 |
| `debug.export` | 修改 | 支持 `temp_file_path` payload，Android 写临时文件 |
| `diagnostics.export` | 修改 | 支持 `temp_file_path` payload |
| `config.save` | 保持 | v1.1 不自动备份（移到 v1.2） |

### 6.2 Android Capacitor Plugin 接口

| 方法 | 变更类型 | 说明 |
|------|---------|------|
| `ShareFile` | 新增 | 分享文件到其他应用（临时缓存 + content URI） |
| `OpenLogFile` | 新增 | 打开最新导出日志文件 |
| `GetExportedFileUri` | 新增 | 获取导出文件的临时 content URI |
| `OpenLogDirectory` | 修改 | 实际打开 SAF 导出目录 |

---

## 7. 数据迁移详细方案

见 **FR-STORAGE-010**。核心调整（v1.1）：
- 日志启动时强制迁移（非惰性）
- 全量扫描 + 后台异步迁移
- `storage.json` 位置不变
- Android 触发点明确（`MY_PACKAGE_REPLACED` + `onCreate` 兜底）
- 旧文件保留 7 天后清理

---

## 8. 验收标准汇总

### 8.1 功能验收

- [ ] FR-001: 业务逻辑层无硬编码文件名，`storage_paths.go` 是唯一常量源
- [ ] FR-002: **前置验证通过**（gomobile AAR + 性能基准 + 并发延迟）；SQLite 热层正常工作；大字段拆分到文件系统；归档标记不删除
- [ ] FR-003: 日志追加写+轮转；flush 间隔 2 秒；关键节点强制 flush；轮转不阻塞写；error-log 共用 LogRotator
- [ ] FR-004: 任务标记归档不删除行；单一数据源；超期自动清理；启动时异步清理保底
- [ ] FR-005: Kotlin 创建临时文件传路径；Go 写临时文件不返回 base64；64KiB 流式复制；临时文件清理
- [ ] FR-006: 分享用临时缓存文件；FileProvider 只暴露 share/；系统分享面板；ActivityNotFoundException 处理
- [ ] FR-009: 健康检查异步缓存；简化统计（只总字节数）；refresh 参数
- [ ] FR-010: 日志强制迁移；全量扫描后台迁移；storage.json 位置不变；Android 触发点明确；旧文件 7 天清理

### 8.2 性能验收

- [ ] 存储初始化 <50ms
- [ ] 任务快照元数据读写 <1ms（SQLite 验证后）
- [ ] 并发 200 goroutine 写快照 p99 <50ms
- [ ] 日志写入吞吐 ≥10000 条/秒，延迟 <5μs
- [ ] 日志崩溃丢失 ≤2 秒
- [ ] Android 50MB 日志导出内存 <10MB，耗时 <3 秒
- [ ] 健康检查接口响应 <10ms（缓存）
- [ ] `go test -race` 通过

### 8.3 兼容性验收

- [ ] Windows/macOS/Linux 三平台构建通过
- [ ] Android AAR 构建通过（含 SQLite），APK 安装运行正常
- [ ] 旧版本用户升级后数据完整（配置、日志、任务、导出）
- [ ] 桌面端导出行为与 Wails bridge 兼容

---

## 9. 里程碑与排期（v1.1 调整后）

| 阶段 | 内容 | 预估工时 | 依赖 | 交付物 |
|------|------|---------|------|--------|
| **P0** | FR-001 存储布局管理 | 1 天 | 无 | StorageLayout 重构、路径常量统一 |
| **P0** | FR-003 日志系统优化 | 2 天 | FR-001 | 追加写、轮转、缓冲、LogRotator |
| **P0** | FR-005 Android 导出优化 | 2 天 | FR-003 | 临时文件机制、流式复制 |
| **P0** | FR-006 Android 跨应用读取 | 1 天 | FR-005 | 分享面板、FileProvider、临时缓存 |
| **P0** | FR-010 数据迁移 | 2 天 | FR-001 | 日志强制迁移、后台迁移、Android 触发 |
| **P1** | FR-002 SQLite 热层（含验证） | 3-5 天 | FR-001 | 前置验证（1天）+ 热层开发（2-4天） |
| **P1** | FR-004 任务归档与清理 | 2 天 | FR-002 | 标记归档、清理策略、异步清理 |
| **P2** | FR-009 存储健康检查 | 1 天 | FR-001 | 异步缓存、简化统计 |
| **测试** | 全平台测试 + 文档更新 | 2 天 | 全部 | 测试报告、文档更新 |
| **合计** | | **16-18 天** | | |

> **并行建议**：P0 五个需求不依赖 SQLite 验证结果，可以先开工。FR-002 的前置验证（1天）可以与 P0 并行执行，验证通过后再启动 FR-002 全面开发。

---

## 10. 风险与依赖（v1.1 更新）

### 10.1 技术风险

| 风险 | 严重度 | 需求 | 缓解措施 |
|------|--------|------|---------|
| `modernc.org/sqlite` 与 gomobile AAR 不兼容 | **致命** | FR-002 | P1 第一个任务验证，不通过则回退到文件系统热层（接口不变，业务层无感知） |
| 任务快照实际大小超出小 BLOB 范围 | **中** | FR-002 | 大字段拆分到 `warm/task-data/`，SQLite 只存元数据（<2KB） |
| 日志惰性迁移导致旧日志丢失 | **已修正** | FR-010 | v1.1 改为启动时强制迁移日志 |
| Go 侧无法获取 Android cacheDir | **已修正** | FR-005 | v1.1 改为 Kotlin 侧创建临时文件并传路径 |
| 写缓冲崩溃丢失关键日志 | **中** | FR-003 | flush 间隔降为 2 秒，关键节点强制 flush |
| SQLite 单写者串行导致并发延迟 | **中** | FR-002 | debounce 批量写 + 内存缓存读，验收标准 p99 <50ms |
| 归档双读路径复杂度 | **已修正** | FR-004 | v1.1 改为标记 archived 不删除，单一数据源 |
| FileProvider 暴露面过大 | **已修正** | FR-006 | v1.1 只暴露 `cacheDir/share/` 临时文件 |
| 健康检查遍历耗时 | **已修正** | FR-009 | v1.1 改为异步缓存 + 简化统计 |

### 10.2 依赖

| 依赖 | 说明 | 状态 |
|------|------|------|
| `modernc.org/sqlite` | 纯 Go SQLite 实现，需确认与项目 Go 版本和 gomobile 兼容 | **待验证** |
| AndroidX `FileProvider` | 已在项目依赖中 | 已满足 |
| gomobile AAR 构建链 | 需验证 SQLite 纯 Go 实现可被绑定 | **待验证** |
| 前端 bridge 适配 | 新增 `ShareFile`/`OpenLogFile` 等方法需前端调用适配 | 待开发 |
| Wails bridge 文件传递 | 桌面端导出临时文件方案需确认 Wails 支持 | 待确认 |

---

## 11. 移出 v1.1 范围的需求

| 原编号 | 需求 | 移出原因 | 去向 |
|--------|------|---------|------|
| ~~FR-007~~ | Android Logcat 采集 | 与分层存储核心目标关联弱，设备兼容性需测试，移到诊断增强 PRD | `docs/PRD-diagnostic-enhancement.md`（待创建） |
| ~~FR-008~~ | 配置版本化备份 | 与核心目标关联弱，"每次保存都备份"策略有缺陷（频繁修改产生大量无用备份），需重新设计 | v1.2 重新设计后纳入 |

---

## 12. 附录

### 12.1 参考文档

- `docs/storage-layered-design.md` — 分层存储优化设计方案
- `docs/storage-tiering-comparison.md` — 10 方案对比与选型（最终选方案 J）
- `docs/PRD-storage-tiering-review.md` — 双向钢人论证审核报告（v1.1 基于此调整）
- `docs/android-mobile.md` — Android 移动端架构
- SQLite WAL 模式：https://www.sqlite.org/wal.html
- SQLite 比文件系统快 35%：https://www.sqlite.org/fasterthanfs.html
- Android FileProvider：https://developer.android.com/reference/androidx/core/content/FileProvider

### 12.2 术语表

见第 2 节。
