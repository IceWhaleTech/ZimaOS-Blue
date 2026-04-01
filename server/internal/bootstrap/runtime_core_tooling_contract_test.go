package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeCoreToolingContractGo_DelegatesSchedulerToolingAnalyzeAndProviderLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_core_tooling_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_core_tooling_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_core_tooling_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_core_tooling_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_core_tooling_contract.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 28 {
		t.Fatalf("expected runtime_core_tooling_contract_types.go to stay below 28 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindCoreToolingRuntime(",
		"func bindRouteRuntimeCoreToolingContract(",
		"binding routeRuntimeCoreToolingBinding,",
		"binding.BindSchedulerServices(",
		"binding.BindTooling(",
		"binding.BindAnalyzeTool(",
		"binding.BindProviderPoolRuntime(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_core_tooling_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeCoreToolingBinding interface {",
		"var _ routeRuntimeCoreToolingBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractCoreToolingOptions struct {",
		"type routeRuntimeContractCoreToolingResult struct {",
		"schedulerBound bool",
		"tooling        routeRuntimeContractToolingResult",
		"analyzeTool    *tools.AnalyzeTool",
		"provider       routeRuntimeContractProviderPoolResult",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_core_tooling_contract_types.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractCoreToolingOptions struct {",
		"type routeRuntimeContractCoreToolingResult struct {",
		"binding *runtimeContractBinding,",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_core_tooling_contract.go to delegate token %q", token)
		}
	}
}

func TestBindCoreToolingRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	cfg := &config.Config{}
	contract := newRuntimeContractBinding(nil, nil, cfg, zap.NewNop(), nil, buildWebSearchConfig(cfg))
	if contract == nil {
		t.Fatal("expected runtime contract binding")
	}

	e := echo.New()
	registry := tools.NewRegistry()
	cronHandler := &stubRuntimeCronHandlerTarget{
		service: cron.NewService(cron.DefaultConfig(), zap.NewNop()),
	}
	scheduler := &stubRuntimeSchedulerSkillTarget{}
	broker := sse.NewBroker()
	defer broker.Close()
	voiceSource := &stubRuntimeVoiceServiceSource{}

	result := contract.BindCoreToolingRuntime(routeRuntimeContractCoreToolingOptions{
		scheduler: routeRuntimeContractSchedulerOptions{
			registry:    registry,
			cronHandler: cronHandler,
			scheduler:   scheduler,
			broker:      broker,
			logger:      zap.NewNop(),
		},
		tooling: routeRuntimeContractToolingOptions{
			registry:              registry,
			mediaDir:              t.TempDir(),
			browserBackend:        &stubRuntimeBrowserBackend{},
			lazyBrowser:           func() *browser.RodService { return nil },
			pushService:           &push.Service{},
			workspaceAllowedPaths: []string{t.TempDir()},
			mediaManager:          &mediagen.Manager{},
			mediaStorage:          &mediagen.MediaStorage{},
			ocr:                   &ocrruntime.TesseractService{},
			voiceSource:           voiceSource,
		},
		analyze: routeRuntimeContractAnalyzeOptions{
			registry:        registry,
			mediaDir:        t.TempDir(),
			browserBackend:  &stubRuntimeBrowserBackend{},
			webSearchConfig: tools.WebSearchConfig{Provider: "duckduckgo"},
		},
		provider: routeRuntimeContractProviderPoolOptions{
			protected: e.Group("/api"),
		},
	})

	if !result.schedulerBound {
		t.Fatalf("expected core tooling contract to bind scheduler lane, got %#v", result)
	}
	if result.tooling.uiReviewerTool == nil {
		t.Fatalf("expected core tooling contract to aggregate tooling lane, got %#v", result.tooling)
	}
	if result.analyzeTool == nil || registry.Get("analyze") != result.analyzeTool {
		t.Fatalf("expected core tooling contract to aggregate analyze lane, tool=%#v registered=%#v", result.analyzeTool, registry.Get("analyze"))
	}
	if result.provider.oauthManager != nil {
		t.Fatalf("expected core tooling contract to keep provider oauth nil without provider pool, got %#v", result.provider)
	}
	if scheduler.service == nil || scheduler.calls != 1 {
		t.Fatalf("expected core tooling contract to wire scheduler skill, got %#v", scheduler)
	}
	if len(cronHandler.initHooks) != 1 {
		t.Fatalf("expected core tooling contract to install one cron init hook, got %#v", cronHandler.initHooks)
	}
	if voiceSource.calls != 1 {
		t.Fatalf("expected core tooling contract to resolve voice source once, got %#v", voiceSource)
	}
	if !routeExists(e, "GET", "/api/providers") {
		t.Fatalf("expected core tooling contract to register provider-unavailable routes, got %#v", e.Routes())
	}
}
