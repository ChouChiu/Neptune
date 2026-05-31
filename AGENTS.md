# AGENTS.md

Telegram group management bot (Neptune), deployed on Cloudflare Workers.

## Commands

```bash
bun install              # install deps
bun run lint             # Biome check (NOT ESLint)
bun run lint:fix         # auto-fix
bun run typecheck        # tsc --noEmit
bun run dev              # wrangler dev (local)
bun run deploy           # wrangler deploy
```

No test suite exists. Verify changes with `lint` + `typecheck` only.

## Key constraints

- Package manager is **Bun** (no npm/yarn lockfiles).
- Linter/formatter is **Biome** (not ESLint/Prettier). Uses **tabs** and **double quotes**.
- `wrangler.toml` is **gitignored** — `wrangler.example.toml` is the template. D1 database_id, R2 bucket_name, and KV id must be filled in.
- Secrets via `wrangler secret put`: `BOT_TOKEN`, `MIMO_API_KEY`, `GITHUB_WEBHOOK_SECRET`.
- Wrangler doesn't support Bun runtime directly; use `bunx wrangler` (macOS 13.5+) or `--remote` flag.
- Deploy requires running `/set-webhook?token=<GITHUB_WEBHOOK_SECRET>` on the worker URL to register the Telegram webhook and sync BotFather command list.
- New commands must be added to the `setMyCommands` array in `src/index.ts`.
- `/test` endpoint requires `Authorization: Bearer <GITHUB_WEBHOOK_SECRET>` header.

## Bindings vs Env interface

`wrangler.toml` bindings and `src/types.ts` Env must stay in sync:

| wrangler.toml binding | Env field      | Type           |
|-----------------------|---------------|----------------|
| `db`                  | `env.db`      | `D1Database`   |
| `captcha`             | `env.captcha` | `R2Bucket`     |
| `aiContext`           | `env.aiContext` | `KVNamespace` |

Optional env vars: `REUSE_CAPTCHA` (`"true"` to enable captcha reuse up to 10 times), `RELEASE_CHANNEL_ID` (Telegram channel ID for GitHub release notifications).

## Database

- Schema: `src/shared/db/schema.sql` — apply with `wrangler d1 execute neptune --remote --file=src/shared/db/schema.sql`
- Queries: `src/shared/db/queries.ts` — all DB access goes through this file
- Tables: `groups`, `keywords`, `admin_connections`, `admin_current_group`, `pending_verifications`, `active_votes`, `vote_records`, `ai_chat_usage`
- Migrations in `migrations/` — apply manually with `wrangler d1 execute <name> --remote --file=migrations/<file>.sql` (no auto-migration on deploy)

## Architecture

```
src/
├── index.ts              # Workers fetch handler: /webhook, /set-webhook, /test, /github-webhook
├── bot.ts                # createBot(env) — calls registerFeatures()
├── types.ts              # Env interface + data models
├── features/
│   ├── index.ts          # registerFeatures() — central wiring
│   ├── message-orchestrator.ts  # dispatches DM→captcha, group→AI→keywords
│   ├── admin/            # /id, /connect, /switch
│   ├── help/             # /help
│   ├── ping/             # /ping
│   ├── rule/             # /rule
│   ├── welcome/          # /setwelcome, join event handler
│   ├── verify/           # /setverifybutton, /testverify, /start verify, captcha
│   ├── keywords/         # /addkeyword, keyword matching handler
│   ├── ai-chat/          # AI chat (MiMo API, skills, context)
│   ├── votekick/         # /kick, vote callback handler
│   └── github-release/   # GitHub webhook → Telegram
├── shared/
│   ├── db/               # schema.sql + queries.ts
│   └── utils/            # botInfo, captcha, markdown, nickname, permissions, placeholders, reply, resolve-group
└── migrations/           # D1 migration files
```

Entry point is `src/index.ts` (`main` in wrangler.toml). Bot instance created per-request via `createBot(env)`.

Each feature folder contains:
- `commands.ts` — slash command handlers
- `handlers.ts` — event/callback handlers
- `index.ts` — registers the feature (`registerXxxFeature(bot, db, ...)`)
- Internal utils (e.g. `vote.ts`, `skills.ts`) when feature-specific

## AI chat feature

- Triggered by @mention of the bot or replying to the bot's messages in groups.
- Uses Xiaomi MiMo V2.5 API (`https://token-plan-sgp.xiaomimimo.com/v1`).
- Context stored in KV (`aiContext`) as `ai:context:{groupId}`, limited to 50 messages (pruned by timestamp and count). KV TTL is 8 days as safety net.
- Daily usage tracked in D1 `ai_chat_usage` table (15/day per user, admins exempt).
- System prompt (Neptune persona) is in `src/features/ai-chat/system-prompt.json`, rendered by `systemPromptToText()`.
- API call has 25s timeout via AbortController, retries up to 2 times on 429/5xx errors.
- `shouldTriggerAi()` in `src/features/ai-chat/ai-chat.ts` filters out replies to system messages using a keyword list — update this list if new bot message types are added.

## GitHub Release webhook

- Endpoint: `POST /github-webhook` in `src/index.ts`
- Receives GitHub `release` events, verifies `X-Hub-Signature-256` (HMAC-SHA256), sends formatted release note to Telegram channel
- Only processes `action: "published"` (ignores `created`, `edited`, `prereleased`, `draft`)
- GFM → MarkdownV2 conversion in `src/features/github-release/github-release.ts`: code blocks protected, links preserved, headings→bold, list markers (`- * +`)→`⦁`, GitHub callouts (`[!NOTE]` etc.) stripped, blockquotes→Telegram `>text`
- `!` is reserved in Telegram MarkdownV2 — callout markers like `[!Note]` must be stripped before escaping, otherwise send fails with 400
- GitHub webhook payload uses `\r\n` line endings — normalized to `\n` before regex processing

## Permission model

- Admin commands check `checkAdminPermission()` in `src/shared/utils/permissions.ts`
- In groups: checks Telegram `administrator`/`creator` status via `getChatMember`
- In private chat: checks `admin_connections` table (bound via `/connect`)
- `/id` in a group auto-connects the user as admin
- `/connect <groupId>` in private chat verifies Telegram admin status before binding

## Keyword matching

- Uses `chinese-conv` package (`sify()`) for traditional/simplified Chinese normalization — both keyword and regex patterns match across traditional ↔ simplified automatically
- Regex patterns are compiled once and cached in `keywordCache` (60s TTL) as `RegExp` objects
- `/addregex` validates regex length (≤200 chars) and runs a complexity test (100ms threshold) to prevent ReDoS

## Conventions

- Reply helpers in `src/shared/utils/reply.ts` — `replyOptions(ctx)` and `replyOptionsWithParse(ctx)` (Markdown mode)
- `escapeMarkdown()` escapes all Telegram MarkdownV2 special chars
- Placeholder replacement: `{nickname}`, `{userid}`, `{groupname}` in `src/shared/utils/placeholders.ts`
- `getNickname(user)` in `src/shared/utils/nickname.ts` — use this instead of inline `first_name + last_name` concatenation
- `buildVoteText()` and `VOTE_THRESHOLD` in `src/features/votekick/vote.ts` — shared by votekick command and handler
- Captcha images stored as BMP in R2 at key `captcha/{groupId}/{userId}.bmp`, 5 attempts max before lockout
- `/rule` command sets group rules shown during verification flow (10s reading time enforced via `rule_ack` callback)
- Dependencies: `grammy` (bot framework), `chinese-conv` (繁简转换)
