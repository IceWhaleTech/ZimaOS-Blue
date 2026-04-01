package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

type routeToolRuntimeBinding struct {
	registry     *tools.Registry
	executor     *tools.Executor
	workspaceDir string
}

func newRouteToolRuntimeBinding(services *Services, cfg *ServerConfig, deps *RoutesDeps) routeToolRuntimeBinding {
	var registry *tools.Registry
	if services != nil {
		registry = services.ToolRegistry
	}
	if registry == nil {
		registry = tools.NewRegistry()
	}

	executor := tools.NewExecutor(registry)
	if deps != nil && deps.ChatHandler != nil {
		executor.SetTraceStore(deps.ChatHandler.GetToolTraceStore())
	}

	workspaceDir := "."
	if cfg != nil {
		workspaceDir = ResolveWorkspaceDir(cfg.DataDir, nil)
		if deps != nil {
			workspaceDir = ResolveWorkspaceDir(cfg.DataDir, deps.Config)
		}
	}

	return routeToolRuntimeBinding{
		registry:     registry,
		executor:     executor,
		workspaceDir: workspaceDir,
	}
}
