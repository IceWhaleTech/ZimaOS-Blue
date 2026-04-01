package bootstrap

import (
	"context"
	"runtime"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	serviceutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/service"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voicewake"
)

func registerRouteRuntimeVoiceWakeSurface(
	settingsHandler *serverpkg.SettingsHandler,
	options routeRuntimeContractBootstrapSupportOptions,
) {
	if settingsHandler == nil || options.chatHandler == nil {
		return
	}

	mode := routeRuntimeServerMode(options.serverConfig)
	supported := runtime.GOOS == "darwin" &&
		(strings.EqualFold(mode, "embedded") || serviceutil.IsInteractive())

	voiceWakeManager := voicewake.NewManager(voicewake.ManagerConfig{
		Settings:  settingsHandler,
		Submitter: options.chatHandler,
		Supported: supported,
	})
	settingsHandler.SetVoiceWakeManager(voiceWakeManager)
	if options.protected != nil {
		voicewake.NewHandler(voiceWakeManager).RegisterRoutes(options.protected.Group(
			"/voice-wake",
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageSettings))...,
		))
	}

	if options.ctx != nil {
		go func(done <-chan struct{}) {
			<-done
			_ = voiceWakeManager.Close()
		}(options.ctx.Done())
		if settingsHandler.GetVoiceWakeEnabled() {
			_ = voiceWakeManager.Refresh(options.ctx)
		}
		return
	}

	if settingsHandler.GetVoiceWakeEnabled() {
		_ = voiceWakeManager.Refresh(context.Background())
	}
}
