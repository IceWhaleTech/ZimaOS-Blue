package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	concpool "github.com/sourcegraph/conc/pool"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	skillEmbed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/embedded"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
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

func main() {
	// Fast-path: CLI subcommands bypass cobra to minimize page faults and RSS.
	// All init() functions have already run, but we avoid touching cobra's
	// command tree, flag parsing, and the heavy code paths they pull in.
	// This must run BEFORE macosRequestSTTAuthorization() so IPC calls
	// (e.g. `blue web_search ...`) don't trigger CGo/Speech framework
	// initialization, log output, or any server-side side effects.
	if len(os.Args) > 1 {
		if cliDispatch(os.Args[1:]) {
			return
		}
	}

	// On macOS, request speech recognition authorization on thread 0
	// BEFORE starting the server. runtime.LockOSThread() in macos_init.go
	// pins this goroutine to thread 0 (required by AppKit/TCC).
	macosRequestSTTAuthorization()

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
	// Tune GC for lower memory usage (shared with bluelib)
	bootstrap.TuneGC()

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
	if err := applyPendingBackupRestore(dataDir); err != nil {
		logger.Warn().Err(err).Msg("Failed to apply pending backup restore before database initialization")
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

	// Shared kvstore for all config persistence (replaces scattered JSON files)
	sqliteKV, err := kvstore.NewSQLiteStoreWithDB(db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize config kvstore")
	}
	configKV := kvstore.NewCachedStore(sqliteKV)

	// Import config.yaml into kvstore (first-run: imports; subsequent: loads from DB)
	configStore := config.NewConfigStore(configKV)
	cfg, err = configStore.LoadOrImport(cfg)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to load config from DB, using YAML defaults")
	}

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
	tools.RegisterFactoryToolDefinitions(toolRegistry)
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
	if cfg.Session.Audit.Enabled {
		auditDBPath := cfg.Session.Audit.Path
		if auditDBPath == "" {
			auditDBPath = filepath.Join(dataDir, "session_audit.db")
		}
		if !filepath.IsAbs(auditDBPath) {
			// Keep audit DB under dataDir by default for predictable deployment paths.
			auditDBPath = filepath.Join(dataDir, filepath.Base(auditDBPath))
		}
		if err := os.MkdirAll(filepath.Dir(auditDBPath), 0o750); err != nil {
			logger.Warn().Err(err).Str("path", auditDBPath).Msg("Failed to create session audit directory")
		} else {
			auditStore, err := sessionaudit.NewSQLiteStore(auditDBPath, sessionaudit.StoreConfig{
				RetentionDays:    cfg.Session.Audit.RetentionDays,
				CleanupInterval:  cfg.Session.Audit.CleanupInterval,
				CleanupBatchSize: cfg.Session.Audit.CleanupBatchSize,
			})
			if err != nil {
				logger.Warn().Err(err).Str("path", auditDBPath).Msg("Failed to initialize session audit store")
			} else {
				chatHandler.SetSessionAuditStore(auditStore)
				logger.Info().Str("path", auditDBPath).Int("retention_days", cfg.Session.Audit.RetentionDays).Msg("Session tool payload audit store enabled")
			}
		}
	}
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		_ = ctx
		chatHandler.Shutdown()
		return nil
	})

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
		metricsCollector  *metrics.Collector
		metricsWriter     *metrics.MetricsWriter
		backupManager     *backup.Manager
		backupHandler     *backup.Handler
		threatDetector    *security.ThreatDetector
		securityHandler   *security.Handler
		mfaHandler        *mfa.Handler
		sandboxManager    *sandbox.Manager
		sandboxHandler    *sandbox.Handler
		cronHandler       *cron.Handler
		haService         *homeassistant.HAService
		haHandler         *homeassistant.Handler
		browserHandler    *browser.Handler
		sttService        stt.Service
		ttsService        tts.Service
		voiceHandler      *voice.Handler
		workflowHandler   *workflow.Handler
		formfillerStore   *formfiller.Store
		formfillerHandler *formfiller.Handler
		ngrokTunnelMgr    *ngrok.SDKTunnelManager
		ngrokConfigStore  *ngrok.ConfigStore
	)

	// Use conc/pool for safer parallel initialization with automatic panic recovery
	initPool := concpool.New().WithMaxGoroutines(8)

	// Metrics: async init — chat handler nil-checks metricsRecorder, so first
	// requests simply skip recording until metrics is ready.  This avoids
	// blocking server start on metrics.db open + collector goroutine.
	go func() {
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

	initPool.Go(func() {
		// Backup manager
		var err error
		backupManager, err = backup.NewManager(backup.Config{
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
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize backup manager")
			return
		}
		backupManager.StartAutoBackup(lm.Context())
		backupHandler = backup.NewHandler(backupManager)
		backupHandler.SetRestartFunc(func() error {
			logger.Info().Msg("Backup restore staged; sending SIGTERM for graceful restart")
			proc, err := os.FindProcess(os.Getpid())
			if err != nil {
				return err
			}
			return proc.Signal(syscall.SIGTERM)
		})
		logger.Info().Msg("Backup manager initialized")
	})

	// Browser automation service — lazy init, only when first API call arrives
	// Chromium is very heavy on memory, skip at startup
	logger.Info().Msg("Browser automation will be initialized on first use")

	// TTS service — pick OS-appropriate default provider (async, not needed at startup)
	initPool.Go(func() {
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
	})

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

	cronIPC := sockipc.NewCronIPCAdapter(cron.NewSkillAdapter(cronHandler.GetService))

	// SSE event broker — created early so push service can use it as EventPublisher
	sseBroker := ssePkg.NewBroker()

	// Wire Web Push notification support (shared with bluelib)
	wpSender := bootstrap.InitWebPushSender(db, configKV, zapLogger)

	// Wire push notification service (shared with bluelib)
	pushResult := bootstrap.InitPushService(&bootstrap.PushServiceDeps{
		DB:          db,
		MemoryStore: memoryStore,
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

	// Wire browser service — lazy init, creates rod service on first use (for IPC only)
	var browserBackend tools.BrowserBackend
	var lazyBrowserSvc func() *browser.RodService
	{
		var browserOnce sync.Once
		var browserSvc *browser.RodService
		lazyBrowserSvc = func() *browser.RodService {
			browserOnce.Do(func() {
				svc, err := browser.NewService(nil)
				if err != nil {
					logger.Warn().Err(err).Msg("Failed to create browser service")
					return
				}
				browserSvc = svc
				logger.Info().Msg("Browser service initialized")
			})
			return browserSvc
		}

		browserBackend = tools.NewLazyRodBrowserBackend(lazyBrowserSvc)
	}

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

	var companionHandler *companion.Handler
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

	// Initialize voice WebSocket handler (created here so shutdown can close connections)
	var voiceWSHandler *voice.WSHandler
	if voiceHandler != nil {
		voiceWSHandler = voice.NewWSHandler(voiceHandler.Service())
	}

	// Initialize ngrok config store (kvstore-based, lazy initialization)
	ngrokConfigStore = ngrok.NewConfigStore(configKV)
	ngrokTunnelMgr = ngrok.NewSDKTunnelManager(nil)
	logger.Info().Msg("Ngrok tunnel services initialized (lightweight)")

	// Note: Metrics recorder is set on chat handler asynchronously after metrics initialization completes

	// Initialize HTTP server
	srv := server.New(&cfg.Server)
	server.SetVersion(version)
	srv.RegisterHealthRoutes()

	// Register API routes
	registerAPIRoutes(srv, pool, userHandler, extauthHandler, userService, chatHandler, autoreplyService, autoreplyHandler, metricsCollector, metricsWriter, authMiddleware, apiKeyHandler, apiKeyService, skillRegistry, pluginRegistry, pluginStore, backupHandler, toolRegistry, securityHandler, sandboxHandler, sandboxManager, cronHandler, haHandler, browserHandler, workflowHandler, mfaHandler, voiceHandler, voiceWSHandler, formfillerHandler, companionHandler, companionWSHandler, ngrokTunnelMgr, ngrokConfigStore, zapLogger, version, buildTime, gitCommit, dataDir, cfg, llmRegistry, db, jwtService, permissionHandler, sttService, ttsService, lm, hotReloader, sseBroker, pushIPC, pushSvc, cronIPC, browserBackend, lazyBrowserSvc, configKV, configStore)

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

	// Force-cancel on second signal — don't let the process hang
	go func() {
		sig := <-quit
		logger.Warn().Str("signal", sig.String()).Msg("Received second signal, cancelling graceful shutdown")
		cancel()
	}()

	server.SetReady(false)

	// Close SSE broker first — this makes all SSE handlers return,
	// so httpServer.Shutdown() won't block waiting for long-lived connections.
	sseBroker.Close()

	// Close WebSocket connections so httpServer.Shutdown() doesn't block.
	if companionWSHandler != nil {
		companionWSHandler.Close()
	}
	if voiceWSHandler != nil {
		voiceWSHandler.Close()
	}

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

	// Clean up extracted web dist from tmpfs
	web.CleanupDist()

	logger.Info().Msg("ZimaOS-Blue stopped")
}

func registerAPIRoutes(srv *server.Server, pool *worker.Pool, userHandler *user.Handler, extauthHandler *extauth.Handler, userService *user.Service, chatHandler *server.ChatHandler, autoreplyService *autoreply.Service, autoreplyHandler *autoreply.Handler, metricsCollector *metrics.Collector, metricsWriter *metrics.MetricsWriter, authMiddleware *auth.AuthMiddleware, apiKeyHandler *auth.APIKeyHandler, apiKeyService *auth.APIKeyService, skillRegistry *skill.Registry, pluginRegistry *plugin.Registry, pluginStore *plugin.Store, backupHandler *backup.Handler, toolRegistry *tools.Registry, securityHandler *security.Handler, sandboxHandler *sandbox.Handler, sandboxManager *sandbox.Manager, cronHandler *cron.Handler, haHandler *homeassistant.Handler, browserHandler *browser.Handler, workflowHandler *workflow.Handler, mfaHandler *mfa.Handler, voiceHandler *voice.Handler, voiceWSHandler *voice.WSHandler, formfillerHandler *formfiller.Handler, companionHandler *companion.Handler, companionWSHandler *companion.WebSocketHandler, ngrokTunnelMgr *ngrok.SDKTunnelManager, ngrokConfigStore *ngrok.ConfigStore, zapLogger *zap.Logger, version, buildTime, gitCommit, dataDir string, cfg *config.Config, llmRegistry *llm.ProviderRegistry, db *sql.DB, jwtService *auth.JWTService, permissionHandler *permission.Handler, sttService stt.Service, ttsService tts.Service, lm *lifecycle.Manager, hotReloader *config.HotReloader, sseBroker *ssePkg.Broker, pushIPC sockipc.PushBackend, pushSvc *push.Service, cronIPC sockipc.CronBackend, browserBackend tools.BrowserBackend, lazyBrowserSvc func() *browser.RodService, configKV kvstore.Store, configStore *config.ConfigStore) {
	e := srv.Echo()
	logger := zapLogger

	// Initialize speech handler
	speechKV := configKV
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

	// Restore persisted settings from kvstore
	speechHandler.RestoreEditBeforeSend()

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
	// On macOS/Windows, initialize native STT in background — not needed for first request
	logger.Info("ASR provider setup", zap.String("goos", runtime.GOOS), zap.Bool("sttServiceNil", sttService == nil))
	go func() {
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
	}()

	// Initialize workspace (SOUL.md, USER.md, IDENTITY.md, etc.)
	workspaceMgr := workspace.NewManager(filepath.Join(dataDir, "workspace"))
	if err := workspaceMgr.EnsureWorkspace(); err != nil {
		logger.Warn("Failed to initialize workspace", zap.Error(err))
	}

	// Initialize Claude Code handler
	claudeCodeHandler := claudecode.NewHandlerWithDataDir(nil, dataDir, configKV)
	workspaceDir := workspaceMgr.Dir() // {dataDir}/workspace/
	systemPromptBuilder := claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{WorkspaceDir: workspaceDir})
	systemPromptBuilder.SetToolRegistry(toolRegistry)
	systemPromptBuilder.SetWorkspace(workspaceMgr)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)

	// Initialize memory handler (markdown primary, optional dual-write with vector store)
	var memoryHandler *server.MemoryHandler
	memoryHandler = server.NewLazyMemoryHandler(func(h *server.MemoryHandler) error {
		memoryDir := cfg.Memory.MarkdownDir
		if memoryDir == "" {
			memoryDir = workspaceMgr.MemoryDir()
		}
		mdBackend, err := memory.NewPureMarkdownBackend(memoryDir)
		if err != nil {
			logger.Warn("Failed to initialize markdown backend", zap.Error(err))
			return err
		}
		unifiedService := memory.NewUnifiedMemoryService(mdBackend)
		h.SetUnifiedService(unifiedService)

		// Try to set up dual-write backend with vector store + hybrid search
		if cfg.Memory.VectorStore.Enabled {
			if dualBackend := initDualWriteBackend(cfg, mdBackend); dualBackend != nil {
				unifiedService.SetBackend(dualBackend)
				logger.Info("Memory service initialized (dual-write: markdown + vector store)")
			} else {
				logger.Info("Memory service initialized (markdown-only, vector store init failed)")
			}
		} else {
			logger.Info("Memory service initialized (markdown backend)", zap.String("dir", memoryDir))
		}

		layeredService, err := memory.NewLayeredMemoryService(unifiedService, memory.LayeredMemoryConfig{
			BaseDir:            memoryDir,
			LongTermDir:        workspaceMgr.Dir(),
			DailyRetentionDays: 30,
		})
		if err != nil {
			logger.Warn("Failed to initialize layered memory service", zap.Error(err))
		} else {
			h.SetLayeredService(layeredService)
		}
		toolsAdapter := memory.NewToolsAdapter(unifiedService)
		tools.RegisterMemoryTools(toolRegistry, toolsAdapter)

		return nil
	})
	// Trigger init asynchronously — recall/extract gracefully skip when layeredMemory is nil
	go memoryHandler.Init()

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(configKV)

	// Initialize provider pool (SQLite-backed, auto-migrates from JSON files)
	var providerPool *providerpool.Pool
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	providerPool, ppErr := providerpool.NewPool(providerPoolPath, providerpool.WithDB(db))
	if ppErr != nil {
		logger.Warn("Failed to initialize provider pool", zap.Error(ppErr))
		providerPool = nil
	}

	if providerPool != nil {
		// Stop provider pool background goroutines (health checks, usage tracker) on shutdown
		pp := providerPool
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			pp.Stop()
			return nil
		})

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

	}

	// Call bootstrap.RegisterAllRoutes with all dependencies
	routeUserRepo, _ := user.NewSQLiteRepository(db)

	// Initialize session manager + handler (shared control plane for API + gateway methods).
	sessionDBPath := cfg.Session.Persistence.Path
	if sessionDBPath == "" {
		sessionDBPath = filepath.Join(dataDir, "sessions.db")
	}
	if !filepath.IsAbs(sessionDBPath) {
		// Keep persistence under dataDir for predictable deployment paths.
		sessionDBPath = filepath.Join(dataDir, filepath.Base(sessionDBPath))
	}
	if err := os.MkdirAll(filepath.Dir(sessionDBPath), 0o750); err != nil {
		logger.Warn("Failed to create session persistence directory", zap.String("path", sessionDBPath), zap.Error(err))
	}

	var sessionStore session.SessionStore
	if cfg.Session.Persistence.Enabled {
		store, err := session.NewSQLiteSessionStore(sessionDBPath, cfg.Session.MaxTokens)
		if err != nil {
			logger.Warn("Failed to initialize session store", zap.String("path", sessionDBPath), zap.Error(err))
		} else {
			sessionStore = store
		}
	}
	var compactionProvider llm.Provider
	if providerNames := llmRegistry.List(); len(providerNames) > 0 {
		compactionProvider = llmRegistry.Get(providerNames[0])
	}
	sessionCompactor := session.NewSessionCompactor(compactionProvider, cfg.Session.Compaction)
	sessionManager := session.NewSessionManager(cfg.Session, sessionStore, sessionCompactor)
	if err := sessionManager.Recover(); err != nil {
		logger.Warn("Failed to recover persisted sessions", zap.Error(err))
	}
	sessionHandler := server.NewSessionHandler(sessionManager)
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		_ = ctx
		return sessionManager.Stop()
	})

	// Initialize gateway runtime + HTTP handler; method handlers are wired in bootstrap routes.
	gatewayRuntime := gateway.NewGateway(gateway.DefaultConfig(), zapLogger)
	gatewayHandler := gateway.NewHandlerWithAuth(gatewayRuntime, zapLogger, jwtService)

	// Create IPC adapters for browser, UI reviewer, and push notification SKILLs
	browserIPC := sockipc.NewToolBrowserIPCAdapter(browserBackend)
	uiReviewerIPC := sockipc.NewUIReviewIPCAdapter(&tools.UIReviewerTool{}) // placeholder, routes.go creates the real one

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
			UserRepo:      routeUserRepo,
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
		SessionHandler:     sessionHandler,
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
		BrowserIPC:         browserIPC,
		UIReviewerIPC:      uiReviewerIPC,
		PushIPC:            pushIPC,
		PushService:        pushSvc,
		CronIPC:            cronIPC,
		WorkflowHandler:    workflowHandler,
		VoiceHandler:       voiceHandler,
		VoiceWSHandler:     voiceWSHandler,
		FormfillerHandler:  formfillerHandler,
		CompanionHandler:   companionHandler,
		CompanionWSHandler: companionWSHandler,
		ProviderPool:       providerPool,
		APIKeyService:      apiKeyService,
		SpeechHandler:      speechHandler,
		STTService:         sttService,
		NgrokTunnelMgr:     ngrokTunnelMgr,
		NgrokConfigStore:   ngrokConfigStore,
		ClaudeCodeHandler:  claudeCodeHandler,
		MemoryHandler:      memoryHandler,
		ChannelConfigStore: channelConfigStore,
		ConfigKV:           configKV,
		ConfigStore:        configStore,
		HotReloader:        hotReloader,
		WorkspaceHandler:   workspace.NewHandler(workspaceMgr),
		SSEBroker:          sseBroker,
		Gateway:            gatewayRuntime,
		GatewayHandler:     gatewayHandler,
		// Consolidated init deps
		SkillEmbedFS:        skillEmbed.SkillsFS,
		SandboxManager:      sandboxManager,
		SystemPromptBuilder: systemPromptBuilder,
		LazyBrowserSvc:      lazyBrowserSvc,
		BrowserBackend:      browserBackend,
	}

	_ = bootstrap.RegisterAllRoutes(e, deps)

	// Register shutdown hooks for closers started during route registration (e.g., sockipc)
	for _, c := range deps.Closers {
		closer := c // capture for closure
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			return closer.Close()
		})
	}
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

// initDualWriteBackend creates a DualWriteBackend with VectorStore + HybridSearcher.
// Returns nil if initialization fails (caller should fall back to markdown-only).
func initDualWriteBackend(cfg *config.Config, mdBackend *memory.PureMarkdownBackend) *memory.DualWriteBackend {
	log := logger.Get()

	// Resolve DB path
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

	// Use configured dims or fallback; actual dims resolved lazily on first embed
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

	// Create VectorStore (after embedding provider so dims are known)
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

	// Create HybridSearcher → MemoryService → DualWriteBackend
	searcher := memory.NewHybridSearcher(vs, embProvider, cfg.Memory)
	memSvc := memory.NewMemoryService(searcher)
	return memory.NewDualWriteBackend(mdBackend, memSvc)
}
