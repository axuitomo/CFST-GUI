# Cloudflare API Token 权限设置教程

本文档说明 CFST-GUI 的 Cloudflare DNS 推送功能应该如何创建和配置 Cloudflare API Token。普通使用场景只需要把 Token 限制到一个 Zone，并授予 DNS 编辑权限；不要使用 Global API Key，也不要给账号级管理权限。

以下步骤按 Cloudflare 官方文档整理，界面入口截至 2026-06-03 仍是 Cloudflare Dashboard 的 `My Profile` -> `API Tokens`。如果 Cloudflare 页面文案有细微变化，以官方文档为准。

## 权限结论

推荐使用 Cloudflare API Token，并限制到单个 Zone：

| 配置项 | 推荐值 |
| --- | --- |
| Token 类型 | User API Token |
| 模板 | `Edit zone DNS`，或自定义 Token |
| Permissions | `Zone` -> `DNS` -> `Edit` |
| Zone Resources | `Include` -> `Specific zone` -> 选择要推送的域名 |
| Client IP Address Filtering | 可选，固定出口 IP 时再开启 |
| TTL / Expiration | 建议设置过期时间并定期轮换 |
| 其他 Account / Zone / User 权限 | 不授予 |

CFST-GUI 会读取目标记录名下的 A/AAAA 记录，并在覆盖推送时创建、更新或删除 DNS 记录，所以需要 `Zone - DNS - Edit`。Cloudflare 官方 DNS Records API 中，读取记录接受 `DNS Read` 或 `DNS Write`，创建、更新和删除记录都要求 `DNS Write`；因此只给 `DNS Read` 只能读取，不能推送。

不需要授予这些权限：

- Account Settings
- Account Rulesets
- Analytics
- Billing
- Cache
- Page Rules
- Workers
- Zone Settings
- SSL and Certificates

## 创建 API Token

1. 打开 Cloudflare Dashboard。
2. 点击右上角头像，进入 `My Profile`。
3. 左侧进入 `API Tokens`。
4. 点击 `Create Token`。
5. 推荐选择 `Edit zone DNS` 模板；如果使用自定义 Token，权限手动设置为 `Zone` -> `DNS` -> `Edit`。
6. 在 `Zone Resources` 中选择 `Include` -> `Specific zone`，只选 CFST-GUI 要覆盖推送的域名。
7. 可选：如果运行 CFST-GUI 的服务器出口 IP 固定，可以在 `Client IP Address Filtering` 中限制来源 IP。
8. 可选：设置 Token TTL 或过期时间，建议不要长期无限期使用。
9. 点击 `Continue to summary`，确认只有目标 Zone 的 DNS Edit 权限。
10. 点击 `Create Token` 并复制 Token。Cloudflare 只展示一次，关闭页面后无法再次查看。

创建完成页会给出一个 `/user/tokens/verify` 的 `curl` 示例，可以先在终端验证 Token 是否有效：

```bash
curl "https://api.cloudflare.com/client/v4/user/tokens/verify" \
  --header "Authorization: Bearer <API_TOKEN>"
```

返回 `success: true` 且状态为 `active` 表示 Token 本身有效。这个验证只说明 Token 存在且未过期，不代表 DNS Zone 权限一定正确。

## 获取 Zone ID

CFST-GUI 需要填写 Cloudflare `Zone ID`。获取方式：

1. 在 Cloudflare Dashboard 进入目标域名。
2. 打开域名首页或右侧信息栏。
3. 找到 `Zone ID` 并复制。

`Zone ID` 是 32 位左右的十六进制字符串，不是域名本身，也不是 Account ID。Token 的 Zone Resources 必须包含这个 Zone，否则读取或推送会返回权限错误。

## 在 CFST-GUI 中填写

进入设置页的“Cloudflare 配置”区域，填写：

| 字段 | 示例 | 说明 |
| --- | --- | --- |
| API Token | `cfut_...` 或 Cloudflare 生成的 Token | 粘贴完整 API Token。 |
| Zone ID | `023e105f4ecef8ad9ca31a8372d0c353` | 目标域名的 Zone ID。 |
| 记录名称 | `edge.example.com` | 要覆盖推送的完整 DNS 记录名。 |
| TTL | `60`、`300` 或 `600` | CFST-GUI 当前支持 1、5、10 分钟三档。 |
| 备注 | `CFST-GUI auto update` | 可选，会写入 DNS record comment。 |

DNS 面板可以点击“读取记录”查看当前配置下匹配记录名的 A/AAAA 记录，也可以粘贴 IP 后点击“推送到 DNS”。从当前结果推送或定时任务自动 DNS 推送会复用同一套 Cloudflare 配置。

CFST-GUI 当前行为：

- IPv4 自动写入 A 记录。
- IPv6 自动写入 AAAA 记录。
- 覆盖推送会让目标记录名下的 A/AAAA 记录与本次输入 IP 对齐，可能创建、更新和删除记录。
- DNS 上传固定使用灰色解析，后端会把 `proxied` 写为 `false`。
- 当工作流或分流规则覆盖 `record_name`、`record_type`、`ttl`、`comment` 时，会基于同一个 API Token 和 Zone ID 执行。

## 常见报错

| 现象 | 可能原因 | 处理方式 |
| --- | --- | --- |
| `缺少完整 Cloudflare API Token` | Token 为空，或只保存了掩码占位符 | 重新粘贴完整 Token 并保存。 |
| `缺少 Cloudflare Zone ID` | 未填写 Zone ID，或误填了域名 / Account ID | 到 Cloudflare 域名页面复制 Zone ID。 |
| `缺少 Cloudflare DNS 记录名称` | 未填写记录名称 | 填写完整记录名，例如 `edge.example.com`。 |
| `403 Forbidden` | Token 没有目标 Zone 的 DNS Edit 权限，或 Zone Resources 选错 | 确认权限是 `Zone - DNS - Edit`，资源包含目标 Zone。 |
| `7003` / `7000` 类 Zone 错误 | Zone ID 错误或 Token 无法访问该 Zone | 重新复制 Zone ID，确认 Token 绑定的是同一个域名。 |
| `81057` 或记录冲突 | 同名记录类型冲突，例如 A/AAAA 与 CNAME 冲突 | 删除或改名冲突的 CNAME/NS 等记录后再推送。 |
| 推送后橙云变灰云 | CFST-GUI 固定写入 `proxied=false` | 这是当前设计，测速结果 DNS 记录按直连记录管理。 |
| 读取成功但推送失败 | Token 可能只有 DNS Read，或分支场景需要删除/更新旧记录 | 改为 `Zone - DNS - Edit`。 |

推送会真实修改线上 DNS 记录。首次配置时建议先点击“读取记录”，确认记录名和 Zone ID 指向预期域名，再用少量 IP 做一次手动推送。

## 安全建议

- 使用 API Token，不使用 Global API Key。
- 只选择一个 Specific zone，不选择 All zones。
- 只授予 `Zone - DNS - Edit`，不要授予账号级或全局权限。
- 给 Token 设置过期时间，并定期轮换。
- 不要把 Token 提交到 Git 仓库、Issue、聊天记录或截图中。
- 配置导出、配置归档、WebDAV 备份和本地备份文件可能包含完整 Cloudflare API Token，只保存到可信位置。
- 如果怀疑泄露，立即在 Cloudflare `My Profile` -> `API Tokens` 中 Revoke Token，然后在 CFST-GUI 中替换新 Token。

## Global API Key 说明

不推荐使用 Global API Key。Global API Key 绑定账号级能力，权限面过大；CFST-GUI 的实现使用 `Authorization: Bearer <token>` 调用 Cloudflare API，设计目标是 API Token，而不是 `X-Auth-Email` + `X-Auth-Key` 的旧式认证方式。

## 参考

- [Cloudflare: Create API token](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/)
- [Cloudflare API: List DNS Records](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/list/)
- [Cloudflare API: Create DNS Record](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/)
- [Cloudflare API: Update DNS Record](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/edit/)
- [Cloudflare API: Delete DNS Record](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/delete/)
- [配置详解](./configuration.md)
- [Cloudflare DNS 实现](../internal/cloudflarecore/dns.go)
