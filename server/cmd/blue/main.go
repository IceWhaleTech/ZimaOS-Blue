package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	concpool "github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/lifecycle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

var (
	version   = "0.10.28"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// On macOS, request speech recognition authorization on thread 0
	// BEFORE starting the server. runtime.LockOSThread() in macos_init.go
	// pins this goroutine to thread 0 (required by AppKit/TCC).
	macosRequestSTTAuthorization()

	// Fast-path: CLI subcommands bypass cobra to minimize page faults and RSS.
	// All init() functions have already run, but we avoid touching cobra's
	// command tree, flag parsing, and the heavy code paths they pull in.
	if len(os.Args) > 1 {
		if cliDispatch(os.Args[1:]) {
			return
		}
	}

	// On macOS, the main goroutine (thread 0) must pump the Cocoa run loop
	// for Speech framework callbacks. Run the server on a goroutine and
	// keep thread 0 for the run loop. On non-darwin, macosRunMainRunLoop()
	// is a no-op so we fall through to Execute() directly.
	if isDarwin() {
		go func() {
			Execute()
			// Server shut down — stop the run loop so main() can return
			macosStopMainRunLoop()
		}()
		macosRunMainRunLoop() // blocks thread 0 until StopMainRunLoop()
	} else {
		Execute()
	}
}

// runServer is the main server entry point, called by cobra rootCmd
func runServer() {
	// Tune GC: GOGC=50 balances memory vs CPU; 128MB soft limit avoids excessive GC thrashing
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(128 * 1024 * 1024) // 128MB soft limit

	// Load configuration (cfgFile is set by cobra's --config flag)
	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize HotReloader for config changes
	var hotReloader *config.HotReloader
	var err2 error
	hotReloader, err2 = config.NewHotReloader(cfgFile, cfg, &config.HotReloadConfig{
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
		Msg("Starting ZimaOS-Blue")

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

	dbPath := filepath.Join(dataDir, "blue.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	// Configure database connection pool (shared by user, memory, apikey tables)
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(3)
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
	if _, err := db.Exec("PRAGMA cache_size=-2000"); err != nil { // ~2MB page cache (reduced from 8MB for lower idle memory)
		logger.Warn().Err(err).Msg("Failed to set cache size")
	}
	db.Exec("PRAGMA shrink_memory") // Release unused memory after pragma changes

	// Initialize user repository and service
	userRepo, err := user.NewSQLiteRepository(db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize user repository")
	}

	// Create password hasher with secure defaults
	passwordHasher := password.NewHasher(&password.Config{
		Memory:      32 * 1024, // 32 MB (reduced from 64MB for lower memory spikes, still OWASP-compliant)
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

	// Initialize memory store for conversations (shares main blue.db)
	memoryStore, err := memory.NewStoreWithDB(db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize memory store")
	}

	// Initialize LLM provider registry
	llmRegistry := llm.NewProviderRegistry()

	// Register default LLM providers from environment variables
	// Only register providers with actual API keys to reduce idle memory

	// OpenAI provider
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		llmRegistry.Register(llm.NewOpenAIProvider(openaiKey, ""))
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
	if customKey != "" || customURL != "" {
		llmRegistry.Register(llm.NewCustomProvider(customKey, customURL))
	}

	// Grok provider (xAI)
	grokKey := os.Getenv("GROK_API_KEY")
	if grokKey != "" {
		llmRegistry.Register(llm.NewGrokProvider(grokKey, ""))
	}

	// Qwen provider (Alibaba Cloud)
	qwenKey := os.Getenv("QWEN_API_KEY")
	if qwenKey != "" {
		llmRegistry.Register(llm.NewQwenProvider(qwenKey, ""))
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
	pluginStoreConfig := plugin.DefaultStoreConfig()
	pluginStoreConfig.CacheDir = filepath.Join(getDataDir(), "plugin-cache")
	pluginStore := plugin.NewStore(pluginStoreConfig, pluginRegistry)
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

	// Initialize API Key service (shares main blue.db)
	apiKeyService, err := auth.NewAPIKeyServiceWithDB(db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize API key service")
	}

	// Initialize Claude Code CLI provider with internal API key for local proxy
	// This must be done after apiKeyService is ready
	// Create three internal API keys for different routing modes:
	// - cc-cli-auto: Auto mode (system chooses best provider)
	// - cc-cli-cloud: Cloud mode (force cloud provider)
	// - cc-cli-local: Local mode (force local CC CLI)
	var ccCliAutoKey, ccCliCloudKey, ccCliLocalKey string

	if providerpool.GetTrialLicense() != "" {
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
		cronHandler        *cron.Handler
		haService          *homeassistant.HAService
		haHandler          *homeassistant.Handler
		browserHandler     *browser.Handler
		sttService         stt.Service
		ttsService         tts.Service
		voiceHandler       *voice.Handler
		workflowHandler    *workflow.Handler
		formfillerStore    *formfiller.Store
		formfillerHandler  *formfiller.Handler
		ngrokTunnelMgr     *ngrok.SDKTunnelManager
		ngrokConfigStore   *ngrok.ConfigStore
	)

	// Use conc/pool for safer parallel initialization with automatic panic recovery
	initPool := concpool.New().WithMaxGoroutines(8)

	// Use WaitGroup to coordinate critical service initialization
	var criticalServicesWg sync.WaitGroup

	// Group 1: Critical services that must be ready before server starts
	// Initialize metrics synchronously to ensure it's ready for first request
	criticalServicesWg.Add(1)
	go func() {
		defer criticalServicesWg.Done()
		// Metrics collector (collect every 10 seconds, keep 5 minutes of history)
		metricsCollector = metrics.NewCollector(10*time.Second, 30)
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

	// Browser automation service — lazy init, only when first API call arrives
	// Chromium is very heavy on memory, skip at startup
	logger.Info().Msg("Browser automation will be initialized on first use")

	// TTS service — pick OS-appropriate default provider
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
			logger.Warn().Err(err).Msg("Failed to initialize TTS service, speech features will be limited")
		} else {
			logger.Info().Msgf("TTS service initialized (%s)", defaultTTSProvider)
		}
	}

	// Critical services in parallel pool
	initPool.Go(func() {
		// Security threat detector
		threatDetector = security.NewThreatDetector()
		securityHandler = security.NewHandler(threatDetector)
		logger.Info().Msg("Security handler initialized")
	})

	// Workflow service — lazy init on first API call (avoids cron goroutine + DB queries at startup)
	workflowHandler = workflow.NewLazyHandler(func() *workflow.WorkflowService {
		repo, err := workflow.NewRepository(db)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize workflow repository")
			return nil
		}
		svc, err := workflow.NewService(nil, repo)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize workflow service")
			return nil
		}
		logger.Info().Msg("Workflow service initialized lazily")
		return svc
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

	// Cron service — lazy init on first API call (avoids robfig/cron goroutine at startup)
	cronHandler = cron.NewLazyHandler(func() *cron.Service {
		svc := cron.NewService(cron.DefaultConfig(), zapLogger)
		svc.RegisterBuiltinHandlers()
		if err := svc.Start(); err != nil {
			logger.Warn().Err(err).Msg("Failed to start cron service")
		}
		return svc
	}, zapLogger)
	logger.Info().Msg("Cron service configured for lazy initialization")

	initPool.Go(func() {
		haService = homeassistant.NewHAService()
		haHandler = homeassistant.NewHandler(haService)
		logger.Info().Msg("Home Assistant handler initialized")
	})

	// TTS/STT services are initialized lazily when chat page is opened
	// This avoids heavy initialization at startup
	logger.Info().Msg("TTS/STT services will be initialized on demand (when chat page is opened)")

	initPool.Go(func() {
		// Form filler store
		var err error
		formfillerStore, err = formfiller.NewStore(filepath.Join(dataDir, "formfiller"))
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize form filler store, form filler features will be disabled")
			return
		}
		formfillerHandler = formfiller.NewHandler(formfillerStore)
		logger.Info().Msg("Form filler handler initialized")
	})

	var companionHandler   *companion.Handler
	var companionWSHandler *companion.WebSocketHandler

	if cfg.Companion.Enabled {
		// Companion service — eager init so chat handler can emit events immediately
		companionConfig := companion.DefaultConfig()
		companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
		companionStorage, err := companion.NewJSONLStorage(companionConfig.Storage.BasePath)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize companion storage")
		}
		if companionStorage != nil {
			companionStreamer := companion.NewEventStreamer(companionConfig)
			if err := companionStreamer.Start(lm.Context()); err != nil {
				logger.Warn().Err(err).Msg("Failed to start companion streamer")
			}
			companionManager := companion.NewManager(companionStorage, companionStreamer, companionConfig)
			companionHandler = companion.NewHandler(companionManager, companionStorage)
			companionWSHandler = companion.NewWebSocketHandler(companionStreamer, companionConfig)
			chatHandler.SetCompanionManager(companionManager)
			lm.RegisterShutdownHook(func(ctx context.Context) error {
				return companionStreamer.Stop()
			})
			logger.Info().Msg("Companion service initialized")
		}
	} else {
		logger.Info().Msg("Companion service disabled by config")
	}

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
	registerAPIRoutes(srv, pool, userHandler, extauthHandler, userService, chatHandler, autoreplyService, autoreplyHandler, metricsCollector, metricsWriter, authMiddleware, apiKeyHandler, apiKeyService, skillRegistry, pluginRegistry, pluginStore, backupHandler, toolRegistry, securityHandler, sandboxHandler, cronHandler, haHandler, browserHandler, workflowHandler, mfaHandler, voiceHandler, formfillerHandler, companionHandler, companionWSHandler, ngrokTunnelMgr, ngrokConfigStore, zapLogger, version, buildTime, gitCommit, dataDir, cfg, llmRegistry, db, jwtService, permissionHandler, sttService, ttsService, lm, hotReloader)

	// Register shutdown hook for server
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	// Store global server reference for dynamic TLS start
	server.SetGlobalServer(srv)

	// Register callback so cert generation/upload dynamically starts HTTPS
	security.OnCertReady(func() {
		srv.EnsureTLSStarted()
	})

	// Start server in background
	lm.Go(func(ctx context.Context) {
		if err := srv.Start(); err != nil {
			logger.Error().Err(err).Msg("Server error")
		}
	})

	// Start HTTPS server if TLS is enabled and certificate is available
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager != nil && tlsManager.GetCertificate() != nil {
		srv.EnsureTLSStarted()
	}

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
		logger.Error().Err(err).Msg("Shutdown error - some services may not have stopped cleanly")
		// Don't use os.Exit here - let deferred cleanup run
	}

	// Wait for worker pool
	if err := pool.Wait(); err != nil {
		logger.Warn().Err(err).Msg("Worker pool error during shutdown")
	}

	// Clean up cron service if it was initialized
	if cronHandler != nil {
		if svc := cronHandler.GetService(); svc != nil {
			svc.Stop(shutdownCtx)
			logger.Info().Msg("Cron service stopped")
		}
	}

	// Clean up STT service (important for CGO resources like Whisper)
	if sttService != nil {
		if wp := sttService.GetWhisperProvider(); wp != nil {
			wp.Close()
			logger.Info().Msg("STT Whisper provider cleaned up")
		}
	}

	// Clean up TTS service (important for CGO resources like eSpeak-NG)
	if ttsService != nil {
		ttsService.Close()
		logger.Info().Msg("TTS service cleaned up")
	}

	logger.Info().Msg("ZimaOS-Blue stopped")
}

func registerAPIRoutes(srv *server.Server, pool *worker.Pool, userHandler *user.Handler, extauthHandler *extauth.Handler, userService *user.Service, chatHandler *server.ChatHandler, autoreplyService *autoreply.Service, autoreplyHandler *autoreply.Handler, metricsCollector *metrics.Collector, metricsWriter *metrics.MetricsWriter, authMiddleware *auth.AuthMiddleware, apiKeyHandler *auth.APIKeyHandler, apiKeyService *auth.APIKeyService, skillRegistry *skill.Registry, pluginRegistry *plugin.Registry, pluginStore *plugin.Store, backupHandler *backup.Handler, toolRegistry *tools.Registry, securityHandler *security.Handler, sandboxHandler *sandbox.Handler, cronHandler *cron.Handler, haHandler *homeassistant.Handler, browserHandler *browser.Handler, workflowHandler *workflow.Handler, mfaHandler *mfa.Handler, voiceHandler *voice.Handler, formfillerHandler *formfiller.Handler, companionHandler *companion.Handler, companionWSHandler *companion.WebSocketHandler, ngrokTunnelMgr *ngrok.SDKTunnelManager, ngrokConfigStore *ngrok.ConfigStore, zapLogger *zap.Logger, version, buildTime, gitCommit, dataDir string, cfg *config.Config, llmRegistry *llm.ProviderRegistry, db *sql.DB, jwtService *auth.JWTService, permissionHandler *permission.Handler, sttService stt.Service, ttsService tts.Service, lm *lifecycle.Manager, hotReloader *config.HotReloader) {
	e := srv.Echo()
	logger := zapLogger

	// Initialize voice WebSocket handler
	var voiceWSHandler *voice.WSHandler
	if voiceHandler != nil {
		voiceWSHandler = voice.NewWSHandler(voiceHandler.Service())
	}

	// Initialize speech handler
	speechKV, _ := kvstore.NewSQLiteStoreWithDB(db)
	asrProvider := "whisper"
	if runtime.GOOS == "darwin" {
		asrProvider = "macos-native"
	} else if runtime.GOOS == "windows" {
		asrProvider = "windows-native"
	}
	speechService := speech.NewService(&speech.Config{
		TTS: speech.TTSConfig{Provider: "edge", Model: ""},
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
				logger.Warn("Failed to restore persisted TTS provider", zap.String("provider", saved), zap.Error(err))
			} else {
				if p := ttsService.GetProvider(tts.ProviderType(saved)); p != nil {
					speechService.SetTTSProvider(p)
				}
				logger.Info("Restored persisted TTS provider", zap.String("provider", saved))
			}
		}
	}
	// On macOS, prefer native STT (always available, no download needed)
	logger.Info("ASR provider setup", zap.String("goos", runtime.GOOS), zap.Bool("sttServiceNil", sttService == nil))
	if runtime.GOOS == "darwin" {
		logger.Info("macOS detected, initializing native STT...")
		macosSTT := speech.NewMacOSNativeSTT()
		if err := macosSTT.Initialize(); err == nil {
			speechService.SetASRProvider(macosSTT)
			// Also update voice service to use macOS native STT for /voice/transcribe
			if voiceHandler != nil {
				voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(macosSTT))
			}
			logger.Info("macOS native STT initialized OK",
				zap.String("providerType", string(macosSTT.Type())),
				zap.String("providerName", macosSTT.Name()))
		} else {
			logger.Warn("macOS native STT init failed, falling back to whisper", zap.Error(err))
			speechService.SetASRPermissionDenied(err.Error())
			if sttService != nil {
				if wp := sttService.GetWhisperProvider(); wp != nil && wp.IsInitialized() {
					speechService.SetASRProvider(wp)
				}
			}
		}
	} else if runtime.GOOS == "windows" {
		logger.Info("Windows detected, initializing native ASR...")
		windowsASR := speech.NewWindowsNativeASR()
		if windowsASR != nil {
			speechService.SetASRProvider(windowsASR)
			// Also update voice service to use Windows native ASR for /voice/transcribe
			if voiceHandler != nil {
				voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(windowsASR))
			}
			logger.Info("Windows native ASR initialized OK",
				zap.String("providerType", string(windowsASR.Type())),
				zap.String("providerName", windowsASR.Name()))
		} else {
			logger.Warn("Windows native ASR init failed, falling back to whisper")
			if sttService != nil {
				if wp := sttService.GetWhisperProvider(); wp != nil {
					speechService.SetASRProvider(wp)
				}
			}
		}
	} else if sttService != nil {
		if wp := sttService.GetWhisperProvider(); wp != nil {
			speechService.SetASRProvider(wp)
		}
	}
	// Log final ASR provider state
	if p := speechService.GetASRProvider(); p != nil {
		logger.Info("Final ASR provider", zap.String("type", string(p.Type())), zap.String("name", p.Name()))
	} else {
		logger.Warn("No ASR provider configured")
	}

	// Initialize Claude Code handler
	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir)
	systemPromptBuilder := claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{WorkspaceDir: dataDir})
	systemPromptBuilder.SetToolRegistry(toolRegistry)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)

	// Initialize memory handler with lazy init (vector_memory.db created on first request)
	var memoryHandler *server.MemoryHandler
	if cfg.Memory.VectorStore.Enabled {
		memoryHandler = server.NewLazyMemoryHandler(func(h *server.MemoryHandler) error {
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
				return err
			}
			hybridSearcher := memory.NewHybridSearcher(vectorStore, nil, cfg.Memory)
			memoryService := memory.NewMemoryService(hybridSearcher)
			unifiedService := memory.NewUnifiedMemoryService(memoryService, cfg.Memory)
			h.SetService(memoryService)
			h.SetUnifiedService(unifiedService)

			// Initialize markdown backend inside lazy init (must happen after SetUnifiedService)
			memoryDir := cfg.Memory.MarkdownDir
			if memoryDir == "" {
				memoryDir = filepath.Join(dataDir, "memory")
			}
			if mdBackend, mdErr := memory.NewPureMarkdownBackend(memoryDir); mdErr != nil {
				logger.Warn("Failed to initialize markdown backend", zap.Error(mdErr))
			} else {
				unifiedService.SetMarkdownBackend(mdBackend)
			}

			// Set backend mode from config
			backendMode := "markdown"
			if cfg.Memory.Backend != "" {
				backendMode = cfg.Memory.Backend
			}
			if setErr := unifiedService.SetBackend(backendMode); setErr != nil {
				logger.Warn("Failed to set memory backend mode", zap.String("mode", backendMode), zap.Error(setErr))
			} else {
				logger.Info("Memory backend configured", zap.String("mode", backendMode))
			}

			layeredService, err := memory.NewLayeredMemoryService(unifiedService, memory.LayeredMemoryConfig{
				BaseDir:            memoryDir,
				DailyRetentionDays: 30,
			})
			if err != nil {
				logger.Warn("Failed to initialize layered memory service", zap.Error(err))
			} else {
				h.SetLayeredService(layeredService)
				chatHandler.SetLayeredMemory(layeredService)
			}
			toolsAdapter := memory.NewToolsAdapter(unifiedService)
			tools.RegisterMemoryTools(toolRegistry, toolsAdapter)

			// Initialize progressive searcher
			if localSvc := unifiedService.GetLocalBackend(); localSvc != nil {
				ps := memory.NewProgressiveSearcher(localSvc.GetSearcher())
				h.SetProgressiveSearcher(ps)
				psTool := memory.NewMemoryProgressiveSearchTool(ps)
				tools.SetProgressiveSearchTool(psTool)
				logger.Info("Progressive search enabled")
			}

			logger.Info("Vector memory store initialized lazily")
			return nil
		})
	} else {
		logger.Info("Vector memory store disabled by config")
	}

	// Initialize shared cache
	var sharedCache *proxy.CCCache
	cacheConfig := proxy.DefaultCacheConfig()
	if cfg.Proxy != nil && cfg.Proxy.Cache != nil {
		cacheConfig = cfg.Proxy.Cache
	}
	// Set disk cache path relative to dataDir if using default
	if cacheConfig.StoragePath == "" || cacheConfig.StoragePath == "./data/cache.db" {
		cacheConfig.StoragePath = filepath.Join(dataDir, "cache.db")
	}
	// Ensure disk cache directory exists
	if cacheConfig.StoragePath != "" {
		if dir := filepath.Dir(cacheConfig.StoragePath); dir != "" {
			os.MkdirAll(dir, 0755)
		}
	}
	sharedCache = proxy.NewCCCache(cacheConfig)
	// Startup warmup: load hot entries from L2 disk into L1 memory
	if cacheConfig.Warming.Enabled {
		topN := cacheConfig.Warming.MaxRequests
		if topN <= 0 {
			topN = 1000
		}
		go sharedCache.Warmup(topN)
	}
	chatHandler.SetCache(sharedCache)

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(dataDir)

	// Initialize provider pool
	var providerPool *providerpool.Pool
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	providerPool, err := providerpool.NewPool(providerPoolPath)
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
			WorkerPool:    pool,
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

	apiProtected := bootstrap.RegisterAllRoutes(e, deps)

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

	// Heartbeat runner + handler (needs channelManager, so created after bootstrap)
	hbCfg := convertHeartbeatConfig(&cfg.Heartbeat)
	hbRunner := heartbeat.NewRunner(heartbeat.RunnerDeps{
		Config: hbCfg,
		ChatFn: func() heartbeat.ChatFunc {
			pc := server.NewProxyClient(cfg.Server.Port)
			if apiKeyService != nil {
				if info, err := apiKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
					Name:   "heartbeat-internal",
					Scopes: []string{"chat", "proxy", "route:auto"},
				}); err == nil {
					pc.SetAPIKey(info.Key)
				}
			}
			return pc.Chat
		}(),
		Channels: channelManager,
		Logger:   zapLogger,
	})
	hbHandler := heartbeat.NewHandler(hbRunner)
	hbHandler.RegisterRoutes(apiProtected)
	lm.Go(hbRunner.Run)
	logger.Info("Heartbeat runner initialized", zap.Bool("enabled", cfg.Heartbeat.Enabled))

	// OTA update checker (background, non-blocking)
	updateCfg := &update.Config{
		Enabled:        true,
		StoragePath:    filepath.Join(dataDir, "updates"),
		BackupCount:    2,
		ReleaseChannel: "stable",
	}
	updateHandler := update.NewHandler(version, updateCfg)
	otaChecker := update.NewOTAChecker(version, dataDir, "")
	updateHandler.SetOTAChecker(otaChecker)
	updateHandler.RegisterRoutes(apiProtected)
	lm.Go(otaChecker.Run)
	logger.Info("OTA update checker started")
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

// convertHeartbeatConfig converts config.HeartbeatConfig to heartbeat.Config.
func convertHeartbeatConfig(cfg *config.HeartbeatConfig) *heartbeat.Config {
	hbCfg := &heartbeat.Config{
		Enabled:         cfg.Enabled,
		Interval:        cfg.Interval,
		Prompt:          cfg.Prompt,
		AckMaxChars:     cfg.AckMaxChars,
		WorkspaceDir:    cfg.WorkspaceDir,
		LLMProvider:     cfg.LLMProvider,
		LLMModel:        cfg.LLMModel,
		DeliveryChannel: cfg.DeliveryChannel,
		DeliveryChatID:  cfg.DeliveryChatID,
		Visibility: heartbeat.VisibilityConfig{
			ShowOk:       cfg.Visibility.ShowOk,
			ShowAlerts:   cfg.Visibility.ShowAlerts,
			UseIndicator: cfg.Visibility.UseIndicator,
		},
	}
	if cfg.ActiveHours != nil {
		hbCfg.ActiveHours = &heartbeat.ActiveHours{
			Start:    cfg.ActiveHours.Start,
			End:      cfg.ActiveHours.End,
			Timezone: cfg.ActiveHours.Timezone,
		}
	}
	return hbCfg
}
