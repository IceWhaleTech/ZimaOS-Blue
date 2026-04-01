package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func TestNewRuntimeProxySurfaceBundle_ConfiguresHandlerAndFailoverRoutes(t *testing.T) {
	e := echo.New()
	protected := e.Group("/api")
	routingConfig := &proxy.RouteConfig{
		LoadBalancing: "priority",
		Providers: []*proxy.ProviderConfig{
			{
				Name:     "primary",
				Enabled:  true,
				Priority: 1,
			},
		},
		Failover: proxy.DefaultProxyConfig().Routing.Failover,
	}

	bundle := newRuntimeProxySurfaceBundle(runtimeProxySurfaceOptions{
		protected:     protected,
		failoverGuard: nil,
		routingConfig: routingConfig,
		dataMasker:    newRuntimeProxyDataMasker(),
	})
	if bundle == nil || bundle.handler == nil || bundle.connPool == nil || bundle.smartFailover == nil || bundle.failoverAPIHandler == nil {
		t.Fatalf("expected proxy surface bundle, got %#v", bundle)
	}
	if !bundle.handler.IsPromptCacheEnabled() {
		t.Fatal("expected prompt cache to default to enabled")
	}
	if !routeExists(e, http.MethodGet, "/api/proxy/failover/config") {
		t.Fatalf("expected failover routes to be registered, got %#v", e.Routes())
	}
}

func TestRegisterRuntimeProxyGatewayAndRestrictionRoutes_ExposeSurfaces(t *testing.T) {
	e := echo.New()
	apiV1 := e.Group("/api/v1")
	bundle := newRuntimeProxySurfaceBundle(runtimeProxySurfaceOptions{
		routingConfig: &proxy.RouteConfig{
			LoadBalancing: "priority",
			Providers: []*proxy.ProviderConfig{
				{
					Name:     "primary",
					Enabled:  true,
					Priority: 1,
				},
			},
			Failover: proxy.DefaultProxyConfig().Routing.Failover,
		},
		dataMasker: newRuntimeProxyDataMasker(),
	})
	if bundle == nil {
		t.Fatal("expected proxy surface bundle")
	}

	if !registerRuntimeProxyGatewayRoutes(e, bundle.handler) {
		t.Fatal("expected proxy gateway routes to register")
	}
	if !registerRuntimeProxyRestrictionRoutes(apiV1, bundle.handler) {
		t.Fatal("expected restriction routes to register")
	}
	if !routeExists(e, http.MethodGet, "/v1/chat/completions") {
		t.Fatalf("expected proxy gateway route, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/restrictions/providers/:provider") {
		t.Fatalf("expected restrictions route, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/restrictions/providers/test-provider", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET restrictions status=%d body=%s", rec.Code, rec.Body.String())
	}
}
