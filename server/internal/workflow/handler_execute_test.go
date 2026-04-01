package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type stubExecutionLauncher struct {
	request ExecutionLaunchRequest
	result  *Execution
	err     error
	calls   int

	cancelExecutionID string
	cancelReason      string
	cancelHandled     bool
	cancelErr         error
	cancelCalls       int

	resumeExecutionID string
	resumeInput       ExecutionResumeInput
	resumeResult      *Execution
	resumeHandled     bool
	resumeErr         error
	resumeCalls       int
}

func (s *stubExecutionLauncher) LaunchExecution(_ context.Context, req ExecutionLaunchRequest) (*Execution, error) {
	s.calls++
	s.request = req
	return s.result, s.err
}

func (s *stubExecutionLauncher) CancelExecution(_ context.Context, executionID string, reason string) (bool, error) {
	s.cancelCalls++
	s.cancelExecutionID = executionID
	s.cancelReason = reason
	return s.cancelHandled, s.cancelErr
}

func (s *stubExecutionLauncher) ResumeExecution(_ context.Context, executionID string, resume ExecutionResumeInput) (bool, *Execution, error) {
	s.resumeCalls++
	s.resumeExecutionID = executionID
	s.resumeInput = resume
	return s.resumeHandled, s.resumeResult, s.resumeErr
}

func TestHandlerExecuteWorkflowUsesExecutionLauncher(t *testing.T) {
	e := echo.New()
	body := bytes.NewBufferString(`{"trigger_data":{"conversation_id":"conv-1","source":"manual"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/execute", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/workflows/:id/execute")
	c.SetParamNames("id")
	c.SetParamValues("wf-1")
	c.Set("tenant_id", "tenant-1")
	c.Set("user_id", "user-1")

	launcher := &stubExecutionLauncher{
		result: &Execution{
			ID:         "exec-1",
			WorkflowID: "wf-1",
			TenantID:   "tenant-1",
			Status:     ExecutionStatusRunning,
		},
	}
	handler := NewHandler(nil)
	handler.SetExecutionLauncher(launcher)

	if err := handler.ExecuteWorkflow(c); err != nil {
		t.Fatalf("ExecuteWorkflow() error = %v", err)
	}
	if launcher.calls != 1 {
		t.Fatalf("LaunchExecution calls = %d, want 1", launcher.calls)
	}
	if launcher.request.WorkflowID != "wf-1" {
		t.Fatalf("workflow_id = %q, want wf-1", launcher.request.WorkflowID)
	}
	if launcher.request.TriggerType != TriggerTypeManual {
		t.Fatalf("trigger_type = %q, want %q", launcher.request.TriggerType, TriggerTypeManual)
	}
	if launcher.request.UserID != "user-1" || launcher.request.TenantID != "tenant-1" {
		t.Fatalf("launcher request = %+v, want user/tenant propagated", launcher.request)
	}
	if got, _ := launcher.request.TriggerData["source"].(string); got != "manual" {
		t.Fatalf("trigger_data.source = %q, want manual", got)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var payload Execution
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ID != "exec-1" || payload.WorkflowID != "wf-1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestHandlerExecuteWorkflowTimeoutReturnsRequestTimeout(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/execute", bytes.NewBufferString(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/workflows/:id/execute")
	c.SetParamNames("id")
	c.SetParamValues("wf-1")

	launcher := &stubExecutionLauncher{
		err: fmt.Errorf("waiting for workflow execution slot: %w", context.DeadlineExceeded),
	}
	handler := NewHandler(nil)
	handler.SetExecutionLauncher(launcher)

	if err := handler.ExecuteWorkflow(c); err != nil {
		t.Fatalf("ExecuteWorkflow() error = %v", err)
	}
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestTimeout)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "request timed out while waiting for an execution slot" {
		t.Fatalf("error = %q, want timeout message", payload["error"])
	}
}
