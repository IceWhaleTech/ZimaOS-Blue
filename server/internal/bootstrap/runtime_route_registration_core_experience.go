package bootstrap

func bindRouteRuntimeCoreExperience(state *routeRegistrationState) {
	state.setExperienceRuntime(state.runtimeContract.BindExperienceRuntime(newRouteRuntimeCoreExperienceOptions(state)))
}
