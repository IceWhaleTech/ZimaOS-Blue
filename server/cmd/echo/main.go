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
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/lifecycle"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/setup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/worker"
)

var (
	version   = "0.9.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

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

	// Register default LLM providers
	// OpenAI provider (API key can be set via environment or config)
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		llmRegistry.Register(llm.NewOpenAIProvider(openaiKey, ""))
	} else {
		// Register with empty key - will fail on actual API calls but allows listing
		llmRegistry.Register(llm.NewOpenAIProvider("", ""))
	}

	// Claude provider
	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	if claudeKey != "" {
		llmRegistry.Register(llm.NewClaudeProvider(claudeKey, ""))
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

	// Initialize metrics collector (collect every 5 seconds, keep 10 minutes of history)
	metricsCollector := metrics.NewCollector(5*time.Second, 120)
	metricsCollector.Start()
	defer metricsCollector.Stop()

	// Initialize backup manager
	backupManager, err := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(dataDir, "backups"),
	}, dataDir, dataDir)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize backup manager")
	}
	backupHandler := backup.NewHandler(backupManager)

	// Initialize HTTP server
	srv := server.New(&cfg.Server)
	server.SetVersion(version)
	srv.RegisterHealthRoutes()

	// Register API routes
	registerAPIRoutes(srv, pool, userHandler, extauthHandler, userService, chatHandler, autoreplyHandler, metricsCollector, authMiddleware, apiKeyHandler, skillRegistry, pluginRegistry, pluginStore, backupHandler, toolRegistry, version, buildTime, gitCommit, dataDir)

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

func registerAPIRoutes(srv *server.Server, pool *worker.Pool, userHandler *user.Handler, extauthHandler *extauth.Handler, userService *user.Service, chatHandler *server.ChatHandler, autoreplyHandler *autoreply.Handler, metricsCollector *metrics.Collector, authMiddleware *auth.AuthMiddleware, apiKeyHandler *auth.APIKeyHandler, skillRegistry *skill.Registry, pluginRegistry *plugin.Registry, pluginStore *plugin.Store, backupHandler *backup.Handler, toolRegistry *tools.Registry, version, buildTime, gitCommit, dataDir string) {
	e := srv.Echo()

	// Setup wizard routes (no auth required)
	setupHandler := setup.NewHandler("./data")
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
	setupHandler.RegisterRoutes(e)

	// API v1 group
	v1 := e.Group("/api/v1")

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

	// Register chat routes (conversations, providers, tools)
	chatHandler.RegisterRoutes(v1)

	// Register auto-reply routes
	autoreplyHandler.RegisterRoutes(v1)

	// Register health routes under /api/v1 as well
	srv.RegisterHealthRoutesOnGroup(v1)

	// Register metrics routes
	metricsHandler := server.NewMetricsHandler(metricsCollector)
	metricsHandler.RegisterRoutes(v1)

	// Register system routes (logs, config, info)
	systemHandler := server.NewSystemHandler(version, buildTime, gitCommit, dataDir)
	systemHandler.RegisterRoutes(v1)

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
