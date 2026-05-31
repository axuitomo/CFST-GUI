# agents-progressive-disclosure

[![skills.sh](https://skills.sh/b/Caph-dev/agents-progressive-disclosure)](https://skills.sh/Caph-dev/agents-progressive-disclosure)

[中文说明](#中文说明)

`agents-progressive-disclosure` is an agent skill for refactoring bloated `AGENTS.md`, `CLAUDE.md`, or similar agent instruction files into a compact routing entrypoint plus focused on-demand docs or rules files.

It helps keep always-loaded instructions small while preserving detailed rules in on-demand documentation.

## Purpose

Agent instruction files tend to grow over time. Teams add command examples, search policies, package-manager rules, deployment notes, tool-specific caveats, and project conventions until the root file becomes a dense rule warehouse.

That creates two problems:

- The most important rules lose signal because they sit beside low-frequency details.
- Every task pays the context cost for instructions that only matter in specific situations.

This skill turns the root instruction file back into a router.

## Philosophy

The core idea is progressive disclosure:

- Keep high-frequency, long-lived, must-always-follow rules in the entrypoint.
- Move detailed task-specific guidance into focused on-demand docs or rules files.
- Tell future agents exactly which doc or rule file to read for each task type.
- Preserve rule intent while reducing always-loaded context.

In short: `AGENTS.md` should act like a navigation page, not an encyclopedia.

## What It Does

This skill guides an agent through:

- Reading the existing instruction file fully.
- Scanning for contradictory instructions before moving rules.
- Classifying rules into always-on rules and task-specific details.
- Designing a small routed docs/rules map from the source rule clusters.
- Rewriting the root instruction file as a compact entrypoint.
- Moving long-form rules into focused docs or platform-specific rule files while preserving required frontmatter, imports, path globs, and file extensions.
- Validating that rules were preserved with a temporary preservation checklist.

## Workflow

1. Identify the target instruction file, usually `AGENTS.md`, `CLAUDE.md`, or `GEMINI.md`.
2. Inspect the existing file and current docs/rules structure.
3. Scan for contradictory instructions such as conflicting style rules, package-manager choices, test commands, safety boundaries, or precedence statements; resolve from evidence when possible and ask only when needed.
4. Classify each rule:
   - keep always-on rules in the entrypoint;
   - move detailed, task-specific rules into routed docs/rules files.
5. Design a docs/rules map from existing project conventions or source rule clusters.
6. Rewrite the entrypoint as a compact router with:
   - scope statement;
   - core principles;
   - read-on-demand docs index;
   - always-on rules;
   - priority rules.
7. Create or update focused docs/rules files.
8. Validate with line counts, preservation-checklist keyword checks, and a final read-through of the new entrypoint.

## Example Prompts

```text
Use $agents-progressive-disclosure to refactor the current AGENTS.md into a compact entrypoint plus focused routed docs/rules files.
```

```text
Apply progressive disclosure to this CLAUDE.md. Keep only always-on rules in the root file and move detailed workflow rules into routed docs or rule files.
```

```text
Use $agents-progressive-disclosure to reduce this project's agent instruction bloat while preserving all safety and tool-use rules.
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

---

## 中文说明

`agents-progressive-disclosure` 是一个 agent skill，用来把膨胀的 `AGENTS.md`、`CLAUDE.md` 或类似代理指令文件，重构成“精简入口文件 + 按需读取的专项文档或规则文件”的渐进式披露结构。

它的目标是在不丢失规则的前提下，减少每次任务都会加载的上下文。

## 目的

Agent 指令文件很容易越写越大。团队会不断把命令示例、搜索策略、包管理器规则、部署说明、工具细节和项目约定塞进根文件。

这样会产生两个问题：

- 最重要的规则被低频细节稀释，信号变弱。
- 每个任务都要承担无关规则的上下文成本。

这个 skill 的目标，是把根指令文件重新变成“路由入口”。

## 哲学思想

核心思想是渐进式披露：

- 高频、长期有效、必须始终遵守的规则留在入口文件。
- 具体任务才需要的长规则，下沉到聚焦的按需文档或规则文件。
- 在入口文件里明确告诉后续 agent：遇到什么任务应该读哪份文档或规则文件。
- 保留规则意图，同时降低常驻上下文负担。

一句话：`AGENTS.md` 应该是导航页，不应该是百科全书。

## 功能

这个 skill 会引导 agent 完成：

- 完整读取现有指令文件。
- 在移动规则前扫描相互矛盾的指令。
- 把规则分类为常驻规则和任务细节。
- 从源文件规则簇设计精简的文档/规则文件路由地图。
- 把根指令文件重写为紧凑入口。
- 将长规则移动到聚焦文档或平台专属规则文件，同时保留必要的 frontmatter、imports、path globs 和文件扩展名。
- 使用临时 preservation checklist 验证规则仍然被保留，并且能被检索到。

## 工作流

1. 确定目标指令文件，通常是 `AGENTS.md`、`CLAUDE.md` 或 `GEMINI.md`。
2. 检查现有文件和文档/规则文件结构。
3. 扫描相互矛盾的指令，例如冲突的代码风格、包管理器选择、测试命令、安全边界或优先级说明；能基于证据解决就直接解决，必要时才询问用户。
4. 分类每条规则：
   - 常驻规则留在入口文件；
   - 任务相关细节移动到路由文档或规则文件。
5. 根据项目既有约定或源文件规则簇设计文档/规则文件地图。
6. 把入口文件重写为紧凑路由器，包含：
   - 适用范围；
   - 核心原则；
   - 按需读取索引；
   - 常驻规则；
   - 优先级规则。
7. 创建或更新聚焦的文档/规则文件。
8. 通过行数、preservation-checklist 关键词检索和入口文件复读进行验证。

## 示例提示词

```text
使用 $agents-progressive-disclosure，把当前 AGENTS.md 重构成精简入口文件和聚焦的路由文档/规则文件。
```

```text
对这个 CLAUDE.md 应用渐进式披露：根文件只保留常驻规则，详细工作流移动到路由文档或规则文件。
```

```text
使用 $agents-progressive-disclosure，在保留安全和工具规则的前提下，降低项目 agent 指令文件的上下文膨胀。
```

## 安装方法

### 推荐：skills CLI

使用 [skills CLI](https://github.com/vercel-labs/skills) 安装：

```bash
npx skills add Caph-dev/agents-progressive-disclosure
```

```bash
# 全局安装，所有项目可用
npx skills add Caph-dev/agents-progressive-disclosure -g

# 只安装到 Cursor
npx skills add Caph-dev/agents-progressive-disclosure -a cursor -y

# 只列出仓库中的 skills，不安装
npx skills add Caph-dev/agents-progressive-disclosure --list
```

CLI 会检测支持的 agent，并把 skill 接入对应目录。

### 手动安装

让你的 agent 安装这个 skill：

```text
安装来自 https://github.com/Caph-dev/agents-progressive-disclosure 的 skill。
```

对于 Codex，可以克隆到 skills 目录：

```zsh
git clone https://github.com/Caph-dev/agents-progressive-disclosure ~/.codex/skills/agents-progressive-disclosure
```

安装后重启 agent，以便加载新 skill。

## 仓库结构

- `SKILL.md`：skill 行为、工作流和约束
- `agents/openai.yaml`：UI 元数据
- `README.md`：面向人的介绍和安装说明

本项目积极参与并认可 [linux.do社区](linux.do)
