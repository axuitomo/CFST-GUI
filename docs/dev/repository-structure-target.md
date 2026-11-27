# 仓库结构目标与迁移路线

本文定义 CFST-GUI 的目标命名范式、模块连接方向和渐进迁移路线。它是后续代码变更的归位与迁移依据，不代表需要立即搬迁目录。现有结构和跨端契约仍以[架构约束](architecture-constraints.md)为准；两者不一致时，先更新决策文档并验证影响，再实施迁移。

## 目标原则

按“业务域 × 技术层 × 平台适配”组合组织代码：业务域表达产品能力，技术层表达依赖和职责，平台适配隔离运行环境差异。只在一个维度确实产生边界时才拆目录，不建立空壳目录，也不要求每个业务域都具备完整的分层矩阵。

演进顺序保持不变：先按职责分段；业务域开始独立变化时再按域归拢；只有独立部署、数据所有权或团队治理成为实际需求时才拆服务。目录整理本身不是拆服务的理由。

## 命名规则

- 业务域使用简短、稳定的业务名词，如 `probe`、`config`、`update`、`archive`。
- 技术层使用职责名词，如 `core`、`app`、`store`、`transport`。只有代码量和依赖边界足以支撑时才在业务域内增加子目录。
- 平台适配使用实际平台或通道名，如 `desktop`、`webui`、`android`、`cli`。共享逻辑不得以平台名命名。
- Go 包名使用小写、简短且无冗余后缀；路径应表达唯一职责。保留已形成稳定含义的现有包名，不为命名对齐批量改名。
- 前端继续使用 Vue 生态约定的 `views`、`components`、`composables`、`lib`，不另造 Go 风格层名。

## 目标结构

```text
.
├── main.go                              # 桌面/WebUI 薄入口与资源嵌入（保持 //go:embed 契约）
├── internal/
│   ├── app/                             # L4 桌面/WebUI 端侧适配器（Wails、WebUI HTTP、托盘、更新安装）
│   ├── kernel/                          # 后端共享内核（Desktop / WebUI / Android / Installer 共用）
│   │   ├── appcore/                     # L3 应用编排（唯一有状态 Service、命令派发、任务/调度/历史/诊断）
│   │   ├── domain/                      # L2 领域能力（无 UI/平台依赖）
│   │   │   ├── archivecore/             # 配置归档与 WebDAV
│   │   │   ├── config/                  # 配置域契约与值转换
│   │   │   ├── dns/                     # DNS 域（alidnscore、cloudflarecore、dnshecore、dnspodcore、dnspush、doh）
│   │   │   ├── github/                  # GitHub 域（githubcore、githubdownload）
│   │   │   ├── probe/                   # 探测域（colodict、mcis、probecore、task）
│   │   │   ├── sourceparse/             # 多域复用的输入源解析能力
│   │   │   └── updatecore/              # 更新版本比较与资产匹配
│   │   └── infra/                       # L1 基础设施（httpcfg、httpclient、runtimecleanup、sqliteprobe、utils）
│   └── contracttest/                    # 跨端契约测试（同时驱动 internal/app 与 mobileapi）
├── cmd/installer/                       # L4 Windows 安装器端侧壳层
├── mobileapi/                           # L4 Android Go bridge 传输适配器
├── sqlitegate/                          # L4 Android SQLite 验证入口
├── mobile/android/                      # Android 原生适配壳（Capacitor + Kotlin）
├── tools/cfst-probe-worker/             # 可选 Cloudflare Worker 探测探针
├── tests/                               # Playwright 端到端与视觉布局回归
└── frontend/src/
    ├── views/                           # 页面编排
    ├── components/                      # UI 组件
    ├── composables/                     # Vue 状态逻辑
    └── lib/
        ├── bridge.ts                    # 唯一对外门面（views/components/composables 仅从此导入）
        ├── bridge/                      # 平台无关共享桥接（命令信封、数据归一化、配置契约、域 API）
        └── adapters/                    # 显式三端前端适配器（desktop / webui / android + registry）
```

## 模块连接方向

```text
桌面/WebUI ──> internal/app ──┐
                              ├──> internal/kernel/appcore ──> kernel/domain ──> kernel/infra
Android bridge ──> mobileapi ─┘
Android 原生壳 ──> mobileapi / 前端适配器
前端 views/composables ──> lib/bridge.ts ──> lib/bridge/* ──> lib/adapters/{registry,desktop,webui,android}
```

依赖和调用约束：

1. 入口只组装资源并启动平台适配，不承载业务规则。
2. 平台适配把输入转换为稳定的应用命令，调用 `internal/kernel/appcore`；Android Go bridge 只做薄传输并持有共享服务。
3. `internal/kernel/appcore` 负责跨平台用例编排，可调用 `kernel/domain` 与 `kernel/infra`；领域层与基础设施层不得反向依赖 `appcore`、UI、Wails、Capacitor 或 gomobile。
4. 业务域之间优先通过小型、稳定的领域函数或应用用例协作。`kernel/infra` 可被多个域调用，但不得反向导入任何业务域或通过 `utils` 传递业务状态。
5. 平台专属操作由平台适配执行。更新规则可共享，安装、窗口、文件授权等动作留在对应平台。
6. 前端页面只编排视图和状态；平台判断与三端运行时（Wails / WebUI / Capacitor）完全收口在 `frontend/src/lib/adapters/`，对外通过 `frontend/src/lib/bridge.ts` 门面暴露。
7. 配置、命令、事件、存储路径和发布清单都是兼容契约。目录移动不能隐式改变字段、行为或序列化格式。

禁止形成反向依赖环：平台适配 → 应用编排 → 领域能力 → 基础设施是唯一合法方向。

## 当前结构映射

| 目标职责 | 当前目录/包 | 说明 |
| --- | --- | --- |
| 入口与资源 | 根目录 `main.go`、`resources.go`、`frontend_assets.go`、`tray_icon*.go` | 保持薄入口与 `//go:embed` 路径稳定，不新增根级业务包。 |
| 桌面/WebUI 适配 | `internal/app` | 承载 Wails、WebUI 生命周期与平台操作，保留 Wails bindings 稳定路径。 |
| Windows 安装器适配 | `cmd/installer` | 独立 Windows 安装器二进制与自绘 UI。 |
| 跨端应用编排 | `internal/kernel/appcore` | 唯一有状态共享服务及用例编排；跨端行为优先落在此处或更底层内核包。 |
| 配置域 | `internal/kernel/domain/config` | 配置 schema 与值转换，保持导入别名与函数 API 不变。 |
| 探测域 | `internal/kernel/domain/probe/{probecore,task,mcis,colodict}`、`internal/kernel/domain/sourceparse` | 探测规则、执行引擎、批量抽样、COLO 字典与输入源解析。 |
| DNS 域 | `internal/kernel/domain/dns/{alidnscore,cloudflarecore,dnshecore,dnspodcore,dnspush,doh}` | DNS 平台适配、对账同步引擎与 DoH 解析。 |
| 更新与归档/导出域 | `internal/kernel/domain/{updatecore,archivecore}`、`internal/kernel/domain/github/{githubcore,githubdownload}` | 共享更新规则、配置归档与 GitHub 导出/下载解析。 |
| 基础设施 | `internal/kernel/infra/{httpcfg,httpclient,runtimecleanup,sqliteprobe,utils}` | 零业务语义技术底座。 |
| Android Go bridge 与原生端 | `mobileapi`、`sqlitegate`、`mobile/android` | 保持桥接协议稳定；原生权限、前台服务、SAF 和安装能力留在 Android 侧。 |
| 前端与三端适配器 | `frontend/src/views`、`components`、`composables`、`lib/bridge*`、`lib/adapters/{desktop,webui,android}` | 共享 UI 与域 API 通过统一 `PlatformAdapter` 接口委托到三端适配器，不复制平台分支。 |

表中映射描述职责对应关系，不表示所有包都要搬到目标示意树中。Go 的 `internal` 可见性、现有导入路径和外部工具调用也是迁移成本，必须纳入评估。

## 渐进迁移路线

### 阶段 0：按目标归位新改动

从现在开始，新代码按上述职责和连接方向放置。扩展已有能力时先检查对应现有包；不创建空目录、不做无行为收益的全仓改名。每个跨层改动都验证依赖方向和跨端兼容。

### 阶段 1：随业务变更局部归拢

当某个业务域持续出现同域文件分散、边界难以理解或依赖方向被破坏的问题时，在该域的正常功能变更中局部归拢。一次迁移尽量保持包 API、行为和持久化格式不变；调用方同步更新，不引入长期兼容转发包。

### 阶段 2：验证并收紧边界

迁移后补充或调整最小必要的包边界测试，验证禁止依赖仍被禁止、跨端行为基线未变，并运行受影响平台的构建与测试。若拆分导致更多循环依赖、跨包跳转或重复状态，则回到原有职责切分，不为符合目录树而强行保留。

### 阶段 3：仅在组织需求出现时拆部署单元

只有当业务域需要独立发布/扩缩容、独立数据所有权或明确团队治理边界时，才评估进程、模块仓库或服务拆分。单纯代码量增长、目录变多或未来可能复用均不足以触发服务化。拆分前先定义 API、数据所有权、失败语义、版本兼容与迁移路径。

## 迁移检查清单

每次计划目录迁移时，在变更说明中明确：

- 现有包到目标职责的映射，以及不迁移的原因。
- 迁移前后的 import/调用方向，确认没有平台依赖泄漏或循环依赖。
- 配置、命令、事件、bridge、文件路径和发布资产等契约是否保持不变。
- 需要同步移动或更新的测试、构建标签、生成代码、脚本和文档。
- 迁移后验证命令与回滚边界；纯结构迁移不得夹带无关行为重写。

默认使用小步、可独立验证的迁移；对外契约变化必须单独设计兼容与迁移方案，不得作为目录重排的副作用。
