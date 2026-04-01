package bootstrap

func newRouteRuntimeInfrastructureChatSurfaceOptions(state *routeRegistrationState) routeRuntimeContractChatSurfaceOptions {
	return routeRuntimeContractChatSurfaceOptions{
		v1:             state.v1,
		authMiddleware: state.deps.AuthMiddleware,
		chatHandler:    state.deps.ChatHandler,
		sseBroker:      state.deps.SSEBroker,
	}
}
