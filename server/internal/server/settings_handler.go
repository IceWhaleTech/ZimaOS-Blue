package server

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
	"github.com/google/uuid"
)

const settingsKVKey = "config:settings"
const soulProposalKVKey = "config:soul_proposals"

// SettingsHandler handles user settings API endpoints
type SettingsHandler struct {
	mu                        sync.RWMutex
	kv                        kvstore.Store
	settings                  *Settings
	skillRerankerModelManager *claudecode.SkillRerankerModelManager
	smallModelManager         *smallmodel.Manager
	soulProposals             map[string]*SoulProposal
}

// Settings represents user preferences stored on backend
type Settings struct {
	Locale                             string   `json:"locale,omitempty"`                                  // User's preferred locale (e.g., "zh-CN", "en-US")
	Timezone                           string   `json:"timezone,omitempty"`                                // User's timezone
	ThemeStyle                         string   `json:"theme_style,omitempty"`                             // Chat theme style
	SmartToolSelection                 *bool    `json:"smart_tool_selection,omitempty"`                    // IR-based tool filtering (nil = default true)
	SmartSkillSelection                *bool    `json:"smart_skill_selection,omitempty"`                   // Progressive skill selector (nil = default true)
	SkillSelectorMode                  string   `json:"skill_selector_mode,omitempty"`                     // hybrid|ir_only|llm_only
	SkillRerankEnabled                 *bool    `json:"skill_rerank_enabled,omitempty"`                    // Enable stage-2 rerank (nil = default true)
	SkillRerankModel                   string   `json:"skill_rerank_model,omitempty"`                      // Reranker model repo (e.g. cross-encoder/ms-marco-MiniLM-L6-v2)
	SkillRerankONNXEnabled             *bool    `json:"skill_rerank_onnx_enabled,omitempty"`               // Enable ONNX reranker path (nil = default false)
	SkillRerankONNXAutoDownload        *bool    `json:"skill_rerank_onnx_auto_download,omitempty"`         // Allow ONNX model auto-download (nil = default false)
	SkillSelectorConfidenceThreshold   *float64 `json:"skill_selector_confidence_threshold,omitempty"`     // default 0.78
	MemoryRecallMode                   string   `json:"memory_recall_mode,omitempty"`                      // Memory recall strategy: aggressive|balanced|quality
	AgentMode                          *bool    `json:"agent_mode,omitempty"`                              // Autonomous agent mode (nil = default false)
	AgentAutoConfirm                   *bool    `json:"agent_auto_confirm,omitempty"`                      // Skip confirmation in agent mode (nil = default false)
	AgentAskTimeoutSeconds             *int     `json:"agent_ask_timeout_seconds,omitempty"`               // Ask timeout in seconds (default 120, range 15-1800)
	AgentAskTimeoutAction              string   `json:"agent_ask_timeout_action,omitempty"`                // default|error
	SmallModelEnabled                  *bool    `json:"small_model_enabled,omitempty"`                     // default false
	SmallModelRuntime                  string   `json:"small_model_runtime,omitempty"`                     // fixed: llama_cpp_native
	SmallModelID                       string   `json:"small_model_id,omitempty"`                          // fixed: lfm2.5-1.2b-instruct-q4km
	SmallModelAutoDownload             *bool    `json:"small_model_auto_download,omitempty"`               // default true
	SmallModelShadowRatio              *float64 `json:"small_model_shadow_ratio,omitempty"`                // default 0.1, (0,1]
	SmallModelShadowGateMinSamples     *int     `json:"small_model_shadow_gate_min_samples,omitempty"`     // default 40, [1,10000]
	SmallModelShadowGateThresholdDelta *float64 `json:"small_model_shadow_gate_threshold_delta,omitempty"` // default 0.35, (0,1]
	SmallModelShadowGateScene          string   `json:"small_model_shadow_gate_scene,omitempty"`           // default "", optional scene filter
	SmallModelSummaryEnabled           *bool    `json:"small_model_summary_enabled,omitempty"`             // default true
	SmallModelDocExtractEnabled        *bool    `json:"small_model_doc_extract_enabled,omitempty"`         // default true
	SmallModelRerankEnabled            *bool    `json:"small_model_rerank_enabled,omitempty"`              // default true
	SmallModelContextPruneEnabled      *bool    `json:"small_model_context_prune_enabled,omitempty"`       // default true
	SmallModelRouteShortQAEnabled      *bool    `json:"small_model_route_short_qa_enabled,omitempty"`      // default false
	SmallModelRouteToolDispatchEnabled *bool    `json:"small_model_route_tool_dispatch_enabled,omitempty"` // default false
	NoLLMDegradeMode                   string   `json:"no_llm_degrade_mode,omitempty"`                     // fixed default deepresearch
	SmallModelUnavailablePolicy        string   `json:"small_model_unavailable_policy,omitempty"`          // default ir_first
}

type SoulProposal struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Source     string     `json:"source,omitempty"`
	Status     string     `json:"status"` // pending|approved|rejected
	CreatedAt  time.Time  `json:"created_at"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
}

var allowedThemeStyles = map[string]struct{}{
	"default":  {},
	"bubble":   {},
	"minimal":  {},
	"gradient": {},
	"ocean":    {},
}

var allowedMemoryRecallModes = map[string]struct{}{
	"aggressive": {},
	"balanced":   {},
	"quality":    {},
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(kv kvstore.Store) *SettingsHandler {
	h := &SettingsHandler{
		kv:            kv,
		settings:      &Settings{},
		soulProposals: map[string]*SoulProposal{},
	}
	h.load()
	h.loadSoulProposals()
	return h
}

// RegisterRoutes registers settings routes
func (h *SettingsHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/settings", h.Get)
	g.PUT("/settings", h.Update)
	g.PATCH("/settings", h.Patch)
	g.GET("/settings/skill-reranker/model/status", h.GetSkillRerankerModelStatus)
	g.POST("/settings/skill-reranker/model/download", h.StartSkillRerankerModelDownload)
	g.POST("/settings/skill-reranker/model/cancel", h.CancelSkillRerankerModelDownload)
	g.GET("/settings/small-model/status", h.GetSmallModelStatus)
	g.POST("/settings/small-model/download", h.StartSmallModelDownload)
	g.POST("/settings/small-model/cancel", h.CancelSmallModelDownload)
	g.GET("/settings/soul/proposals", h.ListSoulProposals)
	g.POST("/settings/soul/proposals/:id/approve", h.ApproveSoulProposal)
	g.POST("/settings/soul/proposals/:id/reject", h.RejectSoulProposal)
}

// SetSkillRerankerModelManager wires ONNX skill-reranker model manager for UI download APIs.
func (h *SettingsHandler) SetSkillRerankerModelManager(mgr *claudecode.SkillRerankerModelManager) {
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

// Get handles GET /api/settings
func (h *SettingsHandler) Get(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return c.JSON(http.StatusOK, h.settings)
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
		return c.JSON(http.StatusOK, claudecode.SkillRerankerModelStatus{Ready: false})
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

// ListSoulProposals returns pending/handled SOUL write proposals.
func (h *SettingsHandler) ListSoulProposals(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]SoulProposal, 0, len(h.soulProposals))
	for _, p := range h.soulProposals {
		cp := *p
		list = append(list, cp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return c.JSON(http.StatusOK, map[string]interface{}{"proposals": list})
}

// ApproveSoulProposal marks a pending proposal as approved.
func (h *SettingsHandler) ApproveSoulProposal(c echo.Context) error {
	return h.reviewSoulProposal(c, "approved")
}

// RejectSoulProposal marks a pending proposal as rejected.
func (h *SettingsHandler) RejectSoulProposal(c echo.Context) error {
	return h.reviewSoulProposal(c, "rejected")
}

func (h *SettingsHandler) reviewSoulProposal(c echo.Context, status string) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "proposal id is required"})
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.soulProposals[id]
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "proposal not found"})
	}
	now := time.Now().UTC()
	p.Status = status
	p.ReviewedAt = &now
	if err := h.saveSoulProposalsLocked(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save proposal review"})
	}
	return c.JSON(http.StatusOK, p)
}

// AddSoulProposal enqueues a pending proposal that requires manual review.
func (h *SettingsHandler) AddSoulProposal(title, content, source string) (*SoulProposal, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p := &SoulProposal{
		ID:        uuid.NewString(),
		Title:     title,
		Content:   content,
		Source:    source,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	h.soulProposals[p.ID] = p
	if err := h.saveSoulProposalsLocked(); err != nil {
		delete(h.soulProposals, p.ID)
		return nil, err
	}
	return p, nil
}

// Update handles PUT /api/settings (full update)
func (h *SettingsHandler) Update(c echo.Context) error {
	var newSettings Settings
	if err := c.Bind(&newSettings); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if newSettings.ThemeStyle != "" {
		if _, valid := allowedThemeStyles[newSettings.ThemeStyle]; !valid {
			newSettings.ThemeStyle = ""
		}
	}
	if newSettings.MemoryRecallMode != "" {
		if _, valid := allowedMemoryRecallModes[newSettings.MemoryRecallMode]; !valid {
			newSettings.MemoryRecallMode = ""
		}
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
	if newSettings.SmallModelRuntime != "" && newSettings.SmallModelRuntime != smallmodel.RuntimeType {
		newSettings.SmallModelRuntime = ""
	}
	if newSettings.SmallModelID != "" && newSettings.SmallModelID != smallmodel.ModelID {
		newSettings.SmallModelID = ""
	}
	if newSettings.SmallModelShadowRatio != nil {
		v := *newSettings.SmallModelShadowRatio
		if v <= 0 || v > 1 {
			newSettings.SmallModelShadowRatio = nil
		}
	}
	newSettings.SmallModelShadowGateScene = normalizeSmallModelShadowGateScene(newSettings.SmallModelShadowGateScene)
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

	h.mu.Lock()
	h.settings = &newSettings
	h.mu.Unlock()

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
	}

	return c.JSON(http.StatusOK, h.settings)
}

// Patch handles PATCH /api/settings (partial update)
func (h *SettingsHandler) Patch(c echo.Context) error {
	var updates map[string]interface{}
	if err := c.Bind(&updates); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Apply partial updates
	if locale, ok := updates["locale"].(string); ok {
		h.settings.Locale = locale
	}
	if timezone, ok := updates["timezone"].(string); ok {
		h.settings.Timezone = timezone
	}
	if themeStyle, ok := updates["theme_style"].(string); ok {
		if _, valid := allowedThemeStyles[themeStyle]; valid {
			h.settings.ThemeStyle = themeStyle
		}
	}
	if v, ok := updates["smart_tool_selection"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmartToolSelection = &b
		}
	}
	if v, ok := updates["smart_skill_selection"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmartSkillSelection = &b
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
	if v, ok := updates["agent_mode"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.AgentMode = &b
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
	if v, ok := updates["small_model_shadow_ratio"]; ok {
		switch n := v.(type) {
		case float64:
			if n > 0 && n <= 1 {
				h.settings.SmallModelShadowRatio = &n
			}
		case float32:
			f := float64(n)
			if f > 0 && f <= 1 {
				h.settings.SmallModelShadowRatio = &f
			}
		}
	}
	if v, ok := updates["small_model_shadow_gate_min_samples"]; ok {
		switch n := v.(type) {
		case float64:
			iv := int(n)
			if iv >= 1 && iv <= 10000 {
				h.settings.SmallModelShadowGateMinSamples = &iv
			}
		case int:
			if n >= 1 && n <= 10000 {
				iv := n
				h.settings.SmallModelShadowGateMinSamples = &iv
			}
		}
	}
	if v, ok := updates["small_model_shadow_gate_threshold_delta"]; ok {
		switch n := v.(type) {
		case float64:
			if n > 0 && n <= 1 {
				h.settings.SmallModelShadowGateThresholdDelta = &n
			}
		case float32:
			f := float64(n)
			if f > 0 && f <= 1 {
				h.settings.SmallModelShadowGateThresholdDelta = &f
			}
		}
	}
	if v, ok := updates["small_model_shadow_gate_scene"]; ok {
		if s, isString := v.(string); isString {
			h.settings.SmallModelShadowGateScene = normalizeSmallModelShadowGateScene(s)
		}
	}
	if v, ok := updates["small_model_summary_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelSummaryEnabled = &b
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
	if v, ok := updates["small_model_route_short_qa_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelRouteShortQAEnabled = &b
		}
	}
	if v, ok := updates["small_model_route_tool_dispatch_enabled"]; ok {
		if b, isBool := v.(bool); isBool {
			h.settings.SmallModelRouteToolDispatchEnabled = &b
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

	if err := h.save(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save settings"})
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

// GetSmartToolSelection returns whether smart tool selection is enabled (default true).
func (h *SettingsHandler) GetSmartToolSelection() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmartToolSelection == nil {
		return true
	}
	return *h.settings.SmartToolSelection
}

// GetSmartSkillSelection returns whether smart skill selection is enabled (default true).
func (h *SettingsHandler) GetSmartSkillSelection() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmartSkillSelection == nil {
		return true
	}
	return *h.settings.SmartSkillSelection
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

// GetSkillRerankEnabled returns whether stage-2 rerank is enabled (default true).
func (h *SettingsHandler) GetSkillRerankEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SkillRerankEnabled == nil {
		return true
	}
	return *h.settings.SkillRerankEnabled
}

// GetEffectiveSkillRerankEnabled returns the effective rerank switch after
// applying the small-model rerank scene toggle.
func (h *SettingsHandler) GetEffectiveSkillRerankEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rerankEnabled := true
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

	smallModelRerankEnabled := true
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

// GetAgentMode returns whether agent mode is enabled (default false).
func (h *SettingsHandler) GetAgentMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentMode == nil {
		return false
	}
	return *h.settings.AgentMode
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

// GetSmallModelEnabled returns whether small-model routing features are enabled (default false).
func (h *SettingsHandler) GetSmallModelEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelEnabled == nil {
		return false
	}
	return *h.settings.SmallModelEnabled
}

// GetSmallModelRuntime returns the runtime type (fixed llama_cpp_native).
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

// GetSmallModelShadowRatio returns shadow ratio for guarded rollout (default 0.1).
func (h *SettingsHandler) GetSmallModelShadowRatio() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelShadowRatio == nil {
		return 0.1
	}
	v := *h.settings.SmallModelShadowRatio
	if v <= 0 || v > 1 {
		return 0.1
	}
	return v
}

// GetSmallModelShadowGateMinSamples returns gate minimum samples (default 40).
func (h *SettingsHandler) GetSmallModelShadowGateMinSamples() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelShadowGateMinSamples == nil {
		return 40
	}
	v := *h.settings.SmallModelShadowGateMinSamples
	if v < 1 || v > 10000 {
		return 40
	}
	return v
}

// GetSmallModelShadowGateThresholdDelta returns gate threshold delta (default 0.35).
func (h *SettingsHandler) GetSmallModelShadowGateThresholdDelta() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelShadowGateThresholdDelta == nil {
		return 0.35
	}
	v := *h.settings.SmallModelShadowGateThresholdDelta
	if v <= 0 || v > 1 {
		return 0.35
	}
	return v
}

// GetSmallModelShadowGateScene returns optional gate scene filter (default "").
func (h *SettingsHandler) GetSmallModelShadowGateScene() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return normalizeSmallModelShadowGateScene(h.settings.SmallModelShadowGateScene)
}

func (h *SettingsHandler) GetSmallModelSummaryEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelSummaryEnabled == nil {
		return true
	}
	return *h.settings.SmallModelSummaryEnabled
}

func (h *SettingsHandler) GetSmallModelDocExtractEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelDocExtractEnabled == nil {
		return true
	}
	return *h.settings.SmallModelDocExtractEnabled
}

func (h *SettingsHandler) GetSmallModelRerankEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRerankEnabled == nil {
		return true
	}
	return *h.settings.SmallModelRerankEnabled
}

func (h *SettingsHandler) GetSmallModelContextPruneEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelContextPruneEnabled == nil {
		return true
	}
	return *h.settings.SmallModelContextPruneEnabled
}

func (h *SettingsHandler) GetSmallModelRouteShortQAEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRouteShortQAEnabled == nil {
		return false
	}
	return *h.settings.SmallModelRouteShortQAEnabled
}

func (h *SettingsHandler) GetSmallModelRouteToolDispatchEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.SmallModelRouteToolDispatchEnabled == nil {
		return false
	}
	return *h.settings.SmallModelRouteToolDispatchEnabled
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

// SetSmallModelRouteToolDispatchEnabled updates tool-dispatch route switch and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelRouteToolDispatchEnabled(enabled bool) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelRouteToolDispatchEnabled != nil && *h.settings.SmallModelRouteToolDispatchEnabled == enabled {
		return false, nil
	}
	h.settings.SmallModelRouteToolDispatchEnabled = &enabled
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelShadowGateMinSamples updates shadow gate min-samples setting and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelShadowGateMinSamples(samples int) (bool, error) {
	if samples < 1 {
		samples = 1
	}
	if samples > 10000 {
		samples = 10000
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelShadowGateMinSamples != nil && *h.settings.SmallModelShadowGateMinSamples == samples {
		return false, nil
	}
	h.settings.SmallModelShadowGateMinSamples = &samples
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelShadowGateThresholdDelta updates shadow gate delta threshold and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelShadowGateThresholdDelta(delta float64) (bool, error) {
	if delta <= 0 {
		delta = 0.01
	}
	if delta > 1 {
		delta = 1
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelShadowGateThresholdDelta != nil && *h.settings.SmallModelShadowGateThresholdDelta == delta {
		return false, nil
	}
	h.settings.SmallModelShadowGateThresholdDelta = &delta
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelShadowGateScene updates shadow gate scene filter and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelShadowGateScene(scene string) (bool, error) {
	normalized := normalizeSmallModelShadowGateScene(scene)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelShadowGateScene == normalized {
		return false, nil
	}
	h.settings.SmallModelShadowGateScene = normalized
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

// SetSmallModelShadowRatio updates shadow ratio and persists it.
// Returns true when value changed.
func (h *SettingsHandler) SetSmallModelShadowRatio(ratio float64) (bool, error) {
	if ratio <= 0 {
		ratio = 0.01
	}
	if ratio > 1 {
		ratio = 1
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.settings.SmallModelShadowRatio != nil && *h.settings.SmallModelShadowRatio == ratio {
		return false, nil
	}
	h.settings.SmallModelShadowRatio = &ratio
	if err := h.save(); err != nil {
		return false, err
	}
	return true, nil
}

func normalizeSmallModelShadowGateScene(scene string) string {
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return ""
	}
	if len(scene) > 128 {
		scene = scene[:128]
	}
	return strings.TrimSpace(scene)
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

// load reads settings from kvstore
func (h *SettingsHandler) load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.kv.GetJSON(context.Background(), settingsKVKey, h.settings); err != nil {
		// Key not found or error — use defaults
		h.settings = &Settings{}
	}
}

func (h *SettingsHandler) loadSoulProposals() {
	h.mu.Lock()
	defer h.mu.Unlock()
	var list []SoulProposal
	if err := h.kv.GetJSON(context.Background(), soulProposalKVKey, &list); err != nil {
		h.soulProposals = map[string]*SoulProposal{}
		return
	}
	h.soulProposals = make(map[string]*SoulProposal, len(list))
	for i := range list {
		p := list[i]
		cp := p
		h.soulProposals[p.ID] = &cp
	}
}

func (h *SettingsHandler) saveSoulProposalsLocked() error {
	list := make([]SoulProposal, 0, len(h.soulProposals))
	for _, p := range h.soulProposals {
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return h.kv.SetJSON(context.Background(), soulProposalKVKey, list, 0)
}

// save writes settings to kvstore
func (h *SettingsHandler) save() error {
	return h.kv.SetJSON(context.Background(), settingsKVKey, h.settings, 0)
}
