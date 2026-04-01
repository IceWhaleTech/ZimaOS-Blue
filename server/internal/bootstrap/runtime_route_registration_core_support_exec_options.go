package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func newRouteRuntimeCoreExecSupportOptions(state *routeRegistrationState) routeRuntimeContractExecSupportOptions {
	return routeRuntimeContractExecSupportOptions{
		writeDB:              state.runtimeWriteDB,
		readDB:               state.runtimeReadDB,
		dataDir:              state.cfg.DataDir,
		workspaceDir:         state.workspaceDir,
		ripgrep:              state.deps.Config.ToolCalling.Ripgrep,
		workspaceAllowedPath: state.workspaceAllowedPaths,
		memoryStore:          state.services.MemoryStore,
		toolRegistry:         state.services.ToolRegistry,
		skillRegistry:        state.services.SkillRegistry,
		selectorSource:       state.deps.ChatHandler,
		broker:               state.deps.SSEBroker,
		sandboxManager:       state.deps.SandboxManager,
		chatHandler:          state.deps.ChatHandler,
		logger:               state.logger,
		closers:              &state.deps.Closers,
		profileRoutes:        state.authSurface.authPageV1Group(permission.PageSettings),
		sessionRoutes:        state.authSurface.authPageV1Group(permission.PageChat),
		oauthSource: func() agentsessions.OAuthCredentialSource {
			return state.providerRuntime.oauthManager
		},
		lookupAPIKey: state.lookupProviderAPIKey,
	}
}
