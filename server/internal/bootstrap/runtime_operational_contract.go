package bootstrap

func (binding *runtimeContractBinding) BindOperationalRuntime(options routeRuntimeContractOperationalOptions) routeRuntimeContractOperationalResult {
	if binding == nil {
		return routeRuntimeContractOperationalResult{}
	}
	return bindRouteRuntimeOperational(binding, options)
}

func bindRouteRuntimeOperational(binding routeRuntimeOperationalBinding, options routeRuntimeContractOperationalOptions) routeRuntimeContractOperationalResult {
	if binding == nil {
		return routeRuntimeContractOperationalResult{}
	}
	taskSurface := binding.RegisterTaskSurface(options.taskSurface)
	result := routeRuntimeContractOperationalResult{
		taskSurface: taskSurface,
	}
	result.activation = binding.ActivateRouteRuntime(options.activation)
	result.support = bindRouteRuntimeOperationalSupport(binding, result.activation, options)
	return result
}
