.PHONY: all build test lint clean run dev

# Variables
BINARY_NAME=echo
BUILD_DIR=bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Trial provider configuration (injected from environment)
TRIAL_API_KEY?=$(ZIMAOS_TRIAL_API_KEY)
TRIAL_BASE_URL?=$(ZIMAOS_TRIAL_BASE_URL)

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME) -X github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool.trialAPIKey=$(TRIAL_API_KEY) -X github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool.trialBaseURL=$(TRIAL_BASE_URL)"

# Default target
all: lint test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/echo

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode with hot reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate mocks (for future use)
mocks:
	@echo "Generating mocks..."
	@which mockgen > /dev/null || go install go.uber.org/mock/mockgen@latest
	# Add mockgen commands here

# Cross-compile for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/echo
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/echo
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/echo
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/echo
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/echo

# Docker build
docker-build:
	docker build -t zimaos-echo:$(VERSION) .

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Run lint, test, and build"
	@echo "  build        - Build the binary"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  lint         - Run linter"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the application"
	@echo "  dev          - Run with hot reload (requires air)"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  build-all    - Cross-compile for multiple platforms"
	@echo "  docker-build - Build Docker image"
	@echo "  help         - Show this help"
