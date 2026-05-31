# Agent Workflow and Validation

Read this document before editing files and again before final handoff.

## Modification Flow

1. Read the relevant code, tests, and documentation before naming the edit.
2. Use the decision method for non-trivial work; prefer shared packages when the behavior is reusable.
3. Make the smallest necessary change and avoid unrelated refactors.
4. Add or update tests when behavior changes or risk warrants it.
5. Decide whether documentation must be updated; update the relevant file when needed.
6. Review the final diff for secrets, unrelated formatting, generated files, and accidental churn.

## Validation

Run the narrowest useful automatic validation:

- Go core or backend changes: `go test ./...` or the smallest relevant package set when full tests are too costly.
- Frontend changes: from `frontend/`, run `npm run typecheck` and the necessary build or lint command for the touched area.
- Build scripts or release logic: run the smallest affected script target, or explain why a full run was not possible.
- Documentation-only changes: check links, commands, paths, and version text against the current repository.

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
