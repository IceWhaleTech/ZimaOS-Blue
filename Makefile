# Makefile for ZimaOS-Echo
# Supports building single binary with embedded frontend

.PHONY: all build build-frontend build-backend dev clean help
.PHONY: build-linux build-darwin build-windows build-all
.PHONY: download-claude-code prepare-claude-code-dir
.PHONY: tauri-dev tauri-build tauri-build-debug tauri-clean tauri-sidecar

# Version info
VERSION ?= 0.9.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Directories
PROJECT_ROOT := $(shell pwd)
WEB_DIR := $(PROJECT_ROOT)/web
SERVER_DIR := $(PROJECT_ROOT)/server
EMBED_DIR := $(SERVER_DIR)/internal/web/dist
CLAUDE_CODE_DIR := $(SERVER_DIR)/internal/claudecode/bin
DIST_DIR := $(PROJECT_ROOT)/dist

# Claude Code CLI embedding options (default: no embedding, download on first use)
EMBED_CLAUDE_CODE ?= false
EMBED_PLATFORM ?= $(shell go env GOOS)-$(shell go env GOARCH | sed 's/amd64/x64/')
EMBED_ALL_PLATFORMS ?= false

# Go build flags
LDFLAGS := -s -w
LDFLAGS += -X main.version=$(VERSION)
LDFLAGS += -X main.buildTime=$(BUILD_TIME)
LDFLAGS += -X main.gitCommit=$(GIT_COMMIT)

# Default target
all: build

# Build everything (frontend + backend)
# Claude Code CLI is downloaded on first use by default
build: build-frontend copy-frontend prepare-claude-code-dir build-backend
	@echo "Build complete! Binary at $(DIST_DIR)/zimaos-echo"

# Build with embedded Claude Code CLI
build-embedded: EMBED_CLAUDE_CODE=true
build-embedded: build-frontend copy-frontend download-claude-code build-backend
	@echo "Build complete with embedded Claude Code CLI! Binary at $(DIST_DIR)/zimaos-echo"

# Build frontend only
build-frontend:
	@echo "Building frontend..."
	@cd $(WEB_DIR) && npm install && npm run build

# Copy frontend to embed directory (exclude source maps)
copy-frontend:
	@echo "Copying frontend to embed directory..."
	@rm -rf $(EMBED_DIR)
	@mkdir -p $(EMBED_DIR)
	@cd $(WEB_DIR)/dist && find . -type f ! -name '*.map' -exec cp --parents {} $(EMBED_DIR)/ \; 2>/dev/null || \
		cd $(WEB_DIR)/dist && rsync -av --exclude='*.map' . $(EMBED_DIR)/ 2>/dev/null || \
		(cd $(WEB_DIR)/dist && for f in $$(find . -type f ! -name '*.map'); do mkdir -p $(EMBED_DIR)/$$(dirname $$f) && cp $$f $(EMBED_DIR)/$$f; done)

# Prepare Claude Code CLI directory (create .gitkeep for go:embed)
prepare-claude-code-dir:
	@mkdir -p $(CLAUDE_CODE_DIR)
	@touch $(CLAUDE_CODE_DIR)/.gitkeep
ifeq ($(EMBED_CLAUDE_CODE),false)
	@echo "Claude Code CLI will be downloaded on first use"
endif

# Download Claude Code CLI binaries (for embedding)
download-claude-code:
ifeq ($(EMBED_CLAUDE_CODE),true)
ifeq ($(EMBED_ALL_PLATFORMS),true)
	@echo "Downloading Claude Code CLI for all platforms..."
	@$(PROJECT_ROOT)/scripts/download-claude-code.sh --all
else
	@echo "Downloading Claude Code CLI for $(EMBED_PLATFORM)..."
	@$(PROJECT_ROOT)/scripts/download-claude-code.sh --platform $(EMBED_PLATFORM)
endif
else
	@echo "Skipping Claude Code CLI embedding (EMBED_CLAUDE_CODE=false)"
	@echo "CLI will be downloaded on first use"
	@mkdir -p $(CLAUDE_CODE_DIR)
	@touch $(CLAUDE_CODE_DIR)/.gitkeep
endif

# Build backend only (assumes frontend is already built and copied)
build-backend:
	@echo "Building backend..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo ./cmd/echo
	@echo "Binary size: $$(du -h $(DIST_DIR)/zimaos-echo | cut -f1)"

# Development mode (run frontend and backend separately)
dev:
	@echo "Starting development servers..."
	@echo "Run 'cd web && npm run dev' in one terminal"
	@echo "Run 'cd server && go run -tags dev ./cmd/echo' in another terminal"

# Cross-compilation targets
build-linux: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-amd64 ./cmd/echo

build-linux-arm64: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building for Linux (arm64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-arm64 ./cmd/echo

build-darwin: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building for macOS (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-amd64 ./cmd/echo

build-darwin-arm64: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building for macOS (arm64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-arm64 ./cmd/echo

build-windows: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building for Windows (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-windows-amd64.exe ./cmd/echo

# Build for all platforms
build-all: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-amd64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-arm64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-amd64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-arm64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-windows-amd64.exe ./cmd/echo
	@echo "All builds complete!"
	@ls -lh $(DIST_DIR)/

# Build for all platforms with embedded Claude Code CLI
build-all-embedded: EMBED_CLAUDE_CODE=true
build-all-embedded: EMBED_ALL_PLATFORMS=true
build-all-embedded: build-frontend copy-frontend download-claude-code
	@echo "Building for all platforms with embedded Claude Code CLI..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-amd64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-arm64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-amd64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-arm64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-windows-amd64.exe ./cmd/echo
	@echo "All builds complete with embedded Claude Code CLI!"
	@ls -lh $(DIST_DIR)/

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR)
	@rm -rf $(EMBED_DIR)
	@rm -rf $(WEB_DIR)/dist
	@rm -rf $(WEB_DIR)/node_modules/.cache
	@rm -f $(CLAUDE_CODE_DIR)/claude-*
	@rm -f $(CLAUDE_CODE_DIR)/VERSION
	@echo "Clean complete!"

# GoReleaser targets
.PHONY: release release-snapshot release-check

# Check GoReleaser configuration
release-check:
	@echo "Checking GoReleaser configuration..."
	@goreleaser check

# Build snapshot release (for testing, no publish)
release-snapshot: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building snapshot release..."
	@goreleaser release --snapshot --clean

# Build and publish release (requires GITHUB_TOKEN)
release: build-frontend copy-frontend prepare-claude-code-dir
	@echo "Building and publishing release..."
	@goreleaser release --clean

# Tauri Desktop App targets
TAURI_DIR := $(PROJECT_ROOT)/tauri-app
TAURI_BIN_DIR := $(TAURI_DIR)/src-tauri/bin

# Build Go sidecar for Tauri (current platform)
tauri-sidecar: build-frontend copy-frontend
	@echo "Building Go sidecar for Tauri..."
	@mkdir -p $(TAURI_BIN_DIR)
ifeq ($(shell uname -s),Darwin)
ifeq ($(shell uname -m),arm64)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/echo-server-aarch64-apple-darwin ./cmd/echo
else
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/echo-server-x86_64-apple-darwin ./cmd/echo
endif
else ifeq ($(shell uname -s),Linux)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/echo-server-x86_64-unknown-linux-gnu ./cmd/echo
else
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/echo-server-x86_64-pc-windows-msvc.exe ./cmd/echo
endif
	@echo "Sidecar built successfully"

# Run Tauri in development mode
tauri-dev: tauri-sidecar
	@echo "Starting Tauri development mode..."
	@cd $(TAURI_DIR) && npm install && npm run dev

# Build Tauri app for production
tauri-build: tauri-sidecar
	@echo "Building Tauri app..."
	@rm -rf $(TAURI_DIR)/src-tauri/data
	@mkdir -p $(TAURI_DIR)/src-tauri/data
	@cd $(TAURI_DIR) && npm install && npm run build
ifeq ($(shell uname -s),Darwin)
	@echo "Setting DMG file icon..."
	@DMG_FILE=$$(find $(TAURI_DIR)/src-tauri/target/release/bundle/dmg -name "*.dmg" -type f 2>/dev/null | head -1); \
	if [ -n "$$DMG_FILE" ] && command -v fileicon >/dev/null 2>&1; then \
		fileicon set "$$DMG_FILE" "$(TAURI_DIR)/src-tauri/icons/icon.icns"; \
		echo "DMG file icon set successfully"; \
	elif [ -n "$$DMG_FILE" ]; then \
		echo "Warning: fileicon not found. Install with: brew install fileicon"; \
	fi
endif

# Build Tauri app in debug mode
tauri-build-debug: tauri-sidecar
	@echo "Building Tauri app (debug)..."
	@cd $(TAURI_DIR) && npm install && npm run build:debug

# Clean Tauri build artifacts
tauri-clean:
	@echo "Cleaning Tauri build artifacts..."
	@rm -rf $(TAURI_DIR)/src-tauri/target
	@rm -rf $(TAURI_BIN_DIR)/echo-server-*
	@rm -rf $(TAURI_DIR)/src-tauri/data
	@echo "Tauri clean complete!"

# Build Tauri app using the full build script (recommended)
tauri-package: build-frontend copy-frontend
	@echo "Building Tauri package..."
	@chmod +x $(TAURI_DIR)/build.sh
	@$(TAURI_DIR)/build.sh

# Show help
help:
	@echo "ZimaOS-Echo Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Go Binary Targets:"
	@echo "  build              Build binary (Claude Code CLI downloaded on first use)"
	@echo "  build-embedded     Build binary with embedded Claude Code CLI"
	@echo "  build-frontend     Build frontend only"
	@echo "  build-backend      Build backend only (requires frontend to be built)"
	@echo "  download-claude-code  Download Claude Code CLI binaries for embedding"
	@echo "  dev                Show development mode instructions"
	@echo "  build-linux        Build for Linux (amd64)"
	@echo "  build-linux-arm64  Build for Linux (arm64)"
	@echo "  build-darwin       Build for macOS (amd64)"
	@echo "  build-darwin-arm64 Build for macOS (arm64)"
	@echo "  build-windows      Build for Windows (amd64)"
	@echo "  build-all          Build for all platforms"
	@echo "  build-all-embedded Build for all platforms with embedded Claude Code CLI"
	@echo "  clean              Remove build artifacts"
	@echo ""
	@echo "Tauri Desktop App Targets:"
	@echo "  tauri-sidecar      Build Go sidecar for Tauri"
	@echo "  tauri-dev          Run Tauri in development mode"
	@echo "  tauri-build        Build Tauri app for production"
	@echo "  tauri-package      Build Tauri package with full script (recommended)"
	@echo "  tauri-build-debug  Build Tauri app in debug mode"
	@echo "  tauri-clean        Clean Tauri build artifacts"
	@echo ""
	@echo "Release Targets:"
	@echo "  release-check      Check GoReleaser configuration"
	@echo "  release-snapshot   Build snapshot release (for testing)"
	@echo "  release            Build and publish release (requires GITHUB_TOKEN)"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION              Set version string (default: $(VERSION))"
	@echo "  EMBED_CLAUDE_CODE    Embed Claude Code CLI (default: false)"
	@echo "  EMBED_PLATFORM       Platform to embed (default: current platform)"
	@echo "  EMBED_ALL_PLATFORMS  Embed all platforms (default: false)"
	@echo ""
	@echo "Examples:"
	@echo "  make build                           # Build without embedded CLI"
	@echo "  make build-embedded                  # Build with embedded CLI for current platform"
	@echo "  make tauri-dev                       # Run Tauri desktop app in dev mode"
	@echo "  make tauri-build                     # Build Tauri desktop app"
	@echo "  make tauri-package                   # Build Tauri package with full script"
