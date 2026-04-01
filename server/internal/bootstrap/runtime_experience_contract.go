package bootstrap

func (binding *runtimeContractBinding) BindExperienceRuntime(options routeRuntimeContractExperienceOptions) routeRuntimeContractExperienceResult {
	if binding == nil {
		return routeRuntimeContractExperienceResult{}
	}
	return bindRouteRuntimeExperience(binding, options)
}

func bindRouteRuntimeExperience(binding routeRuntimeExperienceBinding, options routeRuntimeContractExperienceOptions) routeRuntimeContractExperienceResult {
	bindRouteRuntimeExperienceSurface(binding, options.surface)
	result := routeRuntimeContractExperienceResult{
		skillAutoReranker: binding.ConfigureChatRuntime(options.chat),
		chat:              binding.BindChatRuntime(options.chatBinding),
		productivity:      binding.BindProductivityTools(options.productivity),
	}
	binding.BindPlatformSurfaceRuntime(options.platform)
	result.skill = binding.BindSkillRuntime(options.skill)
	return result
}

func bindRouteRuntimeExperienceSurface(binding routeRuntimeResearchSurface, options routeRuntimeContractExperienceSurfaceOptions) {
	bindRouteRuntimeResearchSurface(binding, options.research)
}
