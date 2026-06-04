# AGENTS.md

Telegram group management bot (Neptune). Go single-binary on Debian VPS with SQLite.

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

### Before committing

```bash
make generate          # if any *.templ files changed
make lint              # golangci-lint (NOT ESLint/Biome)
make vet               # go vet
go build ./...         # verify compilation
```

CI (`.github/workflows/deploy.yml`) auto-deploys on push to `main` but does **not** run lint/vet/tests — it only builds and deploys. Run checks locally before push.

Commit style: Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`, etc.)

## Environment

Loaded from `.env` (gitignored). See `.env.example` for all variables. Key ones:

| Variable | Notes |
|----------|-------|
| `BOT_TOKEN` | Required, exits if missing |
| `BOT_USERNAME` | Without `@` — registers `cmd@username` handler variants |
| `HERMES_API_URL` | Hermes Agent API (default `http://127.0.0.1:8642/v1`) |
| `HERMES_API_KEY` | Bearer token for Hermes |
| `GITHUB_WEBHOOK_SECRET` | Also used as `/set-webhook` auth token |
| `RELEASE_CHANNEL_ID` | Telegram channel for GitHub release notifications |
| `REUSE_CAPTCHA` | `"true"` to reuse captcha images across users |

## Key constraints

- Module path: `github.com/kazumi-group/neptune`
- **templ**: `*.templ` → `make generate` → `*_templ.go` (gitignored). Must generate before building.
- **SQLite**: `modernc.org/sqlite` (pure Go, no CGO), WAL mode. All DB access via `internal/db/queries.go` `DB` struct methods.
- **Bot framework**: `github.com/go-telegram/bot` (not grammy)
- **Admin panel**: `github.com/go-chi/chi/v5` + `github.com/a-h/templ` + HTMX. Separate Chi router mounted at `/admin/`.
- **Main server**: standard `net/http` ServeMux (not Chi)
- **Linter**: golangci-lint. **Formatter**: `go fmt` (tabs).

## Architecture

```
cmd/neptune/main.go           # entry: HTTP server, bot init, graceful shutdown
internal/
├── bot/bot.go                # New() creates bot, registerCommand() registers cmd + cmd@username
├── bot/middleware.go          # logging, recovery, groupInit
├── handler/                  # all command + callback handlers
│   ├── orchestrator.go       # catch-all message dispatch (registered as WithDefaultHandler)
│   └── ...
├── adminpanel/               # Chi + Templ + HTMX admin panel
│   ├── server.go             # Chi router, returns http.Handler
│   ├── auth.go               # Telegram Login Widget + HMAC session cookie
│   └── handler/              # API handlers returning HTML fragments
├── github/release.go         # GitHub webhook + GFM→MarkdownV2
├── db/                       # SQLite: db.go (connection), schema.go (auto-apply), queries.go
├── model/model.go            # data structs + Config
└── util/                     # shared helpers (see Conventions below)
migrations/                   # SQL migrations, tracked in schema_migrations table
```

**Adding a feature**: handler in `internal/handler/` → register in `internal/bot/bot.go` → if new DB table: schema in `schema.go`, migration in `migrations/`, queries in `queries.go` → `make generate && make lint && make vet`

## Database

- Schema auto-applied on startup via `ApplySchema()`
- Migrations in `migrations/*.sql`, tracked via `schema_migrations` table, each file runs once
- Tables: `groups`, `keywords`, `admin_connections`, `admin_current_group`, `pending_verifications`, `active_votes`, `vote_records`, `ai_chat_usage`, `warnings`, `reports`, `locks`, `kv`, `schema_migrations`

## AI chat

- Triggered by @mention or replying to bot messages in groups
- Hermes Agent API: `POST {HERMES_API_URL}/chat/completions` (OpenAI-compatible), model `hermes-agent`
- Context: SQLite `kv` table, key `ai:context:{groupId}`, 50 messages, 7-day window
- Daily limit: 15/user/day, admins exempt (`ai_chat_usage` table)
- Timeout 25s, retries up to 2 on 429/5xx
- `ShouldTriggerAi()` filters system-message replies via keyword list
- Typing indicator: goroutine + `context.WithCancel`
- Single-process mutex (`aiContextMu`) — no distributed lock needed

## Other subsystems

- **GitHub webhook**: `POST /github-webhook`, verifies `X-Hub-Signature-256`, processes `action: "published"` only. `!` is reserved in Telegram MarkdownV2 — strip callout markers like `[!Note]` before escaping. Payload uses `\r\n` — normalized to `\n`.
- **Permissions**: `CheckAdminPermission()` in `internal/util/permission.go`. Groups: Telegram admin/creator via `GetChatMember`. Private chat: `admin_connections` table. `/id` in group auto-connects user.
- **Keywords**: `github.com/liuzl/gocc` for Chinese normalization. Regex cached 60s TTL. `/addregex` validates ≤200 chars + 100ms complexity test (ReDoS prevention).
- **Captcha**: BMP at `data/captcha/{groupId}/{userId}.bmp`, 5 attempts max before lockout.

## Conventions

- Reply helpers: `internal/util/reply.go` → `ReplyOptions()`
- MarkdownV2 escaping: `internal/util/markdown.go` → `EscapeMarkdownV2()`
- Placeholders: `{nickname}`, `{userid}`, `{groupname}` in `internal/util/placeholder.go`
- `GetNickname(user)` in `internal/util/nickname.go`
- Orchestrator dispatch order: DM → captcha reply → group AI → keyword match

## Deployment

```bash
make setup DEPLOY_HOST=root@server DOMAIN=bot.example.com   # init server
make deploy DEPLOY_HOST=user@server                          # daily deploy
make webhook DOMAIN=bot.example.com WEBHOOK_SECRET=secret    # register webhook
```

Full guide in [DEPLOY.md](DEPLOY.md). CI needs `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PASSWORD` as GitHub secrets.
