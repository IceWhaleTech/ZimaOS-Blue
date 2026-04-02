package bootstrap

import (
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindHarnessRuntimeApproval(
	bundle *HarnessRuntimeBundle,
	handler *networkapi.ApprovalHandler,
	execApprovals *tools.ApprovalManager,
	registry *tools.Registry,
	auxiliaryLLM llm.Provider,
	metrics workflowRuntimeMetricsRecorder,
	detailTarget approvalRuntimeDetailTarget,
	handlerTarget approvalRuntimeHandlerTarget,
	workflowTarget workflowRuntimeHookTarget,
	approverTargets ...runtimeToolApproverTarget,
) {
	binding := newApprovalRuntimeBinding(bundle, handler, execApprovals, registry, auxiliaryLLM, metrics)
	binding.applyDetail(detailTarget)
	binding.applyHandler(handlerTarget)
	newWorkflowHarnessDriverBinding(bundle).register(workflowTarget)
	for _, target := range approverTargets {
		binding.applyApprover(target)
	}
	binding.registerWorkflow(workflowTarget)
}
