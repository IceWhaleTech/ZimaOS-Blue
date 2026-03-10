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
