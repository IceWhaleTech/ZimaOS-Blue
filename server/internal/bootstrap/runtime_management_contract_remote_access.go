package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
)

func bindRouteRuntimeRemoteAccessSupport(options routeRuntimeContractManagementSupportOptions) {
	if options.ngrokTunnelMgr == nil || options.ngrokConfigStore == nil || options.authPageV1Group == nil {
		return
	}
	handler := networkapi.NewSDKRemoteAccessHandler(
		options.ngrokTunnelMgr,
		options.ngrokConfigStore,
		routeRuntimeServerPort(options.serverConfig),
	)
	handler.SetJWTService(options.jwtService)
	handler.RegisterGroupRoutes(options.authPageV1Group(permission.PageChannels))
}
