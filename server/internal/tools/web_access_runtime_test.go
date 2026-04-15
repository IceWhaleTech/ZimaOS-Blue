package tools

import (
	"context"
	"testing"
)

func TestFetchOrchestratorPlanExecutionStableWithSessionCore(t *testing.T) {
	tool := NewWebFetchTool(WebFetchConfig{
		LayeredFetchEnabled:   true,
		SessionMemoryEnabled:  true,
		DomainStrategyEnabled: true,
		AdapterMemoryEnabled:  true,
	})
	orchestrator := tool.orchestrator
	if orchestrator == nil {
		t.Fatal("expected layered fetch orchestrator")
	}

	req := FetchRequest{
		URL:               "https://example.com/account",
		Mode:              webReadFormatText,
		PreferredLane:     webAccessLaneAuto,
		AllowBrowser:      true,
		AllowProxy:        true,
		AllowSession:      true,
		AllowAutoFallback: true,
	}
	hints := webRetrievePlanningHints{
		AutoAllowed:       true,
		EffectiveTargetID: "tab-account",
		HasBrowserTarget:  true,
		HasStrategy:       true,
		Strategy: DomainStrategy{
			Host:             "example.com",
			PreferredLane:    webAccessLaneLightpandaShim,
			NeedsRealBrowser: false,
			LoginRequired:    true,
		},
		HasSessionCore: true,
	}

	first := orchestrator.planExecution(context.Background(), req, hints)
	second := orchestrator.planExecution(context.Background(), req, hints)

	if first.Kind != WebTaskKindRetrieve {
		t.Fatalf("kind = %q, want %q", first.Kind, WebTaskKindRetrieve)
	}
	if first.PrimaryRuntime != webAccessLaneLightpandaShim {
		t.Fatalf("primary runtime = %q, want %q", first.PrimaryRuntime, webAccessLaneLightpandaShim)
	}
	if len(first.Steps) == 0 {
		t.Fatal("expected non-empty retrieve execution steps")
	}
	if first.Steps[0].Runtime != webAccessLaneLightpandaShim {
		t.Fatalf("first runtime = %q, want %q", first.Steps[0].Runtime, webAccessLaneLightpandaShim)
	}
	if got := first.TraceLabels["session_core"]; got != "true" {
		t.Fatalf("session_core trace label = %q, want true", got)
	}
	if got := first.TraceLabels["has_browser_target"]; got != "true" {
		t.Fatalf("has_browser_target trace label = %q, want true", got)
	}
	if len(first.Steps) != len(second.Steps) {
		t.Fatalf("steps length changed across identical plans: first=%d second=%d", len(first.Steps), len(second.Steps))
	}
	for idx := range first.Steps {
		if first.Steps[idx] != second.Steps[idx] {
			t.Fatalf("step %d mismatch across identical plans: first=%+v second=%+v", idx, first.Steps[idx], second.Steps[idx])
		}
	}
}

func TestBuildWebSearchProviderPlanStaysProviderOnly(t *testing.T) {
	plan := buildWebSearchProviderPlan([]string{"bing", "duckduckgo"}, "latest blue", 5)

	if plan.Kind != WebTaskKindSearch {
		t.Fatalf("kind = %q, want %q", plan.Kind, WebTaskKindSearch)
	}
	if plan.PrimaryRuntime != "search_provider:bing" {
		t.Fatalf("primary runtime = %q, want search_provider:bing", plan.PrimaryRuntime)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(plan.Steps))
	}
	if plan.Steps[0].Runtime != "search_provider:bing" {
		t.Fatalf("step 0 runtime = %q, want search_provider:bing", plan.Steps[0].Runtime)
	}
	if plan.Steps[1].Runtime != "search_provider:duckduckgo" {
		t.Fatalf("step 1 runtime = %q, want search_provider:duckduckgo", plan.Steps[1].Runtime)
	}
	for _, step := range plan.Steps {
		if step.Runtime == webAccessLaneBrowser {
			t.Fatalf("plan steps = %+v, want no browser runtime in direct web_search provider plan", plan.Steps)
		}
	}
}
