package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func newWorkflowRuntimeBinding(
	registry *tools.Registry,
	approver tools.ToolApprover,
	observer tools.RuntimeEventObserver,
	metrics workflowRuntimeMetricsRecorder,
) workflowRuntimeBinding {
	gateway := newRuntimeToolGateway(registry, approver, observer, metrics)
	if gateway == nil {
		return workflowRuntimeBinding{}
	}
	return workflowRuntimeBinding{
		toolRuntime: workflowToolRuntimeAdapter{gateway: gateway},
		metrics:     metrics,
	}
}

func (binding workflowRuntimeBinding) apply(target workflowRuntimeApplyTarget) {
	if target == nil || binding.toolRuntime == nil {
		return
	}
	target.ApplyToolGateway(binding.toolRuntime)
	if binding.metrics != nil {
		target.ApplyMetricsRecorder(binding.metrics)
	}
}

func (binding workflowRuntimeBinding) register(handler workflowRuntimeHookTarget) {
	if handler == nil || binding.toolRuntime == nil {
		return
	}
	handler.SetServiceInitHook(func(svc *workflow.WorkflowService) {
		if svc == nil {
			return
		}
		svc.SetToolGateway(binding.toolRuntime)
		if binding.metrics != nil {
			svc.SetMetricsRecorder(workflowMetricsRecorderAdapter{recorder: binding.metrics})
		}
	})
}
