# AGENTS.MD

> Scope: This is the project-level entrypoint for AI coding agents in this repository. Keep it compact; detailed rules live in focused `docs/agent-*.md` files and should be read only when relevant.

## Core Principles

1. Preserve correctness, stability, and backward compatibility across desktop, WebUI, Android, CLI, and config data unless the task explicitly asks for a breaking change and migration path.
2. Prefer KISS / YAGNI. Make the smallest change that solves the real current problem.
3. Reuse shared capabilities before adding local one-off logic. Common behavior should prefer `internal/*core`, `internal/sourceparse`, `internal/httpclient`, `frontend/src/lib`, or another existing shared module.
4. After code changes, decide whether user or developer documentation must also change. Update `README.md` or `docs/` when behavior, commands, config, APIs, builds, or platform support change.
5. Never expose or commit tokens, secrets, private keys, credentials, real user data, or private local paths.

## Read-On-Demand Index

| Task type | Read first | Trigger |
| --- | --- | --- |
| Architecture, code placement, shared package reuse | [`docs/agent-code-architecture.md`](docs/agent-code-architecture.md) | Before adding modules, moving logic, touching cross-platform behavior, or introducing abstractions |
| Implementation choices and trade-offs | [`docs/agent-decision-method.md`](docs/agent-decision-method.md) | Before non-trivial edits, cross-file changes, config/API changes, or when multiple approaches are plausible |
| Edit workflow, validation, and documentation sync | [`docs/agent-workflow-validation.md`](docs/agent-workflow-validation.md) | Before modifying files and again before final handoff |

## Always-On Rules

- Follow existing project style and ownership boundaries; avoid unrelated refactors, generated churn, and broad formatting sweeps.
- Use WSL as the primary day-to-day development environment. Default to WSL for writing code, repository navigation, `rg`/`fd` searches, Go commands, bash scripts, and most tests or validation runs.
- Mix WSL and Windows when that best matches the toolchain. Use Windows/PowerShell for `pnpm install`, frontend `pnpm lint` / `pnpm typecheck` / `pnpm build`, `wails dev` when the desktop shell or path handling is more reliable there, Windows desktop packaging and signing, and Android or Windows native toolchain tasks.
- Package-manager operations and native-toolchain commands may be run automatically by the agent when required for the task, but prefer the environment above instead of treating WSL and Windows as interchangeable defaults.
- Keep functions focused, names accurate, control flow clear, and core logic testable.
- Configuration schema, CLI arguments, API response shapes, bridge fields, and mobile behavior require explicit compatibility handling when changed.
- Ordinary product docs, deployment docs, API docs, and release notes must not trigger edits to this file. Modify `AGENTS.MD` only when agent collaboration rules themselves change.
- If validation cannot be run, state why and list the remaining risk.

## Priority

1. System, developer, and current user instructions.
2. The nearest applicable project instruction file.
3. This entrypoint.
4. Routed `docs/agent-*.md` details.

When rules appear to conflict, keep safety and compatibility first, then choose the simpler maintainable path that best fits the existing codebase.
