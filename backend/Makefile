.PHONY: help build run test clean migrate-up migrate-down sqlc-generate lint

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build all services"
	@echo "  run-services   - Run all backend services"
	@echo "  test           - Run tests"
	@echo "  lint           - Run linter"
	@echo "  clean          - Clean build artifacts"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback database migrations"
	@echo "  sqlc-generate  - Generate sqlc database code"
	@echo "  install-tools  - Install development tools"

# Build all services
build:
	@echo "Building services..."
	@mkdir -p bin
	@go build -o bin/api-gateway ./cmd/api-gateway
	@go build -o bin/market-data ./cmd/market-data
	@go build -o bin/trading-bot ./cmd/trading-bot
	@go build -o bin/migrate ./cmd/migrate
	@echo "Build complete!"

# Run all backend services (in development mode)
run-services:
	@echo "Starting services..."
	@go run ./cmd/api-gateway & \
	go run ./cmd/market-data & \
	go run ./cmd/trading-bot & \
	wait

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"

# Run database migrations up
migrate-up:
	@echo "Running migrations..."
	@go run ./cmd/migrate up
	@echo "Migrations complete!"

# Run database migrations down
migrate-down:
	@echo "Rolling back migrations..."
	@go run ./cmd/migrate down
	@echo "Rollback complete!"

# Generate sqlc code
sqlc-generate:
	@echo "Generating sqlc code..."
	@sqlc generate
	@echo "Code generation complete!"

# Install development tools
install-tools:
	@echo "Installing tools..."
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Tools installed!"

