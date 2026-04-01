package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractGatewayOptions struct {
	gateway        *gateway.Gateway
	handler        *gateway.Handler
	e              *echo.Echo
	protected      *echo.Group
	toolRegistry   *tools.Registry
	browserBackend tools.BrowserBackend
	mediaDir       string
	chat           *serverpkg.ChatHandler
	pluginRegistry *plugin.Registry
	closers        *[]interface{ Close() error }
}

type routeRuntimeContractGatewayResult struct {
	toolRegistered    bool
	methodsRegistered bool
	routesRegistered  bool
	closerRegistered  bool
}

type gatewayStopper struct {
	gateway *gateway.Gateway
}
