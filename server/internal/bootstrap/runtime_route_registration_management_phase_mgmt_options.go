package bootstrap

func newRouteRuntimeManagementMgmtOptions(state *routeRegistrationState) routeRuntimeContractMgmtOptions {
	return routeRuntimeContractMgmtOptions{
		registry:      state.services.ToolRegistry,
		providerPool:  state.deps.ProviderPool,
		skillRegistry: state.services.SkillRegistry,
		workspaceDir:  state.workspaceDir,
		version:       state.cfg.Version,
		userService:   state.services.UserService,
		apiKeyService: state.services.APIKeyService,
	}
}
