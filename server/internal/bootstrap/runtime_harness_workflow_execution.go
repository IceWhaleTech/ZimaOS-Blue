package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type harnessWorkflowExecutionLauncher struct {
	manager              *harness.Controller
	resolve              func() *workflow.WorkflowService
	defaultWorkspaceRoot string
}

type harnessWorkflowNodesService struct {
	resolve  func() *workflow.WorkflowService
	launcher workflow.ExecutionLauncher
}

func bindRuntimeWorkflowExecution(
	registry *tools.Registry,
	workflowResolver func() *workflow.WorkflowService,
	workflowHandler *workflow.Handler,
	bundle *HarnessRuntimeBundle,
	workspaceDir string,
) {
	launcher := newHarnessWorkflowExecutionLauncher(harnessRuntimeController(bundle), workflowResolver, workspaceDir)
	if workflowHandler != nil && launcher != nil {
		workflowHandler.SetExecutionLauncher(launcher)
		workflowHandler.SetServiceInitHook(func(svc *workflow.WorkflowService) {
			if svc == nil {
				return
			}
			svc.SetExecutionLauncher(launcher)
		})
	}
	if registry == nil {
		return
	}
	if launcher != nil {
		registry.Register(tools.NewNodesTool(newHarnessWorkflowNodesService(workflowResolver, launcher)))
		return
	}
	if workflowResolver != nil {
		tools.RegisterLazyNodesTool(registry, workflowResolver)
	}
}

func newHarnessWorkflowExecutionLauncher(
	manager *harness.Controller,
	resolve func() *workflow.WorkflowService,
	defaultWorkspaceRoot string,
) workflow.ExecutionLauncher {
	if manager == nil || resolve == nil {
		return nil
	}
	return &harnessWorkflowExecutionLauncher{
		manager:              manager,
		resolve:              resolve,
		defaultWorkspaceRoot: strings.TrimSpace(defaultWorkspaceRoot),
	}
}

func newHarnessWorkflowNodesService(resolve func() *workflow.WorkflowService, launcher workflow.ExecutionLauncher) tools.NodesService {
	return harnessWorkflowNodesService{resolve: resolve, launcher: launcher}
}

var _ workflow.ExecutionLauncher = (*harnessWorkflowExecutionLauncher)(nil)
var _ tools.NodesService = harnessWorkflowNodesService{}
