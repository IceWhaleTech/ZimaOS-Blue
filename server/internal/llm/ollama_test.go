package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test Ollama provider creation
func TestNewOllamaProvider(t *testing.T) {
	provider := NewOllamaProvider("")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if provider.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got '%s'", provider.Name())
	}
}

// Test Ollama provider with custom base URL
func TestNewOllamaProviderCustomBaseURL(t *testing.T) {
	provider := NewOllamaProvider("http://custom:11434")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
}

// Test Ollama provider models (static list)
func TestOllamaProviderModels(t *testing.T) {
	provider := NewOllamaProvider("")
	models := provider.Models()
	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// Check for common Ollama models
	hasLlama := false
	for _, m := range models {
		if m == "llama3.2" || m == "llama3.1" || m == "mistral" {
			hasLlama = true
			break
		}
	}
	if !hasLlama {
		t.Error("expected common Ollama model in list")
	}
}

// Test Ollama provider chat with mock server
func TestOllamaProviderChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}

		// Verify request body
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["stream"] != false {
			t.Error("expected stream to be false for non-streaming request")
		}

		// Return mock response
		resp := map[string]interface{}{
			"model":      "llama3.2",
			"created_at": "2024-01-01T12:00:00Z",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Hello! How can I help you?",
			},
			"done": true,
			"total_duration":      1000000000,
			"prompt_eval_count":   10,
			"eval_count":          8,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)
	req := ChatRequest{
		Model: "llama3.2",
		Messages: []Message{
			{Role: RoleUser, Content: "Hello!"},
		},
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Model != "llama3.2" {
		t.Errorf("expected model 'llama3.2', got '%s'", resp.Model)
	}
	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("expected content 'Hello! How can I help you?', got '%s'", resp.Message.Content)
	}
}

// Test Ollama provider chat with system message
func TestOllamaProviderChatWithSystem(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)

		resp := map[string]interface{}{
			"model": "llama3.2",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "I am a helpful assistant.",
			},
			"done": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)
	req := ChatRequest{
		Model: "llama3.2",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a helpful assistant."},
			{Role: RoleUser, Content: "Who are you?"},
		},
	}

	_, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify messages include system message
	messages, ok := receivedBody["messages"].([]interface{})
	if !ok {
		t.Fatal("expected messages array")
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

// Test Ollama provider chat with tool calls
func TestOllamaProviderChatWithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"model": "llama3.2",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]interface{}{
					{
						"function": map[string]interface{}{
							"name": "calculator",
							"arguments": map[string]interface{}{
								"expression": "2 + 2",
							},
						},
					},
				},
			},
			"done": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)
	req := ChatRequest{
		Model: "llama3.2",
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

// Test Ollama provider error handling
func TestOllamaProviderChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		resp := map[string]interface{}{
			"error": "model not found",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)
	req := ChatRequest{
		Model:    "nonexistent-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Test Ollama provider context cancellation
func TestOllamaProviderContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := ChatRequest{
		Model:    "llama3.2",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(ctx, req)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

// Test Ollama provider with temperature and options
func TestOllamaProviderChatWithOptions(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)

		resp := map[string]interface{}{
			"model": "llama3.2",
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Response",
			},
			"done": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL)
	req := ChatRequest{
		Model:       "llama3.2",
		Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	_, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify options were passed
	options, ok := receivedBody["options"].(map[string]interface{})
	if !ok {
		t.Fatal("expected options in request")
	}
	if options["temperature"] != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", options["temperature"])
	}
}
