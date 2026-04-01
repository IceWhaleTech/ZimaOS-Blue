package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

type runtimeProxyRuntimeOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	protected             *echo.Group
	restrictionGroup      *echo.Group
	failoverGuard         echo.MiddlewareFunc
	authMiddleware        echo.MiddlewareFunc
	pageMiddleware        echo.MiddlewareFunc
	maskingPageMiddleware echo.MiddlewareFunc
	deps                  *RoutesDeps
	runtimeLLM            *runtimeLLMProviderRef
	oauthManager          proxy.OAuthTokenProvider
	logger                *zap.Logger
}

type runtimeProxyRuntimeResult struct {
	lane           *runtimeProxyLaneBundle
	dataMasker     *proxy.DataMasker
	agentLLMCaller agent.LLMCaller
	auxiliaryLLM   *auxiliaryLLMCaller
}

func activateRuntimeProxyRuntime(options runtimeProxyRuntimeOptions) runtimeProxyRuntimeResult {
	result := newRuntimeProxyRuntimeResult(options.deps)
	entry := activateRuntimeProxyEntry(newRuntimeProxyEntryOptions(options, result))
	result.lane = entry.lane
	result.dataMasker = entry.dataMasker
	result.agentLLMCaller = entry.agentLLMCaller
	return result
}
