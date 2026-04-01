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

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestBindRouteRuntimeSkills_ReturnsWorkspaceSkillsDirWithoutServices(t *testing.T) {
	dataDir := t.TempDir()
	result := bindRouteRuntimeSkills(routeRuntimeContractSkillOptions{
		dataDir: dataDir,
		ctx:     context.Background(),
	})

	want := filepath.Join(dataDir, "workspace", ".claude", "skills")
	if result.skillsDir != want {
		t.Fatalf("skillsDir=%q, want %q", result.skillsDir, want)
	}
}

func TestRuntimeSkillContractGo_DelegatesTypesStoreMarketplaceAndSupportLanes(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_skill_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_skill_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	storeContent, err := os.ReadFile(filepath.Join("runtime_skill_contract_store.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_contract_store.go: %v", err)
	}
	storeSource := string(storeContent)

	marketContent, err := os.ReadFile(filepath.Join("runtime_skill_contract_marketplace.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_contract_marketplace.go: %v", err)
	}
	marketSource := string(marketContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_skill_contract_support.go"))
	if err != nil {
		t.Fatalf("read runtime_skill_contract_support.go: %v", err)
	}
	supportSource := string(supportContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_skill_contract.go to stay below 80 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 50 {
		t.Fatalf("expected runtime_skill_contract_types.go to stay below 50 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(storeSource, "\n") + 1; lines > 50 {
		t.Fatalf("expected runtime_skill_contract_store.go to stay below 50 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(marketSource, "\n") + 1; lines > 130 {
		t.Fatalf("expected runtime_skill_contract_marketplace.go to stay below 130 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 90 {
		t.Fatalf("expected runtime_skill_contract_support.go to stay below 90 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindSkillRuntime(",
		"func bindRouteRuntimeSkills(",
		"bindRouteRuntimeSkillStore(",
		"bindRouteRuntimeSkillMarketplace(",
		"bindRouteRuntimeSkillSupport(",
		"bindRouteRuntimeSkillRoutes(",
		"bindRouteRuntimeSkillIPC(",
		"releaseRouteRuntimeEmbeddedSkills(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_skill_contract.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type routeRuntimeContractSkillOptions struct {",
		"type routeRuntimeContractSkillResult struct {",
		"storeBound",
		"marketplaceConfigured",
		"ipcHandlersRegistered",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_skill_contract_types.go to contain token %q", token)
		}
	}

	if !strings.Contains(storeSource, "func bindRouteRuntimeSkillStore(") {
		t.Fatalf("expected runtime_skill_contract_store.go to contain bindRouteRuntimeSkillStore")
	}
	if !strings.Contains(marketSource, "func bindRouteRuntimeSkillMarketplace(") {
		t.Fatalf("expected runtime_skill_contract_marketplace.go to contain bindRouteRuntimeSkillMarketplace")
	}
	for _, token := range []string{
		"func bindRouteRuntimeSkillSupport(",
		"func bindRouteRuntimeSkillRoutes(",
		"func bindRouteRuntimeSkillIPC(",
		"func releaseRouteRuntimeEmbeddedSkills(",
	} {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_skill_contract_support.go to contain token %q", token)
		}
	}

	forbiddenContract := []string{
		"type routeRuntimeContractSkillOptions struct {",
		"func bindRouteRuntimeSkillStore(",
		"func bindRouteRuntimeSkillMarketplace(",
		"func bindRouteRuntimeSkillSupport(",
		"func bindRouteRuntimeSkillRoutes(",
		"func bindRouteRuntimeSkillIPC(",
		"func releaseRouteRuntimeEmbeddedSkills(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_skill_contract.go to delegate token %q", token)
		}
	}

	forbiddenTypes := []string{
		"func bindRouteRuntimeSkillStore(",
		"func bindRouteRuntimeSkillMarketplace(",
		"func bindRouteRuntimeSkillSupport(",
	}
	for _, token := range forbiddenTypes {
		if strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_skill_contract_types.go to delegate token %q", token)
		}
	}

	if strings.Contains(storeSource, "func bindRouteRuntimeSkillMarketplace(") {
		t.Fatalf("expected runtime_skill_contract_store.go to delegate marketplace binding")
	}
	if strings.Contains(marketSource, "func bindRouteRuntimeSkillStore(") {
		t.Fatalf("expected runtime_skill_contract_marketplace.go to delegate store binding")
	}
	if strings.Contains(supportSource, "func bindRouteRuntimeSkillStore(") {
		t.Fatalf("expected runtime_skill_contract_support.go to delegate store binding")
	}
	if strings.Contains(supportSource, "func bindRouteRuntimeSkillMarketplace(") {
		t.Fatalf("expected runtime_skill_contract_support.go to delegate marketplace binding")
	}
}

func TestBindRouteRuntimeSkills_ReturnsFirstClassSkillContractState(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "skills.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := echo.New()
	closers := make([]interface{ Close() error }, 0, 1)
	result := bindRouteRuntimeSkills(routeRuntimeContractSkillOptions{
		writeDB:   db,
		readDB:    db,
		dataDir:   tmp,
		services:  &Services{SkillRegistry: skillpkg.NewRegistry(), ToolRegistry: tools.NewRegistry()},
		closers:   &closers,
		ipcServer: sockipc.NewServer(filepath.Join(tmp, "skills.sock"), zap.NewNop()),
		ctx:       ctx,
		logger:    zap.NewNop(),
		authPageV1Group: func(string) *echo.Group {
			return e.Group("/api/v1")
		},
	})

	want := filepath.Join(tmp, "workspace", ".claude", "skills")
	if result.skillsDir != want {
		t.Fatalf("skillsDir=%q, want %q", result.skillsDir, want)
	}
	if !result.storeBound || !result.featuredLoaderConfigured || !result.localScannerConfigured {
		t.Fatalf("expected skill contract to configure store/featured/local scanner state, got %#v", result)
	}
	if !result.marketplaceConfigured || !result.routesRegistered || !result.ipcHandlersRegistered || !result.closerRegistered {
		t.Fatalf("expected skill contract to report marketplace/routes/ipc/closer state, got %#v", result)
	}
	if len(closers) != 1 {
		t.Fatalf("expected skill handler closer to be captured, got %d", len(closers))
	}
	if !routeExists(e, "GET", "/api/v1/skills") || !routeExists(e, "GET", "/api/v1/skill-store/sources") {
		t.Fatalf("expected skill routes to register, got %#v", e.Routes())
	}
}
