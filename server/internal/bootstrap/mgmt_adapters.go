package bootstrap

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sysinfo"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

// --- Provider adapter ---

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
	// Try to find an existing provider by name (case-insensitive) or ID
	var existing *providerpool.Provider
	for _, p := range a.pool.Registry.List() {
		if strings.EqualFold(p.Name, name) || strings.EqualFold(p.ID, name) {
			existing = p
			break
		}
	}

	if existing != nil {
		// Found existing provider — just add the API key
		if apiKey != "" {
			newKey := &providerpool.APIKey{Key: apiKey, Enabled: true}
			if err := a.pool.Registry.AddAPIKey(existing.ID, newKey); err != nil {
				return nil, err
			}
		}
		// Enable if it was disabled
		if !existing.Enabled {
			_ = a.pool.Registry.Enable(existing.ID)
		}
		updated, err := a.pool.Registry.Get(existing.ID)
		if err != nil {
			return nil, err
		}
		return a.providerToInfo(updated), nil
	}

	// No existing provider found — create a new custom one
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

func (a *mgmtProviderAdapter) TestProvider(_ context.Context, id string) (map[string]interface{}, error) {
	p, err := a.pool.Registry.Get(id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":       p.ID,
		"name":     p.Name,
		"enabled":  p.Enabled,
		"status":   string(p.Status),
		"base_url": p.BaseURL,
		"type":     string(p.Type),
	}, nil
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

// --- Settings adapter ---

type mgmtSettingsAdapter struct {
	handler *server.SettingsHandler
}

func (a *mgmtSettingsAdapter) GetAll(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"locale":                                  a.handler.GetLocale(),
		"smart_skill_selection":                   a.handler.GetSmartSkillSelection(),
		"skill_selector_mode":                     a.handler.GetSkillSelectorMode(),
		"skill_rerank_enabled":                    a.handler.GetSkillRerankEnabled(),
		"skill_rerank_model":                      a.handler.GetSkillRerankModel(),
		"skill_rerank_onnx_enabled":               a.handler.GetSkillRerankONNXEnabled(),
		"skill_rerank_onnx_auto_download":         a.handler.GetSkillRerankONNXAutoDownload(),
		"skill_selector_confidence_threshold":     a.handler.GetSkillSelectorConfidenceThreshold(),
		"im_history_limit":                        a.handler.GetIMHistoryLimit(),
		"agent_mode":                              a.handler.GetAgentMode(),
		"agent_auto_reflect":                      a.handler.GetAgentAutoReflect(),
		"agent_auto_confirm":                      a.handler.GetAgentAutoConfirm(),
		"small_model_enabled":                     a.handler.GetSmallModelEnabled(),
		"small_model_runtime":                     a.handler.GetSmallModelRuntime(),
		"small_model_id":                          a.handler.GetSmallModelID(),
		"small_model_auto_download":               a.handler.GetSmallModelAutoDownload(),
		"small_model_summary_enabled":             a.handler.GetSmallModelSummaryEnabled(),
		"context_compression_mode":                a.handler.GetContextCompressionMode(),
		"small_model_context_compress_enabled":    a.handler.GetSmallModelContextCompressEnabled(),
		"small_model_doc_extract_enabled":         a.handler.GetSmallModelDocExtractEnabled(),
		"small_model_rerank_enabled":              a.handler.GetSmallModelRerankEnabled(),
		"small_model_context_prune_enabled":       a.handler.GetSmallModelContextPruneEnabled(),
		"small_model_context_prune_tool_allow":    a.handler.GetSmallModelContextPruneToolAllow(),
		"small_model_context_prune_tool_deny":     a.handler.GetSmallModelContextPruneToolDeny(),
		"small_model_media_intent_enabled":        a.handler.GetSmallModelMediaIntentEnabled(),
		"offline_ir_fallback_enabled":             a.handler.GetOfflineIRFallbackEnabled(),
		"feature_intent_ir_enabled":               a.handler.GetFeatureIntentIREnabled(),
		"small_model_route_image_qa_enabled":      a.handler.GetSmallModelRouteImageQAEnabled(),
		"small_model_route_short_qa_enabled":      a.handler.GetSmallModelRouteShortQAEnabled(),
		"no_llm_degrade_mode":                     a.handler.GetNoLLMDegradeMode(),
		"small_model_unavailable_policy":          a.handler.GetSmallModelUnavailablePolicy(),
	}, nil
}

func (a *mgmtSettingsAdapter) Set(_ context.Context, key, value string) error {
	// Delegate to the settings handler's internal logic
	switch key {
	case "locale", "timezone", "smart_skill_selection", "skill_selector_mode", "skill_rerank_enabled", "skill_rerank_model", "skill_rerank_onnx_enabled", "skill_rerank_onnx_auto_download", "skill_selector_confidence_threshold", "im_history_limit", "agent_mode", "agent_auto_reflect", "agent_auto_confirm", "small_model_enabled", "small_model_runtime", "small_model_id", "small_model_auto_download", "small_model_summary_enabled", "context_compression_mode", "small_model_context_compress_enabled", "small_model_doc_extract_enabled", "small_model_rerank_enabled", "small_model_context_prune_enabled", "small_model_context_prune_tool_allow", "small_model_context_prune_tool_deny", "small_model_media_intent_enabled", "offline_ir_fallback_enabled", "feature_intent_ir_enabled", "small_model_route_image_qa_enabled", "small_model_route_short_qa_enabled", "no_llm_degrade_mode", "small_model_unavailable_policy":
		// Valid keys — handled below
	default:
		return fmt.Errorf("unknown setting key: %s (valid: locale, timezone, smart_skill_selection, skill_selector_mode, skill_rerank_enabled, skill_rerank_model, skill_rerank_onnx_enabled, skill_rerank_onnx_auto_download, skill_selector_confidence_threshold, im_history_limit, agent_mode, agent_auto_reflect, agent_auto_confirm, small_model_enabled, small_model_runtime, small_model_id, small_model_auto_download, small_model_summary_enabled, context_compression_mode, small_model_context_compress_enabled, small_model_doc_extract_enabled, small_model_rerank_enabled, small_model_context_prune_enabled, small_model_context_prune_tool_allow, small_model_context_prune_tool_deny, small_model_media_intent_enabled, offline_ir_fallback_enabled, feature_intent_ir_enabled, small_model_route_image_qa_enabled, small_model_route_short_qa_enabled, no_llm_degrade_mode, small_model_unavailable_policy)", key)
	}
	// We can't easily call Patch without an echo.Context, so we expose a direct setter.
	// For now, return a helpful message.
	return fmt.Errorf("settings.set via tool not yet wired — use the web UI or PATCH /api/settings with {%q: %q}", key, value)
}

// --- Channel adapter ---

type mgmtChannelAdapter struct {
	mgr *channel.Manager
}

func (a *mgmtChannelAdapter) ListChannels(_ context.Context) ([]tools.AdminChannelInfo, error) {
	if a.mgr == nil {
		return nil, nil
	}
	channels := a.mgr.List()
	result := make([]tools.AdminChannelInfo, 0, len(channels))
	for _, ch := range channels {
		result = append(result, tools.AdminChannelInfo{
			Name:   ch.Name,
			Type:   ch.Type,
			Status: string(ch.Status),
		})
	}
	return result, nil
}

func (a *mgmtChannelAdapter) GetStatus(_ context.Context, name string) (*tools.AdminChannelInfo, error) {
	if a.mgr == nil {
		return nil, fmt.Errorf("channel manager not available")
	}
	ch, ok := a.mgr.Get(name)
	if !ok {
		return nil, fmt.Errorf("channel %q not found", name)
	}
	info := ch.Info()
	return &tools.AdminChannelInfo{
		Name:   info.Name,
		Type:   info.Type,
		Status: string(info.Status),
	}, nil
}

// --- Skill adapter ---

type mgmtSkillAdapter struct {
	registry *skill.Registry
}

func (a *mgmtSkillAdapter) ListSkills(_ context.Context) ([]tools.AdminSkillInfo, error) {
	skills := a.registry.List()
	result := make([]tools.AdminSkillInfo, 0, len(skills))
	for _, s := range skills {
		result = append(result, tools.AdminSkillInfo{
			ID:       s.Manifest.ID,
			Name:     s.Manifest.Name,
			Category: s.Manifest.Category,
			Enabled:  s.Enabled,
			Builtin:  s.Builtin,
		})
	}
	return result, nil
}

func (a *mgmtSkillAdapter) EnableSkill(_ context.Context, id string) error {
	return a.registry.Enable(id)
}

func (a *mgmtSkillAdapter) DisableSkill(_ context.Context, id string) error {
	return a.registry.Disable(id)
}

// --- Tool adapter ---

type mgmtToolAdapter struct {
	registry *tools.Registry
}

func (a *mgmtToolAdapter) ListTools(_ context.Context) ([]tools.AdminToolInfo, error) {
	// Active tools
	names := a.registry.List()
	result := make([]tools.AdminToolInfo, 0, len(names))
	for _, name := range names {
		t := a.registry.Get(name)
		if t == nil {
			continue
		}
		def := t.Definition()
		result = append(result, tools.AdminToolInfo{
			Name:                def.Name,
			Description:         def.Description,
			RiskLevel:           def.RiskLevel,
			VisibilityAllowlist: append([]string(nil), def.VisibilityAllowlist...),
		})
	}
	// Disabled tools
	for _, name := range a.registry.ListDisabled() {
		t := a.registry.Get(name)
		if t == nil {
			continue
		}
		def := t.Definition()
		result = append(result, tools.AdminToolInfo{
			Name:                def.Name,
			Description:         def.Description,
			Disabled:            true,
			RiskLevel:           def.RiskLevel,
			VisibilityAllowlist: append([]string(nil), def.VisibilityAllowlist...),
		})
	}
	return result, nil
}

func (a *mgmtToolAdapter) EnableTool(_ context.Context, name string) error {
	if !a.registry.Enable(name) {
		return fmt.Errorf("tool %q not found or already enabled", name)
	}
	return nil
}

func (a *mgmtToolAdapter) DisableTool(_ context.Context, name string) error {
	if !a.registry.Disable(name) {
		return fmt.Errorf("tool %q not found or already disabled", name)
	}
	return nil
}

// --- System adapter ---

type mgmtSystemAdapter struct {
	version string
}

func (a *mgmtSystemAdapter) Health(_ context.Context) (*tools.AdminSystemInfo, error) {
	procMem := sysinfo.GetProcessMemInfo()
	uptime := time.Since(routesStartTime)
	return &tools.AdminSystemInfo{
		Version:       a.version,
		Uptime:        formatDuration(uptime),
		UptimeSeconds: uptime.Seconds(),
		GoVersion:     runtime.Version(),
		NumCPU:        runtime.NumCPU(),
		Goroutines:    runtime.NumGoroutine(),
		MemAllocMB:    float64(procMem.GoAlloc) / (1024 * 1024),
		MemRSSMB:      float64(procMem.RSSB) / (1024 * 1024),
	}, nil
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// --- User adapter ---

type mgmtUserAdapter struct {
	service *user.Service
}

func (a *mgmtUserAdapter) ListUsers(ctx context.Context) ([]tools.AdminUserInfo, error) {
	resp, err := a.service.List(ctx, &user.ListUsersQuery{})
	if err != nil {
		return nil, err
	}
	result := make([]tools.AdminUserInfo, 0, len(resp.Users))
	for _, u := range resp.Users {
		result = append(result, tools.AdminUserInfo{
			ID:       u.ID.String(),
			Username: u.Username,
			Role:     string(u.Role),
			Locked:   u.Status == user.StatusLocked,
		})
	}
	return result, nil
}

func (a *mgmtUserAdapter) LockUser(ctx context.Context, id string) error {
	uid, err := parseUserUUID(id)
	if err != nil {
		return err
	}
	return a.service.Lock(ctx, uid)
}

func (a *mgmtUserAdapter) UnlockUser(ctx context.Context, id string) error {
	uid, err := parseUserUUID(id)
	if err != nil {
		return err
	}
	return a.service.Unlock(ctx, uid)
}

// --- API Key adapter ---

type mgmtAPIKeyAdapter struct {
	service *auth.APIKeyService
}

func (a *mgmtAPIKeyAdapter) ListKeys(ctx context.Context) ([]tools.AdminAPIKeyInfo, error) {
	userID := tools.GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}
	keys, err := a.service.ListKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]tools.AdminAPIKeyInfo, 0, len(keys))
	for _, k := range keys {
		result = append(result, tools.AdminAPIKeyInfo{
			ID:        k.ID,
			Name:      k.Name,
			Prefix:    k.Prefix,
			CreatedAt: k.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (a *mgmtAPIKeyAdapter) CreateKey(ctx context.Context, name string) (*tools.AdminAPIKeyCreateResult, error) {
	userID := tools.GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}
	info, err := a.service.CreateKey(ctx, &auth.CreateKeyRequest{
		Name:   name,
		UserID: userID,
		Scopes: []string{"chat", "api"},
	})
	if err != nil {
		return nil, err
	}
	return &tools.AdminAPIKeyCreateResult{
		ID:     info.ID,
		Name:   info.Name,
		Key:    info.Key,
		Prefix: info.Prefix,
	}, nil
}

func (a *mgmtAPIKeyAdapter) RevokeKey(ctx context.Context, id string) error {
	userID := tools.GetUserID(ctx)
	if userID == "" {
		userID = "default"
	}
	return a.service.RevokeKey(ctx, id, userID)
}

// --- Helpers ---

func parseUserUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid UUID: %s", s)
	}
	return id, nil
}

// --- Upgrade / OTA adapter ---

type mgmtUpgradeAdapter struct {
	handler    *update.Handler
	otaChecker *update.OTAChecker
	version    string
}

func (a *mgmtUpgradeAdapter) GetOTAStatus(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.otaChecker == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion:  a.version,
			UpdateAvailable: false,
		}, nil
	}

	latest := a.otaChecker.GetLatest()
	available := latest != nil && latest.Version != "" && update.IsNewerVersionString(a.version, latest.Version)

	info := &tools.AdminUpgradeInfo{
		CurrentVersion:  a.version,
		UpdateAvailable: available,
	}
	if latest != nil {
		info.LatestVersion = latest.Version
		info.DownloadURLs = latest.Packages
		info.ReleaseNoteURL = latest.ReleaseNoteURL
		if latest.Delay > 0 {
			info.Delay = latest.Delay
		}
	}
	return info, nil
}

func (a *mgmtUpgradeAdapter) CheckForUpdate(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion:  a.version,
			UpdateAvailable: false,
		}, nil
	}

	updateInfo, err := a.handler.CheckForUpdate(ctx)
	if err != nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion:  a.version,
			UpdateAvailable: false,
		}, nil
	}

	info := &tools.AdminUpgradeInfo{
		CurrentVersion:  a.version,
		UpdateAvailable: updateInfo.UpdateAvailable,
	}
	if updateInfo.UpdateAvailable {
		info.LatestVersion = updateInfo.LatestVersion
		if updateInfo.DownloadURL != "" {
			info.DownloadURLs = []string{updateInfo.DownloadURL}
		}
	}
	return info, nil
}

func (a *mgmtUpgradeAdapter) GetUpdateStatus(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          "idle",
		}, nil
	}

	status := a.handler.GetStatus()
	info := &tools.AdminUpgradeInfo{
		CurrentVersion: a.version,
		State:          status.State,
		Progress:       status.Progress,
		Error:          status.Error,
		DownloadedPath: status.DownloadedPath,
	}
	return info, nil
}

func (a *mgmtUpgradeAdapter) StartDownload(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			Error:          "update handler not available",
		}, nil
	}

	// Get current status first
	status := a.handler.GetStatus()
	if status.State == "downloading" || status.State == "applying" {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Progress:       status.Progress,
			Error:          "update already in progress",
		}, nil
	}

	// Start download in background
	if err := a.handler.StartDownload(ctx); err != nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Error:          err.Error(),
		}, nil
	}

	return &tools.AdminUpgradeInfo{
		CurrentVersion: a.version,
		State:          "downloading",
		Progress:       0,
	}, nil
}

func (a *mgmtUpgradeAdapter) ApplyUpdate(ctx context.Context) (*tools.AdminUpgradeInfo, error) {
	if a.handler == nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			Error:          "update handler not available",
		}, nil
	}

	// Check if update is downloaded
	status := a.handler.GetStatus()
	if status.DownloadedPath == "" {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Error:          "no update downloaded. use upgrade.download first",
		}, nil
	}

	if status.State == "applying" || status.State == "restarting" {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          status.State,
			Error:          "update already in progress",
		}, nil
	}

	// Apply update in background
	if err := a.handler.ApplyUpdate(); err != nil {
		return &tools.AdminUpgradeInfo{
			CurrentVersion: a.version,
			State:          "failed",
			Error:          err.Error(),
		}, nil
	}

	return &tools.AdminUpgradeInfo{
		CurrentVersion: a.version,
		State:          "applying",
		Progress:       100,
	}, nil
}
