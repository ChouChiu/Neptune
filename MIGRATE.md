# MIGRATE.md — Neptune: TypeScript → Go 迁移计划

> **源项目**: kazumi_group_bot (Cloudflare Workers + TypeScript + grammy)
> **目标**: Go 单二进制部署 (Debian VPS 4c4g)
> **日期**: 2026-06-02

---

## 一、技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | Go 1.22+ | 静态类型、单二进制、优秀并发 |
| Bot 框架 | `github.com/go-telegram/bot` v1.21.0 | 零依赖、Bot API 10.0、原生 webhook + middleware |
| HTTP 路由 | `github.com/go-chi/chi/v5` | 与 `net/http` 完全兼容、轻量可组合 |
| 数据库 | SQLite (`modernc.org/sqlite`) | 纯 Go 无 CGO、与 D1 语法兼容、单文件 |
| 模板引擎 | `github.com/a-h/templ` v0.3+ | 编译时类型安全、Go 原生组件 |
| UI 组件库 | templUI | shadcn 风格、Tailwind CSS、copy-paste |
| 前端交互 | HTMX 2.x | 零 JS 框架、服务端渲染 |
| CSS | Tailwind CSS v4 | CDN 引入（开发）/ CLI 编译（生产） |
| 繁简转换 | `github.com/liuzl/gocc` | Go 原生繁简中文转换 |
| 部署 | Debian VPS + systemd + Nginx | 单二进制 + SQLite 文件 |

**不使用 ORM。** 当前项目全部使用原生 SQL（38 个查询函数），Go 的 `database/sql` 标准库配合 prepared statements 已足够。

---

## 二、项目布局

遵循 Go 官方 Server Project 布局。`internal/` 由编译器强制私有，不使用 `pkg/`。

```
neptune/
├── go.mod
├── go.sum
├── Makefile                          # dev, build, generate, lint, deploy
├── cmd/
│   └── neptune/
│       └── main.go                   # 入口：HTTP server + bot 初始化
├── internal/
│   ├── bot/
│   │   ├── bot.go                    # Bot 创建与 handler 注册
│   │   └── middleware.go             # 全局中间件（日志、recovery）
│   ├── handler/
│   │   ├── admin.go                  # /id, /connect, /switch + callback
│   │   ├── help.go                   # /help
│   │   ├── ping.go                   # /ping
│   │   ├── rule.go                   # /rule
│   │   ├── welcome.go                # /setwelcome, /enablewelcome, /disablewelcome, new_chat_members
│   │   ├── verify.go                 # /start verify*, rule_ack callback, sendCaptcha, restrictUser
│   │   ├── captcha_handler.go        # handleCaptchaReply (DM 验证码回复)
│   │   ├── keywords.go               # /addkeyword, /addregex, /listkeywords, /removekeyword, handleKeywordMatch
│   │   ├── ai_chat.go                # MiMo API、上下文、技能匹配、shouldTriggerAi
│   │   ├── votekick.go               # /enablevotekick, /disablevotekick, /kick, vk:* callback
│   │   ├── report.go                 # /report
│   │   ├── warn.go                   # /warn
│   │   └── orchestrator.go           # 消息分发：DM→captcha, group→AI→keywords
│   ├── adminpanel/
│   │   ├── server.go                 # Chi router 路由注册
│   │   ├── auth.go                   # Telegram Login Widget + HMAC session
│   │   ├── middleware.go             # Session 验证中间件
│   │   ├── handler/
│   │   │   ├── dashboard.go          # 仪表盘
│   │   │   ├── reports.go            # 举报管理
│   │   │   └── warnings.go           # 警告管理
│   │   └── components/
│   │       ├── layout.templ          # 基础布局（sidebar + topbar）
│   │       ├── login.templ           # Telegram Login Widget
│   │       ├── dashboard.templ       # 仪表盘
│   │       ├── reports.templ         # 举报列表/操作
│   │       ├── warnings.templ        # 警告列表
│   │       └── common.templ          # 通用组件（toast, table, pagination）
│   ├── github/
│   │   └── release.go                # GitHub webhook + GFM→MarkdownV2
│   ├── db/
│   │   ├── db.go                     # SQLite 连接（WAL, busy_timeout, foreign_keys）
│   │   ├── schema.go                 # Schema 管理与迁移执行
│   │   └── queries.go                # 38 个查询函数
│   ├── model/
│   │   └── model.go                  # 数据 struct 定义
│   └── util/
│       ├── captcha.go                # BMP 验证码生成（5x7 dot-matrix font）
│       ├── markdown.go               # MarkdownV2 转义 + placeholder store + GFM 转换
│       ├── nickname.go               # getNickname
│       ├── permission.go             # checkAdminPermission, checkGroupAdmin
│       ├── placeholder.go            # {nickname}, {userid}, {groupname} 替换
│       ├── reply.go                  # replyOptions helper
│       ├── sendqueue.go              # 限流发送队列（channel + goroutine）
│       ├── chinese.go                # 繁简中文转换
│       └── time.go                   # currentTimestamp
├── migrations/
├── static/                           # Tailwind 编译输出
└── data/
    ├── neptune.db                    # SQLite 数据库文件
    └── captcha/                      # 验证码图片
```

---

## 三、并发模型：goroutine + channel（非轮询）

所有异步/队列模式统一使用 goroutine + channel，不使用轮询循环。

### 3.1 发送限流队列

原 TS 使用 `sendQueue` 数组 + `processing` flag + `setTimeout` 轮询循环。Go 版改用带缓冲 channel + 单 worker goroutine，阻塞在 channel receive 上（零 CPU 空闲）：

```go
// internal/util/sendqueue.go
package util

import (
	"log/slog"
	"time"
)

const (
	sendInterval    = time.Second
	maxSendQueueCap = 10
)

type sendJob struct {
	run    func() error
	result chan string // "sent" | "failed"
}

type SendQueue struct {
	ch chan sendJob
}

func NewSendQueue() *SendQueue {
	sq := &SendQueue{ch: make(chan sendJob, maxSendQueueCap)}
	go sq.worker()
	return sq
}

// worker 从 channel 阻塞读取，处理后 sleep 限流。
// channel 关闭时 goroutine 自动退出。
func (sq *SendQueue) worker() {
	for job := range sq.ch {
		err := job.run()
		if err != nil {
			slog.Error("Telegram send error", "err", err)
			job.result <- "failed"
		} else {
			job.result <- "sent"
		}
		time.Sleep(sendInterval)
	}
}

// Enqueue 入队。队列满时返回 "dropped"，不阻塞调用方。
func (sq *SendQueue) Enqueue(run func() error) string {
	resultCh := make(chan string, 1)
	select {
	case sq.ch <- sendJob{run: run, result: resultCh}:
		return <-resultCh
	default:
		return "dropped"
	}
}
```

### 3.2 Typing 指示器

原 TS 使用 `setInterval` 定时刷新。Go 版改用 goroutine + `select` on context cancel：

```go
func startTypingIndicator(b *bot.Bot, chatID int64) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// 立即发送一次
		_, _ = b.SendChatAction(ctx, &bot.SendChatActionParams{
			ChatID: chatID, Action: "typing",
		})
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = b.SendChatAction(ctx, &bot.SendChatActionParams{
					ChatID: chatID, Action: "typing",
				})
			}
		}
	}()

	return cancel
}
```

### 3.3 重试退避

`ai_chat` 和 `github_release` 中的重试延迟使用 `time.Sleep`（顺序执行，非轮询），无需改动。

---

## 四、Cloudflare 特有组件替代方案

| 原组件 | Go 替代 | 实现细节 |
|--------|---------|---------|
| D1 (SQLite) | `modernc.org/sqlite` | `database/sql` + prepared statements, WAL mode |
| R2 (验证码 BMP) | 本地文件系统 | `data/captcha/{groupId}/{userId}.bmp`，cron 清理 >24h |
| KV (AI 上下文) | SQLite 表 | `kv` 表，key=`ai:context:{groupId}`，带 expires_at |
| KV (分布式锁) | `sync.Mutex` | 单进程部署无需分布式锁 |
| Workers fetch | Chi HTTP router | `chi.Mux` + `http.ListenAndServe` |
| waitUntil | goroutine | `go processAiChat(...)` 后台执行 |
| Web Crypto API | `crypto/hmac` + `crypto/sha256` | 标准库完全等效 |

**新增 SQLite 表**（原 schema.sql 缺失，需补入）：

```sql
-- 原 migration 003 中存在但 schema.sql 遗漏
CREATE TABLE IF NOT EXISTS ai_chat_usage (
  user_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  date TEXT NOT NULL,
  count INTEGER DEFAULT 1,
  PRIMARY KEY (user_id, group_id, date)
);

-- KV 替代
CREATE TABLE IF NOT EXISTS kv (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_kv_expires ON kv(expires_at);
```

---

## 五、迁移步骤（6 阶段）

### 阶段 0：项目初始化与 DB 层

| # | 任务 | 产出 |
|---|------|------|
| 0.1 | `go mod init github.com/your-org/neptune` | go.mod |
| 0.2 | 安装依赖：`go-telegram/bot`, `modernc.org/sqlite`, `chi/v5`, `a-h/templ` | go.sum |
| 0.3 | 创建目录骨架 | 目录结构 |
| 0.4 | 配置 Makefile（dev/build/generate/lint 目标） | Makefile |
| 0.5 | 安装工具链：`templ`, `golangci-lint`, `air` | 开发环境 |
| 0.6 | `internal/model/model.go` — 将 types.ts 转为 Go struct | 数据模型 |
| 0.7 | `internal/db/db.go` — SQLite 连接 | DB 初始化 |
| 0.8 | 复制 schema.sql + migrations + 补充缺失表 | Schema |
| 0.9 | `internal/db/queries.go` — 38 个查询函数 | 查询层 |

**关键注意事项**：

- `addPendingVerification` 使用 `ON CONFLICT ... DO UPDATE`（UPSERT），确认驱动支持
- `acquireLock` 的条件更新 `ON CONFLICT(name) DO UPDATE SET ... WHERE locks.expires_at < ?` 需精确翻译
- `addVoteRecord` 通过捕获 UNIQUE 约束错误返回 false，Go 中检查 `sqlite3.ErrConstraintUnique`
- `incrementAiUsage` 的 `ON CONFLICT ... DO UPDATE SET count = count + 1 RETURNING count` 需确认驱动支持 `RETURNING`；否则改为两步操作
- 所有时间戳使用 `time.Now().Unix()`（秒级），与原 `currentTimestamp()` 一致

### 阶段 1：核心共享层

| # | 任务 | 原文件 | 复杂度 |
|---|------|--------|--------|
| 1.1 | `util/time.go` | `time.ts` (3 行) | ★ |
| 1.2 | `util/nickname.go` | `nickname.ts` (8 行) | ★ |
| 1.3 | `util/placeholder.go` | `placeholders.ts` (20 行) | ★ |
| 1.4 | `util/reply.go` | `reply.ts` (18 行) | ★ |
| 1.5 | `util/chinese.go` | `chinese-conv` (npm) | ★★ |
| 1.6 | `util/permission.go` | `permissions.ts` (44 行) | ★★ |
| 1.7 | `util/sendqueue.go` | `telegram-send-queue.ts` (67 行) | ★★ |
| 1.8 | `util/markdown.go` | `markdown.ts` (118 行) | ★★★ |
| 1.9 | `util/captcha.go` | `captcha.ts` (244 行) | ★★★★ |

**captcha.go 关键细节**：

- 30 个字符的 5x7 点阵 bitmap 数据必须逐字节复制（`FONT` map）
- BMP 文件头 54 字节 + 24-bit 像素 + 4 字节对齐行（`rowSize = ceil(width*3/4)*4`）
- 像素坐标 bottom-up 映射：`srcIdx = ((height-1-y)*width + x) * 3`
- R2 缓存 → 本地文件 `data/captcha/cache/meta.json` + `captcha.bmp`

**markdown.go 关键细节**：

- `MDV2_SPECIAL` 正则包含 `!`（Telegram 保留字符）
- placeholder store 使用零宽空格 `\u200B` 作为 token
- `formatGeneratedMarkdownV2` 处理顺序：code block → inline code → link → heading → bold → italic → list marker → final escape
- 列表标记 `-`/`*`/`+` → `⦁` 必须在最终 escape 之前执行

### 阶段 2：Bot 框架与简单命令

| # | 任务 | 说明 |
|---|------|------|
| 2.1 | `bot/bot.go` | 创建 Bot，注册 handler |
| 2.2 | `bot/middleware.go` | 日志、recovery、群组自动初始化 |
| 2.3 | `handler/ping.go` | /ping → "Pong!" |
| 2.4 | `handler/help.go` | /help → 帮助文本 |
| 2.5 | `handler/admin.go` | /id, /connect, /switch + switch: callback |
| 2.6 | `cmd/neptune/main.go` | HTTP server + webhook + graceful shutdown |

**go-telegram/bot handler 注册**：

```go
// 命令
b.RegisterHandler(bot.HandlerTypeMessageText, "ping", bot.MatchTypeCommand, pingHandler)
b.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommand, startHandler)

// Callback（前缀匹配）
b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "switch:", bot.MatchTypePrefix, switchCallback)
b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "vk:", bot.MatchTypePrefix, votekickCallback)
b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "rule_ack:", bot.MatchTypePrefix, ruleAckCallback)

// 消息编排器作为 default handler
b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypeContains, orchestratorHandler)
```

### 阶段 3：群组管理功能

| # | 任务 | 原文件 | 复杂度 |
|---|------|--------|--------|
| 3.1 | `handler/welcome.go` | `welcome/` (3 文件) | ★★★ |
| 3.2 | `handler/verify.go` | `verify/handlers.ts` (264 行) | ★★★★ |
| 3.3 | `handler/captcha_handler.go` | `verify/captcha-handler.ts` (118 行) | ★★★ |
| 3.4 | `handler/rule.go` | `rule/` (2 文件) | ★★ |
| 3.5 | `handler/keywords.go` | `keywords/` (3 文件, 138 行) | ★★★★ |
| 3.6 | `handler/votekick.go` | `votekick/` (4 文件) | ★★★ |
| 3.7 | `handler/warn.go` | `warn/` (2 文件) | ★★ |
| 3.8 | `handler/report.go` | `report/` (2 文件) | ★★ |

**verify.go 关键细节**：

- `restrictUser()` 需设置 14 个权限字段全部 false/true
- `rule_ack` callback 中 `showTime` 作为 nonce 防重放，`rule_ack_done` flag 防双重确认
- `handleCaptchaReply` 中对所有 pending verifications 批量递增 attempts（`WHERE user_id = ? AND expires_at > ?`）
- 验证码尝试上限 5 次，超限删除所有 pending verification 并提示重新加入

**keywords.go 关键细节**：

- `keywordCache` 用 `sync.Map` 或带 `sync.RWMutex` 的普通 map（模块级缓存，60s TTL）
- 正则匹配 4 种组合：原文、简体原文、简体正则对原文、简体正则对简体文
- plain 匹配仅测试简体包含
- `/addregex` 需 ReDoS 防护：在 1000 字符字符串上运行，超过 100ms 拒绝

### 阶段 4：高级功能

| # | 任务 | 原文件 | 复杂度 |
|---|------|--------|--------|
| 4.1 | `handler/ai_chat.go` | `ai-chat/` (6 文件, 610 行) | ★★★★★ |
| 4.2 | `handler/orchestrator.go` | `message-orchestrator.ts` (34 行) | ★★ |
| 4.3 | `github/release.go` | `github-release/` (2 文件, 194 行) | ★★★★ |

**ai_chat.go 关键细节**：

- MiMo API：`POST https://token-plan-sgp.xiaomimimo.com/v1/chat/completions`，model `mimo-v2.5`
- Header 用 `api-key`（非 `Authorization: Bearer`）
- 重试：429/5xx 最多 2 次，指数退避 1s/2s，超时 25s（`context.WithTimeout`）
- 上下文：KV → SQLite `kv` 表，key=`ai:context:{groupId}`，7 天窗口 + 最大 50 条
- 每日限额：15 次/用户/天，管理员豁免
- 分布式锁：`locks` 表 → `sync.Mutex`（单进程）
- `shouldTriggerAi()` 系统消息关键词过滤：`["验证","欢迎","命令","踢人","投票","群规","关键词","Pong"]`
- `sendAiReply()` 先 MarkdownV2 失败回退纯文本，经 `SendQueue.Enqueue` 限流
- `systemPromptToText()` 递归格式化 `system-prompt.json` Neptune 人设
- `matchSkills()` 按触发关键词大小写不敏感匹配

**orchestrator.go**：

- 注册为 `bot.WithDefaultHandler`（catch-all）
- 私聊：`handleCaptchaReply()` → 命中 return
- 群聊：`handleAiChat()` → 命中 return → 否则 `handleKeywordMatch()`

**github/release.go 关键细节**：

- `verifySignature()`：Web Crypto API → `crypto/hmac`
- `convertGfmToMarkdownV2()` 顺序：normalize CRLF → strip callouts → fenced code → inline code → remove images → protect links → headings→bold → list→`⦁` → bold→`*` → blockquotes→`>` → strip HTML → final escape
- `sendToTelegram()` 直接调 Telegram API（非 bot 框架），3 次重试

### 阶段 5：Admin Panel（Chi + Templ + HTMX）

| # | 任务 | 说明 |
|---|------|------|
| 5.1 | `adminpanel/server.go` | Chi router 注册所有 admin 路由 |
| 5.2 | `adminpanel/auth.go` | Telegram Login Widget 验证 + HMAC session cookie |
| 5.3 | `adminpanel/middleware.go` | Chi 中间件：cookie→验证→注入 userId |
| 5.4 | `adminpanel/components/layout.templ` | sidebar + topbar + 主内容区 |
| 5.5 | `adminpanel/components/login.templ` | Telegram Login Widget 嵌入 |
| 5.6 | `adminpanel/components/dashboard.templ` | 统计卡片 |
| 5.7 | `adminpanel/handler/reports.go` + `reports.templ` | HTMX 列表加载 + 审批/驳回 |
| 5.8 | `adminpanel/handler/warnings.go` + `warnings.templ` | 警告列表 |
| 5.9 | `adminpanel/components/common.templ` | toast, table, pagination |

**Chi 路由**：

```go
r := chi.NewRouter()
r.Use(adminPanelAuthMiddleware)
r.Get("/admin", serveHTML)
r.Post("/admin/auth/login", handleLogin)
r.Get("/admin/auth/me", handleMe)
r.Route("/admin/api", func(r chi.Router) {
    r.Get("/reports", listReports)
    r.Post("/reports/{id}/resolve", resolveReport)
    r.Get("/warnings", listWarnings)
})
```

**HTMX 交互**：

```html
<!-- 筛选 -->
<button hx-get="/admin/api/reports?status=pending"
        hx-target="#report-list" hx-swap="innerHTML">待处理</button>

<!-- 审批 -->
<button hx-post="/admin/api/reports/123/resolve"
        hx-vals='{"status":"approved"}'
        hx-target="closest tr" hx-swap="delete">通过</button>
```

### 阶段 6：部署与收尾

| # | 任务 | 说明 |
|---|------|------|
| 6.1 | 数据迁移 | `wrangler d1 export` → 导入 SQLite |
| 6.2 | R2 验证码迁移 | 下载到 `data/captcha/` |
| 6.3 | Makefile deploy 目标 | rsync + systemctl restart |
| 6.4 | systemd service | `/etc/systemd/system/neptune.service` |
| 6.5 | Nginx 反向代理 | HTTPS + 路由 |
| 6.6 | Telegram webhook 注册 | setWebhook API 调用 |
| 6.7 | 端到端测试 | 21 个命令逐一验证 |
| 6.8 | 文档更新 | README, AGENTS.md |

---

## 六、部署架构

```
┌──────────────────────────────────────┐
│          Nginx (:443 SSL)            │
│  /webhook        → 127.0.0.1:8080    │
│  /admin/*        → 127.0.0.1:8080    │
│  /github-webhook → 127.0.0.1:8080    │
└──────────────┬───────────────────────┘
               │
┌──────────────▼───────────────────────┐
│       neptune (Go binary)            │
│  Chi HTTP :8080                      │
│  ├── go-telegram/bot (webhook)       │
│  ├── Admin Panel (Templ + HTMX)      │
│  └── GitHub webhook                  │
│                                      │
│  SQLite: /opt/neptune/data/neptune.db│
│  Captcha: /opt/neptune/data/captcha/ │
└──────────────────────────────────────┘
```

**systemd** (`/etc/systemd/system/neptune.service`):

```ini
[Unit]
Description=Neptune Telegram Bot
After=network.target

[Service]
Type=simple
User=neptune
WorkingDirectory=/opt/neptune
ExecStart=/opt/neptune/neptune
Restart=always
RestartSec=5
EnvironmentFile=/opt/neptune/.env

[Install]
WantedBy=multi-user.target
```

**Nginx**:

```nginx
server {
    listen 443 ssl http2;
    server_name bot.example.com;
    ssl_certificate     /etc/letsencrypt/live/bot.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/bot.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 七、风险与应对

| 风险 | 应对 |
|------|------|
| `ON CONFLICT ... RETURNING` 兼容性 | 确认 `modernc.org/sqlite` 支持；否则改为两步操作 |
| `acquireLock` 条件更新语义 | 编写单元测试精确验证 |
| 繁简转换库差异 | 准备 100+ 繁简对照词组对比测试 |
| 验证码 BMP 像素一致性 | FONT bitmap 逐字节复制 + 像素对比测试 |
| MarkdownV2 转义边界 | 覆盖所有特殊字符的单元测试 |
| 单进程 SQLite 并发 | WAL 模式 + bot 写入频率极低，无压力 |
| 验证码文件磁盘增长 | cron 定时清理 >24h 文件 |
| go-telegram/bot 与 grammy 差异 | 逐 handler 对比测试，关注嵌套字段映射 |

---

## 八、工具使用计划

| 环节 | 工具 | 用途 |
|------|------|------|
| Go 代码编写 | `golang-pro` Skill | 并发模式、错误处理、接口设计 |
| go-telegram/bot API | Context7 (`/go-telegram/bot`) | handler/middleware/webhook 代码示例 |
| Chi router | Context7 + Web Search | 路由分组、中间件链 |
| Templ 开发 | `templui` Skill + Web Search | .templ 语法、组件生成、Tailwind 集成 |
| SQLite | Web Search | WAL、RETURNING 子句 |
| Cloudflare 导出 | `wrangler` Skill + `cloudflare` Skill | D1 数据导出 |
| 静态分析 | `golangci-lint` | goroutine 泄漏、错误未处理 |
| 单元测试 | `go test` + `testify` | markdown、captcha、keywords、chinese |
| 热重载 | `air` | 文件变更自动重编译 |
| 调试 | `dlv` | verify、ai_chat 复杂流程 |
| 部署 | Makefile + rsync + systemctl | 构建→上传→重启 |

---

## 九、代码映射速查表

| 原 TS | Go | 行数 | 复杂度 | 阶段 |
|-------|-----|------|--------|------|
| `types.ts` | `model/model.go` | 103 | ★★ | 0 |
| `schema.sql` | `db/schema.go` | 93 | ★ | 0 |
| `queries.ts` | `db/queries.go` | 641 | ★★★ | 0 |
| `time.ts` | `util/time.go` | 3 | ★ | 1 |
| `nickname.ts` | `util/nickname.go` | 8 | ★ | 1 |
| `placeholders.ts` | `util/placeholder.go` | 20 | ★ | 1 |
| `reply.ts` | `util/reply.go` | 18 | ★ | 1 |
| `chinese-conv` | `util/chinese.go` | — | ★★ | 1 |
| `permissions.ts` | `util/permission.go` | 44 | ★★ | 1 |
| `telegram-send-queue.ts` | `util/sendqueue.go` | 67 | ★★ | 1 |
| `markdown.ts` | `util/markdown.go` | 118 | ★★★ | 1 |
| `captcha.ts` | `util/captcha.go` | 244 | ★★★★ | 1 |
| `bot.ts` + `features/index.ts` | `bot/bot.go` | 90 | ★★ | 2 |
| `ping/` | `handler/ping.go` | 15 | ★ | 2 |
| `help/` | `handler/help.go` | 50 | ★ | 2 |
| `admin/` | `handler/admin.go` | 100 | ★★ | 2 |
| `index.ts` | `cmd/neptune/main.go` | 170 | ★★★ | 2 |
| `welcome/` | `handler/welcome.go` | 120 | ★★★ | 3 |
| `verify/handlers.ts` | `handler/verify.go` | 264 | ★★★★ | 3 |
| `verify/captcha-handler.ts` | `handler/captcha_handler.go` | 118 | ★★★ | 3 |
| `rule/` | `handler/rule.go` | 60 | ★★ | 3 |
| `keywords/` | `handler/keywords.go` | 188 | ★★★★ | 3 |
| `votekick/` | `handler/votekick.go` | 150 | ★★★ | 3 |
| `warn/` | `handler/warn.go` | 40 | ★★ | 3 |
| `report/` | `handler/report.go` | 40 | ★★ | 3 |
| `ai-chat/` (6 文件) | `handler/ai_chat.go` | 610 | ★★★★★ | 4 |
| `message-orchestrator.ts` | `handler/orchestrator.go` | 34 | ★★ | 4 |
| `github-release/` | `github/release.go` | 194 | ★★★★ | 4 |
| `admin-panel/` (15 文件) | `adminpanel/` | 500+ | ★★★★★ | 5 |

---

## 十、开发顺序

```
Week 1      阶段 0  项目骨架 + DB 层
Week 1-2    阶段 1  共享工具层
Week 2-3    阶段 2  Bot 框架 + 简单命令
Week 3-5    阶段 3  群组管理功能
Week 5-6    阶段 4  AI 对话 + GitHub webhook
Week 6-7    阶段 5  Admin Panel
Week 7-8    阶段 6  部署、数据迁移、端到端测试
```

原则：从底层到上层，从简单到复杂。每阶段完成后有可运行的增量产出。

---

## 十一、验证标准

| 阶段 | 验证 |
|------|------|
| 0 | `go build ./...` 编译通过；DB 查询单元测试通过 |
| 1 | `go test ./internal/util/...` 全通过；captcha BMP 可查看 |
| 2 | 本地启动 bot，响应 /ping /help /id |
| 3 | 测试群：入群→欢迎→验证码→验证通过→发言；关键词触发；投票流程 |
| 4 | @bot 对话正常；GitHub test release → 频道收到消息 |
| 5 | 浏览器 /admin → 登录 → 报告列表 → 审批 |
| 6 | VPS 部署，所有命令在线验证 |

---

## 十二、进度跟踪

### 阶段 0：项目初始化与 DB 层 ✅ 已完成

**完成日期**: 2026-06-02

| # | 任务 | 状态 | 备注 |
|---|------|------|------|
| 0.1 | `go mod init github.com/kazumi-group/neptune` | ✅ | go.mod 已创建 |
| 0.2 | 安装依赖：`go-telegram/bot`, `modernc.org/sqlite`, `chi/v5`, `a-h/templ` | ✅ | go.sum 已生成 |
| 0.3 | 创建目录骨架 | ✅ | cmd/, internal/, migrations/, static/, data/ |
| 0.4 | 配置 Makefile（dev/build/generate/lint 目标） | ✅ | 12 个 make 目标 |
| 0.5 | 安装工具链：`templ`, `golangci-lint`, `air` | ⏳ | 待安装 |
| 0.6 | `internal/model/model.go` — 将 types.ts 转为 Go struct | ✅ | 14 个 struct |
| 0.7 | `internal/db/db.go` — SQLite 连接 | ✅ | WAL mode, busy_timeout |
| 0.8 | 复制 schema.sql + migrations + 补充缺失表 | ✅ | 包含 ai_chat_usage, kv 表 |
| 0.9 | `internal/db/queries.go` — 38 个查询函数 | ✅ | 42 个函数（含兼容包装） |

**验证结果**:
- [x] `go build ./...` 编译通过
- [ ] DB 查询单元测试（待编写）

**下一步**: 阶段 1 — 核心共享层

### 阶段 1：核心共享层 ✅ 已完成

**完成日期**: 2026-06-02

| # | 任务 | 状态 | 备注 |
|---|------|------|------|
| 1.1 | `util/time.go` | ✅ | CurrentTimestamp() |
| 1.2 | `util/nickname.go` | ✅ | GetNickname() |
| 1.3 | `util/placeholder.go` | ✅ | ReplacePlaceholders() |
| 1.4 | `util/reply.go` | ✅ | ReplyOptions() |
| 1.5 | `util/chinese.go` | ✅ | ToSimplified() / ToTraditional() |
| 1.6 | `util/permission.go` | ✅ | CheckAdminPermission() / CheckGroupAdmin() |
| 1.7 | `util/sendqueue.go` | ✅ | SendQueue with channel + goroutine |
| 1.8 | `util/markdown.go` | ✅ | MarkdownV2 转义 + GFM 转换 |
| 1.9 | `util/captcha.go` | ✅ | BMP 验证码生成 + 本地文件存储 |

**验证结果**:
- [x] `go build ./...` 编译通过
- [ ] `go test ./internal/util/...` 全通过（待编写）
- [ ] captcha BMP 可查看（待验证）

**下一步**: 阶段 2 — Bot 框架与简单命令

### 阶段 2：Bot 框架与简单命令 ✅ 已完成

**完成日期**: 2026-06-02

| # | 任务 | 状态 | 备注 |
|---|------|------|------|
| 2.1 | `bot/bot.go` | ✅ | New() 创建 Bot，注册 handler + middleware |
| 2.2 | `bot/middleware.go` | ✅ | loggingMiddleware, recoveryMiddleware, groupInitMiddleware |
| 2.3 | `handler/ping.go` | ✅ | /ping → "🏓 Pong!" |
| 2.4 | `handler/help.go` | ✅ | /help → 完整帮助文本 |
| 2.5 | `handler/admin.go` | ✅ | /id, /connect, /switch + switch: callback |
| 2.6 | `cmd/neptune/main.go` | ✅ | HTTP server + webhook + graceful shutdown |

**验证结果**:
- [x] `go build ./...` 编译通过
- [x] `go vet ./...` 无警告
- [ ] 本地启动 bot，响应 /ping /help /id（待手动验证）

**下一步**: 阶段 3 — 群组管理功能
