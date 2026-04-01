package bootstrap

func newRouteRuntimeExperienceProductivityOptions(state *routeRegistrationState) routeRuntimeContractProductivityOptions {
	return routeRuntimeContractProductivityOptions{
		writeDB:       state.runtimeWriteDB,
		readDB:        state.runtimeReadDB,
		registry:      state.services.ToolRegistry,
		skillRegistry: state.services.SkillRegistry,
		logger:        state.logger,
	}
}
