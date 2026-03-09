package mediagen

import (
	"os"
	"strconv"
	"strings"
)

const (
	fakeMediaProviderID  = "fake-media-dev"
	fakeMediaModelID     = "fake-image"
	fakeMediaProviderEnv = "ZIMA_ENABLE_FAKE_MEDIA_PROVIDER"
)

// MediaProviderConfig holds configuration for a media generation provider.
type MediaProviderConfig struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Enabled     bool             `json:"enabled"`
	Priority    int              `json:"priority"` // lower = higher priority; controls model conflict resolution & default selection
	BaseURL     string           `json:"base_url,omitempty"`
	APIKey      string           `json:"-"`           // never serialized to API responses
	HasAPIKey   bool             `json:"has_api_key"` // frontend indicator
	KeyHash     string           `json:"key_hash,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Description string           `json:"description,omitempty"`
	Website     string           `json:"website,omitempty"`
	APIKeyURL   string           `json:"api_key_url,omitempty"`
	Models      []MediaModelInfo `json:"models,omitempty"` // available models when registered
}

// hashAPIKey creates a display-safe mask: first8...last4 (same as provider pool).
func hashAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		result := make([]byte, len(key))
		for i := range result {
			result[i] = '*'
		}
		return string(result)
	}
	return key[:8] + "..." + key[len(key)-4:]
}

func providerRequiresAPIKey(id string) bool {
	return id != fakeMediaProviderID
}

func providerHasCredential(c *MediaProviderConfig) bool {
	if c == nil {
		return false
	}
	if !providerRequiresAPIKey(c.ID) {
		return true
	}
	return strings.TrimSpace(c.APIKey) != ""
}

func fakeMediaProviderEnabled() bool {
	v := strings.TrimSpace(os.Getenv(fakeMediaProviderEnv))
	if v == "" {
		return false
	}
	if ok, err := strconv.ParseBool(v); err == nil {
		return ok
	}
	switch strings.ToLower(v) {
	case "on", "yes", "y":
		return true
	default:
		return false
	}
}

// isChineseLocale returns true if the locale string indicates a Chinese region.
func isChineseLocale(locale string) bool {
	locale = strings.ToLower(locale)
	return strings.HasPrefix(locale, "zh") || locale == "cn"
}

// BuiltinMediaProviders returns the default media provider configurations.
// Priority is locale-aware: Chinese locales prefer DashScope, others prefer Gemini.
func BuiltinMediaProviders(locale string) []*MediaProviderConfig {
	// Priority: lower number = higher priority (wins model ID conflicts)
	// CN: DashScope(10) > Gemini(20) > MuleRouter(30)
	// Other: Gemini(10) > DashScope(20) > MuleRouter(30)
	cn := isChineseLocale(locale)
	dashPri, geminiPri := 20, 10
	if cn {
		dashPri, geminiPri = 10, 20
	}

	providers := []*MediaProviderConfig{
		{
			ID:          "dashscope-image",
			Name:        "DashScope (Qwen Image)",
			Priority:    dashPri,
			BaseURL:     "https://dashscope.aliyuncs.com",
			Icon:        "qwen",
			Description: "Alibaba DashScope - Qwen Image, Wanx text-to-image models",
			Website:     "https://dashscope.aliyun.com",
			APIKeyURL:   "https://dashscope.console.aliyun.com/apiKey",
		},
		{
			ID:          "gemini-image",
			Name:        "Gemini Image",
			Priority:    geminiPri,
			BaseURL:     "https://generativelanguage.googleapis.com",
			Icon:        "google",
			Description: "Google Gemini - Native image generation via Imagen and Gemini models",
			Website:     "https://ai.google.dev",
			APIKeyURL:   "https://aistudio.google.com/apikey",
		},
		{
			ID:          "mulerouter",
			Name:        "MuleRouter",
			Priority:    30,
			BaseURL:     "https://api.mulerouter.ai",
			Icon:        "mulerouter",
			Description: "MuleRouter - Unified aggregator for DALL-E, Midjourney, Qwen Image, Wan2 video",
			Website:     "https://www.mulerouter.ai/",
			APIKeyURL:   "https://www.mulerouter.ai/app/api-keys",
		},
		{
			ID:          "minimax-media",
			Name:        "MiniMax (Hailuo)",
			Priority:    40,
			BaseURL:     "https://api.minimax.chat",
			Icon:        "minimax",
			Description: "MiniMax - Hailuo video generation and text-to-speech",
			Website:     "https://www.minimax.io",
			APIKeyURL:   "https://platform.minimaxi.com/user-center/basic-information/interface-key",
		},
	}

	if fakeMediaProviderEnabled() {
		providers = append(providers, &MediaProviderConfig{
			ID:          fakeMediaProviderID,
			Name:        "Fake Media (Dev)",
			Priority:    999,
			BaseURL:     "dev://fake-media",
			HasAPIKey:   true,
			Icon:        "sparkles",
			Description: "Dev-only fake media provider for local image-generation smoke tests. Returns placeholder images without external API calls.",
		})
	}

	return providers
}
