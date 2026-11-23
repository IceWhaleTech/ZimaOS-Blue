package agent

import (
	"encoding/json"
	"os"
	"testing"
)

func TestWebCompletionRequiresCollectedEvidence(t *testing.T) {
	raw, err := os.ReadFile("testdata/web-empty-query.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Goal   string
		Call   GroundedToolCall
		Result map[string]any
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		payload map[string]any
		want    bool
	}{
		{"real-empty-search", fixture.Result, false},
		{"content", map[string]any{"content": "Responses API reference: POST /responses"}, true},
		{"source-list", map[string]any{"status": "partial", "sources": []any{map[string]any{"title": "API reference", "url": "https://example.com/docs"}}}, true},
		{"cli-source-list", map[string]any{"sources": `[{"title":"API reference","url":"https://example.com/docs"}]`}, true},
		{"legacy-cli-content", map[string]any{"stdout": "input: API docs\ncontent: Responses API reference: POST /responses"}, true},
		{"legacy-cli-query-only", map[string]any{"stdout": "input: API docs\nstatus: partial\ncontent: \nsources: []\nnext_action: refine_query"}, false},
		{"legacy-xml-source", map[string]any{"stdout": `format: xml
result: <web_search><results><result><title>API reference</title><url>https://example.com/docs</url><description>POST /responses</description></result></results></web_search>`}, true},
		{"legacy-empty-xml", map[string]any{"stdout": "format: xml\nresult: <web_search><result>ok</result></web_search>"}, false},
		{"json-stdout", map[string]any{"stdout": `{"status":"ok","content":"POST /responses"}`}, true},
		{"json-search-results", map[string]any{"result": `{"results":[{"title":"API reference","url":"https://example.com/docs","description":"POST /responses"}]}`}, true},
		{"xml-entities", map[string]any{"result": `<web_search><results><result><title>API &amp; Reference</title><url>https://example.com/docs?a=1&amp;b=2</url></result></results></web_search>`}, true},
		{"content-mentions-challenge", map[string]any{"status": "ok", "content": "The API may return an authentication challenge."}, true},
		{"error-envelope", map[string]any{"status": "error", "data": map[string]any{"content": "Unsuccessful draft"}}, false},
		{"pending-envelope", map[string]any{"status": "pending", "data": map[string]any{"content": "Unfinished draft"}}, false},
		{"failed-stdout", map[string]any{"stdout": "status: failed\ncontent: Unsuccessful draft"}, false},
	}
	for _, status := range []string{"", "ok", "partial"} {
		tests = append(tests, struct {
			name    string
			payload map[string]any
			want    bool
		}{"echo-only-" + status, map[string]any{"status": status, "input": "API docs", "url": "https://example.com/docs", "message": "Search ready", "diagnostics": "No candidates", "sources": "[]"}, false})
	}
	for _, test := range tests {
		for _, wrapper := range []string{"native", "cli"} {
			t.Run(test.name+"/"+wrapper, func(t *testing.T) {
				payload := test.payload
				if wrapper == "cli" {
					payload = map[string]any{"status": "completed", "exit_code": 0, "data": payload}
				}
				state := NewGroundTruthState()
				call := fixture.Call
				call.ToolCallID = "web-result"
				result := GroundedToolResult{ToolCallID: call.ToolCallID, Tool: call.Tool, OK: true, Result: payload}
				state.Calls[call.ToolCallID] = call
				state.Results[call.ToolCallID] = result
				task := &Task{Goal: fixture.Goal, GroundState: state, Metadata: map[string]any{
					"routing_contract":      map[string]any{"gate_type": "execution_equivalence", "expected_cli_action": "blue web_query", "enforce_cli_route": true},
					"required_observations": []any{"evidence_tool_used"},
				}}
				if got := shouldStopAfterGroundedExecutionEvidence(task); got != test.want {
					t.Errorf("stop = %v, want %v", got, test.want)
				}
				if got := groundedExecutionSatisfiesExecutionContract(task, &GroundedExecution{Call: call, Result: result}); got != test.want {
					t.Errorf("execution contract = %v, want %v", got, test.want)
				}
				response := deterministicWebGroundedResponse(call.ToolCallID, call, result)
				if (response != nil) != test.want {
					t.Errorf("deterministic response = %#v, want evidence=%v", response, test.want)
				}
				if response != nil {
					if decision := NewGroundedVerifier(nil).Verify(state, response); !decision.Valid {
						t.Errorf("collected evidence did not verify: %v", decision.Violations)
					}
				}
				result.OK = false
				if deterministicWebGroundedResponse(call.ToolCallID, call, result) != nil || groundedResultHasUsefulEvidence(result) {
					t.Error("failed tool result must not complete from its draft payload")
				}
			})
		}
	}
}
