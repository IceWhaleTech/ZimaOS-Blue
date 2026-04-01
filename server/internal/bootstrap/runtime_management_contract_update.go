package bootstrap

import (
	"context"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
)

func bindRouteRuntimeUpdateSupport(
	ctx context.Context,
	options routeRuntimeContractManagementSupportOptions,
) (*update.Handler, *update.OTAChecker) {
	updateHandler := update.NewHandler(
		routeRuntimeServerVersion(options.serverConfig),
		newRouteRuntimeUpdateConfig(options.config),
	)
	updateHandler.SetResumeRecoverer(buildUpdateResumeRecoverer(options.cronHandler, options.logger))
	if options.authPageV1Group != nil {
		updateHandler.RegisterRoutes(options.authPageV1Group(permission.PageSettings))
	}

	otaChecker := update.NewOTAChecker(
		routeRuntimeServerVersion(options.serverConfig),
		routeRuntimeServerDataDir(options.serverConfig),
		"",
	)
	updateHandler.SetOTAChecker(otaChecker)
	go otaChecker.Run(ctx)
	updateHandler.StartAutoUpdater(ctx)

	if options.logger != nil && options.config != nil {
		options.logger.Info("OTA update routes registered", zap.Bool("enabled", options.config.Update.Enabled))
	}
	return updateHandler, otaChecker
}

func newRouteRuntimeUpdateConfig(cfg *config.Config) *update.Config {
	if cfg == nil {
		return &update.Config{}
	}
	return &update.Config{
		Enabled:        cfg.Update.Enabled,
		CheckInterval:  cfg.Update.CheckInterval,
		AutoDownload:   cfg.Update.AutoDownload,
		AutoApply:      cfg.Update.AutoApply,
		ReleaseChannel: cfg.Update.ReleaseChannel,
		BackupCount:    cfg.Update.BackupCount,
		StoragePath:    cfg.Update.StoragePath,
	}
}
