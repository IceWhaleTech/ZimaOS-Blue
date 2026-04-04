package bootstrap

import (
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

type runtimeProxyMaskingRoutesOptions struct {
	v1             *echo.Group
	authMiddleware echo.MiddlewareFunc
	pageMiddleware echo.MiddlewareFunc
	dataMasker     *proxy.DataMasker
	onToggle       func()
}

type runtimeProxyPrunerOptions struct {
	v1             *echo.Group
	authMiddleware echo.MiddlewareFunc
	pageMiddleware echo.MiddlewareFunc
	dataDir        string
	initialConfig  *pruner.Config
	handler        *proxy.ProxyHandler
}

type runtimeProxyPrunerRuntime struct {
	config  *pruner.Config
	handler *pruner.APIHandler
	current func() *pruner.Middleware
	ensure  func() *pruner.Middleware
}

type runtimeProxyAvailableModelsSource interface {
	ListAvailableModels() []*providerpool.Model
}

type runtimeProxyProviderChangeListener interface {
	AddProviderChangeListener(func(provider *providerpool.Provider, action string))
}

type runtimeProxyRoutingOptions struct {
	modelCatalog      runtimeProxyAvailableModelsSource
	providerChanges   runtimeProxyProviderChangeListener
	modelRouterConfig *proxy.ModelRouterConfig
	ruleRoutingConfig *proxy.RoutingConfig
}

type runtimeProxyRoutingSetup struct {
	tierResolver   *proxy.TierResolver
	modelRouter    *proxy.ModelRouter
	ruleEngine     *proxy.RuleEngine
	routingEnabled bool
}

type runtimeProxyFailoverCallbackTarget interface {
	SetFailoverCallback(func(*providerpool.FailoverResult))
	UpdateLatency(providerID string, latency time.Duration)
}

type runtimeProxyProviderRegistryTarget interface {
	SetOnStatusChange(func(providerID string, oldStatus, newStatus providerpool.ProviderStatus))
	ListEnabled() []*providerpool.Provider
}

type runtimeProxyProviderBindingsOptions struct {
	handler       *proxy.ProxyHandler
	providerPool  *providerpool.Pool
	router        runtimeProxyFailoverCallbackTarget
	registry      runtimeProxyProviderRegistryTarget
	oauthManager  proxy.OAuthTokenProvider
	apiKeys       *auth.APIKeyService
	smartFailover *proxy.SmartFailoverHandler
	pipelineStats *proxy.PipelineStatsCollector
	broker        *sse.Broker
	connPool      *proxy.ConnectionPool
}
