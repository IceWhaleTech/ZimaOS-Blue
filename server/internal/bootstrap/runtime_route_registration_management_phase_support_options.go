package bootstrap

func newRouteRuntimeManagementSupportOptions(state *routeRegistrationState) routeRuntimeContractManagementSupportOptions {
	return routeRuntimeContractManagementSupportOptions{
		authPageV1Group:       state.authSurface.authPageV1Group,
		protected:             state.protected,
		requirePagePermission: state.authSurface.requirePagePermission,
		config:                state.deps.Config,
		serverConfig:          state.cfg,
		ctx:                   state.deps.Ctx,
		logger:                state.logger,
		cronHandler:           state.deps.CronHandler,
		ngrokTunnelMgr:        state.deps.NgrokTunnelMgr,
		ngrokConfigStore:      state.deps.NgrokConfigStore,
		jwtService:            state.services.JWTService,
		providerRegistry:      state.deps.ChatHandler.GetProviderRegistry(),
		configKV:              state.deps.ConfigKV,
	}
}
