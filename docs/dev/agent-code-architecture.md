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
- 仓库根入口（`main.go`、`resources.go`、`frontend_assets.go`、`tray_icon*.go`）与 `cmd/**` 由 `mobileapi-isolation` 兜底规则覆盖，不再靠逐文件列举：`depguard` 的 `files` 只认带 `**/` 前缀的通配，裸 `*.go` 匹配不到根目录文件，会让入口成为治理盲区。该规则以 `**/*.go` 覆盖全仓，要求除 `mobileapi` 自身外任何 Go 文件都不得导入 `mobileapi`。
- `internal/contracttest` 是跨端契约测试支持包，目前只含 `_test.go` 且必须同时驱动桌面壳与 Android 桥，因此在 `.golangci.yml` 的 `exclusions.rules` 中对 `depguard` 豁免。若该包新增非测试代码，必须先登记层级并删除该豁免。

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
