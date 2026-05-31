---
name: agents-progressive-disclosure
description: Refactor bloated AGENTS.md/CLAUDE.md-style instruction files into a compact router plus focused on-demand docs. Use when users ask to apply progressive disclosure, split agent rules into docs, reduce instruction bloat, or turn one large rule file into an entrypoint.
---

# Agents Progressive Disclosure

Use this skill to convert a long agent instruction file into a high-signal entrypoint that routes to focused documentation files. The goal is to preserve rules while reducing always-loaded context.

## Core Model

Treat the root instruction file as a router, not a rule warehouse.

- The entry file keeps only high-frequency, long-lived, must-always-apply rules.
- Detailed task-specific rules move into `docs/` files.
- The entry file includes a clear “read this doc when...” index.
- The agent should load only the docs relevant to the current task.

## Workflow

1. Identify the target instruction file.
   - Prefer the current directory's `AGENTS.md` unless the user names another file.
   - Also support `CLAUDE.md`, `GEMINI.md`, or project-specific equivalents.
   - If multiple candidates exist and the user did not specify one, report the candidates and choose the file most broadly used by the current agent setup; ask only when that choice is unclear or risky.

2. Inspect existing structure.
   - Read the target file fully.
   - List existing `docs/` files, if any.
   - Check for backups before editing.

3. Detect contradictions before classification.
   - Scan for conflicting style rules, package-manager choices, test commands, safety boundaries, precedence statements, or duplicated rules with different wording.
   - Resolve contradictions from explicit user instructions, nearest instruction-file priority, or clear local evidence when possible.
   - Ask the user only when a contradiction affects the refactor and cannot be resolved from available evidence.

4. Classify rules into buckets.
   - Keep in entrypoint: language defaults, safety boundaries, tool priority, conflict priority, and critical must-always-follow rules.
   - Move to docs: command catalogs, search strategies, framework-specific instructions, package-manager rules, environment setup, Git workflow, deployment, testing, document style, domain terminology, long examples.

5. Design the docs map.
   - Prefer the project's existing agent-doc convention; otherwise default to `docs/` for cross-agent portability.
   - Existing conventions may include `.claude/`, `.claude/rules/`, `.cursor/rules/`, or another agent-doc directory.
   - Use existing document names when they already fit; otherwise derive focused names from the source rule clusters.
   - Preserve platform-specific file formats, required frontmatter, imports, path globs, and file extensions.
   - Create `docs/README.md` only when it helps navigate multiple docs.

6. Edit conservatively.
   - Create a same-directory backup with a non-overwriting name before replacing the target file.
   - Rewrite the entrypoint as a compact router with:
     - scope statement;
     - core principles;
     - on-demand docs index;
     - always-on safety/tool rules;
     - precedence rules.
   - Move detailed rules into docs without changing their intent.
   - Avoid duplicating the same long rule in multiple places.

7. Validate preservation.
   - Create a short preservation checklist that maps each original rule category to its new location, one verification keyword or heading, and a pass/fail result.
   - Treat the checklist as temporary validation work; do not write it into the repo unless the user asks or the project already has a migration-note convention.
   - Compare line counts before and after.
   - Search for checklist keywords from the original file across the new entrypoint and routed docs or rules files.
   - Verify the entrypoint tells future agents when to read each doc.
   - Check that no doc contradicts the entrypoint.

## Suggested Entrypoint Shape

```md
# Agent Instructions

> Scope: This file is the entrypoint. It keeps only always-on rules; task details live in docs/.

## Core Principles

- [language/default behavior]
- [safety boundary]
- [primary tool or evidence policy]
- [this file is a router, not a warehouse]

## Read-On-Demand Index

| Task type | Read first | Trigger |
| --- | --- | --- |
| [rule cluster] | `[focused-doc]` | [when to read it] |
| [rule cluster] | `[focused-doc]` | [when to read it] |

## Always-On Rules

- [short critical rules]

## Priority

1. User's current explicit instruction.
2. Nearest project instruction file.
3. This file.
4. Routed docs details.
```

## Validation Commands

Use commands compatible with the current environment, replacing placeholders with the actual target file, routed docs/rules files, and preservation-checklist keywords. The examples below use POSIX-style shell commands; adapt them for Windows or other shells when needed.

```zsh
find <docs-dir> -maxdepth 1 -type f -print | sort
wc -l <target-file> <routed-docs...>
rg -n '<checklist-keyword>|<another-checklist-keyword>' <target-file> <routed-docs...>
sed -n '1,180p' <target-file>
```

Choose checklist keywords from the source file, not from this template.

## Guardrails

- Do not delete rules merely because they are verbose; move them to the right doc.
- Do not bury safety-critical rules only in a routed doc.
- Do not create many tiny docs with overlapping responsibilities.
- Do not add project-specific opinions that were not in the source file unless the user asks.
- Do not claim installation into Codex unless you actually copy or install the skill into the active skills directory and verify it.
