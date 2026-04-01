package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
)

func newAgentRuntimeBinding(bundle *HarnessRuntimeBundle, runner *agent.Runner, store *agent.Store) agentRuntimeBinding {
	if runner == nil || store == nil {
		return agentRuntimeBinding{}
	}
	controller := harnessRuntimeController(bundle)
	if controller == nil {
		return agentRuntimeBinding{}
	}
	agentDriver := harnessdrivers.NewAgentDriver(harness.RunKindAgentTask, runner, store, controller)
	subagentDriver := harnessdrivers.NewAgentDriver(harness.RunKindSubagent, runner, store, controller)
	return agentRuntimeBinding{
		taskObserver:     agentDriver,
		toolObserver:     harnessRuntimeObserver(bundle),
		subagentExecutor: bundle.SubagentExecutor,
		writeGuard:       bundle.WriteGuard,
		execGuard:        bundle.ExecGuard,
		agentDriver:      agentDriver,
		subagentDriver:   subagentDriver,
		useCompatHandler: true,
	}
}

func (binding agentRuntimeBinding) apply(runner agentRuntimeTarget) {
	if runner == nil {
		return
	}
	if binding.taskObserver != nil {
		runner.SetEventObserver(binding.taskObserver)
	}
	if binding.toolObserver != nil {
		runner.SetToolEventObserver(binding.toolObserver)
	}
	if binding.subagentExecutor != nil {
		runner.SetSubagentExecutor(binding.subagentExecutor)
	}
	if binding.writeGuard != nil {
		runner.SetWritePathGuard(binding.writeGuard)
	}
	if binding.execGuard != nil {
		runner.SetExecPathGuard(binding.execGuard)
	}
}

func (binding agentRuntimeBinding) register(controller *harness.Controller) {
	if controller == nil {
		return
	}
	if binding.agentDriver != nil {
		registerHarnessRuntimeAgentDriver(controller, binding.agentDriver)
	}
	if binding.subagentDriver != nil {
		controller.RegisterDriver(binding.subagentDriver)
	}
}

func bindHarnessRuntimeToAgentRunner(bundle *HarnessRuntimeBundle, agentGroup *echo.Group, runner *agent.Runner, store *agent.Store, workspaceDir string) {
	if agentGroup == nil || runner == nil || store == nil {
		return
	}
	controller := harnessRuntimeController(bundle)
	binding := newAgentRuntimeBinding(bundle, runner, store)
	binding.apply(runner)
	binding.register(controller)
	binding.routeRegistrar(controller, store, runner, workspaceDir).RegisterRoutes(agentGroup)
}
