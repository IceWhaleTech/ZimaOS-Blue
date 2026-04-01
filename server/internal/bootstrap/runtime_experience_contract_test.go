package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestRuntimeExperienceContractGo_DelegatesChatSurfaceProductivityAndSkillLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_experience_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_experience_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_experience_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_experience_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_experience_contract.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_experience_contract_types.go to stay below 40 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindExperienceRuntime(",
		"func bindRouteRuntimeExperience(",
		"binding routeRuntimeExperienceBinding,",
		"func bindRouteRuntimeExperienceSurface(",
		"bindRouteRuntimeExperienceSurface(binding, options.surface)",
		"bindRouteRuntimeResearchSurface(binding, options.research)",
		"binding.ConfigureChatRuntime(",
		"binding.BindChatRuntime(",
		"binding.BindProductivityTools(",
		"binding.BindPlatformSurfaceRuntime(",
		"binding.BindSkillRuntime(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_experience_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeExperienceBinding interface {",
		"routeRuntimeResearchSurface",
		"var _ routeRuntimeExperienceBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractExperienceSurfaceOptions struct {",
		"type routeRuntimeContractExperienceOptions struct {",
		"type routeRuntimeContractExperienceResult struct {",
		"skillAutoReranker",
		"chat              routeRuntimeContractChatBindingResult",
		"productivity      routeRuntimeContractProductivityResult",
		"skill             routeRuntimeContractSkillResult",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_experience_contract_types.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractExperienceOptions struct {",
		"type routeRuntimeContractExperienceResult struct {",
		"binding *runtimeContractBinding,",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_experience_contract.go to delegate token %q", token)
		}
	}
}

func TestBindExperienceRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-experience.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")
	cfg.ToolCalling.SmartSelectionMaxTools = 5
	cfg.ToolCalling.SmartSkillSelection = true

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

	registry := tools.NewRegistry()
	skillRegistry := skillpkg.NewRegistry()
	broker := sse.NewBroker()
	defer broker.Close()
	chatHandler := &serverpkg.ChatHandler{}
	target := &stubChatResearchRuntimeTarget{}
	e := echo.New()
	closers := make([]interface{ Close() error }, 0, 1)

	result := contract.BindExperienceRuntime(routeRuntimeContractExperienceOptions{
		chat: routeRuntimeContractChatOptions{
			chat:          chatHandler,
			config:        cfg,
			dataDir:       tmp,
			workspaceDir:  tmp,
			flagEvaluator: config.NewFlagEvaluator(&config.GrayscaleConfig{}),
			logger:        zap.NewNop(),
		},
		chatBinding: routeRuntimeContractChatBindingOptions{
			target:  target,
			handler: chatHandler,
		},
		surface: routeRuntimeContractExperienceSurfaceOptions{
			research: routeRuntimeContractResearchOptions{
				target:       target,
				broker:       broker,
				registry:     registry,
				workspaceDir: tmp,
			},
		},
		productivity: routeRuntimeContractProductivityOptions{
			writeDB:       db,
			readDB:        db,
			registry:      registry,
			skillRegistry: skillRegistry,
			logger:        zap.NewNop(),
		},
		skill: routeRuntimeContractSkillOptions{
			writeDB:  db,
			readDB:   db,
			dataDir:  tmp,
			services: &Services{SkillRegistry: skillRegistry, ToolRegistry: registry},
			authPageV1Group: func(string) *echo.Group {
				return e.Group("/api/v1")
			},
			ctx:     context.Background(),
			logger:  zap.NewNop(),
			closers: &closers,
		},
	})

	if result.skillAutoReranker == nil {
		t.Fatalf("expected experience contract to configure chat reranker, got %#v", result)
	}
	if !result.chat.autoHarnessHookBound {
		t.Fatalf("expected experience contract to bind chat auto-harness hook, got %#v", result.chat)
	}
	if result.productivity.emailTool == nil || result.productivity.calendarTool == nil {
		t.Fatalf("expected experience contract to aggregate productivity tools, got %#v", result.productivity)
	}
	if !result.skill.storeBound || !result.skill.marketplaceConfigured || !result.skill.routesRegistered || !result.skill.closerRegistered {
		t.Fatalf("expected experience contract to aggregate skill runtime state, got %#v", result.skill)
	}
	if target.service == nil || target.hook == nil || target.serviceCalls != 1 || target.hookCalls != 1 {
		t.Fatalf("expected experience contract to bind service and hook onto chat target, got %#v", target)
	}
	if chatHandler.GetToolSelector() == nil || chatHandler.GetToolSelector().MaxTools != 5 {
		t.Fatalf("expected experience contract to configure chat tool selection, got %#v", chatHandler.GetToolSelector())
	}
	if registry.Get("deep_research") == nil {
		t.Fatalf("expected experience contract to register deep_research tool, got %#v", registry.List())
	}
	if len(closers) != 1 {
		t.Fatalf("expected experience contract to capture skill handler closer, got %d", len(closers))
	}
	if !routeExists(e, "GET", "/api/v1/skills") || !routeExists(e, "GET", "/api/v1/skill-store/sources") {
		t.Fatalf("expected experience contract to register skill routes, got %#v", e.Routes())
	}
}
