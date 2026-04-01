package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type approvalObserverStub struct {
	requested []tools.ApprovalRuntimeEvent
	resolved  []tools.ApprovalRuntimeEvent
}

type execResolverStub struct {
	result ExecApprovalResolveResult
}

func (s execResolverStub) ResolveApproval(id string, decision string, bindingHash string) ExecApprovalResolveResult {
	return s.result
}

func (s *approvalObserverStub) OnToolRequested(event tools.ToolRuntimeEvent)         {}
func (s *approvalObserverStub) OnToolFinished(event tools.ToolRuntimeEvent)          {}
func (s *approvalObserverStub) OnQuestionRequested(event tools.QuestionRuntimeEvent) {}
func (s *approvalObserverStub) OnQuestionResolved(event tools.QuestionRuntimeEvent)  {}
func (s *approvalObserverStub) OnApprovalRequested(event tools.ApprovalRuntimeEvent) {
	s.requested = append(s.requested, event)
}
func (s *approvalObserverStub) OnApprovalResolved(event tools.ApprovalRuntimeEvent) {
	s.resolved = append(s.resolved, event)
}

func TestApprovalListPendingFiltersBySessionID(t *testing.T) {
	e := echo.New()
	h := NewApprovalHandler(nil)
	h.pending["req-1"] = &PendingRequest{
		ID:        "req-1",
		ToolName:  "browser",
		SessionID: "conv-1",
	}
	h.pending["req-2"] = &PendingRequest{
		ID:        "req-2",
		ToolName:  "exec",
		SessionID: "conv-2",
	}

	req := httptest.NewRequest(http.MethodGet, "/approval/pending?session_id=conv-2", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.ListPending(c); err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []PendingRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ID != "req-2" {
		t.Fatalf("got[0].ID = %q, want %q", got[0].ID, "req-2")
	}
	if got[0].SessionID != "conv-2" {
		t.Fatalf("got[0].SessionID = %q, want %q", got[0].SessionID, "conv-2")
	}
}

func TestApprovalResolveRejectsBindingHashMismatch(t *testing.T) {
	e := echo.New()
	h := NewApprovalHandler(nil)
	h.pending["req-1"] = &PendingRequest{
		ID:          "req-1",
		ToolName:    "browser",
		SessionID:   "conv-1",
		BindingHash: "binding-1",
	}

	req := httptest.NewRequest(http.MethodPost, "/approval/resolve", strings.NewReader(`{"request_id":"req-1","decision":"approve","binding_hash":"binding-x"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Resolve(c); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if _, ok := h.pending["req-1"]; !ok {
		t.Fatal("pending request should remain after binding mismatch")
	}
}

func TestApprovalResolveRejectsExecBindingHashMismatch(t *testing.T) {
	e := echo.New()
	h := NewApprovalHandler(nil)
	h.SetExecResolver(execResolverStub{
		result: ExecApprovalResolveResult{BindingMismatch: true},
	})

	req := httptest.NewRequest(http.MethodPost, "/approval/resolve", strings.NewReader(`{"request_id":"exec-1","decision":"allow-once"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Resolve(c); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestApprovalAuthorizeToolCallDenyPolicy(t *testing.T) {
	h := NewApprovalHandler(nil)
	h.config.DefaultPolicy = "deny"

	decision, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
		ToolName:    "browser",
		RouteKind:   tools.ToolRouteKindChat,
		BindingHash: "binding-1",
		RiskLevel:   "medium",
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected decision to deny")
	}
	if decision.Approval.PolicySource != "approval.default_policy" {
		t.Fatalf("policy source = %q, want approval.default_policy", decision.Approval.PolicySource)
	}
}

func TestApprovalAuthorizeToolCallObserverLifecycle(t *testing.T) {
	e := echo.New()
	h := NewApprovalHandler(nil)
	h.config.DefaultPolicy = "ask"
	observer := &approvalObserverStub{}
	h.SetObserver(observer)

	ctx := tools.WithRunID(context.Background(), "run-tool-approval")
	ctx = tools.WithRunStep(ctx, 8)
	done := make(chan tools.ToolApprovalDecision, 1)
	errCh := make(chan error, 1)
	go func() {
		decision, err := h.AuthorizeToolCall(ctx, tools.ToolApprovalRequest{
			ToolName:    "browser",
			RouteKind:   tools.ToolRouteKindAgent,
			SessionID:   "conv-1",
			UserID:      "user-1",
			BindingHash: "binding-1",
			RiskLevel:   "medium",
		})
		if err != nil {
			errCh <- err
			return
		}
		done <- decision
	}()

	var requestID string
	for i := 0; i < 100; i++ {
		h.mu.RLock()
		for id := range h.pending {
			requestID = id
			break
		}
		h.mu.RUnlock()
		if requestID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if requestID == "" {
		t.Fatal("expected pending approval request")
	}

	req := httptest.NewRequest(http.MethodPost, "/approval/resolve", strings.NewReader(`{"request_id":"`+requestID+`","decision":"approve","binding_hash":"binding-1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.Resolve(c); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case err := <-errCh:
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	case decision := <-done:
		if !decision.Allowed {
			t.Fatal("expected decision to allow")
		}
	}

	if len(observer.requested) != 1 {
		t.Fatalf("requested len = %d, want 1", len(observer.requested))
	}
	if len(observer.resolved) != 1 {
		t.Fatalf("resolved len = %d, want 1", len(observer.resolved))
	}
	if observer.requested[0].RunID != "run-tool-approval" || observer.resolved[0].RunID != "run-tool-approval" {
		t.Fatalf("unexpected run ids: req=%q res=%q", observer.requested[0].RunID, observer.resolved[0].RunID)
	}
	if observer.resolved[0].Decision != "approve" {
		t.Fatalf("decision = %q, want approve", observer.resolved[0].Decision)
	}
}

func TestApprovalAuthorizeToolCall_DeniesRecursiveFileDelete(t *testing.T) {
	h := NewApprovalHandler(nil)

	decision, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
		ToolName: "file_delete",
		Arguments: map[string]interface{}{
			"path":      "/workspace",
			"recursive": true,
		},
		RouteKind: tools.ToolRouteKindAgent,
		SessionID: "conv-file-delete",
		UserID:    "user-1",
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected recursive file_delete to be denied")
	}
	if !decision.Approval.Required {
		t.Fatal("expected denial to be surfaced as a required approval decision")
	}
	if decision.Approval.RiskLevel != "critical" {
		t.Fatalf("risk = %q, want critical", decision.Approval.RiskLevel)
	}
	if len(h.pending) != 0 {
		t.Fatalf("expected high-risk file_delete denial to avoid pending approvals, got=%d", len(h.pending))
	}
	if !strings.Contains(decision.Approval.Reason, "blocked by approval policy") {
		t.Fatalf("expected denial reason, got=%q", decision.Approval.Reason)
	}
}

func TestApprovalAuthorizeToolCall_AutoAllowsSimpleFileDelete(t *testing.T) {
	h := NewApprovalHandler(nil)

	decision, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
		ToolName: "file_delete",
		Arguments: map[string]interface{}{
			"path": "scratch.txt",
		},
		RouteKind: tools.ToolRouteKindAgent,
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	}
	if !decision.Allowed {
		t.Fatal("expected simple file delete to remain auto-allowed")
	}
	if decision.Approval.Required {
		t.Fatal("expected no approval requirement for simple file delete")
	}
}

func TestApprovalAuthorizeToolCallReturnsStructuredCancelError(t *testing.T) {
	h := NewApprovalHandler(nil)
	h.config.DefaultPolicy = "ask"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := h.AuthorizeToolCall(ctx, tools.ToolApprovalRequest{
		ToolName:  "browser",
		RouteKind: tools.ToolRouteKindAgent,
		SessionID: "conv-cancelled",
		UserID:    "user-1",
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	var runtimeErr tools.ToolRuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected ToolRuntimeError, got %T: %v", err, err)
	}
	if runtimeErr.ToolRuntimeCode() != "tool_approval_cancelled" {
		t.Fatalf("code = %q, want tool_approval_cancelled", runtimeErr.ToolRuntimeCode())
	}
}
