package bootstrap

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestRuntimeCoreSupportContractGo_DelegatesAskExecAndCapabilityLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_core_support_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_core_support_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_core_support_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_core_support_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_core_support_contract.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 28 {
		t.Fatalf("expected runtime_core_support_contract_types.go to stay below 28 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindCoreSupportRuntime(",
		"func bindRouteRuntimeSupport(",
		"binding routeRuntimeCoreSupportBinding,",
		"binding.NewAskSupportBundle(",
		"binding.NewExecSupportBundle(",
		"binding.BindCapabilitySupportRuntime(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_core_support_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeCoreSupportBinding interface {",
		"var _ routeRuntimeCoreSupportBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractCoreSupportOptions struct {",
		"type routeRuntimeContractCoreSupportResult struct {",
		"ask        runtimeAskSupportBundle",
		"exec       runtimeExecSupportBundle",
		"capability routeRuntimeContractCapabilitySupportResult",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_core_support_contract_types.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractCoreSupportOptions struct {",
		"type routeRuntimeContractCoreSupportResult struct {",
		"binding *runtimeContractBinding,",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_core_support_contract.go to delegate token %q", token)
		}
	}
}

func TestBindCoreSupportRuntime_ReturnsAggregatedLaneState(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-core-support.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	memStore, err := memory.NewStoreWithDB(db)
	if err != nil {
		t.Fatalf("memory.NewStoreWithDB: %v", err)
	}

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
	chatTarget := &stubChatAskRuntimeTarget{}
	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api")
	apiProtected := e.Group("/api")
	closers := make([]interface{ Close() error }, 0, 1)

	result := contract.BindCoreSupportRuntime(routeRuntimeContractCoreSupportOptions{
		ask: routeRuntimeContractAskSupportOptions{
			writeDB:       db,
			readDB:        db,
			appConfig:     cfg,
			toolRegistry:  registry,
			skillRegistry: skillRegistry,
			broker:        broker,
			chatTarget:    chatTarget,
			mediaDir:      tmp,
		},
		exec: routeRuntimeContractExecSupportOptions{
			writeDB:              db,
			readDB:               db,
			dataDir:              tmp,
			workspaceDir:         tmp,
			workspaceAllowedPath: []string{tmp},
			memoryStore:          memStore,
			toolRegistry:         registry,
			skillRegistry:        skillRegistry,
			broker:               broker,
			logger:               zap.NewNop(),
			closers:              &closers,
			profileRoutes:        v1,
			sessionRoutes:        v1,
			oauthSource: func() agentsessions.OAuthCredentialSource {
				return nil
			},
			lookupAPIKey: func(string) (string, error) {
				return "", nil
			},
		},
		capability: routeRuntimeContractCapabilitySupportOptions{
			e:            e,
			v1:           v1,
			protected:    protected,
			apiProtected: apiProtected,
			dataDir:      tmp,
			deps: &RoutesDeps{
				Config: cfg,
			},
			logger: zap.NewNop(),
		},
	})

	if result.ask.QuestionManager == nil || result.ask.BrowserCheckpointManager == nil || result.ask.BrowserSiteStore == nil {
		t.Fatalf("expected core support contract to aggregate ask support, got %#v", result.ask)
	}
	if result.exec.Approvals == nil || result.exec.DirStore == nil || result.exec.AuditStore == nil || result.exec.ConvertHandler == nil {
		t.Fatalf("expected core support contract to aggregate exec support, got %#v", result.exec)
	}
	if !result.capability.convertRegistered {
		t.Fatalf("expected core support contract to aggregate capability support, got %#v", result.capability)
	}
	if chatTarget.toolObserver != contract.HarnessRuntime().RuntimeObserver || chatTarget.toolObserverCalls != 1 {
		t.Fatalf("expected core support contract to inherit shared harness observer, got %#v", chatTarget)
	}
	if len(closers) != 1 {
		t.Fatalf("expected core support contract to capture convert service closer, got %d", len(closers))
	}
	for _, route := range []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/api/v1/ask-user-question/pending"},
		{method: "GET", path: "/api/v1/exec/approvals/pending"},
		{method: "GET", path: "/api/convert/tasks"},
	} {
		if !routeExists(e, route.method, route.path) {
			t.Fatalf("expected %s %s to be registered through core support contract, got %#v", route.method, route.path, e.Routes())
		}
	}
}
