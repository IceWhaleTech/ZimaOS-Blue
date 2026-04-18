package bootstrap

func bindRouteRuntimeCorePhase(state *routeRegistrationState) {
	if state == nil || routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeCoreInfrastructure(state); if routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeCoreExperience(state); if routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeCoreTooling(state); if routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeCoreSupport(state)
}
