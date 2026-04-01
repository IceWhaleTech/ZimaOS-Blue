package bootstrap

func newRouteRuntimeInfrastructureMediaOptions(state *routeRegistrationState) routeRuntimeContractMediaOptions {
	return routeRuntimeContractMediaOptions{
		e:                     state.e,
		v1:                    state.v1,
		deps:                  state.deps,
		dataDir:               state.dataDir,
		logger:                state.logger,
		requirePagePermission: state.authSurface.requirePagePermission,
	}
}
