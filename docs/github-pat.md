# GitHub PAT 权限设置教程

本文档说明 CFST-GUI 在“GitHub 结果导出”中应该如何创建和配置 Personal Access Token。普通用户只需要给目标仓库最小 Contents 权限；发布 Release、触发 Actions 或推送 GHCR 镜像是维护者流程，不应把这些额外权限授给日常测速结果导出 Token。

以下步骤按 GitHub 当前文档整理，界面入口截至 2026-06-03 仍是 `Settings` -> `Developer settings` -> `Personal access tokens` -> `Fine-grained tokens`。如果 GitHub 页面文案有细微变化，以官方文档为准。

## 权限结论

推荐使用 fine-grained PAT，并限制到一个目标仓库：

| 配置项 | 推荐值 |
| --- | --- |
| Token 类型 | Fine-grained personal access token |
| Resource owner | 拥有目标仓库的个人账号或组织 |
| Repository access | Only select repositories |
| 选中的仓库 | 只选 CFST-GUI 要写入结果的仓库 |
| Repository permissions: Contents | Read and write |
| Repository permissions: Metadata | Read-only，GitHub 自动授予 |
| 其他 Repository / Account permissions | No access |

CFST-GUI 的 GitHub 导出使用 GitHub Contents API。导出时会先读取目标文件的 SHA，再创建或更新文件，所以需要 Contents 读写权限；只有写权限不足以覆盖已有文件。

不需要授予这些权限：

- Actions
- Administration
- Issues
- Pull requests
- Packages
- Secrets
- Webhooks
- Workflows，除非你故意把导出路径配置到 `.github/workflows/`，不建议这么做

## 创建 fine-grained PAT

1. 打开 GitHub，点击右上角头像，进入 `Settings`。
2. 左侧进入 `Developer settings`。
3. 进入 `Personal access tokens` -> `Fine-grained tokens`。
4. 点击 `Generate new token`。
5. 填写 `Token name`，例如 `CFST-GUI results export`。
6. 设置 `Expiration`。建议选择 30 到 90 天，避免长期 Token 泄露后难以及时止损。
7. `Resource owner` 选择目标仓库所在的个人账号或组织。
8. `Repository access` 选择 `Only select repositories`，只勾选要写入测速结果的仓库。
9. 在 `Repository permissions` 中找到 `Contents`，设置为 `Read and write`。
10. 确认其他权限保持 `No access`，然后生成 Token。
11. 复制生成的 Token。GitHub 只会展示一次，关闭页面后无法再次查看。

如果目标仓库属于组织，组织可能要求管理员审批 fine-grained PAT。审批完成前，CFST-GUI 测试 GitHub 导出可能会返回 403。

## 在 CFST-GUI 中填写

进入设置页的“执行 / DNS / GitHub”区域，找到“GitHub 结果导出”，填写：

| 字段 | 示例 | 说明 |
| --- | --- | --- |
| Owner | `axuitomo` | 目标仓库 owner，可以是个人或组织。 |
| Repo | `CFST-GUI` | 目标仓库名。 |
| Branch | `main` | 要写入的目标分支，必须存在。 |
| PAT Token | `github_pat_...` | 粘贴完整 fine-grained PAT。 |
| 格式 | `CSV` 或 `TXT` | GitHub 导出的文件格式。 |
| 目标路径模板 | `cfst-results/{date}/{time}-{task_id}.csv` | 仓库内文件路径，不要以 `/` 开头。 |
| 提交信息模板 | `CFST results {date} {time}` | 写入文件时的 commit message。 |

保存配置后点击“测试 GitHub”。测试通过表示 owner、repo、branch 和 Contents 读取权限可用。正式导出可以在当前结果页点击 `GitHub`，也可以在定时任务中启用自动 GitHub 导出。

路径模板支持：

| 占位符 | 含义 |
| --- | --- |
| `{date}` | 当前日期，格式 `YYYY-MM-DD`。 |
| `{time}` | 当前时间，格式 `HHMMSS`。 |
| `{task_id}` / `{taskId}` | 当前任务 ID，已做路径安全清理。 |
| `{timestamp}` | 当前时间戳，格式 `YYYYMMDD-HHMMSS`。 |

默认路径会写到 `cfst-results/` 目录下。GitHub Contents API 会在提交时自动创建不存在的中间目录。

## 常见报错

| 现象 | 可能原因 | 处理方式 |
| --- | --- | --- |
| `缺少完整 GitHub PAT` | Token 为空，或只保存了掩码占位符 | 重新粘贴完整 Token 并保存。 |
| `401 Unauthorized` | Token 错误、过期、被吊销或复制不完整 | 重新生成 fine-grained PAT。 |
| `403 Forbidden` | 缺少 Contents Read and write，组织 Token 未审批，或组织 SSO 未授权 | 检查权限、组织审批和 SSO 授权状态。 |
| `404 Not Found` | owner、repo、branch 写错，或 Token 没有访问该仓库 | 校对仓库信息，确认 Token 选中了目标仓库。 |
| `409 Conflict` | 同一路径被并发写入，或分支状态冲突 | 避免多个任务同时写同一个路径，给路径模板加 `{time}` 或 `{task_id}`。 |
| 分支保护拒绝写入 | 目标分支要求 PR、签名提交或状态检查 | 换一个允许机器人直接写入的分支，或调整仓库分支保护策略。 |

如果只测试通过但正式导出失败，优先检查目标路径模板。测试主要验证仓库、分支和读取权限；正式导出还会写入具体文件路径。

## 安全建议

- 使用 fine-grained PAT，不使用 classic PAT。
- 只选择一个目标仓库，不选择 `All repositories`。
- 只授予 `Contents: Read and write`，其他权限保持 `No access`。
- 给 Token 设置过期时间，并定期轮换。
- 不要把 Token 提交到 Git 仓库、Issue、聊天记录或截图中。
- 配置导出、配置归档、WebDAV 备份和本地备份文件可能包含完整 Token，只保存到可信位置。
- 如果怀疑泄露，立即在 GitHub `Developer settings` 中 Revoke Token，然后在 CFST-GUI 中替换新 Token。

## classic PAT 兼容说明

不推荐使用 classic PAT。若必须使用 classic PAT，请按 GitHub Contents API 文档为目标仓库写入文件准备 `repo` scope。classic PAT 的权限粒度较粗，容易把不相关仓库也暴露给 Token，因此仅作为兼容选项。

如果导出路径落到 `.github/workflows/`，GitHub 还会要求 workflow 相关权限。CFST-GUI 的默认路径不是工作流目录，也不建议把测速结果写入该目录。

## 维护者发布权限

本节只面向项目维护者，和普通用户的测速结果导出无关。

| 场景 | 推荐凭据 | 权限要点 |
| --- | --- | --- |
| GitHub Release 发布 | GitHub Actions `GITHUB_TOKEN` | workflow 中需要 `contents: write` 才能创建 Release 和上传资产。 |
| 手动触发或管理 Actions | GitHub Actions `GITHUB_TOKEN` 或维护者 PAT | 只有调用 Actions API 或 workflow dispatch 时才需要 Actions / workflow 相关权限。 |
| GHCR 镜像发布 | GitHub Actions `GITHUB_TOKEN` | `.github/workflows/container.yml` 使用 `packages: write` 发布镜像。 |
| Android / Windows 签名 | GitHub Actions Secrets | 使用签名证书和 keystore Secret，不应放进 GitHub PAT。 |

日常 CFST-GUI 结果导出不需要 Actions、Packages、Release 或仓库管理权限。

## 参考

- [GitHub: Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [GitHub REST API: Repository contents](https://docs.github.com/rest/repos/contents)
- [GitHub: Permissions required for fine-grained PATs](https://docs.github.com/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)
- [配置详解](./configuration.md)
- [GitHub 导出实现](../internal/githubcore/github.go)
