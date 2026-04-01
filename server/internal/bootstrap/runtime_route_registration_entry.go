package bootstrap

func bindRouteRuntimeRegistration(state *routeRegistrationState) {
	if state == nil {
		return
	}
	bindRouteRuntimeFastPath(state)
	bindRouteRuntimeDeferredPath(state)
}
