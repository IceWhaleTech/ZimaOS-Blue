package proxy

import (
	"fmt"
	"sync"
	"time"
)

// ModelFeatures describes model capabilities
type ModelFeatures struct {
	Model            string `json:"model"`
	ToolCalling      bool   `json:"tool_calling"`
	Vision           bool   `json:"vision"`
	Streaming        bool   `json:"streaming"`
	SystemPrompt     bool   `json:"system_prompt"`
	MaxContextTokens int    `json:"max_context_tokens"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
}

// ModelCompatConfig model compatibility configuration
type ModelCompatConfig struct {
	AutoDetect       bool                      `json:"auto_detect"`
	DetectionCache   time.Duration             `json:"detection_cache"`
	ModelOverrides   map[string]*ModelFeatures `json:"model_overrides"`
	ToolCallFallback string                    `json:"tool_call_fallback"` // error, prompt, skip
}

// DefaultModelCompatConfig returns default model compatibility config
func DefaultModelCompatConfig() *ModelCompatConfig {
	return &ModelCompatConfig{
		AutoDetect:       true,
		DetectionCache:   1 * time.Hour,
		ModelOverrides:   make(map[string]*ModelFeatures),
		ToolCallFallback: "prompt",
	}
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// Tool represents a tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	System      string        `json:"system,omitempty"`
	Tools       []Tool        `json:"tools,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// HasImages checks if request contains images
func (r *ChatRequest) HasImages() bool {
	for _, msg := range r.Messages {
		if content, ok := msg.Content.([]interface{}); ok {
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if partMap["type"] == "image" || partMap["type"] == "image_url" {
						return true
					}
				}
			}
		}
	}
	return false
}

// TokenCount estimates token count (simplified)
func (r *ChatRequest) TokenCount() int {
	count := 0
	// Rough estimation: 4 chars per token
	for _, msg := range r.Messages {
		if str, ok := msg.Content.(string); ok {
			count += len(str) / 4
		}
	}
	count += len(r.System) / 4
	return count
}

// ModelCompatLayer handles model compatibility
type ModelCompatLayer struct {
	config        *ModelCompatConfig
	features      map[string]*ModelFeatures
	detectionTime map[string]time.Time
	mu            sync.RWMutex
}

// NewModelCompatLayer creates a new compatibility layer
func NewModelCompatLayer(config *ModelCompatConfig) *ModelCompatLayer {
	if config == nil {
		config = DefaultModelCompatConfig()
	}

	mcl := &ModelCompatLayer{
		config:        config,
		features:      make(map[string]*ModelFeatures),
		detectionTime: make(map[string]time.Time),
	}
	mcl.loadDefaultFeatures()
	return mcl
}

// loadDefaultFeatures loads known model features
func (mcl *ModelCompatLayer) loadDefaultFeatures() {
	defaults := map[string]*ModelFeatures{
		"claude-opus-4-5": {
			Model:            "claude-opus-4-5",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 200000,
			MaxOutputTokens:  32000,
		},
		"claude-sonnet-4-5": {
			Model:            "claude-sonnet-4-5",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 200000,
			MaxOutputTokens:  64000,
		},
		"claude-3-opus": {
			Model:            "claude-3-opus",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 200000,
			MaxOutputTokens:  4096,
		},
		"claude-3-sonnet": {
			Model:            "claude-3-sonnet",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 200000,
			MaxOutputTokens:  4096,
		},
		"claude-3-haiku": {
			Model:            "claude-3-haiku",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 200000,
			MaxOutputTokens:  4096,
		},
		"gpt-4o": {
			Model:            "gpt-4o",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 128000,
			MaxOutputTokens:  16384,
		},
		"gpt-4-turbo": {
			Model:            "gpt-4-turbo",
			ToolCalling:      true,
			Vision:           true,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 128000,
			MaxOutputTokens:  4096,
		},
		"gpt-3.5-turbo": {
			Model:            "gpt-3.5-turbo",
			ToolCalling:      true,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 16385,
			MaxOutputTokens:  4096,
		},
		"llama3": {
			Model:            "llama3",
			ToolCalling:      false,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 8192,
			MaxOutputTokens:  4096,
		},
		"llama3.1": {
			Model:            "llama3.1",
			ToolCalling:      true,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 128000,
			MaxOutputTokens:  4096,
		},
		"deepseek-chat": {
			Model:            "deepseek-chat",
			ToolCalling:      true,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 64000,
			MaxOutputTokens:  8192,
		},
		"deepseek-coder": {
			Model:            "deepseek-coder",
			ToolCalling:      true,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 64000,
			MaxOutputTokens:  8192,
		},
		"qwen2": {
			Model:            "qwen2",
			ToolCalling:      true,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 32768,
			MaxOutputTokens:  4096,
		},
		"mistral": {
			Model:            "mistral",
			ToolCalling:      true,
			Vision:           false,
			Streaming:        true,
			SystemPrompt:     true,
			MaxContextTokens: 32768,
			MaxOutputTokens:  4096,
		},
	}

	mcl.mu.Lock()
	defer mcl.mu.Unlock()

	for model, features := range defaults {
		mcl.features[model] = features
	}

	// Apply overrides
	for model, override := range mcl.config.ModelOverrides {
		mcl.features[model] = override
	}
}

// GetFeatures returns features for a model
func (mcl *ModelCompatLayer) GetFeatures(model string) *ModelFeatures {
	mcl.mu.RLock()
	defer mcl.mu.RUnlock()

	if features, ok := mcl.features[model]; ok {
		return features
	}

	// Return default features for unknown models
	return &ModelFeatures{
		Model:            model,
		ToolCalling:      false,
		Vision:           false,
		Streaming:        true,
		SystemPrompt:     true,
		MaxContextTokens: 8192,
		MaxOutputTokens:  4096,
	}
}

// SetFeatures sets features for a model
func (mcl *ModelCompatLayer) SetFeatures(model string, features *ModelFeatures) {
	mcl.mu.Lock()
	defer mcl.mu.Unlock()

	mcl.features[model] = features
}

// ListModels returns all known models
func (mcl *ModelCompatLayer) ListModels() []*ModelFeatures {
	mcl.mu.RLock()
	defer mcl.mu.RUnlock()

	models := make([]*ModelFeatures, 0, len(mcl.features))
	for _, f := range mcl.features {
		copy := *f
		models = append(models, &copy)
	}
	return models
}

// AdaptRequest adapts request for model compatibility
func (mcl *ModelCompatLayer) AdaptRequest(model string, req *ChatRequest) (*ChatRequest, error) {
	features := mcl.GetFeatures(model)

	// Handle tool calling
	if len(req.Tools) > 0 && !features.ToolCalling {
		switch mcl.config.ToolCallFallback {
		case "error":
			return nil, ErrToolCallingNotSupported
		case "skip":
			req.Tools = nil
		case "prompt":
			req = mcl.convertToolsToPrompt(req)
		}
	}

	// Handle vision
	if req.HasImages() && !features.Vision {
		return nil, ErrVisionNotSupported
	}

	// Truncate context if needed
	if req.TokenCount() > features.MaxContextTokens {
		req = mcl.truncateContext(req, features.MaxContextTokens)
	}

	return req, nil
}

// convertToolsToPrompt converts tool definitions to prompt
func (mcl *ModelCompatLayer) convertToolsToPrompt(req *ChatRequest) *ChatRequest {
	toolsPrompt := "You have access to the following tools:\n\n"
	for _, tool := range req.Tools {
		toolsPrompt += fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description)
		if tool.Parameters != nil {
			toolsPrompt += fmt.Sprintf("  Parameters: %v\n\n", tool.Parameters)
		}
	}
	toolsPrompt += "\nTo use a tool, respond with JSON: {\"tool\": \"name\", \"args\": {...}}\n"

	// Prepend to system message
	if req.System != "" {
		req.System = toolsPrompt + "\n" + req.System
	} else {
		req.System = toolsPrompt
	}

	req.Tools = nil
	return req
}

// truncateContext truncates context to fit model limits
func (mcl *ModelCompatLayer) truncateContext(req *ChatRequest, maxTokens int) *ChatRequest {
	// Simple truncation: remove oldest messages
	for req.TokenCount() > maxTokens && len(req.Messages) > 1 {
		req.Messages = req.Messages[1:]
	}
	return req
}

// Stats returns compatibility layer statistics
func (mcl *ModelCompatLayer) Stats() map[string]interface{} {
	mcl.mu.RLock()
	defer mcl.mu.RUnlock()

	return map[string]interface{}{
		"auto_detect":        mcl.config.AutoDetect,
		"tool_call_fallback": mcl.config.ToolCallFallback,
		"known_models":       len(mcl.features),
		"override_count":     len(mcl.config.ModelOverrides),
	}
}
