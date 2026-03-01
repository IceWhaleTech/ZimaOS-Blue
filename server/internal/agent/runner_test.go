package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// --- helpers ---

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // in-memory DB is per-connection; force single conn
	t.Cleanup(func() { db.Close() })
	return db
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(testDB(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// mockLLM returns a canned plan on first call, then a text-only response.
type mockLLM struct {
	calls    int
	planJSON string // returned on first call (planning phase)
}

func (m *mockLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.calls++
	// Planning call: return JSON plan
	if m.calls == 1 && m.planJSON != "" {
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: m.planJSON},
		}, nil
	}
	// Execution calls: return simple text (no tool calls)
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Done."},
	}, nil
}

// --- Store tests ---

func TestStore_CreateAndGet(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{
		ID:     "t1",
		UserID: "u1",
		Goal:   "test goal",
		Status: TaskStatusPending,
	}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Goal != "test goal" {
		t.Errorf("goal = %q, want %q", got.Goal, "test goal")
	}
	if got.Status != TaskStatusPending {
		t.Errorf("status = %q, want %q", got.Status, TaskStatusPending)
	}
}

func TestStore_ListByUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for _, id := range []string{"t1", "t2", "t3"} {
		_ = s.Create(ctx, &Task{ID: id, UserID: "u1", Goal: "g"})
	}
	_ = s.Create(ctx, &Task{ID: "t4", UserID: "u2", Goal: "g"})

	tasks, err := s.ListByUser(ctx, "u1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Errorf("got %d tasks, want 3", len(tasks))
	}
}

func TestStore_Update(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusPending}
	_ = s.Create(ctx, task)

	task.Status = TaskStatusCompleted
	task.Progress = 100
	task.Result = "all done"
	if err := s.Update(ctx, task); err != nil {
		t.Fatal(err)
	}

	got, _ := s.Get(ctx, "t1")
	if got.Status != TaskStatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
	if got.Progress != 100 {
		t.Errorf("progress = %d, want 100", got.Progress)
	}
}

func TestStore_Delete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g"})
	if err := s.Delete(ctx, "t1"); err != nil {
		t.Fatal(err)
	}
	_, err := s.Get(ctx, "t1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestStore_CountRunning(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting})
	_ = s.Create(ctx, &Task{ID: "t2", UserID: "u1", Goal: "g", Status: TaskStatusPlanning})
	_ = s.Create(ctx, &Task{ID: "t3", UserID: "u1", Goal: "g", Status: TaskStatusCompleted})

	count, err := s.CountRunning(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("running count = %d, want 2", count)
	}
}

func TestStore_SetStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusPending})
	if err := s.SetStatus(ctx, "t1", TaskStatusFailed, "oops"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, "t1")
	if got.Status != TaskStatusFailed {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.Error != "oops" {
		t.Errorf("error = %q, want %q", got.Error, "oops")
	}
}

func TestStore_Cleanup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusCompleted})
	// Manually backdate
	s.db.ExecContext(ctx, `UPDATE agent_tasks SET created_at=? WHERE id=?`,
		time.Now().Add(-48*time.Hour), "t1")

	n, err := s.Cleanup(ctx, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("cleaned up %d, want 1", n)
	}
}

// --- Task type tests ---

func TestMarshalUnmarshalPlan(t *testing.T) {
	steps := []PlanStep{
		{Index: 0, Description: "step 1", Status: StepStatusPending},
		{Index: 1, Description: "step 2", Status: StepStatusCompleted},
	}
	data := MarshalPlan(steps)
	got := UnmarshalPlan(data)
	if len(got) != 2 {
		t.Fatalf("got %d steps, want 2", len(got))
	}
	if got[0].Description != "step 1" {
		t.Errorf("step 0 desc = %q", got[0].Description)
	}
	if got[1].Status != StepStatusCompleted {
		t.Errorf("step 1 status = %q", got[1].Status)
	}
}

func TestUnmarshalPlan_Empty(t *testing.T) {
	if got := UnmarshalPlan(""); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// --- Runner tests ---

func TestRunner_Submit(t *testing.T) {
	s := testStore(t)
	plan := `[{"description":"step one"},{"description":"step two"}]`
	m := &mockLLM{planJSON: plan}
	registry := tools.NewRegistry()
	executor := tools.NewExecutor(registry)

	runner := NewRunner(s, m, registry, executor, nil, RunnerConfig{
		MaxConcurrent: 2,
		TaskTimeout:   10 * time.Second,
	})
	t.Cleanup(func() {
		runner.Shutdown()
	})

	ctx := context.Background()
	task, err := runner.Submit(ctx, "u1", "do something", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("initial status = %q, want pending", task.Status)
	}

	// Wait for background execution to finish
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := s.Get(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == TaskStatusCompleted || got.Status == TaskStatusFailed || got.Status == TaskStatusAborted {
			// Verify plan was populated
			if len(got.Plan) < 2 {
				t.Errorf("plan has %d steps, want >= 2", len(got.Plan))
			}
			if got.Progress != 100 && got.Status == TaskStatusCompleted {
				t.Errorf("progress = %d, want 100", got.Progress)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("task did not complete within deadline")
}

func TestRunner_ConcurrencyLimit(t *testing.T) {
	s := testStore(t)
	// Pre-create running tasks to hit the limit
	ctx := context.Background()
	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting})
	_ = s.Create(ctx, &Task{ID: "t2", UserID: "u1", Goal: "g", Status: TaskStatusExecuting})

	m := &mockLLM{planJSON: `[{"description":"x"}]`}
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{MaxConcurrent: 2})

	_, err := runner.Submit(ctx, "u1", "another", "", "")
	if err == nil {
		t.Error("expected concurrency limit error")
	}
}

func TestRunner_Cancel(t *testing.T) {
	s := testStore(t)
	// Use a slow mock that blocks
	slowLLM := &slowMockLLM{delay: 2 * time.Second}
	runner := NewRunner(s, slowLLM, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	ctx := context.Background()
	task, err := runner.Submit(ctx, "u1", "slow task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if !runner.Cancel(task.ID) {
		t.Error("Cancel returned false, expected true")
	}

	// Non-existent task
	if runner.Cancel("nonexistent") {
		t.Error("Cancel returned true for nonexistent task")
	}
}

type slowMockLLM struct {
	delay time.Duration
}

func (m *slowMockLLM) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: `[{"description":"x"}]`},
	}, nil
}

type captureLLM struct {
	requests []llm.ChatRequest
}

func (m *captureLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.requests = append(m.requests, req)
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "done"},
	}, nil
}

func TestRunner_LLMTools(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&dummyTool{name: "exec", desc: "run commands"})
	registry.Register(&dummyTool{name: "memory", desc: "search memory"})

	runner := &Runner{registry: registry}
	llmTools := runner.llmTools()

	if len(llmTools) != 2 {
		t.Fatalf("got %d tools, want 2", len(llmTools))
	}

	names := map[string]bool{}
	for _, tool := range llmTools {
		names[tool.Name] = true
	}
	if !names["exec"] || !names["memory"] {
		t.Errorf("expected exec and memory tools, got %v", names)
	}
}

func TestRunner_LLMTools_NilRegistry(t *testing.T) {
	runner := &Runner{}
	if got := runner.llmTools(); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestRunner_ExecuteStep_SystemPromptAvoidsBlueCLI(t *testing.T) {
	llmStub := &captureLLM{}
	runner := &Runner{
		llm: llmStub,
	}

	task := &Task{
		ID:   "t1",
		Goal: "implement parser improvements",
		Plan: []PlanStep{
			{Index: 0, Description: "update parser tests", Status: StepStatusRunning},
		},
	}
	step := &task.Plan[0]

	_, err := runner.executeStep(context.Background(), task, step)
	if err != nil {
		t.Fatalf("executeStep returned unexpected error: %v", err)
	}
	if len(llmStub.requests) == 0 {
		t.Fatal("expected at least one LLM request")
	}

	req := llmStub.requests[0]
	if len(req.Messages) < 1 || req.Messages[0].Role != llm.RoleSystem {
		t.Fatalf("expected first message to be system prompt, got: %#v", req.Messages)
	}
	systemPrompt := req.Messages[0].Content
	if !strings.Contains(systemPrompt, "Do not depend on 'blue' CLI subcommands") {
		t.Fatalf("expected no-blue-cli guidance in system prompt, got: %q", systemPrompt)
	}
}

// --- helpers ---

type dummyTool struct {
	name string
	desc string
}

func (d *dummyTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        d.name,
		Description: d.desc,
		Parameters:  map[string]interface{}{"type": "object"},
	}
}

func (d *dummyTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	return "ok", nil
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("got %q", got)
	}
}

// --- Plan JSON round-trip through store ---

func TestStore_PlanRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	steps := []PlanStep{
		{Index: 0, Description: "install deps", Status: StepStatusCompleted, Output: "ok"},
		{Index: 1, Description: "run tests", Status: StepStatusFailed, Output: "error: timeout"},
	}
	task := &Task{ID: "t1", UserID: "u1", Goal: "build", Plan: steps, Status: TaskStatusFailed}
	_ = s.Create(ctx, task)

	got, _ := s.Get(ctx, "t1")
	if len(got.Plan) != 2 {
		t.Fatalf("plan has %d steps, want 2", len(got.Plan))
	}
	if got.Plan[1].Output != "error: timeout" {
		t.Errorf("step 1 output = %q", got.Plan[1].Output)
	}
}

// --- TaskEvent JSON ---

func TestTaskEvent_JSON(t *testing.T) {
	ev := TaskEvent{
		TaskID:    "t1",
		EventType: "task_progress",
		StepIndex: 2,
		Progress:  50,
		Message:   "running step 3",
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var got TaskEvent
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.TaskID != "t1" || got.Progress != 50 {
		t.Errorf("round-trip failed: %+v", got)
	}
}

func TestTaskEvent_DurationMs(t *testing.T) {
	ev := TaskEvent{
		TaskID:     "t1",
		EventType:  "task_step_completed",
		StepIndex:  0,
		DurationMs: 1234,
	}
	b, _ := json.Marshal(ev)
	var got map[string]interface{}
	json.Unmarshal(b, &got)
	if got["duration_ms"] != float64(1234) {
		t.Errorf("duration_ms = %v, want 1234", got["duration_ms"])
	}
}

func TestTaskEvent_DurationMs_OmitEmpty(t *testing.T) {
	ev := TaskEvent{TaskID: "t1", EventType: "task_progress"}
	b, _ := json.Marshal(ev)
	var got map[string]interface{}
	json.Unmarshal(b, &got)
	if _, ok := got["duration_ms"]; ok {
		t.Error("duration_ms should be omitted when zero")
	}
}

// --- Store: RecoverStaleTasks ---

func TestStore_RecoverStaleTasks(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create tasks in various states
	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting})
	_ = s.Create(ctx, &Task{ID: "t2", UserID: "u1", Goal: "g", Status: TaskStatusPlanning})
	_ = s.Create(ctx, &Task{ID: "t3", UserID: "u1", Goal: "g", Status: TaskStatusPending})
	_ = s.Create(ctx, &Task{ID: "t4", UserID: "u1", Goal: "g", Status: TaskStatusCompleted})
	_ = s.Create(ctx, &Task{ID: "t5", UserID: "u1", Goal: "g", Status: TaskStatusFailed})

	recovered, err := s.RecoverStaleTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 3 {
		t.Errorf("recovered %d, want 3", recovered)
	}

	// Verify the stale tasks are now failed
	for _, id := range []string{"t1", "t2", "t3"} {
		got, _ := s.Get(ctx, id)
		if got.Status != TaskStatusFailed {
			t.Errorf("task %s status = %q, want failed", id, got.Status)
		}
		if got.Error != "interrupted by server restart" {
			t.Errorf("task %s error = %q", id, got.Error)
		}
	}

	// Verify completed/failed tasks are untouched
	got4, _ := s.Get(ctx, "t4")
	if got4.Status != TaskStatusCompleted {
		t.Errorf("task t4 status = %q, want completed", got4.Status)
	}
	got5, _ := s.Get(ctx, "t5")
	if got5.Status != TaskStatusFailed {
		t.Errorf("task t5 status = %q, want failed", got5.Status)
	}
}

func TestStore_RecoverStaleTasks_None(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusCompleted})
	recovered, err := s.RecoverStaleTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 0 {
		t.Errorf("recovered %d, want 0", recovered)
	}
}

// --- Store: edge cases ---

func TestStore_Get_NotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestStore_ListByUser_Empty(t *testing.T) {
	s := testStore(t)
	tasks, err := s.ListByUser(context.Background(), "nobody", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("got %d tasks, want 0", len(tasks))
	}
}

func TestStore_ListByUser_DefaultLimit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_ = s.Create(ctx, &Task{ID: fmt.Sprintf("t%d", i), UserID: "u1", Goal: "g"})
	}
	// Pass 0 limit — should default to 50
	tasks, err := s.ListByUser(ctx, "u1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 5 {
		t.Errorf("got %d tasks, want 5", len(tasks))
	}
}

func TestStore_CountRunning_NoTasks(t *testing.T) {
	s := testStore(t)
	count, err := s.CountRunning(context.Background(), "nobody")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestStore_Cleanup_KeepsRunning(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create an old but still-running task
	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting})
	s.db.ExecContext(ctx, `UPDATE agent_tasks SET created_at=? WHERE id=?`,
		time.Now().Add(-48*time.Hour), "t1")

	n, err := s.Cleanup(ctx, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("cleaned up %d, want 0 (running tasks should be kept)", n)
	}
}

// --- Runner: consecutive failures skip remaining steps ---

type failingLLM struct {
	planJSON    string
	callCount   int
	failCount   int // number of execution calls that should fail (0 = never fail)
	failedSoFar int
}

func (m *failingLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.callCount++
	// Planning call
	if m.callCount == 1 && m.planJSON != "" {
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: m.planJSON},
		}, nil
	}
	// Execution calls: fail up to failCount times
	if m.failCount > 0 && m.failedSoFar < m.failCount {
		m.failedSoFar++
		return nil, fmt.Errorf("simulated LLM error")
	}
	// Remaining calls (verify, summary) succeed
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Done."},
	}, nil
}

type scriptedLLMCall struct {
	content string
	err     error
}

type scriptedLLM struct {
	calls []scriptedLLMCall
	idx   int
}

func (m *scriptedLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.idx >= len(m.calls) {
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: "Done."},
		}, nil
	}
	call := m.calls[m.idx]
	m.idx++
	if call.err != nil {
		return nil, call.err
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: call.content},
	}, nil
}

func TestRunner_ConsecutiveFailures_SkipsRemaining(t *testing.T) {
	s := testStore(t)
	// 5 steps, all will fail
	plan := `[{"description":"s1"},{"description":"s2"},{"description":"s3"},{"description":"s4"},{"description":"s5"}]`
	m := &failingLLM{planJSON: plan, failCount: 6} // 3 steps × 2 (original + retry) = 6 failed calls
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	ctx := context.Background()
	task, err := runner.Submit(ctx, "u1", "fail task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for completion
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		got, err := s.Get(ctx, task.ID)
		if err != nil || got == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if got.Status == TaskStatusCompleted || got.Status == TaskStatusFailed || got.Status == TaskStatusAborted {
			// After 3 consecutive failures, remaining steps should be skipped
			skipped := 0
			failed := 0
			for _, step := range got.Plan {
				if step.Status == StepStatusSkipped {
					skipped++
				}
				if step.Status == StepStatusFailed {
					failed++
				}
			}
			// 3 failed (with retries) + 2 skipped = 5 total
			if failed < 3 {
				t.Errorf("expected >= 3 failed steps, got %d", failed)
			}
			if skipped < 1 {
				t.Errorf("expected >= 1 skipped steps, got %d", skipped)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	runner.Cancel(task.ID)
	t.Error("task did not complete within deadline")
}

// --- Runner: memory recall ---

type mockMemory struct {
	results []MemoryResult
	err     error
	called  bool
	query   string
}

func (m *mockMemory) Recall(_ context.Context, query string, _ int) ([]MemoryResult, error) {
	m.called = true
	m.query = query
	return m.results, m.err
}

func TestRunner_RecallMemories(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{Content: "relevant fact 1", Score: 0.8},
			{Content: "relevant fact 2", Score: 0.6},
			{Content: "low score", Score: 0.3}, // should be filtered
		},
	}
	runner := &Runner{memory: mem}
	ctx := context.Background()

	result := runner.recallMemories(ctx, "test query")
	if !mem.called {
		t.Error("memory.Recall was not called")
	}
	if mem.query != "test query" {
		t.Errorf("query = %q, want %q", mem.query, "test query")
	}
	if !strings.Contains(result, "relevant fact 1") {
		t.Error("result should contain fact 1")
	}
	if !strings.Contains(result, "relevant fact 2") {
		t.Error("result should contain fact 2")
	}
	if strings.Contains(result, "low score") {
		t.Error("result should not contain low-score memory")
	}
}

func TestRunner_RecallMemories_NilMemory(t *testing.T) {
	runner := &Runner{memory: nil}
	result := runner.recallMemories(context.Background(), "query")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestRunner_RecallMemories_EmptyQuery(t *testing.T) {
	mem := &mockMemory{}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
	if mem.called {
		t.Error("should not call Recall with empty query")
	}
}

func TestRunner_RecallMemories_Error(t *testing.T) {
	mem := &mockMemory{err: fmt.Errorf("memory error")}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "query")
	if result != "" {
		t.Errorf("expected empty string on error, got %q", result)
	}
}

func TestRunner_RecallMemories_AllLowScore(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{Content: "low1", Score: 0.2},
			{Content: "low2", Score: 0.4},
		},
	}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "query")
	if result != "" {
		t.Errorf("expected empty string when all scores below threshold, got %q", result)
	}
}

// --- Runner: EnqueueMessage / drainMessages ---

func TestRunner_EnqueueMessage_NotRunning(t *testing.T) {
	runner := &Runner{
		running:   make(map[string]context.CancelFunc),
		msgQueues: make(map[string][]string),
	}
	if runner.EnqueueMessage("nonexistent", "hello") {
		t.Error("expected false for non-running task")
	}
}

func TestRunner_EnqueueAndDrain(t *testing.T) {
	runner := &Runner{
		running:   make(map[string]context.CancelFunc),
		msgQueues: make(map[string][]string),
	}
	// Simulate a running task
	runner.mu.Lock()
	runner.running["t1"] = func() {}
	runner.mu.Unlock()

	if !runner.EnqueueMessage("t1", "msg1") {
		t.Error("expected true for running task")
	}
	if !runner.EnqueueMessage("t1", "msg2") {
		t.Error("expected true for running task")
	}

	msgs := runner.drainMessages("t1")
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0] != "msg1" || msgs[1] != "msg2" {
		t.Errorf("messages = %v", msgs)
	}

	// Drain again — should be empty
	msgs2 := runner.drainMessages("t1")
	if len(msgs2) != 0 {
		t.Errorf("expected 0 messages after drain, got %d", len(msgs2))
	}
}

// --- Runner: SetMemory ---

func TestRunner_SetMemory(t *testing.T) {
	runner := &Runner{}
	if runner.memory != nil {
		t.Error("memory should be nil initially")
	}
	mem := &mockMemory{}
	runner.SetMemory(mem)
	if runner.memory != mem {
		t.Error("memory was not set")
	}
}

// --- Runner: with conversation context ---

func TestRunner_SubmitWithContext(t *testing.T) {
	s := testStore(t)
	plan := `[{"description":"step one"}]`
	m := &mockLLM{planJSON: plan}
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	ctx := context.Background()
	task, err := runner.Submit(ctx, "u1", "do something", "conv-123", "recent chat context here")
	if err != nil {
		t.Fatal(err)
	}
	if task.ConversationID != "conv-123" {
		t.Errorf("conversation_id = %q, want conv-123", task.ConversationID)
	}

	// Wait for completion
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := s.Get(ctx, task.ID)
		if got != nil && (got.Status == TaskStatusCompleted || got.Status == TaskStatusFailed || got.Status == TaskStatusAborted) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	runner.Cancel(task.ID)
	t.Error("task did not complete within deadline")
}

// --- Plan generation edge cases ---

func TestUnmarshalPlan_InvalidJSON(t *testing.T) {
	got := UnmarshalPlan("not json")
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
}

func TestMarshalPlan_Empty(t *testing.T) {
	data := MarshalPlan(nil)
	if data != "null" {
		t.Errorf("expected null for nil plan, got %q", data)
	}
}

func TestMarshalPlan_WithTimestamps(t *testing.T) {
	now := time.Now()
	steps := []PlanStep{
		{Index: 0, Description: "s1", Status: StepStatusCompleted, StartedAt: &now, CompletedAt: &now},
	}
	data := MarshalPlan(steps)
	got := UnmarshalPlan(data)
	if len(got) != 1 {
		t.Fatalf("got %d steps, want 1", len(got))
	}
	if got[0].StartedAt == nil {
		t.Error("StartedAt should be preserved")
	}
}

// --- Truncate edge cases ---

func TestTruncate_ExactLength(t *testing.T) {
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncate_Empty(t *testing.T) {
	if got := truncate("", 10); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- Store: Update with plan ---

func TestStore_UpdatePlan(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting}
	_ = s.Create(ctx, task)

	now := time.Now()
	task.Plan = []PlanStep{
		{Index: 0, Description: "step 1", Status: StepStatusCompleted, Output: "done", StartedAt: &now, CompletedAt: &now},
		{Index: 1, Description: "step 2", Status: StepStatusRunning, StartedAt: &now},
		{Index: 2, Description: "step 3", Status: StepStatusSkipped},
	}
	task.CurrentStep = 1
	task.Progress = 33
	_ = s.Update(ctx, task)

	got, _ := s.Get(ctx, "t1")
	if len(got.Plan) != 3 {
		t.Fatalf("plan has %d steps, want 3", len(got.Plan))
	}
	if got.Plan[0].Status != StepStatusCompleted {
		t.Errorf("step 0 status = %q", got.Plan[0].Status)
	}
	if got.Plan[2].Status != StepStatusSkipped {
		t.Errorf("step 2 status = %q, want skipped", got.Plan[2].Status)
	}
	if got.CurrentStep != 1 {
		t.Errorf("current_step = %d, want 1", got.CurrentStep)
	}
}

// --- Store: Create with default status ---

func TestStore_CreateDefaultStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "t1", UserID: "u1", Goal: "g"} // no status set
	_ = s.Create(ctx, task)

	got, _ := s.Get(ctx, "t1")
	if got.Status != TaskStatusPending {
		t.Errorf("status = %q, want pending", got.Status)
	}
}

// --- Store: SetStatus on nonexistent task (no error, just no-op) ---

func TestStore_SetStatus_Nonexistent(t *testing.T) {
	s := testStore(t)
	err := s.SetStatus(context.Background(), "nonexistent", TaskStatusFailed, "err")
	if err != nil {
		t.Errorf("expected no error for nonexistent task, got %v", err)
	}
}

// --- Store: Delete nonexistent (no error) ---

func TestStore_Delete_Nonexistent(t *testing.T) {
	s := testStore(t)
	err := s.Delete(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// --- AskUser / SubmitAnswers tests ---

func TestRunner_AskUser_SubmitAnswers(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "ask1", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})
	// Mark task as running so AskUser can find it
	r.mu.Lock()
	r.running["ask1"] = func() {}
	r.mu.Unlock()

	questions := []AgentQuestion{
		{ID: "q1", Question: "Pick a color", Header: "Color", Options: []QuestionOption{
			{Label: "Red", Value: "red"},
			{Label: "Blue", Value: "blue"},
		}},
	}

	// Submit answers in a goroutine (AskUser blocks)
	go func() {
		time.Sleep(50 * time.Millisecond)
		ok := r.SubmitAnswers("ask1", []QuestionAnswer{
			{QuestionID: "q1", Values: []string{"blue"}},
		})
		if !ok {
			t.Error("SubmitAnswers returned false")
		}
	}()

	answers, err := r.AskUser(ctx, "ask1", questions, 0)
	if err != nil {
		t.Fatalf("AskUser error: %v", err)
	}
	if len(answers) != 1 || answers[0].QuestionID != "q1" || answers[0].Values[0] != "blue" {
		t.Errorf("unexpected answers: %+v", answers)
	}
}

func TestRunner_AskUser_Timeout(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	task := &Task{ID: "ask2", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})
	r.mu.Lock()
	r.running["ask2"] = func() {}
	r.mu.Unlock()

	questions := []AgentQuestion{{ID: "q1", Question: "Pick one", Header: "Q"}}
	_, err := r.AskUser(ctx, "ask2", questions, 0)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got: %v", err)
	}
}

func TestRunner_AskUser_TimeoutDefaultAction(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "ask_default", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{
		AskTimeout:       20 * time.Millisecond,
		AskTimeoutAction: "default",
	})
	questions := []AgentQuestion{{
		ID:       "q1",
		Question: "Pick one",
		Header:   "Q",
		Options: []QuestionOption{
			{Label: "A (Recommended)", Value: "a"},
			{Label: "B", Value: "b"},
		},
	}}
	answers, err := r.AskUser(ctx, "ask_default", questions, 0)
	if err != nil {
		t.Fatalf("AskUser should fallback to defaults, got error: %v", err)
	}
	if len(answers) != 1 || len(answers[0].Values) != 1 || answers[0].Values[0] != "a" {
		t.Fatalf("unexpected default answers: %+v", answers)
	}
}

func TestRunner_AskUser_DynamicTimeoutOverride(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task := &Task{ID: "ask_override", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{
		AskTimeout:       2 * time.Minute,
		AskTimeoutAction: "default",
	})
	r.SetAskTimeoutFunc(func() time.Duration { return 25 * time.Millisecond })
	start := time.Now()
	_, err := r.AskUser(ctx, "ask_override", []AgentQuestion{{
		ID:       "q1",
		Question: "Pick one",
		Header:   "Q",
		Options:  []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
	}}, 0)
	if err != nil {
		t.Fatalf("AskUser should fallback to defaults, got error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("dynamic timeout override not applied, elapsed=%s", elapsed)
	}
}

func TestRunner_SubmitAnswers_NoPending(t *testing.T) {
	s := testStore(t)
	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})

	ok := r.SubmitAnswers("nonexistent", []QuestionAnswer{
		{QuestionID: "q1", Values: []string{"yes"}},
	})
	if ok {
		t.Error("expected false for non-pending task")
	}
}

func TestRunner_AskUser_Cleanup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "ask3", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})
	r.mu.Lock()
	r.running["ask3"] = func() {}
	r.mu.Unlock()

	// Start AskUser in background, then cancel context
	askCtx, askCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.AskUser(askCtx, "ask3", []AgentQuestion{{ID: "q1", Question: "test", Header: "Q"}}, 0)
	}()

	time.Sleep(20 * time.Millisecond)
	askCancel()
	<-done

	// Verify channel was cleaned up
	r.askMu.Lock()
	_, exists := r.askQueues["ask3"]
	r.askMu.Unlock()
	if exists {
		t.Error("askQueues should be cleaned up after AskUser returns")
	}
}

func TestRunner_HandleAskUser_InvalidArgs(t *testing.T) {
	s := testStore(t)
	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})

	task := &Task{ID: "ask4", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	result := r.handleAskUser(context.Background(), task, `{"invalid": true}`)
	if !strings.Contains(result, "error") {
		t.Errorf("expected error in result, got: %s", result)
	}

	result2 := r.handleAskUser(context.Background(), task, `not json`)
	if !strings.Contains(result2, "error") {
		t.Errorf("expected error for bad JSON, got: %s", result2)
	}
}

func TestRunner_HandleAskUser_EmptyQuestions(t *testing.T) {
	s := testStore(t)
	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})

	task := &Task{ID: "ask5", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	result := r.handleAskUser(context.Background(), task, `{"questions": []}`)
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for empty questions, got: %s", result)
	}
}

func TestRunner_HandleAskUser_ObjectOptions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task := &Task{ID: "ask6", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{
		AskTimeout:       20 * time.Millisecond,
		AskTimeoutAction: "default",
	})

	result := r.handleAskUser(ctx, task, `{"q":"Choose strategy","a":[{"label":"Balanced (Recommended)","description":"safe","value":"balanced"},{"label":"Fast","value":"fast"}]}`)
	if strings.Contains(result, `"error"`) {
		t.Fatalf("expected success for object options, got: %s", result)
	}
	if !strings.Contains(result, "balanced") {
		t.Fatalf("expected default selection from first option object, got: %s", result)
	}
}

func TestRunner_VerifyRecoveryFailure_MarksFailed(t *testing.T) {
	s := testStore(t)
	m := &scriptedLLM{
		calls: []scriptedLLMCall{
			{content: `{"goal":"build","subtasks":[{"description":"primary step"}],"success_criteria":["verify passes"],"fallback_plan":["recover once"]}`},
			{content: "step completed"},
			{err: fmt.Errorf("verify failed")},
			{err: fmt.Errorf("recover failed")},
		},
	}
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	ctx := context.Background()
	task, err := runner.Submit(ctx, "u1", "execute and verify", "", "")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := s.Get(ctx, task.ID)
		if err == nil && got != nil && (got.Status == TaskStatusCompleted || got.Status == TaskStatusFailed || got.Status == TaskStatusAborted) {
			if got.Status != TaskStatusFailed {
				t.Fatalf("status = %q, want failed", got.Status)
			}
			if got.RuntimeState != RuntimeStateAborted {
				t.Fatalf("runtime_state = %q, want ABORTED", got.RuntimeState)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	runner.Cancel(task.ID)
	t.Fatal("task did not reach terminal state within deadline")
}

func TestAgentQuestion_JSON(t *testing.T) {
	q := AgentQuestion{
		ID:          "q1",
		Question:    "Pick a framework",
		Header:      "Framework",
		MultiSelect: false,
		Required:    true,
		Options: []QuestionOption{
			{Label: "React", Value: "react", Description: "Popular UI library"},
			{Label: "Vue", Value: "vue"},
		},
	}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	var got AgentQuestion
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "q1" || got.Header != "Framework" || len(got.Options) != 2 {
		t.Errorf("round-trip failed: %+v", got)
	}
}

func TestQuestionAnswer_JSON(t *testing.T) {
	a := QuestionAnswer{
		QuestionID: "q1",
		Values:     []string{"react", "vue"},
		OtherText:  "also svelte",
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var got QuestionAnswer
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.QuestionID != "q1" || len(got.Values) != 2 || got.OtherText != "also svelte" {
		t.Errorf("round-trip failed: %+v", got)
	}
}

func TestTaskEvent_Questions(t *testing.T) {
	ev := TaskEvent{
		TaskID:    "t1",
		EventType: "task_question",
		Questions: []AgentQuestion{
			{ID: "q1", Question: "Pick one", Header: "Choice"},
		},
	}
	b, _ := json.Marshal(ev)
	if !strings.Contains(string(b), `"task_question"`) {
		t.Error("missing event_type in JSON")
	}
	if !strings.Contains(string(b), `"questions"`) {
		t.Error("missing questions in JSON")
	}
}

func TestTaskEvent_NoQuestions_OmitsField(t *testing.T) {
	ev := TaskEvent{
		TaskID:    "t1",
		EventType: "task_progress",
	}
	b, _ := json.Marshal(ev)
	if strings.Contains(string(b), `"questions"`) {
		t.Error("questions should be omitted when empty")
	}
}
