.PHONY: build run dev test lint lint-fix clean generate deploy

# Build variables
BINARY_NAME=neptune
BUILD_DIR=./bin
CMD_DIR=./cmd/neptune

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

# Build for production
build-prod:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags="-w -s" $(CMD_DIR)

# Deploy to server (requires rsync and SSH access)
deploy: build-prod
	rsync -avz --delete $(BUILD_DIR)/$(BINARY_NAME) user@server:/opt/neptune/
	rsync -avz --delete migrations/ user@server:/opt/neptune/migrations/
	ssh user@server "sudo systemctl restart neptune"

# Database migrations
migrate:
	$(GOCMD) run $(CMD_DIR) -migrate

# Show help
help:
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Build and run the application"
	@echo "  dev          - Run with hot reload (requires air)"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  lint         - Lint the code (requires golangci-lint)"
	@echo "  lint-fix     - Fix lint issues"
	@echo "  vet          - Vet the code"
	@echo "  tidy         - Tidy dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  generate     - Generate templ files"
	@echo "  fmt          - Format code"
	@echo "  build-prod   - Build for production (Linux)"
	@echo "  deploy       - Deploy to server"
	@echo "  migrate      - Run database migrations"
	@echo "  help         - Show this help"
