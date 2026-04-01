package bootstrap

import "time"

func newRouteRuntimeCoreAskSupportOptions(state *routeRegistrationState) routeRuntimeContractAskSupportOptions {
	return routeRuntimeContractAskSupportOptions{
		writeDB:       state.runtimeWriteDB,
		readDB:        state.runtimeReadDB,
		appConfig:     state.deps.Config,
		toolRegistry:  state.services.ToolRegistry,
		skillRegistry: state.services.SkillRegistry,
		broker:        state.deps.SSEBroker,
		chatTarget:    state.deps.ChatHandler,
		mediaDir:      state.mediaDir,
		timeout:       5 * time.Minute,
	}
}
