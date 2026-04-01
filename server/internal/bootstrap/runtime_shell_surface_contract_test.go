package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
)

type stubRuntimeWorkerStatsSource struct {
	stats worker.Stats
}

func (s *stubRuntimeWorkerStatsSource) Stats() worker.Stats {
	return s.stats
}

func TestRuntimeShellSurfaceContractGo_DelegatesHealthConfigAndRouteSlices(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_shell_surface_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_shell_surface_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_shell_surface_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_shell_surface_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	healthContent, err := os.ReadFile(filepath.Join("runtime_shell_surface_contract_health_static.go"))
	if err != nil {
		t.Fatalf("read runtime_shell_surface_contract_health_static.go: %v", err)
	}
	healthSource := string(healthContent)

	configContent, err := os.ReadFile(filepath.Join("runtime_shell_surface_contract_config_canvas.go"))
	if err != nil {
		t.Fatalf("read runtime_shell_surface_contract_config_canvas.go: %v", err)
	}
	configSource := string(configContent)

	routeContent, err := os.ReadFile(filepath.Join("runtime_shell_surface_contract_routes.go"))
	if err != nil {
		t.Fatalf("read runtime_shell_surface_contract_routes.go: %v", err)
	}
	routeSource := string(routeContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_shell_surface_contract.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_shell_surface_contract_types.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(healthSource, "\n") + 1; lines > 85 {
		t.Fatalf("expected runtime_shell_surface_contract_health_static.go to stay below 85 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(configSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_shell_surface_contract_config_canvas.go to stay below 60 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(routeSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_shell_surface_contract_routes.go to stay below 35 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindShellSurfaceRuntime(",
		"func bindRouteRuntimeShellSurfaces(",
		"registerRouteRuntimeHealthSurface(",
		"registerRouteRuntimeStaticSurface(",
		"registerRouteRuntimeConfigSurface(",
		"registerFormfillerRoutes(",
		"registerExternalAuthRoutes(",
		"registerRouteRuntimeCanvasSurface(",
		"registerRouteRuntimeMFASurface(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_shell_surface_contract.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type routeRuntimeContractShellSurfaceOptions struct {") {
		t.Fatal("expected runtime_shell_surface_contract_types.go to keep shell surface option type")
	}
	if !strings.Contains(healthSource, "func registerRouteRuntimeHealthSurface(") {
		t.Fatal("expected runtime_shell_surface_contract_health_static.go to keep health surface registration")
	}
	if !strings.Contains(configSource, "func registerRouteRuntimeConfigSurface(") {
		t.Fatal("expected runtime_shell_surface_contract_config_canvas.go to keep config surface registration")
	}
	if !strings.Contains(routeSource, "func registerFormfillerRoutes(") {
		t.Fatal("expected runtime_shell_surface_contract_routes.go to keep formfiller routes")
	}

	forbiddenContract := []string{
		"type routeRuntimeContractShellSurfaceOptions struct {",
		"func registerRouteRuntimeHealthSurface(",
		"func registerRouteRuntimeStaticSurface(",
		"func registerRouteRuntimeConfigSurface(",
		"func registerRouteRuntimeCanvasSurface(",
		"func registerRouteRuntimeMFASurface(",
		"type runtimeWorkerStatsSource interface {",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_shell_surface_contract.go to delegate token %q", token)
		}
	}
}

func TestRegisterRouteRuntimeHealthSurface_UsesWorkerStatsAndDefaults(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	registerRouteRuntimeHealthSurface(v1, &ServerConfig{Version: "1.2.3"}, &stubRuntimeWorkerStatsSource{
		stats: worker.Stats{PoolSize: 3},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workers/stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload worker.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode workers stats: %v", err)
	}
	if payload.PoolSize != 3 {
		t.Fatalf("pool_size=%d, want 3", payload.PoolSize)
	}

	e2 := echo.New()
	v12 := e2.Group("/api/v1")
	registerRouteRuntimeHealthSurface(v12, nil, nil, nil)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/workers/stats", nil)
	rec = httptest.NewRecorder()
	e2.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("nil worker pool status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	payload = worker.Stats{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode nil workers stats: %v", err)
	}
	if payload != (worker.Stats{}) {
		t.Fatalf("nil worker pool payload=%+v, want zero-value stats", payload)
	}
}

func TestRegisterRouteRuntimeConfigSurface_RegistersConfigAndTemplatesRoutes(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := e.Group("/api/v1")
	registerRouteRuntimeConfigSurface(func(page string) *echo.Group {
		if page != permission.PageSettings {
			t.Fatalf("page=%q, want %q", page, permission.PageSettings)
		}
		return protected
	}, v1, nil, nil)

	for _, path := range []string{"/api/v1/config/status", "/api/v1/templates"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d, want 200, body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
