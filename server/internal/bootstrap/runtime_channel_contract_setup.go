package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/labstack/echo/v4"
)

func registerRouteRuntimeWechatILinkSetupRoutes(
	options routeRuntimeContractChannelOptions,
	pageMiddleware echo.MiddlewareFunc,
	channelManager *channel.Manager,
	channelFactory *serverpkg.ChannelFactory,
) {
	setupHandler := serverpkg.NewWeChatILinkSetupHandler(
		options.channelConfigStore,
		channelManager,
		channelFactory,
		options.logger,
	)
	setupGroup := options.api.Group("", filterRouteMiddlewares(options.authMiddleware, pageMiddleware)...)
	setupHandler.RegisterRoutes(setupGroup)
}
