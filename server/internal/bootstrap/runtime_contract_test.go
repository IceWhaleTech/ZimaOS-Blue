package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestRuntimeContractBinding_BindsChatAndRegistersTaskSurface(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-contract.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	contract := newRuntimeContractBinding(
		db,
		db,
		cfg,
		zap.NewNop(),
		&stubRuntimeWorkspaceManagerSource{mgr: workspace.NewManager(tmp)},
		buildWebSearchConfig(cfg),
	)
	if contract == nil {
		t.Fatal("expected runtime contract binding")
	}
	if contract.NewRuntimeLLMRef() == nil {
		t.Fatal("expected runtime contract to create runtime llm ref")
	}
	if contract.HarnessRuntime() == nil {
		t.Fatalf("expected shared harness runtime, got %#v", contract.HarnessRuntime())
	}
	if contract.approvalDetailTarget() != nil {
		t.Fatalf("approval detail target should be empty before task surface registration, got %#v", contract.approvalDetailTarget())
	}

	target := &stubChatResearchRuntimeTarget{}
	chatTarget := &stubAutoHarnessTurnHookTarget{}
	registry := tools.NewRegistry()
	broker := sse.NewBroker()
	defer broker.Close()

	chatRuntime := contract.BindChatRuntime(routeRuntimeContractChatBindingOptions{
		target:  chatTarget,
		handler: &serverpkg.ChatHandler{},
	})
	if chatTarget.hook == nil || chatTarget.calls != 1 {
		t.Fatalf("expected contract to bind auto harness turn hook through chat runtime, got %#v", chatTarget)
	}
	if !chatRuntime.autoHarnessHookBound {
		t.Fatalf("expected contract to report chat runtime hook binding, got %#v", chatRuntime)
	}

	researchRuntime := bindRouteRuntimeResearchSurface(&contract.runtime, routeRuntimeContractResearchOptions{
		target:       target,
		broker:       broker,
		registry:     registry,
		workspaceDir: tmp,
	})
	if target.service == nil || target.serviceCalls != 1 {
		t.Fatalf("expected contract to bind shared research service, got %#v", target)
	}
	if target.hook != nil || target.hookCalls != 0 {
		t.Fatalf("expected route contract to skip direct turn hook wiring, got %#v", target)
	}
	if registry.Get("deep_research") == nil {
		t.Fatalf("expected deep_research tool to be registered, got %#v", registry.List())
	}
	if !researchRuntime.serviceBound || researchRuntime.turnHookBound || !researchRuntime.eventPublisherBound || !researchRuntime.driverRegistered || !researchRuntime.toolRegistered {
		t.Fatalf("expected contract to report research runtime state without direct turn hook binding, got %#v", researchRuntime)
	}

	e := echo.New()
	registration := contract.RegisterTaskSurface(runtimeTaskSurfaceOptions{
		protected:    e.Group("/api"),
		apiProtected: e.Group("/api/v1"),
		workspaceDir: tmp,
		logger:       zap.NewNop(),
	})
	if !registration.deepResearchRegistered || !registration.harnessResearchRegistered || !registration.harnessRoutesRegistered || !registration.selfReflectRoutesApplied {
		t.Fatalf("expected runtime contract to register research compatibility/capability, harness, and reflect routes, got %#v", registration)
	}
	if registration.detailProvider == nil {
		t.Fatalf("expected approval detail provider after task surface registration, got %#v", registration)
	}
	if contract.approvalDetailTarget() != registration.detailProvider {
		t.Fatalf("expected contract approval detail target to track registration detail provider, got target=%#v registration=%#v", contract.approvalDetailTarget(), registration.detailProvider)
	}
	if !routeExists(e, "POST", "/api/deep-research/jobs") || !routeExists(e, "POST", "/api/harness/research/jobs") || !routeExists(e, "GET", "/api/harness/runs") || !routeExists(e, "GET", "/api/self-reflect/proposals") {
		t.Fatalf("expected runtime contract task routes to register, got %#v", e.Routes())
	}
}

func TestRuntimeContractBinding_CreatesSupportBundlesWithSharedHarnessRuntime(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-contract-support.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	contract := newRuntimeContractBinding(
		db,
		db,
		cfg,
		zap.NewNop(),
		&stubRuntimeWorkspaceManagerSource{mgr: workspace.NewManager(tmp)},
		buildWebSearchConfig(cfg),
	)
	if contract == nil || contract.HarnessRuntime() == nil || contract.HarnessRuntime().RuntimeObserver == nil {
		t.Fatalf("expected harness runtime observer, got %#v", contract)
	}

	askTarget := &stubChatAskRuntimeTarget{}
	askBroker := sse.NewBroker()
	defer askBroker.Close()
	askBundle := contract.NewAskSupportBundle(routeRuntimeContractAskSupportOptions{
		writeDB:       db,
		readDB:        db,
		appConfig:     cfg,
		toolRegistry:  tools.NewRegistry(),
		skillRegistry: skillpkg.NewRegistry(),
		broker:        askBroker,
		chatTarget:    askTarget,
		mediaDir:      " /tmp/media ",
		timeout:       time.Minute,
	})
	if askBundle.QuestionManager == nil || askBundle.BrowserCheckpointManager == nil || askBundle.BrowserSiteStore == nil {
		t.Fatalf("expected ask support bundle, got %#v", askBundle)
	}
	if askTarget.toolObserver != contract.HarnessRuntime().RuntimeObserver || askTarget.toolObserverCalls != 1 {
		t.Fatalf("expected ask support to inherit shared harness observer, got %#v", askTarget)
	}

	memStore, err := memory.NewStoreWithDB(db)
	if err != nil {
		t.Fatalf("memory.NewStoreWithDB: %v", err)
	}
	execBroker := sse.NewBroker()
	defer execBroker.Close()
	execClosers := make([]interface{ Close() error }, 0, 1)
	execRegistry := tools.NewRegistry()
	e := echo.New()

	analyzeSkill := &stubRuntimeAnalyzeSkillTarget{}
	webSearchSkill := &stubRuntimeWebSearchSkillTarget{}
	deepResearchSkill := &stubRuntimeDeepResearchSkillTarget{}
	analyzeTool := contract.BindAnalyzeTool(routeRuntimeContractAnalyzeOptions{
		registry:          execRegistry,
		mediaDir:          tmp,
		analyzeSkill:      analyzeSkill,
		webSearchSkill:    webSearchSkill,
		webSearchConfig:   tools.WebSearchConfig{Provider: "duckduckgo"},
		deepResearchSkill: deepResearchSkill,
	})
	if analyzeTool == nil || execRegistry.Get("analyze") != analyzeTool {
		t.Fatalf("expected analyze tool to be registered through contract, got tool=%#v registered=%#v", analyzeTool, execRegistry.Get("analyze"))
	}
	if analyzeSkill.executor != analyzeTool || analyzeSkill.calls != 1 {
		t.Fatalf("expected analyze skill binding through contract, got %#v", analyzeSkill)
	}
	if webSearchSkill.searcher == nil || webSearchSkill.calls != 1 {
		t.Fatalf("expected web search skill binding through contract, got %#v", webSearchSkill)
	}
	if deepResearchSkill.executor == nil || deepResearchSkill.calls != 1 {
		t.Fatalf("expected deep research skill binding through contract, got %#v", deepResearchSkill)
	}

	scheduler := &stubRuntimeSchedulerSkillTarget{}
	calendar := &stubRuntimeSchedulerCalendarTarget{}
	cronHandler := &stubRuntimeCronHandlerTarget{
		service: cron.NewService(cron.DefaultConfig(), zap.NewNop()),
	}
	workflowResolutions := 0
	contract.BindSchedulerServices(routeRuntimeContractSchedulerOptions{
		registry: execRegistry,
		workflowResolver: func() *workflow.WorkflowService {
			workflowResolutions++
			return nil
		},
		cronHandler: cronHandler,
		scheduler:   scheduler,
		calendar:    calendar,
		memoryStore: memStore,
		broker:      execBroker,
		logger:      zap.NewNop(),
	})
	if workflowResolutions != 0 {
		t.Fatalf("expected workflow resolver to remain lazy, got %d", workflowResolutions)
	}
	if scheduler.service == nil || scheduler.calls != 1 {
		t.Fatalf("expected scheduler binding through contract, got %#v", scheduler)
	}
	if calendar.service == nil || calendar.calls != 1 {
		t.Fatalf("expected calendar binding through contract, got %#v", calendar)
	}
	if len(cronHandler.initHooks) != 1 {
		t.Fatalf("expected contract scheduler binding to install one init hook, got %#v", cronHandler.initHooks)
	}
	execBundle := contract.NewExecSupportBundle(routeRuntimeContractExecSupportOptions{
		writeDB:              db,
		readDB:               db,
		dataDir:              tmp,
		workspaceAllowedPath: []string{tmp},
		memoryStore:          memStore,
		toolRegistry:         execRegistry,
		skillRegistry:        skillpkg.NewRegistry(),
		broker:               execBroker,
		logger:               zap.NewNop(),
		closers:              &execClosers,
		profileRoutes:        e.Group("/api/profile"),
		sessionRoutes:        e.Group("/api/chat"),
		oauthSource: func() agentsessions.OAuthCredentialSource {
			return nil
		},
		lookupAPIKey: func(string) (string, error) {
			return "", nil
		},
	})
	if execBundle.Approvals == nil || execBundle.DirStore == nil || execBundle.AuditStore == nil || execBundle.ConvertHandler == nil {
		t.Fatalf("expected exec support bundle, got %#v", execBundle)
	}
	if tools.GetExecTool(execRegistry) == nil {
		t.Fatalf("expected exec tool registration, got %v", execRegistry.List())
	}
	if len(execClosers) != 1 {
		t.Fatalf("expected convert service closer to be captured, got %d", len(execClosers))
	}
}

func TestRuntimeContractRuntimeBundle_BindsResearchThroughSurfaceBoundary(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-contract-research-surface.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	contract := newRuntimeContractBinding(
		db,
		db,
		cfg,
		zap.NewNop(),
		&stubRuntimeWorkspaceManagerSource{mgr: workspace.NewManager(tmp)},
		buildWebSearchConfig(cfg),
	)
	if contract == nil {
		t.Fatal("expected runtime contract binding")
	}

	var surface routeRuntimeResearchSurface = &contract.runtime
	if surface.HarnessRuntime() != contract.HarnessRuntime() {
		t.Fatalf("expected runtime bundle research surface to expose shared harness runtime, got surface=%#v contract=%#v", surface.HarnessRuntime(), contract.HarnessRuntime())
	}
	if surface.ResearchService() != contract.runtime.ResearchService() {
		t.Fatalf("expected runtime bundle research surface to expose shared research service, got surface=%#v contract=%#v", surface.ResearchService(), contract.runtime.ResearchService())
	}

	target := &stubChatResearchRuntimeTarget{}
	registry := tools.NewRegistry()
	broker := sse.NewBroker()
	defer broker.Close()

	result := bindRouteRuntimeResearchSurface(surface, routeRuntimeContractResearchOptions{
		target:       target,
		broker:       broker,
		registry:     registry,
		workspaceDir: tmp,
	})
	if !result.serviceBound || result.turnHookBound || !result.eventPublisherBound || !result.driverRegistered || !result.toolRegistered {
		t.Fatalf("expected runtime bundle surface to preserve research binding state, got %#v", result)
	}
}

func TestRuntimeContractRuntimeBundle_ToleratesMissingCapabilitySurface(t *testing.T) {
	var nilBundle *runtimeContractRuntimeBundle
	if nilBundle.HarnessRuntime() != nil {
		t.Fatalf("expected nil harness runtime for nil bundle, got %#v", nilBundle.HarnessRuntime())
	}
	if nilBundle.ResearchService() != nil {
		t.Fatalf("expected nil research service for nil bundle, got %#v", nilBundle.ResearchService())
	}
	if nilBundle.ReflectService() != nil {
		t.Fatalf("expected nil reflect service for nil bundle, got %#v", nilBundle.ReflectService())
	}
	if got := nilBundle.RegisterTaskSurface(runtimeTaskSurfaceOptions{}); got != (runtimeTaskSurfaceRegistration{}) {
		t.Fatalf("expected empty task surface registration for nil bundle, got %#v", got)
	}
	if nilBundle.ApprovalDetailTarget() != nil {
		t.Fatalf("expected nil approval detail target for nil bundle, got %#v", nilBundle.ApprovalDetailTarget())
	}

	bundle := newRuntimeContractRuntimeBundle(nil)
	if bundle.HarnessRuntime() != nil {
		t.Fatalf("expected nil harness runtime for empty capability surface, got %#v", bundle.HarnessRuntime())
	}
	if bundle.ResearchService() != nil {
		t.Fatalf("expected nil research service for empty capability surface, got %#v", bundle.ResearchService())
	}
	if bundle.ReflectService() != nil {
		t.Fatalf("expected nil reflect service for empty capability surface, got %#v", bundle.ReflectService())
	}
	if got := bundle.RegisterTaskSurface(runtimeTaskSurfaceOptions{}); got != (runtimeTaskSurfaceRegistration{}) {
		t.Fatalf("expected empty task surface registration for empty capability surface, got %#v", got)
	}
	if bundle.ApprovalDetailTarget() != nil {
		t.Fatalf("expected nil approval detail target for empty capability surface, got %#v", bundle.ApprovalDetailTarget())
	}
}

func TestRuntimeContractBinding_ConfiguresChatRuntimeThroughBoundary(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.ToolCalling.SmartSelectionMaxTools = 7
		cfg.ToolCalling.SmartSkillSelection = true
		cfg.ToolCalling.SkillRerankEnabled = true
		cfg.ToolCalling.SkillRerankONNXEnabled = true
		cfg.ToolCalling.SkillRerankONNXAutoDownload = true
		cfg.ToolCalling.SkillRerankModel = "repo/model"

		contract := newRuntimeContractBinding(nil, nil, cfg, zap.NewNop(), nil, buildWebSearchConfig(cfg))
		chatHandler := &serverpkg.ChatHandler{}
		chatPrompt := &stubRuntimePromptGuardTarget{}
		security := &stubRuntimePromptGuardTarget{}

		reranker := contract.ConfigureChatRuntime(routeRuntimeContractChatOptions{
			chat:          chatHandler,
			chatPrompt:    chatPrompt,
			security:      security,
			config:        cfg,
			dataDir:       t.TempDir(),
			workspaceDir:  "/tmp/workspace",
			flagEvaluator: config.NewFlagEvaluator(&config.GrayscaleConfig{}),
			logger:        zap.NewNop(),
		})

		if reranker == nil {
			t.Fatal("expected chat runtime contract to create skill reranker")
		}
		if chatPrompt.detector == nil || security.detector == nil || chatPrompt.detector != security.detector {
			t.Fatalf("expected chat runtime contract to install shared prompt guard, chat=%#v security=%#v", chatPrompt, security)
		}
		if chatHandler.GetToolSelector() == nil || chatHandler.GetToolSelector().MaxTools != 7 {
			t.Fatalf("expected tool selection wiring through contract, got %#v", chatHandler.GetToolSelector())
		}
		if chatHandler.GetToolPolicyResolver() == nil || chatHandler.GetToolTraceStore() == nil || chatHandler.GetToolRouter() == nil || chatHandler.GetSkillSelector() == nil {
			t.Fatalf("expected chat runtime contract to wire selection dependencies, selector=%#v policy=%#v trace=%#v router=%#v skill=%#v", chatHandler.GetToolSelector(), chatHandler.GetToolPolicyResolver(), chatHandler.GetToolTraceStore(), chatHandler.GetToolRouter(), chatHandler.GetSkillSelector())
		}
	})

	t.Run("disabled", func(t *testing.T) {
		contract := newRuntimeContractBinding(nil, nil, &config.Config{}, zap.NewNop(), nil, tools.WebSearchConfig{})
		chatPrompt := &stubRuntimePromptGuardTarget{detector: promptguard.NewDetector(promptguard.DefaultDetectorConfig())}
		security := &stubRuntimePromptGuardTarget{detector: promptguard.NewDetector(promptguard.DefaultDetectorConfig())}

		reranker := contract.ConfigureChatRuntime(routeRuntimeContractChatOptions{
			chatPrompt: chatPrompt,
			security:   security,
			disabled:   true,
			logger:     zap.NewNop(),
		})

		if reranker != nil {
			t.Fatalf("expected no reranker without chat/config, got %#v", reranker)
		}
		if chatPrompt.detector != nil || security.detector != nil {
			t.Fatalf("expected disabled chat runtime contract to clear prompt guard, chat=%#v security=%#v", chatPrompt, security)
		}
	})
}

func TestRuntimeContractBinding_RegistersMgmtToolThroughBoundary(t *testing.T) {
	cfg := &config.Config{}
	contract := newRuntimeContractBinding(nil, nil, cfg, zap.NewNop(), nil, buildWebSearchConfig(cfg))

	registry := tools.NewRegistry()
	registry.Register(tools.NewMockTool("demo_tool", "demo"))
	skillRegistry := skillpkg.NewRegistry()
	if err := skillRegistry.Register(newStubRuntimeContractSkill("demo_skill"), true); err != nil {
		t.Fatalf("register skill: %v", err)
	}

	mgmtTool := contract.RegisterMgmtTool(routeRuntimeContractMgmtOptions{
		registry:      registry,
		providerPool:  &providerpool.Pool{},
		skillRegistry: skillRegistry,
		workspaceDir:  "",
		version:       "1.2.3",
		userService:   &user.Service{},
	})
	if mgmtTool == nil || registry.Get("mgmt") != mgmtTool {
		t.Fatalf("expected mgmt tool to be registered through contract, got tool=%#v registered=%#v", mgmtTool, registry.Get("mgmt"))
	}

	rawSystem, err := mgmtTool.Execute(context.Background(), map[string]any{"action": "system.version"})
	if err != nil {
		t.Fatalf("mgmt system.version execute: %v", err)
	}
	systemResp, ok := rawSystem.(string)
	if !ok || !strings.Contains(systemResp, `"version":"1.2.3"`) {
		t.Fatalf("expected mgmt system.version to reflect bound version, got %#v", rawSystem)
	}

	rawSkills, err := mgmtTool.Execute(context.Background(), map[string]any{"action": "skills.list"})
	if err != nil {
		t.Fatalf("mgmt skills.list execute: %v", err)
	}
	skillsResp, ok := rawSkills.(string)
	if !ok || !strings.Contains(skillsResp, `"id":"demo_skill"`) {
		t.Fatalf("expected mgmt skills.list to reflect bound skill registry, got %#v", rawSkills)
	}

	target := &stubRuntimeMgmtToolTarget{}
	contract.BindMgmtUpgrade(target, routeRuntimeContractMgmtUpgradeOptions{
		handler:    &update.Handler{},
		otaChecker: &update.OTAChecker{},
		version:    "9.9.9",
	})
	if target.upgrade == nil || target.upgradeCalls != 1 {
		t.Fatalf("expected mgmt upgrade binding through contract, got %#v", target)
	}
}

func TestRuntimeContractBinding_BindsProductivityToolsThroughBoundary(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-contract-productivity.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	contract := newRuntimeContractBinding(db, db, &config.Config{}, zap.NewNop(), nil, tools.WebSearchConfig{})
	registry := tools.NewRegistry()
	emailSkill := &stubRuntimeEmailSkillTarget{}
	calendarSkill := &stubRuntimeCalendarSkillTarget{}

	result := contract.BindProductivityTools(routeRuntimeContractProductivityOptions{
		writeDB:       db,
		readDB:        db,
		registry:      registry,
		emailSkill:    emailSkill,
		calendarSkill: calendarSkill,
		logger:        zap.NewNop(),
	})
	if result.emailService == nil || result.emailTool == nil || result.calendarService == nil || result.calendarTool == nil {
		t.Fatalf("expected productivity binding to register local email/calendar tools, got %#v", result)
	}
	if registry.Get("email") != result.emailTool || registry.Get("calendar") != result.calendarTool {
		t.Fatalf("expected productivity tools to be registered in tool registry, tools=%v", registry.List())
	}
	if emailSkill.executor != result.emailTool || emailSkill.calls != 1 {
		t.Fatalf("expected email skill wiring through contract, got %#v", emailSkill)
	}
	if calendarSkill.executor != result.calendarTool || calendarSkill.calls != 1 {
		t.Fatalf("expected calendar skill wiring through contract, got %#v", calendarSkill)
	}
}

func TestRuntimeContractBinding_BindsToolingThroughBoundary(t *testing.T) {
	cfg := &config.Config{}
	contract := newRuntimeContractBinding(
		nil,
		nil,
		cfg,
		zap.NewNop(),
		nil,
		buildWebSearchConfig(cfg),
	)

	registry := tools.NewRegistry()
	backend := &stubRuntimeBrowserBackend{}
	var backendIface tools.BrowserBackend = backend
	mediaDir := t.TempDir()
	browserSkill := &stubRuntimeBrowserSkillTarget{}
	uiReviewerSkill := &stubRuntimeUIReviewerSkillTarget{}
	reminderSkill := &stubRuntimeReminderSkillTarget{}
	voiceSource := &stubRuntimeVoiceServiceSource{}

	tooling := contract.BindTooling(routeRuntimeContractToolingOptions{
		registry:              registry,
		mediaDir:              mediaDir,
		browserBackend:        backend,
		lightpanda:            &browser.LightpandaService{},
		lazyBrowser:           func() *browser.RodService { return nil },
		browserSkill:          browserSkill,
		uiReviewerSkill:       uiReviewerSkill,
		pushService:           &push.Service{},
		reminderSkill:         reminderSkill,
		workspaceAllowedPaths: []string{t.TempDir()},
		mediaManager:          &mediagen.Manager{},
		mediaStorage:          &mediagen.MediaStorage{},
		ocr:                   &ocrruntime.TesseractService{},
		speechSource:          nil,
		voiceSource:           voiceSource,
	})
	if tooling.uiReviewerTool == nil {
		t.Fatalf("expected tooling contract to return a ui reviewer tool, got %#v", tooling)
	}
	// Browser tool is no longer registered as native tool - now skill-based only
	// The browser backend is still wired into browserSkill for internal use
	_ = backendIface
	if tools.GetBrowserTool(registry) != nil {
		t.Fatalf("expected browser tool to NOT be registered as native tool after migration, got %#v", tools.GetBrowserTool(registry))
	}
	for _, name := range []string{"reminder", "message", "image", "ppt", "tts"} {
		if registry.Get(name) == nil {
			t.Fatalf("expected tooling contract to register %q, tools=%v", name, registry.List())
		}
	}
	if browserSkill.browserService == nil || browserSkill.mediaDir != mediaDir || browserSkill.serviceCalls != 1 || browserSkill.mediaCalls != 1 {
		t.Fatalf("expected browser skill wiring through contract, got %#v", browserSkill)
	}
	if uiReviewerSkill.browserService == nil || uiReviewerSkill.calls != 1 {
		t.Fatalf("expected ui reviewer skill wiring through contract, got %#v", uiReviewerSkill)
	}
	if reminderSkill.service == nil || reminderSkill.calls != 1 {
		t.Fatalf("expected reminder skill wiring through contract, got %#v", reminderSkill)
	}
	if voiceSource.calls != 1 {
		t.Fatalf("expected tooling contract to resolve voice source once, got %#v", voiceSource)
	}
}

func TestRuntimeContractBinding_ResolvesSkillTargetsFromRegistry(t *testing.T) {
	cfg := &config.Config{}
	contract := newRuntimeContractBinding(
		nil,
		nil,
		cfg,
		zap.NewNop(),
		nil,
		buildWebSearchConfig(cfg),
	)

	browserSkill := &stubRuntimeContractBrowserSkill{
		stubRuntimeSkill:              newStubRuntimeContractSkill("browser"),
		stubRuntimeBrowserSkillTarget: &stubRuntimeBrowserSkillTarget{},
	}
	uiReviewerSkill := &stubRuntimeContractUIReviewerSkill{
		stubRuntimeSkill:                 newStubRuntimeContractSkill("ui_reviewer"),
		stubRuntimeUIReviewerSkillTarget: &stubRuntimeUIReviewerSkillTarget{},
	}
	reminderSkill := &stubRuntimeContractReminderSkill{
		stubRuntimeSkill:               newStubRuntimeContractSkill("reminder"),
		stubRuntimeReminderSkillTarget: &stubRuntimeReminderSkillTarget{},
	}
	emailSkill := &stubRuntimeContractEmailSkill{
		stubRuntimeSkill:            newStubRuntimeContractSkill("email"),
		stubRuntimeEmailSkillTarget: &stubRuntimeEmailSkillTarget{},
	}
	calendarSkill := &stubRuntimeContractCalendarSkill{
		stubRuntimeSkill:               newStubRuntimeContractSkill("calendar"),
		stubRuntimeCalendarSkillTarget: &stubRuntimeCalendarSkillTarget{},
	}
	schedulerSkill := &stubRuntimeContractSchedulerSkill{
		stubRuntimeSkill:                newStubRuntimeContractSkill("scheduler"),
		stubRuntimeSchedulerSkillTarget: &stubRuntimeSchedulerSkillTarget{},
	}
	analyzeSkill := &stubRuntimeContractAnalyzeSkill{
		stubRuntimeSkill:              newStubRuntimeContractSkill("analyze"),
		stubRuntimeAnalyzeSkillTarget: &stubRuntimeAnalyzeSkillTarget{},
	}
	webSearchSkill := &stubRuntimeContractWebSearchSkill{
		stubRuntimeSkill:                newStubRuntimeContractSkill("web_search"),
		stubRuntimeWebSearchSkillTarget: &stubRuntimeWebSearchSkillTarget{},
	}
	deepResearchSkill := &stubRuntimeContractDeepResearchSkill{
		stubRuntimeSkill:                   newStubRuntimeContractSkill("deep_research"),
		stubRuntimeDeepResearchSkillTarget: &stubRuntimeDeepResearchSkillTarget{},
	}
	skillRegistry := newStubRuntimeContractSkillRegistry(map[string]skillpkg.Skill{
		"browser":       browserSkill,
		"ui_reviewer":   uiReviewerSkill,
		"reminder":      reminderSkill,
		"email":         emailSkill,
		"calendar":      calendarSkill,
		"scheduler":     schedulerSkill,
		"analyze":       analyzeSkill,
		"web_search":    webSearchSkill,
		"deep_research": deepResearchSkill,
	})

	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-contract-productivity-registry.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	productivityRegistry := tools.NewRegistry()
	productivity := contract.BindProductivityTools(routeRuntimeContractProductivityOptions{
		writeDB:       db,
		readDB:        db,
		registry:      productivityRegistry,
		skillRegistry: skillRegistry,
		logger:        zap.NewNop(),
	})
	if productivity.emailTool == nil || productivity.calendarTool == nil {
		t.Fatalf("expected productivity tools to resolve through skill registry, got %#v", productivity)
	}
	if emailSkill.stubRuntimeEmailSkillTarget.calls != 1 || calendarSkill.stubRuntimeCalendarSkillTarget.calls != 1 {
		t.Fatalf("expected productivity contract to resolve email/calendar skills via registry, got email=%#v calendar=%#v", emailSkill, calendarSkill)
	}

	toolingRegistry := tools.NewRegistry()
	tooling := contract.BindTooling(routeRuntimeContractToolingOptions{
		registry:              toolingRegistry,
		skillRegistry:         skillRegistry,
		mediaDir:              t.TempDir(),
		browserBackend:        &stubRuntimeBrowserBackend{},
		lazyBrowser:           func() *browser.RodService { return nil },
		pushService:           &push.Service{},
		workspaceAllowedPaths: []string{t.TempDir()},
		mediaManager:          &mediagen.Manager{},
		mediaStorage:          &mediagen.MediaStorage{},
		ocr:                   &ocrruntime.TesseractService{},
	})
	if tooling.uiReviewerTool == nil {
		t.Fatalf("expected tooling resolution through skill registry, got %#v", tooling)
	}
	if browserSkill.serviceCalls != 1 || uiReviewerSkill.stubRuntimeUIReviewerSkillTarget.calls != 1 || reminderSkill.stubRuntimeReminderSkillTarget.calls != 1 {
		t.Fatalf("expected tooling contract to resolve browser/ui/reminder skills via registry, got browser=%#v ui=%#v reminder=%#v", browserSkill, uiReviewerSkill, reminderSkill)
	}

	analyzeRegistry := tools.NewRegistry()
	analyzeTool := contract.BindAnalyzeTool(routeRuntimeContractAnalyzeOptions{
		registry:        analyzeRegistry,
		skillRegistry:   skillRegistry,
		mediaDir:        t.TempDir(),
		browserBackend:  &stubRuntimeBrowserBackend{},
		webSearchConfig: tools.WebSearchConfig{Provider: "duckduckgo"},
	})
	if analyzeTool == nil {
		t.Fatalf("expected analyze contract binding through skill registry")
	}
	if analyzeSkill.stubRuntimeAnalyzeSkillTarget.calls != 1 || webSearchSkill.stubRuntimeWebSearchSkillTarget.calls != 1 || deepResearchSkill.stubRuntimeDeepResearchSkillTarget.calls != 1 {
		t.Fatalf("expected analyze contract to resolve skills via registry, got analyze=%#v web=%#v research=%#v", analyzeSkill, webSearchSkill, deepResearchSkill)
	}

	cronHandler := &stubRuntimeCronHandlerTarget{
		service: cron.NewService(cron.DefaultConfig(), zap.NewNop()),
	}
	broker := sse.NewBroker()
	defer broker.Close()
	workflowResolutions := 0
	contract.BindSchedulerServices(routeRuntimeContractSchedulerOptions{
		registry:      tools.NewRegistry(),
		skillRegistry: skillRegistry,
		workflowResolver: func() *workflow.WorkflowService {
			workflowResolutions++
			return nil
		},
		cronHandler: cronHandler,
		broker:      broker,
		logger:      zap.NewNop(),
	})
	if workflowResolutions != 0 {
		t.Fatalf("expected scheduler workflow resolver to remain lazy, got %d", workflowResolutions)
	}
	if schedulerSkill.stubRuntimeSchedulerSkillTarget.calls != 1 {
		t.Fatalf("expected scheduler contract to resolve skill via registry, got %#v", schedulerSkill)
	}
}

func TestRuntimeContractGo_StaysFocusedOnBoundaryDefinition(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_contract.go: %v", err)
	}
	source := string(content)

	if lines := strings.Count(source, "\n") + 1; lines > 120 {
		t.Fatalf("expected runtime_contract.go to stay thin, got %d lines", lines)
	}

	required := []string{
		"type routeRuntimeContract interface {",
		"type runtimeContractBinding struct {",
		"func newRouteRuntimeContract(",
		"newRouteRuntimePhaseContract(",
		"func newRuntimeContractBinding(",
		"newRuntimeCapabilitySurface(",
		"newRuntimeContractRuntimeBundle(",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_contract.go to keep boundary token %q", token)
		}
	}

	forbidden := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractActivationOptions struct {",
		"newRuntimeCapabilityContract(",
		"func (binding *runtimeContractBinding) ConfigureChatRuntime(",
		"func (binding *runtimeContractBinding) ActivateRouteRuntime(",
		"func skillRegistryFromRuntimeSource(",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_contract.go to delegate heavy implementation via %q", token)
		}
	}
}

func TestRuntimeContractRuntimeBundleGo_CentralizesSharedCapabilityAndTaskSurfaceState(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_contract_runtime_bundle.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_runtime_bundle.go: %v", err)
	}
	source := string(content)

	surfaceContent, err := os.ReadFile(filepath.Join("runtime_contract_runtime_bundle_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_runtime_bundle_surface.go: %v", err)
	}
	surfaceSource := string(surfaceContent)

	taskSurfaceContent, err := os.ReadFile(filepath.Join("runtime_contract_runtime_bundle_task_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_runtime_bundle_task_surface.go: %v", err)
	}
	taskSurfaceSource := string(taskSurfaceContent)

	capabilityContent, err := os.ReadFile(filepath.Join("runtime_capability_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_contract.go: %v", err)
	}
	capabilitySource := string(capabilityContent)

	factoryContent, err := os.ReadFile(filepath.Join("runtime_capability_factory.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_factory.go: %v", err)
	}
	factorySource := string(factoryContent)

	implContent, err := os.ReadFile(filepath.Join("runtime_capability_impl.go"))
	if err != nil {
		t.Fatalf("read runtime_capability_impl.go: %v", err)
	}
	implSource := string(implContent)

	if lines := strings.Count(source, "\n") + 1; lines > 15 {
		t.Fatalf("expected runtime_contract_runtime_bundle.go to stay below 15 lines, got %d", lines)
	}
	if lines := strings.Count(surfaceSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_contract_runtime_bundle_surface.go to stay below 35 lines, got %d", lines)
	}
	if lines := strings.Count(taskSurfaceSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_contract_runtime_bundle_task_surface.go to stay below 20 lines, got %d", lines)
	}
	if lines := strings.Count(capabilitySource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_capability_contract.go to stay below 35 lines, got %d", lines)
	}
	if lines := strings.Count(factorySource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_capability_factory.go to stay below 30 lines, got %d", lines)
	}
	if lines := strings.Count(implSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_capability_impl.go to stay below 30 lines, got %d", lines)
	}

	required := []string{
		"type runtimeContractRuntimeBundle struct {",
		"var _ routeRuntimeResearchSurface = (*runtimeContractRuntimeBundle)(nil)",
		"capabilities runtimeCapabilitySurface",
		"taskSurface  runtimeTaskSurfaceRegistration",
		"func newRuntimeContractRuntimeBundle(",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_contract_runtime_bundle.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func (bundle *runtimeContractRuntimeBundle) capabilitySurface(",
		"func (bundle *runtimeContractRuntimeBundle) HarnessRuntime(",
		"func (bundle *runtimeContractRuntimeBundle) ResearchService(",
		"func (bundle *runtimeContractRuntimeBundle) ReflectService(",
	} {
		if !strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_contract_runtime_bundle_surface.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func (bundle *runtimeContractRuntimeBundle) RegisterTaskSurface(",
		"func (bundle *runtimeContractRuntimeBundle) ApprovalDetailTarget(",
	} {
		if !strings.Contains(taskSurfaceSource, token) {
			t.Fatalf("expected runtime_contract_runtime_bundle_task_surface.go to contain token %q", token)
		}
	}

	requiredCapability := []string{
		"type runtimeCapabilitySurface interface {",
		"type runtimeCapabilityResearchSurface interface {",
		"type runtimeCapabilityTaskSurface interface {",
		"runtimeCapabilityBoundary()",
		"registerTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration",
		"var _ runtimeCapabilitySurface = runtimeCapabilityAdapter" + "{}",
		"var _ runtimeCapabilityResearchSurface = runtimeCapabilityAdapter" + "{}",
		"var _ runtimeCapabilityTaskSurface = runtimeCapabilityAdapter" + "{}",
		"func (runtimeCapabilityAdapter) runtimeCapabilityBoundary() {}",
	}
	for _, token := range requiredCapability {
		if !strings.Contains(capabilitySource, token) {
			t.Fatalf("expected runtime_capability_contract.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func newRuntimeCapabilitySurface(",
		"return newRuntimeCapabilityContract(",
	} {
		if !strings.Contains(factorySource, token) {
			t.Fatalf("expected runtime_capability_factory.go to contain token %q", token)
		}
	}

	for _, token := range []string{
		"func newRuntimeCapabilityImplementation(",
		"newRuntimeCapabilityServices(",
		"normalizeRuntimeWorkspaceManagerSource(",
		"bindRuntimeCapabilityProposalStore(",
		"bindRuntimeCapabilityWorkspaceManager(",
		"bindRuntimeCapabilityHarness(",
	} {
		if !strings.Contains(implSource, token) {
			t.Fatalf("expected runtime_capability_impl.go to contain token %q", token)
		}
	}

	forbidden := []string{
		"func newRuntimeCapabilityContract(",
		"func (bundle *runtimeContractRuntimeBundle) HarnessRuntime(",
		"func (bundle *runtimeContractRuntimeBundle) RegisterTaskSurface(",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_contract_runtime_bundle.go to delegate token %q", token)
		}
	}

	for _, token := range []string{
		"func newRuntimeCapabilityContract(",
		"type runtimeContractRuntimeBundle struct {",
	} {
		if strings.Contains(capabilitySource, token) {
			t.Fatalf("expected runtime_capability_contract.go to stay focused on capability boundary token %q", token)
		}
	}

	for _, token := range []string{
		"newRuntimeCapabilityServices(",
		"bindRuntimeCapabilityProposalStore(",
		"type runtimeContractRuntimeBundle struct {",
	} {
		if strings.Contains(factorySource, token) {
			t.Fatalf("expected runtime_capability_factory.go to stay focused on surface factory token %q", token)
		}
	}

	for _, token := range []string{
		"func newRuntimeCapabilityContract(",
		"func newRuntimeCapabilitySurface(",
		"type runtimeContractRuntimeBundle struct {",
	} {
		if strings.Contains(implSource, token) {
			t.Fatalf("expected runtime_capability_impl.go to stay focused on concrete capability assembly token %q", token)
		}
	}
}

func TestRuntimeRoutePhaseContractGo_DelegatesOnlyPhaseBindings(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_contract_route_phase.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_route_phase.go: %v", err)
	}
	source := string(content)

	if lines := strings.Count(source, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_contract_route_phase.go to stay below 60 lines, got %d", lines)
	}

	required := []string{
		"type routeRuntimePhaseBinding interface {",
		"var _ routeRuntimePhaseBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimePhaseContract struct{ binding routeRuntimePhaseBinding }",
		"var _ routeRuntimeContract = (*routeRuntimePhaseContract)(nil)",
		"func newRouteRuntimePhaseContract(",
		"func (contract *routeRuntimePhaseContract) routeRuntimePhaseBoundary()",
		"func (contract *routeRuntimePhaseContract) NewRuntimeLLMRef(",
		"func (contract *routeRuntimePhaseContract) BindStartupAuthRuntime(",
		"func (contract *routeRuntimePhaseContract) BindBootstrapPhaseRuntime(",
		"func (contract *routeRuntimePhaseContract) BindInfrastructureRuntime(",
		"func (contract *routeRuntimePhaseContract) BindExperienceRuntime(",
		"func (contract *routeRuntimePhaseContract) BindCoreToolingRuntime(",
		"func (contract *routeRuntimePhaseContract) BindCoreSupportRuntime(",
		"func (contract *routeRuntimePhaseContract) BindManagementRuntime(",
		"func (contract *routeRuntimePhaseContract) BindOperationalRuntime(",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_contract_route_phase.go to contain token %q", token)
		}
	}

	forbidden := []string{
		"binding *runtimeContractBinding",
		"func (contract *routeRuntimePhaseContract) BindChatRuntime(",
		"func (contract *routeRuntimePhaseContract) ConfigureChatRuntime(",
		"func (contract *routeRuntimePhaseContract) BindTooling(",
		"func (contract *routeRuntimePhaseContract) NewAskSupportBundle(",
		"func (contract *routeRuntimePhaseContract) RegisterMgmtTool(",
		"func (contract *routeRuntimePhaseContract) ActivateRouteRuntime(",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_contract_route_phase.go to hide lower-level binder token %q", token)
		}
	}
}

func TestNewRouteRuntimeContract_ReturnsPhaseAdapter(t *testing.T) {
	contract := newRouteRuntimeContract(nil, nil, &config.Config{}, zap.NewNop(), nil, tools.WebSearchConfig{})
	phase, ok := contract.(*routeRuntimePhaseContract)
	if !ok {
		t.Fatalf("expected route runtime contract adapter, got %T", contract)
	}
	if phase.binding == nil {
		t.Fatalf("expected route runtime phase contract to retain internal runtime binding, got %#v", phase)
	}
}

func TestRouteRuntimeContract_InterfaceRemainsPhaseOriented(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_contract.go: %v", err)
	}
	source := string(content)

	required := []string{
		"routeRuntimePhaseBoundary()",
		"NewRuntimeLLMRef() *runtimeLLMProviderRef",
		"BindStartupAuthRuntime(options routeRuntimeContractStartupAuthOptions) routeRuntimeContractStartupAuthResult",
		"BindBootstrapPhaseRuntime(options routeRuntimeContractBootstrapPhaseOptions) routeRuntimeContractBootstrapPhaseResult",
		"BindInfrastructureRuntime(options routeRuntimeContractInfrastructureOptions) routeRuntimeContractInfrastructureResult",
		"BindExperienceRuntime(options routeRuntimeContractExperienceOptions) routeRuntimeContractExperienceResult",
		"BindCoreToolingRuntime(options routeRuntimeContractCoreToolingOptions) routeRuntimeContractCoreToolingResult",
		"BindCoreSupportRuntime(options routeRuntimeContractCoreSupportOptions) routeRuntimeContractCoreSupportResult",
		"BindManagementRuntime(options routeRuntimeContractManagementRuntimeOptions) routeRuntimeContractManagementRuntimeResult",
		"BindOperationalRuntime(options routeRuntimeContractOperationalOptions) routeRuntimeContractOperationalResult",
	}
	for _, token := range required {
		if !strings.Contains(source, token) {
			t.Fatalf("expected routeRuntimeContract to keep route-facing phase token %q", token)
		}
	}

	forbidden := []string{
		"BindChatRuntime(options routeRuntimeContractChatBindingOptions)",
		"ConfigureChatRuntime(options routeRuntimeContractChatOptions)",
		"BindProductivityTools(options routeRuntimeContractProductivityOptions)",
		"BindTooling(options routeRuntimeContractToolingOptions)",
		"BindSchedulerServices(options routeRuntimeContractSchedulerOptions)",
		"BindAnalyzeTool(options routeRuntimeContractAnalyzeOptions)",
		"BindMediaRuntime(options routeRuntimeContractMediaOptions)",
		"BindSkillRuntime(options routeRuntimeContractSkillOptions)",
		"BindHeartbeatRuntime(options routeRuntimeContractHeartbeatOptions)",
		"BindProviderPoolRuntime(options routeRuntimeContractProviderPoolOptions)",
		"BindChannelRuntime(options routeRuntimeContractChannelOptions)",
		"BindManagementSupport(options routeRuntimeContractManagementSupportOptions)",
		"BindGatewayRuntime(options routeRuntimeContractGatewayOptions)",
		"BindAuthSurfaceRuntime(options routeRuntimeContractAuthSurfaceOptions)",
		"BindCapabilitySupportRuntime(options routeRuntimeContractCapabilitySupportOptions)",
		"BindUserSurfaceRuntime(options routeRuntimeContractUserSurfaceOptions)",
		"BindPlatformSurfaceRuntime(options routeRuntimeContractPlatformSurfaceOptions)",
		"BindAccountSurfaceRuntime(options routeRuntimeContractAccountSurfaceOptions)",
		"BindBootstrapSupportRuntime(options routeRuntimeContractBootstrapSupportOptions)",
		"BindShellSurfaceRuntime(options routeRuntimeContractShellSurfaceOptions)",
		"BindStartupSurfaceRuntime(options routeRuntimeContractStartupSurfaceOptions)",
		"BindTLSRuntime(options routeRuntimeContractTLSOptions)",
		"BindChatSurfaceRuntime(options routeRuntimeContractChatSurfaceOptions)",
		"RegisterTaskSurface(options runtimeTaskSurfaceOptions)",
		"NewAskSupportBundle(options routeRuntimeContractAskSupportOptions)",
		"NewExecSupportBundle(options routeRuntimeContractExecSupportOptions)",
		"RegisterMgmtTool(options routeRuntimeContractMgmtOptions)",
		"BindMgmtUpgrade(target runtimeMgmtToolTarget, options routeRuntimeContractMgmtUpgradeOptions)",
		"ActivateRouteRuntime(options routeRuntimeContractActivationOptions)",
		"RegisterActivationSupportRoutes(activation runtimeActivationResult, options routeRuntimeContractSupportOptions)",
		"BindDeferredSupport(activation runtimeActivationResult, options routeRuntimeContractDeferredSupportOptions)",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected routeRuntimeContract to hide lower-level binder token %q", token)
		}
	}
}

func TestRuntimeContractBindingCoreGo_DelegatesActivationGlue(t *testing.T) {
	coreContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_core.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_core.go: %v", err)
	}
	coreSource := string(coreContent)

	activationContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_activation.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_activation.go: %v", err)
	}
	activationSource := string(activationContent)

	if lines := strings.Count(coreSource, "\n") + 1; lines > 50 {
		t.Fatalf("expected runtime_contract_binding_core.go to stay below 50 lines after extraction, got %d", lines)
	}

	requiredCore := []string{
		"func (binding *runtimeContractBinding) NewRuntimeLLMRef(",
		"func (binding *runtimeContractBinding) HarnessRuntime(",
		"func (binding *runtimeContractBinding) ResearchService(",
		"func (binding *runtimeContractBinding) RegisterTaskSurface(",
		"binding.runtime.HarnessRuntime()",
		"binding.runtime.ResearchService()",
		"binding.runtime.RegisterTaskSurface(",
	}
	for _, token := range requiredCore {
		if !strings.Contains(coreSource, token) {
			t.Fatalf("expected runtime_contract_binding_core.go to keep token %q", token)
		}
	}

	forbiddenCore := []string{
		"func (binding *runtimeContractBinding) approvalDetailTarget(",
		"func (binding *runtimeContractBinding) ActivateRouteRuntime(",
		"func (binding *runtimeContractBinding) RegisterActivationSupportRoutes(",
		"func (binding *runtimeContractBinding) BindDeferredSupport(",
	}
	for _, token := range forbiddenCore {
		if strings.Contains(coreSource, token) {
			t.Fatalf("expected runtime_contract_binding_core.go to delegate token %q", token)
		}
	}

	requiredActivation := []string{
		"func (binding *runtimeContractBinding) approvalDetailTarget(",
		"func (binding *runtimeContractBinding) ActivateRouteRuntime(",
		"func (binding *runtimeContractBinding) RegisterActivationSupportRoutes(",
		"func (binding *runtimeContractBinding) BindDeferredSupport(",
		"binding.runtime.ApprovalDetailTarget()",
		"binding.runtime.ReflectService()",
		"binding.runtime.ResearchService()",
	}
	for _, token := range requiredActivation {
		if !strings.Contains(activationSource, token) {
			t.Fatalf("expected runtime_contract_binding_activation.go to contain token %q", token)
		}
	}
}

func TestRuntimeContractBindingFiles_HideDirectCapabilityAndTaskSurfaceFields(t *testing.T) {
	files := []string{
		"runtime_contract_binding_core.go",
		"runtime_contract_binding_chat.go",
		"runtime_contract_binding_analyze_scheduler.go",
		"runtime_contract_binding_support.go",
		"runtime_contract_binding_activation.go",
		"runtime_research_contract.go",
	}
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		for _, token := range []string{"binding.capabilities", "binding.taskSurface"} {
			if strings.Contains(source, token) {
				t.Fatalf("expected %s to route shared runtime state through runtime bundle, found %q", name, token)
			}
		}
	}
}

func TestRuntimeContractBindingSlices_DelegateChatSupportAndToolingGlue(t *testing.T) {
	chatContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_chat.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_chat.go: %v", err)
	}
	chatSource := string(chatContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_support.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_support.go: %v", err)
	}
	supportSource := string(supportContent)
	productivityContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_productivity.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_productivity.go: %v", err)
	}
	productivitySource := string(productivityContent)
	toolingContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_tooling.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_tooling.go: %v", err)
	}
	toolingSource := string(toolingContent)
	analyzeSchedulerContent, err := os.ReadFile(filepath.Join("runtime_contract_binding_analyze_scheduler.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_binding_analyze_scheduler.go: %v", err)
	}
	analyzeSchedulerSource := string(analyzeSchedulerContent)

	if lines := strings.Count(productivitySource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_contract_binding_productivity.go to stay below 80 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(toolingSource, "\n") + 1; lines > 110 {
		t.Fatalf("expected runtime_contract_binding_tooling.go to stay below 110 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(analyzeSchedulerSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_contract_binding_analyze_scheduler.go to stay below 80 lines after extraction, got %d", lines)
	}

	requiredProductivity := []string{
		"func (binding *runtimeContractBinding) BindProductivityTools(",
	}
	for _, token := range requiredProductivity {
		if !strings.Contains(productivitySource, token) {
			t.Fatalf("expected runtime_contract_binding_productivity.go to keep token %q", token)
		}
	}

	requiredTooling := []string{
		"func (binding *runtimeContractBinding) BindTooling(",
	}
	for _, token := range requiredTooling {
		if !strings.Contains(toolingSource, token) {
			t.Fatalf("expected runtime_contract_binding_tooling.go to keep token %q", token)
		}
	}

	requiredAnalyzeScheduler := []string{
		"func (binding *runtimeContractBinding) BindSchedulerServices(",
		"func (binding *runtimeContractBinding) BindAnalyzeTool(",
	}
	for _, token := range requiredAnalyzeScheduler {
		if !strings.Contains(analyzeSchedulerSource, token) {
			t.Fatalf("expected runtime_contract_binding_analyze_scheduler.go to keep token %q", token)
		}
	}

	requiredChat := []string{
		"func (binding *runtimeContractBinding) BindChatRuntime(",
		"func (binding *runtimeContractBinding) ConfigureChatRuntime(",
	}
	for _, token := range requiredChat {
		if !strings.Contains(chatSource, token) {
			t.Fatalf("expected runtime_contract_binding_chat.go to contain token %q", token)
		}
	}

	requiredSupport := []string{
		"func (binding *runtimeContractBinding) NewAskSupportBundle(",
		"func (binding *runtimeContractBinding) NewExecSupportBundle(",
		"func (binding *runtimeContractBinding) RegisterMgmtTool(",
		"func (binding *runtimeContractBinding) BindMgmtUpgrade(",
	}
	for _, token := range requiredSupport {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_contract_binding_support.go to contain token %q", token)
		}
	}

	forbiddenProductivity := []string{
		"func (binding *runtimeContractBinding) BindTooling(",
		"func (binding *runtimeContractBinding) BindAnalyzeTool(",
		"func (binding *runtimeContractBinding) ConfigureChatRuntime(",
	}
	for _, token := range forbiddenProductivity {
		if strings.Contains(productivitySource, token) {
			t.Fatalf("expected runtime_contract_binding_productivity.go to delegate token %q", token)
		}
	}

	forbiddenTooling := []string{
		"func (binding *runtimeContractBinding) BindProductivityTools(",
		"func (binding *runtimeContractBinding) BindAnalyzeTool(",
		"func (binding *runtimeContractBinding) ConfigureChatRuntime(",
	}
	for _, token := range forbiddenTooling {
		if strings.Contains(toolingSource, token) {
			t.Fatalf("expected runtime_contract_binding_tooling.go to delegate token %q", token)
		}
	}

	forbiddenAnalyzeScheduler := []string{
		"func (binding *runtimeContractBinding) BindProductivityTools(",
		"func (binding *runtimeContractBinding) BindTooling(",
		"func (binding *runtimeContractBinding) ConfigureChatRuntime(",
	}
	for _, token := range forbiddenAnalyzeScheduler {
		if strings.Contains(analyzeSchedulerSource, token) {
			t.Fatalf("expected runtime_contract_binding_analyze_scheduler.go to delegate token %q", token)
		}
	}
}

func TestRuntimeContractTypeSlices_DelegateChatToolingAndSupportOptions(t *testing.T) {
	chatContent, err := os.ReadFile(filepath.Join("runtime_contract_types_chat.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_chat.go: %v", err)
	}
	chatSource := string(chatContent)

	productivityContent, err := os.ReadFile(filepath.Join("runtime_contract_types_productivity.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_productivity.go: %v", err)
	}
	productivitySource := string(productivityContent)
	schedulerContent, err := os.ReadFile(filepath.Join("runtime_contract_types_scheduler.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_scheduler.go: %v", err)
	}
	schedulerSource := string(schedulerContent)
	toolingContent, err := os.ReadFile(filepath.Join("runtime_contract_types_tooling.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_tooling.go: %v", err)
	}
	toolingSource := string(toolingContent)
	analyzeContent, err := os.ReadFile(filepath.Join("runtime_contract_types_analyze.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_analyze.go: %v", err)
	}
	analyzeSource := string(analyzeContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_contract_types_tool_support.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_tool_support.go: %v", err)
	}
	supportSource := string(supportContent)

	activationContent, err := os.ReadFile(filepath.Join("runtime_contract_types_activation.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_activation.go: %v", err)
	}
	activationSource := string(activationContent)

	authSurfaceContent, err := os.ReadFile(filepath.Join("runtime_contract_types_auth_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_auth_surface.go: %v", err)
	}
	authSurfaceSource := string(authSurfaceContent)

	capabilityContent, err := os.ReadFile(filepath.Join("runtime_contract_types_capability_support.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_capability_support.go: %v", err)
	}
	capabilitySource := string(capabilityContent)

	deferredContent, err := os.ReadFile(filepath.Join("runtime_contract_types_deferred_support.go"))
	if err != nil {
		t.Fatalf("read runtime_contract_types_deferred_support.go: %v", err)
	}
	deferredSource := string(deferredContent)

	requiredChat := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractChatBindingOptions struct {",
		"type routeRuntimeContractChatBindingResult struct {",
	}
	for _, token := range requiredChat {
		if !strings.Contains(chatSource, token) {
			t.Fatalf("expected runtime_contract_types_chat.go to contain token %q", token)
		}
	}

	requiredProductivity := []string{
		"type routeRuntimeContractProductivityOptions struct {",
		"type routeRuntimeContractProductivityResult struct {",
	}
	for _, token := range requiredProductivity {
		if !strings.Contains(productivitySource, token) {
			t.Fatalf("expected runtime_contract_types_productivity.go to contain token %q", token)
		}
	}

	requiredScheduler := []string{
		"type routeRuntimeContractSchedulerOptions struct {",
	}
	for _, token := range requiredScheduler {
		if !strings.Contains(schedulerSource, token) {
			t.Fatalf("expected runtime_contract_types_scheduler.go to contain token %q", token)
		}
	}

	requiredTooling := []string{
		"type routeRuntimeContractToolingOptions struct {",
		"type routeRuntimeContractToolingResult struct {",
	}
	for _, token := range requiredTooling {
		if !strings.Contains(toolingSource, token) {
			t.Fatalf("expected runtime_contract_types_tooling.go to contain token %q", token)
		}
	}

	requiredAnalyze := []string{
		"type routeRuntimeContractAnalyzeOptions struct {",
	}
	for _, token := range requiredAnalyze {
		if !strings.Contains(analyzeSource, token) {
			t.Fatalf("expected runtime_contract_types_analyze.go to contain token %q", token)
		}
	}

	requiredSupport := []string{
		"type routeRuntimeContractAskSupportOptions struct {",
		"type routeRuntimeContractExecSupportOptions struct {",
		"type routeRuntimeContractMgmtOptions struct {",
		"type routeRuntimeContractMgmtUpgradeOptions struct {",
	}
	for _, token := range requiredSupport {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_contract_types_tool_support.go to contain token %q", token)
		}
	}

	requiredActivation := []string{
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractSupportOptions struct {",
	}
	for _, token := range requiredActivation {
		if !strings.Contains(activationSource, token) {
			t.Fatalf("expected runtime_contract_types_activation.go to contain token %q", token)
		}
	}

	requiredAuthSurface := []string{
		"type routeRuntimeContractAuthSurfaceOptions struct {",
		"type routeRuntimeContractAuthSurfaceResult struct {",
	}
	for _, token := range requiredAuthSurface {
		if !strings.Contains(authSurfaceSource, token) {
			t.Fatalf("expected runtime_contract_types_auth_surface.go to contain token %q", token)
		}
	}

	requiredCapability := []string{
		"type routeRuntimeContractCapabilitySupportOptions struct {",
		"type routeRuntimeContractCapabilitySupportResult struct {",
	}
	for _, token := range requiredCapability {
		if !strings.Contains(capabilitySource, token) {
			t.Fatalf("expected runtime_contract_types_capability_support.go to contain token %q", token)
		}
	}

	requiredDeferred := []string{
		"type routeRuntimeContractDeferredSupportOptions struct {",
	}
	for _, token := range requiredDeferred {
		if !strings.Contains(deferredSource, token) {
			t.Fatalf("expected runtime_contract_types_deferred_support.go to contain token %q", token)
		}
	}

	forbiddenChat := []string{
		"type routeRuntimeContractAskSupportOptions struct {",
		"type routeRuntimeContractProductivityOptions struct {",
	}
	for _, token := range forbiddenChat {
		if strings.Contains(chatSource, token) {
			t.Fatalf("expected runtime_contract_types_chat.go to delegate token %q", token)
		}
	}

	forbiddenProductivity := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractAskSupportOptions struct {",
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractToolingOptions struct {",
	}
	for _, token := range forbiddenProductivity {
		if strings.Contains(productivitySource, token) {
			t.Fatalf("expected runtime_contract_types_productivity.go to delegate token %q", token)
		}
	}

	forbiddenScheduler := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractAskSupportOptions struct {",
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractToolingOptions struct {",
	}
	for _, token := range forbiddenScheduler {
		if strings.Contains(schedulerSource, token) {
			t.Fatalf("expected runtime_contract_types_scheduler.go to delegate token %q", token)
		}
	}

	forbiddenTooling := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractAskSupportOptions struct {",
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractProductivityOptions struct {",
		"type routeRuntimeContractAnalyzeOptions struct {",
	}
	for _, token := range forbiddenTooling {
		if strings.Contains(toolingSource, token) {
			t.Fatalf("expected runtime_contract_types_tooling.go to delegate token %q", token)
		}
	}

	forbiddenAnalyze := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractAskSupportOptions struct {",
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractToolingOptions struct {",
	}
	for _, token := range forbiddenAnalyze {
		if strings.Contains(analyzeSource, token) {
			t.Fatalf("expected runtime_contract_types_analyze.go to delegate token %q", token)
		}
	}

	forbiddenSupport := []string{
		"type routeRuntimeContractChatOptions struct {",
		"type routeRuntimeContractProductivityOptions struct {",
		"type routeRuntimeContractActivationOptions struct {",
	}
	for _, token := range forbiddenSupport {
		if strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_contract_types_tool_support.go to delegate token %q", token)
		}
	}

	forbiddenActivation := []string{
		"type routeRuntimeContractAuthSurfaceOptions struct {",
		"type routeRuntimeContractCapabilitySupportOptions struct {",
		"type routeRuntimeContractDeferredSupportOptions struct {",
	}
	for _, token := range forbiddenActivation {
		if strings.Contains(activationSource, token) {
			t.Fatalf("expected runtime_contract_types_activation.go to delegate token %q", token)
		}
	}

	forbiddenAuthSurface := []string{
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractCapabilitySupportOptions struct {",
		"type routeRuntimeContractDeferredSupportOptions struct {",
	}
	for _, token := range forbiddenAuthSurface {
		if strings.Contains(authSurfaceSource, token) {
			t.Fatalf("expected runtime_contract_types_auth_surface.go to delegate token %q", token)
		}
	}

	forbiddenCapability := []string{
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractAuthSurfaceOptions struct {",
		"type routeRuntimeContractDeferredSupportOptions struct {",
	}
	for _, token := range forbiddenCapability {
		if strings.Contains(capabilitySource, token) {
			t.Fatalf("expected runtime_contract_types_capability_support.go to delegate token %q", token)
		}
	}

	forbiddenDeferred := []string{
		"type routeRuntimeContractActivationOptions struct {",
		"type routeRuntimeContractAuthSurfaceOptions struct {",
		"type routeRuntimeContractCapabilitySupportOptions struct {",
	}
	for _, token := range forbiddenDeferred {
		if strings.Contains(deferredSource, token) {
			t.Fatalf("expected runtime_contract_types_deferred_support.go to delegate token %q", token)
		}
	}
}

func TestRoutesGo_UsesRouteRuntimeContractBoundary(t *testing.T) {
	routeContent, err := os.ReadFile(filepath.Join("routes.go"))
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	registrationFiles, err := filepath.Glob("runtime_route_registration*.go")
	if err != nil {
		t.Fatalf("glob runtime_route_registration*.go: %v", err)
	}
	registrationFiles = filterRuntimeRouteRegistrationFiles(registrationFiles)
	if len(registrationFiles) == 0 {
		t.Fatal("expected runtime_route_registration*.go files")
	}
	routeSource := string(routeContent)
	var registrationBuilder strings.Builder
	for _, name := range registrationFiles {
		content, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		registrationBuilder.Write(content)
		registrationBuilder.WriteString("\n")
	}
	registrationSource := registrationBuilder.String()

	routeRequired := []string{
		"newRouteRegistrationState(",
		"bindRouteRuntimeRegistration(",
	}
	for _, token := range routeRequired {
		if !strings.Contains(routeSource, token) {
			t.Fatalf("expected routes.go to delegate via phase token %q", token)
		}
	}

	for _, token := range []string{
		"bindRouteRuntimeFastPath(",
		"bindRouteRuntimeDeferredPath(",
	} {
		if !strings.Contains(registrationSource, token) {
			t.Fatalf("expected runtime_route_registration*.go to retain path orchestration token %q", token)
		}
	}

	required := []string{
		"newRouteRuntimeContract(",
		".NewRuntimeLLMRef(",
		".BindStartupAuthRuntime(",
		".BindBootstrapPhaseRuntime(",
		".BindInfrastructureRuntime(",
		".BindExperienceRuntime(",
		".BindCoreToolingRuntime(",
		".BindCoreSupportRuntime(",
		".BindManagementRuntime(",
		".BindOperationalRuntime(",
	}
	for _, token := range required {
		if !strings.Contains(registrationSource, token) {
			t.Fatalf("expected runtime_route_registration*.go to consume route runtime contract token %q", token)
		}
	}

	boundarySource := routeSource + "\n" + registrationSource
	forbidden := []string{
		"newRuntimeCapabilityContract(",
		"newRuntimeContractBinding(",
		"newRuntimeLLMProviderRef(",
		"type RoutesDeps struct",
		"func resolveMCPWorkspaceRoot(",
		"type proxyBridgeLLMCaller struct",
		"type providerRegistryLLMCaller struct",
		"func providerSupportsModel(",
		"func tryExecuteToolFallback(",
		"func toolResultToIPCData(",
		"func flattenToolResultMap(",
		"func resolveRequestUserID(",
		"func fallbackFirstProvider(",
		"func convertFallbackPublicSpaces(",
		"func InitMetrics(",
		"func InitMetricsWithReadDB(",
		"type sandboxExecAdapter struct",
		".bindChatResearch(",
		".registerTaskSurface(",
		"bindRuntimePromptGuard(",
		"bindRuntimeToolSelection(",
		"bindRuntimeLayeredMemory(",
		"NewLocalEmailService(",
		"NewLocalEmailServiceWithReadDB(",
		"RegisterEmailTool(",
		"NewLocalCalendarService(",
		"NewLocalCalendarServiceWithReadDB(",
		"RegisterCalendarTool(",
		"newRuntimeUIReviewerTool(",
		"bindRuntimeBrowserTargets(",
		"bindRuntimeSchedulerServices(",
		"bindRuntimeReminderServices(",
		"bindRuntimeAnalyzeTool(",
		"mediagen.NewMediaStorage(",
		"mediagen.NewManager(",
		"mediagen.NewHandler(",
		"sockipc.NewServer(",
		"sockipc.RegisterMediaHandlers(",
		"sockipc.RegisterSkillFallback(",
		"server.NewSkillHandler(",
		"skillstore.NewStore(",
		"skillstore.NewStoreWithReadDB(",
		"skillstore.NewSyncService(",
		"skillstore.NewFeaturedSkillsLoader(",
		"skillstore.NewLocalSkillScanner(",
		"skillmarket.DefaultConfig(",
		"skillmarket.NewServiceWithDBPath(",
		"sockipc.RegisterSkillManagerHandlers(",
		"newSkillManagerAdapter(",
		".ReleaseSkills(",
		"heartbeat.NewRunner(",
		"heartbeat.NewHandler(",
		"webpush.GetOrCreateVAPIDKeys(",
		"providerpool.NewHandler(",
		"oauth.NewLazyManager(",
		".SetMediaPricingLookup(",
		".SetMediaPricingApplier(",
		".RegisterOAuthCallbackRoute(",
		".StartAutoRefresh(",
		"channel.NewManager(",
		"server.NewChannelFactory(",
		"server.NewChannelConfigHandler(",
		".SetChannelSender(",
		".SetChannelSenderWithID(",
		".SetChannelMessageUpdater(",
		"networkapi.NewSDKRemoteAccessHandler(",
		"update.NewHandler(",
		"update.NewOTAChecker(",
		"server.NewProviderSettingsHandler(",
		".SetResumeRecoverer(",
		".SetOTAChecker(",
		".StartAutoUpdater(",
		"bindRuntimeImageTools(",
		".BindStartupSurfaceRuntime(",
		".BindAuthSurfaceRuntime(",
		".BindAccountSurfaceRuntime(",
		".BindShellSurfaceRuntime(",
		".BindBootstrapSupportRuntime(",
		"newRuntimeAskSupportBundle(",
		"newRuntimeExecSupportBundle(",
		".BindSchedulerServices(",
		".BindTooling(",
		".BindAnalyzeTool(",
		".BindProviderPoolRuntime(",
		".NewAskSupportBundle(",
		".NewExecSupportBundle(",
		".BindCapabilitySupportRuntime(",
		".RegisterMgmtTool(",
		".BindHeartbeatRuntime(",
		".BindManagementSupport(",
		".BindUserSurfaceRuntime(",
		".BindMgmtUpgrade(",
		".BindChannelRuntime(",
		"bindRuntimeMgmtTool(",
		"bindRuntimeMgmtUpgrade(",
		"bindRuntimeTTSTool(",
		"registerCurrentUserRoutes(",
		".RegisterCurrentUserRoutes(",
		"permission.NewRepository(",
		"permission.NewRepositoryWithReadDB(",
		"permission.NewService(",
		"permission.NewHandler(",
		".Authenticate()",
		"server.NewSettingsHandler(",
		".SetChatHandler(",
		"smallmodel.NewManager(",
		".SetSmallModelManager(",
		"sse.NewHandler(",
		"networkapi.NewApprovalHandler(",
		"voicewake.NewManager(",
		"voicewake.NewHandler(",
		"networkapi.NewTunnelHandler(",
		"permHandler.RegisterAdminRoutes(",
		"deps.UserHandler.ChangePassword",
		"deps.APIKeyHandler.RegisterRoutes(",
		"registerAutoreplyRoutes(",
		"preview.NewModeService(",
		"preview.NewUpgradeService(",
		"preview.NewUpgradeServiceWithReadDB(",
		"preview.NewHandler(",
		"registerPublicAuthRoutes(",
		".SetPreviewModeChecker(",
		".SetModeService(",
		"security.SetGlobalTLSManagerConfig(",
		"security.GetGlobalTLSManager(",
		".LoadSettings(",
		".LoadCertificate(",
		".RequestACMECertificate(",
		".HTTPSRedirectMiddleware(",
		".SetSSEBroker(",
		"OptionalAuthenticate()",
		".RegisterRoutes(chatGroup)",
		"web.RegisterStaticRoutes(",
		`v1.Static("/media",`,
		`v1.GET("/health",`,
		`v1.GET("/health/stats",`,
		`v1.GET("/workers/stats",`,
		"server.NewConfigHandler(",
		"server.NewTemplatesHandler(",
		"registerFormfillerRoutes(",
		"registerExternalAuthRoutes(",
		"registerRuntimeExecSupportRoutes(",
		"registerRuntimeAskSupportRoutes(",
		"registerBackupRoutes(",
		"registerSecurityRoutes(",
		"registerSandboxRoutes(",
		"registerCronRoutes(",
		"registerBrowserAutomationRoutes(",
		"registerWorkflowRoutes(",
		"registerVoiceRoutes(",
		"registerSpeechRoutes(",
		".registerConvertRoutes(",
		"tools.RegisterCanvasTools(",
		"a2ui.NewHandler(",
		"tools.RegisterPDFTool(",
		"mfa.NewHandler(",
		"billing.NewService(",
		"billing.NewHandler(",
		"registerBillingRoutes(protected,",
		"networkapi.NewNetworkHandler(",
		"server.OnServerStart(",
		"networkapi.NewLinkPreviewHandler(",
		"server.NewMetricsHandler(",
		"metrics.NewHandler(deps.MetricsWriter)",
		"server.NewSystemHandler(",
		"server.NewServiceHandler(",
		"connection.NewHandler(",
		"tools.RegisterGatewayTool(",
		"registerGatewayMethods(",
		"deps.GatewayHandler.RegisterRoutes(",
		"gatewayStopper{gateway: deps.Gateway}",
		"registerPluginTools(",
		"server.NewPluginHandler(",
		"server.NewPluginStoreHandler(",
		"server.NewToolStoreHandler(",
		"activateRouteRuntimeActivation(",
		"registerRuntimeActivationSupportRoutes(",
		"bindRuntimeActivationSupport(",
		"registerCompanionRoutes(",
		"registerUserScopedRoutes(",
		"registerMyUsageRoute(",
		"server.NewUserProviderHandler(",
		"server.NewUserProviderHandlerWithReadDB(",
		"server.NewUserSkillHandler(",
		"server.NewUserSkillHandlerWithReadDB(",
		"deps.WorkspaceHandler.RegisterRoutes(",
		".ResearchService()",
		".ReflectService()",
		".ApprovalDetailTarget()",
		".HarnessRuntime()",
	}
	for _, token := range forbidden {
		if strings.Contains(boundarySource, token) {
			t.Fatalf("route registration boundary should not bypass route runtime contract via %q", token)
		}
	}
}

func TestRuntimeRouteRegistration_UsesOnlyPhaseRuntimeContractMethods(t *testing.T) {
	registrationFiles, err := filepath.Glob("runtime_route_registration*.go")
	if err != nil {
		t.Fatalf("glob runtime_route_registration*.go: %v", err)
	}
	registrationFiles = filterRuntimeRouteRegistrationFiles(registrationFiles)
	if len(registrationFiles) == 0 {
		t.Fatal("expected runtime_route_registration*.go files")
	}

	expectedMethodsByFile := map[string][]string{
		"runtime_route_registration_state_init.go":          {"NewRuntimeLLMRef"},
		"runtime_route_registration_fast_path.go":           {"BindBootstrapPhaseRuntime", "BindStartupAuthRuntime"},
		"runtime_route_registration_core_infrastructure.go": {"BindInfrastructureRuntime"},
		"runtime_route_registration_core_experience.go":     {"BindExperienceRuntime"},
		"runtime_route_registration_core_tooling.go":        {"BindCoreToolingRuntime"},
		"runtime_route_registration_core_support.go":        {"BindCoreSupportRuntime"},
		"runtime_route_registration_management_phase.go":    {"BindManagementRuntime"},
		"runtime_route_registration_operational_phase.go":   {"BindOperationalRuntime"},
	}

	methodPattern := regexp.MustCompile(`runtimeContract\.([A-Za-z0-9_]+)\(`)
	covered := make(map[string]bool, len(expectedMethodsByFile))

	for _, name := range registrationFiles {
		content, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}

		actual := uniqueRuntimeContractMethods(methodPattern.FindAllStringSubmatch(string(content), -1))
		expected, tracked := expectedMethodsByFile[filepath.Base(name)]
		if !tracked {
			if len(actual) > 0 {
				t.Fatalf("expected %s to avoid direct runtimeContract access, got %v", name, actual)
			}
			continue
		}

		expected = append([]string(nil), expected...)
		sort.Strings(expected)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("expected %s to use only phase runtime contract methods %v, got %v", name, expected, actual)
		}
		covered[filepath.Base(name)] = true
	}

	for name := range expectedMethodsByFile {
		if !covered[name] {
			t.Fatalf("expected anti-drift test to cover %s", name)
		}
	}
}

func TestPhaseRuntimeContracts_UseNarrowBindingInterfaces(t *testing.T) {
	expectedByFile := map[string][]string{
		"runtime_fast_path_contract.go": {
			"func bindRouteRuntimeStartupAuth(binding routeRuntimeFastPathBinding,",
			"func bindRouteRuntimeBootstrapPhase(binding routeRuntimeFastPathBinding,",
		},
		"runtime_infrastructure_contract.go": {
			"func bindRouteRuntimeInfrastructure(binding routeRuntimeInfrastructureBinding,",
		},
		"runtime_experience_contract.go": {
			"func bindRouteRuntimeExperience(binding routeRuntimeExperienceBinding,",
		},
		"runtime_core_tooling_contract.go": {
			"func bindRouteRuntimeCoreToolingContract(binding routeRuntimeCoreToolingBinding,",
		},
		"runtime_core_support_contract.go": {
			"func bindRouteRuntimeSupport(binding routeRuntimeCoreSupportBinding,",
		},
		"runtime_management_runtime_contract.go": {
			"func bindRouteRuntimeManagement(binding routeRuntimeManagementBinding,",
		},
		"runtime_operational_contract.go": {
			"func bindRouteRuntimeOperational(binding routeRuntimeOperationalBinding,",
		},
	}

	requiredTypeTokens := []string{
		"var _ routeRuntimeFastPathBinding = (*runtimeContractBinding)(nil)",
		"var _ routeRuntimeInfrastructureBinding = (*runtimeContractBinding)(nil)",
		"var _ routeRuntimeExperienceBinding = (*runtimeContractBinding)(nil)",
		"var _ routeRuntimeCoreToolingBinding = (*runtimeContractBinding)(nil)",
		"var _ routeRuntimeCoreSupportBinding = (*runtimeContractBinding)(nil)",
		"var _ routeRuntimeManagementBinding = (*runtimeContractBinding)(nil)",
		"var _ routeRuntimeOperationalBinding = (*runtimeContractBinding)(nil)",
	}

	for name, required := range expectedByFile {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		for _, token := range required {
			if !strings.Contains(source, token) {
				t.Fatalf("expected %s to contain narrow binding token %q", name, token)
			}
		}
		if strings.Contains(source, "binding *runtimeContractBinding,") {
			t.Fatalf("expected %s to hide concrete runtime binding helper parameters", name)
		}
	}

	typeFiles := []string{
		"runtime_fast_path_contract_types.go",
		"runtime_infrastructure_contract_types.go",
		"runtime_experience_contract_types.go",
		"runtime_core_tooling_contract_types.go",
		"runtime_core_support_contract_types.go",
		"runtime_management_runtime_contract_types.go",
		"runtime_operational_contract_types.go",
	}
	var typeSource strings.Builder
	for _, name := range typeFiles {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		typeSource.Write(content)
		typeSource.WriteString("\n")
	}
	combined := typeSource.String()
	for _, token := range requiredTypeTokens {
		if !strings.Contains(combined, token) {
			t.Fatalf("expected phase runtime contract types to keep token %q", token)
		}
	}
}

func filterRuntimeRouteRegistrationFiles(files []string) []string {
	filtered := make([]string, 0, len(files))
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
}

func uniqueRuntimeContractMethods(matches [][]string) []string {
	set := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		set[match[1]] = struct{}{}
	}
	methods := make([]string, 0, len(set))
	for method := range set {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

type stubRuntimeContractAnalyzeSkill struct {
	*stubRuntimeSkill
	*stubRuntimeAnalyzeSkillTarget
}

type stubRuntimeContractWebSearchSkill struct {
	*stubRuntimeSkill
	*stubRuntimeWebSearchSkillTarget
}

type stubRuntimeContractDeepResearchSkill struct {
	*stubRuntimeSkill
	*stubRuntimeDeepResearchSkillTarget
}

type stubRuntimeContractSchedulerSkill struct {
	*stubRuntimeSkill
	*stubRuntimeSchedulerSkillTarget
}

type stubRuntimeContractBrowserSkill struct {
	*stubRuntimeSkill
	*stubRuntimeBrowserSkillTarget
}

type stubRuntimeContractUIReviewerSkill struct {
	*stubRuntimeSkill
	*stubRuntimeUIReviewerSkillTarget
}

type stubRuntimeContractReminderSkill struct {
	*stubRuntimeSkill
	*stubRuntimeReminderSkillTarget
}

type stubRuntimeContractEmailSkill struct {
	*stubRuntimeSkill
	*stubRuntimeEmailSkillTarget
}

type stubRuntimeContractCalendarSkill struct {
	*stubRuntimeSkill
	*stubRuntimeCalendarSkillTarget
}

func newStubRuntimeContractSkill(id string) *stubRuntimeSkill {
	return &stubRuntimeSkill{
		manifest: &skillpkg.Manifest{ID: id, Name: id},
		result:   &skillpkg.Result{Success: true},
	}
}

func newStubRuntimeContractSkillRegistry(skills map[string]skillpkg.Skill) *stubRuntimeSkillRegistrySource {
	enabled := make(map[string]bool, len(skills))
	for id := range skills {
		enabled[id] = true
	}
	return &stubRuntimeSkillRegistrySource{
		skills:  skills,
		enabled: enabled,
	}
}

type stubRuntimeEmailSkillTarget struct {
	executor builtin.EmailExecutor
	calls    int
}

func (s *stubRuntimeEmailSkillTarget) SetExecutor(e builtin.EmailExecutor) {
	s.executor = e
	s.calls++
}

type stubRuntimeCalendarSkillTarget struct {
	executor builtin.CalendarExecutor
	calls    int
}

func (s *stubRuntimeCalendarSkillTarget) SetExecutor(e builtin.CalendarExecutor) {
	s.executor = e
	s.calls++
}
