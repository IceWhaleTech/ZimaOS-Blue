package providerpool

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Build-time variables (injected via -ldflags)
// Example: go build -ldflags "-X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=..."
var (
	trialLicense string // Ed25519-signed license injected at build time
)

// builtinProvidersFallback returns the embedded fallback snapshot of built-in providers.
// The trial provider is only included if a trial API key was injected at build time.
func builtinProvidersFallback() []*Provider {
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
			APIKeyURL:   "https://platform.openai.com/api-keys",
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
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
			APIKeyURL:   "https://console.anthropic.com/settings/keys",
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "openrouter",
			Name:        "OpenRouter",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://openrouter.ai/api",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "openrouter",
			Description: "OpenRouter - Access multiple models through one API",
			Website:     "https://openrouter.ai",
			APIKeyURL:   "https://openrouter.ai/keys",
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "minimax",
			Name:        "MiniMax",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.minimaxi.com/anthropic",
			APIVersion:  "v1",
			APIFormat:   APIFormatAnthropic,
			Priority:    30,
			Icon:        "minimax",
			Description: "MiniMax API - M2.7 series models",
			Website:     "https://platform.minimax.io",
			APIKeyURL:   "https://platform.minimax.io/user-center/basic-information/interface-key",
			AlternateBaseURLs: []string{
				"https://api.minimaxi.com",           // China OpenAI endpoint (works with API key)
				"https://api.minimax.io",             // International OpenAI endpoint
				"https://api.minimaxi.com/anthropic", // China Anthropic-compatible endpoint
				"https://api.minimax.io/anthropic",   // International Anthropic-compatible endpoint
			},
			CreatedAt: timeutil.NowTime(),
			UpdatedAt: timeutil.NowTime(),
		},
		{
			ID:          "qwen",
			Name:        "Qwen (Alibaba Cloud)",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    35,
			Icon:        "qwen",
			Description: "Alibaba Cloud Qwen API - Qwen series models with multilingual support",
			Website:     "https://tongyi.aliyun.com",
			APIKeyURL:   "https://dashscope.console.aliyun.com/apiKey",
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
	}

	// Untested providers are exposed under "Other" in UI and marked as beta.
	betaProviders := []*Provider{
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
			Priority:    20,
			Icon:        "google",
			Description: "Google Gemini API",
			Website:     "https://ai.google.dev",
			APIKeyURL:   "https://aistudio.google.com/app/apikey",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "deepseek",
			Name:        "DeepSeek",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.deepseek.com",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "deepseek",
			Description: "DeepSeek API",
			Website:     "https://platform.deepseek.com",
			APIKeyURL:   "https://platform.deepseek.com/api_keys",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "moonshot",
			Name:        "Moonshot (Kimi)",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.moonshot.ai/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "moonshot",
			Description: "Moonshot/Kimi API",
			Website:     "https://platform.moonshot.cn",
			APIKeyURL:   "https://platform.moonshot.cn/console/api-keys",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
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
			Description: "Run open-source models locally via Ollama",
			Website:     "https://ollama.com",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "grok",
			Name:        "xAI Grok",
			Type:        ProviderTypeBuiltin,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.x.ai/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "grok",
			Description: "xAI Grok API",
			Website:     "https://x.ai",
			APIKeyURL:   "https://console.x.ai",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
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
			Priority:    20,
			Icon:        "glm",
			Description: "Zhipu GLM API",
			Website:     "https://www.bigmodel.cn",
			APIKeyURL:   "https://open.bigmodel.cn/usercenter/apikeys",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "siliconflow",
			Name:        "SiliconFlow",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.siliconflow.cn/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "siliconflow",
			Description: "SiliconFlow model platform",
			Website:     "https://cloud.siliconflow.cn",
			APIKeyURL:   "https://cloud.siliconflow.cn/account/ak",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "nvidia",
			Name:        "NVIDIA NIM",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://integrate.api.nvidia.com/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "nvidia",
			Description: "NVIDIA NIM API",
			Website:     "https://build.nvidia.com",
			APIKeyURL:   "https://build.nvidia.com",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "venice",
			Name:        "Venice AI",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.venice.ai/api/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "venice",
			Description: "Venice privacy-focused AI API",
			Website:     "https://venice.ai",
			APIKeyURL:   "https://venice.ai",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "bedrock",
			Name:        "Amazon Bedrock",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "",
			APIVersion:  "2023-05-31",
			APIFormat:   APIFormatAnthropic,
			Priority:    20,
			Icon:        "aws",
			Description: "Amazon Bedrock managed model platform",
			Website:     "https://aws.amazon.com/bedrock",
			APIKeyURL:   "https://console.aws.amazon.com/bedrock",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "aihubmix",
			Name:        "AiHubMix",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://aihubmix.com/v1",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "aihubmix",
			Description: "AiHubMix aggregator API",
			Website:     "https://aihubmix.com",
			APIKeyURL:   "https://aihubmix.com",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "openrouter-free",
			Name:        "OpenRouter (Free Tier)",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://openrouter.ai/api",
			APIVersion:  "v1",
			APIFormat:   APIFormatOpenAI,
			Priority:    20,
			Icon:        "openrouter",
			Description: "OpenRouter free-tier profile",
			Website:     "https://openrouter.ai",
			APIKeyURL:   "https://openrouter.ai/keys",
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "google-antigravity",
			Name:        "Google Cloud Code Assist (Antigravity)",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://cloudcode-pa.googleapis.com",
			APIFormat:   APIFormatCloudCode,
			Priority:    20,
			Icon:        "google",
			Description: "OAuth-based Cloud Code Assist via Antigravity",
			Website:     "https://cloud.google.com/code-assist",
			OAuth:       &OAuthConfig{ProviderType: "antigravity"},
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "google-gemini-cli",
			Name:        "Google Cloud Code Assist (Gemini CLI)",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://cloudcode-pa.googleapis.com",
			APIFormat:   APIFormatCloudCode,
			Priority:    20,
			Icon:        "google",
			Description: "OAuth-based Cloud Code Assist via Gemini CLI",
			Website:     "https://cloud.google.com/code-assist",
			OAuth:       &OAuthConfig{ProviderType: "gemini-cli"},
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "github-copilot",
			Name:        "GitHub Copilot",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://api.githubcopilot.com",
			APIFormat:   APIFormatCopilot,
			Priority:    20,
			Icon:        "copilot",
			Description: "OAuth-based GitHub Copilot provider",
			Website:     "https://github.com/features/copilot",
			OAuth:       &OAuthConfig{ProviderType: "copilot"},
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
		{
			ID:          "openai-codex",
			Name:        "OpenAI Codex CLI (OAuth)",
			Type:        ProviderTypePlatform,
			Location:    ProviderLocationCloud,
			Enabled:     false,
			Status:      ProviderStatusInactive,
			BaseURL:     "https://chatgpt.com/backend-api/codex/responses",
			APIFormat:   APIFormatResponses,
			Priority:    20,
			Icon:        "codex",
			Description: "OAuth-based OpenAI Codex subscription provider",
			Website:     "https://openai.com/codex",
			OAuth:       &OAuthConfig{ProviderType: "codex"},
			Beta:        true,
			CreatedAt:   timeutil.NowTime(),
			UpdatedAt:   timeutil.NowTime(),
		},
	}
	providers = append(providers, betaProviders...)

	// Only include trial provider if a valid signed license was injected at build time
	if claims := getTrialClaims(); claims != nil {
		baseURL := claims.URL
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
				Label: "Trial API Key", Enabled: true, CreatedAt: timeutil.NowTime(),
			}},
			CreatedAt: timeutil.NowTime(),
			UpdatedAt: timeutil.NowTime(),
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

// builtinModelsFallback returns the embedded fallback snapshot of built-in models.
// Prices are per 1M tokens in USD (as of Jan 2026)
func builtinModelsFallback() map[string][]*Model {
	return map[string][]*Model{
		"openai": {
			{
				ID:          "gpt-5.4",
				ProviderID:  "openai",
				Name:        "gpt-5.4",
				DisplayName: "GPT-5.4",
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
				ContextWindow: 1050000, // 1,050K tokens per official spec
				MaxOutput:     128000,  // 128K per official spec
				InputPrice:    2.5,     // $2.50 per 1M input tokens
				OutputPrice:   15.0,    // $15.00 per 1M output tokens
				CachePrice:    0.25,    // $0.25 per 1M cached input tokens
			},
			{
				ID:          "gpt-5.4-mini",
				ProviderID:  "openai",
				Name:        "gpt-5.4-mini",
				DisplayName: "GPT-5.4 Mini",
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
				ContextWindow: 1050000, // 1,050K tokens per official spec
				MaxOutput:     128000,  // 128K per official spec
				InputPrice:    0.75,    // $0.75 per 1M input tokens
				OutputPrice:   4.50,    // $4.50 per 1M output tokens
				CachePrice:    0.075,   // $0.075 per 1M cached input tokens
			},
			{
				ID:          "gpt-5.4-nano",
				ProviderID:  "openai",
				Name:        "gpt-5.4-nano",
				DisplayName: "GPT-5.4 Nano",
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
				ContextWindow: 400000,  // 400K tokens per official spec
				MaxOutput:     128000,  // 128K per official spec
				InputPrice:    0.20,    // $0.20 per 1M input tokens
				OutputPrice:   1.25,    // $1.25 per 1M output tokens
				CachePrice:    0.02,    // $0.02 per 1M cached input tokens
			},
			{
				ID:          "gpt-5.4-pro",
				ProviderID:  "openai",
				Name:        "gpt-5.4-pro",
				DisplayName: "GPT-5.4 Pro",
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
				ContextWindow: 1050000, // 1,050K tokens per official spec
				MaxOutput:     128000,  // 128K per official spec
				InputPrice:    30.00,   // $30.00 per 1M input tokens
				OutputPrice:   180.00,  // $180.00 per 1M output tokens
				CachePrice:    3.00,    // $3.00 per 1M cached input tokens (1/10th of input)
			},
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
				ContextWindow: 128000,
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
				ContextWindow: 128000,
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
				ID:          "MiniMax-M2.7",
				ProviderID:  "minimax",
				Name:        "MiniMax-M2.7",
				DisplayName: "MiniMax M2.7",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     16384,
				InputPrice:    1.0,
				OutputPrice:   8.0,
			},
			{
				ID:          "MiniMax-M2.7-highspeed",
				ProviderID:  "minimax",
				Name:        "MiniMax-M2.7-highspeed",
				DisplayName: "MiniMax M2.7 Highspeed",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 1000000,
				MaxOutput:     16384,
				InputPrice:    1.0,
				OutputPrice:   8.0,
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
		"siliconflow": {
			{
				ID:          "deepseek-ai/DeepSeek-V3",
				ProviderID:  "siliconflow",
				Name:        "deepseek-ai/DeepSeek-V3",
				DisplayName: "DeepSeek V3",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 65536,
				MaxOutput:     8192,
				InputPrice:    0.27,
				OutputPrice:   1.10,
			},
			{
				ID:          "deepseek-ai/DeepSeek-R1",
				ProviderID:  "siliconflow",
				Name:        "deepseek-ai/DeepSeek-R1",
				DisplayName: "DeepSeek R1",
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					JSON:         true,
					SystemPrompt: true,
				},
				ContextWindow: 65536,
				MaxOutput:     8192,
				InputPrice:    0.55,
				OutputPrice:   2.19,
			},
			{
				ID:          "Qwen/Qwen2.5-72B-Instruct",
				ProviderID:  "siliconflow",
				Name:        "Qwen/Qwen2.5-72B-Instruct",
				DisplayName: "Qwen 2.5 72B Instruct",
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
				InputPrice:    0.56,
				OutputPrice:   0.77,
			},
			{
				ID:          "Qwen/QwQ-32B",
				ProviderID:  "siliconflow",
				Name:        "Qwen/QwQ-32B",
				DisplayName: "QwQ 32B",
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
				InputPrice:    0.28,
				OutputPrice:   0.50,
			},
		},
		"nvidia": {
			{
				ID:          "meta/llama-3.3-70b-instruct",
				ProviderID:  "nvidia",
				Name:        "meta/llama-3.3-70b-instruct",
				DisplayName: "Llama 3.3 70B Instruct",
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
				InputPrice:    0.35,
				OutputPrice:   0.40,
			},
			{
				ID:          "deepseek-ai/deepseek-r1",
				ProviderID:  "nvidia",
				Name:        "deepseek-ai/deepseek-r1",
				DisplayName: "DeepSeek R1",
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
				InputPrice:    0.55,
				OutputPrice:   2.19,
			},
			{
				ID:          "qwen/qwen2.5-72b-instruct",
				ProviderID:  "nvidia",
				Name:        "qwen/qwen2.5-72b-instruct",
				DisplayName: "Qwen 2.5 72B Instruct",
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
				InputPrice:    0.56,
				OutputPrice:   0.77,
			},
			{
				ID:          "nvidia/llama-3.1-nemotron-70b-instruct",
				ProviderID:  "nvidia",
				Name:        "nvidia/llama-3.1-nemotron-70b-instruct",
				DisplayName: "Nemotron 70B Instruct",
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
				InputPrice:    0.35,
				OutputPrice:   0.40,
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
		// --- OAuth-based Provider Models ---
		"google-antigravity": {
			{
				ID: "claude-sonnet-4-5-20250929", ProviderID: "google-antigravity",
				Name: "claude-sonnet-4-5-20250929", DisplayName: "Claude Sonnet 4.5 (Cloud Code)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, Thinking: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 200000, MaxOutput: 64000,
				InputPrice: 3.0, OutputPrice: 15.0, CachePrice: 0.3,
			},
			{
				ID: "gemini-2.0-flash", ProviderID: "google-antigravity",
				Name: "gemini-2.0-flash", DisplayName: "Gemini 2.0 Flash (Cloud Code)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 1000000, MaxOutput: 8192,
				InputPrice: 0.1, OutputPrice: 0.4, CachePrice: 0.025,
			},
		},
		"google-gemini-cli": {
			{
				ID: "gemini-2.0-flash", ProviderID: "google-gemini-cli",
				Name: "gemini-2.0-flash", DisplayName: "Gemini 2.0 Flash (Gemini CLI)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 1000000, MaxOutput: 8192,
				InputPrice: 0.1, OutputPrice: 0.4, CachePrice: 0.025,
			},
			{
				ID: "gemini-1.5-pro", ProviderID: "google-gemini-cli",
				Name: "gemini-1.5-pro", DisplayName: "Gemini 1.5 Pro (Gemini CLI)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 2000000, MaxOutput: 8192,
				InputPrice: 1.25, OutputPrice: 5.0, CachePrice: 0.3125,
			},
		},
		"github-copilot": {
			{
				ID: "gpt-4o", ProviderID: "github-copilot",
				Name: "gpt-4o", DisplayName: "GPT-4o (Copilot)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 128000, MaxOutput: 16384,
				InputPrice: 2.5, OutputPrice: 10.0, CachePrice: 1.25,
			},
			{
				ID: "gpt-4o-mini", ProviderID: "github-copilot",
				Name: "gpt-4o-mini", DisplayName: "GPT-4o mini (Copilot)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 128000, MaxOutput: 16384,
				InputPrice: 0.2, OutputPrice: 0.8, CachePrice: 0.1,
			},
		},
		"openai-codex": {
			{
				ID: "o3", ProviderID: "openai-codex",
				Name: "o3", DisplayName: "o3 (Codex)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, Thinking: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 200000, MaxOutput: 100000,
				InputPrice: 2.0, OutputPrice: 8.0, CachePrice: 1.0,
			},
			{
				ID: "o4-mini", ProviderID: "openai-codex",
				Name: "o4-mini", DisplayName: "o4-mini (Codex)",
				Enabled: true,
				Capabilities: ModelCapabilities{
					Chat: true, Vision: true, FunctionCall: true, Streaming: true, Thinking: true, JSON: true, SystemPrompt: true,
				},
				ContextWindow: 200000, MaxOutput: 100000,
				InputPrice: 1.1, OutputPrice: 4.4, CachePrice: 0.55,
			},
		},
	}
}

// BuiltinProviders returns the effective built-in provider list with remote catalog overrides applied.
func BuiltinProviders() []*Provider {
	providers := builtinProvidersFallback()
	for _, provider := range providers {
		if provider == nil || provider.MetadataMode != "" {
			continue
		}
		provider.MetadataMode = defaultProviderMetadataMode(provider)
	}
	applyOfficialProviderCatalogToProviders(providers)
	return providers
}

// BuiltinModels returns effective built-in models with remote catalog overrides applied.
func BuiltinModels() map[string][]*Model {
	models := builtinModelsFallback()
	applyOfficialProviderCatalogToModels(models)
	return models
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
					CachePrice:  model.CachePrice,
					IsCustom:    false,
					UpdatedAt:   config.UpdatedAt,
				}
			}
		}
	}

	return config
}
