package bootstrap

func newRouteRuntimeManagementOptions(state *routeRegistrationState) routeRuntimeContractManagementRuntimeOptions {
	return routeRuntimeContractManagementRuntimeOptions{
		mgmt:      newRouteRuntimeManagementMgmtOptions(state),
		heartbeat: newRouteRuntimeManagementHeartbeatOptions(state),
		support:   newRouteRuntimeManagementSupportOptions(state),
		user:      newRouteRuntimeManagementUserOptions(state),
		channel:   newRouteRuntimeManagementChannelOptions(state),
	}
}
