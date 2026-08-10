# 跨端行为基线

本文档概括桌面、WebUI、Android 和 CLI 共享核心重构后的行为基线。基线用于防止平台适配层重新产生业务分叉，不替代各功能的细粒度单元测试。

## 固定契约

- 共享业务由 `internal/appcore.Service` 承载；Wails、WebUI 和 gomobile 只负责命令传输及平台能力适配。
- 共享命令使用稳定的分域 ID 和 snake_case payload。返回 envelope 的 schema 固定为 `cfst-gui-command-v2`，包含 `code`、`data`、`message`、`ok`、`task_id` 和非空 `warnings` 数组。
- 业务事件统一通过 `probe:event` 传输，事件 envelope 的 schema 固定为 `cfst-gui-event-v2`。同一任务的序号单调递增，`completed`、`cancelled` 或 `failed` 终态只提交一次。
- 配置读取兼容 v1、无 envelope 和旧字段；读取本身不改写文件，首次成功保存或导入时写入 v2，并为旧配置保留一次性 `.v1.bak`。
- 每个 `Service` 同时最多运行一个探测任务，不同 `Service`、`task.Engine`、输出目录、筛选条件和调试日志互相隔离。

## Golden 覆盖

共享 fixture 位于 `internal/contracttest/testdata/shared_behavior.golden.json`，由 `internal/contracttest/shared_behavior_golden_test.go` 消费。目前固定以下代表性输入与输出：

| 场景 | 固定内容 |
| --- | --- |
| 配置迁移 | 旧配置读取结果、尾随内容兼容信息和 v1 备份 |
| 命令 | 非法 payload、暂停、取消、恢复和任务查询的 code/ok/task_id |
| 分端口探测 | 输入源端口覆盖全局端口后的稳定分组与排序 |
| 上传 | 共享筛选结果、Cloudflare/GitHub 各自 Top N 和 warnings |
| 调度 | interval 与 daily time 共同存在时的下一次执行时间 |
| 恢复 | 进度、暂停、恢复、完成事件归并后的持久化任务快照 |
| 事件序列 | schema、task_id、时间戳、递增序号和终态唯一性 |

`internal/contracttest/invoke_test.go` 另外比较桌面与 gomobile 适配器对相同共享命令的完整 `CommandResult`，避免平台层改写错误码或返回结构。WebUI 的命令路由、鉴权与 SSE 由 `internal/app/webui_test.go` 覆盖。

## 允许的平台差异

以下能力保留在平台适配层，不要求产生完全相同的系统交互：

- Android 的 SAF 发布、WorkManager、前台服务、电池与通知权限、APK 安装。
- 桌面的窗口、托盘、文件选择器和安装更新。
- WebUI 的 HTTP 鉴权、SSE 连接和受限文件访问。

差异只限于能力触发和传输。配置净化、探测阶段、任务状态、上传筛选、调度决策和业务事件仍以共享核心结果为准。

## 验证与维护

在仓库根目录使用 Windows PowerShell 运行共享基线：

```powershell
go test ./internal/contracttest ./internal/appcore ./internal/task
go test -race ./internal/task ./internal/appcore
pnpm typecheck
```

Windows 本地执行 `-race` 需要启用 CGO 并安装兼容的 C 编译器；缺少该工具链时，应由配置完整的开发机或 CI 补跑，不能把普通 `go test` 视为竞态检查的替代。

修改命令、错误码、事件、配置迁移、端口策略、上传筛选、调度或任务恢复时，应先判断这是兼容修复还是有意变更。只有确认新行为是新的跨端契约后才更新 golden，并同步相关接口或配置文档；不要用更新 fixture 的方式掩盖非预期回归。

完整发布验收还包括 `pnpm build`、Windows Wails 构建、WebUI 构建、gomobile AAR、Gradle debug APK、16KB page alignment 和 APK manifest 检查，具体入口见 [部署与构建](deployment.md)。
