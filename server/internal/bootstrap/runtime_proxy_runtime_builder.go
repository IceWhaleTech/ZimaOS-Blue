package bootstrap

import (
	"database/sql"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
)

type runtimeProxyDatabaseRefs struct {
	writeDB *sql.DB
	readDB  *sql.DB
}

func resolveRuntimeProxyDatabaseRefs(services *Services, fallback *sql.DB) runtimeProxyDatabaseRefs {
	if services != nil && services.DBConn != nil {
		return runtimeProxyDatabaseRefs{
			writeDB: services.DBConn.Writer,
			readDB:  services.DBConn.Reader,
		}
	}
	return runtimeProxyDatabaseRefs{
		writeDB: fallback,
		readDB:  fallback,
	}
}

func newRuntimeProxyRuntimeResult(deps *RoutesDeps) runtimeProxyRuntimeResult {
	result := runtimeProxyRuntimeResult{
		auxiliaryLLM:   newAuxiliaryLLMCaller(),
		agentLLMCaller: agent.LLMCaller(newProviderRegistryLLMCaller(nil)),
	}
	if deps != nil && deps.Services != nil {
		result.agentLLMCaller = agent.LLMCaller(newProviderRegistryLLMCaller(deps.Services.LLMRegistry))
	}
	return result
}

func newRuntimeProxyEntryOptions(options runtimeProxyRuntimeOptions, result runtimeProxyRuntimeResult) runtimeProxyEntryOptions {
	var (
		services   *Services
		fallbackDB *sql.DB
	)
	if options.deps != nil {
		services = options.deps.Services
		fallbackDB = options.deps.DB
	}
	dbRefs := resolveRuntimeProxyDatabaseRefs(services, fallbackDB)

	entryOptions := runtimeProxyEntryOptions{
		e:                  options.e,
		v1:                 options.v1,
		protected:          options.protected,
		restrictionGroup:   options.restrictionGroup,
		failoverGuard:      options.failoverGuard,
		authMiddleware:     options.authMiddleware,
		pageMiddleware:     options.pageMiddleware,
		maskingAuth:        options.authMiddleware,
		maskingPage:        options.maskingPageMiddleware,
		fallbackWriteDB:    dbRefs.writeDB,
		fallbackReadDB:     dbRefs.readDB,
		oauthManager:       options.oauthManager,
		proxyBridgeSurface: newRuntimeProxyBridgeSurface(services, options.deps),
		runtimeLLM:         options.runtimeLLM,
		auxiliaryLLM:       result.auxiliaryLLM,
		defaultAgentCaller: result.agentLLMCaller,
		logger:             options.logger,
	}
	if options.deps != nil {
		entryOptions.appConfig = options.deps.Config
		entryOptions.sttService = options.deps.STTService
		entryOptions.kv = options.deps.ConfigKV
		entryOptions.metricsWriter = options.deps.MetricsWriter
		entryOptions.closers = &options.deps.Closers
		entryOptions.providerPool = options.deps.ProviderPool
		entryOptions.apiKeyService = options.deps.APIKeyService
		entryOptions.sseBroker = options.deps.SSEBroker
		if options.deps.Config != nil {
			entryOptions.prunerConfig = options.deps.Config.Pruner
		}
		if options.deps.ServerConfig != nil {
			entryOptions.dataDir = options.deps.ServerConfig.DataDir
		}
	}
	return entryOptions
}
