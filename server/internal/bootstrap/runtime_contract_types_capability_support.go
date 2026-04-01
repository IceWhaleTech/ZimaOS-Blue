package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

type routeRuntimeContractCapabilitySupportOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	protected             *echo.Group
	apiProtected          *echo.Group
	authMiddleware        *auth.AuthMiddleware
	authRouteMiddleware   echo.MiddlewareFunc
	requirePagePermission func(string) echo.MiddlewareFunc
	dataDir               string
	deps                  *RoutesDeps
	askSupport            runtimeAskSupportBundle
	execSupport           runtimeExecSupportBundle
	flagEvaluator         *config.FlagEvaluator
	logger                *zap.Logger
	trace                 *StartupTrace
}

type routeRuntimeContractCapabilitySupportResult struct {
	convertRegistered bool
}
