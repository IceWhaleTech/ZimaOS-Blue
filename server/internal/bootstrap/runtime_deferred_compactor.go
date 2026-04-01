package bootstrap

import (
	"fmt"
	"log/slog"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
)

func bindRuntimeCompactorMemory(
	chat runtimeCompactorMemoryChatTarget,
	memoryHandler *serverpkg.MemoryHandler,
	auxiliary llm.Provider,
	compaction config.SessionCompactionConfig,
	sessionMaxTokens int,
) {
	if chat == nil {
		return
	}

	memoryRefreshCfg := session.DefaultMemoryRefreshConfig()
	memoryRefreshCfg.Enabled = memoryRefreshCfg.Enabled && compaction.Enabled && memoryHandler != nil
	if memoryRefreshCfg.Enabled {
		sessionCompactor := session.NewSessionCompactor(auxiliary, compaction)
		sessionMemRefresher := serverpkg.NewSessionMemoryRefresher(memoryHandler)
		chat.SetCompactorMemoryIntegration(session.NewCompactorMemoryIntegration(
			sessionCompactor,
			sessionMemRefresher,
			auxiliary,
			memoryRefreshCfg,
		), sessionMaxTokens)
		return
	}

	chat.SetCompactorMemoryIntegration(nil, sessionMaxTokens)
}

func bindRuntimeSelectorDryRun(settings *serverpkg.SettingsHandler, bundle *HarnessRuntimeBundle) {
	if settings == nil || bundle == nil || bundle.Controller == nil {
		slog.Warn("Harness selector dry-run binding skipped", "settings_nil", settings == nil, "bundle_nil", bundle == nil, "controller_nil", bundle == nil || bundle.Controller == nil)
		return
	}
	existing := bundle.Controller.GetRegisteredDriver(harness.RunKindAgentTask)
	selectorDriver := harnessdrivers.NewSelectorDryRunDriver(settings)
	if mux, ok := existing.(*harnessdrivers.DriverMux); ok {
		mux.Register("selector_dry_run", selectorDriver)
		bundle.Controller.RegisterDriver(mux)
		slog.Info("Harness selector dry-run driver registered", "mode", "existing_mux", "default_driver_type", fmt.Sprintf("%T", existing))
		return
	}
	mux := harnessdrivers.NewDriverMux(harness.RunKindAgentTask, existing)
	mux.Register("selector_dry_run", selectorDriver)
	bundle.Controller.RegisterDriver(mux)
	slog.Info("Harness selector dry-run driver registered", "mode", "new_mux", "default_driver_type", fmt.Sprintf("%T", existing))
}
