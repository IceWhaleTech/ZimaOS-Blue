package bootstrap

func (binding *runtimeContractBinding) BindTooling(options routeRuntimeContractToolingOptions) routeRuntimeContractToolingResult {
	if binding == nil {
		return routeRuntimeContractToolingResult{}
	}
	options = withRuntimeToolingSkillFallbacks(options)

	result := routeRuntimeContractToolingResult{
		uiReviewerTool: newRuntimeUIReviewerTool(options.acquireBrowser, options.lazyBrowser, options.mediaDir),
	}
	bindRuntimeToolingBrowserRuntime(options)
	bindRuntimeToolingReminderRuntime(options)
	bindRuntimeToolingHostA11yRuntime(options)
	bindRuntimeToolingImageRuntime(options, result.uiReviewerTool)
	bindRuntimeTTSTool(options.registry, options.speechSource, options.voiceSource)

	return result
}
