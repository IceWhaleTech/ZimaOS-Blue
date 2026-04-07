package bootstrap

import (
	"context"

	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

func bindRuntimeActivationDeferred(
	ctx context.Context,
	activation runtimeActivationResult,
	options runtimeActivationDeferredSupportOptions,
	runner agentRuntimePolicyTarget,
) {
	bindHarnessRuntimeOptimization(options.settings, options.harnessRuntime)
	autoDownload := true
	if options.settings != nil {
		autoDownload = options.settings.GetSmallModelAutoDownload()
	}
	bindDeferredRuntimeWiring(ctx, runtimeDeferredWiring{
		settings:               options.settings,
		smallRuntime:           smallmodel.NewLlamaCppRuntime(options.smallModelManager, smallmodel.LlamaCppRuntimeOptions{AutoDownload: autoDownload}),
		chatSmallModel:         options.chatSmallModel,
		auxiliarySmallModel:    activation.auxiliaryLLM,
		imageSmallModel:        runtimeActivationImageTool(options.services),
		analyzeSmallModel:      options.analyzeSmallModel,
		smallModelStats:        options.smallModelStats,
		harnessRuntime:         options.harnessRuntime,
		reflectionTarget:       options.reflectionTarget,
		reflectionLLM:          activation.auxiliaryLLM,
		reflectionProposalGate: options.reflectionProposalGate,
		compactorChat:          options.compactorChat,
		memoryHandler:          options.memoryHandler,
		auxiliaryLLM:           activation.auxiliaryLLM,
		sessionCompaction:      runtimeActivationSessionCompaction(options.deps),
		sessionMaxTokens:       runtimeActivationSessionMaxTokens(options.deps),
		providerSettings:       options.providerSettings,
		chatSettings:           options.chatSettings,
		researchSettings:       options.researchSettings,
		skillReranker:          options.skillReranker,
		skillRerankerDefaults:  options.skillRerankerDefaults,
		execAutoConfirm:        runtimeActivationExecAutoConfirm(options.services),
		promptSettings:         options.promptSettings,
		pushLocale:             options.pushLocale,
		maskerLocale:           activation.dataMasker,
		mgmtSettings:           options.mgmtSettings,
		questionMgr:            options.questionMgr,
		agentRunner:            runner,
	})
}

func bindRuntimeActivationApproval(
	activation runtimeActivationResult,
	options runtimeActivationDeferredSupportOptions,
) {
	if chatHandler, ok := options.chatApprover.(*serverpkg.ChatHandler); ok {
		chatHandler.SetConversationBootstrapToolApprovalSource(options.approvalHandler)
		chatHandler.SetConversationBootstrapExecApprovalSource(options.execApprovals)
	}
	approverTargets := make([]runtimeToolApproverTarget, 0, 2)
	if options.chatApprover != nil {
		approverTargets = append(approverTargets, options.chatApprover)
	}
	if activation.agentRunner != nil {
		approverTargets = append(approverTargets, activation.agentRunner)
	}
	bindDeferredRuntimeApproval(runtimeDeferredApprovalWiring{
		handler:         options.approvalHandler,
		harnessRuntime:  options.harnessRuntime,
		execApprovals:   options.execApprovals,
		registry:        runtimeActivationToolRegistry(options.services),
		auxiliaryLLM:    activation.auxiliaryLLM,
		metrics:         options.metrics,
		detailTarget:    options.detailTarget,
		handlerTarget:   options.approvalHandler,
		workflowTarget:  options.workflowTarget,
		approverTargets: approverTargets,
	})
}
