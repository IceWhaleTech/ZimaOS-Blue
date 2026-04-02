package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

type routeRuntimeManagementBinding interface {
	RegisterMgmtTool(options routeRuntimeContractMgmtOptions) *tools.MgmtTool
	BindManagementSupport(options routeRuntimeContractManagementSupportOptions) routeRuntimeContractManagementSupportResult
	BindUserSurfaceRuntime(options routeRuntimeContractUserSurfaceOptions)
	BindMgmtUpgrade(target runtimeMgmtToolTarget, options routeRuntimeContractMgmtUpgradeOptions)
	BindChannelRuntime(options routeRuntimeContractChannelOptions)
}

var _ routeRuntimeManagementBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractManagementRuntimeOptions struct {
	mgmt    routeRuntimeContractMgmtOptions
	support routeRuntimeContractManagementSupportOptions
	user    routeRuntimeContractUserSurfaceOptions
	channel routeRuntimeContractChannelOptions
}

type routeRuntimeContractManagementRuntimeResult struct {
	mgmtTool         *tools.MgmtTool
	support          routeRuntimeContractManagementSupportResult
	userSurfaceBound bool
	upgradeBound     bool
	channelBound     bool
}
