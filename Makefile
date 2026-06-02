.PHONY: build run dev test lint lint-fix clean generate deploy deploy-full setup migrate e2e help

# Build variables
BINARY_NAME=neptune
BUILD_DIR=./bin
CMD_DIR=./cmd/neptune
DEPLOY_HOST?=user@server
DEPLOY_DIR?=/opt/neptune

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet

# Build the application
build:
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(CMD_DIR)

# Run the application
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Development with hot reload (requires air)
dev:
	air

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Lint the code (requires golangci-lint)
lint:
	golangci-lint run ./...

# Fix lint issues
lint-fix:
	golangci-lint run --fix ./...

# Vet the code
vet:
	$(GOVET) ./...

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Generate templ files (requires templ)
generate:
	templ generate

# Format code
fmt:
	$(GOCMD) fmt ./...

# Build for production (Linux amd64, no CGO)
build-prod:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags="-w -s" $(CMD_DIR)

# Deploy binary + migrations to server
deploy: build-prod
	rsync -avz --delete $(BUILD_DIR)/$(BINARY_NAME) $(DEPLOY_HOST):$(DEPLOY_DIR)/
	rsync -avz --delete migrations/ $(DEPLOY_HOST):$(DEPLOY_DIR)/migrations/
	rsync -avz deploy/ $(DEPLOY_HOST):$(DEPLOY_DIR)/deploy/
	ssh $(DEPLOY_HOST) "sudo systemctl restart neptune"

# Full deploy: binary + migrations + data migration (D1 → SQLite)
deploy-full: deploy
	./deploy/migrate-d1-to-sqlite.sh
	./deploy/migrate-r2-captcha.sh

# Initial server setup (run once on new VPS)
setup:
	ssh $(DEPLOY_HOST) "bash -s" < deploy/setup.sh

# Register Telegram webhook
webhook:
	./deploy/register-webhook.sh $(DOMAIN) $(WEBHOOK_SECRET)

# Run e2e tests
e2e:
	./deploy/e2e-test.sh $(BASE_URL)

# Database migrations
migrate:
	$(GOCMD) run $(CMD_DIR) -migrate

# Show help
help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  build          Build the application"
	@echo "  run            Build and run the application"
	@echo "  dev            Run with hot reload (requires air)"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  lint           Lint the code (requires golangci-lint)"
	@echo "  lint-fix       Fix lint issues"
	@echo "  vet            Vet the code"
	@echo "  tidy           Tidy dependencies"
	@echo "  clean          Clean build artifacts"
	@echo "  generate       Generate templ files"
	@echo "  fmt            Format code"
	@echo ""
	@echo "Deployment:"
	@echo "  build-prod     Build for production (Linux amd64)"
	@echo "  deploy         Deploy binary + migrations to server"
	@echo "  deploy-full    Deploy + migrate D1 data + R2 captcha"
	@echo "  setup          Initial server setup (run once)"
	@echo "  webhook        Register Telegram webhook"
	@echo "  e2e            Run end-to-end tests"
	@echo "  migrate        Run database migrations"
	@echo ""
	@echo "Variables:"
	@echo "  DEPLOY_HOST    SSH host (default: user@server)"
	@echo "  DEPLOY_DIR     Remote dir (default: /opt/neptune)"
	@echo "  DOMAIN         Bot domain for webhook"
	@echo "  WEBHOOK_SECRET Webhook secret token"
	@echo "  BASE_URL       Base URL for e2e tests"
