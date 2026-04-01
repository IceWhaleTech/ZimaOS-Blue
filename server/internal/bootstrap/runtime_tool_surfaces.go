package bootstrap

func activateRuntimeToolSurfaces(options runtimeToolSurfacesOptions) runtimeToolSurfacesResult {
	runtime := newRouteToolRuntimeBinding(options.services, options.cfg, options.deps)
	return newRuntimeToolSurfacesResult(runtime, options)
}
