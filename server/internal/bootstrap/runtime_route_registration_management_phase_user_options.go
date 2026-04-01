package bootstrap

func newRouteRuntimeManagementUserOptions(state *routeRegistrationState) routeRuntimeContractUserSurfaceOptions {
	return routeRuntimeContractUserSurfaceOptions{
		v1:                    state.v1,
		protected:             state.protected,
		authPageV1Group:       state.authSurface.authPageV1Group,
		authPageAPIGroup:      state.authSurface.authPageAPIGroup,
		requirePagePermission: state.authSurface.requirePagePermission,
		workspace:             state.deps.WorkspaceHandler,
		companionHandler:      state.deps.CompanionHandler,
		companionWSHandler:    state.deps.CompanionWSHandler,
		writeDB:               state.runtimeWriteDB,
		readDB:                state.runtimeReadDB,
		llmRegistry:           state.services.LLMRegistry,
		skillsDir:             state.skillRuntime.skillsDir,
		metricsWriter:         state.deps.MetricsWriter,
		logger:                state.logger,
		trace:                 state.trace,
	}
}
