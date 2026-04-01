package bootstrap

import (
	"context"
	"database/sql"
	"io/fs"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// RoutesDeps holds all dependencies needed for route registration
type RoutesDeps struct {
	DB     *sql.DB
	Config *config.Config
	// DisablePromptGuard disables chat prompt interception for controlled runs.
	DisablePromptGuard bool
	ServerConfig       *ServerConfig
	Services           *Services
	Logger             *zap.Logger
	Ctx                context.Context
	MetricsWriter      *metrics.MetricsWriter
	MetricsCollector   *metrics.Collector
	FlagEvaluator      *config.FlagEvaluator
	ChatHandler        *server.ChatHandler
	PluginRegistry     *plugin.Registry
	PluginStore        *plugin.Store
	ExtauthService     extauth.Service
	ExtauthHandler     *extauth.Handler
	AutoreplyService   *autoreply.Service
	AutoreplyHandler   *autoreply.Handler
	AuthMiddleware     *auth.AuthMiddleware
	APIKeyHandler      *auth.APIKeyHandler
	UserHandler        *user.Handler
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
	// RegisterIPCExtensions allows cmd/blue to add main-process-only CLI IPC
	// handlers after the shared runtime IPC surface is initialized.
	RegisterIPCExtensions func(*sockipc.Server)

	// Consolidated init deps (previously only in cmd/blue/main.go)
	SkillEmbedFS              fs.FS            // embedded SKILL.md filesystem for ReleaseSkills
	SandboxManager            *sandbox.Manager // for sandbox skill wiring
	SystemPromptBuilder       *agentcore.SystemPromptBuilder
	LazyBrowserSvc            func() *browser.RodService // for UI reviewer lazy adapter
	AcquireBrowserSvc         func() (*browser.RodService, func(), error)
	AcquireFallbackBrowserSvc func() (*browser.RodService, func(), error)
	LightpandaShimSvc         *browser.LightpandaService
	BrowserBackend            tools.BrowserBackend // for browser tool + IPC

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
