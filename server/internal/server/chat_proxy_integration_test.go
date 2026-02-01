package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
	"github.com/labstack/echo/v4"
)

// TestChatHandler_WithProxyClient tests that chat handler routes requests through proxy
func TestChatHandler_WithProxyClient(t *testing.T) {
	// Track if proxy was called
	proxyCalled := false
	var receivedModel string
	var receivedMessages []OpenAIChatMessage

	// Create mock proxy server
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled = true

		// Parse request
		var req OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedModel = req.Model
		receivedMessages = req.Messages

		// Return response
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					}{
						Role:    "assistant",
						Content: "Response from proxy",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer proxyServer.Close()

	// Create in-memory store
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	// Create mock LLM provider (should NOT be called when proxy is enabled)
	mockProvider := &mockLLMProvider{
		name:   "mock",
		models: []string{"test-model"},
	}

	// Create provider registry
	registry := llm.NewProviderRegistry()
	registry.Register(mockProvider)

	// Create tool registry
	toolRegistry := tools.NewRegistry()

	// Create chat handler
	handler := NewChatHandler(store, registry, toolRegistry)

	// Create proxy client pointing to mock server
	proxyClient := &ProxyClient{
		baseURL:    proxyServer.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	handler.SetProxyClient(proxyClient)

	// Verify proxy is enabled
	if !handler.useProxy {
		t.Fatal("expected useProxy to be true")
	}

	// Create a conversation
	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "Test Conversation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Create Echo context for SendMessage
	e := echo.New()
	reqBody := `{"message": "Hello from test", "model": "test-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	// Call SendMessage
	err = handler.SendMessage(c)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify proxy was called
	if !proxyCalled {
		t.Error("expected proxy to be called")
	}

	// Verify request was forwarded correctly
	if receivedModel != "test-model" {
		t.Errorf("expected model test-model, got %s", receivedModel)
	}

	// Verify messages were forwarded (should include user message)
	found := false
	for _, msg := range receivedMessages {
		if msg.Role == "user" && msg.Content == "Hello from test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected user message to be forwarded to proxy")
	}

	// Verify mock provider was NOT called
	if mockProvider.chatCalled {
		t.Error("expected mock provider NOT to be called when proxy is enabled")
	}

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Content != "Response from proxy" {
		t.Errorf("expected content 'Response from proxy', got '%s'", resp.Content)
	}
}

// TestChatHandler_WithoutProxyClient tests that chat handler uses direct provider when proxy is not set
func TestChatHandler_WithoutProxyClient(t *testing.T) {
	// Create in-memory store
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}

	// Create mock LLM provider
	mockProvider := &mockLLMProvider{
		name:   "mock",
		models: []string{"test-model"},
		chatResponse: &llm.ChatResponse{
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "Response from direct provider",
			},
			Usage: llm.Usage{
				PromptTokens:     5,
				CompletionTokens: 3,
				TotalTokens:      8,
			},
		},
	}

	// Create provider registry
	registry := llm.NewProviderRegistry()
	registry.Register(mockProvider)

	// Create tool registry
	toolRegistry := tools.NewRegistry()

	// Create chat handler WITHOUT proxy client
	handler := NewChatHandler(store, registry, toolRegistry)

	// Verify proxy is NOT enabled
	if handler.useProxy {
		t.Fatal("expected useProxy to be false")
	}

	// Create a conversation
	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "Test Conversation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Create Echo context
	e := echo.New()
	reqBody := `{"message": "Hello", "model": "test-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	// Call SendMessage
	err = handler.SendMessage(c)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify mock provider WAS called
	if !mockProvider.chatCalled {
		t.Error("expected mock provider to be called when proxy is not enabled")
	}

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Content != "Response from direct provider" {
		t.Errorf("expected content 'Response from direct provider', got '%s'", resp.Content)
	}
}

// mockLLMProvider is a mock LLM provider for testing
type mockLLMProvider struct {
	name         string
	models       []string
	chatCalled   bool
	chatResponse *llm.ChatResponse
}

func (m *mockLLMProvider) Name() string {
	return m.name
}

func (m *mockLLMProvider) Models() []string {
	return m.models
}

func (m *mockLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.chatCalled = true
	if m.chatResponse != nil {
		return m.chatResponse, nil
	}
	return &llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Mock response",
		},
	}, nil
}

func (m *mockLLMProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Delta: "Mock", Done: true}
	close(ch)
	return ch, nil
}

func (m *mockLLMProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	m.chatCalled = true
	return callback(llm.StreamChunk{Delta: "Mock response", Done: true})
}
