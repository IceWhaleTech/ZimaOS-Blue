package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type runtimeProxyEntryOptions struct {
	e                  *echo.Echo
	v1                 *echo.Group
	protected          *echo.Group
	restrictionGroup   *echo.Group
	failoverGuard      echo.MiddlewareFunc
	authMiddleware     echo.MiddlewareFunc
	pageMiddleware     echo.MiddlewareFunc
	maskingAuth        echo.MiddlewareFunc
	maskingPage        echo.MiddlewareFunc
	appConfig          *config.Config
	dataDir            string
	dataMasker         *proxy.DataMasker
	sttService         stt.Service
	kv                 kvstore.Store
	prunerConfig       *pruner.Config
	metricsWriter      *metrics.MetricsWriter
	fallbackWriteDB    *sql.DB
	fallbackReadDB     *sql.DB
	closers            *[]interface{ Close() error }
	providerPool       *providerpool.Pool
	apiKeyService      *auth.APIKeyService
	sseBroker          *sse.Broker
	oauthManager       proxy.OAuthTokenProvider
	proxyBridgeSurface runtimeProxyBridgeSurface
	runtimeLLM         *runtimeLLMProviderRef
	auxiliaryLLM       *auxiliaryLLMCaller
	defaultAgentCaller agent.LLMCaller
	logger             *zap.Logger
}

type runtimeProxyEntryResult struct {
	lane            *runtimeProxyLaneBundle
	dataMasker      *proxy.DataMasker
	agentLLMCaller  agent.LLMCaller
	maskingOnToggle func()
}
