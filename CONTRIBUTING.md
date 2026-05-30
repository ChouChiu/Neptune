# Contributing to Neptune

感谢你对 Neptune 项目的关注！我们欢迎任何形式的贡献。

## 目录

- [行为准则](#行为准则)
- [如何贡献](#如何贡献)
- [开发环境](#开发环境)
- [项目架构](#项目架构)
- [开发指南](#开发指南)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [Pull Request 指南](#pull-request-指南)
- [License](#license)

## 行为准则

本项目采用 [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md)，参与本项目即表示你同意遵守其中的条款。

## 如何贡献

### 报告 Bug

1. 在 GitHub Issues 中搜索是否已有相同问题
2. 如果没有，创建一个新的 Issue，包含：
   - 清晰的标题和描述
   - 复现步骤
   - 期望行为与实际行为
   - 运行环境信息（OS、Bun 版本等）

### 提交功能建议

1. 在 GitHub Issues 中创建一个新的 Feature Request
2. 说明使用场景和期望的实现方式

### 提交代码

1. Fork 本仓库
2. 创建你的特性分支：`git checkout -b feature/my-feature`
3. 提交你的修改：`git commit -m 'feat: add my feature'`
4. 推送到分支：`git push origin feature/my-feature`
5. 创建一个 Pull Request

## 开发环境

### 前置条件

- [Bun](https://bun.sh)（包管理 & 运行时）
- [Wrangler CLI](https://developers.cloudflare.com/workers/wrangler/)（`bunx wrangler`）
- Cloudflare 账号

### 初始化

```bash
# 克隆项目
git clone https://github.com/ChouChiu/Neptune.git
cd neptune

# 安装依赖
bun install

# 复制配置文件
cp wrangler.example.toml wrangler.toml
```

### 创建 Cloudflare 资源

```bash
# 创建 D1 数据库，将输出的 database_id 填入 wrangler.toml
bunx wrangler d1 create neptune

# 创建 R2 Bucket（存储验证码图片）
bunx wrangler r2 bucket create neptune-captcha

# 创建 KV 命名空间（AI 聊天上下文），将输出的 id 填入 wrangler.toml
bunx wrangler kv namespace create aiContext
```

### 初始化数据库

```bash
bunx wrangler d1 execute neptune --remote --file=src/db/schema.sql
bunx wrangler d1 execute neptune --remote --file=migrations/003_ai_chat_usage.sql
bunx wrangler d1 execute neptune --remote --file=migrations/004_rule.sql
bunx wrangler d1 execute neptune --remote --file=migrations/005_captcha_attempts.sql
bunx wrangler d1 execute neptune --remote --file=migrations/006_indexes.sql
```

### 配置 Secrets

Secrets 通过 Wrangler 管理，**不要**写入 `wrangler.toml`：

```bash
echo "YOUR_BOT_TOKEN" | bunx wrangler secret put BOT_TOKEN
echo "YOUR_MIMO_API_KEY" | bunx wrangler secret put MIMO_API_KEY
echo "YOUR_GITHUB_WEBHOOK_SECRET" | bunx wrangler secret put GITHUB_WEBHOOK_SECRET
```

### 本地开发

```bash
bun run dev           # 启动本地开发服务器（wrangler dev）
bun run lint          # Biome 代码检查
bun run lint:fix      # 自动修复 lint 问题
bun run typecheck     # TypeScript 类型检查
```

本地测试 Telegram Webhook 可使用 [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) 或 [ngrok](https://ngrok.com/) 将本地服务暴露到公网。

## 项目架构

```
src/
├── index.ts              # Workers 入口：路由 /webhook, /set-webhook, /test, /github-webhook
├── bot.ts                # createBot(env) —— 注册所有命令和处理器
├── types.ts              # Env 接口 + 数据模型定义
├── commands/             # Bot 命令实现
│   ├── admin.ts          # /connect, /switch, /id（管理员绑定与切换）
│   ├── help.ts           # /help
│   ├── keywords.ts       # /addkeyword, /addregex, /listkeywords, /removekeyword
│   ├── ping.ts           # /ping
│   ├── rule.ts           # /rule（群规管理）
│   ├── verify.ts         # /setverifybutton, /setverifytimeout, /testverify
│   ├── votekick.ts       # /enablevotekick, /disablevotekick, /kick
│   └── welcome.ts        # /setwelcome, /enablewelcome, /disablewelcome
├── handlers/             # 事件处理器
│   ├── chatMember.ts     # 入群事件 → 欢迎 + 群规 + 验证码流程
│   ├── message.ts        # 消息事件 → 关键词匹配 + AI 聊天 + 验证码回复
│   └── votekick.ts       # 投票踢人回调处理
├── db/
│   ├── schema.sql        # 数据库表结构
│   └── queries.ts        # 所有数据库查询（唯一 DB 访问入口）
└── utils/
    ├── ai-chat.ts        # AI 聊天（MiMo API + KV 上下文 + 用量限制 + 超时重试）
    ├── botInfo.ts        # Bot 信息缓存（带 inflight 去重）
    ├── captcha.ts        # BMP 验证码生成 & R2 存储
    ├── github-release.ts # GitHub Release → Telegram MarkdownV2 格式化
    ├── nickname.ts       # getNickname() 用户昵称拼接
    ├── permissions.ts    # 管理员权限检查
    ├── placeholders.ts   # {nickname}, {userid}, {groupname} 占位符替换
    ├── reply.ts          # replyOptions() + escapeMarkdown()
    ├── skills.ts         # AI 技能匹配（基于关键词触发）
    ├── system-prompt.json # Neptune 人格系统提示词
    └── vote.ts           # buildVoteText() + VOTE_THRESHOLD（投票踢人共享）
migrations/               # 数据库迁移文件
```

### 数据库表

| 表名 | 用途 |
|------|------|
| `groups` | 群组配置（欢迎、验证、群规、投票踢人开关等） |
| `keywords` | 关键词/正则自动回复规则 |
| `admin_connections` | 管理员与群组的绑定关系 |
| `admin_current_group` | 管理员当前选中的群组 |
| `pending_verifications` | 待验证用户（验证码、过期时间、尝试次数） |
| `active_votes` | 进行中的投票踢人 |
| `vote_records` | 投票记录 |
| `ai_chat_usage` | AI 聊天每日用量统计 |

## 开发指南

### 添加新命令

1. 在 `src/commands/` 下创建文件（或在已有文件中添加）：

```typescript
import type { Bot } from "grammy";
import type { D1Database } from "@cloudflare/workers-types";
import { replyOptions } from "../utils/reply";

export function registerMyCommands(bot: Bot, db: D1Database): void {
	bot.command("mycommand", async (ctx) => {
		// 命令逻辑
		await ctx.reply("Hello!", replyOptions(ctx));
	});
}
```

2. 在 `src/bot.ts` 中注册：

```typescript
import { registerMyCommands } from "./commands/mycommand";
// ...
registerMyCommands(bot, env.db);
```

3. 在 `src/index.ts` 的 `setMyCommands` 中添加命令描述（用于 `/set-webhook` 同步到 BotFather）。

> 重要：`/set-webhook` 需要 `?token=<GITHUB_WEBHOOK_SECRET>` 认证，新命令添加后需重新访问该 URL 同步。

### 添加管理员命令

使用 `checkAdminPermission()` 检查权限：

```typescript
import { checkAdminPermission } from "../utils/permissions";

bot.command("adminonly", async (ctx) => {
	const { allowed, groupId } = await checkAdminPermission(db, ctx);
	if (!allowed) {
		await ctx.reply("仅管理员可用", replyOptions(ctx));
		return;
	}
	// 使用 groupId 执行操作
});
```

### 数据库迁移

1. 在 `migrations/` 下创建迁移文件，命名格式：`NNN_description.sql`
2. 编写 SQL（参考 `src/db/schema.sql` 中的表结构）
3. 在 `src/db/queries.ts` 中添加对应的查询函数
4. 部署后手动执行迁移：

```bash
bunx wrangler d1 execute neptune --remote --file=migrations/NNN_description.sql
```

> 注意：没有自动迁移机制，所有迁移必须手动执行。

### AI 聊天功能

- 触发方式：@机器人 或回复机器人消息
- 系统提示词（Neptune 人格）在 `src/utils/system-prompt.json`，由 `systemPromptToText()` 渲染
- 上下文存储在 KV（`aiContext`），按 `ai:context:{groupId}` 键存储，限制最近 50 条消息（7 天窗口）
- API 调用有 25 秒超时（AbortController），429/5xx 错误自动重试最多 2 次
- 每日用量限制：普通用户 15 次/天，管理员不限（记录在 `ai_chat_usage` 表）
- `shouldTriggerAi()` 中有过滤系统消息的关键词列表，新增 bot 消息类型时需同步更新

### 关键词匹配

- 使用 `chinese-conv`（`sify()`）自动处理繁简中文互匹配
- 正则规则在 `keywordCache`（60s TTL）中编译为 `RegExp` 对象缓存
- `/addregex` 会校验正则长度（≤200 字符）和复杂度（100ms 阈值），防止 ReDoS

### 验证码与入群认证

- 验证码为 5 位 BMP 图片，存储在 R2 `captcha/{groupId}/{userId}.bmp`
- 暴力破解防护：`pending_verifications.attempts` 字段，5 次错误自动锁定
- 支持验证码复用（`REUSE_CAPTCHA=true`），同一验证码最多复用 10 次
- `/rule` 设置群规后，新成员入群需先阅读 10 秒才能开始验证

## 代码规范

- **Linter / Formatter**：Biome（不是 ESLint / Prettier）
- **缩进**：Tab
- **引号**：双引号
- **导入排序**：Biome 自动整理（`organizeImports: "on"`）

提交前请确保两项检查均通过：

```bash
bun run lint          # Biome 代码检查
bun run typecheck     # TypeScript 类型检查
```

## 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

| 前缀 | 说明 |
|------|------|
| `feat:` | 新功能 |
| `fix:` | Bug 修复 |
| `docs:` | 文档变更 |
| `style:` | 代码格式调整（不影响逻辑） |
| `refactor:` | 重构（既不修复 Bug 也不添加功能） |
| `perf:` | 性能优化 |
| `test:` | 测试相关 |
| `chore:` | 构建工具或辅助工具的变动 |

示例：

```
feat: 添加群组禁言命令
fix: 验证码超时后未自动踢出用户
docs: 更新 README 命令列表
```

## Pull Request 指南

- PR 标题使用 Conventional Commits 格式
- 一个 PR 只做一件事，保持精简
- 确保 `bun run lint` 和 `bun run typecheck` 通过
- 如涉及新功能，请在 PR 描述中说明使用方式
- 如涉及数据库变更，请在 `migrations/` 中添加迁移文件
- 如涉及新命令，请同步更新 README 的命令列表和 `src/index.ts` 中的 `setMyCommands`
- 如涉及新 bot 消息类型，请检查 `shouldTriggerAi()` 的系统消息关键词列表是否需要更新

## License

提交代码即表示你同意你的贡献以 [MIT License](LICENSE) 发布。
