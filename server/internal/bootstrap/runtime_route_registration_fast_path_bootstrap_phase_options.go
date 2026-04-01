package bootstrap

func newRouteRuntimeBootstrapPhaseOptions(state *routeRegistrationState) routeRuntimeContractBootstrapPhaseOptions {
	return routeRuntimeContractBootstrapPhaseOptions{
		dataDir:      state.dataDir,
		shell:        newRouteRuntimeShellSurfaceOptions(state),
		bootstrap:    newRouteRuntimeBootstrapSupportOptions(state),
		onEarlyReady: state.deps.OnEarlyReady,
	}
}
