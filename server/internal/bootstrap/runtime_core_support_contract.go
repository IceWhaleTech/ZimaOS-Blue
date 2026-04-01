package bootstrap

func (binding *runtimeContractBinding) BindCoreSupportRuntime(options routeRuntimeContractCoreSupportOptions) routeRuntimeContractCoreSupportResult {
	if binding == nil {
		return routeRuntimeContractCoreSupportResult{}
	}
	return bindRouteRuntimeSupport(binding, options)
}

func bindRouteRuntimeSupport(binding routeRuntimeCoreSupportBinding, options routeRuntimeContractCoreSupportOptions) routeRuntimeContractCoreSupportResult {
	result := routeRuntimeContractCoreSupportResult{
		ask:  binding.NewAskSupportBundle(options.ask),
		exec: binding.NewExecSupportBundle(options.exec),
	}
	options.capability.askSupport = result.ask
	options.capability.execSupport = result.exec
	result.capability = binding.BindCapabilitySupportRuntime(options.capability)
	return result
}
