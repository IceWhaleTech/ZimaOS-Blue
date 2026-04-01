package bootstrap

func newRouteRuntimeCoreToolingOptions(state *routeRegistrationState) routeRuntimeContractCoreToolingOptions {
	return routeRuntimeContractCoreToolingOptions{
		scheduler: newRouteRuntimeCoreSchedulerOptions(state),
		tooling:   newRouteRuntimeCoreToolingSurfaceOptions(state),
		analyze:   newRouteRuntimeCoreAnalyzeOptions(state),
		provider:  newRouteRuntimeCoreProviderOptions(state),
	}
}
