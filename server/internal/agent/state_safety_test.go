package agent

import (
	"strings"
	"testing"
)

type recursiveStatePayload struct {
	Name string                 `json:"name"`
	Self *recursiveStatePayload `json:"self,omitempty"`
}

func TestAsStringGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveStatePayload{Name: "root"}
	payload.Self = payload

	got := asString(payload)
	if !strings.Contains(got, `"name":"root"`) {
		t.Fatalf("asString() = %q, want name field", got)
	}
	if !strings.Contains(got, `[circular payload omitted]`) {
		t.Fatalf("asString() = %q, want circular marker", got)
	}
}

func TestEvaluateAssertionsTreatsCanonicalWebSearchCommandAsWebFamilyEvidence(t *testing.T) {
	state := NewGroundTruthState()
	state.Commands["task/assertions/tc/1"] = GroundedCommandFact{
		ToolCallID: "task/assertions/tc/1",
		Tool:       "exec",
		Command:    `blue web_search query="OpenAI Responses API latest docs"`,
		ExitCode:   0,
	}

	failures := EvaluateAssertions(state, []PlannerAssertion{{
		Type: "tool_called",
		Tool: "web_query",
	}})
	if len(failures) != 0 {
		t.Fatalf("EvaluateAssertions() failures = %v, want no failures", failures)
	}
}
