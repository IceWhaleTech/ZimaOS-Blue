package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	concpool "github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/bootstrap"
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
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/workflow"
)

var (
	version   = "0.10.22"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// getDataDir returns the appropriate data directory based on the platform.
// On macOS, it uses ~/Library/Application Support/com.zimaos.echo/
// On other platforms, it uses ./data
func getDataDir() string {
	// On macOS (darwin), use Application Support directory
	if runtime.GOOS == "darwin" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, "Library", "Application Support", "com.zimaos.echo")
		}
	}

	return "./data"
}

// runServer is the main server entry point, called by cobra commands
func runServer() {
	main()
}

func main() {
	// Handle Windows service commands (install, uninstall, start, stop, status)
	// if HandleServiceCommand(os.Args) {
	// 	return
	// }

	// Parse flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("v", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help information")
	flag.Parse()

	if *showHelp {
		fmt.Printf("ZimaOS-Echo %s - NAS-Native Agent Runtime\n\n", version)
		fmt.Println("Usage: echo [options] [command]")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --config <path>  Path to config file")
		fmt.Println("  -v               Show version information")
		fmt.Println("  --help           Show this help message")
		fmt.Println("")
		// PrintServiceHelp()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("ZimaOS-Echo %s\n", version)
		fmt.Printf("Build time: %s\n", buildTime)
		fmt.Printf("Git commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize HotReloader for config changes
	var hotReloader *config.HotReloader
	var err2 error
	hotReloader, err2 = config.NewHotReloader(*configPath, cfg, &config.HotReloadConfig{
		Enabled:             true,
		WatchInterval:       5 * time.Second,
		ValidateBeforeApply: true,
	})
	if err2 != nil {
		// Continue without hot reloader
		hotReloader = nil
	}

	// Initialize logger
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info().
		Str("version", version).
		Str("build_time", buildTime).
		Str("git_commit", gitCommit).
		Msg("Starting ZimaOS-Echo")

	// Initialize lifecycle manager
	lm := lifecycle.New()

	// Initialize worker pool
	pool := worker.NewPool(lm.Context(), cfg.Worker.PoolSize)
	logger.Info().Int("pool_size", cfg.Worker.PoolSize).Msg("Worker pool initialized")

	// Initialize database
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create data directory")
	}

	dbPath := filepath.Join(dataDir, "echo.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	// Configure database connection pool (optimized for startup performance)
	db.SetMaxOpenConns(15)      // 优化: 增加到 15 以支持并发初始化
	db.SetMaxIdleConns(8)       // 优化: 增加到 8 以减少连接创建开销
	db.SetConnMaxLifetime(time.Hour)

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logger.Warn().Err(err).Msg("Failed to enable WAL mode")
	}
	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		logger.Warn().Err(err).Msg("Failed to enable foreign keys")
	}
	// 优化: 添加更多 SQLite 性能优化
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logger.Warn().Err(err).Msg("Failed to set synchronous mode")
	}
	if _, err := db.Exec("PRAGMA cache_size=-64000"); err != nil {
		logger.Warn().Err(err).Msg("Failed to set cache size")
	}

	// Warm up connection pool for faster startup (parallel)
	for i := 0; i < 5; i++ {
		go func() {
			conn, err := db.Conn(context.Background())
			if err == nil {
				conn.Close()
			}
		}()
	}

	// Initialize user repository and service
	userRepo, err := user.NewSQLiteRepository(db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize user repository")
	}

	// Create password hasher with secure defaults
	passwordHasher := password.NewHasher(&password.Config{
		Memory:      64 * 1024, // 64 MB
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	})

	// Create password policy based on config
	passwordPolicy := password.NewPolicy(&password.PolicyConfig{
		MinLength:        cfg.Security.Password.MinLength,
		RequireUppercase: cfg.Security.Password.RequireUppercase,
		RequireLowercase: cfg.Security.Password.RequireLowercase,
		RequireNumber:    cfg.Security.Password.RequireNumber,
		RequireSpecial:   cfg.Security.Password.RequireSpecial,
	})

	userService := user.NewService(userRepo, passwordHasher, passwordPolicy, nil)
	userHandler := user.NewHandler(userService)

	// Initialize permission repository and service
	permissionRepo, err := permission.NewRepository(db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize permission repository")
	}
	permissionService := permission.NewService(permissionRepo, userRepo)
	permissionHandler := permission.NewHandler(permissionService, userRepo)

	// Set permission service on user handler
	userHandler.SetPermissionService(permissionService)

	// Initialize memory store for conversations
	memoryDbPath := filepath.Join(dataDir, "memory.db")
	memoryStore, err := memory.NewStore(memoryDbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize memory store")
	}

	// Initialize LLM provider registry
	llmRegistry := llm.NewProviderRegistry()

	// Register default LLM providers from environment variables

	// OpenAI provider
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		llmRegistry.Register(llm.NewOpenAIProvider(openaiKey, ""))
	} else {
		// Register with empty key - will fail on actual API calls but allows listing
		llmRegistry.Register(llm.NewOpenAIProvider("", ""))
	}

	// Claude provider
	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	claudeBaseURL := ""
	// Also check config.yaml for Claude Code CLI settings
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

	// Ollama provider (local, no API key needed)
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	llmRegistry.Register(llm.NewOllamaProvider(ollamaURL))

	// Custom OpenAI-compatible provider (for third-party services like DeepSeek, Together, etc.)
	customKey := os.Getenv("CUSTOM_API_KEY")
	customURL := os.Getenv("CUSTOM_API_URL")
	llmRegistry.Register(llm.NewCustomProvider(customKey, customURL))

	// Grok provider (xAI)
	grokKey := os.Getenv("GROK_API_KEY")
	if grokKey != "" {
		llmRegistry.Register(llm.NewGrokProvider(grokKey, ""))
	} else {
		llmRegistry.Register(llm.NewGrokProvider("", ""))
	}

	// Qwen provider (Alibaba Cloud)
	qwenKey := os.Getenv("QWEN_API_KEY")
	if qwenKey != "" {
		llmRegistry.Register(llm.NewQwenProvider(qwenKey, ""))
	} else {
		llmRegistry.Register(llm.NewQwenProvider("", ""))
	}

	// Initialize tools registry and register built-in tools
	toolRegistry := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)
	logger.Info().Int("count", len(toolRegistry.List())).Msg("Built-in tools registered")

	// Initialize skill registry and register built-in skills
	skillRegistry := skill.NewRegistry()
	if err := builtin.RegisterAll(skillRegistry); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register built-in skills")
	}
	logger.Info().Int("count", skillRegistry.Count()).Msg("Built-in skills registered")

	// Initialize plugin registry and store
	pluginRegistry := plugin.NewRegistry()
	pluginStore := plugin.NewStore(plugin.DefaultStoreConfig(), pluginRegistry)
	logger.Info().Msg("Plugin store initialized")

	// Initialize chat handler
	chatHandler := server.NewChatHandler(memoryStore, llmRegistry, toolRegistry)

	// Initialize external auth service (for OAuth/OIDC providers)
	extauthService, err := extauth.NewService(&extauth.ServiceConfig{
		Providers:    []*extauth.ProviderConfig{}, // No providers configured by default
		StateStore:   extauth.NewMemoryStateStore(),
		AccountStore: extauth.NewMemoryAccountStore(),
		UserStore:    nil, // TODO: Implement user store adapter
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize external auth service")
	}
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
		logger.Fatal().Err(err).Msg("Failed to initialize API key service")
	}
	defer apiKeyService.Close()

	// Initialize Claude Code CLI provider with internal API key for local proxy
	// This must be done after apiKeyService is ready
	// Create three internal API keys for different routing modes:
	// - cc-cli-auto: Auto mode (system chooses best provider)
	// - cc-cli-cloud: Cloud mode (force cloud provider)
	// - cc-cli-local: Local mode (force local CC CLI)
	var ccCliAutoKey, ccCliCloudKey, ccCliLocalKey string

	if os.Getenv("ZIMAOS_TRIAL_API_KEY") != "" {
		// Helper function to create or recreate an API key
		createOrRecreateKey := func(name string, scopes []string) string {
			keyInfo, err := apiKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
				UserID: "system",
				Name:   name,
				Scopes: scopes,
			})
			if err != nil {
				// Key might already exist, revoke and recreate
				keys, _ := apiKeyService.ListKeys(context.Background(), "system")
				for _, k := range keys {
					if k.Name == name {
						apiKeyService.RevokeKey(context.Background(), k.ID, "system")
						keyInfo, _ = apiKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
							UserID: "system",
							Name:   name,
							Scopes: scopes,
						})
						break
					}
				}
			}
			if keyInfo != nil {
				return keyInfo.Key
			}
			return ""
		}

		// Create three keys for different routing modes
		ccCliAutoKey = createOrRecreateKey("cc-cli-auto", []string{"chat", "proxy", "route:auto"})
		ccCliCloudKey = createOrRecreateKey("cc-cli-cloud", []string{"chat", "proxy", "route:cloud"})
		ccCliLocalKey = createOrRecreateKey("cc-cli-local", []string{"chat", "proxy", "route:local"})

		logger.Info().
			Str("auto_key", ccCliAutoKey[:8]+"...").
			Str("cloud_key", ccCliCloudKey[:8]+"...").
			Str("local_key", ccCliLocalKey[:8]+"...").
			Msg("Created three internal API keys for CC CLI routing modes")
	}

	// Note: Claude Code CLI is no longer registered as an LLM provider.
	// All chat requests are routed through the proxy, which handles provider selection internally.

	// Initialize auth middleware
	authMiddleware := auth.NewAuthMiddleware(jwtService, apiKeyService)

	// Initialize API Key handler
	apiKeyHandler := auth.NewAPIKeyHandler(apiKeyService)

	// Set JWT service on user handler for token generation
	userHandler.SetJWTService(jwtService)

	// Initialize auto-reply service and handler
	zapLogger, _ := zap.NewProduction()
	autoreplyService := autoreply.NewService(autoreply.DefaultConfig(), zapLogger)
	autoreplyHandler := autoreply.NewHandler(autoreplyService, zapLogger)

	// Initialize independent services in parallel for faster startup
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
		ngrokTunnelMgr     *ngrok.SDKTunnelManager
		ngrokConfigStore   *ngrok.ConfigStore
	)

	// Use conc/pool for safer parallel initialization with automatic panic recovery
	initPool := concpool.New().WithMaxGoroutines(20)

	// Use WaitGroup to coordinate critical service initialization
	var criticalServicesWg sync.WaitGroup

	// Group 1: Critical services that must be ready before server starts
	// Initialize metrics synchronously to ensure it's ready for first request
	criticalServicesWg.Add(1)
	go func() {
		defer criticalServicesWg.Done()
		// Metrics collector (collect every 5 seconds, keep 10 minutes of history)
		metricsCollector = metrics.NewCollector(5*time.Second, 120)
		metricsCollector.Start()
		// Metrics writer for detailed API metrics with SQLite persistence
		metricsConfig := metrics.DefaultWriterConfig()
		metricsConfig.SQLiteDBPath = filepath.Join(dataDir, "metrics.db")
		metricsWriter = metrics.NewMetricsWriter(nil, metricsConfig)
		metricsWriter.Start()
		logger.Info().Msg("Metrics services initialized")

		// Set metrics recorder on chat handler after initialization
		chatHandler.SetMetricsRecorder(metricsWriter)
		logger.Info().Msg("Metrics recorder set on chat handler")
	}()

	// Group 2: Non-critical services (async initialization for faster startup)

	go func() {
		// Backup manager
		var err error
		backupManager, err = backup.NewManager(backup.Config{
			Enabled:       true,
			RetentionDays: 7,
			Path:          filepath.Join(dataDir, "backups"),
		}, dataDir, dataDir)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize backup manager")
			return
		}
		backupHandler = backup.NewHandler(backupManager)
		logger.Info().Msg("Backup manager initialized")
	}()

	go func() {
		// Browser automation service
		var err error
		browserService, err = browser.NewService(nil)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize browser service, browser automation will be disabled")
			return
		}
		browserHandler = browser.NewHandler(browserService)
		logger.Info().Msg("Browser automation handler initialized")
	}()

	// Initialize TTS service asynchronously (non-blocking)
	go func() {
		var err error
		ttsService, err = tts.NewService(&tts.ServiceConfig{
			DefaultProvider: tts.ProviderEdge,
			Providers: []tts.ProviderConfig{
				{
					Type:    tts.ProviderEdge,
					Enabled: true,
				},
				{
					Type:    tts.ProviderEspeakNG,
					Enabled: true,
				},
			},
		})
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize TTS service")
		} else {
			logger.Info().Msg("TTS service initialized with Edge TTS (default) and eSpeak-NG (fallback)")
		}
	}()

	// Critical services in parallel pool
	initPool.Go(func() {
		// Security threat detector
		threatDetector = security.NewThreatDetector()
		securityHandler = security.NewHandler(threatDetector)
		logger.Info().Msg("Security handler initialized")
	})

	initPool.Go(func() {
		// Workflow repository (depends on db)
		var err error
		workflowRepo, err = workflow.NewRepository(db)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize workflow repository, workflow features will be disabled")
			return
		}
		workflowService, err := workflow.NewService(nil, workflowRepo)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize workflow service, workflow features will be disabled")
			return
		}
		workflowHandler = workflow.NewHandler(workflowService)
		logger.Info().Msg("Workflow handler initialized")
	})

	// Async initialization for MFA handler
	go func() {
		mfaHandler = mfa.NewHandler(nil, nil)
		logger.Info().Msg("MFA handler initialized")
	}()

	// Sync initialization for Sandbox manager (must complete before route registration)
	{
		var err error
		sandboxManager, err = sandbox.NewManager(nil)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize sandbox manager, sandbox features will be disabled")
		} else {
			sandboxHandler = sandbox.NewHandler(sandboxManager)
			logger.Info().Bool("supported", sandboxManager.IsSupported()).Msg("Sandbox handler initialized")
		}
	}

	// Async initialization for Cron service
	go func() {
		cronService = cron.NewService(cron.DefaultConfig(), zapLogger)
		cronService.RegisterBuiltinHandlers()
		cronHandler = cron.NewHandler(cronService, zapLogger)
		if err := cronService.Start(); err != nil {
			logger.Warn().Err(err).Msg("Failed to start cron service")
		}
		logger.Info().Msg("Cron service initialized")
	}()

	// Async initialization for Home Assistant service
	go func() {
		haService = homeassistant.NewHAService()
		haHandler = homeassistant.NewHandler(haService)
		logger.Info().Msg("Home Assistant handler initialized")
	}()

	// TTS/STT services are initialized lazily when chat page is opened
	// This avoids heavy initialization at startup
	logger.Info().Msg("TTS/STT services will be initialized on demand (when chat page is opened)")

	// Async initialization for non-critical services
	go func() {
		// Form filler store
		var err error
		formfillerStore, err = formfiller.NewStore(filepath.Join(dataDir, "formfiller"))
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize form filler store, form filler features will be disabled")
			return
		}
		formfillerHandler = formfiller.NewHandler(formfillerStore)
		logger.Info().Msg("Form filler handler initialized")
	}()

	initPool.Go(func() {
		// Companion service (Echo Companion - real-time AI Agent monitoring)
		companionConfig := companion.DefaultConfig()
		companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
		var err error
		companionStorage, err = companion.NewJSONLStorage(companionConfig.Storage.BasePath)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize companion storage, companion features will be disabled")
			return
		}
		companionStreamer := companion.NewEventStreamer(companionConfig)
		if err := companionStreamer.Start(lm.Context()); err != nil {
			logger.Warn().Err(err).Msg("Failed to start companion streamer")
		}
		companionManager = companion.NewManager(companionStorage, companionStreamer, companionConfig)
		companionHandler = companion.NewHandler(companionManager, companionStorage)
		companionWSHandler = companion.NewWebSocketHandler(companionStreamer, companionConfig)
		logger.Info().Msg("Companion handler initialized")

		// Register shutdown hook for companion streamer
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			return companionStreamer.Stop()
		})
	})

	// Wait for all parallel initializations to complete
	initPool.Wait()

	// Wait for critical services to be ready before continuing
	criticalServicesWg.Wait()
	logger.Info().Msg("Critical services initialized and ready")

	// Whisper ASR provider will be initialized lazily on first use
	// This significantly speeds up startup time
	whisperModelPath := filepath.Join(dataDir, "whisper-models")

	// Create a lazy-loading STT service that initializes Whisper on first use
	sttService = stt.NewLazyService(whisperModelPath)
	logger.Info().Msg("STT service configured for lazy initialization")

	// Register deferred cleanup for metrics services
	if metricsCollector != nil {
		defer metricsCollector.Stop()
	}
	if metricsWriter != nil {
		defer metricsWriter.Stop()
	}

	// Initialize voice service (depends on STT and TTS)
	// Create voice handler even if services are not initialized yet
	// The services will be initialized on demand when chat page is opened
	var voiceService voice.Service
	if sttService != nil && ttsService != nil {
		voiceService = voice.NewService(&voice.ServiceConfig{
			STTService: sttService,
			TTSService: ttsService,
		})
	} else {
		// Create a placeholder service that will work with lazy initialization
		voiceService = voice.NewService(&voice.ServiceConfig{
			STTService: sttService,
			TTSService: ttsService,
		})
	}
	voiceHandler = voice.NewHandler(voiceService)
	logger.Info().Msg("Voice handler initialized")

	// Initialize ngrok config store (JSON-based, lazy initialization)
	ngrokConfigStore = ngrok.NewConfigStore(dataDir)
	ngrokTunnelMgr = ngrok.NewSDKTunnelManager(nil)
	logger.Info().Msg("Ngrok tunnel services initialized (lightweight)")

	// Note: Metrics recorder is set on chat handler asynchronously after metrics initialization completes

	// Initialize HTTP server
	srv := server.New(&cfg.Server)
	server.SetVersion(version)
	srv.RegisterHealthRoutes()

	// Register API routes
	registerAPIRoutes(srv, pool, userHandler, extauthHandler, userService, chatHandler, autoreplyService, autoreplyHandler, metricsCollector, metricsWriter, authMiddleware, apiKeyHandler, apiKeyService, skillRegistry, pluginRegistry, pluginStore, backupHandler, toolRegistry, securityHandler, sandboxHandler, cronHandler, haHandler, browserHandler, workflowHandler, mfaHandler, voiceHandler, formfillerHandler, companionHandler, companionWSHandler, companionManager, ngrokTunnelMgr, ngrokConfigStore, zapLogger, version, buildTime, gitCommit, dataDir, cfg, llmRegistry, db, jwtService, permissionHandler, sttService, ttsService, lm, hotReloader)

	// Register shutdown hook for server
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	// Start server in background
	lm.Go(func(ctx context.Context) {
		if err := srv.Start(); err != nil {
			logger.Error().Err(err).Msg("Server error")
		}
	})

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case <-lm.Done():
		logger.Info().Msg("Lifecycle manager done")
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.SetReady(false)

	if err := lm.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Shutdown error")
		os.Exit(1)
	}

	// Wait for worker pool
	if err := pool.Wait(); err != nil {
		logger.Warn().Err(err).Msg("Worker pool error during shutdown")
	}

	// Clean up TTS service (important for CGO resources like eSpeak-NG)
	if ttsService != nil {
		ttsService.Close()
		logger.Info().Msg("TTS service cleaned up")
	}

	logger.Info().Msg("ZimaOS-Echo stopped")
}

func registerAPIRoutes(srv *server.Server, pool *worker.Pool, userHandler *user.Handler, extauthHandler *extauth.Handler, userService *user.Service, chatHandler *server.ChatHandler, autoreplyService *autoreply.Service, autoreplyHandler *autoreply.Handler, metricsCollector *metrics.Collector, metricsWriter *metrics.MetricsWriter, authMiddleware *auth.AuthMiddleware, apiKeyHandler *auth.APIKeyHandler, apiKeyService *auth.APIKeyService, skillRegistry *skill.Registry, pluginRegistry *plugin.Registry, pluginStore *plugin.Store, backupHandler *backup.Handler, toolRegistry *tools.Registry, securityHandler *security.Handler, sandboxHandler *sandbox.Handler, cronHandler *cron.Handler, haHandler *homeassistant.Handler, browserHandler *browser.Handler, workflowHandler *workflow.Handler, mfaHandler *mfa.Handler, voiceHandler *voice.Handler, formfillerHandler *formfiller.Handler, companionHandler *companion.Handler, companionWSHandler *companion.WebSocketHandler, companionManager *companion.Manager, ngrokTunnelMgr *ngrok.SDKTunnelManager, ngrokConfigStore *ngrok.ConfigStore, zapLogger *zap.Logger, version, buildTime, gitCommit, dataDir string, cfg *config.Config, llmRegistry *llm.ProviderRegistry, db *sql.DB, jwtService *auth.JWTService, permissionHandler *permission.Handler, sttService stt.Service, ttsService tts.Service, lm *lifecycle.Manager, hotReloader *config.HotReloader) {
	e := srv.Echo()
	logger := zapLogger

	// Initialize voice WebSocket handler
	var voiceWSHandler *voice.WSHandler
	if voiceHandler != nil {
		voiceWSHandler = voice.NewWSHandler(voiceHandler.Service())
	}

	// Initialize speech handler
	speechService := speech.NewService(&speech.Config{
		TTS: speech.TTSConfig{Provider: "edge", Model: ""},
		ASR: speech.ASRConfig{Enabled: true, Provider: "whisper", EditBeforeSend: true},
	}, nil, ttsService)
	if ttsService != nil {
		if provider := ttsService.GetProvider(tts.ProviderEdge); provider != nil {
			speechService.SetTTSProvider(provider)
		}
	}
	speechHandler := speech.NewHandler(speechService)
	if sttService != nil {
		if wp := sttService.GetWhisperProvider(); wp != nil {
			speechService.SetASRProvider(wp)
		}
	}

	// Initialize Claude Code handler
	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir)
	systemPromptBuilder := claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{WorkspaceDir: dataDir})
	systemPromptBuilder.SetToolRegistry(toolRegistry)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)

	// Initialize memory handler
	var memoryHandler *server.MemoryHandler
	vectorDbPath := filepath.Join(dataDir, "vector_memory.db")
	vectorStore, err := memory.NewVectorStore(memory.VectorStoreConfig{
		DBPath:       vectorDbPath,
		EmbeddingDim: cfg.Memory.VectorStore.Dimensions,
		MaxChunks:    10000,
		EnableFTS:    true,
		EnableVec:    true,
	})
	if err != nil {
		logger.Warn("Failed to initialize vector store", zap.Error(err))
	} else {
		hybridSearcher := memory.NewHybridSearcher(vectorStore, nil, cfg.Memory)
		memoryService := memory.NewMemoryService(hybridSearcher)
		unifiedService := memory.NewUnifiedMemoryService(memoryService, cfg.Memory)
		memoryHandler = server.NewMemoryHandler(memoryService)
		memoryHandler.SetUnifiedService(unifiedService)

		memoryDir := filepath.Join(dataDir, "memory")
		layeredService, err := memory.NewLayeredMemoryService(unifiedService, memory.LayeredMemoryConfig{
			BaseDir:            memoryDir,
			DailyRetentionDays: 30,
		})
		if err != nil {
			logger.Warn("Failed to initialize layered memory service", zap.Error(err))
		} else {
			memoryHandler.SetLayeredService(layeredService)
		}
		toolsAdapter := memory.NewToolsAdapter(unifiedService)
		tools.RegisterMemoryTools(toolRegistry, toolsAdapter)
	}

	// Initialize shared cache
	var sharedCache *proxy.CCCache
	cacheConfig := proxy.DefaultCacheConfig()
	if cfg.Proxy != nil && cfg.Proxy.Cache != nil {
		cacheConfig = cfg.Proxy.Cache
	}
	sharedCache = proxy.NewCCCache(cacheConfig)
	chatHandler.SetCache(sharedCache)

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(dataDir)

	// Initialize provider pool
	var providerPool *providerpool.Pool
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	providerPool, err = providerpool.NewPool(providerPoolPath)
	if err != nil {
		logger.Warn("Failed to initialize provider pool", zap.Error(err))
		providerPool = nil
	}

	if providerPool != nil {
		bootstrap.LoadProvidersFromPool(providerPool, llmRegistry)
		chatHandler.SetProviderPool(providerPool)

		if providerpool.CheckMigrationNeeded(dataDir) {
			result, err := providerpool.MigrateFromLegacy(dataDir, providerPool)
			if err != nil {
				logger.Warn("Failed to migrate legacy provider settings", zap.Error(err))
			} else if result.Migrated > 0 {
				logger.Info("Legacy provider settings migrated", zap.Int("migrated", result.Migrated))
			}
		}

		server.OnServerStart(func(port int) {
			go func() {
				providerPool.Start(context.Background())
			}()
		})
	}

	// Call bootstrap.RegisterAllRoutes with all dependencies
	deps := &bootstrap.RoutesDeps{
		DB:     db,
		Config: cfg,
		ServerConfig: &bootstrap.ServerConfig{
			Version:   version,
			BuildTime: buildTime,
			GitCommit: gitCommit,
			DataDir:   dataDir,
			Port:      cfg.Server.Port,
		},
		Services: &bootstrap.Services{
			DB:            db,
			UserService:   userService,
			JWTService:    jwtService,
			SkillRegistry: skillRegistry,
			ToolRegistry:  toolRegistry,
		},
		Logger:             zapLogger,
		Ctx:                lm.Context(),
		MetricsWriter:      metricsWriter,
		MetricsCollector:   metricsCollector,
		ChatHandler:        chatHandler,
		PluginRegistry:     pluginRegistry,
		PluginStore:        pluginStore,
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
		WorkflowHandler:    workflowHandler,
		VoiceHandler:       voiceHandler,
		VoiceWSHandler:     voiceWSHandler,
		FormfillerHandler:  formfillerHandler,
		CompanionHandler:   companionHandler,
		CompanionWSHandler: companionWSHandler,
		ProviderPool:       providerPool,
		APIKeyService:      apiKeyService,
		SpeechHandler:      speechHandler,
		NgrokTunnelMgr:     ngrokTunnelMgr,
		NgrokConfigStore:   ngrokConfigStore,
		ClaudeCodeHandler:  claudeCodeHandler,
		MemoryHandler:      memoryHandler,
		ChannelConfigStore: channelConfigStore,
		SharedCache:        sharedCache,
		HotReloader:        hotReloader,
	}

	bootstrap.RegisterAllRoutes(e, deps)

	// Register companion and channel routes (after bootstrap)
	if companionHandler != nil {
		companionHandler.RegisterRoutes(e)
	}
	if companionWSHandler != nil {
		companionWSHandler.RegisterRoutes(e)
	}

	channelConfigHandler := server.NewChannelConfigHandler(channelConfigStore)
	channelManager := channel.NewManager(channel.DefaultConfig(), zapLogger)
	channelFactory := server.NewChannelFactory(zapLogger)

	channelManager.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		if autoreplyService != nil {
			response, rule, err := autoreplyService.Match(ctx, msg.Content, msg.ChannelName, msg.UserID, "", msg.ChatID)
			if err == nil && response != "" && rule != nil {
				return &channel.OutgoingMessage{ChatID: msg.ChatID, Content: response}, nil
			}
		}
		if chatHandler != nil {
			aiResponse, err := chatHandler.ProcessChannelMessage(ctx, msg)
			if err == nil && aiResponse != "" {
				return &channel.OutgoingMessage{ChatID: msg.ChatID, Content: aiResponse}, nil
			}
		}
		return nil, nil
	})

	enabledChannels := channelConfigStore.GetEnabled()
	if len(enabledChannels) > 0 {
		var wg sync.WaitGroup
		for _, cfg := range enabledChannels {
			wg.Add(1)
			go func(cfg *server.ChannelConfig) {
				defer wg.Done()
				ch, err := channelFactory.CreateChannel(cfg)
				if err != nil || ch == nil {
					return
				}
				if err := channelManager.Register(ch); err != nil {
					return
				}
				if err := channelManager.StartChannel(context.Background(), cfg.ID); err != nil {
					cfg.Status = "error"
					cfg.LastError = err.Error()
					_ = channelConfigStore.Set(cfg.ID, cfg)
				} else {
					cfg.Status = "connected"
					cfg.LastError = ""
					_ = channelConfigStore.Set(cfg.ID, cfg)
				}
			}(cfg)
		}
		wg.Wait()
	}

	channelConfigHandler.SetManager(channelManager)
	channelConfigHandler.SetFactory(channelFactory)
	channelConfigHandler.RegisterRoutes(e.Group("/api"))
}

// convertClaudeCodeConfig converts config.ClaudeCodeConfig to claudecode.ClaudeCodeConfig.
func convertClaudeCodeConfig(cfg *config.ClaudeCodeConfig, apiKey, baseURL string) *claudecode.ClaudeCodeConfig {
	return &claudecode.ClaudeCodeConfig{
		Enabled:      cfg.Enabled,
		Command:      cfg.Command,
		WorkspaceDir: cfg.WorkspaceDir,
		DefaultModel: cfg.DefaultModel,
		Timeout:      cfg.Timeout,
		SessionTTL:   cfg.SessionTTL,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Backend: claudecode.CliBackendConfig{
			Command:           cfg.Command,
			Args:              cfg.Backend.Args,
			ResumeArgs:        cfg.Backend.ResumeArgs,
			Output:            cfg.Backend.Output,
			Input:             cfg.Backend.Input,
			MaxPromptArgChars: cfg.Backend.MaxPromptArgChars,
			Env:               cfg.Backend.Env,
			ClearEnv:          cfg.Backend.ClearEnv,
			ModelArg:          cfg.Backend.ModelArg,
			ModelAliases:      cfg.Backend.ModelAliases,
			SessionArg:        cfg.Backend.SessionArg,
			SessionMode:       cfg.Backend.SessionMode,
			SystemPromptArg:   cfg.Backend.SystemPromptArg,
			SystemPromptMode:  cfg.Backend.SystemPromptMode,
			SystemPromptWhen:  cfg.Backend.SystemPromptWhen,
			Serialize:         cfg.Backend.Serialize,
		},
	}
}
