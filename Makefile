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
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ") \
		--build-arg GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		.
	@echo "✅ Docker image built: $(DOCKER_IMAGE):$(VERSION)"

docker-build-no-cache: ## Build Docker image without cache
	@echo "Building Docker image (no cache)..."
	@docker build --no-cache \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ") \
		--build-arg GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		.
	@echo "✅ Docker image built: $(DOCKER_IMAGE):$(VERSION)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8080:8080 \
		-e JWT_SECRET=${JWT_SECRET} \
		-e REDIS_HOST=redis \
		-e USER_SERVICE_ADDRESS=user-service:50051 \
		-e MONOLITH_SERVICE_ADDRESS=hub-monolith:50060 \
		--name hub-api-gateway \
		--rm \
		$(DOCKER_IMAGE):$(VERSION)

docker-stop: ## Stop running Docker container
	@echo "Stopping Docker container..."
	@docker stop hub-api-gateway 2>/dev/null || true
	@echo "✅ Container stopped"

docker-logs: ## View Docker container logs
	@docker logs -f hub-api-gateway

docker-shell: ## Open shell in running container
	@docker exec -it hub-api-gateway /bin/sh

docker-inspect: ## Inspect Docker image
	@docker inspect $(DOCKER_IMAGE):$(VERSION)

docker-size: ## Show Docker image size
	@docker images $(DOCKER_IMAGE):$(VERSION) --format "{{.Repository}}:{{.Tag}} - {{.Size}}"

docker-compose-up: ## Start all services with docker-compose
	@echo "Starting services..."
	@if [ ! -f .env ]; then \
		echo "⚠️  .env file not found. Creating from env.example..."; \
		cp env.example .env; \
		echo "⚠️  Please edit .env and set JWT_SECRET before running again"; \
		exit 1; \
	fi
	@docker compose up -d
	@echo "✅ Services started"
	@echo ""
	@echo "📊 Service Status:"
	@docker compose ps
	@echo ""
	@echo "📡 Gateway: http://localhost:8080"
	@echo "📈 Metrics: http://localhost:8080/metrics"
	@echo "🔍 Health: http://localhost:8080/health"

docker-compose-down: ## Stop all services
	@echo "Stopping services..."
	@docker compose down
	@echo "✅ Services stopped"

docker-compose-logs: ## View docker-compose logs
	@docker compose logs -f

docker-compose-ps: ## Show docker-compose service status
	@docker compose ps

docker-compose-restart: ## Restart docker-compose services
	@echo "Restarting services..."
	@docker compose restart
	@echo "✅ Services restarted"

docker-compose-build: ## Build docker-compose services
	@echo "Building services..."
	@docker compose build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ") \
		--build-arg GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
	@echo "✅ Services built"

docker-clean: ## Clean Docker resources (containers, images, volumes)
	@echo "Cleaning Docker resources..."
	@docker compose down -v
	@docker rmi $(DOCKER_IMAGE):$(VERSION) 2>/dev/null || true
	@docker rmi $(DOCKER_IMAGE):latest 2>/dev/null || true
	@echo "✅ Docker resources cleaned"

docker-prune: ## Remove all unused Docker resources
	@echo "Pruning Docker resources..."
	@docker system prune -af --volumes
	@echo "✅ Docker pruned"

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

