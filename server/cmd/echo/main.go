package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

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
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/proxy"
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

// loadProvidersFromPool loads providers from Provider Pool and registers them in LLM registry
// This replaces the environment variable-based provider registration with encrypted storage
func loadProvidersFromPool(pool *providerpool.Pool, llmRegistry *llm.ProviderRegistry) {
	if pool == nil || llmRegistry == nil {
		return
	}

	// Get all enabled providers from the pool
	providers := pool.Registry.ListEnabled()

	registeredCount := 0
	for _, provider := range providers {
		// Get the first enabled API key
		var apiKey string
		var baseURL string

		if len(provider.APIKeys) > 0 {
			for _, key := range provider.APIKeys {
				if key.Enabled && key.Key != "" {
					apiKey = key.Key
					break
				}
			}
		}

		baseURL = provider.BaseURL

		// Map Provider Pool provider to LLM provider
		var llmProvider llm.Provider

		switch provider.ID {
		case "openai":
			llmProvider = llm.NewOpenAIProvider(apiKey, baseURL)
		case "anthropic":
			llmProvider = llm.NewClaudeProvider(apiKey, baseURL)
		case "ollama":
			// Ollama doesn't need API key
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			llmProvider = llm.NewOllamaProvider(baseURL)
		case "custom":
			llmProvider = llm.NewCustomProvider(apiKey, baseURL)
		case "grok":
			llmProvider = llm.NewGrokProvider(apiKey, baseURL)
		case "qwen":
			llmProvider = llm.NewQwenProvider(apiKey, baseURL)
		default:
			// For unknown providers, try to use custom provider
			logger.Debug().
				Str("provider_id", provider.ID).
				Str("provider_name", provider.Name).
				Msg("Unknown provider type, skipping")
			continue
		}

		if llmProvider != nil {
			llmRegistry.Register(llmProvider)
			registeredCount++
			logger.Info().
				Str("provider_id", provider.ID).
				Str("provider_name", provider.Name).
				Bool("has_api_key", apiKey != "").
				Str("base_url", baseURL).
				Msg("Registered provider from Provider Pool")
		}
	}

	logger.Info().
		Int("total_enabled", len(providers)).
		Int("registered", registeredCount).
		Msg("Loaded providers from Provider Pool")
}

func main() {
	// Handle Windows service commands (install, uninstall, start, stop, status)
	if HandleServiceCommand(os.Args) {
		return
	}

	// Parse flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help information")
	flag.Parse()

	if *showHelp {
		fmt.Printf("ZimaOS-Echo %s - NAS-Native Agent Runtime\n\n", version)
		fmt.Println("Usage: echo [options] [command]")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --config <path>  Path to config file")
		fmt.Println("  --version        Show version information")
		fmt.Println("  --help           Show this help message")
		fmt.Println("")
		PrintServiceHelp()
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
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create data directory")
	}

	dbPath := filepath.Join(dataDir, "echo.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	// Configure database connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logger.Warn().Err(err).Msg("Failed to enable WAL mode")
	}
	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		logger.Warn().Err(err).Msg("Failed to enable foreign keys")
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

	// Initialize memory store for conversations
	memoryDbPath := filepath.Join(dataDir, "memory.db")
	memoryStore, err := memory.NewStore(memoryDbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize memory store")
	}

	// Initialize LLM provider registry
	llmRegistry := llm.NewProviderRegistry()

	// Load saved LLM configuration from setup wizard
	savedLLMConfig, err := setup.LoadLLMConfig(dataDir)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to load saved LLM config, falling back to environment variables")
	}

	// Register default LLM providers
	// Priority: saved config > environment variables > empty (for listing)

	// OpenAI provider
	openaiKey := os.Getenv("OPENAI_API_KEY")
	openaiBaseURL := ""
	if savedLLMConfig != nil && savedLLMConfig.Provider == "openai" && savedLLMConfig.APIKey != "" {
		openaiKey = savedLLMConfig.APIKey
		openaiBaseURL = savedLLMConfig.BaseURL
	}
	if openaiKey != "" {
		llmRegistry.Register(llm.NewOpenAIProvider(openaiKey, openaiBaseURL))
	} else {
		// Register with empty key - will fail on actual API calls but allows listing
		llmRegistry.Register(llm.NewOpenAIProvider("", ""))
	}

	// Claude provider
	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	claudeBaseURL := ""
	if savedLLMConfig != nil && savedLLMConfig.Provider == "anthropic" && savedLLMConfig.APIKey != "" {
		claudeKey = savedLLMConfig.APIKey
		claudeBaseURL = savedLLMConfig.BaseURL
	}
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
	if savedLLMConfig != nil && savedLLMConfig.Provider == "ollama" && savedLLMConfig.BaseURL != "" {
		ollamaURL = savedLLMConfig.BaseURL
	}
	llmRegistry.Register(llm.NewOllamaProvider(ollamaURL))

	// Custom OpenAI-compatible provider (for third-party services like DeepSeek, Together, etc.)
	customKey := os.Getenv("CUSTOM_API_KEY")
	customURL := os.Getenv("CUSTOM_API_URL")
	if savedLLMConfig != nil && savedLLMConfig.Provider == "custom" {
		if savedLLMConfig.APIKey != "" {
			customKey = savedLLMConfig.APIKey
		}
		if savedLLMConfig.BaseURL != "" {
			customURL = savedLLMConfig.BaseURL
		}
	}
	llmRegistry.Register(llm.NewCustomProvider(customKey, customURL))

	// Grok provider (xAI)
	grokKey := os.Getenv("GROK_API_KEY")
	grokBaseURL := ""
	if savedLLMConfig != nil && savedLLMConfig.Provider == "grok" && savedLLMConfig.APIKey != "" {
		grokKey = savedLLMConfig.APIKey
		grokBaseURL = savedLLMConfig.BaseURL
	}
	if grokKey != "" {
		llmRegistry.Register(llm.NewGrokProvider(grokKey, grokBaseURL))
	} else {
		llmRegistry.Register(llm.NewGrokProvider("", ""))
	}

	// Qwen provider (Alibaba Cloud)
	qwenKey := os.Getenv("QWEN_API_KEY")
	qwenBaseURL := ""
	if savedLLMConfig != nil && savedLLMConfig.Provider == "qwen" && savedLLMConfig.APIKey != "" {
		qwenKey = savedLLMConfig.APIKey
		qwenBaseURL = savedLLMConfig.BaseURL
	}
	if qwenKey != "" {
		llmRegistry.Register(llm.NewQwenProvider(qwenKey, qwenBaseURL))
	} else {
		llmRegistry.Register(llm.NewQwenProvider("", ""))
	}

	// Initialize tools registry and register built-in tools
	toolRegistry := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)
	logger.Info().Int("count", len(toolRegistry.List())).Msg("Built-in tools registered")

	// Claude Code CLI provider (v0.10)
	if cfg.ClaudeCode.Enabled {
		ccConfig := convertClaudeCodeConfig(&cfg.ClaudeCode, claudeKey, claudeBaseURL)
		ccProvider := claudecode.NewProvider(ccConfig)
		ccProvider.SetToolRegistry(toolRegistry) // Set tool registry for system prompt
		ccProvider.Start()
		llmRegistry.Register(ccProvider)
		logger.Info().Str("command", cfg.ClaudeCode.Command).Msg("Claude Code CLI provider registered")

		// Register shutdown hook for Claude Code provider
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			return ccProvider.Close()
		})
	}

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
	var initMu sync.Mutex
	initErrors := make([]error, 0)

	// Group 1: Independent services (no dependencies on each other)
	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// Metrics collector (collect every 5 seconds, keep 10 minutes of history)
		metricsCollector = metrics.NewCollector(5*time.Second, 120)
		metricsCollector.Start()
		// Metrics writer for detailed API metrics
		metricsWriter = metrics.NewMetricsWriter(nil, metrics.DefaultWriterConfig())
		metricsWriter.Start()
		logger.Info().Msg("Metrics services initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// Backup manager
		var err error
		backupManager, err = backup.NewManager(backup.Config{
			Enabled:       true,
			RetentionDays: 7,
			Path:          filepath.Join(dataDir, "backups"),
		}, dataDir, dataDir)
		if err != nil {
			initMu.Lock()
			initErrors = append(initErrors, fmt.Errorf("backup manager: %w", err))
			initMu.Unlock()
			return
		}
		backupHandler = backup.NewHandler(backupManager)
		logger.Info().Msg("Backup manager initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// Security threat detector
		threatDetector = security.NewThreatDetector()
		securityHandler = security.NewHandler(threatDetector)
		logger.Info().Msg("Security handler initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// MFA handler
		mfaHandler = mfa.NewHandler(nil, nil)
		logger.Info().Msg("MFA handler initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// Sandbox manager
		var err error
		sandboxManager, err = sandbox.NewManager(nil)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize sandbox manager, sandbox features will be disabled")
			return
		}
		sandboxHandler = sandbox.NewHandler(sandboxManager)
		logger.Info().Bool("supported", sandboxManager.IsSupported()).Msg("Sandbox handler initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// Cron service
		cronService = cron.NewService(cron.DefaultConfig(), zapLogger)
		cronService.RegisterBuiltinHandlers()
		cronHandler = cron.NewHandler(cronService, zapLogger)
		if err := cronService.Start(); err != nil {
			logger.Warn().Err(err).Msg("Failed to start cron service")
		}
		logger.Info().Msg("Cron service initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// Home Assistant service
		haService = homeassistant.NewHAService()
		haHandler = homeassistant.NewHandler(haService)
		logger.Info().Msg("Home Assistant handler initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
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

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// A2UI manager
		a2uiManager = a2ui.NewManager(zapLogger)
		a2uiHandler = a2ui.NewHandler(a2uiManager)
		logger.Info().Msg("A2UI handler initialized")
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// STT service (Speech-to-Text)
		var err error
		sttService, err = stt.NewService(&stt.ServiceConfig{
			DefaultProvider: stt.ProviderWhisperAPI,
			Providers: []stt.ProviderConfig{
				{
					Type:    stt.ProviderWhisperAPI,
					Enabled: true,
					APIKey:  os.Getenv("OPENAI_API_KEY"),
				},
			},
		})
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize STT service, voice transcription will be disabled")
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
		// TTS service (Text-to-Speech)
		var err error
		ttsService, err = tts.NewService(&tts.ServiceConfig{
			DefaultProvider: tts.ProviderEdge,
			Providers: []tts.ProviderConfig{
				{
					Type:    tts.ProviderEdge,
					Enabled: true,
				},
				{
					Type:    tts.ProviderOpenAI,
					Enabled: os.Getenv("OPENAI_API_KEY") != "",
					APIKey:  os.Getenv("OPENAI_API_KEY"),
				},
			},
		})
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize TTS service, voice synthesis will be disabled")
		}
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
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
	}()

	initWg.Add(1)
	go func() {
		defer initWg.Done()
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

	initWg.Add(1)
	go func() {
		defer initWg.Done()
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
	}()

	// Wait for all parallel initializations to complete
	initWg.Wait()

	// Register deferred cleanup for metrics services
	if metricsCollector != nil {
		defer metricsCollector.Stop()
	}
	if metricsWriter != nil {
		defer metricsWriter.Stop()
	}

	// Check for fatal errors
	if len(initErrors) > 0 {
		for _, err := range initErrors {
			logger.Error().Err(err).Msg("Initialization error")
		}
		logger.Fatal().Msg("Failed to initialize required services")
	}

	// Initialize voice service (depends on STT and TTS)
	if sttService != nil && ttsService != nil {
		voiceService := voice.NewService(&voice.ServiceConfig{
			STTService: sttService,
			TTSService: ttsService,
		})
		voiceHandler = voice.NewHandler(voiceService)
		logger.Info().Msg("Voice handler initialized")
	}

	// Set metrics recorder on chat handler for API call tracking
	chatHandler.SetMetricsRecorder(metricsWriter)

	// Initialize HTTP server
	srv := server.New(&cfg.Server)
	server.SetVersion(version)
	srv.RegisterHealthRoutes()

	// Register API routes
	registerAPIRoutes(srv, pool, userHandler, extauthHandler, userService, chatHandler, autoreplyHandler, metricsCollector, metricsWriter, authMiddleware, apiKeyHandler, skillRegistry, pluginRegistry, pluginStore, backupHandler, toolRegistry, securityHandler, sandboxHandler, cronHandler, haHandler, browserHandler, a2uiHandler, workflowHandler, mfaHandler, voiceHandler, formfillerHandler, companionHandler, companionWSHandler, companionManager, zapLogger, version, buildTime, gitCommit, dataDir, cfg, llmRegistry)

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

	logger.Info().Msg("ZimaOS-Echo stopped")
}

func registerAPIRoutes(srv *server.Server, pool *worker.Pool, userHandler *user.Handler, extauthHandler *extauth.Handler, userService *user.Service, chatHandler *server.ChatHandler, autoreplyHandler *autoreply.Handler, metricsCollector *metrics.Collector, metricsWriter *metrics.MetricsWriter, authMiddleware *auth.AuthMiddleware, apiKeyHandler *auth.APIKeyHandler, skillRegistry *skill.Registry, pluginRegistry *plugin.Registry, pluginStore *plugin.Store, backupHandler *backup.Handler, toolRegistry *tools.Registry, securityHandler *security.Handler, sandboxHandler *sandbox.Handler, cronHandler *cron.Handler, haHandler *homeassistant.Handler, browserHandler *browser.Handler, a2uiHandler *a2ui.Handler, workflowHandler *workflow.Handler, mfaHandler *mfa.Handler, voiceHandler *voice.Handler, formfillerHandler *formfiller.Handler, companionHandler *companion.Handler, companionWSHandler *companion.WebSocketHandler, companionManager *companion.Manager, zapLogger *zap.Logger, version, buildTime, gitCommit, dataDir string, cfg *config.Config, llmRegistry *llm.ProviderRegistry) {
	e := srv.Echo()

	// Setup wizard routes (no auth required)
	setupHandler := setup.NewHandlerWithVersion(dataDir, version)
	// Set user creator for setup wizard to create admin user
	setupHandler.SetUserCreator(func(username, password string, isAdmin bool) error {
		role := user.RoleUser
		if isAdmin {
			role = user.RoleAdmin
		}
		_, err := userService.Create(context.Background(), &user.CreateUserRequest{
			Username: username,
			Password: password,
			Role:     role,
		})
		return err
	})
	// Set user checker for setup wizard to check if username exists
	setupHandler.SetUserChecker(func(username string) (bool, error) {
		return userService.ExistsByUsername(context.Background(), username)
	})
	setupHandler.RegisterRoutes(e)

	// API v1 group
	v1 := e.Group("/api/v1")

	// API group (for routes that don't use /api/v1 prefix)
	api := e.Group("/api")

	// Public auth routes (login, logout, providers list)
	v1.POST("/auth/login", userHandler.Login)
	v1.POST("/auth/logout", userHandler.Logout)

	// Register external auth routes (OAuth/OIDC providers) - public routes
	authGroup := v1.Group("/auth")
	extauthHandler.RegisterRoutes(authGroup)

	// Protected routes - require authentication
	protected := v1.Group("")
	protected.Use(authMiddleware.Authenticate())

	// Register protected external auth routes (account linking)
	protectedAuthGroup := protected.Group("/auth")
	extauthHandler.RegisterProtectedRoutes(protectedAuthGroup)

	// Register MFA routes (protected) - /api/v1/auth/mfa/*
	mfaHandler.RegisterRoutes(protected)

	// User routes (protected)
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

	// Password change (protected)
	protected.POST("/auth/password", userHandler.ChangePassword)

	// API Keys routes (protected)
	apiKeysGroup := protected.Group("/apikeys")
	apiKeyHandler.RegisterRoutes(apiKeysGroup)

	// Set up companion manager and prompt guard for chat handler
	if companionManager != nil {
		chatHandler.SetCompanionManager(companionManager)
		logger.Info().Msg("Companion manager set on chat handler")
	}
	promptGuard := promptguard.NewDetector(promptguard.DefaultDetectorConfig())
	chatHandler.SetPromptGuard(promptGuard)
	logger.Info().Msg("Prompt guard set on chat handler")

	// Register chat routes (conversations, providers, tools)
	chatHandler.RegisterRoutes(v1)

	// Register auto-reply routes
	autoreplyHandler.RegisterRoutes(v1)

	// Register health routes under /api/v1 as well
	srv.RegisterHealthRoutesOnGroup(v1)

	// Register network routes (public - for desktop app to get LAN addresses)
	networkHandler := networkapi.NewNetworkHandler(server.GetActualPort())
	networkHandler.RegisterRoutes(e)
	logger.Info().Msg("Network routes registered")

	// Register metrics routes (system metrics from collector)
	metricsHandler := server.NewMetricsHandler(metricsCollector)
	metricsHandler.RegisterRoutes(v1)

	// Register detailed metrics routes (API call stats, token usage, latency, etc.)
	detailedMetricsHandler := metrics.NewHandler(metricsWriter)
	metricsGroup := v1.Group("/metrics")
	detailedMetricsHandler.RegisterRoutes(metricsGroup)
	logger.Info().Msg("Detailed metrics routes registered")

	// Register system routes (logs, config, info)
	systemHandler := server.NewSystemHandler(version, buildTime, gitCommit, dataDir)
	systemHandler.RegisterRoutes(v1)

	// Register service management routes
	serviceHandler := server.NewServiceHandler()
	serviceHandler.RegisterRoutes(v1)

	// Register backup routes
	backupHandler.RegisterRoutes(v1)

	// Register skill routes (skills and skill store)
	skillHandler := server.NewSkillHandler(skillRegistry)
	skillHandler.RegisterRoutes(v1)

	// Register plugin routes (installed plugins management)
	pluginHandler := server.NewPluginHandler(pluginRegistry)
	pluginHandler.RegisterRoutes(v1)

	// Register plugin store routes
	pluginStoreHandler := server.NewPluginStoreHandler(pluginStore)
	pluginStoreHandler.RegisterRoutes(v1)

	// Register tool store routes (tools and tool store)
	toolStoreHandler := server.NewToolStoreHandler(toolRegistry)
	toolStoreHandler.RegisterRoutes(v1)

	// Register security routes (protected)
	securityGroup := protected.Group("/security")
	securityHandler.RegisterRoutes(securityGroup)

	// Register sandbox routes (protected)
	if sandboxHandler != nil {
		sandboxGroup := protected.Group("/sandbox")
		sandboxHandler.RegisterRoutes(sandboxGroup)
	}

	// Register cron routes under /api (protected via middleware on api group)
	apiProtected := api.Group("")
	apiProtected.Use(authMiddleware.Authenticate())

	// Register cron routes (protected) - /api/cron/*
	cronHandler.RegisterRoutes(apiProtected)

	// Register Home Assistant routes (protected) - /api/homeassistant/*
	haGroup := apiProtected.Group("/homeassistant")
	haHandler.RegisterRoutes(haGroup)

	// Register browser automation routes (protected) - /api/browser/*
	if browserHandler != nil {
		browserGroup := apiProtected.Group("/browser")
		browserHandler.RegisterRoutes(browserGroup)
	}

	// Register A2UI routes (protected) - /api/a2ui/*
	a2uiGroup := apiProtected.Group("/a2ui")
	a2uiHandler.RegisterRoutes(a2uiGroup)

	// Register workflow routes - /api/v1/workflows/*
	if workflowHandler != nil {
		workflowHandler.RegisterRoutes(e)
	}

	// Register voice routes (public for transcribe/synthesize) - /api/v1/voice/*
	if voiceHandler != nil {
		voiceGroup := v1.Group("/voice")
		voiceHandler.RegisterRoutes(voiceGroup)
		logger.Info().Msg("Voice routes registered")
	}

	// Register form filler routes (protected) - /api/v1/formfiller/*
	if formfillerHandler != nil {
		formfillerGroup := protected.Group("/formfiller")
		formfillerHandler.RegisterRoutes(formfillerGroup)
		logger.Info().Msg("Form filler routes registered")
	}

	// Register Claude Code CLI version management routes (protected) - /api/v1/claudecode/*
	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir)
	claudeCodeGroup := protected.Group("/claudecode")
	claudeCodeHandler.RegisterRoutes(claudeCodeGroup)
	chatHandler.SetClaudeCodeHandler(claudeCodeHandler)
	logger.Info().Msg("Claude Code CLI routes registered")

	// Register provider settings routes (protected) - /api/v1/providers/settings/*
	providerSettingsHandler := server.NewProviderSettingsHandler(chatHandler.GetProviderRegistry(), dataDir)
	providerSettingsGroup := protected.Group("/providers/settings")
	providerSettingsHandler.RegisterRoutes(providerSettingsGroup)
	logger.Info().Msg("Provider settings routes registered")

	// Register provider pool routes (protected) - /api/v1/providers/*
	providerPoolPath := filepath.Join(dataDir, "providerpool")

	// Create provider pool (no encryption)
	providerPool, err := providerpool.NewPool(providerPoolPath)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to initialize provider pool, provider pool features will be disabled")
	} else {
		providerPool.Start(context.Background())

		// Auto-migrate from legacy provider settings
		if providerpool.CheckMigrationNeeded(dataDir) {
			logger.Info().Msg("Legacy provider settings detected, starting migration...")
			result, err := providerpool.MigrateFromLegacy(dataDir, providerPool)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to migrate legacy provider settings")
			} else if result.Migrated > 0 {
				logger.Info().
					Int("migrated", result.Migrated).
					Int("skipped", result.Skipped).
					Strs("migrated_names", result.MigratedNames).
					Str("backup_path", result.BackupPath).
					Msg("Legacy provider settings migrated successfully")
			}
		}

		// Load providers from Provider Pool and register them in LLM registry
		// This replaces the environment variable-based provider registration
		loadProvidersFromPool(providerPool, llmRegistry)

		providerPoolHandler := providerpool.NewHandler(providerPool)
		providersGroup := protected.Group("/providers")
		providerPoolHandler.RegisterRoutes(providersGroup)
		// Register model routes
		modelsGroup := v1.Group("/models")
		providerPoolHandler.RegisterModelRoutes(modelsGroup)
		// Register IDE routes
		ideGroup := protected.Group("/ide")
		providerPoolHandler.RegisterIDERoutes(ideGroup)
		// Register pricing routes
		pricingGroup := protected.Group("/pricing")
		providerPoolHandler.RegisterPricingRoutes(pricingGroup)
		logger.Info().Msg("Provider pool routes registered")

		// Register proxy failover routes
		failoverConfig := proxy.DefaultProxyConfig().Routing.Failover
		failoverHandler := proxy.NewFailoverAPIHandler(nil, &failoverConfig)
		failoverGroup := protected.Group("/proxy/failover")
		failoverHandler.RegisterRoutes(failoverGroup)
		logger.Info().Msg("Proxy failover routes registered")

		// Register OpenAI-compatible proxy routes on /v1/* (unified port architecture)
		// This allows external apps and Claude Code CLI to use Echo as an OpenAI-compatible API
		if cfg.Proxy != nil && cfg.Proxy.Enabled {
			logger.Info().Msg("Initializing OpenAI-compatible proxy on /v1/*")

			// Use Routing config (prefer Route over deprecated Routing field)
			routingConfig := &cfg.Proxy.Routing
			if cfg.Proxy.Route != nil {
				routingConfig = cfg.Proxy.Route
			}

			// Create proxy components
			proxyRouter := proxy.NewRouter(routingConfig)
			proxyConnPool := proxy.NewConnectionPool(&cfg.Proxy.Connection)
			proxyFailover := proxy.NewFailoverHandler(&routingConfig.Failover, proxyRouter)
			proxyHandler := proxy.NewProxyHandler(proxyRouter, proxyConnPool, proxyFailover)

			// Create /v1 group (no /api prefix - OpenAI-compatible)
			v1ProxyGroup := e.Group("/v1")

			// Optional: Add authentication middleware if configured
			// For now, we'll make it public to allow easy integration with Claude Code CLI
			// Users can add authentication via API keys in the provider configuration

			// Register OpenAI-compatible endpoints
			// These endpoints forward requests to configured providers (Anthropic, OpenAI, etc.)
			v1ProxyGroup.Any("/chat/completions", echo.WrapHandler(proxyHandler))
			v1ProxyGroup.Any("/completions", echo.WrapHandler(proxyHandler))
			v1ProxyGroup.Any("/embeddings", echo.WrapHandler(proxyHandler))
			v1ProxyGroup.Any("/models", echo.WrapHandler(proxyHandler))

			logger.Info().
				Str("path", "/v1/*").
				Str("providers", fmt.Sprintf("%d configured", len(routingConfig.Providers))).
				Str("load_balancing", routingConfig.LoadBalancing).
				Bool("failover", routingConfig.Failover.Enabled).
				Msg("OpenAI-compatible proxy routes registered (unified port)")
		}
	}

	// Register companion routes (Echo Companion - real-time AI Agent monitoring)
	if companionHandler != nil {
		companionHandler.RegisterRoutes(e)
		logger.Info().Msg("Companion REST routes registered")
	}
	if companionWSHandler != nil {
		companionWSHandler.RegisterRoutes(e)
		logger.Info().Msg("Companion WebSocket routes registered")
	}

	// Register channel config routes (public for now, channels page needs to work without auth)
	channelConfigStore := server.NewChannelConfigStore(dataDir)
	channelConfigHandler := server.NewChannelConfigHandler(channelConfigStore)

	// Initialize channel manager for connection lifecycle management
	channelManager := channel.NewManager(channel.DefaultConfig(), zapLogger)
	channelFactory := server.NewChannelFactory(zapLogger)

	// Register channels from saved configurations and start enabled ones in parallel
	enabledChannels := channelConfigStore.GetEnabled()
	if len(enabledChannels) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex // Protect channelConfigStore writes

		for _, cfg := range enabledChannels {
			wg.Add(1)
			go func(cfg *server.ChannelConfig) {
				defer wg.Done()

				ch, err := channelFactory.CreateChannel(cfg)
				if err != nil {
					logger.Warn().Str("channel", cfg.ID).Err(err).Msg("Failed to create channel")
					return
				}
				if ch == nil {
					return // Unknown channel type
				}
				if err := channelManager.Register(ch); err != nil {
					logger.Warn().Str("channel", cfg.ID).Err(err).Msg("Failed to register channel")
					return
				}
				// Start the channel
				if err := channelManager.StartChannel(context.Background(), cfg.ID); err != nil {
					logger.Warn().Str("channel", cfg.ID).Err(err).Msg("Failed to start channel")
					// Update config status to error
					mu.Lock()
					cfg.Status = "error"
					cfg.LastError = err.Error()
					_ = channelConfigStore.Set(cfg.ID, cfg)
					mu.Unlock()
				} else {
					// Update config status to connected
					mu.Lock()
					cfg.Status = "connected"
					cfg.LastError = ""
					_ = channelConfigStore.Set(cfg.ID, cfg)
					mu.Unlock()
					logger.Info().Str("channel", cfg.ID).Msg("Channel started successfully")
				}
			}(cfg)
		}

		// Wait for all channels to start (with timeout)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Info().Int("count", len(enabledChannels)).Msg("All channels started")
		case <-time.After(10 * time.Second):
			logger.Warn().Msg("Channel startup timed out, some channels may still be starting")
		}
	}

	// Set manager on handler for runtime connection management
	channelConfigHandler.SetManager(channelManager)
	channelConfigHandler.SetFactory(channelFactory)
	channelConfigHandler.RegisterRoutes(api)
	logger.Info().Msg("Channel config routes registered")

	// Worker stats endpoint
	v1.GET("/workers/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, pool.Stats())
	})

	// Register static file routes for embedded frontend (must be last)
	if web.IsEmbedded() {
		logger.Info().Msg("Serving embedded frontend assets")
	} else {
		logger.Info().Msg("Development mode: proxying to Vite dev server")
	}
	web.RegisterStaticRoutes(e)
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
