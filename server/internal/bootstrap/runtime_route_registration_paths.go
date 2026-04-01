package bootstrap

func bindRouteRuntimeFastPath(state *routeRegistrationState) {
	if state == nil {
		return
	}
	bindRouteRuntimeHTTPBootstrap(state)
	bindRouteRuntimeStartupAuthSurfaces(state)
	bindRouteRuntimeBootstrapSupportPhase(state)
}

func bindRouteRuntimeDeferredPath(state *routeRegistrationState) {
	if state == nil {
		return
	}
	bindRouteRuntimeCorePhase(state)
	bindRouteRuntimeManagementPhase(state)
	bindRouteRuntimeOperationalPhase(state)
}
