package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func newRouteRuntimeCoreSchedulerOptions(state *routeRegistrationState) routeRuntimeContractSchedulerOptions {
	return routeRuntimeContractSchedulerOptions{
		registry:      state.services.ToolRegistry,
		skillRegistry: state.services.SkillRegistry,
		workflowResolver: func() *workflow.WorkflowService {
			if state.deps.WorkflowHandler == nil {
				return nil
			}
			return state.deps.WorkflowHandler.GetService()
		},
		workflowHandler: state.deps.WorkflowHandler,
		workspaceDir:    state.workspaceDir,
		cronHandler:     state.deps.CronHandler,
		calendar:        tools.GetCalendarTool(state.services.ToolRegistry),
		memoryStore:     state.services.MemoryStore,
		broker:          state.deps.SSEBroker,
		logger:          state.logger,
	}
}
