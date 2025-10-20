.PHONY: help build test run docker clean lint fmt vet tidy

# Variables
BINARY_NAME=gateway
DOCKER_IMAGE=hub-api-gateway
VERSION?=latest

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the gateway binary
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) cmd/server/main.go
	@echo "✅ Build complete: bin/$(BINARY_NAME)"

test: ## Run all tests
	@echo "Running tests..."
	@go test -v -cover ./...
	@echo "✅ Tests complete"

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

run: ## Run the gateway locally
	@echo "Starting gateway..."
	@if [ -z "$$JWT_SECRET" ]; then \
		echo "⚠️  JWT_SECRET not set. Loading from .env if available..."; \
		if [ -f .env ]; then \
			export $$(cat .env | grep -v '^#' | xargs) && go run cmd/server/main.go; \
		else \
			echo "❌ JWT_SECRET environment variable is required"; \
			echo "Run: export JWT_SECRET=\"HubInv3stm3nts_S3cur3_JWT_K3y_2024_!@#$%^\""; \
			exit 1; \
		fi \
	else \
		go run cmd/server/main.go; \
	fi

dev: ## Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
	@echo "Starting gateway with hot reload..."
	@air

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(VERSION) .
	@echo "✅ Docker image built: $(DOCKER_IMAGE):$(VERSION)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8080:8080 \
		-e REDIS_HOST=redis \
		-e USER_SERVICE_ADDRESS=user-service:50051 \
		$(DOCKER_IMAGE):$(VERSION)

docker-compose-up: ## Start all services with docker-compose
	@echo "Starting services..."
	@docker-compose up -d
	@echo "✅ Services started"

docker-compose-down: ## Stop all services
	@echo "Stopping services..."
	@docker-compose down
	@echo "✅ Services stopped"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./...
	@echo "✅ Lint complete"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✅ Format complete"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
	@echo "✅ Vet complete"

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	@go mod tidy
	@echo "✅ Tidy complete"

install-tools: ## Install development tools
	@echo "Installing tools..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Tools installed"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "✅ Dependencies downloaded"

all: clean fmt vet lint test build ## Run all checks and build

.DEFAULT_GOAL := help

