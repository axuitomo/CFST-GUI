# agents-progressive-disclosure

[![skills.sh](https://skills.sh/b/Caph-dev/agents-progressive-disclosure)](https://skills.sh/Caph-dev/agents-progressive-disclosure)

[English](#english)

「开源」是时候精简你的 `AGENTS.md` 了：By 渐进式披露。

`agents-progressive-disclosure` 是一个 skill，用来把膨胀的 `AGENTS.md`、
`CLAUDE.md` 或类似 agent 指令文件，重构成“精简路由入口 + 聚焦的 `docs/` 参考文件”。

核心思路是渐进式披露：根指令文件保持精简，只保留高频、长期有效、必须始终遵守的规则；项目细节、规范、技术文档和任务专项流程下沉到
`docs/`，由项目级 `AGENTS.md` 按需引用。

## 好处

- `AGENTS.md` 保持精简。
- 省上下文。
- 技术文档可以写得非常详细，因为只有在需要时才加载。也可以把技术文档进一步提炼成 skill，达到类似效果。
- 更容易维护。

> [!IMPORTANT]
> 在用完这个 skill 之后，务必亲自检查一遍。你可以问 agent：当前策略会不会区分得太细？有些常用流程是不是没必要放到
> `docs/` 下？

<!-- -->

> [!CAUTION]
> 其实将**全局** `AGENTS.md` 拆分的必要性并不高，主要是方便维护。**省上下文的收益几乎为 0**，反正都会命中缓存。
>
> 但是对于**项目级**的 `AGENTS.md` 用处就不小了，可以把项目相关的细节、规范、技术文档等都写到
> `docs/` 下，通过项目级的 `AGENTS.md` 来引用。

## 使用方法

```text
使用 $agents-progressive-disclosure，把当前 AGENTS.md 重构成精简入口文件和 docs/ 专项文档。
```

## 安装方法

### 推荐：skills CLI

```bash
npx skills add Caph-dev/agents-progressive-disclosure
```

也可以把这个仓库链接发给你的 agent，让它安装：

```text
https://github.com/Caph-dev/agents-progressive-disclosure
```

## 仓库结构

- `SKILL.md`：skill 行为、工作流和约束
- `agents/openai.yaml`：UI 元数据
- `README.md`：面向人的介绍和安装说明

本项目积极参与并认可 [LINUX DO 社区](https://linux.do)。

---

## English

`agents-progressive-disclosure` is a skill to refactor bloated `AGENTS.md`,
`CLAUDE.md`, or similar agent instruction files into a compact routing
entrypoint plus focused `docs/` reference files.

It is built around progressive disclosure: keep the root agent instruction file
short, move detailed project rules into focused documents, and make the root
file tell future agents which document to read for each kind of task.

## Why Use It

- Keep `AGENTS.md` concise.
- Save context by keeping low-frequency details out of the always-loaded entrypoint.
- Let technical documentation be detailed, because it is loaded only when needed.
- Make agent instructions easier to maintain over time.

> [!IMPORTANT]
> After using the skill, inspect the result yourself. A useful follow-up question
> is whether the strategy split the rules too finely, or whether some common
> workflows should stay in the entrypoint instead of moving under `docs/`.

<!-- -->

> [!CAUTION]
> Splitting a global `AGENTS.md` is not especially necessary. It is mostly useful
> for maintenance, and the context savings are close to zero because global
> instructions usually hit the cache anyway.
> For project-level `AGENTS.md` files, the value is much higher: project details,
> conventions, and technical docs can live under `docs/` and be referenced by the
> project-level `AGENTS.md`.

## What It Does

The skill guides an agent through:

1. Reading the existing instruction file fully.
2. Checking for contradictory rules before moving anything.
3. Classifying rules into always-on entrypoint rules and task-specific details.
4. Rewriting the root instruction file as a compact router.
5. Moving long-form guidance into focused `docs/` or platform-specific rule files.
6. Validating that the original rule intent was preserved.

## Usage

```text
Use $agents-progressive-disclosure to refactor the current AGENTS.md into a compact
entrypoint plus focused docs/ reference files.
```

You can also ask in Chinese:

```text
使用 $agents-progressive-disclosure，把当前 AGENTS.md 重构成精简入口文件和 docs/ 专项文档。
```

## Installation

### Recommended: skills CLI

Install with the [skills CLI](https://github.com/vercel-labs/skills) ([docs](https://www.skills.sh/docs)):

```bash
npx skills add Caph-dev/agents-progressive-disclosure
```

```bash
# Global install, available in all projects
npx skills add Caph-dev/agents-progressive-disclosure -g

# Install to Cursor only
npx skills add Caph-dev/agents-progressive-disclosure -a cursor -y

# List skills in this repo without installing
npx skills add Caph-dev/agents-progressive-disclosure --list
```

The CLI detects supported agents and wires the skill into the right directory.

### Manual Install

Ask your agent to install it:

```text
Install the skill from https://github.com/Caph-dev/agents-progressive-disclosure.
```

For Codex, clone the repository into your skills directory:

```zsh
git clone https://github.com/Caph-dev/agents-progressive-disclosure ~/.codex/skills/agents-progressive-disclosure
```

Restart the agent after installation so it picks up the new skill.

## Repository Structure

- `SKILL.md`: skill behavior, workflow, and guardrails
- `agents/openai.yaml`: UI metadata
- `README.md`: human-facing overview and installation guide
