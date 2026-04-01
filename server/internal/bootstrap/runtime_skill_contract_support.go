package bootstrap

import (
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"go.uber.org/zap"
)

func bindRouteRuntimeSkillSupport(
	handler *serverpkg.SkillHandler,
	options routeRuntimeContractSkillOptions,
	result *routeRuntimeContractSkillResult,
) *skillstore.LocalSkillScanner {
	if handler == nil || result == nil {
		return nil
	}

	handler.SetSkillsDir(result.skillsDir)
	if options.eventBroker != nil {
		handler.SetEventBroker(options.eventBroker)
	}

	featuredDataPath := filepath.Join(strings.TrimSpace(options.dataDir), "featured_skills.json")
	handler.SetFeaturedLoader(skillstore.NewFeaturedSkillsLoader(featuredDataPath))
	result.featuredLoaderConfigured = true
	if options.logger != nil {
		options.logger.Info("Featured skills loader configured for lazy initialization", zap.String("path", featuredDataPath))
	}

	localScanner := skillstore.NewLocalSkillScannerWithRoots(skillmanifest.ResolvePeerRootsForManagedDir(result.skillsDir))
	handler.SetLocalScanner(localScanner)
	result.localScannerConfigured = true
	if options.logger != nil {
		options.logger.Info("Local skill scanner configured for lazy initialization", zap.String("path", result.skillsDir))
	}
	return localScanner
}

func bindRouteRuntimeSkillRoutes(
	handler *serverpkg.SkillHandler,
	options routeRuntimeContractSkillOptions,
	result *routeRuntimeContractSkillResult,
) {
	if handler == nil || result == nil || options.authPageV1Group == nil {
		return
	}
	handler.RegisterRoutes(options.authPageV1Group(permission.PageSkills))
	result.routesRegistered = true
}

func bindRouteRuntimeSkillIPC(
	skillStoreDb *skillstore.Store,
	localScanner *skillstore.LocalSkillScanner,
	options routeRuntimeContractSkillOptions,
	result *routeRuntimeContractSkillResult,
) {
	if result == nil || options.ipcServer == nil || options.services == nil {
		return
	}
	skillMgr := newSkillManagerAdapter(skillStoreDb, options.services.SkillRegistry, localScanner, result.skillsDir)
	sockipc.RegisterSkillManagerHandlers(options.ipcServer, skillMgr, options.logger)
	result.ipcHandlersRegistered = true
	if options.logger != nil {
		options.logger.Info("Skill manager IPC handlers registered")
	}
}

func releaseRouteRuntimeEmbeddedSkills(
	localScanner *skillstore.LocalSkillScanner,
	options routeRuntimeContractSkillOptions,
) {
	if localScanner == nil || options.skillEmbedFS == nil || options.workspace == nil {
		return
	}
	if mgr := options.workspace.Manager(); mgr != nil {
		if err := mgr.ReleaseSkills(options.skillEmbedFS); err != nil {
			if options.logger != nil {
				options.logger.Warn("Failed to release embedded skills", zap.Error(err))
			}
		}
		localScanner.Invalidate()
	}
}
