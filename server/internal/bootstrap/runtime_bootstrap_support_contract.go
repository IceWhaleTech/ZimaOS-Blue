package bootstrap

import (
	"time"

	"go.uber.org/zap"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func (binding *runtimeContractBinding) BindBootstrapSupportRuntime(
	options routeRuntimeContractBootstrapSupportOptions,
) routeRuntimeContractBootstrapSupportResult {
	if binding == nil {
		return routeRuntimeContractBootstrapSupportResult{}
	}
	return bindRouteRuntimeBootstrapSupport(options)
}

func bindRouteRuntimeBootstrapSupport(
	options routeRuntimeContractBootstrapSupportOptions,
) routeRuntimeContractBootstrapSupportResult {
	result := routeRuntimeContractBootstrapSupportResult{}

	settingsInitStart := time.Now()
	settingsHandler := serverpkg.NewSettingsHandler(options.configKV)
	settingsHandler.SetChatHandler(options.chatHandler)
	smallModelManager := smallmodel.NewManager(routeRuntimeServerDataDir(options.serverConfig))
	settingsHandler.SetSmallModelManager(smallModelManager)
	if options.authPageV1Group != nil {
		settingsHandler.RegisterRoutes(options.authPageV1Group(permission.PageSettings))
	}
	if options.logger != nil {
		options.logger.Info("Settings fast path initialized", zap.Duration("elapsed", time.Since(settingsInitStart)))
	}
	result.settingsHandler = settingsHandler
	result.smallModelManager = smallModelManager

	if options.sseBroker != nil && options.apiProtected != nil {
		sse.NewHandler(options.sseBroker).RegisterRoutes(options.apiProtected.Group("/v1"))
		approvalHandler := networkapi.NewApprovalHandler(options.sseBroker)
		approvalHandler.RegisterRoutes(options.apiProtected.Group("/v1"))
		result.approvalHandler = approvalHandler
	}

	registerRouteRuntimeVoiceWakeSurface(settingsHandler, options)
	registerRouteRuntimeTunnelSurface(options)

	return result
}
