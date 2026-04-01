package bootstrap

func bindRouteRuntimeCoreInfrastructure(state *routeRegistrationState) {
	state.setInfrastructureRuntime(state.runtimeContract.BindInfrastructureRuntime(
		newRouteRuntimeCoreInfrastructureOptions(state),
	))
}
