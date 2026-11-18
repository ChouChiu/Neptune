# AGENTS.md

Telegram group management bot (Neptune). Single-binary Go service with Docker deployment on Debian/Ubuntu VPS, pure-Go SQLite storage, HTMX/Templ admin panel, and GitHub Release webhook integration.

## 1. Eight Honors and Eight Shames

1. Shame on guessing interfaces; honor careful consultation.
2. Shame on vague execution; honor seeking confirmation.
3. Shame on blindly imagining business; honor human confirmation.
4. Shame on inventing interfaces; honor reusing existing ones.
5. Shame on skipping verification; honor proactive testing.
6. Shame on breaking architecture; honor following conventions.
7. Shame on pretending to understand; honor honest ignorance.
8. Shame on blind modification; honor cautious refactoring.

## 2. Agent Working Rules

1. Use first-principles thinking; first confirm the real goals, constraints, and success criteria, and do not apply conventions by default.
2. For simple questions, answer directly; for complex questions, use `answer-me-with-html`.
3. Write pitfalls encountered into the project's `HANDOFF.md`; before executing a new task, check `HANDOFF.md` first to avoid repeating past pitfalls.
4. For tasks involving release, deployment, or production environments, include performance overhead in design and acceptance criteria.
5. UI: Avoid unnecessary secondary labels and descriptions. Let the layout do the explaining.

## 3. HANDOFF.md Format Reference

`HANDOFF.md` must be written in English and include the following format reference:

```markdown
# HANDOFF.md

This document records traps, pitfalls, edge cases, and lessons learned across development, deployment, and configuration tasks.

> **Rule for Agents:**  
> 1. Check this file **before starting any new task** to avoid repeating past mistakes.  
> 2. Append new findings and pitfalls encountered during tasks immediately.

---

## Pitfall Log & Lessons Learned

| Date | Category | Issue / Trap Encountered | Root Cause & Solution |
| :--- | :--- | :--- | :--- |
| *(Initial)* | Setup | Placeholder | Document issues here as they are resolved. |
| 2026-10-25 | Tooling | `wrangler types` and `tsconfig` layout | `wrangler types [path]` supports explicit output path (`src/worker/worker-configuration.d.ts`), which automatically shortens module imports to `./index`. Sub-tsconfigs placed in `tsconfig/` must resolve relative paths (`../src/*`) while keeping `@/*` alias in root `tsconfig.json` for shadcn CLI. |
| 2026-10-25 | Config | `wrangler.example.jsonc` placeholders | Wrangler rejects an empty string id (`"kv_namespaces[0]" bindings should have a string "id" field`), so the example config must use non-empty placeholders (`<kv-namespace-id>`). The local D1/KV state in `.wrangler/state` is keyed by that string, so tests and `bun run dev` work with placeholders unchanged. |
| 2026-10-25 | Deployment | Untracked `wrangler.jsonc` breaks the Deploy button | `wrangler.jsonc` holds account resource IDs and is now gitignored (template: `wrangler.example.jsonc`, created automatically by `bun run setup`). The Deploy to Cloudflare flow reads the wrangler config from the forked repository, so that path requires committing a copy of the example to your own fork and pointing the button's `url` at it. |
```

---

## 4. Tech Stack & Key Constraints

- **Language & Module**: Go `1.26.3`, module `github.com/ChouChiu/neptune`.
- **Telegram Framework**: `github.com/go-telegram/bot` (`v1.29.0`) operating in webhook mode.
- **Database**: `modernc.org/sqlite` (`v1.62.0`) — pure Go (no CGO required). Opened with `_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)` and pooled with `MaxOpenConns(1)` / `MaxIdleConns(1)`.
- **Main HTTP Server**: Standard library `net/http.ServeMux` (`ReadTimeout: 10s`, `WriteTimeout: 30s`, `IdleTimeout: 60s`).
- **Admin Panel**: `github.com/go-chi/chi/v5` (`v5.5.0`) + `github.com/a-h/templ` (`v0.3.1118`) + HTMX (`2.0.6` via CDN) + Tailwind browser runtime + Server-Sent Events (SSE), mounted under `/admin/`.
- **Chinese Normalization**: `github.com/liuzl/gocc` (`t2s` Traditional-to-Simplified converter initialized at package init in `internal/util/chinese.go`).
- **Template Generation**: `*.templ` files compile via `templ generate` into gitignored `*_templ.go` files. Always run `make generate` after editing `.templ` files (`make build`, `make test`, `make lint`, `make vet`, and `make tidy` invoke `generate` automatically).
- **Formatting & Linting**: `go fmt` (tabs) and `golangci-lint` (NOT ESLint/Biome).

---

## 5. Commands & Verification

```bash
make generate          # run `templ generate` for *.templ templates
make build             # generate templ + compile -> ./bin/neptune
make run               # build and execute ./bin/neptune
make dev               # hot reload (requires `air`)
make test              # generate templ + go test -v ./...
make test-coverage     # generate templ + coverage.out + coverage.html
make lint              # generate templ + golangci-lint run ./...
make lint-fix          # generate templ + golangci-lint run --fix ./...
make vet               # generate templ + go vet ./...
make tidy              # generate templ + go mod tidy
make fmt               # go fmt ./...
make clean             # remove ./bin, coverage.out, coverage.html
make docker-build      # docker compose build
make docker-up         # docker compose up -d
make docker-down       # docker compose down
make docker-logs       # docker compose logs -f
make webhook           # register Telegram webhook (requires DOMAIN, WEBHOOK_SECRET, BOT_TOKEN)
make e2e               # run deploy/e2e-test.sh against BASE_URL
```

### Required Pre-Commit Checks

```bash
make generate
make fmt
make lint
make vet
go test -count=1 -race -v ./...
go build ./...
```

> **CI Warning**: `.github/workflows/deploy.yml` triggers on push to `main` and directly SSHes into the VPS to run `git pull origin main` and `docker compose up --build -d --remove-orphans`. It does **not** run `golangci-lint`, `go vet`, or `go test`. Always run all checks locally before pushing.

Commit message format: Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`, `perf:`, `test:`, `chore:`).

---

## 6. Environment Configuration

Configuration is loaded and validated at startup by `model.LoadConfigFromEnv()` in `internal/model/model.go`. Local `.env` is gitignored; see `.env.example` for the template.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `BOT_TOKEN` | Yes | — | Telegram Bot API token from `@BotFather`; startup exits with code `1` if empty |
| `BOT_USERNAME` | Recommended | `""` | Bot username without `@` (leading `@` is stripped). Used for `cmd@username` matching, verification deep links, and Telegram Login Widget. If empty, `bot.New` calls `b.GetMe` once at startup |
| `GITHUB_WEBHOOK_SECRET` | Recommended | `""` | HMAC-SHA256 secret for `POST /github-webhook` and bearer/query token for `GET /set-webhook` |
| `RELEASE_CHANNEL_ID` | No | `0` | Target Telegram channel ID (e.g. `-100...`) for GitHub release notifications |
| `LISTEN_ADDR` | No | `:8080` | HTTP server listen address |
| `DB_PATH` | No | `data/neptune.db` | SQLite file path; parent directory is created automatically (`0755`) |

---

## 7. Repository Architecture & File Map

```
cmd/neptune/
└── main.go                            # App entrypoint: log collector + multi-slog, env config, DB init/migrations,
                                       # bot init + SetCommands, HTTP mux routes, webhook worker, graceful shutdown

internal/
├── model/
│   └── model.go                       # Config, LoadConfigFromEnv(), and DB/domain structs:
│                                      # GroupConfig, KeywordRule, PendingVerification, ActiveVote, VoteRecord, Warning, Report
├── db/
│   ├── db.go                          # DB struct (*sql.DB wrapper), New() with WAL/busy_timeout/FK pragmas, Close(), helpers
│   ├── schema.go                      # baseSchema DDL, ApplySchema(), ApplyMigrations() tracking migrations/*.sql
│   ├── group.go                       # InitGroup, GetGroupConfig, group setting updaters, ConnectAdmin,
│   │                                  # GetAdminGroupID, GetAdminGroups, IsAdminConnected, SetCurrentGroup, ErrAdminNotConnected
│   ├── keyword.go                     # AddKeyword, GetKeywords, RemoveKeyword
│   ├── verification.go                # AddPendingVerification, SetPendingCaptcha, SetRuleAckDone,
│   │                                  # MarkFirstMessagePrompted, UpdatePendingWelcomeMessageID,
│   │                                  # GetPendingVerification, RemovePendingVerification, GetPendingVerificationsByUser,
│   │                                  # CleanExpiredVerifications, IncrementVerificationAttempts, AcquireLock, ReleaseLock
│   ├── votekick.go                    # CreateActiveVote, GetActiveVote, GetActiveVoteForTarget, GetLastVoteByInitiator,
│   │                                  # UpdateVoteMessageID, DeleteActiveVote, AddVoteRecord, GetVoteCounts, CleanExpiredVotes
│   ├── moderation.go                  # AddWarning, GetWarningCount, GetWarnings, GetAllWarnings,
│   │                                  # AddReport, GetReports, GetReport, ResolvePendingReport
│   └── db_test.go                     # Integration tests against temp SQLite DB + full migrations
├── bot/
│   ├── bot.go                         # New() creates tgbot.Bot, resolves BotUsername via GetMe if unset,
│   │                                  # registers 22 commands (+ @username variants), 3 callbacks, join matchers; SetCommands()
│   └── middleware.go                  # loggingMiddleware, recoveryMiddleware, groupInitMiddleware, unverifiedMessageMiddleware
├── handler/
│   ├── orchestrator.go                # WithDefaultHandler: private chat -> HandleCaptchaReply; group/supergroup -> HandleKeywordMatch
│   ├── helpers.go                     # requireGroupReply, replyTarget, requireReplyTarget, requireNonBot, requireAdmin,
│   │                                  # chatMemberUser, sendMessage, answerCallbackQuery, editMessageText/ReplyMarkup, deleteMessage
│   ├── ping.go                        # /ping -> "🏓 Pong!"
│   ├── help.go                        # /help -> static Chinese command reference
│   ├── admin.go                       # /id (group auto-connect), /connect (DM admin binding), /switch + "switch:" callback
│   ├── welcome.go                     # /setwelcome, /enablewelcome, /disablewelcome, WelcomeNewMembers,
│   │                                  # WelcomeNewMembersFromChatMember, HandleUnverifiedGroupMessage, delayed prompt deletion
│   ├── welcome_test.go                # Unit tests for IsChatMemberJoin
│   ├── verify.go                      # /start (verify<groupID>_<userID> deep link), "rule_ack:" callback,
│   │                                  # /setverifybutton, /setverifytimeout, /testverify, sendCaptcha
│   ├── captcha_handler.go             # HandleCaptchaReply (DM captcha verification, max 5 attempts lockout, prompt cleanup & unrestrict)
│   ├── rule.go                        # /rule (view/set/off, max 2048 bytes)
│   ├── keywords.go                    # /addkeyword, /addregex, /listkeywords, /removekeyword, HandleKeywordMatch, 60s TTL cache
│   ├── keywords_test.go               # Unit tests for regex & traditional/simplified matching and arg splitting
│   ├── votekick.go                    # /enablevotekick, /disablevotekick, /kick, "vk:" callback (5 yes votes threshold)
│   ├── warn.go                        # /warn [reason] (reply-based, admin-only, cannot warn bots/admins)
│   └── report.go                      # /report <reason> (reply-based, stores message text/caption & ID)
├── adminpanel/
│   ├── server.go                      # NewServer() Chi router (Recoverer, Compress(5)), SPA /, /auth/*, /api/*, SSE log stream
│   ├── auth.go                        # VerifyTelegramAuth (SHA256(botToken) HMAC + 24h auth_date window), SignSession, VerifySession
│   ├── auth_test.go                   # Unit tests for Telegram auth, session HMAC, and 409 duplicate report resolution
│   ├── middleware.go                  # extractSessionToken (nep_session cookie) + SessionAuthMiddleware
│   ├── logcollector.go                # Ring-buffer LogCollector + slog.Handler + MultiHandler (stdout + SSE fanout)
│   ├── sessionctx/
│   │   └── sessionctx.go              # Typed context key helpers WithUserID / UserID
│   ├── handler/
│   │   ├── helpers.go                 # getUserID, handleApproved (delete msg, warn, notify group & reporter), handleDismissed, tgAPICall
│   │   ├── reports.go                 # ListReports (HTMX HTML fragment), ResolveReport (atomic status transition + side effects)
│   │   └── warnings.go                # ListWarnings (HTMX HTML fragment)
│   └── components/
│       ├── layout.templ               # SPA shell, Telegram Login Widget, sidebar, client-side section scripts & SSE viewer
│       ├── common.templ               # EmptyState, StatusBadge
│       ├── reports.templ              # ReportCards, reportCard
│       ├── warnings.templ             # WarningRows, warningRow
│       └── helpers.go                 # FormatTimeAgo
├── github/
│   ├── release.go                     # VerifySignature (HMAC-SHA256), ConvertGfmToMarkdownV2, FormatReleaseMessage,
│   │                                  # SendToTelegram (3 retries), HandleGitHubWebhook
│   └── release_test.go                # Unit tests for signature, GFM conversion, UTF-8 rune truncation
└── util/
    ├── captcha.go                     # GenerateCaptcha (24-bit BMP with 5x7 dot-matrix font, interference lines/dots)
    ├── chinese.go                     # ToSimplified() using gocc "t2s"
    ├── markdown.go                    # EscapeMd, EscapeMarkdownCode, EscapeMarkdownLinkUrl, MarkdownBold, MarkdownLink,
    │                                  # MarkdownPlaceholderStore, TrimCodeFencePadding
    ├── nickname.go                    # GetNickname(user)
    ├── permission.go                  # CheckAdminPermission(ctx, b, db, msg)
    ├── placeholder.go                 # ReplacePlaceholders({nickname}, {userid}, {groupname}, plain or MarkdownV2 + pre-escaped)
    ├── reply.go                       # ReplyOptions, CommandArgs
    ├── time.go                        # CurrentTimestamp, RandomString
    └── util_test.go                   # Unit tests for captcha, CommandArgs, placeholders, markdown, t2s

migrations/                            # Numbered SQL migrations (0002..011), applied once in lexicographical order
deploy/
├── setup.sh                           # Debian/Ubuntu VPS bootstrap: Docker install, clone to /opt/neptune, .env, data/
├── register-webhook.sh                # Calls /set-webhook or Telegram setWebhook + syncs 21 bot commands
├── e2e-test.sh                        # Smoke tests against live HTTP endpoints & Telegram getMe/getWebhookInfo/getMyCommands
└── nginx.conf.example                 # Reverse proxy config with rate limiting for /tghook and unbuffered SSE for /admin/
```

---

## 8. HTTP Endpoints & Bot Dispatch Reference

### Main Server Routes (`cmd/neptune/main.go`)

| Method & Path | Auth | Behavior |
|---------------|------|----------|
| `POST /tghook` | Telegram webhook | Dispatches incoming Telegram `Update` JSON via `b.WebhookHandler()` |
| `GET /health` | None | Returns `200 OK` (`"OK"`); used by Docker `HEALTHCHECK` |
| `GET /set-webhook` | Query `token == GITHUB_WEBHOOK_SECRET` | Sets Telegram webhook to `<scheme>://<host>/tghook` with `AllowedUpdates: ["message", "chat_member", "callback_query"]` |
| `POST /github-webhook` | Header `X-Hub-Signature-256` | Verifies HMAC-SHA256 (max 5 MiB body), formats published non-draft release, sends to `RELEASE_CHANNEL_ID` |
| `/admin/*` | Cookie `nep_session` (except `/`, `/auth/*`) | Stripped to `adminpanel.NewServer` Chi router |

### Admin Panel Routes (`internal/adminpanel/server.go`)

| Method & Path | Auth | Response |
|---------------|------|----------|
| `GET /admin/` | None | Renders `components.Layout(botUsername)` HTML SPA |
| `POST /admin/auth/login` | Telegram Login Widget HMAC | Validates widget payload (max 32 KiB body), verifies `len(GetAdminGroups) > 0`, sets `HttpOnly` `SameSite=Lax` cookie `nep_session` (`Path=/admin`, TTL `86400s`) |
| `GET /admin/auth/me` | Optional `nep_session` | Returns `{"user": ...}` or `{"user": null}` (clears invalid cookie) |
| `GET /admin/api/reports?status=...` | `SessionAuthMiddleware` | Returns `components.ReportCards` HTML fragment scoped to admin's connected groups |
| `POST /admin/api/reports/{reportID}/resolve` | `SessionAuthMiddleware` | Accepts `{"status":"approved"|"dismissed"}` (max 4 KiB); returns `409 Conflict` if not pending; executes Telegram side effects |
| `GET /admin/api/warnings` | `SessionAuthMiddleware` | Returns `components.WarningRows` HTML fragment scoped to admin's connected groups |
| `GET /admin/api/logs/stream` | `SessionAuthMiddleware` | SSE stream (`text/event-stream`, `X-Accel-Buffering: no`) replaying ring buffer (`500` entries) and streaming live `slog` records |

### Bot Middleware & Handler Pipeline (`internal/bot/`)

Middlewares execute in exact registration order:
1. `loggingMiddleware`: logs `Message`, `CallbackQuery`, `ChatMember`, or `MyChatMember` metadata.
2. `recoveryMiddleware`: recovers panics with `debug.Stack()`.
3. `groupInitMiddleware`: ensures `INSERT OR IGNORE INTO groups (group_id)` exists for group/supergroup `Message` and `ChatMember` updates.
4. `unverifiedMessageMiddleware`: intercepts group/supergroup messages from unverified non-bot members via `HandleUnverifiedGroupMessage`.

Registered handlers:
- **Commands** (both `/cmd` and `/cmd@<BOT_USERNAME>`):
  - General: `/ping`, `/help`, `/id`
  - Admin DM binding: `/connect`, `/switch`
  - Welcome & Verification: `/start`, `/setwelcome`, `/enablewelcome`, `/disablewelcome`, `/setverifybutton`, `/setverifytimeout`, `/testverify`, `/rule`
  - Auto-reply: `/addkeyword`, `/addregex`, `/listkeywords`, `/removekeyword`
  - Votekick: `/enablevotekick`, `/disablevotekick`, `/kick`
  - Moderation: `/warn`, `/report`
- **Callbacks**: `switch:<groupID>`, `rule_ack:<groupID>:<showTime>`, `vk:<voteID>:<0|1>`
- **Join Matchers**: `Message.NewChatMembers` (`WelcomeNewMembers`) and `ChatMember` join transitions (`WelcomeNewMembersFromChatMember` via `IsChatMemberJoin`).
- **Default Handler**: `handler.Orchestrator` — private messages dispatch to `HandleCaptchaReply`; group/supergroup messages dispatch to `HandleKeywordMatch`.

---

## 9. Database Schema & Migrations

All schema changes must be reflected in **both** `internal/db/schema.go` (`baseSchema`) and a new numbered file in `migrations/NNN_description.sql`. Duplicate-column `ALTER TABLE ADD COLUMN` errors during migrations are ignored idempotently in `applyMigrationSQL`.

| Table | Primary Key | Key Columns & Foreign Keys |
|-------|-------------|----------------------------|
| `groups` | `group_id` | `welcome_enabled`, `welcome_message`, `verify_button_text`, `verify_timeout` (default `300`), `votekick_enabled`, `rule` |
| `keywords` | `id` (auto) | `group_id` (FK `groups`), `pattern`, `is_regex`, `reply_content`, `reply_type` |
| `admin_connections` | `(user_id, group_id)` | `group_id` (FK `groups`) |
| `admin_current_group` | `user_id` | `group_id` (FK `groups`) |
| `pending_verifications` | `(user_id, group_id)` | `captcha_text`, `expires_at` (idx), `welcome_message_id`, `attempts`, `rule_ack_done`, `prompted_on_message` |
| `active_votes` | `vote_id` | `group_id`, `target_id`, `initiator_id`, `message_id`, `created_at`, `expires_at` (idx) |
| `vote_records` | `(vote_id, voter_id)` | `choice` (`1` = yes, `0` = no) |
| `warnings` | `id` (auto) | `group_id` (FK `groups`), `user_id`, `admin_id`, `reason`, `created_at`; idx `(group_id, user_id)` |
| `reports` | `id` (auto) | `group_id` (FK `groups`), `reporter_id`, `reported_user_id`, `reported_message_id`, `reported_message_text`, `content`, `status` (idx), `reviewed_by`, `reviewed_at`, `created_at` |
| `locks` | `name` | `expires_at` |
| `schema_migrations` | `version` | `applied_at` |

---

## 10. Subsystem Details

### Two-Prompt Captcha Verification
1. **Member Join (`WelcomeNewMembers` / `WelcomeNewMembersFromChatMember`)**:
   - Acquires DB lock `welcome:<groupID>:<userID>` (`10s` TTL) to deduplicate simultaneous `new_chat_members` and `chat_member` updates.
   - Deletes the Telegram join service message (when arriving via `new_chat_members`).
   - Sends the **1st verification prompt** with an inline deep-link button (`https://t.me/<bot>?start=verify<groupID>_<userID>`), **without restricting the user**.
   - Upserts `pending_verifications` with `prompted_on_message = 0`, `attempts = 0`, `welcome_message_id`, and schedules message deletion after `5m`.
2. **First Group Message Before Verification (`HandleUnverifiedGroupMessage`)**:
   - Intercepted in `unverifiedMessageMiddleware` whenever `GetPendingVerification(userID, groupID) != nil`.
   - Immediately **deletes the user's group message** and **restricts the user** (`restrictUser`, all permissions false).
   - Atomically transitions `prompted_on_message` from `0` to `1` via `MarkFirstMessagePrompted` (preventing duplicate 2nd prompts on concurrent messages).
   - Deletes the 1st prompt if still present, sends the **2nd verification prompt**, updates `welcome_message_id`, and schedules deletion after `5m`.
3. **DM Verification (`StartVerify` -> `RuleAckCallback` -> `HandleCaptchaReply`)**:
   - Checks lockout (`Attempts >= 5`) and expiration (`ExpiresAt <= now`); does **not** delete locked records on max attempts so users cannot reset attempts by re-clicking the link.
   - If group `Rule != ""` and `RuleAckDone == 0`, enforces a `10s` countdown (`ruleAckWaitSeconds`) before issuing the captcha.
   - Generates an in-memory 5-character 24-bit BMP captcha (`util.GenerateCaptcha(5)`).
   - On case-insensitive match: immediately deletes the current group prompt (`WelcomeMessageID`), unrestricts the member via `unrestrictUser` if `PromptedOnMessage != 0`, deletes the `pending_verifications` row, and notifies the user in DM.
   - `CleanExpiredVerifications()` deletes rows only when `prompted_on_message = 1 AND expires_at < now`, ensuring unspoken new members do not bypass first-message verification by waiting.

### Admin Permissions & Multi-Group Management
- `util.CheckAdminPermission`: in groups/supergroups, checks Telegram `administrator` or `owner` via `GetChatMember`; in private chats, queries `GetAdminGroupID` (which joins `admin_current_group` with `admin_connections` with **no fallback**, preventing stale unbound access).
- Sending `/id` in a group auto-binds Telegram admins via `ConnectAdmin`. `/connect <groupID>` binds from DM after verifying group admin status. `/switch` displays an inline keyboard (`switch:<groupID>`) to toggle `admin_current_group`.

### Keyword & Regex Auto-Reply
- Limits: pattern `<= 200` bytes, reply `<= 4096` bytes (`maxKeywordPatternLen`, `maxKeywordReplyLength`).
- Compiled rules are cached per group in memory with a `60s` TTL (`keywordCacheTTL`) using `sync.RWMutex` double-checked locking; mutations (`/addkeyword`, `/addregex`, `/removekeyword`) invalidate the group's cache entry.
- Both plain substring and `(?i)` regex rules are normalized with `util.ToSimplified` (`gocc` `t2s`) for Traditional/Simplified Chinese cross-matching.
- Placeholder substitution (`{nickname}`, `{userid}`, `{groupname}`) handles both raw tokens and `EscapeMd`-escaped `\{...\}` tokens when `markdown == true`.

### Votekick & Moderation
- **Votekick**: Requires `VotekickEnabled != 0`. Targets cannot be self, bots, or admins. Initiator cooldown `60s`, vote duration `300s`, kick threshold `5` yes votes (`voteThreshold`). On reaching threshold, immediately deletes vote message, bans target, deletes active vote row, and answers callback.
- **Reports & Warnings**: `/report <reason>` (reply-based) captures `ReportedMessageID` and message text/caption. `/warn [reason]` (reply-based, admin-only) records a warning. Admin Panel `ResolveReport` uses `ResolvePendingReport` (`WHERE id = ? AND status = 'pending'`) so concurrent/duplicate resolutions return `409 Conflict`, then records a warning with the reviewing admin's `userID` and sends Telegram notifications.

### GitHub Release Webhook
- Validates `X-Hub-Signature-256` with `hmac.Equal`. Processes `action == "published"` and `!release.Release.Draft`.
- Strips GFM callout markers (`[!NOTE]`, `[!WARNING]`, `[!TIP]`, `[!IMPORTANT]`, `[!CAUTION]`), protects code blocks/links/bold/blockquotes via `MarkdownPlaceholderStore`, escapes remaining text with `EscapeMd`, and binary-searches rune truncation to fit within Telegram's `4096`-rune limit (`maxMessageLength`). Sends with `3` retries (`1s` delay).

---

## 11. Coding Conventions & Adding Features

- **Reply & Arg Parsing**: Use `util.ReplyOptions(msg)` and `util.CommandArgs(text, "/cmd")`.
- **MarkdownV2**: Always escape dynamic user/group strings with `util.EscapeMd()` when using `models.ParseModeMarkdown` (note: in `go-telegram/bot`, `models.ParseModeMarkdown` equals `"MarkdownV2"`).
- **Handler Guards**: Reuse `requireGroupReply`, `requireAdmin`, `requireReplyTarget`, `requireNonBot`, and `sendMessage` in `internal/handler/helpers.go`.
- **Adding a Bot Command**:
  1. Implement the handler in `internal/handler/`.
  2. Register it via `registerCommand(...)` and add it to `SetCommands(...)` in `internal/bot/bot.go` (also update `deploy/register-webhook.sh`, `deploy/e2e-test.sh`, and `README.md`).
  3. If modifying persistence, update `internal/model/model.go`, `internal/db/schema.go`, add `migrations/NNN_<name>.sql`, and add methods to the matching domain file in `internal/db/`.
- **Adding an Admin Panel Feature**:
  1. Add/update `.templ` templates in `internal/adminpanel/components/` and run `make generate`.
  2. Add HTTP handlers in `internal/adminpanel/handler/` (read admin ID via `sessionctx.UserID(r)`).
  3. Register routes under the `SessionAuthMiddleware` group in `internal/adminpanel/server.go`.

---

## 12. Deployment & Operations

- **Container**: Multi-stage [Dockerfile](Dockerfile) (`golang:1.26-alpine` builder with `CGO_ENABLED=0 -ldflags="-w -s"` and `templ@v0.3.1118` -> `alpine:3.22` runtime running as unprivileged user `neptune:65532` with `migrations/` copied in).
- **Compose**: [docker-compose.yml](docker-compose.yml) runs `network_mode: host`, mounts `./data:/app/data`, loads `.env`, and rotates JSON logs (`10m`, max `3` files).
- **Performance & Overhead**: SQLite WAL mode with single-writer connection (`MaxOpenConns(1)`), 60s in-memory regex/keyword cache, ring-buffered SSE logs (`500` entries), and body-size caps (`5 MiB` GitHub webhook, `32 KiB` login, `4 KiB` report resolve) keep memory and CPU footprint minimal for a 1C1G VPS.

```bash
# Initial VPS bootstrap (as root)
ssh root@server "bash -s" < deploy/setup.sh

# Manual update on VPS
ssh root@server "cd /opt/neptune && git pull && docker compose up --build -d"

# Register webhook and sync commands
BOT_TOKEN=token ./deploy/register-webhook.sh bot.example.com secret
```

See [DEPLOY.md](DEPLOY.md) for reverse-proxy setup (Nginx/Caddy) and GitHub Actions secrets (`DEPLOY_HOST`, `DEPLOY_USER`, `VPS_SSH_KEY`).
