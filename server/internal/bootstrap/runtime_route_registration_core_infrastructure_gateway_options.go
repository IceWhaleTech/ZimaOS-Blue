package bootstrap

func newRouteRuntimeInfrastructureGatewayOptions(state *routeRegistrationState) routeRuntimeContractGatewayOptions {
	return routeRuntimeContractGatewayOptions{
		gateway:        state.deps.Gateway,
		handler:        state.deps.GatewayHandler,
		e:              state.e,
		protected:      state.protected,
		toolRegistry:   state.services.ToolRegistry,
		browserBackend: state.deps.BrowserBackend,
		mediaDir:       state.mediaDir,
		chat:           state.deps.ChatHandler,
		closers:        &state.deps.Closers,
	}
}
