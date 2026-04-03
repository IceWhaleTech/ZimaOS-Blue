package bootstrap

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type mgmtSettingsAdapter struct {
	handler *server.SettingsHandler
}

func (a *mgmtSettingsAdapter) GetAll(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"locale":                               a.handler.GetLocale(),
		"skill_selector_mode":                  a.handler.GetSkillSelectorMode(),
		"skill_rerank_enabled":                 a.handler.GetSkillRerankEnabled(),
		"skill_rerank_model":                   a.handler.GetSkillRerankModel(),
		"skill_rerank_onnx_enabled":            a.handler.GetSkillRerankONNXEnabled(),
		"skill_rerank_onnx_auto_download":      a.handler.GetSkillRerankONNXAutoDownload(),
		"skill_selector_confidence_threshold":  a.handler.GetSkillSelectorConfidenceThreshold(),
		"im_history_limit":                     a.handler.GetIMHistoryLimit(),
		"agent_mode":                           a.handler.GetAgentMode(),
		"agent_auto_reflect":                   a.handler.GetAgentAutoReflect(),
		"agent_auto_confirm":                   a.handler.GetAgentAutoConfirm(),
		"small_model_enabled":                  a.handler.GetSmallModelEnabled(),
		"small_model_runtime":                  a.handler.GetSmallModelRuntime(),
		"small_model_id":                       a.handler.GetSmallModelID(),
		"small_model_auto_download":            a.handler.GetSmallModelAutoDownload(),
		"small_model_summary_enabled":          a.handler.GetSmallModelSummaryEnabled(),
		"context_compression_mode":             a.handler.GetContextCompressionMode(),
		"small_model_context_compress_enabled": a.handler.GetSmallModelContextCompressEnabled(),
		"small_model_doc_extract_enabled":      a.handler.GetSmallModelDocExtractEnabled(),
		"small_model_rerank_enabled":           a.handler.GetSmallModelRerankEnabled(),
		"small_model_context_prune_enabled":    a.handler.GetSmallModelContextPruneEnabled(),
		"small_model_context_prune_tool_allow": a.handler.GetSmallModelContextPruneToolAllow(),
		"small_model_context_prune_tool_deny":  a.handler.GetSmallModelContextPruneToolDeny(),
		"small_model_media_intent_enabled":     a.handler.GetSmallModelMediaIntentEnabled(),
		"offline_ir_fallback_enabled":          a.handler.GetOfflineIRFallbackEnabled(),
		"feature_intent_ir_enabled":            a.handler.GetFeatureIntentIREnabled(),
		"small_model_route_image_qa_enabled":   a.handler.GetSmallModelRouteImageQAEnabled(),
		"small_model_route_short_qa_enabled":   a.handler.GetSmallModelRouteShortQAEnabled(),
		"no_llm_degrade_mode":                  a.handler.GetNoLLMDegradeMode(),
		"small_model_unavailable_policy":       a.handler.GetSmallModelUnavailablePolicy(),
	}, nil
}

func (a *mgmtSettingsAdapter) Set(_ context.Context, key, value string) error {
	switch key {
	case "locale", "timezone", "skill_selector_mode", "skill_rerank_enabled", "skill_rerank_model", "skill_rerank_onnx_enabled", "skill_rerank_onnx_auto_download", "skill_selector_confidence_threshold", "im_history_limit", "agent_mode", "agent_auto_reflect", "agent_auto_confirm", "small_model_enabled", "small_model_runtime", "small_model_id", "small_model_auto_download", "small_model_summary_enabled", "context_compression_mode", "small_model_context_compress_enabled", "small_model_doc_extract_enabled", "small_model_rerank_enabled", "small_model_context_prune_enabled", "small_model_context_prune_tool_allow", "small_model_context_prune_tool_deny", "small_model_media_intent_enabled", "offline_ir_fallback_enabled", "feature_intent_ir_enabled", "small_model_route_image_qa_enabled", "small_model_route_short_qa_enabled", "no_llm_degrade_mode", "small_model_unavailable_policy":
	default:
		return fmt.Errorf("unknown setting key: %s (valid: locale, timezone, skill_selector_mode, skill_rerank_enabled, skill_rerank_model, skill_rerank_onnx_enabled, skill_rerank_onnx_auto_download, skill_selector_confidence_threshold, im_history_limit, agent_mode, agent_auto_reflect, agent_auto_confirm, small_model_enabled, small_model_runtime, small_model_id, small_model_auto_download, small_model_summary_enabled, context_compression_mode, small_model_context_compress_enabled, small_model_doc_extract_enabled, small_model_rerank_enabled, small_model_context_prune_enabled, small_model_context_prune_tool_allow, small_model_context_prune_tool_deny, small_model_media_intent_enabled, offline_ir_fallback_enabled, feature_intent_ir_enabled, small_model_route_image_qa_enabled, small_model_route_short_qa_enabled, no_llm_degrade_mode, small_model_unavailable_policy)", key)
	}
	return fmt.Errorf("settings.set via tool not yet wired — use the web UI or PATCH /api/settings with {%q: %q}", key, value)
}

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
