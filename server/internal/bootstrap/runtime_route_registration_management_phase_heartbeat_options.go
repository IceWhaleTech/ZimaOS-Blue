package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"

func newRouteRuntimeManagementHeartbeatOptions(state *routeRegistrationState) routeRuntimeContractHeartbeatOptions {
	return routeRuntimeContractHeartbeatOptions{
		apiProtected: state.apiProtected,
		config: &heartbeat.Config{
			Enabled:      state.deps.Config.Heartbeat.Enabled,
			Interval:     state.deps.Config.Heartbeat.Interval,
			Prompt:       state.deps.Config.Heartbeat.Prompt,
			AckMaxChars:  state.deps.Config.Heartbeat.AckMaxChars,
			WorkspaceDir: state.deps.Config.Heartbeat.WorkspaceDir,
			LLMProvider:  state.deps.Config.Heartbeat.LLMProvider,
			LLMModel:     state.deps.Config.Heartbeat.LLMModel,
			Visibility: heartbeat.VisibilityConfig{
				ShowOk:       state.deps.Config.Heartbeat.Visibility.ShowOk,
				ShowAlerts:   state.deps.Config.Heartbeat.Visibility.ShowAlerts,
				UseIndicator: state.deps.Config.Heartbeat.Visibility.UseIndicator,
			},
		},
		dataDir:    state.dataDir,
		runtimeLLM: state.runtimeLLM,
		ctx:        state.deps.Ctx,
		logger:     state.logger,
	}
}
