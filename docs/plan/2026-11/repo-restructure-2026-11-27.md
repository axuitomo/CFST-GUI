# 仓库结构整理与大文件拆解记录（2026-11-27）

本记录归档继单体仓库、后端共享内核（`internal/kernel/`）与前端平台适配器（`packages/frontend/src/lib/adapters/`）分层改造之后，围绕仓库卫生、超大文件拆解、兼容性重导出瘦身与文档同步所做的结构整理。所有对外协议、跨端行为与兼容契约均保持不变。

## 1. 工作区清理与忽略规则

- 清理临时目录、测试报告与残留本地构建产物（`.tmp/`、`playwright-report/`、`test-results/`、`build/bin/`、`build/android-build.log`、`build/artifacts/device-apk/`、`devtools/lib/`、`devtools/profiles/`）。
- 同步更新 [`.gitignore`](../../../.gitignore) 与 [`scripts/dev/clean.sh`](../../../scripts/dev/clean.sh)，保证清理脚本与忽略规则覆盖测试和移动端临时产物。

## 2. 前端大文件拆解

- 将 `SettingsView.vue` 从原先 3069 行收口为 171 行页面编排组件，按业务域与构建阶段拆分为 14 个独立文件放入 [`packages/frontend/src/components/settings/`](../../../packages/frontend/src/components/settings/)（全部 SFC 保持 `<script setup vapor lang="ts">`）：
  - 样式与状态基础设施：`settings-theme.css`（136 行）、`settings-layout.css`（295 行）、`types.ts`（101 行）、`useSettingsSections.ts`（94 行）。
  - 分域设置组件：`SettingsGeneralDomain.vue`（305 行）、`SettingsStorageDomain.vue`（147 行）、`SettingsProbeSection.vue`（276 行）、`SettingsProbeStage2.vue`（203 行）、`SettingsProbeStage3.vue`（175 行）、`SettingsDnsProvidersSection.vue`（392 行）、`SettingsNetworkDomain.vue`（249 行）、`SettingsAutomationDomain.vue`（345 行）、`SettingsNotifyDomain.vue`（238 行）、`SettingsSecurityDomain.vue`（264 行）。
- 从 `packages/frontend/src/App.vue`（当前 997 行）抽离 [`AndroidSelectSheet.vue`](../../../packages/frontend/src/components/ui/AndroidSelectSheet.vue)（42 行）与 [`useResultDisplay.ts`](../../../packages/frontend/src/composables/useResultDisplay.ts)（134 行），并更新指向 `internal/adapters/desktop/notify.go` 的过期注释。

## 3. 后端 Go 大文件同包拆解

在不改动包边界与对外 API 的前提下，将超过 650 行的后端非测试 Go 文件按职责同包拆分：

| 包路径 | 拆解前主文件 | 拆解后文件（当前行数） |
| --- | --- | --- |
| `internal/kernel/appcore/notify` | `upload_notification.go` | `upload_notification.go`（386）、`notification_telegram.go`（412）、`notification_format.go`（287）、`notification_webhook.go`（232）、`email_notification.go`（203） |
| `internal/kernel/appcore` | `dns_selftest.go` | `dns_selftest.go`（297）、`dns_selftest_stages.go`（347）、`dns_selftest_checks.go`（292）、`dns_selftest_adapters.go`（403） |
| `internal/kernel/appcore` | `probe_service.go` | `probe_service.go`（275）、`probe_group.go`（349）、`probe_events.go`（331）、`probe_sources.go`（280） |
| `internal/kernel/domain/probe/task` | `download.go` | `download.go`（297）、`download_workers.go`（343）、`download_measurement.go`（330）、`download_attempt.go`（300）、`download_integrity.go`（205）、`download_helpers.go`（173） |
| `internal/kernel/domain/probe/task` | `head.go` | `head.go`（366）、`head_request.go`（395）、`head_colo_filter.go`（243） |
| `internal/kernel/domain/probe/probecore` | `config_snapshot.go` | `config_snapshot.go`（372）、`config_snapshot_compat.go`（413）、`config_snapshot_values.go`（219）、`config_snapshot_mapping.go`（175） |
| `internal/kernel/domain/probe/colodict` | `dictionary.go` | `dictionary.go`（323）、`filter.go`（324）、`geofeed_map.go`（308）、`fetch.go`（240）、`parser.go`（223）、`cidr.go`（126） |
| `internal/adapters/host` | `update.go` | `update.go`（295）、`update_archive.go`（256）、`update_install.go`（190）、`update_download.go`（152） |
| `internal/kernel/domain/dns/dnshecore` | `dnshe.go` | `dnshe.go`（366）、`resources.go`（269）、`push.go`（241） |
| `internal/kernel/domain/github/githubcore` | `github.go` | `github.go`（375）、`export_format.go`（310） |

## 4. `appcore` 重导出瘦身与文档同步

- 审计 `internal/kernel/appcore/` 外部所有 Go 调用方后，将 [`internal/kernel/appcore/reexports.go`](../../../internal/kernel/appcore/reexports.go) 瘦身至 51 行，仅保留外部适配层、Android bridge 与契约测试实际引用的 25 个兼容重导出符号。
- `internal/kernel/appcore` 内部 53 个文件的 302 处内部别名引用已直接迁移到对应内部子包（`contracts`、`diagnostics`、`dns`、`export`、`history`、`notify`、`probe`、`scheduler`、`sources`、`taskstate`、`fileio`、`jsoncompat`）。
- 同步更新受拆解与路径调整影响的活跃文档：[`agent-log-observation.md`](../../dev/agent-log-observation.md)、[`bridge-trace-ddd-design.md`](../../dev/bridge-trace-ddd-design.md)、[`upload-design.md`](../../integration/upload-design.md)、[`external-interfaces.md`](../../reference/external-interfaces.md)、[`limits-and-code-location.md`](../../reference/limits-and-code-location.md) 与 [维护记录索引](../README.md)。
