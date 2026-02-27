package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

// StreamingMockProvider is a mock provider that properly implements streaming
type StreamingMockProvider struct {
	response    string
	streamError error
	lastReq     llm.ChatRequest
}

// autoContinueFailingProxyHandler simulates:
// 1) first stream round returns a TODO checklist (triggers auto-continue)
// 2) follow-up rounds fail before any chunks with upstream_error 502
type autoContinueFailingProxyHandler struct {
	callCount             int
	requestModels         []string
	requestPinnedProvider []string
}

func (h *autoContinueFailingProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++
	h.requestPinnedProvider = append(h.requestPinnedProvider, proxy.GetPinnedProvider(r.Context()))

	var body struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	h.requestModels = append(h.requestModels, body.Model)

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_ui_reviewer"
		rr.Model = "gpt-5.3-codex-spark"
	}

	// First round succeeds with TODO checklist content + stop.
	if h.callCount == 1 {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"- [ ] run command"},"finish_reason":"stop"}],"model":"auto"}`)
			f.Flush()
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"- [ ] run command"},"finish_reason":"stop"}],"model":"auto"}`)
		return
	}

	// Next round(s) fail before any SSE chunk.
	http.Error(w, `{"error":{"message":"Upstream request failed","type":"upstream_error"}}`, http.StatusBadGateway)
}

func NewStreamingMockProvider(response string) *StreamingMockProvider {
	return &StreamingMockProvider{response: response}
}

func (p *StreamingMockProvider) Name() string {
	return "streaming-mock"
}

func (p *StreamingMockProvider) Models() []string {
	return []string{"streaming-mock-model"}
}

func (p *StreamingMockProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.lastReq = req
	return &llm.ChatResponse{
		ID:      "mock-id",
		Model:   req.Model,
		Message: llm.Message{Role: llm.RoleAssistant, Content: p.response},
		Usage:   llm.Usage{TotalTokens: 10},
	}, nil
}

func (p *StreamingMockProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 10)
	go func() {
		defer close(ch)
		// Send content in chunks
		words := strings.Split(p.response, " ")
		for i, word := range words {
			if i > 0 {
				word = " " + word
			}
			select {
			case <-ctx.Done():
				return
			case ch <- llm.StreamChunk{Delta: word, Done: false}:
			}
		}
		// Send final chunk
		select {
		case <-ctx.Done():
			return
		case ch <- llm.StreamChunk{Done: true, Usage: &llm.Usage{TotalTokens: 10}}:
		}
	}()
	return ch, nil
}

func (p *StreamingMockProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	p.lastReq = req
	if p.streamError != nil {
		return p.streamError
	}

	// Send content in chunks
	words := strings.Split(p.response, " ")
	for i, word := range words {
		if i > 0 {
			word = " " + word
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := callback(llm.StreamChunk{Delta: word, Done: false}); err != nil {
			return err
		}
	}

	// Send final chunk with Done=true
	return callback(llm.StreamChunk{
		Done:  true,
		Usage: &llm.Usage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
	})
}

// TestStreamMessageBasic tests basic streaming functionality
func TestStreamMessageBasic(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Stream Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("Hello this is a streaming response")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "streaming-mock", "model": "streaming-mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	// Check response headers
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	// Parse SSE response
	body := rec.Body.String()
	t.Logf("Response body:\n%s", body)

	if body == "" {
		t.Error("response body is empty - this is the bug!")
	}

	// Check for SSE data lines
	if !strings.Contains(body, "data: ") {
		t.Error("response does not contain SSE data lines")
	}

	// Parse and verify chunks
	var fullContent string
	var gotDone bool
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}

			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				t.Logf("failed to parse chunk: %s, error: %v", data, err)
				continue
			}

			if delta, ok := chunk["delta"].(string); ok {
				fullContent += delta
			}
			if done, ok := chunk["done"].(bool); ok && done {
				gotDone = true
			}
		}
	}

	t.Logf("Full content received: %s", fullContent)
	t.Logf("Got done signal: %v", gotDone)

	if fullContent == "" {
		t.Error("no content received from stream")
	}

	if !gotDone {
		t.Error("did not receive done signal")
	}
}

func TestStreamMessageInjectsConversationAnchor(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Stream Anchor Title")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Initial stream goal: keep context"})
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "ack"})

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("stream response")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSystemPromptBuilder(claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{}))

	e := echo.New()
	reqBody := `{"message":"B","provider":"streaming-mock","model":"streaming-mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	if !hasSystemAnchor(mockProvider.lastReq.Messages, "Stream Anchor Title", "Initial stream goal: keep context") {
		t.Fatalf("expected conversation anchor in stream system messages, got %d messages", len(mockProvider.lastReq.Messages))
	}
}

// TestStreamMessageWithNoProvider tests streaming when no provider is available
func TestStreamMessageWithNoProvider(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Empty registry - no providers
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)

	// Should return error when no provider is available
	if err == nil {
		// Check if error is in response body (SSE sends 200 then error in data)
		body := rec.Body.String()
		t.Logf("Response when no provider: %s", body)
		if !strings.Contains(body, "error") {
			t.Error("expected error in response body when no provider available")
		}
	}
}

// TestMockProviderChatStreamCallback tests the mock provider's ChatStreamCallback directly
func TestMockProviderChatStreamCallback(t *testing.T) {
	provider := NewStreamingMockProvider("Hello world test")

	var chunks []llm.StreamChunk
	err := provider.ChatStreamCallback(context.Background(), llm.ChatRequest{
		Model: "test",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
	}, func(chunk llm.StreamChunk) error {
		chunks = append(chunks, chunk)
		t.Logf("Received chunk: delta=%q, done=%v", chunk.Delta, chunk.Done)
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStreamCallback error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("no chunks received")
	}

	// Check that we got a done chunk
	var gotDone bool
	var fullContent string
	for _, chunk := range chunks {
		fullContent += chunk.Delta
		if chunk.Done {
			gotDone = true
		}
	}

	if !gotDone {
		t.Error("did not receive done chunk")
	}

	t.Logf("Full content: %s", fullContent)
	if fullContent == "" {
		t.Error("no content in chunks")
	}
}

// TestStreamMessageEmptyModel tests streaming when model is empty
func TestStreamMessageEmptyModel(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("Response with empty model")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	// Request without model specified
	reqBody := `{"message": "Hello!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)

	body := rec.Body.String()
	t.Logf("Response with empty model: %s", body)
	t.Logf("Status code: %d", rec.Code)
	t.Logf("Error: %v", err)

	// Should still work - provider should use default model
	if body == "" && err == nil {
		t.Error("empty response body with no error - this is the bug!")
	}
}

// TestDefaultMockProviderChatStreamCallback tests the default llm.MockProvider
func TestDefaultMockProviderChatStreamCallback(t *testing.T) {
	provider := llm.NewMockProvider()
	provider.SetResponse(llm.ChatResponse{
		ID:      "test-id",
		Model:   "test-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Test response content"},
	})

	var chunks []llm.StreamChunk
	err := provider.ChatStreamCallback(context.Background(), llm.ChatRequest{
		Model: "test",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
	}, func(chunk llm.StreamChunk) error {
		chunks = append(chunks, chunk)
		t.Logf("Received chunk: delta=%q, done=%v", chunk.Delta, chunk.Done)
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStreamCallback error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("no chunks received from default MockProvider")
	}

	var fullContent string
	var gotDone bool
	for _, chunk := range chunks {
		fullContent += chunk.Delta
		if chunk.Done {
			gotDone = true
		}
	}

	t.Logf("Default MockProvider full content: %s", fullContent)
	t.Logf("Default MockProvider got done: %v", gotDone)

	if fullContent == "" {
		t.Error("default MockProvider returned empty content")
	}
}

func TestStreamMessageAutoContinue_PreContent502GracefulCompletion(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Auto Continue 502")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	// Enable agent mode path (`getMaxToolRounds() > maxToolRounds`) so TODO auto-continue can trigger.
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	fakeProxy := &autoContinueFailingProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"continue the task","model":"auto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected graceful completion without STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fakeProxy.callCount != 4 {
		t.Fatalf("expected exactly 4 proxy calls (initial + one continuation with 2 retries), got %d", fakeProxy.callCount)
	}
	if len(fakeProxy.requestModels) < 2 || len(fakeProxy.requestPinnedProvider) < 2 {
		t.Fatalf("expected captured request metadata for at least 2 calls, models=%d providers=%d", len(fakeProxy.requestModels), len(fakeProxy.requestPinnedProvider))
	}
	if got := fakeProxy.requestModels[1]; got != "gpt-5.3-codex-spark" {
		t.Fatalf("expected auto-continue to pin model from first round, got %q", got)
	}
	if got := fakeProxy.requestPinnedProvider[1]; got != "prov_ui_reviewer" {
		t.Fatalf("expected auto-continue to pin provider from first round, got %q", got)
	}
}
