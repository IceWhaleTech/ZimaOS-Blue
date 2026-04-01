package bootstrap

func newRouteRuntimeCoreExperienceOptions(state *routeRegistrationState) routeRuntimeContractExperienceOptions {
	return routeRuntimeContractExperienceOptions{
		chat:         newRouteRuntimeExperienceChatOptions(state),
		chatBinding:  newRouteRuntimeExperienceChatBindingOptions(state),
		surface:      newRouteRuntimeExperienceSurfaceOptions(state),
		productivity: newRouteRuntimeExperienceProductivityOptions(state),
		platform:     newRouteRuntimeExperiencePlatformOptions(state),
		skill:        newRouteRuntimeExperienceSkillOptions(state),
	}
}
