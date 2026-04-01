package bootstrap

import (
	"context"
	"path/filepath"
	"strings"

	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func (binding *runtimeContractBinding) BindSkillRuntime(
	options routeRuntimeContractSkillOptions,
) routeRuntimeContractSkillResult {
	if binding == nil {
		return routeRuntimeContractSkillResult{}
	}
	return bindRouteRuntimeSkills(options)
}

func bindRouteRuntimeSkills(options routeRuntimeContractSkillOptions) routeRuntimeContractSkillResult {
	result := routeRuntimeContractSkillResult{
		skillsDir: filepath.Join(strings.TrimSpace(options.dataDir), "workspace", ".claude", "skills"),
	}
	if options.services == nil {
		return result
	}

	ctx := options.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	logger := options.logger
	services := options.services
	skillHandler := serverpkg.NewSkillHandler(services.SkillRegistry)

	skillStoreDb := bindRouteRuntimeSkillStore(skillHandler, options, ctx)
	result.storeBound = skillStoreDb != nil
	localScanner := bindRouteRuntimeSkillSupport(skillHandler, options, &result)

	if options.appConfig == nil || options.appConfig.SkillMarket.Enabled {
		bindRouteRuntimeSkillMarketplace(skillHandler, localScanner, options, ctx)
		result.marketplaceConfigured = true
		if options.closers != nil {
			*options.closers = append(*options.closers, skillHandler)
			result.closerRegistered = true
		}
		if logger != nil {
			logger.Info("Skill marketplace registered for lazy initialization")
		}
	}

	bindRouteRuntimeSkillRoutes(skillHandler, options, &result)
	bindRouteRuntimeSkillIPC(skillStoreDb, localScanner, options, &result)
	releaseRouteRuntimeEmbeddedSkills(localScanner, options)

	return result
}
