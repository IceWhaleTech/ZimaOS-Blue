package bootstrap

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRuntimeProviderPoolContractGo_DelegatesHandlerAndOAuthSlices(t *testing.T) {
	contractContent, err := os.ReadFile(filepath.Join("runtime_provider_pool_contract.go"))
	if err != nil {
		t.Fatalf("read runtime_provider_pool_contract.go: %v", err)
	}
	contractSource := string(contractContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_provider_pool_contract_types.go"))
	if err != nil {
		t.Fatalf("read runtime_provider_pool_contract_types.go: %v", err)
	}
	typeSource := string(typeContent)

	handlerContent, err := os.ReadFile(filepath.Join("runtime_provider_pool_contract_handler.go"))
	if err != nil {
		t.Fatalf("read runtime_provider_pool_contract_handler.go: %v", err)
	}
	handlerSource := string(handlerContent)

	oauthContent, err := os.ReadFile(filepath.Join("runtime_provider_pool_contract_oauth.go"))
	if err != nil {
		t.Fatalf("read runtime_provider_pool_contract_oauth.go: %v", err)
	}
	oauthSource := string(oauthContent)

	authHelperContent, err := os.ReadFile(filepath.Join("runtime_route_auth_helpers.go"))
	if err != nil {
		t.Fatalf("read runtime_route_auth_helpers.go: %v", err)
	}
	authHelperSource := string(authHelperContent)

	if lines := strings.Count(contractSource, "\n") + 1; lines > 35 {
		t.Fatalf("expected runtime_provider_pool_contract.go to stay below 35 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(typeSource, "\n") + 1; lines > 30 {
		t.Fatalf("expected runtime_provider_pool_contract_types.go to stay below 30 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(handlerSource, "\n") + 1; lines > 80 {
		t.Fatalf("expected runtime_provider_pool_contract_handler.go to stay below 80 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(oauthSource, "\n") + 1; lines > 90 {
		t.Fatalf("expected runtime_provider_pool_contract_oauth.go to stay below 90 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(authHelperSource, "\n") + 1; lines > 25 {
		t.Fatalf("expected runtime_route_auth_helpers.go to stay below 25 lines after extraction, got %d", lines)
	}

	requiredContract := []string{
		"func (binding *runtimeContractBinding) BindProviderPoolRuntime(",
		"func bindRouteRuntimeProviderPool(",
		"newRouteRuntimeProviderPoolHandler(",
		"bindRouteRuntimeProviderPoolOAuth(",
		"registerRouteRuntimeProviderPoolRoutes(",
		"startRouteRuntimeProviderPool(",
	}
	for _, token := range requiredContract {
		if !strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_provider_pool_contract.go to contain token %q", token)
		}
	}

	if !strings.Contains(typeSource, "type routeRuntimeContractProviderPoolOptions struct {") {
		t.Fatal("expected runtime_provider_pool_contract_types.go to keep provider pool option type")
	}
	if !strings.Contains(handlerSource, "func newRouteRuntimeProviderPoolHandler(") {
		t.Fatal("expected runtime_provider_pool_contract_handler.go to keep provider pool handler wiring")
	}
	if !strings.Contains(oauthSource, "func bindRouteRuntimeProviderPoolOAuth(") {
		t.Fatal("expected runtime_provider_pool_contract_oauth.go to keep provider pool oauth wiring")
	}
	if !strings.Contains(authHelperSource, "func routeRuntimePageMiddleware(") {
		t.Fatal("expected runtime_route_auth_helpers.go to contain routeRuntimePageMiddleware")
	}

	forbiddenContract := []string{
		"type routeRuntimeContractProviderPoolOptions struct {",
		"func newRouteRuntimeOAuthStore(",
		"func routeRuntimePageMiddleware(",
	}
	for _, token := range forbiddenContract {
		if strings.Contains(contractSource, token) {
			t.Fatalf("expected runtime_provider_pool_contract.go to delegate token %q", token)
		}
	}
}

func TestRouteRuntimePageMiddleware_UsesResolverWhenPresent(t *testing.T) {
	if routeRuntimePageMiddleware(nil, "providers") != nil {
		t.Fatal("expected nil resolver to return nil middleware")
	}

	middleware := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	got := routeRuntimePageMiddleware(func(page string) echo.MiddlewareFunc {
		if page != "providers" {
			t.Fatalf("page=%q, want providers", page)
		}
		return middleware
	}, "providers")
	if got == nil {
		t.Fatal("expected middleware from resolver")
	}
}

func TestNewRouteRuntimeOAuthStore_UsesWriteDBWhenReadDBMissing(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "oauth-store.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store, err := newRouteRuntimeOAuthStore(db, nil)
	if err != nil {
		t.Fatalf("newRouteRuntimeOAuthStore: %v", err)
	}
	if store == nil {
		t.Fatal("expected oauth store")
	}
}
