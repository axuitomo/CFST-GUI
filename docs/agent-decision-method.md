# Agent Decision Method

Read this document before non-trivial implementation work, cross-file changes, config/API changes, or whenever multiple approaches are plausible.

## Decision Rule

When implementation is uncertain, compare the main options with brief pros/cons and a weighted score. The scoring is not ceremony; use it to make maintainable choices explicit.

Recommended weights:

| Criterion | Weight | Question |
| --- | ---: | --- |
| Correctness and compatibility | 30 | Does it preserve current behavior and handle old configs, old platforms, and invalid input? |
| Shared reuse | 20 | Does it reuse shared packages and reduce cross-platform duplication? |
| Simplicity | 15 | Does it solve the problem with few concepts and little over-design? |
| Maintainability | 15 | Can future maintainers understand, locate, and modify it? |
| Testing and validation | 10 | Can it be verified with automated tests or clear commands? |
| Documentation sync | 10 | Is user-visible behavior documented where needed? |

Score each option from 0-5 per criterion and compute:

```text
total = sum(score / 5 * weight)
```

Default to the highest score. If a lower-scoring option is chosen, document why and how the risk is controlled.

## Shared Reuse Bias

If a task involves cross-platform, cross-module, or reusable business rules, the shared-package option should score strongly on shared reuse. Choose a local implementation only when a shared abstraction would clearly increase complexity, harm performance, or blur package boundaries.

## Pros/Cons Template

```markdown
Option A: reuse or extend a shared package
Pros: reduces duplication; keeps cross-platform behavior consistent; centralizes tests.
Cons: may require package-boundary adjustment; can touch more files in the short term.
Weighted conclusion: prefer when it has the highest total.

Option B: implement locally in the current module
Pros: smaller immediate diff; fast; narrow blast radius.
Cons: can duplicate logic; future desktop/WebUI/Android behavior may diverge.
Weighted conclusion: use only for one-off, strongly local behavior with no reuse value.
```

## Socratic Checks

Use these questions before and after coding:

1. Is this a real problem, or an optimization for a hypothetical case?
2. What is the smallest viable change?
3. Does an existing shared package already provide similar behavior?
4. If the logic lives here, who can maintain it three months from now?
5. Could this affect desktop, WebUI, Android, CLI, or config compatibility?
6. Are there old data, failure paths, or counterexamples that break the approach?
7. Do user docs, developer docs, or release notes need to change?
