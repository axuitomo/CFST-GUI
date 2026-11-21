# 重构执行记录（2026-11-21）

对应计划：[`docs/dev/refactor-plan.md`](../../dev/refactor-plan.md) Phase 2 与 Phase 3 Wave A。本文件只记录**已落地**的提交与实测数据，作为下一次继续执行的起点。

## 一、本轮完成的 11 个提交

| 提交 | 内容 | 结果 |
| --- | --- | --- |
| `cb21ea8` | `.editorconfig` 移除从未生效的 Kotlin `#` 注释行 | 死配置清理，非 P1 回归 |
| `7e8e957` | `.gitignore` 归整 build/ 与 internal 产物路径 | 无新增忽略盲区 |
| `9ee0453` | `docs/plan` 按年月分桶 + 索引 | 88 个 md 通过 docs-check |
| `ac53b1c` | WebView 性能文档迁入 `docs/mobile/performance/` | 链接深度修正 |
| `ff89b2f` | 说明 Android Studio `.run/APP.run.xml` 未入库 | 文档一致性 |
| `673c1a1` | README / commands / deployment / index 同步 `scripts/checks/` 路径 | 文档一致性 |
| `ec10cd4` | `lib/probeEventApply.ts` 555 → 110 行 + `probeEventApply/` 5 文件 | 17 条用例通过 |
| `3a9ad6b` | `composables/useResultExport.ts` 482 → 110 行 + `lib/resultExport/` 6 文件 | 8 条用例通过 |
| `165e1b8` | `composables/useStartupSync.ts` 471 → 241 行 + `lib/startup/` 3 文件 + deps 契约 | 4 条用例通过 |
| `a4a722f` | `composables/useProbeTask.ts` 698 → 263 行 + launch/control/deps 三文件 | 8 条用例通过 |
| `57fbf9a` | `lib/bridge/types.ts` 712 → 9 行门面 + `types/` 8 文件 | 导出集合逐一比对一致 |
| `7308a82` | `lib/bridge/config.ts` 755 → 5 行门面 + `config/` 5 文件 | `config.test.ts` 12 条通过 |

拆分均保持对外导出/返回键不变，逐项跑 `pnpm check:fast` 后提交；最后一次 `scripts/checks/check.ps1` 全量门禁退出码 0（含 WebUI 构建）。

## 二、拆分后的实测行数

- `probeEventApply.ts` 110 ｜ `useResultExport.ts` 110 ｜ `useStartupSync.ts` 241 ｜ `useProbeTask.ts` 263
- `bridge/types.ts` 9（最大子模块 `types/config.ts` 242）
- `bridge/config.ts` 5（最大子模块 `config/snapshot.ts` 302）

## 三、仍未过 400 行门禁的文件（精确口径 `(Get-Content).Count`）

前端 6 个：

- `views/SettingsView.vue` 3158（script 825 / template 1921 / style 406）
- `lib/bridge.ts` 1545
- `App.vue` 907（script 493 + template 411）
- `views/SourcesView.vue` 844、`views/ResultsView.vue` 798、`views/DashboardView.vue` 617

Go 生产代码 28 个 >400，前 7（Phase 3 Wave A）：`task/download.go` 1589、`colodict/dictionary.go` 1500、`appcore/dns_selftest.go` 1367、`appcore/upload_notification.go` 1312、`probecore/config_snapshot.go` 1155、`appcore/probe_service.go` 1081、`task/head.go` 983。Kotlin 3 个：`CfstPlugin.kt` 584、`AndroidUpdateDownloads.kt` 521、`ProbeForegroundService.kt` 451。

## 四、下一步的切入点（按计划顺序）

1. `SettingsView.vue`：20 个分区边界已勘明（顶层 `<details>` 起始行 842/882/931/973/1047/1076/1157/1206/1227/1251/1746/1965/2057/2169/2231/2261/2340/2442/2548/2644，最大的 probe 区 494 行）。script 里 99 个声明先分到 `lib/settings/{sectionState,formatters,probe,dnsProviders,summary}.ts`，再按 §4.3 建 `components/settings/*Section.vue`；DNS 平台卡片逻辑（现 131-351）单独进 `DnsProviderCard.vue` + `lib/dnsProviderForm.ts`。注意 `expandedSections`/`mountedSections`/`syncSectionOpen` 必须留在编排层。
2. `lib/bridge.ts` 1545 → 门面 ≤200 + `bridge/{mode,webuiTransport,files,events,normalize,api/*}.ts`（§4.4 已给出分段区间）；types/config 已拆完，barrel 模式可直接复用。
3. `App.vue` 907 → 接线壳 ≤450（§4.2 的声明簇）。
4. Phase 3 Wave A 七个 Go 文件，逐个「搬移 → 测试 → `go test -count=1 ./...`」推进。

## 五、验证基线（本轮实测）

- `pnpm check:fast`：oxlint + vitest（49 文件 / 301 用例）+ vue-tsc 增量，约 4.5s
- `scripts/checks/check.ps1` 全量：前端构建 853ms，Go/Kotlin/文档/边界/字符串守卫全绿
- 精确行数必须用 `(Get-Content -LiteralPath $f).Count`；`Measure-Object -Line` 会偏小约 4%
