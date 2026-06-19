# Contributing to Neptune

本指南帮你快速上手 Neptune 开发：设置环境、提交代码、发起 PR。

## 目录

- [如何贡献](#如何贡献)
- [开发环境](#开发环境)
- [项目架构](#项目架构)
- [开发指南](#开发指南)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [Pull Request 指南](#pull-request-指南)
- [行为准则](#行为准则)
- [License](#license)

## 如何贡献

### 报告 Bug

1. 在 GitHub Issues 中搜索是否已有相同问题
2. 如果没有，创建一个新的 Issue，包含：
   - 清晰的标题和描述
   - 复现步骤
   - 期望行为与实际行为
   - 运行环境信息（OS、Go 版本等）

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

- [Go 1.26+](https://go.dev/dl/)
- [templ](https://templ.dev)（模板代码生成）
- [golangci-lint](https://golangci-lint.run)（静态分析）
- [air](https://github.com/air-verse/air)（可选，热重载）

### 初始化

```bash
git clone https://github.com/ChouChiu/neptune.git
cd neptune
go mod download
cp .env.example .env  # 编辑填入 secrets
make generate         # 生成 templ 模板代码
make build            # 编译
```

### 本地开发

```bash
make dev              # air 热重载（需要安装 air）
make build            # 编译 → ./bin/neptune
make test             # go test ./...
make lint             # golangci-lint
make vet              # go vet
```

## 项目架构

**Go 项目布局**：`internal/` 目录下的包无法被外部模块导入（Go 编译器强制），`cmd/` 为程序入口。

```
cmd/neptune/main.go           # 入口：HTTP server + bot 初始化 + graceful shutdown
internal/
├── bot/bot.go                # Bot 创建、handler 注册
├── bot/middleware.go         # 日志、recovery、群组初始化中间件
├── handler/                  # 所有命令 + 回调 handler
│   ├── orchestrator.go       # 消息分发：DM→验证码, 群组→AI→关键词
│   └── ...                   # 各功能 handler
├── adminpanel/               # Web 管理面板（Chi 路由 + Templ 模板 + HTMX 前端交互）
│   ├── server.go             # Chi 路由注册
│   ├── auth.go               # Telegram Login Widget + HMAC-SHA256 签名验证 + session cookie
│   ├── handler/              # API handler（reports, warnings）
│   └── components/           # Templ 模板
├── github/release.go         # GitHub webhook + GitHub Flavored Markdown→Telegram MarkdownV2 转换
├── db/                       # SQLite 数据库层
├── model/                    # 数据结构体
└── util/                     # 共享工具函数
migrations/                   # SQL 迁移文件
deploy/                       # 部署脚本 + 配置
```

### 数据库

- **SQLite**（驱动：`modernc.org/sqlite`，纯 Go 实现，无需 CGO 编译），使用 WAL（Write-Ahead Logging）模式提升并发性能
- Schema：`internal/db/schema.go` — 启动时自动执行 `ApplySchema()`
- 迁移：`migrations/*.sql` — 通过 `ApplyMigrations()` 按 `schema_migrations` 表追踪
- 查询：`internal/db/queries.go` — 所有 DB 访问通过 `DB` 结构体方法

### 关键依赖

| 依赖 | 用途 |
|------|------|
| `github.com/go-telegram/bot` | Telegram Bot API（webhook + middleware） |
| `github.com/go-chi/chi/v5` | Admin panel HTTP 路由 |
| `github.com/a-h/templ` | 编译时模板引擎（admin panel） |
| `modernc.org/sqlite` | 纯 Go SQLite 驱动 |
| `github.com/liuzl/gocc` | 繁简中文转换 |

## 开发指南

### 添加新功能

1. 在 `internal/handler/` 下创建 `<name>.go`
2. 在 `internal/bot/bot.go` 中注册命令或回调
3. 如果需要新数据库表：
   - 在 `internal/db/schema.go` 中添加表定义
   - 在 `migrations/` 下创建迁移文件（命名格式：`NNN_description.sql`）
   - 在 `internal/db/queries.go` 中添加查询函数
4. 运行 `make generate`（如果有 templ 文件变更）
5. 运行 `make lint && make vet`，确保全部通过
6. 更新 README 命令列表

### 添加管理员命令

使用 `CheckAdminPermission()` 检查权限：

```go
func MyAdminCommand(database *db.DB) handler.HandlerFunc {
    return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
        allowed, groupId, err := util.CheckAdminPermission(database, update)
        if err != nil {
            // 处理数据库错误
            return
        }
        if !allowed {
            // 回复无权限
            return
        }
        // 使用 groupId 执行操作
    }
}
```

### Admin Panel 模块

`internal/adminpanel/` 使用 [Chi](https://github.com/go-chi/chi)（HTTP 路由）+ [Templ](https://templ.dev)（Go 模板引擎）+ [HTMX](https://htmx.org)（前端交互）：

- `server.go` — 路由注册
- `auth.go` — Telegram Login Widget 验证 + HMAC-SHA256 签名校验 + session cookie
- `handler/` — API 端点返回 HTML fragment（Templ 渲染）
- `components/` — Templ 模板

添加新管理面板模块：在 `internal/adminpanel/handler/` 下创建 handler，在 `server.go` 中注册路由。

### Templ 模板

`*.templ` 文件必须先编译才能构建：

```bash
make generate          # templ generate（生成 *_templ.go 文件）
```

生成的 `*_templ.go` 文件已 gitignored，每次修改 `.templ` 文件后需重新生成。

### 数据库迁移

1. 在 `migrations/` 下创建迁移文件，命名格式：`NNN_description.sql`
2. 编写 SQL（参考 `internal/db/schema.go` 中的表结构）
3. 在 `internal/db/queries.go` 中添加对应的查询函数
4. 迁移在启动时自动执行（通过 `ApplyMigrations()`）

> 注意：迁移通过 `schema_migrations` 表追踪，每个文件只执行一次。

## 代码规范

- **Linter**：golangci-lint（不是 ESLint / Biome）
- **Formatter**：`go fmt`（tab 缩进）
- **模板引擎**：templ（不是 JSX / EJS）

提交前运行以下检查，确保全部通过：

```bash
make generate          # 如果修改了 *.templ 文件
make lint              # golangci-lint
make vet               # go vet
go build ./...         # 确认编译通过
```

> CI（`.github/workflows/deploy.yml`）会在推送到 `main` 时自动部署，但本地仍建议手动检查。

## 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 格式 —— 在提交消息前缀一个类型标签，让提交历史一目了然：

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
feat: add group mute command
fix: captcha not auto-kicking user after timeout
docs: update README command list
```

## Pull Request 指南

- PR 标题使用 Conventional Commits 格式
- 一个 PR 只做一件事，保持精简
- 运行 `make lint` 和 `make vet`，确保全部通过
- 如涉及新功能，请在 PR 描述中说明使用方式
- 如涉及数据库变更，请在 `migrations/` 中添加迁移文件
- 如涉及新命令，请同步更新 README 的命令列表
- 如涉及管理面板新模块，请参考 [Admin Panel 模块](#admin-panel-模块) 章节

## 行为准则

本项目采用 [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md)，参与本项目即表示你同意遵守其中的条款。

## License

提交代码即表示你同意你的贡献以 [MIT License](LICENSE) 发布。
