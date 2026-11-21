# 重构计划进度与执行审计（2026-11-21）

> 对象：[`docs/dev/refactor-plan.md`](../../dev/refactor-plan.md)
> 基线：HEAD `bf6d5f4`（refactor(frontend): 提取 useTaskSync），工作树含 17 项未提交文档改动
> 方法：全仓精确行数统计（`(Get-Content).Count`）+ 本轮实测门禁 + 提交历史逐条核对 + 守卫负向注入
> 说明：本报告只审计，不改动产品代码。

## 一、总体结论

| Phase | 完成度 | 判定 |
| --- | --- | --- |
| Phase 0 秒级闭环 | 100%（4/4 子项） | 已交付且实测达标 |
| Phase 1 字符串代码歼灭 | 100%（S1–S5 + 守卫） | 已交付，守卫负向测试有效 |
| Phase 2 前端拆解 | 约 45% | 类型单点 + 模块提取完成；目标尺寸、SettingsView、bridge 门面未完成 |
| Phase 3 Go/Kotlin 拆解 | 0% | 未启动，大文件行数与计划快照逐字一致 |
| Phase 4 收尾固化 | 约 15% | 协作规则已写；架构文档、终验、三端冒烟未做 |

加权总进度约 **40%**。质量门禁全绿，无 P0 风险；主要欠账是「拆出的新模块自己越线」和「三个目标文件未动」。

## 二、进度审计（逐项对照）

### Phase 0：秒级报错闭环

| 计划项 | 状态 | 证据 |
| --- | --- | --- |
| §2.1 Go `-short` 守卫 | ✅ | 全仓 `testing.Short` 8 处（5 处 `probe_strategy_test.go` + 三处点名测试）；实测 `go test -short ./internal/appcore/ ./internal/task/` = 2.51s（appcore 1.78s / task 1.39s），达 ≤3s |
| §2.2 vue-tsc 增量 | ✅ | `frontend/tsconfig.json:19-20` 已开 incremental + tsBuildInfo；本轮 warm 实测 **3.39s**（优于计划登记的 5.6–6.0s） |
| §2.3 fast 脚本 | ✅ | `scripts/checks/fast.ps1`（复用 `scripts/lib/common.ps1`）+ `fast.sh` 183 行；直调 node、前端三件套并行、>400 行警告 |
| §2.4 入口与文档 | ✅ | `package.json` `check:fast`、`Taskfile.yml:37`、`AGENTS.MD:30`、`docs/dev/agent-workflow-validation.md:38` |

fast 实测（本轮，同机 warm）：

| 场景 | 耗时 | 结果 |
| --- | --- | --- |
| 文档变更（17 文件） | 0.3s | 只跑 no-string-code，符合设计 |
| 前端变更（-Base HEAD~1，20 文件） | **4.3s** | oxlint/vitest/vue-tsc 并行 3.7s + vapor/boundary 0.1s + stringcode 0.2s |

> 注：前端路径 4.3s 已落在 ≤5s 档，不再需要按 §2.2 的 ≤8s 回退口径声明，但计划文档仍写着回退口径，属文档陈旧。

### Phase 1：5 处字符串代码 + 防回流守卫

| 站点 | 状态 | 证据 |
| --- | --- | --- |
| S1 animation-preview | ✅ | `main.ts` 6620→**39 行**；`frontend/animation-preview.html` 487 行真实 markup；`preview.css` 仅 16 行 `@import`，16 个 `styles/*.css` |
| S2 bridge WebUI 目录选择 | ✅ | 新增 `lib/bridge/directoryPicker.ts` + `.css`；全仓 `innerHTML/outerHTML/insertAdjacentHTML/document.write/cssText/insertRule/new Function/eval(` 扫描 **0 命中** |
| S3 安装器三件套 | ✅ | `ui/index.html` 727→**99 行** + `installer.css` + `installer.js`；`cmd/installer/embed.go:14` 已改 `//go:embed all:ui` |
| S4 sqliteprobe SQL 外置 | ✅ | `internal/sqliteprobe/queries/{schema,upsert_fixed,upsert_param,select}.sql` 四文件，逐条 `go:embed`，两条 INSERT 分文件 |
| S5 build-release 模板外置 | ✅ | `scripts/build/templates/webui-bundle/` 六文件齐全；`build-release.sh` 内 `<<` 仅剩 `<<<` 字符串比较，无 heredoc 写配置 |
| §3.6 守卫 | ✅ | `no-string-code.mjs` 扫描 123 前端 / 178 Go / 5 脚本；**负向注入实测**：临时新增 `el.innerHTML = "<div>x</div>"` → exit=1 并定位 `__audit_probe.ts:1`，移除后恢复 exit=0 |
| 接线 | ✅ | `.githooks/pre-commit:65`、`check.ps1:35`、`check.sh:27`、CI `quality.yml:118`、`package.json` `check:stringcode`、fast.ps1 |

### Phase 2：前端拆解

| 子项 | 状态 | 现状 |
| --- | --- | --- |
| §4.1 共享类型单点 | ✅ | `lib/settingsForm.ts`（359 行）、`lib/appTypes.ts` 已建；`DNSProviderKind` 只从 `./bridge` re-export（`settingsForm.ts:18`），无第三份定义；`App.vue` 与 `SettingsView.vue` 均 import |
| §4.2 模块提取 | ✅ 结构 / ❌ 目标 | 计划表 20 个模块全部落位（28 个 composable 均带 `.test.ts`）；但 `App.vue` 仍有 **907 行**（目标 ≤450），script 段 **493 行**（目标约 40），template 411 行未动 |
| §4.3 SettingsView | ❌ 0% | **3158 行**（script 825 / template 1921 / scoped style 406）；`components/settings/` 目录不存在；只做了类型外移（script 1098→825） |
| §4.4 bridge 层 | ⏳ 约 12% | `bridge.ts` 1639→**1545**（仅 directoryPicker 外移），目标门面 ≤200；`bridge/config.ts` **755**、`bridge/types.ts` **712** 完全未拆 |
| §4.5 视图与 CSS | ❌ 0% | `SourcesView.vue` 844、`ResultsView.vue` 798、`DashboardView.vue` 617、`styles.css` 1351，与计划快照一致或几乎一致 |

本轮前端生产文件 >400 行仍有 **13 个**：SettingsView 3158 / bridge.ts 1545 / styles.css 1351 / App.vue 907 / SourcesView 844 / ResultsView 798 / config.ts 755 / types.ts 712 / useProbeTask 698 / DashboardView 617 / probeEventApply 555 / useResultExport 482 / useStartupSync 471。

### Phase 3：Go / Kotlin 拆解

**0%**。计划 §0.3 的七个千行级行号与本轮实测逐字一致：`download.go` 1589、`dictionary.go` 1500、`dns_selftest.go` 1367、`upload_notification.go` 1312、`config_snapshot.go` 1155、`probe_service.go` 1081、`head.go` 983。当前 >400 行：Go 生产 **25 个**、Go 测试 **9 个**（`probe_strategy_test.go` 2430 居首）、Kotlin **3 个**（584/521/451，与计划一致）。

### Phase 4：收尾固化

约 15%：`AGENTS.MD` 与 `docs/dev/agent-workflow-validation.md` 已写 fast 闭环；`docs/dev/agent-code-architecture.md` 未补 bridge 门面与模块归位；`check.ps1` 全量轮次本轮未执行（只跑了分项门禁）；三端人工冒烟、行数分布报告未产出（本报告补位）。

## 三、执行审计（质量与合规）

### 3.1 门禁实测（本轮）

| 检查 | 耗时 | 结果 |
| --- | ---: | --- |
| oxlint | 0.08s | pass |
| vitest | 1.89s（内部 1.40s） | 49 文件 / **301 用例** 全过 |
| vue-tsc --noEmit | 3.39s | pass |
| `go test -count=1 ./...` | 7.71s | 全过（task 4.36s 为最大单包） |
| `go test -tags webui ./internal/app/` | 0.85s | pass（S2 专项验证） |
| no-string-code | 0.2s | pass（123/178/5） |
| `pnpm check:fast`（前端路径） | 4.3s | pass |

测试规模较计划基线：**18 文件 / 87 用例 → 49 文件 / 301 用例**（+31 文件 / +214 用例），特征测试先行要求基本兑现。

### 3.2 提交粒度与红线

- 每个 refactor 提交结构一致：`App.vue` 减行 + 新 composable + 配套 test（如 `bf6d5f4` 150 删/154 测 + `52dcbfb` 418 删/471 模块）。符合「禁止一次性大爆炸」。
- `f1b7a13..HEAD` 对 `tests/` 零改动：重构链未触碰 aria 与像素基线；`tests/__screenshots__` 最近三次改动均为 2.0.6 发布/CI 语境，非本次重构。
- `f1b7a13` 的 aria 变更发生在重构链之前，提交说明明确归因于 DNS 多平台读取特性（非重构），不违反红线。
- `bridge.ts` 对外 re-export 契约未破（`bridgeAudit.test.ts` 在位）；`settingsForm` 类型逐字迁移。

### 3.3 执行的偏差（重要）

1. **新模块自己越线**：计划要求「每个新模块 ≤300 行」，实际 `useProbeTask` 698、`probeEventApply` 555、`useResultExport` 482、`useStartupSync` 471 均 >400，另有 `useConfigPersistence` 395、`useFileExport` 362、`useSources` 361、`settingsForm` 359、`configSnapshotBuild` 309 超 300。等于把巨石切成中石，二次拆解成为新债。
2. **App.vue 只走了 78%**：6620→907（-86%），但剩余 493 行 script 几乎全是接线，模板 411 行原样；要走完 ≤450 还需把接线按域收进 composable/上下文，当前未做。
3. **Phase 2 后三块停摆**：4.3/4.4/4.5 零进展，而 4.4 是 bridge 门面契约的核心工程。
4. **计划允许的并行未用**：Phase 3 可与 Phase 2 交替，实际一行未动。

### 3.4 工作树卫生（P1）

当前未提交 17 项，直接影响「每处独立提交、失败即 revert」的回滚前提：

- ~~`.editorconfig` 未提交 diff 删除整段 Kotlin ktlint 配置~~ **（2026-11-21 复核修正：该行以 `#` 开头，是整行注释，`[*.{kt,kts}]` 段头与 17 条规则全部被注释掉，从未生效；真正的 ktlint 规则在 `mobile/android/app/.editorconfig`（`root = true`，含同样 17 条 + 3 条依赖规则如 `string-template-indent`）。删除属清理死配置，无功能损失，按普通 chore 提交即可。）**
- 8 个 `docs/` 重命名已 staged、10 个文档改路径（README/docs 索引/commands 里的 `scripts\check.ps1` → `scripts\checks\check.ps1`），属仓库重组工作，与重构计划混在同一工作树。
- 仓库根新增未跟踪 `refactor-plan.md`，与 `docs/dev/refactor-plan.md` **哈希不同**（陈旧副本），易误导读者。
- 未跟踪 `docs/plan/README.md`（新索引）。

### 3.5 计划文档自身陈旧点（P3）

- §0.1 基线表仍写「vitest 18 文件 87 用例」，现为 49/301；lint/vitest/vue-tsc 实测均优于表中值。
- §0.1 称 `changed-check.sh` 只有 bash 版且 Go 跑全量——仍成立（`changed-check.sh:120` 为全量 `go test`），但 fast.ps1 已覆盖同一职责，两套变更集算法并存，存在长期漂移风险。
- §2.2 回退口径（前端 ≤8s）已被 4.3s 实测超越。
- 行数口径：`Measure-Object -Line` 比真实行数少约 4%，后续统计请用 `(Get-Content).Count`（本报告已按精确口径）。

## 四、风险分级

| 级别 | 事项 | 影响 |
| --- | --- | --- |
| P0 | 无 | 基线全绿 |
| P1 | ~~`.editorconfig` Kotlin 配置被删~~ 复核为死注释清理，无影响 | 已降级，无需恢复 |
| P1 | 工作树 17 项未提交 + 根目录陈旧计划副本 | 回滚粒度失效，审计/交接误读 |
| P2 | 4 个新模块 >400 行 | 计划「≤300/≤400」目标反被稀释 |
| P2 | App.vue 907 行、SettingsView 3158 行、bridge.ts 1545 行 | Phase 2 验收不达 |
| P2 | Phase 3 零进度，50 个 >400 文件仍在 | 总目标未过半 |
| P3 | 计划文档基线过时、双变更集脚本 | 后续审计口径不一致 |

## 五、建议的下一步（按 ROI 排序）

1. **先清工作树（半天内）**：确认 `.editorconfig` 是否为误删并恢复；把 docs 重组按「路径修正 / 索引新增 / 布局说明」拆成独立提交；删除根目录 `refactor-plan.md` 副本（保留 `docs/dev/` 唯一事实源）。
2. **补 4 个越线模块的二次拆解**：`useProbeTask` 698 → 任务动作/快照/结果三块；`probeEventApply` 555 → 按 stage 再分文件；`useResultExport` 482、`useStartupSync` 471 同法。这是唯一「重构过程新造债务」。
3. **Phase 2 收口**：SettingsView 3158 → 20 个 `*Section.vue`（`components/settings/` 从零建，工程量最大、单文件收益最大）→ bridge 三拆（1545/755/712，门面 ≤200）→ App.vue 接线压缩（907 → ≤450）→ 视图与 styles.css。
4. **Phase 3 Wave A 与 Phase 2 交替**：7 个千行级同包拆文件，`grep '^func'` 定缝后逐文件提交，先做 `task/download.go` 与 `appcore/dns_selftest.go`。
5. **Phase 4 收口**：`check.ps1` 全量 + CI Quality 跑一轮；`agent-code-architecture.md` 补 bridge 门面；同步计划 §0.1 基线表；三端冒烟。

## 六、复测命令

```powershell
pnpm check:stringcode                       # 0.2s
pnpm check:fast                             # 变更区域最小集
pwsh -NoProfile -File scripts/checks/fast.ps1 -Base HEAD~1   # 前端路径实测
go test -short -count=1 ./internal/appcore/ ./internal/task/
go test -count=1 ./...
go test -tags webui ./internal/app/
```
