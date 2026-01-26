package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test Claude provider creation
func TestNewClaudeProvider(t *testing.T) {
	provider := NewClaudeProvider("test-api-key", "")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if provider.Name() != "claude" {
		t.Errorf("expected name 'claude', got '%s'", provider.Name())
	}
}

// Test Claude provider with custom base URL
func TestNewClaudeProviderCustomBaseURL(t *testing.T) {
	provider := NewClaudeProvider("test-api-key", "https://custom.api.com")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
}

// Test Claude provider models
func TestClaudeProviderModels(t *testing.T) {
	provider := NewClaudeProvider("test-api-key", "")
	models := provider.Models()
	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// Check for Claude models
	hasClaude := false
	for _, m := range models {
		if m == "claude-3-5-sonnet-20241022" || m == "claude-3-opus-20240229" {
			hasClaude = true
			break
		}
	}
	if !hasClaude {
		t.Error("expected Claude model in list")
	}
}

// Test Claude provider chat with mock server
func TestClaudeProviderChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key test-api-key, got %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("expected anthropic-version header")
		}

		// Return mock response
		resp := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Hello! How can I help you?",
				},
			},
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewClaudeProvider("test-api-key", server.URL)
	req := ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello!"},
		},
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "msg_123" {
		t.Errorf("expected ID 'msg_123', got '%s'", resp.ID)
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("expected content 'Hello! How can I help you?', got '%s'", resp.Message.Content)
	}
}

// Test Claude provider chat with system message
func TestClaudeProviderChatWithSystem(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)

		resp := map[string]interface{}{
			"id":    "msg_456",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": []map[string]interface{}{
				{"type": "text", "text": "I am a helpful assistant."},
			},
			"usage": map[string]interface{}{
				"input_tokens":  15,
				"output_tokens": 10,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewClaudeProvider("test-api-key", server.URL)
	req := ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a helpful assistant."},
			{Role: RoleUser, Content: "Who are you?"},
		},
	}

	_, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify system message was extracted
	if receivedBody["system"] != "You are a helpful assistant." {
		t.Errorf("expected system message to be extracted, got %v", receivedBody["system"])
	}
}

// Test Claude provider chat with tool calls
func TestClaudeProviderChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":    "msg_789",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": []map[string]interface{}{
				{
					"type": "tool_use",
					"id":   "toolu_123",
					"name": "calculator",
					"input": map[string]interface{}{
						"expression": "2 + 2",
					},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]interface{}{
				"input_tokens":  20,
				"output_tokens": 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewClaudeProvider("test-api-key", server.URL)
	req := ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
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

// Test Claude provider error handling
func TestClaudeProviderChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewClaudeProvider("invalid-key", server.URL)
	req := ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Test Claude provider context cancellation
func TestClaudeProviderContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := NewClaudeProvider("test-api-key", server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := ChatRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(ctx, req)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}
