package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/labstack/echo/v4"
)

func performAgentJSONRequest(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func waitForTaskStatusFromStore(t *testing.T, store *Store, taskID string, timeout time.Duration, wanted ...TaskStatus) *Task {
	t.Helper()
	want := make(map[TaskStatus]struct{}, len(wanted))
	for _, status := range wanted {
		want[status] = struct{}{}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := store.Get(context.Background(), taskID)
		if err == nil {
			if _, ok := want[task.Status]; ok {
				return task
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach expected statuses %v within %s", taskID, wanted, timeout)
	return nil
}

type highRiskToolCallLLM struct {
	calls int
}

func (m *highRiskToolCallLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.calls++
	if m.calls == 1 {
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: `{"goal":"build","subtasks":[{"description":"run risky operation"}]}`},
		}, nil
	}
	if isGroundedPlannerPrompt(req) {
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: `{"status":"continue","reason":"Need to execute the requested risky command.","next_tool":{"tool":"exec","args":{"command":"rm -rf /tmp/smoke-risk"}},"assertions":[]}`},
		}, nil
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: defaultResponseForRequest(req)},
	}, nil
}

func TestAgentHandlerSmoke_CreateAskAnswerComplete(t *testing.T) {
	store := testStore(t)
	runner := NewRunner(store, &mockLLM{planJSON: `[{"description":"implement primary task"}]`}, nil, nil, nil, RunnerConfig{
		TaskTimeout:      8 * time.Second,
		AskTimeout:       4 * time.Second,
		AskTimeoutAction: "error",
		MaxConcurrent:    2,
	})
	t.Cleanup(func() { runner.Shutdown() })

	handler := NewHandler(store, runner)
	e := echo.New()
	handler.RegisterRoutes(e.Group("/api/v1/agent"))

	createRec := performAgentJSONRequest(e, http.MethodPost, "/api/v1/agent/tasks", `{"goal":"TBD: build feature"}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create task status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created Task
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("create response missing task id: %s", createRec.Body.String())
	}

	waiting := waitForTaskStatusFromStore(t, store, created.ID, 3*time.Second, TaskStatusWaitingInput)
	if waiting.RuntimeState != RuntimeStateClarify {
		t.Fatalf("runtime_state=%q, want %q while waiting for answer", waiting.RuntimeState, RuntimeStateClarify)
	}

	answerBody := `{"answers":[{"question_id":"goal_scope","values":["balanced"]}]}`
	answerRec := performAgentJSONRequest(e, http.MethodPost, "/api/v1/agent/tasks/"+created.ID+"/answer", answerBody)
	if answerRec.Code != http.StatusOK {
		t.Fatalf("submit answer status=%d body=%s", answerRec.Code, answerRec.Body.String())
	}

	done := waitForTaskStatusFromStore(t, store, created.ID, 6*time.Second, TaskStatusCompleted, TaskStatusFailed, TaskStatusAborted)
	if done.Status != TaskStatusCompleted {
		t.Fatalf("final status=%q, want completed; error=%s", done.Status, done.Error)
	}
	if done.RuntimeState != RuntimeStateDone {
		t.Fatalf("runtime_state=%q, want %q", done.RuntimeState, RuntimeStateDone)
	}
	if !strings.Contains(done.Goal, "Execution preference: balanced") {
		t.Fatalf("expected chosen answer to be injected into goal, got: %q", done.Goal)
	}
}

func TestAgentHandlerSmoke_SubmitAnswerNoPending(t *testing.T) {
	store := testStore(t)
	runner := NewRunner(store, &mockLLM{}, nil, nil, nil, RunnerConfig{})
	t.Cleanup(func() { runner.Shutdown() })

	handler := NewHandler(store, runner)
	e := echo.New()
	handler.RegisterRoutes(e.Group("/api/v1/agent"))

	rec := performAgentJSONRequest(e, http.MethodPost, "/api/v1/agent/tasks/nonexistent/answer", `{"answers":[{"question_id":"q1","values":["a"]}]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentHandlerSmoke_ConfirmGateAbortHighRiskCall(t *testing.T) {
	store := testStore(t)
	runner := NewRunner(store, &highRiskToolCallLLM{}, nil, nil, nil, RunnerConfig{
		TaskTimeout:      8 * time.Second,
		AskTimeout:       4 * time.Second,
		AskTimeoutAction: "error",
		MaxConcurrent:    2,
	})
	t.Cleanup(func() { runner.Shutdown() })

	handler := NewHandler(store, runner)
	e := echo.New()
	handler.RegisterRoutes(e.Group("/api/v1/agent"))

	createRec := performAgentJSONRequest(e, http.MethodPost, "/api/v1/agent/tasks", `{"goal":"build feature with confirmation gate"}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create task status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created Task
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("create response missing task id: %s", createRec.Body.String())
	}

	waiting := waitForTaskStatusFromStore(t, store, created.ID, 4*time.Second, TaskStatusWaitingInput)
	if waiting.RuntimeState != RuntimeStateConfirmGate {
		t.Fatalf("runtime_state=%q, want %q while waiting for confirmation", waiting.RuntimeState, RuntimeStateConfirmGate)
	}

	answerBody := `{"answers":[{"question_id":"tool_gate","values":["abort"]}]}`
	answerRec := performAgentJSONRequest(e, http.MethodPost, "/api/v1/agent/tasks/"+created.ID+"/answer", answerBody)
	if answerRec.Code != http.StatusOK {
		t.Fatalf("submit answer status=%d body=%s", answerRec.Code, answerRec.Body.String())
	}

	done := waitForTaskStatusFromStore(t, store, created.ID, 6*time.Second, TaskStatusCompleted, TaskStatusFailed, TaskStatusAborted)
	if done.Status != TaskStatusAborted {
		t.Fatalf("final status=%q, want aborted; error=%s", done.Status, done.Error)
	}
	if done.RuntimeState != RuntimeStateAborted {
		t.Fatalf("runtime_state=%q, want %q", done.RuntimeState, RuntimeStateAborted)
	}
	if !hasRuntimeTransition(done.RuntimeAudit, RuntimeStateExecute, RuntimeStateConfirmGate) {
		t.Fatal("expected runtime transition EXECUTE -> CONFIRM_GATE")
	}
	if !hasRuntimeTransition(done.RuntimeAudit, RuntimeStateConfirmGate, RuntimeStateAborted) {
		t.Fatal("expected runtime transition CONFIRM_GATE -> ABORTED")
	}
}
