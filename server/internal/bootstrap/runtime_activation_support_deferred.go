package bootstrap

import (
	"context"

	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
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
	bindDeferredRuntimeWiring(ctx, newRuntimeActivationDeferredWiring(activation, options, runner, autoDownload))
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
