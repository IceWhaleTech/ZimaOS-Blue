package bootstrap

import (
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type routeRuntimeContractSchedulerOptions struct {
	registry         *tools.Registry
	skillRegistry    runtimeSkillRegistrySource
	workflowResolver func() *workflow.WorkflowService
	workflowHandler  *workflow.Handler
	workspaceDir     string
	cronHandler      runtimeCronHandlerTarget
	scheduler        runtimeSchedulerSkillTarget
	calendar         runtimeSchedulerCalendarTarget
	memoryStore      *memory.Store
	broker           *sse.Broker
	logger           *zap.Logger
}
