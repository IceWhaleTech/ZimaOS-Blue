// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/personality/controller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/personality/model"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/personality/view"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/preview"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

// routesStartTime records when the server started, used for uptime calculation
var routesStartTime = time.Now()

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
	HotReloader        *config.HotReloader
	HeartbeatHandler   *heartbeat.Handler
}

// RegisterAllRoutes registers all API routes on the Echo instance
func RegisterAllRoutes(e *echo.Echo, deps *RoutesDeps) {
	s := deps.Services
	cfg := deps.ServerConfig
	logger := deps.Logger
	dataDir := cfg.DataDir

	// Initialize TLS manager with correct data directory
	certsDir := filepath.Join(dataDir, "certs")
	acmeDir := deps.Config.Server.TLS.ACMEDir
	if acmeDir == "" {
		acmeDir = filepath.Join(certsDir, "acme")
	}
	security.SetGlobalTLSManagerConfig(&security.TLSManagerConfig{
		CertFile:     filepath.Join(certsDir, "server.crt"),
		KeyFile:      filepath.Join(certsDir, "server.key"),
		ACMEDir:      acmeDir,
		SelfSigned:   deps.Config.Server.TLS.SelfSigned,
		ACMEEmail:    deps.Config.Server.TLS.ACMEEmail,
		ACMEDomains:  strings.Split(deps.Config.Server.TLS.ACMEDomains, ","),
		ACMEProvider: deps.Config.Server.TLS.ACMEProvider,
		AutoCert:     deps.Config.Server.TLS.AutoCert,
		HTTPSOnly:    deps.Config.Server.TLS.Enabled,
		HTTPSPort:    deps.Config.Server.TLS.Port,
	})

	// HTTPS redirect middleware (must be first)
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager != nil {
		// Try to load existing certificate from disk
		if err := tlsManager.LoadCertificate(); err != nil {
			logger.Debug("No existing TLS certificate found", zap.Error(err))
		} else {
			logger.Info("TLS certificate loaded from disk")
		}

		// Start ACME auto-renewal if configured
		if deps.Config.Server.TLS.AutoCert && deps.Config.Server.TLS.ACMEEmail != "" && deps.Config.Server.TLS.ACMEDomains != "" {
			domains := strings.Split(deps.Config.Server.TLS.ACMEDomains, ",")
			for i := range domains {
				domains[i] = strings.TrimSpace(domains[i])
			}
			if err := tlsManager.RequestACMECertificate(&security.ACMEConfig{
				Email:    deps.Config.Server.TLS.ACMEEmail,
				Domains:  domains,
				Provider: deps.Config.Server.TLS.ACMEProvider,
				CacheDir: acmeDir,
			}); err != nil {
				logger.Warn("Failed to initialize ACME auto-renewal", zap.Error(err))
			} else {
				logger.Info("ACME auto-renewal initialized", zap.Strings("domains", domains))
			}
		}

		e.Use(tlsManager.HTTPSRedirectMiddleware())
	}

	// Initialize connection manager
	connManager := connection.NewManager(10000, 5*time.Second)
	e.Use(connManager.Middleware())

	// Preview mode routes (no auth required)
	previewModeService := preview.NewModeService(s.UserService)
	previewModeService.SetDataDir(dataDir) // Set data dir for logging
	previewUpgradeService := preview.NewUpgradeService(s.UserService, s.DB)
	previewHandler := preview.NewHandler(previewModeService, previewUpgradeService, s.JWTService)
	previewHandler.SetDataDir(dataDir)
	previewHandler.RegisterRoutes(e)
	logger.Info("Preview mode routes registered")

	// Set mode service to user handler for preview mode support
	deps.UserHandler.SetModeService(previewModeService)

	// API groups
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	// Health endpoint (with full runtime stats)
	v1.GET("/health", func(c echo.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		uptime := time.Since(routesStartTime).Truncate(time.Second)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":          "ok",
			"service":         "zimaos-blue",
			"timestamp":       time.Now(),
			"uptime":          uptime.String(),
			"uptime_seconds":  uptime.Seconds(),
			"version":         cfg.Version,
			"go_version":      runtime.Version(),
			"num_cpu":         runtime.NumCPU(),
			"goroutines":      runtime.NumGoroutine(),
			"mem_alloc_bytes": m.Alloc,
		})
	})

	// Worker stats endpoint (public, for bootstrap/health checks)
	v1.GET("/workers/stats", func(c echo.Context) error {
		if s.WorkerPool == nil {
			return c.JSON(http.StatusOK, worker.Stats{})
		}
		return c.JSON(http.StatusOK, s.WorkerPool.Stats())
	})

	// Public auth routes
	v1.POST("/auth/login", deps.UserHandler.Login)
	v1.POST("/auth/logout", deps.UserHandler.Logout)

	// Public config and templates routes (no auth required)
	configHandler := server.NewConfigHandler(deps.HotReloader)
	configHandler.RegisterRoutes(v1)
	templatesHandler := server.NewTemplatesHandler()
	templatesHandler.RegisterRoutes(v1)

	// Public /me endpoint for preview mode
	v1.GET("/users/me", deps.UserHandler.GetCurrentUser)
	v1.PUT("/users/me", deps.UserHandler.UpdateCurrentUser)

	// Public formfiller routes for preview mode
	if deps.FormfillerHandler != nil {
		formfillerGroup := v1.Group("/formfiller")
		deps.FormfillerHandler.RegisterRoutes(formfillerGroup)
	}

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

	// Form filler routes are now public (registered above in v1)

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

		// Proxy cache routes
		if deps.SharedCache != nil {
			cacheConfig := proxy.DefaultCacheConfig()
			cacheHandler := proxy.NewCacheAPIHandler(deps.SharedCache, cacheConfig)
			cacheGroup := v1.Group("/proxy/cache")
			cacheHandler.RegisterRoutes(cacheGroup)
		}
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

		// Context pruner middleware (optional, disabled by default)
		if deps.Config.Pruner != nil && deps.Config.Pruner.Enabled {
			prunerCfg := *deps.Config.Pruner
			backend, err := pruner.NewBackend(prunerCfg)
			if err != nil {
				slog.Warn("Failed to create pruner backend", "error", err)
			} else {
				prunerStats := pruner.NewStats()
				prunerMw := pruner.NewMiddleware(backend, prunerCfg, prunerStats)
				proxyHandler.SetPruner(prunerMw)
				slog.Info("Context pruner enabled", "backend", prunerCfg.Backend, "threshold", prunerCfg.Threshold)

				// Register pruner API routes
				prunerHandler := pruner.NewAPIHandler(prunerMw, &prunerCfg)
				prunerGroup := v1.Group("/proxy/pruner")
				prunerHandler.RegisterRoutes(prunerGroup)
			}
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

	// Memory Service v2 routes (versioned entries, namespaces)
	{
		memDBPath := filepath.Join(dataDir, "memory_service.db")
		memDB, err := sql.Open("sqlite", memDBPath)
		if err != nil {
			logger.Error("Failed to open memory service database", zap.Error(err))
		} else {
			memRepo, err := memory.NewMemoryRepository(memDB)
			if err != nil {
				logger.Error("Failed to initialize memory service repository", zap.Error(err))
			} else {
				memNS, err := memory.NewNamespaceStore(memDB)
				if err != nil {
					logger.Error("Failed to initialize memory namespace store", zap.Error(err))
				} else {
					memAPIHandler := memory.NewAPIHandler(memRepo, memNS)
					v2 := api.Group("/v2")
					memAPIHandler.RegisterRoutes(v2)
					logger.Info("Memory Service v2 routes registered")

					// Initialize content encryption if configured
					var memEncryptor *memory.ContentEncryptor
					if deps.Config != nil {
						encCfg := deps.Config.Security.Encryption
						memEncryptor, err = memory.NewContentEncryptor(
							encCfg.Passphrase, encCfg.KeyPath, encCfg.Enabled,
						)
						if err != nil {
							logger.Warn("Failed to initialize memory encryption", zap.Error(err))
						} else {
							memRepo.SetEncryptor(memEncryptor)
							if encCfg.Enabled {
								logger.Info("Memory encryption enabled (AES-256-GCM)")
							}
						}
					}
					if memEncryptor == nil {
						memEncryptor, _ = memory.NewContentEncryptor("", "", false)
					}

					// Register encryption management routes
					encHandler := memory.NewEncryptionHandler(memRepo, memEncryptor)
					encHandler.RegisterRoutes(v2)

					// Start background purge scheduler
					purgeScheduler := memory.NewPurgeScheduler(memRepo, 6*time.Hour)
					purgeScheduler.Start(deps.Ctx)
					logger.Info("Memory Service v2 purge scheduler started")

					// Create v2 bridge for existing memory system
					bridge := memory.NewV2Bridge(memRepo)
					if deps.MemoryHandler != nil {
						deps.MemoryHandler.SetV2Bridge(bridge)
					}
				}
			}
		}
	}

	// Personality routes (protected)
	personalityStorage, err := model.NewFileStorage(cfg.DataDir)
	if err != nil {
		logger.Error("Failed to initialize personality storage", zap.Error(err))
	} else {
		// Initialize default personality from SOUL.md
		if err := model.InitializeDefaultPersonality(cfg.DataDir); err != nil {
			logger.Warn("Failed to initialize default personality", zap.Error(err))
		}
		personalityService := controller.NewService(personalityStorage)
		personalityHandler := view.NewHandler(personalityService)
		personalityGroup := protected.Group("/personalities")
		personalityHandler.RegisterRoutes(personalityGroup)
		logger.Info("Personality routes registered")
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

	// Heartbeat routes
	if deps.HeartbeatHandler != nil {
		deps.HeartbeatHandler.RegisterRoutes(api)
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
