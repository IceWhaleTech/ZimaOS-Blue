package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

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

	if len(resp) < 3 {
		t.Errorf("expected at least 3 tools, got %d", len(resp))
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
