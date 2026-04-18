package bootstrap

func bindRouteRuntimeFastPath(state *routeRegistrationState) {
	if state == nil { return }
	bindRouteRuntimeHTTPBootstrap(state); bindRouteRuntimeStartupAuthSurfaces(state); bindRouteRuntimeBootstrapSupportPhase(state)
}

func bindRouteRuntimeDeferredPath(state *routeRegistrationState) {
	if state == nil || routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeCorePhase(state); if routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeManagementPhase(state); if routeRegistrationContextCanceled(state) { return }
	bindRouteRuntimeOperationalPhase(state)
}
