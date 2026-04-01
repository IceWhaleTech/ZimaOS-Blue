package bootstrap

func newRouteRuntimeShellSurfaceOptions(state *routeRegistrationState) routeRuntimeContractShellSurfaceOptions {
	return routeRuntimeContractShellSurfaceOptions{
		e:                     state.e,
		v1:                    state.v1,
		protected:             state.protected,
		authPageV1Group:       state.authSurface.authPageV1Group,
		authMiddleware:        state.authSurface.authMiddleware,
		requirePagePermission: state.authSurface.requirePagePermission,
		serverConfig:          state.cfg,
		logger:                state.logger,
		workerPool:            state.services.WorkerPool,
		hotReloader:           state.deps.HotReloader,
		configStore:           state.deps.ConfigStore,
		formfillerHandler:     state.deps.FormfillerHandler,
		extauthHandler:        state.deps.ExtauthHandler,
		toolRegistry:          state.services.ToolRegistry,
		a2uiManager:           state.services.A2UIManager,
		pdfService:            state.services.PDFService,
	}
}
