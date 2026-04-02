package bootstrap

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

	"github.com/labstack/echo/v4"

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
)

type routeApprovalFileWriteTool struct {
	mu      sync.Mutex
	path    string
	content string
	calls   int
}

func (t *routeApprovalFileWriteTool) Definition() tools.ToolDefinition {
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

func (t *routeApprovalFileWriteTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	t.path = strings.TrimSpace(routeApprovalAnyToString(args["path"]))
	t.content = routeApprovalAnyToString(args["content"])
	return map[string]interface{}{
		"path":    t.path,
		"success": true,
		"append":  false,
	}, nil
}

func (t *routeApprovalFileWriteTool) snapshot() (path, content string, calls int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.path, t.content, t.calls
}

type routeApprovalRiskProviderStub struct {
	mu        sync.Mutex
	requests  []llm.ChatRequest
	responses []llm.ChatResponse
}

func (p *routeApprovalRiskProviderStub) Name() string { return "approval-risk-stub" }

func (p *routeApprovalRiskProviderStub) Models() []string { return []string{"auto"} }

func (p *routeApprovalRiskProviderStub) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
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

func (p *routeApprovalRiskProviderStub) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *routeApprovalRiskProviderStub) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return fmt.Errorf("not implemented")
}

func (p *routeApprovalRiskProviderStub) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

type routeApprovalProxyHandler struct {
	mu        sync.Mutex
	callCount int
}

func (h *routeApprovalProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.callCount++
	callCount := h.callCount
	h.mu.Unlock()

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_route_tool_approval_e2e"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		defer flusher.Flush()
	}

	switch callCount {
	case 1:
		_, _ = w.Write([]byte("data: " + `{"id":"route_approval_round_1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_route_write_1","type":"function","function":{"name":"file_write","arguments":"{\"path\":\"approved.txt\",\"content\":\"hello after approval\"}"}}]},"finish_reason":"tool_calls"}],"model":"gpt-5.3-codex-spark"}` + "\n\n"))
	default:
		_, _ = w.Write([]byte("data: " + `{"id":"route_approval_round_2","choices":[{"delta":{"content":"文件 approved.txt 已写入完成。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}` + "\n\n"))
	}
}

func (h *routeApprovalProxyHandler) snapshot() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.callCount
}

func routeApprovalAnyToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func newRouteApprovalRiskProvider() *routeApprovalRiskProviderStub {
	return &routeApprovalRiskProviderStub{
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

type routeApprovalHarness struct {
	echo            *echo.Echo
	chatHandler     *serverpkg.ChatHandler
	approvalHandler *networkapi.ApprovalHandler
	writeTool       *routeApprovalFileWriteTool
	proxyHandler    *routeApprovalProxyHandler
	riskProvider    *routeApprovalRiskProviderStub
	broker          *sse.Broker
	token           string
	convID          string
}

func newRouteApprovalHarness(t *testing.T, withActiveSSE bool) *routeApprovalHarness {
	t.Helper()

	const userID = "route-approval-user"

	store, err := memory.NewStore(t.TempDir() + "/chat-route-approval.db")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	toolRegistry := tools.NewRegistry()
	writeTool := &routeApprovalFileWriteTool{}
	toolRegistry.Register(writeTool)

	chatHandler := serverpkg.NewChatHandler(store, llm.NewProviderRegistry(), toolRegistry)
	chatHandler.SetSettingsHandler(serverpkg.NewSettingsHandler(kvstore.NewMemoryStore()))
	t.Cleanup(chatHandler.Close)

	proxyHandler := &routeApprovalProxyHandler{}
	chatHandler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))

	broker := sse.NewBroker()
	t.Cleanup(broker.Close)
	if withActiveSSE {
		sub := broker.Subscribe(userID)
		t.Cleanup(func() { broker.Unsubscribe(userID, sub) })
	}

	approvalHandler := networkapi.NewApprovalHandler(broker)
	riskProvider := newRouteApprovalRiskProvider()
	approvalHandler.SetRiskScorer(networkapi.NewLLMToolApprovalRiskScorer(riskProvider, toolRegistry))
	chatHandler.SetToolApprover(approvalHandler)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	middleware := auth.NewAuthMiddleware(jwtSvc, nil)
	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   userID,
		Username: "route-approval",
		Role:     "user",
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	conv, err := store.CreateConversation(context.Background(), "Route approval e2e", userID)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	e := echo.New()
	api := e.Group("/api/v1")
	api.Use(middleware.OptionalAuthenticate())
	chatHandler.RegisterRoutes(api)
	approvalHandler.RegisterRoutes(api)

	return &routeApprovalHarness{
		echo:            e,
		chatHandler:     chatHandler,
		approvalHandler: approvalHandler,
		writeTool:       writeTool,
		proxyHandler:    proxyHandler,
		riskProvider:    riskProvider,
		broker:          broker,
		token:           token,
		convID:          conv.ID,
	}
}

func routeApprovalAuthRequest(method, path, token string, body *bytes.Buffer) *http.Request {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body.Bytes())
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if strings.TrimSpace(token) != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return req
}

func routeApprovalPending(t *testing.T, e *echo.Echo, token, sessionID string) []networkapi.PendingRequest {
	t.Helper()
	req := routeApprovalAuthRequest(http.MethodGet, "/api/v1/approval/pending?session_id="+sessionID, token, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pending status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var pending []networkapi.PendingRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode pending approvals: %v", err)
	}
	return pending
}

func waitForRouteApprovalPending(t *testing.T, e *echo.Echo, token, sessionID string) networkapi.PendingRequest {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pending := routeApprovalPending(t, e, token, sessionID)
		if len(pending) > 0 {
			return pending[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for route-level pending approval for session %q", sessionID)
	return networkapi.PendingRequest{}
}

func routeApprovalResolve(t *testing.T, e *echo.Echo, token, requestID, bindingHash string) {
	t.Helper()
	body := bytes.NewBufferString(`{"request_id":"` + requestID + `","decision":"approve","binding_hash":"` + bindingHash + `"}`)
	req := routeApprovalAuthRequest(http.MethodPost, "/api/v1/approval/resolve", token, body)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func routeApprovalRunStreamAsync(t *testing.T, e *echo.Echo, token, convID string) (*httptest.ResponseRecorder, <-chan struct{}) {
	t.Helper()
	body := bytes.NewBufferString(`{"message":"请把结果写入 approved.txt","model":"gpt-5.3-codex-spark"}`)
	req := routeApprovalAuthRequest(http.MethodPost, "/api/v1/conversations/"+convID+"/messages/stream", token, body)
	rec := httptest.NewRecorder()
	done := make(chan struct{}, 1)
	go func() {
		e.ServeHTTP(rec, req)
		done <- struct{}{}
	}()
	return rec, done
}

func TestBootstrapChatApprovalRoutes_EscalateAndResumeWithLLMRiskScore(t *testing.T) {
	h := newRouteApprovalHarness(t, true)

	rec, done := routeApprovalRunStreamAsync(t, h.echo, h.token, h.convID)

	pending := waitForRouteApprovalPending(t, h.echo, h.token, h.convID)
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

	path, content, calls := h.writeTool.snapshot()
	if calls != 0 || path != "" || content != "" {
		t.Fatalf("expected write tool to wait for approval, got path=%q content=%q calls=%d", path, content, calls)
	}
	if got := h.proxyHandler.snapshot(); got != 1 {
		t.Fatalf("proxy call count before approval = %d, want 1", got)
	}

	routeApprovalResolve(t, h.echo, h.token, pending.ID, pending.BindingHash)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for routed chat stream to resume after approval")
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool execution event, body=%s", body)
	}

	path, content, calls = h.writeTool.snapshot()
	if calls != 1 || path != "approved.txt" || content != "hello after approval" {
		t.Fatalf("unexpected write capture path=%q content=%q calls=%d", path, content, calls)
	}
	if got := h.proxyHandler.snapshot(); got != 1 {
		t.Fatalf("proxy call count after approval = %d, want 1", got)
	}
	if pendingAfter := routeApprovalPending(t, h.echo, h.token, h.convID); len(pendingAfter) != 0 {
		t.Fatalf("expected no pending approvals after resolution, got=%+v", pendingAfter)
	}
	if _, ok := h.riskProvider.RequestAt(0); !ok {
		t.Fatal("expected route-level risk scorer to be invoked")
	}
}

func TestBootstrapChatApprovalRoutes_SkipEscalationWithoutActiveSSEClient(t *testing.T) {
	h := newRouteApprovalHarness(t, false)

	rec, done := routeApprovalRunStreamAsync(t, h.echo, h.token, h.convID)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for routed chat stream without SSE client")
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool execution event, body=%s", body)
	}

	path, content, calls := h.writeTool.snapshot()
	if calls != 1 || path != "approved.txt" || content != "hello after approval" {
		t.Fatalf("unexpected write capture path=%q content=%q calls=%d", path, content, calls)
	}
	if pending := routeApprovalPending(t, h.echo, h.token, h.convID); len(pending) != 0 {
		t.Fatalf("expected no pending approvals without active SSE client, got=%+v", pending)
	}
	if _, ok := h.riskProvider.RequestAt(0); ok {
		t.Fatal("expected route-level risk scorer to be skipped when SSE delivery is unavailable")
	}
}
