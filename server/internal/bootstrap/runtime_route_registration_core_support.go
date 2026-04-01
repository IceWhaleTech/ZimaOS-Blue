package bootstrap

func bindRouteRuntimeCoreSupport(state *routeRegistrationState) {
	state.setCoreSupportRuntime(state.runtimeContract.BindCoreSupportRuntime(newRouteRuntimeCoreSupportOptions(state)))
}
