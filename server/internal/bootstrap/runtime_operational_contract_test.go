package bootstrap

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func TestRuntimeOperationalContractGo_DelegatesSupportSnapshotHelper(t *testing.T) {
	mainContent, err := os.ReadFile(filepath.Join("runtime_operational_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_operational_contract.go: %v", err)
	}
	mainSource := string(mainContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_operational_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_operational_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	supportContent, err := os.ReadFile(filepath.Join("runtime_operational_contract_support.go"))
	if err != nil {
		t.Fatalf("read runtime_operational_contract_support.go: %v", err)
	}
	supportSource := string(supportContent)

	applyContent, err := os.ReadFile(filepath.Join("runtime_operational_contract_support_apply.go"))
	if err != nil {
		t.Fatalf("read runtime_operational_contract_support_apply.go: %v", err)
	}
	applySource := string(applyContent)

	if lines := strings.Count(mainSource, "\n") + 1; lines > 70 {
		t.Fatalf("expected runtime_operational_contract.go to stay below 70 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 55 {
		t.Fatalf("expected runtime_operational_contract_types.go to stay below 55 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(supportSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_operational_contract_support.go to stay below 20 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(applySource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_operational_contract_support_apply.go to stay below 45 lines after extraction, got %d", lines)
	}

	requiredMain := []string{
		"func (binding *runtimeContractBinding) BindOperationalRuntime(",
		"func bindRouteRuntimeOperational(",
		"binding routeRuntimeOperationalBinding,",
		"binding.RegisterTaskSurface(",
		"binding.ActivateRouteRuntime(",
		"bindRouteRuntimeOperationalSupport(",
	}
	for _, token := range requiredMain {
		if !strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_operational_contract.go to contain token %q", token)
		}
	}
	requiredTypes := []string{
		"type routeRuntimeOperationalBinding interface {",
		"var _ routeRuntimeOperationalBinding = (*runtimeContractBinding)(nil)",
		"type routeRuntimeContractHeartbeatOptions struct {",
		"type routeRuntimeContractOperationalOptions struct {",
		"type routeRuntimeContractOperationalSupportResult struct {",
		"type routeRuntimeContractOperationalResult struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_operational_contract_types.go to contain token %q", token)
		}
	}

	requiredSupport := []string{
		"func bindRouteRuntimeOperationalSupport(",
		"binding routeRuntimeOperationalBinding,",
		"applyRouteRuntimeOperationalSupport(",
		"applyRouteRuntimeOperationalDeferredSupport(",
	}
	for _, token := range requiredSupport {
		if !strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_operational_contract_support.go to contain token %q", token)
		}
	}

	requiredApply := []string{
		"func applyRouteRuntimeOperationalSupport(",
		"func applyRouteRuntimeOperationalDeferredSupport(",
		"binding.RegisterActivationSupportRoutes(",
		"binding.BindDeferredSupport(",
	}
	for _, token := range requiredApply {
		if !strings.Contains(applySource, token) {
			t.Fatalf("expected runtime_operational_contract_support_apply.go to contain token %q", token)
		}
	}

	forbiddenMain := []string{
		"type routeRuntimeContractOperationalSupportResult struct {",
		"type routeRuntimeContractOperationalResult struct {",
		"binding.RegisterActivationSupportRoutes(",
		"binding.BindDeferredSupport(",
	}
	for _, token := range forbiddenMain {
		if strings.Contains(mainSource, token) {
			t.Fatalf("expected runtime_operational_contract.go to delegate token %q", token)
		}
	}

	forbiddenSupport := []string{
		"*runtimeContractBinding",
	}
	for _, token := range forbiddenSupport {
		if strings.Contains(supportSource, token) {
			t.Fatalf("expected runtime_operational_contract_support.go to hide concrete binding token %q", token)
		}
	}

	forbiddenApply := []string{
		"*runtimeContractBinding",
	}
	for _, token := range forbiddenApply {
		if strings.Contains(applySource, token) {
			t.Fatalf("expected runtime_operational_contract_support_apply.go to hide concrete binding token %q", token)
		}
	}
}

func TestBindOperationalRuntime_ReturnsTaskSurfaceAndSupportSnapshot(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-operational.db"))
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

	e := echo.New()
	result := contract.BindOperationalRuntime(routeRuntimeContractOperationalOptions{
		taskSurface: runtimeTaskSurfaceOptions{
			protected:    e.Group("/api"),
			apiProtected: e.Group("/api/v1"),
			workspaceDir: tmp,
			logger:       zap.NewNop(),
		},
	})

	if !result.taskSurface.deepResearchRegistered || !result.taskSurface.harnessResearchRegistered || !result.taskSurface.harnessRoutesRegistered || !result.taskSurface.selfReflectRoutesApplied {
		t.Fatalf("expected operational runtime to preserve task surface snapshot, got %#v", result.taskSurface)
	}
	if !result.taskSurface.research.deepResearchRegistered || !result.taskSurface.research.harnessResearchRegistered {
		t.Fatalf("expected operational runtime to preserve first-class research lane inside task surface snapshot, got %#v", result.taskSurface.research)
	}
	if result.support.activationSupportApplied || result.support.deferredSupportApplied {
		t.Fatalf("expected minimal operational runtime to skip optional support wiring, got %#v", result.support)
	}
	if !routeExists(e, "POST", "/api/deep-research/jobs") || !routeExists(e, "GET", "/api/harness/runs") || !routeExists(e, "GET", "/api/self-reflect/proposals") {
		t.Fatalf("expected operational runtime task routes to register, got %#v", e.Routes())
	}
}
