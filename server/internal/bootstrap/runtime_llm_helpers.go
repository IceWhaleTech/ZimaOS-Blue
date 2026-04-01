package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// routesStartTime records when the server started, used for uptime calculation
var routesStartTime = timeutil.NowTime()

func resolveMCPWorkspaceRoot(dataDir string, appCfg *config.Config) string {
	return ResolveWorkspaceDir(dataDir, appCfg)
}

type proxyBridgeLLMCaller struct {
	bridge       *proxybridge.Bridge
	providerPool *providerpool.Pool
	retry        llmChatRetryConfig
}

func (c *proxyBridgeLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c == nil || c.bridge == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	retry := c.retry.withDefaults()
	return retry.chat(ctx, req, func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		req.Model = resolveDefaultRuntimeModel(req.Model, c.providerPool)
		return c.bridge.Chat(ctx, req)
	})
}

type providerRegistryLLMCaller struct {
	registry *llm.ProviderRegistry
	retry    llmChatRetryConfig
}

func newProviderRegistryLLMCaller(registry *llm.ProviderRegistry) *providerRegistryLLMCaller {
	return &providerRegistryLLMCaller{
		registry: registry,
		retry:    defaultLLMChatRetryConfig(),
	}
}

func (c *providerRegistryLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c == nil || c.registry == nil {
		return nil, fmt.Errorf("no llm provider registry configured")
	}

	names := c.registry.List()
	if len(names) == 0 {
		return nil, fmt.Errorf("no llm providers configured")
	}

	model := strings.TrimSpace(req.Model)
	retry := c.retry.withDefaults()
	if model != "" && !strings.EqualFold(model, "auto") {
		for _, name := range names {
			provider := c.registry.Get(name)
			if provider == nil {
				continue
			}
			if providerSupportsModel(provider, model) {
				return retry.chat(ctx, req, provider.Chat)
			}
		}
	}

	var provider llm.Provider
	for _, name := range names {
		if p := c.registry.Get(name); p != nil {
			provider = p
			break
		}
	}
	if provider == nil {
		return nil, fmt.Errorf("no llm providers configured")
	}

	if model == "" || strings.EqualFold(model, "auto") {
		if models := provider.Models(); len(models) > 0 && strings.TrimSpace(models[0]) != "" {
			req.Model = strings.TrimSpace(models[0])
		}
	}

	return retry.chat(ctx, req, provider.Chat)
}

func providerSupportsModel(provider llm.Provider, model string) bool {
	if provider == nil {
		return false
	}
	target := strings.TrimSpace(model)
	if target == "" {
		return false
	}
	for _, m := range provider.Models() {
		if strings.EqualFold(strings.TrimSpace(m), target) {
			return true
		}
	}
	return false
}
