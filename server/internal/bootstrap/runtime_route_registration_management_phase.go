package bootstrap

func bindRouteRuntimeManagementPhase(state *routeRegistrationState) {
	state.setManagementRuntime(state.runtimeContract.BindManagementRuntime(newRouteRuntimeManagementOptions(state)))
}
