package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

const settingsKVKey = "config:settings"

// SettingsHandler handles user settings API endpoints
type SettingsHandler struct {
	mu                        sync.RWMutex
	kv                        kvstore.Store
	settings                  *Settings
	skillRerankerModelManager *claudecode.SkillRerankerModelManager
}

// Settings represents user preferences stored on backend
type Settings struct {
	Locale                           string   `json:"locale,omitempty"`                              // User's preferred locale (e.g., "zh-CN", "en-US")
	Timezone                         string   `json:"timezone,omitempty"`                            // User's timezone
	ThemeStyle                       string   `json:"theme_style,omitempty"`                         // Chat theme style
	SmartToolSelection               *bool    `json:"smart_tool_selection,omitempty"`                // IR-based tool filtering (nil = default true)
	SmartSkillSelection              *bool    `json:"smart_skill_selection,omitempty"`               // Progressive skill selector (nil = default true)
	SkillSelectorMode                string   `json:"skill_selector_mode,omitempty"`                 // hybrid|ir_only|llm_only
	SkillRerankEnabled               *bool    `json:"skill_rerank_enabled,omitempty"`                // Enable stage-2 rerank (nil = default true)
	SkillRerankModel                 string   `json:"skill_rerank_model,omitempty"`                  // Reranker model repo (e.g. cross-encoder/ms-marco-MiniLM-L-6-v2)
	SkillRerankONNXEnabled           *bool    `json:"skill_rerank_onnx_enabled,omitempty"`           // Enable ONNX reranker path (nil = default false)
	SkillRerankONNXAutoDownload      *bool    `json:"skill_rerank_onnx_auto_download,omitempty"`     // Allow ONNX model auto-download (nil = default false)
	SkillSelectorConfidenceThreshold *float64 `json:"skill_selector_confidence_threshold,omitempty"` // default 0.78
	MemoryRecallMode                 string   `json:"memory_recall_mode,omitempty"`                  // Memory recall strategy: aggressive|balanced|quality
	AgentMode                        *bool    `json:"agent_mode,omitempty"`                          // Autonomous agent mode (nil = default false)
	AgentAutoConfirm                 *bool    `json:"agent_auto_confirm,omitempty"`                  // Skip confirmation in agent mode (nil = default false)
	AgentAskTimeoutSeconds           *int     `json:"agent_ask_timeout_seconds,omitempty"`           // Ask timeout in seconds (default 120, range 15-1800)
	AgentAskTimeoutAction            string   `json:"agent_ask_timeout_action,omitempty"`            // default|error
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
	g.GET("/settings/skill-reranker/model/status", h.GetSkillRerankerModelStatus)
	g.POST("/settings/skill-reranker/model/download", h.StartSkillRerankerModelDownload)
	g.POST("/settings/skill-reranker/model/cancel", h.CancelSkillRerankerModelDownload)
}

// SetSkillRerankerModelManager wires ONNX skill-reranker model manager for UI download APIs.
func (h *SettingsHandler) SetSkillRerankerModelManager(mgr *claudecode.SkillRerankerModelManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.skillRerankerModelManager = mgr
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
		return "cross-encoder/ms-marco-MiniLM-L-6-v2"
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

// GetAgentMode returns whether agent mode is enabled (default true).
func (h *SettingsHandler) GetAgentMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.settings.AgentMode == nil {
		return true
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

// load reads settings from kvstore
func (h *SettingsHandler) load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.kv.GetJSON(context.Background(), settingsKVKey, h.settings); err != nil {
		// Key not found or error — use defaults
		h.settings = &Settings{}
	}
}

// save writes settings to kvstore
func (h *SettingsHandler) save() error {
	return h.kv.SetJSON(context.Background(), settingsKVKey, h.settings, 0)
}
