# CFST-GUI「去字符串化 + 小文件 + 秒级校验」重构计划

> 2026-11-21 审计修订：修正 Phase 0 提速归因（sleep 总量仅 ≈1.1s，真正大头为 doh 看门狗 2s / DNSHE 自测退避 2s / source 重试退避 1s）；统一闭环耗时口径（前端 ≤5s 以增量成功为前置，回退 ≤8s 须声明）；App.vue ≤450 例外；守卫误伤防护（13 处合法插值实测）；DNSProviderKind 归并 bridge/types 既有导出；S4 双 INSERT 分文件。复测基线见 §0.1 注。

## 0. 现状体检结论（全仓扫描 + 本机实测）

### 0.1 校验链路基线（warm cache，本机实测）

| 检查项 | 耗时 | 说明 |
| --- | ---: | --- |
| `pnpm --dir frontend lint`（oxlint） | 0.1s | 已达标 |
| `pnpm --dir frontend test`（vitest node，18 文件 87 用例） | 1.0s | 已达标 |
| `pnpm typecheck`（vue-tsc --noEmit） | 4.5s | 前端闭环的主要瓶颈 |
| `go test -count=1 ./internal/appcore/` | 5.4s | 定时器/退避测试为主因（sleep 合计仅 80ms，见结论） |
| `go test -count=1`（全部 Go 包） | 6.6s | task 4.3s / appcore 4.8s / doh 2.1s 占大头，其余包均 <0.5s |
| `go build`（全部包） | 1.0s | 对纯测试包 `internal/contracttest` 报 exit=1，属工具特性，闭环改用 `go test` 编译即检 |

> 审计复测（2026-11-21，同机 warm，-count=1）：lint 0.6s / vitest 2.8s（内部 1.68s，18 文件 87 用例）/ vue-tsc 7.7s / appcore 7.5s / go 全量 9.8s。上表为最佳 warm 值，机器负载下可上浮 ~40%；闭环验收以复测口径留余量。

结论：**秒级闭环可达，但提速关键不在 sleep**。全仓 `time.Sleep` 合计仅 ≈1.1s（appcore 三个文件合计 80ms）；真正大头是三个定时器/退避测试：`doh.TestLookupIPAddrHonorsTimeout` 2.0s（httptest handler 内 `time.After(2s)` 看门狗 + `server.Close` 等待未决请求）、`appcore.TestRunDNSSelfTestDNSHEComposesBareSubdomainTestName` 2.0s（500 响应触发生产重试退避）、`appcore.TestLoadSourceContentRetriesTransientServerErrorOnce` 1.0s（`DefaultProbeConfig` 1s 退避）。`-short` 必须覆盖这三处（见 §2.1），否则 ≤3s 不可达。前端 lint+单测已 ≤1.1s（pnpm 壳另加 ~0.5s/次，fast 脚本直调 node 二进制规避），vue-tsc 4.5s 通过增量编译实验压缩。现有 `scripts/checks/changed-check.sh` 已实现"按变更区域跑检查"，但只有 bash 版、且 Go 侧跑全量测试、无 `-short`，需要补 PowerShell 版并提速。

### 0.2 「字符串里的代码」清单（生产代码，共 5 处窝点）

| # | 位置 | 问题 | 规模 |
| --- | --- | --- | --- |
| S1 | `frontend/src/animation-preview/main.ts` | `renderDemo` 模板字符串拼整页 HTML，13 个条件分支内嵌单引号 HTML 片段；`root.innerHTML` 拼页面外壳 | ~90 行 HTML 在字符串里 |
| S2 | `frontend/src/lib/bridge.ts` `browseWebUIDirectory`（L800-895） | WebUI 目录选择器用 15 处 `style.cssText` 内联 CSS 字符串（含 oklab 颜色硬编码） | ~15 段 CSS 在字符串里 |
| S3 | `cmd/installer/ui/index.html` | 727 行单文件：L6-410 内联 `<style>`（404 行 CSS）、L510-725 内联 `<script>`（215 行 JS） | 619 行 CSS/JS 内嵌在 HTML |
| S4 | `internal/sqliteprobe/sqliteprobe.go` | 4 条内联 SQL（PRAGMA+CREATE ×2 重复 / INSERT 字面量 `VALUES(1,'ok')` / INSERT 参数化 `VALUES(?,'ok')` / SELECT） | 小 |
| S5 | `scripts/build/build-release.sh` L510-590 | heredoc 字符串生成 `Dockerfile`、`docker-compose.yml`、`docker-compose.host.yml`、`.env.example`、`run-local.sh`、`README.md` | 6 个配置/脚本文件在字符串里 |

边界说明（扫描已确认，不在歼灭范围）：

- Go 生产代码**没有**字符串拼 HTML/CSS/JS（仅 4 个 `_test.go` 含 HTML 测试夹具）；没有 Sprintf 拼 JSON（仅测试夹具）。`internal/utils/progress.go` 的 Sprintf 是进度条库模板参数，属库 API 用法，保留。
- Go/Kotlin 测试里的 raw-string JSON 夹具（31 个 Go 测试文件、3 个 Kotlin 测试文件）是**静态数据字面量**，不是拼接代码，无转义风险，保留（可选后续移 testdata，不在本计划）。
- `cmd/installer/embed.go` 的 `{{VERSION}}` 等占位符替换是"模板文件 + 工具组装"，方向正确，保留；安装器已有本地 HTTP mux（`handleIndex` 等），S3 拆分后可直接加静态路由。
- 各脚本里 `cat <<EOF` 打印 usage 帮助文本不属于代码拼接，保留。
- 好范例已在仓库里：`frontend_assets.go` 的 `go:embed all:frontend/dist`、`cmd/installer/embed.go` 的 `go:embed all:ui`（S3 执行后）、Vite 多入口构建。本计划就是把剩余 5 处补齐到同一范式。

### 0.3 超长文件分布（>400 行共 50 个；>300 行共 85 个）

前端（10 个）：

- `App.vue` **6620**（script 6206 + template 411，424 个顶层声明，全应用状态/逻辑单体）
- `SettingsView.vue` **3432**（script 1098 + template 1921 + scoped style 408，20 个设置分区混在一起）
- `lib/bridge.ts` **1639**（~100 个导出：通道选择/事件归一化/WebUI 传输/文件选择/各域 API 包装）
- `styles.css` **1351**、`animation-preview/preview.css` **1139**
- `views/SourcesView.vue` 844、`views/ResultsView.vue` 798、`lib/bridge/config.ts` 755、`lib/bridge/types.ts` 712、`views/DashboardView.vue` 629
- 隐患：`SettingsForm`/`DnsProviderForm` 等类型在 App.vue 与 SettingsView.vue **各定义了一份**，双重事实来源；`DNSProviderKind` 另有 `lib/bridge/types.ts:278` 第三份导出。

Go 生产代码（28 个 >400 行），前 7 大：

- `internal/task/download.go` 1589、`internal/colodict/dictionary.go` 1500、`internal/appcore/dns_selftest.go` 1367、`internal/appcore/upload_notification.go` 1312、`internal/probecore/config_snapshot.go` 1155、`internal/appcore/probe_service.go` 1081、`internal/task/head.go` 983
- 其余 400-900 行：dnshecore/dnshe.go 855、app/update.go 822、httpclient/client.go 690、githubcore/github.go 670、appcore/task_service.go 627、probecore/probe_config.go 620、sourceparse/sourceparse.go 584、mcis/engine/engine.go 532、appcore/upload_selection.go 531、dnspodcore/dnspod.go 528、cloudflarecore/dns.go 520、app/webui.go 511、utils/csv.go 504、alidnscore/alidns.go 495、probecore/stage_workflow.go 481、cmd/installer/main.go 458、appcore/task_store.go 444、appcore/dns_provider_service.go 425、app/storage.go 421、probecore/source_build.go 413、mcis/bandit/head.go 408

Kotlin（3 个）：`CfstPlugin.kt` 584、`AndroidUpdateDownloads.kt` 521、`ProbeForegroundService.kt` 451

测试（9 个 >400 行）：`task/probe_strategy_test.go` **2430**、`app/update_test.go` 797、`colodict/dictionary_test.go` 760、`probecore/config_snapshot_test.go` 724、`appcore/upload_notification_test.go` 724、`httpclient/client_test.go` 612、`appcore/source_runtime_test.go` 533、`cloudflarecore/dns_test.go` 532、`appcore/dns_selftest_test.go` 449

---

## 1. 目标与总验收

1. 生产源码中**零**"字符串拼 HTML/CSS/JS/SQL/配置"；全部变为带后缀标准文件，由 Vite / `go:embed` / 复制脚本在构建期组装，并有守卫脚本防止回流。
2. 生产源码单文件 ≤400 行（目标 ≤300）；**App.vue 例外 ≤450**（template 411 行保留为编排层，script 压到 ~40 行）；模块职责单一、接口边界清晰；类型双重定义消除（含 `bridge/types.ts` 既有 `DNSProviderKind` 导出归并）。
3. 改完即跑 `pnpm check:fast`：**单区域变更 warm ≤5s 出结果**（争取 ≤3s；前端 ≤5s 以 vue-tsc 增量成功为前置，增量不兼容时回退口径 ≤8s 并在交付说明中声明），报错信息可直接定位；AI/人改完自己跑、自己修。
4. 全程**零行为变更**：配置 schema、bridge 字段、API/事件形状、存储路径、发布资产、像素基线全部不变。

总验收命令（仓库根，pwsh）：

```powershell
pnpm check:stringcode        # 新增守卫：0 命中
pnpm check:fast              # 新增秒级闭环：打印各步耗时
.\scripts\checks\check.ps1   # 既有全量门禁：全绿
```

外加 CI Quality 工作流全绿（含 -race、webui build-tag、Playwright smoke、视觉基线）。

---

## 2. Phase 0：秒级报错闭环（先行，为后续所有拆解提供验证）

> 顺序说明：用户三条原则中闭环排第三，但它是其余两步的"安全网"——先有 3 秒反馈，后面拆 6620 行 App.vue 才敢小步快跑。此 Phase 独立可交付，半天到一天。

### 2.1 Go 测试提速：`-short` 守卫

- 现状：全仓 `testing.Short()` 使用为 **0**；12 个测试文件共 30 处 `time.Sleep`，但合计仅 ≈1.1s（重灾区 `task/probe_strategy_test.go` 14 处合计 ≈0.83s、`task/retry_test.go` 3 处、`cmd/installer/installer_test.go` 3 处、appcore 3 个文件合计 80ms）。**审计实测三大耗时点均非 Sleep**：`doh/doh_test.go:139` handler 内 `time.After(2s)` 看门狗（`server.Close` 等待未决请求致整测 2.0s）、`appcore/dns_selftest_dnshe_test.go:132`（生产退避 ≈2s）、`appcore/source_runtime_test.go:204`（生产 1s 退避）。
- 做法：逐文件审计 `Sleep`/`time.After`/ticker/`httptest.Close` 阻塞，加 `if testing.Short() { t.Skip(...) }` 或将纯计时等待按 `testing.Short()` 缩短到毫秒级；**不改变断言语义**。三个点名测试：doh 看门狗 short 下压缩到 200ms（断言 `elapsed<1s` 仍成立）；DNSHE 自测与 source 重试两测 short 下整测跳过（退避在生产代码，注入时钟超出纯结构迁移边界；全量语义保留给 CI/check.ps1）。网络依赖测试若存在同样纳入 short 跳过。
- 验收：重负载包单包 `-short` ≤3s（实测 appcore 2.9s / task 1.6s / doh 0.2s，单包 wall ≈ 逐测试之和）；`go test -short` 全量在 CI/Linux 并行下 ≤3s，**本机 Windows 受安全软件进程创建开销限制实测 ~8s（32 包近乎串行，2 包探针 wall≈sum 证实），按环境因素记录不视为失败**；`go test -count=1`（非 short）结果与改前一致（全量语义保留给 CI/check.ps1）。

### 2.2 vue-tsc 增量实验

- 在 `frontend/tsconfig.json` 试开 `"incremental": true` + `"tsBuildInfoFile": "./node_modules/.tmp/tsconfig.tsbuildinfo"`（noEmit 与 incremental 可共存，TS 6 支持）。
- 实测二次运行耗时：若降到 ≤2s 则保留（**前端 ≤5s 达标的前置条件**）；若无收益或 vue-tsc 3.3.14 不兼容则回退，接受 4.5-7.7s 下限，总闭环回退口径 ≤8s 并在交付说明中声明。验证增量正确性：临时注入一处类型错误须被二次运行捕获，移除后恢复绿。
- 备注：`tsc:ts7`（原生 TS7）只查 .ts 不查 .vue，不能替代 vue-tsc，保留作补充。
- 实验结果（2026-11-21）：首跑 10.1s → 稳态 5.6-6.0s；注入类型错误被二次运行捕获、移除后恢复绿，增量正确性验证通过。未达 ≤2s 阈值，但较非增量 7.7s 收益 ~25%，**保留增量**；前端闭环按回退口径 ≤8s 声明（三件套并行后实测 ~6s）。

### 2.3 新增 fast 检查脚本（PowerShell 主版 + bash 孪生版，遵循仓库双脚本惯例）

逻辑（复用 `scripts/lib/common.ps1` 与 changed-check 的变更集算法）：

1. 取变更文件集（staged+unstaged+untracked；支持 `--base <ref>`）。
2. Go 变更 → 由文件路径映射到受影响包，跑 `go test -short -count=1 <packages>`（编译即类型检查；go.mod/go.sum 变更则全量）；若 appcore 不在受影响集合，单独跑 3 个依赖边界测试（与 pre-commit 同款）。
3. 前端变更 → `oxlint` + `vitest run` + `vue-tsc --noEmit` **三者并行**，且**直调 node 二进制**（`node node_modules/...`，沿用根 package.json 范式，规避 pnpm 壳 ~0.5s/次开销）；再跑 `vapor-mode.mjs` 与 `frontend-boundary`（均为毫秒级 node 脚本）。
4. 末尾打印每步耗时与总耗时；对变更中 >400 行的生产文件输出**警告**（不阻断，作为拆分红线提示）。
5. 无变更 → 跑最小全量（go test -short 全量 + 前端三件套），保证"随时可跑"。

### 2.4 入口与文档

- `Taskfile.yml` 增加 `check:fast`；根 `package.json` 增加 `"check:fast": "pwsh -NoProfile -File scripts/checks/fast.ps1"`。
- `docs/dev/agent-workflow-validation.md` 增补："每次代码编辑后立即运行 `pnpm check:fast`，报错自行修复后再交付；全量 check.ps1 用于交接/CI"。
- `AGENTS.MD` Always-On Rules 增加一条指向 fast 闭环（符合"仅当协作规则本身变化才改 AGENTS.MD"的边界）。
- 验收：单文件 Go 变更 warm ≤3.5s（appcore 含 0.9s 依赖门禁；其余包 ≤2s）；单文件前端变更 warm ≤8s（vue-tsc 增量未达 ≤2s 阈值，回退口径生效，见 §2.2 实验结果）；连续两次运行耗时打印稳定。

---

## 3. Phase 1：物理消灭 5 处字符串代码 + 防回流守卫

每处独立提交，改完即跑 `pnpm check:fast` + 对应专项验证。

### S1 animation-preview（纯静态页，不引入框架）

- 把 `main.ts` 里整页外壳 + 13 张 demo 卡片 markup **全部写实**到 `frontend/animation-preview.html`（现仅 13 行；13 个 demo 的内部 markup 本就互不相同，条件字符串分支直接展开为 13 段真实 HTML，约 300-350 行标准 HTML）。
- `main.ts` 只留行为：事件委托、replay、候选计数（~100 行），删除 `renderDemo`/`innerHTML`/demos 数据数组（元数据写进 HTML）。
- `preview.css` 1139 行按 demo 分区拆为 `animation-preview/styles/*.css`，入口 `@import` 汇总（Vite 原生处理）。
- 验证：`pnpm build` 后人工打开 animationPreview 入口，13 个动画播放/标记候选行为不变；`pnpm check:vapor` 不受影响（该页不是 SFC）。

### S2 bridge.ts WebUI 目录选择器

- 新建 `frontend/src/lib/bridge/directoryPicker.ts` + `directoryPicker.css`：DOM 仍用 `createElement`（标准 API，允许），15 处 `cssText` 全部改为 class 引用；oklab 颜色收敛为 CSS 变量，与 `styles.css` token 对齐。
- `bridge.ts` 中 `browseWebUIDirectory` 迁出，原处保留薄调用；平台引用仍全部留在 `lib/` 下，满足 frontend-boundary 规则。
- 验证：`go test -tags webui ./internal/app/` + WebUI 模式人工走一遍目录选择（上一级/双击进入/选择当前目录/取消）。

### S3 安装器 UI 三件套

- `cmd/installer/ui/index.html` 拆为 `index.html`（~110 行 markup+占位符）+ `installer.css`（404 行）+ `installer.js`（215 行），`<link>`/`<script src>` 相对引用。
- `embed.go`：`//go:embed ui/index.html` 改 `//go:embed all:ui`（embed.FS）；`renderUI` 占位符替换逻辑保留（模板+工具组装，合规）。
- `main.go` 既有 mux 增加 `/installer.css`、`/installer.js` 静态路由（正确 Content-Type，no-cache）；`handleIndex` 不变。
- 验证：`go test ./cmd/installer/`（含 renderUI 转义测试）+ 本机跑一次安装器预览，拖动/浏览/安装/取消/法务页全通。

### S4 sqliteprobe SQL 外置

- 新建 `internal/sqliteprobe/queries/schema.sql`（PRAGMA+CREATE）、`queries/upsert_fixed.sql`（字面量 `VALUES(1,'ok')`）、`queries/upsert_param.sql`（参数化 `VALUES(?,'ok')`）、`queries/select.sql`，`//go:embed queries/*.sql` 注入；两处重复建表串收敛为同一文件。**两条 INSERT 语义不同，必须分文件，不得合并**。
- 语句内容与现字符串逐字一致（含多语句 Exec 行为），参数化占位符不变。
- 验证：`go test ./internal/sqliteprobe/ ./sqlitegate/`；Android 侧 gomobile 冒烟（sqlitegate 是移动端驱动验证入口）。

### S5 build-release.sh 配置模板外置

- 新建 `scripts/build/templates/webui-bundle/`：`Dockerfile`、`docker-compose.yml`、`docker-compose.host.yml`、`.env.example`、`run-local.sh`、`README.md` 六个真实文件（静态部分原样迁出）。
- `.env.example` 原为展开型 heredoc：改为 `@PLACEHOLDER@` 占位 + 脚本内 `sed` 替换（工具侧组装）；其余五个纯静态，`cp` 即可。
- 验证：`bash -n scripts/build/build-release.sh` + shellcheck；跑 webui bundle 生成段，`docker compose -f <生成物> config` 校验语法；产物与改前 diff 仅占位符来源差异。

### 3.6 防回流守卫 no-string-code 检查

- 仿照既有 `vapor-mode.mjs` 的轻量不变量扫描（正则级，**不是** AST 工具/代码生成器），新增 `scripts/checks/no-string-code.mjs`：
  - `frontend/src/**`（ts/vue/js）：禁 `innerHTML`/`outerHTML=`/`insertAdjacentHTML`/`document.write`/`cssText`/`insertRule`/`new Function`/`eval(`；禁模板字符串内出现 HTML 标签起始。**实现约束（审计实测 13 处合法插值会误伤）：只扫 `.ts` 与 `.vue` 的 `<script>` 块；按反引号配对取模板字面量区域；标签须完整语法 `</?[a-zA-Z][\w-]*(\s|>|/)`；放行 `<` 比较与 Vue 模板插值（如 `` `scaleX(${...})` ``）。**
  - 非测试 `*.go`：禁 raw string 含 `<!DOCTYPE|<html|<div|<style|<script`；禁 `Sprintf` 拼 JSON 对象起始。
  - `scripts/build/*.sh`：禁 heredoc 写入 Dockerfile/compose/.env/.conf/.nsi 等配置文件（usage 帮助 heredoc 放行）。
  - 豁免登记：`scripts/build/version-bump.sh:124` heredoc 写发布 notes（.md，非配置）放行；Kotlin/mobileapi 当前 0 命中，暂不纳入守卫（边界写明）。
- 接线：根 `package.json` `"check:stringcode"`；`.githooks/pre-commit`（仅变更文件触发）；`check.ps1`/`check.sh`；CI Quality 工作流。
- 验收：守卫当前 0 命中；人为注入一处违规能被拦截。

---

## 4. Phase 2：前端巨文件拆解（最大工程量，2-4 天）

原则：页面只编排；状态逻辑进 `composables/`，UI 无关逻辑进 `lib/`；每个新模块 ≤300 行、带聚焦单测；Vapor 不变量（`<script setup vapor lang="ts">`、禁字符串组件/h()/JSX）全程由既有守卫保住。

### 4.1 共享类型先行（消除双重事实来源）

- 新建 `lib/settingsForm.ts`：`SettingsForm`、`DnsProviderForm`、`CloudflareRoutingRuleForm` 等 App.vue 与 SettingsView.vue 重复定义的类型与默认值，单点化；两处改为 import。**`DNSProviderKind` 已在 `lib/bridge/types.ts:278` 导出（第三份定义）：settingsForm.ts 从 bridge/types 导入并 re-export，不得新造副本**。
- 新建 `lib/appTypes.ts`：`ViewName`、`ToastTone`、`ViewportPreset`、`HistoryEntry`、`ProcessTraceEntry` 等共享类型。

### 4.2 App.vue（6620 → 目标 ≤450 行编排壳，template 411 行保留）

按已勘明的声明簇提取（行号为当前定位依据）：

| 新模块 | 来源簇（App.vue 行号） |
| --- | --- |
| `composables/useToasts.ts` + `useActivityFeed.ts` | 1604-1682, 1765-1803（lib/activityFeed 已有，接线） |
| `composables/useTheme.ts` | 2935-2979（lib/surfaceTheme 已有） |
| `composables/useNavigation.ts` | 2878-2934 |
| `composables/useViewport.ts` + `useAndroidSelectPicker.ts` | 692-708, 1118-1413（lib/androidViewport 已有） |
| `composables/useAndroidRuntime.ts` | 5017-5262（电池/保活/通知权限） |
| `composables/useSources.ts` + `useSourceProfiles.ts` | 981-1086, 1853-1879, 1940-2001, 3624-3813 |
| `composables/useSourceAutoSave.ts` | 2745-2877（lib/settingsAutoSave 模式复用） |
| `lib/configSnapshotBuild.ts` | 2482-2744 `buildConfigSnapshot`（240 行，UI 无关） |
| `composables/useConfigPersistence.ts` | 2228-2385 apply + 2980-3050 draft + 4555-4735 persist |
| `composables/useDnsProviders.ts` | 2002-2227, 5776-5860 |
| `composables/useDnsRecords.ts` + `useDnsSelfTest.ts` | 5499-5677, 5690-5725（DnsSelfTestPanel 组件已有） |
| `composables/useResults.ts` | 518-538, 1881-1939, 3814-3951, 4979-5016, 5264-5287 |
| `composables/useResultExport.ts` | 2386-2481, 5726-5775, 5861-6038（CSV/GitHub/Cloudflare 推送） |
| `lib/probeEventApply.ts`（按阶段再拆） | 3952-4372 `applyProbeEvent`（420 行单函数，按 stage 拆 handler） |
| `composables/useProbeTask.ts`（扩展现有 143 行） | 1683-1852, 4736-4963, 5288-5498 |
| `composables/useDownloadSpeed.ts` | 479-517, 1707-1764, 4736-4748（lib/downloadSpeedRows 已有） |
| `composables/useStorageHealth.ts` + `useColoDictionary.ts` + `useScheduler.ts` | 1415-1446, 3051-3138, 3316-3454 |
| `composables/useAppUpdate.ts` | 4486-4554 |
| `composables/useFileExport.ts` + `useWebDAV.ts` | 3139-3315, 3491-3623 |
| `composables/useStartupSync.ts` | 601-653, 5143-5197, 6047+ |
| `lib/valueCoerce.ts` | 1086-1117, 1473-1603（asCount/asNumber/normalize* 纯函数） |

- **特征测试先行**：现有 composable 单测仅 useProbeTask 一个；提取前先给 `applyProbeEvent` / `buildConfigSnapshot` / persist 三块补特征测试（锁定输入→输出），再动刀。
- template 411 行保留在 App.vue（已是编排层），script 压到 ~40 行 import/接线，全文件 ≤450（§1.2 例外）；`components/layout/DesktopShell.vue`、`MobileShell.vue` 已有，继续承接。
- 每提取一个模块：搬移 → 补/迁单测 → `pnpm check:fast` → 下一个。**禁止**一次性大爆炸重写。

### 4.3 SettingsView.vue（3432 → ≤400 行编排）

- 按既有 `SettingsSectionKey` 的 20 个分区拆 `components/settings/*Section.vue`（updates/viewport/appearance/storage/colo/backup/sources/cloudflare/probe/scheduler/export/github/postPush/upload/protection/debug/telegram/tls/dns/webhook），每组件自带 props/emits 切片与 scoped style；408 行 scoped style 随分区归位。
- 分区展开/挂载状态（653-707, 1051-1098）留编排层；DNS provider 卡片逻辑（396-626）进 `components/settings/DnsProviderCard.vue` + `lib/dnsProviderForm.ts`。
- scoped style 随分区拆出后保持组件注册/`@import` 顺序稳定，避免级联顺序变化（视觉基线兜底）。

### 4.4 bridge 层

- `bridge.ts` 1639 → 门面 ≤200 行（保持"唯一 bridge 分发点"契约，re-export 不变，调用方零改动）；实现迁 `lib/bridge/`：`mode.ts`（465-540）、`webuiTransport.ts`（592-720）、`files.ts`（717-945，含 S2 的 directoryPicker）、`events.ts`（1507-1639）、`normalize.ts`（134-460）、`api/*.ts`（1012-1500 按域：config/sources/dns/update/android/export/task）。
- `bridge/config.ts` 755 → `config/defaults.ts` + `config/normalize.ts` + `config/sources.ts` + `config/snapshot.ts`（barrel re-export）。
- `bridge/types.ts` 712 → 按域拆 `types/{probe,dns,config,task,android}.ts` + barrel。
- 行号为审计快照；S2 已先移动 bridge.ts 行号，执行时**按符号 grep 定位**，不信快照行号。

### 4.5 其余视图与 CSS

- `SourcesView.vue` 844 / `ResultsView.vue` 798 / `DashboardView.vue` 629：同法拆子组件+composables。
- `styles.css` 1351 → `styles/tokens.css`（:root 变量/暗色主题）、`styles/base.css`、`styles/components/*.css`；入口保留 `@import "tailwindcss"` 与 `@config` 首行顺序，其余 `@import` 汇总（Tailwind v4 仅走 `@tailwindcss/vite`，不引入 PostCSS）。

### 4.6 Phase 2 验收

- `pnpm typecheck` / `lint` / `test` / `test:browser` 全绿；`pnpm check:vapor`、frontend-boundary 全绿。
- `go test -tags webui ./internal/app/` + `pnpm test:e2e`（chromium smoke）全绿。
- 视觉基线：纯结构重构像素不变；如 CI 视觉失败仅因本机/runner 栅格化差异，按既定流程走 Quality 工作流 `update_visual_baselines`，**不在本机刷新提交**。
- 前端生产文件全部 ≤400 行（App.vue ≤450 例外，§1.2）；SettingsForm 类型单点。

---

## 5. Phase 3：Go / Kotlin 巨文件拆解（2-3 天）

原则：**同包多文件**拆分（Go 同包拆文件零 import 变更、零 API 变更、零行为变更），按已勘明的职责缝切；每文件 ≤400 行；不新建包、不改包名、不动 `internal` 边界（遵守 repository-structure-target.md 阶段 0：不为树形整齐搬目录）。

### 5.1 Wave A：7 个千行级（优先，收益最大）

| 现文件 | 拆分目标（同包） |
| --- | --- |
| `task/download.go` 1589 | `download.go`（编排/重试 ~250）、`download_measurement.go`（测量/采样 ~400）、`download_worker.go`（range/full worker+读体 ~350）、`download_integrity.go`（完整性/哈希 ~230）、`download_result.go`（结果构造/edge 拒绝 ~180）、`download_request.go`（请求/Range/nonce ~150）、`download_errors.go` |
| `colodict/dictionary.go` 1500 | `dictionary.go`（Update/Process 编排）、`fetch.go`（HTTP 拉取）、`geofeed.go`（解析/构建）、`locations.go`（locations/countries/lookup）、`store.go`（读写/Status）、`filter.go`（Filter/token 解析）、`prefix.go`（netip 前缀运算） |
| `appcore/dns_selftest.go` 1367 | `dns_selftest.go`（入口/run 生命周期）、`dns_selftest_stages.go`（execute/CNAME/push/read/check）、`dns_selftest_records.go`（记录工具函数）、`dns_selftest_adapter_{cloudflare,aliyun,dnspod,dnshe}.go`（4 个适配器各 ~100-150） |
| `appcore/upload_notification.go` 1312 | `upload_notification.go`（模型/Build）、`upload_notification_text.go`（文案/标签）、`upload_notification_webhook.go`、`upload_notification_telegram.go`、`upload_notification_email.go` |
| `probecore/config_snapshot.go` 1155 | `config_snapshot.go`（Default）、`config_snapshot_sanitize.go`（Sanitize/ToProbeConfig）、`config_snapshot_compat.go`（legacy 兼容迁移）、`config_snapshot_value.go`（取值/克隆工具）、`config_snapshot_options.go`、`export_template.go` |
| `appcore/probe_service.go` 1081 | 按命令域拆（启动/停止/恢复/快照/事件），执行时先 grep `^func` 定缝 |
| `task/head.go` 983 | 按 HEAD 探测阶段/判定/调试日志拆 |

### 5.2 Wave B：其余 21 个 400-900 行 Go 文件

同一手法（dnshe/dnspod/alidns/cloudflare 四个 DNS 核心包按"客户端/记录操作/错误归一化"拆；app/update.go 按"检查/下载/安装编排"拆；httpclient 按"传输/重试/profile"拆；installer/main.go 按"HTTP 路由/安装动作/窗口"拆）。每个文件拆前先 `grep '^func'` 列职责清单，拆后跑受影响包测试。

### 5.3 Wave C：Kotlin 3 个 + 测试 9 个

- `CfstPlugin.kt` 584 → 插件壳 + 按能力委托类（probe/file/update/notification bridge）。
- `AndroidUpdateDownloads.kt` 521 → 下载管理/通知/安装 Intent 三文件。
- `ProbeForegroundService.kt` 451 → 服务/通知构建/WakeLock 辅助。
- 测试按场景拆文件（`probe_strategy_test.go` 2430 优先，按策略阶段拆 5-6 个 `_test.go`）；纯搬移不改断言。
- Kotlin 验证：`gradlew ktlintMainSourceSetCheck detektDebug detektDebugUnitTest` + 既有 JVM 单测（全量 lint 链已含）。

### 5.4 Phase 3 验收

- `$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' }); go test $goPackages` 全绿；`go test -tags webui ./internal/app/` 全绿。
- 并发密集包（task/appcore/probecore/app）受影响者本地 `-race` 跑一遍。
- appcore 依赖边界测试（pre-commit 同款 3 条）全绿；生产 Go 文件全部 ≤400 行。
- 四个 stage-0 基准（sourceparse/probecore）`-benchmem -count=5` 取最小，B/op、allocs/op 无回归（纯搬移不应有分配变化）。

---

## 6. Phase 4：收尾固化（半天）

1. 文档同步：`docs/dev/agent-workflow-validation.md`（fast 闭环 + 各 Phase 新验证命令）、`docs/dev/agent-code-architecture.md`（bridge 门面/模块归位说明）、README 开发命令段（若新增用户可见命令）；`AGENTS.MD` 仅增补 fast 闭环协作规则一条。
2. 守卫全接线复核：pre-commit / check.ps1 / check.sh / CI Quality 均含 `check:stringcode`；fast 闭环含文件行数警告。
3. 终验：`.\scripts\checks\check.ps1` 全量 + CI Quality 全绿 + 安装器/WebUI/Android 三端人工冒烟各一遍。
4. 输出体检报告（可选交付）：重构前后文件行数分布、字符串代码命中数、闭环耗时对比，存 `docs/dev/` 或 artifacts。

---

## 7. 风险与兼容红线（每个 Phase 通用）

- **契约冻结**：配置 schema、bridge 字段、API/事件 payload、存储路径、发布资产、更新清单一律不变；拆分是纯结构迁移，不夹带行为重写。
- **Vapor 不变量**：新 SFC 一律 `<script setup vapor lang="ts">`；禁字符串 `<component :is>`、h()、JSX；vue 版本钉死不动。
- **视觉基线**：像素基线由 CI Windows runner 生成；本机不刷新不提交；纯重构预期像素零差异。
- **行尾/格式**：全仓 LF；不跑全树 oxfmt/gofmt；遇 CRLF 假阳性用 `git add --renormalize .`。
- **生成物**：Wails bindings、dist、payload 不手改；`go:embed` 新文件必须随源码提交。
- **平台引用收敛**：window.wails/@capacitor//api/ 等引用只允许在 `frontend/src/lib/` 下（既有 frontend-boundary 守卫）。
- **回滚**：每处独立提交，任一步失败即 revert 该提交，不影响已落地部分。
- **验证不能跑时**：记录原因与剩余风险（AGENTS 规则），不静默跳过。

## 8. 执行顺序与工作量

| 阶段 | 内容 | 预估 | 依赖 |
| --- | --- | --- | --- |
| Phase 0 | 秒级闭环（-short 守卫 + fast 脚本 + 增量 tsc 实验 + 入口文档） | 0.5-1 天 | 无 |
| Phase 1 | 5 处字符串代码歼灭 + no-string-code 守卫 | 1 天 | Phase 0 |
| Phase 2 | 前端拆解（类型单点 → App.vue → SettingsView → bridge → 视图/CSS） | 2-4 天 | Phase 0/1 |
| Phase 3 | Go Wave A/B + Kotlin/测试 Wave C | 2-3 天 | Phase 0（可与 Phase 2 并行交替） |
| Phase 4 | 文档固化 + 三端冒烟 + 终验 | 0.5 天 | 全部 |

合计约 6-9 个工作日；每个 Phase 独立可交付、可暂停。

## 9. 明确不做的事

- 不写代码生成器、AST 改写脚本、脚手架模板（用户明令：拆小文件比生成器管用）。
- 不搬 Go 包目录、不改包名、不建空壳分层（repository-structure-target 阶段 0 纪律）。
- 不动测试夹具里的静态 JSON raw-string（数据字面量，非拼接代码）。
- 不引入 PostCSS/autoprefixer、不降级 vue、不改 Tailwind v4 接线。
- 不做无关重构、不顺手改行为、不批量格式化。
