package bootstrap

import (
	"context"

	"github.com/labstack/echo/v4"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeActivationRouteSupportOptions struct {
	v1             *echo.Group
	authMiddleware echo.MiddlewareFunc
	pageMiddleware echo.MiddlewareFunc
	memoryHandler  memoryRouteRegistrar
	chat           layeredMemoryChatTarget
	reflector      layeredMemoryReflectionTarget
}

type runtimeActivationDeferredSupportOptions struct {
	ctx                    context.Context
	services               *Services
	deps                   *RoutesDeps
	settings               *serverpkg.SettingsHandler
	smallModelManager      *smallmodel.Manager
	chatSmallModel         runtimeSmallModelChatTarget
	analyzeSmallModel      runtimeAnalyzeSmallModelTarget
	smallModelStats        runtimeSmallModelStatsSource
	harnessRuntime         *HarnessRuntimeBundle
	reflectionTarget       reflectionRuntimeTarget
	reflectionProposalGate func() bool
	compactorChat          runtimeCompactorMemoryChatTarget
	memoryHandler          *serverpkg.MemoryHandler
	providerSettings       runtimeSettingsHandlerTarget
	chatSettings           runtimeSettingsHandlerTarget
	researchSettings       runtimeResearchSettingsTarget
	skillReranker          runtimeSkillRerankerTarget
	skillRerankerDefaults  runtimeSkillRerankerDefaults
	promptSettings         runtimePromptSettingsTarget
	pushLocale             runtimeLocaleTarget
	mgmtSettings           runtimeAdminSettingsTarget
	questionMgr            questionRuntimePolicyTarget
	approvalHandler        *networkapi.ApprovalHandler
	execApprovals          *tools.ApprovalManager
	detailTarget           approvalRuntimeDetailTarget
	workflowTarget         workflowRuntimeHookTarget
	metrics                workflowRuntimeMetricsRecorder
	chatApprover           runtimeToolApproverTarget
}
