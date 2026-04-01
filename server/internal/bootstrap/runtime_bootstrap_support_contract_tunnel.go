package bootstrap

import (
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func registerRouteRuntimeTunnelSurface(options routeRuntimeContractBootstrapSupportOptions) {
	if options.authPageV1Group == nil {
		return
	}
	tunnelHandler := networkapi.NewTunnelHandler(options.ngrokConfigStore, routeRuntimeServerPort(options.serverConfig))
	tunnelHandler.SetJWTService(options.jwtService)
	tunnelHandler.RegisterGroupRoutes(options.authPageV1Group(permission.PageChannels))
}
