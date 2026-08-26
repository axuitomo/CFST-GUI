# CFST-GUI 分层存储设计优化方案

> 版本：v1.0 | 日期：2026-08-25 | 重点：方案设计 | 目标：维护性 + 性能

---

## 1. 现状与问题诊断

### 1.1 当前存储布局

桌面端与 Android 端共用 `appcore.StorageLayout`，根目录下扁平排列：

```
<storage_root>/
├── config.json          # 配置（低频读写）
├── desktop-draft.json   # 草稿（中频）
├── scheduler.json       # 调度状态（中频）
├── source-profiles.json # 源配置（低频）
├── tasks/               # 任务快照 + 结果（高频写、读多）
├── logs/                # 调试日志（高频写）
├── exports/             # 导出文件（一次性写）
└── backups/             # 备份（低频）
```

Android 端额外在 app 私有目录有：
- `mobile-config.json` — 配置
- `imports/` — 输入源副本
- `tasks/` — 任务快照与分页缓存
- `update_downloads/` — 更新包

### 1.2 核心问题

| 维度 | 问题 | 影响 |
|------|------|------|
| **分层缺失** | 热数据（日志/任务快照）与冷数据（配置/备份）扁平混存 | 无法按访问频率差异化管理，备份/迁移时全量拷贝 |
| **日志丢失** | `DebugLogger.Configure` 使用 `O_TRUNC`，每次启动探测清空旧日志 | 历史调试信息不可追溯，问题复现困难 |
| **无轮转** | 日志文件无限增长，无大小/时间轮转 | 长期运行后单文件过大，读取/导出内存压力大 |
| **导出性能** | Android 日志导出走「全量读取 → base64 → JS Bridge → 解码 → SAF」 | 大日志时内存峰值高，WebView 与原生层双重拷贝 |
| **跨应用读取** | 导出日志仅写入 SAF 目录，未通过 FileProvider 暴露 content:// URI | 其他应用无法直接通过 Intent 读取，用户需手动找文件 |
| **路径分散** | 存储路径常量散落在 `storage.go`、`storage_layout.go`、`diagnostic_service.go` | 修改布局需多处同步，易遗漏 |
| **无生命周期** | 任务快照、日志无自动清理策略 | 长期使用后存储膨胀，冷数据占用空间 |

---

## 2. 分层存储架构设计

### 2.1 三层模型

按数据访问频率与生命周期划分为 **热层（Hot）**、**温层（Warm）**、**冷层（Cold）**：

```
<storage_root>/
├── hot/                        # 热层：高频读写，生命周期短
│   ├── logs/                   #   运行时日志（轮转）
│   ├── tasks/                  #   活动任务快照
│   └── runtime/                #   运行时状态（调度锁、PID）
│
├── warm/                       # 温层：中频读写，生命周期中
│   ├── config/                 #   配置文件
│   │   ├── config.json
│   │   ├── desktop-draft.json
│   │   ├── scheduler.json
│   │   └── source-profiles.json
│   ├── imports/                #   输入源缓存（Android）
│   └── task-archive/           #   已完成任务快照（可清理）
│
└── cold/                       # 冷层：低频读写，长期保留
    ├── exports/                #   导出文件
    ├── backups/                #   配置备份
    └── diagnostics/            #   诊断包归档
```

### 2.2 分层策略表

| 层级 | 目录 | 数据类型 | 访问模式 | 生命周期 | 清理策略 |
|------|------|----------|----------|----------|----------|
| Hot | `hot/logs/` | 调试日志 | 追加写、顺序读 | 短（轮转） | 按大小/天数轮转，保留 N 份 |
| Hot | `hot/tasks/` | 活动任务快照 | 频繁覆盖写 | 中（任务存续） | 任务完成后归档到 warm，超期删除 |
| Hot | `hot/runtime/` | 运行时锁/PID | 启动写、退出删 | 极短 | 进程退出自动清理 |
| Warm | `warm/config/` | 配置/草稿/调度 | 低频写、启动读 | 长 | 永不自动删除，版本化备份 |
| Warm | `warm/imports/` | 输入源副本 | 一次写、多次读 | 中 | 配置变更时清理旧文件 |
| Warm | `warm/task-archive/` | 已完成任务 | 写一次、偶尔读 | 长（可配置） | 超过保留天数（默认 30 天）删除 |
| Cold | `cold/exports/` | 导出文件 | 一次写 | 长 | 用户手动管理，不自动删除 |
| Cold | `cold/backups/` | 配置备份 | 一次写 | 长 | 保留最近 N 份（默认 10） |
| Cold | `cold/diagnostics/` | 诊断包归档 | 一次写 | 中 | 超过保留天数（默认 7 天）删除 |

### 2.3 StorageLayout 重构

将当前扁平的 `StorageLayout` 升级为分层感知的布局管理器：

```go
// internal/appcore/storage_layout.go

type StorageTier string

const (
    TierHot  StorageTier = "hot"
    TierWarm StorageTier = "warm"
    TierCold StorageTier = "cold"
)

type StorageLayout struct {
    Root string
    // 热层
    HotLogsDir    string // hot/logs
    HotTasksDir   string // hot/tasks
    HotRuntimeDir string // hot/runtime
    // 温层
    WarmConfigDir      string // warm/config
    WarmImportsDir     string // warm/imports
    WarmTaskArchiveDir string // warm/task-archive
    // 冷层
    ColdExportsDir     string // cold/exports
    ColdBackupsDir     string // cold/backups
    ColdDiagnosticsDir string // cold/diagnostics
}

func NewStorageLayout(root string) StorageLayout {
    return StorageLayout{
        Root:               root,
        HotLogsDir:         filepath.Join(root, "hot", "logs"),
        HotTasksDir:        filepath.Join(root, "hot", "tasks"),
        HotRuntimeDir:      filepath.Join(root, "hot", "runtime"),
        WarmConfigDir:      filepath.Join(root, "warm", "config"),
        WarmImportsDir:     filepath.Join(root, "warm", "imports"),
        WarmTaskArchiveDir: filepath.Join(root, "warm", "task-archive"),
        ColdExportsDir:     filepath.Join(root, "cold", "exports"),
        ColdBackupsDir:     filepath.Join(root, "cold", "backups"),
        ColdDiagnosticsDir: filepath.Join(root, "cold", "diagnostics"),
    }
}

// 兼容旧路径的方法，迁移期使用
func (l StorageLayout) LegacyPath(name string) string {
    return filepath.Join(l.Root, name)
}
```

### 2.4 迁移策略

采用**双路径兼容 + 惰性迁移**，避免一次性迁移导致启动卡顿：

1. **启动时检测**：检查根目录下是否存在旧版扁平文件（如 `config.json`）
2. **惰性迁移**：首次访问某类数据时，若旧路径存在则迁移到新分层路径
3. **兼容读取**：`StorageLayout` 提供 `ResolveConfigPath()` 等方法，优先新路径，回退旧路径
4. **迁移标记**：在 `storage.json` 中记录 `layered_migration_completed`，全部迁移完成后清理旧文件
5. **Android 端**：因使用 app 私有目录，可在版本升级时由 `AndroidStorageMigration` 执行一次性迁移

---

## 3. 日志系统优化设计

### 3.1 日志轮转机制

替换当前 `O_TRUNC` 模式为**追加写 + 大小/时间轮转**：

```
hot/logs/
├── cfip-log.txt          # 当前活动日志（追加写）
├── cfip-log.20260825-143000.txt  # 轮转归档（按时间戳）
├── cfip-log.20260824-091500.txt
├── error-log.txt         # 错误日志（追加写）
└── error-log.20260820-180000.txt
```

**轮转触发条件**（满足任一即轮转）：
- 文件大小超过阈值（默认 10 MiB，可配置）
- 跨自然日（可选，默认关闭）
- 应用启动时（仅当当前日志非空，保留上一会话日志）

**保留策略**：
- `cfip-log` 保留最近 5 份轮转文件 + 当前活动文件
- `error-log` 保留最近 10 份（错误日志价值更高）
- 超过保留数量的最旧文件自动删除

### 3.2 DebugLogger 重构

```go
// internal/utils/debug_logger.go

type LogRotationConfig struct {
    MaxFileSize   int64  // 单文件最大字节数，默认 10MiB
    MaxFileCount  int    // 保留轮转文件数，默认 5
    RotateOnStart bool   // 启动时是否轮转，默认 true
}

type DebugLogger struct {
    mu          sync.Mutex
    enabled     bool
    file        *os.File
    filePath    string
    fileSize    int64
    taskID      string
    mode        string
    format      string
    verbosity   string
    console     io.Writer
    clock       func() time.Time
    rotation    LogRotationConfig
    writeBuffer *bufio.Writer  // 写缓冲，减少 syscall
}
```

**关键改进**：
1. **追加写**：`O_CREATE|O_WRONLY|O_APPEND`，不再截断
2. **写缓冲**：使用 `bufio.Writer`（默认 64 KiB），批量写入降低 syscall 频率；`Event` 高频调用时性能显著提升
3. **轮转检查**：每次写入后累加 `fileSize`，超过阈值触发 `rotateLocked()`
4. **启动轮转**：`Configure` 时若当前日志非空且 `RotateOnStart=true`，先轮转再创建新文件
5. **Flush 保障**：提供 `Flush()` 方法，任务结束/应用退出时强制刷盘

### 3.3 日志写入性能优化

| 优化点 | 当前 | 优化后 | 预期收益 |
|--------|------|--------|----------|
| 写入模式 | 每次 `Write` 直接 syscall | `bufio.Writer` 64KiB 缓冲 | syscall 次数降低 90%+ |
| 锁粒度 | 全局 `mu.Lock()` 覆盖序列化+写入 | 序列化在锁外完成，锁内仅缓冲写入 | 并发写入延迟降低 |
| JSON 序列化 | 每次 `json.Marshal` | 复用 `sync.Pool` 中的 `bytes.Buffer` | GC 压力降低 |
| 时间格式化 | `time.Now().Format(RFC3339Nano)` | 缓存秒级时间戳，仅 nanos 部分动态 | CPU 占用降低 |

### 3.4 错误日志独立通道

当前 `AppendErrorLog` 是独立函数，与 `DebugLogger` 无关联。优化为：

- `DebugLogger` 内部持有 `errorFilePath`，错误级别事件同时写入 error-log
- 错误日志使用独立轮转配置（更大保留数）
- 提供 `Error(event, fields)` 方法，统一入口

---

## 4. Android 日志导出与跨应用读取设计

### 4.1 当前导出链路问题

```
Go 侧读取日志 → base64 编码 → JS Bridge (JSON) → Kotlin 解码 → SAF 写入
```

问题：
- 大日志时 base64 膨胀 33%，JS Bridge 传递字符串有大小限制
- WebView 内存中同时存在原始字节 + base64 字符串 + 解码后字节
- 导出后日志仅存在 SAF 目录，其他应用需用户手动定位

### 4.2 优化导出链路：原生直写

**方案：Go 侧写入临时文件 → Kotlin 直接文件复制到 SAF → 清理临时文件**

```
Go 侧: 读取日志 → 写入 <cache_dir>/export/cfip-log.txt → 返回临时文件路径
Kotlin: 接收路径 → FileInputStream → SAF OutputStream → 删除临时文件 → 返回 content URI
```

**Go 侧修改**（`diagnostic_service.go`）：
- `invokeDebugExport` 新增 `target_path` 为应用缓存目录时，直接写文件而非 base64
- 返回 `temp_file_path` 字段，Kotlin 侧据此复制

**Kotlin 侧修改**（`AndroidExportResponses.kt`）：
- `writeDebugLogExportToURI` 优先处理 `temp_file_path`，走文件流复制
- 无 `temp_file_path` 时回退到 base64 模式（兼容）

**预期收益**：
- 内存峰值从「原始 + base64 + 解码」降至「8KiB 缓冲区」
- 导出 50 MiB 日志时，WebView 无 OOM 风险

### 4.3 跨应用读取：FileProvider + Content URI

当前 `file_paths.xml` 仅配置了 `update_downloads`。新增日志导出目录的 FileProvider 映射：

```xml
<!-- res/xml/file_paths.xml -->
<paths xmlns:android="http://schemas.android.com/apk/res/android">
    <files-path name="update_downloads" path="update_downloads/" />
    <cache-path name="export_logs" path="export/" />
    <files-path name="app_logs" path="hot/logs/" />
</paths>
```

**导出后自动生成 content URI 并提供分享入口**：

```kotlin
// AndroidExportResponses.kt 新增
fun buildContentUriForExportedFile(context: Context, file: File): Uri {
    return FileProvider.getUriForFile(
        context,
        "${context.packageName}.fileprovider",
        file
    )
}

fun createShareIntent(contentUri: Uri, mimeType: String): Intent {
    return Intent(Intent.ACTION_SEND).apply {
        type = mimeType
        putExtra(Intent.EXTRA_STREAM, contentUri)
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
}
```

**导出响应新增字段**：
```json
{
  "code": "DEBUG_LOG_EXPORT_OK",
  "data": {
    "file_name": "cfip-log-20260825-143000.txt",
    "content_uri": "content://io.github.axuitomo.cfstgui.fileprovider/export_logs/cfip-log.txt",
    "saf_uri": "content://com.android.externalstorage.documents/tree/...",
    "mime_type": "text/plain",
    "size_bytes": 1048576,
    "can_share": true
  }
}
```

**前端交互**：
- 导出成功后展示「分享日志」按钮，调用 `Cfst.shareFile(contentUri, mimeType)`
- 系统弹出分享面板，用户可选择邮件、微信、文件管理器等任意应用
- 接收方通过 content URI 直接读取，无需存储权限

### 4.4 Logcat 日志采集

当前诊断包仅包含 Go 侧 `cfip-log.txt`，缺少 Android 原生层崩溃和异常。新增 Logcat 采集：

```kotlin
// AndroidLogcatCollector.kt
object AndroidLogcatCollector {
    fun collect(context: Context, maxLines: Int = 500): File {
        val outputFile = File(context.cacheDir, "export/logcat.txt")
        outputFile.parentFile?.mkdirs()
        val process = Runtime.getRuntime().exec(arrayOf(
            "logcat", "-d", "-t", maxLines.toString(),
            "-v", "time",
            "CfstPlugin:V", "ProbeForegroundService:V", "*:S"
        ))
        outputFile.outputStream().use { process.inputStream.copyTo(it) }
        process.waitFor(5, TimeUnit.SECONDS)
        return outputFile
    }
}
```

诊断包导出时自动包含 `logs/logcat.txt`，便于排查原生层崩溃。

### 4.5 「打开日志目录」增强

当前 `OpenLogDirectory` 仅返回 URI 不实际打开。优化为：

- 若已配置 SAF 导出目录：使用 `ACTION_VIEW` 打开该目录的文档 URI
- 若未配置：引导用户先选择导出目录
- 新增 `OpenLogFile` 方法：直接用 `ACTION_VIEW` + FileProvider content URI 打开最新日志文件，系统自动选择文本阅读器

---

## 5. 维护性优化设计

### 5.1 存储路径统一管理

将分散在各文件中的路径常量收敛到 `StorageLayout`：

**当前问题**：
- `storage.go`：`desktopDraftFileName`、`sourceProfilesFileName`、`storageBootstrapFileName`
- `storage_layout.go`：`ConfigFileName`、`DraftFileName` 等字段
- `diagnostic_service.go`：硬编码 `"cfip-log.txt"`、`"error-log.txt"`

**优化方案**：
```go
// internal/appcore/storage_paths.go — 唯一路径常量源
const (
    FileConfig         = "config.json"
    FileDesktopDraft   = "desktop-draft.json"
    FileScheduler      = "scheduler.json"
    FileSourceProfiles = "source-profiles.json"
    FileDebugLog       = "cfip-log.txt"
    FileErrorLog       = "error-log.txt"
    FileBootstrap      = "storage.json"
    DirTasks           = "tasks"
    DirLogs            = "logs"
    DirExports         = "exports"
    DirBackups         = "backups"
)

func (l StorageLayout) DebugLogPath() string {
    return filepath.Join(l.HotLogsDir, FileDebugLog)
}
func (l StorageLayout) ErrorLogPath() string {
    return filepath.Join(l.HotLogsDir, FileErrorLog)
}
func (l StorageLayout) ConfigPath() string {
    return filepath.Join(l.WarmConfigDir, FileConfig)
}
// ... 其余路径方法
```

所有业务代码通过 `layout.ConfigPath()` 等方法获取路径，禁止硬编码文件名。

### 5.2 存储健康检查增强

当前 `checkStorageHealthForPath` 仅检查根目录可写。增强为分层检查：

```go
type StorageTierHealth struct {
    Tier       string `json:"tier"`
    Path       string `json:"path"`
    Exists     bool   `json:"exists"`
    Writable   bool   `json:"writable"`
    UsedBytes  int64  `json:"used_bytes"`
    FileCount  int    `json:"file_count"`
    OldestFile string `json:"oldest_file,omitempty"`
}

type StorageHealth struct {
    // ... 现有字段
    Tiers []StorageTierHealth `json:"tiers"`
    TotalUsedBytes int64       `json:"total_used_bytes"`
}
```

前端设置页展示各层使用量，帮助用户判断是否需要清理。

### 5.3 配置版本化

配置写入时自动备份前一版本到 `cold/backups/config-<timestamp>.json`：
- 保留最近 10 份
- 提供「恢复配置」功能，从备份列表选择恢复
- 诊断包自动包含最近 3 份配置备份

### 5.4 接口契约稳定

`StorageLayout` 的路径方法作为公共 API，新增字段时：
- 新路径方法有默认值（向后兼容）
- 废弃方法标注 `// Deprecated:` 并保留至少 2 个版本
- Android 端 `AndroidStorageBridge` 中的路径常量同步收敛

---

## 6. 性能优化设计

### 6.1 启动性能

| 优化点 | 方案 | 预期收益 |
|--------|------|----------|
| 存储目录创建 | 启动时仅创建热层目录，温/冷层惰性创建 | 启动时 syscall 减少 60% |
| 配置读取 | 热缓存配置快照，启动时一次读取，后续内存访问 | 避免频繁磁盘读 |
| 日志初始化 | 懒初始化：首次 `Event` 调用时才打开文件 | 无调试需求时零开销 |
| 迁移检测 | 异步执行，不阻塞启动；迁移进度通过事件通知 | 大存储库启动不卡顿 |

### 6.2 运行时性能

**任务快照写入优化**：
- 当前每次状态变更都全量写 JSON 文件
- 优化为：内存中维护快照，每 500ms 批量刷盘（debounce）
- 任务结束时强制刷盘，确保不丢失

**日志写入优化**（见 3.3）：
- 缓冲写入 + 锁外序列化 + 对象池

**诊断包构建优化**：
- 当前全量读取所有文件到内存再打包
- 优化为流式写入：`zip.Create` 后直接 `io.Copy` 文件流，不经过内存缓冲
- 大文件（>10 MiB）使用 `zip.RegisterCompressor` 启用存储模式（不压缩），减少 CPU

### 6.3 存储清理策略

新增 `StorageCleanupService`，在应用空闲时（如探测任务结束后）执行：

```go
type CleanupPolicy struct {
    TaskArchiveRetentionDays int  // 任务归档保留天数，默认 30
    DiagnosticRetentionDays  int  // 诊断包保留天数，默认 7
    LogRotationMaxFiles      int  // 日志轮转保留数，默认 5
    BackupMaxFiles           int  // 配置备份保留数，默认 10
}
```

清理操作异步执行，不阻塞用户操作，清理结果记录到日志。

---

## 7. 实施路线图

### Phase 1：基础架构（1-2 天）

- [ ] 重构 `StorageLayout` 为三层模型，新增路径方法
- [ ] 创建 `storage_paths.go` 统一路径常量
- [ ] 实现双路径兼容读取（新路径优先，旧路径回退）
- [ ] 编写 `StorageLayout` 单元测试

### Phase 2：日志系统（2-3 天）

- [ ] `DebugLogger` 改为追加写 + 写缓冲
- [ ] 实现日志轮转（大小阈值 + 启动轮转）
- [ ] 错误日志整合进 `DebugLogger`
- [ ] 日志序列化性能优化（对象池 + 锁外序列化）
- [ ] 编写日志轮转与缓冲单元测试

### Phase 3：Android 导出（2-3 天）

- [ ] Go 侧 `debug.export` 支持临时文件模式
- [ ] Kotlin 侧 `writeDebugLogExportToURI` 支持文件流复制
- [ ] `file_paths.xml` 新增 export 和 logs 目录映射
- [ ] 导出响应新增 `content_uri` 字段
- [ ] 新增 `shareFile` Plugin 方法
- [ ] Logcat 采集集成到诊断包
- [ ] `OpenLogDirectory` / `OpenLogFile` 增强

### Phase 4：维护性与清理（1-2 天）

- [ ] 存储健康检查增强（分层用量统计）
- [ ] 配置版本化备份
- [ ] `StorageCleanupService` 实现
- [ ] 任务快照 debounce 刷盘
- [ ] 诊断包流式构建优化

### Phase 5：迁移与验证（1 天）

- [ ] 实现惰性迁移逻辑
- [ ] Android 端存储迁移集成
- [ ] 端到端测试：桌面端 + Android
- [ ] 性能基准测试（日志写入、大日志导出、启动时间）
- [ ] 更新 `docs/android-mobile.md` 和 `docs/configuration.md`

---

## 8. 验收标准

### 功能验收

- [ ] 桌面端与 Android 端均使用三层存储布局
- [ ] 日志不再因启动探测而清空，历史日志可追溯
- [ ] 日志文件超过 10 MiB 自动轮转，保留最近 5 份
- [ ] Android 端导出 50 MiB 日志无 OOM，导出时间 < 3 秒
- [ ] 导出日志后可通过「分享」按钮发送到其他应用，接收方可正常读取
- [ ] 诊断包包含 Go 侧日志 + Logcat 日志 + 配置快照 + 任务快照
- [ ] 旧版本数据可无缝迁移到新布局，无数据丢失

### 性能验收

- [ ] 日志写入吞吐 ≥ 10,000 条/秒（当前约 2,000 条/秒）
- [ ] 应用启动时存储初始化耗时 < 50ms（当前约 150ms）
- [ ] 诊断包构建（含 10 MiB 日志）内存峰值 < 50 MiB
- [ ] 任务快照写入 CPU 占用降低 50%+

### 维护性验收

- [ ] 所有存储路径通过 `StorageLayout` 方法获取，无硬编码文件名
- [ ] `StorageLayout` 公共 API 有完整文档注释
- [ ] 分层存储布局有架构图和目录说明文档
- [ ] 迁移逻辑有回滚方案，迁移失败不影响旧路径读取
