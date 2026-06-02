.PHONY: build run dev test lint lint-fix clean generate deploy setup webhook e2e help

# ========== 构建参数 ==========
BINARY_NAME=neptune
BUILD_DIR=./bin
CMD_DIR=./cmd/neptune

# ========== 部署参数 ==========
# 用法示例：
#   make deploy DEPLOY_HOST=root@your-server DOMAIN=bot.example.com
DEPLOY_HOST?=user@server
DEPLOY_DIR?=/opt/neptune
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

# ========== 部署命令 ==========

build-prod:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags="-w -s" $(CMD_DIR)

# 首次部署：初始化服务器（创建用户、目录、systemd 服务、Nginx 配置）
# 用法：make setup DEPLOY_HOST=root@your-server DOMAIN=bot.example.com
setup:
	@[ -n "$(DEPLOY_HOST)" ] || { echo "错误：请指定 DEPLOY_HOST，例如 DEPLOY_HOST=root@your-server"; exit 1; }
	@[ -n "$(DOMAIN)" ] || { echo "错误：请指定 DOMAIN，例如 DOMAIN=bot.example.com"; exit 1; }
	@echo ""
	@echo "=== 第一步：初始化服务器（创建用户、目录、Nginx、systemd）==="
	NEPTUNE_DOMAIN=$(DOMAIN) ssh $(DEPLOY_HOST) "bash -s" < deploy/setup.sh
	@echo ""
	@echo "=== 第二步：编译并上传二进制 ==="
	$(MAKE) build-prod
	rsync -avz $(BUILD_DIR)/$(BINARY_NAME) $(DEPLOY_HOST):$(DEPLOY_DIR)/
	rsync -avz migrations/ $(DEPLOY_HOST):$(DEPLOY_DIR)/migrations/
	@echo ""
	@echo "✅ 服务器初始化完成！请接下来手动完成："
	@echo ""
	@echo "  ① 编辑服务器配置文件：sudo nano /opt/neptune/.env"
	@echo "  ② 启动服务：sudo systemctl start neptune"
	@echo "  ③ 配置 SSL：sudo certbot certonly --nginx -d $(DOMAIN)"
	@echo "  ④ 注册 webhook：make webhook DOMAIN=$(DOMAIN) WEBHOOK_SECRET=你的密钥"

# 日常部署：编译上传并重启服务
# 用法：make deploy DEPLOY_HOST=user@your-server
deploy: build-prod
	@[ -n "$(DEPLOY_HOST)" ] || { echo "错误：请指定 DEPLOY_HOST，例如 DEPLOY_HOST=user@your-server"; exit 1; }
	rsync -avz --delete $(BUILD_DIR)/$(BINARY_NAME) $(DEPLOY_HOST):$(DEPLOY_DIR)/
	rsync -avz --delete migrations/ $(DEPLOY_HOST):$(DEPLOY_DIR)/migrations/
	rsync -avz deploy/ $(DEPLOY_HOST):$(DEPLOY_DIR)/deploy/
	ssh $(DEPLOY_HOST) "sudo systemctl restart neptune"
	@echo "✅ 部署完成"

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
	@echo "🚀 首次部署（从零开始）"
	@echo "──────────────────────────────────────────────────────────"
	@echo "  1. 在本地准备好 .env 配置文件："
	@echo "       cp .env.example .env    # 然后编辑填入 secrets"
	@echo ""
	@echo "  2. 初始化服务器（创建用户、目录、上传二进制）："
	@echo "       make setup DEPLOY_HOST=root@你的服务器 DOMAIN=你的域名"
	@echo ""
	@echo "  3. SSH 到服务器，编辑 /opt/neptune/.env 填入 secrets"
	@echo ""
	@echo "  4. 在服务器上启动服务："
	@echo "       sudo systemctl enable neptune"
	@echo "       sudo systemctl start neptune"
	@echo ""
	@echo "  5. 配置 SSL 证书："
	@echo "       sudo certbot certonly --nginx -d 你的域名"
	@echo ""
	@echo "  6. 注册 Telegram webhook："
	@echo "       make webhook DOMAIN=你的域名 WEBHOOK_SECRET=你的密钥"
	@echo ""
	@echo "  7. 验证部署："
	@echo "       make e2e BASE_URL=https://你的域名"
	@echo ""
	@echo "📦 日常更新"
	@echo "──────────────────────────────────────────────────────────"
	@echo "  make deploy DEPLOY_HOST=user@你的服务器"
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
	@echo "  DEPLOY_HOST    SSH 地址（如 root@1.2.3.4）"
	@echo "  DOMAIN         Bot 域名（如 bot.example.com）"
	@echo "  WEBHOOK_SECRET Webhook 密钥"
	@echo "  DEPLOY_DIR     服务器目录（默认 /opt/neptune）"
	@echo "  BASE_URL       E2E 测试地址"
