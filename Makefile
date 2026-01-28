# Makefile for ZimaOS-Echo
# Supports building single binary with embedded frontend

.PHONY: all build build-frontend build-backend dev clean help
.PHONY: build-linux build-darwin build-windows build-all
.PHONY: download-claude-code prepare-claude-code-dir

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

# Copy frontend to embed directory
copy-frontend:
	@echo "Copying frontend to embed directory..."
	@rm -rf $(EMBED_DIR)
	@mkdir -p $(EMBED_DIR)
	@cp -r $(WEB_DIR)/dist/* $(EMBED_DIR)/

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

# Show help
help:
	@echo "ZimaOS-Echo Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
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
	@echo "  EMBED_CLAUDE_CODE=true make build    # Same as build-embedded"
	@echo "  EMBED_ALL_PLATFORMS=true make build-embedded  # Embed all platforms"
