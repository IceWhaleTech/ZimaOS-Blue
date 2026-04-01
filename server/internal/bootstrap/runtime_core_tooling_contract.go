package bootstrap

func (binding *runtimeContractBinding) BindCoreToolingRuntime(options routeRuntimeContractCoreToolingOptions) routeRuntimeContractCoreToolingResult {
	if binding == nil {
		return routeRuntimeContractCoreToolingResult{}
	}
	return bindRouteRuntimeCoreToolingContract(binding, options)
}

func bindRouteRuntimeCoreToolingContract(binding routeRuntimeCoreToolingBinding, options routeRuntimeContractCoreToolingOptions) routeRuntimeContractCoreToolingResult {
	resolvedScheduler := options.scheduler.scheduler
	if resolvedScheduler == nil {
		resolvedScheduler = runtimeSkillAsSchedulerTarget(options.scheduler.skillRegistry, "scheduler")
	}
	binding.BindSchedulerServices(options.scheduler)
	return routeRuntimeContractCoreToolingResult{
		schedulerBound: options.scheduler.cronHandler != nil && resolvedScheduler != nil,
		tooling:        binding.BindTooling(options.tooling),
		analyzeTool:    binding.BindAnalyzeTool(options.analyze),
		provider:       binding.BindProviderPoolRuntime(options.provider),
	}
}
