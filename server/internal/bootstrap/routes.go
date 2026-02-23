// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// routesStartTime records when the server started, used for uptime calculation
var routesStartTime = timeutil.NowTime()

// featureDisabled returns an echo handler that responds with a standard
// "feature not enabled" JSON payload.  This is used as a catch-all for
// optional features whose handler was not initialised at startup so that
// the frontend never sees a raw 404.
func featureDisabled(feature string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": false,
			"feature": feature,
			"status":  "not_initialized",
			"message": feature + " is not enabled",
		})
	}
}

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
	HotReloader        *config.HotReloader
	WorkspaceHandler   *workspace.Handler
}

// RegisterAllRoutes registers all API routes on the Echo instance.
// Returns the authenticated API group for late-binding route registration.
func RegisterAllRoutes(e *echo.Echo, deps *RoutesDeps) *echo.Group {
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
		// Load persisted TLS settings (overrides YAML defaults)
		if err := tlsManager.LoadSettings(); err != nil {
			logger.Warn("Failed to load persisted TLS settings", zap.Error(err))
		}

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
	previewUpgradeService := preview.NewUpgradeService(s.UserService, s.DB)
	previewHandler := preview.NewHandler(previewModeService, previewUpgradeService, s.JWTService, s.UserService)
	previewHandler.SetDataDir(dataDir)
	previewHandler.RegisterRoutes(e)
	logger.Info("Preview mode routes registered")

	// Set mode service to user handler for preview mode support
	deps.UserHandler.SetModeService(previewModeService)

	// API groups
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	// Static media serving (screenshots, etc.) — no auth required
	mediaDir := filepath.Join(dataDir, "media")
	_ = os.MkdirAll(mediaDir, 0750)
	v1.Static("/media", mediaDir)

	// Media generation (image/video/audio) — reads from ProviderTypeMedia providers
	{
		mediaGenDir := filepath.Join(dataDir, "media", "generated")
		mediaStorage := mediagen.NewMediaStorage(mediaGenDir, "/api/media/generated")
		if err := mediaStorage.EnsureDirs(); err != nil {
			logger.Warn("Failed to create media generation dirs", zap.Error(err))
		}
		mediaManager := mediagen.NewManager(mediaStorage)

		// Register providers from provider pool (media-type providers)
		if deps.ProviderPool != nil {
			for _, p := range deps.ProviderPool.Registry.List() {
				if p.Type != providerpool.ProviderTypeMedia || !p.Enabled {
					continue
				}
				var apiKey string
				for _, k := range p.APIKeys {
					if k.Enabled && k.Key != "" {
						apiKey = k.Key
						break
					}
				}
				if apiKey == "" {
					continue
				}
				switch p.ID {
				case "gemini-image":
					mediaManager.RegisterProvider(mediagen.NewGeminiProvider(apiKey, p.BaseURL))
				case "dashscope-image":
					mediaManager.RegisterProvider(mediagen.NewDashScopeProvider(apiKey, p.BaseURL))
				case "mulerouter":
					mediaManager.RegisterProvider(mediagen.NewMuleRouterProvider(apiKey, p.BaseURL))
				}
				logger.Info("Registered media provider", zap.String("id", p.ID))
			}
		}

		// Register tools
		s.ToolRegistry.Register(mediagen.NewImageGenerateTool(mediaManager))
		s.ToolRegistry.Register(mediagen.NewVideoGenerateTool(mediaManager))

		// Register HTTP routes
		mediaHandler := mediagen.NewHandler(mediaManager, mediaStorage)
		mediaGroup := v1.Group("/media")
		mediaHandler.RegisterRoutes(mediaGroup)
		mediaHandler.RegisterStorageRoutes(e)
		logger.Info("Media generation routes registered")
	}

	// Health endpoint (with full runtime stats)
	v1.GET("/health", func(c echo.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		uptime := timeutil.SinceTime(routesStartTime).Truncate(time.Second)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":          "ok",
			"service":         "zimaos-blue",
			"timestamp":       timeutil.NowTime(),
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
	} else {
		stub := featureDisabled("formfiller")
		formfillerGroup := v1.Group("/formfiller")
		formfillerGroup.GET("/templates", stub)
		formfillerGroup.GET("/config", stub)
		formfillerGroup.Any("/*", stub)
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
	// Add LAN addresses to CORS allowed origins after actual port is known
	server.OnServerStart(func(port int) {
		h := networkapi.NewNetworkHandler(port)
		if err := h.InitializeCORSOrigins(); err != nil {
			logger.Warn("Failed to initialize CORS origins from network addresses", zap.Error(err))
		}
	})

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
	}
	if deps.MetricsCollector == nil && deps.MetricsWriter == nil {
		stub := featureDisabled("metrics")
		metricsGroup := v1.Group("/metrics")
		metricsGroup.GET("/summary", stub)
		metricsGroup.GET("/all", stub)
		metricsGroup.Any("/*", stub)
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
	} else {
		stub := featureDisabled("backup")
		backupGroup := v1.Group("/backup")
		backupGroup.GET("", stub)
		backupGroup.GET("/progress", stub)
		backupGroup.Any("/*", stub)
	}

	// Security routes (protected)
	if deps.SecurityHandler != nil {
		securityGroup := protected.Group("/security")
		deps.SecurityHandler.RegisterRoutes(securityGroup)
	} else {
		stub := featureDisabled("security")
		securityGroup := protected.Group("/security")
		securityGroup.GET("/sessions", stub)
		securityGroup.GET("/settings", stub)
		securityGroup.GET("/events", stub)
		securityGroup.GET("/stats", stub)
		securityGroup.GET("/blocked-ips", stub)
		securityGroup.GET("/threats/stats", stub)
		securityGroup.GET("/threats", stub)
		securityGroup.GET("/scan", stub)
		securityGroup.GET("/cors", stub)
		securityGroup.GET("/tls", stub)
		securityGroup.Any("/*", stub)
	}

	// Sandbox routes (protected)
	if deps.SandboxHandler != nil {
		sandboxGroup := protected.Group("/sandbox")
		deps.SandboxHandler.RegisterRoutes(sandboxGroup)
	} else {
		stub := featureDisabled("sandbox")
		sandboxGroup := protected.Group("/sandbox")
		sandboxGroup.GET("/info", stub)
		sandboxGroup.Any("/*", stub)
	}

	// Cron routes (protected) - /api/cron/*
	if deps.CronHandler != nil {
		deps.CronHandler.RegisterRoutes(apiProtected)
	} else {
		stub := featureDisabled("cron")
		cronGroup := apiProtected.Group("/cron")
		cronGroup.GET("", stub)
		cronGroup.Any("/*", stub)
	}

	// Heartbeat routes (protected) - /api/heartbeat/*
	{
		hbCfg := &heartbeat.Config{
			Enabled:      deps.Config.Heartbeat.Enabled,
			Interval:     deps.Config.Heartbeat.Interval,
			Prompt:       deps.Config.Heartbeat.Prompt,
			AckMaxChars:  deps.Config.Heartbeat.AckMaxChars,
			WorkspaceDir: deps.Config.Heartbeat.WorkspaceDir,
			LLMProvider:  deps.Config.Heartbeat.LLMProvider,
			LLMModel:     deps.Config.Heartbeat.LLMModel,
			Visibility: heartbeat.VisibilityConfig{
				ShowOk:       deps.Config.Heartbeat.Visibility.ShowOk,
				ShowAlerts:   deps.Config.Heartbeat.Visibility.ShowAlerts,
				UseIndicator: deps.Config.Heartbeat.Visibility.UseIndicator,
			},
		}
		if hbCfg.Interval == 0 {
			hbCfg.Interval = heartbeat.DefaultInterval
		}
		if hbCfg.WorkspaceDir == "" {
			hbCfg.WorkspaceDir = dataDir
		}
		hbRunner := heartbeat.NewRunner(heartbeat.RunnerDeps{
			Config: hbCfg,
			ChatFn: func() heartbeat.ChatFunc {
				pc := server.NewProxyClient(cfg.Port)
				if deps.APIKeyService != nil {
					if info, err := deps.APIKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
						Name:   "heartbeat-internal",
						Scopes: []string{"chat", "proxy", "route:auto"},
					}); err == nil {
						pc.SetAPIKey(info.Key)
					}
				}
				return pc.Chat
			}(),
			Logger: logger,
		})
		go hbRunner.Run(deps.Ctx)
		hbHandler := heartbeat.NewHandler(hbRunner)
		hbHandler.RegisterRoutes(apiProtected)
		logger.Info("Heartbeat routes registered", zap.Bool("enabled", hbCfg.Enabled))
	}

	// Home Assistant routes (protected) - /api/homeassistant/*
	if deps.HAHandler != nil {
		haGroup := apiProtected.Group("/homeassistant")
		deps.HAHandler.RegisterRoutes(haGroup)
	} else {
		stub := featureDisabled("homeassistant")
		haGroup := apiProtected.Group("/homeassistant")
		haGroup.GET("/status", stub)
		haGroup.GET("/entities", stub)
		haGroup.GET("/scenes", stub)
		haGroup.GET("/automations", stub)
		haGroup.Any("/*", stub)
	}

	// Browser automation routes (protected) - /api/browser/*
	if deps.BrowserHandler != nil {
		browserGroup := apiProtected.Group("/browser")
		deps.BrowserHandler.RegisterRoutes(browserGroup)
	} else {
		stub := featureDisabled("browser")
		browserGroup := apiProtected.Group("/browser")
		browserGroup.GET("/tasks", stub)
		browserGroup.GET("/sessions", stub)
		browserGroup.GET("/security", stub)
		browserGroup.Any("/*", stub)
	}

	// Workflow routes
	if deps.WorkflowHandler != nil {
		deps.WorkflowHandler.RegisterRoutes(e)
	} else {
		stub := featureDisabled("workflow")
		wfGroup := v1.Group("/workflows")
		wfGroup.GET("", stub)
		wfGroup.GET("/stats", stub)
		wfGroup.GET("/templates", stub)
		wfGroup.Any("/*", stub)
	}

	// Voice routes - /api/v1/voice/*
	if deps.VoiceHandler != nil {
		voiceGroup := v1.Group("/voice")
		deps.VoiceHandler.RegisterRoutes(voiceGroup)
		// WebSocket handler for voice streaming (requires auth, supports token in query param)
		if deps.VoiceWSHandler != nil {
			voiceWSGroup := protected.Group("/voice")
			deps.VoiceWSHandler.RegisterRoutes(voiceWSGroup)
		}
	} else {
		stub := featureDisabled("voice")
		voiceGroup := v1.Group("/voice")
		voiceGroup.GET("/voices", stub)
		voiceGroup.GET("/sessions", stub)
		voiceGroup.POST("/transcribe", stub)
		voiceGroup.POST("/synthesize", stub)
		voiceGroup.Any("/*", stub)
	}

	// Speech routes - /api/v1/speech/*
	if deps.SpeechHandler != nil {
		speechGroup := v1.Group("/speech")
		deps.SpeechHandler.RegisterRoutes(speechGroup)
	} else {
		stub := featureDisabled("speech")
		speechGroup := v1.Group("/speech")
		speechGroup.GET("/status", stub)
		speechGroup.GET("/models", stub)
		speechGroup.GET("/asr/status", stub)
		speechGroup.GET("/asr/models", stub)
		speechGroup.GET("/tts/status", stub)
		speechGroup.GET("/tts/models", stub)
		speechGroup.Any("/*", stub)
	}

	// Form filler routes are now public (registered above in v1)

	// Companion routes
	if deps.CompanionHandler != nil {
		deps.CompanionHandler.RegisterRoutes(e)
	}
	if deps.CompanionWSHandler != nil {
		deps.CompanionWSHandler.RegisterRoutes(e)
	}
	if deps.CompanionHandler == nil {
		stub := featureDisabled("companion")
		v1.GET("/companion/stream", stub)
	}

	// Provider pool routes (protected)
	var oauthManager *oauth.Manager // hoisted for proxy handler wiring
	if deps.ProviderPool != nil {
		providerPoolHandler := providerpool.NewHandler(deps.ProviderPool)

		// Initialize OAuth manager for OAuth-based providers
		oauthDataDir := filepath.Join(dataDir, "providers")
		oauthStore, oauthErr := oauth.NewStore(oauthDataDir)
		if oauthErr != nil {
			logger.Warn("Failed to initialize OAuth store", zap.Error(oauthErr))
		} else {
			oauthManager = oauth.NewManager(oauthStore)
			providerPoolHandler.SetOAuthManager(oauthManager)
			providerPoolHandler.RegisterOAuthCallbackRoute(e)
			logger.Info("OAuth manager initialized for provider pool")
		}

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

		// Proxy failover routes are registered below in the proxy block
		// so they share the same FailoverConfig pointer as the actual failover handler.
	} else {
		stub := featureDisabled("providers")
		providersGroup := protected.Group("/providers")
		providersGroup.GET("", stub)
		providersGroup.Any("/*", stub)
		modelsGroup := protected.Group("/models")
		modelsGroup.GET("", stub)
		ideGroup := protected.Group("/ide")
		ideGroup.GET("/scan", stub)
		ideGroup.Any("/*", stub)
		pricingGroup := protected.Group("/pricing")
		pricingGroup.GET("", stub)
		pricingGroup.Any("/*", stub)
		failoverStub := featureDisabled("proxy_failover")
		failoverGroup := protected.Group("/proxy/failover")
		failoverGroup.GET("/config", failoverStub)
		failoverGroup.GET("/metrics", failoverStub)
		failoverGroup.GET("/breakers", failoverStub)
		failoverGroup.Any("/*", failoverStub)
	}

	// Proxy cache routes (deprecated — cache removed)
	{
		stub := featureDisabled("proxy_cache")
		cacheGroup := v1.Group("/proxy/cache")
		cacheGroup.GET("/stats", stub)
		cacheGroup.GET("/config", stub)
		cacheGroup.Any("/*", stub)
	}

	// Data masking (hoisted so toggle state is accessible from proxy block)
	dataMasker := proxy.NewDataMasker(nil)
	var maskingOnToggle func() // wired later when toggleStore is available
	{
		maskingGroup := v1.Group("/proxy/masking")
		maskingGroup.GET("/stats", func(c echo.Context) error {
			return c.JSON(200, dataMasker.Stats())
		})
		maskingGroup.GET("/rules", func(c echo.Context) error {
			return c.JSON(200, map[string]interface{}{
				"rules":         dataMasker.ListRules(),
				"default_rules": proxy.GetDefaultRules(),
			})
		})
		maskingGroup.POST("/rules", func(c echo.Context) error {
			var rule proxy.MaskingRule
			if err := c.Bind(&rule); err != nil {
				return c.JSON(400, map[string]string{"error": "invalid request"})
			}
			if err := dataMasker.AddRule(&rule); err != nil {
				return c.JSON(400, map[string]string{"error": err.Error()})
			}
			return c.JSON(201, map[string]interface{}{"message": "rule added", "rule": rule})
		})
		maskingGroup.DELETE("/rules", func(c echo.Context) error {
			id := c.QueryParam("id")
			if id == "" {
				return c.JSON(400, map[string]string{"error": "id required"})
			}
			if dataMasker.RemoveRule(id) {
				return c.JSON(200, map[string]string{"message": "rule removed"})
			}
			return c.JSON(404, map[string]string{"error": "rule not found"})
		})
		maskingGroup.PUT("/toggle", func(c echo.Context) error {
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := c.Bind(&req); err != nil || req.Enabled == nil {
				return c.JSON(400, map[string]string{"error": "enabled field required"})
			}
			dataMasker.SetEnabled(*req.Enabled)
			if maskingOnToggle != nil {
				maskingOnToggle()
			}
			return c.JSON(200, dataMasker.Stats())
		})
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

		// Smart failover handler for metrics + intelligent error classification
		smartFailover := proxy.NewSmartFailoverHandler(&routingConfig.Failover, proxyRouter)

		// Failover API routes — share the same config pointer so API changes take effect
		failoverAPIHandler := proxy.NewFailoverAPIHandler(smartFailover, &routingConfig.Failover)
		failoverGroup := protected.Group("/proxy/failover")
		failoverAPIHandler.RegisterRoutes(failoverGroup)

		// Pipeline stats collector: unified async batch persistence for routing/failover
		var pipelineStats *proxy.PipelineStatsCollector
		if deps.DB != nil {
			pipelineStats = proxy.NewPipelineStatsCollector(deps.DB, proxyHandler.GetRoutingStatsRef())
			pipelineStats.Start()
			proxyHandler.SetPipelineStats(pipelineStats)
		}

		if deps.ProviderPool != nil {
			proxyHandler.SetProviderPool(deps.ProviderPool)

			// Wire OAuth manager into proxy handler for OAuth-based provider auth
			if oauthManager != nil {
				proxyHandler.SetOAuthManager(oauthManager)
			}

			// Wire failover callback for pipeline stats collection
			if deps.ProviderPool.Router != nil && pipelineStats != nil {
				deps.ProviderPool.Router.SetFailoverCallback(pipelineStats.OnFailover)
			}

			// Wire health check latency into router for latency-based routing
			if deps.ProviderPool.Registry != nil && deps.ProviderPool.Router != nil {
				deps.ProviderPool.Registry.SetOnHealthResult(func(providerID string, result *providerpool.HealthCheckResult) {
					if result.Healthy && result.Latency > 0 {
						deps.ProviderPool.Router.UpdateLatency(providerID, result.Latency)
					}
				})
			}

			// Connection warmup: pre-establish TCP+TLS to all providers (async)
			go func() {
				providers := deps.ProviderPool.Registry.ListEnabled()
				urls := make([]string, 0, len(providers))
				for _, p := range providers {
					if p.BaseURL != "" {
						urls = append(urls, p.BaseURL)
					}
				}
				warmup := proxy.NewConnWarmup(proxyConnPool)
				warmup.WarmProviders(urls)
			}()
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
		var prunerMw *pruner.Middleware
		prunerCfg := pruner.Config{Enabled: false}
		if deps.Config.Pruner != nil {
			prunerCfg = *deps.Config.Pruner
		}
		if prunerCfg.Enabled {
			backend, err := pruner.NewBackend(prunerCfg)
			if err != nil {
				slog.Warn("Failed to create pruner backend", "error", err)
			} else {
				prunerStats := pruner.NewStats()
				prunerMw = pruner.NewMiddleware(backend, prunerCfg, prunerStats)
				proxyHandler.SetPruner(prunerMw)
				slog.Info("Context pruner enabled", "backend", prunerCfg.Backend, "threshold", prunerCfg.Threshold)
			}
		}
		// Pruner model manager (always available for model download)
		prunerModelDir := filepath.Join(cfg.DataDir, "pruner-models")
		prunerCfg.ModelDir = prunerModelDir // Set ModelDir for ONNX backend
		prunerModelMgr := pruner.NewPrunerModelManager(prunerModelDir)

		// Always register pruner API routes (handler returns disabled status when pruner is off)
		prunerHandler := pruner.NewAPIHandler(prunerMw, &prunerCfg, prunerModelMgr)
		prunerGroup := v1.Group("/proxy/pruner")
		prunerHandler.RegisterRoutes(prunerGroup)

		// Dynamic tier resolver: classifies models by pricing for smart routing
		tierResolver := proxy.NewTierResolver()
		if deps.ProviderPool != nil && deps.ProviderPool.Router != nil {
			models := deps.ProviderPool.Router.ListAvailableModels()
			if tierResolver.Resolve(models) {
				slog.Info("Tier resolver initialized", "stats", tierResolver.Stats())
			}
			// Re-resolve tiers when providers change (async to avoid deadlock)
			if deps.ProviderPool.Registry != nil {
				router := deps.ProviderPool.Router
				deps.ProviderPool.Registry.AddProviderChangeListener(func(_ *providerpool.Provider, _ string) {
					go func() {
						tierResolver.Resolve(router.ListAvailableModels())
					}()
				})
			}
		}

		// Model router: family-based routing + background task downgrade
		modelRouterCfg := deps.Config.Proxy.ModelRouter
		if modelRouterCfg == nil {
			modelRouterCfg = proxy.DefaultModelRouterConfig()
		}
		mr, err := proxy.NewModelRouter(modelRouterCfg)
		if err != nil {
			slog.Warn("Failed to create model router", "error", err)
		} else {
			proxyHandler.SetModelRouter(mr)
			slog.Info("Model router loaded", "families", len(modelRouterCfg.Families), "rules", len(modelRouterCfg.RegexCustomRules), "enabled", modelRouterCfg.Enabled)
		}

		// Condition-based rule routing (tier-based rules)
		ruleRoutingCfg := deps.Config.Proxy.RuleRouting
		if ruleRoutingCfg == nil {
			ruleRoutingCfg = proxy.DefaultRoutingConfig()
		}
		if len(ruleRoutingCfg.Rules) > 0 {
			proxyHandler.SetRuleEngine(ruleRoutingCfg.ToRuleEngine(tierResolver))
			slog.Info("Rule engine loaded", "rules", len(ruleRoutingCfg.Rules), "enabled", ruleRoutingCfg.Enabled)
		}
		proxyHandler.SetTierResolver(tierResolver)
		proxyHandler.SetRoutingEnabled(ruleRoutingCfg.Enabled)

		// Toggle persistence: use main blue.db for kvstore (merged from former settings.db)
		toggleKV, kvErr := kvstore.NewSQLiteStoreWithDB(deps.DB)
		if kvErr == nil {
			// Migrate data from legacy settings.db if it exists
			migrateSettingsDB(filepath.Join(cfg.DataDir, "settings.db"), toggleKV)
		}
		var toggleStore *proxy.ToggleStore
		getToggleState := func() *proxy.ToggleState {
			state := &proxy.ToggleState{
				PrunerEnabled:      prunerMw != nil && prunerMw.Enabled(),
				PrunerBackend:      prunerCfg.Backend,
				RoutingEnabled:     proxyHandler.IsRoutingEnabled(),
				MaskingEnabled:     dataMasker.IsEnabled(),
				PromptCacheEnabled: proxyHandler.IsPromptCacheEnabled(),
			}
			// Capture individual routing rule states
			if rules := proxyHandler.GetRoutingRules(); len(rules) > 0 {
				state.RoutingRules = make(map[string]bool, len(rules))
				for _, r := range rules {
					state.RoutingRules[r.Name] = r.Enabled != nil && *r.Enabled
				}
			}
			return state
		}
		if kvErr != nil {
			slog.Warn("Failed to create toggle kvstore", "error", kvErr)
		} else {
			toggleStore = proxy.NewToggleStore(toggleKV)
			if saved, loadErr := toggleStore.Load(context.Background()); loadErr == nil {
				if prunerMw != nil {
					prunerMw.SetEnabled(saved.PrunerEnabled)
				}
				if saved.PrunerBackend != "" && saved.PrunerBackend != prunerCfg.Backend {
					prunerCfg.Backend = saved.PrunerBackend
					if b, err := pruner.NewBackend(prunerCfg); err == nil {
						if prunerMw != nil {
							prunerMw.SetBackend(b)
						}
					}
				}
				proxyHandler.SetRoutingEnabled(saved.RoutingEnabled)
				proxyHandler.SetPromptCacheEnabled(saved.PromptCacheEnabled)
				dataMasker.SetEnabled(saved.MaskingEnabled)
				// Restore individual routing rule states
				for name, enabled := range saved.RoutingRules {
					proxyHandler.SetRoutingRuleEnabled(name, enabled)
				}
				slog.Info("Restored feature toggles",
					"pruner", saved.PrunerEnabled,
					"pruner_backend", saved.PrunerBackend,
					"routing", saved.RoutingEnabled,
					"masking", saved.MaskingEnabled,
					"prompt_cache", saved.PromptCacheEnabled,
				)
			}
			saveToggle := func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				toggleStore.Save(ctx, getToggleState())
			}
			prunerHandler.SetOnToggle(func(enabled bool) { saveToggle() })
			prunerHandler.SetOnBackendChange(func(backend string) { saveToggle() })
			maskingOnToggle = saveToggle
		}

		// ProxyBridge: route ChatHandler LLM calls through proxy pipeline
		bridge := proxybridge.NewBridge(proxyHandler)
		deps.ChatHandler.SetProxyBridge(bridge)
		deps.ChatHandler.SetIMModel("auto") // proxy auto-selects model

		// Wire VLM bridge into native UI reviewer tool
		if uiTool := tools.GetUIReviewerTool(s.ToolRegistry); uiTool != nil {
			uiTool.SetVLMBridge(tools.NewProxyBridgeVLMAdapter(bridge))
		}

		v1ProxyGroup := e.Group("/v1")
		v1ProxyGroup.Any("/chat/completions", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/completions", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/embeddings", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/models", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/messages", echo.WrapHandler(proxyHandler))

		// Model routing toggle API
		routingGroup := v1.Group("/proxy/routing")
		routingGroup.GET("/config", func(c echo.Context) error {
			return c.JSON(200, map[string]interface{}{
				"enabled": proxyHandler.IsRoutingEnabled(),
			})
		})
		routingGroup.PUT("/config", func(c echo.Context) error {
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(400, map[string]string{"error": "invalid request"})
			}
			if req.Enabled != nil {
				proxyHandler.SetRoutingEnabled(*req.Enabled)
				if toggleStore != nil {
					ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
					defer cancel()
					toggleStore.Save(ctx, getToggleState())
				}
			}
			return c.JSON(200, map[string]interface{}{
				"success": true,
				"enabled": proxyHandler.IsRoutingEnabled(),
			})
		})
		routingGroup.GET("/rules", func(c echo.Context) error {
			rules := proxyHandler.GetRoutingRules()
			if rules == nil {
				rules = []proxy.RoutingRule{}
			}
			return c.JSON(200, map[string]interface{}{
				"rules": rules,
			})
		})
		routingGroup.PUT("/rules/:name", func(c echo.Context) error {
			name := c.Param("name")
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(400, map[string]string{"error": "invalid request"})
			}
			if req.Enabled == nil {
				return c.JSON(400, map[string]string{"error": "enabled field required"})
			}
			if !proxyHandler.SetRoutingRuleEnabled(name, *req.Enabled) {
				return c.JSON(404, map[string]string{"error": "rule not found"})
			}
			if toggleStore != nil {
				ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
				defer cancel()
				toggleStore.Save(ctx, getToggleState())
			}
			return c.JSON(200, map[string]interface{}{
				"success": true,
				"name":    name,
				"enabled": *req.Enabled,
			})
		})
		routingGroup.GET("/stats", func(c echo.Context) error {
			return c.JSON(200, proxyHandler.GetRoutingStats())
		})

		// Pipeline stats: unified cache + routing + failover stats
		v1.GET("/proxy/pipeline/stats", func(c echo.Context) error {
			if pipelineStats != nil {
				return c.JSON(200, pipelineStats.Snapshot())
			}
			return c.JSON(200, map[string]string{"status": "not configured"})
		})

		// Prompt cache toggle API
		promptCacheGroup := v1.Group("/proxy/prompt-cache")
		promptCacheGroup.GET("/config", func(c echo.Context) error {
			return c.JSON(200, map[string]interface{}{
				"enabled": proxyHandler.IsPromptCacheEnabled(),
			})
		})
		promptCacheGroup.PUT("/config", func(c echo.Context) error {
			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := c.Bind(&req); err != nil {
				return c.JSON(400, map[string]string{"error": "invalid request"})
			}
			if req.Enabled != nil {
				proxyHandler.SetPromptCacheEnabled(*req.Enabled)
				if toggleStore != nil {
					ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
					defer cancel()
					toggleStore.Save(ctx, getToggleState())
				}
			}
			return c.JSON(200, map[string]interface{}{
				"success": true,
				"enabled": proxyHandler.IsPromptCacheEnabled(),
			})
		})

		// Provider restriction management (blacklist/throttle clearing)
		restrictionsHandler := proxy.NewRestrictionsHandler(proxyHandler.GetProviderMemory())
		restrictionsHandler.RegisterRoutes(v1)
	}

	// Ngrok remote access routes
	if deps.NgrokTunnelMgr != nil && deps.NgrokConfigStore != nil {
		remoteAccessHandler := networkapi.NewSDKRemoteAccessHandler(deps.NgrokTunnelMgr, deps.NgrokConfigStore, cfg.Port)
		remoteAccessHandler.SetJWTService(s.JWTService)
		remoteAccessHandler.RegisterRoutes(e)
		tunnelHandler := networkapi.NewTunnelHandler(deps.NgrokConfigStore, cfg.Port)
		tunnelHandler.RegisterRoutes(e)
	}

	// Claude Code CLI routes (protected)
	if deps.ClaudeCodeHandler != nil {
		claudeCodeGroup := protected.Group("/claudecode")
		deps.ClaudeCodeHandler.RegisterRoutes(claudeCodeGroup)
		deps.ChatHandler.SetClaudeCodeHandler(deps.ClaudeCodeHandler)
	} else {
		stub := featureDisabled("claudecode")
		claudeCodeGroup := protected.Group("/claudecode")
		claudeCodeGroup.GET("/version", stub)
		claudeCodeGroup.GET("/config", stub)
		claudeCodeGroup.Any("/*", stub)
	}

	// Memory routes
	if deps.MemoryHandler != nil {
		deps.MemoryHandler.RegisterRoutes(v1)

		// Initialize markdown backend for dual-write and backend switching
		mdDir := ""
		if deps.Config != nil {
			mdDir = deps.Config.Memory.MarkdownDir
		}
		if mdDir == "" {
			mdDir = filepath.Join(dataDir, "memory")
		}
		mdBackend, mdErr := memory.NewPureMarkdownBackend(mdDir)
		if mdErr != nil {
			logger.Warn("Failed to initialize markdown backend", zap.Error(mdErr))
		} else if deps.MemoryHandler.GetUnifiedService() != nil {
			uSvc := deps.MemoryHandler.GetUnifiedService()
			uSvc.SetMarkdownBackend(mdBackend)

			// Set backend mode from config (default: "markdown")
			backendMode := "markdown"
			if deps.Config != nil && deps.Config.Memory.Backend != "" {
				backendMode = deps.Config.Memory.Backend
			}
			if setErr := uSvc.SetBackend(backendMode); setErr != nil {
				logger.Warn("Failed to set memory backend mode", zap.String("mode", backendMode), zap.Error(setErr))
			} else {
				logger.Info("Memory backend configured", zap.String("mode", backendMode))
			}

			// Register unified memory tool in tool registry
			memAdapter := memory.NewToolsAdapter(uSvc)

			// Initialize progressive searcher if local backend available
			if localSvc := uSvc.GetLocalBackend(); localSvc != nil {
				ps := memory.NewProgressiveSearcher(localSvc.GetSearcher())
				deps.MemoryHandler.SetProgressiveSearcher(ps)
				memAdapter.SetProgressiveSearcher(ps)
				logger.Info("Progressive search enabled")
			}

			tools.RegisterMemoryTools(s.ToolRegistry, memAdapter)
			logger.Info("Unified memory tool registered")
		}
	} else {
		stub := featureDisabled("memory")
		memGroup := v1.Group("/memory")
		memGroup.GET("/stats", stub)
		memGroup.GET("/backend", stub)
		memGroup.POST("/search", stub)
		memGroup.Any("/*", stub)
	}

	// Memory Service v2 routes (versioned entries, namespaces)
	// Reuse the main database (blue.db) — v2 tables have distinct names and use IF NOT EXISTS.
	{
		memDB := deps.DB
		memRepo, err := memory.NewMemoryRepository(memDB)
		if err != nil {
			logger.Error("Failed to initialize memory service repository", zap.Error(err))
		} else {
			memNS, err := memory.NewNamespaceStore(memDB)
			if err != nil {
				logger.Error("Failed to initialize memory namespace store", zap.Error(err))
			} else {
				memAPIHandler := memory.NewAPIHandler(memRepo, memNS)
				memAPIHandler.RegisterRoutes(v1)
				logger.Info("Memory Service routes registered")

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
				encHandler.RegisterRoutes(v1)

				// Start background purge scheduler
				purgeScheduler := memory.NewPurgeScheduler(memRepo, 6*time.Hour)
				purgeScheduler.Start(deps.Ctx)
				logger.Info("Memory Service purge scheduler started")

				// Create v2 bridge for existing memory system
				bridge := memory.NewV2Bridge(memRepo)
				if deps.MemoryHandler != nil {
					deps.MemoryHandler.SetV2Bridge(bridge)
				}
			}
		}
	}

	// Personality routes (protected)
	personalityStorage, err := model.NewFileStorage(cfg.DataDir)
	if err != nil {
		logger.Error("Failed to initialize personality storage", zap.Error(err))
	} else {
		// Initialize default personality from workspace SOUL.md
		var wsDir string
		if deps.WorkspaceHandler != nil {
			wsDir = filepath.Join(cfg.DataDir, "workspace")
		}
		if err := model.InitializeDefaultPersonality(cfg.DataDir, wsDir); err != nil {
			logger.Warn("Failed to initialize default personality", zap.Error(err))
		}
		personalityService := controller.NewService(personalityStorage)
		personalityHandler := view.NewHandler(personalityService)
		personalityGroup := protected.Group("/personalities")
		personalityHandler.RegisterRoutes(personalityGroup)
		logger.Info("Personality routes registered")
	}

	// Workspace routes (protected) — SOUL.md, USER.md, IDENTITY.md, etc.
	if deps.WorkspaceHandler != nil {
		workspaceGroup := protected.Group("/workspace")
		deps.WorkspaceHandler.RegisterRoutes(workspaceGroup)
		logger.Info("Workspace routes registered")
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

	// Wire up OTA background checker so DownloadOTA can find packages
	otaChecker := update.NewOTAChecker(cfg.Version, cfg.DataDir, "")
	updateHandler.SetOTAChecker(otaChecker)
	go otaChecker.Run(deps.Ctx)

	logger.Info("OTA update routes registered", zap.Bool("enabled", deps.Config.Update.Enabled))

	// Channel config routes
	if deps.ChannelConfigStore != nil {
		channelConfigHandler := server.NewChannelConfigHandler(deps.ChannelConfigStore)
		channelConfigHandler.SetFactory(server.NewChannelFactory(deps.Logger))
		channelConfigHandler.RegisterRoutes(api)
	} else {
		stub := featureDisabled("channels")
		api.GET("/channels", stub)
	}

	// Provider settings routes (protected)
	providerSettingsHandler := server.NewProviderSettingsHandler(deps.ChatHandler.GetProviderRegistry(), cfg.DataDir)
	providerSettingsGroup := protected.Group("/providers/settings")
	providerSettingsHandler.RegisterRoutes(providerSettingsGroup)

	// User settings routes (protected)
	settingsHandler := server.NewSettingsHandler(cfg.DataDir)
	settingsHandler.RegisterRoutes(protected)
	// Also make locale available to provider settings handler
	providerSettingsHandler.SetSettingsHandler(settingsHandler)
	// Wire settings into chat handler for runtime smart tool selection toggle
	deps.ChatHandler.SetSettingsHandler(settingsHandler)

	// User-level routes (protected) — /api/v1/my/*
	myGroup := protected.Group("/my")

	// Per-user provider config
	userProviderHandler, err := server.NewUserProviderHandler(deps.DB, s.LLMRegistry)
	if err != nil {
		logger.Warn("Failed to initialize user provider handler", zap.Error(err))
	} else {
		userProviderHandler.RegisterRoutes(myGroup.Group("/providers"))
		logger.Info("User provider routes registered")
	}

	// Per-user skill config
	userSkillHandler, err := server.NewUserSkillHandler(deps.DB, skillStoreDb)
	if err != nil {
		logger.Warn("Failed to initialize user skill handler", zap.Error(err))
	} else {
		userSkillHandler.RegisterRoutes(myGroup.Group("/skills"))
		logger.Info("User skill routes registered")
	}

	// Per-user usage metrics
	if deps.MetricsWriter != nil {
		detailedMetricsHandler := metrics.NewHandler(deps.MetricsWriter)
		myGroup.GET("/usage", detailedMetricsHandler.GetMyUsage)
		logger.Info("User usage route registered")
	} else {
		myGroup.GET("/usage", featureDisabled("metrics"))
	}

	// Static routes (must be last)
	web.RegisterStaticRoutes(e)

	logger.Info("All routes registered")
	return apiProtected
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

// migrateSettingsDB copies data from the legacy settings.db into the main kvstore table
// (now in blue.db) and removes the old file.
func migrateSettingsDB(oldPath string, dest *kvstore.SQLiteStore) {
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return
	}

	old, err := kvstore.NewSQLiteStore(oldPath)
	if err != nil {
		slog.Warn("Failed to open legacy settings.db for migration", "error", err)
		return
	}
	defer old.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	keys, err := old.Keys(ctx, "%")
	if err != nil {
		slog.Warn("Failed to read keys from legacy settings.db", "error", err)
		return
	}

	for _, key := range keys {
		val, err := old.Get(ctx, key)
		if err != nil {
			continue
		}
		if str, ok := val.(string); ok {
			dest.Set(ctx, key, str, 0)
		}
	}

	old.Close()
	if err := os.Remove(oldPath); err != nil {
		slog.Warn("Failed to remove legacy settings.db", "error", err)
	} else {
		slog.Info("Migrated settings.db into blue.db and removed legacy file", "keys", len(keys))
	}
	// Clean up WAL/SHM files
	os.Remove(oldPath + "-wal")
	os.Remove(oldPath + "-shm")
}
