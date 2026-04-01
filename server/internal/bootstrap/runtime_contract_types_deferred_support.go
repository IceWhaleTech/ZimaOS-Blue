package bootstrap

import (
	"context"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractDeferredSupportOptions struct {
	ctx                    context.Context
	services               *Services
	deps                   *RoutesDeps
	settings               *serverpkg.SettingsHandler
	smallModelManager      *smallmodel.Manager
	chatSmallModel         runtimeSmallModelChatTarget
	analyzeSmallModel      runtimeAnalyzeSmallModelTarget
	smallModelStats        runtimeSmallModelStatsSource
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
