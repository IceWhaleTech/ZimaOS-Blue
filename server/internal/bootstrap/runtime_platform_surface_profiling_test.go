package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestRegisterRouteRuntimeProfilingSurface_RegistersHeapProfileEndpoint(t *testing.T) {
	e := echo.New()
	cfg := &config.Config{}
	cfg.Performance.Profiling.PprofEnabled = true
	cfg.Performance.Profiling.PprofPath = "/debug/pprof"

	registerRouteRuntimeProfilingSurface(routeRuntimeContractPlatformSurfaceOptions{
		e:             e,
		authMiddleware: nil,
		appConfig:     cfg,
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /debug/pprof/heap status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(strings.ToLower(body), "<!doctype html") {
		t.Fatalf("GET /debug/pprof/heap returned SPA HTML instead of a profile payload: %q", body)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/octet-stream") && !strings.Contains(contentType, "text/plain") {
		t.Fatalf("GET /debug/pprof/heap content-type=%q, want profile payload", contentType)
	}
}
