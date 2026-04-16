package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

const dreamCronHandlerName = "memory-dream-run"

func BindRuntimeDreamCron(handler *cron.Handler, service *memory.DreamService) {
	if handler == nil || service == nil || !service.Config().Enabled {
		return
	}

	cfg := service.Config()
	handler.SetServiceInitHook(func(svc *cron.Service) {
		if svc == nil {
			return
		}
		svc.RegisterHandler(dreamCronHandlerName, func(ctx context.Context, job *cron.Job) (interface{}, error) {
			return service.RunConsolidation(ctx)
		})

		for _, job := range svc.List() {
			if job != nil && job.Handler == dreamCronHandlerName {
				return
			}
		}

		_, _ = svc.Create(
			"Dream Memory Consolidation",
			"System dream pass for archived sessions and aged daily logs",
			cfg.Schedule,
			dreamCronHandlerName,
			map[string]interface{}{"system": true},
		)
	})
}
