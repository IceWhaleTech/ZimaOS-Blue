package builtin

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type mockSelfReflectExecutor struct {
	input  selfreflect.Input
	result *selfreflect.Result
	err    error
}

func (m *mockSelfReflectExecutor) Reflect(_ context.Context, input selfreflect.Input) (*selfreflect.Result, error) {
	m.input = input
	return m.result, m.err
}

func TestSelfReflectSkill_Execute(t *testing.T) {
	exec := &mockSelfReflectExecutor{result: &selfreflect.Result{
		Summary:       "Reflection complete.",
		MemoryWritten: 2,
		Lessons: []selfreflect.Lesson{{
			Kind:        selfreflect.LessonKindHeuristic,
			Lesson:      "Run focused verification first.",
			WhenToApply: "After a parser edit.",
			Evidence:    "Verification caught the regression before broader tests.",
		}},
	}}
	skill := NewSelfReflect()
	skill.SetExecutor(exec)

	res, err := skill.Execute(context.Background(), map[string]any{
		"task_id":      "task-1",
		"goal":         "Fix parser",
		"final_status": "completed",
		"plan": []interface{}{
			map[string]any{"description": "patch parser", "status": "completed"},
		},
		"step_outputs": []interface{}{"updated parser branch"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if exec.input.TaskID != "task-1" {
		t.Fatalf("task_id = %q, want task-1", exec.input.TaskID)
	}
	if len(exec.input.Plan) != 1 || exec.input.Plan[0].Output != "updated parser branch" {
		t.Fatalf("plan merge failed: %#v", exec.input.Plan)
	}
	data := res.Data.(map[string]any)
	if got := data["memory_written"].(int); got != 2 {
		t.Fatalf("memory_written = %d, want 2", got)
	}
}

func TestSelfReflectSkill_ValidateRejectsInvalidStatus(t *testing.T) {
	skill := NewSelfReflect()
	err := skill.Validate(map[string]any{"goal": "Fix parser", "final_status": "broken"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSelfReflectSkill_ValidateAcceptsCommonAliases(t *testing.T) {
	skill := NewSelfReflect()
	input := map[string]any{
		"task":    "Fix parser",
		"status":  "COMPLETED",
		"summary": "Patched parser and verified it",
	}
	if err := skill.Validate(input); err != nil {
		t.Fatalf("expected aliases to pass validation: %v", err)
	}
	if got, _ := input["goal"].(string); got != "Fix parser" {
		t.Fatalf("goal = %q, want Fix parser", got)
	}
	if got, _ := input["final_status"].(string); got != "completed" {
		t.Fatalf("final_status = %q, want completed", got)
	}
	if got, _ := input["result_summary"].(string); got != "Patched parser and verified it" {
		t.Fatalf("result_summary = %q, want summary alias", got)
	}
}

func TestSelfReflectSkill_ExecuteAcceptsAliases(t *testing.T) {
	exec := &mockSelfReflectExecutor{result: &selfreflect.Result{Summary: "ok"}}
	skill := NewSelfReflect()
	skill.SetExecutor(exec)

	res, err := skill.Execute(context.Background(), map[string]any{
		"taskId":              "task-2",
		"task":                "Fix parser",
		"status":              "FAILED",
		"outcome":             "Patch incomplete",
		"error":               "edge case still broken",
		"verification_result": "focused test still failing",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if exec.input.TaskID != "task-2" {
		t.Fatalf("task_id = %q, want task-2", exec.input.TaskID)
	}
	if exec.input.FinalStatus != "failed" {
		t.Fatalf("final_status = %q, want failed", exec.input.FinalStatus)
	}
	if exec.input.ResultSummary != "Patch incomplete" {
		t.Fatalf("result_summary = %q", exec.input.ResultSummary)
	}
	if exec.input.FailureReason != "edge case still broken" {
		t.Fatalf("failure_reason = %q", exec.input.FailureReason)
	}
	if exec.input.VerificationOutput != "focused test still failing" {
		t.Fatalf("verification_output = %q", exec.input.VerificationOutput)
	}
}

func TestSelfReflectSkill_ExecuteForwardsRuntimeReflectionFields(t *testing.T) {
	exec := &mockSelfReflectExecutor{result: &selfreflect.Result{Summary: "ok"}}
	skill := NewSelfReflect()
	skill.SetExecutor(exec)

	res, err := skill.Execute(context.Background(), map[string]any{
		"goal":                 "Investigate browser regression",
		"final_status":         "running",
		"trigger_kind":         "interval_tool_finishes",
		"disable_memory_write": true,
		"runtime_signals": map[string]any{
			"tool_finishes_since_review": 10,
		},
		"evidence_window": []interface{}{
			map[string]any{
				"id":            "ev-1",
				"event_type":    "tool_finished",
				"step_index":    2,
				"planner_round": 1,
				"summary":       "browser navigate returned partial output",
				"payload_json":  `{"error":"missing_title"}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if exec.input.TriggerKind != "interval_tool_finishes" {
		t.Fatalf("trigger_kind = %q, want interval_tool_finishes", exec.input.TriggerKind)
	}
	if !exec.input.DisableMemoryWrite {
		t.Fatal("expected disable_memory_write to be forwarded")
	}
	if got, _ := exec.input.RuntimeSignals["tool_finishes_since_review"].(int); got != 10 {
		t.Fatalf("runtime_signals.tool_finishes_since_review = %#v, want 10", exec.input.RuntimeSignals["tool_finishes_since_review"])
	}
	if len(exec.input.EvidenceWindow) != 1 || exec.input.EvidenceWindow[0].ID != "ev-1" {
		t.Fatalf("evidence_window = %#v, want one forwarded record", exec.input.EvidenceWindow)
	}
}
