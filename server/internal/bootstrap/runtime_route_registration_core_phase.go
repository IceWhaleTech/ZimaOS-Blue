package bootstrap

func bindRouteRuntimeCorePhase(state *routeRegistrationState) {
	if state == nil {
		return
	}

	bindRouteRuntimeCoreInfrastructure(state)
	bindRouteRuntimeCoreExperience(state)
	bindRouteRuntimeCoreTooling(state)
	bindRouteRuntimeCoreSupport(state)
}
