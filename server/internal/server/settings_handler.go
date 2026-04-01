package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voicewake"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

const settingsKVKey = "config:settings"
const defaultIMHistoryLimit = 3
const defaultDirectoryWhitelistPath = "/tmp"

var removedSettingsKeys = map[string]struct{}{
	"smart_tool_selection":                    {},
	"small_model_route_tool_dispatch_enabled": {},
}

var (
	errSelectorDryRunQueryRequired = errors.New("query is required")
	errSelectorDryRunUnavailable   = errors.New("chat handler not configured")
)

// SettingsHandler handles user settings API endpoints
type SettingsHandler struct {
	mu                        sync.RWMutex
	kv                        kvstore.Store
	settings                  *Settings
	skillRerankerModelManager *agentcore.SkillRerankerModelManager
	smallModelManager         *smallmodel.Manager
	chatHandler               *ChatHandler
	voiceWakeManager          *voicewake.Manager
	skillAdvisor              *skilladvisor.Service
}

type DirectoryWhitelistEntry struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
}

// Settings represents user preferences stored on backend
type Settings struct {
	Locale                              string                    `json:"locale,omitempty"`                                    // User's preferred locale (e.g., "zh-CN", "en-US")
	Timezone                            string                    `json:"timezone,omitempty"`                                  // User's timezone
	SmartSkillSelection                 *bool                     `json:"smart_skill_selection,omitempty"`                     // Progressive skill selector (nil = default false)
	SkillSelectorMode                   string                    `json:"skill_selector_mode,omitempty"`                       // hybrid|ir_only|llm_only
	SkillRerankEnabled                  *bool                     `json:"skill_rerank_enabled,omitempty"`                      // Enable stage-2 rerank (nil = default false)
	SkillRerankModel                    string                    `json:"skill_rerank_model,omitempty"`                        // Reranker model repo (e.g. cross-encoder/ms-marco-MiniLM-L6-v2)
	SkillRerankONNXEnabled              *bool                     `json:"skill_rerank_onnx_enabled,omitempty"`                 // Enable ONNX reranker path (nil = default false)
	SkillRerankONNXAutoDownload         *bool                     `json:"skill_rerank_onnx_auto_download,omitempty"`           // Allow ONNX model auto-download (nil = default false)
	SkillSelectorConfidenceThreshold    *float64                  `json:"skill_selector_confidence_threshold,omitempty"`       // default 0.78
	SkillDynamicExposure                *bool                     `json:"skill_dynamic_exposure,omitempty"`                    // Enable runtime nested discovery + path activation (nil = default false)
	PromptPolicyVersion                 string                    `json:"prompt_policy_version,omitempty"`                     // prompt policy version marker
	PromptPolicyProfile                 string                    `json:"prompt_policy_profile,omitempty"`                     // prompt policy profile
	MemoryRecallMode                    string                    `json:"memory_recall_mode,omitempty"`                        // Memory recall strategy: aggressive|balanced|quality
	IMHistoryLimit                      *int                      `json:"im_history_limit,omitempty"`                          // IM no-history tier keeps recent rounds (default 3, 0 disables)
	AgentMode                           *bool                     `json:"agent_mode,omitempty"`                                // Autonomous agent mode (nil = default false)
	AgentAutoReflect                    *bool                     `json:"agent_auto_reflect,omitempty"`                        // Run post-task reflection in agent mode (nil = default true)
	AgentAutoConfirm                    *bool                     `json:"agent_auto_confirm,omitempty"`                        // Skip confirmation in agent mode (nil = default false)
	AgentAskTimeoutSeconds              *int                      `json:"agent_ask_timeout_seconds,omitempty"`                 // Ask timeout in seconds (default 120, range 15-1800)
	AgentAskTimeoutAction               string                    `json:"agent_ask_timeout_action,omitempty"`                  // default|error
	AgentLoopPolicyMaxToolRounds        *int                      `json:"agent_loop_policy_max_tool_rounds,omitempty"`         // default maxToolRoundsAgent
	AgentLoopPolicyMaxAutoContinue      *int                      `json:"agent_loop_policy_max_auto_continue,omitempty"`       // default maxAutoContinueAgent
	AgentLoopPolicyPseudoToolCallBudget *int                      `json:"agent_loop_policy_pseudo_tool_call_budget,omitempty"` // default maxPseudoToolCallAutoContinueAgent
	AgentLoopPolicyActionPledgeBudget   *int                      `json:"agent_loop_policy_action_pledge_budget,omitempty"`    // default maxActionPledgeAutoContinueAgent
	AgentLoopPolicyMissingTodoBudget    *int                      `json:"agent_loop_policy_missing_todo_budget,omitempty"`     // default maxMissingTodoAutoContinueAgent
	AgentLoopPolicyPendingTodoBudget    *int                      `json:"agent_loop_policy_pending_todo_budget,omitempty"`     // default maxPendingTodoAutoContinueAgent
	SmallModelEnabled                   *bool                     `json:"small_model_enabled,omitempty"`                       // default false
	SmallModelRuntime                   string                    `json:"small_model_runtime,omitempty"`                       // fixed: llama.cpp
	SmallModelID                        string                    `json:"small_model_id,omitempty"`                            // fixed: qwen3.5-0.8b-gguf-q4km
	SmallModelAutoDownload              *bool                     `json:"small_model_auto_download,omitempty"`                 // default true
	SmallModelSummaryEnabled            *bool                     `json:"small_model_summary_enabled,omitempty"`               // default false
	ContextCompressionMode              string                    `json:"context_compression_mode,omitempty"`                  // offline|small_model|auto (legacy "off" coerces to auto)
	SmallModelContextCompressEnabled    *bool                     `json:"small_model_context_compress_enabled,omitempty"`      // default false
	SmallModelDocExtractEnabled         *bool                     `json:"small_model_doc_extract_enabled,omitempty"`           // default false
	SmallModelRerankEnabled             *bool                     `json:"small_model_rerank_enabled,omitempty"`                // default false
	SmallModelContextPruneEnabled       *bool                     `json:"small_model_context_prune_enabled,omitempty"`         // default false
	SmallModelContextPruneToolAllow     []string                  `json:"small_model_context_prune_tool_allow,omitempty"`      // glob allow list for prunable tools
	SmallModelContextPruneToolDeny      []string                  `json:"small_model_context_prune_tool_deny,omitempty"`       // glob deny list for prunable tools
	SmallModelMediaIntentEnabled        *bool                     `json:"small_model_media_intent_enabled,omitempty"`          // default false
	OfflineIRFallbackEnabled            *bool                     `json:"offline_ir_fallback_enabled,omitempty"`               // default false
	FeatureIntentIREnabled              *bool                     `json:"feature_intent_ir_enabled,omitempty"`                 // default false
	DeepResearchV2Enabled               *bool                     `json:"deep_research_v2_enabled,omitempty"`                  // default false
	SmallModelRouteImageQAEnabled       *bool                     `json:"small_model_route_image_qa_enabled,omitempty"`        // default inherits short-qa
	SmallModelRouteShortQAEnabled       *bool                     `json:"small_model_route_short_qa_enabled,omitempty"`        // default false
	NoLLMDegradeMode                    string                    `json:"no_llm_degrade_mode,omitempty"`                       // fixed default deepresearch
	SmallModelUnavailablePolicy         string                    `json:"small_model_unavailable_policy,omitempty"`            // default ir_first
	DirectoryWhitelistEnabled           *bool                     `json:"directory_whitelist_enabled,omitempty"`               // default true with /tmp on non-Windows when unset
	DirectoryWhitelist                  []DirectoryWhitelistEntry `json:"directory_whitelist,omitempty"`                       // optional extra tool roots outside workspace; defaults to [/tmp] on non-Windows when unset
	VoiceWakeEnabled                    *bool                     `json:"voice_wake_enabled,omitempty"`                        // default false
	VoiceWakeTriggers                   []string                  `json:"voice_wake_triggers,omitempty"`                       // default ["Hey Blue"]
	VoiceWakeLocale                     string                    `json:"voice_wake_locale,omitempty"`                         // optional locale override
	VoiceWakeTargetConversationID       string                    `json:"voice_wake_target_conversation_id,omitempty"`         // fixed background target conversation
}

var allowedMemoryRecallModes = map[string]struct{}{
	"aggressive": {},
	"balanced":   {},
	"quality":    {},
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(kv kvstore.Store) *SettingsHandler {
	h := &SettingsHandler{
		kv:       kv,
		settings: &Settings{},
	}
	h.load()
	return h
}

// RegisterRoutes registers settings routes
func (h *SettingsHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/settings", h.Get)
	g.PUT("/settings", h.Update)
	g.PATCH("/settings", h.Patch)
	g.GET("/settings/prompt-policy", h.GetPromptPolicyStatus)
	g.POST("/settings/selector/dry-run", h.SelectorDryRun)
	g.GET("/settings/skill-reranker/model/status", h.GetSkillRerankerModelStatus)
	g.POST("/settings/skill-reranker/model/download", h.StartSkillRerankerModelDownload)
	g.POST("/settings/skill-reranker/model/cancel", h.CancelSkillRerankerModelDownload)
	g.GET("/settings/small-model/status", h.GetSmallModelStatus)
	g.POST("/settings/small-model/download", h.StartSmallModelDownload)
	g.POST("/settings/small-model/cancel", h.CancelSmallModelDownload)
}

// SetSkillRerankerModelManager wires ONNX skill-reranker model manager for UI download APIs.
func (h *SettingsHandler) SetSkillRerankerModelManager(mgr *agentcore.SkillRerankerModelManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.skillRerankerModelManager = mgr
}

// SetSmallModelManager wires fixed small-model manager for runtime status/download APIs.
func (h *SettingsHandler) SetSmallModelManager(mgr *smallmodel.Manager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.smallModelManager = mgr
}

// SetChatHandler wires chat handler for selector dry-run endpoints.
func (h *SettingsHandler) SetChatHandler(ch *ChatHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.chatHandler = ch
}

// SetSkillAdvisor wires the optional skill advisor used by selector dry-run.
func (h *SettingsHandler) SetSkillAdvisor(advisor *skilladvisor.Service) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.skillAdvisor = advisor
}

// SetVoiceWakeManager wires the VoiceWake manager so settings changes can refresh runtime state.
func (h *SettingsHandler) SetVoiceWakeManager(manager *voicewake.Manager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.voiceWakeManager = manager
}

// Get handles GET /api/settings
func (h *SettingsHandler) Get(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return c.JSON(http.StatusOK, h.settings)
}

func decodeSettingsRequestBody(c echo.Context, target interface{}) (map[string]json.RawMessage, error) {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}
	c.Request().Body = io.NopCloser(bytes.NewReader(body))

	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func firstRemovedSettingsKey(raw map[string]json.RawMessage) string {
	for key := range raw {
		if _, removed := removedSettingsKeys[key]; removed {
			return key
		}
	}
	return ""
}

// StartSkillRerankerModelDownload starts downloading ONNX model in background.
func (h *SettingsHandler) StartSkillRerankerModelDownload(c echo.Context) error {
	h.mu.RLock()
	mgr := h.skillRerankerModelManager
	h.mu.RUnlock()
	if mgr == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "skill reranker model manager not initialized"})
	}

	go func() {
		_ = mgr.Download(context.Background())
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "download started",
	})
}

// CancelSkillRerankerModelDownload cancels current ONNX model download.
func (h *SettingsHandler) CancelSkillRerankerModelDownload(c echo.Context) error {
	h.mu.RLock()
	mgr := h.skillRerankerModelManager
	h.mu.RUnlock()
	if mgr == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "skill reranker model manager not initialized"})
	}

	mgr.CancelDownload()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// GetSkillRerankerModelStatus returns ONNX model file/download status.
func (h *SettingsHandler) GetSkillRerankerModelStatus(c echo.Context) error {
	h.mu.RLock()
	mgr := h.skillRerankerModelManager
	h.mu.RUnlock()
	if mgr == nil {
		return c.JSON(http.StatusOK, agentcore.SkillRerankerModelStatus{Ready: false})
	}
	return c.JSON(http.StatusOK, mgr.GetStatus())
}

// StartSmallModelDownload starts downloading fixed llama.cpp small model in background.
func (h *SettingsHandler) StartSmallModelDownload(c echo.Context) error {
	h.mu.RLock()
	mgr := h.smallModelManager
	h.mu.RUnlock()
	if mgr == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "small model manager not initialized"})
	}
	go func() {
		_ = mgr.Download(context.Background())
	}()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "download started",
	})
}

// CancelSmallModelDownload cancels current small-model download.
func (h *SettingsHandler) CancelSmallModelDownload(c echo.Context) error {
	h.mu.RLock()
	mgr := h.smallModelManager
	h.mu.RUnlock()
	if mgr == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "small model manager not initialized"})
	}
	mgr.CancelDownload()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// GetSmallModelStatus returns fixed small-model readiness/download status.
func (h *SettingsHandler) GetSmallModelStatus(c echo.Context) error {
	h.mu.RLock()
	mgr := h.smallModelManager
	h.mu.RUnlock()
	if mgr == nil {
		return c.JSON(http.StatusOK, smallmodel.Status{
			Ready:   false,
			ModelID: smallmodel.ModelID,
			Runtime: smallmodel.RuntimeType,
		})
	}
	return c.JSON(http.StatusOK, mgr.GetStatus())
}

// GetPromptPolicyStatus returns the effective prompt policy and loop policy snapshot.
func (h *SettingsHandler) GetPromptPolicyStatus(c echo.Context) error {
	if !isAdminRequest(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin role required"})
	}
	policy := resolvePromptPolicy(h.GetPromptPolicyVersion(), h.GetPromptPolicyProfile())
	loopPolicy := AgentLoopPolicy{
		MaxToolRounds:        h.GetAgentLoopPolicyMaxToolRounds(),
		MaxAutoContinue:      h.GetAgentLoopPolicyMaxAutoContinue(),
		PseudoToolCallBudget: h.GetAgentLoopPolicyPseudoToolCallBudget(),
		ActionPledgeBudget:   h.GetAgentLoopPolicyActionPledgeBudget(),
		MissingTodoBudget:    h.GetAgentLoopPolicyMissingTodoBudget(),
		PendingTodoBudget:    h.GetAgentLoopPolicyPendingTodoBudget(),
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"prompt_policy_version": policy.Version,
		"prompt_policy_profile": policy.Profile,
		"prompt_policy_hash":    policy.Hash,
		"agent_loop_policy":     loopPolicy,
	})
}

// SelectorDryRun previews tool/skill selection decisions for a query.
func (h *SettingsHandler) SelectorDryRun(c echo.Context) error {
	if !isAdminRequest(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin role required"})
	}
	var req struct {
		Query string `json:"query"`
		Model string `json:"model,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	response, err := h.PreviewSelectorDryRun(c.Request().Context(), req.Query, req.Model)
	if err != nil {
		switch {
		case errors.Is(err, errSelectorDryRunQueryRequired):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, errSelectorDryRunUnavailable):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
	return c.JSON(http.StatusOK, response)
}

// PreviewSelectorDryRun returns the same selector/tool routing preview payload
// used by the admin dry-run endpoint, without requiring an HTTP context.
func (h *SettingsHandler) PreviewSelectorDryRun(ctx context.Context, query string, model string) (map[string]interface{}, error) {
	query = strings.TrimSpace(query)
	model = strings.TrimSpace(model)
	if query == "" {
		return nil, errSelectorDryRunQueryRequired
	}
	if model == "" {
		model = "auto"
	}

	h.mu.RLock()
	chatHandler := h.chatHandler
	advisor := h.skillAdvisor
	h.mu.RUnlock()
	if chatHandler == nil {
		return nil, errSelectorDryRunUnavailable
	}

	policyReq := tools.ToolPolicyRequest{
		Model:     model,
		RouteKind: tools.ToolRouteKindChat,
	}
	selection := chatHandler.previewChatToolSurfacesForRequest(ctx, query, policyReq, nil, nil)
	selectedDefs := selection.RoutedDefs
	toolNames := make([]string, len(selectedDefs))
	for i, def := range selectedDefs {
		toolNames[i] = def.Name
	}
	toolSurface := tools.MeasureLLMToolSurface(selectedDefs)
	selectedNativeNames := make([]string, len(selection.NativeDefs))
	for i, def := range selection.NativeDefs {
		selectedNativeNames[i] = def.Name
	}
	selectedNativeSurface := tools.MeasureLLMToolSurface(selection.NativeDefs)
	var toolDebug *tools.ToolSelectionDebug
	if chatHandler.toolSelector != nil {
		allDefs := chatHandler.toolDefinitionsForPolicy(policyReq)
		debug := chatHandler.toolSelector.SelectDetailed(query, allDefs).Debug
		toolDebug = &debug
	}

	response := map[string]interface{}{
		"query":                        query,
		"model":                        model,
		"smart_skill_selection":        h.GetSmartSkillSelection(),
		"skill_dynamic_exposure":       h.GetSkillDynamicExposure(),
		"selected_tools":               toolNames,
		"selected_tool_surface":        toolSurface,
		"selected_native_tools":        selectedNativeNames,
		"selected_native_tool_surface": selectedNativeSurface,
		"selected_native_surface_mode": string(selection.NativeMode),
	}
	if toolDebug != nil {
		response["tool_debug"] = toolDebug
	}
	if selection.DiscoveryDecision != nil {
		copied := *selection.DiscoveryDecision
		surfaceReason := selectorDryRunDiscoverySurfaceReason(selection, h.GetSkillDynamicExposure())
		response["discovery_decision"] = copied
		if copied.CanonicalTarget != "" && copied.CanonicalTarget != agentcore.CanonicalUnknown {
			response["selected_canonical_skill"] = string(copied.CanonicalTarget)
		}
		if alias := strings.TrimSpace(copied.AliasResolved); alias != "" {
			response["selected_alias"] = alias
		}
		observation := copied.ToObservation()
		response["execution_profile"] = string(observation.ExecutionProfile)
		response["skill_exec_cutover"] = observation.SkillExecCutover
		response["forked_skill_execution"] = observation.ForkedSkillExecution
		if surfaceReason != "" {
			response["selected_native_surface_reason"] = surfaceReason
		}
		if outcome := strings.TrimSpace(observation.ClarifyOutcome); outcome != "" {
			response["clarify_outcome"] = outcome
		}
		discoveryRuntime := map[string]interface{}{
			"dynamic_exposure_enabled": h.GetSkillDynamicExposure(),
			"native_surface_mode":      string(copied.NativeSurfaceMode),
			"selected_native_mode":     string(selection.NativeMode),
			"surface_reason":           surfaceReason,
			"execution_profile":        string(observation.ExecutionProfile),
			"skill_exec_cutover":       observation.SkillExecCutover,
			"forked_skill_execution":   observation.ForkedSkillExecution,
		}
		if copied.CanonicalTarget != "" && copied.CanonicalTarget != agentcore.CanonicalUnknown {
			discoveryRuntime["canonical_target"] = string(copied.CanonicalTarget)
		}
		if alias := strings.TrimSpace(copied.AliasResolved); alias != "" {
			discoveryRuntime["selected_alias"] = alias
		}
		if entry, ok := agentcore.GetDiscoveryEntry(copied.CanonicalTarget); ok {
			response["selected_canonical_entry"] = entry
			response["selected_canonical_cutover_eligible"] = entry.CutoverEligible
			discoveryRuntime["cutover_eligible"] = entry.CutoverEligible
			discoveryRuntime["entry_kind"] = entry.Kind
			if len(entry.Aliases) > 0 {
				discoveryRuntime["aliases"] = append([]string(nil), entry.Aliases...)
			}
			if len(entry.CapabilityTags) > 0 {
				discoveryRuntime["capability_tags"] = append([]string(nil), entry.CapabilityTags...)
			}
			if len(entry.SearchHints) > 0 {
				discoveryRuntime["search_hints"] = append([]string(nil), entry.SearchHints...)
			}
		}
		response["discovery_runtime"] = discoveryRuntime
	}

	var selectedDecision *agentcore.Decision
	selectedSkillName := ""
	candidateNames := make([]string, 0, 4)
	if selection.SkillDecision != nil {
		copied := *selection.SkillDecision
		selectedDecision = &copied
		decision := copied
		selectedSkillName = strings.TrimSpace(decision.SelectedSkill)
		for _, candidate := range decision.Candidates {
			candidateNames = append(candidateNames, candidate.Name)
		}
		response["skill_decision"] = decision
		response["skill_prompt_hint"] = decision.PromptHint(3)
		response["canonical_skill_id"] = decision.SelectedSkill
		response["skill_need_clarify"] = decision.NeedClarify
		response["skill_route_outcome"] = selectorDryRunOutcome(decision)
		response["decision_reason"] = decision.Reason
		response["decision_stage"] = decision.Stage
		if strings.TrimSpace(decision.Reason) != "" {
			if decision.NeedClarify {
				response["clarify_reason"] = decision.Reason
			} else {
				response["fallback_reason"] = decision.Reason
			}
		}
	} else if chatHandler.skillSelector != nil && h.GetSmartSkillSelection() {
		opts := agentcore.SelectOptions{
			Mode:                h.GetSkillSelectorMode(),
			EnableRerank:        h.GetEffectiveSkillRerankEnabled(),
			ConfidenceThreshold: h.GetSkillSelectorConfidenceThreshold(),
		}
		decision, err := chatHandler.skillSelector.Select(ctx, query, opts)
		if err != nil {
			response["skill_selector_error"] = err.Error()
		} else {
			copied := decision
			selectedDecision = &copied
			selectedSkillName = strings.TrimSpace(decision.SelectedSkill)
			for _, candidate := range decision.Candidates {
				candidateNames = append(candidateNames, candidate.Name)
			}
			response["skill_decision"] = decision
			response["skill_prompt_hint"] = decision.PromptHint(3)
			response["canonical_skill_id"] = decision.SelectedSkill
			response["skill_need_clarify"] = decision.NeedClarify
			response["skill_route_outcome"] = selectorDryRunOutcome(decision)
			response["decision_reason"] = decision.Reason
			response["decision_stage"] = decision.Stage
			if strings.TrimSpace(decision.Reason) != "" {
				if decision.NeedClarify {
					response["clarify_reason"] = decision.Reason
				} else {
					response["fallback_reason"] = decision.Reason
				}
			}
		}
	}
	if chatHandler.skillSelector != nil {
		if debugState, err := chatHandler.skillSelector.DebugState(); err == nil {
			response["active_skill_count"] = debugState.ActiveSkillCount
			response["dormant_skill_count"] = debugState.DormantSkillCount
			response["skill_cache_invalidation_count"] = debugState.CacheInvalidationCount
			if len(debugState.DiscoveredDirs) > 0 {
				response["skill_discovered_dirs"] = debugState.DiscoveredDirs
			}
			if len(debugState.ActivatedConditionalSkills) > 0 {
				response["skill_activated_conditional_skills"] = debugState.ActivatedConditionalSkills
			}
		} else {
			response["skill_debug_error"] = err.Error()
		}

		if selectedSkillName != "" {
			if view, ok, err := chatHandler.skillSelector.LookupSkillRuntimeView(selectedSkillName); err == nil && ok {
				response["activation_state"] = view.ActivationState
				response["activation_source"] = view.ActivationSource
				response["model_invocable"] = view.ModelInvocable
				response["user_invocable"] = view.UserInvocable
				response["selected_skill_runtime"] = view
			} else if err != nil {
				response["selected_skill_runtime_error"] = err.Error()
			}
		}

		if len(candidateNames) > 0 {
			candidateRuntime := make(map[string]agentcore.SkillSelectorSkillRuntimeView, len(candidateNames))
			for _, candidate := range candidateNames {
				if view, ok, err := chatHandler.skillSelector.LookupSkillRuntimeView(candidate); err == nil && ok {
					candidateRuntime[candidate] = view
				}
			}
			if len(candidateRuntime) > 0 {
				response["candidate_runtime"] = candidateRuntime
			}
		}
	}
	if advisor != nil {
		advice, err := advisor.Advise(ctx, query, selectedDecision)
		if err != nil {
			response["skill_advice_error"] = err.Error()
		} else if advice != nil {
			response["skill_advice"] = advice
		}
	}
	return response, nil
}

func selectorDryRunOutcome(decision agentcore.Decision) string {
	switch {
	case decision.NeedClarify:
		return "clarify"
	case strings.TrimSpace(decision.SelectedSkill) != "":
		return "selected"
	default:
		return "none"
	}
}

func selectorDryRunDiscoverySurfaceReason(selection chatToolSurfaceSelection, skillDynamicExposure bool) string {
	if selection.DiscoveryDecision == nil {
		return ""
	}
	switch selection.NativeMode {
	case chatNativeToolSurfaceModeClarifyNone:
		return "clarify_required"
	case chatNativeToolSurfaceModeSkillExec:
		if skillDynamicExposure {
			return "discover_first_cutover"
		}
		return "legacy_exec_collapse_compat"
	default:
		switch {
		case selection.DiscoveryDecision.CanonicalTarget == agentcore.CanonicalUnknown:
			return "legacy_no_canonical_match"
		case selection.DiscoveryDecision.NativeSurfaceMode == agentcore.NativeSurfaceModeLegacy:
			return "legacy_non_cutover_skill"
		default:
			return "cutover_blocked_or_unavailable"
		}
	}
}

// Update handles PUT /api/settings (full update)
func (h *SettingsHandler) Update(c echo.Context) error {
	var newSettings Settings
	raw, err := decodeSettingsRequestBody(c, &newSettings)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if removed := firstRemovedSettingsKey(raw); removed != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Removed setting: " + removed})
	}
	if newSettings.MemoryRecallMode != "" {
		if _, valid := allowedMemoryRecallModes[newSettings.MemoryRecallMode]; !valid {
			newSettings.MemoryRecallMode = ""
		}
	}
	if newSettings.IMHistoryLimit != nil {
		v := *newSettings.IMHistoryLimit
		if v < 0 {
			v = 0
		}
		newSettings.IMHistoryLimit = &v
	}
	if newSettings.SkillSelectorMode != "" {
		switch newSettings.SkillSelectorMode {
		case "hybrid", "ir_only", "llm_only":
		default:
			newSettings.SkillSelectorMode = ""
		}
	}
	if newSettings.SkillSelectorConfidenceThreshold != nil {
		v := *newSettings.SkillSelectorConfidenceThreshold
		if v <= 0 || v > 1 {
			newSettings.SkillSelectorConfidenceThreshold = nil
		}
	}
	if newSettings.AgentAskTimeoutSeconds != nil {
		v := *newSettings.AgentAskTimeoutSeconds
		if v < 15 || v > 1800 {
			newSettings.AgentAskTimeoutSeconds = nil
		}
	}
	if newSettings.AgentAskTimeoutAction != "" {
		switch newSettings.AgentAskTimeoutAction {
		case "default", "error":
		default:
			newSettings.AgentAskTimeoutAction = ""
		}
	}
	if newSettings.PromptPolicyProfile != "" {
		switch strings.ToLower(strings.TrimSpace(newSettings.PromptPolicyProfile)) {
		case "default":
			newSettings.PromptPolicyProfile = "default"
		default:
			newSettings.PromptPolicyProfile = ""
		}
	}
	if newSettings.AgentLoopPolicyMaxToolRounds != nil {
		if !isWithinIntRange(*newSettings.AgentLoopPolicyMaxToolRounds, 1, 200) {
			newSettings.AgentLoopPolicyMaxToolRounds = nil
		}
	}
	if newSettings.AgentLoopPolicyMaxAutoContinue != nil {
		if !isWithinIntRange(*newSettings.AgentLoopPolicyMaxAutoContinue, 0, 100) {
			newSettings.AgentLoopPolicyMaxAutoContinue = nil
		}
	}
	if newSettings.AgentLoopPolicyPseudoToolCallBudget != nil {
		if !isWithinIntRange(*newSettings.AgentLoopPolicyPseudoToolCallBudget, 0, 100) {
			newSettings.AgentLoopPolicyPseudoToolCallBudget = nil
		}
	}
	if newSettings.AgentLoopPolicyActionPledgeBudget != nil {
		if !isWithinIntRange(*newSettings.AgentLoopPolicyActionPledgeBudget, 0, 100) {
			newSettings.AgentLoopPolicyActionPledgeBudget = nil
		}
	}
	if newSettings.AgentLoopPolicyMissingTodoBudget != nil {
		if !isWithinIntRange(*newSettings.AgentLoopPolicyMissingTodoBudget, 0, 100) {
			newSettings.AgentLoopPolicyMissingTodoBudget = nil
		}
	}
	if newSettings.AgentLoopPolicyPendingTodoBudget != nil {
		if !isWithinIntRange(*newSettings.AgentLoopPolicyPendingTodoBudget, 0, 100) {
			newSettings.AgentLoopPolicyPendingTodoBudget = nil
		}
	}
	if newSettings.SmallModelRuntime != "" && newSettings.SmallModelRuntime != smallmodel.RuntimeType {
		newSettings.SmallModelRuntime = ""
	}
	if newSettings.SmallModelID != "" && newSettings.SmallModelID != smallmodel.ModelID {
		newSettings.SmallModelID = ""
	}
	if newSettings.NoLLMDegradeMode != "" {
		switch newSettings.NoLLMDegradeMode {
		case "deepresearch":
		default:
			newSettings.NoLLMDegradeMode = ""
		}
	}
	if newSettings.SmallModelUnavailablePolicy != "" {
		switch newSettings.SmallModelUnavailablePolicy {
		case "ir_first":
		default:
			newSettings.SmallModelUnavailablePolicy = ""
		}
	}
	newSettings.SmallModelContextPruneToolAllow = sanitizeStringList(newSettings.SmallModelContextPruneToolAllow, 32, 128)
	newSettings.SmallModelContextPruneToolDeny = sanitizeStringList(newSettings.SmallModelContextPruneToolDeny, 32, 128)
	newSettings.DirectoryWhitelist = sanitizeDirectoryWhitelistEntries(newSettings.DirectoryWhitelist, 32)
	newSettings.VoiceWakeTriggers = sanitizeStringList(newSettings.VoiceWakeTriggers, 8, 64)
	newSettings.VoiceWakeLocale = strings.TrimSpace(newSettings.VoiceWakeLocale)
	newSettings.VoiceWakeTargetConversationID = strings.TrimSpace(newSettings.VoiceWakeTargetConversationID)
	applyDefaultDirectoryWhitelist(&newSettings)

	h.mu.Lock()
	h.settings = &newSettings
	voiceWakeManager := h.voiceWakeManager
	h.mu.Unlock()

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
	}
	if voiceWakeManager != nil {
		_ = voiceWakeManager.Refresh(c.Request().Context())
	}

	return c.JSON(http.StatusOK, h.settings)
}

// Patch handles PATCH /api/settings (partial update)
func (h *SettingsHandler) Patch(c echo.Context) error {
	var updates map[string]interface{}
	raw, err := decodeSettingsRequestBody(c, &updates)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if removed := firstRemovedSettingsKey(raw); removed != "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Removed setting: " + removed})
	}

	h.mu.Lock()
	// Apply partial updates
	if locale, ok := updates["locale"].(string); ok {
		h.settings.Locale = locale
	}
	if timezone, ok := updates["timezone"].(string); ok {
		h.settings.Timezone = timezone
	}
	if v, ok := updates["smart_skill_selection"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmartSkillSelection = &b
		}
	}
	if v, ok := updates["skill_dynamic_exposure"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SkillDynamicExposure = &b
		}
	}
	if mode, ok := updates["skill_selector_mode"].(string); ok {
		switch mode {
		case "hybrid", "ir_only", "llm_only":
			h.settings.SkillSelectorMode = mode
		}
	}
	if v, ok := updates["skill_rerank_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SkillRerankEnabled = &b
		}
	}
	if v, ok := updates["skill_rerank_onnx_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SkillRerankONNXEnabled = &b
		}
	}
	if v, ok := updates["skill_rerank_onnx_auto_download"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SkillRerankONNXAutoDownload = &b
		}
	}
	if model, ok := updates["skill_rerank_model"].(string); ok {
		h.settings.SkillRerankModel = model
	}
	if v, ok := updates["skill_selector_confidence_threshold"]; ok {
		switch n := v.(type) {
		case float64:
			h.settings.SkillSelectorConfidenceThreshold = &n
		case float32:
			f := float64(n)
			h.settings.SkillSelectorConfidenceThreshold = &f
		}
	}
	if mode, ok := updates["memory_recall_mode"].(string); ok {
		if _, valid := allowedMemoryRecallModes[mode]; valid {
			h.settings.MemoryRecallMode = mode
		}
	}
	if v, ok := updates["im_history_limit"]; ok {
		if iv, ok := intFromAny(v); ok {
			if iv < 0 {
				iv = 0
			}
			h.settings.IMHistoryLimit = &iv
		}
	}
	if v, ok := updates["agent_mode"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.AgentMode = &b
		}
	}
	if v, ok := updates["agent_auto_reflect"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.AgentAutoReflect = &b
		}
	}
	if v, ok := updates["agent_auto_confirm"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.AgentAutoConfirm = &b
		}
	}
	if v, ok := updates["agent_ask_timeout_seconds"]; ok {
		switch n := v.(type) {
		case float64:
			iv := int(n)
			if iv >= 15 && iv <= 1800 {
				h.settings.AgentAskTimeoutSeconds = &iv
			}
		case int:
			if n >= 15 && n <= 1800 {
				iv := n
				h.settings.AgentAskTimeoutSeconds = &iv
			}
		}
	}
	if v, ok := updates["agent_ask_timeout_action"]; ok {
		if s, isString := v.(string); isString {
			switch s {
			case "default", "error":
				h.settings.AgentAskTimeoutAction = s
			}
		}
	}
	if version, ok := updates["prompt_policy_version"].(string); ok {
		h.settings.PromptPolicyVersion = strings.TrimSpace(version)
	}
	if profile, ok := updates["prompt_policy_profile"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(profile)) {
		case "default":
			h.settings.PromptPolicyProfile = "default"
		}
	}
	if v, ok := updates["agent_loop_policy_max_tool_rounds"]; ok {
		if iv, ok := intFromAny(v); ok && isWithinIntRange(iv, 1, 200) {
			h.settings.AgentLoopPolicyMaxToolRounds = &iv
		}
	}
	if v, ok := updates["agent_loop_policy_max_auto_continue"]; ok {
		if iv, ok := intFromAny(v); ok && isWithinIntRange(iv, 0, 100) {
			h.settings.AgentLoopPolicyMaxAutoContinue = &iv
		}
	}
	if v, ok := updates["agent_loop_policy_pseudo_tool_call_budget"]; ok {
		if iv, ok := intFromAny(v); ok && isWithinIntRange(iv, 0, 100) {
			h.settings.AgentLoopPolicyPseudoToolCallBudget = &iv
		}
	}
	if v, ok := updates["agent_loop_policy_action_pledge_budget"]; ok {
		if iv, ok := intFromAny(v); ok && isWithinIntRange(iv, 0, 100) {
			h.settings.AgentLoopPolicyActionPledgeBudget = &iv
		}
	}
	if v, ok := updates["agent_loop_policy_missing_todo_budget"]; ok {
		if iv, ok := intFromAny(v); ok && isWithinIntRange(iv, 0, 100) {
			h.settings.AgentLoopPolicyMissingTodoBudget = &iv
		}
	}
	if v, ok := updates["agent_loop_policy_pending_todo_budget"]; ok {
		if iv, ok := intFromAny(v); ok && isWithinIntRange(iv, 0, 100) {
			h.settings.AgentLoopPolicyPendingTodoBudget = &iv
		}
	}
	if v, ok := updates["small_model_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelEnabled = &b
		}
	}
	if v, ok := updates["small_model_auto_download"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelAutoDownload = &b
		}
	}
	if runtimeName, ok := updates["small_model_runtime"].(string); ok {
		if runtimeName == smallmodel.RuntimeType {
			h.settings.SmallModelRuntime = runtimeName
		}
	}
	if modelID, ok := updates["small_model_id"].(string); ok {
		if modelID == smallmodel.ModelID {
			h.settings.SmallModelID = modelID
		}
	}
	if v, ok := updates["small_model_summary_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelSummaryEnabled = &b
		}
	}
	if mode, ok := updates["context_compression_mode"].(string); ok {
		switch mode {
		case "off":
			h.settings.ContextCompressionMode = "auto"
		case "offline", "small_model", "auto":
			h.settings.ContextCompressionMode = mode
		}
	}
	if v, ok := updates["small_model_context_compress_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelContextCompressEnabled = &b
		}
	}
	if v, ok := updates["small_model_doc_extract_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelDocExtractEnabled = &b
		}
	}
	if v, ok := updates["small_model_rerank_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelRerankEnabled = &b
		}
	}
	if v, ok := updates["small_model_context_prune_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelContextPruneEnabled = &b
		}
	}
	if v, ok := updates["small_model_context_prune_tool_allow"]; ok {
		if v == nil {
			h.settings.SmallModelContextPruneToolAllow = nil
		} else if list, ok := stringSliceFromAny(v); ok {
			h.settings.SmallModelContextPruneToolAllow = sanitizeStringList(list, 32, 128)
		}
	}
	if v, ok := updates["small_model_context_prune_tool_deny"]; ok {
		if v == nil {
			h.settings.SmallModelContextPruneToolDeny = nil
		} else if list, ok := stringSliceFromAny(v); ok {
			h.settings.SmallModelContextPruneToolDeny = sanitizeStringList(list, 32, 128)
		}
	}
	if v, ok := updates["small_model_media_intent_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelMediaIntentEnabled = &b
		}
	}
	if v, ok := updates["offline_ir_fallback_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.OfflineIRFallbackEnabled = &b
		}
	}
	if v, ok := updates["feature_intent_ir_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.FeatureIntentIREnabled = &b
		}
	}
	if v, ok := updates["deep_research_v2_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.DeepResearchV2Enabled = &b
		}
	}
	if v, ok := updates["small_model_route_image_qa_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelRouteImageQAEnabled = &b
		}
	}
	if v, ok := updates["small_model_route_short_qa_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelRouteShortQAEnabled = &b
		}
	}
	if mode, ok := updates["no_llm_degrade_mode"].(string); ok {
		if mode == "deepresearch" {
			h.settings.NoLLMDegradeMode = mode
		}
	}
	if policy, ok := updates["small_model_unavailable_policy"].(string); ok {
		switch policy {
		case "ir_first":
			h.settings.SmallModelUnavailablePolicy = policy
		}
	}
	if v, ok := updates["directory_whitelist_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.DirectoryWhitelistEnabled = &b
		}
	}
	if v, ok := updates["directory_whitelist"]; ok {
		if v == nil {
			h.settings.DirectoryWhitelist = nil
		} else if entries, ok := directoryWhitelistEntriesFromAny(v); ok {
			h.settings.DirectoryWhitelist = sanitizeDirectoryWhitelistEntries(entries, 32)
		}
	}
	if v, ok := updates["voice_wake_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.VoiceWakeEnabled = &b
		}
	}
	if v, ok := updates["voice_wake_triggers"]; ok {
		if v == nil {
			h.settings.VoiceWakeTriggers = nil
		} else if list, ok := stringSliceFromAny(v); ok {
			h.settings.VoiceWakeTriggers = sanitizeStringList(list, 8, 64)
		}
	}
	if locale, ok := updates["voice_wake_locale"].(string); ok {
		h.settings.VoiceWakeLocale = strings.TrimSpace(locale)
	}
	if target, ok := updates["voice_wake_target_conversation_id"].(string); ok {
		h.settings.VoiceWakeTargetConversationID = strings.TrimSpace(target)
	}
	applyDefaultDirectoryWhitelist(h.settings)
	voiceWakeManager := h.voiceWakeManager
	h.mu.Unlock()

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
	}
	if voiceWakeManager != nil {
		_ = voiceWakeManager.Refresh(c.Request().Context())
	}

	return c.JSON(http.StatusOK, h.settings)
}

// GetLocale returns the current locale setting, falling back to OS-detected locale.
func (h *SettingsHandler) GetLocale() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.Locale != "" {
		return h.settings.Locale
	}
	return workspace.DetectLocale()
}

// GetVoiceWakeEnabled returns whether VoiceWake should be enabled.
func (h *SettingsHandler) GetVoiceWakeEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.VoiceWakeEnabled != nil && *h.settings.VoiceWakeEnabled
}

func (h *SettingsHandler) DirectoryWhitelistSnapshot() (bool, []DirectoryWhitelistEntry) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	enabled := h.settings.DirectoryWhitelistEnabled != nil && *h.settings.DirectoryWhitelistEnabled
	entries := sanitizeDirectoryWhitelistEntries(h.settings.DirectoryWhitelist, 32)
	if len(entries) == 0 {
		return enabled, nil
	}

	out := make([]DirectoryWhitelistEntry, len(entries))
	copy(out, entries)
	return enabled, out
}

// GetVoiceWakeTriggers returns configured VoiceWake triggers with defaults applied.
func (h *SettingsHandler) GetVoiceWakeTriggers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), voicewake.DefaultTriggers(sanitizeStringList(h.settings.VoiceWakeTriggers, 8, 64))...)
}

// GetVoiceWakeLocale returns the optional VoiceWake locale override.
func (h *SettingsHandler) GetVoiceWakeLocale() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return strings.TrimSpace(h.settings.VoiceWakeLocale)
}

// GetVoiceWakeTargetConversationID returns the fixed VoiceWake target conversation.
func (h *SettingsHandler) GetVoiceWakeTargetConversationID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return strings.TrimSpace(h.settings.VoiceWakeTargetConversationID)
}

// GetSmartSkillSelection returns whether smart skill selection is enabled (default false).
func (h *SettingsHandler) GetSmartSkillSelection() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmartSkillSelection == nil {
		return false
	}
	return *h.settings.SmartSkillSelection
}

// GetSkillDynamicExposure returns whether runtime skill dynamic exposure is enabled (default false).
func (h *SettingsHandler) GetSkillDynamicExposure() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillDynamicExposure == nil {
		return false
	}
	return *h.settings.SkillDynamicExposure
}

func (h *SettingsHandler) GetSkillDynamicExposureExplicit() (bool, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillDynamicExposure == nil {
		return false, false
	}
	return *h.settings.SkillDynamicExposure, true
}

// GetSkillSelectorMode returns selector mode (default "hybrid").
func (h *SettingsHandler) GetSkillSelectorMode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	switch h.settings.SkillSelectorMode {
	case "ir_only", "llm_only":
		return h.settings.SkillSelectorMode
	default:
		return "hybrid"
	}
}

// GetSkillRerankEnabled returns whether stage-2 rerank is enabled (default false).
func (h *SettingsHandler) GetSkillRerankEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillRerankEnabled == nil {
		return false
	}
	return *h.settings.SkillRerankEnabled
}

// GetEffectiveSkillRerankEnabled returns the effective rerank switch after
// applying the small-model rerank scene toggle.
func (h *SettingsHandler) GetEffectiveSkillRerankEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rerankEnabled := false
	if h.settings.SkillRerankEnabled != nil {
		rerankEnabled = *h.settings.SkillRerankEnabled
	}
	if !rerankEnabled {
		return false
	}

	smallModelEnabled := false
	if h.settings.SmallModelEnabled != nil {
		smallModelEnabled = *h.settings.SmallModelEnabled
	}
	if !smallModelEnabled {
		return true
	}

	smallModelRerankEnabled := false
	if h.settings.SmallModelRerankEnabled != nil {
		smallModelRerankEnabled = *h.settings.SmallModelRerankEnabled
	}
	return smallModelRerankEnabled
}

// IsSkillRerankEnabledSet returns true when user explicitly set this value.
func (h *SettingsHandler) IsSkillRerankEnabledSet() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.SkillRerankEnabled != nil
}

// GetSkillRerankModel returns reranker model repo (default cross-encoder mini model).
func (h *SettingsHandler) GetSkillRerankModel() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillRerankModel == "" {
		return "cross-encoder/ms-marco-MiniLM-L6-v2"
	}
	return h.settings.SkillRerankModel
}

// GetSkillRerankONNXEnabled returns whether ONNX reranker path is enabled (default false).
func (h *SettingsHandler) GetSkillRerankONNXEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillRerankONNXEnabled == nil {
		return false
	}
	return *h.settings.SkillRerankONNXEnabled
}

// IsSkillRerankONNXEnabledSet returns true when user explicitly set this value.
func (h *SettingsHandler) IsSkillRerankONNXEnabledSet() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.SkillRerankONNXEnabled != nil
}

// GetSkillRerankONNXAutoDownload returns whether ONNX model auto-download is enabled (default false).
func (h *SettingsHandler) GetSkillRerankONNXAutoDownload() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillRerankONNXAutoDownload == nil {
		return false
	}
	return *h.settings.SkillRerankONNXAutoDownload
}

// IsSkillRerankONNXAutoDownloadSet returns true when user explicitly set this value.
func (h *SettingsHandler) IsSkillRerankONNXAutoDownloadSet() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.SkillRerankONNXAutoDownload != nil
}

// GetSkillSelectorConfidenceThreshold returns confidence threshold (default 0.78).
func (h *SettingsHandler) GetSkillSelectorConfidenceThreshold() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillSelectorConfidenceThreshold == nil {
		return 0.78
	}
	v := *h.settings.SkillSelectorConfidenceThreshold
	if v <= 0 || v > 1 {
		return 0.78
	}
	return v
}

// GetMemoryRecallMode returns memory recall mode (default "balanced").
func (h *SettingsHandler) GetMemoryRecallMode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if _, ok := allowedMemoryRecallModes[h.settings.MemoryRecallMode]; ok {
		return h.settings.MemoryRecallMode
	}
	return "balanced"
}

// GetIMHistoryLimit returns recent rounds retained for IM no-history tier.
// Defaults to 3; values below 0 are clamped to 0.
func (h *SettingsHandler) GetIMHistoryLimit() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.IMHistoryLimit == nil {
		return defaultIMHistoryLimit
	}
	v := *h.settings.IMHistoryLimit
	if v < 0 {
		return 0
	}
	return v
}

// HasIMHistoryLimit reports whether IM history limit is explicitly configured.
func (h *SettingsHandler) HasIMHistoryLimit() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.IMHistoryLimit != nil
}

// GetAgentMode returns whether agent mode is enabled (default false).
func (h *SettingsHandler) GetAgentMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentMode == nil {
		return false
	}
	return *h.settings.AgentMode
}

// GetAgentAutoReflect returns whether post-task reflection is enabled.
// Defaults to true when not explicitly configured.
func (h *SettingsHandler) GetAgentAutoReflect() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentAutoReflect != nil {
		return *h.settings.AgentAutoReflect
	}
	return true
}

// GetAgentAutoConfirm returns whether agent mode skips confirmation (default false).
func (h *SettingsHandler) GetAgentAutoConfirm() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentAutoConfirm == nil {
		return false
	}
	return *h.settings.AgentAutoConfirm
}

// GetAgentAskTimeoutSeconds returns ask timeout in seconds (default 120, bounded).
func (h *SettingsHandler) GetAgentAskTimeoutSeconds() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentAskTimeoutSeconds == nil {
		return 120
	}
	v := *h.settings.AgentAskTimeoutSeconds
	if v < 15 {
		return 15
	}
	if v > 1800 {
		return 1800
	}
	return v
}

// GetAgentAskTimeoutAction returns timeout action (default "default").
func (h *SettingsHandler) GetAgentAskTimeoutAction() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	switch h.settings.AgentAskTimeoutAction {
	case "error":
		return "error"
	default:
		return "default"
	}
}

// GetPromptPolicyVersion returns the active prompt policy version marker.
func (h *SettingsHandler) GetPromptPolicyVersion() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if strings.TrimSpace(h.settings.PromptPolicyVersion) == "" {
		return DefaultPromptPolicyVersion
	}
	return strings.TrimSpace(h.settings.PromptPolicyVersion)
}

// GetPromptPolicyProfile returns the active prompt policy profile.
func (h *SettingsHandler) GetPromptPolicyProfile() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	switch strings.ToLower(strings.TrimSpace(h.settings.PromptPolicyProfile)) {
	case "default":
		return "default"
	default:
		return DefaultPromptPolicyProfile
	}
}

func (h *SettingsHandler) GetAgentLoopPolicyMaxToolRounds() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentLoopPolicyMaxToolRounds == nil {
		return maxToolRoundsAgent
	}
	v := *h.settings.AgentLoopPolicyMaxToolRounds
	if v < 1 {
		return 1
	}
	if v > 200 {
		return 200
	}
	return v
}

func (h *SettingsHandler) GetAgentLoopPolicyMaxAutoContinue() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentLoopPolicyMaxAutoContinue == nil {
		return maxAutoContinueAgent
	}
	v := *h.settings.AgentLoopPolicyMaxAutoContinue
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (h *SettingsHandler) GetAgentLoopPolicyPseudoToolCallBudget() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentLoopPolicyPseudoToolCallBudget == nil {
		return maxPseudoToolCallAutoContinueAgent
	}
	v := *h.settings.AgentLoopPolicyPseudoToolCallBudget
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (h *SettingsHandler) GetAgentLoopPolicyActionPledgeBudget() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentLoopPolicyActionPledgeBudget == nil {
		return maxActionPledgeAutoContinueAgent
	}
	v := *h.settings.AgentLoopPolicyActionPledgeBudget
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (h *SettingsHandler) GetAgentLoopPolicyMissingTodoBudget() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentLoopPolicyMissingTodoBudget == nil {
		return maxMissingTodoAutoContinueAgent
	}
	v := *h.settings.AgentLoopPolicyMissingTodoBudget
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (h *SettingsHandler) GetAgentLoopPolicyPendingTodoBudget() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentLoopPolicyPendingTodoBudget == nil {
		return maxPendingTodoAutoContinueAgent
	}
	v := *h.settings.AgentLoopPolicyPendingTodoBudget
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// GetSmallModelEnabled returns whether small-model routing features are enabled (default false).
func (h *SettingsHandler) GetSmallModelEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelEnabled == nil {
		return false
	}
	return *h.settings.SmallModelEnabled
}

// GetSmallModelRuntime returns the runtime type (fixed llama.cpp).
func (h *SettingsHandler) GetSmallModelRuntime() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRuntime == smallmodel.RuntimeType {
		return h.settings.SmallModelRuntime
	}
	return smallmodel.RuntimeType
}

// GetSmallModelID returns fixed model id.
func (h *SettingsHandler) GetSmallModelID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelID == smallmodel.ModelID {
		return h.settings.SmallModelID
	}
	return smallmodel.ModelID
}

// GetSmallModelAutoDownload returns whether model auto-download is enabled (default true).
func (h *SettingsHandler) GetSmallModelAutoDownload() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelAutoDownload == nil {
		return true
	}
	return *h.settings.SmallModelAutoDownload
}

func (h *SettingsHandler) GetSmallModelSummaryEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelSummaryEnabled == nil {
		return false
	}
	return *h.settings.SmallModelSummaryEnabled
}

func (h *SettingsHandler) GetContextCompressionMode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	switch h.settings.ContextCompressionMode {
	case "offline", "small_model":
		return h.settings.ContextCompressionMode
	default:
		return "auto"
	}
}

func (h *SettingsHandler) GetSmallModelContextCompressEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelContextCompressEnabled == nil {
		return false
	}
	return *h.settings.SmallModelContextCompressEnabled
}

func (h *SettingsHandler) GetSmallModelDocExtractEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelDocExtractEnabled == nil {
		return false
	}
	return *h.settings.SmallModelDocExtractEnabled
}

func (h *SettingsHandler) GetSmallModelRerankEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRerankEnabled == nil {
		return false
	}
	return *h.settings.SmallModelRerankEnabled
}

func (h *SettingsHandler) GetSmallModelContextPruneEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelContextPruneEnabled == nil {
		return false
	}
	return *h.settings.SmallModelContextPruneEnabled
}

// GetSmallModelContextPruneExplicit returns the context-prune preference and
// whether the user explicitly set it, so callers can distinguish "unset" from
// "off".
func (h *SettingsHandler) GetSmallModelContextPruneExplicit() (bool, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelContextPruneEnabled == nil {
		return false, false
	}
	return *h.settings.SmallModelContextPruneEnabled, true
}

func (h *SettingsHandler) GetSmallModelContextPruneToolAllow() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), sanitizeStringList(h.settings.SmallModelContextPruneToolAllow, 32, 128)...)
}

func (h *SettingsHandler) GetSmallModelContextPruneToolDeny() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), sanitizeStringList(h.settings.SmallModelContextPruneToolDeny, 32, 128)...)
}

func (h *SettingsHandler) GetSmallModelMediaIntentEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelMediaIntentEnabled == nil {
		return false
	}
	return *h.settings.SmallModelMediaIntentEnabled
}

func (h *SettingsHandler) GetOfflineIRFallbackEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.OfflineIRFallbackEnabled == nil {
		return false
	}
	return *h.settings.OfflineIRFallbackEnabled
}

func (h *SettingsHandler) GetFeatureIntentIREnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.FeatureIntentIREnabled == nil {
		return false
	}
	return *h.settings.FeatureIntentIREnabled
}

// GetDeepResearchV2Enabled returns whether deep research V2 pipeline is enabled (default false).
func (h *SettingsHandler) GetDeepResearchV2Enabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.DeepResearchV2Enabled == nil {
		return false
	}
	return *h.settings.DeepResearchV2Enabled
}

func (h *SettingsHandler) GetSmallModelRouteImageQAEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRouteImageQAEnabled != nil {
		return *h.settings.SmallModelRouteImageQAEnabled
	}
	if h.settings.SmallModelRouteShortQAEnabled == nil {
		return false
	}
	return *h.settings.SmallModelRouteShortQAEnabled
}

func (h *SettingsHandler) GetSmallModelRouteShortQAEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRouteShortQAEnabled == nil {
		return false
	}
	return *h.settings.SmallModelRouteShortQAEnabled
}

// SetSmallModelRouteShortQAEnabled updates short-qa route switch and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelRouteShortQAEnabled(enabled bool) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelRouteShortQAEnabled != nil && *h.settings.SmallModelRouteShortQAEnabled == enabled {
		return false, nil
	}
	h.settings.SmallModelRouteShortQAEnabled = &enabled
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelRouteImageQAEnabled updates image-qa route switch and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelRouteImageQAEnabled(enabled bool) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelRouteImageQAEnabled != nil && *h.settings.SmallModelRouteImageQAEnabled == enabled {
		return false, nil
	}
	h.settings.SmallModelRouteImageQAEnabled = &enabled
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelSummaryEnabled updates summary enhancement switch and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelSummaryEnabled(enabled bool) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelSummaryEnabled != nil && *h.settings.SmallModelSummaryEnabled == enabled {
		return false, nil
	}
	h.settings.SmallModelSummaryEnabled = &enabled
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelContextCompressEnabled updates context-compress enhancement switch and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelContextCompressEnabled(enabled bool) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelContextCompressEnabled != nil && *h.settings.SmallModelContextCompressEnabled == enabled {
		return false, nil
	}
	h.settings.SmallModelContextCompressEnabled = &enabled
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetContextCompressionMode updates the unified context-compression mode and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetContextCompressionMode(mode string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch mode {
	case "off":
		mode = "auto"
	case "offline", "small_model", "auto":
	default:
		mode = "auto"
	}
	if h.settings.ContextCompressionMode == mode {
		return false, nil
	}
	h.settings.ContextCompressionMode = mode
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelDocExtractEnabled updates doc-extract enhancement switch and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelDocExtractEnabled(enabled bool) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelDocExtractEnabled != nil && *h.settings.SmallModelDocExtractEnabled == enabled {
		return false, nil
	}
	h.settings.SmallModelDocExtractEnabled = &enabled
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// GetNoLLMDegradeMode returns fallback mode when no provider is available (default deepresearch).
func (h *SettingsHandler) GetNoLLMDegradeMode() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.NoLLMDegradeMode == "deepresearch" {
		return h.settings.NoLLMDegradeMode
	}
	return "deepresearch"
}

// GetSmallModelUnavailablePolicy returns fallback policy when small model is unavailable.
func (h *SettingsHandler) GetSmallModelUnavailablePolicy() string {
	return "ir_first"
}

func intFromAny(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	default:
		return 0, false
	}
}

func stringSliceFromAny(v interface{}) ([]string, bool) {
	switch vv := v.(type) {
	case []string:
		return vv, true
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func directoryWhitelistEntriesFromAny(v interface{}) ([]DirectoryWhitelistEntry, bool) {
	switch vv := v.(type) {
	case []DirectoryWhitelistEntry:
		return vv, true
	case []interface{}:
		out := make([]DirectoryWhitelistEntry, 0, len(vv))
		for _, item := range vv {
			obj, ok := item.(map[string]interface{})
			if !ok {
				return nil, false
			}
			entry := DirectoryWhitelistEntry{}
			if path, ok := obj["path"].(string); ok {
				entry.Path = path
			}
			if alias, ok := obj["alias"].(string); ok {
				entry.Alias = alias
			}
			out = append(out, entry)
		}
		return out, true
	default:
		return nil, false
	}
}

func sanitizeStringList(values []string, maxItems, maxLen int) []string {
	if len(values) == 0 || maxItems <= 0 || maxLen <= 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if len([]rune(s)) > maxLen {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
		if len(out) >= maxItems {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeDirectoryWhitelistEntries(entries []DirectoryWhitelistEntry, maxItems int) []DirectoryWhitelistEntry {
	if len(entries) == 0 || maxItems <= 0 {
		return nil
	}

	out := make([]DirectoryWhitelistEntry, 0, len(entries))
	seenPaths := make(map[string]struct{}, len(entries))
	seenAliases := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			continue
		}

		cleanPath := filepath.Clean(path)
		if cleanPath == "." || !filepath.IsAbs(cleanPath) || len([]rune(cleanPath)) > 1024 {
			continue
		}

		pathKey := strings.ToLower(cleanPath)
		if _, ok := seenPaths[pathKey]; ok {
			continue
		}

		alias := sanitizeDirectoryWhitelistAlias(entry.Alias)
		if alias != "" {
			aliasKey := strings.ToLower(alias)
			if _, ok := seenAliases[aliasKey]; ok {
				alias = ""
			} else {
				seenAliases[aliasKey] = struct{}{}
			}
		}

		seenPaths[pathKey] = struct{}{}
		out = append(out, DirectoryWhitelistEntry{Path: cleanPath, Alias: alias})
		if len(out) >= maxItems {
			break
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeDirectoryWhitelistAlias(alias string) string {
	s := strings.TrimSpace(alias)
	if s == "" || len([]rune(s)) > 64 {
		return ""
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return ""
		}
	}
	return s
}

func defaultDirectoryWhitelistEntries() []DirectoryWhitelistEntry {
	if runtime.GOOS == "windows" {
		return nil
	}
	return []DirectoryWhitelistEntry{{Path: defaultDirectoryWhitelistPath}}
}

func applyDefaultDirectoryWhitelist(settings *Settings) {
	if settings == nil {
		return
	}

	settings.DirectoryWhitelist = sanitizeDirectoryWhitelistEntries(settings.DirectoryWhitelist, 32)
	if settings.DirectoryWhitelistEnabled != nil {
		return
	}

	enabled := true
	if len(settings.DirectoryWhitelist) > 0 {
		settings.DirectoryWhitelistEnabled = &enabled
		return
	}

	defaults := sanitizeDirectoryWhitelistEntries(defaultDirectoryWhitelistEntries(), 32)
	if len(defaults) == 0 {
		return
	}
	settings.DirectoryWhitelistEnabled = &enabled
	settings.DirectoryWhitelist = defaults
}

func isWithinIntRange(v, min, max int) bool {
	return v >= min && v <= max
}

func isAdminRequest(c echo.Context) bool {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return strings.EqualFold(strings.TrimSpace(claims.Role), "admin")
	}
	return false
}

// load reads settings from kvstore
func (h *SettingsHandler) load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.kv.GetJSON(context.Background(), settingsKVKey, h.settings); err != nil {
		// Key not found or error — use defaults
		h.settings = &Settings{}
	}
	applyDefaultDirectoryWhitelist(h.settings)
}

// save writes settings to kvstore
func (h *SettingsHandler) save() error {
	return h.kv.SetJSON(context.Background(), settingsKVKey, h.settings, 0)
}
