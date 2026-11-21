# Agent Code Architecture Rules

Read this document before changing architecture, adding files, choosing where code belongs, or touching behavior shared by desktop, WebUI, Android, or config compatibility. For the full developer-facing boundary rules, read [`docs/dev/architecture-constraints.md`](architecture-constraints.md).

目标命名范式、当前包映射、模块连接方向与渐进迁移阶段见 [`docs/dev/repository-structure-target.md`](repository-structure-target.md)。

## Required Boundary Check

- Keep root Go files as thin entry/resource adapters; do not add new root importable Go packages unless a public module boundary is explicitly intended.
- Put cross-platform business behavior in `internal/appcore` or a focused `internal/*core` package before wiring it through `internal/app` or `mobileapi`.
- Keep CFST probe stages in `internal/task` and internal helpers in `internal/utils`; these packages are implementation details, not public APIs.
- Keep frontend pages as orchestration; shared UI-independent logic belongs in `frontend/src/lib` or `frontend/src/composables`.
- Treat config schema, bridge fields, API shapes, event payloads, storage paths, release assets, and update manifests as compatibility contracts.

## Go Layered Dependencies

Go 代码按四层严格单向依赖组织：`L4 端侧壳层 -> L3 应用编排 -> L2 领域能力 -> L1 基础设施`，禁止任何反向依赖。

| 层级 | 目录 |
| --- | --- |
| L4 端侧壳层 / 传输适配 | 根目录入口、`internal/app`、`mobileapi`、`cmd/installer` |
| L3 应用编排 | `internal/appcore` |
| L2 领域能力 | `internal/*core`、`internal/colodict`、`internal/config`、`internal/dnshecore`、`internal/dnspodcore`、`internal/dnspush`、`internal/doh`、`internal/githubdownload`、`internal/mcis`、`internal/sourceparse`、`internal/task` |
| L1 基础设施 | `internal/httpcfg`、`internal/httpclient`、`internal/utils`、`internal/sqliteprobe`、`internal/runtimecleanup`、`sqlitegate` |

- 新增 Go 包必须先归入某一层，并补进 `.golangci.yml` 中对应 `depguard` 规则的 `files` 与 `deny` 列表；未登记的包不受约束，等于绕过分层。
- Go 分层依赖由 `.golangci.yml` 的 `depguard` 强制校验，严禁通过删除规则或添加 `//nolint:depguard` 绕过架构报错。每条 `deny` 的 `desc` 已写明修复方向（公共类型下沉、接口/回调注入，或由上层编排）。
- `internal/contracttest` 是跨端契约测试支持包，不参与分层，因此不出现在任何 `depguard` 规则中。

## Decision Bias

Prefer the smallest maintainable change that preserves existing desktop, WebUI, Android, and config behavior. Reuse established shared packages instead of copying rules into platform adapters or UI components.

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
