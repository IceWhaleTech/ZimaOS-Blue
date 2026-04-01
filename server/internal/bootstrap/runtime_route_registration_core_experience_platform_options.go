package bootstrap

func newRouteRuntimeExperiencePlatformOptions(state *routeRegistrationState) routeRuntimeContractPlatformSurfaceOptions {
	return routeRuntimeContractPlatformSurfaceOptions{
		v1:                    state.v1,
		protected:             state.protected,
		authPageV1Group:       state.authSurface.authPageV1Group,
		requirePagePermission: state.authSurface.requirePagePermission,
		authMiddleware:        state.authSurface.authMiddleware,
		serverConfig:          state.cfg,
		logger:                state.logger,
		billingPool:           state.deps.ProviderPool,
		metricsCollector:      state.deps.MetricsCollector,
		metricsWriter:         state.deps.MetricsWriter,
		metricsTarget:         state.deps.ChatHandler,
		connectionManager:     state.connManager,
		pluginRegistry:        state.deps.PluginRegistry,
		pluginStore:           state.deps.PluginStore,
		toolRegistry:          state.services.ToolRegistry,
	}
}
