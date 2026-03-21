package deepresearch

import "testing"

func TestResolveRouteMode_DefaultsToWeb(t *testing.T) {
	requested, effective, reason, err := resolveRouteMode("Summarize the latest OpenAI announcements with sources", "", RoutePolicy{})
	if err != nil {
		t.Fatalf("resolveRouteMode error = %v", err)
	}
	if requested != RouteModeWeb {
		t.Fatalf("requested = %q, want %q", requested, RouteModeWeb)
	}
	if effective != RouteModeWeb {
		t.Fatalf("effective = %q, want %q", effective, RouteModeWeb)
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestResolveRouteMode_InvalidRequestedMode(t *testing.T) {
	_, _, _, err := resolveRouteMode("topic", RouteMode("bogus"), RoutePolicy{})
	if err != ErrInvalidRouteMode {
		t.Fatalf("resolveRouteMode invalid mode err = %v, want %v", err, ErrInvalidRouteMode)
	}
}

func TestResolveRouteMode_WebRequestStaysWeb(t *testing.T) {
	requested, effective, reason, err := resolveRouteMode("topic", RouteModeWeb, RoutePolicy{})
	if err != nil {
		t.Fatalf("resolveRouteMode error = %v", err)
	}
	if requested != RouteModeWeb {
		t.Fatalf("requested = %q, want %q", requested, RouteModeWeb)
	}
	if effective != RouteModeWeb {
		t.Fatalf("effective = %q, want %q", effective, RouteModeWeb)
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}
