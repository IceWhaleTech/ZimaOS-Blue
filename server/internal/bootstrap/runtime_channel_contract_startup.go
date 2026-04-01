package bootstrap

import (
	"context"
	"runtime"

	"go.uber.org/zap"

	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func startRouteRuntimeEnabledChannelsAsync(
	enabledChannels []*serverpkg.ChannelConfig,
	store runtimeChannelConfigUpdater,
	factory runtimeChannelFactory,
	manager runtimeChannelLifecycleManager,
	logger *zap.Logger,
) {
	if len(enabledChannels) == 0 {
		return
	}
	go startRouteRuntimeEnabledChannels(enabledChannels, store, factory, manager, logger, runtime.GOOS)
}

func startRouteRuntimeEnabledChannels(
	enabledChannels []*serverpkg.ChannelConfig,
	store runtimeChannelConfigUpdater,
	factory runtimeChannelFactory,
	manager runtimeChannelLifecycleManager,
	logger *zap.Logger,
	goos string,
) {
	for _, chCfg := range enabledChannels {
		if !routeRuntimeChannelSupportedOnPlatform(chCfg.ID, goos) {
			if logger != nil {
				logger.Info("Skipping iMessage channel — not available on this platform")
			}
			continue
		}
		ch, err := factory.CreateChannel(chCfg)
		if err != nil || ch == nil {
			continue
		}
		if err := manager.Register(ch); err != nil {
			continue
		}
		if err := manager.StartChannel(context.Background(), chCfg.ID); err != nil {
			chCfg.Status = "error"
			chCfg.LastError = err.Error()
			if store != nil {
				_ = store.Set(chCfg.ID, chCfg)
			}
			continue
		}
		chCfg.Status = "connected"
		chCfg.LastError = ""
		if store != nil {
			_ = store.Set(chCfg.ID, chCfg)
		}
	}
	if logger != nil {
		logger.Info("Enabled channels started in background", zap.Int("count", len(enabledChannels)))
	}
}

func routeRuntimeChannelSupportedOnPlatform(channelID, goos string) bool {
	return channelID != "imessage" || goos == "darwin"
}
