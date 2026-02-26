.PHONY: all build build-full build-bluecli build-launcher test lint clean run trim-dist bump-version copy-skills

# Variables
BINARY_NAME=blue
BLUECLI_NAME=bluecli
BUILD_DIR=bin
LAUNCHER_DIR=cmd/launcher
VERSION?=$(shell cat ../VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Trial license (Ed25519-signed, injected from environment)
TRIAL_LICENSE?=$(ZIMAOS_TRIAL_LICENSE)

# Build flags
# -s: Omit symbol table and debug info
# -w: Omit DWARF symbol table
BASE_LDFLAGS=-s -w -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME) -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=$(TRIAL_LICENSE)

# On macOS, embed Info.plist into __TEXT,__info_plist for NSBundle.mainBundle to read.
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
LDFLAGS=-ldflags "$(BASE_LDFLAGS) -extldflags '-sectcreate __TEXT __info_plist Info.plist'"
else
LDFLAGS=-ldflags "$(BASE_LDFLAGS)"
endif

# CGO is required for mattn/go-sqlite3
# Windows users: install MinGW-w64 first (https://winlibs.com or `scoop install mingw`)
export CGO_ENABLED=1

# Build tags
# fts5: enable SQLite FTS5 full-text search (mattn/go-sqlite3)
# espeak/kokoro: eSpeak-NG G2P + Kokoro ONNX TTS (Linux/Windows only)
# whisper: whisper.cpp ASR support (Linux/Windows only)
# macOS uses native Speech framework — no espeak/kokoro/whisper needed
ifeq ($(UNAME_S),Darwin)
TAGS=-tags 'fts5'
TAGS_FULL=-tags 'fts5'
else
TAGS=-tags 'fts5 espeak kokoro'
TAGS_FULL=-tags 'fts5 espeak kokoro whisper'
endif

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
# NOTE: On macOS this is skipped — appending data breaks the Mach-O structure
# and TCC's strict signature validation rejects the binary, preventing
# speech recognition authorization.
pack-dist:
	@echo "Packing dist into binary..."
	@tar czf /tmp/zimaos-dist.tar.gz -C internal/web/dist .
	@OFFSET=$$(stat -f%z $(BUILD_DIR)/$(BINARY_NAME) 2>/dev/null || stat -c%s $(BUILD_DIR)/$(BINARY_NAME)); \
	cat /tmp/zimaos-dist.tar.gz >> $(BUILD_DIR)/$(BINARY_NAME); \
	V=$$OFFSET; for i in 0 1 2 3 4 5 6 7; do printf "\\$$(printf '%03o' $$((V & 0xFF)))"; V=$$((V >> 8)); done >> $(BUILD_DIR)/$(BINARY_NAME); \
	echo "Appended dist ($$(du -h /tmp/zimaos-dist.tar.gz | cut -f1)) at offset $$OFFSET"
	@rm -f /tmp/zimaos-dist.tar.gz

# Code-sign macOS binary (requires APPLE_SIGNING_IDENTITY env var)
# Usage: make codesign CODESIGN_TARGET=bluecli
CODESIGN_TARGET?=$(BLUECLI_NAME)
codesign:
ifeq ($(UNAME_S),Darwin)
ifdef APPLE_SIGNING_IDENTITY
	@echo "Code-signing $(BUILD_DIR)/$(CODESIGN_TARGET)..."
	codesign --force --options runtime --sign "$(APPLE_SIGNING_IDENTITY)" \
		--entitlements ../tauri-app/src-tauri/entitlements.plist $(BUILD_DIR)/$(CODESIGN_TARGET)
else
	@echo "APPLE_SIGNING_IDENTITY not set, skipping code signing"
endif
endif

# Copy canonical skills from assets/skills/ for go:embed
SKILLS_SRC := $(shell cd .. && pwd)/assets/skills
SKILLS_EMBED := internal/skill/embedded/skills

copy-skills:
	@echo "Copying skills to $(SKILLS_EMBED)..."
	@rm -rf $(SKILLS_EMBED)
	@mkdir -p $(SKILLS_EMBED)
	@cp -r $(SKILLS_SRC)/* $(SKILLS_EMBED)/

# Build bluecli (the actual server binary, with CGo)
build-bluecli: copy-skills
	@echo "Building $(BLUECLI_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BLUECLI_NAME) ./cmd/blue
ifeq ($(UNAME_S),Darwin)
	@$(MAKE) codesign
endif

# Build launcher (embeds bluecli + dist.tar.gz, no CGo)
build-launcher:
	@echo "Preparing launcher assets..."
	@tar czf $(BUILD_DIR)/dist.tar.gz -C internal/web/dist .
	@cp $(BUILD_DIR)/$(BLUECLI_NAME) $(LAUNCHER_DIR)/bluecli
	@cp $(BUILD_DIR)/dist.tar.gz $(LAUNCHER_DIR)/dist.tar.gz
	@echo "Building launcher..."
	CGO_ENABLED=0 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(LAUNCHER_DIR)
	@rm -f $(LAUNCHER_DIR)/bluecli $(LAUNCHER_DIR)/dist.tar.gz
ifeq ($(UNAME_S),Darwin)
	@$(MAKE) codesign CODESIGN_TARGET=$(BINARY_NAME)
endif
	@echo "Launcher: $(BUILD_DIR)/$(BINARY_NAME) ($$(du -h $(BUILD_DIR)/$(BINARY_NAME) | cut -f1))"

# Build the default distribution
# macOS: launcher (embeds bluecli + dist) — single file, self-extracting
# Linux/Windows: bluecli + pack-dist (appended dist.tar.gz)
build: trim-dist copy-skills
	@mkdir -p $(BUILD_DIR)
ifeq ($(UNAME_S),Darwin)
	@$(MAKE) build-bluecli
	@$(MAKE) build-launcher
else
	@echo "Building $(BINARY_NAME)..."
	go build $(TAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/blue
	@$(MAKE) pack-dist
endif

# Build full binary (with whisper/ASR)
build-full: trim-dist copy-skills
	@mkdir -p $(BUILD_DIR)
ifeq ($(UNAME_S),Darwin)
	@echo "Building $(BLUECLI_NAME) (full, with ASR)..."
	go build $(TAGS_FULL) $(LDFLAGS) -o $(BUILD_DIR)/$(BLUECLI_NAME) ./cmd/blue
	@$(MAKE) codesign
	@$(MAKE) build-launcher
else
	@echo "Building $(BINARY_NAME) (full, with ASR)..."
	go build $(TAGS_FULL) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/blue
	@$(MAKE) pack-dist
endif

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
	rm -rf $(SKILLS_EMBED)
	rm -f coverage.out coverage.html
	rm -f $(LAUNCHER_DIR)/bluecli $(LAUNCHER_DIR)/dist.tar.gz

# Run the application
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

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
	@echo "  build        - Build launcher (embeds bluecli + dist)"
	@echo "  build-bluecli- Build server binary only"
	@echo "  build-full   - Build launcher with full binary (whisper/ASR)"
	@echo "  trim-dist    - Remove bloat from embedded web dist"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  lint         - Run linter"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run the application"
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
