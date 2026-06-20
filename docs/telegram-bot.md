# Telegram Bot 通知教程

本文档说明如何为 CFST-GUI 配置 Telegram Bot 通知。配置完成后，手动推送、测速后自动上传、定时任务自动上传和定时工作流自动上传会通过 Telegram 发送上传结论；也可以选择额外发送 Top N 结果列表。

## 功能范围

Telegram 通知当前用于上传相关结果，不替代本地 CSV、Cloudflare DNS 推送或 GitHub 导出。

| 通知类型 | 说明 |
| --- | --- |
| 上传结论 | 推送来源、整体状态、Cloudflare/GitHub 上传状态、上传条数、任务 ID 和时间。 |
| Top N 列表 | 可选；使用上传筛选后的结果，数量范围为 1 到 50，默认 5。 |
| 任务失败 | 上传链路失败时发送失败摘要，包含任务、阶段和原因。 |
| 测试通知 | 在设置页点击“测试 Telegram”后发送，用于确认 Token 和 Chat ID 可用。 |

Telegram 单条消息有长度限制，CFST-GUI 会在发送前截断过长内容。Bot Token 会随配置快照保存；导出配置、WebDAV 备份或本地备份时，请只保存到可信位置。

## 创建 Bot Token

1. 在 Telegram 中搜索并打开 `@BotFather`。
2. 发送 `/newbot`。
3. 按提示填写 Bot 显示名称。
4. 按提示填写 Bot 用户名，用户名必须以 `bot` 结尾，例如 `cfst_notify_bot`。
5. 复制 BotFather 返回的 Token，格式类似 `123456789:AA...`。
6. 打开新建 Bot 的聊天窗口，点击 `Start` 或发送 `/start`。如果要推送到个人聊天，这一步必须完成。

不要把 Bot Token 发到公开聊天、Issue、日志或截图里。怀疑泄露时，在 `@BotFather` 中使用 `/revoke` 重新生成 Token。

## 获取个人 Chat ID

如果只需要推送到个人聊天，可以用 Telegram 官方 Bot API 查询最近消息。

1. 用自己的 Telegram 账号打开 Bot 聊天窗口。
2. 给 Bot 发送任意消息，例如 `hello`。
3. 在浏览器打开下面的地址，把 `<TOKEN>` 替换成完整 Bot Token：

```text
https://api.telegram.org/bot<TOKEN>/getUpdates
```

4. 在返回 JSON 中找到 `message.chat.id`。个人 Chat ID 通常是正整数。

如果返回 `result: []`，通常是还没有给 Bot 发消息，或者这条消息已经被读取过。再给 Bot 发送一条新消息后刷新即可。

## 获取群组或频道 Chat ID

群组和频道 Chat ID 通常是负数；超级群组和频道常见格式为 `-100...`。

### 群组

1. 把 Bot 加入目标群组。
2. 在群组里发送一条消息，建议直接提及 Bot 或发送一条测试文本。
3. 打开：

```text
https://api.telegram.org/bot<TOKEN>/getUpdates
```

4. 在返回 JSON 中找到对应群组消息的 `message.chat.id`。

如果群组消息没有出现在 `getUpdates` 里，可能是 Bot 隐私模式限制。可以在 `@BotFather` 中进入 `/mybots`，选择该 Bot，进入 `Bot Settings` -> `Group Privacy`，关闭隐私模式后再发送一条新消息。

### 频道

1. 把 Bot 加入频道，并授予发送消息权限。
2. 在频道中发布一条新消息。
3. 打开：

```text
https://api.telegram.org/bot<TOKEN>/getUpdates
```

4. 在返回 JSON 中找到 `channel_post.chat.id`。

公开频道也可以尝试使用 `@channel_username` 作为目标，但推荐使用数字 Chat ID，避免频道改名后失效。

## 在 CFST-GUI 中填写

进入设置页，找到“通知配置”中的 “Telegram” 卡片，点击“展开”。

| 字段 | 示例 | 说明 |
| --- | --- | --- |
| Telegram 上传通知 | 开启 | 总开关；关闭时不会发送 Telegram 通知。 |
| Bot Token | `123456789:AA...` | 从 `@BotFather` 获取的完整 Token。 |
| 群组/频道 Chat ID | `-1001234567890` | 群组、超级群组或频道目标。 |
| 个人 Chat ID | `123456789` | 个人聊天目标。 |
| 上传目标模式 | `群组/频道`、`仅个人`、`个人+群组/频道` | 控制上传结论发到哪里。 |
| 推送 Top N 列表 | 按需开启 | 开启后会额外发送 Top N 列表。 |
| Top N 数量 | `5` | 可填 1 到 50。 |
| Top N 目标模式 | `群组/频道`、`仅个人`、`个人+群组/频道` | 控制 Top N 列表发到哪里，可和上传结论不同。 |

保存配置后点击“测试 Telegram”。测试通过时，目标聊天会收到类似下面的消息：

```text
CFST Telegram 通知测试
状态：Telegram 通知渠道可用。
用途：上传结论、Top N 列表
```

如果上传结论和 Top N 使用同一个目标，测试通知只会向该目标发送一条，并在“用途”中合并显示。

## 使用建议

- 只想给自己看结果：填写个人 Chat ID，上传目标模式选择“仅个人”。
- 想让团队看到上传结论：填写群组/频道 Chat ID，上传目标模式选择“群组/频道”。
- 想自己收到详细 Top N，但群组只看摘要：上传目标模式选择“群组/频道”，Top N 目标模式选择“仅个人”。
- 想同时发到个人和群组：两个 Chat ID 都填写，并选择“个人+群组/频道”。
- Top N 使用上传筛选后的结果；如果上传筛选条件过严，可能没有可发送的 Top N 列表。

Telegram 通知只会报告上传链路结果。若要实际写入 DNS 或导出文件，还需要分别配置并启用 Cloudflare、GitHub、测速后自动推送或定时任务。

## 常见报错

| 现象 | 可能原因 | 处理方式 |
| --- | --- | --- |
| `Telegram 通知配置不完整` | Bot Token 为空、复制不完整，或配置中仍是掩码占位符 | 重新粘贴完整 Bot Token 并保存。 |
| `Telegram 通知目标配置不完整` | 当前目标模式需要的 Chat ID 没有填写 | 按目标模式补齐个人或群组/频道 Chat ID。 |
| `401 Unauthorized` | Bot Token 错误、过期或已被撤销 | 到 `@BotFather` 重新生成 Token。 |
| `400 Bad Request: chat not found` | Chat ID 错误，Bot 未加入群组/频道，或个人未和 Bot 建立会话 | 校对 Chat ID；给 Bot 发送 `/start`；确认 Bot 已加入目标聊天。 |
| `403 Forbidden: bot was blocked by the user` | 个人用户屏蔽了 Bot | 在 Telegram 中解除屏蔽并重新发送 `/start`。 |
| `403 Forbidden: bot is not a member of the channel chat` | Bot 不在频道中，或没有发送权限 | 把 Bot 加入频道并授予发送消息权限。 |
| 测试通过但正式任务没通知 | Telegram 总开关关闭、上传链路没有执行、没有可上传结果，或任务没有进入上传阶段 | 检查“Telegram 上传通知”、Cloudflare/GitHub 上传配置、测速后自动推送或定时任务设置。 |
| Top N 没有发送 | 未开启“推送 Top N 列表”，结果为空，或 Top N 目标模式缺少对应 Chat ID | 开启 Top N，放宽上传筛选，补齐目标 Chat ID。 |

## 配置字段参考

配置快照中 Telegram 字段位于 `notifications.telegram`。常用字段如下：

```json
{
  "notifications": {
    "telegram": {
      "enabled": true,
      "bot_token": "123456789:AA...",
      "chat_id": "-1001234567890",
      "personal_chat_id": "123456789",
      "upload_recipient_mode": "chat",
      "include_top_n": true,
      "top_n": 5,
      "top_n_recipient_mode": "personal"
    }
  }
}
```

`upload_recipient_mode` 和 `top_n_recipient_mode` 支持：

| 值 | 含义 |
| --- | --- |
| `chat` | 群组/频道 Chat ID。 |
| `personal` | 个人 Chat ID。 |
| `both` | 同时发送到群组/频道和个人。 |

旧配置中的 `telegram`、`tg`、`botToken`、`chatId`、`recipient_mode` 等别名仍兼容读取；保存后会按当前字段结构写回。

## 参考

- [配置详解](./configuration.md)
- [上传工作流设计](./upload-workflow-design.md)
- [Telegram Bot API: sendMessage](https://core.telegram.org/bots/api#sendmessage)
- [Telegram BotFather](https://core.telegram.org/bots/features#botfather)
