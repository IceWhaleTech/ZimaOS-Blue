package bootstrap

func bindRouteRuntimeResearchSurface(
	surface routeRuntimeResearchSurface,
	options routeRuntimeContractResearchOptions,
) routeRuntimeContractResearchResult {
	if surface == nil {
		return routeRuntimeContractResearchResult{}
	}
	harnessRuntime := surface.HarnessRuntime()
	researchService := surface.ResearchService()
	binding := newChatResearchRuntimeBinding(
		harnessRuntime,
		researchService,
		nil,
		options.broker,
		options.workspaceDir,
	)
	return applyChatResearchRuntimeBinding(binding, chatResearchRuntimeBindingTargets{
		chatTarget:     options.target,
		researchTarget: researchService,
		controller:     harnessRuntimeController(harnessRuntime),
		registry:       options.registry,
	})
}
