package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type runtimeProxyLaneOptions struct {
	e                 *echo.Echo
	v1                *echo.Group
	protected         *echo.Group
	restrictionGroup  *echo.Group
	failoverGuard     echo.MiddlewareFunc
	authMiddleware    echo.MiddlewareFunc
	pageMiddleware    echo.MiddlewareFunc
	routingConfig     *proxy.RouteConfig
	connectionConfig  *proxy.ConnectionConfig
	dataMasker        *proxy.DataMasker
	sttService        stt.Service
	kv                kvstore.Store
	dataDir           string
	prunerConfig      *pruner.Config
	modelRouterConfig *proxy.ModelRouterConfig
	ruleRoutingConfig *proxy.RoutingConfig
	metricsWriter     *metrics.MetricsWriter
	fallbackWriteDB   *sql.DB
	fallbackReadDB    *sql.DB
	closers           *[]interface{ Close() error }
	providerPool      *providerpool.Pool
	apiKeyService     *auth.APIKeyService
	sseBroker         *sse.Broker
	oauthManager      proxy.OAuthTokenProvider
}

type runtimeProxyLaneBundle struct {
	handler       *proxy.ProxyHandler
	smartFailover *proxy.SmartFailoverHandler
	pipelineStats *proxy.PipelineStatsCollector
	prunerRuntime *runtimeProxyPrunerRuntime
	persistence   runtimeProxyTogglePersistence
	surface       *runtimeProxySurfaceBundle
	saveToggle    func()
}
