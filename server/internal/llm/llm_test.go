package llm

import (
	"context"
	"errors"
	"testing"
)

// Test Provider interface definition
func TestProviderInterface(t *testing.T) {
	// Verify that MockProvider implements Provider interface
	var _ Provider = (*MockProvider)(nil)
}

// Test Message struct
func TestMessage(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "Hello, world!",
	}

	if msg.Role != RoleUser {
		t.Errorf("expected role %s, got %s", RoleUser, msg.Role)
	}
	if msg.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got '%s'", msg.Content)
	}
}

// Test ChatRequest struct
func TestChatRequest(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are a helpful assistant."},
			{Role: RoleUser, Content: "Hello!"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	if req.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%s'", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", req.Temperature)
	}
	if req.MaxTokens != 100 {
		t.Errorf("expected max_tokens 100, got %d", req.MaxTokens)
	}
}

// Test ChatResponse struct
func TestChatResponse(t *testing.T) {
	resp := ChatResponse{
		ID:      "chatcmpl-123",
		Model:   "gpt-4",
		Message: Message{Role: RoleAssistant, Content: "Hello! How can I help you?"},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected ID 'chatcmpl-123', got '%s'", resp.ID)
	}
	if resp.Message.Role != RoleAssistant {
		t.Errorf("expected role %s, got %s", RoleAssistant, resp.Message.Role)
	}
	if resp.Usage.TotalTokens != 18 {
		t.Errorf("expected total_tokens 18, got %d", resp.Usage.TotalTokens)
	}
}

// Test MockProvider Chat method
func TestMockProviderChat(t *testing.T) {
	provider := NewMockProvider()
	provider.SetResponse(ChatResponse{
		ID:      "mock-123",
		Model:   "mock-model",
		Message: Message{Role: RoleAssistant, Content: "Mock response"},
		Usage:   Usage{TotalTokens: 10},
	})

	req := ChatRequest{
		Model:    "mock-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "mock-123" {
		t.Errorf("expected ID 'mock-123', got '%s'", resp.ID)
	}
	if resp.Message.Content != "Mock response" {
		t.Errorf("expected content 'Mock response', got '%s'", resp.Message.Content)
	}
}

// Test MockProvider error handling
func TestMockProviderError(t *testing.T) {
	provider := NewMockProvider()
	expectedErr := errors.New("mock error")
	provider.SetError(expectedErr)

	req := ChatRequest{
		Model:    "mock-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// Test MockProvider context cancellation
func TestMockProviderContextCancellation(t *testing.T) {
	provider := NewMockProvider()
	provider.SetResponse(ChatResponse{
		ID:      "mock-123",
		Message: Message{Role: RoleAssistant, Content: "Response"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := ChatRequest{
		Model:    "mock-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
	}

	_, err := provider.Chat(ctx, req)
	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// Test Provider Name method
func TestMockProviderName(t *testing.T) {
	provider := NewMockProvider()
	if provider.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", provider.Name())
	}
}

// Test Provider Models method
func TestMockProviderModels(t *testing.T) {
	provider := NewMockProvider()
	models := provider.Models()
	if len(models) == 0 {
		t.Error("expected at least one model")
	}
	if models[0] != "mock-model" {
		t.Errorf("expected model 'mock-model', got '%s'", models[0])
	}
}

// Test ProviderRegistry
func TestProviderRegistry(t *testing.T) {
	registry := NewProviderRegistry()

	// Test empty registry
	if registry.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent provider")
	}

	// Register a provider
	mockProvider := NewMockProvider()
	registry.Register(mockProvider)

	// Get registered provider
	provider := registry.Get("mock")
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if provider.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", provider.Name())
	}

	// List providers
	providers := registry.List()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(providers))
	}
}

// Test ProviderRegistry duplicate registration
func TestProviderRegistryDuplicate(t *testing.T) {
	registry := NewProviderRegistry()

	mockProvider1 := NewMockProvider()
	mockProvider2 := NewMockProvider()

	registry.Register(mockProvider1)
	registry.Register(mockProvider2) // Should overwrite

	providers := registry.List()
	if len(providers) != 1 {
		t.Errorf("expected 1 provider after duplicate registration, got %d", len(providers))
	}
}

// Test Tool struct
func TestTool(t *testing.T) {
	tool := Tool{
		Name:        "calculator",
		Description: "Performs basic arithmetic",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "The arithmetic expression to evaluate",
				},
			},
			"required": []string{"expression"},
		},
	}

	if tool.Name != "calculator" {
		t.Errorf("expected name 'calculator', got '%s'", tool.Name)
	}
	if tool.Description != "Performs basic arithmetic" {
		t.Errorf("expected description 'Performs basic arithmetic', got '%s'", tool.Description)
	}
}

// Test ToolCall struct
func TestToolCall(t *testing.T) {
	toolCall := ToolCall{
		ID:        "call_123",
		Name:      "calculator",
		Arguments: `{"expression": "2 + 2"}`,
	}

	if toolCall.ID != "call_123" {
		t.Errorf("expected ID 'call_123', got '%s'", toolCall.ID)
	}
	if toolCall.Name != "calculator" {
		t.Errorf("expected name 'calculator', got '%s'", toolCall.Name)
	}
}

// Test Message with ToolCalls
func TestMessageWithToolCalls(t *testing.T) {
	msg := Message{
		Role:    RoleAssistant,
		Content: "",
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "calculator", Arguments: `{"expression": "2 + 2"}`},
		},
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Name != "calculator" {
		t.Errorf("expected tool name 'calculator', got '%s'", msg.ToolCalls[0].Name)
	}
}

// Test Message with ToolCallID (tool response)
func TestMessageWithToolCallID(t *testing.T) {
	msg := Message{
		Role:       RoleTool,
		Content:    "4",
		ToolCallID: "call_1",
	}

	if msg.Role != RoleTool {
		t.Errorf("expected role %s, got %s", RoleTool, msg.Role)
	}
	if msg.ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id 'call_1', got '%s'", msg.ToolCallID)
	}
}

// Test ChatRequest with Tools
func TestChatRequestWithTools(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleUser, Content: "What is 2 + 2?"},
		},
		Tools: []Tool{
			{Name: "calculator", Description: "Performs arithmetic"},
		},
	}

	if len(req.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(req.Tools))
	}
}
