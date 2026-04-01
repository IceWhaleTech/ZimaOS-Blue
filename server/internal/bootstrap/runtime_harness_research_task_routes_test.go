package bootstrap

import (
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
)

func TestRegisterHarnessRuntimeResearchTaskRoutes_TracksRouteKindsWithoutHarness(t *testing.T) {
	e := echo.New()
	target := &stubDeepResearchRuntimeRouteTarget{}

	registration := registerHarnessRuntimeResearchTaskRoutes(
		nil,
		target,
		deepresearch.NewService(nil, nil),
		"/tmp/workspace",
		[]*echo.Group{
			e.Group("/deep-research"),
			e.Group("/api/deep-research"),
		},
		[]*echo.Group{
			e.Group("/harness/research"),
			e.Group("/api/harness/research"),
		},
	)

	if registration.creatorBound {
		t.Fatalf("expected creator binding to stay disabled without harness runtime, got %#v", registration)
	}
	if !registration.deepResearchRegistered || !registration.harnessResearchRegistered {
		t.Fatalf("expected research compatibility and capability routes to register, got %#v", registration)
	}
	if target.creatorCalls != 0 || target.creator != nil {
		t.Fatalf("expected no job creator binding without harness runtime, got %#v", target)
	}
	if target.registerCalls != 4 {
		t.Fatalf("expected four research route registrations, got %#v", target)
	}
}

func TestRegisterHarnessRuntimeResearchTaskRoutes_TracksCreatorBindingWithHarness(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	e := echo.New()
	target := &stubDeepResearchRuntimeRouteTarget{}

	registration := registerHarnessRuntimeResearchTaskRoutes(
		bundle,
		target,
		deepresearch.NewService(nil, nil),
		"/tmp/workspace",
		[]*echo.Group{e.Group("/deep-research")},
		[]*echo.Group{e.Group("/harness/research")},
	)

	if !registration.creatorBound || !registration.deepResearchRegistered || !registration.harnessResearchRegistered {
		t.Fatalf("expected harness research lane to bind creator and routes, got %#v", registration)
	}
	if target.creatorCalls != 1 || target.creator == nil {
		t.Fatalf("expected harness-backed creator binding, got %#v", target)
	}
	if target.registerCalls != 2 {
		t.Fatalf("expected two research route registrations, got %#v", target)
	}
}
