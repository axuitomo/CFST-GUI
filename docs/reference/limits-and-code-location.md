# 已知限制与代码定位

> 本页属于 [参考文档](./README.md) 的一部分，汇总当前已知限制与注意事项，并提供按主题定位源码的参考表。

## 1. 已知限制与注意事项

| 项目 | 当前状态 |
| --- | --- |
| REST API | 桌面 Wails/CLI 不开放 HTTP 端口；Linux WebUI 提供 `/api/health`、`/api/command/{command}`、`/api/platform/{command}`、`/api/events/probe`、`/api/files/*` |
| DNS 推送 | DNS 读取页不修改线上记录；手动结果推送、定时任务和测速后自动推送会真实调用 Cloudflare API 创建、更新和删除匹配记录 |
| 暂停/终止/恢复 | `probe.pause` 在可恢复安全点进入暂停等待；`probe.cancel` 取消整条任务上下文并最终发出唯一的 `probe.cancelled`；`probe.resume` 仅能唤醒可恢复暂停任务 |
| 冷却/重试 | 重试退避和连续失败冷却均响应终止；暂停会冻结剩余等待时间，恢复后继续计时；阶段 3 仍会在单 IP 时长内自动续连短文件完成或临时断流 |
| 任务历史 | 桌面、WebUI 和 Android 都会持久化任务快照和结果，并在启动时恢复最新任务；只有仍绑定当前进程/Android 原生服务的运行态可以继续，失联快照会标为 `failed/persisted_only` |
| 配置敏感信息 | JSON/ZIP 导出、WebDAV 备份和本地备份可能包含完整 Cloudflare Token、WebDAV 凭据和本地路径 |
| WebUI 文件访问 | 只能访问 `/data`、`storageRoot()` 和 `CFST_WEBUI_ALLOWED_ROOTS` 指定的受限根目录 |
| Android 导出 | Android 文件选择、导入和结果保存依赖系统 SAF；在线更新需要新旧 APK 使用同一签名 |
| Wails 生成代码 | `frontend/bindings` 可能需要通过 `wails3 generate bindings` 生成 |
| 发行版平台 | 发布 Windows amd64 EXE、Linux amd64/arm64 WebUI tar.gz、Android arm64-v8a Release APK，不发布 macOS 或 iOS 资产 |
| Go 工具链 | `go.mod` 固定 `go 1.27.0`，本地与 CI/CD 均要求 Go 1.27.0 |
| 速度单位 | 后端 `downloadSpeedMb` 按 MB/s 计算；前端文案展示为 MB/s，旧字段名 `download_mbps` 仅作兼容保留 |
| 追踪延迟 | 阶段 2 GET `/cdn-cgi/trace` 的耗时会进入 `traceDelayMs`，当前结果页展示，CSV 保持旧列格式 |

## 2. 代码定位参考

| 主题 | 文件 |
| --- | --- |
| GUI 启动入口 | `main.go`、`internal/app/run.go` |
| 资源注入与嵌入 | `resources.go`、`frontend_assets.go`、`tray_icon*.go`、`internal/app/resources.go` |
| Wails 窗口和绑定 | `internal/app/gui.go` |
| Linux WebUI API | `internal/app/webui.go`、`internal/app/webui_event_hub.go`、`internal/app/app_webui.go` |
| 共享业务命令与有状态 Service | `internal/appcore/service.go`、`internal/appcore/invoke.go` |
| 配置归档和 WebDAV | `internal/appcore/archive_service.go`、`internal/archivecore/` |
| 储存布局和档案 | `internal/appcore/storage_layout.go`、`internal/appcore/profiles.go` |
| 输入源读取和 MICS抽样 | `internal/appcore/source_service.go`、`internal/appcore/source_entries.go` |
| COLO 字典 | `internal/app/desktop_colo_dictionary.go`、`internal/colodict/` |
| 共享事件与 Wails 传输 | `internal/appcore/events.go`、`internal/app/probe_events_wails.go` |
| 前端 bridge | `frontend/src/lib/bridge.ts` |
| 前端页面编排 | `frontend/src/App.vue` |
| 任务看板 UI | `frontend/src/views/DashboardView.vue` |
| 输入源 UI | `frontend/src/views/SourcesView.vue` |
| 系统配置 UI | `frontend/src/views/SettingsView.vue` |
| DNS 读取页 UI | `frontend/src/views/DnsView.vue` |
| HTTP 请求配置 | `internal/httpcfg/profile.go`、`internal/task/request_profile.go` |
| TCPing | `internal/task/tcping.go` |
| 追踪探测 | `internal/task/head.go`、`internal/task/colo.go` |
| 下载测速 | `internal/task/download.go` |
| CSV 和结果输出 | `internal/utils/csv.go` |
| Android bridge | `mobile/android/app/src/main/java/io/github/axuitomo/cfstgui/CfstPlugin.kt`、`mobileapi/` |
| 在线更新 | `internal/app/update.go`、`.github/workflows/release.yml`、`scripts/build/build-release.sh` |
