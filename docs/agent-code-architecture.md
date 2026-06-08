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

Use project validation entrypoints rather than bare package globs:

```bash
bash scripts/check.sh
bash -lc 'source scripts/lib/common.sh; go test $(cfst_go_packages)'
```

For documentation-only changes, run `bash scripts/docs-check.sh` and verify paths against the current repository.
