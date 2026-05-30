# Neptune

Telegram 群管理 Bot，基于 Cloudflare Workers 构建。

## 功能

- 入群欢迎 - 自定义欢迎消息，支持 Markdown 和占位符
- 入群认证 - BMP 图片验证码验证，防止机器人入群（支持复用，超过 10 次自动轮换）
- 自动回复 - 关键词/正则匹配自动回复（支持繁简中文互匹配）
- AI 聊天 - 搭载涅普顿人格的 AI 对话，支持上下文记忆
- 投票踢人 - 群成员投票踢人，支持启用/禁用，过期自动清理
- 管理员私聊管理 - 通过私聊管理群组设置，支持多群组切换
- GitHub Release 通知 - 自动接收 Kazumi 仓库新 Release 并发送到 Telegram 频道
- 权限控制 - 所有管理命令仅限管理员使用

## 技术栈

- 运行时：Cloudflare Workers
- 数据库：Cloudflare D1 (SQLite)
- 对象存储：Cloudflare R2 (存储验证码图片)
- KV 存储：Cloudflare KV (AI 聊天上下文)
- Bot 框架：Grammy
- 语言：TypeScript
- 包管理：Bun
- Linter：Biome

## 部署

前置条件：
- Cloudflare 账号
- Telegram Bot Token（通过 @BotFather 获取）
- 小米 MiMo API Key（从 [platform.xiaomimimo.com](https://platform.xiaomimimo.com) 获取）
- 安装 Bun

步骤：

1. 克隆项目
   ```bash
   git clone https://github.com/ChouChiu/Neptune.git
   cd neptune
   bun install
   ```

2. 创建配置文件
   ```bash
   cp wrangler.example.toml wrangler.toml
   ```

3. 创建 D1 数据库
   ```bash
   wrangler d1 create neptune
   ```
   将输出的 `database_id` 填入 `wrangler.toml`

4. 创建 R2 Bucket
   ```bash
   wrangler r2 bucket create neptune-captcha
   ```

5. 创建 KV 命名空间（用于 AI 聊天上下文）
   ```bash
   wrangler kv namespace create aiContext
   ```
   将输出的 `id` 填入 `wrangler.toml`

6. 初始化数据库
   ```bash
   wrangler d1 execute neptune --remote --file=src/db/schema.sql
   wrangler d1 execute neptune --remote --file=migrations/003_ai_chat_usage.sql
   ```

7. 配置 Secrets
   ```bash
   echo "YOUR_BOT_TOKEN" | wrangler secret put BOT_TOKEN
   echo "YOUR_MIMO_API_KEY" | wrangler secret put MIMO_API_KEY
   echo "YOUR_GITHUB_WEBHOOK_SECRET" | wrangler secret put GITHUB_WEBHOOK_SECRET
   ```

8. 部署
   ```bash
   bun run deploy
   ```

9. 设置 Telegram Webhook
   访问 `https://<your-worker-url>/set-webhook`

10. 配置 GitHub Webhook（可选，用于 Release 通知）
    - 在 `wrangler.toml` 中设置 `RELEASE_CHANNEL_ID`（Telegram 频道 ID）
    - 在 GitHub 仓库 Settings → Webhooks 添加：
      - URL: `https://<your-worker-url>/github-webhook`
      - Content type: `application/json`
      - Secret: 与步骤 7 中 `GITHUB_WEBHOOK_SECRET` 相同
      - Events: 仅勾选 **Release**

## 命令列表

通用
| 命令 | 说明 |
|------|------|
| `/help` | 显示帮助信息 |
| `/ping` | 检查机器人是否在线 |
| `/id` | 获取当前群组 ID（自动关联群组） |

管理员（私聊）
| 命令 | 说明 |
|------|------|
| `/connect <群组ID>` | 绑定私聊与群组（需群组管理员权限） |
| `/switch` | 切换管理的群组（按钮选择） |

入群欢迎（需要管理员权限）
| 命令 | 说明 |
|------|------|
| `/setwelcome <消息>` | 设置欢迎消息 |
| `/enablewelcome` | 启用入群欢迎 |
| `/disablewelcome` | 禁用入群欢迎 |

入群认证（需要管理员权限）
| 命令 | 说明 |
|------|------|
| `/setverifybutton <文案>` | 设置认证按钮文案 |
| `/setverifytimeout <秒>` | 设置认证超时时间 |
| `/testverify` | 测试验证消息（群组中使用） |

自动回复（需要管理员权限）
| 命令 | 说明 |
|------|------|
| `/addkeyword <关键词> <回复>` | 添加关键词规则 |
| `/addregex <正则> <回复>` | 添加正则规则 |
| `/listkeywords` | 列出所有规则 |
| `/removekeyword <关键词>` | 删除规则 |

> 关键词和正则均支持繁简中文自动匹配，例如关键词为"学习"可匹配"學習"，反之亦然。

投票踢人（`/enablevotekick` `/disablevotekick` 需要管理员权限）
| 命令 | 说明 |
|------|------|
| `/enablevotekick` | 启用投票踢人 |
| `/disablevotekick` | 禁用投票踢人 |
| `/kick` | 回复目标用户消息发起踢人投票（群成员均可使用） |

AI 聊天
| 触发方式 | 说明 |
|----------|------|
| @机器人 + 消息 | 直接与涅普顿对话 |
| 回复机器人消息 | 继续对话 |

- 每日限额：普通用户 15 次/天，管理员不限
- 记忆机制：群内共享上下文，滚动保留最近 7 天对话（第 7 天删除第 1 天，第 8 天删除第 2 天，以此类推）
- 模型：小米 MiMo V2.5

## 占位符

欢迎消息和自动回复支持以下占位符：

| 占位符 | 说明 |
|--------|------|
| `{nickname}` | 用户昵称 |
| `{userid}` | 用户 ID |
| `{groupname}` | 群组名称 |

## 使用流程

1. 将 Bot 加入群组并设为管理员
2. 在群组中发送 `/id` 获取群组 ID（自动关联你的账号）
3. 在私聊中发送 `/switch` 选择要管理的群组
4. 使用各种命令配置群组功能

## 本地开发

```bash
bun run dev           # 启动本地开发服务器
bun run lint          # 运行 lint 检查
bun run lint:fix      # 自动修复 lint 问题
bun run typecheck     # 类型检查
```

## License

MIT
