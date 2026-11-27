# Agent Code Architecture Rules

Read this document before changing architecture, adding files, choosing where code belongs, or touching behavior shared by desktop, WebUI, Android, or config compatibility. For the full developer-facing boundary rules, read [`docs/dev/architecture-constraints.md`](architecture-constraints.md).

目标命名范式、当前包映射、模块连接方向与渐进迁移阶段见 [`docs/dev/repository-structure-target.md`](repository-structure-target.md)。

## Required Boundary Check

- Keep app entrypoints (`apps/desktop/`, `apps/webui/`, `apps/installer/`) as thin entry/resource adapters; shared embedded asset packages live in `build/` (`package buildassets`) and `packages/frontend/assets.go` (`package frontendassets`). Do not add new root Go packages unless a public module boundary is explicitly intended.
- Put cross-platform business behavior in `internal/kernel/appcore` or a focused `internal/kernel/domain/...` package before wiring it through `internal/adapters/{host,desktop,webui}` or `apps/android/bridge/mobileapi`.
- Keep CFST probe stages in `internal/kernel/domain/probe/task` and internal helpers in `internal/kernel/infra/utils`; these packages are implementation details, not public APIs.
- Keep frontend pages as orchestration; shared UI-independent logic belongs in `packages/frontend/src/lib/` (facade `bridge.ts`, pure contracts/codecs `bridge/*`, platform-agnostic RPC clients `client/*`, domain helpers) or `packages/frontend/src/composables`, and platform-specific runtime/channel implementations live solely in `packages/frontend/src/lib/adapters/`.
- Treat config schema, bridge fields, API shapes, event payloads, storage paths, release assets, and update manifests as compatibility contracts.

## Go Layered Dependencies

Go 代码按四层严格单向依赖组织：`L4 端侧壳层 -> L3 应用编排 -> L2 领域能力 -> L1 基础设施`，禁止任何反向依赖。

| 层级 | 目录 |
| --- | --- |
| L4 端侧壳层 / 传输适配 | `apps/desktop`、`apps/webui`、`apps/installer`、`internal/adapters/{host,desktop,webui}`、`apps/android/bridge/{mobileapi,sqlitegate}` |
| L3 应用编排 | `internal/kernel/appcore` |
| L2 领域能力 | `internal/kernel/domain/**`（`archivecore`、`config`、`dns/*`、`github/*`、`probe/*`、`sourceparse`、`updatecore`） |
| L1 基础设施 | `internal/kernel/infra/**`（`httpcfg`、`httpclient`、`runtimecleanup`、`sqliteprobe`、`utils`） |

- 新增 Go 包必须先归入某一层：`internal/kernel/infra/**`（L1）与 `internal/kernel/domain/**`（L2）由 `.golangci.yml` 中的目录前缀通配直接覆盖，若在 `internal/` 下新增非 `kernel/` 包会触发 `dependency_boundary_test.go` 报错；新增 L4 壳层包须补进 `.golangci.yml` 对应 `depguard` 规则。
- Go 分层依赖由 `.golangci.yml` 的 `depguard` 强制校验，严禁通过删除规则或添加 `//nolint:depguard` 绕过架构报错。每条 `deny` 的 `desc` 已写明修复方向（公共类型下沉、接口/回调注入，或由上层编排）。
- 端侧入口（`apps/desktop/**`、`apps/webui/**`、`apps/installer/**`）由 `mobileapi-isolation` 兜底规则覆盖：`depguard` 的 `files` 只认带 `**/` 前缀的通配，该规则以 `**/*.go` 覆盖全仓，要求除 `apps/android/bridge/mobileapi/**` 外任何 Go 文件都不得导入 `mobileapi`。
- `internal/contracttest` 是跨端契约测试支持包，目前只含 `_test.go` 且必须同时驱动桌面壳与 Android 桥，因此在 `.golangci.yml` 的 `exclusions.rules` 中对 `depguard` 豁免。若该包新增非测试代码，必须先登记层级并删除该豁免。

## Decision Bias

Prefer the smallest maintainable change that preserves existing desktop, WebUI, Android, and config behavior. Reuse established shared packages instead of copying rules into platform adapters or UI components.

## Validation

Run validation from the repository root in PowerShell:

```powershell
$goPackages = @(go list ./... | Where-Object { $_ -notmatch '/packages/frontend/node_modules(?:/|$)' })
go test $goPackages
pnpm install --frozen-lockfile
pnpm typecheck
pnpm build
```

For a narrower change, run only the affected commands. For documentation-only changes, use `rg` and `Test-Path` from PowerShell to check links, referenced scripts, commands, and paths against the current repository.
