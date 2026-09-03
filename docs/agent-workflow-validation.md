# Agent Workflow and Validation

Read this document before editing files and again before final handoff.

## Repository Environment

- Prefer PowerShell 7 (`pwsh`) for editing, navigation, `rg`/`fd`, Go, `pnpm`, Wails, tests, validation, and nested PowerShell processes. Fall back to Windows PowerShell (`powershell.exe`) only when `pwsh` is unavailable or a Windows PowerShell-specific compatibility requirement applies.
- Run commands from the real Windows drive working directory. Use explicit Windows paths when changing processes or invoking wrappers; do not rely on WSL paths, UNC translations, or an installed WSL distribution.
- Prefer native PowerShell cmdlets and Windows-native toolchains for packaging, signing, WebView2, NSIS, SignTool, and package-manager work.
- Do not use WSL or Bash for ordinary work. Use Bash only for an explicitly targeted Bash-specific script or release flow with no PowerShell-native equivalent.
- Package-manager and native-toolchain commands may run automatically when the task requires them.

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
- Frontend changes: from the repository root, run `pnpm typecheck` and the necessary build or lint command for the touched area.
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
