package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

func bindRuntimeMgmtTool(
	target runtimeMgmtToolTarget,
	providerPool *providerpool.Pool,
	skillRegistry *skillpkg.Registry,
	workspaceDir string,
	toolRegistry *tools.Registry,
	version string,
	userService *user.Service,
	apiKeyService *auth.APIKeyService,
) {
	if target == nil {
		return
	}
	if providerPool != nil {
		target.SetProviders(&mgmtProviderAdapter{pool: providerPool})
	}
	target.SetSkills(&mgmtSkillAdapter{registry: skillRegistry, workspaceDir: workspaceDir})
	target.SetTools(&mgmtToolAdapter{registry: toolRegistry})
	target.SetSystem(&mgmtSystemAdapter{version: version, startTime: routesStartTime})
	if userService != nil {
		target.SetUsers(&mgmtUserAdapter{service: userService})
	}
	if apiKeyService != nil {
		target.SetAPIKeys(&mgmtAPIKeyAdapter{service: apiKeyService})
	}
}

func bindRuntimeMgmtUpgrade(target runtimeMgmtToolTarget, handler *update.Handler, otaChecker *update.OTAChecker, version string) {
	if target == nil {
		return
	}
	target.SetUpgrade(&mgmtUpgradeAdapter{
		handler:    handler,
		otaChecker: otaChecker,
		version:    version,
	})
}
