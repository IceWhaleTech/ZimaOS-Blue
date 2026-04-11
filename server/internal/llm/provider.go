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

// ContentPart represents a part of a multimodal message content.
type ContentPart struct {
	Type      string `json:"type"`                 // "text" or "image"
	Text      string `json:"text,omitempty"`       // for text type
	MediaType string `json:"media_type,omitempty"` // for image type (e.g., "image/jpeg")
	Data      string `json:"data,omitempty"`       // base64 encoded data for image type
}

// Message represents a chat message.
type Message struct {
	Role         Role          `json:"role"`
	Content      string        `json:"content"`
	ContentParts []ContentPart `json:"content_parts,omitempty"` // for multimodal messages
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string        `json:"tool_call_id,omitempty"`
	ToolName     string        `json:"tool_name,omitempty"` // optional explicit tool name for tool-result messages
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
	// Responses API fields (used by native /v1/responses path).
	PreviousResponseID string `json:"previous_response_id,omitempty"`
	Instructions       string `json:"instructions,omitempty"`
	PromptCacheKey     string `json:"prompt_cache_key,omitempty"`
	Store              *bool  `json:"store,omitempty"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	TotalTokens              int `json:"total_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID         string  `json:"id"`
	Model      string  `json:"model"`
	Provider   string  `json:"provider,omitempty"`
	ProviderID string  `json:"provider_id,omitempty"` // internal ID for sticky routing
	Message    Message `json:"message"`
	Usage      Usage   `json:"usage"`
}

// StreamChunk represents a chunk of a streaming response.
type StreamChunk struct {
	ID         string     `json:"id"`
	Model      string     `json:"model"`
	Provider   string     `json:"provider,omitempty"`
	ProviderID string     `json:"provider_id,omitempty"` // internal ID for sticky routing
	Delta      string     `json:"delta"`
	Done       bool       `json:"done"`
	Progress   string     `json:"progress,omitempty"` // upstream progress/metadata signal (no user-visible delta)
	Usage      *Usage     `json:"usage,omitempty"`
	Error      string     `json:"error,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
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

// List returns all registered provider names in a localized order.
// The order is optimized for different regions:
// - zh-CN: Chinese providers first (Qwen, DeepSeek, GLM, Kimi, MiniMax)
// - Default: International providers first (Anthropic, OpenAI, Gemini)
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		// Skip codex provider (deprecated/removed)
		if name == "codex" {
			continue
		}
		names = append(names, name)
	}

	// Default order (international)
	defaultOrder := []string{
		"claude", "openai", "gemini", "nvidia", "grok",
		"qwen", "deepseek", "glm", "kimi", "minimax",
		"openrouter", "azure", "bedrock", "siliconflow", "venice", "ollama", "aihubmix",
	}

	// Sort by predefined order
	sortByOrder(names, defaultOrder)
	return names
}

// ListForLocale returns provider names ordered for a specific locale.
func (r *ProviderRegistry) ListForLocale(locale string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		// Skip codex provider (deprecated/removed)
		if name == "codex" {
			continue
		}
		names = append(names, name)
	}

	var order []string

	// Chinese (Simplified) - prioritize Chinese providers
	if locale == "zh-CN" || locale == "zh_CN" {
		order = []string{
			"qwen", "deepseek", "glm", "kimi", "minimax",
			"nvidia", "claude", "openai", "gemini", "grok",
			"openrouter", "azure", "bedrock", "siliconflow", "venice", "ollama", "aihubmix",
		}
	} else {
		// Default order (international)
		order = []string{
			"claude", "openai", "gemini", "nvidia", "grok",
			"qwen", "deepseek", "glm", "kimi", "minimax",
			"openrouter", "azure", "bedrock", "siliconflow", "venice", "ollama", "aihubmix",
		}
	}

	sortByOrder(names, order)
	return names
}

// sortByOrder sorts names according to the predefined order.
// Names not in the order list are appended at the end alphabetically.
func sortByOrder(names []string, order []string) {
	// Create a map for O(1) lookup of order indices
	orderMap := make(map[string]int, len(order))
	for i, name := range order {
		orderMap[name] = i
	}

	// Sort using the order map
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			iOrder, iExists := orderMap[names[i]]
			jOrder, jExists := orderMap[names[j]]

			// Both exist in order map - compare by order
			if iExists && jExists {
				if iOrder > jOrder {
					names[i], names[j] = names[j], names[i]
				}
			} else if !iExists && jExists {
				// j exists in order, i doesn't - j should come first
				names[i], names[j] = names[j], names[i]
			} else if !iExists && !jExists {
				// Neither exists - sort alphabetically
				if names[i] > names[j] {
					names[i], names[j] = names[j], names[i]
				}
			}
			// If iExists && !jExists, keep current order (i before j)
		}
	}
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

// ChatStreamCallback sends a streaming chat completion request and calls the callback for each chunk.
func (m *MockProvider) ChatStreamCallback(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if m.err != nil {
		return m.err
	}

	content := "Mock response"
	if m.response != nil {
		content = m.response.Message.Content
	}

	// Send content chunk
	if err := callback(StreamChunk{
		ID:    "mock-stream",
		Model: req.Model,
		Delta: content,
		Done:  false,
	}); err != nil {
		return err
	}

	// Send final done chunk
	return callback(StreamChunk{
		ID:    "mock-stream",
		Model: req.Model,
		Done:  true,
		Usage: &Usage{TotalTokens: 10},
	})
}
