package bootstrap

func newRouteRuntimeExperienceSkillOptions(state *routeRegistrationState) routeRuntimeContractSkillOptions {
	return routeRuntimeContractSkillOptions{
		writeDB:         state.runtimeWriteDB,
		readDB:          state.runtimeReadDB,
		dataDir:         state.cfg.DataDir,
		appConfig:       state.deps.Config,
		services:        state.services,
		settings:        state.bootstrapSupport.settingsHandler,
		chat:            state.deps.ChatHandler,
		authPageV1Group: state.authSurface.authPageV1Group,
		ipcServer:       state.mediaRuntime.ipcServer,
		ctx:             state.deps.Ctx,
		logger:          state.logger,
		closers:         &state.deps.Closers,
		eventBroker:     state.deps.SSEBroker,
		skillEmbedFS:    state.deps.SkillEmbedFS,
		workspace:       state.deps.WorkspaceHandler,
	}
}
