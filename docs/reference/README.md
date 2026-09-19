# CFST-GUI 参考文档

> 本文件是 `docs/reference/` 的入口：只保留文档说明与导航，功能、接口、配置等细节按主题拆分到下方聚焦文档，按需阅读。

本文档基于当前仓库源码整理 CFST-GUI 的功能、运行链路和接口边界。当前项目是一个基于 Wails v3.0.0-beta.20 + Vue 的桌面端 Cloudflare/CDN IP 测速工具，后端 Go 目标版本为 1.27.0。

当前接口边界如下：

- 桌面 GUI 默认通过 Wails 启动，共享业务经 `window.go.app.App.Invoke(command, payloadJSON)` 调用；窗口、托盘和更新等平台能力仍使用明确的 Wails 方法。
- Linux WebUI 通过 `webui` build tag 提供本地 HTTP Server、REST API、SSE 进度事件和浏览器页面路由；发行时提供 `linux/amd64` 与 `linux/arm64` 两种 bundle，并同时支持 Docker Compose 与 `run-local.sh` 本地运行。
- 文档中的“接口”主要指三端统一命令、平台适配命令、WebUI HTTP API、前端 bridge、`probe:event` 事件、配置结构以及外部 HTTP 请求。

## 文档地图

| 场景 | 文档 |
| --- | --- |
| 功能总览、探测策略、DNS 读取链路和 GUI/探测/预览运行链路 | [功能与运行链路](./capabilities-and-flows.md) |
| 三端统一命令、返回结构、能力矩阵、WebUI HTTP API、前端 Bridge 和 `probe:event` 事件 | [接口契约](./api-contracts.md) |
| 配置文件位置、配置快照字段、输入源结构和配置归档 | [配置与数据结构](./configuration-and-data.md) |
| 对外部 HTTP 与 Cloudflare DNS API 的调用，以及输入、输出形态 | [外部接口与输入输出](./external-interfaces.md) |
| 已知限制与注意事项、按主题定位源码的参考表 | [已知限制与代码定位](./limits-and-code-location.md) |

## 关键入口文件

| 文件 | 作用 |
| --- | --- |
| `main.go` | 根目录薄入口，注入嵌入资源并调用 `internal/app.Run` |
| `resources.go`、`frontend_assets.go`、`tray_icon*.go` | 根目录资源桥接，保持 `frontend/dist` 和 `build/` 图标嵌入路径稳定 |
| `internal/app/run.go` | GUI/CLI 启动判定、CLI 参数到共享 payload 的转换和版本信息 |
| `internal/app/gui.go` | Wails 桌面窗口启动与 `App` 绑定 |
| `internal/app/webui.go` | Linux WebUI HTTP 服务、API 分发、SSE 和受限文件访问 |
| `internal/app/app.go`、`invoke.go` | Wails/WebUI 生命周期和统一命令传输适配 |
| `internal/app/storage.go` | 桌面储存根、旧目录迁移和平台健康检查 |
| `internal/app/desktop_colo_dictionary.go` | 桌面端 COLO 字典路径能力 |
| `internal/appcore/service.go`、`task_service.go` | 唯一共享 Service、探测运行时、任务状态和事件发布 |
| `internal/appcore/invoke.go` | 共享业务命令路由与统一返回结构 |
| `internal/appcore/source_service.go`、`source_entries.go` | 输入源预览、抓取、解析、COLO 过滤和 MICS抽样 |
| `internal/appcore/archive_service.go` | 配置归档、本地备份与 WebDAV 备份还原 |
| `internal/appcore/cloudflare_service.go`、`github_service.go` | Cloudflare DNS 与 GitHub 上传业务 |
| `internal/appcore/task_snapshot.go`、`task_history.go` | 三端共享的任务状态归并、历史快照读取与排序 |
| `frontend/src/lib/bridge.ts` | 三端统一命令调用器和少量平台命令适配 |
| `frontend/src/App.vue`、`frontend/src/composables/useProbeTask.ts` | 页面副作用编排，以及独立的任务状态和操作可用性管理 |
| `mobileapi/` | Android gomobile 薄适配器，公开 `Init`、`SetEventSink` 和 `Invoke` |
| `mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/CfstPlugin.kt` | Capacitor `Cfst` plugin，转发前端调用到 gomobile |
| `internal/task/` | 实例化 `Engine` 及 CFST TCP、追踪、文件测速核心逻辑 |
| `internal/utils/` | CSV 导出、调试日志、进度工具 |

## 结论

CFST-GUI 当前的核心产品形态是 Wails 桌面 GUI、Linux WebUI 和 Android 移动端桥接。三端分别编译和发布，但共用 `internal/appcore.Service`、实例化探测引擎、业务命令、任务状态机和 `probe:event` 事件契约。DNS 读取页已接入 Cloudflare 官方 API 且不修改线上记录；Cloudflare 推送由手动结果推送、定时任务或测速后自动推送链路触发。

## 相关文档

- [配置详解](../guide/configuration.md)：桌面配置、探测策略、导出、备份、Cloudflare 和 UI 字段说明。
- [Docker 与环境变量](../guide/docker-env.md)：WebUI 监听地址、鉴权 token、允许文件根目录和容器部署参数。
- [Android 移动端](../mobile/android-mobile.md)：Capacitor bridge、gomobile 服务、SAF 文件访问和 APK 在线更新注意事项。
- [CLI 使用说明](../dev/cli.md)：运行模式、CLI 参数、输入输出文件和验证命令。
