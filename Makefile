# Go-Cache Makefile

.PHONY: test test-integration test-containers test-fast test-redis test-memory test-all build clean coverage lint security help docker-up docker-down docker-test docker-test-single docker-test-fast docker-test-latency docker-reset docker-validate docker-configure-latency

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Integration Test Scenarios:'
	@echo '  test-containers      - Full cold start with containers (Redis)'
	@echo '  test-fast           - Memory-only testing (fastest)'
	@echo '  test-redis          - Real Redis + Memory cache'
	@echo '  test-memory         - Memory provider only'
	@echo '  docker-test         - Run integration tests with docker-compose services'
	@echo '  docker-test-fast    - Run fast tests with docker-compose services'
	@echo ''
	@echo 'Docker Compose:'
	@echo '  docker-up           - Start docker-compose services'
	@echo '  docker-down         - Stop docker-compose services'
	@echo '  docker-reset        - Reset services and volumes'
	@echo '  docker-validate     - Validate services are ready'
	@echo '  docker-test-latency - Run tests with network latency simulation'
	@echo '  docker-configure-latency - Configure toxiproxy latency (set LATENCY_MS)'
	@echo ''
	@echo 'Single Test Execution (set TEST_NAME):'
	@echo '  test-integration-single     - Run single integration test'
	@echo '  docker-test-single          - Run single test with compose services'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Testing targets
test: ## Run unit tests only
	go test -short ./...

test-integration: ## Run integration tests (requires Docker)
	go test -tags=integration ./...

test-integration-single: ## Run single integration test (set TEST_NAME)
	@if [ -z "$(TEST_NAME)" ]; then echo "Usage: make test-integration-single TEST_NAME=TestMemoryCache_BasicOperations"; exit 1; fi
	@echo "🧪 Running $(TEST_NAME) integration test..."
	go test -tags=integration -run=$(TEST_NAME) ./...

# Cold-start container integration testing targets
test-containers: ## Cold start with Redis containers
	@echo "🐳 Running integration tests with container cold start..."
	@echo "This will start fresh Redis containers, run tests, then cleanup"
	GOCACHE_TEST_MODE=containers GOCACHE_TEST_PRESET=production go test -tags=integration -v -timeout=120s ./...

test-fast: ## Fast integration tests (memory only)
	@echo "⚡ Running fast integration tests (memory only)..."
	GOCACHE_TEST_PRESET=fast go test -tags=integration -v -timeout=30s ./...

test-redis: ## Real Redis + memory cache
	@echo "🔧 Running integration tests with real Redis..."
	GOCACHE_TEST_PRESET=realistic go test -tags=integration -v -timeout=60s ./...

test-memory: ## Memory provider only tests
	@echo "🧠 Running memory provider tests..."
	GOCACHE_TEST_PRESET=memory go test -tags=integration -v -timeout=30s ./...

# Specific test targets
test-containers-single: ## Run single test with containers (set TEST_NAME)
	@if [ -z "$(TEST_NAME)" ]; then echo "Usage: make test-containers-single TEST_NAME=TestRedisCache_BasicOperations"; exit 1; fi
	@echo "🐳 Running $(TEST_NAME) with container cold start..."
	GOCACHE_TEST_MODE=containers go test -tags=integration -v -timeout=60s -run=$(TEST_NAME) ./...

test-fast-single: ## Run single test fast (set TEST_NAME)
	@if [ -z "$(TEST_NAME)" ]; then echo "Usage: make test-fast-single TEST_NAME=TestMemoryCache_BasicOperations"; exit 1; fi
	@echo "⚡ Running $(TEST_NAME) fast..."
	GOCACHE_TEST_PRESET=fast go test -tags=integration -v -timeout=30s -run=$(TEST_NAME) ./...

# Build targets
build: ## Build the module
	go build ./...

clean: ## Clean build artifacts
	go clean ./...
	rm -f coverage.out unit_coverage.out

# Coverage targets
coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

unit-coverage: ## Run unit tests with coverage
	go test -short -coverprofile=unit_coverage.out ./...
	go tool cover -html=unit_coverage.out -o unit_coverage.html
	@echo "Unit test coverage report generated: unit_coverage.html"

# Code quality targets
lint: ## Run linter
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run

security: ## Run security scanner
	@command -v gosec >/dev/null 2>&1 || { echo "gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; exit 1; }
	gosec ./...

# Performance targets
benchmark: ## Run benchmarks
	go test -bench=. -benchmem ./...

benchmark-latency: ## Run latency benchmarks
	go test -tags=latency -bench=BenchmarkLatencyComparison -benchmem ./internal/providers/redis/ -v

# Development targets
mod-tidy: ## Tidy go modules
	go mod tidy

mod-download: ## Download go modules
	go mod download

# Docker targets for latency testing
docker-toxiproxy: ## Start Toxiproxy container for manual testing
	@echo "Starting Toxiproxy container..."
	docker run --rm -d --name toxiproxy-test -p 8474:8474 -p 8080:8080 shopify/toxiproxy:2.9.0
	@echo "Toxiproxy started. API available at http://localhost:8474"
	@echo "Stop with: docker stop toxiproxy-test"

docker-redis: ## Start Redis container for manual testing
	@echo "Starting Redis container..."
	docker run --rm -d --name redis-test \
		-p 6379:6379 \
		redis:8.0.3 redis-server --appendonly yes
	@echo "Redis started. Connection: redis://localhost:6379"
	@echo "Stop with: docker stop redis-test"

docker-stop: ## Stop test containers
	@echo "Stopping test containers..."
	-docker stop toxiproxy-test redis-test 2>/dev/null || true

# Docker-compose targets (with service validation)
docker-up: ## Start docker-compose services
	@echo "🐳 Starting docker-compose services..."
	docker compose up -d
	@echo "⏳ Waiting for services to be ready..."
	@cd scripts && go run wait-for-services.go

docker-down: ## Stop and remove docker-compose services
	@echo "🛑 Stopping docker-compose services..."
	docker compose down -v

docker-test: ## Run integration tests with docker-compose services (with validation)
	@echo "🧪 Running integration tests with docker-compose services..."
	GOCACHE_TEST_MODE=compose go test -tags=integration github.com/MichaelAJay/go-cache

docker-test-fast: ## Run fast preset with docker-compose services (with validation)
	@echo "⚡ Running fast integration tests with docker-compose services..."
	GOCACHE_TEST_MODE=compose GOCACHE_TEST_PRESET=fast go test -tags=integration ./...

docker-test-single: ## Run single test with docker-compose services (set TEST_NAME)
	@if [ -z "$(TEST_NAME)" ]; then echo "Usage: make docker-test-single TEST_NAME=TestRedisCache_BasicOperations"; exit 1; fi
	@echo "🧪 Running $(TEST_NAME) with docker-compose services..."
	GOCACHE_TEST_MODE=compose go test -tags=integration -run=$(TEST_NAME) github.com/MichaelAJay/go-cache

docker-test-latency: ## Run integration tests with network latency simulation
	@echo "🐌 Running integration tests with network latency simulation..."
	GOCACHE_TEST_MODE=compose GOCACHE_TEST_LATENCY=enabled go test -tags=integration github.com/MichaelAJay/go-cache

docker-configure-latency: ## Configure toxiproxy latency (set LATENCY_MS)
	@if [ -z "$(LATENCY_MS)" ]; then echo "Usage: make docker-configure-latency LATENCY_MS=50"; exit 1; fi
	@echo "⚡ Configuring toxiproxy latency: $(LATENCY_MS)ms..."
	@cd scripts && GOCACHE_TEST_REDIS_LATENCY_MS=$(LATENCY_MS) go run setup-toxiproxy.go

docker-reset: ## Reset docker-compose volumes and restart fresh
	@echo "🔄 Resetting docker-compose services..."
	docker compose down -v
	docker compose up -d
	@echo "⏳ Waiting for services to be ready..."
	@cd scripts && go run wait-for-services.go

docker-validate: ## Validate docker-compose services are ready
	@echo "✅ Validating docker-compose services..."
	@cd scripts && go run validate-services.go

docker-setup-toxiproxy: ## Setup toxiproxy proxies (run after docker-up)
	@echo "🧪 Setting up toxiproxy proxies..."
	@cd scripts && go run setup-toxiproxy.go

test-all: ## Run all tests including latency tests
	@echo "Running unit tests..."
	go test -short ./...
	@echo "Running integration tests..."
	go test -tags=integration ./...
	@echo "Running latency injection tests..."
	go test -tags=latency -timeout=5m ./internal/providers/redis/ -v