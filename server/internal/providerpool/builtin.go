package providerpool

import (
	"time"
)

// Build-time variables (injected via -ldflags)
// Example: go build -ldflags "-X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=..."
var (
	trialLicense string // Ed25519-signed license injected at build time
)

const (
	// DefaultTrialBaseURL is the default base URL for trial provider
	DefaultTrialBaseURL = "https://paid.tribiosapi.top/"
)

// BuiltinProviders returns the list of built-in provider configurations.
// The trial provider is only included if a trial API key was injected at build time.
func BuiltinProviders() []*Provider {
	providers := []*Provider{
		{
			ID:          "openai",
			Name:        "OpenAI",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.openai.com/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    50,
			Icon:        "openai",
			Description: "OpenAI API - GPT-4, GPT-4o, o1, and more",
			Website:     "https://openai.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "anthropic",
			Name:        "Anthropic",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.anthropic.com",
			APIVersion:  "2024-01-01",
			APIFormat:   APIFormatAnthropic,
			Priority:    50,
			Icon:        "anthropic",
			Description: "Anthropic API - Claude Opus, Sonnet, Haiku",
			Website:     "https://anthropic.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "google",
			Name:        "Google Gemini",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://generativelanguage.googleapis.com",
			APIVersion:  "v1beta",
			APIFormat:   APIFormatGoogle,
			Priority:    40,
			Icon:        "google",
			Description: "Google Gemini API - Gemini Pro, Ultra",
			Website:     "https://ai.google.dev",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "deepseek",
			Name:        "DeepSeek",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.deepseek.com/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    30,
			Icon:        "deepseek",
			Description: "DeepSeek API - DeepSeek Chat, Coder",
			Website:     "https://deepseek.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "moonshot",
			Name:        "Moonshot",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.moonshot.cn/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    30,
			Icon:        "moonshot",
			Description: "Moonshot AI (Kimi) API",
			Website:     "https://moonshot.cn",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "azure-openai",
			Name:        "Azure OpenAI",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "", // User must configure
			APIVersion:  "2024-02-01",
			APIFormat:   APIFormatOpenAI,
			Priority:    45,
			Icon:        "azure",
			Description: "Azure OpenAI Service",
			Website:     "https://azure.microsoft.com/products/ai-services/openai-service",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "openrouter",
			Name:        "OpenRouter",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://openrouter.ai/api/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "openrouter",
			Description: "OpenRouter - Access multiple models through one API",
			Website:     "https://openrouter.ai",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "aihubmix",
			Name:        "AiHubMix",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://aihubmix.com/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "aihubmix",
			Description: "AiHubMix - AI model aggregator",
			Website:     "https://aihubmix.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "ollama",
			Name:        "Ollama",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationLocal,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "http://localhost:11434",
			APIVersion:  "v1",
			APIFormat:   APIFormatOllama,
			Priority:    20,
			Icon:        "ollama",
			Description: "Ollama - Run LLMs locally",
			Website:     "https://ollama.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "minimax",
			Name:        "MiniMax",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.minimax.chat/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    30,
			Icon:        "minimax",
			Description: "MiniMax API - abab series models",
			Website:     "https://minimax.chat",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "codex",
			Name:        "Codex",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.codex.com/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    30,
			Icon:        "codex",
			Description: "Codex API - Code generation models",
			Website:     "https://codex.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "grok",
			Name:        "Grok (xAI)",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.x.ai/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    40,
			Icon:        "grok",
			Description: "xAI Grok API - Advanced AI models from xAI",
			Website:     "https://x.ai",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "qwen",
			Name:        "Qwen (Alibaba Cloud)",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "qwen",
			Description: "Alibaba Cloud Qwen API - Qwen series models with multilingual support",
			Website:     "https://tongyi.aliyun.com",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "venice",
			Name:        "Venice AI",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.venice.ai/api/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "venice",
			Description: "Venice AI - Privacy-focused AI with uncensored models",
			Website:     "https://venice.ai",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "bedrock",
			Name:        "Amazon Bedrock",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "", // User must configure with their AWS region endpoint
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    45,
			Icon:        "aws",
			Description: "Amazon Bedrock - AWS managed AI service with Claude, Llama, and more",
			Website:     "https://aws.amazon.com/bedrock",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "glm",
			Name:        "GLM (Zhipu AI)",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
			APIVersion:  "v4",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "glm",
			Description: "Zhipu AI GLM API - GLM-4 series models with Chinese language support",
			Website:     "https://open.bigmodel.cn",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	// Only include trial provider if a valid signed license was injected at build time
	if claims := getTrialClaims(); claims != nil {
		baseURL := claims.URL
		if baseURL == "" {
			baseURL = DefaultTrialBaseURL
		}
		apiFormat := APIFormatAnthropic
		if claims.Format != "" {
			apiFormat = APIFormat(claims.Format)
		}
		providers = append(providers, &Provider{
			ID:          TrialProviderID,
			Name:        "ZimaOS Blue Trial",
			Type:        ProviderTypeTrial,
			Location:    ProviderLocationCloud,
			Enabled:     true,
			Status:      ProviderStatusActive,
			BaseURL:     baseURL,
			APIVersion:  "v1",
			APIFormat:   apiFormat,
			Priority:    100,
			Icon:        "blue",
			Description: "Blue Trial Provider - Free trial with limited quota",
			Website:     "https://zimaos.com",
			APIKeys: []APIKey{{
				ID: "trial-key", Key: claims.Key, KeyHash: HashAPIKey(claims.Key),
				Label: "Trial API Key", Enabled: true, CreatedAt: time.Now(),
			}},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	return providers
}

// getTrialClaims verifies the build-time license and returns claims, or nil if invalid/absent/expired.
func getTrialClaims() *LicenseClaims {
	if trialLicense == "" {
		return nil
	}
	claims, _, err := VerifyLicense(trialLicense)
	if err != nil {
		// Expired or invalid → don't show trial provider
		return nil
	}
	return claims
}

// GetTrialLicense returns the raw build-time license string.
func GetTrialLicense() string {
	return trialLicense
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
				InputPrice:    2.5,  // $2.50 per 1M input tokens
				OutputPrice:   10.0, // $10.00 per 1M output tokens
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
				InputPrice:    0.15, // $0.15 per 1M input tokens
				OutputPrice:   0.6,  // $0.60 per 1M output tokens
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
				InputPrice:    15.0, // $15.00 per 1M input tokens
				OutputPrice:   60.0, // $60.00 per 1M output tokens
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
				InputPrice:    3.0,  // $3.00 per 1M input tokens
				OutputPrice:   12.0, // $12.00 per 1M output tokens
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
				InputPrice:    1.1, // $1.10 per 1M input tokens
				OutputPrice:   4.4, // $4.40 per 1M output tokens
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
				InputPrice:    10.0, // $10.00 per 1M input tokens
				OutputPrice:   30.0, // $30.00 per 1M output tokens
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
				InputPrice:    15.0, // $15.00 per 1M input tokens
				OutputPrice:   75.0, // $75.00 per 1M output tokens
				CachePrice:    1.5,  // $1.50 per 1M cache read tokens
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
				InputPrice:    3.0,  // $3.00 per 1M input tokens
				OutputPrice:   15.0, // $15.00 per 1M output tokens
				CachePrice:    0.3,  // $0.30 per 1M cache read tokens
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
				InputPrice:    0.8,  // $0.80 per 1M input tokens
				OutputPrice:   4.0,  // $4.00 per 1M output tokens
				CachePrice:    0.08, // $0.08 per 1M cache read tokens
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
				InputPrice:    3.0,  // $3.00 per 1M input tokens
				OutputPrice:   15.0, // $15.00 per 1M output tokens
				CachePrice:    0.3,  // $0.30 per 1M cache read tokens
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
				InputPrice:    0.075, // $0.075 per 1M input tokens
				OutputPrice:   0.3,   // $0.30 per 1M output tokens
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
				InputPrice:    0.075, // $0.075 per 1M input tokens
				OutputPrice:   0.3,   // $0.30 per 1M output tokens
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
				InputPrice:    1.25, // $1.25 per 1M input tokens
				OutputPrice:   5.0,  // $5.00 per 1M output tokens
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
				InputPrice:    0.075, // $0.075 per 1M input tokens
				OutputPrice:   0.3,   // $0.30 per 1M output tokens
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
				InputPrice:    0.27, // $0.27 per 1M input tokens (cache miss)
				OutputPrice:   1.1,  // $1.10 per 1M output tokens
				CachePrice:    0.07, // $0.07 per 1M cache hit tokens
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
				InputPrice:    0.55, // $0.55 per 1M input tokens
				OutputPrice:   2.19, // $2.19 per 1M output tokens
				CachePrice:    0.14, // $0.14 per 1M cache hit tokens
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
				InputPrice:    1.7, // ¥12/1M tokens ≈ $1.7
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
				InputPrice:    3.4, // ¥24/1M tokens ≈ $3.4
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
				InputPrice:    8.5, // ¥60/1M tokens ≈ $8.5
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
				InputPrice:    0.0, // Local, no cost
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
				InputPrice:    0.0, // Local, no cost
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
				InputPrice:    0.0, // Local, no cost
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
				InputPrice:    1.0, // ¥1/1M tokens
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
		"grok": {
			{
				ID:          "grok-beta",
				ProviderID:  "grok",
				Name:        "grok-beta",
				DisplayName: "Grok Beta",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 131072,
				MaxOutput:     16384,
				InputPrice:    5.0,  // $5.00 per 1M input tokens
				OutputPrice:   15.0, // $15.00 per 1M output tokens
			},
			{
				ID:          "grok-vision-beta",
				ProviderID:  "grok",
				Name:        "grok-vision-beta",
				DisplayName: "Grok Vision Beta",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8192,
				MaxOutput:     4096,
				InputPrice:    5.0,  // $5.00 per 1M input tokens
				OutputPrice:   15.0, // $15.00 per 1M output tokens
			},
		},
		"qwen": {
			{
				ID:          "qwen-turbo",
				ProviderID:  "qwen",
				Name:        "qwen-turbo",
				DisplayName: "Qwen Turbo",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8192,
				MaxOutput:     2048,
				InputPrice:    0.3, // $0.30 per 1M input tokens
				OutputPrice:   0.6, // $0.60 per 1M output tokens
			},
			{
				ID:          "qwen-plus",
				ProviderID:  "qwen",
				Name:        "qwen-plus",
				DisplayName: "Qwen Plus",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 32768,
				MaxOutput:     8192,
				InputPrice:    0.8, // $0.80 per 1M input tokens
				OutputPrice:   2.0, // $2.00 per 1M output tokens
			},
			{
				ID:          "qwen-max",
				ProviderID:  "qwen",
				Name:        "qwen-max",
				DisplayName: "Qwen Max",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 32768,
				MaxOutput:     8192,
				InputPrice:    4.0,  // $4.00 per 1M input tokens
				OutputPrice:   12.0, // $12.00 per 1M output tokens
			},
			{
				ID:          "qwen-max-longcontext",
				ProviderID:  "qwen",
				Name:        "qwen-max-longcontext",
				DisplayName: "Qwen Max (Long Context)",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     8192,
				InputPrice:    4.0,  // $4.00 per 1M input tokens
				OutputPrice:   12.0, // $12.00 per 1M output tokens
			},
			{
				ID:          "qwen-vl-plus",
				ProviderID:  "qwen",
				Name:        "qwen-vl-plus",
				DisplayName: "Qwen VL Plus",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8192,
				MaxOutput:     2048,
				InputPrice:    0.8, // $0.80 per 1M input tokens
				OutputPrice:   2.0, // $2.00 per 1M output tokens
			},
			{
				ID:          "qwen-vl-max",
				ProviderID:  "qwen",
				Name:        "qwen-vl-max",
				DisplayName: "Qwen VL Max",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 32768,
				MaxOutput:     8192,
				InputPrice:    4.0,  // $4.00 per 1M input tokens
				OutputPrice:   12.0, // $12.00 per 1M output tokens
			},
		},
		"venice": {
			{
				ID:          "llama-3.3-70b",
				ProviderID:  "venice",
				Name:        "llama-3.3-70b",
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
				InputPrice:    0.35, // $0.35 per 1M input tokens
				OutputPrice:   0.4,  // $0.40 per 1M output tokens
			},
			{
				ID:          "deepseek-r1-llama-70b",
				ProviderID:  "venice",
				Name:        "deepseek-r1-llama-70b",
				DisplayName: "DeepSeek R1 Llama 70B",
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
				InputPrice:    0.35, // $0.35 per 1M input tokens
				OutputPrice:   0.4,  // $0.40 per 1M output tokens
			},
			{
				ID:          "qwen-2.5-72b",
				ProviderID:  "venice",
				Name:        "qwen-2.5-72b",
				DisplayName: "Qwen 2.5 72B",
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
				InputPrice:    0.35, // $0.35 per 1M input tokens
				OutputPrice:   0.4,  // $0.40 per 1M output tokens
			},
			{
				ID:          "llama-3.2-3b",
				ProviderID:  "venice",
				Name:        "llama-3.2-3b",
				DisplayName: "Llama 3.2 3B",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 128000,
				MaxOutput:     4096,
				InputPrice:    0.035, // $0.035 per 1M input tokens
				OutputPrice:   0.04,  // $0.04 per 1M output tokens
			},
		},
		"bedrock": {
			{
				ID:          "anthropic.claude-3-5-sonnet-20241022-v2:0",
				ProviderID:  "bedrock",
				Name:        "anthropic.claude-3-5-sonnet-20241022-v2:0",
				DisplayName: "Claude 3.5 Sonnet v2 (Bedrock)",
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
				InputPrice:    3.0,  // $3.00 per 1M input tokens
				OutputPrice:   15.0, // $15.00 per 1M output tokens
			},
			{
				ID:          "anthropic.claude-3-5-haiku-20241022-v1:0",
				ProviderID:  "bedrock",
				Name:        "anthropic.claude-3-5-haiku-20241022-v1:0",
				DisplayName: "Claude 3.5 Haiku (Bedrock)",
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
				InputPrice:    0.8, // $0.80 per 1M input tokens
				OutputPrice:   4.0, // $4.00 per 1M output tokens
			},
			{
				ID:          "meta.llama3-3-70b-instruct-v1:0",
				ProviderID:  "bedrock",
				Name:        "meta.llama3-3-70b-instruct-v1:0",
				DisplayName: "Llama 3.3 70B Instruct (Bedrock)",
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
				InputPrice:    0.72, // $0.72 per 1M input tokens
				OutputPrice:   0.72, // $0.72 per 1M output tokens
			},
			{
				ID:          "amazon.nova-pro-v1:0",
				ProviderID:  "bedrock",
				Name:        "amazon.nova-pro-v1:0",
				DisplayName: "Amazon Nova Pro",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 300000,
				MaxOutput:     5000,
				InputPrice:    0.8, // $0.80 per 1M input tokens
				OutputPrice:   3.2, // $3.20 per 1M output tokens
			},
			{
				ID:          "amazon.nova-lite-v1:0",
				ProviderID:  "bedrock",
				Name:        "amazon.nova-lite-v1:0",
				DisplayName: "Amazon Nova Lite",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 300000,
				MaxOutput:     5000,
				InputPrice:    0.06, // $0.06 per 1M input tokens
				OutputPrice:   0.24, // $0.24 per 1M output tokens
			},
		},
		"glm": {
			{
				ID:          "glm-4-plus",
				ProviderID:  "glm",
				Name:        "glm-4-plus",
				DisplayName: "GLM-4 Plus",
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
				InputPrice:    7.0, // ¥50/1M tokens ≈ $7.0
				OutputPrice:   7.0,
			},
			{
				ID:          "glm-4-air",
				ProviderID:  "glm",
				Name:        "glm-4-air",
				DisplayName: "GLM-4 Air",
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
				InputPrice:    0.14, // ¥1/1M tokens ≈ $0.14
				OutputPrice:   0.14,
			},
			{
				ID:          "glm-4-airx",
				ProviderID:  "glm",
				Name:        "glm-4-airx",
				DisplayName: "GLM-4 AirX",
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
				InputPrice:    1.4, // ¥10/1M tokens ≈ $1.4
				OutputPrice:   1.4,
			},
			{
				ID:          "glm-4-flash",
				ProviderID:  "glm",
				Name:        "glm-4-flash",
				DisplayName: "GLM-4 Flash",
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
				InputPrice:    0.0, // Free tier
				OutputPrice:   0.0,
			},
			{
				ID:          "glm-4-long",
				ProviderID:  "glm",
				Name:        "glm-4-long",
				DisplayName: "GLM-4 Long",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     4096,
				InputPrice:    0.14, // ¥1/1M tokens ≈ $0.14
				OutputPrice:   0.14,
			},
			{
				ID:          "glm-4v-plus",
				ProviderID:  "glm",
				Name:        "glm-4v-plus",
				DisplayName: "GLM-4V Plus",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					Vision:       true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 8192,
				MaxOutput:     1024,
				InputPrice:    1.4, // ¥10/1M tokens ≈ $1.4
				OutputPrice:   1.4,
			},
		},
		"zimaos-blue-trial": {
			// Default models for trial provider
			{
				ID:          "claude-haiku-4-5",
				ProviderID:  "zimaos-blue-trial",
				Name:        "claude-haiku-4-5",
				DisplayName: "Claude Haiku 4.5",
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
				InputPrice:    0.8,
				OutputPrice:   4.0,
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

// GetBuiltinPricingConfig returns pricing configuration from built-in models
func GetBuiltinPricingConfig() *PricingConfig {
	config := DefaultPricingConfig()

	// Extract pricing from all built-in models
	allModels := BuiltinModels()
	for _, models := range allModels {
		for _, model := range models {
			if model.InputPrice > 0 || model.OutputPrice > 0 {
				config.CustomPricing[model.ID] = &ModelPricing{
					ModelID:     model.ID,
					ProviderID:  model.ProviderID,
					InputPrice:  model.InputPrice,
					OutputPrice: model.OutputPrice,
					CachePrice:  0, // Built-in models don't have cache pricing
					IsCustom:    false,
					UpdatedAt:   config.UpdatedAt,
				}
			}
		}
	}

	return config
}
