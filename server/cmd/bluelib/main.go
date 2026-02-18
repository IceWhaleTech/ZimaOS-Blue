package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"net"
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
	"go.uber.org/zap"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

var (
	version   = "0.10.28"
	buildTime = "unknown"
	gitCommit = "unknown"
)

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

			// Trigger server stop via BlueServerStop
			serverMu.Lock()
			if isRunning && serverCancel != nil {
				serverCancel()
				serverMu.Unlock()

				// Wait for server to stop with timeout
				select {
				case <-serverDone:
					fmt.Fprintf(os.Stderr, "Server stopped gracefully\n")
				case <-time.After(10 * time.Second):
					fmt.Fprintf(os.Stderr, "Server shutdown timeout - forcing cleanup\n")
				}

				// Perform cleanup
				performCleanup()
			} else {
				serverMu.Unlock()
			}

			// For CGO library, we need to exit the process
			// But we've already done cleanup above
			os.Exit(0)
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

	ctx, cancel := context.WithCancel(context.Background())
	serverCancel = cancel
	serverDone = make(chan struct{})

	go func() {
		defer close(serverDone)
		if err := runServer(ctx, goPort, goDataDir, cfgFile); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	isRunning = true
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

	ctx, cancel := context.WithCancel(context.Background())
	serverCancel = cancel
	serverDone = make(chan struct{})

	go func() {
		defer close(serverDone)
		if err := runServer(ctx, goPort, goDataDir, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	isRunning = true
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
	case <-time.After(10 * time.Second):
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

//export BlueServerFreeString
func BlueServerFreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

//export BlueServerCleanup
func BlueServerCleanup() {
	performCleanup()
}

func runServer(ctx context.Context, port int, dataDir string, cfgFile string) error {
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
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(dataDir, "backups"),
	}, dataDir, dataDir)
	var backupHandler *backup.Handler
	if backupManager != nil {
		backupHandler = backup.NewHandler(backupManager)
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
	cronService.Start()
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

	// Initialize voice handler with STT service
	voiceService := voice.NewService(&voice.ServiceConfig{
		STTService: sttService,
	})
	voiceHandler := voice.NewHandler(voiceService)

	// Initialize speech handler with ASR provider
	speechService := speech.NewService(&speech.Config{
		TTS: speech.TTSConfig{Provider: "edge-tts"},
		ASR: speech.ASRConfig{Enabled: true, Provider: "whisper"},
	}, nil, nil)
	if whisperASRProvider != nil {
		speechService.SetASRProvider(whisperASRProvider)
	}
	speechHandler := speech.NewHandler(speechService, nil)

	// Initialize companion handler
	companionConfig := companion.DefaultConfig()
	companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
	companionStorage, _ := companion.NewJSONLStorage(companionConfig.Storage.BasePath)
	var companionHandler *companion.Handler
	var companionWSHandler *companion.WebSocketHandler
	if companionStorage != nil {
		companionStreamer := companion.NewEventStreamer(companionConfig)
		companionStreamer.Start(ctx)
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

	// Initialize provider pool
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	providerPool, _ := providerpool.NewPool(providerPoolPath)
	if providerPool != nil {
		bootstrap.LoadProvidersFromPool(providerPool, services.LLMRegistry)
		chatHandler.SetProviderPool(providerPool)
	}

	// Initialize ngrok
	ngrokConfigStore := ngrok.NewConfigStore(dataDir)
	ngrokTunnelMgr := ngrok.NewSDKTunnelManager(nil)

	// Initialize claudecode handler
	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir)

	// Set up system prompt builder
	systemPromptBuilder := claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{
		WorkspaceDir: dataDir,
	})
	systemPromptBuilder.SetToolRegistry(services.ToolRegistry)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(dataDir)

	// Initialize shared cache
	sharedCache := proxy.NewCCCache(proxy.DefaultCacheConfig())

	// Initialize memory handler with lazy init (vector_memory.db created on first request)
	var memoryHandler *server.MemoryHandler
	memoryHandler = server.NewLazyMemoryHandler(func(h *server.MemoryHandler) error {
		vectorDbPath := filepath.Join(dataDir, "vector_memory.db")
		vectorStore, err := memory.NewVectorStore(memory.VectorStoreConfig{
			DBPath:       vectorDbPath,
			EmbeddingDim: 1536,
			MaxChunks:    10000,
			EnableFTS:    true,
			EnableVec:    true,
		})
		if err != nil {
			zapLogger.Warn("Failed to initialize vector store", zap.Error(err))
			return err
		}
		hybridSearcher := memory.NewHybridSearcher(vectorStore, nil, cfg.Memory)
		memoryService := memory.NewMemoryService(hybridSearcher)
		unifiedService := memory.NewUnifiedMemoryService(memoryService, cfg.Memory)
		h.SetService(memoryService)
		h.SetUnifiedService(unifiedService)

		// Initialize LayeredMemoryService for dual-layer memory architecture
		memoryDir := filepath.Join(dataDir, "memory")
		layeredService, err := memory.NewLayeredMemoryService(unifiedService, memory.LayeredMemoryConfig{
			BaseDir:            memoryDir,
			DailyRetentionDays: 30,
		})
		if err != nil {
			zapLogger.Warn("Failed to initialize layered memory service", zap.Error(err))
		} else {
			h.SetLayeredService(layeredService)
			zapLogger.Info("Layered memory service initialized", zap.String("dir", memoryDir))
		}

		// Register memory tools for AI agent access
		toolsAdapter := memory.NewToolsAdapter(unifiedService)
		tools.RegisterMemoryTools(services.ToolRegistry, toolsAdapter)
		zapLogger.Info("Vector memory store initialized lazily")
		return nil
	})

	// Create Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	echoServer = e

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
		FormfillerHandler:  formfillerHandler,
		WorkflowHandler:    workflowHandler,
		CompanionHandler:   companionHandler,
		CompanionWSHandler: companionWSHandler,
		ProviderPool:       providerPool,
		APIKeyService:      services.APIKeyService,
		SpeechHandler:      speechHandler,
		NgrokTunnelMgr:     ngrokTunnelMgr,
		NgrokConfigStore:   ngrokConfigStore,
		ClaudeCodeHandler:  claudeCodeHandler,
		ChannelConfigStore: channelConfigStore,
		SharedCache:        sharedCache,
		MemoryHandler:      memoryHandler,
		HotReloader:        hotReloader,
	})

	// Start HTTP server with explicit listener (to capture actual port)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Get actual port and propagate to server/security/network packages
	actualPort := ln.Addr().(*net.TCPAddr).Port
	server.SetActualPort(actualPort)
	security.SetServerPort(actualPort)
	network.SetDynamicPort(actualPort)

	httpServer = &http.Server{
		Handler:      e,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	zapLogger.Info("Starting HTTP server", zap.String("addr", addr), zap.Int("actual_port", actualPort))

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		zapLogger.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
