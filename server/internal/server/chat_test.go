package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type requestCaptureProvider struct {
	lastReq llm.ChatRequest
	mu      sync.Mutex
}

type scriptedChatProvider struct {
	name      string
	models    []string
	responses []llm.ChatResponse

	mu        sync.Mutex
	callCount int
	requests  []llm.ChatRequest
}

type deepResearchExecMock struct {
	result map[string]interface{}
	err    error
	mu     sync.Mutex
	calls  int
	last   map[string]interface{}
}

func (m *deepResearchExecMock) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.mu.Lock()
	m.calls++
	m.last = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.last[k] = v
	}
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

type smallModelRuntimeMock struct {
	respText string
	err      error
	calls    int
	calledCh chan struct{}
}

type staticToolMock struct {
	def tools.ToolDefinition
}

func (m *staticToolMock) Definition() tools.ToolDefinition {
	return m.def
}

func (m *staticToolMock) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (m *smallModelRuntimeMock) Ready() bool { return true }

func (m *smallModelRuntimeMock) Generate(_ context.Context, _ smallmodel.GenerateRequest) (*smallmodel.GenerateResponse, error) {
	m.calls++
	if m.calledCh != nil {
		select {
		case m.calledCh <- struct{}{}:
		default:
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return &smallmodel.GenerateResponse{Text: m.respText}, nil
}

func (p *requestCaptureProvider) Name() string { return "capture" }

func (p *requestCaptureProvider) Models() []string { return []string{"capture-model"} }

func (p *requestCaptureProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.lastReq = req
	p.mu.Unlock()
	return &llm.ChatResponse{
		ID:      "capture-resp",
		Model:   req.Model,
		Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		Usage:   llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
	}, nil
}

func (p *requestCaptureProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *requestCaptureProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	p.mu.Lock()
	p.lastReq = req
	p.mu.Unlock()
	return callback(llm.StreamChunk{Delta: "ok", Done: true, Usage: &llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}})
}

func (p *requestCaptureProvider) LastRequest() llm.ChatRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastReq
}

func cloneChatRequestForTest(req llm.ChatRequest) llm.ChatRequest {
	raw, err := json.Marshal(req)
	if err != nil {
		return req
	}
	var copied llm.ChatRequest
	if err := json.Unmarshal(raw, &copied); err != nil {
		return req
	}
	return copied
}

func (p *scriptedChatProvider) Name() string {
	if strings.TrimSpace(p.name) != "" {
		return p.name
	}
	return "scripted"
}

func (p *scriptedChatProvider) Models() []string {
	if len(p.models) > 0 {
		return p.models
	}
	return []string{"gpt-5.3-codex-spark", "gpt-5.3-codex"}
}

func (p *scriptedChatProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	p.callCount++
	p.requests = append(p.requests, cloneChatRequestForTest(req))
	idx := p.callCount - 1
	if idx >= len(p.responses) {
		idx = len(p.responses) - 1
	}
	p.mu.Unlock()

	if idx < 0 {
		return &llm.ChatResponse{
			ID:      "scripted-default",
			Model:   req.Model,
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}, nil
	}
	resp := p.responses[idx]
	return &resp, nil
}

func (p *scriptedChatProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		close(ch)
		return nil, err
	}
	ch <- llm.StreamChunk{
		ID:    resp.ID,
		Model: resp.Model,
		Delta: resp.Message.Content,
		Done:  true,
		Usage: &resp.Usage,
	}
	close(ch)
	return ch, nil
}

func (p *scriptedChatProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return callback(llm.StreamChunk{
		ID:    resp.ID,
		Model: resp.Model,
		Delta: resp.Message.Content,
		Done:  true,
		Usage: &resp.Usage,
	})
}

func (p *scriptedChatProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

func (p *scriptedChatProvider) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

func hasSystemAnchor(messages []llm.Message, title, goal string) bool {
	for _, m := range messages {
		if m.Role != llm.RoleSystem {
			continue
		}
		if strings.Contains(m.Content, "Conversation title: "+title) && strings.Contains(m.Content, "Initial user goal: "+goal) {
			return true
		}
	}
	return false
}

func TestIsAwaitingUserInput_ProtocolMarker(t *testing.T) {
	content := "Need your decision.\n<awaiting_user_input>true</awaiting_user_input>"
	if !isAwaitingUserInput(content) {
		t.Fatal("expected protocol marker to be detected as awaiting input")
	}
}

func TestIsAwaitingUserInput_StructuredAskGate(t *testing.T) {
	content := "请选择执行策略\nA. 快速方案\nB. 平衡方案\nC. 工程化方案"
	if !isAwaitingUserInput(content) {
		t.Fatal("expected structured options ask gate to be detected as awaiting input")
	}
}

func TestIsAwaitingUserInput_AskGateTag(t *testing.T) {
	content := "<ask_gate>one-line question\nA. yes\nB. no</ask_gate>"
	if !isAwaitingUserInput(content) {
		t.Fatal("expected ask_gate tag to be detected as awaiting input")
	}
}

func TestShouldAutoContinueForTodo_StopsOnAwaitingInput(t *testing.T) {
	current := "- [ ] implement feature\n<awaiting_user_input>true</awaiting_user_input>"
	if shouldAutoContinueForTodo(current, "") {
		t.Fatal("auto-continue should stop when awaiting user input marker is present")
	}
}

func TestDeriveContinuationContext_AffirmativeWithDefaultPlan(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "你可以帮我查一下 ZimaOS 的信息吗"},
		{
			Role:    llm.RoleAssistant,
			Content: "- [ ] 产品定位与功能概览\n- [ ] 最新动态\n\n如果你不想选，我可以默认按「功能概览 + 最新动态」先查一版。",
		},
		{Role: llm.RoleUser, Content: "好的"},
	}

	cc := deriveContinuationContext("好的", msgs)
	if strings.TrimSpace(cc.Hint) == "" {
		t.Fatal("expected continuation hint for short affirmative reply with default plan")
	}
	if cc.ToolQuery != "你可以帮我查一下 ZimaOS 的信息吗" {
		t.Fatalf("unexpected tool query: %q", cc.ToolQuery)
	}
}

func TestDeriveContinuationContext_DoesNotForceWhenAwaitingWithoutDefault(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "查天气"},
		{Role: llm.RoleAssistant, Content: "- [ ] 确认城市\n- [ ] 查询天气\n你要查哪个城市？"},
		{Role: llm.RoleUser, Content: "好的"},
	}

	cc := deriveContinuationContext("好的", msgs)
	if cc.Hint != "" || cc.ToolQuery != "" {
		t.Fatalf("expected no continuation context, got hint=%q tool_query=%q", cc.Hint, cc.ToolQuery)
	}
}

func TestIsAffirmativeContinuationMessage(t *testing.T) {
	cases := map[string]bool{
		"好的":        true,
		"继续吧":       true,
		"ok":        true,
		"sure":      true,
		"请继续执行":     true,
		"我想换个主题聊电影": false,
	}
	for input, want := range cases {
		if got := isAffirmativeContinuationMessage(input); got != want {
			t.Fatalf("input=%q got=%v want=%v", input, got, want)
		}
	}
}

// Test ChatHandler creation
func TestNewChatHandler(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()

	handler := NewChatHandler(store, registry, toolRegistry)
	if handler == nil {
		t.Fatal("expected handler, got nil")
	}
}

func TestDefaultModelForCCCLI(t *testing.T) {
	h := &ChatHandler{}
	if got := h.defaultModelForCCCLI(""); got != "auto" {
		t.Fatalf("default model without cc cli = %q, want %q", got, "auto")
	}
	if got := h.defaultModelForCCCLI("auto"); got != "auto" {
		t.Fatalf("auto model without cc cli = %q, want %q", got, "auto")
	}

	cc := claudecode.NewHandlerWithDataDir(nil, "", kvstore.NewMemoryStore())
	h.SetClaudeCodeHandler(cc)
	if got := h.defaultModelForCCCLI(""); got != defaultCCCLIModel {
		t.Fatalf("empty model with cc cli = %q, want %q", got, defaultCCCLIModel)
	}
	if got := h.defaultModelForCCCLI("auto"); got != defaultCCCLIModel {
		t.Fatalf("auto model with cc cli = %q, want %q", got, defaultCCCLIModel)
	}
	if got := h.defaultModelForCCCLI("gpt-4o"); got != "gpt-4o" {
		t.Fatalf("explicit model with cc cli = %q, want %q", got, "gpt-4o")
	}
}

func TestChatOnce_InjectsLocaleFromSettingsToProxyBridge(t *testing.T) {
	var gotLocale string
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLocale = r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_1","model":"auto","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)
	handler.SetProxyBridge(bridge)

	_, err := handler.chatOnce(context.Background(), llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	if gotLocale != "fr-FR" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "fr-FR")
	}
}

func TestChatOnce_DoesNotOverrideContextLocale(t *testing.T) {
	var gotLocale string
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLocale = r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_2","model":"auto","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)
	handler.SetProxyBridge(bridge)

	ctx := proxy.WithLocale(context.Background(), "ja-JP")
	_, err := handler.chatOnce(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	if gotLocale != "ja-JP" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "ja-JP")
	}
}

func TestSendMessage_PropagatesSettingsLocaleToUpstreamAcceptLanguage(t *testing.T) {
	const (
		modelID = "gpt-4o-mini"
		msgText = "nonstream locale from settings"
	)

	var gotLocale string
	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload)
		if bytes.Contains(raw, []byte(msgText)) {
			gotLocale = strings.TrimSpace(r.Header.Get("Accept-Language"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_nonstream_locale","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`))
	}))
	defer upstream.Close()

	proxyHandler := newSingleModelOpenAIProxyHandler(t, upstream.URL, "nonstream-locale-settings-provider", modelID)
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Non-stream locale settings")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "zh-CN"
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"`+msgText+`","model":"`+modelID+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotLocale != "zh-CN" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "zh-CN")
	}
}

func TestSendMessage_ContextLocaleOverridesSettingsForUpstreamAcceptLanguage(t *testing.T) {
	const (
		modelID = "gpt-4o-mini"
		msgText = "nonstream locale from context"
	)

	var gotLocale string
	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload)
		if bytes.Contains(raw, []byte(msgText)) {
			gotLocale = strings.TrimSpace(r.Header.Get("Accept-Language"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_nonstream_locale_ctx","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`))
	}))
	defer upstream.Close()

	proxyHandler := newSingleModelOpenAIProxyHandler(t, upstream.URL, "nonstream-locale-context-provider", modelID)
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Non-stream locale context")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"`+msgText+`","model":"`+modelID+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(proxy.WithLocale(req.Context(), "ja-JP"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotLocale != "ja-JP" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "ja-JP")
	}
}

// Test CreateConversation endpoint
func TestChatHandlerCreateConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"title": "Test Conversation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["id"] == nil {
		t.Error("expected id in response")
	}
	if resp["title"] != "Test Conversation" {
		t.Errorf("expected title 'Test Conversation', got '%v'", resp["title"])
	}
}

// Test ListConversations endpoint
func TestChatHandlerListConversations(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	// Create some conversations
	store.CreateConversation(context.Background(), "Conv 1")
	store.CreateConversation(context.Background(), "Conv 2")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListConversations(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(resp))
	}
}

// Test GetConversation endpoint
func TestChatHandlerGetConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["id"] != conv.ID {
		t.Errorf("expected id '%s', got '%v'", conv.ID, resp["id"])
	}
}

// Test GetConversation not found
func TestChatHandlerGetConversationNotFound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := handler.GetConversation(c)
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

// Test DeleteConversation endpoint
func TestChatHandlerDeleteConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "To Delete")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+conv.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.DeleteConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

// Test GetMessages endpoint
func TestChatHandlerGetMessages(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Hello"})
	store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "Hi!"})

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/messages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetMessages(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Errorf("expected 2 messages, got %d", len(resp))
	}
}

// Test SendMessage endpoint with mock provider
func TestChatHandlerSendMessage(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-123",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Hello! How can I help?"},
	})
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "mock", "model": "mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["content"] != "Hello! How can I help?" {
		t.Errorf("expected content 'Hello! How can I help?', got '%v'", resp["content"])
	}
}

func TestChatHandlerSendMessageAutoContinue_PseudoToolCallCommandWorkdirJSON(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Pseudo Tool Call SendMessage")

	registry := llm.NewProviderRegistry()
	pseudoContent := "好的，我来给你设一个 10 秒后的提醒。{\"command\":\"blue reminder.add message=\\\"喝水\\\" time=10s\",\"workdir\":\"/Users/orca/.zimaos-blue/data/workspace\"}{\"command\":\"...\"}\n```\nLet's do that exactly.{\"command\":\"blue help reminder\",...}\n```"
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "scripted-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 50, CompletionTokens: 90, TotalTokens: 140},
			},
			{
				ID:    "scripted-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "提醒已创建。",
				},
				Usage: llm.Usage{PromptTokens: 70, CompletionTokens: 8, TotalTokens: 78},
			},
		},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"提醒我10秒后喝水","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); got != "提醒已创建。" {
		t.Fatalf("expected retried final content, got %q", got)
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected two LLM rounds (pseudo + retry), got %d", scripted.CallCount())
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if len(secondReq.Messages) < 2 {
		t.Fatalf("expected second request to include continuation messages, got %d", len(secondReq.Messages))
	}
	last := secondReq.Messages[len(secondReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "fake tool-call text") {
		t.Fatalf("expected pseudo-tool nudge in second request, got role=%s content=%q", last.Role, last.Content)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, `{"command":"blue reminder.add`) || strings.Contains(m.Content, "Let's do that exactly") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from stored assistant message, got=%q", m.Content)
		}
	}
}

func TestChatHandlerSendMessageInjectsConversationAnchor(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Anchor Test Title")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Initial objective: fix context continuity"})
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "ack"})

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSystemPromptBuilder(claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{}))

	e := echo.New()
	reqBody := `{"message":"B","provider":"capture","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	lastReq := capture.LastRequest()
	if !hasSystemAnchor(lastReq.Messages, "Anchor Test Title", "Initial objective: fix context continuity") {
		t.Fatalf("expected conversation anchor in system messages, got %d messages", len(lastReq.Messages))
	}
}

func TestBuildConversationAnchorPromptEdgeCases(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())

	t.Run("empty_title_uses_fallback_title", func(t *testing.T) {
		conv, _ := store.CreateConversation(context.Background(), "")
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Goal A"})
		prompt := handler.buildConversationAnchorPrompt(context.Background(), conv.ID)
		if !strings.Contains(prompt, "Conversation title: Untitled conversation") {
			t.Fatalf("expected fallback title, got: %q", prompt)
		}
		if !strings.Contains(prompt, "Initial user goal: Goal A") {
			t.Fatalf("expected initial goal in prompt, got: %q", prompt)
		}
	})

	t.Run("long_title_and_goal_are_truncated", func(t *testing.T) {
		longTitle := strings.Repeat("测", 130)
		longGoal := strings.Repeat("g", 230)
		conv, _ := store.CreateConversation(context.Background(), longTitle)
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: longGoal})

		prompt := handler.buildConversationAnchorPrompt(context.Background(), conv.ID)
		expectedTitle := "Conversation title: " + strings.Repeat("测", 120) + "..."
		expectedGoal := "Initial user goal: " + strings.Repeat("g", 220) + "..."

		if !strings.Contains(prompt, expectedTitle) {
			t.Fatalf("expected truncated title in prompt, got: %q", prompt)
		}
		if !strings.Contains(prompt, expectedGoal) {
			t.Fatalf("expected truncated goal in prompt, got: %q", prompt)
		}
	})

	t.Run("empty_user_content_falls_back_to_scope_only", func(t *testing.T) {
		conv, _ := store.CreateConversation(context.Background(), "Only Scope")
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: ""})
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "ack"})
		prompt := handler.buildConversationAnchorPrompt(context.Background(), conv.ID)

		if !strings.Contains(prompt, "Conversation title: Only Scope") {
			t.Fatalf("expected title in prompt, got: %q", prompt)
		}
		if strings.Contains(prompt, "Initial user goal:") {
			t.Fatalf("did not expect initial goal line for empty user content, got: %q", prompt)
		}
		if !strings.Contains(prompt, "Keep replies aligned with this conversation scope") {
			t.Fatalf("expected scope fallback guidance, got: %q", prompt)
		}
	})
}

// Test SendMessage with invalid provider
func TestChatHandlerSendMessageInvalidProvider(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "nonexistent", "model": "model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestChatHandlerSendMessage_NoProviderFallsBackToDeepResearch(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Fallback Conv")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	settings.settings.Locale = "ja-JP"
	handler.SetSettingsHandler(settings)
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "Fallback answer from deep research.",
			"citations": []map[string]interface{}{
				{"title": "Doc A", "url": "https://example.com/a"},
			},
			"confidence":     0.9,
			"evidence_count": 1,
			"support_count":  3,
			"conflict_count": 1,
			"has_conflict":   true,
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"no provider configured","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "deepresearch" {
		t.Fatalf("provider = %v, want deepresearch", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "Fallback answer from deep research.") {
		t.Fatalf("content = %q, want fallback deep research answer", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "情報源:") {
		t.Fatalf("content = %q, want localized sources label", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "```typeless") {
		t.Fatalf("content = %q, want typeless card block", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "\"type\":\"deep-research\"") {
		t.Fatalf("content = %q, want deep-research typeless card", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "\"has_conflict\":true") {
		t.Fatalf("content = %q, want conflict fields in typeless card", got)
	}
	execMock.mu.Lock()
	gotLang, _ := execMock.last["lang"].(string)
	execMock.mu.Unlock()
	if gotLang != "ja-JP" {
		t.Fatalf("fallback lang = %q, want ja-JP", gotLang)
	}
}

func TestChatHandlerSendMessage_NoProviderFallsBackToIROnlyWhenDeepResearchUnavailable(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "IR fallback Conv")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"no provider and no deepresearch","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := resp["model"]; got != "ir-only-fallback" {
		t.Fatalf("model = %v, want ir-only-fallback", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "IR-only fallback") {
		t.Fatalf("content = %q, want IR-only fallback marker", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[fallbackReasonDeepResearchUnavailable] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonDeepResearchUnavailable, stats.FallbackReasons[fallbackReasonDeepResearchUnavailable])
	}
}

func TestChatHandlerSendMessage_NoProviderIROnlyUsesLayeredMemoryRecall(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "IR layered memory fallback")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	memDir := t.TempDir()
	mdBackend, err := memory.NewPureMarkdownBackend(memDir)
	if err != nil {
		t.Fatalf("create markdown backend: %v", err)
	}
	baseSvc := memory.NewUnifiedMemoryService(mdBackend)
	layeredSvc, err := memory.NewLayeredMemoryService(baseSvc, memory.LayeredMemoryConfig{
		BaseDir:     memDir,
		LongTermDir: memDir,
	})
	if err != nil {
		t.Fatalf("create layered memory service: %v", err)
	}
	handler.SetLayeredMemory(layeredSvc)
	if _, err := baseSvc.Remember(context.Background(), "Alpha project timeline is April 2026 with two release gates.", []string{"project", "timeline"}); err != nil {
		t.Fatalf("seed memory: %v", err)
	}

	e := echo.New()
	reqBody := `{"message":"What is Alpha project timeline?","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := resp["model"]; got != "ir-only-fallback" {
		t.Fatalf("model = %v, want ir-only-fallback", got)
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "Alpha project timeline is April 2026") {
		t.Fatalf("content = %q, want layered memory snippet", content)
	}
}

func TestChatHandlerSendMessage_ShortQARoutesToSmallModel(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA")
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{respText: "small model answer"}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"今天上海天气怎么样？","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sm.calls == 0 {
		t.Fatal("expected small model runtime to be called")
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "smallmodel" {
		t.Fatalf("provider = %v, want smallmodel", got)
	}
	if got := resp["content"]; got != "small model answer" {
		t.Fatalf("content = %v, want small model answer", got)
	}
}

func TestChatHandlerSendMessage_ShortQAFallbackIRFirst(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA IR First")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "Project X timeline is planned for Q4 delivery.",
	})
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{err: smallmodel.ErrNotReady}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"Project X timeline?","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sm.calls == 0 {
		t.Fatal("expected small model runtime to be called")
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := resp["model"]; got != "ir-first-fallback" {
		t.Fatalf("model = %v, want ir-first-fallback", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "Project X timeline is planned for Q4") {
		t.Fatalf("content = %q, want local IR snippet", got)
	}
}

func TestChatHandlerSendMessage_ShortQAFallbackIRMissGoesToLLM(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA IR Miss")
	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-ir-miss",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "llm fallback answer"},
	})
	registry.Register(mockProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{err: smallmodel.ErrNotReady}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"timeline?","provider":"","model":"mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got == "ir" {
		t.Fatalf("provider = %v, want non-IR LLM fallback", got)
	}
	if got := resp["content"]; got != "llm fallback answer" {
		t.Fatalf("content = %v, want llm fallback answer", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[fallbackReasonIRNoSignal] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonIRNoSignal, stats.FallbackReasons[fallbackReasonIRNoSignal])
	}
}

func TestChatHandlerSendMessage_ShortQACircuitBreakerFallsBackToIR(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA Circuit")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "Project Y milestone is still targeted for this quarter.",
	})
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)
	handler.smallModelBreaker = newSmallModelCircuitBreaker(2, time.Minute)
	sm := &smallModelRuntimeMock{err: context.DeadlineExceeded}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	send := func(msg string) map[string]interface{} {
		reqBody := `{"message":"` + msg + `","provider":"","model":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)
		if err := handler.SendMessage(c); err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp
	}

	_ = send("Project Y milestone?")
	_ = send("Project Y milestone?")
	resp3 := send("Project Y milestone?")
	if got := resp3["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := sm.calls; got != 2 {
		t.Fatalf("small model calls = %d, want 2 (third request should be circuit-open bypass)", got)
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[smallmodel.FallbackReasonTimeout] != 2 {
		t.Fatalf("fallback reason %q = %d, want 2", smallmodel.FallbackReasonTimeout, stats.FallbackReasons[smallmodel.FallbackReasonTimeout])
	}
	if stats.FallbackReasons[smallmodel.FallbackReasonCircuitOpen] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", smallmodel.FallbackReasonCircuitOpen, stats.FallbackReasons[smallmodel.FallbackReasonCircuitOpen])
	}
}

func TestTrySmallModelShortQA_CircuitBreakerRecoversAfterWindow(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.smallModelBreaker = newSmallModelCircuitBreaker(1, 25*time.Millisecond)
	sm := &smallModelRuntimeMock{err: context.DeadlineExceeded, respText: "ok after recover"}
	handler.SetSmallModelRuntime(sm)

	if _, err := handler.trySmallModelShortQA(context.Background(), "Q?", 32, 0.2); err == nil {
		t.Fatal("expected first call to fail")
	}
	if _, err := handler.trySmallModelShortQA(context.Background(), "Q?", 32, 0.2); !errors.Is(err, smallmodel.ErrCircuitOpen) {
		t.Fatalf("expected circuit-open error, got %v", err)
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1 before recovery window", sm.calls)
	}

	time.Sleep(40 * time.Millisecond)
	sm.err = nil
	resp, err := handler.trySmallModelShortQA(context.Background(), "Q?", 32, 0.2)
	if err != nil {
		t.Fatalf("expected recovery success, got %v", err)
	}
	if resp == nil || resp.Message.Content != "ok after recover" {
		t.Fatalf("unexpected response after recovery: %+v", resp)
	}
}

func TestMaybeAutoRollbackShortQARoute_DisablesRouteOnHighFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordShortQARoute(false)
	}

	handler.maybeAutoRollbackShortQARoute()
	if settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route to be auto-disabled")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
	if stats.FallbackReasons[fallbackReasonAutoRollback] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonAutoRollback, stats.FallbackReasons[fallbackReasonAutoRollback])
	}
}

func TestMaybeAutoRollbackShortQARoute_DoesNotDisableBelowThreshold(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 39; i++ {
		handler.smallModelStats.RecordShortQARoute(false)
	}

	handler.maybeAutoRollbackShortQARoute()
	if !settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route to stay enabled below sample threshold")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 0 {
		t.Fatalf("AutoRollbackTotal = %d, want 0", stats.AutoRollbackTotal)
	}
}

func TestMaybeAutoRollbackShortQARoute_UsesWindowedFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)

	// Healthy first window should not disable and should advance baseline.
	for i := 0; i < 400; i++ {
		handler.smallModelStats.RecordShortQARoute(true)
	}
	handler.maybeAutoRollbackShortQARoute()
	if !settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route still enabled after healthy window")
	}

	// Next window is unhealthy. Cumulative fail rate is still low (40 / 440 < 0.15),
	// so this would fail if logic were not windowed.
	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordShortQARoute(false)
	}
	handler.maybeAutoRollbackShortQARoute()
	if settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route auto-disabled based on windowed failure rate")
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
}

func TestChatHandlerSendMessage_ToolDispatchRoutesToSmallModel(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Tool Dispatch Route")
	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "alpha_tool", Description: "alpha"}})
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "beta_tool", Description: "beta"}})

	handler := NewChatHandler(store, registry, toolRegistry)
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	toolDispatchEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteToolDispatchEnabled = &toolDispatchEnabled
	handler.SetSettingsHandler(settings)
	handler.SetSmallModelRuntime(&smallModelRuntimeMock{respText: "beta_tool"})

	e := echo.New()
	reqBody := `{"message":"请帮我处理这个任务","provider":"capture","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	lastReq := capture.LastRequest()
	if got := len(lastReq.Tools); got != 1 {
		t.Fatalf("tool count = %d, want 1", got)
	}
	if got := lastReq.Tools[0].Name; got != "beta_tool" {
		t.Fatalf("selected tool = %q, want beta_tool", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.ToolDispatchRouteAttempts != 1 || stats.ToolDispatchRouteSuccess != 1 {
		t.Fatalf("unexpected tool dispatch stats: %+v", stats)
	}
}

func TestChatHandlerSendMessage_ToolDispatchInvalidChoiceFallsBackToDefaultTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Tool Dispatch Fallback")
	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "alpha_tool", Description: "alpha"}})
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "beta_tool", Description: "beta"}})

	handler := NewChatHandler(store, registry, toolRegistry)
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	toolDispatchEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteToolDispatchEnabled = &toolDispatchEnabled
	handler.SetSettingsHandler(settings)
	handler.SetSmallModelRuntime(&smallModelRuntimeMock{respText: "unknown_tool"})

	e := echo.New()
	reqBody := `{"message":"请帮我处理这个任务","provider":"capture","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	lastReq := capture.LastRequest()
	if got := len(lastReq.Tools); got != 2 {
		t.Fatalf("tool count = %d, want 2", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.ToolDispatchRouteAttempts != 1 || stats.ToolDispatchRouteSuccess != 0 {
		t.Fatalf("unexpected tool dispatch stats: %+v", stats)
	}
	if stats.FallbackReasons[smallmodel.FallbackReasonResourceGuard] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", smallmodel.FallbackReasonResourceGuard, stats.FallbackReasons[smallmodel.FallbackReasonResourceGuard])
	}
}

func TestChatHandlerStreamMessage_ToolDispatchRoutesToSmallModel(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Tool Dispatch Stream Route")
	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "alpha_tool", Description: "alpha"}})
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "beta_tool", Description: "beta"}})

	handler := NewChatHandler(store, registry, toolRegistry)
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	toolDispatchEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteToolDispatchEnabled = &toolDispatchEnabled
	handler.SetSettingsHandler(settings)
	handler.SetSmallModelRuntime(&smallModelRuntimeMock{respText: "alpha_tool"})

	e := echo.New()
	reqBody := `{"message":"请帮我处理这个任务","provider":"capture","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", rec.Body.String())
	}

	lastReq := capture.LastRequest()
	if got := len(lastReq.Tools); got != 1 {
		t.Fatalf("tool count = %d, want 1", got)
	}
	if got := lastReq.Tools[0].Name; got != "alpha_tool" {
		t.Fatalf("selected tool = %q, want alpha_tool", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.ToolDispatchRouteAttempts != 1 || stats.ToolDispatchRouteSuccess != 1 {
		t.Fatalf("unexpected tool dispatch stats: %+v", stats)
	}
}

func TestMaybeAutoRollbackToolDispatchRoute_DisablesRouteOnHighFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	toolDispatchEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteToolDispatchEnabled = &toolDispatchEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordToolDispatchRoute(false)
	}

	handler.maybeAutoRollbackToolDispatchRoute()
	if settings.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected tool-dispatch route to be auto-disabled")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
	if stats.FallbackReasons[fallbackReasonAutoRollbackToolDispatch] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonAutoRollbackToolDispatch, stats.FallbackReasons[fallbackReasonAutoRollbackToolDispatch])
	}
}

func TestMaybeAutoRollbackToolDispatchRoute_UsesWindowedFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	toolDispatchEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteToolDispatchEnabled = &toolDispatchEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 400; i++ {
		handler.smallModelStats.RecordToolDispatchRoute(true)
	}
	handler.maybeAutoRollbackToolDispatchRoute()
	if !settings.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected tool-dispatch route still enabled after healthy window")
	}

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordToolDispatchRoute(false)
	}
	handler.maybeAutoRollbackToolDispatchRoute()
	if settings.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected tool-dispatch route auto-disabled based on windowed failure rate")
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
}

func TestMaybeAutoRollbackSummaryRoute_DisablesRouteOnHighFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	summaryEnabled := true
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordSummaryAttempt()
	}

	handler.maybeAutoRollbackSummaryRoute()
	if settings.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary route to be auto-disabled")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
	if stats.FallbackReasons[fallbackReasonAutoRollbackSummary] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonAutoRollbackSummary, stats.FallbackReasons[fallbackReasonAutoRollbackSummary])
	}
}

func TestMaybeAutoRollbackSummaryRoute_UsesWindowedFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	summaryEnabled := true
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 400; i++ {
		handler.smallModelStats.RecordSummaryAttempt()
		handler.smallModelStats.RecordSummarySuccess()
	}
	handler.maybeAutoRollbackSummaryRoute()
	if !settings.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary route still enabled after healthy window")
	}

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordSummaryAttempt()
	}
	handler.maybeAutoRollbackSummaryRoute()
	if settings.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary route auto-disabled based on windowed failure rate")
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
}

func TestRunSmallModelShadow_RecordsQualityDeltaAndPersistsSample(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSmallModelRuntime(&smallModelRuntimeMock{respText: "beta_tool"})
	handler.SetShadowQualityStore(NewShadowQualityStore(kvstore.NewMemoryStore(), 16))

	handler.runSmallModelShadow(context.Background(), "tool_dispatch_shadow", "pick tool", 32, 0.2, "alpha_tool")

	stats := handler.smallModelStats.Snapshot()
	if stats.ShadowQualitySamples != 1 {
		t.Fatalf("ShadowQualitySamples = %d, want 1", stats.ShadowQualitySamples)
	}
	if stats.ShadowQualityDelta != 1 {
		t.Fatalf("ShadowQualityDelta = %v, want 1", stats.ShadowQualityDelta)
	}
	if len(handler.shadowQualityStore.Snapshot()) != 1 {
		t.Fatalf("persisted shadow samples = %d, want 1", len(handler.shadowQualityStore.Snapshot()))
	}
}

func TestChatHandlerShouldDisableProxyPruner(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetSettingsHandler(settings)

	if handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner enabled by default")
	}

	smallModelEnabled := true
	contextPruneEnabled := false
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	if !handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner disabled when small model enabled and context prune switch off")
	}

	contextPruneEnabled = true
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	if handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner enabled when context prune switch on")
	}
}

func TestChatHandlerSendMessage_ShortQAShadowDoesNotAffectMainPath(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA Shadow")
	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-shadow",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "main path answer"},
	})
	registry.Register(mockProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := false // shadow only
	shadowRatio := 1.0
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	settings.settings.SmallModelShadowRatio = &shadowRatio
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{
		respText: "shadow answer",
		calledCh: make(chan struct{}, 1),
	}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"这是一个短问题吗？","provider":"","model":"mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["content"]; got != "main path answer" {
		t.Fatalf("content = %v, want main path answer", got)
	}
	select {
	case <-sm.calledCh:
		// expected: shadow executed
	case <-time.After(2 * time.Second):
		t.Fatal("expected shadow inference call")
	}
}

func TestShouldRunSmallModelShadow_RatioOneAlwaysTrue(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()
	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	ratio := 1.0
	settings.settings.SmallModelShadowRatio = &ratio
	handler.SetSettingsHandler(settings)

	if !handler.shouldRunSmallModelShadow("shadow-key-1") {
		t.Fatal("expected shadow sampling to pass when ratio=1")
	}
	if !handler.shouldRunSmallModelShadow("shadow-key-2") {
		t.Fatal("expected shadow sampling to pass when ratio=1")
	}
}

func TestShouldRunSmallModelShadow_DeterministicForSameKey(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()
	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	ratio := 0.1
	settings.settings.SmallModelShadowRatio = &ratio
	handler.SetSettingsHandler(settings)

	got1 := handler.shouldRunSmallModelShadow("short_qa_shadow|conv-a|你好")
	got2 := handler.shouldRunSmallModelShadow("short_qa_shadow|conv-a|你好")
	if got1 != got2 {
		t.Fatalf("expected deterministic shadow sampling, got %v then %v", got1, got2)
	}
}

func TestSmallModelStatsHandlers(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.smallModelStats.RecordShortQARoute(true)
	handler.smallModelStats.RecordToolDispatchRoute(true)
	handler.smallModelStats.RecordShadow("short_qa_shadow", true)
	handler.smallModelStats.RecordFallback(fallbackReasonDeepResearchUnavailable)
	handler.smallModelStats.RecordFallback("timeout")
	handler.smallModelStats.RecordLatencyWithScene("short_qa", 10*time.Millisecond)
	handler.smallModelStats.RecordLatencyWithScene("tool_dispatch", 30*time.Millisecond)
	handler.smallModelStats.RecordLatencyWithScene("summary", 20*time.Millisecond)
	handler.smallModelStats.RecordShadowQualityDelta(0.5)
	handler.smallModelStats.RecordAutoRollback()
	handler.smallModelStats.RecordIRTakeover()

	e := echo.New()

	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/stats", nil)
	statsRec := httptest.NewRecorder()
	statsCtx := e.NewContext(statsReq, statsRec)
	if err := handler.SmallModelStatsHandler(statsCtx); err != nil {
		t.Fatalf("SmallModelStatsHandler failed: %v", err)
	}
	if statsRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", statsRec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(statsRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if got := payload["short_qa_route_attempts"]; got != float64(1) {
		t.Fatalf("short_qa_route_attempts = %v, want 1", got)
	}
	if got := payload["tool_dispatch_route_attempts"]; got != float64(1) {
		t.Fatalf("tool_dispatch_route_attempts = %v, want 1", got)
	}
	if got := payload["ir_takeover_total"]; got != float64(1) {
		t.Fatalf("ir_takeover_total = %v, want 1", got)
	}
	if got := payload["auto_rollback_total"]; got != float64(1) {
		t.Fatalf("auto_rollback_total = %v, want 1", got)
	}
	if got := payload["small_model_fallback_total"]; got != float64(2) {
		t.Fatalf("small_model_fallback_total = %v, want 2", got)
	}
	if got := payload["small_model_timeout_total"]; got != float64(1) {
		t.Fatalf("small_model_timeout_total = %v, want 1", got)
	}
	if got := payload["small_model_latency_samples"]; got != float64(3) {
		t.Fatalf("small_model_latency_samples = %v, want 3", got)
	}
	if got := payload["small_model_latency_ms"]; got != float64(20) {
		t.Fatalf("small_model_latency_ms = %v, want 20", got)
	}
	if got := payload["short_qa_latency_ms"]; got != float64(10) {
		t.Fatalf("short_qa_latency_ms = %v, want 10", got)
	}
	if got := payload["tool_dispatch_latency_ms"]; got != float64(30) {
		t.Fatalf("tool_dispatch_latency_ms = %v, want 30", got)
	}
	if got := payload["summary_latency_ms"]; got != float64(20) {
		t.Fatalf("summary_latency_ms = %v, want 20", got)
	}
	if got := payload["shadow_quality_delta"]; got != float64(0.5) {
		t.Fatalf("shadow_quality_delta = %v, want 0.5", got)
	}
	if got := payload["shadow_quality_samples"]; got != float64(1) {
		t.Fatalf("shadow_quality_samples = %v, want 1", got)
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/small-model/stats/reset", nil)
	resetRec := httptest.NewRecorder()
	resetCtx := e.NewContext(resetReq, resetRec)
	if err := handler.ResetSmallModelStatsHandler(resetCtx); err != nil {
		t.Fatalf("ResetSmallModelStatsHandler failed: %v", err)
	}
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", resetRec.Code)
	}

	statsReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/stats", nil)
	statsRec2 := httptest.NewRecorder()
	statsCtx2 := e.NewContext(statsReq2, statsRec2)
	if err := handler.SmallModelStatsHandler(statsCtx2); err != nil {
		t.Fatalf("SmallModelStatsHandler after reset failed: %v", err)
	}
	var payload2 map[string]interface{}
	if err := json.Unmarshal(statsRec2.Body.Bytes(), &payload2); err != nil {
		t.Fatalf("decode stats after reset: %v", err)
	}
	if got := payload2["short_qa_route_attempts"]; got != float64(0) {
		t.Fatalf("short_qa_route_attempts = %v, want 0", got)
	}
	if got := payload2["tool_dispatch_route_attempts"]; got != float64(0) {
		t.Fatalf("tool_dispatch_route_attempts = %v, want 0", got)
	}
	if got := payload2["ir_takeover_total"]; got != float64(0) {
		t.Fatalf("ir_takeover_total = %v, want 0", got)
	}
	if got := payload2["auto_rollback_total"]; got != float64(0) {
		t.Fatalf("auto_rollback_total = %v, want 0", got)
	}
	if got := payload2["small_model_fallback_total"]; got != float64(0) {
		t.Fatalf("small_model_fallback_total = %v, want 0", got)
	}
	if got := payload2["small_model_timeout_total"]; got != float64(0) {
		t.Fatalf("small_model_timeout_total = %v, want 0", got)
	}
	if got := payload2["small_model_latency_samples"]; got != float64(0) {
		t.Fatalf("small_model_latency_samples = %v, want 0", got)
	}
	if got := payload2["short_qa_latency_samples"]; got != float64(0) {
		t.Fatalf("short_qa_latency_samples = %v, want 0", got)
	}
}

func TestSmallModelShadowQualityHandlers(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetShadowQualityStore(NewShadowQualityStore(kvstore.NewMemoryStore(), 32))
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{
		Scene:        "short_qa_shadow",
		Delta:        0.2,
		MainDigest:   "main answer A",
		ShadowDigest: "shadow answer A",
		CreatedAt:    time.Now().UTC(),
	})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{
		Scene:        "tool_dispatch_shadow",
		Delta:        1.0,
		MainDigest:   "alpha_tool",
		ShadowDigest: "beta_tool",
		CreatedAt:    time.Now().UTC(),
	})

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/shadow-quality?scene=tool_dispatch_shadow&limit=1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := handler.SmallModelShadowQualityHandler(ctx); err != nil {
		t.Fatalf("SmallModelShadowQualityHandler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["total"]; got != float64(1) {
		t.Fatalf("total = %v, want 1", got)
	}
	if got := payload["average_delta"]; got != float64(1) {
		t.Fatalf("average_delta = %v, want 1", got)
	}
	samples, ok := payload["samples"].([]interface{})
	if !ok || len(samples) != 1 {
		t.Fatalf("samples = %T/%v, want one sample", payload["samples"], payload["samples"])
	}
	sample, ok := samples[0].(map[string]interface{})
	if !ok {
		t.Fatalf("sample type = %T, want object", samples[0])
	}
	if got := sample["scene"]; got != "tool_dispatch_shadow" {
		t.Fatalf("scene = %v, want tool_dispatch_shadow", got)
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/small-model/shadow-quality/reset", nil)
	resetRec := httptest.NewRecorder()
	resetCtx := e.NewContext(resetReq, resetRec)
	if err := handler.ResetSmallModelShadowQualityHandler(resetCtx); err != nil {
		t.Fatalf("ResetSmallModelShadowQualityHandler failed: %v", err)
	}
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", resetRec.Code)
	}
	if got := len(handler.shadowQualityStore.Snapshot()); got != 0 {
		t.Fatalf("samples after reset = %d, want 0", got)
	}
}

func TestSmallModelShadowQualityGateEvalHandler(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetShadowQualityStore(NewShadowQualityStore(kvstore.NewMemoryStore(), 64))
	now := time.Now().UTC()
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.2, CreatedAt: now})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.3, CreatedAt: now.Add(time.Second)})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "tool_dispatch_shadow", Delta: 0.9, CreatedAt: now.Add(2 * time.Second)})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "tool_dispatch_shadow", Delta: 1.0, CreatedAt: now.Add(3 * time.Second)})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/shadow-quality/gate-eval?min_samples=2&threshold_delta=0.4", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := handler.SmallModelShadowQualityGateEvalHandler(ctx); err != nil {
		t.Fatalf("SmallModelShadowQualityGateEvalHandler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["overall_pass"]; got != false {
		t.Fatalf("overall_pass = %v, want false", got)
	}
	if got := payload["evaluated_samples"]; got != float64(4) {
		t.Fatalf("evaluated_samples = %v, want 4", got)
	}
	scenes, ok := payload["scenes"].([]interface{})
	if !ok || len(scenes) != 2 {
		t.Fatalf("scenes = %T/%v, want 2 scenes", payload["scenes"], payload["scenes"])
	}
	foundFail := false
	for _, row := range scenes {
		scene, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		if scene["scene"] == "tool_dispatch_shadow" {
			foundFail = true
			if scene["pass"] != false {
				t.Fatalf("tool_dispatch pass = %v, want false", scene["pass"])
			}
			if scene["reason"] != "delta_exceeds_threshold" {
				t.Fatalf("tool_dispatch reason = %v, want delta_exceeds_threshold", scene["reason"])
			}
		}
	}
	if !foundFail {
		t.Fatal("expected tool_dispatch_shadow result in scenes")
	}
}

func TestSmallModelShadowQualityGateEvalHandler_UsesSettingsDefaults(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetShadowQualityStore(NewShadowQualityStore(kvstore.NewMemoryStore(), 64))
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	_, _ = settings.SetSmallModelShadowGateMinSamples(2)
	_, _ = settings.SetSmallModelShadowGateThresholdDelta(0.2)
	handler.SetSettingsHandler(settings)

	now := time.Now().UTC()
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.25, CreatedAt: now})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.25, CreatedAt: now.Add(time.Second)})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/shadow-quality/gate-eval", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := handler.SmallModelShadowQualityGateEvalHandler(ctx); err != nil {
		t.Fatalf("SmallModelShadowQualityGateEvalHandler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["threshold_delta"]; got != 0.2 {
		t.Fatalf("threshold_delta = %v, want 0.2", got)
	}
	if got := payload["min_samples"]; got != float64(2) {
		t.Fatalf("min_samples = %v, want 2", got)
	}
	if got := payload["overall_pass"]; got != false {
		t.Fatalf("overall_pass = %v, want false", got)
	}
}

func TestSmallModelShadowQualityGateEvalHandler_UsesSettingsDefaultScene(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetShadowQualityStore(NewShadowQualityStore(kvstore.NewMemoryStore(), 64))
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	_, _ = settings.SetSmallModelShadowGateMinSamples(2)
	_, _ = settings.SetSmallModelShadowGateThresholdDelta(0.4)
	_, _ = settings.SetSmallModelShadowGateScene("tool_dispatch_shadow")
	handler.SetSettingsHandler(settings)

	now := time.Now().UTC()
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.1, CreatedAt: now})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "tool_dispatch_shadow", Delta: 0.2, CreatedAt: now.Add(time.Second)})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "tool_dispatch_shadow", Delta: 0.3, CreatedAt: now.Add(2 * time.Second)})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/shadow-quality/gate-eval", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := handler.SmallModelShadowQualityGateEvalHandler(ctx); err != nil {
		t.Fatalf("SmallModelShadowQualityGateEvalHandler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["scene_filter"]; got != "tool_dispatch_shadow" {
		t.Fatalf("scene_filter = %v, want tool_dispatch_shadow", got)
	}
	if got := payload["evaluated_samples"]; got != float64(2) {
		t.Fatalf("evaluated_samples = %v, want 2", got)
	}
}

func TestSmallModelShadowAutoRolloutExecuteHandler(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetShadowQualityStore(NewShadowQualityStore(kvstore.NewMemoryStore(), 64))
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	_, _ = settings.SetSmallModelShadowRatio(0.1)
	_, _ = settings.SetSmallModelShadowGateMinSamples(2)
	_, _ = settings.SetSmallModelShadowGateThresholdDelta(0.4)
	_, _ = settings.SetSmallModelShadowGateScene("short_qa_shadow")
	handler.SetSettingsHandler(settings)

	now := time.Now().UTC()
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.2, CreatedAt: now})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "short_qa_shadow", Delta: 0.3, CreatedAt: now.Add(time.Second)})
	_ = handler.shadowQualityStore.Append(ShadowQualitySample{Scene: "tool_dispatch_shadow", Delta: 1.0, CreatedAt: now.Add(2 * time.Second)})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/small-model/shadow-quality/auto-rollout/execute", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := handler.SmallModelShadowAutoRolloutExecuteHandler(ctx); err != nil {
		t.Fatalf("SmallModelShadowAutoRolloutExecuteHandler failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["advanced"]; got != true {
		t.Fatalf("advanced = %v, want true", got)
	}
	if got := payload["current_ratio"]; got != 0.1 {
		t.Fatalf("current_ratio = %v, want 0.1", got)
	}
	if got := payload["next_ratio"]; got != 0.3 {
		t.Fatalf("next_ratio = %v, want 0.3", got)
	}
	if got := settings.GetSmallModelShadowRatio(); got != 0.3 {
		t.Fatalf("persisted ratio = %v, want 0.3", got)
	}

	gateEval, ok := payload["gate_eval"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_eval type = %T, want object", payload["gate_eval"])
	}
	if got := gateEval["overall_pass"]; got != true {
		t.Fatalf("gate_eval.overall_pass = %v, want true", got)
	}

	_, _ = settings.SetSmallModelShadowGateThresholdDelta(0.1)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/small-model/shadow-quality/auto-rollout/execute", nil)
	rec2 := httptest.NewRecorder()
	ctx2 := e.NewContext(req2, rec2)
	if err := handler.SmallModelShadowAutoRolloutExecuteHandler(ctx2); err != nil {
		t.Fatalf("SmallModelShadowAutoRolloutExecuteHandler second call failed: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", rec2.Code)
	}
	var payload2 map[string]interface{}
	if err := json.Unmarshal(rec2.Body.Bytes(), &payload2); err != nil {
		t.Fatalf("decode second payload: %v", err)
	}
	if got := payload2["advanced"]; got != false {
		t.Fatalf("second advanced = %v, want false", got)
	}
	if got := payload2["reason"]; got != "gate_not_passed" {
		t.Fatalf("second reason = %v, want gate_not_passed", got)
	}
	if got := settings.GetSmallModelShadowRatio(); got != 0.3 {
		t.Fatalf("persisted ratio after hold = %v, want 0.3", got)
	}
}

func TestChatHandlerSendMessageSlashCommandsAndOffline(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	e := echo.New()

	callSend := func(body string) map[string]interface{} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)

		if err := handler.SendMessage(c); err != nil {
			t.Fatalf("SendMessage error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp
	}

	resp := callSend(`{"message":"/model test-model","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "test-model") {
		t.Fatalf("unexpected /model response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/model","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "test-model") {
		t.Fatalf("unexpected /model query response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/status","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "test-model") {
		t.Fatalf("unexpected /status response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/ping","provider":"","model":""}`)
	if resp["content"] != "pong" {
		t.Fatalf("unexpected /ping response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/title Renamed Conv","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "Renamed Conv") {
		t.Fatalf("unexpected /title response: %v", resp["content"])
	}
	updatedConv, err := store.GetConversation(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("failed to get conversation after /title: %v", err)
	}
	if updatedConv.Title != "Renamed Conv" {
		t.Fatalf("conversation title = %q, want %q", updatedConv.Title, "Renamed Conv")
	}

	resp = callSend(`{"message":"/models","provider":"","model":""}`)
	modelsContent := resp["content"].(string)
	if !strings.Contains(modelsContent, "Available models") && !strings.Contains(modelsContent, "No model list") {
		t.Fatalf("unexpected /models response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/offline on","provider":"","model":""}`)
	if !strings.Contains(strings.ToLower(resp["content"].(string)), "offline mode is now on") {
		t.Fatalf("unexpected /offline on response: %v", resp["content"])
	}

	resp = callSend(`{"message":"hello offline","provider":"","model":""}`)
	content := resp["content"].(string)
	if !strings.Contains(content, "Offline mode response") {
		t.Fatalf("expected offline response, got: %s", content)
	}
	if got := resp["provider"]; got != "local" {
		t.Fatalf("provider = %v, want local", got)
	}
	if got := resp["model"]; got != "offline" {
		t.Fatalf("model = %v, want offline", got)
	}

	resp = callSend(`{"message":"/clear","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "Conversation cleared") {
		t.Fatalf("unexpected /clear response: %v", resp["content"])
	}

	msgs, err := store.GetMessages(context.Background(), conv.ID, 1000, 0)
	if err != nil {
		t.Fatalf("failed to list messages after clear: %v", err)
	}
	// /clear command stores one user + one assistant confirmation after wiping old history.
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after clear command, got %d", len(msgs))
	}
}

func TestChatHandlerStreamMessageOfflineMode(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	e := echo.New()

	// Enable offline mode first.
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"/offline on","provider":"","model":""}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)
		if err := handler.SendMessage(c); err != nil {
			t.Fatalf("enable offline failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(`{"message":"stream hello","provider":"","model":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Offline mode response") {
		t.Fatalf("expected offline stream content, got: %s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done chunk in stream, got: %s", body)
	}
}

// Test ListProviders endpoint
func TestChatHandlerListProviders(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	registry.Register(llm.NewMockProvider())

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListProviders(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 1 {
		t.Errorf("expected 1 provider, got %d", len(resp))
	}
}

// Test ListTools endpoint
func TestChatHandlerListTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)

	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListTools(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	expected := len(toolRegistry.Definitions())
	if len(resp) != expected {
		t.Errorf("expected %d tools, got %d", expected, len(resp))
	}
}

// Test StreamMessage with Provider Pool ID mapping
func TestChatHandlerStreamMessageWithProviderPoolID(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	// Use CustomProvider which has Name() = "custom"
	// Provider Pool IDs that don't match known mappings will map to "custom"
	customProvider := llm.NewCustomProvider("test-key", "http://localhost")
	registry.Register(customProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	// Use a Provider Pool ID format (prov_<hex>)
	// This should be mapped to "custom" by mapProviderID()
	reqBody := `{"message": "Hello!", "provider": "prov_992ef6e8c8ad938a", "model": "test-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.StreamMessage(c)
	// The provider should be found (mapped to "custom")
	// We expect an error from the actual API call (since we're using a fake endpoint),
	// but NOT a "provider not found" error
	if err != nil {
		httpErr, ok := err.(*echo.HTTPError)
		if ok && httpErr.Code == http.StatusBadRequest {
			// Check if it's the "provider not found" error
			if msg, ok := httpErr.Message.(string); ok && bytes.Contains([]byte(msg), []byte("provider not found")) {
				t.Fatalf("StreamMessage should map Provider Pool ID to LLM provider name, but got error: %v", err)
			}
		}
		// Other errors are acceptable (e.g., API call failures, streaming setup issues)
		// The important thing is that the provider was found
	}
}

// MockMetricsRecorder implements MetricsRecorder for testing
type MockMetricsRecorder struct {
	APICalls   []MockAPICallRecord
	SpeedCalls []MockSpeedRecord
}

type MockAPICallRecord struct {
	Model        string
	Success      bool
	LatencyMs    float64
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	ErrorType    string
}

type MockSpeedRecord struct {
	Model           string
	TokensPerSecond float64
	TTFTMs          float64
	DecodeSpeed     float64
}

func (m *MockMetricsRecorder) RecordAPICall(model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string) {
	m.APICalls = append(m.APICalls, MockAPICallRecord{
		Model:        model,
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		ErrorType:    errorType,
	})
}

func (m *MockMetricsRecorder) RecordAPICallForUser(userID, model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string) {
	m.APICalls = append(m.APICalls, MockAPICallRecord{
		Model:        model,
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		ErrorType:    errorType,
	})
}

func (m *MockMetricsRecorder) RecordSpeed(model string, tokensPerSecond, ttftMs, decodeSpeed float64) {
	m.SpeedCalls = append(m.SpeedCalls, MockSpeedRecord{
		Model:           model,
		TokensPerSecond: tokensPerSecond,
		TTFTMs:          ttftMs,
		DecodeSpeed:     decodeSpeed,
	})
}

// Test SetMetricsRecorder
func TestChatHandlerSetMetricsRecorder(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	mockRecorder := &MockMetricsRecorder{}
	handler.SetMetricsRecorder(mockRecorder)

	if handler.metricsRecorder == nil {
		t.Error("expected metricsRecorder to be set")
	}
}

// Test SendMessage records metrics
func TestChatHandlerSendMessageRecordsMetrics(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-123",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Hello!"},
		Usage: llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	})
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	mockRecorder := &MockMetricsRecorder{}
	handler.SetMetricsRecorder(mockRecorder)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "mock", "model": "mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	// Verify metrics were recorded
	if len(mockRecorder.APICalls) != 1 {
		t.Errorf("expected 1 API call recorded, got %d", len(mockRecorder.APICalls))
	}

	if len(mockRecorder.APICalls) > 0 {
		call := mockRecorder.APICalls[0]
		if !call.Success {
			t.Error("expected successful API call")
		}
		if call.InputTokens != 10 {
			t.Errorf("expected 10 input tokens, got %d", call.InputTokens)
		}
		if call.OutputTokens != 5 {
			t.Errorf("expected 5 output tokens, got %d", call.OutputTokens)
		}
		// Latency can be 0 for mock provider (instant response)
		if call.LatencyMs < 0 {
			t.Error("expected non-negative latency")
		}
	}
}

// Test mapProviderID function
func TestMapProviderID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"anthropic", "claude"},
		{"openai", "openai"},
		{"ollama", "ollama"},
		{"custom", "custom"},
		{"grok", "grok"},
		{"qwen", "qwen"},
		{"venice", "venice"},
		{"bedrock", "bedrock"},
		{"glm", "glm"},
		{"claude", "claude"},
		// Unknown providers return as-is
		{"prov_abc123", "prov_abc123"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapProviderID(tt.input)
			if result != tt.expected {
				t.Errorf("mapProviderID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test message stats persistence
func TestMessageStatsPersistence(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	// Add a message with stats
	stats := &memory.MessageStats{
		InputTokens:     100,
		OutputTokens:    50,
		TotalTokens:     150,
		LatencyMs:       1234,
		TTFTMs:          567,
		TokensPerSecond: 25.5,
	}

	msg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Test response",
		Provider: "openai",
		Model:    "gpt-4",
		Stats:    stats,
	})
	if err != nil {
		t.Fatalf("failed to add message: %v", err)
	}

	// Retrieve messages and verify stats
	messages, err := store.GetMessages(context.Background(), conv.ID, 100, 0)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	retrieved := messages[0]

	// Verify basic fields
	if retrieved.ID != msg.ID {
		t.Errorf("expected ID %s, got %s", msg.ID, retrieved.ID)
	}
	if retrieved.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", retrieved.Provider)
	}
	if retrieved.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%s'", retrieved.Model)
	}

	// Verify stats
	if retrieved.Stats == nil {
		t.Fatal("expected stats to be present")
	}
	if retrieved.Stats.InputTokens != 100 {
		t.Errorf("expected input_tokens 100, got %d", retrieved.Stats.InputTokens)
	}
	if retrieved.Stats.OutputTokens != 50 {
		t.Errorf("expected output_tokens 50, got %d", retrieved.Stats.OutputTokens)
	}
	if retrieved.Stats.TotalTokens != 150 {
		t.Errorf("expected total_tokens 150, got %d", retrieved.Stats.TotalTokens)
	}
	if retrieved.Stats.LatencyMs != 1234 {
		t.Errorf("expected latency_ms 1234, got %d", retrieved.Stats.LatencyMs)
	}
	if retrieved.Stats.TTFTMs != 567 {
		t.Errorf("expected ttft_ms 567, got %d", retrieved.Stats.TTFTMs)
	}
	if retrieved.Stats.TokensPerSecond != 25.5 {
		t.Errorf("expected tokens_per_second 25.5, got %f", retrieved.Stats.TokensPerSecond)
	}
}

// Test GetMessages returns stats in JSON response
func TestChatHandlerGetMessagesWithStats(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	// Add user message
	store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Hello",
	})

	// Add assistant message with stats
	store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Hi there!",
		Provider: "openai",
		Model:    "gpt-4",
		Stats: &memory.MessageStats{
			InputTokens:     10,
			OutputTokens:    5,
			TotalTokens:     15,
			LatencyMs:       500,
			TTFTMs:          100,
			TokensPerSecond: 10.0,
		},
	})

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/messages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetMessages(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp))
	}

	// Check assistant message has stats
	assistantMsg := resp[1]
	if assistantMsg["provider"] != "openai" {
		t.Errorf("expected provider 'openai', got '%v'", assistantMsg["provider"])
	}
	if assistantMsg["model"] != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%v'", assistantMsg["model"])
	}

	stats, ok := assistantMsg["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("expected stats to be present in response")
	}

	// JSON numbers are float64
	if stats["input_tokens"].(float64) != 10 {
		t.Errorf("expected input_tokens 10, got %v", stats["input_tokens"])
	}
	if stats["output_tokens"].(float64) != 5 {
		t.Errorf("expected output_tokens 5, got %v", stats["output_tokens"])
	}
	if stats["ttft_ms"].(float64) != 100 {
		t.Errorf("expected ttft_ms 100, got %v", stats["ttft_ms"])
	}
	if stats["tokens_per_second"].(float64) != 10.0 {
		t.Errorf("expected tokens_per_second 10.0, got %v", stats["tokens_per_second"])
	}
}

// Test token estimation functions
func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		minTokens int
		maxTokens int
	}{
		{
			name:      "empty string",
			text:      "",
			minTokens: 0,
			maxTokens: 0,
		},
		{
			name:      "short English text",
			text:      "Hello world",
			minTokens: 2,
			maxTokens: 4,
		},
		{
			name:      "longer English text",
			text:      "The quick brown fox jumps over the lazy dog",
			minTokens: 8,
			maxTokens: 15,
		},
		{
			name:      "Chinese text",
			text:      "你好世界",
			minTokens: 2,
			maxTokens: 4,
		},
		{
			name:      "mixed Chinese and English",
			text:      "Hello 你好 World 世界",
			minTokens: 4,
			maxTokens: 8,
		},
		{
			name:      "Japanese text",
			text:      "こんにちは世界",
			minTokens: 3,
			maxTokens: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.text)
			if result < tt.minTokens || result > tt.maxTokens {
				t.Errorf("estimateTokens(%q) = %d, want between %d and %d", tt.text, result, tt.minTokens, tt.maxTokens)
			}
		})
	}
}

// Test CJK character detection
func TestIsCJK(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'A', false},
		{'a', false},
		{'1', false},
		{' ', false},
		{'中', true},
		{'国', true},
		{'あ', true}, // Hiragana
		{'ア', true}, // Katakana
		{'한', true}, // Hangul
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := isCJK(tt.char)
			if result != tt.expected {
				t.Errorf("isCJK(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// Test input token estimation from messages
func TestEstimateInputTokens(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant."},
		{Role: llm.RoleUser, Content: "Hello!"},
		{Role: llm.RoleAssistant, Content: "Hi there! How can I help you today?"},
	}

	result := estimateInputTokens(messages)

	// Should be at least 12 tokens (4 per message overhead) + content tokens
	if result < 15 {
		t.Errorf("estimateInputTokens() = %d, expected at least 15", result)
	}

	// Should be reasonable (not too high)
	if result > 50 {
		t.Errorf("estimateInputTokens() = %d, expected at most 50", result)
	}
}
