package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func selectHarnessRuntimeObserver(controller *harness.Controller, cfg config.RuntimeReflectionConfig, hasReflectService bool) tools.RuntimeEventObserver {
	runtimeObserver := harness.NewRuntimeObserver(controller)
	if !hasReflectService || !cfg.Enabled {
		return runtimeObserver
	}
	coordinator := harness.NewRuntimeReflectionCoordinator(controller, cfg)
	controller.SetRuntimeReflectionCoordinator(coordinator)
	return harness.NewRuntimeEventObserverMux(runtimeObserver, coordinator)
}

func buildHarnessRuntimeBundle(controller *harness.Controller, runTracer *harness.RunTraceCollector, runtimeObserver tools.RuntimeEventObserver, agents *config.AgentsConfig) *HarnessRuntimeBundle {
	return &HarnessRuntimeBundle{
		Controller:       controller,
		GroupDispatcher:  harness.NewGroupDispatcher(controller),
		RunTracer:        runTracer,
		RuntimeObserver:  runtimeObserver,
		SubagentExecutor: harness.NewSubagentExecutor(controller, agents),
		WriteGuard:       harness.NewWritePathGuard(controller),
		ExecGuard:        harness.NewExecPathGuard(controller),
	}
}
