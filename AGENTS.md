# AGENTS.md

Telegram group management bot (Neptune). Two deployment targets:
- **Go VPS** (primary): single binary on Debian VPS with SQLite
- **Cloudflare Workers** (legacy): TypeScript on Workers with D1/KV/R2

## Commands

### Go version (primary)

```bash
make build             # compile
make dev               # hot reload (air)
make test              # go test
make lint              # golangci-lint
make vet               # go vet
make generate          # templ generate
make build-prod        # Linux amd64 static binary
make deploy            # rsync + systemctl restart
make deploy-full       # deploy + D1 data migration + R2 captcha migration
make setup             # initial VPS setup (run once)
make webhook           # register Telegram webhook
make e2e               # end-to-end tests
```

### Cloudflare Workers version (legacy)

```bash
bun install              # install deps
bun run lint             # Biome check (NOT ESLint)
bun run lint:fix         # auto-fix
bun run typecheck        # tsc --noEmit
bun run dev              # wrangler dev (local)
bun run deploy           # wrangler deploy
```

## Key constraints

### Go version

- Package manager: **Go modules** (`go.mod` / `go.sum`)
- Linter: **golangci-lint** (not ESLint/Biome)
- Formatter: `go fmt` (tabs, standard Go conventions)
- Templ: `*.templ` files must be compiled with `templ generate` before building
- SQLite: `modernc.org/sqlite` (pure Go, no CGO), WAL mode, `data/neptune.db`
- Captcha images: `data/captcha/{groupId}/{userId}.bmp` (local filesystem)
- AI context: `kv` table in SQLite (replaces Cloudflare KV)
- Bot framework: `github.com/go-telegram/bot` (not grammy)
- HTTP router: `github.com/go-chi/chi/v5` for admin panel
- Admin panel: `github.com/a-h/templ` templates + HTMX frontend
- Deployment: systemd service + Nginx reverse proxy
- Environment: `.env` file (not `wrangler.toml`), secrets via environment variables
- `/set-webhook?token=<GITHUB_WEBHOOK_SECRET>` registers webhook and syncs command list

### Cloudflare Workers version (legacy)

- Package manager is **Bun** (no npm/yarn lockfiles).
- Linter/formatter is **Biome** (not ESLint/Prettier). Uses **tabs** and **double quotes**.
- `wrangler.toml` is **gitignored** — `wrangler.example.toml` is the template.
- Secrets via `wrangler secret put`: `BOT_TOKEN`, `MIMO_API_KEY`, `GITHUB_WEBHOOK_SECRET`.

## Database (Go version)

- Schema: `internal/db/schema.go` — applied automatically on startup via `ApplySchema()`
- Migrations: `migrations/*.sql` — applied via `ApplyMigrations()` with tracking table
- Queries: `internal/db/queries.go` — all DB access through `DB` struct methods
- Tables: `groups`, `keywords`, `admin_connections`, `admin_current_group`, `pending_verifications`, `active_votes`, `vote_records`, `ai_chat_usage`, `warnings`, `reports`, `locks`, `kv`
- Data migration from D1: `deploy/migrate-d1-to-sqlite.sh`

## Architecture (Go version)

**Standard Go Server Layout**: `internal/` enforced private by compiler, `cmd/` for entry points.

```
neptune/
├── cmd/neptune/main.go           # Entry: HTTP server + bot init + graceful shutdown
├── internal/
│   ├── bot/bot.go                # Bot creation, handler registration
│   ├── bot/middleware.go         # Logging, recovery, group init middleware
│   ├── handler/                  # All command + callback handlers
│   │   ├── admin.go              # /id, /connect, /switch
│   │   ├── ai_chat.go            # MiMo API, context, skills, typing indicator
│   │   ├── captcha_handler.go    # DM captcha reply handler
│   │   ├── helpers.go            # requireGroup, requireReplyTarget, etc.
│   │   ├── keywords.go           # /addkeyword, /addregex, keyword matching
│   │   ├── orchestrator.go       # Message dispatch: DM→captcha, group→AI→keywords
│   │   ├── ping.go               # /ping
│   │   ├── help.go               # /help
│   │   ├── report.go             # /report
│   │   ├── rule.go               # /rule
│   │   ├── verify.go             # /start verify*, captcha flow
│   │   ├── votekick.go           # /kick, vote callbacks
│   │   ├── warn.go               # /warn
│   │   ├── welcome.go            # /setwelcome, new_chat_members
│   │   └── data/                 # Embedded JSON (system-prompt, skills)
│   ├── adminpanel/               # Web admin panel (Chi + Templ + HTMX)
│   │   ├── server.go             # Chi router setup
│   │   ├── auth.go               # Telegram Login Widget + HMAC session
│   │   ├── middleware.go         # Session validation middleware
│   │   ├── handler/              # API handlers (reports, warnings)
│   │   └── components/           # Templ templates (layout, reports, warnings, common)
│   ├── github/release.go         # GitHub webhook + GFM→MarkdownV2
│   ├── db/                       # SQLite database layer
│   ├── model/                    # Data structs
│   └── util/                     # Shared utilities
├── deploy/                       # Deployment scripts + configs
│   ├── setup.sh                  # Initial VPS setup
│   ├── neptune.service           # systemd unit
│   ├── nginx.conf                # Nginx reverse proxy
│   ├── migrate-d1-to-sqlite.sh   # D1 → SQLite data migration
│   ├── migrate-r2-captcha.sh     # R2 → local captcha migration
│   ├── register-webhook.sh       # Telegram webhook registration
│   └── e2e-test.sh               # End-to-end test suite
├── migrations/                   # SQL migration files
├── static/                       # Tailwind CSS output
├── data/
│   ├── neptune.db                # SQLite database
│   └── captcha/                  # Captcha BMP images
└── Makefile                      # Build + deploy targets
```

### Adding a new feature (Go)

1. Create handler in `internal/handler/<name>.go`
2. Register handler in `internal/bot/bot.go` (command or callback)
3. Add command to `SetCommands` in `internal/bot/bot.go`
4. If new DB table: add to `schema.go`, create migration in `migrations/`, add queries in `queries.go`
5. Update README command list

### Admin panel modules

`internal/adminpanel/` uses Chi router + Templ templates + HTMX:
- `server.go` — route registration
- `auth.go` — Telegram Login Widget HMAC-SHA256 verification + session cookie
- `handler/` — API endpoints returning HTML fragments (Templ rendered)
- `components/` — Templ templates (layout, reports, warnings, common)

## AI chat feature

- Triggered by @mention of the bot or replying to the bot's messages in groups.
- Uses Xiaomi MiMo V2.5 API (`https://token-plan-sgp.xiaomimimo.com/v1`).
- Header: `api-key` (not `Authorization: Bearer`).
- Context stored in SQLite `kv` table as `ai:context:{groupId}`, limited to 50 messages (7-day window).
- Daily usage tracked in `ai_chat_usage` table (15/day per user, admins exempt).
- System prompt: `internal/handler/data/system-prompt.json` (embedded via `//go:embed`).
- API call has 25s timeout via `context.WithTimeout`, retries up to 2 times on 429/5xx errors.
- `ShouldTriggerAi()` filters out replies to system messages using keyword list.
- Typing indicator: goroutine + `context.WithCancel` (not `setInterval`).
- Single-process mutex replaces distributed lock (`aiContextMu sync.Mutex`).

## GitHub Release webhook

- Endpoint: `POST /github-webhook` in `cmd/neptune/main.go`
- Receives GitHub `release` events, verifies `X-Hub-Signature-256` (HMAC-SHA256)
- Only processes `action: "published"` (ignores `created`, `edited`, `prereleased`, `draft`)
- GFM → MarkdownV2 conversion in `internal/github/release.go`
- `!` is reserved in Telegram MarkdownV2 — callout markers like `[!Note]` must be stripped before escaping
- GitHub webhook payload uses `\r\n` line endings — normalized to `\n` before regex processing

## Permission model

- Admin commands check `CheckAdminPermission()` in `internal/util/permission.go`
- In groups: checks Telegram `administrator`/`creator` status via `GetChatMember`
- In private chat: checks `admin_connections` table (bound via `/connect`)
- `/id` in a group auto-connects the user as admin
- `/connect <groupId>` in private chat verifies Telegram admin status before binding

## Keyword matching

- Uses `github.com/liuzl/gocc` for traditional/simplified Chinese normalization
- Regex patterns are compiled once and cached with 60s TTL
- `/addregex` validates regex length (≤200 chars) and runs complexity test (100ms threshold) to prevent ReDoS

## Conventions

- Reply helpers in `internal/util/reply.go` — `ReplyOptions()`
- `EscapeMarkdownV2()` escapes all Telegram MarkdownV2 special chars
- Placeholder replacement: `{nickname}`, `{userid}`, `{groupname}` in `internal/util/placeholder.go`
- `GetNickname(user)` in `internal/util/nickname.go`
- Captcha images stored as BMP in `data/captcha/{groupId}/{userId}.bmp`, 5 attempts max before lockout
- `/rule` command sets group rules shown during verification flow (10s reading time via `rule_ack` callback)
- Dependencies: `go-telegram/bot`, `modernc.org/sqlite`, `chi/v5`, `a-h/templ`, `liuzl/gocc`

## Deployment

### VPS setup (first time)

```bash
make setup DEPLOY_HOST=root@your-server DOMAIN=bot.example.com
# Edit /opt/neptune/.env with secrets
```

### Regular deploy

```bash
make deploy DEPLOY_HOST=user@your-server
```

### Full deploy (with data migration from Cloudflare)

```bash
make deploy-full DEPLOY_HOST=user@your-server
```

### Webhook registration

```bash
make webhook DOMAIN=bot.example.com WEBHOOK_SECRET=your-secret
# or directly:
curl https://bot.example.com/set-webhook?token=your-secret
```
