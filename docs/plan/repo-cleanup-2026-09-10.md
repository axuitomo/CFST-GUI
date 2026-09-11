# 仓库整理 DIFF 修改文档（2026-09-10）

> 本文档记录本次 CFST-GUI 仓库常规整理的完整变更，供提交（commit）与日后追溯使用。
> 基线提交：`99f7afb docs: add Android device debug and log guide`

## 一、决策确认

| # | 决策点 | 确认结果 |
| --- | --- | --- |
| 1 | docs 分类档位 | 完整档（子目录分类） |
| 2 | 调试脚本 / Python 库 | 保留在仓库（`devtools/`） |
| 3 | 浏览器档案 | 删除 `edge-profile/`；保留 `chrome-profile/` 并添加忽略规则 |
| 4 | `device-apk/`（25MB） | 归档到 `build/artifacts/device-apk/` |
| 5 | 历史 CSV（90 个被跟踪文件） | 删除（git 索引 + 磁盘） |
| 6 | 依赖缓存 | 不清理 |

## 二、变更统计

| 类型 | 数量 | 说明 |
| --- | --- | --- |
| 修改（M） | 13 | `.gitignore`、`AGENTS.MD`、`README.md`、`docs/index.md`、`scripts/build/version-bump.sh`、`scripts/checks/docs-check.sh`、`frontend/src/App.vue`、`frontend/src/views/SettingsView.vue`、`frontend/src/views/SourcesView.vue`、`frontend/dist/index.html`、`mobile/android/.../AndroidManifest.xml`、`MainActivity.kt`、`SchedulerWorker.kt` |
| 修改（整批提交时一并纳入） | 2 | `frontend/src/lib/bridge.ts`（删除未使用的 `fetchSource` 导出）、`scripts/checks/check-android-apk-manifest.sh`（补 `SystemForegroundService` 的 dataSync 断言） |
| 删除（D） | 93 | `cfst-results/**/*.csv` 90 个历史测速结果（6/18–8/31）、`frontend/package.json.md5`、2 个旧 `frontend/dist/assets/` 哈希文件 |
| 重命名（R/RM） | 19 | `docs/` 分类归档（`quick-start.md` 与 `功能与相关接口文档.md` 在改名后又有内容修改，故状态为 `RM`） |
| 新增（已跟踪） | 17 | `devtools/README.md` + `devtools/cdp/` 11 个自研调试脚本、`docs/plan/repo-cleanup-2026-09-10.md`、2 个新 `frontend/dist/assets/` 哈希文件、`WebViewDevToolsRelay.kt` 及其单测 |
| 归档（未跟踪） | 1 目录 | `build/artifacts/device-apk/`（25MB，按决策归档到忽略目录，不提交） |

> 清理基线之后，同一工作日又追加了两组改动并一并纳入本次提交：前端输入源页精简（移除“抓取”入口、`storage` 区默认折叠、去掉卡片底色与“运行模式”只读字段）与 Android 调试/后台修复（新增 WebView CDP relay、按 `FLAG_DEBUGGABLE` 开关 WebView 调试、`SchedulerWorker` 补前台服务类型、manifest 合并 `SystemForegroundService`）。旧记录提到的 `mobile/android/capacitor.settings.gradle` 改动当前已不在工作区（与 `HEAD` 无差异）。

## 三、分区变更明细

### A. 文档区（docs 完整档分类）

| 原路径 | 新路径 |
| --- | --- |
| `介绍产品.md`（根目录） | `docs/guide/介绍产品.md` |
| `docs/quick-start.md` | `docs/guide/quick-start.md` |
| `docs/configuration.md` | `docs/guide/configuration.md` |
| `docs/deployment.md` | `docs/guide/deployment.md` |
| `docs/docker-env.md` | `docs/guide/docker-env.md` |
| `docs/agent-code-architecture.md` | `docs/dev/agent-code-architecture.md` |
| `docs/agent-decision-method.md` | `docs/dev/agent-decision-method.md` |
| `docs/agent-workflow-validation.md` | `docs/dev/agent-workflow-validation.md` |
| `docs/architecture-constraints.md` | `docs/dev/architecture-constraints.md` |
| `docs/behavior-baseline.md` | `docs/dev/behavior-baseline.md` |
| `docs/bridge-trace-ddd-design.md` | `docs/dev/bridge-trace-ddd-design.md` |
| `docs/cli.md` | `docs/dev/cli.md` |
| `docs/telegram-bot.md` | `docs/integration/telegram-bot.md` |
| `docs/cloudflare-api-token.md` | `docs/integration/cloudflare-api-token.md` |
| `docs/github-pat.md` | `docs/integration/github-pat.md` |
| `docs/upload-design.md` | `docs/integration/upload-design.md` |
| `docs/android-mobile.md` | `docs/mobile/android-mobile.md` |
| `docs/android-debug.md` | `docs/mobile/android-debug.md` |
| `docs/功能与相关接口文档.md` | `docs/reference/功能与相关接口文档.md` |

最终结构：`docs/{index.md, guide/, dev/, integration/, mobile/, reference/, release-notes/, plan/}`。

**同步更新（链接修复）**：

- `AGENTS.MD`：Read-On-Demand Index 中 3 个 `docs/agent-*.md` 路径改为 `docs/dev/`（共 4 处引用）。
- `README.md`：「文档入口」表格 13 行、`android-mobile.md#...` 锚点、`配置详解` 正文链接、`仓库结构` 章节（新增 `devtools/` 与 `docs/` 说明）、「项目结构」树（docs 分类说明、新增 `devtools/` 与 `cfst-results/` 行）。
- `docs/index.md`：快速入口表格 16 行 + 文档地图 13 处路径全部更新。
- `docs/reference/功能与相关接口文档.md`：内部引用改为 `../guide/`、`../dev/`、`../mobile/` 相对路径（5 处）。
- `docs/dev/behavior-baseline.md`：`(deployment.md)` → `(../guide/deployment.md)`。
- `docs/dev/cli.md`：`docker-env.md` → `../guide/docker-env.md`。
- `docs/guide/deployment.md`：`android-mobile.md` → `../mobile/android-mobile.md`（2 处）。
- `docs/guide/configuration.md`：`cloudflare-api-token.md`、`github-pat.md` → `../integration/`（2 处）。
- `docs/integration/{telegram-bot,cloudflare-api-token,github-pat}.md`：`./configuration.md` → `../guide/configuration.md`。
- `docs/integration/{cloudflare-api-token,github-pat}.md`：`../internal/...go` → `../../internal/...go`。
- `scripts/checks/docs-check.sh`：markdown 扫描入口 `介绍产品.md` → `docs/guide/介绍产品.md`。
- `scripts/build/version-bump.sh`：版本号同步目标 `docs/docker-env.md`、`docs/deployment.md` → `docs/guide/`。

### B. 工具环境区（cfst-results 剥离）

| 原位置 | 去向 |
| --- | --- |
| `cfst-results/cdp_*.py`（7 个）、`expr_*.js`（3 个）、`viz_debug_chain.html` | `devtools/cdp/` |
| `cfst-results/cdp-lib/`（websocket Python 库、wsdump.exe 等） | `devtools/lib/cdp-lib/` |
| `cfst-results/chrome-profile/`（131MB） | `devtools/profiles/chrome-profile/`（保留） |
| `cfst-results/edge-profile/`（189MB） | 删除 |
| `cfst-results/device-apk/`（含 23.8MB APK、4 个 mjs 脚本、截图/日志） | `build/artifacts/device-apk/` |
| `cfst-results/bridge-debug.log`（0 字节） | 删除 |

`.gitignore` 新增：

```gitignore
# Debug toolchain runtime data (sensitive browser profiles)
/devtools/profiles/
```

### C. 常规清理

- 删除根目录 `AGENTS.md.bak`（备份残留，原已被 `*.bak` 规则忽略）。
- 依赖缓存（`node_modules/`、`.pnpm-store/` 等）按决策不清理。

### D. git 治理

- `git rm -r cfst-results`：从索引与磁盘删除全部 90 个历史 CSV（git 历史中仍可追溯，未重写历史）。
- `.gitignore` 中 `/cfst-results/` 规则保持，后续运行结果不再进入版本库。

## 四、验证结果

| 检查项 | 结果 |
| --- | --- |
| `scripts/checks/docs-check.sh` | 通过（54 个 markdown 文件，无断链；移除了对 `介绍产品.md` 的重复扫描） |
| `pnpm lint:md`（markdownlint-cli2） | 通过，55 个文件 0 问题 |
| `pnpm typecheck`、`pnpm lint` | 通过 |
| `pnpm test`（vitest） | 通过，4 个文件 11 个用例 |
| `./gradlew detekt ktlintMainSourceSetCheck` | 通过（分两轮累计报出 `WebViewDevToolsRelay.kt` 的 6 个 detekt 问题，已按最小改动处理） |
| `./gradlew :app:testDebugUnitTest` | 通过；`WebViewDevToolsRelayTest` 4 个用例（含新增的请求头解析用例） |
| `pnpm build` 重建 `frontend/dist` | 产物哈希与已提交版本一致，无漂移 |
| `pnpm format:check` | 18 个文件因 Windows `core.autocrlf` 产生 CRLF 假阳性；按内容比对（去 CR 后）与 Prettier 输出逐行一致，Linux CI 不受影响 |
| `scripts/checks/secrets-scan.sh` | 通过，无疑似密钥 |
| `git status` | 142 项变更，与整理范围一致 |
| `git check-ignore devtools/profiles/...` | 已生效（忽略规则命中） |
| `cfst-results/` | 已清空（历史 CSV 删除，新结果由程序重新写入） |
| `scripts/checks/validate-results.sh --dir cfst-results` | 提示 "No CSV files found"（预期：目录为空） |

## 五、工具链整理（补充，2026-09-10）

根目录工具链审计结论：`tools/`（Go build-tag，gomobile 依赖）、`.agents/`（agent skill）、`skills-lock.json`（skill 版本锁）、`.codegraph/`（本地数据忽略规则）、`scripts/{build,checks,dev,lib}/` 均为运行时依赖或已分层规范，无需改动。

实际执行：

1. `devtools/README.md` 新增：说明 `cdp/`（自研脚本，提交）、`lib/`（第三方库，不提交）、`profiles/`（敏感档案，不提交）的定位与使用方式。
2. `devtools/cdp/` 11 个调试脚本 `git add` 纳入版本控制（"保留在仓库"完整落地）。
3. `.gitignore` 新增 `/devtools/lib/`（第三方 Python 库 110 个文件，保留磁盘不提交）。
4. 修复 `devtools/cdp/` 7 个 `.py` 脚本中硬编码的本机绝对路径 `C:\Users\Administrator\Desktop\CFST-GUI\cfst-results\cdp-lib` → 基于脚本位置的相对路径 `os.path.join(..., "..", "lib", "cdp-lib")`（原路径因 cdp-lib 迁移已失效，且绝对路径不可移植）；`python -m py_compile` 全部通过。

## 六、遗留说明

1. **提交范围**：`devtools/cdp/`（自研脚本）已 `git add` 纳入版本控制；`devtools/lib/`（第三方 Python 库 110 个文件）与 `devtools/profiles/`（含登录态的浏览器档案）保持忽略，仅在磁盘保留；`build/artifacts/device-apk/` 同样留在忽略目录中不提交。
2. **mobile 改动**：`MainActivity.kt`、`SchedulerWorker.kt`、`AndroidManifest.xml` 的修改与 `WebViewDevToolsRelay.kt` 及其单测已纳入本次提交；Android 侧只做了单元测试与 manifest 合并结果核验，未在本机重跑 `arm64-v8a` 真机联调，上机验证仍按 `docs/mobile/android-debug.md` 自行执行。`check-android-apk-manifest.sh` 已补 `SystemForegroundService` 的 dataSync 类型断言，该脚本需要 APK 产物，本次未运行。
3. **历史 CSV**：如需回溯旧结果，可通过 git 历史（`git show <commit>:cfst-results/...`）恢复；工作区不再保留。
4. **docs/plan/**：本目录现用于存放整理方案与变更记录文档。
5. **`frontend/package.json.md5`**：过期生成物（基线 hash 与当前 `package.json` 不符，全库零引用），经用户确认已 `git rm` 删除（索引+磁盘，git 历史可追溯）。

## 七、追加改动与修正（同批提交）

### A. 前端输入源页精简

- `SourcesView.vue`：移除 4 处“抓取”按钮、`requestSourceFetch()` 与 `fetch-source` 事件；剩余按钮的栅格由 `grid-cols-4/2` 收敛为 `grid-cols-3/1`；两处输入组标题去掉 `bg-slate-50/70` 底色。
- `App.vue`：`inspectSource()` 去掉 `action` 参数与 fetch 分支，只调用 `previewSource()`；删除仅服务 fetch 的 `applySourceStatus()`。输入源读取状态仍由 `probe:event` 的 `source_statuses` 与配置加载路径刷新。
- `SettingsView.vue`：删除“运行模式 / 单任务模式”只读字段，`storage` 区默认折叠。
- `bridge.ts`：删除已无调用方的 `fetchSource()` 导出（后端 `source.fetch` 命令与单测保留）。
- 文档口径同步：`docs/guide/quick-start.md` 第 8 步、`docs/reference/功能与相关接口文档.md` 的 3.3 链路图与两处 bridge 方法表改为只描述“预览”，并注明 `source.fetch` 仍作为命令保留。
- `frontend/dist` 已重建；`pnpm build` 复现同一组产物哈希。

### B. Android 调试与后台修复

- 新增 `WebViewDevToolsRelay.kt`：Debug 构建在设备 `127.0.0.1:9223` 监听，读取请求头后转发到 `webview_devtools_remote_<pid>`；仅剥离 `Origin: devtools://devtools` 与 `Origin: chrome-devtools://`，其他 Origin 保持由 Chromium 自行拒绝。
- `MainActivity.kt`：按 `FLAG_DEBUGGABLE` 在创建 WebView 前调用 `WebView.setWebContentsDebuggingEnabled()`，Debug 构建随后启动 relay；Release 构建两者都不启用。
- `SchedulerWorker.kt`：`ForegroundInfo` 显式传入 `FOREGROUND_SERVICE_TYPE_DATA_SYNC`；`AndroidManifest.xml` 用 `tools:node="merge"` 为 WorkManager 的 `SystemForegroundService` 补 `foregroundServiceType="dataSync"`（WorkManager 2.11.2，`targetSdk 37`）。合并后的 manifest 已核对：`dataSync` 类型生效且仍 `exported=false`。
- `docs/mobile/android-debug.md` 记录 relay 端口、`adb forward` 用法、DevTools frontend URL 与数据外泄风险。

### C. 本次修正（针对上方两组的遗留问题）

| 问题 | 处理 |
| --- | --- |
| `WebViewDevToolsRelay.kt` 触发 detekt 失败（`TooManyFunctions`、2× `NestedBlockDepth`、`ComplexCondition`、`MagicNumber`×2），会使 `quality.yml` 的 android job 与 `scripts/checks/lint.sh` 失败 | 抽出 `forward()`、合并原 `relay()` 到 `relayAsync()`、`readHeaders()` 改用 4 字节滚动窗口、backlog 与位移量提成常量 |
| `Read-On-Demand` 目录外的文档仍描述已移除的“抓取”UI 入口 | 同步 `quick-start.md` 与接口文档，保留后端命令语义 |
| `docs-check.sh` 对 `docs/guide/介绍产品.md` 重复扫描 | 删除多余 `push`（该文件已随 `docs/` 一起 walk） |
| WorkManager 前台服务类型缺少回归断言 | `check-android-apk-manifest.sh` 增加 `SystemForegroundService` 的 dataSync 类型断言 |
| `WebViewDevToolsRelayTest` 未覆盖请求头解析 | 补充正常终止与截断两种用例（`readHeaders` 改为 `internal`） |
