package bootstrap

import (
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func TestRuntimeCapabilityContractRegisterTaskSurface_RegistersResearchAndReflectRoutes(t *testing.T) {
	e := echo.New()
	protected := e.Group("/api")
	apiProtected := e.Group("/api/v1")

	registration := newTestRuntimeCapabilityContract(nil).registerTaskSurface(runtimeTaskSurfaceOptions{
		protected:    protected,
		apiProtected: apiProtected,
		workspaceDir: t.TempDir(),
		logger:       zap.NewNop(),
	})

	if !registration.deepResearchRegistered {
		t.Fatalf("expected deep research routes to register, got %#v", registration)
	}
	if !registration.knowledgeRoutesRegistered {
		t.Fatalf("expected knowledge routes to register, got %#v", registration)
	}
	if !registration.harnessResearchRegistered {
		t.Fatalf("expected harness research capability routes to register, got %#v", registration)
	}
	if registration.research.creatorBound {
		t.Fatalf("expected research lane creator binding to stay disabled without harness runtime, got %#v", registration.research)
	}
	if !registration.research.deepResearchRegistered || !registration.research.harnessResearchRegistered {
		t.Fatalf("expected first-class research registration to mirror route results, got %#v", registration.research)
	}
	if registration.harnessRoutesRegistered || registration.detailProvider != nil {
		t.Fatalf("expected harness routes to stay disabled without runtime bundle, got %#v", registration)
	}
	if !registration.selfReflectRoutesApplied {
		t.Fatalf("expected self-reflect routes to register, got %#v", registration)
	}
	if !routeExists(e, "POST", "/api/deep-research/jobs") || !routeExists(e, "POST", "/api/v1/deep-research/jobs") {
		t.Fatalf("expected deep research routes on protected/api groups, got %#v", e.Routes())
	}
	if !routeExists(e, "POST", "/api/knowledge/jobs") || !routeExists(e, "POST", "/api/v1/knowledge/jobs") {
		t.Fatalf("expected knowledge routes on protected/api groups, got %#v", e.Routes())
	}
	if !routeExists(e, "POST", "/api/harness/research/jobs") || !routeExists(e, "POST", "/api/v1/harness/research/jobs") {
		t.Fatalf("expected harness research capability routes on protected/api groups, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/api/self-reflect/proposals") || !routeExists(e, "GET", "/api/v1/self-reflect/proposals") {
		t.Fatalf("expected self-reflect routes on protected group, got %#v", e.Routes())
	}
}

func TestRuntimeCapabilityContractRegisterTaskSurface_RegistersHarnessRoutesWhenBundlePresent(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api")
	apiProtected := e.Group("/api/v1")

	registration := newTestRuntimeCapabilityContract(bundle).registerTaskSurface(runtimeTaskSurfaceOptions{
		protected:    protected,
		apiProtected: apiProtected,
		workspaceDir: t.TempDir(),
		logger:       zap.NewNop(),
	})

	if !registration.deepResearchRegistered || !registration.knowledgeRoutesRegistered || !registration.harnessResearchRegistered || !registration.harnessRoutesRegistered {
		t.Fatalf("expected deep research, knowledge, harness research, and harness routes to register, got %#v", registration)
	}
	if !registration.research.creatorBound || !registration.research.deepResearchRegistered || !registration.research.harnessResearchRegistered {
		t.Fatalf("expected first-class research lane registration to record creator/route state, got %#v", registration.research)
	}
	if registration.detailProvider == nil {
		t.Fatalf("expected harness detail provider when runtime bundle is present, got %#v", registration)
	}
	if !routeExists(e, "GET", "/api/harness/runs") || !routeExists(e, "GET", "/api/v1/harness/runs") {
		t.Fatalf("expected harness routes on protected/api groups, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/api/knowledge/pages") || !routeExists(e, "GET", "/api/v1/knowledge/pages") {
		t.Fatalf("expected knowledge browse routes on protected/api groups, got %#v", e.Routes())
	}
	if !routeExists(e, "POST", "/api/harness/research/jobs") || !routeExists(e, "POST", "/api/v1/harness/research/jobs") {
		t.Fatalf("expected harness research capability routes on protected/api groups, got %#v", e.Routes())
	}
	if routeExists(e, "GET", "/api/tasks") || routeExists(e, "GET", "/api/v1/tasks") {
		t.Fatalf("expected standalone task list routes to stay removed, got %#v", e.Routes())
	}
	if !routeExists(e, "GET", "/api/tasks/:id") || !routeExists(e, "GET", "/api/v1/tasks/:id") {
		t.Fatalf("expected task detail routes on protected/api groups, got %#v", e.Routes())
	}
	if !routeExists(e, "POST", "/api/tasks/:id/actions/:action") || !routeExists(e, "POST", "/api/v1/tasks/:id/actions/:action") {
		t.Fatalf("expected task action routes on protected/api groups, got %#v", e.Routes())
	}
}
