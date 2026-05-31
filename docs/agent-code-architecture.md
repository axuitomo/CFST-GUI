# Agent Code Architecture Rules

Read this document before changing architecture, adding files, choosing where code belongs, or touching behavior shared by desktop, WebUI, Android, CLI, or config compatibility.

## Code Quality

Code should be simple, clean, maintainable, and testable.

- Simplicity: each function should have one clear responsibility, with readable control flow and minimal hidden side effects.
- Cleanliness: use accurate names, remove dead code, avoid duplicated logic, magic values, and temporary debug output.
- Maintainability: follow existing project style and avoid unnecessary frameworks, directories, or abstractions.
- Testability: move core logic into testable Go packages or frontend utility functions instead of relying only on UI manual checks.
- Compatibility: config schema, CLI parameters, API responses, bridge fields, and mobile behavior must handle old data and old callers carefully.

## Shared Package Preference

Prefer existing shared locations before adding local implementations:

- Cross-platform or reusable Go logic: `internal/probecore`, `internal/archivecore`, `internal/cloudflarecore`, `internal/sourceparse`, `internal/httpclient`, or another established `internal/*core` package.
- App orchestration, Wails/WebUI entrypoints, and platform adaptation: `internal/app`.
- Android Go bridge behavior: `mobileapi`, while reusing stable `internal` logic where possible.
- Frontend bridge utilities, naming maps, input-source helpers, and shared UI-independent logic: `frontend/src/lib`.
- Documentation: `docs/`, with `README.md` updated for quick-start or important user-visible behavior.

Use a local implementation only when a shared abstraction would clearly add complexity, hurt performance, or pollute a core package boundary. If a local implementation gains a second caller later, reassess whether it should move into a shared package.

## Repository Organization

New files must live in directories that express their responsibility. Before creating a new directory, confirm:

1. Existing directories cannot express the responsibility.
2. The new directory will not overlap with an existing module.
3. The name makes the boundary obvious to maintainers.
4. Documentation indexes or README notes are updated when helpful.

## Examples

Avoid:

- Copying existing parsing, HTTP, archive, DNS, or bridge logic for a single page.
- Adding complex interfaces, plugin systems, or global state for hypothetical future requirements.
- Changing config fields without compatibility migration, tests, and documentation.
- Hardcoding reusable rules inside business components so desktop, WebUI, and Android behavior diverge.

Prefer:

- Moving cross-endpoint logic into shared core packages.
- Keeping frontend common logic in `frontend/src/lib`.
- Adding tests around new core behavior before wiring app, mobileapi, or UI layers to it.
- Updating `README.md`, `docs/index.md`, or the matching topic document when user-visible behavior changes.
