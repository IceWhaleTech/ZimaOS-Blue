package agent

import "testing"

func TestTrimStructuredContent_ExtractsJSONWithTrailingCommentary(t *testing.T) {
	raw := "{\"status\":\"continue\",\"reason\":\"search docs\",\"next_tool\":{\"tool\":\"web_search\",\"args\":{\"query\":\"OpenAI Responses API\"}}}\n- note: do not invent results"
	got := trimStructuredContent(raw)
	want := "{\"status\":\"continue\",\"reason\":\"search docs\",\"next_tool\":{\"tool\":\"web_search\",\"args\":{\"query\":\"OpenAI Responses API\"}}}"
	if got != want {
		t.Fatalf("trimStructuredContent() = %q, want %q", got, want)
	}
}

func TestTrimStructuredContent_ExtractsFencedJSONAfterPreface(t *testing.T) {
	raw := "Use this payload:\n```json\n{\"status\":\"pass\",\"summary\":\"ok\",\"criteria_results\":[{\"criterion\":\"tool used\",\"status\":\"pass\",\"evidence\":\"web_search called\"}],\"executed_checks\":[\"grounded runtime status review\"]}\n```\nThanks."
	got := trimStructuredContent(raw)
	want := "{\"status\":\"pass\",\"summary\":\"ok\",\"criteria_results\":[{\"criterion\":\"tool used\",\"status\":\"pass\",\"evidence\":\"web_search called\"}],\"executed_checks\":[\"grounded runtime status review\"]}"
	if got != want {
		t.Fatalf("trimStructuredContent() = %q, want %q", got, want)
	}
}

func TestParsePlannerDecision_AllowsTrailingBulletsAfterJSONObject(t *testing.T) {
	raw := "{\"status\":\"continue\",\"reason\":\"Need live docs\",\"next_tool\":{\"tool\":\"web_search\",\"args\":{\"query\":\"OpenAI Responses API latest documentation\"}},\"assertions\":[{\"type\":\"tool_called\",\"tool\":\"web_search\"}]}\n- follow the grounded policy"
	decision, err := parsePlannerDecision(raw)
	if err != nil {
		t.Fatalf("parsePlannerDecision() error = %v", err)
	}
	if decision.Status != PlannerDecisionContinue {
		t.Fatalf("status = %q, want %q", decision.Status, PlannerDecisionContinue)
	}
	if decision.NextTool == nil || decision.NextTool.Tool != "web_search" {
		t.Fatalf("next_tool = %#v, want web_search", decision.NextTool)
	}
}
