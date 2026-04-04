package bootstrap

import (
	"context"
	"log/slog"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func runtimeProxyWarmupBaseURLs(providers []*providerpool.Provider) []string {
	if len(providers) == 0 {
		return nil
	}
	urls := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		baseURL := strings.TrimSpace(provider.BaseURL)
		if baseURL == "" {
			continue
		}
		urls = append(urls, baseURL)
	}
	return urls
}

func bindRuntimeProxyProviderBindings(options runtimeProxyProviderBindingsOptions) bool {
	if options.handler == nil {
		return false
	}

	if options.providerPool != nil {
		options.handler.SetProviderPool(options.providerPool)
	}
	if options.oauthManager != nil {
		options.handler.SetOAuthManager(options.oauthManager)
	}
	if options.apiKeys != nil {
		options.handler.SetAPIKeyValidator(func(key string) ([]string, error) {
			info, err := options.apiKeys.ValidateKey(context.Background(), key)
			if err != nil {
				return nil, err
			}
			return info.Scopes, nil
		})
	}

	if options.router != nil {
		options.router.SetFailoverCallback(func(result *providerpool.FailoverResult) {
			if options.smartFailover != nil {
				options.smartFailover.GetMetrics().RecordProviderPoolResult(result)
			}
			if options.pipelineStats != nil {
				options.pipelineStats.OnFailover(result)
			}
			if result != nil && (len(result.FailedAttempts) > 0 || result.SuccessProvider == "" || result.TotalAttempts > 1) {
				slog.Info("[router] failover summary",
					"request_id", result.RequestID,
					"attempts", result.TotalAttempts,
					"trace", result.Summary(),
					"success_provider", result.SuccessProvider,
					"success_model", result.SuccessModel,
					"final_error", result.FinalError,
					"duration_ms", result.EndTime.Sub(result.StartTime).Milliseconds(),
				)
			}
		})
	}

	if options.registry != nil && options.broker != nil {
		options.registry.SetOnStatusChange(func(providerID string, oldStatus, newStatus providerpool.ProviderStatus) {
			options.broker.Broadcast("provider_status_changed", map[string]string{
				"provider_id": providerID,
				"old_status":  string(oldStatus),
				"status":      string(newStatus),
			})
		})
	}

	if options.registry != nil && options.connPool != nil {
		urls := runtimeProxyWarmupBaseURLs(options.registry.ListEnabled())
		if len(urls) > 0 {
			go proxy.NewConnWarmup(options.connPool).WarmProviders(urls)
		}
	}

	return true
}
