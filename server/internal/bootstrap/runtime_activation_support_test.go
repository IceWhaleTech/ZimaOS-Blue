package bootstrap

import (
	"context"
	"net/http"
	"testing"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestRegisterRuntimeActivationSupportRoutes_RegistersMemorySupport(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	memoryHandler := &stubMemoryRouteRegistrar{}
	memoryHandler.register = func(g *echo.Group) {
		g.GET("/memory/ready", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	}

	registerRuntimeActivationSupportRoutes(runtimeActivationResult{}, runtimeActivationRouteSupportOptions{
		v1:            v1,
		memoryHandler: memoryHandler,
	})

	if memoryHandler.calls != 1 {
		t.Fatalf("expected memory handler registration, got %d", memoryHandler.calls)
	}
	if memoryHandler.onLayeredReady == nil {
		t.Fatal("expected layered memory hook to be bound")
	}
	if !routeExists(e, http.MethodGet, "/api/v1/memory/ready") {
		t.Fatalf("expected memory support route, got %#v", e.Routes())
	}
}

func TestBindRuntimeActivationSupport_BindsDeferredTargetsAndApprovals(t *testing.T) {
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	broker := sse.NewBroker()
	defer broker.Close()

	activation := runtimeActivationResult{
		auxiliaryLLM: newAuxiliaryLLMCaller(),
		dataMasker:   newRuntimeProxyDataMasker(),
	}
	chatSmallModel := &stubRuntimeSmallModelChatTarget{}
	analyzeSmallModel := &stubRuntimeAnalyzeSmallModelTarget{}
	statsSource := &stubRuntimeSmallModelStatsSource{stats: &serverpkg.SmallModelStats{}}
	reflectTarget := &stubReflectionRuntimeTarget{}
	compactorChat := &stubRuntimeCompactorMemoryChatTarget{}
	providerSettings := &stubSettingsHandlerTarget{}
	chatSettings := &stubSettingsHandlerTarget{}
	researchSettings := &stubRuntimeResearchSettingsTarget{}
	skillReranker := &stubRuntimeSkillRerankerTarget{}
	promptSettings := &stubRuntimePromptSettingsTarget{}
	pushLocale := &stubRuntimeLocaleTarget{}
	mgmtSettings := &stubRuntimeAdminSettingsTarget{}
	questionMgr := &stubQuestionRuntimePolicyTarget{}
	chatApprover := &stubToolApproverTarget{}
	workflowTarget := &stubWorkflowRuntimeHookTarget{}

	bindRuntimeActivationSupport(activation, runtimeActivationDeferredSupportOptions{
		ctx:               context.Background(),
		services:          &Services{ToolRegistry: tools.NewRegistry()},
		deps:              &RoutesDeps{Config: &config.Config{Session: config.SessionConfig{MaxTokens: 2048}}},
		settings:          settings,
		smallModelManager: smallmodel.NewManager(t.TempDir()),
		chatSmallModel:    chatSmallModel,
		analyzeSmallModel: analyzeSmallModel,
		smallModelStats:   statsSource,
		reflectionTarget:  reflectTarget,
		reflectionProposalGate: func() bool {
			return true
		},
		compactorChat:         compactorChat,
		providerSettings:      providerSettings,
		chatSettings:          chatSettings,
		researchSettings:      researchSettings,
		skillReranker:         skillReranker,
		skillRerankerDefaults: runtimeSkillRerankerDefaults{dataDir: t.TempDir(), modelRepo: "repo/model"},
		promptSettings:        promptSettings,
		pushLocale:            pushLocale,
		mgmtSettings:          mgmtSettings,
		questionMgr:           questionMgr,
		approvalHandler:       networkapi.NewApprovalHandler(broker),
		workflowTarget:        workflowTarget,
		chatApprover:          chatApprover,
	})

	if chatSmallModel.calls != 1 || chatSmallModel.runtime == nil {
		t.Fatalf("expected small model chat wiring, got %#v", chatSmallModel)
	}
	if analyzeSmallModel.runtimeCalls != 1 || analyzeSmallModel.runtime == nil {
		t.Fatalf("expected small model analyze wiring, got %#v", analyzeSmallModel)
	}
	if reflectTarget.llmCalls != 1 || reflectTarget.llmCaller != activation.auxiliaryLLM || reflectTarget.proposalGateCalls != 1 {
		t.Fatalf("expected reflection wiring, got %#v", reflectTarget)
	}
	if compactorChat.calls != 1 || compactorChat.sessionMaxTokens != 2048 {
		t.Fatalf("expected compactor memory wiring, got %#v", compactorChat)
	}
	if providerSettings.calls != 1 || providerSettings.handler != settings {
		t.Fatalf("expected provider settings wiring, got %#v", providerSettings)
	}
	if chatSettings.calls != 1 || chatSettings.handler != settings {
		t.Fatalf("expected chat settings wiring, got %#v", chatSettings)
	}
	if researchSettings.calls != 1 {
		t.Fatalf("expected research settings wiring, got %#v", researchSettings)
	}
	if skillReranker.switchCalls != 1 {
		t.Fatalf("expected skill reranker wiring, got %#v", skillReranker)
	}
	if promptSettings.calls() != 3 || pushLocale.calls != 1 || mgmtSettings.calls != 1 {
		t.Fatalf("expected prompt/push/mgmt wiring, got prompt=%#v push=%#v mgmt=%#v", promptSettings, pushLocale, mgmtSettings)
	}
	if questionMgr.timeoutCalls != 1 || questionMgr.timeoutFunc == nil {
		t.Fatalf("expected ask policy wiring, got %#v", questionMgr)
	}
	if chatApprover.calls != 1 || chatApprover.approver == nil {
		t.Fatalf("expected approval wiring for chat target, got %#v", chatApprover)
	}
	if workflowTarget.hook == nil {
		t.Fatalf("expected workflow hook registration, got %#v", workflowTarget)
	}
}
