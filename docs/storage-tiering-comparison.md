# CFST-GUI 多平台存储分层方案对比与选型

> 版本：v1.0 | 日期：2026-08-25 | 目标：从 10 个可行方案中按性能+可维护性加权选出最终方案

---

## 一、方案列举（10 个）

### 方案 A：应用内目录分层（In-App Directory Tiering）

**核心思路**：在应用私有根目录下创建 `hot/`、`warm/`、`cold/` 子目录，纯文件系统实现分层。

```
<app_data>/
├── hot/    # logs/, tasks/, runtime/
├── warm/   # config/, imports/, task-archive/
└── cold/   # exports/, backups/, diagnostics/
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | 所有平台均支持文件系统目录，零平台差异 |
| **热层实现** | `hot/logs/` 追加写日志，`hot/tasks/` 活动任务快照 |
| **温层实现** | `warm/config/` 配置文件，`warm/task-archive/` 已完成任务 |
| **冷层实现** | `cold/exports/` 导出文件，`cold/backups/` 配置备份 |
| **数据迁移** | 惰性迁移：任务完成后从 hot/tasks 移到 warm/task-archive |
| **优点** | 实现最简单，路径直观，无额外依赖，调试方便 |
| **缺点** | 热温冷在同一物理介质，无真实 IO 性能差异；需自行实现清理/轮转；小文件多导致 inode 浪费 |

---

### 方案 B：平台标准目录映射（Platform Standard Directory Mapping）

**核心思路**：利用各操作系统原生的目录语义做分层——热层用系统缓存目录（可被系统自动清理），温层用应用数据目录，冷层用外部/共享存储。

| 平台 | 热层（Cache） | 温层（Files） | 冷层（External） |
|------|---------------|---------------|-----------------|
| Windows | `%LOCALAPPDATA%/CFST-GUI/cache` | `%APPDATA%/CFST-GUI` | `~/Downloads/CFST-GUI` |
| macOS | `~/Library/Caches/CFST-GUI` | `~/Library/Application Support/CFST-GUI` | `~/Downloads/CFST-GUI` |
| Linux | `~/.cache/CFST-GUI` | `~/.config/CFST-GUI` | `~/Downloads/CFST-GUI` |
| Android | `context.cacheDir` | `context.filesDir` | SAF 授权目录 |

| 维度 | 说明 |
|------|------|
| **多平台适配** | 需为每个平台实现路径解析器，但 Go 标准库 `os.UserCacheDir()` / `os.UserConfigDir()` 已覆盖桌面端 |
| **热层实现** | 系统缓存目录，空间不足时系统可自动清理，应用需容忍热数据丢失 |
| **温层实现** | 应用数据目录，系统不会自动清理，随应用卸载而删除 |
| **冷层实现** | 用户下载目录或 SAF 授权目录，应用卸载后保留 |
| **优点** | 利用平台原生存储语义，系统级缓存清理；符合各平台设计规范；热层缓存目录通常有 IO 优化 |
| **缺点** | 平台路径差异大，需维护多套路径逻辑；Android cacheDir 清理时机不可控；冷层在 Android 上需 SAF 授权 |

---

### 方案 C：SQLite + 文件系统混合（Hybrid SQLite + FS）

**核心思路**：热数据（任务快照、运行时状态、日志索引）存入 SQLite WAL 模式，温冷数据（配置、日志文件、导出文件）存文件系统。

```
<app_data>/
├── hot.db            # SQLite WAL: tasks, runtime_state, log_index
├── warm/
│   ├── config.json
│   └── task-archive/  # 已完成任务的完整 JSON
└── cold/
    ├── logs/           # 轮转后的日志文件
    ├── exports/
    └── backups/
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | SQLite 跨平台；Android 有原生 SQLite 支持，桌面端用 `modernc.org/sqlite`（纯 Go，无 CGO）或 `mattn/go-sqlite3` |
| **热层实现** | SQLite WAL 模式：任务快照按 task_id 行存储，支持原子更新、并发读、事务批量写入 |
| **温层实现** | 配置文件 + 任务归档 JSON 文件 |
| **冷层实现** | 轮转日志文件、导出文件、备份文件 |
| **数据迁移** | 任务完成时从 hot.db 导出完整 JSON 到 warm/task-archive/，然后删除 hot.db 中的行 |
| **优点** | 热数据读写比文件系统快 35%（SQLite 官方基准）；小 BLOB 存储节省 20% 磁盘空间；支持事务和索引；WAL 模式支持 1 写 N 读并发 |
| **缺点** | 需维护 SQLite schema 迁移；引入数据库依赖（纯 Go 实现约 10MB 二进制增量）；日志大文本不适合存 SQLite（>100KB 应存文件）；gomobile 绑定 SQLite 需确认 AAR 兼容性 |

---

### 方案 D：MMAP 热层 + 文件温冷层（MMAP Hot Tier + FS）

**核心思路**：热数据（日志、任务快照）使用内存映射文件（mmap），读写接近内存速度；温冷数据用普通文件。

```
<app_data>/
├── hot/
│   ├── log.dat        # mmap 环形缓冲区
│   └── tasks.dat      # mmap 固定大小槽位
├── warm/
└── cold/
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | Go 的 `golang.org/x/exp/mmap` 或 `syscall` 跨平台；Android NDK 支持 mmap |
| **热层实现** | 日志用 mmap 环形缓冲区（固定大小，覆盖写），任务快照用 mmap 固定槽位数组 |
| **温层实现** | mmap 缓冲区 flush 后转为普通文件 |
| **冷层实现** | 普通文件系统 |
| **优点** | 热数据读写零拷贝，接近内存速度；无需序列化/反序列化开销 |
| **缺点** | mmap 管理复杂：需处理同步（msync）、大小调整（mremap）、崩溃一致性；环形缓冲区数据结构需自行设计；调试困难；Android 上 mmap 与 Go runtime 内存管理可能冲突；文件大小受限需预分配 |

---

### 方案 E：LSM-Tree 风格日志分层（LSM-Tree Style Log Tiering）

**核心思路**：借鉴 LevelDB/RocksDB 的 LSM-Tree 思想——热层 WAL 追加写，定期 compaction 合并排序后写入温层 SSTable，冷层归档压缩。

```
<app_data>/
├── hot/
│   ├── wal-001.log    # 追加写 WAL
│   └── wal-002.log
├── warm/
│   ├── sst-001.db     # compaction 后的有序文件
│   └── sst-002.db
└── cold/
    └── archive-001.zst # 压缩归档
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | 纯文件系统实现，跨平台；或直接使用 `github.com/syndtr/goleveldb` |
| **热层实现** | WAL 追加写，内存中维护 memtable 索引 |
| **温层实现** | compaction 合并 WAL 为有序 SSTable，支持布隆过滤器快速查询 |
| **冷层实现** | 旧 SSTable 压缩归档为 zstd |
| **优点** | 写入极快（纯追加）；读取通过索引+布隆过滤器高效；天然支持时间范围查询 |
| **缺点** | 实现复杂度极高：需处理 compaction 失败、部分写入、memtable 恢复、SSTable 合并；compaction 时有 IO 尖峰；对于 CFST-GUI 的数据规模（日志+任务快照）属于过度设计；引入 LevelDB 依赖约 3MB |

---

### 方案 F：统一抽象层 + 平台原生后端（Unified Abstraction + Native Backends）

**核心思路**：定义 `StorageBackend` 接口，每个平台使用最适合的原生存储后端——Android 用 Room(SQLite) + SAF，桌面用文件系统，业务代码只依赖接口。

```go
type StorageBackend interface {
    // 热层
    WriteHotSnapshot(taskID string, data []byte) error
    ReadHotSnapshot(taskID string) ([]byte, error)
    DeleteHotSnapshot(taskID string) error
    // 温层
    WriteConfig(key string, data []byte) error
    ReadConfig(key string) ([]byte, error)
    // 冷层
    WriteExport(name string, data []byte) (string, error)
    ReadExport(path string) ([]byte, error)
    // 管理
    List( tier StorageTier) ([]StorageItem, error)
    Cleanup(tier StorageTier, policy CleanupPolicy) error
}
```

| 后端实现 | 平台 | 热层 | 温层 | 冷层 |
|----------|------|------|------|------|
| `FSBackend` | 桌面全平台 | 文件系统 | 文件系统 | 文件系统 |
| `AndroidBackend` | Android | Room(SQLite) | filesDir | SAF |
| `MemoryBackend` | 测试 | 内存 map | 内存 map | 内存 map |

| 维度 | 说明 |
|------|------|
| **多平台适配** | 每个平台独立实现后端，最大化利用平台原生能力 |
| **优点** | 各平台性能最优；业务代码与存储实现解耦；可独立优化/替换后端；测试方便（MemoryBackend） |
| **缺点** | 需维护多套后端实现（至少 2 套：桌面+Android）；接口设计需覆盖所有场景，抽象不当会导致后端能力浪费或接口膨胀；Android 端 Kotlin 实现与 Go 端通过 gomobile 桥接，接口边界复杂 |

---

### 方案 G：对象存储风格 + 元数据索引（Object Storage Style + Metadata Index）

**核心思路**：所有数据以不可变对象（blob）存储在扁平目录中，元数据（类型、层级、时间戳、大小、hash）存在索引文件或 SQLite 中。按元数据判定层级和执行清理。

```
<app_data>/
├── blobs/
│   ├── a1b2c3d4.json   # 内容寻址，文件名=hash
│   ├── e5f6a7b8.csv
│   └── ...
└── index.db             # SQLite: id, hash, type, tier, created_at, size, ref_count
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | 纯文件系统 + SQLite 索引，跨平台 |
| **热层实现** | 元数据 `tier=hot` 的 blob，访问时更新 `last_accessed` |
| **温层实现** | `tier=warm`，定期未访问的 hot 对象降级为 warm |
| **冷层实现** | `tier=cold`，长期未访问的 warm 对象降级为 cold，可压缩 |
| **优点** | 内容寻址天然去重；元数据索引支持复杂查询（按类型/时间/大小筛选）；清理/迁移只需更新元数据+移动文件；存储管理统一 |
| **缺点** | 每次读写需查索引，增加一次 IO；不可变对象意味着更新=写新对象+删除旧对象，写放大；ref_count 管理需处理并发；对于配置这类频繁更新的数据不适合（应排除在对象存储外） |

---

### 方案 H：时间分区 + 自动归档（Time-Partitioned + Auto-Archive）

**核心思路**：按时间窗口（日/周）分目录，当前窗口为热层，历史窗口自动归档压缩为冷层。配置和草稿等非时间数据单独存放。

```
<app_data>/
├── current/             # 热层：当前时间窗口
│   ├── logs.txt
│   └── tasks/
├── history/
│   ├── 2026-08-24/     # 温层：昨日数据
│   │   ├── logs.txt
│   │   └── tasks/
│   └── 2026-08-23/
├── archive/             # 冷层：超过保留期的压缩归档
│   ├── 2026-07.tar.zst
│   └── 2026-06.tar.zst
└── config/              # 非时间数据：配置、草稿
    ├── config.json
    └── draft.json
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | 纯文件系统，跨平台 |
| **热层实现** | `current/` 目录，跨日时自动滚动 |
| **温层实现** | `history/YYYY-MM-DD/`，保留最近 N 天 |
| **冷层实现** | `archive/YYYY-MM.tar.zst`，按月压缩归档 |
| **优点** | 时间分区直观，符合日志/任务数据的时间特性；归档压缩节省空间；清理简单（删除旧月份归档）；当前窗口文件少，读取快 |
| **缺点** | 配置/草稿等非时间数据需单独处理；跨日滚动时需处理正在写入的文件（原子 rename）；按月归档时需遍历历史目录合并压缩；任务可能跨日，需处理分片 |

---

### 方案 I：内存热层 + 持久化温冷层（In-Memory Hot + Persistent Warm/Cold）

**核心思路**：热数据（运行时状态、活动任务快照）纯内存，定期 checkpoint 到温层；冷层归档。应用启动时从温层恢复热数据。

```
内存:
  activeTasks map[string]*TaskSnapshot  # 热层
  runtimeState *RuntimeState

磁盘:
<app_data>/
├── warm/
│   ├── checkpoint.json   # 最近一次 checkpoint
│   ├── config.json
│   └── task-archive/
└── cold/
    ├── exports/
    └── backups/
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | 内存 + 文件系统，跨平台 |
| **热层实现** | Go 内存 map，读写零 IO，互斥锁保护 |
| **温层实现** | 每 500ms 或状态变更时 debounce checkpoint 到 JSON 文件 |
| **冷层实现** | 文件系统 |
| **数据恢复** | 启动时读取 checkpoint.json 恢复热层内存状态 |
| **优点** | 热数据访问最快（纯内存，纳秒级）；无磁盘写放大；checkpoint 可批量压缩 |
| **缺点** | 崩溃可能丢失最近 500ms 的状态变更；需实现 checkpoint 原子性（写临时文件+rename）；内存占用随活动任务数增长；Android 上进程被系统杀死后需恢复逻辑；调试时内存状态不可直接观察 |

---

### 方案 J：混合方案 — SQLite 热层 + 目录温冷层 + 平台缓存映射（推荐组合）

**核心思路**：综合方案 C（SQLite 热层）、方案 A（目录温冷层）、方案 B（平台缓存映射）的优点——热层用 SQLite WAL 存任务快照和运行时状态，日志用追加写文件+轮转，温层用应用数据目录存配置和任务归档，冷层用平台标准外部目录存导出和备份。

```
桌面端:
<os.UserConfigDir()>/CFST-GUI/
├── hot.db                    # SQLite WAL: tasks, runtime_state
├── logs/                     # 追加写日志 + 轮转
│   ├── cfip-log.txt
│   └── cfip-log.20260824.txt
├── config/                   # 温层：配置、草稿
│   ├── config.json
│   ├── desktop-draft.json
│   └── source-profiles.json
├── task-archive/             # 温层：已完成任务 JSON
├── exports/                  # 冷层：导出文件
├── backups/                  # 冷层：配置备份
└── diagnostics/              # 冷层：诊断包归档

Android:
context.filesDir/
├── hot.db                    # SQLite WAL
├── logs/                     # 日志
├── config/                   # 配置
├── task-archive/             # 任务归档
└── imports/                  # 输入源缓存

SAF 授权目录/                  # 冷层：导出文件（其他应用可读）
├── exports/
└── diagnostics/
```

| 维度 | 说明 |
|------|------|
| **多平台适配** | SQLite 跨平台（纯 Go 实现），目录结构桌面/Android 一致，Android 冷层用 SAF |
| **热层实现** | SQLite WAL 存任务快照（按 task_id 行，支持原子更新、索引查询）；日志用独立追加写文件+大小轮转（日志是顺序写，SQLite 无优势） |
| **温层实现** | 应用数据目录下的 config/、task-archive/、imports/ |
| **冷层实现** | 桌面端用 `~/Downloads/CFST-GUI/`，Android 用 SAF 授权目录；FileProvider 提供 content URI 供其他应用读取 |
| **数据迁移** | 任务完成时从 hot.db 导出 JSON 到 task-archive/，删除 hot.db 行；日志超阈值轮转 |
| **优点** | 热数据用 SQLite 获得 35% 读写性能提升和事务保障；日志用文件系统避免 SQLite 大文本开销；温冷层目录结构简单直观；Android 冷层用 SAF+FileProvider 实现跨应用读取；各层使用最适合的存储技术 |
| **缺点** | 需维护 SQLite schema + 文件系统两套机制；引入 SQLite 依赖；迁移逻辑比纯目录方案复杂 |

---

## 二、评价矩阵

### 评分标准

| 指标 | 权重 | 说明 | 评分范围 |
|------|------|------|----------|
| **读写性能** | 25% | 热数据读写延迟、吞吐量、并发能力 | 1-10 |
| **启动/恢复性能** | 15% | 应用启动时存储初始化耗时、崩溃恢复速度 | 1-10 |
| **空间效率** | 10% | 磁盘占用、小文件 inode 浪费、压缩能力 | 1-10 |
| **实现复杂度** | 20% | 代码量、依赖数量、调试难度、边界情况处理 | 1-10（10=最简单） |
| **多平台一致性** | 15% | 桌面/Android 行为一致性、平台特有代码量 | 1-10 |
| **可演进性** | 15% | 新增数据类型/层级的容易程度、schema 迁移能力、清理策略灵活性 | 1-10 |

**性能总分** = 读写性能×0.5 + 启动恢复×0.3 + 空间效率×0.2
**可维护性总分** = 实现复杂度×0.4 + 多平台一致性×0.3 + 可演进性×0.3
**综合得分** = 性能总分×0.5 + 可维护性总分×0.5

### 评分表

| 方案 | 读写性能 | 启动恢复 | 空间效率 | **性能** | 实现复杂度 | 多平台一致 | 可演进性 | **可维护性** | **综合** |
|------|---------|---------|---------|---------|-----------|-----------|---------|------------|---------|
| A 应用内目录 | 5 | 7 | 4 | 5.4 | 9 | 10 | 6 | 8.4 | **6.9** |
| B 平台标准目录 | 6 | 6 | 5 | 5.8 | 6 | 5 | 7 | 6.0 | **5.9** |
| C SQLite+FS | 9 | 6 | 8 | 8.0 | 6 | 8 | 8 | 7.2 | **7.6** |
| D MMAP热层 | 10 | 4 | 6 | 7.4 | 3 | 6 | 4 | 4.2 | **5.8** |
| E LSM-Tree | 9 | 3 | 9 | 7.2 | 2 | 8 | 7 | 5.3 | **6.3** |
| F 统一抽象层 | 8 | 7 | 7 | 7.6 | 4 | 4 | 9 | 5.3 | **6.5** |
| G 对象存储+索引 | 6 | 5 | 9 | 6.3 | 4 | 8 | 8 | 6.4 | **6.4** |
| H 时间分区 | 6 | 7 | 8 | 6.7 | 7 | 9 | 6 | 7.3 | **7.0** |
| I 内存热层 | 10 | 5 | 5 | 7.5 | 6 | 9 | 6 | 6.9 | **7.2** |
| **J 混合方案** | **9** | **7** | **8** | **8.3** | **6** | **8** | **9** | **7.5** | **7.9** |

### 排名

| 排名 | 方案 | 综合得分 | 性能 | 可维护性 |
|------|------|---------|------|---------|
| 1 | **J 混合方案（SQLite热层+目录温冷+平台缓存）** | **7.9** | 8.3 | 7.5 |
| 2 | C SQLite+FS 混合 | 7.6 | 8.0 | 7.2 |
| 3 | I 内存热层+持久化 | 7.2 | 7.5 | 6.9 |
| 4 | H 时间分区+自动归档 | 7.0 | 6.7 | 7.3 |
| 5 | A 应用内目录分层 | 6.9 | 5.4 | 8.4 |
| 6 | F 统一抽象层+原生后端 | 6.5 | 7.6 | 5.3 |
| 7 | G 对象存储+元数据索引 | 6.4 | 6.3 | 6.4 |
| 8 | E LSM-Tree 风格 | 6.3 | 7.2 | 5.3 |
| 9 | B 平台标准目录映射 | 5.9 | 5.8 | 6.0 |
| 10 | D MMAP 热层 | 5.8 | 7.4 | 4.2 |

---

## 三、最终方案选型：方案 J（混合方案）

### 3.1 选型理由

**性能维度（8.3/10，排名第 1）**：
- 热层任务快照用 SQLite WAL，读写比文件系统快 35%，支持事务和并发读
- 日志用独立追加写文件+轮转，避免 SQLite 大文本的写放大，顺序写性能最优
- 温冷层目录结构简单，读取路径短
- 启动时仅打开 SQLite 连接 + 读取配置，惰性创建目录

**可维护性维度（7.5/10，排名第 1）**：
- SQLite schema 集中管理，迁移有版本化机制
- 文件系统路径通过 `StorageLayout` 统一管理，无硬编码
- 各层职责清晰：SQLite=热数据结构化存储，文件=日志/大对象，目录=温冷分层
- 桌面/Android 目录结构一致，仅冷层路径不同（桌面=Downloads，Android=SAF）
- 可演进性高：新增数据类型只需决定存入 SQLite 还是文件系统

### 3.2 与方案 C 的关键差异

方案 C（SQLite+FS）是方案 J 的子集，但方案 J 额外做了：
1. **日志不进 SQLite**：方案 C 可能把日志也存 SQLite，方案 J 明确日志用追加写文件+轮转（日志是顺序写大文本，SQLite 无优势且有写放大）
2. **冷层用平台标准目录**：方案 C 冷层仍在应用数据目录内，方案 J 冷层用 `~/Downloads`（桌面）和 SAF（Android），实现跨应用读取
3. **FileProvider 集成**：方案 J 明确 Android 冷层通过 FileProvider 提供 content URI，其他应用可直接读取
4. **任务归档分离**：方案 J 已完成任务从 SQLite 导出到 `task-archive/` 目录，避免 SQLite 无限增长

### 3.3 最终架构图

```
┌─────────────────────────────────────────────────────────┐
│                    业务层 (appcore.Service)               │
│  probe_service / task_service / config_service / ...     │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                  存储抽象层 (StorageLayout)               │
│  统一路径管理 + 层级判定 + 清理策略调度                    │
└──────┬───────────────────┬───────────────────┬──────────┘
       │                   │                   │
┌──────▼──────┐    ┌──────▼──────┐    ┌──────▼──────┐
│   热层 Hot   │    │   温层 Warm  │    │   冷层 Cold  │
│             │    │             │    │             │
│ SQLite WAL  │    │  文件系统    │    │  平台外部目录 │
│ - tasks     │    │ - config/   │    │ - exports/  │
│ - runtime   │    │ - imports/  │    │ - backups/  │
│             │    │ - task-arch/│    │ - diagnostics│
│ 日志文件     │    │             │    │             │
│ - cfip-log  │    │             │    │ Android: SAF │
│ - 轮转归档   │    │             │    │ FileProvider │
└─────────────┘    └─────────────┘    └─────────────┘
```

### 3.4 实施优先级

| 阶段 | 内容 | 预期收益 |
|------|------|---------|
| P0 | `StorageLayout` 重构为分层路径管理 + 日志追加写+轮转 | 解决日志丢失问题，奠定分层基础 |
| P1 | SQLite 热层引入（任务快照+运行时状态） | 热数据读写性能提升 35% |
| P2 | Android 冷层 SAF + FileProvider content URI | 日志可被其他应用读取 |
| P3 | 任务归档自动迁移 + 清理策略 | 存储自动管理，防止膨胀 |
| P4 | 配置版本化备份 + 诊断包流式构建 | 可维护性增强 |

### 3.5 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| SQLite 纯 Go 实现（modernc.org/sqlite）与 gomobile AAR 兼容性 | Android 构建失败 | 提前做 AAR 构建验证；备选方案用 `mattn/go-sqlite3`（需 CGO+NDK） |
| SQLite 引入增加二进制体积 | 桌面端 exe 增大 ~5-10MB | 可接受；使用 `modernc.org/sqlite` 纯 Go 实现无 CGO 开销 |
| 任务快照从文件迁移到 SQLite 的数据迁移 | 旧版本用户升级后任务丢失 | 启动时检测旧 tasks/ 目录，惰性导入 SQLite，导入成功后删除旧文件 |
| Android SAF 权限丢失导致冷层写入失败 | 导出失败 | 导出前检查权限，失效时引导用户重新授权；临时写入 filesDir 并提示 |
| SQLite WAL 文件在 Android 上被系统清理 | 热数据丢失 | WAL 文件存在 filesDir（非 cacheDir），系统不会自动清理；定期 checkpoint |

---

## 四、结论

**最终选择方案 J：混合方案（SQLite 热层 + 目录温冷层 + 平台缓存映射）**

该方案在 10 个候选方案中综合得分最高（7.9），性能排名第 1（8.3），可维护性排名第 1（7.5）。它不是单一技术的极端方案，而是根据数据特性选择最适合的存储技术：

- **结构化热数据**（任务快照、运行时状态）→ SQLite WAL，获得事务、索引、并发读写能力
- **顺序写大文本**（日志）→ 追加写文件 + 大小轮转，避免 SQLite 写放大
- **低频配置数据**（config、draft）→ JSON 文件，简单直观
- **跨应用共享数据**（导出、诊断包）→ 平台外部目录 + Android FileProvider content URI

这种"按数据特性选存储"的混合策略，在性能和可维护性之间取得了最优平衡，且为未来演进留足了空间。
