package bootstrap

func newRouteRuntimeStartupAuthOptions(state *routeRegistrationState) routeRuntimeContractStartupAuthOptions {
	return routeRuntimeContractStartupAuthOptions{
		startup: newRouteRuntimeStartupSurfaceOptions(state),
		auth:    newRouteRuntimeAuthSurfaceOptions(state),
		account: newRouteRuntimeAccountSurfaceOptions(state),
	}
}
