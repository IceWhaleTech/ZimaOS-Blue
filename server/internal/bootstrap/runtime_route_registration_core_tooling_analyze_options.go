package bootstrap

func newRouteRuntimeCoreAnalyzeOptions(state *routeRegistrationState) routeRuntimeContractAnalyzeOptions {
	return routeRuntimeContractAnalyzeOptions{
		registry:        state.services.ToolRegistry,
		skillRegistry:   state.services.SkillRegistry,
		mediaDir:        state.mediaDir,
		browserBackend:  state.deps.BrowserBackend,
		lazyBrowser:     state.deps.LazyBrowserSvc,
		webSearchConfig: state.webSearchConfig,
	}
}
