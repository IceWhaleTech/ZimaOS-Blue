package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	contextpackembed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack/embedded"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/reclaim"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"

	skillEmbed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/embedded"
	ssePkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

var (
	version   = "0.10.37"
	buildTime = "unknown"
	gitCommit = "unknown"
)

const (
	embeddedServerShutdownGracePeriod = 1200 * time.Millisecond
	embeddedServerStopTimeout         = 1800 * time.Millisecond
	runtimeIdleCheckpointThreshold    = 5 * time.Minute
)

func applyPendingBackupRestore(dataDir string) (bool, error) {
	mgr, err := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(dataDir, "backups"),
		SkillsPath:    filepath.Join(dataDir, "workspace", ".claude", "skills"),
	}, dataDir, dataDir)
	if err != nil {
		return false, err
	}
	if !mgr.HasPendingRestore() {
		return false, nil
	}
	_, err = mgr.ApplyPendingRestore(context.Background())
	return true, err
}

// getDataDir returns the platform-specific data directory path
func getDataDir() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(os.ExpandEnv("$HOME"), "Library", "Application Support", "com.zimaos.blue")
	}
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, "ZimaOS Blue")
	}
	// Linux
	home := os.ExpandEnv("$HOME")
	return filepath.Join(home, ".zimaos-blue")
}

func getLogsDir() string {
	return filepath.Join(getDataDir(), "logs")
}

// registerCleanup registers a cleanup function to be called on shutdown
func registerCleanup(fn func() error) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFuncs = append(cleanupFuncs, fn)
}

// performCleanup executes all registered cleanup functions
func performCleanup() {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()

	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		if err := cleanupFuncs[i](); err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup error: %v\n", err)
		}
	}
	cleanupFuncs = nil
}

// setupSignalHandler sets up signal handling for graceful shutdown
func setupSignalHandler() {
	signalListener.Do(func() {
		signalChan = make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

		go func() {
			sig := <-signalChan
			fmt.Fprintf(os.Stderr, "Received signal %v, initiating graceful shutdown...\n", sig)

			// Cancel the server context — this triggers the shutdown path
			// in runServer() which will close streams, stop HTTP server, etc.
			// The caller (BlueServerStop or Tauri) handles process lifecycle.
			serverMu.Lock()
			if isRunning && serverCancel != nil {
				serverCancel()
			}
			serverMu.Unlock()
		}()
	})
}

// Global state for the server
var (
	serverMu       sync.Mutex
	serverCancel   context.CancelFunc
	serverDone     chan struct{}
	isRunning      bool
	echoServer     *echo.Echo
	httpServer     *http.Server
	cleanupFuncs   []func() error
	cleanupMu      sync.Mutex
	signalChan     chan os.Signal
	signalListener *sync.Once = &sync.Once{}
)

// startEmbeddedServerLocked starts the embedded server.
// Caller must hold serverMu.
func startEmbeddedServerLocked(port int, dataDir string, cfgFile string) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	serverCancel = cancel
	serverDone = done

	go func(runCtx context.Context, runPort int, runDataDir string, runCfgFile string, doneCh chan struct{}) {
		defer close(doneCh)
		if err := runServer(runCtx, runPort, runDataDir, runCfgFile); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}(ctx, port, dataDir, cfgFile, done)

	isRunning = true
}

func triggerEmbeddedRestart(port int, dataDir string, cfgFile string, log *zap.Logger) error {
	go func() {
		if rc := BlueServerStop(); rc != 0 {
			if log != nil {
				log.Warn("Auto-restart: failed to stop embedded server before restart", zap.Int("code", int(rc)))
			}
			return
		}

		serverMu.Lock()
		defer serverMu.Unlock()
		if isRunning {
			if log != nil {
				log.Warn("Auto-restart aborted: server still marked as running")
			}
			return
		}

		setupSignalHandler()
		startEmbeddedServerLocked(port, dataDir, cfgFile)
		if log != nil {
			log.Info("Embedded server restarted to apply staged backup restore")
		}
	}()
	return nil
}

//export BlueServerStartWithArgs
func BlueServerStartWithArgs(port C.int, dataDir *C.char, args *C.char) C.int {
	serverMu.Lock()
	defer serverMu.Unlock()

	if isRunning {
		return 1 // Already running
	}

	// Setup signal handler for graceful shutdown
	setupSignalHandler()

	goPort := int(port)
	goDataDir := getDataDir()
	goArgs := C.GoString(args)
	cfgFile := ""

	// Parse args string: --port N, --config PATH, --data-dir PATH, --dev, --verbose
	if goArgs != "" {
		fields := strings.Fields(goArgs)
		for i := 0; i < len(fields); i++ {
			switch fields[i] {
			case "--port", "-p":
				if i+1 < len(fields) {
					i++
					os.Setenv("BLUE_SERVER_PORT", fields[i])
					var p int
					if _, err := fmt.Sscanf(fields[i], "%d", &p); err == nil && p > 0 {
						goPort = p
					}
				}
			case "--config":
				if i+1 < len(fields) {
					i++
					cfgFile = fields[i]
				}
			case "--data-dir":
				if i+1 < len(fields) {
					i++
					goDataDir = fields[i]
				}
			case "--dev":
				os.Setenv("BLUE_DEV", "1")
				if goDataDir == getDataDir() {
					home, _ := os.UserHomeDir()
					goDataDir = filepath.Join(home, ".zimaos-blue-dev")
				}
			case "--verbose", "-v":
				os.Setenv("BLUE_LOG_LEVEL", "debug")
			}
		}
	}

	// Set port env for config.Load
	if goPort > 0 {
		os.Setenv("BLUE_SERVER_PORT", fmt.Sprintf("%d", goPort))
	}

	startEmbeddedServerLocked(goPort, goDataDir, cfgFile)
	return 0
}

//export BlueServerStart
func BlueServerStart(port C.int, dataDir *C.char) C.int {
	serverMu.Lock()
	defer serverMu.Unlock()

	if isRunning {
		return 1 // Already running
	}

	// Setup signal handler for graceful shutdown
	setupSignalHandler()

	goPort := int(port)
	goDataDir := getDataDir()

	// Set environment variables for config
	if goPort > 0 {
		os.Setenv("BLUE_SERVER_PORT", fmt.Sprintf("%d", goPort))
	}

	startEmbeddedServerLocked(goPort, goDataDir, "")
	return 0
}

//export BlueServerStop
func BlueServerStop() C.int {
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
	case <-time.After(embeddedServerStopTimeout):
		return 2 // Timeout
	}

	// Perform cleanup of CGO resources
	performCleanup()

	isRunning = false
	return 0
}

//export BlueServerIsRunning
func BlueServerIsRunning() C.int {
	serverMu.Lock()
	defer serverMu.Unlock()
	if isRunning {
		return 1
	}
	return 0
}

//export BlueServerGetVersion
func BlueServerGetVersion() *C.char {
	return C.CString(version)
}

//export BlueServerGetPort
func BlueServerGetPort() C.int {
	return C.int(server.GetActualPort())
}

//export BlueServerFreeString
func BlueServerFreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

//export BlueServerCleanup
func BlueServerCleanup() {
	performCleanup()
}

func runServer(ctx context.Context, port int, dataDir string, cfgFile string) error {
	trace := bootstrap.NewStartupTrace("bluelib.run_server", nil)
	trace.Mark("enter")

	// Tune GC for lower memory usage (shared with blue CLI)
	bootstrap.TuneGC()

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	trace.Mark("config_loaded")
	var hotReloader *config.HotReloader

	// Override port if specified
	if port > 0 {
		cfg.Server.Port = port
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	trace.Mark("data_dir_ready")

	previousCleanShutdown, startupIntegrityErr := dbutil.BeginStartupIntegritySession(dataDir)
	if startupIntegrityErr != nil {
		fmt.Fprintf(os.Stderr, "Startup integrity state warning: %v\n", startupIntegrityErr)
		previousCleanShutdown = false
	}
	// After an unclean shutdown, pay the quick_check cost once during startup
	// so power-loss corruption is surfaced before services begin using blue.db.
	startupQuickCheckEnabled := !previousCleanShutdown
	dbutil.SetStartupQuickCheckEnabled(startupQuickCheckEnabled)
	defer dbutil.SetStartupQuickCheckEnabled(true)
	trace.Mark("startup_integrity_ready", zap.Bool("previous_clean_shutdown", previousCleanShutdown))

	// Initialize logger with ring buffer for log viewing
	if err := logger.InitWithMirror(&cfg.Log, filepath.Join(getLogsDir(), "blue.log")); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize zap logger
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer zapLogger.Sync()
	trace.SetLogger(zapLogger)
	trace.Mark("logger_ready")

	zapLogger.Info("Starting ZimaOS-Blue (embedded)",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit),
		zap.Int("port", cfg.Server.Port),
		zap.String("data_dir", dataDir),
	)
	appliedPendingRestore, err := applyPendingBackupRestore(dataDir)
	if previousCleanShutdown {
		zapLogger.Info("Skipping proactive startup database scan after previous clean shutdown")
	} else if !appliedPendingRestore {
		zapLogger.Info("Primary database open will run quick integrity checks after an unclean shutdown",
			zap.Bool("startup_quick_check", startupQuickCheckEnabled),
		)
	}
	if err != nil {
		zapLogger.Warn("Failed to apply pending backup restore before database initialization", zap.Error(err))
	}
	trace.Mark("backup_restore_checked")

	// Create server config
	serverCfg := &bootstrap.ServerConfig{
		Port:      cfg.Server.Port,
		DataDir:   dataDir,
		Version:   version,
		BuildTime: buildTime,
		GitCommit: gitCommit,
		Mode:      "embedded",
	}

	// Initialize services using bootstrap package
	services, err := bootstrap.InitServices(serverCfg, cfg, zapLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	defer services.Close()
	trace.Mark("services_initialized")

	// Shared kvstore for all config persistence
	var sqliteKV *kvstore.SQLiteStore
	if services.DBConn != nil {
		sqliteKV, err = kvstore.NewSQLiteStoreWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
	} else {
		sqliteKV, err = kvstore.NewSQLiteStoreWithDB(services.DB)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize config kvstore: %w", err)
	}
	configKV := kvstore.NewCachedStore(sqliteKV)

	// Import config into kvstore (first-run: imports; subsequent: loads from DB)
	cfgStore := config.NewConfigStore(configKV)
	if updatedCfg, err := cfgStore.LoadOrImport(cfg); err == nil {
		cfg = updatedCfg
	}
	hotReloader, err = config.NewHotReloader(cfgFile, cfg, &config.HotReloadConfig{
		Enabled:             true,
		WatchInterval:       5 * time.Second,
		ValidateBeforeApply: true,
	})
	if err != nil {
		zapLogger.Warn("Failed to initialize hot reloader", zap.Error(err))
		hotReloader = nil
	} else {
		config.SyncHotReloadToStore(hotReloader, cfgStore)
		if err := hotReloader.Start(); err != nil {
			zapLogger.Warn("Failed to start hot reloader", zap.Error(err))
		}
		registerCleanup(func() error {
			return hotReloader.Stop()
		})
	}
	trace.Mark("hot_reloader_ready")
	if result, migrateErr := server.MigrateLegacyProviderSettings(context.Background(), configKV, dataDir); migrateErr != nil {
		zapLogger.Warn("Failed to migrate legacy provider settings into config store", zap.Error(migrateErr))
	} else if result != nil {
		fields := []zap.Field{zap.String("source", result.SourcePath)}
		if result.ArchivedPath != "" {
			fields = append(fields, zap.String("archived_path", result.ArchivedPath))
		}
		zapLogger.Info("Legacy provider settings imported into config store", fields...)
	}
	if result, migrateErr := harness.MigrateLegacyStore(context.Background(), services.DB, dataDir); migrateErr != nil {
		zapLogger.Warn("Failed to migrate legacy harness store", zap.Error(migrateErr))
	} else if result != nil {
		fields := []zap.Field{
			zap.String("source", result.SourcePath),
			zap.Int("rows_imported", result.RowsImported),
		}
		if result.ArchivedPath != "" {
			fields = append(fields, zap.String("archived_path", result.ArchivedPath))
		}
		zapLogger.Info("Legacy harness store imported into blue.db", fields...)
	}
	runtimeActivity := server.NewRuntimeActivityTracker()
	readDB := services.DB
	if services.DBConn != nil && services.DBConn.Reader != nil {
		readDB = services.DBConn.Reader
	}
	if cfg.Performance.Database.CheckpointInterval > 0 {
		dbutil.StartIdleAwarePeriodicWALCheckpoint(
			ctx,
			services.DB,
			cfg.Performance.Database.CheckpointInterval,
			runtimeIdleCheckpointThreshold,
			func(ctx context.Context) (bool, string, error) {
				return runtimeActivity.CheckpointIdle(ctx, readDB, runtimeIdleCheckpointThreshold)
			},
			dbutil.CheckpointTruncate,
			func(err error) {
				zapLogger.Warn("Idle-aware WAL checkpoint failed", zap.Error(err))
			},
		)
		zapLogger.Info("Idle-aware WAL checkpoint enabled", zap.Duration("interval", cfg.Performance.Database.CheckpointInterval), zap.Duration("idle_threshold", runtimeIdleCheckpointThreshold))
	} else {
		zapLogger.Info("Periodic WAL checkpoint disabled")
	}
	trace.Mark("config_store_ready")

	// Register cleanup for services
	registerCleanup(func() error {
		services.Close()
		return nil
	})

	// Initialize chat handler
	chatHandler := server.NewChatHandler(services.MemoryStore, services.LLMRegistry, services.ToolRegistry)
	chatHandler.SetPersistenceOptions(cfg.Session.ChatPersistAsync, cfg.Session.ChatReadLite)
	metricsReadDB := services.DB
	if services.DBConn != nil && services.DBConn.Reader != nil {
		metricsReadDB = services.DBConn.Reader
	}
	metricsCollector, metricsWriter := bootstrap.InitMetricsWithReadDB(dataDir, services.DB, metricsReadDB)
	chatHandler.SetMetricsRecorder(metricsWriter)
	// Register cleanup for metrics
	registerCleanup(func() error {
		metricsCollector.Stop()
		metricsWriter.Stop()
		return nil
	})

	if cfg.Session.Audit.Enabled {
		auditCfg := sessionaudit.StoreConfig{
			RetentionDays:      cfg.Session.Audit.RetentionDays,
			CleanupInterval:    cfg.Session.Audit.CleanupInterval,
			CleanupBatchSize:   cfg.Session.Audit.CleanupBatchSize,
			Durability:         cfg.Session.ChatDBDurability,
			WALAutoCheckpoint:  4000,
			CheckpointInterval: 0,
		}
		auditDBPath := sessionaudit.ResolveDBPath(dataDir, cfg.Session.Audit.Path)
		var (
			auditStore *sessionaudit.Store
			err        error
		)
		if mkErr := os.MkdirAll(filepath.Dir(auditDBPath), 0o750); mkErr != nil {
			zapLogger.Warn("Failed to create session audit directory", zap.String("path", auditDBPath), zap.Error(mkErr))
		} else {
			auditStore, err = sessionaudit.NewSQLiteStore(auditDBPath, auditCfg)
		}
		if err != nil {
			zapLogger.Warn("Failed to initialize session audit store", zap.String("path", auditDBPath), zap.Error(err))
		} else if auditStore != nil {
			chatHandler.SetSessionAuditStore(auditStore)
			registerCleanup(func() error {
				return auditStore.Close()
			})
			zapLogger.Info("Session tool payload audit store enabled", zap.String("path", auditDBPath), zap.Int("retention_days", cfg.Session.Audit.RetentionDays))
		}
	}
	trace.Mark("chat_metrics_ready")

	// Initialize external auth service
	extauthService, _ := extauth.NewService(&extauth.ServiceConfig{
		Providers:    []*extauth.ProviderConfig{},
		StateStore:   extauth.NewMemoryStateStore(),
		AccountStore: extauth.NewMemoryAccountStore(),
		UserStore:    nil,
	})
	extauthHandler := extauth.NewHandler(extauthService)

	// Initialize plugin registry and store
	pluginRegistry := plugin.NewRegistry()
	pluginStore := plugin.NewStore(plugin.DefaultStoreConfig(), pluginRegistry)

	// Initialize auto-reply service
	autoreplyService := autoreply.NewService(autoreply.DefaultConfig(), zapLogger)
	autoreplyHandler := autoreply.NewHandler(autoreplyService, zapLogger)

	// Initialize auth middleware and handlers
	authMiddleware := auth.NewAuthMiddleware(services.JWTService, services.APIKeyService)
	apiKeyHandler := auth.NewAPIKeyHandler(services.APIKeyService)
	userHandler := user.NewHandler(services.UserService)
	userHandler.SetJWTService(services.JWTService)

	// Initialize permission service and set on user handler
	var permRepo *permission.Repository
	if services.DBConn != nil {
		permRepo, _ = permission.NewRepositoryWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
	} else {
		permRepo, _ = permission.NewRepository(services.DB)
	}
	if permRepo != nil {
		permService := permission.NewService(permRepo, services.UserRepo)
		userHandler.SetPermissionService(permService)
	}
	trace.Mark("core_handlers_ready")

	// Initialize backup handler
	backupManager, _ := backup.NewManager(backup.Config{
		Enabled:            true,
		RetentionDays:      7,
		Path:               filepath.Join(dataDir, "backups"),
		SkillsPath:         filepath.Join(dataDir, "workspace", ".claude", "skills"),
		AutoBackup:         false,
		AutoBackupInterval: 6 * time.Hour,
		AutoBackupOnChange: false,
		ChangePollInterval: time.Minute,
		ChangeDebounce:     5 * time.Minute,
	}, dataDir, dataDir)
	var backupHandler *backup.Handler
	if backupManager != nil {
		backupHandler = backup.NewHandler(backupManager)
		backupHandler.SetRestartFunc(func() error {
			zapLogger.Info("Backup restore staged; triggering embedded graceful restart")
			return triggerEmbeddedRestart(port, dataDir, cfgFile, zapLogger)
		})
		zapLogger.Info("Backup manager initialized with runtime auto backup disabled")
	}

	// Initialize security handler
	threatDetector := security.NewThreatDetector()
	securityHandler := security.NewHandler(threatDetector)

	// Initialize sandbox handler
	var sandboxHandler *sandbox.Handler
	sandboxManager, err := bootstrap.NewSandboxManagerFromConfig(cfg)
	if err != nil {
		zapLogger.Warn("Failed to initialize sandbox manager; sandbox features will be disabled", zap.Error(err))
	} else if sandboxManager != nil && sandboxManager.IsSupported() {
		sandboxHandler = sandbox.NewHandler(sandboxManager)
	} else if sandboxManager != nil {
		zapLogger.Warn("Sandbox manager initialized without a supported isolation backend; sandbox features will remain disabled", zap.String("reason", sandboxManager.SupportReason()))
		sandboxManager = nil
	}

	// Initialize cron handler lazily so startup does not block on scheduler setup.
	var (
		cronService   *cron.Service
		cronServiceMu sync.Mutex
	)
	cronHandler := cron.NewLazyHandler(func() *cron.Service {
		svc := cron.NewService(cron.DefaultConfig(), zapLogger)
		svc.RegisterBuiltinHandlers()
		if err := svc.Start(); err != nil {
			zapLogger.Warn("Failed to start cron service", zap.Error(err))
		}
		cronServiceMu.Lock()
		cronService = svc
		cronServiceMu.Unlock()
		return svc
	}, zapLogger)
	// Register cleanup for cron service
	registerCleanup(func() error {
		cronServiceMu.Lock()
		svc := cronService
		cronServiceMu.Unlock()
		if svc != nil {
			svc.Stop(context.Background())
		}
		return nil
	})

	// Initialize browser handler
	var relayInfoProvider func() browser.RelayInfo
	var relayServer *browser.RelayServer
	relayInfoProvider = func() browser.RelayInfo {
		if relayServer == nil {
			return browser.RelayInfo{}
		}
		return relayServer.Info()
	}
	if cfg.Browser.RelayEnabled {
		extensionDir, err := browser.EnsureRelayExtensionDir(dataDir)
		if err != nil {
			zapLogger.Warn("Failed to export browser relay extension assets", zap.Error(err))
		}
		relayServer, err = browser.StartRelayServer(&cfg.Browser, extensionDir)
		if err != nil {
			zapLogger.Warn("Failed to start browser relay server", zap.Error(err))
		} else {
			info := relayServer.Info()
			zapLogger.Info("Browser relay server started",
				zap.String("base_url", info.BaseURL),
				zap.String("cdp_url", info.CDPURL),
				zap.String("extension_dir", info.ExtensionDir),
			)
			registerCleanup(func() error {
				return relayServer.Close()
			})
		}
	}

	browserHandler := browser.NewLazyHandler(func() browser.Service {
		browserService, err := browser.NewService(&cfg.Browser)
		if err != nil {
			zapLogger.Warn("Failed to initialize browser service lazily", zap.Error(err))
			return nil
		}
		return browserService
	})
	browserHandler.SetIdleReclaim(cfg.Performance.ResourceReclaim.BrowserIdleAfter)
	browserHandler.SetRelayInfoProvider(relayInfoProvider)

	// Initialize formfiller handler
	formfillerHandler := formfiller.NewLazyHandler(func() (*formfiller.Store, error) {
		formfillerStore, err := formfiller.NewStore(filepath.Join(dataDir, "formfiller"))
		if err != nil {
			zapLogger.Warn("Failed to initialize form filler store lazily", zap.Error(err))
			return nil, err
		}
		return formfillerStore, nil
	})
	trace.Mark("browser_formfiller_ready", zap.Bool("browser_handler", browserHandler != nil), zap.Bool("formfiller_handler", formfillerHandler != nil))

	// Initialize workflow handler lazily to avoid repository setup on the critical startup path.
	workflowHandler := workflow.NewLazyHandler(func() *workflow.WorkflowService {
		var (
			workflowRepo *workflow.Repository
			err          error
		)
		if services.DBConn != nil {
			workflowRepo, err = workflow.NewRepositoryWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
		} else {
			workflowRepo, err = workflow.NewRepository(services.DB)
		}
		if err != nil {
			zapLogger.Warn("Failed to initialize workflow repository", zap.Error(err))
			return nil
		}
		workflowService, err := workflow.NewService(nil, workflowRepo)
		if err != nil {
			zapLogger.Warn("Failed to initialize workflow service", zap.Error(err))
			return nil
		}
		zapLogger.Info("Workflow service initialized lazily")
		return workflowService
	})

	// Initialize Whisper ASR provider only on platforms that can actually use it during
	// embedded startup. macOS desktop relies on native STT and would otherwise pay the
	// model-manager setup cost without using Whisper on the startup path.
	var whisperASRProvider *stt.WhisperProvider
	if runtime.GOOS != "darwin" {
		whisperASRProvider = stt.NewWhisperProvider(&stt.WhisperConfig{
			ModelPath: filepath.Join(dataDir, "models", "whisper"),
		})
	}
	// Register cleanup for Whisper provider
	if whisperASRProvider != nil {
		registerCleanup(func() error {
			whisperASRProvider.Close()
			return nil
		})
	}

	// Create STT service from whisper provider
	var sttService stt.Service
	if whisperASRProvider != nil {
		sttService = stt.NewServiceWithProvider(whisperASRProvider)
		zapLogger.Info("STT service initialized with Whisper provider")
	}

	// TTS service — pick OS-appropriate default provider
	var ttsService tts.Service
	{
		defaultTTSProvider := tts.ProviderEdge
		if runtime.GOOS == "darwin" {
			defaultTTSProvider = tts.ProviderMacOSNative
		}

		providers := []tts.ProviderConfig{
			{Type: tts.ProviderEdge, Enabled: true},
		}
		if defaultTTSProvider == tts.ProviderMacOSNative {
			providers = append(providers, tts.ProviderConfig{Type: tts.ProviderMacOSNative, Enabled: true})
		}

		var err error
		ttsService, err = tts.NewService(&tts.ServiceConfig{
			DefaultProvider: defaultTTSProvider,
			Providers:       providers,
			DataPath:        dataDir,
		})
		if err != nil {
			zapLogger.Warn("Failed to initialize TTS service", zap.Error(err))
		} else {
			zapLogger.Info("TTS service initialized", zap.String("provider", string(defaultTTSProvider)))
		}
	}

	// Initialize voice handler with STT service
	voiceService := voice.NewService(&voice.ServiceConfig{
		STTService: sttService,
	})
	voiceHandler := voice.NewHandler(voiceService)

	// Initialize speech handler with ASR provider
	asrProvider := "whisper"
	if runtime.GOOS == "darwin" {
		asrProvider = "macos-native"
	} else if runtime.GOOS == "windows" {
		asrProvider = "windows-native"
	}
	speechKV := configKV
	speechService := speech.NewService(&speech.Config{
		TTS: speech.TTSConfig{Provider: "edge"},
		ASR: speech.ASRConfig{Enabled: true, Provider: asrProvider, EditBeforeSend: true},
	}, nil, ttsService)
	if ttsService != nil {
		if provider := ttsService.GetProvider(tts.ProviderEdge); provider != nil {
			speechService.SetTTSProvider(provider)
		}
	}
	speechHandler := speech.NewHandler(speechService, speechKV, dataDir)

	// Restore persisted TTS provider from kvstore
	if ttsService != nil {
		if saved := speechHandler.GetPersistedTTSProvider(); saved != "" {
			if err := ttsService.SetDefaultProvider(tts.ProviderType(saved)); err != nil {
				zapLogger.Warn("Failed to restore persisted TTS provider", zap.String("provider", saved), zap.Error(err))
			} else {
				if p := ttsService.GetProvider(tts.ProviderType(saved)); p != nil {
					speechService.SetTTSProvider(p)
				}
				zapLogger.Info("Restored persisted TTS provider", zap.String("provider", saved))
			}
		}
	}

	// On macOS, use native STT (no whisper in Tauri macOS build)
	if runtime.GOOS == "darwin" {
		status, authErr, authInitialized := speech.CurrentSTTAuthorizationState()
		switch {
		case authErr != nil:
			zapLogger.Warn("macOS native STT authorization failed before server startup", zap.Error(authErr))
			speechService.SetASRPermissionDenied(authErr.Error())
		case !authInitialized:
			err := fmt.Errorf("macOS STT authorization not initialized; call RequestSTTAuthorization() from main() first")
			zapLogger.Warn("macOS native STT authorization state missing at startup", zap.Error(err))
			speechService.SetASRPermissionDenied(err.Error())
		case status != 3:
			err := fmt.Errorf("speech recognition not authorized (status=%d)", status)
			zapLogger.Warn("macOS native STT unavailable after authorization check", zap.Error(err))
			speechService.SetASRPermissionDenied(err.Error())
		default:
			macosSTT := speech.NewMacOSNativeSTT()
			speechService.SetASRProvider(macosSTT)
			if voiceHandler != nil {
				voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(macosSTT))
			}
			zapLogger.Info("macOS native STT authorized; deferring provider initialization until first use", zap.Int("status", status))
		}
	} else if runtime.GOOS == "windows" {
		zapLogger.Info("Windows detected, initializing native ASR...")
		windowsASR := speech.NewWindowsNativeASR()
		if windowsASR != nil {
			speechService.SetASRProvider(windowsASR)
			// Also update voice service to use Windows native ASR for /voice/transcribe
			if voiceHandler != nil {
				voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(windowsASR))
			}
			zapLogger.Info("Windows native ASR initialized OK",
				zap.String("providerType", string(windowsASR.Type())),
				zap.String("providerName", windowsASR.Name()))
		} else {
			zapLogger.Warn("Windows native ASR init failed, falling back to whisper")
			if whisperASRProvider != nil {
				speechService.SetASRProvider(whisperASRProvider)
			}
		}
	} else if whisperASRProvider != nil {
		speechService.SetASRProvider(whisperASRProvider)
	}
	trace.Mark("speech_ready")

	// Initialize companion handler
	companionConfig := companion.DefaultConfig()
	companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
	companionConfig.Retention.EventsDays = cfg.Companion.Retention.EventsDays
	companionConfig.Retention.SessionsDays = cfg.Companion.Retention.SessionsDays
	companionConfig.Retention.AlertsDays = cfg.Companion.Retention.AlertsDays
	var companionHandler *companion.Handler
	var companionWSHandler *companion.WebSocketHandler
	var warmCompanion func()
	{
		var (
			companionInitMu            sync.Mutex
			companionCleanupRegistered bool
			companionManager           *companion.Manager
			companionStorage           companion.Storage
			companionStreamer          *companion.EventStreamer
		)
		initCompanion := func() (*companion.Manager, companion.Storage, companion.Streamer) {
			companionInitMu.Lock()
			defer companionInitMu.Unlock()

			if companionManager != nil && companionStorage != nil && companionStreamer != nil {
				return companionManager, companionStorage, companionStreamer
			}

			storage, err := companion.NewJSONLStorage(companionConfig.Storage.BasePath)
			if err != nil {
				zapLogger.Warn("Failed to initialize companion storage lazily", zap.Error(err))
				return nil, nil, nil
			}

			streamer := companion.NewEventStreamer(companionConfig)
			if err := streamer.Start(ctx); err != nil {
				zapLogger.Warn("Failed to start companion streamer lazily", zap.Error(err))
				return nil, nil, nil
			}

			manager := companion.NewManager(storage, streamer, companionConfig)
			chatHandler.SetCompanionManager(manager)

			if !companionCleanupRegistered {
				registerCleanup(func() error {
					return streamer.Stop()
				})
				companionCleanupRegistered = true
			}

			companionManager = manager
			companionStorage = storage
			companionStreamer = streamer

			zapLogger.Info("Companion services initialized lazily")
			return companionManager, companionStorage, companionStreamer
		}

		companionHandler = companion.NewLazyHandler(func() (*companion.Manager, companion.Storage) {
			manager, storage, _ := initCompanion()
			return manager, storage
		})
		companionWSHandler = companion.NewLazyWebSocketHandler(func() companion.Streamer {
			_, _, streamer := initCompanion()
			return streamer
		}, companionConfig)
		warmCompanion = func() {
			_, _, _ = initCompanion()
		}
	}
	trace.Mark("companion_handlers_ready")

	// Initialize provider pool (SQLite-backed, auto-migrates from JSON files)
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	providerPoolReadDB := services.DB
	if services.DBConn != nil && services.DBConn.Reader != nil {
		providerPoolReadDB = services.DBConn.Reader
	}
	ppOpts := []providerpool.PoolOption{
		providerpool.WithDB(services.DB),
		providerpool.WithReadDB(providerPoolReadDB),
	}
	if cfg.Security.Encryption.Enabled {
		secretEncryptor, encErr := auth.NewEncryptor(&auth.EncryptionConfig{
			KeyPath:    cfg.Security.Encryption.KeyPath,
			Passphrase: cfg.Security.Encryption.Passphrase,
		})
		if encErr != nil {
			zapLogger.Warn("Failed to initialize provider pool secret encryption", zap.Error(encErr))
		} else {
			ppOpts = append(ppOpts, providerpool.WithSecretEncryptor(secretEncryptor))
		}
	}
	providerPool, _ := providerpool.NewPool(providerPoolPath, ppOpts...)
	if providerPool != nil {
		bootstrap.LoadProvidersFromPool(providerPool, services.LLMRegistry)
		chatHandler.SetProviderPool(providerPool)
	}
	trace.Mark("provider_pool_ready", zap.Bool("provider_pool", providerPool != nil))

	// Initialize ngrok
	ngrokConfigStore := ngrok.NewConfigStore(configKV)
	ngrokTunnelMgr := ngrok.NewSDKTunnelManager(nil)

	// Initialize workspace (SOUL.md, USER.md, IDENTITY.md, etc.)
	workspaceMgr := workspace.NewManager(filepath.Join(dataDir, "workspace"))
	if err := workspaceMgr.EnsureWorkspace(); err != nil {
		zapLogger.Warn("Failed to initialize workspace", zap.Error(err))
	}
	// Context packs are only needed when context-aware chat features are used, so
	// release them in the background instead of blocking first paint.
	go func() {
		if err := workspaceMgr.ReleaseContextPacks(contextpackembed.PacksFS); err != nil {
			zapLogger.Warn("Failed to release embedded context packs", zap.Error(err))
		}
	}()
	contextRegistry := contextpack.NewRegistry(workspaceMgr.ContextDir())
	if result, migrateErr := contextpack.MigrateLegacyAnnotations(context.Background(), services.DB, dataDir); migrateErr != nil {
		zapLogger.Warn("Failed to migrate legacy context annotation store", zap.Error(migrateErr))
	} else if result != nil {
		fields := []zap.Field{
			zap.String("source", result.SourcePath),
			zap.Int("rows_imported", result.RowsImported),
		}
		if result.ArchivedPath != "" {
			fields = append(fields, zap.String("archived_path", result.ArchivedPath))
		}
		zapLogger.Info("Legacy context annotation store imported into blue.db", fields...)
	}
	contextAnnotationStoreFactory := func() (*contextpack.AnnotationStore, error) {
		return contextpack.NewAnnotationStoreWithDB(services.DB)
	}
	if services.DBConn != nil {
		contextAnnotationStoreFactory = func() (*contextpack.AnnotationStore, error) {
			return contextpack.NewAnnotationStoreWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
		}
	}
	contextAnnotationStore, err := contextAnnotationStoreFactory()
	if err != nil {
		zapLogger.Warn("Failed to initialize context annotation store", zap.Error(err))
	}
	if contextAnnotationStore != nil {
		registerCleanup(func() error {
			return contextAnnotationStore.Close()
		})
	}
	contextResolver := contextpack.NewResolver(contextRegistry, contextAnnotationStore, contextpack.ResolverConfig{MaxFiles: 3, MaxTokens: 1500, SearchLimit: 5})

	workspaceHandler := workspace.NewHandler(workspaceMgr)

	// Set up system prompt builder
	systemPromptBuilder := agentcore.NewSystemPromptBuilder(&agentcore.Config{
		WorkspaceDir: workspaceMgr.Dir(),
	})
	systemPromptBuilder.SetToolRegistry(services.ToolRegistry)
	systemPromptBuilder.SetWorkspace(workspaceMgr)
	systemPromptBuilder.SetContextResolver(contextResolver)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)
	trace.Mark("workspace_context_ready")

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(configKV)

	// SSE event broker
	sseBroker := ssePkg.NewBroker()

	// Push notification service (scheduled push, native OS notifications, web push)
	wpReadDB := services.DB
	if services.DBConn != nil && services.DBConn.Reader != nil {
		wpReadDB = services.DBConn.Reader
	}
	wpSender := bootstrap.InitWebPushSenderWithReadDB(services.DB, wpReadDB, configKV, zapLogger)
	pushResult := bootstrap.InitPushService(&bootstrap.PushServiceDeps{
		DB:          services.DB,
		ReadDB:      wpReadDB,
		MemoryStore: services.MemoryStore,
		CronGetSvc:  cronHandler.GetService,
		SSEBroker:   sseBroker,
		WPSender:    wpSender,
		Logger:      zapLogger,
	})
	var pushIPC sockipc.PushBackend
	var pushSvc *push.Service
	if pushResult != nil {
		pushIPC = pushResult.IPC
		pushSvc = pushResult.Service
	}
	trace.Mark("push_ready")

	// Clean up extracted web dist from tmpfs on shutdown
	registerCleanup(func() error {
		web.CleanupDist()
		return nil
	})

	// Lazy browser backend for browser tool + UI reviewer
	var lazyBrowserSvc func() *browser.RodService
	var acquireBrowserSvc func() (*browser.RodService, func(), error)
	var syncBrowserMonitorRetention func(time.Duration)
	var cleanupBrowserMonitorFrames func()
	type browserRuntime struct {
		lazy           func() *browser.RodService
		peek           func() *browser.RodService
		acquire        func() (*browser.RodService, func(), error)
		peekVisible    func() *browser.RodService
		acquireVisible func() (*browser.RodService, func(), error)
		rodBackend     *tools.RodBrowserBackend
		backend        tools.BrowserBackend
		close          func(context.Context) error
	}
	var browserRuntimePeekers []func() *browser.RodService
	var browserRuntimeRetentionUpdaters []func(time.Duration)
	initialBrowserMonitorRetention := time.Duration(cfg.Companion.Retention.SessionsDays) * 24 * time.Hour
	cfg.Browser.SessionScreenshotRetention = initialBrowserMonitorRetention
	makeBrowserRuntime := func(baseCfg *browser.Config) browserRuntime {
		headlessCfg := baseCfg.Clone()
		headlessCfg.Headless = true
		headlessRuntime := reclaim.NewManaged[*browser.RodService](
			cfg.Performance.ResourceReclaim.BrowserIdleAfter,
			func() (*browser.RodService, error) {
				return browser.NewService(headlessCfg)
			},
			func(_ context.Context, svc *browser.RodService) error {
				if svc == nil {
					return nil
				}
				return svc.Close()
			},
		)

		visibleCfg := baseCfg.Clone()
		visibleCfg.Headless = false
		visibleRuntime := reclaim.NewManaged[*browser.RodService](
			cfg.Performance.ResourceReclaim.BrowserIdleAfter,
			func() (*browser.RodService, error) {
				return browser.NewService(visibleCfg)
			},
			func(_ context.Context, svc *browser.RodService) error {
				if svc == nil {
					return nil
				}
				return svc.Close()
			},
		)

		updateRetention := func(retention time.Duration) {
			if retention < 0 {
				retention = 0
			}
			headlessCfg.SessionScreenshotRetention = retention
			visibleCfg.SessionScreenshotRetention = retention
			if svc, ok := headlessRuntime.Peek(); ok && svc != nil {
				svc.SetSessionScreenshotRetention(retention)
			}
			if svc, ok := visibleRuntime.Peek(); ok && svc != nil {
				svc.SetSessionScreenshotRetention(retention)
			}
		}

		runtime := browserRuntime{
			lazy: func() *browser.RodService {
				svc, err := headlessRuntime.Get()
				if err != nil {
					zapLogger.Warn("Failed to create browser service", zap.Error(err))
					return nil
				}
				return svc
			},
			peek: func() *browser.RodService {
				svc, ok := headlessRuntime.Peek()
				if !ok {
					return nil
				}
				return svc
			},
			acquire: headlessRuntime.Acquire,
			peekVisible: func() *browser.RodService {
				svc, ok := visibleRuntime.Peek()
				if !ok {
					return nil
				}
				return svc
			},
			acquireVisible: visibleRuntime.Acquire,
			close: func(ctx context.Context) error {
				if err := visibleRuntime.Close(ctx); err != nil {
					_ = headlessRuntime.Close(ctx)
					return err
				}
				return headlessRuntime.Close(ctx)
			},
		}
		runtime.rodBackend = tools.NewPeekLeaseAwareRodBrowserBackend(
			runtime.peek,
			runtime.acquire,
			runtime.peekVisible,
			runtime.acquireVisible,
		)
		runtime.backend = runtime.rodBackend
		registerCleanup(func() error {
			return runtime.close(context.Background())
		})
		browserRuntimePeekers = append(browserRuntimePeekers,
			func() *browser.RodService {
				svc, ok := headlessRuntime.Peek()
				if !ok {
					return nil
				}
				return svc
			},
			func() *browser.RodService {
				svc, ok := visibleRuntime.Peek()
				if !ok {
					return nil
				}
				return svc
			},
		)
		browserRuntimeRetentionUpdaters = append(browserRuntimeRetentionUpdaters, updateRetention)
		return runtime
	}

	defaultRuntime := makeBrowserRuntime(&cfg.Browser)
	managedRuntime := defaultRuntime
	if cfg.Browser.ResolvedDriver() != "managed" {
		managedRuntime = makeBrowserRuntime(cfg.Browser.CloneForDriver("managed"))
	}
	relayRuntime := defaultRuntime
	if cfg.Browser.ResolvedDriver() != "relay" {
		relayRuntime = makeBrowserRuntime(cfg.Browser.CloneForDriver("relay"))
	}
	lazyBrowserSvc = defaultRuntime.lazy
	acquireBrowserSvc = defaultRuntime.acquire
	relayPreferredSites := cfg.Browser.ExpandedRelayPreferredSites()
	lightpandaSvc := browser.NewLightpandaService(&cfg.Browser)
	var lightpandaBinaryBackend *tools.LightpandaBinaryBrowserBackend
	if cfg.Browser.Lightpanda.Enabled {
		lightpandaBinaryRuntime := browser.NewLightpandaBinaryRuntime(&cfg.Browser)
		lightpandaBinaryBackend = tools.NewLightpandaBinaryBrowserBackend(lightpandaBinaryRuntime)
		registerCleanup(func() error {
			return lightpandaBinaryRuntime.Stop(context.Background())
		})
		go func() {
			path, err := lightpandaSvc.WarmBinary(context.Background())
			if err != nil {
				zapLogger.Warn("Lightpanda binary is not ready; hybrid routing will fall back to Chromium until it becomes available", zap.Error(err))
				return
			}
			if strings.TrimSpace(path) != "" {
				zapLogger.Info("Lightpanda binary is ready for hybrid browser routing", zap.String("binary_path", path))
			}
		}()
	}
	var browserBackend tools.BrowserBackend = tools.NewHybridCapabilityBrowserBackend(
		&cfg.Browser,
		lightpandaSvc,
		lightpandaBinaryBackend,
		managedRuntime.rodBackend,
		relayRuntime.rodBackend,
		func(rawURL string) bool {
			return browser.MatchSitePatternList(rawURL, relayPreferredSites)
		},
		func(ctx context.Context) bool {
			if ctx == nil {
				ctx = context.Background()
			}
			if strings.TrimSpace(cfg.Browser.CDPURL) != "" {
				probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				return browser.ProbeCDPURL(probeCtx, cfg.Browser.CDPURL) == nil
			}
			if cfg.Browser.RelayEnabled {
				return relayInfoProvider().ExtensionConnected
			}
			return false
		},
	)
	if provider, ok := browserBackend.(browser.SessionRouteProvider); ok {
		browserHandler.SetSessionRouteProvider(provider)
	}
	syncBrowserMonitorRetention = func(retention time.Duration) {
		if retention < 0 {
			retention = 0
		}
		cfg.Browser.SessionScreenshotRetention = retention
		if browserHandler != nil {
			if svc, ok := browserHandler.PeekService().(*browser.RodService); ok && svc != nil {
				svc.SetSessionScreenshotRetention(retention)
			}
		}
		for _, updateRetention := range browserRuntimeRetentionUpdaters {
			updateRetention(retention)
		}
	}
	cleanupBrowserMonitorFrames = func() {
		seen := make(map[*browser.RodService]struct{})
		if browserHandler != nil {
			if svc, ok := browserHandler.PeekService().(*browser.RodService); ok && svc != nil {
				seen[svc] = struct{}{}
				svc.CleanupExpiredMonitorFrames()
			}
		}
		for _, peek := range browserRuntimePeekers {
			svc := peek()
			if svc == nil {
				continue
			}
			if _, ok := seen[svc]; ok {
				continue
			}
			seen[svc] = struct{}{}
			svc.CleanupExpiredMonitorFrames()
		}
	}
	syncBrowserMonitorRetention(initialBrowserMonitorRetention)
	browserIPC := sockipc.NewToolBrowserIPCAdapter(browserBackend)
	if companionHandler != nil {
		companionHandler.SetRetentionChangeHook(func(_ context.Context, retention companion.RetentionConfig) error {
			syncBrowserMonitorRetention(time.Duration(retention.SessionsDays) * 24 * time.Hour)
			return nil
		})
		companionHandler.SetCleanupHook(func(_ context.Context, retention companion.RetentionConfig) error {
			syncBrowserMonitorRetention(time.Duration(retention.SessionsDays) * 24 * time.Hour)
			cleanupBrowserMonitorFrames()
			return nil
		})
	}
	uiReviewerIPC := sockipc.NewUIReviewIPCAdapter(&tools.UIReviewerTool{})

	// Cron IPC adapter
	cronIPC := sockipc.NewCronIPCAdapter(cron.NewSkillAdapter(cronHandler.GetService))

	// Voice WebSocket handler
	var voiceWSHandler *voice.WSHandler
	if voiceHandler != nil {
		voiceWSHandler = voice.NewWSHandler(voiceHandler.Service())
	}

	// Initialize memory handler (markdown primary, optional dual-write with vector store)
	var memoryHandler *server.MemoryHandler
	memoryHandler = server.NewLazyMemoryHandler(func(h *server.MemoryHandler) error {
		memoryDir := cfg.Memory.MarkdownDir
		if memoryDir == "" {
			memoryDir = workspaceMgr.MemoryDir()
		}
		mdBackend, err := memory.NewPureMarkdownBackend(memoryDir)
		if err != nil {
			zapLogger.Warn("Failed to initialize markdown backend", zap.Error(err))
			return err
		}
		unifiedService := memory.NewUnifiedMemoryService(mdBackend)
		h.SetUnifiedService(unifiedService)

		// Try to set up dual-write backend with vector store + hybrid search
		if cfg.Memory.VectorStore.Enabled {
			if dualBackend := initDualWriteBackendLib(cfg, dataDir, mdBackend); dualBackend != nil {
				unifiedService.SetBackend(dualBackend)
				zapLogger.Info("Memory service initialized (dual-write: markdown + vector store)")
			} else {
				zapLogger.Info("Memory service initialized (markdown-only, vector store init failed)")
			}
		} else {
			zapLogger.Info("Memory service initialized (markdown backend)", zap.String("dir", memoryDir))
		}

		layeredService, err := memory.NewLayeredMemoryService(unifiedService, memory.LayeredMemoryConfig{
			BaseDir:            memoryDir,
			LongTermDir:        workspaceMgr.Dir(),
			DailyRetentionDays: 30,
		})
		if err != nil {
			zapLogger.Warn("Failed to initialize layered memory service", zap.Error(err))
		} else {
			h.SetLayeredService(layeredService)
			zapLogger.Info("Layered memory service initialized", zap.String("dir", memoryDir))
		}

		toolsAdapter := memory.NewToolsAdapter(unifiedService)
		tools.RegisterMemoryTools(services.ToolRegistry, toolsAdapter)

		return nil
	})
	// Initialize memory handler in background — it's not needed until the first
	// memory API call or chat recall, so don't block server startup.
	go memoryHandler.Init()
	trace.Mark("memory_init_started")

	// Create Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(runtimeActivity.MutationMiddleware())
	echoServer = e

	// Bind the listener BEFORE route registration so we can start serving
	// as soon as critical routes (health, system/mode) are registered.
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	ln, actualPort, err := server.ListenWithFallback(addr, cfg.Server.Port, cfg.Server.PortAutoFallback)
	if err != nil {
		return err
	}
	trace.Mark("listener_bound", zap.Int("actual_port", actualPort))

	// Propagate actual port to server/security/network packages
	server.SetActualPort(actualPort)
	security.SetServerPort(actualPort)
	network.SetDynamicPort(actualPort)

	httpServer = &http.Server{
		Handler:      e,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	errCh := make(chan error, 1)

	// Register all routes using bootstrap package
	bootstrap.RegisterAllRoutes(e, &bootstrap.RoutesDeps{
		DB:                 services.DB,
		Config:             cfg,
		ServerConfig:       serverCfg,
		Services:           services,
		Logger:             zapLogger,
		Ctx:                ctx,
		MetricsWriter:      metricsWriter,
		MetricsCollector:   metricsCollector,
		ChatHandler:        chatHandler,
		PluginRegistry:     pluginRegistry,
		PluginStore:        pluginStore,
		ExtauthService:     extauthService,
		ExtauthHandler:     extauthHandler,
		AutoreplyService:   autoreplyService,
		AutoreplyHandler:   autoreplyHandler,
		AuthMiddleware:     authMiddleware,
		APIKeyHandler:      apiKeyHandler,
		UserHandler:        userHandler,
		BackupHandler:      backupHandler,
		SecurityHandler:    securityHandler,
		SandboxHandler:     sandboxHandler,
		CronHandler:        cronHandler,
		BrowserHandler:     browserHandler,
		VoiceHandler:       voiceHandler,
		VoiceWSHandler:     voiceWSHandler,
		FormfillerHandler:  formfillerHandler,
		WorkflowHandler:    workflowHandler,
		CompanionHandler:   companionHandler,
		CompanionWSHandler: companionWSHandler,
		ProviderPool:       providerPool,
		APIKeyService:      services.APIKeyService,
		SpeechHandler:      speechHandler,
		STTService:         sttService,
		NgrokTunnelMgr:     ngrokTunnelMgr,
		NgrokConfigStore:   ngrokConfigStore,
		ChannelConfigStore: channelConfigStore,
		ConfigKV:           configKV,
		ConfigStore:        cfgStore,
		MemoryHandler:      memoryHandler,
		HotReloader:        hotReloader,
		WorkspaceHandler:   workspaceHandler,
		SSEBroker:          sseBroker,
		BrowserIPC:         browserIPC,
		UIReviewerIPC:      uiReviewerIPC,
		PushIPC:            pushIPC,
		PushService:        pushSvc,
		CronIPC:            cronIPC,
		// Consolidated init deps
		SkillEmbedFS:        skillEmbed.SkillsFS,
		SandboxManager:      sandboxManager,
		SystemPromptBuilder: systemPromptBuilder,
		LazyBrowserSvc:      lazyBrowserSvc,
		AcquireBrowserSvc:   acquireBrowserSvc,
		BrowserBackend:      browserBackend,
		// Start serving as soon as critical routes are registered.
		// This lets the Tauri health poll succeed while heavy subsystems
		// (media, skills, IPC) are still initializing.
		OnEarlyReady: func() {
			trace.Mark("routes_early_ready", zap.Int("actual_port", actualPort))
			zapLogger.Info("Critical routes ready, starting HTTP server early",
				zap.String("addr", addr), zap.Int("actual_port", actualPort))
			go func() {
				if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()
		},
	})

	if warmCompanion != nil {
		go warmCompanion()
	}
	trace.Mark("routes_registered")

	zapLogger.Info("All routes registered", zap.Int("actual_port", actualPort))

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		zapLogger.Info("Shutting down server...")

		// Cancel all active SSE streams and close WebSocket connections first,
		// so httpServer.Shutdown() doesn't have to wait for them to time out.
		chatHandler.Shutdown()
		if companionWSHandler != nil {
			companionWSHandler.Close()
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), embeddedServerShutdownGracePeriod)
		defer cancel()

		metricsCollector.Stop()
		metricsWriter.Stop()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			if shutdownCtx.Err() != nil {
				zapLogger.Warn("Graceful HTTP shutdown timed out; forcing close",
					zap.Duration("timeout", embeddedServerShutdownGracePeriod),
					zap.Error(err),
				)
				if closeErr := httpServer.Close(); closeErr != nil && closeErr != http.ErrServerClosed {
					return fmt.Errorf("force close http server: %w", closeErr)
				}
			} else if err != http.ErrServerClosed {
				return err
			}
		}
		if err := dbutil.MarkStartupIntegrityClean(dataDir); err != nil {
			zapLogger.Warn("Failed to mark startup integrity state clean", zap.Error(err))
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// Required for c-archive build mode
func main() {}

func resolveVectorStoreDBPath(dataDir, configuredPath string) string {
	return embedding.ResolveVectorStoreDBPath(dataDir, configuredPath)
}

// initDualWriteBackendLib creates a DualWriteBackend with VectorStore + HybridSearcher.
// Returns nil if initialization fails (caller should fall back to markdown-only).
func initDualWriteBackendLib(cfg *config.Config, dataDir string, mdBackend *memory.PureMarkdownBackend) *memory.DualWriteBackend {
	log := logger.Get()

	dbPath := resolveVectorStoreDBPath(dataDir, cfg.Memory.VectorStore.DBPath)

	dims := cfg.Memory.VectorStore.Dimensions

	// Create cybertron embedding provider (lazy — model downloads on first use)
	embCfg := cfg.Embedding
	modelsDir := embedding.PrepareSharedModelCache(dataDir, cfg.Memory.VectorStore.DBPath, embCfg.Model)
	model := embCfg.Model
	if model == "" {
		model = embedding.DefaultCybertronModel
	}
	embProvider := embedding.NewCybertronProvider(embedding.CybertronConfig{
		ModelsDir:  modelsDir,
		Model:      model,
		Dimensions: embCfg.Dimensions,
	})

	if dims <= 0 {
		if embCfg.Dimensions > 0 {
			dims = embCfg.Dimensions
		} else {
			dims = 512 // BGE-small-zh default
		}
	}

	log.Info().
		Str("provider", embProvider.Name()).
		Str("model", embProvider.Model()).
		Msg("Embedding provider configured (lazy load)")

	vs, err := memory.NewVectorStore(memory.VectorStoreConfig{
		DBPath:       dbPath,
		EmbeddingDim: dims,
		MaxChunks:    10000,
		EnableFTS:    true,
	})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize vector store")
		return nil
	}

	searcher := memory.NewHybridSearcher(vs, embProvider, cfg.Memory)
	memSvc := memory.NewMemoryService(searcher)
	return memory.NewDualWriteBackend(mdBackend, memSvc)
}
