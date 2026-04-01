package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

type routeRuntimeCoreToolingBinding interface {
	BindSchedulerServices(options routeRuntimeContractSchedulerOptions)
	BindTooling(options routeRuntimeContractToolingOptions) routeRuntimeContractToolingResult
	BindAnalyzeTool(options routeRuntimeContractAnalyzeOptions) *tools.AnalyzeTool
	BindProviderPoolRuntime(options routeRuntimeContractProviderPoolOptions) routeRuntimeContractProviderPoolResult
}

var _ routeRuntimeCoreToolingBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractCoreToolingOptions struct {
	scheduler routeRuntimeContractSchedulerOptions
	tooling   routeRuntimeContractToolingOptions
	analyze   routeRuntimeContractAnalyzeOptions
	provider  routeRuntimeContractProviderPoolOptions
}

type routeRuntimeContractCoreToolingResult struct {
	schedulerBound bool
	tooling        routeRuntimeContractToolingResult
	analyzeTool    *tools.AnalyzeTool
	provider       routeRuntimeContractProviderPoolResult
}
