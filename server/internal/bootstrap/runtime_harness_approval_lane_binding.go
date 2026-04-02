package bootstrap

import (
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newHarnessRuntimeExecApprovals(bundle *HarnessRuntimeBundle, broker *sse.Broker) *tools.ApprovalManager {
	if broker == nil {
		return nil
	}
	approvals := tools.NewApprovalManager(broker)
	bindHarnessRuntimeObserver(bundle, approvals)
	return approvals
}

func newApprovalRuntimeBinding(
	bundle *HarnessRuntimeBundle,
	handler *networkapi.ApprovalHandler,
	execApprovals *tools.ApprovalManager,
	registry *tools.Registry,
	auxiliaryLLM llm.Provider,
	metrics workflowRuntimeMetricsRecorder,
) approvalRuntimeBinding {
	if handler == nil {
		return approvalRuntimeBinding{}
	}
	handler.SetRiskScorer(networkapi.NewLLMToolApprovalRiskScorer(auxiliaryLLM, registry))
	observer := harnessRuntimeObserver(bundle)
	binding := approvalRuntimeBinding{
		handler:  handler,
		approver: handler,
		observer: observer,
		workflow: newWorkflowRuntimeBinding(registry, handler, observer, metrics),
	}
	if execApprovals != nil {
		binding.execResolver = execApprovalAdapter{mgr: execApprovals}
	}
	return binding
}

func (binding approvalRuntimeBinding) applyDetail(target approvalRuntimeDetailTarget) {
	if target == nil || binding.handler == nil {
		return
	}
	target.SetApprovalHandler(binding.handler)
}

func (binding approvalRuntimeBinding) applyHandler(target approvalRuntimeHandlerTarget) {
	if target == nil {
		return
	}
	if binding.execResolver != nil {
		target.SetExecResolver(binding.execResolver)
	}
	if binding.observer != nil {
		target.SetObserver(binding.observer)
	}
}

func (binding approvalRuntimeBinding) applyApprover(target runtimeToolApproverTarget) {
	bindRuntimeToolApprover(binding.approver, target)
}

func (binding approvalRuntimeBinding) registerWorkflow(target workflowRuntimeHookTarget) {
	binding.workflow.register(target)
}
