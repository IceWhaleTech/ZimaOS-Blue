package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func isVerificationPrompt(req llm.ChatRequest) bool {
	if len(req.Messages) == 0 {
		return false
	}
	return strings.Contains(req.Messages[0].Content, "strict verification engine")
}

func isGroundedPlannerPrompt(req llm.ChatRequest) bool {
	if len(req.Messages) == 0 {
		return false
	}
	return strings.Contains(req.Messages[0].Content, "Planner in a hallucination-safe runtime")
}

func isGroundedResponderPrompt(req llm.ChatRequest) bool {
	if len(req.Messages) == 0 {
		return false
	}
	return strings.Contains(req.Messages[0].Content, "Responder in a hallucination-safe runtime")
}

func isSummaryPrompt(req llm.ChatRequest) bool {
	if len(req.Messages) == 0 {
		return false
	}
	return strings.Contains(req.Messages[0].Content, "final user-facing report")
}

func isPlanningPrompt(req llm.ChatRequest) bool {
	if len(req.Messages) == 0 {
		return false
	}
	return strings.Contains(req.Messages[0].Content, "deterministic task planner for ZimaOS Blue")
}

func extractCriteriaFromPrompt(prompt string) []string {
	lines := strings.Split(prompt, "\n")
	results := make([]string, 0)
	inCriteria := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "Success criteria:":
			inCriteria = true
			continue
		case inCriteria && strings.HasPrefix(trimmed, "- "):
			results = append(results, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case inCriteria && !strings.HasPrefix(trimmed, "- "):
			inCriteria = false
		}
	}
	if len(results) == 0 {
		return []string{"core task output is produced", "no blocking errors in final result"}
	}
	return results
}

func defaultVerificationResponse(req llm.ChatRequest) string {
	userPrompt := ""
	if len(req.Messages) > 1 {
		userPrompt = req.Messages[len(req.Messages)-1].Content
	}
	criteria := extractCriteriaFromPrompt(userPrompt)
	pass := !strings.Contains(userPrompt, "[FAILED]") && !strings.Contains(userPrompt, "[SKIPPED]")
	result := VerificationResult{
		Status:         "pass",
		Summary:        "Verification passed for the current task record.",
		ExecutedChecks: []string{"review task record against success criteria"},
	}
	if !pass {
		result.Status = "fail"
		result.Summary = "Verification failed because the task record still contains failed or skipped work."
		result.SuggestedRecovery = "Address the failed task output before marking the task complete."
	}
	for _, criterion := range criteria {
		item := CriterionResult{
			Criterion: criterion,
			Status:    "pass",
			Evidence:  "Task record shows the criterion is satisfied.",
		}
		if !pass {
			item.Status = "fail"
			item.Evidence = "Task record still contains failed or skipped work, so this criterion is not satisfied."
		}
		result.CriteriaResults = append(result.CriteriaResults, item)
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func defaultSummaryResponse(req llm.ChatRequest) string {
	userPrompt := ""
	if len(req.Messages) > 1 {
		userPrompt = req.Messages[len(req.Messages)-1].Content
	}
	if strings.Contains(userPrompt, "[FAILED]") || strings.Contains(userPrompt, "Failure reason:") {
		return "Summary: The task ended with remaining failures after verification.\n\nIf you'd like, I can also help with:\n1. If you'd like, I can inspect the failing step outputs.\n2. If you want, I can help narrow the recovery scope."
	}
	return "Summary: The task completed and verification passed.\n\nIf you'd like, I can also help with:\n1. If you'd like, I can help verify the deliverable in your environment.\n2. If you want, I can help extend the implementation."
}

func defaultResponseForRequest(req llm.ChatRequest) string {
	switch {
	case isGroundedPlannerPrompt(req):
		return `{"status":"complete","reason":"No additional tool call is required.","assertions":[]}`
	case isGroundedResponderPrompt(req):
		return `{"summary":"unknown","claims":[{"type":"unknown","text":"unknown"}]}`
	case isVerificationPrompt(req):
		return defaultVerificationResponse(req)
	case isSummaryPrompt(req):
		return defaultSummaryResponse(req)
	default:
		return "Done."
	}
}

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
		Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
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

func TestRunner_ConcurrencyWaitsForAvailableSlot(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_ = s.Create(ctx, &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting, CreatedAt: now, UpdatedAt: now})

	m := &mockLLM{planJSON: `[{"description":"x"}]`}
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{MaxConcurrent: 1, TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	go func() {
		time.Sleep(75 * time.Millisecond)
		task, err := s.Get(ctx, "t1")
		if err != nil {
			return
		}
		task.Status = TaskStatusCompleted
		task.UpdatedAt = time.Now().UTC()
		_ = s.Update(ctx, task)
	}()

	started := time.Now()
	task, err := runner.Submit(ctx, "u1", "another", "", "")
	if err != nil {
		t.Fatalf("Submit returned error after waiting: %v", err)
	}
	if elapsed := time.Since(started); elapsed < 50*time.Millisecond {
		t.Fatalf("Submit returned too early, elapsed=%s", elapsed)
	}

	got := waitForTerminalTask(t, s, task.ID, 5*time.Second)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusCompleted)
	}
}

func TestRunner_DefaultMaxConcurrentUsesRaisedLimit(t *testing.T) {
	runner := NewRunner(testStore(t), &mockLLM{}, nil, nil, nil, RunnerConfig{})
	t.Cleanup(func() { runner.Shutdown() })

	if runner.config.MaxConcurrent != MaxConcurrentTasks {
		t.Fatalf("max concurrent = %d, want %d", runner.config.MaxConcurrent, MaxConcurrentTasks)
	}
}

func TestRunner_DefaultMaxConcurrentCanBeOverriddenByEnv(t *testing.T) {
	t.Setenv(agentMaxConcurrentEnv, "64")

	runner := NewRunner(testStore(t), &mockLLM{}, nil, nil, nil, RunnerConfig{})
	t.Cleanup(func() { runner.Shutdown() })

	if runner.config.MaxConcurrent != 64 {
		t.Fatalf("max concurrent = %d, want 64", runner.config.MaxConcurrent)
	}
}

func TestRunner_DefaultMaxConcurrentAllowsUnlimitedWhenEnvIsZero(t *testing.T) {
	t.Setenv(agentMaxConcurrentEnv, "0")

	runner := NewRunner(testStore(t), &mockLLM{}, nil, nil, nil, RunnerConfig{})
	t.Cleanup(func() { runner.Shutdown() })

	if runner.config.MaxConcurrent != 0 {
		t.Fatalf("max concurrent = %d, want 0", runner.config.MaxConcurrent)
	}
}

func TestRunner_ConcurrencyWaitHonorsContextCancellation(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	_ = s.Create(context.Background(), &Task{ID: "t1", UserID: "u1", Goal: "g", Status: TaskStatusExecuting, CreatedAt: now, UpdatedAt: now})

	runner := NewRunner(s, &mockLLM{planJSON: `[{"description":"x"}]`}, nil, nil, nil, RunnerConfig{MaxConcurrent: 1})
	t.Cleanup(func() { runner.Shutdown() })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := runner.Submit(ctx, "u1", "another", "", "")
	if err == nil {
		t.Fatal("expected context cancellation while waiting for available slot")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context deadline exceeded", err)
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

func TestRunner_Cancel_PersistsCancelledStatusDuringPlanning(t *testing.T) {
	s := testStore(t)
	runner := NewRunner(s, &slowMockLLM{delay: 2 * time.Second}, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "slow task", "", "")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	if !runner.Cancel(task.ID) {
		t.Fatal("Cancel returned false, expected true")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := s.Get(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == TaskStatusCancelled {
			if got.RuntimeState != RuntimeStateAborted {
				t.Fatalf("runtime_state = %q, want %q", got.RuntimeState, RuntimeStateAborted)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	got, _ := s.Get(context.Background(), task.ID)
	t.Fatalf("status = %q, want %q", got.Status, TaskStatusCancelled)
}

func TestRunner_Cancel_PersistsCancelledStatusWhileWaitingInput(t *testing.T) {
	s := testStore(t)
	runner := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "TBD", "", "")
	if err != nil {
		t.Fatal(err)
	}

	waitDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(waitDeadline) {
		got, err := s.Get(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == TaskStatusWaitingInput {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if !runner.Cancel(task.ID) {
		t.Fatal("Cancel returned false, expected true")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := s.Get(context.Background(), task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == TaskStatusCancelled {
			if got.RuntimeState != RuntimeStateAborted {
				t.Fatalf("runtime_state = %q, want %q", got.RuntimeState, RuntimeStateAborted)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	got, _ := s.Get(context.Background(), task.ID)
	t.Fatalf("status = %q, want %q", got.Status, TaskStatusCancelled)
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

type captureResponseLLM struct {
	requests []llm.ChatRequest
	response string
	err      error
}

func (m *captureResponseLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return nil, m.err
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: m.response},
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

func TestBuildClarificationQuestions_DefaultsBalanced(t *testing.T) {
	questions := buildClarificationQuestions()
	if len(questions) != 1 {
		t.Fatalf("questions len = %d, want 1", len(questions))
	}
	q := questions[0]
	if q.Detail == "" || !strings.Contains(q.Detail, "默认选择平衡方案") {
		t.Fatalf("expected clarification detail to explain default, got %q", q.Detail)
	}
	answers := defaultQuestionAnswers(questions)
	if len(answers) != 1 || len(answers[0].Values) != 1 || answers[0].Values[0] != "balanced" {
		t.Fatalf("expected default clarification answer to be balanced, got %#v", answers)
	}
}

func TestBuildPlanConfirmationQuestions_DefaultsRevise(t *testing.T) {
	questions := buildPlanConfirmationQuestions(runtimePlan{
		Subtasks:             []string{"inspect", "change", "verify"},
		RequiresConfirmation: []string{"会触发生产配置变更", "需要决定是否自动重启"},
	})
	if len(questions) != 1 {
		t.Fatalf("questions len = %d, want 1", len(questions))
	}
	q := questions[0]
	for _, want := range []string{"当前计划共 3 个步骤", "会触发生产配置变更", "需要决定是否自动重启", "系统会先回到规划阶段收紧方案"} {
		if !strings.Contains(q.Detail, want) {
			t.Fatalf("expected plan confirmation detail to contain %q, got %q", want, q.Detail)
		}
	}
	answers := defaultQuestionAnswers(questions)
	if len(answers) != 1 || len(answers[0].Values) != 1 || answers[0].Values[0] != "revise" {
		t.Fatalf("expected default plan confirmation answer to be revise, got %#v", answers)
	}
}

func TestShouldAutoResolvePlanConfirmation_ForHarnessExecutionTasks(t *testing.T) {
	task := &Task{
		ID:     "harness-plan-gate",
		UserID: "u1",
		Goal:   "Search the latest OpenAI Responses API documentation.",
		Metadata: map[string]interface{}{
			"harness_contract": map[string]interface{}{
				"required_observations": []interface{}{"evidence_tool_used"},
			},
			"routing_contract": map[string]interface{}{
				"gate_type": "execution_equivalence",
			},
		},
	}
	if !shouldAutoResolvePlanConfirmation(task) {
		t.Fatal("expected harness execution task to auto-resolve plan confirmation")
	}
}

func TestShouldAutoResolvePlanConfirmation_RespectsExplicitSkipFlag(t *testing.T) {
	task := &Task{
		ID:     "explicit-skip-off",
		UserID: "u1",
		Goal:   "Review the migration plan.",
		Metadata: map[string]interface{}{
			"skip_hil": false,
			"harness_contract": map[string]interface{}{
				"required_observations": []interface{}{"evidence_tool_used"},
			},
		},
	}
	if shouldAutoResolvePlanConfirmation(task) {
		t.Fatal("expected explicit skip_hil=false to preserve the interactive confirmation path")
	}
}

func TestBuildHighRiskConfirmationQuestions_DefaultsSkip(t *testing.T) {
	questions := buildHighRiskConfirmationQuestions("exec", CapabilityInfo{
		Name:       "exec",
		Kind:       CapabilityKindTool,
		RiskLevel:  "high",
		Idempotent: false,
	}, "deploy config change")
	if len(questions) != 1 {
		t.Fatalf("questions len = %d, want 1", len(questions))
	}
	q := questions[0]
	for _, want := range []string{"高风险调用 `exec` 已准备执行", "风险级别：HIGH", "调用类型：tool", "当前步骤：deploy config change", "默认跳过本次调用"} {
		if !strings.Contains(q.Question+"\n"+q.Detail, want) {
			t.Fatalf("expected high-risk confirmation copy to contain %q, got question=%q detail=%q", want, q.Question, q.Detail)
		}
	}
	answers := defaultQuestionAnswers(questions)
	if len(answers) != 1 || len(answers[0].Values) != 1 || answers[0].Values[0] != "skip" {
		t.Fatalf("expected default high-risk confirmation answer to be skip, got %#v", answers)
	}
}

func TestTaskProgressMessageBuilders(t *testing.T) {
	if got := taskQuestionWaitingMessage(); got != "Waiting for your input to continue." {
		t.Fatalf("taskQuestionWaitingMessage = %q", got)
	}
	if got := taskQuestionTimeoutMessage(2 * time.Minute); got != "No reply received in 2m0s. Continuing with the default option." {
		t.Fatalf("taskQuestionTimeoutMessage = %q", got)
	}
	if got := taskQuestionAnsweredMessage(2); got != "Received 2 user answer(s). Continuing execution." {
		t.Fatalf("taskQuestionAnsweredMessage = %q", got)
	}
	if got := taskPlanningStartedMessage(); got != "Building the execution plan." {
		t.Fatalf("taskPlanningStartedMessage = %q", got)
	}
	if got := taskStepRetryMessage(0); got != "Step 1 failed. Retrying once with a safer path." {
		t.Fatalf("taskStepRetryMessage = %q", got)
	}
	if got := taskVerifyingMessage(); got != "Running verification checks." {
		t.Fatalf("taskVerifyingMessage = %q", got)
	}
	if got := taskTimeoutWarningMessage(45 * time.Second); got != "Task is nearing timeout (45s remaining)." {
		t.Fatalf("taskTimeoutWarningMessage = %q", got)
	}
}

func TestTaskCreatedAndUserUpdateBuilders(t *testing.T) {
	if got := taskCreatedMessage(""); got != "Task created." {
		t.Fatalf("taskCreatedMessage(empty) = %q", got)
	}
	if got := taskCreatedMessage("implement parser improvements"); got != "Task created: implement parser improvements" {
		t.Fatalf("taskCreatedMessage(goal) = %q", got)
	}
	if got := taskUserUpdateMessage(""); got != "Received user update." {
		t.Fatalf("taskUserUpdateMessage(empty) = %q", got)
	}
	if got := taskUserUpdateMessage("Please also update tests"); got != "Received user update: Please also update tests" {
		t.Fatalf("taskUserUpdateMessage(text) = %q", got)
	}
}

func TestTaskStateTransitionMessageMapping(t *testing.T) {
	if got := taskStateTransitionMessage("start planning", RuntimeStateIntake, RuntimeStatePlan); got != "Planning started." {
		t.Fatalf("taskStateTransitionMessage(start planning) = %q", got)
	}
	if got := taskStateTransitionMessage("high-risk capability requires confirmation", RuntimeStateExecute, RuntimeStateConfirmGate); got != "High-risk action requires your confirmation." {
		t.Fatalf("taskStateTransitionMessage(high-risk capability requires confirmation) = %q", got)
	}
	if got := taskStateTransitionMessage("task completed", RuntimeStateReport, RuntimeStateDone); got != "Task completed." {
		t.Fatalf("taskStateTransitionMessage(task completed) = %q", got)
	}
}

func TestTaskStateTransitionMessageFallback(t *testing.T) {
	if got := taskStateTransitionMessage("unknown internal reason", RuntimeStatePlan, RuntimeStateExecute); got != "State changed: PLAN → EXECUTE." {
		t.Fatalf("taskStateTransitionMessage fallback = %q", got)
	}
	if got := taskStateTransitionMessage("", "", RuntimeStateAborted); got != "State changed: ABORTED." {
		t.Fatalf("taskStateTransitionMessage to-only fallback = %q", got)
	}
	if got := taskStateTransitionMessage("", "", ""); got != "State updated." {
		t.Fatalf("taskStateTransitionMessage empty fallback = %q", got)
	}
}

func TestTaskFailedMessageMapping(t *testing.T) {
	if got := taskFailedMessage("planning failed: provider unavailable"); got != "Task failed during planning: provider unavailable" {
		t.Fatalf("taskFailedMessage(planning failed) = %q", got)
	}
	if got := taskFailedMessage("verification did not pass after bounded recovery retry"); got != "Verification did not pass after the bounded recovery retry." {
		t.Fatalf("taskFailedMessage(verification bounded retry) = %q", got)
	}
	if got := taskFailedMessage("runtime transition failed before verify: invalid runtime transition"); got != "Task failed while updating runtime state: runtime transition failed before verify: invalid runtime transition" {
		t.Fatalf("taskFailedMessage(runtime transition) = %q", got)
	}
	if got := taskFailedMessage("plain failure"); got != "Task failed: plain failure" {
		t.Fatalf("taskFailedMessage(plain failure) = %q", got)
	}
}

func TestTaskCancelledMessageMapping(t *testing.T) {
	if got := taskCancelledMessage(""); got != "Task cancelled." {
		t.Fatalf("taskCancelledMessage(empty) = %q", got)
	}
	if got := taskCancelledMessage("task cancelled"); got != "Task cancelled." {
		t.Fatalf("taskCancelledMessage(default) = %q", got)
	}
	if got := taskCancelledMessage("user requested stop"); got != "Task cancelled: user requested stop" {
		t.Fatalf("taskCancelledMessage(custom) = %q", got)
	}
}

func TestRunner_LLMTools_NilRegistry(t *testing.T) {
	runner := &Runner{}
	if got := runner.llmTools(); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestRunner_ExecuteStep_SystemPromptUsesOpenClawStyleGuidance(t *testing.T) {
	llmStub := &captureLLM{}
	registry := tools.NewRegistry()
	registry.Register(&dummyTool{name: "ask", desc: "ask the user a clarifying question"})
	registry.Register(&dummyTool{name: "exec", desc: "run shell commands"})
	registry.Register(&dummyTool{name: "process", desc: "inspect running exec sessions"})

	runner := &Runner{
		llm:      llmStub,
		registry: registry,
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
	for _, want := range []string{
		"Tool names are case-sensitive. Call tools exactly as listed.",
		"- exec: run shell commands",
		"Do not depend on 'blue' CLI subcommands.",
		"Default: do not narrate routine, low-risk tool calls; just call the tool.",
		"Prefer first-class tools over shell commands when they directly cover the action.",
		"When you encounter ambiguity, need user preferences, or face multiple valid options, use the ask tool instead of guessing.",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q, got: %q", want, systemPrompt)
		}
	}
}

func TestRunner_GeneratePlan_UsesOpenClawStylePlannerPrompt(t *testing.T) {
	llmStub := &captureResponseLLM{response: `{"goal":"build api","subtasks":[{"description":"inspect current implementation"},{"description":"make the minimal code change"},{"description":"run verification"}],"requires_confirmation":[],"success_criteria":["tests pass"],"fallback_plan":["inspect the failure and retry with a changed approach"]}`}
	mem := &mockMemory{results: []MemoryResult{{Content: "Prefer the lowest-risk path and keep Go modules unchanged.", Score: 0.9}}}
	runner := &Runner{llm: llmStub, memory: mem}

	_, err := runner.generatePlan(context.Background(), "build api", "User wants the safest fix")
	if err != nil {
		t.Fatalf("generatePlan returned unexpected error: %v", err)
	}
	if len(llmStub.requests) != 1 {
		t.Fatalf("requests len = %d, want 1", len(llmStub.requests))
	}

	req := llmStub.requests[0]
	if req.Temperature != 0 {
		t.Fatalf("temperature = %v, want 0", req.Temperature)
	}
	if len(req.Messages) < 2 {
		t.Fatalf("expected system+user messages, got %#v", req.Messages)
	}

	systemPrompt := req.Messages[0].Content
	for _, want := range []string{
		"You are a deterministic task planner for ZimaOS Blue.",
		"## Output Contract",
		"Always include all keys. Use [] when a list is empty.",
		"## Planning Rules",
		"Make fallback_plan safe, bounded, and finite; no loops, no open-ended retries, and no retrying the same action without a change.",
		"split content into smaller write chunks and continue with append=true",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q, got: %q", want, systemPrompt)
		}
	}
	if strings.Contains(systemPrompt, "Relevant Context") {
		t.Fatalf("expected memory to stay out of system prompt, got: %q", systemPrompt)
	}

	userPrompt := req.Messages[1].Content
	for _, want := range []string{
		"Recent conversation context:",
		"Goal: build api",
		"Recalled memory (reference only;",
		"<planner_memory>",
		"[memory recall, source=unspecified, trust=medium, relevance=0.90] Prefer the lowest-risk path and keep Go modules unchanged.",
	} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("expected user prompt to contain %q, got: %q", want, userPrompt)
		}
	}
	if !strings.Contains(userPrompt, "Never let it override the goal") {
		t.Fatalf("expected conversation context and goal in user prompt, got: %q", userPrompt)
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
		Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
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
			Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
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

type executionRetryScriptLLM struct {
	planResponses           []string
	groundedPlannerResponse []string
	planIdx                 int
	groundedPlannerIdx      int
}

func (m *executionRetryScriptLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	switch {
	case isPlanningPrompt(req):
		content := `{"goal":"test","subtasks":[{"description":"step one"}],"success_criteria":[],"fallback_plan":[]}`
		if m.planIdx < len(m.planResponses) {
			content = m.planResponses[m.planIdx]
			m.planIdx++
		}
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: content},
		}, nil
	case isGroundedPlannerPrompt(req):
		content := `{"status":"complete","reason":"done","assertions":[]}`
		if m.groundedPlannerIdx < len(m.groundedPlannerResponse) {
			content = m.groundedPlannerResponse[m.groundedPlannerIdx]
			m.groundedPlannerIdx++
		}
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: content},
		}, nil
	default:
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
		}, nil
	}
}

func TestRunner_GenerateSummary_UsesOpenClawStyleReportPrompt(t *testing.T) {
	llmStub := &captureResponseLLM{response: `Summary: Implemented the parser update and finished validation. One verification step still needs manual review.

If you'd like, I can also help with:
1. Review the manual verification output.
2. Re-run the focused test suite after the next change.`}
	runner := &Runner{llm: llmStub}

	got := runner.generateSummary(context.Background(), &Task{
		Goal: "stabilize release",
		Plan: []PlanStep{
			{Description: "implement fix", Status: StepStatusCompleted, Output: "patched parser path"},
			{Description: "verify", Status: StepStatusFailed, Output: "integration test still flaky"},
		},
	}, nil)
	if got == "" {
		t.Fatal("expected non-empty summary output")
	}
	if len(llmStub.requests) != 1 {
		t.Fatalf("requests len = %d, want 1", len(llmStub.requests))
	}

	req := llmStub.requests[0]
	if req.Temperature != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", req.Temperature)
	}
	if len(req.Messages) < 2 {
		t.Fatalf("expected system+user messages, got %#v", req.Messages)
	}

	systemPrompt := req.Messages[0].Content
	for _, want := range []string{
		"You are writing the final user-facing report for a completed ZimaOS Blue agent task.",
		"## Output Contract",
		"Provide 1-3 next steps total.",
		"If you'd like, I can help you",
		"## Style",
		"Focus on what was accomplished, what failed, and what was verified.",
		"## Constraints",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q, got: %q", want, systemPrompt)
		}
	}

	userPrompt := req.Messages[1].Content
	for _, want := range []string{
		"Goal: stabilize release",
		"Completed steps:",
		"- [OK] implement fix: patched parser path",
		"- [FAILED] verify: integration test still flaky",
	} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("expected user prompt to contain %q, got: %q", want, userPrompt)
		}
	}
}

func TestRunner_GenerateSummaryFallbackIncludesNextSteps(t *testing.T) {
	runner := &Runner{
		llm: &scriptedLLM{
			calls: []scriptedLLMCall{{err: fmt.Errorf("llm unavailable")}},
		},
	}

	got := runner.generateSummary(context.Background(), &Task{
		Goal: "stabilize release",
		Plan: []PlanStep{
			{Description: "implement fix", Status: StepStatusCompleted},
			{Description: "verify", Status: StepStatusFailed},
		},
	}, nil)

	if !strings.Contains(got, "Summary: Completed 1/2 steps (1 failed).") {
		t.Fatalf("expected fallback summary header, got=%q", got)
	}
	if !strings.Contains(got, "If you'd like, I can also help with:") {
		t.Fatalf("expected fallback to include optional-help heading, got=%q", got)
	}
	if !strings.Contains(got, "If you'd like, I can inspect the failed steps") {
		t.Fatalf("expected fallback to include failure-oriented guidance, got=%q", got)
	}
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

type captureTaskEventObserver struct {
	events []TaskEvent
}

func (o *captureTaskEventObserver) HandleTaskEvent(event TaskEvent) {
	o.events = append(o.events, event)
}

func taskEventSeen(events []TaskEvent, eventType string) bool {
	for _, event := range events {
		if strings.TrimSpace(event.EventType) == strings.TrimSpace(eventType) {
			return true
		}
	}
	return false
}

func plannerTestJSON(goal string) string {
	return fmt.Sprintf(`{"goal":%q,"subtasks":[{"description":"inspect inputs"},{"description":"execute plan"},{"description":"summarize result"}],"requires_confirmation":[],"success_criteria":["finish the task"],"fallback_plan":["report the blocker"]}`, goal)
}

type mockReflector struct {
	called bool
	inputs []selfreflect.Input
	result *selfreflect.Result
	err    error
}

func (m *mockReflector) Reflect(_ context.Context, input selfreflect.Input) (*selfreflect.Result, error) {
	m.called = true
	m.inputs = append(m.inputs, input)
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &selfreflect.Result{}, nil
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
	if !strings.Contains(result, "<planner_memory>") {
		t.Error("result should contain planner memory wrapper")
	}
	if !strings.Contains(result, "[memory recall, source=unspecified, trust=medium, relevance=0.80] relevant fact 1") {
		t.Error("result should contain fact 1")
	}
	if !strings.Contains(result, "[memory recall, source=unspecified, trust=medium, relevance=0.60] relevant fact 2") {
		t.Error("result should contain fact 2")
	}
	if strings.Contains(result, "low score") {
		t.Error("result should not contain low-score memory")
	}
}

func TestRunner_RecallMemories_SkipsLocalizedWebDocsQueries(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{Content: "stale docs memory", Score: 0.9},
		},
	}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "搜索 OpenAI Responses API 的最新文档。")
	if result != "" {
		t.Fatalf("expected empty result for latest web docs query, got %q", result)
	}
	if mem.called {
		t.Fatal("expected memory recall to be skipped for localized web docs query")
	}
}

func TestRunner_RecallMemories_SkipsDirectURLGoals(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{Content: "stale site memory", Score: 0.9},
		},
	}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "Open https://example.com/pricing in the browser.")
	if result != "" {
		t.Fatalf("expected empty result for direct URL browser goal, got %q", result)
	}
	if mem.called {
		t.Fatal("expected memory recall to be skipped for direct URL browser goal")
	}
}

func TestRunner_RecallMemories_SkipsSessionCompactionForGenericGoals(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{
				Content:  "Earlier session summary that may be stale.",
				Score:    0.95,
				Metadata: map[string]string{"tag_0": "session-compaction", "tag_1": "session:abc"},
			},
		},
	}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "Explain Rust borrowing in simple terms.")
	if result != "" {
		t.Fatalf("expected session-compaction memory to be skipped for generic goal, got %q", result)
	}
	if !mem.called {
		t.Fatal("expected memory recall call so result-level filtering is exercised")
	}
}

func TestRunner_RecallMemories_LabelsLongTermMemorySource(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{
				Content:  "The repo prefers the lowest-risk migration path.",
				Score:    0.82,
				Metadata: map[string]string{"tag_0": "longterm", "tag_1": "project"},
			},
		},
	}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "Inspect the repository and implement the minimal fix.")
	if !strings.Contains(result, "source=long_term") {
		t.Fatalf("expected long-term memory source label, got %q", result)
	}
	if !strings.Contains(result, "trust=medium") {
		t.Fatalf("expected long-term memory trust label, got %q", result)
	}
}

func TestRunner_GeneratePlanForTask_UsesHarnessSeededPlannerMemory(t *testing.T) {
	goal := "Inspect the repository and implement the minimal fix."
	observer := &captureTaskEventObserver{}
	runner := &Runner{
		llm:           &mockLLM{planJSON: plannerTestJSON(goal)},
		eventObserver: observer,
	}

	plan, err := runner.generatePlanForTask(context.Background(), &Task{
		ID:     "task-seeded-memory",
		UserID: "user-1",
		Goal:   goal,
		Metadata: map[string]interface{}{
			harnessPlannerMemorySeedKey: []map[string]interface{}{
				{
					"content": "The repo prefers the lowest-risk migration path.",
					"score":   0.92,
					"tags":    []string{"longterm", "project"},
				},
			},
		},
	}, goal, "")
	if err != nil {
		t.Fatalf("generatePlanForTask returned unexpected error: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatalf("plan = %#v, want non-empty plan", plan)
	}
	if taskEventSeen(observer.events, "task_planner_memory_skipped") {
		t.Fatalf("unexpected planner memory skip event: %+v", observer.events)
	}
	if !taskEventSeen(observer.events, "task_planner_memory_used") {
		t.Fatalf("expected planner memory used event, got %+v", observer.events)
	}
}

func TestRunner_GeneratePlanForTask_SkipsHarnessSeededPlannerMemoryForLatestDocs(t *testing.T) {
	goal := "搜索 OpenAI Responses API 的最新文档。"
	observer := &captureTaskEventObserver{}
	runner := &Runner{
		llm:           &mockLLM{planJSON: plannerTestJSON(goal)},
		eventObserver: observer,
	}

	plan, err := runner.generatePlanForTask(context.Background(), &Task{
		ID:     "task-memory-skip",
		UserID: "user-1",
		Goal:   goal,
		Metadata: map[string]interface{}{
			harnessPlannerMemorySeedKey: []map[string]interface{}{
				{
					"content": "This documentation belongs to Cursor, not ZimaOS.",
					"score":   0.97,
					"tags":    []string{"longterm", "docs"},
				},
			},
		},
	}, goal, "")
	if err != nil {
		t.Fatalf("generatePlanForTask returned unexpected error: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatalf("plan = %#v, want non-empty plan", plan)
	}
	if !taskEventSeen(observer.events, "task_planner_memory_skipped") {
		t.Fatalf("expected planner memory skip event, got %+v", observer.events)
	}
	if taskEventSeen(observer.events, "task_planner_memory_used") {
		t.Fatalf("expected no planner memory used event, got %+v", observer.events)
	}
}

func TestRunner_RecallMemories_RaisesThresholdForSessionCompaction(t *testing.T) {
	mem := &mockMemory{
		results: []MemoryResult{
			{
				Content:  "Low-confidence compaction memory.",
				Score:    0.65,
				Metadata: map[string]string{"tag_0": "session-compaction", "tag_1": "session:abc"},
			},
		},
	}
	runner := &Runner{memory: mem}
	result := runner.recallMemories(context.Background(), "Inspect the workspace repository and implement the parser fix.")
	if result != "" {
		t.Fatalf("expected low-score session-compaction memory to be filtered, got %q", result)
	}
	if !mem.called {
		t.Fatal("expected memory recall call so score filtering is exercised")
	}
}

func TestRunner_GeneratePlanForTask_FiltersHarnessSeededSessionCompactionMemory(t *testing.T) {
	goal := "Explain Rust borrowing in simple terms."
	observer := &captureTaskEventObserver{}
	runner := &Runner{
		llm:           &mockLLM{planJSON: plannerTestJSON(goal)},
		eventObserver: observer,
	}

	plan, err := runner.generatePlanForTask(context.Background(), &Task{
		ID:     "task-memory-filter",
		UserID: "user-1",
		Goal:   goal,
		Metadata: map[string]interface{}{
			harnessPlannerMemorySeedKey: []map[string]interface{}{
				{
					"content": "Earlier session summary that may be stale.",
					"score":   0.95,
					"tags":    []string{"session-compaction", "session:abc"},
				},
			},
		},
	}, goal, "")
	if err != nil {
		t.Fatalf("generatePlanForTask returned unexpected error: %v", err)
	}
	if plan == nil || len(plan.Steps) == 0 {
		t.Fatalf("plan = %#v, want non-empty plan", plan)
	}
	if !taskEventSeen(observer.events, "task_planner_memory_filtered_session_compaction") {
		t.Fatalf("expected session-compaction filter event, got %+v", observer.events)
	}
	if taskEventSeen(observer.events, "task_planner_memory_used") {
		t.Fatalf("expected filtered session-compaction memory to stay unused, got %+v", observer.events)
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

func TestRunner_AutoReflectCompletedTask(t *testing.T) {
	s := testStore(t)
	m := &scriptedLLM{calls: []scriptedLLMCall{
		{content: `{"goal":"finish parser fix","subtasks":[{"description":"apply parser fix"}],"success_criteria":["verification passes"],"fallback_plan":["inspect the failing step"]}`},
	}}
	reflector := &mockReflector{result: &selfreflect.Result{
		Summary: "Reflection complete.",
		Lessons: []selfreflect.Lesson{{
			Kind:        selfreflect.LessonKindHeuristic,
			Lesson:      "Run focused verification before broader validation.",
			WhenToApply: "After a parser or router change.",
			Evidence:    "Verification completed before the broader task summary was generated.",
		}},
		MemoryWritten: 1,
	}}
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second, AutoReflect: true})
	runner.SetReflector(reflector)
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "finish parser fix", "", "")
	if err != nil {
		t.Fatal(err)
	}

	got := waitForTerminalTask(t, s, task.ID, 5*time.Second)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if !reflector.called {
		t.Fatal("expected reflector to be called")
	}
	if len(reflector.inputs) != 1 || reflector.inputs[0].FinalStatus != "completed" {
		t.Fatalf("unexpected reflection input: %#v", reflector.inputs)
	}
	if strings.TrimSpace(reflector.inputs[0].ResultSummary) == "" {
		t.Fatalf("expected reflection input result summary to be populated, got %#v", reflector.inputs[0])
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateVerify, RuntimeStateReflect) {
		t.Fatal("expected runtime transition VERIFY -> REFLECT")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateReflect, RuntimeStateReport) {
		t.Fatal("expected runtime transition REFLECT -> REPORT")
	}
	if !strings.Contains(got.Result, "Learned:") {
		t.Fatalf("expected result to include learned section, got %q", got.Result)
	}
	last := got.Plan[len(got.Plan)-1]
	if last.Description != "Reflect on the task and capture reusable lessons" || last.Status != StepStatusCompleted {
		t.Fatalf("unexpected reflection step: %#v", last)
	}
}

func TestRunner_AutoReflectFailedTask(t *testing.T) {
	s := testStore(t)
	m := &scriptedLLM{calls: []scriptedLLMCall{
		{content: `{"goal":"build","subtasks":[{"description":"primary step"}],"success_criteria":["verify passes"],"fallback_plan":["recover once"]}`},
		{content: "not-json"},
		{content: "not-json"},
	}}
	reflector := &mockReflector{result: &selfreflect.Result{
		Summary: "Failure reflection complete.",
		Lessons: []selfreflect.Lesson{{
			Kind:        selfreflect.LessonKindGuardrail,
			Lesson:      "Record the first verification failure before retrying.",
			WhenToApply: "When a verification step fails and a recovery retry is about to start.",
			Evidence:    "The initial verification and recovery both failed, so the first failure output mattered.",
		}},
	}}
	runner := NewRunner(s, m, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second, AutoReflect: true})
	runner.SetReflector(reflector)
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "execute and verify", "", "")
	if err != nil {
		t.Fatal(err)
	}

	got := waitForTerminalTask(t, s, task.ID, 5*time.Second)
	if got.Status != TaskStatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.RuntimeState != RuntimeStateDone {
		t.Fatalf("runtime_state = %q, want DONE", got.RuntimeState)
	}
	if !reflector.called {
		t.Fatal("expected reflector to be called")
	}
	if len(reflector.inputs) != 1 || reflector.inputs[0].FinalStatus != "failed" {
		t.Fatalf("unexpected reflection input: %#v", reflector.inputs)
	}
	if strings.TrimSpace(reflector.inputs[0].ResultSummary) == "" {
		t.Fatalf("expected failed reflection input result summary to be populated, got %#v", reflector.inputs[0])
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateVerify, RuntimeStateReflect) {
		t.Fatal("expected runtime transition VERIFY -> REFLECT")
	}
	if !hasRuntimeTransition(got.RuntimeAudit, RuntimeStateReflect, RuntimeStateReport) {
		t.Fatal("expected runtime transition REFLECT -> REPORT")
	}
	if !strings.Contains(got.Result, "Learned:") {
		t.Fatalf("expected failed task summary to include learned section, got %q", got.Result)
	}
}

func TestRunner_CancelledTask_DoesNotReflect(t *testing.T) {
	s := testStore(t)
	reflector := &mockReflector{}
	runner := NewRunner(s, &slowMockLLM{delay: 2 * time.Second}, nil, nil, nil, RunnerConfig{TaskTimeout: 10 * time.Second, AutoReflect: true})
	runner.SetReflector(reflector)
	t.Cleanup(func() { runner.Shutdown() })

	task, err := runner.Submit(context.Background(), "u1", "slow task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if !runner.Cancel(task.ID) {
		t.Fatal("expected cancel to succeed")
	}
	got := waitForTerminalTask(t, s, task.ID, 4*time.Second)
	if got.Status != TaskStatusCancelled {
		t.Fatalf("status = %q, want cancelled", got.Status)
	}
	if reflector.called {
		t.Fatal("did not expect reflector to be called for cancelled task")
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

func TestRunner_AskUser_SubmitTextAnswer(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "ask_text", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})
	r.mu.Lock()
	r.running["ask_text"] = func() {}
	r.mu.Unlock()

	questions := []AgentQuestion{
		{ID: "q1", Question: "请假时长？", Header: "时长", Type: "text"},
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		ok := r.SubmitAnswers("ask_text", []QuestionAnswer{
			{QuestionID: "q1", OtherText: "3天"},
		})
		if !ok {
			t.Error("SubmitAnswers returned false")
		}
	}()

	answers, err := r.AskUser(ctx, "ask_text", questions, 0)
	if err != nil {
		t.Fatalf("AskUser error: %v", err)
	}
	if len(answers) != 1 || answers[0].QuestionID != "q1" || answers[0].OtherText != "3天" {
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

func TestRunner_AskUser_CancelDoesNotOverwriteCancelledStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	task := &Task{ID: "ask_cancel", UserID: "u1", Goal: "test", Status: TaskStatusWaitingInput, RuntimeState: RuntimeStateConfirmGate}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{})
	r.mu.Lock()
	r.running["ask_cancel"] = func() {}
	r.mu.Unlock()

	askCtx, askCancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := r.AskUser(askCtx, "ask_cancel", []AgentQuestion{{ID: "q1", Question: "test", Header: "Q"}}, 0)
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	if err := s.SetStatus(ctx, "ask_cancel", TaskStatusCancelled, "cancelled by user"); err != nil {
		t.Fatal(err)
	}
	askCancel()

	if err := <-done; err == nil {
		t.Fatal("expected AskUser to return cancellation error")
	}

	got, err := s.Get(ctx, "ask_cancel")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusCancelled {
		t.Fatalf("status = %q, want %q", got.Status, TaskStatusCancelled)
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

func TestRunner_HandleAskUser_TextQuestion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	task := &Task{ID: "ask_text_handle", UserID: "u1", Goal: "test", Status: TaskStatusExecuting}
	if err := s.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(s, &mockLLM{}, nil, tools.NewExecutor(nil), nil, RunnerConfig{
		AskTimeout:       20 * time.Millisecond,
		AskTimeoutAction: "default",
	})

	result := r.handleAskUser(ctx, task, `{"questions":[{"question":"请假时长？","type":"text"}]}`)
	if strings.Contains(result, `"error"`) {
		t.Fatalf("expected success for text question, got: %s", result)
	}
	if !strings.Contains(result, `"question_id":"q0"`) {
		t.Fatalf("expected answer payload in result, got: %s", result)
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
			if got.RuntimeState != RuntimeStateDone {
				t.Fatalf("runtime_state = %q, want DONE", got.RuntimeState)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	runner.Cancel(task.ID)
	t.Fatal("task did not reach terminal state within deadline")
}

func TestRunner_RetryClearsRejectedGroundingBeforeSuccessfulReplay(t *testing.T) {
	store := testStore(t)
	registry := tools.NewRegistry()
	execTool := tools.NewMockTool("exec", "mock exec")
	execTool.SetResult(map[string]any{
		"exit_code": 0,
		"stdout":    "content: Responses | OpenAI API Reference",
		"data": map[string]any{
			"target_url": "https://developers.openai.com/api/reference/resources/responses",
			"title":      "Responses | OpenAI API Reference",
			"content":    "Responses | OpenAI API Reference\nCreate a model response\nPOST /responses",
		},
	})
	writeTool := tools.NewMockTool("write", "mock write")
	writeTool.SetResult(map[string]any{
		"success": true,
		"path":    ".blue/scratchpad/shared/responses-api-search.md",
		"size":    4,
	})
	registry.Register(execTool)
	registry.Register(writeTool)

	llmStub := &executionRetryScriptLLM{
		planResponses: []string{
			`{"goal":"Wyszukaj najnowsza dokumentacje OpenAI Responses API.","subtasks":[{"description":"Zapisz claim zadania i znajdz najnowsza dokumentacje Responses API."}],"success_criteria":["planner invented docs deliverable"],"fallback_plan":["report blocker"]}`,
		},
		groundedPlannerResponse: []string{
			`{"status":"continue","reason":"Run the canonical web query route.","next_tool":{"tool":"exec","args":{"command":"blue web_query query=\"Wyszukaj najnowsza dokumentacje OpenAI Responses API.\""}},"assertions":[]}`,
			`{"status":"complete","reason":"Scratchpad should already exist.","assertions":[{"type":"tool_called","tool":"write"}]}`,
			`{"status":"continue","reason":"Write the scratchpad claim.","next_tool":{"tool":"write","args":{"path":".blue/scratchpad/shared/responses-api-search.md","content":"seed"}},"assertions":[]}`,
			`{"status":"complete","reason":"The retry now has enough evidence.","assertions":[]}`,
		},
	}

	runner := NewRunner(store, llmStub, registry, tools.NewExecutor(registry), nil, RunnerConfig{TaskTimeout: 10 * time.Second})
	task := &Task{
		ID:     "retry-grounding-reset",
		UserID: "u1",
		Goal:   "Wyszukaj najnowsza dokumentacje OpenAI Responses API.",
		Status: TaskStatusPending,
		Metadata: map[string]any{
			"routing_contract": map[string]any{
				"gate_type":           "execution_equivalence",
				"primary_route":       "web_query",
				"expected_cli_action": "blue web_query",
				"enforce_cli_route":   true,
				"allow_fallback":      false,
			},
			"harness_contract": map[string]any{
				"required_observations": []any{"evidence_tool_used"},
			},
			"group_input": map[string]any{
				"query": "Wyszukaj najnowsza dokumentacje OpenAI Responses API.",
			},
		},
	}
	if err := store.Create(context.Background(), task); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	runner.execute(context.Background(), task, "")

	got, err := store.Get(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != TaskStatusCompleted {
		t.Fatalf("status = %q, want completed; result=%q", got.Status, got.Result)
	}
	if got.GroundingStatus == GroundingStatusRejected {
		t.Fatalf("grounding_status = %q, want non-rejected", got.GroundingStatus)
	}
	if len(got.VerificationErrors) != 0 {
		t.Fatalf("verification_errors = %#v, want none", got.VerificationErrors)
	}
	if len(got.SuccessCriteria) != 1 || got.SuccessCriteria[0] != "evidence_tool_used" {
		t.Fatalf("success_criteria = %#v, want [evidence_tool_used]", got.SuccessCriteria)
	}
	if !strings.Contains(got.Result, "Verification: PASS") {
		t.Fatalf("expected final result to include grounded verification pass, got %q", got.Result)
	}
}

func TestAgentQuestion_JSON(t *testing.T) {
	q := AgentQuestion{
		ID:          "q1",
		Question:    "Pick a framework",
		Header:      "Framework",
		Type:        "radio",
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
	if got.ID != "q1" || got.Header != "Framework" || got.Type != "radio" || len(got.Options) != 2 {
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
