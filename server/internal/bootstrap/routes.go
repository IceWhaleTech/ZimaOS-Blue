// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	networkapi "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/preview"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/workflow"
)

// RoutesDeps holds all dependencies needed for route registration
type RoutesDeps struct {
	DB               *sql.DB
	Config           *config.Config
	ServerConfig     *ServerConfig
	Services         *Services
	Logger           *zap.Logger
	Ctx              context.Context
	MetricsWriter    *metrics.MetricsWriter
	MetricsCollector *metrics.Collector
	ChatHandler      *server.ChatHandler
	PluginRegistry   *plugin.Registry
	PluginStore      *plugin.Store
	ExtauthService   extauth.Service
	ExtauthHandler   *extauth.Handler
	AutoreplyService *autoreply.Service
	AutoreplyHandler *autoreply.Handler
	AuthMiddleware   *auth.AuthMiddleware
	APIKeyHandler    *auth.APIKeyHandler
	UserHandler      *user.Handler
	// Additional handlers
	BackupHandler      *backup.Handler
	SecurityHandler    *security.Handler
	SandboxHandler     *sandbox.Handler
	CronHandler        *cron.Handler
	HAHandler          *homeassistant.Handler
	BrowserHandler     *browser.Handler
	WorkflowHandler    *workflow.Handler
	VoiceHandler       *voice.Handler
	VoiceWSHandler     *voice.WSHandler
	FormfillerHandler  *formfiller.Handler
	CompanionHandler   *companion.Handler
	CompanionWSHandler *companion.WebSocketHandler
	ProviderPool       *providerpool.Pool
	APIKeyService      *auth.APIKeyService
	SpeechHandler      *speech.Handler
	NgrokTunnelMgr     *ngrok.SDKTunnelManager
	NgrokConfigStore   *ngrok.ConfigStore
	ClaudeCodeHandler  *claudecode.Handler
	MemoryHandler      *server.MemoryHandler
	ChannelConfigStore *server.ChannelConfigStore
	SharedCache        *proxy.CCCache
}

// RegisterAllRoutes registers all API routes on the Echo instance
func RegisterAllRoutes(e *echo.Echo, deps *RoutesDeps) {
	s := deps.Services
	cfg := deps.ServerConfig
	logger := deps.Logger

	// HTTPS redirect middleware (must be first)
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager != nil {
		e.Use(tlsManager.HTTPSRedirectMiddleware())
	}

	// Initialize connection manager
	connManager := connection.NewManager(10000, 5*time.Second)
	e.Use(connManager.Middleware())

	// Preview mode routes (no auth required)
	previewModeService := preview.NewModeService(s.UserService)
	previewUpgradeService := preview.NewUpgradeService(s.UserService, s.DB)
	previewHandler := preview.NewHandler(previewModeService, previewUpgradeService, s.JWTService)
	previewHandler.RegisterRoutes(e)
	logger.Info("Preview mode routes registered")

	// API groups
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	// Health endpoint
	v1.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": "zimaos-echo",
			"version": cfg.Version,
		})
	})

	// Worker stats endpoint (public, for bootstrap/health checks)
	v1.GET("/workers/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"pool_size": 10,
			"running":   0,
			"total":     0,
		})
	})

	// Public auth routes
	v1.POST("/auth/login", deps.UserHandler.Login)
	v1.POST("/auth/logout", deps.UserHandler.Logout)

	// External auth routes
	authGroup := v1.Group("/auth")
	if deps.ExtauthHandler != nil {
		deps.ExtauthHandler.RegisterRoutes(authGroup)
	}

	// Protected routes
	protected := v1.Group("")
	protected.Use(deps.AuthMiddleware.Authenticate())

	// Protected external auth routes
	if deps.ExtauthHandler != nil {
		protectedAuthGroup := protected.Group("/auth")
		deps.ExtauthHandler.RegisterProtectedRoutes(protectedAuthGroup)
	}

	// MFA routes
	mfaHandler := mfa.NewHandler(nil, nil)
	mfaHandler.RegisterRoutes(protected)

	// User routes
	usersGroup := protected.Group("/users")
	usersGroup.GET("/me", deps.UserHandler.GetCurrentUser)
	usersGroup.PUT("/me", deps.UserHandler.UpdateCurrentUser)
	usersGroup.GET("", deps.UserHandler.ListUsers)
	usersGroup.POST("", deps.UserHandler.CreateUser)
	usersGroup.GET("/:id", deps.UserHandler.GetUser)
	usersGroup.PUT("/:id", deps.UserHandler.UpdateUser)
	usersGroup.DELETE("/:id", deps.UserHandler.DeleteUser)
	usersGroup.POST("/:id/lock", deps.UserHandler.LockUser)
	usersGroup.POST("/:id/unlock", deps.UserHandler.UnlockUser)
	usersGroup.POST("/:id/reset-password", deps.UserHandler.ResetPassword)

	// Permission routes
	permRepo, _ := permission.NewRepository(s.DB)
	permService := permission.NewService(permRepo, s.UserRepo)
	permHandler := permission.NewHandler(permService, s.UserRepo)
	permHandler.RegisterRoutes(protected)
	logger.Info("Permission routes registered")

	// Password change
	protected.POST("/auth/password", deps.UserHandler.ChangePassword)

	// API Keys routes
	apiKeysGroup := protected.Group("/apikeys")
	deps.APIKeyHandler.RegisterRoutes(apiKeysGroup)

	// Set prompt guard on chat handler
	promptGuard := promptguard.NewDetector(promptguard.DefaultDetectorConfig())
	deps.ChatHandler.SetPromptGuard(promptGuard)

	// Register chat routes
	deps.ChatHandler.RegisterRoutes(v1)

	// Register auto-reply routes
	if deps.AutoreplyHandler != nil {
		deps.AutoreplyHandler.RegisterRoutes(v1)
	}

	// Network routes
	networkHandler := networkapi.NewNetworkHandler(cfg.Port)
	networkHandler.RegisterRoutes(e)

	// Link preview routes
	linkPreviewHandler := networkapi.NewLinkPreviewHandler()
	linkPreviewHandler.RegisterRoutes(v1)

	// Metrics routes
	if deps.MetricsCollector != nil {
		metricsHandler := server.NewMetricsHandler(deps.MetricsCollector)
		metricsHandler.RegisterRoutes(v1)
	}
	if deps.MetricsWriter != nil {
		detailedMetricsHandler := metrics.NewHandler(deps.MetricsWriter)
		metricsGroup := v1.Group("/metrics")
		detailedMetricsHandler.RegisterRoutes(metricsGroup)

		// Set metrics recorder on chat handler
		deps.ChatHandler.SetMetricsRecorder(deps.MetricsWriter)

		// Set cache provider on metrics handler for cache stats
		if deps.SharedCache != nil {
			detailedMetricsHandler.SetCacheProvider(deps.SharedCache)
		}
	}

	// System routes
	systemHandler := server.NewSystemHandler(cfg.Version, cfg.BuildTime, cfg.GitCommit, cfg.DataDir)
	systemHandler.RegisterRoutes(v1)

	serviceHandler := server.NewServiceHandler()
	serviceHandler.RegisterRoutes(v1)

	// Connection monitoring routes
	connHandler := connection.NewHandler(connManager)
	connGroup := protected.Group("/connections")
	connHandler.RegisterRoutes(connGroup)

	// API protected routes
	apiProtected := api.Group("")
	apiProtected.Use(deps.AuthMiddleware.Authenticate())

	// Skill routes
	skillHandler := server.NewSkillHandler(s.SkillRegistry)

	// Initialize skill store for database persistence
	skillStoreDb, err := skillstore.NewStore(deps.DB)
	if err != nil {
		logger.Warn("Failed to initialize skill store", zap.Error(err))
	} else {
		skillHandler.SetStore(skillStoreDb)
		logger.Info("Skill store initialized")

		// Load installed skills from database and register them
		installedSkills, err := skillStoreDb.GetInstalledSkills(context.Background())
		if err != nil {
			logger.Warn("Failed to load installed skills from database", zap.Error(err))
		} else {
			loadedCount := 0
			for _, sk := range installedSkills {
				manifest := &skill.Manifest{
					ID:          sk.ID,
					Name:        sk.Name,
					Version:     sk.Version,
					Description: sk.Summary,
					Author:      sk.Author,
					Category:    sk.Category,
					Tags:        strings.Split(sk.Tags, ","),
					Metadata: map[string]string{
						"source_id":   sk.SourceID,
						"source_name": sk.SourceName,
						"homepage":    sk.Homepage,
					},
				}
				adapter := server.NewRemoteSkillAdapter(manifest)
				if err := s.SkillRegistry.Register(adapter, false); err != nil {
					logger.Warn("Failed to register installed skill", zap.String("skill_id", sk.ID), zap.Error(err))
				} else {
					loadedCount++
				}
			}
			logger.Info("Installed skills loaded from database", zap.Int("count", loadedCount))
		}

		// Initialize sync service for periodic skill updates
		syncConfig := skillstore.DefaultSyncServiceConfig()
		syncService := skillstore.NewSyncService(skillStoreDb, syncConfig, slog.Default())
		skillHandler.SetSyncService(syncService)
		syncService.Start(deps.Ctx)
		logger.Info("Skill sync service started")
	}

	// Initialize featured skills loader
	featuredDataPath := filepath.Join(cfg.DataDir, "featured_skills.json")
	featuredLoader := skillstore.NewFeaturedSkillsLoader(featuredDataPath)
	if err := featuredLoader.Load(); err != nil {
		logger.Warn("Failed to load featured skills", zap.Error(err))
	} else {
		skillHandler.SetFeaturedLoader(featuredLoader)
		logger.Info("Featured skills loaded", zap.Int("count", len(featuredLoader.GetAll())))
	}

	// Initialize local skill scanner
	localScanner := skillstore.NewLocalSkillScanner("")
	skillHandler.SetLocalScanner(localScanner)
	logger.Info("Local skill scanner initialized")

	skillHandler.RegisterRoutes(v1)

	// Plugin routes
	pluginHandler := server.NewPluginHandler(deps.PluginRegistry)
	pluginHandler.RegisterRoutes(v1)

	pluginStoreHandler := server.NewPluginStoreHandler(deps.PluginStore)
	pluginStoreHandler.RegisterRoutes(v1)

	// Tool store routes
	toolStoreHandler := server.NewToolStoreHandler(s.ToolRegistry)
	toolStoreHandler.RegisterRoutes(v1)

	// Backup routes
	if deps.BackupHandler != nil {
		deps.BackupHandler.RegisterRoutes(v1)
	}

	// Security routes (protected)
	if deps.SecurityHandler != nil {
		securityGroup := protected.Group("/security")
		deps.SecurityHandler.RegisterRoutes(securityGroup)
	}

	// Sandbox routes (protected)
	if deps.SandboxHandler != nil {
		sandboxGroup := protected.Group("/sandbox")
		deps.SandboxHandler.RegisterRoutes(sandboxGroup)
	}

	// Cron routes (protected) - /api/cron/*
	if deps.CronHandler != nil {
		deps.CronHandler.RegisterRoutes(apiProtected)
	}

	// Home Assistant routes (protected) - /api/homeassistant/*
	if deps.HAHandler != nil {
		haGroup := apiProtected.Group("/homeassistant")
		deps.HAHandler.RegisterRoutes(haGroup)
	}

	// Browser automation routes (protected) - /api/browser/*
	if deps.BrowserHandler != nil {
		browserGroup := apiProtected.Group("/browser")
		deps.BrowserHandler.RegisterRoutes(browserGroup)
	}

	// Workflow routes
	if deps.WorkflowHandler != nil {
		deps.WorkflowHandler.RegisterRoutes(e)
	}

	// Voice routes - /api/v1/voice/*
	if deps.VoiceHandler != nil {
		voiceGroup := v1.Group("/voice")
		deps.VoiceHandler.RegisterRoutes(voiceGroup)
		// WebSocket handler for voice streaming
		if deps.VoiceWSHandler != nil {
			deps.VoiceWSHandler.RegisterRoutes(voiceGroup)
		}
	}

	// Speech routes - /api/v1/speech/*
	if deps.SpeechHandler != nil {
		speechGroup := v1.Group("/speech")
		deps.SpeechHandler.RegisterRoutes(speechGroup)
	}

	// Form filler routes (protected) - /api/v1/formfiller/*
	if deps.FormfillerHandler != nil {
		formfillerGroup := protected.Group("/formfiller")
		deps.FormfillerHandler.RegisterRoutes(formfillerGroup)
	}

	// Companion routes
	if deps.CompanionHandler != nil {
		deps.CompanionHandler.RegisterRoutes(e)
	}
	if deps.CompanionWSHandler != nil {
		deps.CompanionWSHandler.RegisterRoutes(e)
	}

	// Provider pool routes (protected)
	if deps.ProviderPool != nil {
		providerPoolHandler := providerpool.NewHandler(deps.ProviderPool)
		providersGroup := protected.Group("/providers")
		providerPoolHandler.RegisterRoutes(providersGroup)
		modelsGroup := protected.Group("/models")
		providerPoolHandler.RegisterModelRoutes(modelsGroup)
		ideGroup := protected.Group("/ide")
		providerPoolHandler.RegisterIDERoutes(ideGroup)
		pricingGroup := protected.Group("/pricing")
		providerPoolHandler.RegisterPricingRoutes(pricingGroup)
		configGroup := protected.Group("/config")
		providerPoolHandler.RegisterConfigRoutes(configGroup)

		// Proxy failover routes
		failoverConfig := proxy.DefaultProxyConfig().Routing.Failover
		failoverHandler := proxy.NewFailoverAPIHandler(nil, &failoverConfig)
		failoverGroup := protected.Group("/proxy/failover")
		failoverHandler.RegisterRoutes(failoverGroup)
	}

	// OpenAI-compatible proxy routes on /v1/*
	if deps.Config.Proxy != nil && deps.Config.Proxy.Enabled {
		routingConfig := &deps.Config.Proxy.Routing
		if deps.Config.Proxy.Route != nil {
			routingConfig = deps.Config.Proxy.Route
		}
		proxyRouter := proxy.NewRouter(routingConfig)
		proxyConnPool := proxy.NewConnectionPool(&deps.Config.Proxy.Connection)
		proxyFailover := proxy.NewFailoverHandler(&routingConfig.Failover, proxyRouter)
		proxyHandler := proxy.NewProxyHandler(proxyRouter, proxyConnPool, proxyFailover)
		if deps.ProviderPool != nil {
			proxyHandler.SetProviderPool(deps.ProviderPool)
		}
		if deps.APIKeyService != nil {
			proxyHandler.SetAPIKeyValidator(func(key string) ([]string, error) {
				info, err := deps.APIKeyService.ValidateKey(context.Background(), key)
				if err != nil {
					return nil, err
				}
				return info.Scopes, nil
			})
		}
		v1ProxyGroup := e.Group("/v1")
		v1ProxyGroup.Any("/chat/completions", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/completions", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/embeddings", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/models", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/messages", echo.WrapHandler(proxyHandler))
	}

	// Ngrok remote access routes
	if deps.NgrokTunnelMgr != nil && deps.NgrokConfigStore != nil {
		remoteAccessHandler := networkapi.NewSDKRemoteAccessHandler(deps.NgrokTunnelMgr, deps.NgrokConfigStore, cfg.Port)
		remoteAccessHandler.RegisterRoutes(e)
		tunnelHandler := networkapi.NewTunnelHandler(deps.NgrokConfigStore, cfg.Port)
		tunnelHandler.RegisterRoutes(e)
	}

	// Claude Code CLI routes (protected)
	if deps.ClaudeCodeHandler != nil {
		claudeCodeGroup := protected.Group("/claudecode")
		deps.ClaudeCodeHandler.RegisterRoutes(claudeCodeGroup)
		deps.ChatHandler.SetClaudeCodeHandler(deps.ClaudeCodeHandler)
	}

	// Memory routes
	if deps.MemoryHandler != nil {
		deps.MemoryHandler.RegisterRoutes(v1)
	}

	// OTA Update routes (always register, handler checks if enabled)
	updateCfg := &update.Config{
		Enabled:        deps.Config.Update.Enabled,
		CheckInterval:  deps.Config.Update.CheckInterval,
		AutoDownload:   deps.Config.Update.AutoDownload,
		AutoApply:      deps.Config.Update.AutoApply,
		ReleaseChannel: deps.Config.Update.ReleaseChannel,
		BackupCount:    deps.Config.Update.BackupCount,
		StoragePath:    deps.Config.Update.StoragePath,
	}
	updateHandler := update.NewHandler(cfg.Version, updateCfg)
	updateHandler.RegisterRoutes(v1)
	logger.Info("OTA update routes registered", zap.Bool("enabled", deps.Config.Update.Enabled))

	// Channel config routes
	if deps.ChannelConfigStore != nil {
		channelConfigHandler := server.NewChannelConfigHandler(deps.ChannelConfigStore)
		channelConfigHandler.RegisterRoutes(api)
	}

	// Set shared cache on chat handler
	if deps.SharedCache != nil {
		deps.ChatHandler.SetCache(deps.SharedCache)
	}

	// Provider settings routes (protected)
	providerSettingsHandler := server.NewProviderSettingsHandler(deps.ChatHandler.GetProviderRegistry(), cfg.DataDir)
	providerSettingsGroup := protected.Group("/providers/settings")
	providerSettingsHandler.RegisterRoutes(providerSettingsGroup)

	// Static routes (must be last)
	web.RegisterStaticRoutes(e)

	logger.Info("All routes registered")
}

// InitMetrics initializes metrics services
func InitMetrics(dataDir string) (*metrics.Collector, *metrics.MetricsWriter) {
	metricsCollector := metrics.NewCollector(5*time.Second, 120)
	metricsCollector.Start()

	metricsConfig := metrics.DefaultWriterConfig()
	metricsConfig.SQLiteDBPath = filepath.Join(dataDir, "metrics.db")
	metricsWriter := metrics.NewMetricsWriter(nil, metricsConfig)
	metricsWriter.Start()

	return metricsCollector, metricsWriter
}
