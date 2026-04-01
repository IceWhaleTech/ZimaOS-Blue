package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
)

func (runtime routeToolRuntimeBinding) activateAgentSurface(
	protected *echo.Group,
	deps *RoutesDeps,
	logger *zap.Logger,
	agentLLMCaller agent.LLMCaller,
	harnessRuntime *HarnessRuntimeBundle,
	reflectService *selfreflect.Service,
	skillRegistry *skillpkg.Registry,
) *agent.Runner {
	runner := registerRuntimeAgentRoutes(protected, runtime, deps, logger, agentLLMCaller, harnessRuntime)
	if reflectService != nil && skillRegistry != nil {
		if sk := skillRegistry.Get("self_reflect"); sk != nil {
			if sr, ok := sk.(*builtin.SelfReflect); ok {
				sr.SetExecutor(reflectService)
			}
		}
	}
	var metricsRecorder interface {
		RecordCounter(name string, value int64, tags map[string]string)
	}
	if deps != nil {
		metricsRecorder = deps.MetricsWriter
	}
	bindAgentRuntimeSupport(runner, reflectService, metricsRecorder)
	return runner
}

func registerRuntimeAgentRoutes(
	protected *echo.Group,
	runtime routeToolRuntimeBinding,
	deps *RoutesDeps,
	logger *zap.Logger,
	agentLLMCaller agent.LLMCaller,
	harnessRuntime *HarnessRuntimeBundle,
) *agent.Runner {
	if protected == nil || deps == nil || logger == nil {
		return nil
	}
	writeDB := deps.DB
	readDB := deps.DB
	if deps.Services != nil && deps.Services.DBConn != nil {
		if deps.Services.DBConn.Writer != nil {
			writeDB = deps.Services.DBConn.Writer
		}
		if deps.Services.DBConn.Reader != nil {
			readDB = deps.Services.DBConn.Reader
		}
	}
	return registerHarnessRuntimeAgentFeatureWithReadDB(
		harnessRuntime,
		protected.Group("/agent"),
		writeDB,
		readDB,
		agentLLMCaller,
		runtime.registry,
		runtime.executor,
		deps.SSEBroker,
		runtime.workspaceDir,
		logger,
		featureDisabled("agent"),
	)
}
