# Agent Workflow and Validation

Read this document before editing files and again before final handoff.

## Repository Environment

- Prefer PowerShell 7 (`pwsh`) for editing, navigation, `rg`/`fd`, Go, `pnpm`, Wails, tests, validation, and nested PowerShell processes. Fall back to Windows PowerShell (`powershell.exe`) only when `pwsh` is unavailable or a Windows PowerShell-specific compatibility requirement applies.
- Run commands from the real Windows drive working directory. Use explicit Windows paths when changing processes or invoking wrappers; do not rely on WSL paths, UNC translations, or an installed WSL distribution.
- Prefer native PowerShell cmdlets and Windows-native toolchains for packaging, signing, WebView2, SignTool, and package-manager work.
- Do not use WSL or Bash for ordinary work. Use Bash only for an explicitly targeted Bash-specific script or release flow with no PowerShell-native equivalent.
- Package-manager and native-toolchain commands may run automatically when the task requires them.
- Line endings are LF repository-wide, in the working tree too: `.gitattributes` pins `* text=auto eol=lf` (plus explicit `binary` for `png`/`ico`/`jar`) and `.editorconfig` sets `end_of_line = lf`. Git content was already LF, so a CRLF checkout is only a local `core.autocrlf=true` artifact. When formatting checks flag hundreds of untouched files, do not run `oxfmt --write` or `gofmt -w` over the tree; refresh the index with `git add --renormalize .` (identical blobs, so nothing is staged) and re-check `git status`.

## Modification Flow

1. Read the relevant code, tests, and documentation before naming the edit.
2. Use the decision method for non-trivial work; prefer shared packages when the behavior is reusable.
3. Make the smallest necessary change and avoid unrelated refactors.
4. Add or update tests when behavior changes or risk warrants it.
5. Decide whether documentation must be updated; update the relevant file when needed.
6. Review the final diff for secrets, unrelated formatting, generated files, and accidental churn.

## Performance Baselines and PGO

Claim performance wins only with the four stage-0 benchmarks, and quote allocations rather than wall clock:

- `go test ./internal/sourceparse/ -bench ParseFlatIPs -benchmem` covers a flat IP list, the most expensive input shape (one regexp plus one `net.IP` normalization per line).
- `go test ./internal/probecore/ -bench . -benchmem` covers CIDR enumeration, candidate dedupe, and the full source build.

`ns/op` swings by ±50% on a loaded machine (an open `wails3 dev` is enough), `B/op` and `allocs/op` do not. Compare runs with `-count=5` and take the minimum.

PGO is opt-in through `default.pgo` in the repository root: `go build` uses it automatically when the file exists and does nothing when it does not, so removing the file is the rollback. Generate it with `task pgo`, which launches the GUI under `CFST_CPU_PROFILE` (an `internal/app` hook that also gives ad-hoc profiling of reported slowdowns). Run one full probe in the GUI, then close it; a profile captured from benchmarks or tests misses the network-bound hot path and is not worth committing.

## Validation

Run the narrowest useful automatic validation:

- Fast loop (default after every code edit): `pnpm check:fast` (`scripts/checks/fast.ps1`, bash twin `scripts/checks/fast.sh`) runs the minimal check set for the changed area — Go `-short` on affected packages plus the appcore dependency-boundary gate when appcore itself is untouched, frontend oxlint/vitest/vue-tsc in parallel via direct node invocation, vapor-mode + frontend-boundary + no-string-code, shellcheck (or `bash -n` when shellcheck is absent) on changed `scripts/*.sh`, and a >400-line split-red-line warning (non-blocking). Fix reported errors yourself before handoff. With no changes it runs a minimal full sweep. The full gate (`scripts/checks/check.ps1` / CI Quality) is for handoff and CI.

- 门禁只有一份定义：`.github/actions/quality-gate/action.yml`（工具链版本 + `doctor.sh --strict` + `ci-local.sh`）。`Quality` 与 `Release` 两个工作流都 `uses` 这个 composite action，tag 推送因此不会跑到分支推送没跑过的检查。要改门禁步骤、工具版本或 `CFST_REQUIRE_*` 覆盖，只改这个 action；不要在 workflow 调用点补装工具或另加 env，否则「本地与分支绿、发布红」会重新出现。前端格式检查固定为全量（`format-check.sh` 用 `git ls-files` 列文件，遵守 `.gitignore`），不再按改动集裁剪。

- Go core or backend changes: from the repository root in PowerShell, run `$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' }); go test $goPackages`, or test the smallest relevant package set when full tests are too costly.
- WebUI or `internal/app` changes: also run the webui build-tag tests (`go test -tags webui ./internal/app/`). The tag is excluded from the default build, and `scripts/checks/check.sh`, `scripts/checks/check.ps1`, and the quality workflow each run it explicitly.
- Concurrency-dense packages (`internal/task`, `internal/appcore`, `internal/probecore`, `internal/app`): the quality workflow runs them under `-race`; run the affected package with `-race` locally before handing off worker-pool, event-stream, or task-runtime changes.
- Frontend changes: from the repository root, run `pnpm typecheck` and the necessary build or lint command for the touched area.
- Frontend Vapor Mode (Vue 3.6 RC, versions pinned in `frontend/package.json`): every own SFC uses `<script setup vapor lang="ts">`; `vite.config.ts` keeps `features.vapor: true` and relies on the bare `vue` entry re-exporting the Vapor APIs (Vue 3.6.0-rc.13+ `runtime.esm-bundler` does; do not downgrade past that RC or alias `vue` to a prebuilt entry that omits them), and `main.ts` installs `vaporInteropPlugin` so third-party VDOM components such as `@phosphor-icons/vue` keep rendering. Vapor does not support string-form `<component :is="...">`, `h()` render functions, or JSX — use imported component objects and templates. After frontend changes run typecheck/lint/build plus `tests/webui-smoke.spec.ts` (chromium at minimum; firefox/webkit when installed). Tailwind v4 is handled solely by `@tailwindcss/vite`; do not reintroduce a PostCSS/autoprefixer config. This invariant is enforced by `scripts/checks/vapor-mode.mjs`, wired into `.githooks/pre-commit`, `scripts/checks/check.sh`/`check.ps1`, and CI (`pnpm check:vapor`); run it directly with `pnpm check:vapor`.
- UI 回归基线（`tests/__screenshots__/`）：像素基线由 CI 的 Windows runner 生成，本机渲染与 runner 存在栅格化差异，本地生成的基线在 CI 上必然不匹配。刷新方式：手动触发 `Quality` 工作流并勾选 `update_visual_baselines`，下载 `windows-visual-baselines` 产物覆盖 `tests/__screenshots__/chromium-win32/`；不要在本机跑 `pnpm test:visual -- --update-snapshots` 后直接提交。
- 字符串代码防回流（`docs/dev/refactor-plan.md` §3.6）：HTML/CSS/JS/配置文件必须是真实文件或模板+工具组装，不得字符串拼装。`scripts/checks/no-string-code.mjs` 做正则/词法级扫描（注释、普通字符串与模板 `${}` 插值被掩码，不误伤合法插值与 `<` 比较）：`frontend/src`（`.ts` 全文、`.vue` 仅 `<script>` 块）禁 `innerHTML`/`outerHTML=`/`insertAdjacentHTML`/`document.write`/`cssText`/`insertRule`/`new Function`/`eval(` 与模板字面量内的完整 HTML 标签；非测试 `*.go` 禁 raw string 含 HTML markup、禁 `Sprintf` 拼 JSON 对象起始（mpb `{{...}}` 进度条模板不是 JSON）；`scripts/build/*.sh` 禁 heredoc 写配置目标（usage 帮助 heredoc 放行）。豁免登记：`version-bump.sh` 发布 notes heredoc（`.md`，非配置）；Kotlin/mobileapi 当前 0 命中暂不纳入。接线：`.githooks/pre-commit`（变更文件触发）、`check.sh`/`check.ps1`、`fast.ps1`/`fast.sh`、CI（`pnpm check:stringcode`）；直接跑 `pnpm check:stringcode`。
- Build scripts or release logic: run the smallest affected script target, or explain why a full run was not possible.
- Documentation-only changes: use `rg` and `Test-Path` in PowerShell to check links, commands, paths, and version text against the current repository.

If validation fails, record the failure, what was attempted, and the remaining risk.

## Documentation Sync

After code changes, actively decide whether docs need updates.

Update documentation when changes affect:

- Runtime commands, build commands, or environment variables.
- Config fields, defaults, migration compatibility, or storage paths.
- GUI, WebUI, or Android user-visible behavior.
- APIs, events, bridge contracts, import/export behavior, DNS, WebDAV, or GitHub export behavior.
- Release artifacts, update manifests, or supported platforms.

Documentation may be unnecessary for purely internal refactors that leave behavior, commands, config, and interfaces unchanged, but the reason should be clear.

## AGENTS.MD Change Boundary

Do not edit `AGENTS.MD` while making ordinary product, deployment, API, or release-note documentation changes. Edit this file only when repository collaboration rules, agent behavior, or maintenance conventions themselves change.
