// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/billing"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	convertsvc "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/inject"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mcp"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	serviceutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/service"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voicewake"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/webpush"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func tryExecuteToolFallback(ctx context.Context, registry *tools.Registry, toolName string, input map[string]any) (map[string]string, bool, error) {
	if registry == nil {
		return nil, false, nil
	}
	tool := registry.Get(strings.TrimSpace(toolName))
	if tool == nil {
		return nil, false, nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		return nil, true, err
	}
	return toolResultToIPCData(result), true, nil
}

func toolResultToIPCData(result any) map[string]string {
	switch typed := result.(type) {
	case nil:
		return map[string]string{}
	case map[string]string:
		out := make(map[string]string, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return out
	case map[string]interface{}:
		return flattenToolResultMap(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return map[string]string{}
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return flattenToolResultMap(parsed)
		}
		return map[string]string{"result": typed}
	default:
		sanitized := tools.SafeToolPayloadValue(typed, 64*1024)
		if parsed, ok := sanitized.(map[string]interface{}); ok {
			return flattenToolResultMap(parsed)
		}
		return map[string]string{"result": tools.SafeToolPayloadString(sanitized, 64*1024)}
	}
}

func flattenToolResultMap(data map[string]interface{}) map[string]string {
	out := make(map[string]string, len(data))
	for k, v := range data {
		switch typed := v.(type) {
		case nil:
			out[k] = "null"
		case string:
			out[k] = typed
		default:
			out[k] = tools.SafeToolPayloadString(typed, 64*1024)
		}
	}
	return out
}

// routesStartTime records when the server started, used for uptime calculation
var routesStartTime = timeutil.NowTime()

const defaultCCCLIModel = "gpt-5.3-codex-spark"

func shouldUseDefaultCCCLIModel(pool *providerpool.Pool, modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	if pool == nil || pool.Discovery == nil {
		// Keep legacy behavior when provider pool is unavailable.
		return true
	}
	m, _, err := pool.Discovery.FindModel(modelID)
	return err == nil && m != nil && m.Enabled
}

func resolveDefaultModelForCCCLI(model string, handler *claudecode.Handler, pool *providerpool.Pool) string {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		if handler != nil && handler.IsEnabled() {
			if shouldUseDefaultCCCLIModel(pool, defaultCCCLIModel) {
				return defaultCCCLIModel
			}
			return "auto"
		}
		return "auto"
	}
	// Respect explicit auto selection from UI/API.
	if strings.EqualFold(normalized, "auto") {
		return "auto"
	}
	return model
}

func resolveMCPWorkspaceRoot(dataDir string, appCfg *config.Config) string {
	return ResolveWorkspaceDir(dataDir, appCfg)
}

type proxyBridgeLLMCaller struct {
	bridge            *proxybridge.Bridge
	claudeCodeHandler *claudecode.Handler
	providerPool      *providerpool.Pool
}

func (c *proxyBridgeLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c == nil || c.bridge == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	req.Model = resolveDefaultModelForCCCLI(req.Model, c.claudeCodeHandler, c.providerPool)
	return c.bridge.Chat(ctx, req)
}

type providerRegistryLLMCaller struct {
	registry *llm.ProviderRegistry
}

func newProviderRegistryLLMCaller(registry *llm.ProviderRegistry) *providerRegistryLLMCaller {
	return &providerRegistryLLMCaller{registry: registry}
}

func (c *providerRegistryLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c == nil || c.registry == nil {
		return nil, fmt.Errorf("no llm provider registry configured")
	}

	names := c.registry.List()
	if len(names) == 0 {
		return nil, fmt.Errorf("no llm providers configured")
	}

	model := strings.TrimSpace(req.Model)
	if model != "" && !strings.EqualFold(model, "auto") {
		for _, name := range names {
			provider := c.registry.Get(name)
			if provider == nil {
				continue
			}
			if providerSupportsModel(provider, model) {
				return provider.Chat(ctx, req)
			}
		}
	}

	var provider llm.Provider
	for _, name := range names {
		if p := c.registry.Get(name); p != nil {
			provider = p
			break
		}
	}
	if provider == nil {
		return nil, fmt.Errorf("no llm providers configured")
	}

	if model == "" || strings.EqualFold(model, "auto") {
		if models := provider.Models(); len(models) > 0 && strings.TrimSpace(models[0]) != "" {
			req.Model = strings.TrimSpace(models[0])
		}
	}

	return provider.Chat(ctx, req)
}

func providerSupportsModel(provider llm.Provider, model string) bool {
	if provider == nil {
		return false
	}
	target := strings.TrimSpace(model)
	if target == "" {
		return false
	}
	for _, m := range provider.Models() {
		if strings.EqualFold(strings.TrimSpace(m), target) {
			return true
		}
	}
	return false
}

func registerAgentAndMCPRoutes(
	protected *echo.Group,
	v1 *echo.Group,
	services *Services,
	cfg *ServerConfig,
	deps *RoutesDeps,
	logger *zap.Logger,
	agentLLMCaller agent.LLMCaller,
	harnessController *harness.Controller,
	mcpPermission ...echo.MiddlewareFunc,
) *agent.Runner {
	if protected == nil || v1 == nil || services == nil || cfg == nil || deps == nil || logger == nil {
		return nil
	}

	toolRegistry := services.ToolRegistry
	if toolRegistry == nil {
		toolRegistry = tools.NewRegistry()
	}
	workspaceDir := ResolveWorkspaceDir(cfg.DataDir, deps.Config)
	executor := tools.NewExecutor(toolRegistry)
	if deps.ChatHandler != nil {
		executor.SetTraceStore(deps.ChatHandler.GetToolTraceStore())
	}

	var agentRunnerRef *agent.Runner
	if deps.DB != nil && deps.SSEBroker != nil && agentLLMCaller != nil {
		agentStore, agentErr := agent.NewStore(deps.DB)
		if agentErr != nil {
			logger.Warn("Failed to initialize agent store", zap.Error(agentErr))
			stub := featureDisabled("agent")
			agentGroup := protected.Group("/agent")
			agentGroup.Any("/*", stub)
		} else {
			// Recover stale tasks from previous crash
			if recovered, err := agentStore.RecoverStaleTasks(context.Background()); err != nil {
				logger.Warn("Failed to recover stale agent tasks", zap.Error(err))
			} else if recovered > 0 {
				logger.Info("Recovered stale agent tasks", zap.Int64("count", recovered))
			}
			agentRunner := agent.NewRunner(agentStore, agentLLMCaller, toolRegistry, executor, deps.SSEBroker, agent.RunnerConfig{})
			agentGroup := protected.Group("/agent")
			if harnessController != nil {
				agentDriver := harnessdrivers.NewAgentDriver(harness.RunKindAgentTask, agentRunner, agentStore, harnessController)
				subagentDriver := harnessdrivers.NewAgentDriver(harness.RunKindSubagent, agentRunner, agentStore, harnessController)
				agentRunner.SetEventObserver(agentDriver)
				harnessController.RegisterDriver(agentDriver)
				harnessController.RegisterDriver(subagentDriver)
				harness.NewAgentCompatHandler(harnessController, agentStore, agentRunner, workspaceDir).RegisterRoutes(agentGroup)
			} else {
				agent.NewHandler(agentStore, agentRunner).RegisterRoutes(agentGroup)
			}
			agentRunnerRef = agentRunner
			logger.Info("Agent task routes registered")
		}
	} else {
		stub := featureDisabled("agent")
		agentGroup := protected.Group("/agent")
		agentGroup.Any("/*", stub)
	}

	// MCP server: expose tools to external agents (available with and without proxy mode)
	mcpServer := mcp.NewServer(toolRegistry, executor)
	mcpServer.SetWorkspaceRoot(resolveMCPWorkspaceRoot(cfg.DataDir, deps.Config))
	if agentLLMCaller != nil {
		mcpServer.SetGenerativeRunner(func(ctx context.Context, prompt string, maxTokens int) (string, error) {
			req := llm.ChatRequest{
				Model:       "auto",
				Messages:    []llm.Message{{Role: llm.RoleUser, Content: prompt}},
				Temperature: 0.2,
				MaxTokens:   maxTokens,
			}
			resp, err := agentLLMCaller.Chat(ctx, req)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(resp.Message.Content), nil
		})
	}
	if deps.WorkspaceHandler != nil {
		if mgr := deps.WorkspaceHandler.Manager(); mgr != nil {
			mcpServer.SetWorkspace(mgr)
		}
	}
	mcpHandler := mcp.NewHandler(mcpServer)
	mcpGroup := protected.Group("/mcp")
	if len(mcpPermission) > 0 && mcpPermission[0] != nil {
		mcpGroup = protected.Group("/mcp", mcpPermission[0])
	}
	mcpHandler.RegisterRoutes(mcpGroup)
	logger.Info("MCP server routes registered")

	return agentRunnerRef
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

func updateResumeContextString(ctx map[string]interface{}, key string) string {
	if len(ctx) == 0 || key == "" {
		return ""
	}
	raw, ok := ctx[key]
	if !ok || raw == nil {
		return ""
	}
	if v, ok := raw.(string); ok {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func normalizeUpdateResumeKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none", "noop":
		return ""
	case "once-cron", "cron-once", "once_cron", "cron_once":
		return "once-cron"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func buildUpdateResumeRecoverer(cronHandler *cron.Handler, logger *zap.Logger) update.ResumeRecoverer {
	return func(_ context.Context, input update.ResumeRecoverInput) error {
		kind := normalizeUpdateResumeKind(updateResumeContextString(input.Context, "kind"))
		if kind == "" {
			return nil
		}
		switch kind {
		case "once-cron":
			if cronHandler == nil {
				return fmt.Errorf("resume kind %q requires cron handler", kind)
			}
			jobID := updateResumeContextString(input.Context, "job_id")
			if jobID == "" {
				return fmt.Errorf("resume kind %q requires context.job_id", kind)
			}
			svc := cronHandler.GetService()
			if svc == nil {
				return fmt.Errorf("cron service unavailable")
			}
			if err := svc.Trigger(jobID); err != nil {
				return fmt.Errorf("trigger cron job %s: %w", jobID, err)
			}
			if logger != nil {
				logger.Info("OTA resume context triggered cron job",
					zap.String("task_id", input.TaskID),
					zap.String("job_id", jobID),
					zap.String("kind", kind))
			}
			return nil
		default:
			return fmt.Errorf("unsupported resume context kind %q", kind)
		}
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
	FlagEvaluator    *config.FlagEvaluator
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
	Gateway            *gateway.Gateway
	GatewayHandler     *gateway.Handler

	// IPC backends (optional, wired from main.go)
	BrowserIPC     sockipc.BrowserBackend
	UIReviewerIPC  sockipc.UIReviewBackend
	UIReviewerTool *tools.UIReviewerTool  // for VLM bridge wiring
	AnalyzeTool    *tools.AnalyzeTool     // for LLM bridge wiring
	MediaManager   *mediagen.Manager      // for native image tool wiring
	MediaStorage   *mediagen.MediaStorage // for ppt/image review wiring
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

func registerPublicAuthRoutes(v1 *echo.Group, userHandler *user.Handler) {
	if v1 == nil || userHandler == nil {
		return
	}

	v1.POST("/auth/login", userHandler.Login)
	v1.POST("/auth/logout", userHandler.Logout)
	v1.POST("/auth/refresh", userHandler.RefreshToken)
	v1.GET("/auth/password-policy", userHandler.GetPasswordPolicy)
}

func registerCurrentUserRoutes(v1 *echo.Group, authMiddleware *auth.AuthMiddleware, userHandler *user.Handler) {
	if v1 == nil || userHandler == nil {
		return
	}

	if authMiddleware == nil {
		v1.GET("/users/me", userHandler.GetCurrentUser)
		v1.PUT("/users/me", userHandler.UpdateCurrentUser)
		return
	}

	// Preserve preview-mode access for GET while still hydrating request context
	// from Bearer tokens when present.
	v1.GET("/users/me", userHandler.GetCurrentUser, authMiddleware.OptionalAuthenticate())
	v1.PUT("/users/me", userHandler.UpdateCurrentUser, authMiddleware.Authenticate())
}

func registerBrowserApprovalRoutes(v1 *echo.Group, authMiddleware, pageMiddleware echo.MiddlewareFunc, browserSiteStore *tools.BrowserSiteAllowlistStore) {
	if v1 == nil || browserSiteStore == nil {
		return
	}

	middlewares := make([]echo.MiddlewareFunc, 0, 2)
	if authMiddleware != nil {
		middlewares = append(middlewares, authMiddleware)
	}
	if pageMiddleware != nil {
		middlewares = append(middlewares, pageMiddleware)
	}

	browserGroup := v1.Group("/browser", middlewares...)
	browserGroup.GET("/approvals/sites", func(c echo.Context) error {
		userID := resolveRequestUserID(c)
		entries, err := browserSiteStore.List()
		if err != nil {
			return c.JSON(500, map[string]string{"error": "failed to load approved browser sites"})
		}

		type siteEntry struct {
			ID         string `json:"id"`
			Origin     string `json:"origin"`
			AddedAt    string `json:"added_at"`
			LastUsed   string `json:"last_used"`
			ApprovedBy string `json:"approved_by,omitempty"`
		}
		out := make([]siteEntry, 0, len(entries))
		for _, entry := range entries {
			approvedBy := strings.TrimSpace(entry.ApprovedBy)
			if approvedBy != "" && userID != "default" && userID != approvedBy {
				continue
			}
			out = append(out, siteEntry{
				ID:         entry.ID,
				Origin:     entry.Origin,
				AddedAt:    entry.AddedAt.UTC().Format(time.RFC3339),
				LastUsed:   entry.LastUsed.UTC().Format(time.RFC3339),
				ApprovedBy: approvedBy,
			})
		}

		return c.JSON(200, map[string]interface{}{
			"entries": out,
		})
	})

	browserGroup.DELETE("/approvals/sites/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.JSON(400, map[string]string{"error": "site id is required"})
		}

		userID := resolveRequestUserID(c)
		entries, err := browserSiteStore.List()
		if err != nil {
			return c.JSON(500, map[string]string{"error": "failed to load approved browser sites"})
		}
		found := false
		for _, entry := range entries {
			if entry.ID != id {
				continue
			}
			found = true
			approvedBy := strings.TrimSpace(entry.ApprovedBy)
			if approvedBy != "" && userID != "default" && userID != approvedBy {
				return c.JSON(403, map[string]string{"error": "forbidden"})
			}
			break
		}
		if !found {
			return c.JSON(404, map[string]string{"error": "browser site approval not found"})
		}
		if err := browserSiteStore.Delete(id); err != nil {
			return c.JSON(500, map[string]string{"error": "failed to revoke browser site approval"})
		}
		return c.JSON(200, map[string]bool{"deleted": true})
	})
}

// RegisterAllRoutes registers all API routes on the Echo instance.
// Returns the authenticated API group for late-binding route registration.
func RegisterAllRoutes(e *echo.Echo, deps *RoutesDeps) *echo.Group {
	s := deps.Services
	cfg := deps.ServerConfig
	logger := deps.Logger
	dataDir := cfg.DataDir
	workspaceDir := ResolveWorkspaceDir(cfg.DataDir, deps.Config)
	workspaceAllowedPaths := ResolveBuiltinToolAllowedPaths(deps.Config, cfg.DataDir)
	kv := deps.ConfigKV // shared kvstore for settings, VAPID keys, toggles, etc.
	flagEvaluator := deps.FlagEvaluator
	if flagEvaluator == nil && deps.Config != nil {
		flagEvaluator = config.NewFlagEvaluator(&deps.Config.Grayscale)
		deps.FlagEvaluator = flagEvaluator
	}

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
	if deps.AuthMiddleware != nil {
		deps.AuthMiddleware.SetPreviewModeChecker(previewModeService)
	}

	// Set mode service to user handler for preview mode support
	deps.UserHandler.SetModeService(previewModeService)

	// API groups
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	// Public auth routes must exist before OnEarlyReady so the frontend can
	// fetch password policy and refresh tokens during early startup.
	registerPublicAuthRoutes(v1, deps.UserHandler)

	permRepo, permErr := permission.NewRepository(s.DB)
	if permErr != nil {
		logger.Error("Failed to initialize permission repository", zap.Error(permErr))
	}
	permService := permission.NewService(permRepo, s.UserRepo)
	permHandler := permission.NewHandler(permService, s.UserRepo)
	requirePagePermission := func(page string) echo.MiddlewareFunc {
		return permission.RequirePagePermission(permService, page)
	}
	authPageV1Group := func(page string) *echo.Group {
		return v1.Group("", deps.AuthMiddleware.Authenticate(), requirePagePermission(page))
	}
	authPageAPIGroup := func(page string) *echo.Group {
		return api.Group("", deps.AuthMiddleware.Authenticate(), requirePagePermission(page))
	}
	protected := v1.Group("")
	protected.Use(deps.AuthMiddleware.Authenticate())
	apiProtected := api.Group("")
	apiProtected.Use(deps.AuthMiddleware.Authenticate())

	// Frontend bootstrap blocks on these authenticated endpoints before the Vue
	// app mounts, so register them on the fast path with health/system routes.
	registerCurrentUserRoutes(v1, deps.AuthMiddleware, deps.UserHandler)
	if deps.AuthMiddleware == nil {
		permHandler.RegisterCurrentUserRoutes(v1)
	} else {
		permHandler.RegisterCurrentUserRoutes(v1.Group("", deps.AuthMiddleware.Authenticate()))
	}

	// Lightweight health endpoint — registered FIRST so the Tauri health poll
	// can succeed as soon as the HTTP listener starts, before heavy subsystem init.
	v1.GET("/health", func(c echo.Context) error {
		if StartupTraceEnabled() {
			if mark := strings.TrimSpace(c.QueryParam("startup_mark")); mark != "" {
				fields := []zap.Field{
					zap.String("component", "web.bootstrap"),
					zap.String("label", mark),
					zap.String("path", c.QueryParam("path")),
				}
				if msRaw := strings.TrimSpace(c.QueryParam("startup_ms")); msRaw != "" {
					if ms, err := strconv.ParseFloat(msRaw, 64); err == nil {
						fields = append(fields, zap.Float64("client_ms", ms))
					}
				}
				logger.Info("startup-trace", fields...)
			}
		}

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

	// The desktop/web shell requests these lightweight authenticated endpoints
	// immediately after the health check succeeds. Register them on the fast
	// path so embedded startup does not transiently 404 while deferred init
	// continues below.
	settingsHandler := server.NewSettingsHandler(kv)
	settingsHandler.SetChatHandler(deps.ChatHandler)
	smManager := smallmodel.NewManager(cfg.DataDir)
	settingsHandler.SetSmallModelManager(smManager)
	settingsHandler.RegisterRoutes(authPageV1Group(permission.PageSettings))

	var approvalHandler *networkapi.ApprovalHandler
	if deps.SSEBroker != nil {
		sse.NewHandler(deps.SSEBroker).RegisterRoutes(apiProtected.Group("/v1"))
		approvalHandler = networkapi.NewApprovalHandler(deps.SSEBroker)
		approvalHandler.RegisterRoutes(apiProtected.Group("/v1"))
	}

	if deps.ChatHandler != nil {
		mode := ""
		if cfg != nil {
			mode = strings.TrimSpace(cfg.Mode)
		}
		supported := runtime.GOOS == "darwin" &&
			(strings.EqualFold(mode, "embedded") || serviceutil.IsInteractive())
		voiceWakeManager := voicewake.NewManager(voicewake.ManagerConfig{
			Settings:  settingsHandler,
			Submitter: deps.ChatHandler,
			Supported: supported,
		})
		settingsHandler.SetVoiceWakeManager(voiceWakeManager)
		voicewake.NewHandler(voiceWakeManager).RegisterRoutes(
			protected.Group("/voice-wake", requirePagePermission(permission.PageSettings)),
		)
		if deps.Ctx != nil {
			go func(done <-chan struct{}) {
				<-done
				_ = voiceWakeManager.Close()
			}(deps.Ctx.Done())
			_ = voiceWakeManager.Refresh(deps.Ctx)
		} else {
			_ = voiceWakeManager.Refresh(context.Background())
		}
	}

	tunnelHandler := networkapi.NewTunnelHandler(deps.NgrokConfigStore, cfg.Port)
	tunnelHandler.SetJWTService(s.JWTService)
	tunnelHandler.RegisterGroupRoutes(authPageV1Group(permission.PageChannels))

	claudeCodeGroup := authPageV1Group(permission.PageChat).Group("/claudecode")
	if deps.ClaudeCodeHandler != nil {
		deps.ClaudeCodeHandler.RegisterRoutes(claudeCodeGroup)
	} else {
		stub := featureDisabled("claudecode")
		claudeCodeGroup.GET("/version", stub)
		claudeCodeGroup.GET("/config", stub)
		claudeCodeGroup.Any("/*", stub)
	}

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
		deps.MediaStorage = mediaStorage
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
		mediagen.MigrateFromProviderPool(filepath.Join(dataDir, "providerpool"), mediaConfigStore)

		// Read locale from settings for priority ordering
		locale := readLocaleFromKV(kv)

		mediaManager := mediagen.NewManager(mediaStorage, mediaConfigStore, locale)
		mediaManager.InitConfigs()
		if deps.Config != nil && deps.LazyBrowserSvc != nil {
			fallbackCfg := deps.Config.Media.Fallback
			webSearchTool := tools.NewWebSearchTool(tools.WebSearchConfig{
				Provider:   fallbackFirstProvider(fallbackCfg.SearchProviderChain),
				Providers:  append([]string(nil), fallbackCfg.SearchProviderChain...),
				MaxResults: fallbackCfg.SearchMaxResults,
				Timeout:    deps.Config.ToolCalling.WebSearch.Timeout,
				SafeSearch: deps.Config.ToolCalling.WebSearch.SafeSearch,
				Region:     deps.Config.ToolCalling.WebSearch.Region,
				APIKey:     deps.Config.ToolCalling.WebSearch.APIKey,
				BaseURL:    deps.Config.ToolCalling.WebSearch.BaseURL,
			})
			renderPort := server.GetActualPort()
			if renderPort == 0 {
				renderPort = deps.ServerConfig.Port
			}
			mediaManager.SetFallbackEngine(mediagen.NewFallbackEngine(mediagen.FallbackConfig{
				Enabled:             fallbackCfg.Enabled,
				SearchProviderChain: append([]string(nil), fallbackCfg.SearchProviderChain...),
				SearchMaxResults:    fallbackCfg.SearchMaxResults,
				ScreenshotWidth:     fallbackCfg.ScreenshotWidth,
				ScreenshotHeight:    fallbackCfg.ScreenshotHeight,
				ComplexPromptChars:  fallbackCfg.ComplexPromptChars,
				RenderBaseURL:       fmt.Sprintf("http://127.0.0.1:%d", renderPort),
				PublicSpaces:        convertFallbackPublicSpaces(fallbackCfg.PublicSpaces),
			}, mediaStorage, mediagen.NewToolWebSearcher(webSearchTool), func() mediagen.FallbackBrowserService {
				return deps.LazyBrowserSvc()
			}, locale))
		}
		deps.MediaManager = mediaManager

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
				wpHandler.RegisterRoutes(v1.Group("/webpush", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageProfile)))

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
		resolveMediaConversationScope := func(ctx context.Context, conversationID string) (string, error) {
			if s.MemoryStore == nil {
				return "", nil
			}
			claims, _ := ctx.Value(auth.UserContextKey).(*auth.UserClaims)
			if claims != nil && claims.Role != "admin" {
				if _, err := s.MemoryStore.GetConversation(ctx, conversationID, claims.UserID); err != nil {
					if err == memory.ErrNotFound {
						return "", echo.NewHTTPError(http.StatusNotFound, "conversation not found")
					}
					return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
				}
				return claims.UserID, nil
			}
			conv, err := s.MemoryStore.GetConversation(ctx, conversationID)
			if err != nil {
				if err == memory.ErrNotFound {
					return "", echo.NewHTTPError(http.StatusNotFound, "conversation not found")
				}
				return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
			}
			if claims == nil && strings.TrimSpace(conv.UserID) != "" {
				return "", echo.NewHTTPError(http.StatusNotFound, "conversation not found")
			}
			return strings.TrimSpace(conv.UserID), nil
		}
		mediaHandler.SetResolveConversationScope(resolveMediaConversationScope)

		mediaHandler.SetAddMessage(func(ctx context.Context, conversationID, role, content string) (string, error) {
			scopedUserID, err := resolveMediaConversationScope(ctx, conversationID)
			if err != nil {
				return "", err
			}
			if s.MemoryStore != nil {
				msg, err := s.MemoryStore.AddMessage(ctx, conversationID, memory.Message{Role: role, Content: content}, scopedUserID)
				if err != nil {
					return "", err
				}
				return msg.ID, nil
			}
			msgID := uuid.New().String()
			now := time.Now().UTC().Format(time.RFC3339Nano)
			_, err = deps.DB.ExecContext(ctx,
				`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
				msgID, conversationID, role, content, now,
			)
			if err != nil {
				return "", err
			}
			return msgID, nil
		})

		mediaHandler.SetUpdateTitle(func(ctx context.Context, conversationID, title string) error {
			scopedUserID, err := resolveMediaConversationScope(ctx, conversationID)
			if err != nil {
				return err
			}
			if s.MemoryStore != nil {
				return s.MemoryStore.UpdateConversationTitle(ctx, conversationID, title, scopedUserID)
			}
			_, err = deps.DB.ExecContext(ctx,
				`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`,
				title, time.Now().UTC().Format(time.RFC3339Nano), conversationID,
			)
			return err
		})

		mediaGroup := v1.Group("/media")
		if deps.AuthMiddleware != nil {
			mediaGroup.Use(deps.AuthMiddleware.OptionalAuthenticate())
		}
		mediaHandler.RegisterRoutes(mediaGroup)
		mediaHandler.RegisterStorageRoutes(e)
		logger.Info("Media generation routes registered")

		// Start IPC socket for LLM skills and CLI
		// Auth: Unix socket file permissions (0600) — no token needed.
		sockPath := resolveIPCSocketPath(dataDir)
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
			lookupUserID := strings.TrimSpace(tools.GetUserID(ctx))
			var (
				task *mediagen.MediaTask
				err  error
			)
			if lookupUserID != "" {
				task, err = mediaManager.GetTask(taskID, lookupUserID)
			} else {
				task, err = mediaManager.GetTask(taskID)
			}
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
		// This enables `blue <skill_name> key=value` to invoke any registered skill,
		// and falls back to builtin tools like `web_fetch` when no skill matches.
		sockipc.RegisterSkillFallback(ipcSrv, sockipc.SkillExecutorFunc(func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
			sk := s.SkillRegistry.Get(skillID)
			if sk != nil {
				if !s.SkillRegistry.IsEnabled(skillID) {
					return nil, fmt.Errorf("skill %s is disabled", skillID)
				}
				result, err := sk.Execute(ctx, input)
				if err != nil {
					return nil, err
				}
				return sockipc.SkillResultToMap(result.Data, result.Success, result.Error), nil
			}

			if data, handled, err := tryExecuteToolFallback(ctx, s.ToolRegistry, skillID, input); handled {
				return data, err
			}

			return nil, fmt.Errorf("unknown skill: %s", skillID)
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

	// Templates stay public for lightweight bootstrap flows; config is protected.
	configHandler := server.NewConfigHandler(deps.HotReloader)
	if deps.ConfigStore != nil {
		configHandler.SetConfigStore(deps.ConfigStore)
	}
	configHandler.RegisterRoutes(authPageV1Group(permission.PageSettings))
	templatesHandler := server.NewTemplatesHandler()
	templatesHandler.RegisterRoutes(v1)

	// Formfiller management routes
	if deps.FormfillerHandler != nil {
		formfillerGroup := v1.Group("/formfiller", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageTools))
		deps.FormfillerHandler.RegisterRoutes(formfillerGroup)
	} else {
		stub := featureDisabled("formfiller")
		formfillerGroup := v1.Group("/formfiller", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageTools))
		formfillerGroup.GET("/templates", stub)
		formfillerGroup.GET("/config", stub)
		formfillerGroup.Any("/*", stub)
	}

	// External auth routes
	authGroup := v1.Group("/auth")
	if deps.ExtauthHandler != nil {
		deps.ExtauthHandler.RegisterRoutes(authGroup)
	}

	// Gateway REST + WS routes
	if deps.Gateway != nil {
		tools.RegisterGatewayTool(s.ToolRegistry, deps.Gateway)
	}

	if deps.Gateway != nil && deps.GatewayHandler != nil {
		registerGatewayMethods(deps.Gateway, deps)
		deps.GatewayHandler.RegisterRoutes(e, protected)
		deps.Closers = append(deps.Closers, gatewayStopper{gateway: deps.Gateway})
	}

	// A2UI canvas routes (protected) - /api/v1/a2ui/*
	if s.A2UIManager != nil {
		tools.RegisterCanvasTools(s.ToolRegistry, s.A2UIManager)
		a2uiHandler := a2ui.NewHandler(s.A2UIManager)
		a2uiHandler.RegisterRoutes(protected.Group("/a2ui"))
	}
	if s.PDFService != nil {
		tools.RegisterPDFTool(s.ToolRegistry, s.PDFService)
	}

	// Protected external auth routes
	if deps.ExtauthHandler != nil {
		protectedAuthGroup := protected.Group("/auth")
		deps.ExtauthHandler.RegisterProtectedRoutes(protectedAuthGroup)
	}

	// MFA routes
	mfaHandler := mfa.NewHandler(nil, nil)
	mfaHandler.RegisterRoutes(protected)

	// User routes
	usersGroup := protected.Group("/users", requirePagePermission(permission.PageUsers))
	usersGroup.GET("", deps.UserHandler.ListUsers)
	usersGroup.POST("", deps.UserHandler.CreateUser)
	usersGroup.GET("/:id", deps.UserHandler.GetUser)
	usersGroup.PUT("/:id", deps.UserHandler.UpdateUser)
	usersGroup.DELETE("/:id", deps.UserHandler.DeleteUser)
	usersGroup.POST("/:id/lock", deps.UserHandler.LockUser)
	usersGroup.POST("/:id/unlock", deps.UserHandler.UnlockUser)
	usersGroup.POST("/:id/reset-password", deps.UserHandler.ResetPassword)

	permHandler.RegisterAdminRoutes(protected)
	logger.Info("Permission routes registered")

	// Password change
	protected.POST("/auth/password", deps.UserHandler.ChangePassword, requirePagePermission(permission.PageProfile))

	// API Keys routes
	apiKeysGroup := protected.Group("/apikeys", requirePagePermission(permission.PageProfile))
	deps.APIKeyHandler.RegisterRoutes(apiKeysGroup)

	// Billing routes (admin-only checks are enforced by the billing handler)
	billingGroup := protected.Group("/billing")
	if deps.ProviderPool != nil && deps.ProviderPool.Storage != nil {
		billingService := billing.NewService(deps.ProviderPool.Storage)
		billingHandler := billing.NewHandler(billingService)
		billingHandler.RegisterRoutes(billingGroup)
	} else {
		stub := featureDisabled("billing")
		billingGroup.GET("/summary", stub)
		billingGroup.GET("/lines", stub)
		billingGroup.GET("/export", stub)
		billingGroup.Any("/*", stub)
	}

	// Set prompt guard on chat handler
	promptGuard := promptguard.NewDetector(promptguard.DefaultDetectorConfig())
	deps.ChatHandler.SetPromptGuard(promptGuard)
	if deps.SecurityHandler != nil {
		deps.SecurityHandler.SetPromptGuard(promptGuard)
	}
	var skillAutoReranker *claudecode.AutoSkillReranker

	// Always wire the selector so runtime settings can actually toggle it.
	// Benchmark and web-chat flows rely on this being present even when the
	// persisted config still carries historical defaults.
	ts := tools.DefaultToolSelector()
	if deps.Config.ToolCalling.SmartSelectionMaxTools > 0 {
		ts.MaxTools = deps.Config.ToolCalling.SmartSelectionMaxTools
	}
	deps.ChatHandler.SetToolSelector(ts)
	toolPolicyResolver := tools.NewToolPolicyResolver(deps.Config)
	deps.ChatHandler.SetToolPolicyResolver(toolPolicyResolver)
	deps.ChatHandler.SetToolTraceStore(tools.NewToolTraceStore(1000))
	deps.ChatHandler.SetFlagEvaluator(flagEvaluator)

	// Keep the router available by default so prompt-size reduction and
	// low-risk exposure filtering work in runtime flows without extra setup.
	toolRouter := tools.DefaultToolRouter()
	deps.ChatHandler.SetToolRouter(toolRouter)
	// Smart skill selection (progressive: rule -> IR -> optional rerank)
	if deps.Config.ToolCalling.SmartSkillSelection {
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

	// Register chat routes with optional auth so preview-mode access still works,
	// while authenticated requests carry user context for ownership/SSE-bound tools.
	chatGroup := v1.Group("")
	if deps.AuthMiddleware != nil {
		chatGroup.Use(deps.AuthMiddleware.OptionalAuthenticate())
	}
	deps.ChatHandler.RegisterRoutes(chatGroup)

	webSearchConfig := buildWebSearchConfig(deps.Config)

	// Deep research service (shared by API + skill executor)
	deepResearchService := deepresearch.NewService(nil, deepresearch.NewToolWebSearcherWithConfig(webSearchConfig))
	var harnessController *harness.Controller
	var harnessGroupDispatcher *harness.GroupDispatcher
	var harnessRuntimeObserver tools.RuntimeEventObserver
	var harnessSubagentExecutor tools.SubagentExecutor
	var harnessWriteGuard tools.WritePathGuard
	var harnessExecGuard tools.ExecPathGuard
	if deps.DB != nil && deps.Config != nil && deps.Config.Harness.Enabled {
		harnessStore, err := harness.NewSQLiteStore(deps.DB)
		if err != nil {
			logger.Warn("Failed to initialize harness store", zap.Error(err))
		} else {
			harnessController = harness.NewController(harnessStore, harness.NewPolicyResolver(deps.Config.Harness, &deps.Config.Agents))
			harnessGroupDispatcher = harness.NewGroupDispatcher(harnessController)
			harnessRuntimeObserver = harness.NewRuntimeObserver(harnessController)
			harnessSubagentExecutor = harness.NewSubagentExecutor(harnessController, &deps.Config.Agents)
			harnessWriteGuard = harness.NewWritePathGuard(harnessController)
			harnessExecGuard = harness.NewExecPathGuard(harnessController)
		}
	}
	deepResearchService.SetRoutePolicy(deepresearch.RoutePolicy{
		DefaultMode:     deepresearch.RouteMode(deps.Config.Research.Router.DefaultMode),
		AllowExperiment: deps.Config.Research.Router.AllowExperiment,
		AllowHybrid:     deps.Config.Research.Router.AllowHybrid,
	})
	if deps.Config.Research.Autoresearch.Enabled {
		backend, err := deepresearch.NewAutoresearchBackend(deepresearch.AutoresearchBackendConfig{
			Command:     deps.Config.Research.Autoresearch.Command,
			Args:        deps.Config.Research.Autoresearch.Args,
			WorkingDir:  deps.Config.Research.Autoresearch.WorkingDir,
			Timeout:     deps.Config.Research.Autoresearch.Timeout,
			ArtifactDir: deps.Config.Research.Autoresearch.ArtifactDir,
			Env:         deps.Config.Research.Autoresearch.Env,
		})
		if err != nil {
			logger.Warn("Failed to initialize autoresearch backend", zap.Error(err))
		} else {
			deepResearchService.SetExperimentBackend(backend)
			logger.Info("Autoresearch backend enabled for deep research")
		}
	}
	deps.ChatHandler.SetDeepResearchService(deepResearchService)
	if harnessController != nil {
		researchDriver := harnessdrivers.NewResearchDriver(deepResearchService, harnessController)
		if deps.SSEBroker != nil {
			researchDriver.SetNextPublisher(deps.SSEBroker)
		}
		deepResearchService.SetEventPublisher(researchDriver)
		harnessController.RegisterDriver(researchDriver)
	} else if deps.SSEBroker != nil {
		deepResearchService.SetEventPublisher(deps.SSEBroker)
	}
	tools.RegisterResearchTools(s.ToolRegistry, newDeepResearchToolAdapter(deepResearchService, harnessController, workspaceDir))
	if deps.DB != nil {
		emailService, err := tools.NewLocalEmailService(deps.DB)
		if err != nil {
			logger.Warn("Failed to initialize local email tool", zap.Error(err))
		} else {
			emailTool := tools.RegisterEmailTool(s.ToolRegistry, emailService)
			if sk := s.SkillRegistry.Get("email"); sk != nil {
				if emailSkill, ok := sk.(*builtin.Email); ok {
					emailSkill.SetExecutor(emailTool)
				}
			}
		}
		calendarService, err := tools.NewLocalCalendarService(deps.DB)
		if err != nil {
			logger.Warn("Failed to initialize local calendar tool", zap.Error(err))
		} else {
			calendarTool := tools.RegisterCalendarTool(s.ToolRegistry, calendarService)
			if calendarTool != nil && emailService != nil {
				calendarTool.SetEmailService(emailService)
			}
			if sk := s.SkillRegistry.Get("calendar"); sk != nil {
				if calendarSkill, ok := sk.(*builtin.Calendar); ok {
					calendarSkill.SetExecutor(calendarTool)
				}
			}
		}
	}

	// Register auto-reply routes
	if deps.AutoreplyHandler != nil {
		deps.AutoreplyHandler.RegisterRoutes(authPageV1Group(permission.PageChannels))
	}

	// Network routes
	networkHandler := networkapi.NewNetworkHandler(cfg.Port)
	networkHandler.RegisterGroupRoutes(authPageV1Group(permission.PageSettings))
	// Add LAN addresses to CORS allowed origins after actual port is known
	server.OnServerStart(func(port int) {
		h := networkapi.NewNetworkHandler(port)
		if err := h.InitializeCORSOrigins(); err != nil {
			logger.Warn("Failed to initialize CORS origins from network addresses", zap.Error(err))
		}
	})

	// Link preview routes
	linkPreviewHandler := networkapi.NewLinkPreviewHandler()
	linkPreviewHandler.RegisterRoutes(authPageV1Group(permission.PageChat))

	// Metrics routes
	if deps.MetricsCollector != nil {
		metricsHandler := server.NewMetricsHandler(deps.MetricsCollector)
		metricsHandler.RegisterRoutes(authPageV1Group(permission.PageHome))
	}
	if deps.MetricsWriter != nil {
		detailedMetricsHandler := metrics.NewHandler(deps.MetricsWriter)
		metricsGroup := v1.Group("/metrics", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageHome))
		detailedMetricsHandler.RegisterRoutes(metricsGroup)

		// Set metrics recorder on chat handler
		deps.ChatHandler.SetMetricsRecorder(deps.MetricsWriter)
	}
	if deps.MetricsCollector == nil && deps.MetricsWriter == nil {
		stub := featureDisabled("metrics")
		metricsGroup := v1.Group("/metrics", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageHome))
		metricsGroup.GET("/summary", stub)
		metricsGroup.GET("/all", stub)
		metricsGroup.Any("/*", stub)
	}

	// System routes
	systemHandler := server.NewSystemHandler(cfg.Version, cfg.BuildTime, cfg.GitCommit, cfg.DataDir)
	systemHomeGroup := v1.Group("/system", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageHome))
	systemHomeGroup.GET("/info", systemHandler.GetInfo)
	systemSettingsGroup := v1.Group("/system", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageSettings))
	systemSettingsGroup.GET("/config", systemHandler.GetConfig)
	systemSettingsGroup.PUT("/config", systemHandler.UpdateConfig)
	systemSettingsGroup.POST("/restart", systemHandler.RestartService)
	systemSecurityGroup := v1.Group("/system", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageSecurity))
	systemSecurityGroup.GET("/logs", systemHandler.GetLogs)
	systemSecurityGroup.POST("/logs", systemHandler.WriteLog)
	systemSecurityGroup.GET("/certificate", systemHandler.GetCertificate)
	systemHandler.RegisterFileBridgeRoutes(protected.Group("", requirePagePermission(permission.PageChat)))

	serviceHandler := server.NewServiceHandler()
	serviceHandler.RegisterRoutes(authPageV1Group(permission.PageSettings))

	// Connection monitoring routes
	connHandler := connection.NewHandler(connManager)
	connGroup := protected.Group("/connections", requirePagePermission(permission.PageSecurity))
	connHandler.RegisterRoutes(connGroup)

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

	// Initialize the authoritative SQLite-backed skills marketplace.
	if deps.Config == nil || deps.Config.SkillMarket.Enabled {
		marketCfg := skillmarket.DefaultConfig(cfg.DataDir, skillsDir)
		marketCfg.GitHubToken = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
		if deps.Config != nil {
			marketCfg.SeedURLs = append([]string{}, deps.Config.SkillMarket.SeedURLs...)
			marketCfg.ClawHubMirrorBaseURLs = append([]string{}, deps.Config.SkillMarket.ClawHubMirrorBaseURLs...)
			marketCfg.TencentSkillHubAPIBaseURL = deps.Config.SkillMarket.TencentSkillHubAPIBaseURL
			marketCfg.SkillHubBaseURL = deps.Config.SkillMarket.SkillHubBaseURL
			marketCfg.SkillHubAPIKey = strings.TrimSpace(deps.Config.SkillMarket.SkillHubAPIKey)
			marketCfg.SkillStackBaseURL = deps.Config.SkillMarket.SkillStackBaseURL
			marketCfg.SkillsMPBaseURL = deps.Config.SkillMarket.SkillsMPBaseURL
			marketCfg.SkillsMPAPIKey = strings.TrimSpace(deps.Config.SkillMarket.SkillsMPAPIKey)
			marketCfg.LLMSkillsBaseURL = deps.Config.SkillMarket.LLMSkillsBaseURL
			marketCfg.CuratedConfigPath = deps.Config.SkillMarket.CuratedConfigPath
			marketCfg.CuratedConfigURLs = append([]string{}, deps.Config.SkillMarket.CuratedConfigURLs...)
			marketCfg.CrawlIncrementalInterval = deps.Config.SkillMarket.CrawlIncrementalInterval
			marketCfg.CrawlFullInterval = deps.Config.SkillMarket.CrawlFullInterval
			marketCfg.UpdateCheckInterval = deps.Config.SkillMarket.UpdateCheckInterval
			marketCfg.TelemetryRollupInterval = deps.Config.SkillMarket.TelemetryRollupInterval
			marketCfg.SemanticRatio = deps.Config.SkillMarket.SemanticRatio
			marketCfg.SearchCandidateLimit = deps.Config.SkillMarket.SearchCandidateLimit
		}

		var marketEmbedding embedding.Provider
		if deps.Config != nil && deps.Config.Embedding.Enabled && strings.EqualFold(deps.Config.Embedding.Provider, "cybertron") {
			provider := embedding.NewCybertronProvider(embedding.CybertronConfig{
				ModelsDir:  embedding.PrepareSharedModelCache(cfg.DataDir, deps.Config.Memory.VectorStore.DBPath, deps.Config.Embedding.Model),
				Model:      deps.Config.Embedding.Model,
				Dimensions: deps.Config.Embedding.Dimensions,
				Timeout:    deps.Config.Embedding.Timeout,
			})
			provider.StartAsync()
			marketEmbedding = provider
		}
		market, err := skillmarket.NewService(deps.DB, skillmarket.Options{
			Config:            marketCfg,
			Logger:            logger,
			Registry:          s.SkillRegistry,
			LocalScanner:      localScanner,
			EmbeddingProvider: marketEmbedding,
			HTTPClient:        &http.Client{Timeout: 30 * time.Second},
			// Keep LLM classification optional; deterministic scanning is always available.
			Scanner: skillmarket.NewScanner(nil),
		})
		if err != nil {
			logger.Warn("Failed to initialize skill marketplace", zap.Error(err))
		} else {
			skillHandler.SetMarketplace(market)
			market.Start(deps.Ctx)
			logger.Info("Skill marketplace initialized")
		}
	}

	skillHandler.RegisterRoutes(authPageV1Group(permission.PageSkills))

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
	if deps.CronHandler != nil {
		if deps.WorkflowHandler != nil {
			tools.RegisterLazyNodesTool(s.ToolRegistry, deps.WorkflowHandler.GetService)
		}
		cronAdapter := cron.NewSkillAdapter(deps.CronHandler.GetService)
		if sk := s.SkillRegistry.Get("scheduler"); sk != nil {
			if ss, ok := sk.(*builtin.Scheduler); ok {
				ss.SetCronService(cronAdapter)
			}
		}
		cronTools := cronToolAdapter{resolve: deps.CronHandler.GetService}
		tools.RegisterCronTool(s.ToolRegistry, cronTools)
		if calendarTool := tools.GetCalendarTool(s.ToolRegistry); calendarTool != nil {
			calendarTool.SetCronService(cronTools)
		}
		deps.CronHandler.SetServiceInitHook(func(svc *cron.Service) {
			svc.SetMessageInjector(inject.NewMemoryStoreInjector(s.MemoryStore))
			if deps.SSEBroker != nil {
				svc.SetEventPublisher(deps.SSEBroker)
			}
			svc.RegisterCommandHandler(cron.CommandSecurityConfig{
				Enabled:          true,
				RequireAdminRole: true,
			})
			registerDeepResearchCronHandler(svc, deepResearchService, logger)
		})
	}
	// Browser + UI reviewer: wire backends for IPC and LLM tool use.
	if deps.LazyBrowserSvc != nil {
		uiTool := &tools.UIReviewerTool{}
		uiTool.SetBrowser(tools.NewLazyRodBrowserAdapter(deps.LazyBrowserSvc))
		uiTool.SetMediaDir(mediaDir)
		deps.UIReviewerTool = uiTool
	}
	if deps.BrowserBackend != nil {
		tools.RegisterBrowserTool(s.ToolRegistry, deps.BrowserBackend)
		if browserTool := tools.GetBrowserTool(s.ToolRegistry); browserTool != nil {
			browserTool.SetMediaDir(mediaDir)
		}
		if webFetchTool := tools.GetWebFetchTool(s.ToolRegistry); webFetchTool != nil {
			webFetchTool.SetBrowser(deps.BrowserBackend)
		}
		if webReadTool := tools.GetWebReadTool(s.ToolRegistry); webReadTool != nil {
			webReadTool.SetBrowser(deps.BrowserBackend)
		}
		if webExtractTool := tools.GetWebExtractTool(s.ToolRegistry); webExtractTool != nil {
			webExtractTool.SetBrowser(deps.BrowserBackend)
		}
	}
	// Wire browser backend into browser skill
	if deps.BrowserBackend != nil {
		if sk := s.SkillRegistry.Get("browser"); sk != nil {
			if br, ok := sk.(*builtin.Browser); ok {
				br.SetBrowserService(newLazyBrowserSkillAdapter(deps.LazyBrowserSvc))
				br.SetMediaDir(mediaDir)
			}
		}
	}
	// Wire browser backend into ui_reviewer skill
	if deps.LazyBrowserSvc != nil {
		if sk := s.SkillRegistry.Get("ui_reviewer"); sk != nil {
			if ur, ok := sk.(*builtin.UIReviewer); ok {
				ur.SetBrowserService(newLazyBrowserSkillAdapter(deps.LazyBrowserSvc))
			}
		}
	}

	// Wire push notification service into reminder skill
	if deps.PushService != nil {
		pushTools := push.NewToolsAdapter(func() *push.Service { return deps.PushService })
		tools.RegisterPushTool(s.ToolRegistry, pushTools)
		tools.RegisterMessageTool(s.ToolRegistry, pushTools)
		if calendarTool := tools.GetCalendarTool(s.ToolRegistry); calendarTool != nil {
			calendarTool.SetReminderService(pushTools)
		}
		if sk := s.SkillRegistry.Get("reminder"); sk != nil {
			if r, ok := sk.(*builtin.Reminder); ok {
				r.SetPushService(push.NewSkillAdapter(func() *push.Service { return deps.PushService }))
			}
		}
	}

	// Register native image tool backed by mediagen + ui_reviewer.
	tools.RegisterImageTool(s.ToolRegistry, deps.UIReviewerTool, newImageGenerateAdapter(deps.MediaManager), newImageTaskLookupAdapter(deps.MediaManager))
	pptSvc := newPPTService(deps.MediaManager, deps.MediaStorage, deps.UIReviewerTool)
	tools.RegisterPPTTool(s.ToolRegistry, pptSvc)
	if tool := s.ToolRegistry.Get("image"); tool != nil {
		if imageTool, ok := tool.(*tools.ImageTool); ok {
			imageTool.SetOCRService(newImageOCRAdapter(s.OCRService))
			imageTool.SetPPTService(pptSvc)
		}
	}

	if deps.SpeechHandler != nil || deps.VoiceHandler != nil {
		var speechSvc speech.Service
		if deps.SpeechHandler != nil {
			speechSvc = deps.SpeechHandler.Service()
		}
		var voiceSvc voice.Service
		if deps.VoiceHandler != nil {
			voiceSvc = deps.VoiceHandler.Service()
		}
		tools.RegisterTTSTool(s.ToolRegistry, ttsToolAdapter{speech: speechSvc, voice: voiceSvc})
	}

	// Analyze: skill-only, executed via `blue analyze`.
	// AnalyzeTool does the heavy lifting; wired into the Analyze skill.
	{
		analyzeTool := tools.RegisterAnalyzeTool(s.ToolRegistry, mediaDir)
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
			ws.SetSearcher(tools.NewWebSearchTool(webSearchConfig))
		}
	}

	// Deep research: wire service-backed executor into deep_research skill.
	if sk := s.SkillRegistry.Get("deep_research"); sk != nil {
		if ds, ok := sk.(*builtin.DeepResearch); ok {
			ds.SetExecutor(deepresearch.NewSkillExecutor(deepResearchService))
		}
	}

	var reflectService *selfreflect.Service

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
	browserCheckpointMgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	var browserSiteStore *tools.BrowserSiteAllowlistStore
	if deps.DB != nil {
		var err error
		browserSiteStore, err = tools.NewBrowserSiteAllowlistStore(deps.DB)
		if err != nil {
			slog.Warn("failed to create browser site allowlist store", "error", err)
		}
	}
	if deps.ChatHandler != nil {
		deps.ChatHandler.SetMediaDir(mediaDir)
		deps.ChatHandler.SetQuestionManager(questionMgr)
		deps.ChatHandler.SetBrowserCheckpointManager(browserCheckpointMgr)
		deps.ChatHandler.SetBrowserSiteAllowlistStore(browserSiteStore)
		if harnessRuntimeObserver != nil {
			deps.ChatHandler.SetToolEventObserver(harnessRuntimeObserver)
		}
	}
	if questionMgr != nil && harnessRuntimeObserver != nil {
		questionMgr.SetObserver(harnessRuntimeObserver)
	}

	// Exec tools (shell execution + process management)
	var convertHandler *convertsvc.Handler
	var execApprovals *tools.ApprovalManager
	{
		execConfig := tools.DefaultExecConfig()
		execConfig.DataDir = cfg.DataDir
		// Keep exec aligned with the runtime workspace so no-workdir commands
		// default into the same root the prompt and file tools advertise.
		execConfig.AllowedDirs = workspaceAllowedPaths
		if deps.SSEBroker != nil {
			execApprovals = tools.NewApprovalManager(deps.SSEBroker)
			if harnessRuntimeObserver != nil {
				execApprovals.SetObserver(harnessRuntimeObserver)
			}
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
		tools.RegisterApprovalAwareFileTools(
			s.ToolRegistry,
			workspaceAllowedPaths,
			0,
			execApprovals,
			dirStore,
		)
		convertService, err := convertsvc.NewService(s.DB, cfg.DataDir)
		if err != nil {
			logger.Warn("Failed to initialize convert service", zap.Error(err))
		} else {
			deps.Closers = append(deps.Closers, convertService)
			convertHandler = convertsvc.NewHandler(convertService, func(ctx context.Context, userID, conversationID string) error {
				conv, err := s.MemoryStore.GetConversation(ctx, conversationID, userID)
				if err != nil {
					if err == memory.ErrNotFound {
						return echo.NewHTTPError(http.StatusNotFound, "conversation not found")
					}
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
				}
				_ = conv
				return nil
			})
			if deps.ChatHandler != nil {
				deps.ChatHandler.SetConvertSourceProvider(convertService)
			}
			tools.RegisterConvertTool(
				s.ToolRegistry,
				convertService,
				execApprovals,
				dirStore,
				workspaceAllowedPaths,
			)
		}

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
					opts.EnableRerank = sh.GetEffectiveSkillRerankEnabled()
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

		execGroup := v1.Group("/exec", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageChat))

		// Exec approval REST endpoint (kept for backwards compatibility;
		// the unified /approval/resolve endpoint also handles exec approvals).
		if execApprovals != nil {
			execGroup.GET("/approvals/pending", func(c echo.Context) error {
				userID := resolveRequestUserID(c)
				req := execApprovals.GetPending(userID)
				if req == nil {
					if sessionID := strings.TrimSpace(c.QueryParam("session_id")); sessionID != "" {
						req = execApprovals.GetPendingBySession(sessionID)
					}
				}
				if req == nil {
					return c.JSON(200, map[string]interface{}{"pending": false})
				}
				return c.JSON(200, map[string]interface{}{"pending": true, "approval": req})
			})
			execGroup.POST("/approvals/:id", func(c echo.Context) error {
				id := c.Param("id")
				var body struct {
					Decision    string `json:"decision"`
					BindingHash string `json:"binding_hash"`
				}
				if err := c.Bind(&body); err != nil {
					return c.JSON(400, map[string]string{"error": "invalid body"})
				}
				decision := tools.ApprovalDecision(body.Decision)
				if decision != tools.ApprovalAllowOnce && decision != tools.ApprovalAllowAlways && decision != tools.ApprovalDeny {
					return c.JSON(400, map[string]string{"error": "invalid decision; use allow-once, allow-always, or deny"})
				}
				if !execApprovals.ResolveApprovalWithBinding(id, decision, body.BindingHash) {
					return c.JSON(404, map[string]string{"error": "approval not found or expired"})
				}
				return c.JSON(200, map[string]string{"status": string(decision)})
			})
		}

		// Persistent approved directories management.
		if dirStore != nil {
			execGroup.GET("/approvals/directories", func(c echo.Context) error {
				userID := resolveRequestUserID(c)
				entries, err := dirStore.List()
				if err != nil {
					return c.JSON(500, map[string]string{"error": "failed to load approved directories"})
				}

				type directoryEntry struct {
					ID         string `json:"id"`
					Path       string `json:"path"`
					AddedAt    string `json:"added_at"`
					LastUsed   string `json:"last_used"`
					ApprovedBy string `json:"approved_by,omitempty"`
				}
				out := make([]directoryEntry, 0, len(entries))
				for _, entry := range entries {
					approvedBy := strings.TrimSpace(entry.ApprovedBy)
					if approvedBy != "" && userID != "default" && userID != approvedBy {
						continue
					}
					out = append(out, directoryEntry{
						ID:         entry.ID,
						Path:       entry.Path,
						AddedAt:    entry.AddedAt.UTC().Format(time.RFC3339),
						LastUsed:   entry.LastUsed.UTC().Format(time.RFC3339),
						ApprovedBy: approvedBy,
					})
				}

				return c.JSON(200, map[string]interface{}{
					"entries": out,
				})
			})

			execGroup.DELETE("/approvals/directories/:id", func(c echo.Context) error {
				id := strings.TrimSpace(c.Param("id"))
				if id == "" {
					return c.JSON(400, map[string]string{"error": "directory id is required"})
				}

				userID := resolveRequestUserID(c)
				entries, err := dirStore.List()
				if err != nil {
					return c.JSON(500, map[string]string{"error": "failed to load approved directories"})
				}
				found := false
				for _, entry := range entries {
					if entry.ID != id {
						continue
					}
					found = true
					approvedBy := strings.TrimSpace(entry.ApprovedBy)
					if approvedBy != "" && userID != "default" && userID != approvedBy {
						return c.JSON(403, map[string]string{"error": "forbidden"})
					}
					break
				}
				if !found {
					return c.JSON(404, map[string]string{"error": "directory approval not found"})
				}
				if err := dirStore.Delete(id); err != nil {
					return c.JSON(500, map[string]string{"error": "failed to revoke directory approval"})
				}
				return c.JSON(200, map[string]bool{"deleted": true})
			})
		}
	}

	{
		var browserApprovalAuth echo.MiddlewareFunc
		if deps.AuthMiddleware != nil {
			browserApprovalAuth = deps.AuthMiddleware.Authenticate()
		}
		registerBrowserApprovalRoutes(v1, browserApprovalAuth, requirePagePermission(permission.PageChat), browserSiteStore)
	}

	// Ask-user-question REST endpoints
	if questionMgr != nil {
		askGroup := v1.Group("/ask-user-question", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageChat))
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

	// Management tool: register as a native tool and wire service adapters.
	// It is also used by the mgmt skill/CLI subcommand path.
	mgmtTool := tools.RegisterMgmtTool(s.ToolRegistry)
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

	registerPluginTools(s.ToolRegistry, deps.PluginRegistry)

	// Plugin routes
	pluginHandler := server.NewPluginHandler(deps.PluginRegistry)
	pluginHandler.RegisterRoutes(authPageV1Group(permission.PagePlugins))

	pluginStoreHandler := server.NewPluginStoreHandler(deps.PluginStore)
	pluginStoreHandler.RegisterRoutes(authPageV1Group(permission.PagePlugins))

	// Tool store routes
	toolStoreHandler := server.NewToolStoreHandler(s.ToolRegistry)
	toolStoreHandler.RegisterRoutes(authPageV1Group(permission.PageTools))

	// Backup routes
	if deps.BackupHandler != nil {
		deps.BackupHandler.RegisterRoutes(authPageV1Group(permission.PageSettings))
	} else {
		stub := featureDisabled("backup")
		backupGroup := v1.Group("/backup", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageSettings))
		backupGroup.GET("", stub)
		backupGroup.GET("/progress", stub)
		backupGroup.Any("/*", stub)
	}

	// Security routes (protected)
	if deps.SecurityHandler != nil {
		if deps.Config != nil {
			deps.SecurityHandler.SetScannerConfig(buildSecurityScannerConfig(deps.Config))
		}
		if deps.ConfigKV != nil {
			deps.SecurityHandler.SetKVStore(deps.ConfigKV)
		}
		deps.SecurityHandler.SetDataDir(cfg.DataDir)
		securityGroup := protected.Group("/security", requirePagePermission(permission.PageSecurity))
		deps.SecurityHandler.RegisterRoutes(securityGroup)
	} else {
		stub := featureDisabled("security")
		securityGroup := protected.Group("/security", requirePagePermission(permission.PageSecurity))
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
		sandboxGroup := protected.Group("/sandbox", requirePagePermission(permission.PageTools))
		deps.SandboxHandler.RegisterRoutes(sandboxGroup)
	} else {
		stub := featureDisabled("sandbox")
		sandboxGroup := protected.Group("/sandbox", requirePagePermission(permission.PageTools))
		sandboxGroup.GET("/info", stub)
		sandboxGroup.Any("/*", stub)
	}

	// Cron routes (protected) - /api/cron/*
	if deps.CronHandler != nil {
		deps.CronHandler.RegisterRoutes(apiProtected.Group("", requirePagePermission(permission.PageAutomation)))
	} else {
		stub := featureDisabled("cron")
		cronGroup := apiProtected.Group("/cron", requirePagePermission(permission.PageAutomation))
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
					req.Model = resolveDefaultModelForCCCLI(req.Model, deps.ClaudeCodeHandler, deps.ProviderPool)
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

	// Browser automation routes (protected) - /api/browser/*
	if deps.BrowserHandler != nil {
		browserGroup := apiProtected.Group("/browser", requirePagePermission(permission.PageTools))
		deps.BrowserHandler.RegisterRoutes(browserGroup)
	} else {
		stub := featureDisabled("browser")
		browserGroup := apiProtected.Group("/browser", requirePagePermission(permission.PageTools))
		browserGroup.GET("/tasks", stub)
		browserGroup.GET("/sessions", stub)
		browserGroup.GET("/security", stub)
		browserGroup.Any("/*", stub)
	}

	// Workflow routes
	if deps.WorkflowHandler != nil {
		deps.WorkflowHandler.SetServiceInitHook(func(svc *workflow.WorkflowService) {
			svc.SetFlagEvaluator(flagEvaluator)
		})
		deps.WorkflowHandler.SetRouteMiddlewares(deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageAutomation))
		deps.WorkflowHandler.RegisterRoutes(e)
	} else {
		stub := featureDisabled("workflow")
		wfGroup := v1.Group("/workflows", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageAutomation))
		wfGroup.GET("", stub)
		wfGroup.GET("/stats", stub)
		wfGroup.GET("/templates", stub)
		wfGroup.Any("/*", stub)
	}

	// Deep research routes (protected)
	deepResearchHandler := deepresearch.NewHandler(deepResearchService)
	if creator := newHarnessResearchCreator(harnessController, deepResearchService, workspaceDir); creator != nil {
		deepResearchHandler.SetJobCreator(creator)
	}
	deepResearchHandler.RegisterGroup(protected.Group("/deep-research", requirePagePermission(permission.PageChat)))
	deepResearchHandler.RegisterGroup(apiProtected.Group("/deep-research", requirePagePermission(permission.PageChat)))
	logger.Info("Deep research routes registered")
	var harnessDetailView *harnessDetailProvider
	if harnessController != nil {
		harnessHandler := harness.NewHandler(harnessController)
		harnessDetailView = &harnessDetailProvider{
			execApprovals: execApprovals,
			questionMgr:   questionMgr,
		}
		harnessHandler.SetDetailProvider(harnessDetailView)
		harnessHandler.RegisterRoutes(protected.Group("/harness", requirePagePermission(permission.PageTools)))
		harnessHandler.RegisterRoutes(apiProtected.Group("/harness", requirePagePermission(permission.PageTools)))
		if harnessGroupDispatcher != nil {
			go harnessGroupDispatcher.Start(context.Background())
		}
		logger.Info("Harness routes registered")
	}

	// Voice routes - /api/v1/voice/*
	if deps.VoiceHandler != nil {
		voiceGroup := v1.Group("/voice", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageChat))
		deps.VoiceHandler.RegisterRoutes(voiceGroup)
		// WebSocket handler for voice streaming (requires auth, supports token in query param)
		if deps.VoiceWSHandler != nil {
			voiceWSGroup := protected.Group("/voice", requirePagePermission(permission.PageChat))
			deps.VoiceWSHandler.RegisterRoutes(voiceWSGroup)
		}
	} else {
		stub := featureDisabled("voice")
		voiceGroup := v1.Group("/voice", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageChat))
		voiceGroup.GET("/voices", stub)
		voiceGroup.GET("/sessions", stub)
		voiceGroup.POST("/transcribe", stub)
		voiceGroup.POST("/synthesize", stub)
		voiceGroup.Any("/*", stub)
	}

	// Speech routes - /api/v1/speech/*
	if deps.SpeechHandler != nil {
		speechGroup := v1.Group("/speech", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageSettings))
		deps.SpeechHandler.RegisterRoutes(speechGroup)
	} else {
		stub := featureDisabled("speech")
		speechGroup := v1.Group("/speech", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageSettings))
		speechGroup.GET("/status", stub)
		speechGroup.GET("/models", stub)
		speechGroup.GET("/asr/status", stub)
		speechGroup.GET("/asr/models", stub)
		speechGroup.GET("/tts/status", stub)
		speechGroup.GET("/tts/models", stub)
		speechGroup.Any("/*", stub)
	}
	if convertHandler != nil {
		convertHandler.RegisterRoutes(protected.Group("/convert", requirePagePermission(permission.PageChat)))
		logger.Info("Convert routes registered")
	}

	// Form filler routes are now public (registered above in v1)

	// Companion routes
	if deps.CompanionHandler != nil {
		deps.CompanionHandler.RegisterGroupRoutes(authPageV1Group(permission.PageSecurity))
		deps.CompanionHandler.RegisterCompatGroupRoutes(authPageAPIGroup(permission.PageSecurity))
	}
	if deps.CompanionWSHandler != nil {
		deps.CompanionWSHandler.RegisterGroupRoutes(authPageV1Group(permission.PageSecurity))
		deps.CompanionWSHandler.RegisterCompatGroupRoutes(authPageAPIGroup(permission.PageSecurity))
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

		providersGroup := protected.Group("/providers", requirePagePermission(permission.PageProviders))
		providerPoolHandler.RegisterRoutes(providersGroup)
		modelsGroup := protected.Group("/models", requirePagePermission(permission.PageProviders))
		providerPoolHandler.RegisterModelRoutes(modelsGroup)
		ideGroup := protected.Group("/ide", requirePagePermission(permission.PageProviders))
		providerPoolHandler.RegisterIDERoutes(ideGroup)
		pricingGroup := protected.Group("/pricing", requirePagePermission(permission.PageProviders))
		providerPoolHandler.RegisterPricingRoutes(pricingGroup)
		configGroup := protected.Group("/config", requirePagePermission(permission.PageProviders))
		providerPoolHandler.RegisterConfigRoutes(configGroup)

		// Proxy failover routes are registered below in the proxy block
		// so they share the same FailoverConfig pointer as the actual failover handler.

		// Start provider pool background tasks (health checks, model discovery, pricing)
		server.OnServerStart(func(port int) {
			go deps.ProviderPool.Start(context.Background())
		})
	} else {
		stub := featureDisabled("providers")
		providersGroup := protected.Group("/providers", requirePagePermission(permission.PageProviders))
		providersGroup.GET("", stub)
		providersGroup.Any("/*", stub)
		modelsGroup := protected.Group("/models", requirePagePermission(permission.PageProviders))
		modelsGroup.GET("", stub)
		ideGroup := protected.Group("/ide", requirePagePermission(permission.PageProviders))
		ideGroup.GET("/scan", stub)
		ideGroup.Any("/*", stub)
		pricingGroup := protected.Group("/pricing", requirePagePermission(permission.PageProviders))
		pricingGroup.GET("", stub)
		pricingGroup.Any("/*", stub)
		failoverStub := featureDisabled("proxy_failover")
		failoverGroup := protected.Group("/proxy/failover", requirePagePermission(permission.PageProviders))
		failoverGroup.GET("/config", failoverStub)
		failoverGroup.GET("/metrics", failoverStub)
		failoverGroup.GET("/breakers", failoverStub)
		failoverGroup.Any("/*", failoverStub)
	}

	// Proxy cache routes (deprecated — cache removed)
	{
		stub := featureDisabled("proxy_cache")
		cacheGroup := v1.Group("/proxy/cache", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageProviders))
		cacheGroup.GET("/stats", stub)
		cacheGroup.GET("/config", stub)
		cacheGroup.Any("/*", stub)
	}

	// Data masking (hoisted so toggle state is accessible from proxy block)
	dataMasker := proxy.NewDataMasker(nil)
	// Load built-in default rules (enabled by default when masking is turned on)
	for _, rule := range proxy.GetDefaultRules() {
		dataMasker.AddRule(rule)
	}
	var maskingOnToggle func() // wired later when toggleStore is available
	{
		maskingGroup := v1.Group("/proxy/masking", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageSecurity))
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
		maskingGroup.PUT("/rules/:id", func(c echo.Context) error {
			id := strings.TrimSpace(c.Param("id"))
			if id == "" {
				return c.JSON(400, map[string]string{"error": "id required"})
			}

			var req struct {
				Enabled *bool `json:"enabled"`
			}
			if err := c.Bind(&req); err != nil || req.Enabled == nil {
				return c.JSON(400, map[string]string{"error": "enabled field required"})
			}

			if !dataMasker.SetRuleEnabled(id, *req.Enabled) {
				return c.JSON(404, map[string]string{"error": "rule not found"})
			}
			if maskingOnToggle != nil {
				maskingOnToggle()
			}
			rule, _ := dataMasker.GetRule(id)
			return c.JSON(200, map[string]interface{}{
				"message": "rule updated",
				"rule":    rule,
				"stats":   dataMasker.Stats(),
			})
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
	auxiliaryLLM := newAuxiliaryLLMCaller()
	agentLLMCaller := agent.LLMCaller(newProviderRegistryLLMCaller(s.LLMRegistry))
	if deps.Config.Proxy != nil && deps.Config.Proxy.Enabled {
		routingConfig := &deps.Config.Proxy.Routing
		if deps.Config.Proxy.Route != nil {
			routingConfig = deps.Config.Proxy.Route
		}
		proxyRouter := proxy.NewRouter(routingConfig)
		proxyConnPool := proxy.NewConnectionPool(&deps.Config.Proxy.Connection)
		proxyFailover := proxy.NewFailoverHandler(&routingConfig.Failover, proxyRouter)
		proxyHandler := proxy.NewProxyHandler(proxyRouter, proxyConnPool, proxyFailover)
		proxyHandler.SetResponsesIntegrationEnabled(false)
		proxyHandler.SetSTTService(deps.STTService)
		proxyHandler.SetProviderRaceConfig(routingConfig.Failover.ProviderRace)
		proxyHandler.SetPromptCacheEnabled(true) // default ON for new installs
		proxyHandler.SetDataMasker(dataMasker)

		// Smart failover handler for metrics + intelligent error classification
		smartFailover := proxy.NewSmartFailoverHandler(&routingConfig.Failover, proxyRouter)

		// Failover API routes — share the same config pointer so API changes take effect
		failoverAPIHandler := proxy.NewFailoverAPIHandler(smartFailover, &routingConfig.Failover)
		failoverGroup := protected.Group("/proxy/failover", requirePagePermission(permission.PageProviders))
		failoverAPIHandler.RegisterRoutes(failoverGroup)

		// Pipeline stats collector: unified async batch persistence for routing/failover
		// Uses MetricsWriter persistence; by default this now shares blue.db unless explicitly split
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
			// Fallback to blue.db if MetricsWriter persistence is unavailable
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
					if result != nil && (len(result.FailedAttempts) > 0 || result.SuccessProvider == "" || result.TotalAttempts > 1) {
						slog.Info("[router] failover summary",
							"request_id", result.RequestID,
							"attempts", result.TotalAttempts,
							"trace", result.Summary(),
							"success_provider", result.SuccessProvider,
							"success_model", result.SuccessModel,
							"final_error", result.FinalError,
							"duration_ms", result.EndTime.Sub(result.StartTime).Milliseconds(),
						)
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
		if deps.Config.Pruner != nil {
			prunerCfg = *deps.Config.Pruner
		}
		prunerCfg.Backend = pruner.NormalizeBackendName(prunerCfg.Backend)
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
		prunerModelDir := filepath.Join(cfg.DataDir, "models")
		prunerCfg.ModelDir = prunerModelDir // Set ModelDir for ONNX backend
		prunerModelMgr := pruner.NewPrunerModelManager(prunerModelDir)

		// Always register pruner API routes (handler returns disabled status when pruner is off)
		prunerHandler = pruner.NewAPIHandler(prunerMw, &prunerCfg, prunerModelMgr)
		prunerGroup := v1.Group("/proxy/pruner", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageProviders))
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
				PrunerBackend:      pruner.NormalizeBackendName(prunerCfg.Backend),
				RoutingEnabled:     proxyHandler.IsRoutingEnabled(),
				MaskingEnabled:     dataMasker.IsEnabled(),
				PromptCacheEnabled: proxyHandler.IsPromptCacheEnabled(),
				FailoverConfig:     &routingConfig.Failover,
				Version:            1,
			}
			if rules := dataMasker.ListRules(); len(rules) > 0 {
				state.MaskingRules = make(map[string]bool, len(rules))
				for _, r := range rules {
					if r == nil || strings.TrimSpace(r.ID) == "" {
						continue
					}
					state.MaskingRules[r.ID] = r.Enabled
				}
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
					prunerCfg.Backend = pruner.NormalizeBackendName(saved.PrunerBackend)
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
				for id, enabled := range saved.MaskingRules {
					dataMasker.SetRuleEnabled(id, enabled)
				}
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
		if deps.MetricsWriter != nil {
			bridge.SetMetricsRecorder(deps.MetricsWriter)
		}
		proxyCaller := &proxyBridgeLLMCaller{
			bridge:            bridge,
			claudeCodeHandler: deps.ClaudeCodeHandler,
			providerPool:      deps.ProviderPool,
		}
		auxiliaryLLM.SetFallback(proxyCaller)
		agentLLMCaller = proxyCaller
		deps.ChatHandler.SetProxyBridge(bridge)
		// Leave IM model empty so ChatHandler can dynamically resolve defaults:
		// prefer codex-spark when available, otherwise fall back to auto routing.
		deps.ChatHandler.SetIMModel("")

		// Wire LLM calls for voice mode through the same proxy pipeline
		if deps.VoiceHandler != nil {
			deps.VoiceHandler.Service().SetChatFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
				req.Model = resolveDefaultModelForCCCLI(req.Model, deps.ClaudeCodeHandler, deps.ProviderPool)
				return bridge.Chat(ctx, req)
			})
		}

		// Wire VLM bridge into UI reviewer tool (for IPC-based SKILL)
		if deps.UIReviewerTool != nil {
			deps.UIReviewerTool.SetVLMBridge(tools.NewProxyBridgeVLMAdapter(bridge))
		}
		// Wire VLM bridge into PDF extraction fallback for scanned pages.
		if s.PDFService != nil {
			s.PDFService.SetVisionService(pdfextract.NewProxyBridgeVisionAdapter(bridge))
		}
		if tool := s.ToolRegistry.Get("image"); tool != nil {
			if imageTool, ok := tool.(*tools.ImageTool); ok {
				imageTool.SetVisionBridge(tools.NewProxyBridgeVLMAdapter(bridge))
			}
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

		v1ProxyGroup := e.Group("/v1")
		v1ProxyGroup.Any("/chat/completions", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/completions", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/embeddings", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/models", echo.WrapHandler(proxyHandler))
		v1ProxyGroup.Any("/messages", echo.WrapHandler(proxyHandler))

		// Model routing toggle API
		routingGroup := v1.Group("/proxy/routing", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageProviders))
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
		authPageV1Group(permission.PageProviders).GET("/proxy/pipeline/stats", func(c echo.Context) error {
			if pipelineStats != nil {
				return c.JSON(200, pipelineStats.Snapshot())
			}
			return c.JSON(200, map[string]string{"status": "not configured"})
		})

		// Prompt cache toggle API
		promptCacheGroup := v1.Group("/proxy/prompt-cache", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageProviders))
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
		restrictionsHandler.RegisterRoutes(authPageV1Group(permission.PageProviders))
	}

	agentRunnerRef = registerAgentAndMCPRoutes(protected, v1, s, cfg, deps, logger, agentLLMCaller, harnessController, requirePagePermission(permission.PageTools))
	reflectService = selfreflect.NewService(auxiliaryLLM, nil)
	if sk := s.SkillRegistry.Get("self_reflect"); sk != nil {
		if sr, ok := sk.(*builtin.SelfReflect); ok {
			sr.SetExecutor(reflectService)
		}
	}
	if agentRunnerRef != nil {
		agentRunnerRef.SetReflector(reflectService)
		agentRunnerRef.SetSubagentExecutor(harnessSubagentExecutor)
		agentRunnerRef.SetWritePathGuard(harnessWriteGuard)
		agentRunnerRef.SetExecPathGuard(harnessExecGuard)
		if deps.MetricsWriter != nil {
			agentRunnerRef.SetToolMetricsRecorder(deps.MetricsWriter)
		}
	}

	// Ngrok remote access routes
	if deps.NgrokTunnelMgr != nil && deps.NgrokConfigStore != nil {
		remoteAccessHandler := networkapi.NewSDKRemoteAccessHandler(deps.NgrokTunnelMgr, deps.NgrokConfigStore, cfg.Port)
		remoteAccessHandler.SetJWTService(s.JWTService)
		remoteAccessHandler.RegisterGroupRoutes(authPageV1Group(permission.PageChannels))
	}

	// Claude Code CLI routes (protected)
	if deps.ClaudeCodeHandler != nil {
		deps.ChatHandler.SetClaudeCodeHandler(deps.ClaudeCodeHandler)
	}

	// Memory routes
	if deps.MemoryHandler != nil {
		deps.MemoryHandler.RegisterRoutes(authPageV1Group(permission.PageChat))
		// Wire layered memory into chat handler when lazy init completes
		deps.MemoryHandler.SetOnLayeredReady(func(ls *memory.LayeredMemoryService) {
			deps.ChatHandler.SetLayeredMemory(ls)
			if agentRunnerRef != nil {
				agentRunnerRef.SetMemory(newAgentMemoryAdapter(ls))
			}
			reflectService.SetMemoryWriter(newAgentReflectionMemoryWriter(ls))
		})
	} else {
		stub := featureDisabled("memory")
		memGroup := v1.Group("/memory", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageChat))
		memGroup.GET("/stats", stub)
		memGroup.GET("/backend", stub)
		memGroup.POST("/search", stub)
		memGroup.Any("/*", stub)
	}

	// Workspace routes (protected) — SOUL.md, USER.md, IDENTITY.md, etc.
	if deps.WorkspaceHandler != nil {
		workspaceGroup := protected.Group("/workspace", requirePagePermission(permission.PageChat))
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
	updateHandler.SetResumeRecoverer(buildUpdateResumeRecoverer(deps.CronHandler, logger))
	updateHandler.RegisterRoutes(authPageV1Group(permission.PageSettings))

	// Wire up OTA background checker so DownloadOTA can find packages
	otaChecker := update.NewOTAChecker(cfg.Version, cfg.DataDir, "")
	updateHandler.SetOTAChecker(otaChecker)
	go otaChecker.Run(deps.Ctx)
	updateHandler.StartAutoUpdater(deps.Ctx)

	logger.Info("OTA update routes registered", zap.Bool("enabled", deps.Config.Update.Enabled))

	// Wire up mgmt tool with upgrade/OTA service
	mgmtTool.SetUpgrade(&mgmtUpgradeAdapter{
		handler:    updateHandler,
		otaChecker: otaChecker,
		version:    cfg.Version,
	})

	// Channel config + channel manager
	if deps.ChannelConfigStore != nil {
		channelCfg := channel.DefaultConfig()
		if deps.Config != nil {
			channelCfg = deps.Config.Channels
		}
		if settings, ok := deps.ChannelConfigStore.GetSettings(); ok {
			channelCfg = server.ApplyChannelSettings(channelCfg, settings)
		}
		channelManager := channel.NewManager(channelCfg, logger)
		channelFactory := server.NewChannelFactory(deps.Logger)
		if deps.ChatHandler != nil {
			deps.ChatHandler.SetChannelSender(func(ctx context.Context, channelName string, out channel.OutgoingMessage) error {
				return channelManager.Send(ctx, channelName, out)
			})
			deps.ChatHandler.SetChannelSenderWithID(func(ctx context.Context, channelName string, out channel.OutgoingMessage) (string, error) {
				return channelManager.SendWithID(ctx, channelName, out)
			})
			deps.ChatHandler.SetChannelMessageUpdater(func(ctx context.Context, channelName string, chatID string, messageID string, out channel.OutgoingMessage) error {
				return channelManager.UpdateMessage(ctx, channelName, chatID, messageID, out)
			})
		}
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
				if err != nil {
					return nil, err
				}
				if aiResponse != "" {
					return &channel.OutgoingMessage{ChatID: msg.ChatID, Content: aiResponse}, nil
				}
			}
			return nil, nil
		})

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
		channelConfigHandler.RegisterRoutes(authPageAPIGroup(permission.PageChannels))
	} else {
		stub := featureDisabled("channels")
		api.Group("", deps.AuthMiddleware.Authenticate(), requirePagePermission(permission.PageChannels)).GET("/channels", stub)
	}

	// Provider settings routes (protected)
	providerSettingsHandler := server.NewProviderSettingsHandler(deps.ChatHandler.GetProviderRegistry(), kv)
	providerSettingsGroup := protected.Group("/providers/settings", requirePagePermission(permission.PageProviders))
	providerSettingsHandler.RegisterRoutes(providerSettingsGroup)

	// User settings routes were registered on the startup fast path; continue
	// wiring runtime integrations now that deferred init is available.
	smallRuntime := smallmodel.NewLlamaCppRuntime(smManager)
	deps.ChatHandler.SetSmallModelRuntime(smallRuntime)
	auxiliaryLLM.SetSmallModel(smallRuntime)
	memoryRefreshCfg := session.DefaultMemoryRefreshConfig()
	memoryRefreshCfg.Enabled = memoryRefreshCfg.Enabled && deps.Config.Session.Compaction.Enabled && deps.MemoryHandler != nil
	if memoryRefreshCfg.Enabled {
		sessionCompactor := session.NewSessionCompactor(auxiliaryLLM, deps.Config.Session.Compaction)
		sessionMemRefresher := server.NewSessionMemoryRefresher(deps.MemoryHandler)
		deps.ChatHandler.SetCompactorMemoryIntegration(session.NewCompactorMemoryIntegration(
			sessionCompactor,
			sessionMemRefresher,
			auxiliaryLLM,
			memoryRefreshCfg,
		), deps.Config.Session.MaxTokens)
	} else {
		deps.ChatHandler.SetCompactorMemoryIntegration(nil, deps.Config.Session.MaxTokens)
	}
	// Also make locale available to provider settings handler
	providerSettingsHandler.SetSettingsHandler(settingsHandler)
	// Wire settings into chat handler for runtime smart tool selection toggle
	deps.ChatHandler.SetSettingsHandler(settingsHandler)
	deepResearchService.SetV2Enabled(settingsHandler.GetDeepResearchV2Enabled())
	if deps.AnalyzeTool != nil {
		deps.AnalyzeTool.SetSmallModelRuntime(smallRuntime)
		deps.AnalyzeTool.SetSmallModelSwitchFuncs(
			settingsHandler.GetSmallModelEnabled,
			settingsHandler.GetSmallModelDocExtractEnabled,
		)
		deps.AnalyzeTool.SetSmallModelDocExtractToggle(settingsHandler.SetSmallModelDocExtractEnabled)
		deps.AnalyzeTool.SetSmallModelStatsRecorder(deps.ChatHandler.GetSmallModelStats())
	}
	if skillAutoReranker != nil {
		settingsHandler.SetSkillRerankerModelManager(skillAutoReranker.ModelManager())
		skillAutoReranker.SetSwitchFuncs(
			func() bool {
				rerankEnabled := deps.Config.ToolCalling.SkillRerankEnabled
				if settingsHandler.IsSkillRerankEnabledSet() {
					rerankEnabled = settingsHandler.GetSkillRerankEnabled()
				}
				if settingsHandler.GetSmallModelEnabled() && !settingsHandler.GetSmallModelRerankEnabled() {
					rerankEnabled = false
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
		agentRunnerRef.SetMaxToolRoundsPerStepFunc(settingsHandler.GetAgentLoopPolicyMaxToolRounds)
		agentRunnerRef.SetAutoReflectFunc(settingsHandler.GetAgentAutoReflect)
	}

	// User-level routes (protected) — /api/v1/my/*
	myGroup := protected.Group("/my", requirePagePermission(permission.PageProfile))

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

	// SSE event stream + tool approval endpoints were registered on the fast
	// path; complete the runtime wiring now that deferred init is ready.
	if approvalHandler != nil {
		if harnessDetailView != nil {
			harnessDetailView.approvalHandler = approvalHandler
		}
		// Wire exec approval resolver so /approval/resolve can handle exec approvals too.
		if execApprovals != nil {
			approvalHandler.SetExecResolver(execApprovalAdapter{execApprovals})
		}
		if deps.ChatHandler != nil {
			deps.ChatHandler.SetToolApprover(approvalHandler)
		}
		if agentRunnerRef != nil {
			agentRunnerRef.SetToolApprover(approvalHandler)
			if harnessRuntimeObserver != nil {
				agentRunnerRef.SetToolEventObserver(harnessRuntimeObserver)
			}
		}
		if harnessRuntimeObserver != nil {
			approvalHandler.SetObserver(harnessRuntimeObserver)
		}
		if deps.WorkflowHandler != nil && s != nil && s.ToolRegistry != nil {
			workflowGateway := tools.NewToolGateway(s.ToolRegistry, tools.NewExecutor(s.ToolRegistry))
			workflowGateway.SetApprover(approvalHandler)
			if harnessRuntimeObserver != nil {
				workflowGateway.SetEventObserver(harnessRuntimeObserver)
			}
			if deps.MetricsWriter != nil {
				workflowGateway.SetMetricsRecorder(deps.MetricsWriter)
			}
			deps.WorkflowHandler.SetServiceInitHook(func(svc *workflow.WorkflowService) {
				svc.SetToolGateway(workflowToolRuntimeAdapter{gateway: workflowGateway})
				if deps.MetricsWriter != nil {
					svc.SetMetricsRecorder(deps.MetricsWriter)
				}
			})
		}
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

func fallbackFirstProvider(chain []string) string {
	if len(chain) == 0 {
		return ""
	}
	return strings.TrimSpace(chain[0])
}

func convertFallbackPublicSpaces(spaces []config.MediaFallbackPublicSpaceConfig) []mediagen.FallbackPublicSpacePreset {
	out := make([]mediagen.FallbackPublicSpacePreset, 0, len(spaces))
	for _, preset := range spaces {
		categories := make([]mediagen.MediaCategory, 0, len(preset.Categories))
		for _, category := range preset.Categories {
			if trimmed := strings.TrimSpace(category); trimmed != "" {
				categories = append(categories, mediagen.MediaCategory(trimmed))
			}
		}
		success := make([]mediagen.FallbackResultSelector, 0, len(preset.SuccessSelectors))
		for _, selector := range preset.SuccessSelectors {
			success = append(success, mediagen.FallbackResultSelector{
				Selectors: append([]string(nil), selector.Selectors...),
				Attribute: selector.Attribute,
				Kind:      selector.Kind,
			})
		}
		out = append(out, mediagen.FallbackPublicSpacePreset{
			ID:                      preset.ID,
			DisplayName:             preset.DisplayName,
			URL:                     preset.URL,
			Categories:              categories,
			ReadySelectors:          append([]string(nil), preset.ReadySelectors...),
			PromptSelectors:         append([]string(nil), preset.PromptSelectors...),
			NegativePromptSelectors: append([]string(nil), preset.NegativePromptSelectors...),
			UploadSelectors:         append([]string(nil), preset.UploadSelectors...),
			SubmitSelectors:         append([]string(nil), preset.SubmitSelectors...),
			SuccessSelectors:        success,
			ProcessingSelectors:     append([]string(nil), preset.ProcessingSelectors...),
			ErrorSelectors:          append([]string(nil), preset.ErrorSelectors...),
			PollInterval:            preset.PollInterval,
			Timeout:                 preset.Timeout,
		})
	}
	return out
}

// execApprovalAdapter bridges tools.ApprovalManager to api.ExecApprovalResolver.
type execApprovalAdapter struct {
	mgr *tools.ApprovalManager
}

func (a execApprovalAdapter) ResolveApproval(id string, decision string, bindingHash string) networkapi.ExecApprovalResolveResult {
	switch a.mgr.ResolveApprovalWithBindingStatus(id, tools.ApprovalDecision(decision), bindingHash) {
	case tools.ApprovalResolveSuccess:
		return networkapi.ExecApprovalResolveResult{Resolved: true}
	case tools.ApprovalResolveBindingMismatch:
		return networkapi.ExecApprovalResolveResult{BindingMismatch: true}
	default:
		return networkapi.ExecApprovalResolveResult{}
	}
}

type workflowToolRuntimeAdapter struct {
	gateway *tools.ToolGateway
}

func (a workflowToolRuntimeAdapter) Execute(ctx context.Context, req workflow.ToolExecutionRequest) (*workflow.ToolExecutionResult, error) {
	if a.gateway == nil {
		return nil, fmt.Errorf("workflow tool gateway is not configured")
	}
	argsJSON, _ := json.Marshal(req.Arguments)
	result, err := a.gateway.Execute(tools.WithRouteKind(ctx, tools.ToolRouteKindWorkflow), tools.ToolGatewayRequest{
		ToolName:   strings.TrimSpace(req.ToolName),
		Arguments:  string(argsJSON),
		RouteKind:  tools.ToolRouteKindWorkflow,
		SessionID:  strings.TrimSpace(req.SessionID),
		UserID:     strings.TrimSpace(req.UserID),
		Provider:   strings.TrimSpace(req.Provider),
		ProviderID: strings.TrimSpace(req.ProviderID),
		Model:      strings.TrimSpace(req.Model),
		AgentID:    strings.TrimSpace(req.AgentID),
	})
	if err != nil {
		return nil, err
	}
	return &workflow.ToolExecutionResult{
		ExecutionResult: result.ExecutionResult,
		CompactPayload:  result.CompactLLMPayload,
		AuditPayload:    result.AuditPayload,
	}, nil
}

type gatewayStopper struct {
	gateway *gateway.Gateway
}

type pluginToolAdapter struct {
	registry *plugin.Registry
	name     string
}

func (t *pluginToolAdapter) Definition() tools.ToolDefinition {
	if t == nil || t.registry == nil {
		return tools.ToolDefinition{Name: strings.TrimSpace(t.name)}
	}
	tool := t.registry.GetTool(strings.TrimSpace(t.name))
	if tool == nil {
		return tools.ToolDefinition{Name: strings.TrimSpace(t.name)}
	}
	return tools.ToolDefinition{
		Name:                strings.TrimSpace(tool.Name),
		Description:         strings.TrimSpace(tool.Description),
		Icon:                "extension",
		Parameters:          tool.Parameters,
		RiskLevel:           strings.TrimSpace(tool.RiskLevel),
		VisibilityAllowlist: append([]string(nil), tool.VisibilityAllowlist...),
	}
}

func (t *pluginToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.registry == nil {
		return nil, fmt.Errorf("plugin tool registry is not configured")
	}
	tool := t.registry.GetTool(strings.TrimSpace(t.name))
	if tool == nil || tool.Handler == nil {
		return nil, fmt.Errorf("plugin tool %q is not available", strings.TrimSpace(t.name))
	}
	result, err := tool.Handler(ctx, args)
	if err != nil || result == nil {
		return result, err
	}
	if _, marshalErr := json.Marshal(result); marshalErr != nil {
		slog.Warn("plugin tool result required safe normalization",
			"tool", strings.TrimSpace(t.name),
			"cause", marshalErr.Error(),
		)
		return tools.SafeToolPayloadValue(result, 64*1024), nil
	}
	return result, nil
}

func registerPluginTools(toolRegistry *tools.Registry, pluginRegistry *plugin.Registry) {
	if toolRegistry == nil || pluginRegistry == nil {
		return
	}
	for _, tool := range pluginRegistry.ListTools() {
		if tool == nil || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		if toolRegistry.Get(tool.Name) != nil {
			continue
		}
		toolRegistry.Register(&pluginToolAdapter{
			registry: pluginRegistry,
			name:     tool.Name,
		})
	}
}

func (g gatewayStopper) Close() error {
	if g.gateway != nil {
		g.gateway.Stop()
	}
	return nil
}

func registerGatewayMethods(gw *gateway.Gateway, deps *RoutesDeps) {
	if gw == nil || deps == nil {
		return
	}
	browserGatewayTool := tools.NewBrowserTool()
	if deps.BrowserBackend != nil {
		browserGatewayTool.SetBackend(deps.BrowserBackend)
	}
	if deps.ServerConfig != nil && strings.TrimSpace(deps.ServerConfig.DataDir) != "" {
		browserGatewayTool.SetMediaDir(filepath.Join(deps.ServerConfig.DataDir, "media"))
	}

	gw.RegisterHandler("chat.send", func(ctx context.Context, conn *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if deps.ChatHandler == nil {
			return nil, fmt.Errorf("chat handler not configured")
		}
		var req struct {
			ConversationID string `json:"conversation_id"`
			Content        string `json:"content"`
		}
		if err := decodeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		respContent, err := deps.ChatHandler.GatewaySend(ctx, req.ConversationID, req.Content, conn.UserID)
		if err != nil {
			return nil, err
		}
		return gatewayMessageWithPayload(map[string]interface{}{
			"conversation_id": req.ConversationID,
			"content":         respContent,
		})
	})

	gw.RegisterHandler("chat.abort", func(_ context.Context, _ *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if deps.ChatHandler == nil {
			return nil, fmt.Errorf("chat handler not configured")
		}
		var req struct {
			ConversationID string `json:"conversation_id"`
		}
		if err := decodeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		streamID, cancelled := deps.ChatHandler.CancelConversationStream(req.ConversationID)
		return gatewayMessageWithPayload(map[string]interface{}{
			"conversation_id": req.ConversationID,
			"stream_id":       streamID,
			"cancelled":       cancelled,
		})
	})

	gw.RegisterHandler("browser.request", func(ctx context.Context, _ *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if deps.BrowserBackend == nil {
			return nil, fmt.Errorf("browser backend not configured")
		}
		var req struct {
			Action string                 `json:"action"`
			Params map[string]interface{} `json:"params"`
		}
		if err := decodeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		action := strings.TrimSpace(req.Action)
		if action == "" {
			return nil, fmt.Errorf("action is required")
		}

		args := make(map[string]interface{}, len(req.Params)+1)
		args["action"] = action
		for k, v := range req.Params {
			args[k] = v
		}

		result, err := browserGatewayTool.Execute(ctx, args)
		if err != nil {
			return nil, err
		}
		return gatewayMessageWithPayload(map[string]interface{}{
			"action": strings.ToLower(action),
			"result": result,
		})
	})

	gw.RegisterHandler("hooks.wake", func(ctx context.Context, _ *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if deps.PluginRegistry == nil {
			return nil, fmt.Errorf("plugin registry not configured")
		}
		var req struct {
			HookID  string      `json:"hook_id"`
			Payload interface{} `json:"payload"`
		}
		if err := decodeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.HookID) == "" {
			return nil, fmt.Errorf("hook_id is required")
		}
		if err := deps.PluginRegistry.TriggerHook(ctx, req.HookID, req.Payload); err != nil {
			return nil, err
		}
		return gatewayMessageWithPayload(map[string]interface{}{
			"hook_id": req.HookID,
			"woke":    true,
		})
	})
}

func decodeGatewayPayload(msg *gateway.Message, out interface{}) error {
	if msg == nil {
		return fmt.Errorf("nil message")
	}
	if out == nil {
		return fmt.Errorf("nil payload target")
	}
	if len(msg.Payload) == 0 {
		return fmt.Errorf("payload is required")
	}
	if err := json.Unmarshal(msg.Payload, out); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return nil
}

func gatewayMessageWithPayload(payload interface{}) (*gateway.Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &gateway.Message{Payload: raw}, nil
}

// InitMetrics initializes metrics services.
func InitMetrics(dataDir string, sharedDB *sql.DB) (*metrics.Collector, *metrics.MetricsWriter) {
	metricsCollector := metrics.NewCollector(5*time.Second, 120)
	metricsCollector.Start()

	metricsConfig := metrics.DefaultWriterConfig()
	if sharedDB != nil {
		metricsConfig.SharedSQLiteDB = sharedDB
	} else {
		metricsConfig.SQLiteDBPath = filepath.Join(dataDir, "metrics.db")
	}
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

type agentReflectionMemoryWriter struct {
	svc *memory.LayeredMemoryService
}

func newAgentReflectionMemoryWriter(svc *memory.LayeredMemoryService) *agentReflectionMemoryWriter {
	return &agentReflectionMemoryWriter{svc: svc}
}

func (a *agentReflectionMemoryWriter) Write(ctx context.Context, content string, tags []string) error {
	if a == nil || a.svc == nil {
		return nil
	}
	return a.svc.AppendToDaily(ctx, content, tags)
}
