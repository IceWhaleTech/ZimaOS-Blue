package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type approvalObserverStub struct {
	requested []tools.ApprovalRuntimeEvent
	resolved  []tools.ApprovalRuntimeEvent
}

type execResolverStub struct {
	result ExecApprovalResolveResult
}

type approvalRiskScorerStub struct {
	result *ToolApprovalRiskScore
	err    error
	calls  int
}

type approvalRiskLLMStub struct {
	resp    *llm.ChatResponse
	err     error
	calls   int
	lastReq llm.ChatRequest
}

func (s execResolverStub) ResolveApproval(id string, decision string, bindingHash string) ExecApprovalResolveResult {
	return s.result
}

func (s *approvalRiskScorerStub) ScoreToolApproval(_ context.Context, _ tools.ToolApprovalRequest) (*ToolApprovalRiskScore, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *approvalRiskLLMStub) Name() string { return "stub" }

func (s *approvalRiskLLMStub) Models() []string { return []string{"stub"} }

func (s *approvalRiskLLMStub) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	s.calls++
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	if s.resp != nil {
		return s.resp, nil
	}
	return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "{}"}}, nil
}

func (s *approvalRiskLLMStub) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (s *approvalRiskLLMStub) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return errors.New("not implemented")
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

func TestApprovalGetPendingBySessionFiltersBySessionID(t *testing.T) {
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

	got := h.GetPendingBySession("conv-2")
	if got == nil {
		t.Fatal("GetPendingBySession(conv-2) = nil, want non-nil")
	}
	if got["id"] != "req-2" {
		t.Fatalf("id = %v, want %q", got["id"], "req-2")
	}
	if got["session_id"] != "conv-2" {
		t.Fatalf("session_id = %v, want %q", got["session_id"], "conv-2")
	}
}

func TestApprovalRegisterRoutesDoesNotExposeStandalonePendingRoute(t *testing.T) {
	e := echo.New()
	NewApprovalHandler(nil).RegisterRoutes(e.Group("/api/v1"))

	for _, route := range e.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/approval/pending" {
			t.Fatalf("did not expect standalone approval pending route to remain registered")
		}
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
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	h := NewApprovalHandler(broker)
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

func TestApprovalAuthorizeToolCall_EscalatesAutoPolicyWithRiskScorer(t *testing.T) {
	e := echo.New()
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	h := NewApprovalHandler(broker)
	scorer := &approvalRiskScorerStub{
		result: &ToolApprovalRiskScore{
			Score:           88,
			Confidence:      0.93,
			RiskLevel:       "high",
			RecommendedMode: "ask",
			Reason:          "writes a local file",
		},
	}
	h.SetRiskScorer(scorer)

	done := make(chan tools.ToolApprovalDecision, 1)
	errCh := make(chan error, 1)
	go func() {
		decision, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
			ToolName: "file_write",
			Arguments: map[string]interface{}{
				"path":    "notes.txt",
				"content": "hello",
			},
			RouteKind:   tools.ToolRouteKindAgent,
			SessionID:   "conv-llm-risk",
			UserID:      "user-1",
			BindingHash: "binding-risk-1",
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
		t.Fatal("expected pending approval request after scorer escalation")
	}

	req := httptest.NewRequest(http.MethodPost, "/approval/resolve", strings.NewReader(`{"request_id":"`+requestID+`","decision":"approve","binding_hash":"binding-risk-1"}`))
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
			t.Fatal("expected approved decision")
		}
		if !decision.Approval.Required {
			t.Fatal("expected explicit approval requirement")
		}
		if decision.Approval.Mode != "ask" {
			t.Fatalf("mode = %q, want ask", decision.Approval.Mode)
		}
		if decision.Approval.PolicySource != "approval.llm_risk_score" {
			t.Fatalf("policy source = %q, want approval.llm_risk_score", decision.Approval.PolicySource)
		}
		if decision.Approval.RiskLevel != "high" {
			t.Fatalf("risk level = %q, want high", decision.Approval.RiskLevel)
		}
	}

	if scorer.calls != 1 {
		t.Fatalf("scorer calls = %d, want 1", scorer.calls)
	}
}

func TestApprovalAuthorizeToolCall_ScorerErrorsFallBackToStaticPolicy(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe("user-1")
	defer broker.Unsubscribe("user-1", sub)

	h := NewApprovalHandler(broker)
	scorer := &approvalRiskScorerStub{err: errors.New("scorer unavailable")}
	h.SetRiskScorer(scorer)

	decision, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
		ToolName: "file_write",
		Arguments: map[string]interface{}{
			"path":    "notes.txt",
			"content": "hello",
		},
		RouteKind: tools.ToolRouteKindAgent,
		SessionID: "conv-scorer-fallback",
		UserID:    "user-1",
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	}
	if !decision.Allowed {
		t.Fatal("expected fallback to preserve auto allow")
	}
	if decision.Approval.Required {
		t.Fatal("expected scorer failure to avoid explicit approval")
	}
	if scorer.calls != 1 {
		t.Fatalf("scorer calls = %d, want 1", scorer.calls)
	}
}

func TestApprovalAuthorizeToolCall_StaticOverrideSkipsRiskScorer(t *testing.T) {
	h := NewApprovalHandler(nil)
	scorer := &approvalRiskScorerStub{
		result: &ToolApprovalRiskScore{
			RecommendedMode: "ask",
			RiskLevel:       "high",
		},
	}
	h.SetRiskScorer(scorer)

	decision, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
		ToolName: "file_delete",
		Arguments: map[string]interface{}{
			"path":      "/workspace",
			"recursive": true,
		},
		RouteKind: tools.ToolRouteKindAgent,
		SessionID: "conv-static-override",
		UserID:    "user-1",
	})
	if err != nil {
		t.Fatalf("AuthorizeToolCall() error = %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected static override denial to remain authoritative")
	}
	if scorer.calls != 0 {
		t.Fatalf("scorer calls = %d, want 0", scorer.calls)
	}
}

func TestLLMToolApprovalRiskScorer_NormalizesStructuredResponse(t *testing.T) {
	llmStub := &approvalRiskLLMStub{
		resp: &llm.ChatResponse{
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "```json\n{\"score\":0.83,\"confidence\":88,\"risk_level\":\"HIGH\",\"recommended_mode\":\"deny\",\"reason\":\" writes to disk \",\"signals\":[\"writes file\"]}\n```",
			},
		},
	}
	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{
		Name:        "file_write",
		Description: "Writes content to a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
	})
	scorer := NewLLMToolApprovalRiskScorer(llmStub, registry)

	score, err := scorer.ScoreToolApproval(context.Background(), tools.ToolApprovalRequest{
		ToolName: "file_write",
		Arguments: map[string]interface{}{
			"path":    "notes.txt",
			"content": strings.Repeat("x", 400),
		},
		RouteKind: tools.ToolRouteKindAgent,
	})
	if err != nil {
		t.Fatalf("ScoreToolApproval() error = %v", err)
	}
	if score.Score != 83 {
		t.Fatalf("score = %v, want 83", score.Score)
	}
	if score.Confidence != 0.88 {
		t.Fatalf("confidence = %v, want 0.88", score.Confidence)
	}
	if score.RiskLevel != "high" {
		t.Fatalf("risk level = %q, want high", score.RiskLevel)
	}
	if score.RecommendedMode != "ask" {
		t.Fatalf("recommended mode = %q, want ask", score.RecommendedMode)
	}
	if score.Reason != "writes to disk" {
		t.Fatalf("reason = %q, want writes to disk", score.Reason)
	}
	if llmStub.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", llmStub.calls)
	}
	if !strings.Contains(llmStub.lastReq.Messages[1].Content, "[truncated") {
		t.Fatalf("expected prompt arguments to be truncated, got %q", llmStub.lastReq.Messages[1].Content)
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

func TestApprovalAuthorizeToolCallReturnsImmediateDeliveryErrorWithoutActiveSSEClient(t *testing.T) {
	h := NewApprovalHandler(sse.NewBroker())
	h.config.DefaultPolicy = "ask"

	start := time.Now()
	_, err := h.AuthorizeToolCall(context.Background(), tools.ToolApprovalRequest{
		ToolName:  "browser",
		RouteKind: tools.ToolRouteKindAgent,
		SessionID: "conv-no-sse",
		UserID:    "user-1",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected delivery-unavailable error")
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected immediate failure without timeout wait, elapsed=%s", elapsed)
	}
	if len(h.pending) != 0 {
		t.Fatalf("expected no pending approvals after immediate failure, got=%d", len(h.pending))
	}

	var runtimeErr tools.ToolRuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected ToolRuntimeError, got %T: %v", err, err)
	}
	if runtimeErr.ToolRuntimeCode() != "tool_approval_delivery_unavailable" {
		t.Fatalf("code = %q, want tool_approval_delivery_unavailable", runtimeErr.ToolRuntimeCode())
	}
}
