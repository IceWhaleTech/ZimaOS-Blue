package bootstrap

func bindRouteRuntimeStartupAuthSurfaces(state *routeRegistrationState) {
	state.setStartupAuthRuntime(state.runtimeContract.BindStartupAuthRuntime(newRouteRuntimeStartupAuthOptions(state)))
}

func bindRouteRuntimeBootstrapSupportPhase(state *routeRegistrationState) {
	state.setBootstrapPhaseRuntime(state.runtimeContract.BindBootstrapPhaseRuntime(newRouteRuntimeBootstrapPhaseOptions(state)))
}
