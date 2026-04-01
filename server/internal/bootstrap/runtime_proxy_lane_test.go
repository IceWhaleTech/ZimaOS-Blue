package bootstrap

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func TestActivateRuntimeProxyLane_RegistersRuntimeSurfaces(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", tmp+"/proxy-lane.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api")
	v1 := e.Group("/api/v1")
	restrictions := e.Group("/api/v1")
	closers := make([]interface{ Close() error }, 0, 1)

	bundle := activateRuntimeProxyLane(runtimeProxyLaneOptions{
		e:                e,
		v1:               v1,
		protected:        protected,
		restrictionGroup: restrictions,
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
		connectionConfig: &proxy.ConnectionConfig{},
		dataMasker:       newRuntimeProxyDataMasker(),
		kv:               kvstore.NewMemoryStore(),
		dataDir:          tmp,
		fallbackWriteDB:  db,
		fallbackReadDB:   db,
		closers:          &closers,
	})
	if bundle == nil || bundle.handler == nil || bundle.prunerRuntime == nil || bundle.surface == nil {
		t.Fatalf("expected runtime proxy lane bundle, got %#v", bundle)
	}
	if bundle.pipelineStats == nil || bundle.handler.GetPipelineStats() != bundle.pipelineStats {
		t.Fatalf("expected pipeline stats to be wired, got bundle=%#v handler=%#v", bundle.pipelineStats, bundle.handler.GetPipelineStats())
	}
	if len(closers) != 1 || closers[0] != bundle.pipelineStats {
		t.Fatalf("expected pipeline stats closer captured, got %#v", closers)
	}
	if !routeExists(e, http.MethodGet, "/api/proxy/failover/config") {
		t.Fatalf("expected failover routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/v1/chat/completions") {
		t.Fatalf("expected gateway routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/pruner/config") {
		t.Fatalf("expected pruner routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/proxy/routing/config") {
		t.Fatalf("expected control routes, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodGet, "/api/v1/restrictions/providers/:provider") {
		t.Fatalf("expected restriction routes, got %#v", e.Routes())
	}
}

func TestActivateRuntimeProxyLane_ReturnsNilWithoutRoutingConfig(t *testing.T) {
	if bundle := activateRuntimeProxyLane(runtimeProxyLaneOptions{}); bundle != nil {
		t.Fatalf("expected nil bundle without routing config, got %#v", bundle)
	}
}
