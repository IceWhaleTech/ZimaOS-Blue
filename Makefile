# Makefile for ZimaOS-Echo
# Supports building single binary with embedded frontend

.PHONY: all build build-frontend build-backend dev clean help
.PHONY: build-linux build-darwin build-windows build-all

# Version info
VERSION ?= 0.9.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Directories
PROJECT_ROOT := $(shell pwd)
WEB_DIR := $(PROJECT_ROOT)/web
SERVER_DIR := $(PROJECT_ROOT)/server
EMBED_DIR := $(SERVER_DIR)/internal/web/dist
DIST_DIR := $(PROJECT_ROOT)/dist

# Go build flags
LDFLAGS := -s -w
LDFLAGS += -X main.version=$(VERSION)
LDFLAGS += -X main.buildTime=$(BUILD_TIME)
LDFLAGS += -X main.gitCommit=$(GIT_COMMIT)

# Default target
all: build

# Build everything (frontend + backend)
build: build-frontend copy-frontend build-backend
	@echo "Build complete! Binary at $(DIST_DIR)/zimaos-echo"

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
build-linux: build-frontend copy-frontend
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-amd64 ./cmd/echo

build-linux-arm64: build-frontend copy-frontend
	@echo "Building for Linux (arm64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-arm64 ./cmd/echo

build-darwin: build-frontend copy-frontend
	@echo "Building for macOS (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-amd64 ./cmd/echo

build-darwin-arm64: build-frontend copy-frontend
	@echo "Building for macOS (arm64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-arm64 ./cmd/echo

build-windows: build-frontend copy-frontend
	@echo "Building for Windows (amd64)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-windows-amd64.exe ./cmd/echo

# Build for all platforms
build-all: build-frontend copy-frontend
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-amd64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-linux-arm64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-amd64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-darwin-arm64 ./cmd/echo
	@cd $(SERVER_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/zimaos-echo-windows-amd64.exe ./cmd/echo
	@echo "All builds complete!"
	@ls -lh $(DIST_DIR)/

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR)
	@rm -rf $(EMBED_DIR)
	@rm -rf $(WEB_DIR)/dist
	@rm -rf $(WEB_DIR)/node_modules/.cache
	@echo "Clean complete!"

# Show help
help:
	@echo "ZimaOS-Echo Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build single binary with embedded frontend (default)"
	@echo "  build-frontend Build frontend only"
	@echo "  build-backend  Build backend only (requires frontend to be built)"
	@echo "  dev            Show development mode instructions"
	@echo "  build-linux    Build for Linux (amd64)"
	@echo "  build-linux-arm64  Build for Linux (arm64)"
	@echo "  build-darwin   Build for macOS (amd64)"
	@echo "  build-darwin-arm64 Build for macOS (arm64)"
	@echo "  build-windows  Build for Windows (amd64)"
	@echo "  build-all      Build for all platforms"
	@echo "  clean          Remove build artifacts"
	@echo "  help           Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION        Set version string (default: $(VERSION))"
