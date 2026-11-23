package agent

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAnalyzeCompletionRequiresAnAnswer(t *testing.T) {
	for _, payload := range []map[string]any{
		{"topic": "Summarize the page", "message": "Analysis ready"},
		{"topic": "Summarize the page", "answer": "  "},
		{"status": "running", "answer": "Draft answer"},
		{"status": "pending", "answer": "Draft answer"},
		{"status": "pending", "data": map[string]any{"answer": "Draft answer"}},
		{"status": "error", "data": map[string]any{"answer": "Unsuccessful answer"}},
	} {
		state := NewGroundTruthState()
		call := GroundedToolCall{ToolCallID: "analysis", Tool: "bash", Args: map[string]any{"command": "blue analyze topic=summary"}}
		result := GroundedToolResult{ToolCallID: call.ToolCallID, Tool: "bash", OK: true, Result: map[string]any{"data": payload}}
		state.Calls[call.ToolCallID] = call
		state.Results[call.ToolCallID] = result
		task := &Task{GroundState: state, Metadata: map[string]any{"routing_contract": map[string]any{"gate_type": "execution_equivalence", "expected_cli_action": "blue analyze", "enforce_cli_route": true}}}
		if shouldStopAfterGroundedExecutionEvidence(task) || groundedExecutionSatisfiesExecutionContract(task, &GroundedExecution{Call: call, Result: result}) {
			t.Errorf("analysis without a finished answer completed the task: %v", payload)
		}
		if response := deterministicAnalyzeGroundedResponse(call.ToolCallID, call, result); response != nil {
			t.Errorf("analysis without an answer became a final response: %#v", response)
		}
	}
}

func TestAnalyzeCompletionDeliversWholeAnswer(t *testing.T) {
	answer := strings.Repeat("A source-grounded finding with \"quotes\" and 中文.\n", 50) + "Final finding: delivery is complete."
	for _, field := range []string{"answer", "summary", "content"} {
		state := NewGroundTruthState()
		callID := "analysis"
		state.Calls[callID] = GroundedToolCall{ToolCallID: callID, Tool: "bash", Args: map[string]any{"command": "blue analyze topic=summary"}}
		state.Results[callID] = GroundedToolResult{ToolCallID: callID, Tool: "bash", OK: true, Result: map[string]any{"data": map[string]any{field: answer, "topic": "source summary", "status": "completed"}}}
		verifier := NewGroundedVerifier(nil)
		response, err := verifier.Respond(context.Background(), nil, ResponderInput{GroundState: state, PriorToolCallIDs: []string{callID}})
		if err != nil {
			t.Errorf("field=%s: complete answer required another LLM call: %v", field, err)
			continue
		}
		decision := verifier.Verify(state, response)
		if !decision.Valid || len(response.Claims) == 0 || response.Claims[0].Excerpt != answer || !strings.Contains(decision.Output, "Final finding: delivery is complete.") {
			t.Errorf("field=%s: completed answer was lost or truncated: %q", field, decision.Output)
		}
		report := buildGroundedTaskReport(&Task{Status: TaskStatusCompleted, Plan: []PlanStep{{Status: StepStatusCompleted, Output: decision.Output}}})
		if !strings.Contains(report, decision.Output) {
			t.Errorf("field=%s: final report dropped part of the verified answer", field)
		}
		response.Claims[0].Excerpt = answer + " Invented finding."
		if verifier.Verify(state, response).Valid {
			t.Errorf("field=%s: unsupported answer was accepted", field)
		}
	}
}

func TestGroundedFinalReportPreservesChineseOutput(t *testing.T) {
	output := strings.Repeat("甲", 170) + "FINAL_FINDING=发布前完成双人复核。"
	report := buildGroundedTaskReport(&Task{Status: TaskStatusCompleted, Plan: []PlanStep{{Status: StepStatusCompleted, Output: output}}})
	if !utf8.ValidString(report) || !strings.Contains(report, output) {
		t.Fatal("final report corrupted or omitted the actual Chinese answer")
	}
}
