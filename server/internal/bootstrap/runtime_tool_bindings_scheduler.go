package bootstrap

import (
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/inject"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

func bindRuntimeSchedulerServices(
	registry *tools.Registry,
	workflowResolver func() *workflow.WorkflowService,
	workflowHandler *workflow.Handler,
	harnessRuntime *HarnessRuntimeBundle,
	workspaceDir string,
	cronHandler runtimeCronHandlerTarget,
	scheduler runtimeSchedulerSkillTarget,
	calendar runtimeSchedulerCalendarTarget,
	memoryStore *memory.Store,
	broker *sse.Broker,
	deepResearchService *deepresearch.Service,
	logger *zap.Logger,
) {
	bindRuntimeWorkflowExecution(registry, workflowResolver, workflowHandler, harnessRuntime, workspaceDir)
	if cronHandler == nil {
		return
	}
	if registry != nil && workflowResolver != nil && registry.Get("nodes") == nil {
		tools.RegisterLazyNodesTool(registry, workflowResolver)
	}
	cronAdapter := cron.NewSkillAdapter(cronHandler.GetService)
	if scheduler != nil {
		scheduler.SetCronService(cronAdapter)
	}
	cronTools := cronToolAdapter{resolve: cronHandler.GetService}
	if registry != nil {
		tools.RegisterCronTool(registry, cronTools)
	}
	if calendar != nil {
		calendar.SetCronService(cronTools)
	}

	cronHandler.SetServiceInitHook(func(svc *cron.Service) {
		if svc == nil {
			return
		}
		svc.SetMessageInjector(inject.NewMemoryStoreInjector(memoryStore))
		if broker != nil {
			svc.SetEventPublisher(broker)
		}
		svc.RegisterCommandHandler(cron.CommandSecurityConfig{
			Enabled:          true,
			RequireAdminRole: true,
		})
		registerDeepResearchCronHandler(svc, deepResearchService, logger)
	})
}
