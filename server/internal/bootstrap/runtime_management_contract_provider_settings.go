package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func bindRouteRuntimeProviderSettings(
	options routeRuntimeContractManagementSupportOptions,
) *serverpkg.ProviderSettingsHandler {
	registry := options.providerRegistry
	if registry == nil {
		registry = llm.NewProviderRegistry()
	}

	store := options.configKV
	if store == nil {
		store = kvstore.NewMemoryStore()
	}

	handler := serverpkg.NewProviderSettingsHandler(registry, store)
	if options.protected != nil {
		handler.RegisterRoutes(options.protected.Group(
			"/providers/settings",
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageSettings))...,
		))
	}
	return handler
}
