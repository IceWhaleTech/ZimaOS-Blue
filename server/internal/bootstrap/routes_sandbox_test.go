package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func newSandboxRoutesTestHarness(t *testing.T) (*echo.Echo, *auth.JWTService) {
	t.Helper()

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})

	e := echo.New()
	protected := e.Group("/api/v1")
	protected.Use(auth.NewAuthMiddleware(jwtSvc, nil).Authenticate())
	registerSandboxRoutes(protected, nil, &RoutesDeps{})

	return e, jwtSvc
}

func mustSandboxAccessToken(t *testing.T, jwtSvc *auth.JWTService) string {
	t.Helper()

	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   "sandbox-test-user",
		Username: "sandbox",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return token
}

func TestRegisterSandboxRoutes_DisabledRegistersConfigStub(t *testing.T) {
	e, jwtSvc := newSandboxRoutesTestHarness(t)

	type routeKey struct {
		method string
		path   string
	}

	routes := make([]routeKey, 0, len(e.Routes()))
	for _, route := range e.Routes() {
		routes = append(routes, routeKey{method: route.Method, path: route.Path})
	}

	want := routeKey{method: http.MethodPatch, path: "/api/v1/sandbox/config"}
	if !slices.Contains(routes, want) {
		t.Fatalf("expected sandbox config stub route %s %s to be registered; got routes=%v", want.method, want.path, routes)
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sandbox/config", strings.NewReader(`{"network_enabled":true}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+mustSandboxAccessToken(t, jwtSvc))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/sandbox/config status=%d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["feature"] != "sandbox" {
		t.Fatalf("feature=%v, want sandbox", payload["feature"])
	}
	if payload["status"] != "not_initialized" {
		t.Fatalf("status=%v, want not_initialized", payload["status"])
	}
}
