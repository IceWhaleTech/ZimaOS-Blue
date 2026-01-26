package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test OpenAI provider creation
func TestNewOpenAIProvider(t *testing.T) {
	provider := NewOpenAIProvider("test-api-key", "")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if provider.Name() != "openai" {
		t.Errorf("expected name 'openai', got '%s'", provider.Name())
	}
}

// Test OpenAI provider with custom base URL
func TestNewOpenAIProviderCustomBaseURL(t *testing.T) {
	provider := NewOpenAIProvider("test-api-key", "https://custom.api.com")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
}

// Test OpenAI provider models
func TestOpenAIProviderModels(t *testing.T) {
	provider := NewOpenAIProvider("test-api-key", "")
	models := provider.Models()
	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// Check for common models
	hasGPT4 := false
	for _, m := range models {
		if m == "gpt-4" || m == "gpt-4-turbo" || m == "gpt-4o" {
			hasGPT4 = true
			break
		}
	}
	if !hasGPT4 {
		t.Error("expected GPT-4 model in list")
	}
}

// Test OpenAI provider chat with mock server
func TestOpenAIProviderChat(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
		}

		// Return mock response
		resp := map[string]interface{}{
			"id":    "chatcmpl-123",
			"model": "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello! How can I help you?",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 8,
				"total_tokens":      18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-api-key", server.URL)
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello!"},
		},
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected ID 'chatcmpl-123', got '%s'", resp.ID)
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("expected content 'Hello! How can I help you?', got '%s'", resp.Message.Content)
	}
	if resp.Usage.TotalTokens != 18 {
		t.Errorf("expected total_tokens 18, got %d", resp.Usage.TotalTokens)
	}
}

// Test OpenAI provider chat with tool calls
func TestOpenAIProviderChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "chatcmpl-456",
			"model": "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "calculator",
									"arguments": `{"expression": "2 + 2"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     15,
				"completion_tokens": 10,
				"total_tokens":      25,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-api-key", server.URL)
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleUser, Content: "What is 2 + 2?"},
		},
		Tools: []Tool{
			{Name: "calculator", Description: "Performs arithmetic"},
		},
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Name != "calculator" {
		t.Errorf("expected tool name 'calculator', got '%s'", resp.Message.ToolCalls[0].Name)
	}
}

// Test OpenAI provider error handling
func TestOpenAIProviderChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider("invalid-key", server.URL)
	req := ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Test OpenAI provider context cancellation
func TestOpenAIProviderContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := NewOpenAIProvider("test-api-key", server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(ctx, req)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}
