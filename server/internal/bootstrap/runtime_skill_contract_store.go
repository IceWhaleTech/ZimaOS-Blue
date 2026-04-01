package bootstrap

import (
	"context"
	"log/slog"

	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"go.uber.org/zap"
)

func bindRouteRuntimeSkillStore(
	handler *serverpkg.SkillHandler,
	options routeRuntimeContractSkillOptions,
	ctx context.Context,
) *skillstore.Store {
	if handler == nil || options.writeDB == nil {
		return nil
	}

	readDB := options.readDB
	if readDB == nil {
		readDB = options.writeDB
	}
	skillStoreDb, err := skillstore.NewStoreWithReadDB(options.writeDB, readDB)
	if err != nil {
		if options.logger != nil {
			options.logger.Warn("Failed to initialize skill store", zap.Error(err))
		}
		return nil
	}
	handler.SetStore(skillStoreDb)
	if options.logger != nil {
		options.logger.Info("Skill store initialized (catalog only)")
	}

	syncConfig := skillstore.DefaultSyncServiceConfig()
	syncService := skillstore.NewSyncService(skillStoreDb, syncConfig, slog.Default())
	handler.SetSyncService(syncService)
	syncService.Start(ctx)
	if options.logger != nil {
		options.logger.Info("Skill sync service configured for lazy initialization")
	}
	return skillStoreDb
}
