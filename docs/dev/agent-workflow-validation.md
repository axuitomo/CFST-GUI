# Agent Workflow and Validation

Read this document before editing files and again before final handoff.

## Repository Environment

- Prefer PowerShell 7 (`pwsh`) for editing, navigation, `rg`/`fd`, Go, `pnpm`, Wails, tests, validation, and nested PowerShell processes. Fall back to Windows PowerShell (`powershell.exe`) only when `pwsh` is unavailable or a Windows PowerShell-specific compatibility requirement applies.
- Run commands from the real Windows drive working directory. Use explicit Windows paths when changing processes or invoking wrappers; do not rely on WSL paths, UNC translations, or an installed WSL distribution.
- Prefer native PowerShell cmdlets and Windows-native toolchains for packaging, signing, WebView2, NSIS, SignTool, and package-manager work.
- Do not use WSL or Bash for ordinary work. Use Bash only for an explicitly targeted Bash-specific script or release flow with no PowerShell-native equivalent.
- Package-manager and native-toolchain commands may run automatically when the task requires them.
- Line endings are LF repository-wide, in the working tree too: `.gitattributes` pins `* text=auto eol=lf` (plus explicit `binary` for `png`/`ico`/`jar`) and `.editorconfig` sets `end_of_line = lf`. Git content was already LF, so a CRLF checkout is only a local `core.autocrlf=true` artifact. When formatting checks flag hundreds of untouched files, do not run `prettier --write` or `gofmt -w` over the tree; refresh the index with `git add --renormalize .` (identical blobs, so nothing is staged) and re-check `git status`.

## Modification Flow

1. Read the relevant code, tests, and documentation before naming the edit.
2. Use the decision method for non-trivial work; prefer shared packages when the behavior is reusable.
3. Make the smallest necessary change and avoid unrelated refactors.
4. Add or update tests when behavior changes or risk warrants it.
5. Decide whether documentation must be updated; update the relevant file when needed.
6. Review the final diff for secrets, unrelated formatting, generated files, and accidental churn.

## Validation

Run the narrowest useful automatic validation:

- Go core or backend changes: from the repository root in PowerShell, run `$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' }); go test $goPackages`, or test the smallest relevant package set when full tests are too costly.
- WebUI or `internal/app` changes: also run the webui build-tag tests (`go test -tags webui ./internal/app/`). The tag is excluded from the default build, and `scripts/checks/check.sh`, `scripts/checks/check.ps1`, and the quality workflow each run it explicitly.
- Frontend changes: from the repository root, run `pnpm typecheck` and the necessary build or lint command for the touched area.
- Frontend Vapor Mode (Vue 3.6 RC, versions pinned in `frontend/package.json`): every own SFC uses `<script setup vapor lang="ts">`; `vite.config.ts` must keep aliasing bare `vue` to the `runtime-with-vapor` build (the default `vue` entry omits Vapor APIs), and `main.ts` installs `vaporInteropPlugin` so third-party VDOM components such as `@phosphor-icons/vue` keep rendering. Vapor does not support string-form `<component :is="...">`, `h()` render functions, or JSX — use imported component objects and templates. After frontend changes run typecheck/lint/build plus `tests/webui-smoke.spec.ts` (chromium at minimum; firefox/webkit when installed). Tailwind v4 is handled solely by `@tailwindcss/vite`; do not reintroduce a PostCSS/autoprefixer config. This invariant is enforced by `scripts/checks/vapor-mode.mjs`, wired into `.githooks/pre-commit`, `scripts/checks/check.sh`/`check.ps1`, and CI (`pnpm check:vapor`); run it directly with `pnpm check:vapor`.
- Build scripts or release logic: run the smallest affected script target, or explain why a full run was not possible.
- Documentation-only changes: use `rg` and `Test-Path` in PowerShell to check links, commands, paths, and version text against the current repository.

If validation fails, record the failure, what was attempted, and the remaining risk.

## Documentation Sync

After code changes, actively decide whether docs need updates.

Update documentation when changes affect:

- CLI parameters, runtime commands, build commands, or environment variables.
- Config fields, defaults, migration compatibility, or storage paths.
- GUI, WebUI, or Android user-visible behavior.
- APIs, events, bridge contracts, import/export behavior, DNS, WebDAV, or GitHub export behavior.
- Release artifacts, update manifests, or supported platforms.

Documentation may be unnecessary for purely internal refactors that leave behavior, commands, config, and interfaces unchanged, but the reason should be clear.

## AGENTS.MD Change Boundary

Do not edit `AGENTS.MD` while making ordinary product, deployment, API, or release-note documentation changes. Edit this file only when repository collaboration rules, agent behavior, or maintenance conventions themselves change.
