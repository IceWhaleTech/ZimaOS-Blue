package adapters

import (
	"context"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providers"
)

// Adapter is an interface for tool calling adapters.
type Adapter interface {
	Chat(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error)
}

// RouterConfig holds configuration for the capability router.
type RouterConfig struct {
	// AutoDetect enables automatic capability detection
	AutoDetect bool `json:"auto_detect"`

	// DefaultAdapter is the default adapter to use when no specific adapter is configured
	DefaultAdapter string `json:"default_adapter"` // "cliproxy", "ccnexus"

	// ProviderOverrides allows overriding the adapter for specific providers
	ProviderOverrides map[string]string `json:"provider_overrides"`

	// FallbackEnabled enables fallback to CLIProxy when native tool calling fails
	FallbackEnabled bool `json:"fallback_enabled"`
}

// DefaultRouterConfig returns the default router configuration.
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		AutoDetect:        true,
		DefaultAdapter:    "cliproxy",
		ProviderOverrides: make(map[string]string),
		FallbackEnabled:   true,
	}
}

// CapabilityRouter routes requests to the appropriate adapter based on provider capabilities.
type CapabilityRouter struct {
	mu                 sync.RWMutex
	config             *RouterConfig
	capabilityDetector *providers.CapabilityDetector
	providers          map[string]LLMProvider
	adapters           map[string]Adapter
	cliProxyAdapters   map[string]*CLIProxyAdapter
	ccNexusAdapters    map[string]*ccNexusAdapter
}

// NewCapabilityRouter creates a new capability router.
func NewCapabilityRouter(config *RouterConfig) *CapabilityRouter {
	if config == nil {
		config = DefaultRouterConfig()
	}

	return &CapabilityRouter{
		config:             config,
		capabilityDetector: providers.NewCapabilityDetector(),
		providers:          make(map[string]LLMProvider),
		adapters:           make(map[string]Adapter),
		cliProxyAdapters:   make(map[string]*CLIProxyAdapter),
		ccNexusAdapters:    make(map[string]*ccNexusAdapter),
	}
}

// RegisterProvider registers a provider with the router.
func (r *CapabilityRouter) RegisterProvider(name string, provider LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = provider

	// Create adapters for the provider
	r.cliProxyAdapters[name] = NewCLIProxyAdapter(provider)
	r.ccNexusAdapters[name] = NewCCNexusAdapter("openai", r.getTargetFormat(name))
}

// getTargetFormat returns the target format for a provider.
func (r *CapabilityRouter) getTargetFormat(provider string) string {
	switch provider {
	case "anthropic":
		return "anthropic"
	case "ollama":
		return "ollama"
	case "gemini":
		return "gemini"
	default:
		return "openai"
	}
}

// RegisterTools registers tools with all CLIProxy adapters.
func (r *CapabilityRouter) RegisterTools(tools []Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, adapter := range r.cliProxyAdapters {
		adapter.RegisterTools(tools)
	}
}

// Route routes a request to the appropriate handler based on provider capabilities.
func (r *CapabilityRouter) Route(ctx context.Context, providerName string, messages []Message, options *ChatOptions, requireToolCalling bool) (*ChatResponse, error) {
	r.mu.RLock()
	provider, ok := r.providers[providerName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	// Check if tool calling is required
	if !requireToolCalling {
		// Direct call without tool support
		return provider.Chat(ctx, messages, options)
	}

	// Get provider capability
	cap, _ := r.capabilityDetector.GetCapability(providerName)

	// Check for provider override
	adapterType := r.getAdapterType(providerName, cap)

	switch adapterType {
	case "native":
		// Use native tool calling
		return provider.Chat(ctx, messages, options)

	case "ccnexus":
		// Use ccNexus adapter for schema conversion
		r.mu.RLock()
		adapter := r.ccNexusAdapters[providerName]
		r.mu.RUnlock()

		if adapter == nil {
			return nil, fmt.Errorf("ccNexus adapter not found for provider: %s", providerName)
		}

		// For ccNexus, we still use the native provider but with converted schemas
		return provider.Chat(ctx, messages, options)

	case "cliproxy":
		// Use CLIProxy adapter for prompt-based tool calling
		r.mu.RLock()
		adapter := r.cliProxyAdapters[providerName]
		r.mu.RUnlock()

		if adapter == nil {
			return nil, fmt.Errorf("CLIProxy adapter not found for provider: %s", providerName)
		}

		return adapter.Chat(ctx, messages, options)

	default:
		// Fallback to CLIProxy
		r.mu.RLock()
		adapter := r.cliProxyAdapters[providerName]
		r.mu.RUnlock()

		if adapter != nil {
			return adapter.Chat(ctx, messages, options)
		}

		return provider.Chat(ctx, messages, options)
	}
}

// getAdapterType determines the adapter type to use for a provider.
func (r *CapabilityRouter) getAdapterType(providerName string, cap *providers.ProviderCapability) string {
	// Check for override
	if override, ok := r.config.ProviderOverrides[providerName]; ok {
		return override
	}

	// Use capability detection
	if cap == nil {
		return r.config.DefaultAdapter
	}

	switch cap.ToolCalling {
	case providers.ToolCallingNative:
		return "native"
	case providers.ToolCallingPartial:
		return "ccnexus"
	case providers.ToolCallingNone:
		return "cliproxy"
	default:
		return r.config.DefaultAdapter
	}
}

// SelectAdapter selects the appropriate adapter for a provider.
func (r *CapabilityRouter) SelectAdapter(providerName string) (Adapter, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, _ := r.capabilityDetector.GetCapability(providerName)
	adapterType := r.getAdapterType(providerName, cap)

	switch adapterType {
	case "cliproxy":
		if adapter, ok := r.cliProxyAdapters[providerName]; ok {
			return adapter, "cliproxy", nil
		}
	case "ccnexus":
		// ccNexus doesn't implement Adapter interface directly
		// It's used for schema conversion
		return nil, "ccnexus", nil
	case "native":
		return nil, "native", nil
	}

	return nil, "", fmt.Errorf("no adapter found for provider: %s", providerName)
}

// GetCapability returns the capability for a provider.
func (r *CapabilityRouter) GetCapability(providerName string) (*providers.ProviderCapability, bool) {
	return r.capabilityDetector.GetCapability(providerName)
}

// GetCapabilityMatrix returns all known provider capabilities.
func (r *CapabilityRouter) GetCapabilityMatrix() map[string]providers.ProviderCapability {
	return r.capabilityDetector.GetCapabilityMatrix()
}

// NeedsAdapter returns true if the provider needs an adapter.
func (r *CapabilityRouter) NeedsAdapter(providerName string) bool {
	return r.capabilityDetector.NeedsAdapter(providerName)
}

// GetCLIProxyAdapter returns the CLIProxy adapter for a provider.
func (r *CapabilityRouter) GetCLIProxyAdapter(providerName string) *CLIProxyAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cliProxyAdapters[providerName]
}

// GetCCNexusAdapter returns the ccNexus adapter for a provider.
func (r *CapabilityRouter) GetCCNexusAdapter(providerName string) *ccNexusAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ccNexusAdapters[providerName]
}

// ConvertToolsForProvider converts tools to the appropriate format for a provider.
func (r *CapabilityRouter) ConvertToolsForProvider(providerName string, tools []Tool) ([]interface{}, error) {
	r.mu.RLock()
	adapter := r.ccNexusAdapters[providerName]
	r.mu.RUnlock()

	if adapter == nil {
		// Create a temporary adapter
		adapter = NewCCNexusAdapter("openai", r.getTargetFormat(providerName))
	}

	return adapter.ConvertTools(tools)
}
