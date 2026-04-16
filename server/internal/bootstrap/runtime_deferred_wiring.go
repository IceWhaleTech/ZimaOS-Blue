package bootstrap

import (
	"context"
)

func bindDeferredRuntimeWiring(ctx context.Context, wiring runtimeDeferredWiring) {
	bindRuntimeSmallModel(
		wiring.settings,
		wiring.smallRuntime,
		wiring.chatSmallModel,
		wiring.auxiliarySmallModel,
		nil,
		wiring.analyzeSmallModel,
		wiring.smallModelStats,
	)
	activateHarnessRuntime(ctx, wiring.harnessRuntime, wiring.reflectionTarget, wiring.reflectionLLM, wiring.reflectionProposalGate)
	bindRuntimeSelectorDryRun(wiring.settings, wiring.harnessRuntime)
	bindRuntimeCompactorMemory(
		wiring.compactorChat,
		wiring.memoryHandler,
		wiring.auxiliaryLLM,
		wiring.sessionCompaction,
		wiring.sessionMaxTokens,
	)
	bindRuntimeSettingsTargets(wiring.settings, wiring.providerSettings, wiring.chatSettings, wiring.researchSettings, nil, nil, nil, nil, nil)
	bindRuntimeSkillReranker(wiring.settings, wiring.skillReranker, wiring.skillRerankerDefaults)
	bindRuntimeSettingsTargets(wiring.settings, nil, nil, nil, wiring.execAutoConfirm, wiring.promptSettings, wiring.pushLocale, wiring.maskerLocale, wiring.mgmtSettings)
	bindRuntimeAskPolicy(wiring.settings, wiring.questionMgr, wiring.agentRunner)
}

func bindDeferredRuntimeApproval(wiring runtimeDeferredApprovalWiring) {
	bindHarnessRuntimeApproval(
		wiring.harnessRuntime,
		wiring.handler,
		wiring.execApprovals,
		wiring.registry,
		wiring.auxiliaryLLM,
		wiring.metrics,
		wiring.detailTarget,
		wiring.handlerTarget,
		wiring.workflowTarget,
		wiring.approverTargets...,
	)
}
