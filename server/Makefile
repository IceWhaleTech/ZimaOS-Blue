.PHONY: all build build-full test lint clean run dev trim-dist bump-version

# Variables
BINARY_NAME=blue
BUILD_DIR=bin
VERSION?=$(shell cat ../VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Trial license (Ed25519-signed, injected from environment)
TRIAL_LICENSE?=$(ZIMAOS_TRIAL_LICENSE)

# Build flags
# -s: Omit symbol table and debug info
# -w: Omit DWARF symbol table
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME) -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=$(TRIAL_LICENSE)"

# CGO is required for mattn/go-sqlite3
# Windows users: install MinGW-w64 first (https://winlibs.com or `scoop install mingw`)
export CGO_ENABLED=1

# Build tags
# fts5: enable SQLite FTS5 full-text search (mattn/go-sqlite3)
# whisper: enable whisper.cpp ASR support (requires CGo libs)
TAGS=-tags 'fts5 espeak'
TAGS_FULL=-tags 'fts5 espeak whisper'

# Default target
all: lint test build

# Trim embedded web dist (remove build artifacts and pre-compressed duplicates)
trim-dist:
	@echo "Trimming web dist..."
	@rm -f internal/web/dist/stats.html
	@rm -f internal/web/dist/stats.html.gz
	@find internal/web/dist -name "*.gz" -delete 2>/dev/null || true
	@find internal/web/dist -name "*.br" -delete 2>/dev/null || true
	@echo "Dist size: $$(du -sh internal/web/dist/ | cut -f1)"

# Pack dist.tar.gz into binary (append + 8-byte LE offset trailer)
# Layout: [Go binary][dist.tar.gz][8-byte offset of tar.gz start]
pack-dist:
	@echo "Packing dist into binary..."
	@tar czf /tmp/zimaos-dist.tar.gz -C internal/web/dist .
	@OFFSET=$$(stat -f%z $(BUILD_DIR)/$(BINARY_NAME) 2>/dev/null || stat -c%s $(BUILD_DIR)/$(BINARY_NAME)); \
	cat /tmp/zimaos-dist.tar.gz >> $(BUILD_DIR)/$(BINARY_NAME); \
	V=$$OFFSET; for i in 0 1 2 3 4 5 6 7; do printf "\\$$(printf '%03o' $$((V & 0xFF)))"; V=$$((V >> 8)); done >> $(BUILD_DIR)/$(BINARY_NAME); \
	echo "Appended dist ($$(du -h /tmp/zimaos-dist.tar.gz | cut -f1)) at offset $$OFFSET"
	@rm -f /tmp/zimaos-dist.tar.gz

# Build the default binary (self-extract dist, no whisper/ASR)
build: trim-dist
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/blue
	@$(MAKE) pack-dist

# Build full binary (with whisper/ASR)
build-full: trim-dist
	@echo "Building $(BINARY_NAME) (full, with ASR)..."
	@mkdir -p $(BUILD_DIR)
	go build $(TAGS_FULL) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/blue
	@$(MAKE) pack-dist

# Run tests
test:
	@echo "Running tests..."
	go test $(TAGS) -v -race -coverprofile=coverage.out ./...

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
	GOOS=linux GOARCH=amd64 go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/blue
	GOOS=linux GOARCH=arm64 go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/blue
	GOOS=darwin GOARCH=amd64 go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/blue
	GOOS=darwin GOARCH=arm64 go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/blue
	GOOS=windows GOARCH=amd64 go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/blue

# Docker build
docker-build:
	docker build -t zimaos-blue:$(VERSION) .

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Run lint, test, and build"
	@echo "  build        - Build default binary (self-extract dist, no ASR)"
	@echo "  build-full   - Build full binary (with whisper/ASR)"
	@echo "  trim-dist    - Remove bloat from embedded web dist"
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
	@echo "  bump-version - Sync VERSION file to all project files"

# Bump version: reads ../VERSION and patches all version strings across the project
bump-version:
	@V=$$(cat ../VERSION | tr -d '[:space:]'); \
	echo "Bumping version to $$V ..."; \
	sed -i '' "s/version   = \"[^\"]*\"/version   = \"$$V\"/" cmd/blue/main.go cmd/bluelib/main.go; \
	sed -i '' "s/^version = \"[^\"]*\"/version = \"$$V\"/" ../tauri-app/src-tauri/Cargo.toml; \
	sed -i '' "s/\"version\": \"[^\"]*\"/\"version\": \"$$V\"/" ../tauri-app/package.json ../tauri-app/src-tauri/tauri.conf.json; \
	cd ../tauri-app && npm install --package-lock-only --ignore-scripts 2>/dev/null || true; \
	echo "Done: $$V"
