# 架构约束

本文档定义 CFST-GUI 的仓库分层、代码归位和跨端契约约束。新增模块、移动代码、修改配置或桥接接口前，应先确认本文件中的边界。

## 分层边界

仓库采用“薄入口 + 平台适配 + 共享核心 + 底层能力”的分层。

| 层级 | 目录 | 职责 |
| --- | --- | --- |
| 启动入口 | `main.go`、`resources.go`、`frontend_assets.go`、`tray_icon*.go` | 只负责资源注入、build tag 适配和调用 `internal/app.Run`。 |
| 应用与平台适配 | `internal/app` | Wails 桌面、Linux WebUI、CLI 分发、配置读写、事件推送、发布更新和平台差异封装。 |
| 跨端应用核心 | `internal/appcore` | 桌面、WebUI、Android 可复用的应用业务能力、数据转换、配置归档、输入源、上传筛选和 pipeline 运行逻辑。 |
| 领域核心 | `internal/*core`、`internal/probecore`、`internal/sourceparse`、`internal/httpcfg`、`internal/httpclient`、`internal/colodict`、`internal/mcis` | 可测试、可复用、无 UI 依赖的业务和基础能力。 |
| 底层探测与工具 | `internal/task`、`internal/utils` | CFST 探测阶段、CSV、调试日志、进度和数值工具；仅供本仓库内部使用，不作为公共 API。 |
| Android Go bridge | `mobileapi` | gomobile 暴露给 Android Java 层的服务外壳，优先复用 `internal/appcore` 和领域核心。 |
| Android 原生壳 | `mobile/android` | Capacitor、Java Plugin、前台服务、权限、SAF 文件访问和 Gradle 工程。 |
| 前端 | `frontend/src` | Vue UI、三端 bridge 适配和浏览器端状态编排。 |

`internal/app` 和 `mobileapi` 可以做平台适配，但不应复制核心业务规则。跨端共享行为应先进入 `internal/appcore` 或更底层的 `internal/*core` 包，再由平台层调用。

## Go 代码归位

新增 Go 代码按职责放置：

- 跨平台业务规则、配置转换、上传筛选、归档和 pipeline 能力优先放入 `internal/appcore`。
- 探测配置、阶段流程、结果裁剪和输入源构建优先放入 `internal/probecore`。
- HTTP 协议、请求 profile、DNS、GitHub、归档、COLO 字典等领域能力放入对应 `internal/*core` 或已有专用包。
- Wails/WebUI/CLI 独有编排留在 `internal/app`；Android 独有 bridge 留在 `mobileapi` 或 `mobile/android`。
- CFST TCP、trace、HTTPing、下载测速和重试策略留在 `internal/task`。
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
- Wails bridge 方法、WebUI `/api/*` 分发、Capacitor `Cfst` plugin 方法和返回字段。
- 事件名和事件 payload，尤其是 `desktop:probe` 及任务状态快照。
- 存储目录、便携模式、结果文件、输入源档案和调试日志路径。
- 发布产物名称、版本注入、update manifest 字段和平台安装模式。

变更跨端契约时，应同步更新测试、`README.md` 或 `docs/` 中对应主题文档，并保留旧数据或旧调用方的兼容路径。

## 验证入口

Go 包枚举必须使用项目脚本提供的过滤逻辑，避免裸 `go test ./...` 扫到 `frontend/node_modules` 中依赖自带的 Go 文件。

推荐命令：

```bash
bash scripts/check.sh

# 仅运行 Go 测试时
bash -lc 'source scripts/lib/common.sh; go test $(cfst_go_packages)'
```

文档变更运行：

```bash
bash scripts/docs-check.sh
```

前端变更至少运行：

```bash
cd frontend
npm run typecheck
```
