package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/labstack/echo/v4"
)

func TestNewHarnessRuntimeBundle_DisabledOrMissingConfig(t *testing.T) {
	bundle, err := newHarnessRuntimeBundle(nil, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if bundle != nil {
		t.Fatalf("expected nil bundle when config is missing, got %#v", bundle)
	}

	cfg := &config.Config{}
	bundle, err = newHarnessRuntimeBundle(nil, cfg, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if bundle != nil {
		t.Fatalf("expected nil bundle when harness is disabled, got %#v", bundle)
	}
}

func TestNewHarnessRuntimeBundle_EnabledCreatesComponents(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-bindings.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	reflectService := selfreflect.NewService(nil, nil)
	bundle, err := newHarnessRuntimeBundle(db, cfg, reflectService)
	if err != nil {
		t.Fatalf("newHarnessRuntimeBundle failed: %v", err)
	}
	if bundle == nil || bundle.Controller == nil || bundle.GroupDispatcher == nil {
		t.Fatalf("expected initialized bundle, got %#v", bundle)
	}
	if bundle.RuntimeObserver == nil || bundle.SubagentExecutor == nil || bundle.WriteGuard == nil || bundle.ExecGuard == nil {
		t.Fatalf("expected runtime guards/observer to be wired, got %#v", bundle)
	}
}

func TestNewResearchHarnessRunSpecCopiesNormalizedFields(t *testing.T) {
	spec := newResearchHarnessRunSpec(researchHarnessRunInput{
		Query:          " latest pricing ",
		UserID:         " user-1 ",
		ConversationID: " conv-1 ",
		WorkspaceRoot:  " /tmp/workspace ",
		ProviderID:     " openai-prod ",
		Mode:           "auto",
		ResearchDepth:  "deep",
		RouteMode:      "hybrid",
		Lang:           "en",
		ReportStyle:    "brief",
		TimeWindows:    []string{"2026-Q1"},
		StrictEntity:   true,
		MaxSources:     8,
		MaxSeconds:     120,
	})

	if spec.Goal != "latest pricing" || spec.UserID != "user-1" || spec.ConversationID != "conv-1" {
		t.Fatalf("unexpected normalized spec: %#v", spec)
	}
	if spec.WorkspaceRoot != "/tmp/workspace" {
		t.Fatalf("WorkspaceRoot = %q, want /tmp/workspace", spec.WorkspaceRoot)
	}
	if spec.ProviderID != "openai-prod" {
		t.Fatalf("ProviderID = %q, want openai-prod", spec.ProviderID)
	}
	if spec.Metadata["mode"] != "auto" || spec.Metadata["research_depth"] != "deep" {
		t.Fatalf("unexpected research mode metadata: %#v", spec.Metadata)
	}
	if spec.Metadata["strict_entity"] != true || spec.Metadata["max_sources"] != 8 || spec.Metadata["max_seconds"] != 120 {
		t.Fatalf("unexpected metadata: %#v", spec.Metadata)
	}
}

func TestNewResearchHarnessRunSpecIncludesModeSpecificMetadata(t *testing.T) {
	spec := newResearchHarnessRunSpec(researchHarnessRunInput{
		Query:            " Competitive pricing snapshot ",
		Mode:             "analyze",
		RetrievalProfile: " recent_multi_site_v1 ",
		Topic:            " Pricing snapshot ",
		URLs:             []string{"https://example.com/pricing", "https://example.com/blog"},
		Text:             " Internal notes ",
		SearchQueries:    []string{"example pricing comparison", "competitor plan changes"},
		OutputMode:       "report",
		Action:           "check_accessibility",
		URL:              " https://example.com/app ",
		Image:            " base64-image ",
		Device:           " mobile ",
		Channel:          " telegram ",
		WaitMS:           1500,
		Threshold:        82.5,
		Format:           " human ",
		Profile:          " ppt ",
	})

	if spec.Metadata["topic"] != "Pricing snapshot" {
		t.Fatalf("topic metadata = %#v, want trimmed topic", spec.Metadata["topic"])
	}
	if got, ok := spec.Metadata["urls"].([]string); !ok || !reflect.DeepEqual(got, []string{"https://example.com/pricing", "https://example.com/blog"}) {
		t.Fatalf("urls metadata = %#v, want forwarded urls", spec.Metadata["urls"])
	}
	if got, ok := spec.Metadata["search_queries"].([]string); !ok || !reflect.DeepEqual(got, []string{"example pricing comparison", "competitor plan changes"}) {
		t.Fatalf("search_queries metadata = %#v, want forwarded search queries", spec.Metadata["search_queries"])
	}
	if spec.Metadata["text"] != "Internal notes" || spec.Metadata["output_mode"] != "report" {
		t.Fatalf("unexpected analyze metadata: %#v", spec.Metadata)
	}
	if spec.Metadata["retrieval_profile"] != "recent_multi_site_v1" {
		t.Fatalf("retrieval_profile metadata = %#v, want recent_multi_site_v1", spec.Metadata["retrieval_profile"])
	}
	if spec.Metadata["action"] != "check_accessibility" || spec.Metadata["url"] != "https://example.com/app" || spec.Metadata["image"] != "base64-image" {
		t.Fatalf("unexpected ui-review metadata: %#v", spec.Metadata)
	}
	if spec.Metadata["device"] != "mobile" || spec.Metadata["channel"] != "telegram" || spec.Metadata["wait_ms"] != 1500 {
		t.Fatalf("unexpected ui-review metadata: %#v", spec.Metadata)
	}
	if spec.Metadata["threshold"] != 82.5 || spec.Metadata["format"] != "human" || spec.Metadata["profile"] != "ppt" {
		t.Fatalf("unexpected ui-review metadata: %#v", spec.Metadata)
	}
}

func TestNewWorkflowHarnessRunSpecCopiesNormalizedFields(t *testing.T) {
	spec := newWorkflowHarnessRunSpec(workflowHarnessRunInput{
		WorkflowID:     " wf-1 ",
		WorkflowName:   " Nightly Sync ",
		TriggerType:    workflow.TriggerTypeWebhook,
		UserID:         " user-1 ",
		ConversationID: " conv-1 ",
		TenantID:       " tenant-1 ",
		WorkspaceRoot:  " /tmp/workspace ",
		TriggerData: map[string]interface{}{
			"source": "manual",
		},
	})

	if spec.Kind != harness.RunKindWorkflow || spec.Goal != "Nightly Sync" {
		t.Fatalf("unexpected workflow spec: %#v", spec)
	}
	if spec.UserID != "user-1" || spec.ConversationID != "conv-1" || spec.SessionID != "conv-1" {
		t.Fatalf("unexpected identity fields: %#v", spec)
	}
	if spec.WorkspaceRoot != "/tmp/workspace" {
		t.Fatalf("WorkspaceRoot = %q, want /tmp/workspace", spec.WorkspaceRoot)
	}
	if spec.Metadata["workflow_id"] != "wf-1" || spec.Metadata["tenant_id"] != "tenant-1" || spec.Metadata["trigger_type"] != string(workflow.TriggerTypeWebhook) {
		t.Fatalf("unexpected metadata: %#v", spec.Metadata)
	}
}

func TestNewResearchRuntimeBinding_FallsBackToBrokerWithoutHarness(t *testing.T) {
	service := deepresearch.NewService(nil, nil)
	broker := sse.NewBroker()

	binding := newResearchRuntimeBinding(nil, service, broker)
	if binding.driver != nil {
		t.Fatalf("expected no harness driver without runtime bundle, got %#v", binding.driver)
	}
	if binding.eventPublisher != broker {
		t.Fatalf("expected broker publisher fallback, got %#v", binding.eventPublisher)
	}

	target := &stubResearchPublisherTarget{}
	binding.apply(target)
	if target.publisher != broker {
		t.Fatalf("expected broker to be applied, got %#v", target.publisher)
	}
}

func TestNewResearchRuntimeBinding_UsesHarnessDriverWhenBundlePresent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	service := deepresearch.NewService(nil, nil)
	broker := sse.NewBroker()

	binding := newResearchRuntimeBinding(bundle, service, broker)
	if binding.driver == nil {
		t.Fatalf("expected harness-backed research driver, got %#v", binding)
	}
	if binding.eventPublisher != binding.driver {
		t.Fatalf("expected harness driver publisher, got %#v", binding.eventPublisher)
	}

	target := &stubResearchPublisherTarget{}
	binding.apply(target)
	if target.publisher != binding.driver {
		t.Fatalf("expected harness driver to be applied, got %#v", target.publisher)
	}
}

func TestNewChatResearchRuntimeBinding_AppliesFallbackWithoutHarnessRuntime(t *testing.T) {
	service := deepresearch.NewService(nil, nil)
	broker := sse.NewBroker()

	binding := newChatResearchRuntimeBinding(nil, service, &serverpkg.ChatHandler{}, broker, "/tmp/workspace")
	if binding.service != service {
		t.Fatalf("expected deep research service to be preserved, got %#v", binding.service)
	}
	if binding.turnHook != nil {
		t.Fatalf("expected no auto-harness hook without runtime, got %#v", binding.turnHook)
	}
	if binding.research.driver != nil || binding.research.eventPublisher != broker {
		t.Fatalf("expected broker-backed research runtime without harness, got %#v", binding.research)
	}
	if binding.toolAdapter == nil || binding.toolAdapter.service != service || binding.toolAdapter.manager != nil {
		t.Fatalf("expected fallback tool adapter without harness manager, got %#v", binding.toolAdapter)
	}

	target := &stubChatResearchRuntimeTarget{}
	binding.applyChat(target)
	if target.service != service || target.serviceCalls != 1 {
		t.Fatalf("expected deep research service to be applied once, got %#v", target)
	}
	if target.hookCalls != 0 || target.hook != nil {
		t.Fatalf("expected no hook registration without harness runtime, got %#v", target)
	}

	researchTarget := &stubResearchPublisherTarget{}
	binding.applyResearchService(researchTarget)
	if researchTarget.publisher != broker {
		t.Fatalf("expected broker publisher to be applied, got %#v", researchTarget.publisher)
	}

	registry := tools.NewRegistry()
	binding.register(nil, registry)
	if registry.Get("research") == nil {
		t.Fatalf("expected research tool to be registered, got %#v", registry.List())
	}
}

func TestNewChatResearchRuntimeBinding_UsesHarnessRuntimeEdgesWhenPresent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	service := deepresearch.NewService(nil, nil)
	broker := sse.NewBroker()

	binding := newChatResearchRuntimeBinding(bundle, service, &serverpkg.ChatHandler{}, broker, "/tmp/workspace")
	if binding.service != service {
		t.Fatalf("expected deep research service to be preserved, got %#v", binding.service)
	}
	if binding.turnHook == nil {
		t.Fatal("expected auto-harness hook when runtime is enabled")
	}
	if binding.research.driver == nil || binding.research.eventPublisher != binding.research.driver {
		t.Fatalf("expected harness-backed research runtime, got %#v", binding.research)
	}
	if binding.toolAdapter == nil || binding.toolAdapter.manager != harnessRuntimeController(bundle) {
		t.Fatalf("expected harness-backed research tool adapter, got %#v", binding.toolAdapter)
	}

	target := &stubChatResearchRuntimeTarget{}
	binding.applyChat(target)
	if target.service != service || target.serviceCalls != 1 {
		t.Fatalf("expected deep research service to be applied once, got %#v", target)
	}
	if target.hook == nil || target.hookCalls != 1 {
		t.Fatalf("expected auto-harness hook registration, got %#v", target)
	}

	researchTarget := &stubResearchPublisherTarget{}
	binding.applyResearchService(researchTarget)
	if researchTarget.publisher != binding.research.driver {
		t.Fatalf("expected harness research driver to be applied, got %#v", researchTarget.publisher)
	}

	registry := tools.NewRegistry()
	binding.register(harnessRuntimeController(bundle), registry)
	if registry.Get("research") == nil {
		t.Fatalf("expected research tool to be registered, got %#v", registry.List())
	}
}

func TestNewAgentRuntimeBinding_LeavesNativeModeWithoutHarness(t *testing.T) {
	db, _, store, runner := newTestAgentRuntimeFixture(t)
	defer db.Close()

	binding := newAgentRuntimeBinding(nil, runner, store)
	if binding.useCompatHandler {
		t.Fatalf("expected native route mode without harness runtime, got %#v", binding)
	}
	if binding.taskObserver != nil || binding.toolObserver != nil || binding.subagentExecutor != nil || binding.writeGuard != nil || binding.execGuard != nil {
		t.Fatalf("expected runtime hooks to stay unset without harness runtime, got %#v", binding)
	}
	if binding.agentDriver != nil || binding.subagentDriver != nil {
		t.Fatalf("expected no harness drivers without runtime bundle, got %#v", binding)
	}

	target := &stubAgentRuntimeTarget{}
	binding.apply(target)
	if target.callCount() != 0 {
		t.Fatalf("expected no runner wiring without harness runtime, got %#v", target)
	}

	registrarType := fmt.Sprintf("%T", binding.routeRegistrar(nil, store, runner, "/tmp/workspace"))
	if registrarType != "*agent.Handler" {
		t.Fatalf("expected native agent handler, got %s", registrarType)
	}
}

func TestNewAgentRuntimeBinding_UsesHarnessRuntimeEdgesWhenBundlePresent(t *testing.T) {
	db, bundle, store, runner := newTestAgentRuntimeFixture(t)
	defer db.Close()

	binding := newAgentRuntimeBinding(bundle, runner, store)
	if !binding.useCompatHandler {
		t.Fatalf("expected compat route mode with harness runtime, got %#v", binding)
	}
	if binding.taskObserver == nil || binding.agentDriver == nil || binding.subagentDriver == nil {
		t.Fatalf("expected harness task drivers to be wired, got %#v", binding)
	}
	if binding.toolObserver != bundle.RuntimeObserver {
		t.Fatalf("expected shared runtime observer, got %#v", binding.toolObserver)
	}
	if binding.subagentExecutor != bundle.SubagentExecutor || binding.writeGuard != bundle.WriteGuard || binding.execGuard != bundle.ExecGuard {
		t.Fatalf("expected shared harness guards/executor, got %#v", binding)
	}

	target := &stubAgentRuntimeTarget{}
	binding.apply(target)
	if target.eventObserver != binding.taskObserver {
		t.Fatalf("expected task observer wiring, got %#v", target.eventObserver)
	}
	if target.toolObserver != bundle.RuntimeObserver {
		t.Fatalf("expected tool observer wiring, got %#v", target.toolObserver)
	}
	if target.subagentExecutor != bundle.SubagentExecutor || target.writeGuard != bundle.WriteGuard || target.execGuard != bundle.ExecGuard {
		t.Fatalf("expected guards/executor wiring, got %#v", target)
	}
	if target.callCount() != 5 {
		t.Fatalf("expected five runtime wiring calls, got %d", target.callCount())
	}

	registrarType := fmt.Sprintf("%T", binding.routeRegistrar(harnessRuntimeController(bundle), store, runner, "/tmp/workspace"))
	if registrarType != "*harness.AgentCompatHandler" {
		t.Fatalf("expected harness compat handler, got %s", registrarType)
	}
}

func TestRegisterHarnessRuntimeAgentFeature_RegistersDisabledRoutesWithoutPrerequisites(t *testing.T) {
	e := echo.New()

	runner := registerHarnessRuntimeAgentFeature(
		nil,
		e.Group("/agent"),
		nil,
		nil,
		tools.NewRegistry(),
		tools.NewExecutor(nil),
		nil,
		"/tmp/workspace",
		zap.NewNop(),
		featureDisabled("agent"),
	)
	if runner != nil {
		t.Fatalf("expected nil runner without prerequisites, got %#v", runner)
	}
	if !routeExists(e, "GET", "/agent/*") {
		t.Fatalf("expected disabled catch-all route to be registered, got %#v", e.Routes())
	}
}

func TestRegisterHarnessRuntimeAgentFeature_RegistersAgentRoutesWhenReady(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent-feature.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	bundle, err := newHarnessRuntimeBundle(db, cfg, selfreflect.NewService(nil, nil))
	if err != nil {
		t.Fatalf("newHarnessRuntimeBundle failed: %v", err)
	}

	e := echo.New()
	registry := tools.NewRegistry()
	executor := tools.NewExecutor(registry)
	runner := registerHarnessRuntimeAgentFeature(
		bundle,
		e.Group("/agent"),
		db,
		&stubHarnessJudgeLLMCaller{},
		registry,
		executor,
		sse.NewBroker(),
		"/tmp/workspace",
		zap.NewNop(),
		featureDisabled("agent"),
	)
	if runner == nil {
		t.Fatal("expected agent runner to be registered")
	}
	if !routeExists(e, "POST", "/agent/tasks") {
		t.Fatalf("expected agent task routes to be registered, got %#v", e.Routes())
	}
	if routeExists(e, "GET", "/agent/tasks") {
		t.Fatalf("did not expect standalone agent task list route to remain registered, got %#v", e.Routes())
	}
}

func TestBindHarnessRuntimeToolObserver_OnlyBindsWhenObserverExists(t *testing.T) {
	target := &stubToolEventObserverTarget{}
	bindHarnessRuntimeToolObserver(nil, target)
	if target.calls != 0 {
		t.Fatalf("expected no tool observer binding without harness runtime, got %#v", target)
	}

	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	bindHarnessRuntimeToolObserver(bundle, target)
	if target.calls != 1 || target.observer != bundle.RuntimeObserver {
		t.Fatalf("expected harness tool observer binding, got %#v", target)
	}
}

func TestBindHarnessRuntimeObserver_OnlyBindsWhenObserverExists(t *testing.T) {
	target := &stubRuntimeObserverTarget{}
	bindHarnessRuntimeObserver(nil, target)
	if target.calls != 0 {
		t.Fatalf("expected no observer binding without harness runtime, got %#v", target)
	}

	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	bindHarnessRuntimeObserver(bundle, target)
	if target.calls != 1 || target.observer != bundle.RuntimeObserver {
		t.Fatalf("expected harness observer binding, got %#v", target)
	}
}

func TestBindRuntimeToolApprover_OnlyBindsWhenApproverExists(t *testing.T) {
	target := &stubToolApproverTarget{}
	bindRuntimeToolApprover(nil, target)
	if target.calls != 0 {
		t.Fatalf("expected no approver binding without approver, got %#v", target)
	}

	approver := &stubToolApprover{}
	bindRuntimeToolApprover(approver, target)
	if target.calls != 1 || target.approver != approver {
		t.Fatalf("expected approver binding, got %#v", target)
	}
}

func TestNewChatAskRuntimeBinding_PreservesSupportWithoutHarnessRuntime(t *testing.T) {
	questionMgr := tools.NewQuestionManager(nil, nil, time.Minute)
	checkpointMgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	siteStore := &tools.BrowserSiteAllowlistStore{}

	binding := newChatAskRuntimeBinding(nil, " /tmp/media ", questionMgr, checkpointMgr, siteStore)
	if binding.mediaDir != "/tmp/media" {
		t.Fatalf("expected trimmed media dir, got %q", binding.mediaDir)
	}
	if binding.questionMgr != questionMgr || binding.browserCheckpointMgr != checkpointMgr || binding.browserSiteStore != siteStore {
		t.Fatalf("expected ask support dependencies to be preserved, got %#v", binding)
	}
	if binding.runtimeEventObserver != nil {
		t.Fatalf("expected no runtime observer without harness bundle, got %#v", binding.runtimeEventObserver)
	}

	chatTarget := &stubChatAskRuntimeTarget{}
	binding.applyChat(chatTarget)
	if chatTarget.mediaDir != "/tmp/media" || chatTarget.questionMgr != questionMgr || chatTarget.browserCheckpointMgr != checkpointMgr || chatTarget.browserSiteStore != siteStore {
		t.Fatalf("expected chat ask support wiring, got %#v", chatTarget)
	}
	if chatTarget.toolObserverCalls != 0 || chatTarget.toolObserver != nil {
		t.Fatalf("expected no tool observer wiring without harness runtime, got %#v", chatTarget)
	}

	observerTarget := &stubRuntimeObserverTarget{}
	binding.applyQuestionManager(observerTarget)
	if observerTarget.calls != 0 || observerTarget.observer != nil {
		t.Fatalf("expected no question manager observer binding without runtime, got %#v", observerTarget)
	}
}

func TestNewChatAskRuntimeBinding_UsesHarnessRuntimeObserverWhenPresent(t *testing.T) {
	observer := &stubRuntimeObserver{}
	questionMgr := tools.NewQuestionManager(nil, nil, time.Minute)
	checkpointMgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	siteStore := &tools.BrowserSiteAllowlistStore{}
	bundle := &HarnessRuntimeBundle{RuntimeObserver: observer}

	binding := newChatAskRuntimeBinding(bundle, "/tmp/media", questionMgr, checkpointMgr, siteStore)
	if binding.runtimeEventObserver != observer {
		t.Fatalf("expected harness runtime observer to be preserved, got %#v", binding.runtimeEventObserver)
	}

	chatTarget := &stubChatAskRuntimeTarget{}
	binding.applyChat(chatTarget)
	if chatTarget.mediaDir != "/tmp/media" || chatTarget.questionMgr != questionMgr || chatTarget.browserCheckpointMgr != checkpointMgr || chatTarget.browserSiteStore != siteStore {
		t.Fatalf("expected chat ask support wiring, got %#v", chatTarget)
	}
	if chatTarget.toolObserver != observer || chatTarget.toolObserverCalls != 1 {
		t.Fatalf("expected tool observer wiring on chat target, got %#v", chatTarget)
	}

	observerTarget := &stubRuntimeObserverTarget{}
	binding.applyQuestionManager(observerTarget)
	if observerTarget.calls != 1 || observerTarget.observer != observer {
		t.Fatalf("expected question manager observer wiring, got %#v", observerTarget)
	}
}

func TestNewAgentRuntimeSupportBinding_AppliesReflectorAndMetrics(t *testing.T) {
	reflector := selfreflect.NewService(nil, nil)
	metrics := &stubMetricsRecorder{}

	binding := newAgentRuntimeSupportBinding(reflector, metrics)
	if binding.reflector != reflector || binding.metrics != metrics {
		t.Fatalf("expected agent runtime support binding to preserve dependencies, got %#v", binding)
	}

	target := &stubAgentRuntimeSupportTarget{}
	binding.apply(target)
	if target.reflector != reflector || target.reflectorCalls != 1 {
		t.Fatalf("expected reflector wiring, got %#v", target)
	}
	if target.metrics != metrics || target.metricsCalls != 1 {
		t.Fatalf("expected metrics wiring, got %#v", target)
	}
}

func TestNewRuntimeAskPolicyBinding_AppliesQuestionAndAgentPolicy(t *testing.T) {
	settings := &stubRuntimeAskPolicySettingsSource{
		autoConfirm:    true,
		askTimeoutSecs: 45,
		timeoutAction:  "error",
		maxToolRounds:  9,
		autoReflect:    false,
	}

	binding := newRuntimeAskPolicyBinding(settings)
	if binding.timeoutFunc == nil || binding.timeoutActionFunc == nil || binding.maxToolRoundsFunc == nil || binding.autoReflectFunc == nil {
		t.Fatalf("expected runtime ask policy functions to be wired, got %#v", binding)
	}

	questionTarget := &stubQuestionRuntimePolicyTarget{}
	agentTarget := &stubAgentRuntimePolicyTarget{}
	binding.applyQuestionManager(questionTarget)
	binding.applyAgent(agentTarget)

	if questionTarget.silentFunc != nil {
		t.Fatalf("expected silentFunc to be nil (ask-user-question should never auto-answer), got non-nil function")
	}
	if questionTarget.timeoutFunc == nil || questionTarget.timeoutFunc() != 45*time.Second {
		t.Fatalf("expected timeout func wiring, got %#v", questionTarget)
	}
	if questionTarget.timeoutActionFunc == nil || questionTarget.timeoutActionFunc() != "error" {
		t.Fatalf("expected timeout action wiring, got %#v", questionTarget)
	}
	if agentTarget.askTimeoutFunc == nil || agentTarget.askTimeoutFunc() != 45*time.Second {
		t.Fatalf("expected agent ask timeout wiring, got %#v", agentTarget)
	}
	if agentTarget.askTimeoutActionFunc == nil || agentTarget.askTimeoutActionFunc() != "error" {
		t.Fatalf("expected agent timeout action wiring, got %#v", agentTarget)
	}
	if agentTarget.maxToolRoundsFunc == nil || agentTarget.maxToolRoundsFunc() != 9 {
		t.Fatalf("expected agent max tool rounds wiring, got %#v", agentTarget)
	}
	if agentTarget.autoReflectFunc == nil || agentTarget.autoReflectFunc() {
		t.Fatalf("expected agent auto-reflect wiring, got %#v", agentTarget)
	}
}

func TestNewRuntimeAskPolicyBinding_DefaultsTimeoutWhenSourceReturnsNonPositive(t *testing.T) {
	settings := &stubRuntimeAskPolicySettingsSource{askTimeoutSecs: 0}
	binding := newRuntimeAskPolicyBinding(settings)
	if binding.timeoutFunc == nil {
		t.Fatalf("expected timeout function, got %#v", binding)
	}
	if got := binding.timeoutFunc(); got != 120*time.Second {
		t.Fatalf("timeoutFunc() = %s, want %s", got, 120*time.Second)
	}
}

func TestBindRuntimeLayeredMemory_WiresReadyCallbackToTargets(t *testing.T) {
	handler := &stubLayeredMemoryReadyTarget{}
	chat := &stubLayeredMemoryChatTarget{}
	agentTarget := &stubLayeredMemoryAgentTarget{}
	reflectTarget := &stubLayeredMemoryReflectionTarget{}

	bindRuntimeLayeredMemory(handler, chat, agentTarget, reflectTarget)
	if handler.readyHook == nil {
		t.Fatalf("expected layered memory ready hook, got %#v", handler)
	}

	layered := &memory.LayeredMemoryService{}
	handler.readyHook(layered)

	if chat.layeredMemory != layered || chat.calls != 1 {
		t.Fatalf("expected layered memory to be applied to chat target, got %#v", chat)
	}
	agentMemory, ok := agentTarget.memory.(*agentMemoryAdapter)
	if !ok || agentMemory.svc != layered || agentTarget.calls != 1 {
		t.Fatalf("expected agent memory adapter wiring, got %#v", agentTarget)
	}
	reflectWriter, ok := reflectTarget.writer.(*agentReflectionMemoryWriter)
	if !ok || reflectWriter.svc != layered || reflectTarget.calls != 1 {
		t.Fatalf("expected reflection memory writer wiring, got %#v", reflectTarget)
	}
}

func TestBindRuntimeSettingsTargets_WiresSettingsDrivenTargets(t *testing.T) {
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	provider := &stubSettingsHandlerTarget{}
	chat := &stubSettingsHandlerTarget{}
	research := &stubRuntimeResearchSettingsTarget{}
	exec := &stubRuntimeAutoConfirmTarget{}
	prompt := &stubRuntimePromptSettingsTarget{}
	push := &stubRuntimeLocaleTarget{}
	masker := &stubRuntimeLocaleTarget{}
	mgmt := &stubRuntimeAdminSettingsTarget{}

	bindRuntimeSettingsTargets(settings, provider, chat, research, exec, prompt, push, masker, mgmt)

	if provider.handler != settings || provider.calls != 1 {
		t.Fatalf("expected provider settings handler wiring, got %#v", provider)
	}
	if chat.handler != settings || chat.calls != 1 {
		t.Fatalf("expected chat settings handler wiring, got %#v", chat)
	}
	if research.calls != 1 || research.enabled != settings.GetDeepResearchV2Enabled() {
		t.Fatalf("expected deep research settings wiring, got %#v", research)
	}
	if exec.calls != 1 || exec.autoConfirmFunc == nil || exec.autoConfirmFunc() != settings.GetAgentAutoConfirm() {
		t.Fatalf("expected exec auto-confirm wiring, got %#v", exec)
	}
	if prompt.localeFunc == nil || prompt.timezoneFunc == nil || prompt.agentModeFunc == nil || prompt.agentAutoConfirmFunc == nil {
		t.Fatalf("expected prompt settings wiring, got %#v", prompt)
	}
	if push.calls != 1 || push.localeFunc == nil {
		t.Fatalf("expected push locale wiring, got %#v", push)
	}
	if masker.calls != 1 || masker.localeFunc == nil {
		t.Fatalf("expected data masker locale wiring, got %#v", masker)
	}
	if mgmt.calls != 1 || mgmt.settings == nil {
		t.Fatalf("expected mgmt settings wiring, got %#v", mgmt)
	}
}

func TestBindRuntimeSettingsTargets_SkipsWithoutSettingsHandler(t *testing.T) {
	provider := &stubSettingsHandlerTarget{}
	chat := &stubSettingsHandlerTarget{}
	research := &stubRuntimeResearchSettingsTarget{}
	exec := &stubRuntimeAutoConfirmTarget{}
	prompt := &stubRuntimePromptSettingsTarget{}
	push := &stubRuntimeLocaleTarget{}
	masker := &stubRuntimeLocaleTarget{}
	mgmt := &stubRuntimeAdminSettingsTarget{}

	bindRuntimeSettingsTargets(nil, provider, chat, research, exec, prompt, push, masker, mgmt)

	if provider.calls != 0 || chat.calls != 0 || research.calls != 0 || exec.calls != 0 || prompt.calls() != 0 || push.calls != 0 || masker.calls != 0 || mgmt.calls != 0 {
		t.Fatalf("expected no target wiring without settings handler, got provider=%#v chat=%#v research=%#v exec=%#v prompt=%#v push=%#v masker=%#v mgmt=%#v", provider, chat, research, exec, prompt, push, masker, mgmt)
	}
}

func TestBindDeferredRuntimeWiring_ComposesDeferredRuntimeTargets(t *testing.T) {
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	smallRuntime := &stubSmallModelRuntime{}
	chatSmallModel := &stubRuntimeSmallModelChatTarget{}
	auxiliarySmallModel := &stubRuntimeSmallModelAuxiliaryTarget{}
	imageSmallModel := &stubRuntimeImageSmallModelTarget{}
	analyzeSmallModel := &stubRuntimeAnalyzeSmallModelTarget{}
	statsSource := &stubRuntimeSmallModelStatsSource{stats: &serverpkg.SmallModelStats{}}
	reflectTarget := &stubReflectionRuntimeTarget{}
	compactorChat := &stubRuntimeCompactorMemoryChatTarget{}
	provider := &stubSettingsHandlerTarget{}
	chatSettings := &stubSettingsHandlerTarget{}
	research := &stubRuntimeResearchSettingsTarget{}
	exec := &stubRuntimeAutoConfirmTarget{}
	prompt := &stubRuntimePromptSettingsTarget{}
	push := &stubRuntimeLocaleTarget{}
	masker := &stubRuntimeLocaleTarget{}
	mgmt := &stubRuntimeAdminSettingsTarget{}
	questionTarget := &stubQuestionRuntimePolicyTarget{}
	agentTarget := &stubAgentRuntimePolicyTarget{}

	bindDeferredRuntimeWiring(context.Background(), runtimeDeferredWiring{
		settings:               settings,
		smallRuntime:           smallRuntime,
		chatSmallModel:         chatSmallModel,
		auxiliarySmallModel:    auxiliarySmallModel,
		imageSmallModel:        imageSmallModel,
		analyzeSmallModel:      analyzeSmallModel,
		smallModelStats:        statsSource,
		reflectionTarget:       reflectTarget,
		reflectionLLM:          &stubHarnessJudgeLLMCaller{},
		reflectionProposalGate: func() bool { return true },
		compactorChat:          compactorChat,
		sessionCompaction:      config.SessionCompactionConfig{},
		sessionMaxTokens:       2048,
		providerSettings:       provider,
		chatSettings:           chatSettings,
		researchSettings:       research,
		skillRerankerDefaults: runtimeSkillRerankerDefaults{
			dataDir:   t.TempDir(),
			modelRepo: "repo/model",
		},
		execAutoConfirm: exec,
		promptSettings:  prompt,
		pushLocale:      push,
		maskerLocale:    masker,
		mgmtSettings:    mgmt,
		questionMgr:     questionTarget,
		agentRunner:     agentTarget,
	})

	if chatSmallModel.runtime != smallRuntime || chatSmallModel.calls != 1 {
		t.Fatalf("expected small model chat wiring, got %#v", chatSmallModel)
	}
	if auxiliarySmallModel.runtime != smallRuntime || auxiliarySmallModel.calls != 1 {
		t.Fatalf("expected small model auxiliary wiring, got %#v", auxiliarySmallModel)
	}
	if imageSmallModel.runtime != smallRuntime || imageSmallModel.calls != 1 || imageSmallModel.enabledFunc == nil {
		t.Fatalf("expected small model image wiring, got %#v", imageSmallModel)
	}
	if analyzeSmallModel.runtime != smallRuntime || analyzeSmallModel.runtimeCalls != 1 || analyzeSmallModel.statsRecorder == nil {
		t.Fatalf("expected small model analyze wiring, got %#v", analyzeSmallModel)
	}
	if reflectTarget.llmCaller == nil || reflectTarget.llmCalls != 1 || reflectTarget.proposalGate == nil || reflectTarget.proposalGateCalls != 1 {
		t.Fatalf("expected reflection activation wiring, got %#v", reflectTarget)
	}
	if compactorChat.calls != 1 || compactorChat.integration != nil || compactorChat.sessionMaxTokens != 2048 {
		t.Fatalf("expected compactor memory fallback wiring, got %#v", compactorChat)
	}
	if provider.handler != settings || provider.calls != 1 || chatSettings.handler != settings || chatSettings.calls != 1 {
		t.Fatalf("expected settings handlers to be applied, provider=%#v chat=%#v", provider, chatSettings)
	}
	if research.calls != 1 || research.enabled != settings.GetDeepResearchV2Enabled() {
		t.Fatalf("expected research settings wiring, got %#v", research)
	}
	if exec.calls != 1 || exec.autoConfirmFunc == nil {
		t.Fatalf("expected exec auto-confirm wiring, got %#v", exec)
	}
	if prompt.calls() != 4 || push.calls != 1 || masker.calls != 1 || mgmt.calls != 1 {
		t.Fatalf("expected prompt/push/masker/mgmt wiring, prompt=%#v push=%#v masker=%#v mgmt=%#v", prompt, push, masker, mgmt)
	}
	if questionTarget.timeoutFunc == nil || questionTarget.timeoutCalls != 1 || agentTarget.askTimeoutFunc == nil || agentTarget.askTimeoutCalls != 1 {
		t.Fatalf("expected ask-policy wiring, question=%#v agent=%#v", questionTarget, agentTarget)
	}
}

func TestBindRuntimePromptGuard_TogglesTargets(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		chat := &stubRuntimePromptGuardTarget{}
		security := &stubRuntimePromptGuardTarget{}

		bindRuntimePromptGuard(chat, security, false, zap.NewNop())

		if chat.detector == nil || security.detector == nil {
			t.Fatalf("expected prompt guard detector to be installed, chat=%#v security=%#v", chat, security)
		}
		if chat.detector != security.detector {
			t.Fatalf("expected shared detector across chat/security, chat=%p security=%p", chat.detector, security.detector)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		chat := &stubRuntimePromptGuardTarget{detector: promptguard.NewDetector(promptguard.DefaultDetectorConfig())}
		security := &stubRuntimePromptGuardTarget{detector: promptguard.NewDetector(promptguard.DefaultDetectorConfig())}

		bindRuntimePromptGuard(chat, security, true, zap.NewNop())

		if chat.detector != nil || security.detector != nil {
			t.Fatalf("expected prompt guard detector to be cleared, chat=%#v security=%#v", chat, security)
		}
	})
}

func TestBindRuntimeToolSelection_WiresDefaultsAndOptionalSkillSelector(t *testing.T) {
	handler := &serverpkg.ChatHandler{}
	cfg := &config.Config{}
	cfg.ToolCalling.SmartSelectionMaxTools = 7
	cfg.ToolCalling.SkillRerankEnabled = true
	cfg.ToolCalling.SkillRerankONNXEnabled = true
	cfg.ToolCalling.SkillRerankONNXAutoDownload = true
	cfg.ToolCalling.SkillRerankModel = "repo/model"

	reranker := bindRuntimeToolSelection(handler, cfg, t.TempDir(), "/tmp/workspace", config.NewFlagEvaluator(&config.GrayscaleConfig{}))

	if reranker == nil {
		t.Fatal("expected smart skill selection reranker to be created")
	}
	if handler.GetToolSelector() == nil || handler.GetToolSelector().MaxTools != 7 {
		t.Fatalf("expected tool selector max-tools wiring, got %#v", handler.GetToolSelector())
	}
	if handler.GetToolPolicyResolver() == nil {
		t.Fatal("expected tool policy resolver wiring")
	}
	if handler.GetToolTraceStore() == nil {
		t.Fatal("expected tool trace store wiring")
	}
	if handler.GetToolRouter() == nil {
		t.Fatal("expected tool router wiring")
	}
	if handler.GetSkillSelector() == nil {
		t.Fatal("expected skill selector wiring")
	}
}

func TestBindRuntimeToolSelection_WiresSelectorUnderDefaultCutoverConfig(t *testing.T) {
	handler := &serverpkg.ChatHandler{}
	cfg := &config.Config{}

	reranker := bindRuntimeToolSelection(handler, cfg, t.TempDir(), "/tmp/workspace", nil)

	if reranker == nil {
		t.Fatal("expected cutover skill selector reranker to be created by default")
	}
	if handler.GetToolSelector() == nil || handler.GetToolPolicyResolver() == nil || handler.GetToolTraceStore() == nil || handler.GetToolRouter() == nil {
		t.Fatalf("expected baseline tool-selection wiring, got selector=%#v policy=%#v trace=%#v router=%#v", handler.GetToolSelector(), handler.GetToolPolicyResolver(), handler.GetToolTraceStore(), handler.GetToolRouter())
	}
	if handler.GetSkillSelector() == nil {
		t.Fatalf("expected cutover skill selector to be wired")
	}
}

func TestRuntimeBindingsGo_DelegatesFoundationHelpers(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("runtime_proxy_bridge_binding.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_bridge_binding.go: %v", err)
	}
	source := string(content)
	sharedTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_shared.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_shared.go: %v", err)
	}
	sharedTypeSource := string(sharedTypeContent)
	smallModelTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_smallmodel.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_smallmodel.go: %v", err)
	}
	smallModelTypeSource := string(smallModelTypeContent)
	proxyBridgeTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_proxy_bridge.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_proxy_bridge.go: %v", err)
	}
	proxyBridgeTypeSource := string(proxyBridgeTypeContent)
	toolingCapabilityTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_tooling_capabilities.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_tooling_capabilities.go: %v", err)
	}
	toolingCapabilityTypeSource := string(toolingCapabilityTypeContent)
	toolingSchedulerTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_tooling_scheduler.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_tooling_scheduler.go: %v", err)
	}
	toolingSchedulerTypeSource := string(toolingSchedulerTypeContent)
	toolingExecTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_tooling_exec.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_tooling_exec.go: %v", err)
	}
	toolingExecTypeSource := string(toolingExecTypeContent)
	memoryTypeContent, err := os.ReadFile(filepath.Join("runtime_binding_types_memory.go"))
	if err != nil {
		t.Fatalf("read runtime_binding_types_memory.go: %v", err)
	}
	memoryTypeSource := string(memoryTypeContent)
	taskContent, err := os.ReadFile(filepath.Join("runtime_harness_task_bindings.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_task_bindings.go: %v", err)
	}
	taskSource := string(taskContent)
	chatAskContent, err := os.ReadFile(filepath.Join("runtime_chat_ask_bindings.go"))
	if err != nil {
		t.Fatalf("read runtime_chat_ask_bindings.go: %v", err)
	}
	chatAskSource := string(chatAskContent)
	reflectionContent, err := os.ReadFile(filepath.Join("runtime_reflection_activation.go"))
	if err != nil {
		t.Fatalf("read runtime_reflection_activation.go: %v", err)
	}
	reflectionSource := string(reflectionContent)
	researchSpecContent, err := os.ReadFile(filepath.Join("runtime_harness_research_spec.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_research_spec.go: %v", err)
	}
	researchSpecSource := string(researchSpecContent)
	researchLaneContent, err := os.ReadFile(filepath.Join("runtime_harness_research_lane.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_research_lane.go: %v", err)
	}
	researchLaneSource := string(researchLaneContent)
	agentFeatureLaneContent, err := os.ReadFile(filepath.Join("runtime_harness_agent_feature_lane.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_agent_feature_lane.go: %v", err)
	}
	agentFeatureLaneSource := string(agentFeatureLaneContent)
	approvalLaneContent, err := os.ReadFile(filepath.Join("runtime_harness_approval_lane.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_approval_lane.go: %v", err)
	}
	approvalLaneSource := string(approvalLaneContent)
	workflowGatewayLaneContent, err := os.ReadFile(filepath.Join("runtime_harness_workflow_gateway_lane.go"))
	if err != nil {
		t.Fatalf("read runtime_harness_workflow_gateway_lane.go: %v", err)
	}
	workflowGatewayLaneSource := string(workflowGatewayLaneContent)

	if lines := strings.Count(source, "\n") + 1; lines > 120 {
		t.Fatalf("expected runtime_proxy_bridge_binding.go to stay below 120 lines after lane extraction, got %d", lines)
	}

	forbidden := []string{
		"func newRuntimeConvertSupport(",
		"func bindRuntimeMgmtTool(",
		"func newRuntimeExecSkillExecutor(",
		"func bindRuntimeExecTool(",
		"func bindHarnessRuntimeToolObserver(",
		"func bindRuntimePromptGuard(",
		"func bindRuntimeToolSelection(",
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeImageTools(",
		"func newRuntimeAgentSessionsService(",
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeExecResolverTarget interface {",
		"func newChatAskRuntimeBinding(",
		"func bindHarnessRuntimeAskSupport(",
		"func newRuntimeAskPolicyBinding(",
		"func bindAgentRuntimeSupport(",
		"func newReflectionRuntimeBinding(",
		"func bindHarnessRuntimeReflection(",
		"func activateHarnessRuntime(",
		"type researchHarnessRunInput struct {",
		"func newResearchHarnessRunSpec(",
		"func registerHarnessRuntimeTaskRoutes(",
		"func newRuntimeToolGateway(",
	}
	for _, token := range forbidden {
		if strings.Contains(source, token) {
			t.Fatalf("expected runtime_proxy_bridge_binding.go to delegate helper %q", token)
		}
	}

	requiredRuntime := []string{
		"func bindRuntimeProxyBridge(",
	}
	for _, token := range requiredRuntime {
		if !strings.Contains(source, token) {
			t.Fatalf("expected runtime_proxy_bridge_binding.go to keep remaining runtime lane token %q", token)
		}
	}

	requiredSharedTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeChatFlagEvaluator interface {",
		"type runtimePromptGuardTarget interface {",
	}
	for _, token := range requiredSharedTypes {
		if !strings.Contains(sharedTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_shared.go to contain token %q", token)
		}
	}

	requiredSmallModelTypes := []string{
		"type runtimeSmallModelSettingsSource interface {",
		"type runtimeSkillRerankerTarget interface {",
		"type runtimeCompactorMemoryChatTarget interface {",
	}
	for _, token := range requiredSmallModelTypes {
		if !strings.Contains(smallModelTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_smallmodel.go to contain token %q", token)
		}
	}

	requiredProxyBridgeTypes := []string{
		"type runtimeProxyBridgeChatTarget interface {",
		"type runtimeProxyBridgeAnalyzeTarget interface {",
	}
	for _, token := range requiredProxyBridgeTypes {
		if !strings.Contains(proxyBridgeTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_proxy_bridge.go to contain token %q", token)
		}
	}

	requiredToolingCapabilityTypes := []string{
		"type runtimeAnalyzeTarget interface {",
		"type runtimeBrowserSkillTarget interface {",
	}
	for _, token := range requiredToolingCapabilityTypes {
		if !strings.Contains(toolingCapabilityTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_tooling_capabilities.go to contain token %q", token)
		}
	}

	requiredToolingSchedulerTypes := []string{
		"type runtimeSchedulerSkillTarget interface {",
		"type runtimeReminderSkillTarget interface {",
		"type runtimeVoiceServiceSource interface {",
	}
	for _, token := range requiredToolingSchedulerTypes {
		if !strings.Contains(toolingSchedulerTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_tooling_scheduler.go to contain token %q", token)
		}
	}

	requiredToolingExecTypes := []string{
		"type runtimeExecToolTarget interface {",
		"type runtimeMgmtToolTarget interface {",
		"type runtimeExecSkillSelectionSource interface {",
	}
	for _, token := range requiredToolingExecTypes {
		if !strings.Contains(toolingExecTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_tooling_exec.go to contain token %q", token)
		}
	}

	requiredMemoryTypes := []string{
		"type chatAskRuntimeTarget interface {",
		"type questionRuntimePolicyTarget interface {",
		"type reflectionRuntimeBinding struct {",
	}
	for _, token := range requiredMemoryTypes {
		if !strings.Contains(memoryTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_memory.go to contain token %q", token)
		}
	}

	forbiddenSharedTypes := []string{
		"type runtimeSmallModelSettingsSource interface {",
		"type runtimeProxyBridgeChatTarget interface {",
		"type chatAskRuntimeTarget interface {",
	}
	for _, token := range forbiddenSharedTypes {
		if strings.Contains(sharedTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_shared.go to delegate token %q", token)
		}
	}

	forbiddenSmallModelTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeProxyBridgeChatTarget interface {",
		"type chatAskRuntimeTarget interface {",
	}
	for _, token := range forbiddenSmallModelTypes {
		if strings.Contains(smallModelTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_smallmodel.go to delegate token %q", token)
		}
	}

	forbiddenProxyBridgeTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeAnalyzeTarget interface {",
		"type chatAskRuntimeTarget interface {",
	}
	for _, token := range forbiddenProxyBridgeTypes {
		if strings.Contains(proxyBridgeTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_proxy_bridge.go to delegate token %q", token)
		}
	}

	forbiddenToolingCapabilityTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeSmallModelSettingsSource interface {",
		"type chatAskRuntimeTarget interface {",
		"type runtimeExecToolTarget interface {",
	}
	for _, token := range forbiddenToolingCapabilityTypes {
		if strings.Contains(toolingCapabilityTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_tooling_capabilities.go to delegate token %q", token)
		}
	}

	forbiddenToolingSchedulerTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeSmallModelSettingsSource interface {",
		"type chatAskRuntimeTarget interface {",
		"type runtimeBrowserSkillTarget interface {",
	}
	for _, token := range forbiddenToolingSchedulerTypes {
		if strings.Contains(toolingSchedulerTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_tooling_scheduler.go to delegate token %q", token)
		}
	}

	forbiddenToolingExecTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeSmallModelSettingsSource interface {",
		"type chatAskRuntimeTarget interface {",
		"type runtimeBrowserSkillTarget interface {",
	}
	for _, token := range forbiddenToolingExecTypes {
		if strings.Contains(toolingExecTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_tooling_exec.go to delegate token %q", token)
		}
	}

	forbiddenMemoryTypes := []string{
		"type runtimeToolEventObserverTarget interface {",
		"type runtimeSmallModelSettingsSource interface {",
		"type runtimeProxyBridgeChatTarget interface {",
	}
	for _, token := range forbiddenMemoryTypes {
		if strings.Contains(memoryTypeSource, token) {
			t.Fatalf("expected runtime_binding_types_memory.go to delegate token %q", token)
		}
	}

	requiredChatAsk := []string{
		"func newChatAskRuntimeBinding(",
		"func bindHarnessRuntimeAskSupport(",
		"func newRuntimeAskPolicyBinding(",
	}
	for _, token := range requiredChatAsk {
		if !strings.Contains(chatAskSource, token) {
			t.Fatalf("expected runtime_chat_ask_bindings.go to contain token %q", token)
		}
	}
	forbiddenChatAsk := []string{
		"func bindHarnessRuntimeReflection(",
		"func activateHarnessRuntime(",
		"func newResearchHarnessRunSpec(",
		"func registerHarnessRuntimeTaskRoutes(",
	}
	for _, token := range forbiddenChatAsk {
		if strings.Contains(chatAskSource, token) {
			t.Fatalf("expected runtime_chat_ask_bindings.go to delegate token %q", token)
		}
	}

	requiredReflection := []string{
		"func bindAgentRuntimeSupport(",
		"func newReflectionRuntimeBinding(",
		"func bindHarnessRuntimeReflection(",
		"func activateHarnessRuntime(",
	}
	for _, token := range requiredReflection {
		if !strings.Contains(reflectionSource, token) {
			t.Fatalf("expected runtime_reflection_activation.go to contain token %q", token)
		}
	}
	forbiddenReflection := []string{
		"func newChatAskRuntimeBinding(",
		"func newResearchHarnessRunSpec(",
		"func registerHarnessRuntimeTaskRoutes(",
	}
	for _, token := range forbiddenReflection {
		if strings.Contains(reflectionSource, token) {
			t.Fatalf("expected runtime_reflection_activation.go to delegate token %q", token)
		}
	}

	requiredResearchSpec := []string{
		"type researchHarnessRunInput struct {",
		"func newResearchHarnessRunSpec(",
	}
	for _, token := range requiredResearchSpec {
		if !strings.Contains(researchSpecSource, token) {
			t.Fatalf("expected runtime_harness_research_spec.go to contain token %q", token)
		}
	}
	forbiddenResearchSpec := []string{
		"func bindHarnessRuntimeReflection(",
		"func newChatAskRuntimeBinding(",
		"func registerHarnessRuntimeTaskRoutes(",
	}
	for _, token := range forbiddenResearchSpec {
		if strings.Contains(researchSpecSource, token) {
			t.Fatalf("expected runtime_harness_research_spec.go to delegate token %q", token)
		}
	}

	requiredTask := []string{
		"func registerHarnessRuntimeTaskRoutes(",
	}
	for _, token := range requiredTask {
		if !strings.Contains(taskSource, token) {
			t.Fatalf("expected runtime_harness_task_bindings.go to contain token %q", token)
		}
	}
	forbiddenTask := []string{
		"func registerDeepResearchRuntimeRoutes(",
		"func registerHarnessRuntimeAgentFeature(",
		"func bindHarnessRuntimeApproval(",
		"func newRuntimeToolGateway(",
	}
	for _, token := range forbiddenTask {
		if strings.Contains(taskSource, token) {
			t.Fatalf("expected runtime_harness_task_bindings.go to delegate token %q", token)
		}
	}

	requiredResearchLane := []string{
		"func registerDeepResearchRuntimeRoutes(",
		"func newHarnessRuntimeAutoHarnessTurnHook(",
	}
	for _, token := range requiredResearchLane {
		if !strings.Contains(researchLaneSource, token) {
			t.Fatalf("expected runtime_harness_research_lane.go to contain token %q", token)
		}
	}
	forbiddenResearchLane := []string{
		"func registerHarnessRuntimeAgentFeature(",
		"func bindHarnessRuntimeApproval(",
		"func newRuntimeToolGateway(",
	}
	for _, token := range forbiddenResearchLane {
		if strings.Contains(researchLaneSource, token) {
			t.Fatalf("expected runtime_harness_research_lane.go to delegate token %q", token)
		}
	}

	requiredAgentFeatureLane := []string{
		"func registerHarnessRuntimeAgentFeature(",
	}
	for _, token := range requiredAgentFeatureLane {
		if !strings.Contains(agentFeatureLaneSource, token) {
			t.Fatalf("expected runtime_harness_agent_feature_lane.go to contain token %q", token)
		}
	}
	forbiddenAgentFeatureLane := []string{
		"func registerDeepResearchRuntimeRoutes(",
		"func bindHarnessRuntimeApproval(",
		"func newRuntimeToolGateway(",
	}
	for _, token := range forbiddenAgentFeatureLane {
		if strings.Contains(agentFeatureLaneSource, token) {
			t.Fatalf("expected runtime_harness_agent_feature_lane.go to delegate token %q", token)
		}
	}

	requiredApprovalLane := []string{
		"func bindHarnessRuntimeApproval(",
	}
	for _, token := range requiredApprovalLane {
		if !strings.Contains(approvalLaneSource, token) {
			t.Fatalf("expected runtime_harness_approval_lane.go to contain token %q", token)
		}
	}
	forbiddenApprovalLane := []string{
		"func registerDeepResearchRuntimeRoutes(",
		"func registerHarnessRuntimeAgentFeature(",
		"func newRuntimeToolGateway(",
	}
	for _, token := range forbiddenApprovalLane {
		if strings.Contains(approvalLaneSource, token) {
			t.Fatalf("expected runtime_harness_approval_lane.go to delegate token %q", token)
		}
	}

	requiredWorkflowGatewayLane := []string{
		"func newRuntimeToolGateway(",
	}
	for _, token := range requiredWorkflowGatewayLane {
		if !strings.Contains(workflowGatewayLaneSource, token) {
			t.Fatalf("expected runtime_harness_workflow_gateway_lane.go to contain token %q", token)
		}
	}
	forbiddenWorkflowGatewayLane := []string{
		"func registerDeepResearchRuntimeRoutes(",
		"func registerHarnessRuntimeAgentFeature(",
		"func bindHarnessRuntimeApproval(",
	}
	for _, token := range forbiddenWorkflowGatewayLane {
		if strings.Contains(workflowGatewayLaneSource, token) {
			t.Fatalf("expected runtime_harness_workflow_gateway_lane.go to delegate token %q", token)
		}
	}
}

func TestRuntimeDeferredWiringGo_DelegatesPolicyModelAndCompactorHelpers(t *testing.T) {
	wiringContent, err := os.ReadFile(filepath.Join("runtime_deferred_wiring.go"))
	if err != nil {
		t.Fatalf("read runtime_deferred_wiring.go: %v", err)
	}
	wiringSource := string(wiringContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_deferred_types.go"))
	if err != nil {
		t.Fatalf("read runtime_deferred_types.go: %v", err)
	}
	typeSource := string(typeContent)

	policyContent, err := os.ReadFile(filepath.Join("runtime_deferred_policy.go"))
	if err != nil {
		t.Fatalf("read runtime_deferred_policy.go: %v", err)
	}
	policySource := string(policyContent)

	smallModelContent, err := os.ReadFile(filepath.Join("runtime_deferred_smallmodel.go"))
	if err != nil {
		t.Fatalf("read runtime_deferred_smallmodel.go: %v", err)
	}
	smallModelSource := string(smallModelContent)

	compactorContent, err := os.ReadFile(filepath.Join("runtime_deferred_compactor.go"))
	if err != nil {
		t.Fatalf("read runtime_deferred_compactor.go: %v", err)
	}
	compactorSource := string(compactorContent)

	if lines := strings.Count(wiringSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_deferred_wiring.go to stay below 60 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_deferred_types.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(policySource, "\n") + 1; lines > 90 {
		t.Fatalf("expected runtime_deferred_policy.go to stay below 90 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(smallModelSource, "\n") + 1; lines > 110 {
		t.Fatalf("expected runtime_deferred_smallmodel.go to stay below 110 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(compactorSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_deferred_compactor.go to stay below 70 lines after extraction, got %d", lines)
	}

	requiredWiring := []string{
		"func bindDeferredRuntimeWiring(",
		"func bindDeferredRuntimeApproval(",
		"bindRuntimeSmallModel(",
		"activateHarnessRuntime(",
		"bindRuntimeSelectorDryRun(",
		"bindRuntimeCompactorMemory(",
		"bindRuntimeSettingsTargets(",
		"bindRuntimeSkillReranker(",
		"bindRuntimeAskPolicy(",
		"bindHarnessRuntimeApproval(",
	}
	for _, token := range requiredWiring {
		if !strings.Contains(wiringSource, token) {
			t.Fatalf("expected runtime_deferred_wiring.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type runtimeDeferredWiring struct {",
		"type runtimeDeferredApprovalWiring struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_deferred_types.go to contain token %q", token)
		}
	}

	requiredPolicy := []string{
		"func bindRuntimeAskPolicy(",
		"func bindRuntimeLayeredMemory(",
		"func bindRuntimeSettingsTargets(",
	}
	for _, token := range requiredPolicy {
		if !strings.Contains(policySource, token) {
			t.Fatalf("expected runtime_deferred_policy.go to contain token %q", token)
		}
	}

	requiredSmallModel := []string{
		"func bindRuntimeSmallModel(",
		"func bindRuntimeSkillReranker(",
	}
	for _, token := range requiredSmallModel {
		if !strings.Contains(smallModelSource, token) {
			t.Fatalf("expected runtime_deferred_smallmodel.go to contain token %q", token)
		}
	}

	requiredCompactor := []string{
		"func bindRuntimeCompactorMemory(",
		"func bindRuntimeSelectorDryRun(",
	}
	for _, token := range requiredCompactor {
		if !strings.Contains(compactorSource, token) {
			t.Fatalf("expected runtime_deferred_compactor.go to contain token %q", token)
		}
	}

	forbiddenWiring := []string{
		"type runtimeDeferredWiring struct {",
		"func bindRuntimeLayeredMemory(",
		"func bindRuntimeSmallModel(",
		"func bindRuntimeSkillReranker(",
		"func bindRuntimeCompactorMemory(",
		"func bindRuntimeSelectorDryRun(",
	}
	for _, token := range forbiddenWiring {
		if strings.Contains(wiringSource, token) {
			t.Fatalf("expected runtime_deferred_wiring.go to delegate token %q", token)
		}
	}

	forbiddenTypes := []string{
		"func bindRuntimeAskPolicy(",
		"func bindRuntimeSmallModel(",
		"func bindRuntimeCompactorMemory(",
	}
	for _, token := range forbiddenTypes {
		if strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_deferred_types.go to delegate token %q", token)
		}
	}

	forbiddenPolicy := []string{
		"func bindRuntimeSmallModel(",
		"func bindRuntimeSkillReranker(",
		"func bindRuntimeCompactorMemory(",
		"func bindDeferredRuntimeWiring(",
	}
	for _, token := range forbiddenPolicy {
		if strings.Contains(policySource, token) {
			t.Fatalf("expected runtime_deferred_policy.go to delegate token %q", token)
		}
	}

	forbiddenSmallModel := []string{
		"func bindRuntimeAskPolicy(",
		"func bindRuntimeLayeredMemory(",
		"func bindRuntimeCompactorMemory(",
		"func bindDeferredRuntimeWiring(",
	}
	for _, token := range forbiddenSmallModel {
		if strings.Contains(smallModelSource, token) {
			t.Fatalf("expected runtime_deferred_smallmodel.go to delegate token %q", token)
		}
	}

	forbiddenCompactor := []string{
		"func bindRuntimeAskPolicy(",
		"func bindRuntimeLayeredMemory(",
		"func bindRuntimeSmallModel(",
		"func bindDeferredRuntimeWiring(",
	}
	for _, token := range forbiddenCompactor {
		if strings.Contains(compactorSource, token) {
			t.Fatalf("expected runtime_deferred_compactor.go to delegate token %q", token)
		}
	}
}

func TestRuntimeToolBindingsGo_DelegatesBrowserKnowledgeSchedulerNotificationTTSAndImageLanes(t *testing.T) {
	browserContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_browser.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_browser.go: %v", err)
	}
	browserSource := string(browserContent)

	browserTargetsContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_browser_targets.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_browser_targets.go: %v", err)
	}
	browserTargetsSource := string(browserTargetsContent)

	knowledgeContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_knowledge.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_knowledge.go: %v", err)
	}
	knowledgeSource := string(knowledgeContent)

	schedulerContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_scheduler.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_scheduler.go: %v", err)
	}
	schedulerSource := string(schedulerContent)

	notificationContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_notification.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_notification.go: %v", err)
	}
	notificationSource := string(notificationContent)

	ttsContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_tts.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_tts.go: %v", err)
	}
	ttsSource := string(ttsContent)

	imageContent, err := os.ReadFile(filepath.Join("runtime_tool_bindings_image.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_bindings_image.go: %v", err)
	}
	imageSource := string(imageContent)

	if lines := strings.Count(browserSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_tool_bindings_browser.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(browserTargetsSource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_tool_bindings_browser_targets.go to stay below 45 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(knowledgeSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_tool_bindings_knowledge.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(schedulerSource, "\n") + 1; lines > 65 {
		t.Fatalf("expected runtime_tool_bindings_scheduler.go to stay below 65 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(notificationSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_tool_bindings_notification.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(ttsSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_tool_bindings_tts.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(imageSource, "\n") + 1; lines > 50 {
		t.Fatalf("expected runtime_tool_bindings_image.go to stay below 50 lines after extraction, got %d", lines)
	}

	requiredBrowser := []string{
		"func newRuntimeBrowserSkillService(",
		"func newRuntimeUIReviewerTool(",
	}
	for _, token := range requiredBrowser {
		if !strings.Contains(browserSource, token) {
			t.Fatalf("expected runtime_tool_bindings_browser.go to contain token %q", token)
		}
	}

	requiredBrowserTargets := []string{
		"func bindRuntimeBrowserTargets(",
	}
	for _, token := range requiredBrowserTargets {
		if !strings.Contains(browserTargetsSource, token) {
			t.Fatalf("expected runtime_tool_bindings_browser_targets.go to contain token %q", token)
		}
	}

	requiredKnowledge := []string{
		"func bindRuntimeKnowledgeSkills(",
		"func bindRuntimeAnalyzeTool(",
	}
	for _, token := range requiredKnowledge {
		if !strings.Contains(knowledgeSource, token) {
			t.Fatalf("expected runtime_tool_bindings_knowledge.go to contain token %q", token)
		}
	}

	requiredScheduler := []string{
		"func bindRuntimeSchedulerServices(",
	}
	for _, token := range requiredScheduler {
		if !strings.Contains(schedulerSource, token) {
			t.Fatalf("expected runtime_tool_bindings_scheduler.go to contain token %q", token)
		}
	}

	requiredNotification := []string{
		"func bindRuntimeReminderServices(",
	}
	for _, token := range requiredNotification {
		if !strings.Contains(notificationSource, token) {
			t.Fatalf("expected runtime_tool_bindings_notification.go to contain token %q", token)
		}
	}

	requiredTTS := []string{
		"func bindRuntimeTTSTool(",
	}
	for _, token := range requiredTTS {
		if !strings.Contains(ttsSource, token) {
			t.Fatalf("expected runtime_tool_bindings_tts.go to contain token %q", token)
		}
	}

	requiredImage := []string{
		"func bindRuntimeImageTools(",
	}
	for _, token := range requiredImage {
		if !strings.Contains(imageSource, token) {
			t.Fatalf("expected runtime_tool_bindings_image.go to contain token %q", token)
		}
	}

	forbiddenBrowser := []string{
		"func bindRuntimeKnowledgeSkills(",
		"func bindRuntimeSchedulerServices(",
		"func bindRuntimeImageTools(",
		"func bindRuntimeBrowserTargets(",
	}
	for _, token := range forbiddenBrowser {
		if strings.Contains(browserSource, token) {
			t.Fatalf("expected runtime_tool_bindings_browser.go to delegate token %q", token)
		}
	}

	forbiddenBrowserTargets := []string{
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeKnowledgeSkills(",
		"func bindRuntimeImageTools(",
	}
	for _, token := range forbiddenBrowserTargets {
		if strings.Contains(browserTargetsSource, token) {
			t.Fatalf("expected runtime_tool_bindings_browser_targets.go to delegate token %q", token)
		}
	}

	forbiddenKnowledge := []string{
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeSchedulerServices(",
		"func bindRuntimeImageTools(",
	}
	for _, token := range forbiddenKnowledge {
		if strings.Contains(knowledgeSource, token) {
			t.Fatalf("expected runtime_tool_bindings_knowledge.go to delegate token %q", token)
		}
	}

	forbiddenScheduler := []string{
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeReminderServices(",
		"func bindRuntimeImageTools(",
	}
	for _, token := range forbiddenScheduler {
		if strings.Contains(schedulerSource, token) {
			t.Fatalf("expected runtime_tool_bindings_scheduler.go to delegate token %q", token)
		}
	}

	forbiddenNotification := []string{
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeKnowledgeSkills(",
		"func bindRuntimeImageTools(",
		"func bindRuntimeTTSTool(",
	}
	for _, token := range forbiddenNotification {
		if strings.Contains(notificationSource, token) {
			t.Fatalf("expected runtime_tool_bindings_notification.go to delegate token %q", token)
		}
	}

	forbiddenTTS := []string{
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeKnowledgeSkills(",
		"func bindRuntimeReminderServices(",
		"func bindRuntimeImageTools(",
	}
	for _, token := range forbiddenTTS {
		if strings.Contains(ttsSource, token) {
			t.Fatalf("expected runtime_tool_bindings_tts.go to delegate token %q", token)
		}
	}

	forbiddenImage := []string{
		"func newRuntimeBrowserSkillService(",
		"func bindRuntimeKnowledgeSkills(",
		"func bindRuntimeSchedulerServices(",
	}
	for _, token := range forbiddenImage {
		if strings.Contains(imageSource, token) {
			t.Fatalf("expected runtime_tool_bindings_image.go to delegate token %q", token)
		}
	}
}

func TestBindRuntimeProxyBridge_WiresUnifiedRuntimeTargets(t *testing.T) {
	runtimeProvider := newRuntimeLLMProviderRef()
	auxiliary := newAuxiliaryLLMCaller()
	chat := &stubRuntimeProxyBridgeChatTarget{}
	voiceTarget := &stubRuntimeProxyBridgeVoiceTarget{}
	uiTool := &stubRuntimeProxyBridgeVLMTarget{}
	media := &stubRuntimeProxyBridgeMediaTarget{}
	pdf := &stubRuntimeProxyBridgePDFTarget{}
	image := &stubRuntimeProxyBridgeImageTarget{}
	uiSkill := &stubRuntimeProxyBridgeSkillTarget{}
	analyze := &stubRuntimeProxyBridgeAnalyzeTarget{}

	bindRuntimeProxyBridge(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		nil,
		nil,
		runtimeProvider,
		auxiliary,
		chat,
		voiceTarget,
		uiTool,
		media,
		pdf,
		image,
		uiSkill,
		analyze,
	)

	if runtimeProvider.currentProvider() == nil {
		t.Fatal("expected runtime provider to be initialized from proxy bridge")
	}
	if auxiliary.fallback != runtimeProvider {
		t.Fatalf("expected auxiliary llm fallback to point at runtime provider, got %#v", auxiliary.fallback)
	}
	if chat.runtimeProvider != runtimeProvider || chat.proxyBridge == nil || chat.imModel != "" {
		t.Fatalf("expected chat runtime/provider bridge wiring, got %#v", chat)
	}
	if voiceTarget.chatFunc == nil || voiceTarget.calls != 1 {
		t.Fatalf("expected voice chat function wiring, got %#v", voiceTarget)
	}
	if uiTool.bridge == nil || uiTool.calls != 1 {
		t.Fatalf("expected ui reviewer vlm bridge wiring, got %#v", uiTool)
	}
	if media.bridge == nil || media.calls != 1 {
		t.Fatalf("expected media fallback vision wiring, got %#v", media)
	}
	if pdf.vision == nil || pdf.calls != 1 {
		t.Fatalf("expected pdf vision fallback wiring, got %#v", pdf)
	}
	if image.bridge == nil || image.calls != 1 {
		t.Fatalf("expected image vision bridge wiring, got %#v", image)
	}
	if uiSkill.bridge == nil || uiSkill.calls != 1 {
		t.Fatalf("expected ui reviewer skill bridge wiring, got %#v", uiSkill)
	}
	if analyze.bridge == nil || analyze.calls != 1 {
		t.Fatalf("expected analyze llm bridge wiring, got %#v", analyze)
	}
	if chat.proxyBridge != uiSkill.bridge {
		t.Fatalf("expected chat/ui skill to share proxy bridge instance, chat=%p skill=%p", chat.proxyBridge, uiSkill.bridge)
	}
}

func TestBindRuntimeProxyBridge_SkipsWithoutProxyHandler(t *testing.T) {
	runtimeProvider := newRuntimeLLMProviderRef()
	auxiliary := newAuxiliaryLLMCaller()
	chat := &stubRuntimeProxyBridgeChatTarget{}

	bindRuntimeProxyBridge(nil, nil, nil, runtimeProvider, auxiliary, chat, nil, nil, nil, nil, nil, nil, nil)

	if runtimeProvider.currentProvider() != nil {
		t.Fatalf("expected runtime provider to remain unset without proxy handler, got %#v", runtimeProvider.currentProvider())
	}
	if auxiliary.fallback != nil {
		t.Fatalf("expected auxiliary llm fallback to remain nil, got %#v", auxiliary.fallback)
	}
	if chat.runtimeProvider != nil || chat.proxyBridge != nil || chat.imModel != "" || chat.runtimeCalls != 0 || chat.bridgeCalls != 0 || chat.imModelCalls != 0 {
		t.Fatalf("expected chat runtime wiring to be skipped without proxy handler, got %#v", chat)
	}
}

func TestNewRuntimeUIReviewerTool_RequiresBrowserRuntime(t *testing.T) {
	if tool := newRuntimeUIReviewerTool(nil, nil, "/tmp/media"); tool != nil {
		t.Fatalf("expected nil ui reviewer tool without browser runtime, got %#v", tool)
	}

	if tool := newRuntimeUIReviewerTool(nil, func() *browser.RodService { return nil }, "/tmp/media"); tool == nil {
		t.Fatal("expected ui reviewer tool when lazy browser runtime is available")
	}
}

func TestNewRuntimeBrowserSkillService_PrefersAcquireThenLazy(t *testing.T) {
	acquireSvc := newRuntimeBrowserSkillService(func() (*browser.RodService, func(), error) {
		return nil, func() {}, nil
	}, func() *browser.RodService { return nil })
	if acquireSvc == nil {
		t.Fatal("expected acquire-backed browser skill service")
	}

	lazySvc := newRuntimeBrowserSkillService(nil, func() *browser.RodService { return nil })
	if lazySvc == nil {
		t.Fatal("expected lazy-backed browser skill service")
	}

	if svc := newRuntimeBrowserSkillService(nil, nil); svc != nil {
		t.Fatalf("expected nil browser skill service without browser runtime, got %#v", svc)
	}
}

func TestBindRuntimeBrowserTargets_WiresToolsAndSkills(t *testing.T) {
	mediaDir := "/tmp/media"
	backend := &stubRuntimeBrowserBackend{}
	var backendIface tools.BrowserBackend = backend
	lightpanda := &browser.LightpandaService{}
	browserTool := &stubRuntimeBrowserMediaDirTarget{}
	webTool := &stubRuntimeBrowserAccessTarget{}
	webFetchTool := &stubRuntimeBrowserAccessTarget{}
	browserSkill := &stubRuntimeBrowserSkillTarget{}
	uiSkill := &stubRuntimeUIReviewerSkillTarget{}
	skillService := &stubBuiltinBrowserService{}
	var skillServiceIface builtin.BrowserServiceInterface = skillService

	bindRuntimeBrowserTargets(
		mediaDir,
		backend,
		lightpanda,
		browserTool,
		browserSkill,
		uiSkill,
		skillService,
		webTool,
		webFetchTool,
	)

	// browserTool is nil after migration - browser is now skill-based only
	_ = browserTool
	_ = mediaDir // media dir is set via browserSkill instead
	if webTool.browser != backendIface || webTool.lightpanda != lightpanda || webTool.browserCalls != 1 || webTool.lightpandaCalls != 1 {
		t.Fatalf("expected web tool browser/shim wiring, got %#v", webTool)
	}
	if webFetchTool.browser != backendIface || webFetchTool.lightpanda != lightpanda || webFetchTool.browserCalls != 1 || webFetchTool.lightpandaCalls != 1 {
		t.Fatalf("expected web fetch tool browser/shim wiring, got %#v", webFetchTool)
	}
	if browserSkill.browserService != skillServiceIface || browserSkill.mediaDir != mediaDir || browserSkill.serviceCalls != 1 || browserSkill.mediaCalls != 1 {
		t.Fatalf("expected browser skill wiring, got %#v", browserSkill)
	}
	if uiSkill.browserService != skillServiceIface || uiSkill.calls != 1 {
		t.Fatalf("expected ui reviewer skill browser-service wiring, got %#v", uiSkill)
	}
}

func TestBindRuntimeBrowserTargets_SkipsOptionalInputs(t *testing.T) {
	browserTool := &stubRuntimeBrowserMediaDirTarget{}
	webTool := &stubRuntimeBrowserAccessTarget{}
	browserSkill := &stubRuntimeBrowserSkillTarget{}
	uiSkill := &stubRuntimeUIReviewerSkillTarget{}

	bindRuntimeBrowserTargets("/tmp/media", nil, nil, browserTool, browserSkill, uiSkill, nil, webTool)

	// browserTool is nil after migration - no media dir wiring needed
	_ = browserTool
	if webTool.browser != nil || webTool.lightpanda != nil || webTool.browserCalls != 0 || webTool.lightpandaCalls != 0 {
		t.Fatalf("expected browser/lightpanda wiring to be skipped, got %#v", webTool)
	}
	if browserSkill.browserService != nil || browserSkill.serviceCalls != 0 || browserSkill.mediaCalls != 0 {
		t.Fatalf("expected browser skill wiring to be skipped without service, got %#v", browserSkill)
	}
	if uiSkill.browserService != nil || uiSkill.calls != 0 {
		t.Fatalf("expected ui reviewer skill wiring to be skipped without service, got %#v", uiSkill)
	}
}

func TestBindRuntimeKnowledgeSkills_WiresAnalyzeAndKnowledgeExecutors(t *testing.T) {
	registry := tools.NewRegistry()
	analyzeTool := &stubRuntimeAnalyzeTarget{}
	analyzeSkill := &stubRuntimeAnalyzeSkillTarget{}
	webSearchSkill := &stubRuntimeWebSearchSkillTarget{}
	deepResearchSkill := &stubRuntimeDeepResearchSkillTarget{}
	backend := &stubRuntimeBrowserBackend{}
	var backendIface tools.BrowserBackend = backend

	bindRuntimeKnowledgeSkills(
		registry,
		analyzeTool,
		backend,
		nil,
		analyzeSkill,
		nil,
		webSearchSkill,
		tools.WebSearchConfig{Provider: "duckduckgo"},
		deepResearchSkill,
		deepresearch.NewService(nil, nil),
	)

	if analyzeTool.browser != backendIface || analyzeTool.browserCalls != 1 {
		t.Fatalf("expected analyze tool browser wiring, got %#v", analyzeTool)
	}
	if analyzeTool.executor == nil || analyzeTool.executorCalls != 1 {
		t.Fatalf("expected analyze tool executor wiring, got %#v", analyzeTool)
	}
	if analyzeSkill.executor != analyzeTool || analyzeSkill.calls != 1 {
		t.Fatalf("expected analyze skill executor wiring, got %#v", analyzeSkill)
	}
	if webSearchSkill.searcher == nil || webSearchSkill.calls != 1 {
		t.Fatalf("expected web_search skill searcher wiring, got %#v", webSearchSkill)
	}
	if deepResearchSkill.executor == nil || deepResearchSkill.calls != 1 {
		t.Fatalf("expected deep_research skill executor wiring, got %#v", deepResearchSkill)
	}
}

func TestBindRuntimeKnowledgeSkills_UsesLazyBrowserFallbackAndSkipsMissingTargets(t *testing.T) {
	analyzeTool := &stubRuntimeAnalyzeTarget{}

	bindRuntimeKnowledgeSkills(
		nil,
		analyzeTool,
		nil,
		func() *browser.RodService { return nil },
		nil,
		nil,
		nil,
		tools.WebSearchConfig{},
		nil,
		nil,
	)

	if analyzeTool.browser == nil || analyzeTool.browserCalls != 1 {
		t.Fatalf("expected analyze tool to receive lazy browser backend fallback, got %#v", analyzeTool)
	}
	if analyzeTool.executor != nil || analyzeTool.executorCalls != 0 {
		t.Fatalf("expected analyze tool executor wiring to be skipped without registry, got %#v", analyzeTool)
	}
}

func TestBindRuntimeAnalyzeTool_RegistersAndWiresKnowledgeTargets(t *testing.T) {
	registry := tools.NewRegistry()
	analyzeSkill := &stubRuntimeAnalyzeSkillTarget{}
	webSearchSkill := &stubRuntimeWebSearchSkillTarget{}
	deepResearchSkill := &stubRuntimeDeepResearchSkillTarget{}
	backend := &stubRuntimeBrowserBackend{}

	analyzeTool := bindRuntimeAnalyzeTool(
		registry,
		t.TempDir(),
		backend,
		nil,
		analyzeSkill,
		nil,
		webSearchSkill,
		tools.WebSearchConfig{Provider: "duckduckgo"},
		deepResearchSkill,
		deepresearch.NewService(nil, nil),
	)

	if analyzeTool == nil || registry.Get("analyze") != analyzeTool {
		t.Fatalf("expected analyze tool to be registered and returned, got tool=%#v registered=%#v", analyzeTool, registry.Get("analyze"))
	}
	if analyzeSkill.executor != analyzeTool || analyzeSkill.calls != 1 {
		t.Fatalf("expected analyze skill to be wired to registered tool, got %#v", analyzeSkill)
	}
	if webSearchSkill.searcher == nil || webSearchSkill.calls != 1 {
		t.Fatalf("expected web_search skill searcher wiring, got %#v", webSearchSkill)
	}
	if deepResearchSkill.executor == nil || deepResearchSkill.calls != 1 {
		t.Fatalf("expected deep_research skill executor wiring, got %#v", deepResearchSkill)
	}
}

func TestRegisterChatResearchRuntime_WiresAdvisorDeepAsyncService(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	registry := tools.NewRegistry()
	bindRuntimeAnalyzeTool(
		registry,
		t.TempDir(),
		nil,
		nil,
		nil,
		nil,
		nil,
		tools.WebSearchConfig{},
		nil,
		nil,
	)

	service := deepresearch.NewService(nil, nil)
	binding := newChatResearchRuntimeBinding(bundle, service, &serverpkg.ChatHandler{}, sse.NewBroker(), t.TempDir())
	binding.register(harnessRuntimeController(bundle), registry)

	advisorTool := tools.GetAdvisorTool(registry)
	if advisorTool == nil {
		t.Fatal("expected advisor tool to be registered")
	}

	result, err := advisorTool.Execute(context.Background(), map[string]interface{}{
		"question": "Should we replace Python with Go?",
		"depth":    "deep",
		"output":   "decision_pack",
		"wait":     false,
	})
	if err != nil {
		t.Fatalf("advisor deep async execute failed: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want pending payload map", result)
	}
	if payload["accepted"] != true {
		t.Fatalf("accepted = %#v, want true", payload["accepted"])
	}
	if payload["mode"] != "advisor" {
		t.Fatalf("mode = %#v, want advisor", payload["mode"])
	}
	if strings.TrimSpace(fmt.Sprint(payload["job_id"])) == "" {
		t.Fatalf("job_id = %#v, want non-empty", payload["job_id"])
	}
}

func TestBindRuntimeSchedulerServices_RegistersToolsWiresTargetsAndKeepsResolversLazy(t *testing.T) {
	registry := tools.NewRegistry()
	scheduler := &stubRuntimeSchedulerSkillTarget{}
	calendar := &stubRuntimeSchedulerCalendarTarget{}
	cronHandler := &stubRuntimeCronHandlerTarget{
		service: cron.NewService(cron.DefaultConfig(), zap.NewNop()),
	}
	workflowResolutions := 0

	bindRuntimeSchedulerServices(
		registry,
		func() *workflow.WorkflowService {
			workflowResolutions++
			return nil
		},
		nil,
		nil,
		"",
		cronHandler,
		scheduler,
		calendar,
		nil,
		sse.NewBroker(),
		deepresearch.NewService(nil, nil),
		zap.NewNop(),
	)

	if workflowResolutions != 0 {
		t.Fatalf("expected workflow resolver to remain lazy, got %d eager resolutions", workflowResolutions)
	}
	if cronHandler.getCalls != 0 {
		t.Fatalf("expected cron getter to remain lazy, got %d eager resolutions", cronHandler.getCalls)
	}
	if registry.Get("nodes") == nil || registry.Get("cron") == nil {
		t.Fatalf("expected nodes/cron tools to be registered, tools=%v", registry.List())
	}
	if scheduler.service == nil || scheduler.calls != 1 {
		t.Fatalf("expected scheduler skill wiring, got %#v", scheduler)
	}
	if calendar.service == nil || calendar.calls != 1 {
		t.Fatalf("expected calendar cron wiring, got %#v", calendar)
	}
	if len(cronHandler.initHooks) != 1 {
		t.Fatalf("expected a single cron init hook, got %#v", cronHandler.initHooks)
	}

	cronHandler.initHooks[0](cronHandler.service)
	handlers := cronHandler.service.Handlers()
	for _, name := range []string{"command", researchAndNotifyHandlerName, "research-and-notify"} {
		if !containsRuntimeString(handlers, name) {
			t.Fatalf("expected cron init hook to register handler %q, handlers=%v", name, handlers)
		}
	}
}

func TestBindRuntimeSchedulerServices_SkipsEverythingWithoutCronHandler(t *testing.T) {
	registry := tools.NewRegistry()
	scheduler := &stubRuntimeSchedulerSkillTarget{}
	calendar := &stubRuntimeSchedulerCalendarTarget{}
	workflowResolutions := 0

	bindRuntimeSchedulerServices(
		registry,
		func() *workflow.WorkflowService {
			workflowResolutions++
			return nil
		},
		nil,
		nil,
		"",
		nil,
		scheduler,
		calendar,
		nil,
		sse.NewBroker(),
		deepresearch.NewService(nil, nil),
		zap.NewNop(),
	)

	if workflowResolutions != 0 {
		t.Fatalf("expected workflow resolver to stay unused without cron handler, got %d calls", workflowResolutions)
	}
	if registry.Get("nodes") == nil {
		t.Fatalf("expected workflow nodes tool to register without cron handler, tools=%v", registry.List())
	}
	if registry.Get("cron") != nil {
		t.Fatalf("expected cron tooling to stay unregistered without cron handler, tools=%v", registry.List())
	}
	if scheduler.calls != 0 || calendar.calls != 0 {
		t.Fatalf("expected scheduler/calendar wiring to be skipped without cron handler, scheduler=%#v calendar=%#v", scheduler, calendar)
	}
}

func TestHarnessWorkflowExecutionLauncher_SubmitsWorkflowRunAndReturnsExecution(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	repo, err := workflow.NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}
	service, err := workflow.NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer service.Close()

	if err := repo.CreateWorkflow(workflow.WithTenantContext(context.Background(), "tenant-1"), &workflow.Workflow{
		ID:       "wf-1",
		TenantID: "tenant-1",
		Name:     "Nightly Sync",
		Status:   workflow.WorkflowStatusActive,
		Nodes: []workflow.Node{
			{ID: "trigger-1", Type: workflow.NodeTypeTrigger, Name: "Trigger"},
		},
	}); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	launcher := newHarnessWorkflowExecutionLauncher(bundle.Controller, func() *workflow.WorkflowService { return service }, "/tmp/workspace")
	execution, err := launcher.LaunchExecution(context.Background(), workflow.ExecutionLaunchRequest{
		WorkflowID:     "wf-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
		TriggerData: map[string]interface{}{
			"source": "manual",
		},
	})
	if err != nil {
		t.Fatalf("LaunchExecution failed: %v", err)
	}
	if execution == nil || execution.WorkflowID != "wf-1" {
		t.Fatalf("unexpected execution: %#v", execution)
	}

	runs, err := bundle.Controller.List(context.Background(), harness.RunFilter{
		UserID: "user-1",
		Kind:   harness.RunKindWorkflow,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("List runs failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if got := runs[0].Metadata["workflow_id"]; got != "wf-1" {
		t.Fatalf("workflow_id metadata = %v, want wf-1", got)
	}
	if got := runs[0].Metadata["workflow_execution_id"]; got == "" {
		t.Fatalf("workflow_execution_id metadata missing: %#v", runs[0].Metadata)
	}
}

func TestHarnessWorkflowExecutionLauncher_ResumesWorkflowRunViaHarnessAction(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	repo, err := workflow.NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}
	service, err := workflow.NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer service.Close()

	ctx := workflow.WithTenantContext(context.Background(), "tenant-1")
	if err := repo.CreateWorkflow(ctx, &workflow.Workflow{
		ID:       "wf-resume",
		TenantID: "tenant-1",
		Name:     "Approval Workflow",
		Status:   workflow.WorkflowStatusActive,
		Nodes: []workflow.Node{
			{ID: "trigger-1", Type: workflow.NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "action-1",
				Type: workflow.NodeTypeAction,
				Name: "Checkpointed Action",
				Config: map[string]interface{}{
					"type":              string(workflow.ActionTypeSetVariable),
					"name":              "result",
					"value":             "ready",
					"checkpoint_kind":   string(workflow.ExecutionCheckpointPauseForApproval),
					"checkpoint_reason": "approval_needed",
				},
			},
		},
		Connections: []workflow.Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "action-1"},
		},
	}); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	launcher := newHarnessWorkflowExecutionLauncher(bundle.Controller, func() *workflow.WorkflowService { return service }, "/tmp/workspace")
	service.SetExecutionLauncher(launcher)

	execution, err := launcher.LaunchExecution(ctx, workflow.ExecutionLaunchRequest{
		WorkflowID:     "wf-resume",
		UserID:         "user-1",
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
	})
	if err != nil {
		t.Fatalf("LaunchExecution failed: %v", err)
	}
	if execution == nil {
		t.Fatal("expected execution from launcher")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		paused, getErr := service.GetExecution(ctx, execution.ID)
		if getErr == nil && paused != nil && paused.Status == workflow.ExecutionStatusPaused {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	resumed, err := service.ResumeExecution(ctx, execution.ID, workflow.ExecutionResumeInput{
		Decision: "approve",
		Payload: map[string]interface{}{
			"ticket": "A-1",
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution failed: %v", err)
	}
	if resumed == nil {
		t.Fatal("expected resumed execution")
	}

	run, err := bundle.Controller.FindRunByMetadata(ctx, harness.RunKindWorkflow, "workflow_execution_id", execution.ID)
	if err != nil {
		t.Fatalf("FindRunByMetadata failed: %v", err)
	}
	if run == nil {
		t.Fatal("expected workflow run for resumed execution")
	}
	if run.Status != harness.RunStatusExecuting && run.Status != harness.RunStatusCompleted {
		t.Fatalf("run status = %q, want executing or completed", run.Status)
	}
}

func TestBindRuntimeWorkflowExecution_WiresHandlerAndNodesToolToHarnessLauncher(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	repo, err := workflow.NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}
	service, err := workflow.NewService(nil, repo)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer service.Close()

	ctx := workflow.WithTenantContext(context.Background(), "tenant-1")
	if err := repo.CreateWorkflow(ctx, &workflow.Workflow{
		ID:       "wf-2",
		TenantID: "tenant-1",
		Name:     "Review Workflow",
		Status:   workflow.WorkflowStatusActive,
		Nodes: []workflow.Node{
			{ID: "trigger-1", Type: workflow.NodeTypeTrigger, Name: "Trigger"},
			{
				ID:   "delay-1",
				Type: workflow.NodeTypeDelay,
				Name: "Delay",
				Config: map[string]interface{}{
					"duration": 60,
				},
			},
		},
		Connections: []workflow.Connection{
			{ID: "conn-1", SourceNode: "trigger-1", TargetNode: "delay-1"},
		},
	}); err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	registry := tools.NewRegistry()
	handler := workflow.NewHandler(service)
	bindRuntimeWorkflowExecution(registry, func() *workflow.WorkflowService { return service }, handler, bundle, "/tmp/workspace")

	if registry.Get("nodes") == nil {
		t.Fatalf("expected nodes tool registration, tools=%v", registry.List())
	}
	if err := repo.SaveWebhook(ctx, "wf-2", "/wf-2-hook", "POST", "none", nil); err != nil {
		t.Fatalf("SaveWebhook failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-2/execute", strings.NewReader(`{"trigger_data":{"conversation_id":"conv-http"}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/workflows/:id/execute")
	c.SetParamNames("id")
	c.SetParamValues("wf-2")
	c.Set("tenant_id", "tenant-1")
	c.Set("user_id", "user-http")
	if err := handler.ExecuteWorkflow(c); err != nil {
		t.Fatalf("handler ExecuteWorkflow failed: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("handler status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	toolCtx := tools.WithUserID(context.Background(), "user-tool")
	toolCtx = tools.WithSessionID(toolCtx, "conv-tool")
	toolCtx = workflow.WithTenantContext(toolCtx, "tenant-1")
	result, err := registry.Get("nodes").Execute(toolCtx, map[string]interface{}{
		"action":       "run",
		"id":           "wf-2",
		"trigger_data": map[string]interface{}{"source": "tool"},
	})
	if err != nil {
		t.Fatalf("nodes tool run failed: %v", err)
	}
	payload := result.(map[string]interface{})
	execution := payload["execution"].(*workflow.Execution)
	if execution.WorkflowID != "wf-2" {
		t.Fatalf("unexpected nodes execution: %#v", execution)
	}

	webhookExecution, err := service.HandleWebhook(context.Background(), "/wf-2-hook", http.MethodPost, map[string]string{"X-Test": "1"}, []byte("hook"))
	if err != nil {
		t.Fatalf("HandleWebhook failed: %v", err)
	}
	if webhookExecution.WorkflowID != "wf-2" || webhookExecution.TriggerType != workflow.TriggerTypeWebhook {
		t.Fatalf("unexpected webhook execution: %#v", webhookExecution)
	}
	if err := service.CancelExecution(workflow.WithTenantContext(context.Background(), "tenant-1"), webhookExecution.ID); err != nil {
		t.Fatalf("CancelExecution failed: %v", err)
	}
	cancelledRun, err := bundle.Controller.FindRunByMetadata(context.Background(), harness.RunKindWorkflow, "workflow_execution_id", webhookExecution.ID)
	if err != nil {
		t.Fatalf("FindRunByMetadata failed: %v", err)
	}
	if cancelledRun == nil || cancelledRun.Status != harness.RunStatusCancelled {
		t.Fatalf("expected cancelled workflow run, got %#v", cancelledRun)
	}

	runs, err := bundle.Controller.List(context.Background(), harness.RunFilter{
		Kind:  harness.RunKindWorkflow,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("List workflow runs failed: %v", err)
	}
	if len(runs) < 3 {
		t.Fatalf("expected handler + tool + webhook workflow runs, got %#v", runs)
	}
}

func TestBindRuntimeReminderServices_RegistersToolsAndWiresTargets(t *testing.T) {
	registry := tools.NewRegistry()
	calendar := &stubRuntimeReminderCalendarTarget{}
	reminder := &stubRuntimeReminderSkillTarget{}
	pushService := &push.Service{}

	bindRuntimeReminderServices(registry, pushService, calendar, reminder)

	if registry.Get("reminder") == nil || registry.Get("message") == nil {
		t.Fatalf("expected reminder/message tools to be registered, tools=%v", registry.List())
	}
	if calendar.service == nil || calendar.calls != 1 {
		t.Fatalf("expected calendar reminder wiring, got %#v", calendar)
	}
	if reminder.service == nil || reminder.calls != 1 {
		t.Fatalf("expected reminder skill wiring, got %#v", reminder)
	}
}

func TestBindRuntimeReminderServices_SkipsWithoutPushService(t *testing.T) {
	registry := tools.NewRegistry()
	calendar := &stubRuntimeReminderCalendarTarget{}
	reminder := &stubRuntimeReminderSkillTarget{}

	bindRuntimeReminderServices(registry, nil, calendar, reminder)

	if registry.Get("reminder") != nil || registry.Get("message") != nil {
		t.Fatalf("expected no reminder/message tool registration without push service, tools=%v", registry.List())
	}
	if calendar.service != nil || calendar.calls != 0 || reminder.service != nil || reminder.calls != 0 {
		t.Fatalf("expected target wiring to be skipped without push service, calendar=%#v reminder=%#v", calendar, reminder)
	}
}

func TestBindRuntimeTTSTool_RegistersWithVoiceOnlySource(t *testing.T) {
	registry := tools.NewRegistry()
	voiceSource := &stubRuntimeVoiceServiceSource{}

	bindRuntimeTTSTool(registry, nil, voiceSource)

	if registry.Get("tts") == nil {
		t.Fatalf("expected tts tool registration with voice-only runtime, tools=%v", registry.List())
	}
	if voiceSource.calls != 1 {
		t.Fatalf("expected voice runtime to be resolved once, got %#v", voiceSource)
	}
}

func TestBindRuntimeTTSTool_SkipsWithoutSources(t *testing.T) {
	registry := tools.NewRegistry()

	bindRuntimeTTSTool(registry, nil, nil)

	if registry.Get("tts") != nil {
		t.Fatalf("expected tts registration to be skipped without runtime sources, tools=%v", registry.List())
	}
}

func TestBindRuntimeImageTools_RegistersAndWiresImageDependencies(t *testing.T) {
	registry := tools.NewRegistry()
	image := &stubRuntimeImageToolTarget{}

	bindRuntimeImageTools(
		registry,
		image,
		&tools.UIReviewerTool{},
		[]string{t.TempDir()},
		&mediagen.Manager{},
		&mediagen.MediaStorage{},
		nil,
		&ocrruntime.TesseractService{},
		nil,
	)

	if registry.Get("image") == nil {
		t.Fatalf("expected image tool to be registered, tools=%v", registry.List())
	}
	if registry.Get("ppt") != nil {
		t.Fatalf("expected legacy ppt tool to stay unregistered, tools=%v", registry.List())
	}
	if image.ocr == nil || image.ocrCalls != 1 {
		t.Fatalf("expected image OCR wiring, got %#v", image)
	}
	if image.vision == nil || image.visionCalls != 1 {
		t.Fatalf("expected image provider-vision wiring, got %#v", image)
	}
	if image.ppt == nil || image.pptCalls != 1 {
		t.Fatalf("expected image ppt wiring, got %#v", image)
	}
}

func TestBindRuntimeImageTools_PreservesLegacyImageRegistrationWithoutMediaServices(t *testing.T) {
	registry := tools.NewRegistry()

	bindRuntimeImageTools(registry, nil, nil, nil, nil, nil, nil, nil, nil)

	if registry.Get("image") == nil {
		t.Fatalf("expected legacy image tool registration to remain available, tools=%v", registry.List())
	}
	if registry.Get("ppt") != nil {
		t.Fatalf("expected ppt tool registration to be skipped without media services, tools=%v", registry.List())
	}
}

func TestBindRuntimeSmallModel_WiresTargetsAndSettings(t *testing.T) {
	settings := &stubRuntimeSmallModelSettingsSource{
		enabled:      true,
		imageQA:      true,
		docExtract:   true,
		toggleResult: true,
	}
	runtime := &stubSmallModelRuntime{}
	chat := &stubRuntimeSmallModelChatTarget{}
	auxiliary := &stubRuntimeSmallModelAuxiliaryTarget{}
	image := &stubRuntimeImageSmallModelTarget{}
	analyze := &stubRuntimeAnalyzeSmallModelTarget{}
	stats := serverpkg.NewSmallModelStats()
	statsSource := &stubRuntimeSmallModelStatsSource{stats: stats}

	bindRuntimeSmallModel(settings, runtime, chat, auxiliary, image, analyze, statsSource)

	if chat.runtime != runtime || chat.calls != 1 {
		t.Fatalf("expected chat small-model runtime wiring, got %#v", chat)
	}
	if auxiliary.runtime != runtime || auxiliary.calls != 1 {
		t.Fatalf("expected auxiliary small-model wiring, got %#v", auxiliary)
	}
	if image.runtime != runtime || image.calls != 1 {
		t.Fatalf("expected image small-model runtime wiring, got %#v", image)
	}
	if image.enabledFunc == nil || !image.enabledFunc() {
		t.Fatalf("expected image small-model enablement wiring, got %#v", image)
	}
	if analyze.runtime != runtime || analyze.runtimeCalls != 1 {
		t.Fatalf("expected analyze small-model runtime wiring, got %#v", analyze)
	}
	if analyze.enabledFunc == nil || !analyze.enabledFunc() {
		t.Fatalf("expected analyze enabled func wiring, got %#v", analyze)
	}
	if analyze.docExtractFunc == nil || !analyze.docExtractFunc() {
		t.Fatalf("expected analyze doc-extract func wiring, got %#v", analyze)
	}
	if analyze.toggleFunc == nil {
		t.Fatalf("expected analyze doc-extract toggle wiring, got %#v", analyze)
	}
	if toggled, err := analyze.toggleFunc(false); err != nil || toggled != settings.toggleResult || len(settings.toggleCalls) != 1 || settings.toggleCalls[0] {
		t.Fatalf("expected analyze doc toggle to delegate to settings source, toggled=%v err=%v source=%#v", toggled, err, settings)
	}
	if analyze.statsRecorder != stats || analyze.statsCalls != 1 {
		t.Fatalf("expected analyze stats recorder wiring, got %#v", analyze)
	}
}

func TestBindRuntimeSmallModel_SkipsOptionalTargetsAndSettings(t *testing.T) {
	runtime := &stubSmallModelRuntime{}
	chat := &stubRuntimeSmallModelChatTarget{}
	analyze := &stubRuntimeAnalyzeSmallModelTarget{}

	bindRuntimeSmallModel(nil, runtime, chat, nil, nil, analyze, nil)

	if chat.runtime != runtime || chat.calls != 1 {
		t.Fatalf("expected chat runtime wiring without settings, got %#v", chat)
	}
	if analyze.runtime != runtime || analyze.runtimeCalls != 1 {
		t.Fatalf("expected analyze runtime wiring without settings, got %#v", analyze)
	}
	if analyze.enabledFunc != nil || analyze.docExtractFunc != nil || analyze.toggleFunc != nil || analyze.statsRecorder != nil {
		t.Fatalf("expected analyze settings-specific wiring to be skipped, got %#v", analyze)
	}
}

func TestBindRuntimeSkillReranker_WiresManagerAndDynamicSwitches(t *testing.T) {
	manager := &agentcore.SkillRerankerModelManager{}
	settings := &stubRuntimeSkillRerankerSettingsTarget{
		skillRerankEnabled:             true,
		skillRerankEnabledSet:          true,
		skillRerankONNXEnabled:         true,
		skillRerankONNXEnabledSet:      true,
		skillRerankONNXAutoDownload:    true,
		skillRerankONNXAutoDownloadSet: true,
	}
	reranker := &stubRuntimeSkillRerankerTarget{manager: manager}

	bindRuntimeSkillReranker(settings, reranker, runtimeSkillRerankerDefaults{
		dataDir:       t.TempDir(),
		modelRepo:     "repo/model",
		rerankEnabled: false,
		onnxEnabled:   false,
		autoDownload:  false,
	})

	if settings.manager != manager || settings.managerCalls != 1 {
		t.Fatalf("expected settings handler to receive reranker manager, got %#v", settings)
	}
	if reranker.switchCalls != 1 || reranker.onnxEnabledFunc == nil || reranker.autoDownloadFunc == nil {
		t.Fatalf("expected reranker switch funcs to be wired, got %#v", reranker)
	}
	if !reranker.onnxEnabledFunc() {
		t.Fatalf("expected explicit rerank+onnx settings to enable reranker, got %#v", settings)
	}
	if !reranker.autoDownloadFunc() {
		t.Fatalf("expected explicit auto-download setting to win over defaults, got %#v", settings)
	}

	settings.smallModelEnabled = true
	settings.smallModelRerankEnabled = false
	if reranker.onnxEnabledFunc() {
		t.Fatalf("expected small-model rerank scene toggle to disable reranker, got %#v", settings)
	}

	settings.smallModelRerankEnabled = true
	if !reranker.onnxEnabledFunc() {
		t.Fatalf("expected reranker to re-enable once small-model rerank toggle is restored, got %#v", settings)
	}
}

func TestBindRuntimeSkillReranker_FallsBackToStandaloneManager(t *testing.T) {
	tmp := t.TempDir()
	settings := &stubRuntimeSkillRerankerSettingsTarget{}

	bindRuntimeSkillReranker(settings, nil, runtimeSkillRerankerDefaults{
		dataDir:   tmp,
		modelRepo: "repo/model",
	})

	if settings.manager == nil || settings.managerCalls != 1 {
		t.Fatalf("expected fallback skill-reranker manager to be installed, got %#v", settings)
	}
	want := filepath.Join(tmp, "skill-reranker", "model.onnx")
	if got := settings.manager.ModelPath(); got != want {
		t.Fatalf("fallback manager model path = %q, want %q", got, want)
	}
}

func TestBindRuntimeCompactorMemory_WiresEnabledIntegration(t *testing.T) {
	chat := &stubRuntimeCompactorMemoryChatTarget{}
	memoryHandler := &serverpkg.MemoryHandler{}

	bindRuntimeCompactorMemory(chat, memoryHandler, nil, config.SessionCompactionConfig{
		Enabled:   true,
		Threshold: 0.8,
	}, 4096)

	if chat.calls != 1 || chat.integration == nil {
		t.Fatalf("expected compactor memory integration to be installed, got %#v", chat)
	}
	if chat.sessionMaxTokens != 4096 {
		t.Fatalf("expected session token budget to be forwarded, got %#v", chat)
	}
}

func TestBindRuntimeCompactorMemory_DisablesIntegrationWithoutDependencies(t *testing.T) {
	t.Run("missing memory handler", func(t *testing.T) {
		chat := &stubRuntimeCompactorMemoryChatTarget{}

		bindRuntimeCompactorMemory(chat, nil, nil, config.SessionCompactionConfig{
			Enabled: true,
		}, 2048)

		if chat.calls != 1 || chat.integration != nil || chat.sessionMaxTokens != 2048 {
			t.Fatalf("expected nil integration when memory handler is missing, got %#v", chat)
		}
	})

	t.Run("compaction disabled", func(t *testing.T) {
		chat := &stubRuntimeCompactorMemoryChatTarget{}

		bindRuntimeCompactorMemory(chat, &serverpkg.MemoryHandler{}, nil, config.SessionCompactionConfig{}, 1024)

		if chat.calls != 1 || chat.integration != nil || chat.sessionMaxTokens != 1024 {
			t.Fatalf("expected nil integration when compaction is disabled, got %#v", chat)
		}
	})
}

func TestNewReflectionRuntimeBinding_AppliesLLMCallerProposalGateAndJudge(t *testing.T) {
	llmCaller := &stubHarnessJudgeLLMCaller{}
	proposalGate := func() bool { return true }

	binding := newReflectionRuntimeBinding(llmCaller, proposalGate)
	if binding.llmCaller != llmCaller || binding.proposalGate == nil {
		t.Fatalf("expected reflection runtime binding to preserve dependencies, got %#v", binding)
	}

	target := &stubReflectionRuntimeTarget{}
	binding.apply(target)
	if target.llmCaller != llmCaller || target.llmCalls != 1 {
		t.Fatalf("expected llm caller wiring, got %#v", target)
	}
	if target.proposalGate == nil || target.proposalGateCalls != 1 || !target.proposalGate() {
		t.Fatalf("expected proposal gate wiring, got %#v", target)
	}

	judgeTarget := &stubHarnessJudgeEvaluatorTarget{}
	binding.applyJudge(judgeTarget)
	if judgeTarget.evaluator == nil || judgeTarget.calls != 1 {
		t.Fatalf("expected judge evaluator wiring, got %#v", judgeTarget)
	}
}

func TestActivateHarnessRuntime_AppliesReflectionWithoutBundle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	target := &stubReflectionRuntimeTarget{}
	proposalGate := func() bool { return true }
	activateHarnessRuntime(ctx, nil, target, &stubHarnessJudgeLLMCaller{}, proposalGate)

	if target.llmCaller == nil || target.llmCalls != 1 {
		t.Fatalf("expected llm caller wiring during activation, got %#v", target)
	}
	if target.proposalGate == nil || target.proposalGateCalls != 1 || !target.proposalGate() {
		t.Fatalf("expected proposal gate wiring during activation, got %#v", target)
	}
}

func TestActivateHarnessRuntime_AppliesReflectionWithHarnessBundle(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	target := &stubReflectionRuntimeTarget{}
	proposalGate := func() bool { return true }
	activateHarnessRuntime(ctx, bundle, target, &stubHarnessJudgeLLMCaller{}, proposalGate)

	if target.llmCaller == nil || target.llmCalls != 1 {
		t.Fatalf("expected llm caller wiring during activation, got %#v", target)
	}
	if target.proposalGate == nil || target.proposalGateCalls != 1 || !target.proposalGate() {
		t.Fatalf("expected proposal gate wiring during activation, got %#v", target)
	}
}

func TestBindHarnessRuntimeToDeepResearchHandler_OnlyBindsWithHarnessRuntime(t *testing.T) {
	target := &stubDeepResearchJobCreatorTarget{}
	service := deepresearch.NewService(nil, nil)

	bindHarnessRuntimeToDeepResearchHandler(nil, target, service, "/tmp/workspace")
	if target.calls != 0 || target.creator != nil {
		t.Fatalf("expected no creator binding without harness runtime, got %#v", target)
	}

	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	bindHarnessRuntimeToDeepResearchHandler(bundle, target, service, "/tmp/workspace")
	if target.calls != 1 || target.creator == nil {
		t.Fatalf("expected harness research creator binding, got %#v", target)
	}
}

func TestRegisterDeepResearchRuntimeRoutes_RegistersGroupsWithoutHarnessRuntime(t *testing.T) {
	target := &stubDeepResearchRuntimeRouteTarget{}
	groupA := &echo.Group{}
	groupB := &echo.Group{}

	ok := registerDeepResearchRuntimeRoutes(nil, target, deepresearch.NewService(nil, nil), "/tmp/workspace", []*echo.Group{groupA, nil, groupB})
	if !ok {
		t.Fatal("expected deep research route registration to report success")
	}
	if target.creatorCalls != 0 || target.creator != nil {
		t.Fatalf("expected no harness creator binding without runtime, got %#v", target)
	}
	if target.registerCalls != 2 || len(target.groups) != 2 || target.groups[0] != groupA || target.groups[1] != groupB {
		t.Fatalf("expected groups to be registered in order, got %#v", target)
	}
}

func TestRegisterDeepResearchRuntimeRoutes_BindsHarnessCreatorWhenPresent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	target := &stubDeepResearchRuntimeRouteTarget{}
	group := &echo.Group{}

	ok := registerDeepResearchRuntimeRoutes(bundle, target, deepresearch.NewService(nil, nil), "/tmp/workspace", []*echo.Group{group})
	if !ok {
		t.Fatal("expected deep research route registration to report success")
	}
	if target.creatorCalls != 1 || target.creator == nil {
		t.Fatalf("expected harness creator binding, got %#v", target)
	}
	if target.registerCalls != 1 || len(target.groups) != 1 || target.groups[0] != group {
		t.Fatalf("expected group to be registered, got %#v", target)
	}
}

func TestRegisterHarnessResearchCapabilityRoutes_RegistersGroupsWithoutRebindingCreator(t *testing.T) {
	target := &stubDeepResearchRuntimeRouteTarget{}
	groupA := &echo.Group{}
	groupB := &echo.Group{}

	ok := registerHarnessResearchCapabilityRoutes(target, []*echo.Group{groupA, nil, groupB})
	if !ok {
		t.Fatal("expected harness research capability route registration to report success")
	}
	if target.creatorCalls != 0 || target.creator != nil {
		t.Fatalf("expected no creator binding for capability alias registration, got %#v", target)
	}
	if target.registerCalls != 2 || len(target.groups) != 2 || target.groups[0] != groupA || target.groups[1] != groupB {
		t.Fatalf("expected harness research capability groups to be registered in order, got %#v", target)
	}
}

func TestNewHarnessRuntimeResearchToolAdapter_UsesBundleController(t *testing.T) {
	service := deepresearch.NewService(nil, nil)
	adapter := newHarnessRuntimeResearchToolAdapter(nil, service, "/tmp/workspace")
	if adapter == nil || adapter.service != service || adapter.manager != nil {
		t.Fatalf("expected fallback research adapter without manager, got %#v", adapter)
	}
	if adapter.jobStore != service {
		t.Fatalf("expected fallback adapter to preserve deep research service store, got %#v", adapter.jobStore)
	}

	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	adapter = newHarnessRuntimeResearchToolAdapter(bundle, service, "/tmp/workspace")
	if adapter == nil || adapter.manager != harnessRuntimeController(bundle) {
		t.Fatalf("expected harness-backed research adapter, got %#v", adapter)
	}
}

func TestNewHarnessRuntimeResearchToolAdapter_PreservesServiceFallbackLookupWithoutBundle(t *testing.T) {
	service := deepresearch.NewService(nil, stubDeepResearchSearcher{})
	job, err := service.CreateJob(context.Background(), deepresearch.CreateJobRequest{
		UserID:         "user-fallback",
		ConversationID: "conv-fallback",
		Query:          "recent discussion fallback",
		Mode:           deepresearch.ModeDeep,
	})
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	adapter := newHarnessRuntimeResearchToolAdapter(nil, service, "/tmp/workspace")
	got, err := adapter.GetJobForUser(job.ID, "user-fallback")
	if err != nil {
		t.Fatalf("GetJobForUser failed: %v", err)
	}
	if got == nil || got.ID != job.ID {
		t.Fatalf("job = %#v, want deep research service fallback job", got)
	}
}

func TestNewHarnessRuntimeAutoHarnessTurnHook_RequiresRuntimeAndHandler(t *testing.T) {
	if hook := newHarnessRuntimeAutoHarnessTurnHook(nil, &serverpkg.ChatHandler{}); hook != nil {
		t.Fatalf("expected nil auto-harness hook without runtime, got %#v", hook)
	}
	if hook := newHarnessRuntimeAutoHarnessTurnHook(&HarnessRuntimeBundle{}, nil); hook != nil {
		t.Fatalf("expected nil auto-harness hook without handler, got %#v", hook)
	}

	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	if hook := newHarnessRuntimeAutoHarnessTurnHook(bundle, &serverpkg.ChatHandler{}); hook == nil {
		t.Fatal("expected auto-harness hook with runtime and handler")
	}
}

func TestBindHarnessRuntimeAutoHarnessTurnHook_OnlyRegistersNonNilHooks(t *testing.T) {
	target := &stubAutoHarnessTurnHookTarget{}
	bindHarnessRuntimeAutoHarnessTurnHook(target, nil)
	if target.calls != 0 {
		t.Fatalf("expected no hook registration for nil hook, got %#v", target)
	}

	hook := &stubTurnHook{}
	bindHarnessRuntimeAutoHarnessTurnHook(target, hook)
	if target.calls != 1 || target.hook != hook {
		t.Fatalf("expected hook registration, got %#v", target)
	}
}

func TestBindHarnessRuntimeJudgeEvaluator_OnlyBindsWhenInputsPresent(t *testing.T) {
	target := &stubHarnessJudgeEvaluatorTarget{}
	bindHarnessRuntimeJudgeEvaluator(nil, &stubHarnessJudgeLLMCaller{})
	if target.calls != 0 {
		t.Fatalf("expected no evaluator binding without target, got %#v", target)
	}

	bindHarnessRuntimeJudgeEvaluator(target, nil)
	if target.calls != 0 {
		t.Fatalf("expected no evaluator binding without llm caller, got %#v", target)
	}

	bindHarnessRuntimeJudgeEvaluator(target, &stubHarnessJudgeLLMCaller{})
	if target.calls != 1 || target.evaluator == nil {
		t.Fatalf("expected evaluator binding, got %#v", target)
	}
}

func TestNewHarnessRuntimeExecApprovals_ReturnsNilWithoutBroker(t *testing.T) {
	if approvals := newHarnessRuntimeExecApprovals(nil, nil); approvals != nil {
		t.Fatalf("expected nil approvals without broker, got %#v", approvals)
	}
}

func TestNewHarnessRuntimeExecApprovals_WiresRuntimeObserverWhenPresent(t *testing.T) {
	observer := &stubRuntimeObserver{}
	bundle := &HarnessRuntimeBundle{RuntimeObserver: observer}
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	approvals := newHarnessRuntimeExecApprovals(bundle, broker)
	if approvals == nil {
		t.Fatal("expected approval manager when broker is present")
	}

	done := make(chan tools.ApprovalDecision, 1)
	go func() {
		decision, _ := approvals.RequestApproval(context.Background(), tools.ApprovalRequest{
			ID:        "approval-1",
			Type:      "command",
			Command:   "echo hello",
			UserID:    "user-1",
			SessionID: "observer-session",
		})
		done <- decision
	}()

	deadline := time.Now().Add(2 * time.Second)
	var pending *tools.ApprovalRequest
	for time.Now().Before(deadline) {
		pending = approvals.GetPendingBySession("observer-session")
		if pending != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending == nil {
		t.Fatal("expected pending approval request")
	}

	if !approvals.ResolveApprovalWithBinding(pending.ID, tools.ApprovalAllowOnce, pending.BindingHash) {
		t.Fatal("expected approval resolution to succeed")
	}

	select {
	case decision := <-done:
		if decision != tools.ApprovalAllowOnce {
			t.Fatalf("expected allow-once decision, got %q", decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval resolution")
	}
	if len(observer.approvalRequested) != 1 || len(observer.approvalResolved) != 1 {
		t.Fatalf("expected approval lifecycle events to be observed, got %#v", observer)
	}
}

func TestBindRuntimeExecTool_WiresAuditRegistrySkillExecutorAndSelector(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&stubRuntimeBindingTool{
		def: tools.ToolDefinition{
			Name:        "search",
			Description: "search",
			Parameters: map[string]interface{}{
				"type": "object",
			},
		},
	})
	skills := &stubRuntimeSkillRegistrySource{
		skills: map[string]skillpkg.Skill{
			"lookup": &stubRuntimeSkill{
				manifest: &skillpkg.Manifest{ID: "lookup", Name: "Lookup"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"lookup": true},
	}
	selectorSource := &stubRuntimeExecSkillSelectionSource{}
	target := &stubRuntimeExecToolTarget{}
	auditStore := &tools.ExecAuditStore{}

	bindRuntimeExecTool(target, registry, skills, "", selectorSource, auditStore)

	if target.auditStore != auditStore || target.auditCalls != 1 {
		t.Fatalf("expected audit store wiring, got %#v", target)
	}
	if target.registry != registry || target.registryCalls != 1 {
		t.Fatalf("expected registry wiring, got %#v", target)
	}
	if !containsRuntimeString(target.toolNames, "search") || target.toolNameCalls != 1 {
		t.Fatalf("expected tool names to be wired from registry, got %#v", target)
	}
	if len(target.pinnedSkills) == 0 || target.pinnedCalls != 1 {
		t.Fatalf("expected pinned skills wiring, got %#v", target)
	}
	if target.skillExec == nil || target.skillExecCalls != 1 {
		t.Fatalf("expected skill executor wiring, got %#v", target)
	}
	if target.skillSelect == nil || target.skillSelectCalls != 1 {
		t.Fatalf("expected skill selector wiring, got %#v", target)
	}

	out, err := target.skillExec(context.Background(), "lookup", map[string]any{"query": "latest"})
	if err != nil {
		t.Fatalf("expected wired skill executor to succeed, got %v", err)
	}
	if len(out) == 0 || skills.skills["lookup"].(*stubRuntimeSkill).calls != 1 {
		t.Fatalf("expected lookup skill to execute, out=%v skill=%#v", out, skills.skills["lookup"])
	}

	decision := target.skillSelect(context.Background(), "unknown query")
	if decision.SelectedSkill != "" || decision.Confidence != 0 || decision.NeedClarify || decision.Reason != "" || len(decision.Candidates) != 0 {
		t.Fatalf("expected empty selection result without runtime selector, got %#v", decision)
	}
	if selectorSource.selectorCalls != 1 {
		t.Fatalf("expected selector source to be consulted, got %#v", selectorSource)
	}
}

func TestNewRuntimeExecSkillSelector_RejectsRemovedSmartSkillSettingAfterCutover(t *testing.T) {
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"smart_skill_selection":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if err := settings.Patch(c); err != nil {
		t.Fatalf("Patch settings: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Patch settings status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	selectorSource := &stubRuntimeExecSkillSelectionSource{
		selector: nil,
		settings: settings,
	}

	selectFn := newRuntimeExecSkillSelector(selectorSource)
	if selectFn == nil {
		t.Fatal("expected runtime selector function")
	}

	decision := selectFn(context.Background(), "route this somewhere")
	if decision.SelectedSkill != "" || decision.Confidence != 0 || decision.NeedClarify || decision.Reason != "" || len(decision.Candidates) != 0 {
		t.Fatalf("expected empty selection result when no selector is wired, got %#v", decision)
	}
	if selectorSource.settingsCalls == 0 {
		t.Fatalf("expected settings handler to be consulted, got %#v", selectorSource)
	}
	if selectorSource.selectorCalls != 1 {
		t.Fatalf("expected selector lookup to continue after cutover, got %#v", selectorSource)
	}
}

func TestNewRuntimeExecSkillExecutor_ResolvesRegistryAliasCandidates(t *testing.T) {
	skills := &stubRuntimeSkillRegistrySource{
		skills: map[string]skillpkg.Skill{
			"web_search": &stubRuntimeSkill{
				manifest: &skillpkg.Manifest{ID: "web_search", Name: "Web Search"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"web_search": true},
	}

	executor := newRuntimeExecSkillExecutor(skills, "")
	if executor == nil {
		t.Fatal("expected executor")
	}

	out, err := executor(context.Background(), "web-search", map[string]any{"query": "latest"})
	if err != nil {
		t.Fatalf("expected alias execution to succeed, got %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("status=%q, want ok", out["status"])
	}
	if skills.skills["web_search"].(*stubRuntimeSkill).calls != 1 {
		t.Fatalf("expected canonical registry skill to execute once, got %#v", skills.skills["web_search"])
	}
}

func TestNewRuntimeExecSkillExecutor_UsesWorkspaceManifestToCanonicalizeExecution(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser override
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	skills := &stubRuntimeSkillRegistrySource{
		skills: map[string]skillpkg.Skill{
			"browser": &stubRuntimeSkill{
				manifest: &skillpkg.Manifest{ID: "browser", Name: "Browser"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"browser": true},
	}

	executor := newRuntimeExecSkillExecutor(skills, workspaceDir)
	if executor == nil {
		t.Fatal("expected executor")
	}

	out, err := executor(context.Background(), "team-browser", map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("expected manifest-backed canonical execution to succeed, got %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("status=%q, want ok", out["status"])
	}
	if skills.skills["browser"].(*stubRuntimeSkill).calls != 1 {
		t.Fatalf("expected canonical browser skill to execute once, got %#v", skills.skills["browser"])
	}
}

func TestNewRuntimeExecSkillExecutor_UsesExplicitBlueWorkdirForWorkspaceAliasExecution(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "team-browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser override
---
# Browser
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	skills := &stubRuntimeSkillRegistrySource{
		skills: map[string]skillpkg.Skill{
			"browser": &stubRuntimeSkill{
				manifest: &skillpkg.Manifest{ID: "browser", Name: "Browser"},
				result:   skillpkg.NewResult(map[string]any{"status": "ok"}),
			},
		},
		enabled: map[string]bool{"browser": true},
	}

	executor := newRuntimeExecSkillExecutor(skills, "")
	if executor == nil {
		t.Fatal("expected executor")
	}

	out, err := executor(context.Background(), "team-browser", map[string]any{
		"__blue_workdir": workspaceDir,
		"url":            "https://example.com",
	})
	if err != nil {
		t.Fatalf("expected manifest-backed canonical execution to succeed, got %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("status=%q, want ok", out["status"])
	}
	if skills.skills["browser"].(*stubRuntimeSkill).calls != 1 {
		t.Fatalf("expected canonical browser skill to execute once, got %#v", skills.skills["browser"])
	}
}

func TestNewRuntimeExecSkillExecutor_FallsBackToDeclarativeManifestSkill(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: demo
version: 1.0.0
description: Demo declarative skill
invocation: blue demo
examples:
  - blue demo
capability_tags:
  - demo
interaction_mode: stateless
card_support: none
---
# Demo
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	executor := newRuntimeExecSkillExecutor(nil, workspaceDir)
	if executor == nil {
		t.Fatal("expected executor")
	}

	out, err := executor(context.Background(), "demo", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("expected declarative manifest fallback to succeed, got %v", err)
	}
	if out["skill"] != "demo" {
		t.Fatalf("skill=%q, want demo", out["skill"])
	}
	if out["message"] == "" {
		t.Fatalf("expected declarative execution message, got %#v", out)
	}
}

func TestBindRuntimeMgmtTool_WiresMandatoryAndOptionalServices(t *testing.T) {
	target := &stubRuntimeMgmtToolTarget{}
	providerPool := &providerpool.Pool{}
	userService := &user.Service{}
	apiKeyService := &auth.APIKeyService{}

	bindRuntimeMgmtTool(target, providerPool, skillpkg.NewRegistry(), "", tools.NewRegistry(), "1.2.3", userService, apiKeyService)

	if target.providers == nil || target.providerCalls != 1 {
		t.Fatalf("expected provider service wiring, got %#v", target)
	}
	if target.skills == nil || target.skillCalls != 1 {
		t.Fatalf("expected skill service wiring, got %#v", target)
	}
	if target.tooling == nil || target.toolCalls != 1 {
		t.Fatalf("expected tool service wiring, got %#v", target)
	}
	if target.system == nil || target.systemCalls != 1 {
		t.Fatalf("expected system service wiring, got %#v", target)
	}
	if target.users == nil || target.userCalls != 1 {
		t.Fatalf("expected user service wiring, got %#v", target)
	}
	if target.apiKeys == nil || target.apiKeyCalls != 1 {
		t.Fatalf("expected api key service wiring, got %#v", target)
	}
}

func TestBindRuntimeMgmtUpgrade_WiresUpgradeAdapter(t *testing.T) {
	target := &stubRuntimeMgmtToolTarget{}

	bindRuntimeMgmtUpgrade(target, &update.Handler{}, &update.OTAChecker{}, "9.9.9")

	if target.upgrade == nil || target.upgradeCalls != 1 {
		t.Fatalf("expected upgrade adapter wiring, got %#v", target)
	}
}

func TestNewRuntimeConvertSupport_RegistersConvertToolAndHandler(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "convert.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	registry := tools.NewRegistry()
	service, handler, err := newRuntimeConvertSupport(db, tmp, nil, registry, nil, nil, []string{tmp})
	if err != nil {
		t.Fatalf("newRuntimeConvertSupport() error = %v", err)
	}
	defer service.Close()

	if service == nil || handler == nil {
		t.Fatalf("expected convert service and handler, got service=%#v handler=%#v", service, handler)
	}
	if registry.Get("convert") == nil {
		t.Fatalf("expected convert tool registration, tools=%v", registry.List())
	}
}

func TestNewRuntimeAgentSessionsService_ReturnsNilWithoutDB(t *testing.T) {
	service, err := newRuntimeAgentSessionsService(nil, zap.NewNop(), nil, tools.DefaultExecConfig(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error without db, got %v", err)
	}
	if service != nil {
		t.Fatalf("expected nil service without db, got %#v", service)
	}
}

func TestBindRuntimeAgentSessions_RegistersRoutesAndTools(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent-sessions.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := newRuntimeAgentSessionsService(
		db,
		zap.NewNop(),
		[]string{tmp},
		tools.DefaultExecConfig(),
		tools.NewApprovalManager(sse.NewBroker()),
		nil,
		nil,
		nil,
		func(string) (string, error) { return "", nil },
	)
	if err != nil {
		t.Fatalf("newRuntimeAgentSessionsService() error = %v", err)
	}
	if service == nil {
		t.Fatal("expected agent sessions service")
	}

	registry := tools.NewRegistry()
	e := echo.New()
	bindRuntimeAgentSessions(registry, nil, service, e.Group("/profile"), e.Group("/chat"))

	if !routeExists(e, http.MethodGet, "/profile/agent-sessions/profiles") {
		t.Fatalf("expected profile routes to be registered, routes=%v", e.Routes())
	}
	if !routeExists(e, http.MethodDelete, "/profile/agent-sessions/profiles/:id") {
		t.Fatalf("expected profile delete route to be registered, routes=%v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/chat/agent-sessions/sessions") {
		t.Fatalf("expected session routes to be registered, routes=%v", e.Routes())
	}
	if registry.Get("sessions") == nil {
		t.Fatalf("expected session tools to be registered, tools=%v", registry.List())
	}
}

func TestNewRuntimeToolGateway_ReturnsNilWithoutRegistryOrApprover(t *testing.T) {
	if gateway := newRuntimeToolGateway(nil, &stubToolApprover{}, nil, nil); gateway != nil {
		t.Fatalf("expected nil gateway without registry, got %#v", gateway)
	}
	if gateway := newRuntimeToolGateway(tools.NewRegistry(), nil, nil, nil); gateway != nil {
		t.Fatalf("expected nil gateway without approver, got %#v", gateway)
	}
}

func TestNewRuntimeToolGateway_WiresApproverObserverAndMetrics(t *testing.T) {
	registry := tools.NewRegistry()
	tool := &stubRuntimeBindingTool{
		def: tools.ToolDefinition{
			Name:        "search",
			Description: "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"query"},
				"additionalProperties": true,
			},
		},
		result: map[string]interface{}{"ok": true},
	}
	registry.Register(tool)

	approver := &stubToolApprover{
		decision: tools.ToolApprovalDecision{
			Allowed: true,
			Approval: tools.ToolApprovalEnvelope{
				Required:     true,
				Mode:         "ask",
				PolicySource: "approval_config",
				RiskLevel:    "medium",
			},
		},
	}
	observer := &stubRuntimeObserver{}
	metrics := &stubMetricsRecorder{}

	gateway := newRuntimeToolGateway(registry, approver, observer, metrics)
	if gateway == nil {
		t.Fatal("expected runtime tool gateway, got nil")
	}

	_, err := gateway.Execute(context.Background(), tools.ToolGatewayRequest{
		ToolCallID: "call-1",
		ToolName:   "search",
		Arguments:  `{"query":"latest"}`,
		RouteKind:  tools.ToolRouteKindWorkflow,
		UserID:     "user-1",
		SessionID:  "session-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if tool.calls != 1 {
		t.Fatalf("expected tool to execute once, got %d", tool.calls)
	}
	if approver.calls != 1 {
		t.Fatalf("expected approver to execute once, got %d", approver.calls)
	}
	if len(observer.toolRequested) != 1 || len(observer.toolFinished) != 1 {
		t.Fatalf("expected runtime observer notifications, got %#v", observer)
	}
	if !metrics.hasCall("tool_approval_requested_total") {
		t.Fatalf("expected approval metric to be recorded, got %#v", metrics.calls)
	}
}

func TestNewWorkflowRuntimeBinding_DisabledWithoutGateway(t *testing.T) {
	binding := newWorkflowRuntimeBinding(nil, &stubToolApprover{}, nil, nil)
	if binding.toolRuntime != nil || binding.metrics != nil {
		t.Fatalf("expected empty workflow binding without gateway inputs, got %#v", binding)
	}

	handler := &stubWorkflowRuntimeHookTarget{}
	binding.register(handler)
	if handler.hook != nil {
		t.Fatalf("expected no workflow hook without tool runtime, got %#v", handler)
	}
}

func TestNewWorkflowRuntimeBinding_AppliesRuntimeAndMetrics(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&stubRuntimeBindingTool{
		def: tools.ToolDefinition{
			Name:        "search",
			Description: "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"query"},
				"additionalProperties": true,
			},
		},
		result: map[string]interface{}{"ok": true},
	})
	approver := &stubToolApprover{
		decision: tools.ToolApprovalDecision{
			Allowed:  true,
			Approval: tools.ToolApprovalEnvelope{Mode: "auto"},
		},
	}
	metrics := &stubMetricsRecorder{}

	binding := newWorkflowRuntimeBinding(registry, approver, &stubRuntimeObserver{}, metrics)
	if binding.toolRuntime == nil || binding.metrics != metrics {
		t.Fatalf("expected workflow runtime binding, got %#v", binding)
	}

	service := &stubWorkflowRuntimeServiceTarget{}
	binding.apply(service)
	if service.gateway == nil || service.metrics != metrics {
		t.Fatalf("expected workflow runtime to be applied, got %#v", service)
	}

	handler := &stubWorkflowRuntimeHookTarget{}
	binding.register(handler)
	if handler.hook == nil {
		t.Fatal("expected workflow init hook to be registered")
	}
	handler.hook(nil)
}

func TestNewWorkflowHarnessDriverBinding_RegistersWorkflowDriverOnServiceInit(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	binding := newWorkflowHarnessDriverBinding(bundle)
	if binding.controller != bundle.Controller {
		t.Fatalf("expected harness controller binding, got %#v", binding)
	}

	target := &stubWorkflowRuntimeHookTarget{}
	binding.register(target)
	if target.hook == nil {
		t.Fatal("expected workflow driver init hook to be registered")
	}

	target.hook(new(workflow.WorkflowService))
	driver := bundle.Controller.GetRegisteredDriver(harness.RunKindWorkflow)
	if driver == nil {
		t.Fatal("expected workflow driver to be registered")
	}
}

func TestNewApprovalRuntimeBinding_DisabledWithoutHandler(t *testing.T) {
	binding := newApprovalRuntimeBinding(nil, nil, tools.NewApprovalManager(sse.NewBroker()), tools.NewRegistry(), nil, &stubMetricsRecorder{})
	if binding.handler != nil || binding.approver != nil || binding.execResolver != nil || binding.observer != nil {
		t.Fatalf("expected empty approval binding without handler, got %#v", binding)
	}
	if binding.workflow.toolRuntime != nil || binding.workflow.metrics != nil {
		t.Fatalf("expected empty workflow binding without handler, got %#v", binding.workflow)
	}
}

func TestNewApprovalRuntimeBinding_WiresTargetsWhenPresent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	handler := networkapi.NewApprovalHandler(nil)
	execApprovals := tools.NewApprovalManager(sse.NewBroker())
	metrics := &stubMetricsRecorder{}
	binding := newApprovalRuntimeBinding(bundle, handler, execApprovals, tools.NewRegistry(), nil, metrics)
	if binding.handler != handler || binding.approver != handler {
		t.Fatalf("expected handler-backed binding, got %#v", binding)
	}
	if binding.execResolver == nil {
		t.Fatalf("expected exec resolver to be wired, got %#v", binding)
	}
	if binding.observer != bundle.RuntimeObserver {
		t.Fatalf("expected shared runtime observer, got %#v", binding.observer)
	}
	if binding.workflow.toolRuntime == nil || binding.workflow.metrics != metrics {
		t.Fatalf("expected workflow runtime binding, got %#v", binding.workflow)
	}

	detail := &harnessDetailProvider{}
	binding.applyDetail(detail)
	if detail.approvalHandler != handler {
		t.Fatalf("expected detail provider to receive handler, got %#v", detail)
	}

	handlerTarget := &stubApprovalRuntimeHandlerTarget{}
	binding.applyHandler(handlerTarget)
	if handlerTarget.resolver == nil {
		t.Fatalf("expected exec resolver binding, got %#v", handlerTarget)
	}
	if handlerTarget.observer != bundle.RuntimeObserver {
		t.Fatalf("expected observer binding, got %#v", handlerTarget)
	}

	approverTarget := &stubToolApproverTarget{}
	binding.applyApprover(approverTarget)
	if approverTarget.approver != handler || approverTarget.calls != 1 {
		t.Fatalf("expected approver target binding, got %#v", approverTarget)
	}

	workflowTarget := &stubWorkflowRuntimeHookTarget{}
	binding.registerWorkflow(workflowTarget)
	if workflowTarget.hook == nil {
		t.Fatal("expected workflow runtime hook to be registered")
	}
}

func TestNewApprovalRuntimeBinding_OmitsExecResolverWhenExecApprovalsAbsent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	handler := networkapi.NewApprovalHandler(nil)
	binding := newApprovalRuntimeBinding(bundle, handler, nil, tools.NewRegistry(), nil, nil)

	handlerTarget := &stubApprovalRuntimeHandlerTarget{}
	binding.applyHandler(handlerTarget)
	if handlerTarget.resolverCalls != 0 {
		t.Fatalf("expected no exec resolver binding without exec approvals, got %#v", handlerTarget)
	}
	if handlerTarget.observer != bundle.RuntimeObserver || handlerTarget.observerCalls != 1 {
		t.Fatalf("expected observer binding to remain, got %#v", handlerTarget)
	}
}

func TestNewApprovalRuntimeBinding_WiresLLMRiskScorerWhenAuxiliaryPresent(t *testing.T) {
	e := echo.New()
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	handler := networkapi.NewApprovalHandler(broker)
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{
		Name:        "file_write",
		Description: "Writes content to a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
		},
	})
	auxiliary := &stubApprovalRiskLLMCaller{
		content: `{"score":92,"confidence":0.95,"risk_level":"high","recommended_mode":"ask","reason":"writes a file"}`,
	}
	newApprovalRuntimeBinding(nil, handler, nil, registry, auxiliary, nil)

	done := make(chan tools.ToolApprovalDecision, 1)
	errCh := make(chan error, 1)
	go func() {
		decision, err := handler.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
			ToolName: "file_write",
			Arguments: map[string]interface{}{
				"path":    "notes.txt",
				"content": "hello",
			},
			RouteKind: tools.ToolRouteKindAgent,
			SessionID: "conv-binding-risk",
			UserID:    "user-1",
		})
		if err != nil {
			errCh <- err
			return
		}
		done <- decision
	}()

	var requestID string
	for i := 0; i < 100; i++ {
		if pending := handler.GetPendingBySession("conv-binding-risk"); pending != nil {
			if id, ok := pending["id"].(string); ok {
				requestID = strings.TrimSpace(id)
			}
		}
		if requestID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("expected pending approval request")
	}

	req := httptest.NewRequest(http.MethodPost, "/approval/resolve", strings.NewReader(`{"request_id":"`+requestID+`","decision":"approve"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := handler.Resolve(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case err := <-errCh:
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	case decision := <-done:
		if !decision.Allowed {
			t.Fatal("expected approved decision")
		}
		if decision.Approval.PolicySource != "approval.llm_risk_score" {
			t.Fatalf("policy source = %q, want approval.llm_risk_score", decision.Approval.PolicySource)
		}
		if decision.Approval.Mode != "ask" {
			t.Fatalf("mode = %q, want ask", decision.Approval.Mode)
		}
	}
	if auxiliary.calls != 1 {
		t.Fatalf("auxiliary calls = %d, want 1", auxiliary.calls)
	}
}

func TestBindHarnessRuntimeApproval_WiresTargetsWhenPresent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	handler := networkapi.NewApprovalHandler(nil)
	execApprovals := tools.NewApprovalManager(sse.NewBroker())
	metrics := &stubMetricsRecorder{}
	detail := &harnessDetailProvider{}
	handlerTarget := &stubApprovalRuntimeHandlerTarget{}
	chatApproverTarget := &stubToolApproverTarget{}
	agentApproverTarget := &stubToolApproverTarget{}
	workflowTarget := &stubWorkflowRuntimeHookTarget{}

	bindHarnessRuntimeApproval(
		bundle,
		handler,
		execApprovals,
		tools.NewRegistry(),
		nil,
		metrics,
		detail,
		handlerTarget,
		workflowTarget,
		chatApproverTarget,
		agentApproverTarget,
	)

	if detail.approvalHandler != handler {
		t.Fatalf("expected detail target to receive approval handler, got %#v", detail)
	}
	if handlerTarget.resolver == nil || handlerTarget.resolverCalls != 1 {
		t.Fatalf("expected exec resolver binding, got %#v", handlerTarget)
	}
	if handlerTarget.observer != bundle.RuntimeObserver || handlerTarget.observerCalls != 1 {
		t.Fatalf("expected runtime observer binding, got %#v", handlerTarget)
	}
	if chatApproverTarget.approver != handler || chatApproverTarget.calls != 1 {
		t.Fatalf("expected chat approver binding, got %#v", chatApproverTarget)
	}
	if agentApproverTarget.approver != handler || agentApproverTarget.calls != 1 {
		t.Fatalf("expected agent approver binding, got %#v", agentApproverTarget)
	}
	if workflowTarget.hook == nil {
		t.Fatal("expected workflow hook to be registered")
	}
	workflowTarget.hook(new(workflow.WorkflowService))
	if bundle.Controller.GetRegisteredDriver(harness.RunKindWorkflow) == nil {
		t.Fatal("expected workflow driver to be registered alongside workflow runtime hook")
	}
}

func TestBindRuntimeActivationApproval_RegistersWorkflowDriverWithoutApprovalHandler(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	workflowTarget := &stubWorkflowRuntimeHookTarget{}
	bindRuntimeActivationApproval(runtimeActivationResult{}, runtimeActivationDeferredSupportOptions{
		harnessRuntime: bundle,
		workflowTarget: workflowTarget,
	})

	if workflowTarget.hook == nil {
		t.Fatal("expected workflow hook registration without approval handler")
	}
	workflowTarget.hook(new(workflow.WorkflowService))

	driver := bundle.Controller.GetRegisteredDriver(harness.RunKindWorkflow)
	if driver == nil {
		t.Fatal("expected workflow driver to be registered")
	}
}

func TestRegisterHarnessRuntimeTaskRoutes_RegistersDeepResearchWithoutHarness(t *testing.T) {
	e := echo.New()
	deepResearchTarget := &stubDeepResearchRuntimeRouteTarget{}

	registration := registerHarnessRuntimeTaskRoutes(
		nil,
		deepResearchTarget,
		deepresearch.NewService(nil, nil),
		"/tmp/workspace",
		nil,
		nil,
		[]*echo.Group{
			e.Group("/deep-research"),
			e.Group("/api/deep-research"),
		},
		[]*echo.Group{
			e.Group("/harness/research"),
			e.Group("/api/harness/research"),
		},
		[]*echo.Group{e.Group("/harness")},
		[]*echo.Group{e.Group("")},
	)
	if !registration.deepResearchRegistered || !registration.harnessResearchRegistered {
		t.Fatalf("expected deep research compatibility and harness research capability routes without harness, got %#v", registration)
	}
	if registration.harnessRoutesRegistered || registration.detailProvider != nil {
		t.Fatalf("expected harness routes to stay disabled without runtime bundle, got %#v", registration)
	}
	if deepResearchTarget.creatorCalls != 0 || deepResearchTarget.creator != nil {
		t.Fatalf("expected no harness creator binding without runtime bundle, got %#v", deepResearchTarget)
	}
	if deepResearchTarget.registerCalls != 4 {
		t.Fatalf("expected deep research compatibility + capability groups to be registered four times, got %#v", deepResearchTarget)
	}
	if !routeExists(e, "POST", "/harness/research/jobs") || !routeExists(e, "POST", "/api/harness/research/jobs") {
		t.Fatalf("expected harness research capability routes without runtime bundle, got %#v", e.Routes())
	}
	if routeExists(e, "POST", "/harness/runs") || routeExists(e, "GET", "/tasks") {
		t.Fatalf("expected no harness routes without runtime bundle, got %#v", e.Routes())
	}
}

func TestRegisterHarnessRuntimeTaskRoutes_RegistersDeepResearchAndHarnessWithBundle(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	execApprovals := tools.NewApprovalManager(sse.NewBroker())
	questionMgr := tools.NewQuestionManager(nil, nil, 0)
	e := echo.New()
	deepResearchTarget := &stubDeepResearchRuntimeRouteTarget{}

	registration := registerHarnessRuntimeTaskRoutes(
		bundle,
		deepResearchTarget,
		deepresearch.NewService(nil, nil),
		"/tmp/workspace",
		execApprovals,
		questionMgr,
		[]*echo.Group{
			e.Group("/deep-research"),
			e.Group("/api/deep-research"),
		},
		[]*echo.Group{
			e.Group("/harness/research"),
			e.Group("/api/harness/research"),
		},
		[]*echo.Group{e.Group("/harness")},
		[]*echo.Group{e.Group("")},
	)
	if !registration.deepResearchRegistered || !registration.harnessResearchRegistered || !registration.harnessRoutesRegistered || registration.detailProvider == nil {
		t.Fatalf("expected deep research compatibility, harness research capability, and harness routes to register, got %#v", registration)
	}
	if registration.detailProvider.execApprovals != execApprovals || registration.detailProvider.questionMgr != questionMgr {
		t.Fatalf("expected detail provider to preserve approval/question sources, got %#v", registration.detailProvider)
	}
	if deepResearchTarget.creatorCalls != 1 || deepResearchTarget.creator == nil {
		t.Fatalf("expected harness creator binding for deep research, got %#v", deepResearchTarget)
	}
	if deepResearchTarget.registerCalls != 4 {
		t.Fatalf("expected deep research compatibility + capability groups to be registered four times, got %#v", deepResearchTarget)
	}
	if !routeExists(e, "POST", "/harness/research/jobs") || !routeExists(e, "POST", "/api/harness/research/jobs") {
		t.Fatalf("expected harness research capability routes to be registered, got %#v", e.Routes())
	}
	if !routeExists(e, "POST", "/harness/runs") || !routeExists(e, "POST", "/harness/runs/:id/actions/:action") || !routeExists(e, "POST", "/harness/groups/:id/actions/:action") || !routeExists(e, "GET", "/tasks/:id") || !routeExists(e, "POST", "/tasks/:id/actions/:action") {
		t.Fatalf("expected harness routes to be registered, got %#v", e.Routes())
	}
	if routeExists(e, "GET", "/tasks") {
		t.Fatalf("expected standalone task list route to stay removed, got %#v", e.Routes())
	}
}

func TestNewHarnessRuntimeDetailProvider_PreservesSources(t *testing.T) {
	execApprovals := tools.NewApprovalManager(sse.NewBroker())
	questionMgr := tools.NewQuestionManager(nil, nil, 0)

	provider := newHarnessRuntimeDetailProvider(execApprovals, questionMgr)
	if provider == nil {
		t.Fatal("expected detail provider, got nil")
	}
	if provider.execApprovals != execApprovals || provider.questionMgr != questionMgr {
		t.Fatalf("expected detail provider to preserve sources, got %#v", provider)
	}
}

func TestRegisterHarnessRuntimeWithDetail_SkipsWithoutController(t *testing.T) {
	e := echo.New()

	detailProvider, ok := registerHarnessRuntimeWithDetail(
		nil,
		nil,
		nil,
		nil,
		[]*echo.Group{e.Group("/harness")},
		[]*echo.Group{e.Group("")},
	)
	if ok || detailProvider != nil {
		t.Fatalf("expected no detail provider without runtime bundle, got provider=%#v ok=%v", detailProvider, ok)
	}
	if routeExists(e, "POST", "/harness/runs") || routeExists(e, "GET", "/tasks") {
		t.Fatalf("expected no harness routes to be registered, got %#v", e.Routes())
	}
}

func TestRegisterHarnessRuntimeWithDetail_RegistersRoutesAndReturnsDetailProvider(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	execApprovals := tools.NewApprovalManager(sse.NewBroker())
	questionMgr := tools.NewQuestionManager(nil, nil, 0)
	e := echo.New()

	detailProvider, ok := registerHarnessRuntimeWithDetail(
		bundle,
		nil,
		execApprovals,
		questionMgr,
		[]*echo.Group{e.Group("/harness")},
		[]*echo.Group{e.Group("")},
	)
	if !ok || detailProvider == nil {
		t.Fatalf("expected detail provider and registered routes, got provider=%#v ok=%v", detailProvider, ok)
	}
	if detailProvider.execApprovals != execApprovals || detailProvider.questionMgr != questionMgr {
		t.Fatalf("expected detail provider to keep approval/question sources, got %#v", detailProvider)
	}
	if !routeExists(e, "POST", "/harness/runs") || !routeExists(e, "POST", "/harness/runs/:id/actions/:action") || !routeExists(e, "POST", "/harness/groups/:id/actions/:action") || !routeExists(e, "GET", "/tasks/:id") || !routeExists(e, "POST", "/tasks/:id/actions/:action") {
		t.Fatalf("expected harness and projection routes to be registered, got %#v", e.Routes())
	}
	if routeExists(e, "GET", "/tasks") {
		t.Fatalf("expected standalone task list route to stay removed, got %#v", e.Routes())
	}
}

func TestRegisterHarnessRuntimeRoutes_SkipsWithoutController(t *testing.T) {
	e := echo.New()
	detailProvider := newHarnessRuntimeDetailProvider(nil, nil)

	ok := registerHarnessRuntimeRoutes(
		nil,
		detailProvider,
		nil,
		[]*echo.Group{e.Group("/harness")},
		[]*echo.Group{e.Group("")},
	)
	if ok {
		t.Fatal("expected harness route registration to skip without runtime bundle")
	}
	if routeExists(e, "POST", "/harness/runs") || routeExists(e, "GET", "/tasks") {
		t.Fatalf("expected no harness routes to be registered, got %#v", e.Routes())
	}
}

func TestRegisterHarnessRuntimeRoutes_RegistersHarnessAndProjectionEndpoints(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	e := echo.New()
	detailProvider := newHarnessRuntimeDetailProvider(nil, nil)

	ok := registerHarnessRuntimeRoutes(
		bundle,
		detailProvider,
		nil,
		[]*echo.Group{e.Group("/harness")},
		[]*echo.Group{e.Group("")},
	)
	if !ok {
		t.Fatal("expected harness route registration to succeed")
	}
	if !routeExists(e, "POST", "/harness/runs") || !routeExists(e, "POST", "/harness/runs/:id/actions/:action") || !routeExists(e, "POST", "/harness/groups/:id/actions/:action") || !routeExists(e, "POST", "/tasks/:id/actions/:action") {
		t.Fatalf("expected harness run route to be registered, got %#v", e.Routes())
	}
	if routeExists(e, "GET", "/tasks") {
		t.Fatalf("expected standalone task list route to stay removed, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/tasks/:id") {
		t.Fatalf("expected task detail route to be registered, got %#v", e.Routes())
	}
}

func newTestAgentRuntimeFixture(t *testing.T) (*sql.DB, *HarnessRuntimeBundle, *agent.Store, *agent.Runner) {
	t.Helper()

	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-bindings-fixture.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	bundle, err := newHarnessRuntimeBundle(db, cfg, selfreflect.NewService(nil, nil))
	if err != nil {
		db.Close()
		t.Fatalf("newHarnessRuntimeBundle failed: %v", err)
	}

	store, err := agent.NewStore(db)
	if err != nil {
		db.Close()
		t.Fatalf("new agent store failed: %v", err)
	}

	registry := tools.NewRegistry()
	executor := tools.NewExecutor(registry)
	runner := agent.NewRunner(store, nil, registry, executor, nil, agent.RunnerConfig{})

	return db, bundle, store, runner
}

type stubChatAskRuntimeTarget struct {
	mediaDir             string
	questionMgr          *tools.QuestionManager
	browserCheckpointMgr *tools.BrowserCheckpointManager
	browserSiteStore     *tools.BrowserSiteAllowlistStore
	toolObserver         tools.RuntimeEventObserver
	mediaDirCalls        int
	questionMgrCalls     int
	checkpointCalls      int
	siteStoreCalls       int
	toolObserverCalls    int
}

func (s *stubChatAskRuntimeTarget) SetMediaDir(dir string) {
	s.mediaDir = dir
	s.mediaDirCalls++
}

func (s *stubChatAskRuntimeTarget) SetQuestionManager(mgr *tools.QuestionManager) {
	s.questionMgr = mgr
	s.questionMgrCalls++
}

func (s *stubChatAskRuntimeTarget) SetBrowserCheckpointManager(mgr *tools.BrowserCheckpointManager) {
	s.browserCheckpointMgr = mgr
	s.checkpointCalls++
}

func (s *stubChatAskRuntimeTarget) SetBrowserSiteAllowlistStore(store *tools.BrowserSiteAllowlistStore) {
	s.browserSiteStore = store
	s.siteStoreCalls++
}

func (s *stubChatAskRuntimeTarget) SetToolEventObserver(observer tools.RuntimeEventObserver) {
	s.toolObserver = observer
	s.toolObserverCalls++
}

type stubLayeredMemoryReadyTarget struct {
	readyHook func(*memory.LayeredMemoryService)
}

func (s *stubLayeredMemoryReadyTarget) SetOnLayeredReady(fn func(*memory.LayeredMemoryService)) {
	s.readyHook = fn
}

type stubLayeredMemoryChatTarget struct {
	layeredMemory *memory.LayeredMemoryService
	calls         int
}

func (s *stubLayeredMemoryChatTarget) SetLayeredMemory(svc *memory.LayeredMemoryService) {
	s.layeredMemory = svc
	s.calls++
}

type stubLayeredMemoryAgentTarget struct {
	memory agent.MemoryRecaller
	calls  int
}

func (s *stubLayeredMemoryAgentTarget) SetMemory(m agent.MemoryRecaller) {
	s.memory = m
	s.calls++
}

type stubLayeredMemoryReflectionTarget struct {
	writer selfreflect.MemoryWriter
	calls  int
}

func (s *stubLayeredMemoryReflectionTarget) SetMemoryWriter(writer selfreflect.MemoryWriter) {
	s.writer = writer
	s.calls++
}

type stubSettingsHandlerTarget struct {
	handler *serverpkg.SettingsHandler
	calls   int
}

func (s *stubSettingsHandlerTarget) SetSettingsHandler(handler *serverpkg.SettingsHandler) {
	s.handler = handler
	s.calls++
}

type stubRuntimeResearchSettingsTarget struct {
	enabled bool
	calls   int
}

func (s *stubRuntimeResearchSettingsTarget) SetV2Enabled(enabled bool) {
	s.enabled = enabled
	s.calls++
}

type stubRuntimeAutoConfirmTarget struct {
	autoConfirmFunc func() bool
	calls           int
}

func (s *stubRuntimeAutoConfirmTarget) SetAutoConfirmFunc(fn func() bool) {
	s.autoConfirmFunc = fn
	s.calls++
}

type stubRuntimeLocaleTarget struct {
	localeFunc func() string
	calls      int
}

func (s *stubRuntimeLocaleTarget) SetLocaleFunc(fn func() string) {
	s.localeFunc = fn
	s.calls++
}

type stubRuntimePromptSettingsTarget struct {
	localeFunc            func() string
	timezoneFunc          func() string
	agentModeFunc         func() bool
	agentAutoConfirmFunc  func() bool
	localeCalls           int
	timezoneCalls         int
	agentModeCalls        int
	agentAutoConfirmCalls int
}

func (s *stubRuntimePromptSettingsTarget) SetLocaleFunc(fn func() string) {
	s.localeFunc = fn
	s.localeCalls++
}

func (s *stubRuntimePromptSettingsTarget) SetTimezoneFunc(fn func() string) {
	s.timezoneFunc = fn
	s.timezoneCalls++
}

func (s *stubRuntimePromptSettingsTarget) SetAgentModeFunc(fn func() bool) {
	s.agentModeFunc = fn
	s.agentModeCalls++
}

func (s *stubRuntimePromptSettingsTarget) SetAgentAutoConfirmFunc(fn func() bool) {
	s.agentAutoConfirmFunc = fn
	s.agentAutoConfirmCalls++
}

func (s *stubRuntimePromptSettingsTarget) calls() int {
	return s.localeCalls + s.timezoneCalls + s.agentModeCalls + s.agentAutoConfirmCalls
}

type stubRuntimeAdminSettingsTarget struct {
	settings tools.AdminSettingsService
	calls    int
}

func (s *stubRuntimeAdminSettingsTarget) SetSettings(settings tools.AdminSettingsService) {
	s.settings = settings
	s.calls++
}

type stubRuntimePromptGuardTarget struct {
	detector *promptguard.Detector
	calls    int
}

func (s *stubRuntimePromptGuardTarget) SetPromptGuard(detector *promptguard.Detector) {
	s.detector = detector
	s.calls++
}

type stubRuntimeProxyBridgeChatTarget struct {
	runtimeProvider llm.Provider
	proxyBridge     *proxybridge.Bridge
	imModel         string
	runtimeCalls    int
	bridgeCalls     int
	imModelCalls    int
}

func (s *stubRuntimeProxyBridgeChatTarget) SetRuntimeProvider(provider llm.Provider) {
	s.runtimeProvider = provider
	s.runtimeCalls++
}

func (s *stubRuntimeProxyBridgeChatTarget) SetProxyBridge(bridge *proxybridge.Bridge) {
	s.proxyBridge = bridge
	s.bridgeCalls++
}

func (s *stubRuntimeProxyBridgeChatTarget) SetIMModel(model string) {
	s.imModel = model
	s.imModelCalls++
}

type stubRuntimeProxyBridgeVoiceTarget struct {
	chatFunc voice.ChatFunc
	calls    int
}

func (s *stubRuntimeProxyBridgeVoiceTarget) SetChatFunc(fn voice.ChatFunc) {
	s.chatFunc = fn
	s.calls++
}

type stubRuntimeProxyBridgeVLMTarget struct {
	bridge tools.VLMBridge
	calls  int
}

func (s *stubRuntimeProxyBridgeVLMTarget) SetVLMBridge(bridge tools.VLMBridge) {
	s.bridge = bridge
	s.calls++
}

type stubRuntimeProxyBridgeMediaTarget struct {
	bridge tools.VLMBridge
	calls  int
}

func (s *stubRuntimeProxyBridgeMediaTarget) SetFallbackVisionBridge(bridge tools.VLMBridge) {
	s.bridge = bridge
	s.calls++
}

type stubRuntimeProxyBridgePDFTarget struct {
	vision pdfextract.VisionService
	calls  int
}

func (s *stubRuntimeProxyBridgePDFTarget) SetVisionService(vision pdfextract.VisionService) {
	s.vision = vision
	s.calls++
}

type stubRuntimeProxyBridgeImageTarget struct {
	bridge tools.VLMBridge
	calls  int
}

func (s *stubRuntimeProxyBridgeImageTarget) SetVisionBridge(bridge tools.VLMBridge) {
	s.bridge = bridge
	s.calls++
}

type stubRuntimeProxyBridgeSkillTarget struct {
	bridge *proxybridge.Bridge
	calls  int
}

func (s *stubRuntimeProxyBridgeSkillTarget) SetBridge(bridge *proxybridge.Bridge) {
	s.bridge = bridge
	s.calls++
}

type stubRuntimeProxyBridgeAnalyzeTarget struct {
	bridge tools.LLMBridge
	calls  int
}

func (s *stubRuntimeProxyBridgeAnalyzeTarget) SetLLMBridge(bridge tools.LLMBridge) {
	s.bridge = bridge
	s.calls++
}

type stubRuntimeBrowserBackend struct{}

func (s *stubRuntimeBrowserBackend) Start(context.Context) error { return nil }

func (s *stubRuntimeBrowserBackend) Navigate(context.Context, string, string) (tools.BrowserNavResult, error) {
	return tools.BrowserNavResult{}, nil
}

func (s *stubRuntimeBrowserBackend) CookieHeader(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *stubRuntimeBrowserBackend) ObserveNetwork(context.Context, string, int, bool) (tools.BrowserObservedNetworkResult, error) {
	return tools.BrowserObservedNetworkResult{}, nil
}

func (s *stubRuntimeBrowserBackend) WaitNetworkIdle(context.Context, string, int, int) error {
	return nil
}

func (s *stubRuntimeBrowserBackend) AccessibilityTree(context.Context, string, int) (tools.BrowserA11yTreeResult, error) {
	return tools.BrowserA11yTreeResult{}, nil
}

func (s *stubRuntimeBrowserBackend) InteractiveElements(context.Context, string) (tools.BrowserInteractiveResult, error) {
	return tools.BrowserInteractiveResult{}, nil
}

func (s *stubRuntimeBrowserBackend) CountInteractiveElements(context.Context, string) (int, error) {
	return 0, nil
}

func (s *stubRuntimeBrowserBackend) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return nil
}

func (s *stubRuntimeBrowserBackend) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return nil
}

func (s *stubRuntimeBrowserBackend) Screenshot(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubRuntimeBrowserBackend) ScreenshotTab(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubRuntimeBrowserBackend) CloseTab(context.Context, string) error {
	return nil
}

func (s *stubRuntimeBrowserBackend) Tabs(context.Context) ([]tools.BrowserTabResult, error) {
	return nil, nil
}

func (s *stubRuntimeBrowserBackend) ExecuteRecipe(context.Context, string, map[string]string) (tools.BrowserRecipeResult, error) {
	return tools.BrowserRecipeResult{}, nil
}

func (s *stubRuntimeBrowserBackend) ListRecipes(context.Context) []tools.BrowserRecipeInfo {
	return nil
}

type stubRuntimeBrowserMediaDirTarget struct {
	mediaDir string
	calls    int
}

func (s *stubRuntimeBrowserMediaDirTarget) SetMediaDir(dir string) {
	s.mediaDir = dir
	s.calls++
}

type stubRuntimeBrowserAccessTarget struct {
	browser         tools.BrowserBackend
	lightpanda      *browser.LightpandaService
	browserCalls    int
	lightpandaCalls int
}

func (s *stubRuntimeBrowserAccessTarget) SetBrowser(browser tools.BrowserBackend) {
	s.browser = browser
	s.browserCalls++
}

func (s *stubRuntimeBrowserAccessTarget) SetLightpandaShim(service *browser.LightpandaService) {
	s.lightpanda = service
	s.lightpandaCalls++
}

type stubRuntimeBrowserSkillTarget struct {
	browserService builtin.BrowserServiceInterface
	mediaDir       string
	serviceCalls   int
	mediaCalls     int
}

func (s *stubRuntimeBrowserSkillTarget) SetBrowserService(svc builtin.BrowserServiceInterface) {
	s.browserService = svc
	s.serviceCalls++
}

func (s *stubRuntimeBrowserSkillTarget) SetMediaDir(dir string) {
	s.mediaDir = dir
	s.mediaCalls++
}

type stubRuntimeUIReviewerSkillTarget struct {
	browserService builtin.BrowserServiceInterface
	calls          int
}

func (s *stubRuntimeUIReviewerSkillTarget) SetBrowserService(svc builtin.BrowserServiceInterface) {
	s.browserService = svc
	s.calls++
}

type stubBuiltinBrowserService struct{}

func (s *stubBuiltinBrowserService) Start(context.Context) error { return nil }

func (s *stubBuiltinBrowserService) Navigate(context.Context, string, string) (builtin.BrowserNavResult, error) {
	return builtin.BrowserNavResult{}, nil
}

func (s *stubBuiltinBrowserService) ExtractText(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *stubBuiltinBrowserService) AccessibilityTree(context.Context, string, int) (builtin.BrowserA11yResult, error) {
	return builtin.BrowserA11yResult{}, nil
}

func (s *stubBuiltinBrowserService) InteractiveElements(context.Context, string) (builtin.BrowserInteractiveResult, error) {
	return builtin.BrowserInteractiveResult{}, nil
}

func (s *stubBuiltinBrowserService) CountInteractiveElements(context.Context, string) (int, error) {
	return 0, nil
}

func (s *stubBuiltinBrowserService) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return nil
}

func (s *stubBuiltinBrowserService) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return nil
}

func (s *stubBuiltinBrowserService) Screenshot(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubBuiltinBrowserService) ScreenshotTab(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubBuiltinBrowserService) CloseTab(context.Context, string) error {
	return nil
}

func (s *stubBuiltinBrowserService) Tabs(context.Context) ([]builtin.BrowserTabInfo, error) {
	return nil, nil
}

func (s *stubBuiltinBrowserService) ExecuteRecipe(context.Context, string, map[string]string) (builtin.BrowserRecipeResult, error) {
	return builtin.BrowserRecipeResult{}, nil
}

func (s *stubBuiltinBrowserService) ListRecipes(context.Context) []builtin.BrowserRecipeInfo {
	return nil
}

type stubRuntimeAnalyzeTarget struct {
	browser       tools.BrowserBackend
	executor      *tools.Executor
	browserCalls  int
	executorCalls int
}

func (s *stubRuntimeAnalyzeTarget) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	return map[string]string{"ok": "true"}, nil
}

func (s *stubRuntimeAnalyzeTarget) SetBrowser(browser tools.BrowserBackend) {
	s.browser = browser
	s.browserCalls++
}

func (s *stubRuntimeAnalyzeTarget) SetExecutor(executor *tools.Executor) {
	s.executor = executor
	s.executorCalls++
}

type stubRuntimeAnalyzeSkillTarget struct {
	executor builtin.AnalyzeExecutor
	calls    int
}

func (s *stubRuntimeAnalyzeSkillTarget) SetExecutor(e builtin.AnalyzeExecutor) {
	s.executor = e
	s.calls++
}

type stubRuntimeWebSearchSkillTarget struct {
	searcher builtin.WebSearcher
	calls    int
}

func (s *stubRuntimeWebSearchSkillTarget) SetSearcher(searcher builtin.WebSearcher) {
	s.searcher = searcher
	s.calls++
}

type stubRuntimeDeepResearchSkillTarget struct {
	executor builtin.DeepResearchExecutor
	calls    int
}

func (s *stubRuntimeDeepResearchSkillTarget) SetExecutor(executor builtin.DeepResearchExecutor) {
	s.executor = executor
	s.calls++
}

type stubRuntimeSchedulerSkillTarget struct {
	service builtin.CronServiceInterface
	calls   int
}

func (s *stubRuntimeSchedulerSkillTarget) SetCronService(service builtin.CronServiceInterface) {
	s.service = service
	s.calls++
}

type stubRuntimeSchedulerCalendarTarget struct {
	service tools.CronService
	calls   int
}

func (s *stubRuntimeSchedulerCalendarTarget) SetCronService(service tools.CronService) {
	s.service = service
	s.calls++
}

type stubRuntimeCronHandlerTarget struct {
	service   *cron.Service
	initHooks []func(*cron.Service)
	getCalls  int
}

func (s *stubRuntimeCronHandlerTarget) GetService() *cron.Service {
	s.getCalls++
	return s.service
}

func (s *stubRuntimeCronHandlerTarget) SetServiceInitHook(fn func(*cron.Service)) {
	if fn == nil {
		return
	}
	s.initHooks = append(s.initHooks, fn)
}

type stubRuntimeReminderCalendarTarget struct {
	service tools.PushServiceInterface
	calls   int
}

func (s *stubRuntimeReminderCalendarTarget) SetReminderService(service tools.PushServiceInterface) {
	s.service = service
	s.calls++
}

type stubRuntimeReminderSkillTarget struct {
	service builtin.PushServiceInterface
	calls   int
}

func (s *stubRuntimeReminderSkillTarget) SetPushService(svc builtin.PushServiceInterface) {
	s.service = svc
	s.calls++
}

type stubRuntimeSpeechServiceSource struct {
	service speech.Service
	calls   int
}

func (s *stubRuntimeSpeechServiceSource) Service() speech.Service {
	s.calls++
	return s.service
}

type stubRuntimeVoiceServiceSource struct {
	service voice.Service
	calls   int
}

func (s *stubRuntimeVoiceServiceSource) Service() voice.Service {
	s.calls++
	return s.service
}

type stubRuntimeMgmtToolTarget struct {
	providers     tools.AdminProviderService
	skills        tools.AdminSkillService
	tooling       tools.AdminToolService
	system        tools.AdminSystemService
	users         tools.AdminUserService
	apiKeys       tools.AdminAPIKeyService
	upgrade       tools.AdminUpgradeService
	providerCalls int
	skillCalls    int
	toolCalls     int
	systemCalls   int
	userCalls     int
	apiKeyCalls   int
	upgradeCalls  int
}

func (s *stubRuntimeMgmtToolTarget) SetProviders(svc tools.AdminProviderService) {
	s.providers = svc
	s.providerCalls++
}

func (s *stubRuntimeMgmtToolTarget) SetSkills(svc tools.AdminSkillService) {
	s.skills = svc
	s.skillCalls++
}

func (s *stubRuntimeMgmtToolTarget) SetTools(svc tools.AdminToolService) {
	s.tooling = svc
	s.toolCalls++
}

func (s *stubRuntimeMgmtToolTarget) SetSystem(svc tools.AdminSystemService) {
	s.system = svc
	s.systemCalls++
}

func (s *stubRuntimeMgmtToolTarget) SetUsers(svc tools.AdminUserService) {
	s.users = svc
	s.userCalls++
}

func (s *stubRuntimeMgmtToolTarget) SetAPIKeys(svc tools.AdminAPIKeyService) {
	s.apiKeys = svc
	s.apiKeyCalls++
}

func (s *stubRuntimeMgmtToolTarget) SetUpgrade(svc tools.AdminUpgradeService) {
	s.upgrade = svc
	s.upgradeCalls++
}

type stubRuntimeExecToolTarget struct {
	auditStore       *tools.ExecAuditStore
	toolNames        []string
	registry         *tools.Registry
	pinnedSkills     []string
	skillExec        tools.SkillExecFunc
	skillSelect      tools.SkillSelectFunc
	auditCalls       int
	toolNameCalls    int
	registryCalls    int
	pinnedCalls      int
	skillExecCalls   int
	skillSelectCalls int
}

func (s *stubRuntimeExecToolTarget) SetAuditStore(store *tools.ExecAuditStore) {
	s.auditStore = store
	s.auditCalls++
}

func (s *stubRuntimeExecToolTarget) SetToolNames(names []string) {
	s.toolNames = append([]string(nil), names...)
	s.toolNameCalls++
}

func (s *stubRuntimeExecToolTarget) SetRegistry(registry *tools.Registry) {
	s.registry = registry
	s.registryCalls++
}

func (s *stubRuntimeExecToolTarget) SetPinnedSkills(names []string) {
	s.pinnedSkills = append([]string(nil), names...)
	s.pinnedCalls++
}

func (s *stubRuntimeExecToolTarget) SetSkillExecutor(fn tools.SkillExecFunc) {
	s.skillExec = fn
	s.skillExecCalls++
}

func (s *stubRuntimeExecToolTarget) SetSkillSelector(fn tools.SkillSelectFunc) {
	s.skillSelect = fn
	s.skillSelectCalls++
}

type stubRuntimeSkillRegistrySource struct {
	skills  map[string]skillpkg.Skill
	enabled map[string]bool
}

func (s *stubRuntimeSkillRegistrySource) Get(id string) skillpkg.Skill {
	if s == nil {
		return nil
	}
	return s.skills[id]
}

func (s *stubRuntimeSkillRegistrySource) IsEnabled(id string) bool {
	if s == nil || s.enabled == nil {
		return false
	}
	return s.enabled[id]
}

type stubRuntimeSkill struct {
	manifest    *skillpkg.Manifest
	result      *skillpkg.Result
	validateErr error
	executeErr  error
	calls       int
}

func (s *stubRuntimeSkill) Manifest() *skillpkg.Manifest {
	return s.manifest
}

func (s *stubRuntimeSkill) Execute(_ context.Context, _ map[string]any) (*skillpkg.Result, error) {
	s.calls++
	return s.result, s.executeErr
}

func (s *stubRuntimeSkill) Validate(_ map[string]any) error {
	return s.validateErr
}

type stubRuntimeExecSkillSelectionSource struct {
	selector      *agentcore.SkillSelector
	settings      *serverpkg.SettingsHandler
	selectorCalls int
	settingsCalls int
}

func (s *stubRuntimeExecSkillSelectionSource) GetSkillSelector() *agentcore.SkillSelector {
	s.selectorCalls++
	return s.selector
}

func (s *stubRuntimeExecSkillSelectionSource) GetSettingsHandler() *serverpkg.SettingsHandler {
	s.settingsCalls++
	return s.settings
}

type stubRuntimeImageToolTarget struct {
	ocr         tools.ImageOCRService
	vision      tools.ProviderAwareImageVision
	ppt         tools.PPTGenerateService
	ocrCalls    int
	visionCalls int
	pptCalls    int
}

func (s *stubRuntimeImageToolTarget) SetOCRService(svc tools.ImageOCRService) {
	s.ocr = svc
	s.ocrCalls++
}

func (s *stubRuntimeImageToolTarget) SetProviderVision(vision tools.ProviderAwareImageVision) {
	s.vision = vision
	s.visionCalls++
}

func (s *stubRuntimeImageToolTarget) SetPPTService(svc tools.PPTGenerateService) {
	s.ppt = svc
	s.pptCalls++
}

type stubSmallModelRuntime struct{}

func (s *stubSmallModelRuntime) Generate(_ context.Context, _ smallmodel.GenerateRequest) (*smallmodel.GenerateResponse, error) {
	return &smallmodel.GenerateResponse{Text: "ok"}, nil
}

func (s *stubSmallModelRuntime) Ready() bool {
	return true
}

type stubRuntimeSmallModelSettingsSource struct {
	enabled      bool
	imageQA      bool
	docExtract   bool
	toggleResult bool
	toggleCalls  []bool
}

func (s *stubRuntimeSmallModelSettingsSource) GetSmallModelEnabled() bool {
	return s.enabled
}

func (s *stubRuntimeSmallModelSettingsSource) GetSmallModelRouteImageQAEnabled() bool {
	return s.imageQA
}

func (s *stubRuntimeSmallModelSettingsSource) GetSmallModelDocExtractEnabled() bool {
	return s.docExtract
}

func (s *stubRuntimeSmallModelSettingsSource) SetSmallModelDocExtractEnabled(enabled bool) (bool, error) {
	s.toggleCalls = append(s.toggleCalls, enabled)
	return s.toggleResult, nil
}

type stubRuntimeSmallModelChatTarget struct {
	runtime smallmodel.Runtime
	calls   int
}

func (s *stubRuntimeSmallModelChatTarget) SetSmallModelRuntime(rt smallmodel.Runtime) {
	s.runtime = rt
	s.calls++
}

type stubRuntimeSmallModelAuxiliaryTarget struct {
	runtime smallmodel.Runtime
	calls   int
}

func (s *stubRuntimeSmallModelAuxiliaryTarget) SetSmallModel(rt smallmodel.Runtime) {
	s.runtime = rt
	s.calls++
}

type stubRuntimeImageSmallModelTarget struct {
	runtime     smallmodel.Runtime
	enabledFunc func() bool
	calls       int
}

func (s *stubRuntimeImageSmallModelTarget) SetSmallModelRuntime(rt smallmodel.Runtime) {
	s.runtime = rt
	s.calls++
}

func (s *stubRuntimeImageSmallModelTarget) SetSmallModelEnabledFunc(fn func() bool) {
	s.enabledFunc = fn
}

type stubRuntimeAnalyzeSmallModelTarget struct {
	runtime        smallmodel.Runtime
	enabledFunc    func() bool
	docExtractFunc func() bool
	toggleFunc     func(bool) (bool, error)
	statsRecorder  tools.SmallModelStatsRecorder
	runtimeCalls   int
	statsCalls     int
}

func (s *stubRuntimeAnalyzeSmallModelTarget) SetSmallModelRuntime(rt smallmodel.Runtime) {
	s.runtime = rt
	s.runtimeCalls++
}

func (s *stubRuntimeAnalyzeSmallModelTarget) SetSmallModelSwitchFuncs(enabledFn, docExtractFn func() bool) {
	s.enabledFunc = enabledFn
	s.docExtractFunc = docExtractFn
}

func (s *stubRuntimeAnalyzeSmallModelTarget) SetSmallModelDocExtractToggle(setter func(bool) (bool, error)) {
	s.toggleFunc = setter
}

func (s *stubRuntimeAnalyzeSmallModelTarget) SetSmallModelStatsRecorder(recorder tools.SmallModelStatsRecorder) {
	s.statsRecorder = recorder
	s.statsCalls++
}

type stubRuntimeSmallModelStatsSource struct {
	stats *serverpkg.SmallModelStats
}

func (s *stubRuntimeSmallModelStatsSource) GetSmallModelStats() *serverpkg.SmallModelStats {
	return s.stats
}

type stubRuntimeSkillRerankerSettingsTarget struct {
	manager                        *agentcore.SkillRerankerModelManager
	smallModelEnabled              bool
	smallModelRerankEnabled        bool
	skillRerankEnabled             bool
	skillRerankEnabledSet          bool
	skillRerankONNXEnabled         bool
	skillRerankONNXEnabledSet      bool
	skillRerankONNXAutoDownload    bool
	skillRerankONNXAutoDownloadSet bool
	managerCalls                   int
}

func (s *stubRuntimeSkillRerankerSettingsTarget) SetSkillRerankerModelManager(mgr *agentcore.SkillRerankerModelManager) {
	s.manager = mgr
	s.managerCalls++
}

func (s *stubRuntimeSkillRerankerSettingsTarget) GetSmallModelEnabled() bool {
	return s.smallModelEnabled
}

func (s *stubRuntimeSkillRerankerSettingsTarget) GetSmallModelRerankEnabled() bool {
	return s.smallModelRerankEnabled
}

func (s *stubRuntimeSkillRerankerSettingsTarget) GetSkillRerankEnabled() bool {
	return s.skillRerankEnabled
}

func (s *stubRuntimeSkillRerankerSettingsTarget) IsSkillRerankEnabledSet() bool {
	return s.skillRerankEnabledSet
}

func (s *stubRuntimeSkillRerankerSettingsTarget) GetSkillRerankONNXEnabled() bool {
	return s.skillRerankONNXEnabled
}

func (s *stubRuntimeSkillRerankerSettingsTarget) IsSkillRerankONNXEnabledSet() bool {
	return s.skillRerankONNXEnabledSet
}

func (s *stubRuntimeSkillRerankerSettingsTarget) GetSkillRerankONNXAutoDownload() bool {
	return s.skillRerankONNXAutoDownload
}

func (s *stubRuntimeSkillRerankerSettingsTarget) IsSkillRerankONNXAutoDownloadSet() bool {
	return s.skillRerankONNXAutoDownloadSet
}

type stubRuntimeSkillRerankerTarget struct {
	manager          *agentcore.SkillRerankerModelManager
	onnxEnabledFunc  func() bool
	autoDownloadFunc func() bool
	switchCalls      int
}

func (s *stubRuntimeSkillRerankerTarget) ModelManager() *agentcore.SkillRerankerModelManager {
	return s.manager
}

func (s *stubRuntimeSkillRerankerTarget) SetSwitchFuncs(onnxEnabledFn, autoDownloadFn func() bool) {
	s.onnxEnabledFunc = onnxEnabledFn
	s.autoDownloadFunc = autoDownloadFn
	s.switchCalls++
}

type stubRuntimeCompactorMemoryChatTarget struct {
	integration      *session.CompactorMemoryIntegration
	sessionMaxTokens int
	calls            int
}

func (s *stubRuntimeCompactorMemoryChatTarget) SetCompactorMemoryIntegration(integration *session.CompactorMemoryIntegration, sessionMaxTokens int) {
	s.integration = integration
	s.sessionMaxTokens = sessionMaxTokens
	s.calls++
}

type stubQuestionRuntimePolicyTarget struct {
	silentFunc         func() bool
	timeoutFunc        func() time.Duration
	timeoutActionFunc  func() string
	silentCalls        int
	timeoutCalls       int
	timeoutActionCalls int
}

func (s *stubQuestionRuntimePolicyTarget) SetSilentFunc(fn func() bool) {
	s.silentFunc = fn
	s.silentCalls++
}

func (s *stubQuestionRuntimePolicyTarget) SetTimeoutFunc(fn func() time.Duration) {
	s.timeoutFunc = fn
	s.timeoutCalls++
}

func (s *stubQuestionRuntimePolicyTarget) SetTimeoutActionFunc(fn func() string) {
	s.timeoutActionFunc = fn
	s.timeoutActionCalls++
}

type stubAgentRuntimePolicyTarget struct {
	askTimeoutFunc        func() time.Duration
	askTimeoutActionFunc  func() string
	maxToolRoundsFunc     func() int
	autoReflectFunc       func() bool
	askTimeoutCalls       int
	askTimeoutActionCalls int
	maxToolRoundsCalls    int
	autoReflectCalls      int
}

func (s *stubAgentRuntimePolicyTarget) SetAskTimeoutFunc(fn func() time.Duration) {
	s.askTimeoutFunc = fn
	s.askTimeoutCalls++
}

func (s *stubAgentRuntimePolicyTarget) SetAskTimeoutActionFunc(fn func() string) {
	s.askTimeoutActionFunc = fn
	s.askTimeoutActionCalls++
}

func (s *stubAgentRuntimePolicyTarget) SetMaxToolRoundsPerStepFunc(fn func() int) {
	s.maxToolRoundsFunc = fn
	s.maxToolRoundsCalls++
}

func (s *stubAgentRuntimePolicyTarget) SetAutoReflectFunc(fn func() bool) {
	s.autoReflectFunc = fn
	s.autoReflectCalls++
}

type stubAgentRuntimeSupportTarget struct {
	reflector agent.SelfReflector
	metrics   interface {
		RecordCounter(name string, value int64, tags map[string]string)
	}
	reflectorCalls int
	metricsCalls   int
}

func (s *stubAgentRuntimeSupportTarget) SetReflector(reflector agent.SelfReflector) {
	s.reflector = reflector
	s.reflectorCalls++
}

func (s *stubAgentRuntimeSupportTarget) SetToolMetricsRecorder(recorder interface {
	RecordCounter(name string, value int64, tags map[string]string)
}) {
	s.metrics = recorder
	s.metricsCalls++
}

type stubReflectionRuntimeTarget struct {
	llmCaller         selfreflect.LLMCaller
	proposalGate      func() bool
	llmCalls          int
	proposalGateCalls int
}

func (s *stubReflectionRuntimeTarget) SetLLMCaller(llmCaller selfreflect.LLMCaller) {
	s.llmCaller = llmCaller
	s.llmCalls++
}

func (s *stubReflectionRuntimeTarget) SetProposalGateFunc(fn func() bool) {
	s.proposalGate = fn
	s.proposalGateCalls++
}

type stubAgentRuntimeTarget struct {
	eventObserver    agent.TaskEventObserver
	toolObserver     tools.RuntimeEventObserver
	subagentExecutor tools.SubagentExecutor
	writeGuard       tools.WritePathGuard
	execGuard        tools.ExecPathGuard
	calls            int
}

func (s *stubAgentRuntimeTarget) SetEventObserver(observer agent.TaskEventObserver) {
	s.eventObserver = observer
	s.calls++
}

func (s *stubAgentRuntimeTarget) SetToolEventObserver(observer tools.RuntimeEventObserver) {
	s.toolObserver = observer
	s.calls++
}

func (s *stubAgentRuntimeTarget) SetSubagentExecutor(executor tools.SubagentExecutor) {
	s.subagentExecutor = executor
	s.calls++
}

func (s *stubAgentRuntimeTarget) SetWritePathGuard(guard tools.WritePathGuard) {
	s.writeGuard = guard
	s.calls++
}

func (s *stubAgentRuntimeTarget) SetExecPathGuard(guard tools.ExecPathGuard) {
	s.execGuard = guard
	s.calls++
}

func (s *stubAgentRuntimeTarget) callCount() int {
	return s.calls
}

type stubToolEventObserverTarget struct {
	observer tools.RuntimeEventObserver
	calls    int
}

func (s *stubToolEventObserverTarget) SetToolEventObserver(observer tools.RuntimeEventObserver) {
	s.observer = observer
	s.calls++
}

type stubRuntimeObserverTarget struct {
	observer tools.RuntimeEventObserver
	calls    int
}

func (s *stubRuntimeObserverTarget) SetObserver(observer tools.RuntimeEventObserver) {
	s.observer = observer
	s.calls++
}

type stubToolApproverTarget struct {
	approver tools.ToolApprover
	calls    int
}

func (s *stubToolApproverTarget) SetToolApprover(approver tools.ToolApprover) {
	s.approver = approver
	s.calls++
}

type stubDeepResearchJobCreatorTarget struct {
	creator interface {
		CreateJob(ctx context.Context, req deepresearch.CreateJobRequest) (*deepresearch.Job, error)
	}
	service interface {
		ListJobsForUser(userID, tenantID string, activeOnly bool) ([]deepresearch.JobSummary, error)
		GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error)
		GetReportForUser(id, userID, tenantID string) (*deepresearch.Report, error)
		CancelJobForUser(id, userID, tenantID string) error
		SubscribeForUser(jobID, userID, tenantID string) (<-chan deepresearch.Event, func(), error)
	}
	calls        int
	serviceCalls int
}

func (s *stubDeepResearchJobCreatorTarget) SetJobCreator(creator interface {
	CreateJob(ctx context.Context, req deepresearch.CreateJobRequest) (*deepresearch.Job, error)
}) {
	s.creator = creator
	s.calls++
}

func (s *stubDeepResearchJobCreatorTarget) SetJobService(service interface {
	ListJobsForUser(userID, tenantID string, activeOnly bool) ([]deepresearch.JobSummary, error)
	GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error)
	GetReportForUser(id, userID, tenantID string) (*deepresearch.Report, error)
	CancelJobForUser(id, userID, tenantID string) error
	SubscribeForUser(jobID, userID, tenantID string) (<-chan deepresearch.Event, func(), error)
}) {
	s.service = service
	s.serviceCalls++
}

type stubDeepResearchRuntimeRouteTarget struct {
	creator interface {
		CreateJob(ctx context.Context, req deepresearch.CreateJobRequest) (*deepresearch.Job, error)
	}
	service interface {
		ListJobsForUser(userID, tenantID string, activeOnly bool) ([]deepresearch.JobSummary, error)
		GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error)
		GetReportForUser(id, userID, tenantID string) (*deepresearch.Report, error)
		CancelJobForUser(id, userID, tenantID string) error
		SubscribeForUser(jobID, userID, tenantID string) (<-chan deepresearch.Event, func(), error)
	}
	creatorCalls  int
	serviceCalls  int
	groups        []*echo.Group
	registerCalls int
}

func (s *stubDeepResearchRuntimeRouteTarget) SetJobCreator(creator interface {
	CreateJob(ctx context.Context, req deepresearch.CreateJobRequest) (*deepresearch.Job, error)
}) {
	s.creator = creator
	s.creatorCalls++
}

func (s *stubDeepResearchRuntimeRouteTarget) SetJobService(service interface {
	ListJobsForUser(userID, tenantID string, activeOnly bool) ([]deepresearch.JobSummary, error)
	GetJobForUser(id, userID, tenantID string) (*deepresearch.Job, error)
	GetReportForUser(id, userID, tenantID string) (*deepresearch.Report, error)
	CancelJobForUser(id, userID, tenantID string) error
	SubscribeForUser(jobID, userID, tenantID string) (<-chan deepresearch.Event, func(), error)
}) {
	s.service = service
	s.serviceCalls++
}

func (s *stubDeepResearchRuntimeRouteTarget) RegisterGroup(group *echo.Group) {
	s.groups = append(s.groups, group)
	s.registerCalls++
	if group == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	group.POST("/jobs", func(c echo.Context) error { return c.NoContent(http.StatusAccepted) })
	group.GET("/jobs", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
}

type stubAutoHarnessTurnHookTarget struct {
	hook  serverpkg.TurnHook
	calls int
}

func (s *stubAutoHarnessTurnHookTarget) RegisterTurnHook(hook serverpkg.TurnHook) {
	s.hook = hook
	s.calls++
}

type stubTurnHook struct{}

func (s *stubTurnHook) BeforeModelCall(_ context.Context, _ serverpkg.TurnContext) ([]llm.Message, error) {
	return nil, nil
}

func (s *stubTurnHook) AfterAssistantPersisted(_ context.Context, _ serverpkg.TurnContext, _ *memory.Message) error {
	return nil
}

type stubHarnessJudgeEvaluatorTarget struct {
	evaluator harness.JudgeEvaluator
	calls     int
}

func (s *stubHarnessJudgeEvaluatorTarget) SetJudgeEvaluator(evaluator harness.JudgeEvaluator) {
	s.evaluator = evaluator
	s.calls++
}

type stubHarnessJudgeLLMCaller struct{}

func (s *stubHarnessJudgeLLMCaller) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

type stubApprovalRiskLLMCaller struct {
	content string
	err     error
	calls   int
}

func (s *stubApprovalRiskLLMCaller) Name() string { return "stub-risk" }

func (s *stubApprovalRiskLLMCaller) Models() []string { return []string{"stub-risk"} }

func (s *stubApprovalRiskLLMCaller) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: s.content},
	}, nil
}

func (s *stubApprovalRiskLLMCaller) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubApprovalRiskLLMCaller) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return fmt.Errorf("not implemented")
}

type stubApprovalRuntimeHandlerTarget struct {
	resolver      networkapi.ExecApprovalResolver
	observer      tools.RuntimeEventObserver
	resolverCalls int
	observerCalls int
}

func (s *stubApprovalRuntimeHandlerTarget) SetExecResolver(resolver networkapi.ExecApprovalResolver) {
	s.resolver = resolver
	s.resolverCalls++
}

func (s *stubApprovalRuntimeHandlerTarget) SetObserver(observer tools.RuntimeEventObserver) {
	s.observer = observer
	s.observerCalls++
}

type stubRuntimeBindingTool struct {
	def    tools.ToolDefinition
	result interface{}
	err    error
	calls  int
}

func (t *stubRuntimeBindingTool) Definition() tools.ToolDefinition { return t.def }

func (t *stubRuntimeBindingTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	t.calls++
	return t.result, t.err
}

type stubToolApprover struct {
	decision tools.ToolApprovalDecision
	err      error
	calls    int
}

func (s *stubToolApprover) AuthorizeToolCall(_ context.Context, _ tools.ToolApprovalRequest) (tools.ToolApprovalDecision, error) {
	s.calls++
	return s.decision, s.err
}

type stubMetricsRecorder struct {
	calls []string
}

func (s *stubMetricsRecorder) RecordCounter(name string, _ int64, _ map[string]string) {
	s.calls = append(s.calls, name)
}

func (s *stubMetricsRecorder) hasCall(name string) bool {
	for _, call := range s.calls {
		if call == name {
			return true
		}
	}
	return false
}

type stubRuntimeAskPolicySettingsSource struct {
	autoConfirm    bool
	askTimeoutSecs int
	timeoutAction  string
	maxToolRounds  int
	autoReflect    bool
}

func (s *stubRuntimeAskPolicySettingsSource) GetAgentAutoConfirm() bool {
	return s.autoConfirm
}

func (s *stubRuntimeAskPolicySettingsSource) GetAgentAskTimeoutSeconds() int {
	return s.askTimeoutSecs
}

func (s *stubRuntimeAskPolicySettingsSource) GetAgentAskTimeoutAction() string {
	return s.timeoutAction
}

func (s *stubRuntimeAskPolicySettingsSource) GetAgentLoopPolicyMaxToolRounds() int {
	return s.maxToolRounds
}

func (s *stubRuntimeAskPolicySettingsSource) GetAgentAutoReflect() bool {
	return s.autoReflect
}

func routeExists(e *echo.Echo, method string, path string) bool {
	for _, route := range e.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

func containsRuntimeString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type stubRuntimeObserver struct {
	toolRequested     []tools.ToolRuntimeEvent
	toolFinished      []tools.ToolRuntimeEvent
	approvalRequested []tools.ApprovalRuntimeEvent
	approvalResolved  []tools.ApprovalRuntimeEvent
	questionRequested []tools.QuestionRuntimeEvent
	questionResolved  []tools.QuestionRuntimeEvent
}

func (s *stubRuntimeObserver) OnToolRequested(event tools.ToolRuntimeEvent) {
	s.toolRequested = append(s.toolRequested, event)
}

func (s *stubRuntimeObserver) OnToolFinished(event tools.ToolRuntimeEvent) {
	s.toolFinished = append(s.toolFinished, event)
}

func (s *stubRuntimeObserver) OnApprovalRequested(event tools.ApprovalRuntimeEvent) {
	s.approvalRequested = append(s.approvalRequested, event)
}

func (s *stubRuntimeObserver) OnApprovalResolved(event tools.ApprovalRuntimeEvent) {
	s.approvalResolved = append(s.approvalResolved, event)
}

func (s *stubRuntimeObserver) OnQuestionRequested(event tools.QuestionRuntimeEvent) {
	s.questionRequested = append(s.questionRequested, event)
}

func (s *stubRuntimeObserver) OnQuestionResolved(event tools.QuestionRuntimeEvent) {
	s.questionResolved = append(s.questionResolved, event)
}

type stubWorkflowRuntimeServiceTarget struct {
	gateway workflow.ToolRuntime
	metrics workflowRuntimeMetricsRecorder
}

func (s *stubWorkflowRuntimeServiceTarget) ApplyToolGateway(gateway workflow.ToolRuntime) {
	s.gateway = gateway
}

func (s *stubWorkflowRuntimeServiceTarget) ApplyMetricsRecorder(recorder workflowRuntimeMetricsRecorder) {
	s.metrics = recorder
}

type stubWorkflowRuntimeHookTarget struct {
	hook func(*workflow.WorkflowService)
}

func (s *stubWorkflowRuntimeHookTarget) SetServiceInitHook(hook func(*workflow.WorkflowService)) {
	if hook == nil {
		return
	}
	if s.hook == nil {
		s.hook = hook
		return
	}
	prev := s.hook
	s.hook = func(svc *workflow.WorkflowService) {
		prev(svc)
		hook(svc)
	}
}

// Test that ask-user-question is never affected by auto-confirm mode
func TestNewRuntimeAskPolicyBinding_QuestionManagerNeverGetsSilentFunc(t *testing.T) {
	settings := &stubRuntimeAskPolicySettingsSource{
		autoConfirm:    true, // auto-confirm is enabled
		askTimeoutSecs: 60,
		timeoutAction:  "default",
	}
	binding := newRuntimeAskPolicyBinding(settings)

	questionTarget := &stubQuestionRuntimePolicyTarget{}
	binding.applyQuestionManager(questionTarget)

	// Key assertion: silentFunc should be nil even when auto-confirm is enabled
	// This ensures ask-user-question always requires user interaction
	if questionTarget.silentFunc != nil {
		t.Fatalf("questionTarget.silentFunc should be nil even with auto-confirm=true; ask-user-question must never auto-answer")
	}

	// Timeout settings should still be applied
	if questionTarget.timeoutFunc == nil {
		t.Fatalf("expected timeoutFunc to be set")
	}
	if questionTarget.timeoutActionFunc == nil {
		t.Fatalf("expected timeoutActionFunc to be set")
	}
}

// Test that exec auto-confirm and agent settings are still wired correctly
func TestNewRuntimeAskPolicyBinding_AgentSettingsStillWork(t *testing.T) {
	settings := &stubRuntimeAskPolicySettingsSource{
		autoConfirm:    true,
		askTimeoutSecs: 30,
		timeoutAction:  "error",
		maxToolRounds:  5,
		autoReflect:    true,
	}
	binding := newRuntimeAskPolicyBinding(settings)

	agentTarget := &stubAgentRuntimePolicyTarget{}
	binding.applyAgent(agentTarget)

	// Agent should still get all settings
	if agentTarget.askTimeoutFunc == nil {
		t.Fatalf("expected agent askTimeoutFunc to be set")
	}
	if agentTarget.askTimeoutActionFunc == nil {
		t.Fatalf("expected agent askTimeoutActionFunc to be set")
	}
	if agentTarget.maxToolRoundsFunc == nil {
		t.Fatalf("expected agent maxToolRoundsFunc to be set")
	}
	if agentTarget.autoReflectFunc == nil {
		t.Fatalf("expected agent autoReflectFunc to be set")
	}

	// Verify the values
	if agentTarget.askTimeoutFunc() != 30*time.Second {
		t.Fatalf("askTimeout = %v, want 30s", agentTarget.askTimeoutFunc())
	}
	if agentTarget.askTimeoutActionFunc() != "error" {
		t.Fatalf("askTimeoutAction = %q, want 'error'", agentTarget.askTimeoutActionFunc())
	}
	if agentTarget.maxToolRoundsFunc() != 5 {
		t.Fatalf("maxToolRounds = %d, want 5", agentTarget.maxToolRoundsFunc())
	}
	if !agentTarget.autoReflectFunc() {
		t.Fatalf("autoReflect = %v, want true", agentTarget.autoReflectFunc())
	}
}
