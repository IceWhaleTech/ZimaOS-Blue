package bootstrap

func newRouteRuntimeExperienceSurfaceOptions(state *routeRegistrationState) routeRuntimeContractExperienceSurfaceOptions {
	return routeRuntimeContractExperienceSurfaceOptions{
		research: routeRuntimeContractResearchOptions{
			target:       state.deps.ChatHandler,
			broker:       state.deps.SSEBroker,
			registry:     state.services.ToolRegistry,
			workspaceDir: state.workspaceDir,
		},
	}
}
