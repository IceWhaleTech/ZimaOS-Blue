package bootstrap

func newRouteRuntimeExperienceChatBindingOptions(state *routeRegistrationState) routeRuntimeContractChatBindingOptions {
	return routeRuntimeContractChatBindingOptions{
		target:  state.deps.ChatHandler,
		handler: state.deps.ChatHandler,
	}
}
