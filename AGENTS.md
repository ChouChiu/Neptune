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
- `BOT_TOKEN` and `MIMO_API_KEY` are Wrangler secrets (`wrangler secret put`), not in `wrangler.toml`.
- Wrangler doesn't support Bun runtime directly; use `bunx wrangler` (macOS 13.5+) or `--remote` flag.
- Deploy requires running `/set-webhook` on the worker URL to register the Telegram webhook.
- `/set-webhook` also calls `setMyCommands` to sync the BotFather command list. New commands must be added there in `src/index.ts`.

## Bindings vs Env interface

`wrangler.toml` bindings and `src/types.ts` Env must stay in sync:

| wrangler.toml binding | Env field      | Type           |
|-----------------------|---------------|----------------|
| `db`                  | `env.db`      | `D1Database`   |
| `captcha`             | `env.captcha` | `R2Bucket`     |
| `aiContext`           | `env.aiContext` | `KVNamespace` |

Secrets (not in wrangler.toml):
- `BOT_TOKEN` — Telegram bot token
- `MIMO_API_KEY` — Xiaomi MiMo API key for AI chat

Optional env var: `REUSE_CAPTCHA` (`"true"` to enable captcha reuse up to 10 times).

## Database

- Schema: `src/db/schema.sql` — apply with `wrangler d1 execute neptune --remote --file=src/db/schema.sql`
- Queries: `src/db/queries.ts` — all DB access goes through this file
- Tables: `groups`, `keywords`, `admin_connections`, `admin_current_group`, `pending_verifications`, `active_votes`, `vote_records`, `ai_chat_usage`
- Migrations in `migrations/` — apply with `wrangler d1 execute <name> --remote --file=migrations/<file>.sql`
- Migrations must be applied manually; there is no auto-migration on deploy.

## Architecture

```
src/
├── index.ts           # Workers fetch handler: /webhook, /set-webhook, /test
├── bot.ts             # createBot(env) — registers all commands/handlers
├── types.ts           # Env interface + data models
├── commands/          # help, admin, welcome, verify, keywords, votekick, ping
├── handlers/          # chatMember (join+verify flow), message (keyword match + AI chat + captcha reply), votekick (callback query)
├── db/                # schema.sql + queries.ts
└── utils/             # ai-chat (MiMo API + context in KV), captcha (BMP+R2), placeholders, permissions, reply helpers
```

Entry point is `src/index.ts` (`main` in wrangler.toml). Bot instance created per-request via `createBot(env)`.

## AI chat feature

- Triggered by @mention of the bot or replying to the bot's messages in groups.
- Uses Xiaomi MiMo V2.5 API (`https://token-plan-sgp.xiaomimimo.com/v1`).
- Context stored in KV (`aiContext`) as `ai:context:{groupId}`, rolling 7-day window (pruned by timestamp, not count). KV TTL is 8 days as safety net.
- Daily usage tracked in D1 `ai_chat_usage` table (15/day per user, admins exempt).
- System prompt (Neptune persona) is in `src/utils/ai-chat.ts` — it is very long; don't truncate.
- Admin check: Telegram `creator`/`administrator` status OR `admin_connections` table.

## Permission model

- Admin commands check `checkAdminPermission()` in `src/utils/permissions.ts`
- In groups: checks Telegram `administrator`/`creator` status via `getChatMember`
- In private chat: checks `admin_connections` table (bound via `/connect`)
- `/id` in a group auto-connects the user as admin
- `/connect <groupId>` in private chat verifies Telegram admin status before binding
- `/switch` lets admins with multiple groups switch the active one

## Conventions

- All command/handler registration is in `src/bot.ts` via `registerXxxCommands(bot, db)`
- Reply helpers in `src/utils/reply.ts` — `replyOptions(ctx)` and `replyOptionsWithParse(ctx)` (Markdown mode)
- Placeholder replacement: `{nickname}`, `{userid}`, `{groupname}` in `src/utils/placeholders.ts`
- Captcha images stored as BMP in R2 at key `captcha/{groupId}/{userId}.bmp`
