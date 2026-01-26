// Package context provides conversation context management for LLM interactions.
package context

import (
	"strings"
	"sync"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall represents a tool call made by the LLM.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message represents a chat message.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// EstimateTokens estimates the number of tokens in the message.
// This is a rough estimate based on word count.
func (m *Message) EstimateTokens() int {
	// Rough estimate: ~1.3 tokens per word for English text
	// Add overhead for role and formatting
	words := len(strings.Fields(m.Content))
	tokens := int(float64(words) * 1.3)
	tokens += 4 // Overhead for role, formatting
	return max(tokens, 1)
}

// TruncationStrategy defines how to truncate messages when context is full.
type TruncationStrategy int

const (
	// TruncateOldest removes oldest messages first (preserving system prompt).
	TruncateOldest TruncationStrategy = iota
	// TruncateSlidingWindow keeps a sliding window of recent messages.
	TruncateSlidingWindow
)

// ConversationContext manages the conversation history and context window.
type ConversationContext struct {
	mu           sync.RWMutex
	messages     []Message
	maxTokens    int
	strategy     TruncationStrategy
	systemPrompt *Message
}

// NewConversationContext creates a new conversation context with the given max tokens.
func NewConversationContext(maxTokens int) *ConversationContext {
	return &ConversationContext{
		messages:  make([]Message, 0),
		maxTokens: maxTokens,
		strategy:  TruncateOldest,
	}
}

// MaxTokens returns the maximum token limit.
func (c *ConversationContext) MaxTokens() int {
	return c.maxTokens
}

// SetTruncationStrategy sets the truncation strategy.
func (c *ConversationContext) SetTruncationStrategy(strategy TruncationStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.strategy = strategy
}

// SetSystemPrompt sets or replaces the system prompt.
func (c *ConversationContext) SetSystemPrompt(prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemPrompt = &Message{
		Role:    RoleSystem,
		Content: prompt,
	}
}

// AddMessage adds a message to the context and truncates if necessary.
func (c *ConversationContext) AddMessage(msg Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = append(c.messages, msg)
	c.truncateIfNeeded()
}

// Messages returns all messages including the system prompt.
func (c *ConversationContext) Messages() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Message, 0, len(c.messages)+1)
	if c.systemPrompt != nil {
		result = append(result, *c.systemPrompt)
	}
	result = append(result, c.messages...)
	return result
}

// TotalTokens returns the estimated total tokens in the context.
func (c *ConversationContext) TotalTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	if c.systemPrompt != nil {
		total += c.systemPrompt.EstimateTokens()
	}
	for _, msg := range c.messages {
		total += msg.EstimateTokens()
	}
	return total
}

// Clear removes all messages including the system prompt.
func (c *ConversationContext) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = make([]Message, 0)
	c.systemPrompt = nil
}

// ClearKeepSystem removes all messages but keeps the system prompt.
func (c *ConversationContext) ClearKeepSystem() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages = make([]Message, 0)
}

// LastMessage returns the last message or nil if empty.
func (c *ConversationContext) LastMessage() *Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.messages) == 0 {
		return nil
	}
	return &c.messages[len(c.messages)-1]
}

// MessageCount returns the number of messages (excluding system prompt).
func (c *ConversationContext) MessageCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}

// truncateIfNeeded removes old messages if the context exceeds the token limit.
func (c *ConversationContext) truncateIfNeeded() {
	for c.totalTokensUnsafe() > c.maxTokens && len(c.messages) > 0 {
		switch c.strategy {
		case TruncateOldest:
			// Remove oldest message
			c.messages = c.messages[1:]
		case TruncateSlidingWindow:
			// Remove oldest pair of messages (user + assistant)
			if len(c.messages) >= 2 {
				c.messages = c.messages[2:]
			} else {
				c.messages = c.messages[1:]
			}
		}
	}
}

// totalTokensUnsafe returns total tokens without locking (for internal use).
func (c *ConversationContext) totalTokensUnsafe() int {
	total := 0
	if c.systemPrompt != nil {
		total += c.systemPrompt.EstimateTokens()
	}
	for _, msg := range c.messages {
		total += msg.EstimateTokens()
	}
	return total
}

// TokenCounter provides token counting functionality.
type TokenCounter struct{}

// NewTokenCounter creates a new token counter.
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{}
}

// Count estimates the number of tokens in a text string.
func (tc *TokenCounter) Count(text string) int {
	if text == "" {
		return 0
	}
	// Rough estimate: ~1.3 tokens per word for English text
	words := len(strings.Fields(text))
	tokens := int(float64(words) * 1.3)
	return max(tokens, 1)
}

// CountMessages estimates the total tokens in a slice of messages.
func (tc *TokenCounter) CountMessages(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += msg.EstimateTokens()
	}
	return total
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
