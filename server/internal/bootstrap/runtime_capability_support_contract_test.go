package bootstrap

import (
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestBindRouteRuntimeCapabilitySupport_RegistersSupportRoutesThroughBoundary(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")
	protected := v1.Group("")
	apiProtected := e.Group("/api")

	result := (&runtimeContractBinding{}).BindCapabilitySupportRuntime(routeRuntimeContractCapabilitySupportOptions{
		e:            e,
		v1:           v1,
		protected:    protected,
		apiProtected: apiProtected,
		dataDir:      t.TempDir(),
		deps: &RoutesDeps{
			Config: &config.Config{},
		},
		askSupport: runtimeAskSupportBundle{
			QuestionManager: tools.NewQuestionManager(nil, nil, time.Minute),
		},
		execSupport: runtimeExecSupportBundle{
			Approvals: tools.NewApprovalManager(nil),
		},
		logger: zap.NewNop(),
	})

	if result.convertRegistered {
		t.Fatalf("expected convert routes to stay disabled without convert handler, got %#v", result)
	}

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{method: "GET", path: "/api/v1/ask-user-question/pending"},
		{method: "GET", path: "/api/v1/exec/approvals/pending"},
		{method: "GET", path: "/api/v1/backup"},
		{method: "GET", path: "/api/v1/security/sessions"},
		{method: "GET", path: "/api/v1/sandbox/info"},
		{method: "GET", path: "/api/cron"},
		{method: "GET", path: "/api/browser/tasks"},
		{method: "GET", path: "/api/v1/workflows"},
		{method: "GET", path: "/api/v1/voice/voices"},
		{method: "GET", path: "/api/v1/speech/status"},
	}
	for _, route := range expectedRoutes {
		if !routeExists(e, route.method, route.path) {
			t.Fatalf("expected %s %s to be registered through capability support boundary, got %#v", route.method, route.path, e.Routes())
		}
	}
}
