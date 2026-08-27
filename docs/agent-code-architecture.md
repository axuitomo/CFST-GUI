# Agent Code Architecture Rules

Read this document before changing architecture, adding files, choosing where code belongs, or touching behavior shared by desktop, WebUI, Android, CLI, or config compatibility. For the full developer-facing boundary rules, read [`docs/architecture-constraints.md`](architecture-constraints.md).

## Required Boundary Check

- Keep root Go files as thin entry/resource adapters; do not add new root importable Go packages unless a public module boundary is explicitly intended.
- Put cross-platform business behavior in `internal/appcore` or a focused `internal/*core` package before wiring it through `internal/app` or `mobileapi`.
- Keep CFST probe stages in `internal/task` and internal helpers in `internal/utils`; these packages are implementation details, not public APIs.
- Keep frontend pages as orchestration; shared UI-independent logic belongs in `frontend/src/lib` or `frontend/src/composables`.
- Treat config schema, bridge fields, API shapes, event payloads, storage paths, release assets, and update manifests as compatibility contracts.

## Decision Bias

Prefer the smallest maintainable change that preserves existing desktop, WebUI, Android, CLI, and config behavior. Reuse established shared packages instead of copying rules into platform adapters or UI components.

## Validation

Run validation from the repository root in PowerShell:

```powershell
$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' })
go test $goPackages
pnpm install --frozen-lockfile
pnpm typecheck
pnpm build
```

For a narrower change, run only the affected commands. For documentation-only changes, use `rg` and `Test-Path` from PowerShell to check links, referenced scripts, commands, and paths against the current repository.
