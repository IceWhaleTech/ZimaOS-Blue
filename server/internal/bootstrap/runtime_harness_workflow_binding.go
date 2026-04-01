package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type workflowHarnessDriverBinding struct {
	controller *harness.Controller
}

func newWorkflowHarnessDriverBinding(bundle *HarnessRuntimeBundle) workflowHarnessDriverBinding {
	return workflowHarnessDriverBinding{controller: harnessRuntimeController(bundle)}
}

func (binding workflowHarnessDriverBinding) applyService(svc *workflow.WorkflowService) {
	if binding.controller == nil || svc == nil {
		return
	}
	binding.controller.RegisterDriver(harnessdrivers.NewWorkflowDriver(svc, binding.controller))
}

func (binding workflowHarnessDriverBinding) register(target workflowRuntimeHookTarget) {
	if target == nil || binding.controller == nil {
		return
	}
	target.SetServiceInitHook(binding.applyService)
}
