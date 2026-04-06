package bootstrap

func newRouteRuntimeCoreCapabilitySupportOptions(state *routeRegistrationState) routeRuntimeContractCapabilitySupportOptions {
	return routeRuntimeContractCapabilitySupportOptions{
		e:                     state.e,
		v1:                    state.v1,
		protected:             state.protected,
		apiProtected:          state.apiProtected,
		authMiddleware:        state.deps.AuthMiddleware,
		authRouteMiddleware:   state.authSurface.authMiddleware,
		requirePagePermission: state.authSurface.requirePagePermission,
		approvalHandler:       state.bootstrapSupport.approvalHandler,
		dataDir:               state.cfg.DataDir,
		deps:                  state.deps,
		flagEvaluator:         state.flagEvaluator,
		logger:                state.logger,
		trace:                 state.trace,
	}
}
