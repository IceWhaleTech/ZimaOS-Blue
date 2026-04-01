package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRuntimeUserSurfaceContractGo_DelegatesCompanionWorkspaceAndMySlices(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_user_surface_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_user_surface_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_user_surface_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_user_surface_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	companionContent, err := os.ReadFile(filepath.Join("runtime_user_surface_contract_companion.go"))
	if err != nil {
		t.Fatalf("read runtime_user_surface_contract_companion.go: %v", err)
	}
	companionSource := string(companionContent)

	workspaceContent, err := os.ReadFile(filepath.Join("runtime_user_surface_contract_workspace.go"))
	if err != nil {
		t.Fatalf("read runtime_user_surface_contract_workspace.go: %v", err)
	}
	workspaceSource := string(workspaceContent)

	myContent, err := os.ReadFile(filepath.Join("runtime_user_surface_contract_my.go"))
	if err != nil {
		t.Fatalf("read runtime_user_surface_contract_my.go: %v", err)
	}
	mySource := string(myContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_user_surface_contract.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 40 {
		t.Fatalf("expected runtime_user_surface_contract_types.go to stay below 40 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(companionSource, "\n") + 1; lines > 45 {
		t.Fatalf("expected runtime_user_surface_contract_companion.go to stay below 45 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(workspaceSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_user_surface_contract_workspace.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(mySource, "\n") + 1; lines > 115 {
		t.Fatalf("expected runtime_user_surface_contract_my.go to stay below 115 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindUserSurfaceRuntime(",
		"func bindRouteRuntimeUserSurface(",
		"registerRouteRuntimeCompanionRoutes(",
		"registerRouteRuntimeWorkspaceRoutes(",
		"registerRouteRuntimeMyRoutes(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_user_surface_contract.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type routeRuntimeContractUserSurfaceOptions struct {") {
		t.Fatal("expected runtime_user_surface_contract_types.go to keep user surface option type")
	}
	if !strings.Contains(companionSource, "func registerRouteRuntimeCompanionRoutes(") {
		t.Fatal("expected runtime_user_surface_contract_companion.go to keep companion route registration")
	}
	if !strings.Contains(workspaceSource, "func registerRouteRuntimeWorkspaceRoutes(") {
		t.Fatal("expected runtime_user_surface_contract_workspace.go to keep workspace route registration")
	}
	if !strings.Contains(mySource, "func registerRouteRuntimeMyRoutes(") {
		t.Fatal("expected runtime_user_surface_contract_my.go to keep my route registration")
	}

	forbiddenContract := []string{
		"type routeRuntimeContractUserSurfaceOptions struct {",
		"func routeRuntimeAuthGroup(",
		"func registerRouteRuntimeCompanionRoutes(",
		"func registerRouteRuntimeWorkspaceRoutes(",
		"func registerRouteRuntimeUserScopedRoutes(",
		"func registerRouteRuntimeMyUsageRoute(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_user_surface_contract.go to delegate token %q", token)
		}
	}
}

func TestRouteRuntimeAuthGroup_UsesResolverWhenPresent(t *testing.T) {
	if routeRuntimeAuthGroup(nil, "security") != nil {
		t.Fatal("expected nil resolver to return nil group")
	}

	e := echo.New()
	expected := e.Group("/api/v1/security")
	got := routeRuntimeAuthGroup(func(page string) *echo.Group {
		if page != "security" {
			t.Fatalf("page=%q, want security", page)
		}
		return expected
	}, "security")
	if got != expected {
		t.Fatalf("group=%p, want %p", got, expected)
	}
}

func TestRegisterRouteRuntimeWorkspaceRoutes_UsesHandlerAndSkipsWhenNil(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		protected := e.Group("/api/v1")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "workspace")
				})
			},
		}

		registerRouteRuntimeWorkspaceRoutes(protected, nil, handler, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("nil handler", func(t *testing.T) {
		e := echo.New()
		protected := e.Group("/api/v1")

		registerRouteRuntimeWorkspaceRoutes(protected, nil, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d, want 404, body=%s", rec.Code, rec.Body.String())
		}
	})
}
