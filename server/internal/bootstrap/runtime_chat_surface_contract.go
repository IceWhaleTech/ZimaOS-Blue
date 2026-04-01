package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

type routeRuntimeContractChatSurfaceOptions struct {
	v1             *echo.Group
	authMiddleware *auth.AuthMiddleware
	chatHandler    *serverpkg.ChatHandler
	sseBroker      *sse.Broker
}

func (binding *runtimeContractBinding) BindChatSurfaceRuntime(options routeRuntimeContractChatSurfaceOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeChatSurface(options)
}

func bindRouteRuntimeChatSurface(options routeRuntimeContractChatSurfaceOptions) {
	if options.chatHandler == nil || options.v1 == nil {
		return
	}

	if options.sseBroker != nil {
		options.chatHandler.SetSSEBroker(options.sseBroker)
	}

	chatGroup := options.v1.Group("")
	if options.authMiddleware != nil {
		chatGroup.Use(options.authMiddleware.OptionalAuthenticate())
	}
	options.chatHandler.RegisterRoutes(chatGroup)
}
