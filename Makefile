.PHONY: run build test test-unit test-integration test-all docker-up docker-down docker-clean lint lint-fix

# Application variables
APP_NAME := pr-reviewer-service
DOCKER_COMPOSE := docker-compose
GO := go

# Default target
all: build

# Run the application locally
run:
	@echo "Starting application..."
	cd src && $(GO) run cmd/server/main.go

# Build the application
build:
	@echo "Building application..."
	cd src && $(GO) build -o ../bin/$(APP_NAME) cmd/server/main.go

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	cd src && $(GO) test -v ./internal/service/... -cover

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v ./tests/integration_test.go -tags=integration

# Run all tests
test-all: test-unit test-integration
	@echo "All tests completed"

# Run tests (alias for test-all)
test: test-all

# Start services with Docker Compose
docker-up:
	@echo "Starting services with Docker Compose..."
	$(DOCKER_COMPOSE) up -d

# Stop services with Docker Compose
docker-down:
	@echo "Stopping services..."
	$(DOCKER_COMPOSE) down

# Clean Docker resources
docker-clean: docker-down
	@echo "Cleaning Docker resources..."
	$(DOCKER_COMPOSE) down -v --remove-orphans

# Run linter
lint:
	@echo "Running linter..."
	cd src && golangci-lint run ./...

# Fix linting issues
lint-fix:
	@echo "Fixing linting issues..."
	cd src && golangci-lint run ./... --fix

# Run application with Docker Compose (full setup)
start: docker-up
	@echo "Application is starting..."
	@sleep 5
	@echo "Service should be available at: http://localhost:8080"
	@echo "Run 'make logs' to see application logs"

# View application logs
logs:
	$(DOCKER_COMPOSE) logs -f app

# View database logs
logs-db:
	$(DOCKER_COMPOSE) logs -f db

# Run load tests (if you have k6 setup)
load-test:
	@echo "Running load tests..."
	k6 run tests/load_test.js

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	cd src && $(GO) clean

# Install dependencies
deps:
	@echo "Installing dependencies..."
	cd src && $(GO) mod tidy
	cd src && $(GO) mod download

# Show service status
status:
	@echo "Service status:"
	$(DOCKER_COMPOSE) ps

# Health check
health:
	@echo "Checking service health..."
	curl -f http://localhost:8080/health || echo "Service is not healthy"

# Help target
help:
	@echo "Available targets:"
	@echo "  run           - Run application locally"
	@echo "  build         - Build application binary"
	@echo "  start         - Start with Docker Compose"
	@echo "  docker-up     - Start services only"
	@echo "  docker-down   - Stop services"
	@echo "  test-unit     - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-all      - Run all tests"
	@echo "  lint          - Run linter"
	@echo "  lint-fix      - Fix linting issues"
	@echo "  logs          - View application logs"
	@echo "  status        - Show service status"
	@echo "  health        - Check service health"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  help          - Show this help message"

# Default target
.DEFAULT_GOAL := help