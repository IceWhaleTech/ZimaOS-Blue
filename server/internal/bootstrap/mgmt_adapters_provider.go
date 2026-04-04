package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type mgmtProviderAdapter struct {
	pool *providerpool.Pool
}

func (a *mgmtProviderAdapter) ListProviders(_ context.Context) ([]tools.AdminProviderInfo, error) {
	providers := a.pool.Registry.List()
	result := make([]tools.AdminProviderInfo, 0, len(providers))
	for _, p := range providers {
		models := make([]string, 0)
		if a.pool.Discovery != nil {
			if ms, err := a.pool.Discovery.GetModels(p.ID); err == nil && ms != nil {
				for _, m := range ms {
					models = append(models, m.ID)
				}
			}
		}
		keys := make([]tools.AdminProviderKey, 0, len(p.APIKeys))
		for _, k := range p.APIKeys {
			keys = append(keys, tools.AdminProviderKey{
				ID:      k.ID,
				KeyHash: k.KeyHash,
				Label:   k.Label,
				Enabled: k.Enabled,
			})
		}
		result = append(result, tools.AdminProviderInfo{
			ID:       p.ID,
			Name:     p.Name,
			Type:     string(p.Type),
			Location: string(p.Location),
			BaseURL:  p.BaseURL,
			Enabled:  p.Enabled,
			Models:   models,
			Priority: p.Priority,
			APIKeys:  keys,
		})
	}
	return result, nil
}

func (a *mgmtProviderAdapter) AddProvider(_ context.Context, name, providerType, baseURL, apiKey, location string) (*tools.AdminProviderInfo, error) {
	var existing *providerpool.Provider
	for _, p := range a.pool.Registry.List() {
		if strings.EqualFold(p.Name, name) || strings.EqualFold(p.ID, name) {
			existing = p
			break
		}
	}

	if existing != nil {
		if apiKey != "" {
			newKey := &providerpool.APIKey{Key: apiKey, Enabled: true}
			if err := a.pool.Registry.AddAPIKey(existing.ID, newKey); err != nil {
				return nil, err
			}
		}
		if !existing.Enabled {
			_ = a.pool.Registry.Enable(existing.ID)
		}
		updated, err := a.pool.Registry.Get(existing.ID)
		if err != nil {
			return nil, err
		}
		return a.providerToInfo(updated), nil
	}

	loc := providerpool.ProviderLocationCloud
	if location == "local" {
		loc = providerpool.ProviderLocationLocal
	}
	p := &providerpool.Provider{
		ID:       fmt.Sprintf("custom-%s-%d", name, timeutil.NowTime().UnixMilli()),
		Name:     name,
		Type:     providerpool.ProviderType(providerType),
		Location: loc,
		BaseURL:  baseURL,
		Enabled:  true,
		Status:   providerpool.ProviderStatusActive,
		Priority: 50,
	}
	if apiKey != "" {
		p.APIKeys = []providerpool.APIKey{{
			ID:  "key-1",
			Key: apiKey,
		}}
	}
	if err := a.pool.Registry.Register(p); err != nil {
		return nil, err
	}
	return a.providerToInfo(p), nil
}

func (a *mgmtProviderAdapter) AddKey(_ context.Context, providerID, apiKey string) (*tools.AdminProviderInfo, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	newKey := &providerpool.APIKey{Key: apiKey, Enabled: true}
	if err := a.pool.Registry.AddAPIKey(providerID, newKey); err != nil {
		return nil, err
	}
	_ = a.pool.Registry.Enable(providerID)
	updated, err := a.pool.Registry.Get(providerID)
	if err != nil {
		return nil, err
	}
	return a.providerToInfo(updated), nil
}

func (a *mgmtProviderAdapter) providerToInfo(p *providerpool.Provider) *tools.AdminProviderInfo {
	keys := make([]tools.AdminProviderKey, 0, len(p.APIKeys))
	for _, k := range p.APIKeys {
		keys = append(keys, tools.AdminProviderKey{
			ID:      k.ID,
			KeyHash: k.KeyHash,
			Label:   k.Label,
			Enabled: k.Enabled,
		})
	}
	return &tools.AdminProviderInfo{
		ID:       p.ID,
		Name:     p.Name,
		Type:     string(p.Type),
		Location: string(p.Location),
		BaseURL:  p.BaseURL,
		Enabled:  p.Enabled,
		APIKeys:  keys,
	}
}

func (a *mgmtProviderAdapter) RemoveProvider(_ context.Context, id string) error {
	return a.pool.Registry.Unregister(id)
}

func (a *mgmtProviderAdapter) EnableProvider(_ context.Context, id string) error {
	return a.pool.Registry.Enable(id)
}

func (a *mgmtProviderAdapter) DisableProvider(_ context.Context, id string) error {
	return a.pool.Registry.Disable(id)
}

func (a *mgmtProviderAdapter) ListModels(_ context.Context) ([]map[string]interface{}, error) {
	if a.pool.Discovery == nil {
		return nil, fmt.Errorf("model discovery not available")
	}
	providers := a.pool.Registry.ListEnabled()
	var result []map[string]interface{}
	for _, p := range providers {
		models, err := a.pool.Discovery.GetModels(p.ID)
		if err != nil {
			continue
		}
		for _, m := range models {
			result = append(result, map[string]interface{}{
				"id":          m.ID,
				"name":        m.Name,
				"provider_id": p.ID,
				"provider":    p.Name,
			})
		}
	}
	return result, nil
}
