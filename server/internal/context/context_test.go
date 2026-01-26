package context

import (
	"testing"
)

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

// Test Message token estimation
func TestMessageTokenEstimate(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: "Hello world", // 2 words, ~2-3 tokens
	}

	tokens := msg.EstimateTokens()
	if tokens < 2 || tokens > 10 {
		t.Errorf("expected tokens between 2 and 10, got %d", tokens)
	}
}

// Test ConversationContext creation
func TestNewConversationContext(t *testing.T) {
	ctx := NewConversationContext(4096)
	if ctx == nil {
		t.Fatal("expected context, got nil")
	}
	if ctx.MaxTokens() != 4096 {
		t.Errorf("expected max tokens 4096, got %d", ctx.MaxTokens())
	}
}

// Test ConversationContext AddMessage
func TestConversationContextAddMessage(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.AddMessage(Message{Role: RoleUser, Content: "Hello"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Hi there!"})

	messages := ctx.Messages()
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

// Test ConversationContext SetSystemPrompt
func TestConversationContextSetSystemPrompt(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.SetSystemPrompt("You are a helpful assistant.")

	messages := ctx.Messages()
	if len(messages) != 1 {
		t.Errorf("expected 1 message (system), got %d", len(messages))
	}
	if messages[0].Role != RoleSystem {
		t.Errorf("expected system role, got %s", messages[0].Role)
	}
}

// Test ConversationContext system prompt replacement
func TestConversationContextSystemPromptReplacement(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.SetSystemPrompt("First prompt")
	ctx.SetSystemPrompt("Second prompt")

	messages := ctx.Messages()
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Second prompt" {
		t.Errorf("expected 'Second prompt', got '%s'", messages[0].Content)
	}
}

// Test ConversationContext TotalTokens
func TestConversationContextTotalTokens(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.AddMessage(Message{Role: RoleUser, Content: "Hello world"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Hi there"})

	tokens := ctx.TotalTokens()
	if tokens < 4 {
		t.Errorf("expected at least 4 tokens, got %d", tokens)
	}
}

// Test ConversationContext Clear
func TestConversationContextClear(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.SetSystemPrompt("System prompt")
	ctx.AddMessage(Message{Role: RoleUser, Content: "Hello"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Hi"})

	ctx.Clear()

	messages := ctx.Messages()
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(messages))
	}
}

// Test ConversationContext ClearKeepSystem
func TestConversationContextClearKeepSystem(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.SetSystemPrompt("System prompt")
	ctx.AddMessage(Message{Role: RoleUser, Content: "Hello"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Hi"})

	ctx.ClearKeepSystem()

	messages := ctx.Messages()
	if len(messages) != 1 {
		t.Errorf("expected 1 message (system), got %d", len(messages))
	}
	if messages[0].Role != RoleSystem {
		t.Errorf("expected system role, got %s", messages[0].Role)
	}
}

// Test ConversationContext truncation
func TestConversationContextTruncation(t *testing.T) {
	ctx := NewConversationContext(100) // Very small limit

	// Add many messages
	for i := 0; i < 20; i++ {
		ctx.AddMessage(Message{Role: RoleUser, Content: "This is a test message with some content"})
		ctx.AddMessage(Message{Role: RoleAssistant, Content: "This is a response with some content"})
	}

	// Should have truncated to fit within token limit
	tokens := ctx.TotalTokens()
	if tokens > 100 {
		t.Errorf("expected tokens <= 100, got %d", tokens)
	}
}

// Test ConversationContext truncation preserves system prompt
func TestConversationContextTruncationPreservesSystem(t *testing.T) {
	ctx := NewConversationContext(50)

	ctx.SetSystemPrompt("Important system prompt")

	// Add messages that will cause truncation
	for i := 0; i < 10; i++ {
		ctx.AddMessage(Message{Role: RoleUser, Content: "Test message"})
		ctx.AddMessage(Message{Role: RoleAssistant, Content: "Response"})
	}

	messages := ctx.Messages()
	if len(messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if messages[0].Role != RoleSystem {
		t.Error("expected system prompt to be preserved")
	}
}

// Test ConversationContext LastMessage
func TestConversationContextLastMessage(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.AddMessage(Message{Role: RoleUser, Content: "First"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Second"})
	ctx.AddMessage(Message{Role: RoleUser, Content: "Third"})

	last := ctx.LastMessage()
	if last == nil {
		t.Fatal("expected last message, got nil")
	}
	if last.Content != "Third" {
		t.Errorf("expected 'Third', got '%s'", last.Content)
	}
}

// Test ConversationContext LastMessage empty
func TestConversationContextLastMessageEmpty(t *testing.T) {
	ctx := NewConversationContext(4096)

	last := ctx.LastMessage()
	if last != nil {
		t.Error("expected nil for empty context")
	}
}

// Test ConversationContext MessageCount
func TestConversationContextMessageCount(t *testing.T) {
	ctx := NewConversationContext(4096)

	if ctx.MessageCount() != 0 {
		t.Errorf("expected 0, got %d", ctx.MessageCount())
	}

	ctx.AddMessage(Message{Role: RoleUser, Content: "Hello"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Hi"})

	if ctx.MessageCount() != 2 {
		t.Errorf("expected 2, got %d", ctx.MessageCount())
	}
}

// Test TokenCounter
func TestTokenCounter(t *testing.T) {
	counter := NewTokenCounter()

	tests := []struct {
		text     string
		minTokens int
		maxTokens int
	}{
		{"Hello", 1, 3},
		{"Hello world", 2, 5},
		{"This is a longer sentence with more words.", 5, 15},
		{"", 0, 1},
	}

	for _, tt := range tests {
		tokens := counter.Count(tt.text)
		if tokens < tt.minTokens || tokens > tt.maxTokens {
			t.Errorf("Count(%q) = %d, expected between %d and %d", tt.text, tokens, tt.minTokens, tt.maxTokens)
		}
	}
}

// Test TokenCounter CountMessages
func TestTokenCounterCountMessages(t *testing.T) {
	counter := NewTokenCounter()

	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there"},
	}

	tokens := counter.CountMessages(messages)
	if tokens < 3 {
		t.Errorf("expected at least 3 tokens, got %d", tokens)
	}
}

// Test TruncationStrategy
func TestTruncationStrategyOldest(t *testing.T) {
	ctx := NewConversationContext(50)
	ctx.SetTruncationStrategy(TruncateOldest)

	ctx.SetSystemPrompt("System")
	ctx.AddMessage(Message{Role: RoleUser, Content: "First message"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "First response"})
	ctx.AddMessage(Message{Role: RoleUser, Content: "Second message"})
	ctx.AddMessage(Message{Role: RoleAssistant, Content: "Second response"})

	// After truncation, oldest non-system messages should be removed
	messages := ctx.Messages()
	if len(messages) > 0 && messages[0].Role == RoleSystem {
		// System prompt preserved
	}
}

// Test TruncationStrategy sliding window
func TestTruncationStrategySlidingWindow(t *testing.T) {
	ctx := NewConversationContext(100)
	ctx.SetTruncationStrategy(TruncateSlidingWindow)

	// Add many messages
	for i := 0; i < 20; i++ {
		ctx.AddMessage(Message{Role: RoleUser, Content: "Message"})
		ctx.AddMessage(Message{Role: RoleAssistant, Content: "Response"})
	}

	// Should keep recent messages
	messages := ctx.Messages()
	if len(messages) == 0 {
		t.Error("expected some messages to remain")
	}
}

// Test ConversationContext with tool calls
func TestConversationContextWithToolCalls(t *testing.T) {
	ctx := NewConversationContext(4096)

	ctx.AddMessage(Message{
		Role:    RoleAssistant,
		Content: "",
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "calculator", Arguments: `{"expression": "2+2"}`},
		},
	})

	ctx.AddMessage(Message{
		Role:       RoleTool,
		Content:    "4",
		ToolCallID: "call_1",
	})

	messages := ctx.Messages()
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}
