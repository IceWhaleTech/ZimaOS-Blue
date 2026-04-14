package bootstrap

func newRouteRuntimeManagementChannelOptions(state *routeRegistrationState) routeRuntimeContractChannelOptions {
	return routeRuntimeContractChannelOptions{
		api:                   state.api,
		authMiddleware:        state.authSurface.authMiddleware,
		requirePagePermission: state.authSurface.requirePagePermission,
		config:                state.deps.Config,
		serverPort:            state.cfg.Port,
		logger:                state.deps.Logger,
		chat:                  state.deps.ChatHandler,
		channelConfigStore:    state.deps.ChannelConfigStore,
		channelTaskWatcher:    state.deps.ChannelTaskWatcher,
		ngrokTunnelMgr:        state.deps.NgrokTunnelMgr,
		tunnelHandler:         state.bootstrapSupport.tunnelHandler,
		jwtService:            state.services.JWTService,
		autoreplyService:      state.deps.AutoreplyService,
	}
}
