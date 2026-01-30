package providerpool

import "time"

// BuiltinProviders returns the list of built-in provider configurations
func BuiltinProviders() []*Provider {
	return []*Provider{
		{
			ID:          "openai",
			Name:        "OpenAI",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.openai.com/v1",
			APIVersion:  "v1",
			Priority:    50,
			Icon:        "openai",
			Description: "OpenAI API - GPT-4, GPT-4o, o1, and more",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "anthropic",
			Name:        "Anthropic",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.anthropic.com",
			APIVersion:  "2024-01-01",
			Priority:    50,
			Icon:        "anthropic",
			Description: "Anthropic API - Claude Opus, Sonnet, Haiku",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "google",
			Name:        "Google Gemini",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://generativelanguage.googleapis.com",
			APIVersion:  "v1beta",
			Priority:    40,
			Icon:        "google",
			Description: "Google Gemini API - Gemini Pro, Ultra",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "deepseek",
			Name:        "DeepSeek",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.deepseek.com/v1",
			APIVersion:  "v1",
			Priority:    30,
			Icon:        "deepseek",
			Description: "DeepSeek API - DeepSeek Chat, Coder",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "moonshot",
			Name:        "Moonshot",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.moonshot.cn/v1",
			APIVersion:  "v1",
			Priority:    30,
			Icon:        "moonshot",
			Description: "Moonshot AI (Kimi) API",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "azure-openai",
			Name:        "Azure OpenAI",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "", // User must configure
			APIVersion:  "2024-02-01",
			Priority:    45,
			Icon:        "azure",
			Description: "Azure OpenAI Service",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "openrouter",
			Name:        "OpenRouter",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://openrouter.ai/api/v1",
			APIVersion:  "v1",
			Priority:    35,
			Icon:        "openrouter",
			Description: "OpenRouter - Access multiple models through one API",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "aihubmix",
			Name:        "AiHubMix",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://aihubmix.com/v1",
			APIVersion:  "v1",
			Priority:    35,
			Icon:        "aihubmix",
			Description: "AiHubMix - AI model aggregator",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "ollama",
			Name:        "Ollama",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "http://localhost:11434",
			APIVersion:  "v1",
			Priority:    20,
			Icon:        "ollama",
			Description: "Ollama - Run LLMs locally",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "minimax",
			Name:        "MiniMax",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.minimax.chat/v1",
			APIVersion:  "v1",
			Priority:    30,
			Icon:        "minimax",
			Description: "MiniMax API - abab series models",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "codex",
			Name:        "Codex",
			Type:        ProviderTypeBuiltin,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.codex.com/v1",
			APIVersion:  "v1",
			Priority:    30,
			Icon:        "codex",
			Description: "Codex API - Code generation models",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// BuiltinModels returns known models for built-in providers
// Prices are per 1M tokens in USD (as of Jan 2026)
func BuiltinModels() map[string][]*Model {
	return map[string][]*Model{
		"openai": {
			{
				ID:          "gpt-4o",
				ProviderID:  "openai",
				Name:        "gpt-4o",
				DisplayName: "GPT-4o",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     16384,
				InputPrice:    2.5,   // $2.50 per 1M input tokens
				OutputPrice:   10.0,  // $10.00 per 1M output tokens
			},
			{
				ID:          "gpt-4o-mini",
				ProviderID:  "openai",
				Name:        "gpt-4o-mini",
				DisplayName: "GPT-4o Mini",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     16384,
				InputPrice:    0.15,  // $0.15 per 1M input tokens
				OutputPrice:   0.6,   // $0.60 per 1M output tokens
			},
			{
				ID:          "o1",
				ProviderID:  "openai",
				Name:        "o1",
				DisplayName: "o1",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 200000,
				MaxOutput:     100000,
				InputPrice:    15.0,  // $15.00 per 1M input tokens
				OutputPrice:   60.0,  // $60.00 per 1M output tokens
			},
			{
				ID:          "o1-mini",
				ProviderID:  "openai",
				Name:        "o1-mini",
				DisplayName: "o1 Mini",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     65536,
				InputPrice:    3.0,   // $3.00 per 1M input tokens
				OutputPrice:   12.0,  // $12.00 per 1M output tokens
			},
			{
				ID:          "o3-mini",
				ProviderID:  "openai",
				Name:        "o3-mini",
				DisplayName: "o3 Mini",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 200000,
				MaxOutput:     100000,
				InputPrice:    1.1,   // $1.10 per 1M input tokens
				OutputPrice:   4.4,   // $4.40 per 1M output tokens
			},
			{
				ID:          "gpt-4-turbo",
				ProviderID:  "openai",
				Name:        "gpt-4-turbo",
				DisplayName: "GPT-4 Turbo",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     4096,
				InputPrice:    10.0,  // $10.00 per 1M input tokens
				OutputPrice:   30.0,  // $30.00 per 1M output tokens
			},
		},
		"anthropic": {
			{
				ID:          "claude-opus-4-5-20251101",
				ProviderID:  "anthropic",
				Name:        "claude-opus-4-5-20251101",
				DisplayName: "Claude Opus 4.5",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 200000,
				MaxOutput:     32000,
				InputPrice:    15.0,  // $15.00 per 1M input tokens
				OutputPrice:   75.0,  // $75.00 per 1M output tokens
				CachePrice:    1.5,   // $1.50 per 1M cache read tokens
			},
			{
				ID:          "claude-sonnet-4-5-20250929",
				ProviderID:  "anthropic",
				Name:        "claude-sonnet-4-5-20250929",
				DisplayName: "Claude Sonnet 4.5",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 200000,
				MaxOutput:     64000,
				InputPrice:    3.0,   // $3.00 per 1M input tokens
				OutputPrice:   15.0,  // $15.00 per 1M output tokens
				CachePrice:    0.3,   // $0.30 per 1M cache read tokens
			},
			{
				ID:          "claude-3-5-haiku-20241022",
				ProviderID:  "anthropic",
				Name:        "claude-3-5-haiku-20241022",
				DisplayName: "Claude 3.5 Haiku",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 200000,
				MaxOutput:     8192,
				InputPrice:    0.8,   // $0.80 per 1M input tokens
				OutputPrice:   4.0,   // $4.00 per 1M output tokens
				CachePrice:    0.08,  // $0.08 per 1M cache read tokens
			},
			{
				ID:          "claude-3-5-sonnet-20241022",
				ProviderID:  "anthropic",
				Name:        "claude-3-5-sonnet-20241022",
				DisplayName: "Claude 3.5 Sonnet",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 200000,
				MaxOutput:     8192,
				InputPrice:    3.0,   // $3.00 per 1M input tokens
				OutputPrice:   15.0,  // $15.00 per 1M output tokens
				CachePrice:    0.3,   // $0.30 per 1M cache read tokens
			},
		},
		"google": {
			{
				ID:          "gemini-2.0-flash",
				ProviderID:  "google",
				Name:        "gemini-2.0-flash",
				DisplayName: "Gemini 2.0 Flash",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     8192,
				InputPrice:    0.075,  // $0.075 per 1M input tokens
				OutputPrice:   0.3,    // $0.30 per 1M output tokens
			},
			{
				ID:          "gemini-2.0-flash-thinking",
				ProviderID:  "google",
				Name:        "gemini-2.0-flash-thinking",
				DisplayName: "Gemini 2.0 Flash Thinking",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     65536,
				InputPrice:    0.075,  // $0.075 per 1M input tokens
				OutputPrice:   0.3,    // $0.30 per 1M output tokens
			},
			{
				ID:          "gemini-1.5-pro",
				ProviderID:  "google",
				Name:        "gemini-1.5-pro",
				DisplayName: "Gemini 1.5 Pro",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 2000000,
				MaxOutput:     8192,
				InputPrice:    1.25,   // $1.25 per 1M input tokens
				OutputPrice:   5.0,    // $5.00 per 1M output tokens
			},
			{
				ID:          "gemini-1.5-flash",
				ProviderID:  "google",
				Name:        "gemini-1.5-flash",
				DisplayName: "Gemini 1.5 Flash",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     8192,
				InputPrice:    0.075,  // $0.075 per 1M input tokens
				OutputPrice:   0.3,    // $0.30 per 1M output tokens
			},
		},
		"deepseek": {
			{
				ID:          "deepseek-chat",
				ProviderID:  "deepseek",
				Name:        "deepseek-chat",
				DisplayName: "DeepSeek Chat (V3)",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 64000,
				MaxOutput:     8192,
				InputPrice:    0.27,   // $0.27 per 1M input tokens (cache miss)
				OutputPrice:   1.1,    // $1.10 per 1M output tokens
				CachePrice:    0.07,   // $0.07 per 1M cache hit tokens
			},
			{
				ID:          "deepseek-reasoner",
				ProviderID:  "deepseek",
				Name:        "deepseek-reasoner",
				DisplayName: "DeepSeek Reasoner (R1)",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 64000,
				MaxOutput:     8192,
				InputPrice:    0.55,   // $0.55 per 1M input tokens
				OutputPrice:   2.19,   // $2.19 per 1M output tokens
				CachePrice:    0.14,   // $0.14 per 1M cache hit tokens
			},
		},
		"moonshot": {
			{
				ID:          "moonshot-v1-8k",
				ProviderID:  "moonshot",
				Name:        "moonshot-v1-8k",
				DisplayName: "Moonshot V1 8K",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8000,
				MaxOutput:     4096,
				InputPrice:    1.7,    // ¥12/1M tokens ≈ $1.7
				OutputPrice:   1.7,
			},
			{
				ID:          "moonshot-v1-32k",
				ProviderID:  "moonshot",
				Name:        "moonshot-v1-32k",
				DisplayName: "Moonshot V1 32K",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 32000,
				MaxOutput:     4096,
				InputPrice:    3.4,    // ¥24/1M tokens ≈ $3.4
				OutputPrice:   3.4,
			},
			{
				ID:          "moonshot-v1-128k",
				ProviderID:  "moonshot",
				Name:        "moonshot-v1-128k",
				DisplayName: "Moonshot V1 128K",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     4096,
				InputPrice:    8.5,    // ¥60/1M tokens ≈ $8.5
				OutputPrice:   8.5,
			},
		},
		"ollama": {
			{
				ID:          "llama3.3",
				ProviderID:  "ollama",
				Name:        "llama3.3",
				DisplayName: "Llama 3.3 70B",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     4096,
				InputPrice:    0.0,    // Local, no cost
				OutputPrice:   0.0,
			},
			{
				ID:          "qwen2.5",
				ProviderID:  "ollama",
				Name:        "qwen2.5",
				DisplayName: "Qwen 2.5",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 32000,
				MaxOutput:     4096,
				InputPrice:    0.0,    // Local, no cost
				OutputPrice:   0.0,
			},
			{
				ID:          "deepseek-r1",
				ProviderID:  "ollama",
				Name:        "deepseek-r1",
				DisplayName: "DeepSeek R1 (Local)",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					Thinking:     true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 64000,
				MaxOutput:     8192,
				InputPrice:    0.0,    // Local, no cost
				OutputPrice:   0.0,
			},
		},
		"minimax": {
			{
				ID:          "abab6.5s-chat",
				ProviderID:  "minimax",
				Name:        "abab6.5s-chat",
				DisplayName: "abab6.5s Chat",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 245760,
				MaxOutput:     16384,
				InputPrice:    1.0,    // ¥1/1M tokens
				OutputPrice:   1.0,
			},
			{
				ID:          "abab6.5g-chat",
				ProviderID:  "minimax",
				Name:        "abab6.5g-chat",
				DisplayName: "abab6.5g Chat",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8192,
				MaxOutput:     4096,
				InputPrice:    0.5,
				OutputPrice:   0.5,
			},
			{
				ID:          "abab6.5t-chat",
				ProviderID:  "minimax",
				Name:        "abab6.5t-chat",
				DisplayName: "abab6.5t Chat",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8192,
				MaxOutput:     4096,
				InputPrice:    0.1,
				OutputPrice:   0.1,
			},
		},
		"codex": {
			{
				ID:          "codex-1",
				ProviderID:  "codex",
				Name:        "codex-1",
				DisplayName: "Codex-1",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     16384,
				InputPrice:    2.0,
				OutputPrice:   8.0,
			},
		},
	}
}

// GetBuiltinProvider returns a built-in provider by ID
func GetBuiltinProvider(id string) *Provider {
	for _, p := range BuiltinProviders() {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// GetBuiltinModels returns built-in models for a provider
func GetBuiltinModels(providerID string) []*Model {
	models := BuiltinModels()
	if m, ok := models[providerID]; ok {
		return m
	}
	return nil
}
