package bootstrap

func newRouteRuntimeAccountSurfaceOptions(state *routeRegistrationState) routeRuntimeContractAccountSurfaceOptions {
	return routeRuntimeContractAccountSurfaceOptions{
		v1:               state.v1,
		authMiddleware:   state.deps.AuthMiddleware,
		userHandler:      state.deps.UserHandler,
		apiKeyHandler:    state.deps.APIKeyHandler,
		autoreplyHandler: state.deps.AutoreplyHandler,
		logger:           state.logger,
	}
}
