package bootstrap

func newRouteRuntimeExperienceChatOptions(state *routeRegistrationState) routeRuntimeContractChatOptions {
	return routeRuntimeContractChatOptions{
		chat:          state.deps.ChatHandler,
		security:      state.deps.SecurityHandler,
		disabled:      state.deps.DisablePromptGuard,
		config:        state.deps.Config,
		dataDir:       state.cfg.DataDir,
		workspaceDir:  state.workspaceDir,
		flagEvaluator: state.flagEvaluator,
		logger:        state.logger,
	}
}
