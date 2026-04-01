package bootstrap

func newRouteRuntimeCoreToolingSurfaceOptions(state *routeRegistrationState) routeRuntimeContractToolingOptions {
	return routeRuntimeContractToolingOptions{
		registry:              state.services.ToolRegistry,
		skillRegistry:         state.services.SkillRegistry,
		mediaDir:              state.mediaDir,
		browserBackend:        state.deps.BrowserBackend,
		lightpanda:            state.deps.LightpandaShimSvc,
		acquireBrowser:        state.deps.AcquireBrowserSvc,
		lazyBrowser:           state.deps.LazyBrowserSvc,
		sttService:            state.deps.STTService,
		pushService:           state.deps.PushService,
		workspaceAllowedPaths: state.workspaceAllowedPaths,
		mediaManager:          state.deps.MediaManager,
		mediaStorage:          state.deps.MediaStorage,
		llmRegistry:           state.services.LLMRegistry,
		ocr:                   state.services.OCRService,
		providerPool:          state.deps.ProviderPool,
		speechSource:          state.deps.SpeechHandler,
		voiceSource:           state.deps.VoiceHandler,
	}
}
