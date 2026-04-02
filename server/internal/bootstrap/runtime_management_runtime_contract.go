package bootstrap

func (binding *runtimeContractBinding) BindManagementRuntime(options routeRuntimeContractManagementRuntimeOptions) routeRuntimeContractManagementRuntimeResult {
	if binding == nil {
		return routeRuntimeContractManagementRuntimeResult{}
	}
	return bindRouteRuntimeManagement(binding, options)
}

func bindRouteRuntimeManagement(binding routeRuntimeManagementBinding, options routeRuntimeContractManagementRuntimeOptions) routeRuntimeContractManagementRuntimeResult {
	result := routeRuntimeContractManagementRuntimeResult{
		mgmtTool: binding.RegisterMgmtTool(options.mgmt),
	}
	result.support = binding.BindManagementSupport(options.support)
	binding.BindUserSurfaceRuntime(options.user)
	result.userSurfaceBound = options.user.v1 != nil && options.user.protected != nil
	binding.BindMgmtUpgrade(result.mgmtTool, routeRuntimeContractMgmtUpgradeOptions{
		handler:    result.support.updateHandler,
		otaChecker: result.support.otaChecker,
		version:    options.mgmt.version,
	})
	result.upgradeBound = result.mgmtTool != nil && result.support.updateHandler != nil && result.support.otaChecker != nil
	options.channel.mgmtTool = result.mgmtTool
	binding.BindChannelRuntime(options.channel)
	result.channelBound = options.channel.api != nil
	return result
}
