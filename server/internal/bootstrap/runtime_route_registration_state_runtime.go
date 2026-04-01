package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRegistrationEntrySnapshot struct {
	connManager *connection.Manager
	v1          *echo.Group
	api         *echo.Group
	mediaDir    string
}

type routeRegistrationRuntimeSnapshot struct {
	entry             routeRegistrationEntrySnapshot
	utility           routeRegistrationUtilitySnapshot
	startupAuth       routeRuntimeContractStartupAuthResult
	bootstrapPhase    routeRuntimeContractBootstrapPhaseResult
	auth              routeRuntimeContractAuthSurfaceResult
	bootstrap         routeRuntimeContractBootstrapSupportResult
	infrastructure    routeRuntimeContractInfrastructureResult
	media             routeRuntimeContractMediaResult
	gateway           routeRuntimeContractGatewayResult
	experience        routeRuntimeContractExperienceResult
	coreTooling       routeRuntimeContractCoreToolingResult
	coreSupport       routeRuntimeContractCoreSupportResult
	chat              routeRuntimeContractChatBindingResult
	skill             routeRuntimeContractSkillResult
	ask               runtimeAskSupportBundle
	exec              runtimeExecSupportBundle
	provider          routeRuntimeContractProviderPoolResult
	managementRuntime routeRuntimeContractManagementRuntimeResult
	management        routeRuntimeContractManagementSupportResult
	operational       routeRuntimeContractOperationalResult
}

type routeRegistrationUtilitySnapshot struct {
	skillAutoReranker *agentcore.AutoSkillReranker
	questionMgr       *tools.QuestionManager
	execApprovals     *tools.ApprovalManager
	mgmtTool          *tools.MgmtTool
}
