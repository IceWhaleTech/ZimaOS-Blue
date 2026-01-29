package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/labstack/echo/v4"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/a2ui"
	networkapi "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/lifecycle"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/setup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/workflow"
)

var (
	version   = "0.10.4"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Global state for the server
var (
	serverMu     sync.Mutex
	serverCancel context.CancelFunc
	serverDone   chan struct{}
	isRunning    bool
)

//export EchoServerStart
func EchoServerStart(port C.int, dataDir *C.char) C.int {
	serverMu.Lock()
	defer serverMu.Unlock()

	if isRunning {
		return 1 // Already running
	}

	goPort := int(port)
	goDataDir := C.GoString(dataDir)

	// Set environment variables for config
	if goPort > 0 {
		os.Setenv("ECHO_SERVER_PORT", fmt.Sprintf("%d", goPort))
	}
	if goDataDir != "" {
		os.Setenv("ECHO_DATA_DIR", goDataDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serverCancel = cancel
	serverDone = make(chan struct{})

	go func() {
		defer close(serverDone)
		if err := runServer(ctx, goPort, goDataDir); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	isRunning = true
	return 0
}

//export EchoServerStop
func EchoServerStop() C.int {
	serverMu.Lock()
	defer serverMu.Unlock()

	if !isRunning {
		return 1 // Not running
	}

	if serverCancel != nil {
		serverCancel()
	}

	// Wait for server to stop with timeout
	select {
	case <-serverDone:
	case <-time.After(10 * time.Second):
		return 2 // Timeout
	}

	isRunning = false
	return 0
}

//export EchoServerIsRunning
func EchoServerIsRunning() C.int {
	serverMu.Lock()
	defer serverMu.Unlock()
	if isRunning {
		return 1
	}
	return 0
}

//export EchoServerGetVersion
func EchoServerGetVersion() *C.char {
	return C.CString(version)
}

//export EchoServerFreeString
func EchoServerFreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

func runServer(ctx context.Context, port int, dataDir string) error {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override port if specified
	if port > 0 {
		cfg.Server.Port = port
	}

	// Override data directory if specified
	if dataDir != "" {
		cfg.DataDir = dataDir
	}

	// Initialize logger
	log, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync()

	log.Info("Starting ZimaOS-Echo (embedded)",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit),
	)

	// Initialize worker pool
	workerPool := worker.NewPool(cfg.Worker.PoolSize)
	log.Info("Worker pool initialized", zap.Int("pool_size", cfg.Worker.PoolSize))

	// Initialize database
	dbPath := filepath.Join(cfg.DataDir, "echo.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Initialize all handlers (same as main.go)
	// ... (simplified for brevity - copy from main.go)

	// Create Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Create server instance
	srv := server.New(e, cfg, log)

	// Register health endpoint
	e.GET("/api/v1/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": "zimaos-echo",
			"version": version,
		})
	})

	// Serve embedded frontend
	web.RegisterRoutes(e)
	log.Info("Serving embedded frontend assets")

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		log.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Required for c-shared build mode
func main() {}

// Suppress unused import warnings
var (
	_ = a2ui.Handler{}
	_ = networkapi.Handler{}
	_ = auth.Handler{}
	_ = autoreply.Handler{}
	_ = backup.Handler{}
	_ = browser.Handler{}
	_ = channel.Handler{}
	_ = claudecode.Handler{}
	_ = companion.Handler{}
	_ = cron.Service{}
	_ = extauth.Handler{}
	_ = formfiller.Handler{}
	_ = homeassistant.Handler{}
	_ = lifecycle.Handler{}
	_ = llm.Handler{}
	_ = memory.Store{}
	_ = metrics.Handler{}
	_ = mfa.Handler{}
	_ = password.Handler{}
	_ = plugin.Store{}
	_ = promptguard.Guard{}
	_ = sandbox.Handler{}
	_ = security.Handler{}
	_ = setup.Handler{}
	_ = skill.Registry{}
	_ = builtin.Skills{}
	_ = stt.Handler{}
	_ = tools.Registry{}
	_ = tts.Handler{}
	_ = user.Handler{}
	_ = voice.Handler{}
	_ = workflow.Handler{}
)
