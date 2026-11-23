package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExternalVerificationReceivesCompleteDeliveredEvidence(t *testing.T) {
	answer := strings.Repeat("A source-grounded finding with quotes and 中文.\n", 100) + "Final finding: without needing permission."
	state := NewGroundTruthState()
	callID := "task/analysis/tc/1"
	command := `blue analyze topic="Summarize the policy" url="https://example.com/"`
	state.Calls[callID] = GroundedToolCall{ToolCallID: callID, Tool: "bash", Args: map[string]any{"command": command}}
	state.Results[callID] = GroundedToolResult{ToolCallID: callID, Tool: "bash", OK: true, Result: map[string]any{"data": map[string]any{"answer": answer}}}
	task := &Task{Goal: "Summarize the policy", GroundState: state, SuccessCriteria: []string{"execution_contract_satisfied"},
		Metadata: map[string]any{"routing_contract": map[string]any{"gate_type": "execution_equivalence", "expected_cli_action": "blue analyze", "enforce_cli_route": true}},
		Plan: []PlanStep{
			{Description: "Analyze source", Status: StepStatusCompleted, Output: answer},
			{Description: "Additional scratchpad research", Status: StepStatusSkipped},
		},
	}
	stub := &captureResponseLLM{response: `{"status":"pass","summary":"Source and complete answer are present.","criteria_results":[{"criterion":"execution_contract_satisfied","status":"pass","evidence":"Reviewed original call and delivered answer."}],"executed_checks":["review full grounded evidence"]}`}
	runner := &Runner{llm: stub}
	if _, _, err := runner.runVerification(context.Background(), task, compileVerificationContext(task)); err != nil {
		t.Fatal(err)
	}
	if len(stub.requests) != 1 || len(stub.requests[0].Messages) < 2 {
		t.Fatalf("expected one verifier request, got %#v", stub.requests)
	}
	userPrompt := stub.requests[0].Messages[1].Content
	for _, want := range []string{answer, callID, "blue analyze", "https://example.com/", `"ok":true`, "intentionally skipped", "fail if any criterion lacks concrete evidence"} {
		if !strings.Contains(userPrompt, want) {
			t.Errorf("verifier model did not receive %q", want)
		}
	}
	// An empty answer cannot authorize the explanation for an early completion.
	state.Results[callID] = GroundedToolResult{ToolCallID: callID, Tool: "bash", OK: true, Result: map[string]any{"data": map[string]any{"topic": "Summary ready"}}}
	if prompt := buildVerificationUserPrompt(task, compileVerificationContext(task)); strings.Contains(prompt, "intentionally skipped") {
		t.Error("empty evidence excused skipped work")
	}
}

func TestGroundedVerificationRequiresSuccessfulCompletionEvidence(t *testing.T) {
	for _, status := range []string{GroundingStatusGrounded, GroundingStatusUnknown, GroundingStatusFallback} {
		for _, evidence := range []string{"unknown", "unknown\nunknown", `tool output excerpt="tool bash execution failed"`} {
			t.Run(status+"/"+evidence, func(t *testing.T) {
				state := NewGroundTruthState()
				state.Calls["failed"] = GroundedToolCall{ToolCallID: "failed", Tool: "bash", Args: map[string]any{"command": "python3 -c 'raise SystemExit(7)'"}}
				state.Results["failed"] = GroundedToolResult{ToolCallID: "failed", Tool: "bash", OK: false, ExitCode: 7, Result: map[string]any{"error": "execution failed"}}
				state.Calls["discovery"] = GroundedToolCall{ToolCallID: "discovery", Tool: "tool_search"}
				state.Results["discovery"] = GroundedToolResult{ToolCallID: "discovery", Tool: "tool_search", OK: true, Result: map[string]any{"matches": []any{"exec"}}}
				task := &Task{Goal: "Execute the command and return its actual output.", GroundState: state, GroundingStatus: status, VerifiedOutput: evidence,
					SuccessCriteria: []string{"The specified command exits successfully.", "The returned output preserves the final finding."},
					Plan:            []PlanStep{{Description: "Execute and deliver output", Status: StepStatusCompleted, Output: evidence}}}
				result, output, err := (&GroundedRuntime{}).VerifyTask(task)
				if err == nil || result.Status == "pass" || strings.Contains(output, "[PASS]") {
					t.Fatalf("missing successful evidence passed: %#v\n%s", result, output)
				}
				// A successful shell invocation alone cannot turn an unknown answer into a delivered result.
				state.Results["unknown-answer"] = GroundedToolResult{ToolCallID: "unknown-answer", Tool: "bash", OK: true, Result: map[string]any{"stdout": ""}}
				if !strings.Contains(evidence, "excerpt") {
					if _, _, err := (&GroundedRuntime{}).VerifyTask(task); err == nil {
						t.Fatal("unknown answer passed after a successful empty execution")
					}
				}
				delete(state.Results, "unknown-answer")
				// Historical failed attempts must not block a successful recovery.
				state.Results["recovered"] = GroundedToolResult{ToolCallID: "recovered", Tool: "bash", OK: true, Result: map[string]any{"stdout": "FINAL_FINDING=actual result"}}
				state.Calls["recovered"] = GroundedToolCall{ToolCallID: "recovered", Tool: "bash"}
				task.GroundingStatus = GroundingStatusGrounded
				task.VerifiedOutput = "FINAL_FINDING=actual result"
				task.Plan[0].Output = task.VerifiedOutput
				if _, _, err := (&GroundedRuntime{}).VerifyTask(task); err != nil {
					t.Fatalf("successful recovery rejected: %v", err)
				}
			})
		}
	}
}

// These lifecycle tests need an executed result before testing completion and reflection.
func withCompletedGroundedWork(t *testing.T, runner *Runner) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "completed.txt")
	if err := os.WriteFile(path, []byte("completed fixture work"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	registry.Register(tools.NewFileReadTool([]string{root}, 0))
	model := completedGroundedWorkLLM{path: path}
	runner.groundedRuntime = NewGroundedRuntime(GroundedRuntimeConfig{PlannerLLM: model, ResponderLLM: model, Registry: registry, Executor: tools.NewExecutor(registry), Store: runner.store, Secret: runner.groundedSecret})
}

type completedGroundedWorkLLM struct{ path string }

func (m completedGroundedWorkLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	var output any
	ids := extractToolCallIDsFromPrompt(req)
	if isGroundedPlannerPrompt(req) {
		output = map[string]any{"status": "complete", "assertions": []any{}}
		if len(ids) == 0 {
			output = map[string]any{"status": "continue", "next_tool": map[string]any{"tool": "read", "args": map[string]any{"path": m.path}}}
		}
	} else {
		output = map[string]any{"claims": []any{map[string]any{"type": "tool_output", "tool_call_ids": ids, "excerpt": "completed fixture work"}}}
	}
	raw, _ := json.Marshal(output)
	return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: string(raw)}}, nil
}

func TestGroundedPromptsPreserveActualShellEvidence(t *testing.T) {
	state := NewGroundTruthState()
	output := strings.Repeat("source detail\n", 80) + "FINAL_FINDING=发布前完成双人复核。\n"
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("task/delivery/tc/%d", i+1)
		state.Calls[id] = GroundedToolCall{ToolCallID: id, Tool: "bash"}
		state.Results[id] = GroundedToolResult{ToolCallID: id, Tool: "bash", OK: true, FinishedAt: time.Unix(int64(i), 0), Result: map[string]any{"stdout": output, "stderr": "实际错误流\n", "exit_code": 0, "session_id": id}}
	}
	for name, prompt := range map[string]string{"planner": buildGroundedPlannerUserPrompt(PlannerInput{GroundState: state}), "responder": buildResponderUserPrompt(ResponderInput{GroundState: state})} {
		if !strings.Contains(prompt, output) || !strings.Contains(prompt, "task/delivery/tc/8") {
			t.Fatalf("%s lost actual multiline output or recent evidence: %s", name, prompt)
		}
		if strings.Contains(prompt, "\ufffd") {
			t.Fatalf("%s split UTF-8", name)
		}
	}
	// Oversized output must carry a usable retained-session recovery request.
	id := "task/delivery/tc/9"
	state.Results[id] = GroundedToolResult{ToolCallID: id, Tool: "bash", OK: true, FinishedAt: time.Unix(9, 0), Result: map[string]any{"stdout": strings.Repeat("甲", 20000), "stderr": "尾部\n", "session_id": "recover-session", "exit_code": 0}}
	for _, prompt := range []string{buildGroundedPlannerUserPrompt(PlannerInput{GroundState: state}), buildResponderUserPrompt(ResponderInput{GroundState: state})} {
		if !strings.Contains(prompt, `"output_retrieval"`) || !strings.Contains(prompt, `"session_id":"recover-session"`) || !strings.Contains(prompt, `"llm_output_truncated":true`) {
			t.Fatal("oversized evidence lost recovery metadata")
		}
	}
}

func TestGroundedSummaryPreservesReadableTable(t *testing.T) {
	source := "Project\tCategory\tAmount\nLEDGER_Q7\tHardware\t217.35\nLEDGER_Q7\tService\t82.65\n"
	got := groundedResultSummaryLine(GroundedToolCall{Tool: "bash"}, GroundedToolResult{Tool: "bash", OK: true, Result: map[string]any{"stdout": source, "exit_code": 0, "session_id": "table-read"}})
	if !strings.Contains(got, source) {
		t.Fatal("grounded model summary still escapes source rows")
	}
	if !strings.Contains(got, `"exit_code":0`) || !strings.Contains(got, `"session_id":"table-read"`) {
		t.Fatal("execution metadata lost")
	}
}
