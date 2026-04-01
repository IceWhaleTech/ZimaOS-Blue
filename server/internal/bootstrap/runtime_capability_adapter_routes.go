package bootstrap

func (adapter runtimeCapabilityAdapter) registerTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration {
	return registerRuntimeTaskSurface(adapter, options)
}

func (adapter runtimeCapabilityAdapter) registerResearchTaskSurface(
	options runtimeTaskResearchSurfaceOptions,
) runtimeTaskResearchSurfaceRegistration {
	return registerRuntimeTaskResearchSurface(options)
}
