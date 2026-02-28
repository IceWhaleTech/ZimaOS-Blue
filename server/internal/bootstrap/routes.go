// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepsearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/homeassistant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mcp"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/preview"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/webpush"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// routesStartTime records when the server started, used for uptime calculation
var routesStartTime = timeutil.NowTime()

const defaultCCCLIModel = "gpt-5.3-codex-spark"

func resolveDefaultModelForCCCLI(model string, handler *claudecode.Handler) string {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		if handler != nil && handler.IsEnabled() {
			return defaultCCCLIModel
		}
		return "auto"
	}
	if strings.EqualFold(normalized, "auto") && handler != nil && handler.IsEnabled() {
		return defaultCCCLIModel
	}
	return model
}

func resolveMCPWorkspaceRoot(dataDir string, appCfg *config.Config) string {
	candidates := make([]string, 0, 4)
	if appCfg != nil {
		if v := strings.TrimSpace(appCfg.ClaudeCodeCLI.Backend.WorkspaceDir); v != "" {
			candidates = append(candidates, v)
		}
		if v := strings.TrimSpace(appCfg.ClaudeCode.WorkspaceDir); v != "" {
			candidates = append(candidates, v)
		}
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		candidates = append(candidates, cwd)
	}
	if strings.TrimSpace(dataDir) != "" {
		candidates = append(candidates, filepath.Join(dataDir, "workspace"))
	}
	if len(candidates) == 0 {
		return "."
	}

	var fallback string
	for _, c := range candidates {
		normalized := strings.TrimSpace(c)
		if normalized == "" {
			continue
		}
		if !filepath.IsAbs(normalized) {
			if abs, err := filepath.Abs(normalized); err == nil {
				normalized = abs
			}
		}
		if fallback == "" {
			fallback = normalized
		}
		if st, err := os.Stat(normalized); err == nil && st.IsDir() {
			return normalized
		}
	}
	if fallback == "" {
		return "."
	}
	return fallback
}

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

// readLocaleFromKV reads the locale from kvstore settings without creating a full SettingsHandler.
func readLocaleFromKV(kv kvstore.Store) string {
	if kv == nil {
		return ""
	}
	var s struct {
		Locale string `json:"locale"`
	}
	if kv.GetJSON(context.Background(), "config:settings", &s) != nil {
		return ""
	}
	return s.Locale
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
	STTService         stt.Service
	NgrokTunnelMgr     *ngrok.SDKTunnelManager
	NgrokConfigStore   *ngrok.ConfigStore
	ClaudeCodeHandler  *claudecode.Handler
	MemoryHandler      *server.MemoryHandler
	ChannelConfigStore *server.ChannelConfigStore
	ConfigKV           kvstore.Store       // shared kvstore for config persistence
	ConfigStore        *config.ConfigStore // kvstore-backed config persistence
	HotReloader        *config.HotReloader
	WorkspaceHandler   *workspace.Handler
	SSEBroker          *sse.Broker

	// IPC backends (optional, wired from main.go)
	BrowserIPC     sockipc.BrowserBackend
	UIReviewerIPC  sockipc.UIReviewBackend
	UIReviewerTool *tools.UIReviewerTool // for VLM bridge wiring
	AnalyzeTool    *tools.AnalyzeTool    // for LLM bridge wiring
	PushIPC        sockipc.PushBackend
	PushService    *push.Service // for skill/tool wiring
	CronIPC        sockipc.CronBackend

	// Consolidated init deps (previously only in cmd/blue/main.go)
	SkillEmbedFS        fs.FS            // embedded SKILL.md filesystem for ReleaseSkills
	SandboxManager      *sandbox.Manager // for sandbox skill wiring
	SystemPromptBuilder *claudecode.SystemPromptBuilder
	LazyBrowserSvc      func() *browser.RodService // for UI reviewer lazy adapter
	BrowserBackend      tools.BrowserBackend       // for browser tool + IPC

	// Closers collects io.Closers started during route registration.
	// The caller should close them on shutdown (e.g., via lifecycle hooks).
	Closers []interface{ Close() error }

	// ChannelTaskWatcher is populated by RegisterAllRoutes for main.go to wire the notifier.
	ChannelTaskWatcher *mediagen.ChannelTaskWatcher

	// OnEarlyReady is called after critical routes (health, system/mode, auth)
	// are registered but before heavy subsystem init. The caller can start the
	// HTTP listener here so health checks succeed while the rest initializes.
	OnEarlyReady func()
}

// RegisterAllRoutes registers all API routes on the Echo instance.
// Returns the authenticated API group for late-binding route registration.
func RegisterAllRoutes(e *echo.Echo, deps *RoutesDeps) *echo.Group {
	s := deps.Services
	cfg := deps.ServerConfig
	logger := deps.Logger
	dataDir := cfg.DataDir
	kv := deps.ConfigKV // shared kvstore for settings, VAPID keys, toggles, etc.

	// ── Fast path: register critical routes FIRST so the HTTP listener can ──
	// ── start serving health checks while heavy subsystems initialize.     ──

	// Initialize connection manager (lightweight, needed for all routes)
	connManager := connection.NewManager(10000, 5*time.Second)
	e.Use(connManager.Middleware())

	// Preview mode routes (no auth required) — needed for /api/v1/system/mode
	previewModeService := preview.NewModeService(s.UserService)
	previewUpgradeService := preview.NewUpgradeService(s.UserService, s.DB)
	previewHandler := preview.NewHandler(previewModeService, previewUpgradeService, s.JWTService, s.UserService)
	previewHandler.SetDataDir(dataDir)
	previewHandler.RegisterRoutes(e)

	// Set mode service to user handler for preview mode support
	deps.UserHandler.SetModeService(previewModeService)

	// API groups
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	// Lightweight health endpoint — registered FIRST so the Tauri health poll
	// can succeed as soon as the HTTP listener starts, before heavy subsystem init.
	v1.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"service": "zimaos-blue",
			"version": cfg.Version,
		})
	})

	// Static media serving (screenshots, etc.) — no auth required
	mediaDir := filepath.Join(dataDir, "media")
	_ = os.MkdirAll(mediaDir, 0750)
	v1.Static("/media", mediaDir)

	// Static routes — registered before OnEarlyReady so the frontend is
	// servable as soon as the HTTP listener starts. Echo matches specific
	// routes (/api/v1/*) before the wildcard (/*), so order is safe.
	web.RegisterStaticRoutes(e)

	// Signal that critical routes (health, system/mode) are ready.
	// The caller can start the HTTP listener now while heavy subsystems init below.
	if deps.OnEarlyReady != nil {
		deps.OnEarlyReady()
	}

	// ── Deferred path: TLS, ACME, and heavy subsystem init ──

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

	// HTTPS redirect middleware
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager != nil {
		// Wire kvstore for TLS settings persistence
		if deps.ConfigKV != nil {
			tlsManager.SetKVStore(deps.ConfigKV)
		}
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

	logger.Info("Preview mode routes registered")

	// Media generation (image/video/audio) — self-contained, no dependency on ProviderPool
	var ipcSrv *sockipc.Server
	{
		mediaGenDir := filepath.Join(dataDir, "media", "generated")
		mediaStorage := mediagen.NewMediaStorage(mediaGenDir, "/api/media/generated")
		if err := mediaStorage.EnsureDirs(); err != nil {
			logger.Warn("Failed to create media generation dirs", zap.Error(err))
		}

		// Use SQLite-backed config store (auto-migrates from JSON file)
		var mediaConfigStore mediagen.MediaConfigStore
		sqliteMediaStore, err := mediagen.NewSQLiteConfigStore(s.DB)
		if err != nil {
			logger.Warn("Failed to create SQLite media config store, falling back to JSON", zap.Error(err))
			mediaConfigStore = mediagen.NewConfigStore(filepath.Join(dataDir, "media"))
		} else {
			// Migrate from JSON file if exists
			if migrateErr := sqliteMediaStore.MigrateFromJSON(filepath.Join(dataDir, "media")); migrateErr != nil {
				logger.Warn("Failed to migrate media config from JSON", zap.Error(migrateErr))
			}
			mediaConfigStore = sqliteMediaStore
		}

		// One-time migration from provider pool (if media providers were configured there)
		mediagen.MigrateFromProviderPool(filepath.Join(dataDir, "providerpool"), filepath.Join(dataDir, "media"))

		// Read locale from settings for priority ordering
		locale := readLocaleFromKV(kv)

		mediaManager := mediagen.NewManager(mediaStorage, mediaConfigStore, locale)
		mediaManager.InitConfigs()

		// Task persistence for power-failure recovery (shares main DB)
		taskStore, err := mediagen.NewTaskStore(s.DB)
		if err != nil {
			logger.Warn("Failed to initialize media task store", zap.Error(err))
		} else {
			mediaManager.SetTaskStore(taskStore)
			mediaManager.RecoverTasks()
		}

		// Channel task watcher: monitors channel-sourced tasks and sends results back.
		// Notifier is set later by main.go after the channel manager is available.
		channelWatcher := mediagen.NewChannelTaskWatcher(mediaManager, nil, locale)
		channelWatcher.RecoverChannelTasks()
		deps.Closers = append(deps.Closers, channelWatcher)

		// Web Push notifications for media task completion
		wpPriv, wpPub, wpErr := webpush.GetOrCreateVAPIDKeys(kv)
		if wpErr != nil {
			logger.Warn("Failed to initialize VAPID keys", zap.Error(wpErr))
		} else {
			wpStore, wpStoreErr := webpush.NewStore(s.DB)
			if wpStoreErr != nil {
				logger.Warn("Failed to initialize webpush store", zap.Error(wpStoreErr))
			} else {
				wpSender := webpush.NewSender(wpPriv, wpPub, wpStore, logger)
				wpHandler := webpush.NewHandler(wpStore, wpPub)
				wpHandler.RegisterRoutes(v1.Group("/webpush"))

				isCN := strings.HasPrefix(locale, "zh")
				mediaManager.SetOnTaskDone(func(taskID, status, model, imageURL string) {
					var title, body string
					if isCN {
						title = "媒体生成"
						if status == "failed" {
							body = model + " 生成失败"
						} else {
							body = model + " 生成完成"
						}
					} else {
						title = "Media Generation"
						if status == "failed" {
							body = model + " failed"
						} else {
							body = model + " completed"
						}
					}
					go wpSender.SendToAll(context.Background(), title, body, imageURL)
				})
			}
		}

		// Wire SSE event publisher for real-time task progress
		if deps.SSEBroker != nil {
			mediaManager.SetEventPublisher(deps.SSEBroker)
		}

		// Wire media interceptor to ChatHandler for channel IR classification
		if deps.ChatHandler != nil {
			interceptor := mediagen.NewInterceptor(mediaManager, channelWatcher)
			deps.ChatHandler.SetMediaInterceptor(interceptor)
		}

		// Expose watcher for main.go to set the channel notifier
		deps.ChannelTaskWatcher = channelWatcher

		// Register HTTP routes
		mediaHandler := mediagen.NewHandler(mediaManager, mediaStorage, locale)

		// Wire addMessage so DirectGenerate can persist messages into conversations.
		// This makes media tasks visible from any device (server-side persistence).
		mediaHandler.SetAddMessage(func(ctx context.Context, conversationID, role, content string) (string, error) {
			msgID := uuid.New().String()
			now := time.Now().UTC().Format(time.RFC3339Nano)
			_, err := deps.DB.ExecContext(ctx,
				`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
				msgID, conversationID, role, content, now,
			)
			if err != nil {
				return "", err
			}
			return msgID, nil
		})

		mediaHandler.SetUpdateTitle(func(ctx context.Context, conversationID, title string) error {
			_, err := deps.DB.ExecContext(ctx,
				`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`,
				title, time.Now().UTC().Format(time.RFC3339Nano), conversationID,
			)
			return err
		})

		mediaGroup := v1.Group("/media")
		mediaHandler.RegisterRoutes(mediaGroup)
		mediaHandler.RegisterStorageRoutes(e)
		logger.Info("Media generation routes registered")

		// Start IPC socket for LLM skills and CLI
		// Auth: Unix socket file permissions (0600) — no token needed.
		sockPath := "/tmp/blue.sock"
		ipcSrv = sockipc.NewServer(sockPath, logger)

		// Media generator — creates a persistent task via MediaManager
		mediaGen := func(ctx context.Context, category, model, prompt string, params map[string]string) (string, error) {
			req := &mediagen.MediaRequest{
				Prompt: prompt,
				Model:  model,
			}
			switch category {
			case "t2v", "i2v", "kf2v":
				req.Type = mediagen.MediaTypeVideo
			default:
				req.Type = mediagen.MediaTypeImage
			}
			if v, ok := params["size"]; ok {
				req.Size = v
			}
			messageID := params["message_id"]
			source := params["source"]
			if source == "" {
				source = "ipc"
			}
			task, err := mediaManager.CreateTask(ctx, req, messageID, category, source)
			if err != nil {
				return "", err
			}
			return task.ID, nil
		}

		// Status querier
		statusQuery := func(ctx context.Context, taskID string) (map[string]string, error) {
			task, err := mediaManager.GetTask(taskID)
			if err != nil {
				return nil, err
			}
			if task == nil {
				return nil, nil
			}
			return map[string]string{
				"task_id":     task.ID,
				"task_status": string(task.Status),
				"progress":    fmt.Sprintf("%.2f", task.Progress),
				"error":       task.Error,
			}, nil
		}

		sockipc.RegisterMediaHandlers(ipcSrv, mediaGen, statusQuery, logger)

		// Register browser IPC handlers
		if deps.BrowserIPC != nil {
			sockipc.RegisterBrowserHandlers(ipcSrv, deps.BrowserIPC, logger)
		}

		// Register UI reviewer IPC handlers
		if deps.UIReviewerIPC != nil {
			sockipc.RegisterUIReviewHandlers(ipcSrv, deps.UIReviewerIPC, logger)
		}

		// Register push notification IPC handlers
		if deps.PushIPC != nil {
			sockipc.RegisterPushHandlers(ipcSrv, deps.PushIPC, logger)
		}

		// Register cron/scheduler IPC handlers
		if deps.CronIPC != nil {
			sockipc.RegisterCronHandlers(ipcSrv, deps.CronIPC, logger)
		}

		// Skill fallback: forward unmatched IPC commands to the skill executor.
		// This enables `blue <skill_name> key=value` to invoke any registered skill.
		sockipc.RegisterSkillFallback(ipcSrv, sockipc.SkillExecutorFunc(func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
			sk := s.SkillRegistry.Get(skillID)
			if sk == nil {
				return nil, fmt.Errorf("unknown skill: %s", skillID)
			}
			if !s.SkillRegistry.IsEnabled(skillID) {
				return nil, fmt.Errorf("skill %s is disabled", skillID)
			}
			result, err := sk.Execute(ctx, input)
			if err != nil {
				return nil, err
			}
			return sockipc.SkillResultToMap(result.Data, result.Success, result.Error), nil
		}), logger)

		if err := ipcSrv.Start(); err != nil {
			logger.Warn("Failed to start sockipc server", zap.Error(err))
		} else {
			deps.Closers = append(deps.Closers, ipcSrv)
			logger.Info("Socket IPC server started", zap.String("path", sockPath))
		}
	}

	// Detailed health endpoint (with runtime stats, for dashboard)
	v1.GET("/health/stats", func(c echo.Context) error {
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
	if deps.ConfigStore != nil {
		configHandler.SetConfigStore(deps.ConfigStore)
	}
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
	permRepo, permErr := permission.NewRepository(s.DB)
	if permErr != nil {
		logger.Error("Failed to initialize permission repository", zap.Error(permErr))
	}
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
	var skillAutoReranker *claudecode.AutoSkillReranker

	// Smart tool selection
	if deps.Config.ToolCalling.SmartSelection {
		ts := tools.DefaultToolSelector()
		if deps.Config.ToolCalling.SmartSelectionMaxTools > 0 {
			ts.MaxTools = deps.Config.ToolCalling.SmartSelectionMaxTools
		}
		deps.ChatHandler.SetToolSelector(ts)
	}
	// Tool router: dynamic exposure + schema compression.
	// Keep enabled even when smart selection is off.
	deps.ChatHandler.SetToolRouter(tools.DefaultToolRouter())
	// Smart skill selection (progressive: rule -> IR -> optional rerank)
	if deps.Config.ToolCalling.SmartSkillSelection {
		workspaceDir := filepath.Join(cfg.DataDir, "workspace")
		reranker := claudecode.NewAutoSkillReranker(cfg.DataDir, deps.Config.ToolCalling.SkillRerankModel, claudecode.AutoSkillRerankerOptions{
			ONNXEnabled:  deps.Config.ToolCalling.SkillRerankEnabled && deps.Config.ToolCalling.SkillRerankONNXEnabled,
			AutoDownload: deps.Config.ToolCalling.SkillRerankONNXAutoDownload,
		})
		skillAutoReranker = reranker
		if deps.Config.ToolCalling.SkillRerankEnabled {
			reranker.WarmupAsync()
		}
		ss := claudecode.NewSkillSelector(workspaceDir, reranker)
		deps.ChatHandler.SetSkillSelector(ss)
	}

	// Wire SSE broker for cross-tab conversation_updated events during streaming
	if deps.SSEBroker != nil {
		deps.ChatHandler.SetSSEBroker(deps.SSEBroker)
	}

	// Register chat routes
	deps.ChatHandler.RegisterRoutes(v1)

	// Deep search service (shared by API + skill executor)
	deepSearchService := deepsearch.NewService(nil, nil)

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

	// Initialize skill store for browse/search (catalog only, no install state)
	skillStoreDb, err := skillstore.NewStore(deps.DB)
	if err != nil {
		logger.Warn("Failed to initialize skill store", zap.Error(err))
	} else {
		skillHandler.SetStore(skillStoreDb)
		logger.Info("Skill store initialized (catalog only)")

		// Initialize sync service for periodic skill catalog updates
		syncConfig := skillstore.DefaultSyncServiceConfig()
		syncService := skillstore.NewSyncService(skillStoreDb, syncConfig, slog.Default())
		skillHandler.SetSyncService(syncService)
		syncService.Start(deps.Ctx)
		logger.Info("Skill sync service started")
	}

	// Set skills directory for install/uninstall (filesystem-based)
	skillsDir := filepath.Join(cfg.DataDir, "workspace", ".claude", "skills")
	skillHandler.SetSkillsDir(skillsDir)
	if deps.SSEBroker != nil {
		skillHandler.SetEventBroker(deps.SSEBroker)
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
	localScanner := skillstore.NewLocalSkillScanner(skillsDir)
	skillHandler.SetLocalScanner(localScanner)
	// Initial scan to discover released skills
	if err := localScanner.Scan(); err != nil {
		logger.Warn("Initial local skill scan failed", zap.Error(err))
	} else {
		logger.Info("Local skill scanner initialized", zap.Int("count", localScanner.Count()))
	}

	skillHandler.RegisterRoutes(v1)

	// Register skill manager IPC handlers (after scanner + store are ready)
	if ipcSrv != nil {
		skillMgr := newSkillManagerAdapter(skillStoreDb, s.SkillRegistry, localScanner, skillsDir)
		sockipc.RegisterSkillManagerHandlers(ipcSrv, skillMgr, logger)
		logger.Info("Skill manager IPC handlers registered")
	}

	// Release embedded SKILL.md files to workspace
	if deps.SkillEmbedFS != nil && deps.WorkspaceHandler != nil {
		if mgr := deps.WorkspaceHandler.Manager(); mgr != nil {
			if err := mgr.ReleaseSkills(deps.SkillEmbedFS); err != nil {
				logger.Warn("Failed to release embedded skills", zap.Error(err))
			}
			// Re-scan after releasing — the initial scan ran before skills were extracted
			if err := localScanner.Scan(); err != nil {
				logger.Warn("Post-release skill scan failed", zap.Error(err))
			} else {
				logger.Info("Skills re-scanned after release", zap.Int("count", localScanner.Count()))
			}
		}
	}

	// Wire backing services into skills
	if deps.SandboxManager != nil {
		if sk := s.SkillRegistry.Get("sandbox"); sk != nil {
			if sb, ok := sk.(*builtin.Sandbox); ok {
				sb.SetSandboxService(sandbox.NewSkillAdapter(deps.SandboxManager))
			}
		}
	}
	if deps.CronHandler != nil {
		cronAdapter := cron.NewSkillAdapter(deps.CronHandler.GetService)
		if sk := s.SkillRegistry.Get("scheduler"); sk != nil {
			if ss, ok := sk.(*builtin.Scheduler); ok {
				ss.SetCronService(cronAdapter)
			}
		}
		// Register command handler for scheduler skill
		if svc := deps.CronHandler.GetService(); svc != nil {
			svc.RegisterCommandHandler(cron.CommandSecurityConfig{
				Enabled:          true,
				RequireAdminRole: true,
			})
		}
	}
	if deps.WorkflowHandler != nil {
		if sk := s.SkillRegistry.Get("workflows"); sk != nil {
			if ws, ok := sk.(*builtin.Workflows); ok {
				ws.SetWorkflowService(workflow.NewSkillAdapter(deps.WorkflowHandler.GetService))
			}
		}
	}

	// Browser + UI reviewer: wire backends for IPC and LLM tool use.
	if deps.LazyBrowserSvc != nil {
		uiTool := &tools.UIReviewerTool{}
		uiTool.SetBrowser(tools.NewLazyRodBrowserAdapter(deps.LazyBrowserSvc))
		uiTool.SetMediaDir(mediaDir)
		deps.UIReviewerTool = uiTool
	}
	// Wire browser backend into browser skill
	if deps.BrowserBackend != nil {
		if sk := s.SkillRegistry.Get("browser"); sk != nil {
			if br, ok := sk.(*builtin.Browser); ok {
				br.SetBrowserService(browser.NewLazySkillAdapter(deps.LazyBrowserSvc))
			}
		}
	}
	// Wire browser backend into ui_reviewer skill
	if deps.LazyBrowserSvc != nil {
		if sk := s.SkillRegistry.Get("ui_reviewer"); sk != nil {
			if ur, ok := sk.(*builtin.UIReviewer); ok {
				ur.SetBrowserService(browser.NewLazySkillAdapter(deps.LazyBrowserSvc))
			}
		}
	}

	// Wire push notification service into reminder skill
	if deps.PushService != nil {
		if sk := s.SkillRegistry.Get("reminder"); sk != nil {
			if r, ok := sk.(*builtin.Reminder); ok {
				r.SetPushService(push.NewSkillAdapter(func() *push.Service { return deps.PushService }))
			}
		}
	}

	// Analyze: skill-only, executed via `blue analyze`.
	// AnalyzeTool does the heavy lifting; wired into the Analyze skill.
	{
		analyzeTool := tools.NewAnalyzeTool()
		analyzeTool.SetMediaDir(mediaDir)
		if deps.LazyBrowserSvc != nil {
			analyzeTool.SetBrowser(tools.NewLazyRodBrowserBackend(func() *browser.RodService {
				return deps.LazyBrowserSvc()
			}))
		}
		analyzeTool.SetExecutor(tools.NewExecutor(s.ToolRegistry))
		deps.AnalyzeTool = analyzeTool
		// Wire into skill
		if sk := s.SkillRegistry.Get("analyze"); sk != nil {
			if a, ok := sk.(*builtin.Analyze); ok {
				a.SetExecutor(analyzeTool)
			}
		}
	}

	// Web search: wire WebSearchTool into the web_search skill.
	if sk := s.SkillRegistry.Get("web_search"); sk != nil {
		if ws, ok := sk.(*builtin.WebSearch); ok {
			ws.SetSearcher(tools.NewWebSearchTool(tools.WebSearchConfig{}))
		}
	}

	// Deep research: wire service-backed executor into deep_research skill.
	if sk := s.SkillRegistry.Get("deep_research"); sk != nil {
		if ds, ok := sk.(*builtin.DeepSearch); ok {
			ds.SetExecutor(deepsearch.NewSkillExecutor(deepSearchService))
		}
	}

	// Ask-user-question: QuestionManager for handling question dialogs
	var questionMgr *tools.QuestionManager
	if deps.SSEBroker != nil {
		questionMgr = tools.NewQuestionManager(deps.SSEBroker, nil, 2*time.Minute)
		if sk := s.SkillRegistry.Get("ask"); sk != nil {
			if askSkill, ok := sk.(*builtin.Ask); ok {
				askSkill.SetQuestioner(&questionManagerAskAdapter{mgr: questionMgr})
			}
		}
	}

	// Exec tools (shell execution + process management)
	var execApprovals *tools.ApprovalManager
	{
		execConfig := tools.DefaultExecConfig()
		execConfig.DataDir = cfg.DataDir
		// Restrict exec workdir to the data directory (workspace) by default.
		// Access to other directories requires user approval via SSE.
		execConfig.AllowedDirs = []string{cfg.DataDir}
		if deps.SSEBroker != nil {
			execApprovals = tools.NewApprovalManager(deps.SSEBroker)
		}
		var dirStore *tools.DirAllowlistStore
		if deps.DB != nil {
			var err error
			dirStore, err = tools.NewDirAllowlistStore(deps.DB)
			if err != nil {
				slog.Warn("failed to create exec dir allowlist store", "error", err)
			}
		}
		// Wrap sandbox.Manager as SandboxExecutor if available.
		var sbx tools.SandboxExecutor
		if deps.SandboxManager != nil {
			sbx = &sandboxExecAdapter{mgr: deps.SandboxManager}
		}
		tools.RegisterExecTools(s.ToolRegistry, execConfig, execApprovals, deps.SSEBroker, dirStore, sbx)

		// Wire audit store for exec commands (reuses blue.db — write volume is low: 1 row per exec).
		if deps.DB != nil {
			if auditStore, err := tools.NewExecAuditStore(deps.DB); err != nil {
				slog.Warn("failed to create exec audit store", "error", err)
			} else if et := tools.GetExecTool(s.ToolRegistry); et != nil {
				et.SetAuditStore(auditStore)
			}
		}

		// Wire exec tool with known tool names and registry so it can auto-forward
		// commands that look like tool invocations (e.g. "web_search query").
		if execTool := tools.GetExecTool(s.ToolRegistry); execTool != nil {
			execTool.SetToolNames(s.ToolRegistry.List())
			execTool.SetRegistry(s.ToolRegistry)
			// Short-circuit pinned skills (e.g. "web_search query" → skill executor).
			// This avoids registering skills as tools (which would consume extra prompt tokens).
			execTool.SetPinnedSkills(claudecode.PinnedSkills())
			// Short-circuit `blue <skill>` commands: call skill executor directly
			// instead of spawning subprocess + IPC round-trip.
			execTool.SetSkillExecutor(func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
				sk := s.SkillRegistry.Get(skillID)
				if sk == nil {
					return nil, fmt.Errorf("unknown skill: %s", skillID)
				}
				if !s.SkillRegistry.IsEnabled(skillID) {
					return nil, fmt.Errorf("skill %s is disabled", skillID)
				}
				if err := sk.Validate(input); err != nil {
					return nil, fmt.Errorf("skill %s: %w", skillID, err)
				}
				result, err := sk.Execute(ctx, input)
				if err != nil {
					return nil, err
				}
				return sockipc.SkillResultToMap(result.Data, result.Success, result.Error), nil
			})
			execTool.SetSkillSelector(func(ctx context.Context, query string) tools.SkillSelectionDecision {
				selector := deps.ChatHandler.GetSkillSelector()
				if selector == nil {
					return tools.SkillSelectionDecision{}
				}
				opts := claudecode.SelectOptions{
					Mode:                claudecode.SkillSelectorModeHybrid,
					EnableRerank:        true,
					ConfidenceThreshold: 0.78,
				}
				if sh := deps.ChatHandler.GetSettingsHandler(); sh != nil {
					opts.Mode = sh.GetSkillSelectorMode()
					opts.EnableRerank = sh.GetSkillRerankEnabled()
					opts.ConfidenceThreshold = sh.GetSkillSelectorConfidenceThreshold()
				}
				decision, err := selector.Select(ctx, query, opts)
				if err != nil {
					return tools.SkillSelectionDecision{}
				}
				out := tools.SkillSelectionDecision{
					SelectedSkill: decision.SelectedSkill,
					Confidence:    decision.Confidence,
					NeedClarify:   decision.NeedClarify,
					Reason:        decision.Reason,
				}
				for _, c := range decision.Candidates {
					out.Candidates = append(out.Candidates, c.Name)
				}
				return out
			})
		}

		// Exec approval REST endpoint (kept for backwards compatibility;
		// the unified /approval/resolve endpoint also handles exec approvals).
		if execApprovals != nil {
			execGroup := v1.Group("/exec")
			execGroup.GET("/approvals/pending", func(c echo.Context) error {
				userID := resolveRequestUserID(c)
				req := execApprovals.GetPending(userID)
				if req == nil {
					return c.JSON(200, map[string]interface{}{"pending": false})
				}
				return c.JSON(200, map[string]interface{}{"pending": true, "approval": req})
			})
			execGroup.POST("/approvals/:id", func(c echo.Context) error {
				id := c.Param("id")
				var body struct {
					Decision string `json:"decision"`
				}
				if err := c.Bind(&body); err != nil {
					return c.JSON(400, map[string]string{"error": "invalid body"})
				}
				decision := tools.ApprovalDecision(body.Decision)
				if decision != tools.ApprovalAllowOnce && decision != tools.ApprovalAllowAlways && decision != tools.ApprovalDeny {
					return c.JSON(400, map[string]string{"error": "invalid decision; use allow-once, allow-always, or deny"})
				}
				if !execApprovals.ResolveApproval(id, decision) {
					return c.JSON(404, map[string]string{"error": "approval not found or expired"})
				}
				return c.JSON(200, map[string]string{"status": string(decision)})
			})
		}
	}

	// Ask-user-question REST endpoints
	if questionMgr != nil {
		askGroup := v1.Group("/ask-user-question")
		askGroup.GET("/pending", func(c echo.Context) error {
			userID := resolveRequestUserID(c)
			req := questionMgr.GetPending(userID)
			if req == nil {
				if sessionID := strings.TrimSpace(c.QueryParam("session_id")); sessionID != "" {
					req = questionMgr.GetPendingBySession(sessionID)
				}
			}
			if req == nil {
				return c.JSON(200, map[string]interface{}{"pending": false})
			}
			return c.JSON(200, map[string]interface{}{"pending": true, "question": req})
		})
		askGroup.POST("/:id/answer", func(c echo.Context) error {
			id := c.Param("id")
			var body struct {
				Answers []tools.QuestionAnswerResult `json:"answers"`
			}
			if err := c.Bind(&body); err != nil {
				return c.JSON(400, map[string]string{"error": "invalid body"})
			}
			if !questionMgr.ResolveAnswer(id, body.Answers) {
				return c.JSON(404, map[string]string{"error": "question not found or expired"})
			}
			return c.JSON(200, map[string]string{"status": "ok"})
		})
		askGroup.POST("/:id/dismiss", func(c echo.Context) error {
			id := c.Param("id")
			if !questionMgr.DismissQuestion(id) {
				return c.JSON(404, map[string]string{"error": "question not found or expired"})
			}
			return c.JSON(200, map[string]string{"status": "dismissed"})
		})
	}

	// Management tool: skill-only in v0.10.31 (tool registration removed).
	// Service adapters are still wired for the mgmt skill/CLI subcommand.
	mgmtTool := tools.NewMgmtTool()
	// Wire services that are available now
	if deps.ProviderPool != nil {
		mgmtTool.SetProviders(&mgmtProviderAdapter{pool: deps.ProviderPool})
	}
	mgmtTool.SetSkills(&mgmtSkillAdapter{registry: s.SkillRegistry})
	mgmtTool.SetTools(&mgmtToolAdapter{registry: s.ToolRegistry})
	mgmtTool.SetSystem(&mgmtSystemAdapter{version: cfg.Version})
	if s.UserService != nil {
		mgmtTool.SetUsers(&mgmtUserAdapter{service: s.UserService})
	}
	if s.APIKeyService != nil {
		mgmtTool.SetAPIKeys(&mgmtAPIKeyAdapter{service: s.APIKeyService})
	}
	s.MgmtTool = mgmtTool
	// Settings and channels are wired later (created after this point)

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
				return func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
					req.Model = resolveDefaultModelForCCCLI(req.Model, deps.ClaudeCodeHandler)
					return pc.Chat(ctx, req)
				}
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

	// Deep research routes (protected)
	deepSearchHandler := deepsearch.NewHandler(deepSearchService)
	deepSearchHandler.RegisterGroup(protected.Group("/deep-research"))
	deepSearchHandler.RegisterGroup(apiProtected.Group("/deep-research"))
	logger.Info("Deep research routes registered")

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
		providerPoolHandler.SetMediaPricingLookup(func(modelID string) *providerpool.MediaPricingInfo {
			mp := mediagen.GetMediaModelPricing(modelID)
			if mp == nil {
				return nil
			}
			return &providerpool.MediaPricingInfo{Price: mp.OutputPrice, Unit: string(mp.Unit)}
		})

		// Wire media pricing updates from remote model_pricing.json
		deps.ProviderPool.SetMediaPricingApplier(func(modelID string, outputPrice float64, unit string) {
			mediagen.SetMediaModelPricing(modelID, &mediagen.MediaModelPricing{
				OutputPrice: outputPrice,
				Unit:        mediagen.MediaPricingUnit(unit),
			})
		})

		// Initialize OAuth manager for OAuth-based providers (SQLite-backed)
		oauthStore, oauthErr := oauth.NewSQLiteTokenStore(deps.DB)
		if oauthErr != nil {
			logger.Warn("Failed to initialize OAuth store", zap.Error(oauthErr))
		} else {
			// Migrate legacy JSON tokens if present
			legacyPath := filepath.Join(dataDir, "providers", "providers", "oauth_tokens.json")
			if err := oauthStore.MigrateFromJSON(legacyPath); err != nil {
				logger.Warn("Failed to migrate legacy OAuth tokens", zap.Error(err))
			} else {
				// Remove legacy file after successful migration
				os.Remove(legacyPath)
			}
			// Deduplicate tokens to clean up any duplicates
			if err := oauthStore.Deduplicate(); err != nil {
				logger.Warn("Failed to deduplicate OAuth tokens", zap.Error(err))
			}
			oauthManager = oauth.NewManager(oauthStore)
			providerPoolHandler.SetOAuthManager(oauthManager)
			// Register all OAuth callback paths (one for each provider type)
			providerPoolHandler.RegisterOAuthCallbackRoute(e, oauth.AllProviderConfigs())

			// Set OAuth redirect port after server starts (uses actual port, not hardcoded 51121)
			server.OnServerStart(func(port int) {
				if oauthManager != nil && port > 0 {
					oauthManager.SetPort(port)
					logger.Info("OAuth redirect port set to server port", zap.Int("port", port))
				}
			})

			// Start periodic OAuth token refresh to keep short-lived tokens usable.
			server.OnServerStart(func(port int) {
				if oauthManager != nil {
					oauthManager.StartAutoRefresh(context.Background(), 5*time.Minute)
				}
			})

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

		// Start provider pool background tasks (health checks, model discovery, pricing)
		server.OnServerStart(func(port int) {
			go deps.ProviderPool.Start(context.Background())
		})
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
	// Load default rules (all disabled by default — users enable via API)
	for _, rule := range proxy.GetDefaultRules() {
		dataMasker.AddRule(rule)
	}
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
	var agentRunnerRef *agent.Runner
	if deps.Config.Proxy != nil && deps.Config.Proxy.Enabled {
		routingConfig := &deps.Config.Proxy.Routing
		if deps.Config.Proxy.Route != nil {
			routingConfig = deps.Config.Proxy.Route
		}
		proxyRouter := proxy.NewRouter(routingConfig)
		proxyConnPool := proxy.NewConnectionPool(&deps.Config.Proxy.Connection)
		proxyFailover := proxy.NewFailoverHandler(&routingConfig.Failover, proxyRouter)
		proxyHandler := proxy.NewProxyHandler(proxyRouter, proxyConnPool, proxyFailover)
		proxyHandler.SetSTTService(deps.STTService)
		proxyHandler.SetProviderRaceConfig(routingConfig.Failover.ProviderRace)
		proxyHandler.SetPromptCacheEnabled(true) // default ON for new installs
		proxyHandler.SetDataMasker(dataMasker)

		// Smart failover handler for metrics + intelligent error classification
		smartFailover := proxy.NewSmartFailoverHandler(&routingConfig.Failover, proxyRouter)

		// Failover API routes — share the same config pointer so API changes take effect
		failoverAPIHandler := proxy.NewFailoverAPIHandler(smartFailover, &routingConfig.Failover)
		failoverGroup := protected.Group("/proxy/failover")
		failoverAPIHandler.RegisterRoutes(failoverGroup)

		// Pipeline stats collector: unified async batch persistence for routing/failover
		// Uses metrics.db (via MetricsWriter) instead of blue.db for cleaner separation
		var pipelineStats *proxy.PipelineStatsCollector
		var metricsDB *sql.DB
		if deps.MetricsWriter != nil {
			metricsDB = deps.MetricsWriter.GetDB()
		}
		if metricsDB != nil {
			pipelineStats = proxy.NewPipelineStatsCollector(metricsDB, proxyHandler.GetRoutingStatsRef())
			// Wire smart failover metrics + circuit breaker state for persistence
			pipelineStats.SetSmartFailoverMetrics(smartFailover.GetMetrics())
			pipelineStats.SetFailoverHandler(smartFailover.FailoverHandler)
			pipelineStats.LoadSmartMetrics()
			pipelineStats.LoadBreakerState()
			pipelineStats.Start()
			proxyHandler.SetPipelineStats(pipelineStats)
			deps.Closers = append(deps.Closers, pipelineStats)
		} else if deps.DB != nil {
			// Fallback to blue.db if metrics.db is not available
			pipelineStats = proxy.NewPipelineStatsCollector(deps.DB, proxyHandler.GetRoutingStatsRef())
			pipelineStats.SetSmartFailoverMetrics(smartFailover.GetMetrics())
			pipelineStats.SetFailoverHandler(smartFailover.FailoverHandler)
			pipelineStats.LoadSmartMetrics()
			pipelineStats.LoadBreakerState()
			pipelineStats.Start()
			proxyHandler.SetPipelineStats(pipelineStats)
			deps.Closers = append(deps.Closers, pipelineStats)
		}

		if deps.ProviderPool != nil {
			proxyHandler.SetProviderPool(deps.ProviderPool)

			// Wire OAuth manager into proxy handler for OAuth-based provider auth
			if oauthManager != nil {
				proxyHandler.SetOAuthManager(oauthManager)
			}

			// Wire failover callback for both runtime dashboard metrics and persisted pipeline stats.
			if deps.ProviderPool.Router != nil {
				deps.ProviderPool.Router.SetFailoverCallback(func(result *providerpool.FailoverResult) {
					smartFailover.GetMetrics().RecordProviderPoolResult(result)
					if pipelineStats != nil {
						pipelineStats.OnFailover(result)
					}
				})
			}

			// Wire health check latency into router for latency-based routing
			if deps.ProviderPool.Registry != nil && deps.ProviderPool.Router != nil {
				deps.ProviderPool.Registry.SetOnHealthResult(func(providerID string, result *providerpool.HealthCheckResult) {
					if result.Healthy && result.Latency > 0 {
						deps.ProviderPool.Router.UpdateLatency(providerID, result.Latency)
					}
				})
			}

			// Push provider health status changes to all connected clients via SSE
			if deps.ProviderPool.Registry != nil && deps.SSEBroker != nil {
				deps.ProviderPool.Registry.SetOnStatusChange(func(providerID string, oldStatus, newStatus providerpool.ProviderStatus) {
					deps.SSEBroker.Broadcast("provider_status_changed", map[string]string{
						"provider_id": providerID,
						"old_status":  string(oldStatus),
						"status":      string(newStatus),
					})
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

		// Context pruner middleware (enabled by default with local backend, lazy-loaded on first request)
		var prunerMw *pruner.Middleware
		var prunerHandler *pruner.APIHandler
		prunerCfg := pruner.DefaultConfig()
		prunerCfg.Enabled = true
		if deps.Config.Pruner != nil {
			prunerCfg = *deps.Config.Pruner
		}
		// Lazy factory: backend + middleware created on first proxy request, not at startup
		var prunerMu sync.Mutex
		createPrunerMw := func() *pruner.Middleware {
			prunerMu.Lock()
			defer prunerMu.Unlock()
			if prunerMw != nil {
				return prunerMw
			}
			backend, err := pruner.NewBackend(prunerCfg)
			if err != nil {
				slog.Warn("Failed to create pruner backend", "error", err)
				return nil
			}
			prunerMw = pruner.NewMiddleware(backend, prunerCfg, pruner.NewStats())
			if prunerHandler != nil {
				prunerHandler.SetMiddleware(prunerMw)
			}
			slog.Info("Context pruner initialized (lazy)", "backend", prunerCfg.Backend, "threshold", prunerCfg.Threshold)
			return prunerMw
		}
		if prunerCfg.Enabled {
			proxyHandler.SetPrunerFactory(createPrunerMw)
		}
		// Pruner model manager (always available for model download)
		prunerModelDir := filepath.Join(cfg.DataDir, "pruner-models")
		prunerCfg.ModelDir = prunerModelDir // Set ModelDir for ONNX backend
		prunerModelMgr := pruner.NewPrunerModelManager(prunerModelDir)

		// Always register pruner API routes (handler returns disabled status when pruner is off)
		prunerHandler = pruner.NewAPIHandler(prunerMw, &prunerCfg, prunerModelMgr)
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

		// Toggle persistence: reuse shared kvstore
		var toggleStore *proxy.ToggleStore
		getToggleState := func() *proxy.ToggleState {
			state := &proxy.ToggleState{
				PrunerEnabled:      prunerCfg.Enabled && (prunerMw == nil || prunerMw.Enabled()),
				PrunerBackend:      prunerCfg.Backend,
				RoutingEnabled:     proxyHandler.IsRoutingEnabled(),
				MaskingEnabled:     dataMasker.IsEnabled(),
				PromptCacheEnabled: proxyHandler.IsPromptCacheEnabled(),
				FailoverConfig:     &routingConfig.Failover,
				Version:            1,
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
		if kv == nil {
			slog.Warn("Failed to create shared kvstore, toggles will not persist")
		} else {
			toggleStore = proxy.NewToggleStore(kv)
			if saved, loadErr := toggleStore.Load(context.Background()); loadErr == nil && saved != nil {
				// Migrate v0 → v1: old installs had routing/prompt_cache off by default.
				// These should be on unless the user explicitly disabled them, but v0 has no
				// way to distinguish "never set" from "explicitly off". Flip them on once.
				// Masking stays off by default — user must opt in.
				if saved.Version < 1 {
					saved.RoutingEnabled = true
					saved.MaskingEnabled = false
					saved.PromptCacheEnabled = true
					saved.Version = 1
					slog.Info("Migrated feature toggles to v1 (routing, prompt_cache → enabled, masking → disabled)")
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					toggleStore.Save(ctx, saved)
					cancel()
				}

				// Restore pruner state from saved toggles.
				// Middleware is lazy — just update config; factory will use it.
				if saved.PrunerBackend != "" {
					prunerCfg.Backend = saved.PrunerBackend
				}
				if saved.PrunerEnabled {
					prunerCfg.Enabled = true
					if prunerMw != nil {
						// Already eagerly created (shouldn't happen with lazy, but be safe)
						prunerMw.SetEnabled(true)
					} else {
						// Ensure factory is set so lazy init picks up the config
						proxyHandler.SetPrunerFactory(createPrunerMw)
					}
				} else {
					prunerCfg.Enabled = false
					if prunerMw != nil {
						prunerMw.SetEnabled(false)
					}
				}
				proxyHandler.SetRoutingEnabled(saved.RoutingEnabled)
				proxyHandler.SetPromptCacheEnabled(saved.PromptCacheEnabled)
				dataMasker.SetEnabled(saved.MaskingEnabled)
				if saved.FailoverConfig != nil {
					routingConfig.Failover = *saved.FailoverConfig
				}
				proxyHandler.SetProviderRaceConfig(routingConfig.Failover.ProviderRace)
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
			prunerHandler.SetOnMiddlewareCreated(func(mw *pruner.Middleware) {
				proxyHandler.SetPruner(mw)
			})
			maskingOnToggle = saveToggle
		}
		failoverAPIHandler.SetOnProviderRaceChange(func(cfg proxy.ProviderRaceConfig) {
			proxyHandler.SetProviderRaceConfig(cfg)
		})
		failoverAPIHandler.SetOnConfigSave(func(_ *proxy.FailoverConfig) error {
			if toggleStore == nil {
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return toggleStore.Save(ctx, getToggleState())
		})

		// ProxyBridge: route ChatHandler LLM calls through proxy pipeline
		bridge := proxybridge.NewBridge(proxyHandler)
		deps.ChatHandler.SetProxyBridge(bridge)
		deps.ChatHandler.SetIMModel("auto") // proxy auto-selects model

		// Wire LLM calls for voice mode through the same proxy pipeline
		if deps.VoiceHandler != nil {
			deps.VoiceHandler.Service().SetChatFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				req.Model = resolveDefaultModelForCCCLI(req.Model, deps.ClaudeCodeHandler)
				return bridge.Chat(ctx, req)
			})
		}

		// Wire VLM bridge into UI reviewer tool (for IPC-based SKILL)
		if deps.UIReviewerTool != nil {
			deps.UIReviewerTool.SetVLMBridge(tools.NewProxyBridgeVLMAdapter(bridge))
		}
		// Wire VLM bridge into ui_reviewer skill (registered in skill registry)
		if sk := s.SkillRegistry.Get("ui_reviewer"); sk != nil {
			if ur, ok := sk.(*builtin.UIReviewer); ok {
				ur.SetBridge(bridge)
			}
		}

		// Wire LLM bridge into analyze tool
		if deps.AnalyzeTool != nil {
			deps.AnalyzeTool.SetLLMBridge(tools.NewProxyBridgeLLMAdapter(bridge))
		}

		// Agent runner: autonomous background task execution
		if deps.DB != nil && deps.SSEBroker != nil {
			agentStore, agentErr := agent.NewStore(deps.DB)
			if agentErr != nil {
				logger.Warn("Failed to initialize agent store", zap.Error(agentErr))
			} else {
				// Recover stale tasks from previous crash
				if recovered, err := agentStore.RecoverStaleTasks(context.Background()); err != nil {
					logger.Warn("Failed to recover stale agent tasks", zap.Error(err))
				} else if recovered > 0 {
					logger.Info("Recovered stale agent tasks", zap.Int64("count", recovered))
				}
				agentRunner := agent.NewRunner(agentStore, bridge, s.ToolRegistry, tools.NewExecutor(s.ToolRegistry), deps.SSEBroker, agent.RunnerConfig{})
				agentHandler := agent.NewHandler(agentStore, agentRunner)
				agentGroup := protected.Group("/agent")
				agentHandler.RegisterRoutes(agentGroup)
				agentRunnerRef = agentRunner
				logger.Info("Agent task routes registered")
			}
		}

		// MCP server: expose tools to external agents
		mcpServer := mcp.NewServer(s.ToolRegistry, tools.NewExecutor(s.ToolRegistry))
		mcpServer.SetWorkspaceRoot(resolveMCPWorkspaceRoot(cfg.DataDir, deps.Config))
		mcpServer.SetGenerativeRunner(func(ctx context.Context, prompt string, maxTokens int) (string, error) {
			pc := server.NewProxyClient(cfg.Port)
			req := llm.ChatRequest{
				Model:       resolveDefaultModelForCCCLI("auto", deps.ClaudeCodeHandler),
				Messages:    []llm.Message{{Role: llm.RoleUser, Content: prompt}},
				Temperature: 0.2,
				MaxTokens:   maxTokens,
			}
			resp, err := pc.Chat(ctx, req)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(resp.Message.Content), nil
		})
		if deps.WorkspaceHandler != nil {
			if mgr := deps.WorkspaceHandler.Manager(); mgr != nil {
				mcpServer.SetWorkspace(mgr)
			}
		}
		mcpHandler := mcp.NewHandler(mcpServer)
		mcpGroup := v1.Group("/mcp")
		mcpHandler.RegisterRoutes(mcpGroup)
		logger.Info("MCP server routes registered")

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
		promptCacheGroup.GET("/stats", func(c echo.Context) error {
			return c.JSON(200, proxyHandler.GetPromptCacheStats())
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
		// Wire layered memory into chat handler when lazy init completes
		deps.MemoryHandler.SetOnLayeredReady(func(ls *memory.LayeredMemoryService) {
			deps.ChatHandler.SetLayeredMemory(ls)
			if agentRunnerRef != nil {
				agentRunnerRef.SetMemory(newAgentMemoryAdapter(ls))
			}
		})
	} else {
		stub := featureDisabled("memory")
		memGroup := v1.Group("/memory")
		memGroup.GET("/stats", stub)
		memGroup.GET("/backend", stub)
		memGroup.POST("/search", stub)
		memGroup.Any("/*", stub)
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

	// Wire up mgmt tool with upgrade/OTA service
	mgmtTool.SetUpgrade(&mgmtUpgradeAdapter{
		handler:    updateHandler,
		otaChecker: otaChecker,
		version:    cfg.Version,
	})

	// Channel config + channel manager
	if deps.ChannelConfigStore != nil {
		channelManager := channel.NewManager(channel.DefaultConfig(), logger)
		channelFactory := server.NewChannelFactory(deps.Logger)
		// Wire channel manager into mgmt tool
		mgmtTool.SetChannels(&mgmtChannelAdapter{mgr: channelManager})

		// Wire channel task watcher notifier (for media generation results)
		if deps.ChannelTaskWatcher != nil {
			deps.ChannelTaskWatcher.SetNotifier(func(ctx context.Context, channelName string, msg channel.OutgoingMessage) error {
				return channelManager.Send(ctx, channelName, msg)
			})
			if deps.NgrokTunnelMgr != nil {
				deps.ChannelTaskWatcher.SetURLResolver(&mediagen.SimpleURLResolver{
					TunnelGetURL: deps.NgrokTunnelMgr.GetURL,
					ServerPort:   cfg.Port,
				})
			}
		}

		// Wire message handler: autoreply → chat fallback
		channelManager.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
			if deps.AutoreplyService != nil {
				response, rule, err := deps.AutoreplyService.Match(ctx, msg.Content, msg.ChannelName, msg.UserID, "", msg.ChatID)
				if err == nil && response != "" && rule != nil {
					return &channel.OutgoingMessage{ChatID: msg.ChatID, Content: response}, nil
				}
			}
			if deps.ChatHandler != nil {
				aiResponse, err := deps.ChatHandler.ProcessChannelMessage(ctx, msg)
				if err == nil && aiResponse != "" {
					return &channel.OutgoingMessage{ChatID: msg.ChatID, Content: aiResponse}, nil
				}
			}
			return nil, nil
		})

		// Wire warmup function so channel messages trigger pre-computation
		if deps.ChatHandler != nil {
			channelManager.SetWarmupFunc(deps.ChatHandler.DoChannelWarmup)
		}

		// Start enabled channels in background — network I/O should not block route registration
		enabledChannels := deps.ChannelConfigStore.GetEnabled()
		if len(enabledChannels) > 0 {
			cfgStore := deps.ChannelConfigStore
			go func() {
				for _, chCfg := range enabledChannels {
					// Skip channels not available on this platform (e.g. iMessage on non-macOS)
					if chCfg.ID == "imessage" && runtime.GOOS != "darwin" {
						logger.Info("Skipping iMessage channel — not available on this platform")
						continue
					}
					ch, err := channelFactory.CreateChannel(chCfg)
					if err != nil || ch == nil {
						continue
					}
					if err := channelManager.Register(ch); err != nil {
						continue
					}
					if err := channelManager.StartChannel(context.Background(), chCfg.ID); err != nil {
						chCfg.Status = "error"
						chCfg.LastError = err.Error()
						_ = cfgStore.Set(chCfg.ID, chCfg)
					} else {
						chCfg.Status = "connected"
						chCfg.LastError = ""
						_ = cfgStore.Set(chCfg.ID, chCfg)
					}
				}
				logger.Info("Enabled channels started in background", zap.Int("count", len(enabledChannels)))
			}()
		}

		channelConfigHandler := server.NewChannelConfigHandler(deps.ChannelConfigStore)
		channelConfigHandler.SetManager(channelManager)
		channelConfigHandler.SetFactory(channelFactory)
		channelConfigHandler.RegisterRoutes(api)
	} else {
		stub := featureDisabled("channels")
		api.GET("/channels", stub)
	}

	// Provider settings routes (protected)
	providerSettingsHandler := server.NewProviderSettingsHandler(deps.ChatHandler.GetProviderRegistry(), kv)
	providerSettingsGroup := protected.Group("/providers/settings")
	providerSettingsHandler.RegisterRoutes(providerSettingsGroup)

	// User settings routes (protected)
	settingsHandler := server.NewSettingsHandler(kv)
	settingsHandler.RegisterRoutes(protected)
	// Also make locale available to provider settings handler
	providerSettingsHandler.SetSettingsHandler(settingsHandler)
	// Wire settings into chat handler for runtime smart tool selection toggle
	deps.ChatHandler.SetSettingsHandler(settingsHandler)
	if skillAutoReranker != nil {
		settingsHandler.SetSkillRerankerModelManager(skillAutoReranker.ModelManager())
		skillAutoReranker.SetSwitchFuncs(
			func() bool {
				rerankEnabled := deps.Config.ToolCalling.SkillRerankEnabled
				if settingsHandler.IsSkillRerankEnabledSet() {
					rerankEnabled = settingsHandler.GetSkillRerankEnabled()
				}
				onnxEnabled := deps.Config.ToolCalling.SkillRerankONNXEnabled
				if settingsHandler.IsSkillRerankONNXEnabledSet() {
					onnxEnabled = settingsHandler.GetSkillRerankONNXEnabled()
				}
				return rerankEnabled && onnxEnabled
			},
			func() bool {
				autoDownload := deps.Config.ToolCalling.SkillRerankONNXAutoDownload
				if settingsHandler.IsSkillRerankONNXAutoDownloadSet() {
					autoDownload = settingsHandler.GetSkillRerankONNXAutoDownload()
				}
				return autoDownload
			},
		)
	} else {
		// Keep download/status endpoints usable even when smart skill selection
		// (and thus AutoSkillReranker) is not initialized.
		settingsHandler.SetSkillRerankerModelManager(
			claudecode.NewSkillRerankerModelManager(cfg.DataDir, deps.Config.ToolCalling.SkillRerankModel),
		)
	}
	if execTool := tools.GetExecTool(s.ToolRegistry); execTool != nil {
		execTool.SetAutoConfirmFunc(settingsHandler.GetAgentAutoConfirm)
	}
	// Wire locale into system prompt builder so all skills see the user's locale
	if deps.SystemPromptBuilder != nil {
		deps.SystemPromptBuilder.SetLocaleFunc(settingsHandler.GetLocale)
		deps.SystemPromptBuilder.SetAgentModeFunc(settingsHandler.GetAgentMode)
		deps.SystemPromptBuilder.SetAgentAutoConfirmFunc(settingsHandler.GetAgentAutoConfirm)
	}
	// Wire locale into push service for i18n in notifications
	if deps.PushService != nil {
		deps.PushService.SetLocaleFunc(settingsHandler.GetLocale)
	}
	dataMasker.SetLocaleFunc(settingsHandler.GetLocale)
	// Wire settings into mgmt tool (deferred — settingsHandler created after tool registration)
	mgmtTool.SetSettings(&mgmtSettingsAdapter{handler: settingsHandler})
	// Wire question manager silent func to settingsHandler.GetAgentAutoConfirm
	if questionMgr != nil {
		questionMgr.SetSilentFunc(settingsHandler.GetAgentAutoConfirm)
		questionMgr.SetTimeoutFunc(func() time.Duration {
			seconds := settingsHandler.GetAgentAskTimeoutSeconds()
			if seconds <= 0 {
				seconds = 120
			}
			return time.Duration(seconds) * time.Second
		})
		questionMgr.SetTimeoutActionFunc(settingsHandler.GetAgentAskTimeoutAction)
	}
	if agentRunnerRef != nil {
		agentRunnerRef.SetAskTimeoutFunc(func() time.Duration {
			seconds := settingsHandler.GetAgentAskTimeoutSeconds()
			if seconds <= 0 {
				seconds = 120
			}
			return time.Duration(seconds) * time.Second
		})
		agentRunnerRef.SetAskTimeoutActionFunc(settingsHandler.GetAgentAskTimeoutAction)
	}

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
	userSkillHandler, err := server.NewUserSkillHandler(deps.DB, skillsDir)
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

	// SSE event stream + tool approval endpoints
	if deps.SSEBroker != nil {
		sseHandler := sse.NewHandler(deps.SSEBroker)
		sseHandler.RegisterRoutes(apiProtected.Group("/v1"))

		approvalHandler := networkapi.NewApprovalHandler(deps.SSEBroker)
		// Wire exec approval resolver so /approval/resolve can handle exec approvals too.
		if execApprovals != nil {
			approvalHandler.SetExecResolver(execApprovalAdapter{execApprovals})
		}
		approvalHandler.RegisterRoutes(apiProtected.Group("/v1"))
	}

	logger.Info("All routes registered")
	return apiProtected
}

func resolveRequestUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		if userID := strings.TrimSpace(claims.UserID); userID != "" {
			return userID
		}
	}
	if userID := strings.TrimSpace(c.QueryParam("user_id")); userID != "" {
		return userID
	}
	return "default"
}

// execApprovalAdapter bridges tools.ApprovalManager to api.ExecApprovalResolver.
type execApprovalAdapter struct {
	mgr *tools.ApprovalManager
}

func (a execApprovalAdapter) ResolveApproval(id string, decision string) bool {
	return a.mgr.ResolveApproval(id, tools.ApprovalDecision(decision))
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

// sandboxExecAdapter wraps sandbox.Manager to implement tools.SandboxExecutor.
type sandboxExecAdapter struct {
	mgr *sandbox.Manager
}

func (a *sandboxExecAdapter) RunInSandbox(ctx context.Context, command, workdir string, env map[string]string, timeout time.Duration) (string, string, int, error) {
	shell, shellArgs := tools.GetShellConfig()
	req := sandbox.NewExecutionRequest(shell, append(shellArgs, command)...)
	req.WorkDir = workdir
	req.Timeout = timeout
	req.Env = env

	result, err := a.mgr.Execute(ctx, req)
	if err != nil {
		return "", "", -1, err
	}
	return result.Stdout, result.Stderr, result.ExitCode, nil
}

// agentMemoryAdapter adapts LayeredMemoryService to agent.MemoryRecaller.
type agentMemoryAdapter struct {
	svc *memory.LayeredMemoryService
}

func newAgentMemoryAdapter(svc *memory.LayeredMemoryService) *agentMemoryAdapter {
	return &agentMemoryAdapter{svc: svc}
}

func (a *agentMemoryAdapter) Recall(ctx context.Context, query string, limit int) ([]agent.MemoryResult, error) {
	results, err := a.svc.Recall(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]agent.MemoryResult, len(results))
	for i, r := range results {
		out[i] = agent.MemoryResult{
			Content: r.Chunk.Content,
			Score:   r.Score,
		}
	}
	return out, nil
}
