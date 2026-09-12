# 架构约束

本文档定义 CFST-GUI 的仓库分层、代码归位和跨端契约约束。新增模块、移动代码、修改配置或桥接接口前，应先确认本文件中的边界。

## 分层边界

仓库采用“薄入口 + 平台适配 + 共享核心 + 底层能力”的分层。

| 层级 | 目录 | 职责 |
| --- | --- | --- |
| 启动入口 | `main.go`、`resources.go`、`frontend_assets.go`、`tray_icon*.go` | 只负责资源注入、build tag 适配和调用 `internal/app.Run`。 |
| 应用与平台适配 | `internal/app` | Wails/WebUI 生命周期、CLI 参数适配、HTTP/Wails 传输、窗口、托盘和平台文件交互；更新规则由 `internal/updatecore` 提供，安装启动仍留在平台层；业务命令统一转发到共享核心。 |
| 跨端应用核心 | `internal/appcore` | 唯一有状态 `Service`，持有配置仓库、`ProbeRuntime`、任务存储、调度状态和事件发布器，并承载三端共享业务。 |
| 领域核心 | `internal/*core`、`internal/probecore`、`internal/sourceparse`、`internal/httpcfg`、`internal/httpclient`、`internal/colodict`、`internal/mcis` | 可测试、可复用、无 UI 依赖的业务和基础能力；发布版本比较、manifest 和资产匹配属于 `internal/updatecore`。 |
| 底层探测与工具 | `internal/task`、`internal/utils` | 实例化 `task.Engine` 及 CFST 探测阶段、CSV、调试日志和数值工具；仅供本仓库内部使用，不作为公共 API。 |
| Android Go bridge | `mobileapi` | gomobile 薄传输外壳，只公开 `NewService`、`Init`、`SetEventSink` 和 `Invoke`，并持有一个 `appcore.Service`。 |
| Android 原生壳 | `mobile/android` | Capacitor、Kotlin Plugin、前台服务、权限、SAF 文件访问和 Gradle 工程。 |
| 前端 | `frontend/src` | Vue UI、三端 bridge 适配和浏览器端状态编排。 |

`internal/app` 和 `mobileapi` 只能做传输与平台能力适配，不应复制探测、配置迁移、上传或调度规则。跨端共享行为必须先进入 `internal/appcore` 或更底层的 `internal/*core` 包，再由平台层调用。

## Go 代码归位

新增 Go 代码按职责放置：

- 跨平台业务规则、配置转换、上传筛选和归档能力优先放入 `internal/appcore`。
- 探测配置、阶段流程、结果裁剪和输入源构建优先放入 `internal/probecore`。
- HTTP 协议、请求 profile、DNS、GitHub、归档、COLO 字典等领域能力放入对应 `internal/*core` 或已有专用包。
- Wails/WebUI 独有生命周期与平台交互留在 `internal/app`；Android 前台服务、WorkManager、SAF、权限和安装能力留在 `mobile/android`，gomobile 传输留在 `mobileapi`。
- CFST TCP、trace、HTTPing、下载测速和重试策略留在实例化的 `internal/task.Engine`；不得把任务配置、取消 Hook、输出路径或筛选条件放回包级可变状态。
- CSV、调试日志、进度、精度等通用内部工具留在 `internal/utils`。

除非确实需要对外暴露，不要在仓库根目录新增可 import 的 Go 包。根目录应保持为应用入口和资源桥接层。

## 前端边界

前端本轮保持现有目录结构，新增代码遵循以下约束：

- `frontend/src/views` 只做页面级编排、状态连接和视图组织。
- `frontend/src/components` 放可复用 UI 组件；组件不要直接复制 bridge 调用和业务规则。
- `frontend/src/lib` 放 UI 无关的 bridge、命名映射、URL、时间和数据转换工具。
- `frontend/src/composables` 放跨页面复用的 Vue 状态逻辑。
- 三端能力差异必须通过 `frontend/src/lib/bridge.ts` 或相邻适配层收敛，避免页面内分散判断 Wails/WebUI/Capacitor。

## 跨端契约

以下内容是跨端契约，修改时必须考虑桌面、WebUI、Android、CLI、旧配置和发布更新：

- 配置 schema、默认值、字段净化、导入导出、WebDAV 归档和旧版本迁移。
- 共享业务命令使用 snake_case 字段和稳定分域 ID，经 Wails `App.Invoke`、WebUI `POST /api/command/{command}`、gomobile `mobileapi.Service.Invoke` 传输；平台命令不进入业务核心。
- 命令、配置和事件 schema 分别为 `cfst-gui-command-v2`、`cfst-gui-config-v2`、`cfst-gui-event-v2`。
- 业务事件通道固定为 `probe:event`，生命周期事件名使用 `probe.progress`、`probe.completed` 等 snake_case 名称。
- 存储目录、便携模式、结果文件、输入源档案、调试日志路径，以及结果/输入源读取的大小上限。
- 发布产物名称、版本注入、update manifest 字段和平台安装模式。

变更跨端契约时，应同步更新测试、`README.md` 或 `docs/` 中对应主题文档，并保留旧数据或旧调用方的兼容路径。

`internal/appcore/dependency_boundary_test.go` 固定依赖方向：`internal/appcore` 和 `internal/task` 不得导入 Wails、gomobile、`internal/app` 或 `mobileapi`。新增共享能力时必须继续满足该测试。

`internal/contracttest` 固定共享命令结果、配置迁移、分端口策略、上传筛选、调度、任务恢复和事件序列。变更这些行为前先核对 [跨端行为基线](behavior-baseline.md)，只有有意修改契约时才同步更新 golden fixture。

## 验证入口

Go 包枚举必须使用项目脚本提供的过滤逻辑，避免裸 `go test ./...` 扫到 `frontend/node_modules` 中依赖自带的 Go 文件。

推荐命令：

```powershell
pnpm install --frozen-lockfile
pnpm typecheck
pnpm build

# 仅运行 Go 测试时
$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' })
go test $goPackages
```

文档变更运行：

```powershell
rg -n "\[[^]]+\]\([^)]+\)" README.md docs
```

前端变更至少运行：

```powershell
pnpm typecheck
```
