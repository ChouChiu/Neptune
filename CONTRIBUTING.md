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
bunx wrangler d1 execute neptune --remote --file=src/shared/db/schema.sql
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
├── bot.ts                # createBot(env) —— 调用 registerFeatures()
├── types.ts              # Env 接口 + 数据模型定义
├── features/             # 按功能模块组织
│   ├── index.ts          # registerFeatures() 统一注册所有功能
│   ├── message-orchestrator.ts  # 消息分发：DM→验证码, 群组→AI→关键词
│   ├── admin/            # /id, /connect, /switch
│   ├── help/             # /help
│   ├── ping/             # /ping
│   ├── rule/             # /rule（群规管理）
│   ├── welcome/          # /setwelcome, 入群事件处理
│   ├── verify/           # /setverifybutton, /testverify, /start verify, 验证码流程
│   ├── keywords/         # /addkeyword, 关键词匹配处理
│   ├── ai-chat/          # AI 聊天（MiMo API, skills, 上下文管理）
│   ├── votekick/         # /kick, 投票回调处理
│   └── github-release/   # GitHub webhook → Telegram
├── shared/               # 跨功能共享代码
│   ├── db/               # schema.sql + queries.ts（唯一 DB 访问入口）
│   └── utils/            # botInfo, captcha, markdown, nickname, permissions, placeholders, reply, resolve-group
└── migrations/           # 数据库迁移文件
```

每个功能文件夹包含：
- `commands.ts` — 斜杠命令处理
- `handlers.ts` — 事件/回调处理
- `index.ts` — 注册该功能（`registerXxxFeature(bot, db, ...)`）
- 内部工具（如 `vote.ts`, `skills.ts`）仅当功能特有时

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

1. 在 `src/features/` 下对应功能文件夹中创建或编辑 `commands.ts`：

```typescript
import type { Bot } from "grammy";
import type { D1Database } from "@cloudflare/workers-types";
import { replyOptions } from "../../shared/utils/reply";

export function registerMyCommands(bot: Bot, db: D1Database): void {
	bot.command("mycommand", async (ctx) => {
		// 命令逻辑
		await ctx.reply("Hello!", replyOptions(ctx));
	});
}
```

2. 在功能文件夹的 `index.ts` 中注册：

```typescript
import type { Bot } from "grammy";
import { registerMyCommands } from "./commands";

export function registerMyFeature(bot: Bot, db: D1Database): void {
	registerMyCommands(bot, db);
}
```

3. 在 `src/features/index.ts` 中添加 `registerMyFeature` 调用。
4. 在 `src/index.ts` 的 `setMyCommands` 中添加命令描述（用于 `/set-webhook` 同步到 BotFather）。

> 重要：`/set-webhook` 需要 `?token=<GITHUB_WEBHOOK_SECRET>` 认证，新命令添加后需重新访问该 URL 同步。

### 添加管理员命令

使用 `checkAdminPermission()` 检查权限：

```typescript
import { checkAdminPermission } from "../../shared/utils/permissions";

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
2. 编写 SQL（参考 `src/shared/db/schema.sql` 中的表结构）
3. 在 `src/shared/db/queries.ts` 中添加对应的查询函数
4. 部署后手动执行迁移：

```bash
bunx wrangler d1 execute neptune --remote --file=migrations/NNN_description.sql
```

> 注意：没有自动迁移机制，所有迁移必须手动执行。

### AI 聊天功能

- 触发方式：@机器人 或回复机器人消息
- 系统提示词（Neptune 人格）在 `src/features/ai-chat/system-prompt.json`，由 `systemPromptToText()` 渲染
- 上下文存储在 KV（`aiContext`），按 `ai:context:{groupId}` 键存储，限制最近 50 条消息（7 天窗口）
- API 调用有 25 秒超时（AbortController），429/5xx 错误自动重试最多 2 次
- 每日用量限制：普通用户 15 次/天，管理员不限（记录在 `ai_chat_usage` 表）
- `shouldTriggerAi()` 中有过滤系统消息的关键词列表，新增 bot 消息类型时需同步更新

### 关键词匹配

- 使用 `chinese-conv`（`sify()`）自动处理繁简中文互匹配
- 正则规则在 `keywordCache`（60s TTL）中编译为 `RegExp` 对象缓存
- `/addregex` 会校验正则长度（≤200 字符）和复杂度（100ms 阈值），防止 ReDoS
- 匹配逻辑在 `src/features/keywords/handlers.ts` 的 `matchKeyword()` 函数

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
- 如涉及新 bot 消息类型，请检查 `shouldTriggerAi()` 的系统消息关键词列表是否需要更新（`src/features/ai-chat/ai-chat.ts`）

## License

提交代码即表示你同意你的贡献以 [MIT License](LICENSE) 发布。
