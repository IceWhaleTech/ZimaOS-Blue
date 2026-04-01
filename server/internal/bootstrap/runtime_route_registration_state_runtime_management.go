package bootstrap

func (state *routeRegistrationState) setManagementRuntime(result routeRuntimeContractManagementRuntimeResult) {
	if state == nil {
		return
	}
	state.managementRuntime = result
	state.runtimeSnapshot.managementRuntime = result
	state.setMgmtTool(result.mgmtTool)
	state.setManagementSupport(result.support)
}

func (state *routeRegistrationState) setManagementSupport(result routeRuntimeContractManagementSupportResult) {
	if state == nil {
		return
	}
	state.managementSupport = result
	state.runtimeSnapshot.management = result
}

func (state *routeRegistrationState) setOperationalRuntime(result routeRuntimeContractOperationalResult) {
	if state == nil {
		return
	}
	state.operationalRuntime = result
	state.runtimeSnapshot.operational = result
}
