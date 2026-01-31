package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/a2ui"
	networkapi "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/web"
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
	echoServer   *echo.Echo
	httpServer   *http.Server
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

	// Use provided data directory or default
	if dataDir == "" {
		dataDir = "./data"
	}

	// Initialize zap logger
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer zapLogger.Sync()

	zapLogger.Info("Starting ZimaOS-Echo (embedded)",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit),
		zap.Int("port", cfg.Server.Port),
		zap.String("data_dir", dataDir),
	)

	// Initialize database
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "echo.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Configure database
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	// Initialize user repository and service
	userRepo, err := user.NewSQLiteRepository(db)
	if err != nil {
		return fmt.Errorf("failed to initialize user repository: %w", err)
	}

	passwordHasher := password.NewHasher(&password.Config{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	})

	passwordPolicy := password.NewPolicy(&password.PolicyConfig{
		MinLength:        cfg.Security.Password.MinLength,
		RequireUppercase: cfg.Security.Password.RequireUppercase,
		RequireLowercase: cfg.Security.Password.RequireLowercase,
		RequireNumber:    cfg.Security.Password.RequireNumber,
		RequireSpecial:   cfg.Security.Password.RequireSpecial,
	})

	userService := user.NewService(userRepo, passwordHasher, passwordPolicy, nil)
	userHandler := user.NewHandler(userService)

	// Initialize memory store
	memoryDbPath := filepath.Join(dataDir, "memory.db")
	memoryStore, err := memory.NewStore(memoryDbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize memory store: %w", err)
	}

	// Initialize LLM provider registry
	llmRegistry := llm.NewProviderRegistry()

	// Register LLM providers from environment variables
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		llmRegistry.Register(llm.NewOpenAIProvider(openaiKey, ""))
	} else {
		llmRegistry.Register(llm.NewOpenAIProvider("", ""))
	}

	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	claudeBaseURL := ""
	if cfg.ClaudeCode.APIKey != "" {
		claudeKey = cfg.ClaudeCode.APIKey
	}
	if cfg.ClaudeCode.BaseURL != "" {
		claudeBaseURL = cfg.ClaudeCode.BaseURL
	}
	if claudeKey != "" {
		llmRegistry.Register(llm.NewClaudeProvider(claudeKey, claudeBaseURL))
	} else {
		llmRegistry.Register(llm.NewClaudeProvider("", ""))
	}

	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	llmRegistry.Register(llm.NewOllamaProvider(ollamaURL))

	customKey := os.Getenv("CUSTOM_API_KEY")
	customURL := os.Getenv("CUSTOM_API_URL")
	llmRegistry.Register(llm.NewCustomProvider(customKey, customURL))

	grokKey := os.Getenv("GROK_API_KEY")
	if grokKey != "" {
		llmRegistry.Register(llm.NewGrokProvider(grokKey, ""))
	} else {
		llmRegistry.Register(llm.NewGrokProvider("", ""))
	}

	qwenKey := os.Getenv("QWEN_API_KEY")
	if qwenKey != "" {
		llmRegistry.Register(llm.NewQwenProvider(qwenKey, ""))
	} else {
		llmRegistry.Register(llm.NewQwenProvider("", ""))
	}

	// Initialize tools registry
	toolRegistry := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)

	// Initialize skill registry
	skillRegistry := skill.NewRegistry()
	builtin.RegisterAll(skillRegistry)

	// Initialize plugin registry
	pluginRegistry := plugin.NewRegistry()
	pluginStore := plugin.NewStore(plugin.DefaultStoreConfig(), pluginRegistry)

	// Initialize chat handler
	chatHandler := server.NewChatHandler(memoryStore, llmRegistry, toolRegistry)

	// Initialize external auth service
	extauthService, _ := extauth.NewService(&extauth.ServiceConfig{
		Providers:    []*extauth.ProviderConfig{},
		StateStore:   extauth.NewMemoryStateStore(),
		AccountStore: extauth.NewMemoryAccountStore(),
		UserStore:    nil,
	})
	extauthHandler := extauth.NewHandler(extauthService)

	// Initialize JWT service
	jwtService := auth.NewJWTService(&auth.JWTConfig{
		Secret:            cfg.Security.JWT.Secret,
		Expiration:        cfg.Security.JWT.Expiration,
		RefreshExpiration: cfg.Security.JWT.RefreshExpiration,
		Issuer:            cfg.Security.JWT.Issuer,
	})

	// Initialize API Key service
	apiKeyDbPath := filepath.Join(dataDir, "apikeys.db")
	apiKeyService, err := auth.NewAPIKeyService(apiKeyDbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize API key service: %w", err)
	}
	defer apiKeyService.Close()

	authMiddleware := auth.NewAuthMiddleware(jwtService, apiKeyService)
	apiKeyHandler := auth.NewAPIKeyHandler(apiKeyService)
	userHandler.SetJWTService(jwtService)

	// Initialize auto-reply service
	autoreplyService := autoreply.NewService(autoreply.DefaultConfig(), zapLogger)
	autoreplyHandler := autoreply.NewHandler(autoreplyService, zapLogger)

	// Initialize services in parallel
	var (
		metricsCollector   *metrics.Collector
		metricsWriter      *metrics.MetricsWriter
		backupManager      *backup.Manager
		backupHandler      *backup.Handler
		threatDetector     *security.ThreatDetector
		securityHandler    *security.Handler
		mfaHandler         *mfa.Handler
		sandboxManager     *sandbox.Manager
		sandboxHandler     *sandbox.Handler
		cronService        *cron.Service
		cronHandler        *cron.Handler
		haService          *homeassistant.HAService
		haHandler          *homeassistant.Handler
		browserService     *browser.RodService
		browserHandler     *browser.Handler
		a2uiManager        *a2ui.Manager
		a2uiHandler        *a2ui.Handler
		sttService         stt.Service
		ttsService         tts.Service
		voiceHandler       *voice.Handler
		workflowRepo       *workflow.Repository
		workflowHandler    *workflow.Handler
		formfillerStore    *formfiller.Store
		formfillerHandler  *formfiller.Handler
		companionStorage   *companion.JSONLStorage
		companionHandler   *companion.Handler
		companionWSHandler *companion.WebSocketHandler
		companionManager   *companion.Manager
	)

	var initWg sync.WaitGroup

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		metricsCollector = metrics.NewCollector(5*time.Second, 120)
		metricsCollector.Start()
		metricsWriter = metrics.NewMetricsWriter(nil, metrics.DefaultWriterConfig())
		metricsWriter.Start()
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		backupManager, _ = backup.NewManager(backup.Config{
			Enabled:       true,
			RetentionDays: 7,
			Path:          filepath.Join(dataDir, "backups"),
		}, dataDir, dataDir)
		if backupManager != nil {
			backupHandler = backup.NewHandler(backupManager)
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		threatDetector = security.NewThreatDetector()
		securityHandler = security.NewHandler(threatDetector)
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		mfaHandler = mfa.NewHandler(nil, nil)
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		sandboxManager, _ = sandbox.NewManager(nil)
		if sandboxManager != nil {
			sandboxHandler = sandbox.NewHandler(sandboxManager)
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		cronService = cron.NewService(cron.DefaultConfig(), zapLogger)
		cronService.RegisterBuiltinHandlers()
		cronHandler = cron.NewHandler(cronService, zapLogger)
		cronService.Start()
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		haService = homeassistant.NewHAService()
		haHandler = homeassistant.NewHandler(haService)
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		browserService, _ = browser.NewService(nil)
		if browserService != nil {
			browserHandler = browser.NewHandler(browserService)
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		a2uiManager = a2ui.NewManager(zapLogger)
		a2uiHandler = a2ui.NewHandler(a2uiManager)
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		sttService, _ = stt.NewService(&stt.ServiceConfig{
			DefaultProvider: stt.ProviderWhisperAPI,
			Providers: []stt.ProviderConfig{
				{
					Type:    stt.ProviderWhisperAPI,
					Enabled: true,
					APIKey:  os.Getenv("OPENAI_API_KEY"),
				},
			},
		})
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		ttsService, _ = tts.NewService(&tts.ServiceConfig{
			DefaultProvider: tts.ProviderEdge,
			Providers: []tts.ProviderConfig{
				{Type: tts.ProviderEdge, Enabled: true},
				{Type: tts.ProviderOpenAI, Enabled: os.Getenv("OPENAI_API_KEY") != "", APIKey: os.Getenv("OPENAI_API_KEY")},
			},
		})
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		workflowRepo, _ = workflow.NewRepository(db)
		if workflowRepo != nil {
			workflowService, _ := workflow.NewService(nil, workflowRepo)
			if workflowService != nil {
				workflowHandler = workflow.NewHandler(workflowService)
			}
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		formfillerStore, _ = formfiller.NewStore(filepath.Join(dataDir, "formfiller"))
		if formfillerStore != nil {
			formfillerHandler = formfiller.NewHandler(formfillerStore)
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		companionConfig := companion.DefaultConfig()
		companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
		companionStorage, _ = companion.NewJSONLStorage(companionConfig.Storage.BasePath)
		if companionStorage != nil {
			companionStreamer := companion.NewEventStreamer(companionConfig)
			companionStreamer.Start(ctx)
			companionManager = companion.NewManager(companionStorage, companionStreamer, companionConfig)
			companionHandler = companion.NewHandler(companionManager, companionStorage)
			companionWSHandler = companion.NewWebSocketHandler(companionStreamer, companionConfig)
		}
	}()

	initWg.Wait()

	// Initialize voice handler
	if sttService != nil && ttsService != nil {
		voiceService := voice.NewService(&voice.ServiceConfig{
			STTService: sttService,
			TTSService: ttsService,
		})
		voiceHandler = voice.NewHandler(voiceService)
	}

	// Set metrics recorder on chat handler
	if metricsWriter != nil {
		chatHandler.SetMetricsRecorder(metricsWriter)
	}

	// Create Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Add middleware
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Store server reference
	echoServer = e

	// Register health endpoint
	e.GET("/api/v1/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": "zimaos-echo",
			"version": version,
		})
	})

	// Serve embedded frontend
	web.RegisterStaticRoutes(e)

	// API v1 group
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	// Public auth routes
	v1.POST("/auth/login", userHandler.Login)
	v1.POST("/auth/logout", userHandler.Logout)

	authGroup := v1.Group("/auth")
	extauthHandler.RegisterRoutes(authGroup)

	// Protected routes
	protected := v1.Group("")
	protected.Use(authMiddleware.Authenticate())

	protectedAuthGroup := protected.Group("/auth")
	extauthHandler.RegisterProtectedRoutes(protectedAuthGroup)

	mfaHandler.RegisterRoutes(protected)

	// User routes
	usersGroup := protected.Group("/users")
	usersGroup.GET("/me", userHandler.GetCurrentUser)
	usersGroup.PUT("/me", userHandler.UpdateCurrentUser)
	usersGroup.GET("", userHandler.ListUsers)
	usersGroup.POST("", userHandler.CreateUser)
	usersGroup.GET("/:id", userHandler.GetUser)
	usersGroup.PUT("/:id", userHandler.UpdateUser)
	usersGroup.DELETE("/:id", userHandler.DeleteUser)
	usersGroup.POST("/:id/lock", userHandler.LockUser)
	usersGroup.POST("/:id/unlock", userHandler.UnlockUser)

	protected.POST("/auth/password", userHandler.ChangePassword)

	apiKeysGroup := protected.Group("/apikeys")
	apiKeyHandler.RegisterRoutes(apiKeysGroup)

	// Set companion manager and prompt guard
	if companionManager != nil {
		chatHandler.SetCompanionManager(companionManager)
	}
	promptGuard := promptguard.NewDetector(promptguard.DefaultDetectorConfig())
	chatHandler.SetPromptGuard(promptGuard)

	// Register chat routes
	chatHandler.RegisterRoutes(v1)
	autoreplyHandler.RegisterRoutes(v1)

	// Register network routes
	networkHandler := networkapi.NewNetworkHandler(cfg.Server.Port)
	networkHandler.RegisterRoutes(e)

	// Register metrics routes
	if metricsCollector != nil {
		metricsHandler := server.NewMetricsHandler(metricsCollector)
		metricsHandler.RegisterRoutes(v1)
	}
	if metricsWriter != nil {
		detailedMetricsHandler := metrics.NewHandler(metricsWriter)
		metricsGroup := v1.Group("/metrics")
		detailedMetricsHandler.RegisterRoutes(metricsGroup)
	}

	// Register system routes
	systemHandler := server.NewSystemHandler(version, buildTime, gitCommit, dataDir)
	systemHandler.RegisterRoutes(v1)

	serviceHandler := server.NewServiceHandler()
	serviceHandler.RegisterRoutes(v1)

	if backupHandler != nil {
		backupHandler.RegisterRoutes(v1)
	}

	skillHandler := server.NewSkillHandler(skillRegistry)

	// Initialize skill store and sync service
	skillStore, err := skillstore.NewStore(db)
	if err != nil {
		zapLogger.Warn("failed to initialize skill store, skill sync disabled", zap.Error(err))
	} else {
		skillHandler.SetStore(skillStore)

		// Create sync service with slog logger
		slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		syncConfig := skillstore.DefaultSyncServiceConfig()
		syncService := skillstore.NewSyncService(skillStore, syncConfig, slogLogger)
		skillHandler.SetSyncService(syncService)

		// Start sync service (will sync on startup if not synced today)
		syncService.Start(ctx)
		defer syncService.Stop()

		zapLogger.Info("skill store and sync service initialized")
	}

	skillHandler.RegisterRoutes(v1)

	pluginHandler := server.NewPluginHandler(pluginRegistry)
	pluginHandler.RegisterRoutes(v1)

	pluginStoreHandler := server.NewPluginStoreHandler(pluginStore)
	pluginStoreHandler.RegisterRoutes(v1)

	toolStoreHandler := server.NewToolStoreHandler(toolRegistry)
	toolStoreHandler.RegisterRoutes(v1)

	// Security routes
	if securityHandler != nil {
		securityGroup := protected.Group("/security")
		securityHandler.RegisterRoutes(securityGroup)
	}

	if sandboxHandler != nil {
		sandboxGroup := protected.Group("/sandbox")
		sandboxHandler.RegisterRoutes(sandboxGroup)
	}

	// API protected routes
	apiProtected := api.Group("")
	apiProtected.Use(authMiddleware.Authenticate())

	if cronHandler != nil {
		cronHandler.RegisterRoutes(apiProtected)
	}

	if haHandler != nil {
		haGroup := apiProtected.Group("/homeassistant")
		haHandler.RegisterRoutes(haGroup)
	}

	if browserHandler != nil {
		browserGroup := apiProtected.Group("/browser")
		browserHandler.RegisterRoutes(browserGroup)
	}

	if a2uiHandler != nil {
		a2uiGroup := apiProtected.Group("/a2ui")
		a2uiHandler.RegisterRoutes(a2uiGroup)
	}

	if workflowHandler != nil {
		workflowHandler.RegisterRoutes(e)
	}

	if voiceHandler != nil {
		voiceGroup := v1.Group("/voice")
		voiceHandler.RegisterRoutes(voiceGroup)
	}

	if formfillerHandler != nil {
		formfillerGroup := protected.Group("/formfiller")
		formfillerHandler.RegisterRoutes(formfillerGroup)
	}

	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir)
	claudeCodeGroup := protected.Group("/claudecode")
	claudeCodeHandler.RegisterRoutes(claudeCodeGroup)
	chatHandler.SetClaudeCodeHandler(claudeCodeHandler)

	providerSettingsHandler := server.NewProviderSettingsHandler(chatHandler.GetProviderRegistry(), dataDir)
	providerSettingsGroup := protected.Group("/providers/settings")
	providerSettingsHandler.RegisterRoutes(providerSettingsGroup)

	// Companion routes
	if companionHandler != nil {
		companionHandler.RegisterRoutes(e)
	}
	if companionWSHandler != nil {
		companionWSHandler.RegisterRoutes(e)
	}

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer = &http.Server{
		Addr:         addr,
		Handler:      e,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	zapLogger.Info("Starting HTTP server", zap.String("addr", addr))

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		zapLogger.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Stop services
		if metricsCollector != nil {
			metricsCollector.Stop()
		}
		if metricsWriter != nil {
			metricsWriter.Stop()
		}
		if cronService != nil {
			cronService.Stop(shutdownCtx)
		}

		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Required for c-archive build mode
func main() {}
