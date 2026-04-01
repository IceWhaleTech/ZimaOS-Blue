package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

type routeRuntimeContractActivationOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	protected             *echo.Group
	deps                  *RoutesDeps
	logger                *zap.Logger
	runtimeLLM            *runtimeLLMProviderRef
	oauthManager          proxy.OAuthTokenProvider
	requirePagePermission func(string) echo.MiddlewareFunc
	authPageV1Group       func(string) *echo.Group
}

type routeRuntimeContractSupportOptions struct {
	v1             *echo.Group
	authMiddleware echo.MiddlewareFunc
	pageMiddleware echo.MiddlewareFunc
	memoryHandler  memoryRouteRegistrar
	chat           layeredMemoryChatTarget
}
