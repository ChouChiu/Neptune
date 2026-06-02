# AGENTS.md

Telegram group management bot (Neptune). **Go single-binary** on Debian VPS with SQLite. Legacy Cloudflare Workers version exists but is no longer the primary target.

## Commands

```bash
make build             # compile → ./bin/neptune
make dev               # hot reload (requires `air`)
make test              # go test ./...
make lint              # golangci-lint run ./...
make vet               # go vet ./...
make generate          # templ generate (run after editing *.templ files)
make build-prod        # Linux amd64 static binary (CGO_ENABLED=0)
make deploy            # rsync + systemctl restart (needs DEPLOY_HOST)
make setup             # initial VPS setup (run once, needs DEPLOY_HOST + DOMAIN)
make webhook           # register Telegram webhook (needs DOMAIN + WEBHOOK_SECRET)
make e2e               # end-to-end tests (needs BASE_URL)
```

### Before building or committing

```bash
make generate          # if any *.templ files changed
make lint              # golangci-lint (NOT ESLint/Biome)
make vet               # go vet
go build ./...         # verify compilation
```

There is no CI — these checks are manual.

## Environment variables

Loaded from `.env` (gitignored). Required:

| Variable | Purpose |
|----------|---------|
| `BOT_TOKEN` | Telegram bot token (required, exits if missing) |
| `BOT_USERNAME` | Bot username without `@` — used for `command@username` handler registration |
| `MIMO_API_KEY` | Xiaomi MiMo V2.5 API key for AI chat |
| `GITHUB_WEBHOOK_SECRET` | HMAC-SHA256 secret for GitHub webhook + `/set-webhook` auth |
| `RELEASE_CHANNEL_ID` | Telegram channel ID for GitHub release notifications |
| `REUSE_CAPTCHA` | `"true"` to reuse captcha images across users |
| `LISTEN_ADDR` | HTTP listen address (default `:8080`) |
| `DB_PATH` | SQLite database path (default `data/neptune.db`) |
| `DATA_DIR` | Data directory root (default `./data`) |

## Key constraints

- **Go modules** (`go.mod` / `go.sum`), module path: `github.com/kazumi-group/neptune`
- **Linter**: golangci-lint. **Formatter**: `go fmt` (tabs).
- **Templ**: `*.templ` files must be compiled with `make generate` before building. Generated `*_templ.go` files are gitignored.
- **SQLite**: `modernc.org/sqlite` (pure Go, no CGO), WAL mode, `data/neptune.db`
- **Bot framework**: `github.com/go-telegram/bot` (not grammy)
- **HTTP router**: `github.com/go-chi/chi/v5` for admin panel; standard `net/http` mux for main server
- **Admin panel**: `github.com/a-h/templ` templates + HTMX frontend (no JS framework)
- **Deployment**: systemd service + Nginx reverse proxy
- **Env**: `.env` file (not `wrangler.toml`), secrets via environment variables

## Architecture

Standard Go server layout: `internal/` enforced private by compiler, `cmd/` for entry points.

**Entry point**: `cmd/neptune/main.go` — HTTP server on `:8080`, mounts `/webhook`, `/admin/`, `/github-webhook`, `/set-webhook`, `/health`. Graceful shutdown on SIGINT/SIGTERM.

**Handler registration**: `internal/bot/bot.go` — `registerCommand()` helper registers both `cmd` and `cmd@username` variants. Orchestrator is the default (catch-all) handler.

**Adding a new feature**:
1. Create handler in `internal/handler/<name>.go`
2. Register in `internal/bot/bot.go` (`registerCommand` or `RegisterHandler` for callbacks)
3. If new DB table: add to `internal/db/schema.go`, create migration in `migrations/`, add queries in `internal/db/queries.go`
4. Run `make generate` if templ files changed, then `make lint && make vet`

## Database

- **Schema**: `internal/db/schema.go` — applied automatically on startup via `ApplySchema()`
- **Migrations**: `migrations/*.sql` — tracked in `schema_migrations` table via `ApplyMigrations()`
- **Queries**: `internal/db/queries.go` — all DB access through `DB` struct methods
- **Tables**: `groups`, `keywords`, `admin_connections`, `admin_current_group`, `pending_verifications`, `active_votes`, `vote_records`, `ai_chat_usage`, `warnings`, `reports`, `locks`, `kv`, `schema_migrations`

## AI chat

- Triggered by @mention or replying to bot messages in groups
- MiMo API: `POST https://token-plan-sgp.xiaomimimo.com/v1/chat/completions`, model `mimo-v2.5`
- **Header**: `api-key` (NOT `Authorization: Bearer`)
- Context in SQLite `kv` table as `ai:context:{groupId}`, limited to 50 messages (7-day window)
- Daily usage: 15/day per user, admins exempt (`ai_chat_usage` table)
- System prompt: `internal/handler/data/system-prompt.json` (embedded via `//go:embed`)
- API timeout 25s via `context.WithTimeout`, retries up to 2 times on 429/5xx
- `ShouldTriggerAi()` filters replies to system messages using keyword list
- Typing indicator: goroutine + `context.WithCancel` (not polling)
- Single-process mutex (`aiContextMu sync.Mutex`) replaces distributed lock

## GitHub Release webhook

- Endpoint: `POST /github-webhook` in `cmd/neptune/main.go`
- Verifies `X-Hub-Signature-256` (HMAC-SHA256)
- Only processes `action: "published"`
- GFM → MarkdownV2 conversion in `internal/github/release.go`
- `!` is reserved in Telegram MarkdownV2 — callout markers like `[!Note]` must be stripped before escaping
- GitHub webhook payload uses `\r\n` — normalized to `\n` before processing

## Permission model

- `CheckAdminPermission()` in `internal/util/permission.go`
- Groups: checks Telegram `administrator`/`creator` via `GetChatMember`
- Private chat: checks `admin_connections` table (bound via `/connect`)
- `/id` in a group auto-connects the user as admin

## Keyword matching

- `github.com/liuzl/gocc` for traditional/simplified Chinese normalization
- Regex patterns cached with 60s TTL
- `/addregex` validates length (≤200 chars) and runs complexity test (100ms threshold) to prevent ReDoS

## Conventions

- Reply helpers: `internal/util/reply.go` → `ReplyOptions()`
- MarkdownV2 escaping: `internal/util/markdown.go` → `EscapeMarkdownV2()`
- Placeholders: `{nickname}`, `{userid}`, `{groupname}` in `internal/util/placeholder.go`
- `GetNickname(user)` in `internal/util/nickname.go`
- Captcha: BMP in `data/captcha/{groupId}/{userId}.bmp`, 5 attempts max before lockout
- `/rule` sets group rules shown during verification (10s reading time via `rule_ack` callback)
- Orchestrator dispatch: DM → captcha reply → group AI → keyword match (in `handler/orchestrator.go`)

## Deployment

Full guide in [DEPLOY.md](DEPLOY.md). `make help` shows the quickstart.

```bash
make setup DEPLOY_HOST=root@your-server DOMAIN=bot.example.com  # init server
make deploy DEPLOY_HOST=user@your-server                         # daily deploy
make webhook DOMAIN=bot.example.com WEBHOOK_SECRET=your-secret   # register webhook
```

Prerequisites: Debian/Ubuntu VPS, domain pointing to server, `BOT_TOKEN` + `BOT_USERNAME` + `MIMO_API_KEY`.
