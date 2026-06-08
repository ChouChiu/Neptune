.PHONY: build run dev test lint lint-fix clean generate docker-build docker-up docker-down docker-logs webhook e2e help

# ========== 构建参数 ==========
BINARY_NAME=neptune
BUILD_DIR=./bin
CMD_DIR=./cmd/neptune

# ========== 部署参数 ==========
DOMAIN?=
WEBHOOK_SECRET?=

# ========== 开发命令 ==========

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) -v $(CMD_DIR)

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

dev:
	air

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	go clean
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

generate:
	templ generate

fmt:
	go fmt ./...

# ========== Docker 命令 ==========

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# ========== 部署命令 ==========

# 注册 Telegram webhook
# 用法：make webhook DOMAIN=bot.example.com WEBHOOK_SECRET=你的密钥
webhook:
	@[ -n "$(DOMAIN)" ] || { echo "错误：请指定 DOMAIN，例如 DOMAIN=bot.example.com"; exit 1; }
	@[ -n "$(WEBHOOK_SECRET)" ] || { echo "错误：请指定 WEBHOOK_SECRET"; exit 1; }
	./deploy/register-webhook.sh $(DOMAIN) $(WEBHOOK_SECRET)

# 端到端测试
# 用法：make e2e BASE_URL=https://bot.example.com
e2e:
	@[ -n "$(BASE_URL)" ] || { echo "错误：请指定 BASE_URL，例如 BASE_URL=https://bot.example.com"; exit 1; }
	./deploy/e2e-test.sh $(BASE_URL)

# ========== 帮助 ==========

help:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════╗"
	@echo "║              Neptune 部署助手                            ║"
	@echo "╚══════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "🐳 Docker 部署"
	@echo "──────────────────────────────────────────────────────────"
	@echo "  make docker-build    构建 Docker 镜像"
	@echo "  make docker-up       启动容器（后台运行）"
	@echo "  make docker-down     停止容器"
	@echo "  make docker-logs     查看容器日志"
	@echo ""
	@echo "📦 CI/CD 自动部署"
	@echo "──────────────────────────────────────────────────────────"
	@echo "  推送到 main 分支自动触发 GitHub Actions 部署"
	@echo "  需配置 secrets：DEPLOY_HOST, DEPLOY_USER, VPS_SSH_KEY"
	@echo ""
	@echo "🛠  开发命令"
	@echo "──────────────────────────────────────────────────────────"
	@echo "  make build          编译"
	@echo "  make dev            热重载开发（需要 air）"
	@echo "  make test           运行测试"
	@echo "  make lint           代码检查（需要 golangci-lint）"
	@echo "  make vet            go vet"
	@echo "  make generate       生成 templ 模板（修改 *.templ 后运行）"
	@echo "  make fmt            格式化代码"
	@echo "  make clean          清理构建产物"
	@echo ""
	@echo "📋 部署参数"
	@echo "──────────────────────────────────────────────────────────"
	@echo "  DOMAIN         Bot 域名（如 bot.example.com）"
	@echo "  WEBHOOK_SECRET Webhook 密钥"
	@echo "  BASE_URL       E2E 测试地址"
