package bootstrap

import (
	"fmt"
	"log/slog"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
)

func registerHarnessRuntimeAgentDriver(controller *harness.Controller, driver *harnessdrivers.AgentDriver) {
	if existing := controller.GetRegisteredDriver(driver.Kind()); existing != nil {
		if mux, ok := existing.(*harnessdrivers.DriverMux); ok {
			mux.SetDefaultDriver(driver)
			controller.RegisterDriver(mux)
			slog.Info("Harness agent driver registered", "mode", "mux_default_update", "existing_driver_type", fmt.Sprintf("%T", existing))
			return
		}
		controller.RegisterDriver(driver)
		slog.Info("Harness agent driver registered", "mode", "replace_non_mux", "existing_driver_type", fmt.Sprintf("%T", existing))
		return
	}
	controller.RegisterDriver(driver)
	slog.Info("Harness agent driver registered", "mode", "no_existing_driver")
}

func (binding agentRuntimeBinding) routeRegistrar(controller *harness.Controller, store *agent.Store, runner *agent.Runner, workspaceDir string) agentRouteRegistrar {
	if binding.useCompatHandler && controller != nil {
		return harness.NewAgentCompatHandler(controller, store, runner, workspaceDir)
	}
	return agent.NewHandler(store, runner)
}
