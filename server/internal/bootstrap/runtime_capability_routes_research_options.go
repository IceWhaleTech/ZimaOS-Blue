package bootstrap

func newRuntimeTaskResearchSurfaceOptions(
	contract runtimeCapabilityResearchSurface,
	options runtimeTaskSurfaceOptions,
) runtimeTaskResearchSurfaceOptions {
	return runtimeTaskResearchSurfaceOptions{
		research:     contract.ResearchService(),
		harness:      contract.HarnessRuntime(),
		workspaceDir: options.workspaceDir,
		deepResearchGroups: runtimeTaskSurfaceProjectionGroups(
			options,
			"/deep-research",
			options.chatPermission,
		),
		harnessResearchGroups: runtimeTaskSurfaceProjectionGroups(
			options,
			"/harness/research",
			options.chatPermission,
		),
	}
}
