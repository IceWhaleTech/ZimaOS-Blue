package bootstrap

func newRuntimeToolSurfacesResult(
	runtime routeToolRuntimeBinding,
	options runtimeToolSurfacesOptions,
) runtimeToolSurfacesResult {
	return runtimeToolSurfacesResult{
		agentRunner: runtime.activateAgentSurface(
			options.protected,
			options.deps,
			options.logger,
			options.agentLLMCaller,
			options.harnessRuntime,
			options.reflectService,
			options.skillRegistry,
		),
		mcpRegistered: runtime.registerMCPSurface(
			options.protected,
			options.cfg,
			options.deps,
			options.logger,
			options.agentLLMCaller,
			options.mcpPermission...,
		),
	}
}
