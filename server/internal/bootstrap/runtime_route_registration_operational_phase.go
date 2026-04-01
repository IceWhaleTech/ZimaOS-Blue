package bootstrap

func bindRouteRuntimeOperationalPhase(state *routeRegistrationState) {
	state.setOperationalRuntime(state.runtimeContract.BindOperationalRuntime(newRouteRuntimeOperationalOptions(state)))
}
