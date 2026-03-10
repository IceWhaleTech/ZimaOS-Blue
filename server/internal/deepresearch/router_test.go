package deepresearch

import "testing"

func TestClassifyRouteMode(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  RouteMode
	}{
		{name: "web", query: "Summarize the latest OpenAI announcements with sources", want: RouteModeWeb},
		{name: "experiment", query: "Run an ablation and benchmark the repo changes", want: RouteModeExperiment},
		{name: "hybrid", query: "First research the best LoRA setup, then validate it with a benchmark", want: RouteModeHybrid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := classifyRouteMode(tc.query)
			if got != tc.want {
				t.Fatalf("classifyRouteMode(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestResolveRouteMode_InvalidRequestedMode(t *testing.T) {
	_, _, _, err := resolveRouteMode("topic", RouteMode("bogus"), defaultRoutePolicy(), false)
	if err != ErrInvalidRouteMode {
		t.Fatalf("resolveRouteMode invalid mode err = %v, want %v", err, ErrInvalidRouteMode)
	}
}

func TestResolveRouteMode_DegradesWithoutExperimentBackend(t *testing.T) {
	requested, effective, reason, err := resolveRouteMode("run an ablation benchmark", RouteModeAuto, defaultRoutePolicy(), false)
	if err != nil {
		t.Fatalf("resolveRouteMode error = %v", err)
	}
	if requested != RouteModeAuto {
		t.Fatalf("requested = %q, want %q", requested, RouteModeAuto)
	}
	if effective != RouteModeWeb {
		t.Fatalf("effective = %q, want %q", effective, RouteModeWeb)
	}
	if reason == "" {
		t.Fatal("expected non-empty reason")
	}
}
