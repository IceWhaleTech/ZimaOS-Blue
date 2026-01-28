// Package llm provides interfaces and implementations for LLM providers.
package llm

import (
	"context"
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

// Message represents a chat message.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents a function/tool that can be called by the LLM.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool call made by the LLM.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID      string  `json:"id"`
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Usage   Usage   `json:"usage"`
}

// StreamChunk represents a chunk of a streaming response.
type StreamChunk struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
	Usage *Usage `json:"usage,omitempty"`
	Error string `json:"error,omitempty"`
}

// StreamCallback is a function that receives stream chunks.
// Return an error to stop the stream.
type StreamCallback func(chunk StreamChunk) error

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Models returns the list of available models.
	Models() []string

	// Chat sends a chat completion request and returns the response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ChatStream sends a chat completion request and returns a channel of stream chunks.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)

	// ChatStreamCallback sends a chat completion request and calls the callback for each chunk.
	// This is preferred over ChatStream as it avoids channel-related issues.
	ChatStreamCallback(ctx context.Context, req ChatRequest, callback StreamCallback) error
}

// ModelRefresher is an optional interface for providers that support refreshing models.
type ModelRefresher interface {
	// RefreshModels clears cached models and fetches fresh list from the API.
	RefreshModels() []string
}

// ProviderRegistry manages registered LLM providers.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Get retrieves a provider by name.
func (r *ProviderRegistry) Get(name string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
}

// List returns all registered provider names.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Update replaces an existing provider with a new one.
// This is useful for updating provider credentials at runtime.
func (r *ProviderRegistry) Update(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Remove removes a provider from the registry.
func (r *ProviderRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// MockProvider is a mock implementation of Provider for testing.
type MockProvider struct {
	response *ChatResponse
	err      error
}

// NewMockProvider creates a new mock provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// SetResponse sets the response to return from Chat.
func (m *MockProvider) SetResponse(resp ChatResponse) {
	m.response = &resp
}

// SetError sets the error to return from Chat.
func (m *MockProvider) SetError(err error) {
	m.err = err
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return "mock"
}

// Models returns the list of available models.
func (m *MockProvider) Models() []string {
	return []string{"mock-model"}
}

// Chat sends a chat completion request.
func (m *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if m.err != nil {
		return nil, m.err
	}

	if m.response != nil {
		return m.response, nil
	}

	return &ChatResponse{
		ID:      "mock-default",
		Model:   req.Model,
		Message: Message{Role: RoleAssistant, Content: "Mock response"},
		Usage:   Usage{TotalTokens: 10},
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (m *MockProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan StreamChunk, 1)
	go func() {
		defer close(ch)

		content := "Mock response"
		if m.response != nil {
			content = m.response.Message.Content
		}

		select {
		case <-ctx.Done():
			return
		case ch <- StreamChunk{
			ID:    "mock-stream",
			Model: req.Model,
			Delta: content,
			Done:  true,
			Usage: &Usage{TotalTokens: 10},
		}:
		}
	}()

	return ch, nil
}
