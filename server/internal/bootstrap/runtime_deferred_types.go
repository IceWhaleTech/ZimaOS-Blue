package bootstrap

import (
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeDeferredWiring struct {
	settings               *serverpkg.SettingsHandler
	smallRuntime           smallmodel.Runtime
	chatSmallModel         runtimeSmallModelChatTarget
	auxiliarySmallModel    runtimeSmallModelAuxiliaryTarget
	imageSmallModel        runtimeImageSmallModelTarget
	analyzeSmallModel      runtimeAnalyzeSmallModelTarget
	smallModelStats        runtimeSmallModelStatsSource
	harnessRuntime         *HarnessRuntimeBundle
	reflectionTarget       reflectionRuntimeTarget
	reflectionLLM          selfreflect.LLMCaller
	reflectionProposalGate func() bool
	compactorChat          runtimeCompactorMemoryChatTarget
	memoryHandler          *serverpkg.MemoryHandler
	auxiliaryLLM           llm.Provider
	sessionCompaction      config.SessionCompactionConfig
	sessionMaxTokens       int
	providerSettings       runtimeSettingsHandlerTarget
	chatSettings           runtimeSettingsHandlerTarget
	researchSettings       runtimeResearchSettingsTarget
	skillReranker          runtimeSkillRerankerTarget
	skillRerankerDefaults  runtimeSkillRerankerDefaults
	execAutoConfirm        runtimeAutoConfirmTarget
	promptSettings         runtimePromptSettingsTarget
	pushLocale             runtimeLocaleTarget
	maskerLocale           runtimeLocaleTarget
	mgmtSettings           runtimeAdminSettingsTarget
	questionMgr            questionRuntimePolicyTarget
	agentRunner            agentRuntimePolicyTarget
}

type runtimeDeferredApprovalWiring struct {
	handler         *networkapi.ApprovalHandler
	harnessRuntime  *HarnessRuntimeBundle
	execApprovals   *tools.ApprovalManager
	registry        *tools.Registry
	metrics         workflowRuntimeMetricsRecorder
	detailTarget    approvalRuntimeDetailTarget
	handlerTarget   approvalRuntimeHandlerTarget
	workflowTarget  workflowRuntimeHookTarget
	approverTargets []runtimeToolApproverTarget
}
