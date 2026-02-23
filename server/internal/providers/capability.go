package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ToolCallingSupport represents the level of tool calling support.
type ToolCallingSupport string

const (
	ToolCallingNative  ToolCallingSupport = "native"  // Full native support
	ToolCallingPartial ToolCallingSupport = "partial" // Partial support (may need adapter)
	ToolCallingNone    ToolCallingSupport = "none"    // No support (requires CLIProxy)
)

// ProviderCapability represents the capabilities of an LLM provider.
type ProviderCapability struct {
	Provider        string             `json:"provider"`
	ToolCalling     ToolCallingSupport `json:"tool_calling"`
	Streaming       bool               `json:"streaming"`
	Vision          bool               `json:"vision"`
	MaxTokens       int                `json:"max_tokens"`
	MaxContextSize  int                `json:"max_context_size"`
	SupportedModels []string           `json:"supported_models,omitempty"`
	AdapterRequired bool               `json:"adapter_required"`
	AdapterType     string             `json:"adapter_type,omitempty"` // "cliproxy", "ccnexus", ""
	DetectedAt      time.Time          `json:"detected_at"`
}

// CapabilityDetector detects provider capabilities.
type CapabilityDetector struct {
	Timeout time.Duration
	client  *http.Client

	// Known capabilities (static)
	knownCapabilities map[string]ProviderCapability
}

// NewCapabilityDetector creates a new capability detector.
func NewCapabilityDetector() *CapabilityDetector {
	return &CapabilityDetector{
		Timeout: 10 * time.Second,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		knownCapabilities: getKnownCapabilities(),
	}
}

// getKnownCapabilities returns the known capabilities for common providers.
func getKnownCapabilities() map[string]ProviderCapability {
	return map[string]ProviderCapability{
		"anthropic": {
			Provider:        "anthropic",
			ToolCalling:     ToolCallingNative,
			Streaming:       true,
			Vision:          true,
			MaxTokens:       8192,
			MaxContextSize:  200000,
			SupportedModels: []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku", "claude-3-5-sonnet", "claude-3-5-haiku"},
			AdapterRequired: false,
		},
		"openai": {
			Provider:        "openai",
			ToolCalling:     ToolCallingNative,
			Streaming:       true,
			Vision:          true,
			MaxTokens:       16384,
			MaxContextSize:  128000,
			SupportedModels: []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo"},
			AdapterRequired: false,
		},
		"ollama": {
			Provider:        "ollama",
			ToolCalling:     ToolCallingPartial,
			Streaming:       true,
			Vision:          true, // Some models support vision
			MaxTokens:       4096,
			MaxContextSize:  32768,
			SupportedModels: []string{"llama3.2", "llama3.1", "mistral", "codellama", "phi3", "gemma2"},
			AdapterRequired: true,
			AdapterType:     "ccnexus",
		},
		"deepseek": {
			Provider:        "deepseek",
			ToolCalling:     ToolCallingNative,
			Streaming:       true,
			Vision:          false,
			MaxTokens:       8192,
			MaxContextSize:  64000,
			SupportedModels: []string{"deepseek-chat", "deepseek-coder"},
			AdapterRequired: false,
		},
		"gemini": {
			Provider:        "gemini",
			ToolCalling:     ToolCallingNative,
			Streaming:       true,
			Vision:          true,
			MaxTokens:       8192,
			MaxContextSize:  1000000,
			SupportedModels: []string{"gemini-pro", "gemini-pro-vision", "gemini-1.5-pro", "gemini-1.5-flash"},
			AdapterRequired: false,
		},
		"groq": {
			Provider:        "groq",
			ToolCalling:     ToolCallingNative,
			Streaming:       true,
			Vision:          false,
			MaxTokens:       8192,
			MaxContextSize:  32768,
			SupportedModels: []string{"llama-3.1-70b", "llama-3.1-8b", "mixtral-8x7b"},
			AdapterRequired: false,
		},
		"local": {
			Provider:        "local",
			ToolCalling:     ToolCallingNone,
			Streaming:       true,
			Vision:          false,
			MaxTokens:       4096,
			MaxContextSize:  8192,
			SupportedModels: []string{},
			AdapterRequired: true,
			AdapterType:     "cliproxy",
		},
	}
}

// GetCapability returns the capability for a known provider.
func (d *CapabilityDetector) GetCapability(provider string) (*ProviderCapability, bool) {
	provider = strings.ToLower(provider)
	cap, ok := d.knownCapabilities[provider]
	if ok {
		cap.DetectedAt = timeutil.NowTime()
		return &cap, true
	}
	return nil, false
}

// DetectCapabilities detects capabilities for a provider.
func (d *CapabilityDetector) DetectCapabilities(ctx context.Context, provider string, baseURL string) (*ProviderCapability, error) {
	provider = strings.ToLower(provider)

	// Check known capabilities first
	if cap, ok := d.GetCapability(provider); ok {
		return cap, nil
	}

	// For unknown providers, try to detect via API
	if baseURL != "" {
		return d.detectFromAPI(ctx, provider, baseURL)
	}

	// Return default capability for unknown provider
	return &ProviderCapability{
		Provider:        provider,
		ToolCalling:     ToolCallingNone,
		Streaming:       true,
		Vision:          false,
		MaxTokens:       4096,
		MaxContextSize:  8192,
		AdapterRequired: true,
		AdapterType:     "cliproxy",
		DetectedAt:      timeutil.NowTime(),
	}, nil
}

// detectFromAPI tries to detect capabilities from the provider's API.
func (d *CapabilityDetector) detectFromAPI(ctx context.Context, provider string, baseURL string) (*ProviderCapability, error) {
	cap := &ProviderCapability{
		Provider:        provider,
		ToolCalling:     ToolCallingNone,
		Streaming:       true,
		Vision:          false,
		MaxTokens:       4096,
		MaxContextSize:  8192,
		AdapterRequired: true,
		AdapterType:     "cliproxy",
		DetectedAt:      timeutil.NowTime(),
	}

	// Try to get models list (OpenAI-compatible API)
	modelsURL := strings.TrimSuffix(baseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return cap, nil
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return cap, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var modelsResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil {
			for _, m := range modelsResp.Data {
				cap.SupportedModels = append(cap.SupportedModels, m.ID)
			}
		}

		// If it responds to OpenAI-compatible API, it likely supports tool calling
		cap.ToolCalling = ToolCallingPartial
		cap.AdapterType = "ccnexus"
	}

	return cap, nil
}

// TestToolCalling tests if a provider supports tool calling.
func (d *CapabilityDetector) TestToolCalling(ctx context.Context, provider string, baseURL string, apiKey string) (ToolCallingSupport, error) {
	// For known providers, return known capability
	if cap, ok := d.GetCapability(provider); ok {
		return cap.ToolCalling, nil
	}

	// For unknown providers, we would need to make a test API call
	// This is a simplified version that returns partial support for OpenAI-compatible APIs
	if baseURL != "" {
		return ToolCallingPartial, nil
	}

	return ToolCallingNone, nil
}

// GetCapabilityMatrix returns all known provider capabilities.
func (d *CapabilityDetector) GetCapabilityMatrix() map[string]ProviderCapability {
	result := make(map[string]ProviderCapability)
	for k, v := range d.knownCapabilities {
		v.DetectedAt = timeutil.NowTime()
		result[k] = v
	}
	return result
}

// NeedsAdapter returns true if the provider needs an adapter for tool calling.
func (d *CapabilityDetector) NeedsAdapter(provider string) bool {
	if cap, ok := d.GetCapability(provider); ok {
		return cap.AdapterRequired
	}
	return true // Unknown providers need adapter by default
}

// GetAdapterType returns the recommended adapter type for a provider.
func (d *CapabilityDetector) GetAdapterType(provider string) string {
	if cap, ok := d.GetCapability(provider); ok {
		return cap.AdapterType
	}
	return "cliproxy" // Default to CLIProxy for unknown providers
}

// CapabilityInfo is a simplified capability info for API responses.
type CapabilityInfo struct {
	Provider        string   `json:"provider"`
	ToolCalling     string   `json:"tool_calling"`
	Streaming       bool     `json:"streaming"`
	Vision          bool     `json:"vision"`
	AdapterRequired bool     `json:"adapter_required"`
	AdapterType     string   `json:"adapter_type,omitempty"`
	Models          []string `json:"models,omitempty"`
}

// ToInfo converts ProviderCapability to CapabilityInfo.
func (c *ProviderCapability) ToInfo() *CapabilityInfo {
	return &CapabilityInfo{
		Provider:        c.Provider,
		ToolCalling:     string(c.ToolCalling),
		Streaming:       c.Streaming,
		Vision:          c.Vision,
		AdapterRequired: c.AdapterRequired,
		AdapterType:     c.AdapterType,
		Models:          c.SupportedModels,
	}
}

// CheckCompatibility checks if a provider is compatible with tool calling requirements.
func (d *CapabilityDetector) CheckCompatibility(provider string, requireToolCalling bool, requireVision bool) error {
	cap, ok := d.GetCapability(provider)
	if !ok {
		return fmt.Errorf("unknown provider: %s", provider)
	}

	if requireToolCalling && cap.ToolCalling == ToolCallingNone {
		return fmt.Errorf("provider %s does not support tool calling", provider)
	}

	if requireVision && !cap.Vision {
		return fmt.Errorf("provider %s does not support vision", provider)
	}

	return nil
}
