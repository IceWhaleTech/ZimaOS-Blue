package bootstrap

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// routeRuntimeContract is the route-registration-facing runtime boundary consumed by routes.go.
// Lower-level capability and support binders stay behind the internal runtimeContractBinding type.
type routeRuntimeContract interface {
	routeRuntimePhaseBoundary()
	NewRuntimeLLMRef() *runtimeLLMProviderRef
	BindStartupAuthRuntime(options routeRuntimeContractStartupAuthOptions) routeRuntimeContractStartupAuthResult
	BindBootstrapPhaseRuntime(options routeRuntimeContractBootstrapPhaseOptions) routeRuntimeContractBootstrapPhaseResult
	BindInfrastructureRuntime(options routeRuntimeContractInfrastructureOptions) routeRuntimeContractInfrastructureResult
	BindExperienceRuntime(options routeRuntimeContractExperienceOptions) routeRuntimeContractExperienceResult
	BindCoreToolingRuntime(options routeRuntimeContractCoreToolingOptions) routeRuntimeContractCoreToolingResult
	BindCoreSupportRuntime(options routeRuntimeContractCoreSupportOptions) routeRuntimeContractCoreSupportResult
	BindManagementRuntime(options routeRuntimeContractManagementRuntimeOptions) routeRuntimeContractManagementRuntimeResult
	BindOperationalRuntime(options routeRuntimeContractOperationalOptions) routeRuntimeContractOperationalResult
}

type runtimeContractBinding struct {
	runtime runtimeContractRuntimeBundle
}

func newRouteRuntimeContract(
	writeDB *sql.DB,
	readDB *sql.DB,
	cfg *config.Config,
	logger *zap.Logger,
	workspaceSource runtimeWorkspaceManagerSource,
	webSearchConfig tools.WebSearchConfig,
) routeRuntimeContract {
	return newRouteRuntimePhaseContract(newRuntimeContractBinding(
		writeDB,
		readDB,
		cfg,
		logger,
		workspaceSource,
		webSearchConfig,
	))
}

func newRuntimeContractBinding(
	writeDB *sql.DB,
	readDB *sql.DB,
	cfg *config.Config,
	logger *zap.Logger,
	workspaceSource runtimeWorkspaceManagerSource,
	webSearchConfig tools.WebSearchConfig,
) *runtimeContractBinding {
	return &runtimeContractBinding{
		runtime: newRuntimeContractRuntimeBundle(newRuntimeCapabilitySurface(
			writeDB,
			readDB,
			cfg,
			logger,
			workspaceSource,
			webSearchConfig,
		)),
	}
}
