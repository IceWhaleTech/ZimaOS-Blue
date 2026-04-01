package bootstrap

func newRouteRuntimeStartupSurfaceOptions(state *routeRegistrationState) routeRuntimeContractStartupSurfaceOptions {
	return routeRuntimeContractStartupSurfaceOptions{
		e:              state.e,
		v1:             state.v1,
		dataDir:        state.dataDir,
		services:       state.services,
		authMiddleware: state.deps.AuthMiddleware,
		userHandler:    state.deps.UserHandler,
		logger:         state.logger,
	}
}
