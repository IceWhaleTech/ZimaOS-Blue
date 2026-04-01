package bootstrap

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRegistrationState struct {
	e                     *echo.Echo
	deps                  *RoutesDeps
	services              *Services
	cfg                   *ServerConfig
	logger                *zap.Logger
	registerStart         time.Time
	trace                 *StartupTrace
	dataDir               string
	workspaceDir          string
	workspaceAllowedPaths []string
	webSearchConfig       tools.WebSearchConfig
	runtimeWriteDB        *sql.DB
	runtimeReadDB         *sql.DB
	runtimeContract       routeRuntimeContract
	runtimeLLM            *runtimeLLMProviderRef
	flagEvaluator         *config.FlagEvaluator
	connManager           *connection.Manager
	v1                    *echo.Group
	api                   *echo.Group
	authSurface           routeRuntimeContractAuthSurfaceResult
	protected             *echo.Group
	apiProtected          *echo.Group
	mediaDir              string
	runtimeSnapshot       routeRegistrationRuntimeSnapshot
	startupAuthRuntime    routeRuntimeContractStartupAuthResult
	bootstrapPhaseRuntime routeRuntimeContractBootstrapPhaseResult
	bootstrapSupport      routeRuntimeContractBootstrapSupportResult
	infrastructureRuntime routeRuntimeContractInfrastructureResult
	mediaRuntime          routeRuntimeContractMediaResult
	gatewayRuntime        routeRuntimeContractGatewayResult
	experienceRuntime     routeRuntimeContractExperienceResult
	coreToolingRuntime    routeRuntimeContractCoreToolingResult
	coreSupportRuntime    routeRuntimeContractCoreSupportResult
	chatRuntime           routeRuntimeContractChatBindingResult
	skillRuntime          routeRuntimeContractSkillResult
	providerRuntime       routeRuntimeContractProviderPoolResult
	operationalRuntime    routeRuntimeContractOperationalResult
	askSupport            runtimeAskSupportBundle
	questionMgr           *tools.QuestionManager
	execSupport           runtimeExecSupportBundle
	execApprovals         *tools.ApprovalManager
	managementRuntime     routeRuntimeContractManagementRuntimeResult
	managementSupport     routeRuntimeContractManagementSupportResult
	mgmtTool              *tools.MgmtTool
	skillAutoReranker     *agentcore.AutoSkillReranker
}

func newRouteRegistrationState(e *echo.Echo, deps *RoutesDeps) *routeRegistrationState {
	state := newRouteRegistrationStateBase(e, deps)
	bindRouteRegistrationStateRuntime(state)
	return state
}

func (state *routeRegistrationState) lookupProviderAPIKey(providerID string) (string, error) {
	if state == nil || state.deps == nil || state.deps.ProviderPool == nil || state.deps.ProviderPool.Registry == nil {
		return "", fmt.Errorf("provider pool registry is unavailable")
	}
	apiKey, err := state.deps.ProviderPool.Registry.GetAPIKey(providerID)
	if err != nil || apiKey == nil {
		return "", err
	}
	return apiKey.Key, nil
}
