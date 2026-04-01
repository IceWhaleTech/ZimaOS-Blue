package bootstrap

import "context"

func newRouteRuntimeOperationalDeferredOptions(state *routeRegistrationState) routeRuntimeContractDeferredSupportOptions {
	return routeRuntimeContractDeferredSupportOptions{
		ctx:                    context.Background(),
		services:               state.services,
		deps:                   state.deps,
		settings:               state.bootstrapSupport.settingsHandler,
		smallModelManager:      state.bootstrapSupport.smallModelManager,
		chatSmallModel:         state.deps.ChatHandler,
		analyzeSmallModel:      state.deps.AnalyzeTool,
		smallModelStats:        state.deps.ChatHandler,
		reflectionProposalGate: state.bootstrapSupport.settingsHandler.GetAgentAutoReflect,
		compactorChat:          state.deps.ChatHandler,
		memoryHandler:          state.deps.MemoryHandler,
		providerSettings:       state.managementSupport.providerSettings,
		chatSettings:           state.deps.ChatHandler,
		skillReranker:          state.skillAutoReranker,
		skillRerankerDefaults: runtimeSkillRerankerDefaults{
			dataDir:       state.cfg.DataDir,
			modelRepo:     state.deps.Config.ToolCalling.SkillRerankModel,
			rerankEnabled: state.deps.Config.ToolCalling.SkillRerankEnabled,
			onnxEnabled:   state.deps.Config.ToolCalling.SkillRerankONNXEnabled,
			autoDownload:  state.deps.Config.ToolCalling.SkillRerankONNXAutoDownload,
		},
		promptSettings:  state.deps.SystemPromptBuilder,
		pushLocale:      state.deps.PushService,
		mgmtSettings:    state.mgmtTool,
		questionMgr:     state.questionMgr,
		approvalHandler: state.bootstrapSupport.approvalHandler,
		execApprovals:   state.execApprovals,
		workflowTarget:  state.deps.WorkflowHandler,
		metrics:         state.deps.MetricsWriter,
		chatApprover:    state.deps.ChatHandler,
	}
}
