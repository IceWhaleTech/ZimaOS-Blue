package bootstrap

func bindRouteRuntimeCoreTooling(state *routeRegistrationState) {
	state.setCoreToolingRuntime(state.runtimeContract.BindCoreToolingRuntime(newRouteRuntimeCoreToolingOptions(state)))
}
