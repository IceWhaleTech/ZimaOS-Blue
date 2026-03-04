package builtin

import (
	"context"
	"strings"
	"testing"
)

func resetPlanStoreForTest() {
	globalPlanStore.mu.Lock()
	defer globalPlanStore.mu.Unlock()
	globalPlanStore.plans = make(map[string]*planState)
}

func mustPlanData(t *testing.T, r any) map[string]any {
	t.Helper()
	m, ok := r.(map[string]any)
	if !ok {
		t.Fatalf("result data type=%T, want map[string]any", r)
	}
	return m
}

func TestPlanCreateAcceptsJSONEncodedTasksString(t *testing.T) {
	resetPlanStoreForTest()
	p := NewPlanCreate()

	res, err := p.Execute(context.Background(), map[string]any{
		"tasks": `["design API","write tests"]`,
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%q", res.Error)
	}

	data := mustPlanData(t, res.Data)
	if got := data["task_count"]; got != 2 {
		t.Fatalf("task_count=%v, want 2", got)
	}
	if got := data["completed_count"]; got != 0 {
		t.Fatalf("completed_count=%v, want 0", got)
	}
	if got := data["pending_count"]; got != 2 {
		t.Fatalf("pending_count=%v, want 2", got)
	}
	if got := data["all_completed"]; got != false {
		t.Fatalf("all_completed=%v, want false", got)
	}
	checklist, _ := data["checklist"].(string)
	if !strings.Contains(checklist, "design API") || !strings.Contains(checklist, "write tests") {
		t.Fatalf("unexpected checklist: %q", checklist)
	}
}

func TestPlanCreateAcceptsCommaSeparatedTasksString(t *testing.T) {
	resetPlanStoreForTest()
	p := NewPlanCreate()

	res, err := p.Execute(context.Background(), map[string]any{
		"tasks": "step one, step two",
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%q", res.Error)
	}

	data := mustPlanData(t, res.Data)
	if got := data["task_count"]; got != 2 {
		t.Fatalf("task_count=%v, want 2", got)
	}
}

func TestPlanCreateAcceptsMarkdownChecklistString(t *testing.T) {
	resetPlanStoreForTest()
	p := NewPlanCreate()

	res, err := p.Execute(context.Background(), map[string]any{
		"tasks": "- [ ] first task\n2. second task",
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%q", res.Error)
	}

	data := mustPlanData(t, res.Data)
	if got := data["task_count"]; got != 2 {
		t.Fatalf("task_count=%v, want 2", got)
	}
}

func TestPlanCreateAcceptsQueryFallback(t *testing.T) {
	resetPlanStoreForTest()
	p := NewPlanCreate()

	res, err := p.Execute(context.Background(), map[string]any{
		"query": "alpha;beta",
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%q", res.Error)
	}

	data := mustPlanData(t, res.Data)
	if got := data["task_count"]; got != 2 {
		t.Fatalf("task_count=%v, want 2", got)
	}
}

func TestPlanAppendAndUpdateFlow(t *testing.T) {
	resetPlanStoreForTest()
	create := NewPlanCreate()
	appendSkill := NewPlanAppend()
	update := NewPlanUpdate()

	createRes, err := create.Execute(context.Background(), map[string]any{
		"tasks": []string{"initial task"},
	})
	if err != nil {
		t.Fatalf("create execute error: %v", err)
	}
	if !createRes.Success {
		t.Fatalf("create expected success, got error=%q", createRes.Error)
	}

	appendRes, err := appendSkill.Execute(context.Background(), map[string]any{
		"task": "follow-up task",
	})
	if err != nil {
		t.Fatalf("append execute error: %v", err)
	}
	if !appendRes.Success {
		t.Fatalf("append expected success, got error=%q", appendRes.Error)
	}
	appendData := mustPlanData(t, appendRes.Data)
	if got := appendData["task_count"]; got != 2 {
		t.Fatalf("append task_count=%v, want 2", got)
	}
	if got := appendData["pending_count"]; got != 2 {
		t.Fatalf("append pending_count=%v, want 2", got)
	}
	if got := appendData["all_completed"]; got != false {
		t.Fatalf("append all_completed=%v, want false", got)
	}

	updateRes, err := update.Execute(context.Background(), map[string]any{
		"index": "2",
		"done":  "true",
		"title": "follow-up task done",
	})
	if err != nil {
		t.Fatalf("update execute error: %v", err)
	}
	if !updateRes.Success {
		t.Fatalf("update expected success, got error=%q", updateRes.Error)
	}
	updateData := mustPlanData(t, updateRes.Data)
	checklist, _ := updateData["checklist"].(string)
	if !strings.Contains(checklist, "- [x] follow-up task done") {
		t.Fatalf("unexpected updated checklist: %q", checklist)
	}
	if got := updateData["completed_count"]; got != 1 {
		t.Fatalf("update completed_count=%v, want 1", got)
	}
	if got := updateData["pending_count"]; got != 1 {
		t.Fatalf("update pending_count=%v, want 1", got)
	}
	if got := updateData["all_completed"]; got != false {
		t.Fatalf("update all_completed=%v, want false", got)
	}

	updateAllRes, err := update.Execute(context.Background(), map[string]any{
		"task_index": 1,
		"checked":    true,
	})
	if err != nil {
		t.Fatalf("update-all execute error: %v", err)
	}
	if !updateAllRes.Success {
		t.Fatalf("update-all expected success, got error=%q", updateAllRes.Error)
	}
	updateAllData := mustPlanData(t, updateAllRes.Data)
	if got := updateAllData["completed_count"]; got != 2 {
		t.Fatalf("update-all completed_count=%v, want 2", got)
	}
	if got := updateAllData["pending_count"]; got != 0 {
		t.Fatalf("update-all pending_count=%v, want 0", got)
	}
	if got := updateAllData["all_completed"]; got != true {
		t.Fatalf("update-all all_completed=%v, want true", got)
	}
}

func TestPlanAppendRequiresExistingPlan(t *testing.T) {
	resetPlanStoreForTest()
	appendSkill := NewPlanAppend()

	res, err := appendSkill.Execute(context.Background(), map[string]any{
		"task": "new task",
	})
	if err != nil {
		t.Fatalf("append execute error: %v", err)
	}
	if res.Success {
		t.Fatalf("expected append failure when no plan exists")
	}
	if !strings.Contains(res.Error, "no plan exists") {
		t.Fatalf("unexpected append error: %q", res.Error)
	}
}

func TestPlanUpdateRequiresExistingPlan(t *testing.T) {
	resetPlanStoreForTest()
	update := NewPlanUpdate()

	res, err := update.Execute(context.Background(), map[string]any{
		"task_index": 1,
		"checked":    true,
	})
	if err != nil {
		t.Fatalf("update execute error: %v", err)
	}
	if res.Success {
		t.Fatalf("expected update failure when no plan exists")
	}
	if !strings.Contains(res.Error, "no plan exists") {
		t.Fatalf("unexpected update error: %q", res.Error)
	}
}

func TestPlanUpdateRejectsOutOfRangeIndex(t *testing.T) {
	resetPlanStoreForTest()
	create := NewPlanCreate()
	update := NewPlanUpdate()

	createRes, err := create.Execute(context.Background(), map[string]any{
		"tasks": []string{"only task"},
	})
	if err != nil {
		t.Fatalf("create execute error: %v", err)
	}
	if !createRes.Success {
		t.Fatalf("create expected success, got error=%q", createRes.Error)
	}

	updateRes, err := update.Execute(context.Background(), map[string]any{
		"task_index": 2,
		"checked":    true,
	})
	if err != nil {
		t.Fatalf("update execute error: %v", err)
	}
	if updateRes.Success {
		t.Fatalf("expected update failure for out-of-range index")
	}
	if !strings.Contains(updateRes.Error, "task_index out of range") {
		t.Fatalf("unexpected update error: %q", updateRes.Error)
	}
}
