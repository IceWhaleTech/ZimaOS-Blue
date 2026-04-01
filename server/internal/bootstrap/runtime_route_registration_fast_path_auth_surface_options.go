package bootstrap

func newRouteRuntimeAuthSurfaceOptions(state *routeRegistrationState) routeRuntimeContractAuthSurfaceOptions {
	return routeRuntimeContractAuthSurfaceOptions{
		v1:             state.v1,
		api:            state.api,
		writeDB:        state.runtimeWriteDB,
		readDB:         state.runtimeReadDB,
		authMiddleware: state.deps.AuthMiddleware,
		userRepo:       state.services.UserRepo,
		logger:         state.logger,
	}
}
