package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type fileWriteCaptureTool struct {
	mu      sync.Mutex
	path    string
	content string
	calls   int
}

func (t *fileWriteCaptureTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "file_write",
		Description: "capture file writes",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
			"required":             []string{"path", "content"},
			"additionalProperties": true,
		},
	}
}

func (t *fileWriteCaptureTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	t.path = strings.TrimSpace(anyToString(args["path"]))
	t.content = anyToString(args["content"])
	return map[string]interface{}{
		"path":    t.path,
		"success": true,
		"append":  false,
	}, nil
}

func (t *fileWriteCaptureTool) Captured() (path, content string, calls int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.path, t.content, t.calls
}

type approvalRiskProviderStub struct {
	mu        sync.Mutex
	requests  []llm.ChatRequest
	responses []llm.ChatResponse
}

func (p *approvalRiskProviderStub) Name() string { return "approval-risk-stub" }

func (p *approvalRiskProviderStub) Models() []string { return []string{"auto"} }

func (p *approvalRiskProviderStub) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if len(p.responses) == 0 {
		return &llm.ChatResponse{
			Model:      "auto",
			Provider:   "approval-risk-stub",
			ProviderID: "approval-risk-stub",
			Message:    llm.Message{Role: llm.RoleAssistant, Content: "{}"},
		}, nil
	}
	resp := p.responses[0]
	p.responses = p.responses[1:]
	return &resp, nil
}

func (p *approvalRiskProviderStub) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *approvalRiskProviderStub) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return fmt.Errorf("not implemented")
}

func (p *approvalRiskProviderStub) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

type toolApprovalE2EProxyHandler struct {
	mu                       sync.Mutex
	callCount                int
	secondRequestSawToolCall bool
	secondRequestToolContent string
}

func (h *toolApprovalE2EProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.callCount++
	callCount := h.callCount
	h.mu.Unlock()

	if callCount > 1 {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for i := len(body.Messages) - 1; i >= 0; i-- {
			if body.Messages[i].Role != string(llm.RoleTool) {
				continue
			}
			h.mu.Lock()
			h.secondRequestSawToolCall = true
			h.secondRequestToolContent = body.Messages[i].Content
			h.mu.Unlock()
			break
		}
	}

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_tool_approval_e2e"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		defer flusher.Flush()
	}

	switch callCount {
	case 1:
		_, _ = w.Write([]byte("data: " + `{"id":"approval_round_1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_write_approval_1","type":"function","function":{"name":"file_write","arguments":"{\"path\":\"approved.txt\",\"content\":\"hello after approval\"}"}}]},"finish_reason":"tool_calls"}],"model":"gpt-5.3-codex-spark"}` + "\n\n"))
	default:
		_, _ = w.Write([]byte("data: " + `{"id":"approval_round_2","choices":[{"delta":{"content":"文件 approved.txt 已写入完成。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}` + "\n\n"))
	}
}

func (h *toolApprovalE2EProxyHandler) snapshot() (callCount int, sawTool bool, toolContent string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.callCount, h.secondRequestSawToolCall, h.secondRequestToolContent
}

func runStreamTurnAsUserAsync(t *testing.T, h *serverpkg.ChatHandler, convID, userID, reqBody string) (*httptest.ResponseRecorder, <-chan error) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+convID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: userID,
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(convID)

	done := make(chan error, 1)
	go func() {
		done <- h.StreamMessage(c)
	}()
	return rec, done
}

func runStreamTurnAsUser(t *testing.T, h *serverpkg.ChatHandler, convID, userID, reqBody string) string {
	t.Helper()
	rec, done := runStreamTurnAsUserAsync(t, h, convID, userID, reqBody)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StreamMessage error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stream response")
	}
	return rec.Body.String()
}

func newApprovalRoutesEcho(handler *networkapi.ApprovalHandler) *echo.Echo {
	e := echo.New()
	if handler != nil {
		handler.RegisterRoutes(e.Group("/api/v1"))
	}
	return e
}

func fetchPendingToolApprovals(t *testing.T, handler *networkapi.ApprovalHandler, sessionID string) []networkapi.PendingRequest {
	t.Helper()
	if handler == nil {
		return nil
	}
	payload := handler.GetPendingBySession(sessionID)
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pending approval payload: %v", err)
	}
	var pending networkapi.PendingRequest
	if err := json.Unmarshal(raw, &pending); err != nil {
		t.Fatalf("decode pending approval payload: %v", err)
	}
	return []networkapi.PendingRequest{pending}
}

func waitForPendingToolApproval(t *testing.T, handler *networkapi.ApprovalHandler, sessionID string) networkapi.PendingRequest {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pending := fetchPendingToolApprovals(t, handler, sessionID)
		if len(pending) > 0 {
			return pending[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for pending tool approval for session %q", sessionID)
	return networkapi.PendingRequest{}
}

func resolveToolApproval(t *testing.T, e *echo.Echo, requestID, decision, bindingHash string) {
	t.Helper()
	body := `{"request_id":"` + requestID + `","decision":"` + decision + `","binding_hash":"` + bindingHash + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/approval/resolve", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func newApprovalRiskScorerProvider() *approvalRiskProviderStub {
	return &approvalRiskProviderStub{
		responses: []llm.ChatResponse{{
			Model:      "auto",
			Provider:   "approval-risk-stub",
			ProviderID: "approval-risk-stub",
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: `{"score":92,"confidence":0.95,"risk_level":"high","recommended_mode":"ask","reason":"writes a local file"}`,
			},
		}},
	}
}

func anyToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func TestStreamMessageE2E_LLMRiskApprovalEscalationResolvesAndContinues(t *testing.T) {
	const userID = "approval-e2e-user"

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Tool approval e2e", userID)
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	writeTool := &fileWriteCaptureTool{}
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(writeTool)

	handler := serverpkg.NewChatHandler(store, llm.NewProviderRegistry(), toolRegistry)
	handler.SetSettingsHandler(serverpkg.NewSettingsHandler(kvstore.NewMemoryStore()))

	proxyHandler := &toolApprovalE2EProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))

	broker := sse.NewBroker()
	defer broker.Close()
	sub := broker.Subscribe(userID)
	defer broker.Unsubscribe(userID, sub)

	approvalHandler := networkapi.NewApprovalHandler(broker)
	riskProvider := newApprovalRiskScorerProvider()
	approvalHandler.SetRiskScorer(networkapi.NewLLMToolApprovalRiskScorer(riskProvider, toolRegistry))
	handler.SetToolApprover(approvalHandler)

	approvalRoutes := newApprovalRoutesEcho(approvalHandler)
	rec, done := runStreamTurnAsUserAsync(
		t,
		handler,
		conv.ID,
		userID,
		`{"message":"请把结果写入 approved.txt","model":"gpt-5.3-codex-spark"}`,
	)

	pending := waitForPendingToolApproval(t, approvalHandler, conv.ID)
	if pending.ToolName != "file_write" {
		t.Fatalf("pending tool = %q, want file_write", pending.ToolName)
	}
	if pending.PolicySource != "approval.llm_risk_score" {
		t.Fatalf("policy source = %q, want approval.llm_risk_score", pending.PolicySource)
	}
	if pending.RiskLevel != "high" {
		t.Fatalf("risk level = %q, want high", pending.RiskLevel)
	}
	if pending.BindingHash == "" {
		t.Fatal("expected binding hash on pending approval")
	}

	path, content, calls := writeTool.Captured()
	if calls != 0 || path != "" || content != "" {
		t.Fatalf("expected write tool to wait for approval, got path=%q content=%q calls=%d", path, content, calls)
	}
	if proxyCalls, _, _ := proxyHandler.snapshot(); proxyCalls != 1 {
		t.Fatalf("expected chat to pause before follow-up round, proxy calls=%d", proxyCalls)
	}

	resolveToolApproval(t, approvalRoutes, pending.ID, "approve", pending.BindingHash)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StreamMessage error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for stream to resume after approval")
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool execution event in stream body, got=%s", body)
	}
	if !strings.Contains(body, "approved.txt") {
		t.Fatalf("expected final body to mention approved.txt, got=%s", body)
	}

	path, content, calls = writeTool.Captured()
	if calls != 1 {
		t.Fatalf("write tool calls = %d, want 1", calls)
	}
	if path != "approved.txt" {
		t.Fatalf("write path = %q, want approved.txt", path)
	}
	if content != "hello after approval" {
		t.Fatalf("write content = %q, want hello after approval", content)
	}

	proxyCalls, sawToolResult, toolContent := proxyHandler.snapshot()
	if proxyCalls != 1 {
		t.Fatalf("proxy call count = %d, want 1", proxyCalls)
	}
	if sawToolResult || toolContent != "" {
		t.Fatalf("expected write-completion flow to finish without a second model follow-up, got saw=%v content=%q", sawToolResult, toolContent)
	}

	if pendingAfter := fetchPendingToolApprovals(t, approvalHandler, conv.ID); len(pendingAfter) != 0 {
		t.Fatalf("expected approvals to be resolved, got=%+v", pendingAfter)
	}
	if _, ok := riskProvider.RequestAt(0); !ok {
		t.Fatal("expected auxiliary approval scorer to be invoked")
	}
}

func TestStreamMessageE2E_LLMRiskApprovalSkipsEscalationWithoutActiveSSEClient(t *testing.T) {
	const userID = "approval-no-sse-user"

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Tool approval no sse", userID)
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	writeTool := &fileWriteCaptureTool{}
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(writeTool)

	handler := serverpkg.NewChatHandler(store, llm.NewProviderRegistry(), toolRegistry)
	handler.SetSettingsHandler(serverpkg.NewSettingsHandler(kvstore.NewMemoryStore()))

	proxyHandler := &toolApprovalE2EProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))

	broker := sse.NewBroker()
	defer broker.Close()

	approvalHandler := networkapi.NewApprovalHandler(broker)
	riskProvider := newApprovalRiskScorerProvider()
	approvalHandler.SetRiskScorer(networkapi.NewLLMToolApprovalRiskScorer(riskProvider, toolRegistry))
	handler.SetToolApprover(approvalHandler)

	body := runStreamTurnAsUser(
		t,
		handler,
		conv.ID,
		userID,
		`{"message":"请把结果写入 approved.txt","model":"gpt-5.3-codex-spark"}`,
	)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool execution event in stream body, got=%s", body)
	}

	path, content, calls := writeTool.Captured()
	if calls != 1 {
		t.Fatalf("write tool calls = %d, want 1", calls)
	}
	if path != "approved.txt" || content != "hello after approval" {
		t.Fatalf("unexpected write capture path=%q content=%q", path, content)
	}

	proxyCalls, sawToolResult, toolContent := proxyHandler.snapshot()
	if proxyCalls != 1 {
		t.Fatalf("proxy call count = %d, want 1", proxyCalls)
	}
	if sawToolResult || toolContent != "" {
		t.Fatalf("expected auto-allowed write flow to finish without a second model follow-up, got saw=%v content=%q", sawToolResult, toolContent)
	}

	if pending := fetchPendingToolApprovals(t, approvalHandler, conv.ID); len(pending) != 0 {
		t.Fatalf("expected no pending approvals without active SSE client, got=%+v", pending)
	}
	if _, ok := riskProvider.RequestAt(0); ok {
		t.Fatal("expected scorer to be skipped when approval delivery is unavailable")
	}
}
