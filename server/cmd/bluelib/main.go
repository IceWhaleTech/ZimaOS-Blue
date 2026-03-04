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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
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
	version   = "0.10.31"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func applyPendingBackupRestore(dataDir string) error {
	mgr, err := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(dataDir, "backups"),
		SkillsPath:    filepath.Join(dataDir, "workspace", ".claude", "skills"),
	}, dataDir, dataDir)
	if err != nil {
		return err
	}
	if !mgr.HasPendingRestore() {
		return nil
	}
	_, err = mgr.ApplyPendingRestore(context.Background())
	return err
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
	case <-time.After(5 * time.Second):
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
	// Tune GC for lower memory usage (shared with blue CLI)
	bootstrap.TuneGC()

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize HotReloader for config changes
	var hotReloader *config.HotReloader
	hotReloader, _ = config.NewHotReloader(cfgFile, cfg, &config.HotReloadConfig{
		Enabled:             true,
		WatchInterval:       5 * time.Second,
		ValidateBeforeApply: true,
	})

	// Override port if specified
	if port > 0 {
		cfg.Server.Port = port
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize logger with ring buffer for log viewing
	if err := logger.Init(&cfg.Log); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize zap logger
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer zapLogger.Sync()

	zapLogger.Info("Starting ZimaOS-Blue (embedded)",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit),
		zap.Int("port", cfg.Server.Port),
		zap.String("data_dir", dataDir),
	)
	if err := applyPendingBackupRestore(dataDir); err != nil {
		zapLogger.Warn("Failed to apply pending backup restore before database initialization", zap.Error(err))
	}

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

	// Shared kvstore for all config persistence
	sqliteKV, err := kvstore.NewSQLiteStoreWithDB(services.DB)
	if err != nil {
		return fmt.Errorf("failed to initialize config kvstore: %w", err)
	}
	configKV := kvstore.NewCachedStore(sqliteKV)

	// Import config into kvstore (first-run: imports; subsequent: loads from DB)
	cfgStore := config.NewConfigStore(configKV)
	if updatedCfg, err := cfgStore.LoadOrImport(cfg); err == nil {
		cfg = updatedCfg
	}

	// Register cleanup for services
	registerCleanup(func() error {
		services.Close()
		return nil
	})

	// Initialize metrics
	metricsCollector, metricsWriter := bootstrap.InitMetrics(dataDir)
	defer metricsCollector.Stop()
	defer metricsWriter.Stop()
	// Register cleanup for metrics
	registerCleanup(func() error {
		metricsCollector.Stop()
		metricsWriter.Stop()
		return nil
	})

	// Initialize chat handler
	chatHandler := server.NewChatHandler(services.MemoryStore, services.LLMRegistry, services.ToolRegistry)
	chatHandler.SetMetricsRecorder(metricsWriter)

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
	permRepo, _ := permission.NewRepository(services.DB)
	if permRepo != nil {
		permService := permission.NewService(permRepo, services.UserRepo)
		userHandler.SetPermissionService(permService)
	}

	// Initialize backup handler
	backupManager, _ := backup.NewManager(backup.Config{
		Enabled:            true,
		RetentionDays:      7,
		Path:               filepath.Join(dataDir, "backups"),
		SkillsPath:         filepath.Join(dataDir, "workspace", ".claude", "skills"),
		AutoBackup:         true,
		AutoBackupInterval: 6 * time.Hour,
		AutoBackupOnChange: true,
		ChangePollInterval: time.Minute,
		ChangeDebounce:     5 * time.Minute,
	}, dataDir, dataDir)
	var backupHandler *backup.Handler
	if backupManager != nil {
		backupManager.StartAutoBackup(ctx)
		backupHandler = backup.NewHandler(backupManager)
		backupHandler.SetRestartFunc(func() error {
			zapLogger.Info("Backup restore staged; triggering embedded graceful restart")
			return triggerEmbeddedRestart(port, dataDir, cfgFile, zapLogger)
		})
	}

	// Initialize security handler
	threatDetector := security.NewThreatDetector()
	securityHandler := security.NewHandler(threatDetector)

	// Initialize sandbox handler
	sandboxManager, _ := sandbox.NewManager(nil)
	var sandboxHandler *sandbox.Handler
	if sandboxManager != nil {
		sandboxHandler = sandbox.NewHandler(sandboxManager)
	}

	// Initialize cron handler
	cronService := cron.NewService(cron.DefaultConfig(), zapLogger)
	cronService.RegisterBuiltinHandlers()
	cronHandler := cron.NewHandler(cronService, zapLogger)
	// Defer cron start — not needed until a scheduled job fires
	go func() {
		cronService.Start()
	}()
	// Register cleanup for cron service
	registerCleanup(func() error {
		cronService.Stop(context.Background())
		return nil
	})

	// Initialize Home Assistant handler
	haService := homeassistant.NewHAService()
	haHandler := homeassistant.NewHandler(haService)

	// Initialize browser handler
	browserService, _ := browser.NewService(nil)
	var browserHandler *browser.Handler
	if browserService != nil {
		browserHandler = browser.NewHandler(browserService)
	}

	// Initialize formfiller handler
	formfillerStore, _ := formfiller.NewStore(filepath.Join(dataDir, "formfiller"))
	var formfillerHandler *formfiller.Handler
	if formfillerStore != nil {
		formfillerHandler = formfiller.NewHandler(formfillerStore)
	}

	// Initialize workflow handler
	workflowRepo, _ := workflow.NewRepository(services.DB)
	var workflowHandler *workflow.Handler
	if workflowRepo != nil {
		workflowService, _ := workflow.NewService(nil, workflowRepo)
		if workflowService != nil {
			workflowHandler = workflow.NewHandler(workflowService)
		}
	}

	// Initialize Whisper ASR provider
	whisperASRProvider := stt.NewWhisperProvider(&stt.WhisperConfig{
		ModelPath: filepath.Join(dataDir, "whisper-models"),
	})
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
		zapLogger.Info("macOS detected, initializing native STT...")
		macosSTT := speech.NewMacOSNativeSTT()
		if err := macosSTT.Initialize(); err == nil {
			speechService.SetASRProvider(macosSTT)
			if voiceHandler != nil {
				voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(macosSTT))
			}
			zapLogger.Info("macOS native STT initialized OK")
		} else {
			zapLogger.Warn("macOS native STT init failed, no ASR available", zap.Error(err))
			speechService.SetASRPermissionDenied(err.Error())
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

	// Initialize companion handler
	companionConfig := companion.DefaultConfig()
	companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
	companionStorage, _ := companion.NewJSONLStorage(companionConfig.Storage.BasePath)
	var companionHandler *companion.Handler
	var companionWSHandler *companion.WebSocketHandler
	if companionStorage != nil {
		companionStreamer := companion.NewEventStreamer(companionConfig)
		// Defer streamer start — not needed until companion WebSocket connects
		go companionStreamer.Start(ctx)
		companionManager := companion.NewManager(companionStorage, companionStreamer, companionConfig)
		companionHandler = companion.NewHandler(companionManager, companionStorage)
		companionWSHandler = companion.NewWebSocketHandler(companionStreamer, companionConfig)
		chatHandler.SetCompanionManager(companionManager)
		// Register cleanup for companion streamer
		registerCleanup(func() error {
			companionStreamer.Stop()
			return nil
		})
	}

	// Initialize provider pool (SQLite-backed, auto-migrates from JSON files)
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	providerPool, _ := providerpool.NewPool(providerPoolPath, providerpool.WithDB(services.DB))
	if providerPool != nil {
		bootstrap.LoadProvidersFromPool(providerPool, services.LLMRegistry)
		chatHandler.SetProviderPool(providerPool)
	}

	// Initialize ngrok
	ngrokConfigStore := ngrok.NewConfigStore(configKV)
	ngrokTunnelMgr := ngrok.NewSDKTunnelManager(nil)

	// Initialize workspace (SOUL.md, USER.md, IDENTITY.md, etc.)
	// Defer file creation to background — not needed until chat starts
	workspaceMgr := workspace.NewManager(filepath.Join(dataDir, "workspace"))
	go func() {
		if err := workspaceMgr.EnsureWorkspace(); err != nil {
			zapLogger.Warn("Failed to initialize workspace", zap.Error(err))
		}
	}()

	// Initialize claudecode handler
	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir, configKV)

	// Set up system prompt builder
	systemPromptBuilder := claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{
		WorkspaceDir: workspaceMgr.Dir(),
	})
	systemPromptBuilder.SetToolRegistry(services.ToolRegistry)
	systemPromptBuilder.SetWorkspace(workspaceMgr)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(configKV)

	// SSE event broker
	sseBroker := ssePkg.NewBroker()

	// Push notification service (scheduled push, native OS notifications, web push)
	wpSender := bootstrap.InitWebPushSender(services.DB, configKV, zapLogger)
	pushResult := bootstrap.InitPushService(&bootstrap.PushServiceDeps{
		DB:          services.DB,
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

	// Clean up extracted web dist from tmpfs on shutdown
	registerCleanup(func() error {
		web.CleanupDist()
		return nil
	})

	// Lazy browser backend for browser tool + UI reviewer
	var lazyBrowserSvc func() *browser.RodService
	{
		var browserOnce sync.Once
		var browserSvc *browser.RodService
		lazyBrowserSvc = func() *browser.RodService {
			browserOnce.Do(func() {
				svc, err := browser.NewService(nil)
				if err != nil {
					return
				}
				browserSvc = svc
			})
			return browserSvc
		}
	}
	browserBackend := tools.NewLazyRodBrowserBackend(lazyBrowserSvc)
	browserIPC := sockipc.NewToolBrowserIPCAdapter(browserBackend)
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
			if dualBackend := initDualWriteBackendLib(cfg, mdBackend); dualBackend != nil {
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

	// Create Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	echoServer = e

	// Bind the listener BEFORE route registration so we can start serving
	// as soon as critical routes (health, system/mode) are registered.
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	ln, actualPort, err := server.ListenWithFallback(addr, cfg.Server.Port, cfg.Server.PortAutoFallback)
	if err != nil {
		return err
	}

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
		HAHandler:          haHandler,
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
		ClaudeCodeHandler:  claudeCodeHandler,
		ChannelConfigStore: channelConfigStore,
		ConfigKV:           configKV,
		ConfigStore:        cfgStore,
		MemoryHandler:      memoryHandler,
		HotReloader:        hotReloader,
		WorkspaceHandler:   workspace.NewHandler(workspaceMgr),
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
		BrowserBackend:      browserBackend,
		// Start serving as soon as critical routes are registered.
		// This lets the Tauri health poll succeed while heavy subsystems
		// (media, skills, IPC) are still initializing.
		OnEarlyReady: func() {
			zapLogger.Info("Critical routes ready, starting HTTP server early",
				zap.String("addr", addr), zap.Int("actual_port", actualPort))
			go func() {
				if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()
		},
	})

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

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		metricsCollector.Stop()
		metricsWriter.Stop()

		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Required for c-archive build mode
func main() {}

// initDualWriteBackendLib creates a DualWriteBackend with VectorStore + HybridSearcher.
// Returns nil if initialization fails (caller should fall back to markdown-only).
func initDualWriteBackendLib(cfg *config.Config, mdBackend *memory.PureMarkdownBackend) *memory.DualWriteBackend {
	log := logger.Get()

	dbPath := cfg.Memory.VectorStore.DBPath
	if dbPath == "" {
		dbPath = "./data/memory.db"
	}

	dims := cfg.Memory.VectorStore.Dimensions

	// Create cybertron embedding provider (lazy — model downloads on first use)
	embCfg := cfg.Embedding
	modelsDir := filepath.Join(filepath.Dir(dbPath), "models")
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
