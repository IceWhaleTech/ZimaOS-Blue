# Makefile for ZimaOS-Blue
# Supports building single binary with embedded frontend

.PHONY: all build build-frontend build-backend dev clean help
.PHONY: build-linux build-darwin build-windows build-all
.PHONY: tauri-dev tauri-build tauri-build-debug tauri-clean tauri-sidecar tauri-verify-macos-package
.PHONY: build-blue-lib-macos build-blue-lib-arm64 build-blue-lib-x64 build-blue-lib-universal

# Version info
VERSION ?= 0.10.36
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Directories
PROJECT_ROOT := $(shell pwd)
WEB_DIR := $(PROJECT_ROOT)/web
SERVER_DIR := $(PROJECT_ROOT)/server
EMBED_DIR := $(SERVER_DIR)/internal/web/dist
DIST_DIR := $(PROJECT_ROOT)/dist
TAURI_DIR := $(PROJECT_ROOT)/tauri-app
TAURI_LIB_DIR := $(TAURI_DIR)/src-tauri/lib
SKILLS_SRC := $(PROJECT_ROOT)/assets/skills
SKILLS_EMBED := $(SERVER_DIR)/internal/skill/embedded/skills

# Go build flags
LDFLAGS := -s -w
LDFLAGS += -X main.version=$(VERSION)
LDFLAGS += -X main.buildTime=$(BUILD_TIME)
LDFLAGS += -X main.gitCommit=$(GIT_COMMIT)

# Default target
all: build

# Build everything (frontend + backend)
build: build-frontend copy-frontend copy-skills build-backend
	@echo "Build complete! Binary at $(DIST_DIR)/zimaos-blue"

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

# Copy canonical skills from assets/skills/ to server/internal/skill/embedded/skills/ for go:embed
copy-skills:
	@echo "Copying skills to server/internal/skill/embedded/skills..."
	@rm -rf $(SKILLS_EMBED)
	@mkdir -p $(SKILLS_EMBED)
	@cp -r $(SKILLS_SRC)/* $(SKILLS_EMBED)/

# Build backend only (assumes frontend is already built and copied)
build-backend:
	@echo "Building backend..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue ./cmd/blue
	@echo "Binary size: $$(du -h $(DIST_DIR)/zimaos-blue | cut -f1)"

# Development mode (run frontend and backend separately)
dev:
	@echo "Starting development servers..."
	@echo "Run 'cd web && npm run dev' in one terminal"
	@echo "Run 'cd server && go run -tags dev ./cmd/blue' in another terminal"

# Cross-compilation targets
build-linux: build-frontend copy-frontend copy-skills
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-linux-amd64 ./cmd/blue

build-linux-arm64: build-frontend copy-frontend copy-skills
	@echo "Building for Linux (arm64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-linux-arm64 ./cmd/blue

build-darwin: build-frontend copy-frontend copy-skills
	@echo "Building for macOS (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-darwin-amd64 ./cmd/blue

build-darwin-arm64: build-frontend copy-frontend copy-skills
	@echo "Building for macOS (arm64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-darwin-arm64 ./cmd/blue

build-windows: build-frontend copy-frontend copy-skills
	@echo "Building for Windows (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-windows-amd64.exe ./cmd/blue

# Build for all platforms
build-all: build-frontend copy-frontend copy-skills
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-linux-amd64 ./cmd/blue
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-linux-arm64 ./cmd/blue
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-darwin-amd64 ./cmd/blue
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-darwin-arm64 ./cmd/blue
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-blue-windows-amd64.exe ./cmd/blue
	@echo "All builds complete!"
	@ls -lh $(DIST_DIR)/

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR)
	@rm -rf $(EMBED_DIR)
	@rm -rf $(SKILLS_EMBED)
	@rm -rf $(WEB_DIR)/dist
	@rm -rf $(WEB_DIR)/node_modules/.cache
	@echo "Clean complete!"

# GoReleaser targets
.PHONY: release release-snapshot release-check

# Check GoReleaser configuration
release-check:
	@echo "Checking GoReleaser configuration..."
	@goreleaser check

# Build snapshot release (for testing, no publish)
release-snapshot: build-frontend copy-frontend copy-skills
	@echo "Building snapshot release..."
	@goreleaser release --snapshot --clean

# Build and publish release (requires GITHUB_TOKEN)
release: build-frontend copy-frontend copy-skills
	@echo "Building and publishing release..."
	@goreleaser release --clean

# Tauri Desktop App targets
TAURI_DIR := $(PROJECT_ROOT)/tauri-app
TAURI_BIN_DIR := $(TAURI_DIR)/src-tauri/bin

# Build Go sidecar for Tauri (current platform)
tauri-sidecar: build-frontend copy-frontend copy-skills
	@echo "Building Go sidecar for Tauri..."
	@mkdir -p $(TAURI_BIN_DIR)
ifeq ($(shell uname -s),Darwin)
ifeq ($(shell uname -m),arm64)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/blue-server-aarch64-apple-darwin ./cmd/blue
else
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/blue-server-x86_64-apple-darwin ./cmd/blue
endif
else ifeq ($(shell uname -s),Linux)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/blue-server-x86_64-unknown-linux-gnu ./cmd/blue
else
	@cd $(SERVER_DIR) && CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(TAURI_BIN_DIR)/blue-server-x86_64-pc-windows-msvc.exe ./cmd/blue
endif
	@echo "Sidecar built successfully"

# Run Tauri in development mode
tauri-dev: tauri-sidecar
	@echo "Starting Tauri development mode..."
	@cd $(TAURI_DIR) && npm install && npm run dev

# Build Tauri app for production
tauri-build: tauri-sidecar
	@echo "Building Tauri app..."
ifeq ($(shell uname -s),Darwin)
	@echo "Note: make tauri-build runs plain Tauri bundling and does not notarize the macOS package."
	@echo "Use make tauri-package for a signed/notarized DMG."
endif
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
	@rm -rf $(TAURI_BIN_DIR)/blue-server-*
	@rm -rf $(TAURI_DIR)/src-tauri/data
	@echo "Tauri clean complete!"

# Verify macOS Tauri package signatures and notarization
tauri-verify-macos-package:
ifeq ($(shell uname -s),Darwin)
	@echo "Verifying macOS Tauri package..."
	@bash $(TAURI_DIR)/verify-macos-package.sh
else
	@echo "tauri-verify-macos-package is only supported on macOS"
	@exit 1
endif

# Build Tauri app using the full build script (recommended)
tauri-package: build-frontend copy-frontend copy-skills
	@echo "Building Tauri package..."
ifeq ($(shell uname -s),Darwin)
	@echo "macOS package builds require Apple signing/notarization credentials by default."
	@echo "Set MACOS_REQUIRE_NOTARIZATION=0 only if you intentionally want a local non-notarized package."
endif
	@chmod +x $(TAURI_DIR)/build.sh
	@$(TAURI_DIR)/build.sh

# Show help
help:
	@echo "ZimaOS-Blue Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Go Binary Targets:"
	@echo "  build              Build binary"
	@echo "  build-frontend     Build frontend only"
	@echo "  build-backend      Build backend only (requires frontend to be built)"
	@echo "  dev                Show development mode instructions"
	@echo "  build-linux        Build for Linux (amd64)"
	@echo "  build-linux-arm64  Build for Linux (arm64)"
	@echo "  build-darwin       Build for macOS (amd64)"
	@echo "  build-darwin-arm64 Build for macOS (arm64)"
	@echo "  build-windows      Build for Windows (amd64)"
	@echo "  build-all          Build for all platforms"
	@echo "  clean              Remove build artifacts"
	@echo ""
	@echo "Tauri Desktop App Targets:"
	@echo "  tauri-sidecar      Build Go sidecar for Tauri"
	@echo "  tauri-dev          Run Tauri in development mode"
	@echo "  tauri-build        Build Tauri app for production (non-notarized on macOS)"
	@echo "  tauri-package      Build Tauri package with full signing/notarization flow"
	@echo "  tauri-build-debug  Build Tauri app in debug mode"
	@echo "  tauri-verify-macos-package Verify macOS package signatures/notarization"
	@echo "  tauri-clean        Clean Tauri build artifacts"
	@echo ""
	@echo "macOS CGO Library Targets:"
	@echo "  build-blue-lib-arm64    Build Go static library for macOS ARM64"
	@echo "  build-blue-lib-x64      Build Go static library for macOS x64"
	@echo "  build-blue-lib-universal Create universal binary (fat library)"
	@echo "  build-blue-lib-macos    Build all macOS libraries (recommended)"
	@echo ""
	@echo "Release Targets:"
	@echo "  release-check      Check GoReleaser configuration"
	@echo "  release-snapshot   Build snapshot release (for testing)"
	@echo "  release            Build and publish release (requires GITHUB_TOKEN)"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION              Set version string (default: $(VERSION))"
	@echo "  MACOS_REQUIRE_NOTARIZATION  Require Apple signing/notarization creds in tauri-package (default: 1)"
	@echo ""
	@echo "Examples:"
	@echo "  make build                           # Build the app"
	@echo "  make tauri-dev                       # Run Tauri desktop app in dev mode"
	@echo "  make tauri-build                     # Build Tauri desktop app"
	@echo "  make tauri-package                   # Build signed/notarized macOS package when creds are present"
	@echo "  make tauri-verify-macos-package      # Verify built macOS package signatures and notarization"
	@echo "  make build-blue-lib-macos            # Build Go library for macOS CGO integration"

# =============================================================================
# macOS CGO Library Build Targets
# =============================================================================
# These targets build the Go server as a static library for linking into Rust
# This approach is used on macOS for faster startup (no process spawn overhead)
# =============================================================================

# Build Go static library for macOS ARM64 (Apple Silicon)
build-blue-lib-arm64:
	@echo "Building Go static library for macOS ARM64..."
	@mkdir -p $(TAURI_LIB_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
		go build -buildmode=c-archive \
		-tags 'fts5' \
		-ldflags "$(LDFLAGS)" \
		-o $(TAURI_LIB_DIR)/libblue_arm64.a \
		./cmd/bluelib
	@echo "Built: $(TAURI_LIB_DIR)/libblue_arm64.a"
	@ls -lh $(TAURI_LIB_DIR)/libblue_arm64.a

# Build Go static library for macOS x64 (Intel)
build-blue-lib-x64:
	@echo "Building Go static library for macOS x64..."
	@mkdir -p $(TAURI_LIB_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build -buildmode=c-archive \
		-tags 'fts5' \
		-ldflags "$(LDFLAGS)" \
		-o $(TAURI_LIB_DIR)/libblue_x64.a \
		./cmd/bluelib
	@echo "Built: $(TAURI_LIB_DIR)/libblue_x64.a"
	@ls -lh $(TAURI_LIB_DIR)/libblue_x64.a

# Create universal binary (fat library) from ARM64 and x64
build-blue-lib-universal: build-blue-lib-arm64 build-blue-lib-x64
	@echo "Creating universal binary (fat library)..."
	@lipo -create \
		$(TAURI_LIB_DIR)/libblue_arm64.a \
		$(TAURI_LIB_DIR)/libblue_x64.a \
		-output $(TAURI_LIB_DIR)/libblue.a
	@echo "Built: $(TAURI_LIB_DIR)/libblue.a"
	@ls -lh $(TAURI_LIB_DIR)/libblue.a
	@echo "Verifying architectures:"
	@lipo -info $(TAURI_LIB_DIR)/libblue.a

# Build all macOS libraries (recommended target)
# Builds for current architecture only for faster builds
build-blue-lib-macos:
	@echo "Building Go static library for macOS..."
	@mkdir -p $(TAURI_LIB_DIR)
ifeq ($(shell uname -m),arm64)
	@echo "Detected Apple Silicon, building ARM64 library..."
	@cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
		go build -buildmode=c-archive \
		-tags 'fts5' \
		-ldflags "$(LDFLAGS)" \
		-o $(TAURI_LIB_DIR)/libblue.a \
		./cmd/bluelib
else
	@echo "Detected Intel, building x64 library..."
	@cd $(SERVER_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build -buildmode=c-archive \
		-tags 'fts5' \
		-ldflags "$(LDFLAGS)" \
		-o $(TAURI_LIB_DIR)/libblue.a \
		./cmd/bluelib
endif
	@echo "Built: $(TAURI_LIB_DIR)/libblue.a"
	@ls -lh $(TAURI_LIB_DIR)/libblue.a

# Build Tauri app for macOS with CGO library (no sidecar)
tauri-build-macos-cgo: build-frontend copy-frontend copy-skills build-blue-lib-macos
	@echo "Building Tauri app for macOS with CGO library..."
	@cd $(TAURI_DIR) && npm install && npm run build
	@echo "macOS app built with embedded Go library"

# Clean CGO library artifacts
clean-blue-lib:
	@echo "Cleaning CGO library artifacts..."
	@rm -rf $(TAURI_LIB_DIR)
	@echo "Clean complete!"
