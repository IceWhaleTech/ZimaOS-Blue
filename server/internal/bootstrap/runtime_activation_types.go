package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type runtimeActivationOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	protected             *echo.Group
	restrictionGroup      *echo.Group
	failoverGuard         echo.MiddlewareFunc
	authMiddleware        echo.MiddlewareFunc
	pageMiddleware        echo.MiddlewareFunc
	maskingPageMiddleware echo.MiddlewareFunc
	mcpPermission         []echo.MiddlewareFunc
	services              *Services
	cfg                   *ServerConfig
	deps                  *RoutesDeps
	logger                *zap.Logger
	runtimeLLM            *runtimeLLMProviderRef
	oauthManager          proxy.OAuthTokenProvider
	harnessRuntime        *HarnessRuntimeBundle
	reflectService        *selfreflect.Service
	skillRegistry         *skillpkg.Registry
}

type runtimeActivationResult struct {
	lane           *runtimeProxyLaneBundle
	dataMasker     *proxy.DataMasker
	agentLLMCaller agent.LLMCaller
	auxiliaryLLM   *auxiliaryLLMCaller
	agentRunner    *agent.Runner
	mcpRegistered  bool
}

type routeRuntimeActivationOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	protected             *echo.Group
	deps                  *RoutesDeps
	logger                *zap.Logger
	runtimeLLM            *runtimeLLMProviderRef
	oauthManager          proxy.OAuthTokenProvider
	harnessRuntime        *HarnessRuntimeBundle
	reflectService        *selfreflect.Service
	requirePagePermission func(string) echo.MiddlewareFunc
	authPageV1Group       func(string) *echo.Group
}
