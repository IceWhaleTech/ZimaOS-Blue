package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

type routeRuntimeContractMediaOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	deps                  *RoutesDeps
	dataDir               string
	logger                *zap.Logger
	requirePagePermission func(string) echo.MiddlewareFunc
}

type routeRuntimeContractMediaResult struct {
	ipcServer *sockipc.Server
}
