package bootstrap

func newRouteRuntimeBootstrapSupportOptions(state *routeRegistrationState) routeRuntimeContractBootstrapSupportOptions {
	return routeRuntimeContractBootstrapSupportOptions{
		apiProtected:          state.apiProtected,
		protected:             state.protected,
		authPageV1Group:       state.authSurface.authPageV1Group,
		requirePagePermission: state.authSurface.requirePagePermission,
		serverConfig:          state.cfg,
		configKV:              state.deps.ConfigKV,
		ctx:                   state.deps.Ctx,
		logger:                state.logger,
		chatHandler:           state.deps.ChatHandler,
		sseBroker:             state.deps.SSEBroker,
		ngrokConfigStore:      state.deps.NgrokConfigStore,
		jwtService:            state.services.JWTService,
	}
}
