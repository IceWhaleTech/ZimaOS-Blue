package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/labstack/echo/v4"
)

const (
	promptFirewallKVKey = "config:security_firewall"

	promptFirewallRuleTypeBuiltin = "builtin"
	promptFirewallRuleTypeCustom  = "custom"

	promptFirewallBuiltinRoleInjection       = "builtin_role_injection"
	promptFirewallBuiltinInstructionOverride = "builtin_instruction_override"
	promptFirewallBuiltinDelimiterAttacks    = "builtin_delimiter_attacks"
	promptFirewallBuiltinEncodingAttacks     = "builtin_encoding_attacks"
	promptFirewallBuiltinJailbreakPatterns   = "builtin_jailbreak_patterns"
	promptFirewallBuiltinDataExfiltration    = "builtin_data_exfiltration"
	promptFirewallBuiltinInputLengthGuard    = "builtin_input_length_guard"

	promptFirewallDefaultMaxInputLength = promptguard.DefaultMaxInputLengthChars
)

type promptFirewallBuiltinRule struct {
	ID          string
	Keyword     string
	Description string
}

var promptFirewallBuiltinRules = []promptFirewallBuiltinRule{
	{
		ID:          promptFirewallBuiltinRoleInjection,
		Keyword:     "Role Injection",
		Description: "Detects role prefix and role-switch attempts (system/assistant/user).",
	},
	{
		ID:          promptFirewallBuiltinInstructionOverride,
		Keyword:     "Instruction Override",
		Description: "Blocks attempts to bypass or disable system safety restrictions.",
	},
	{
		ID:          promptFirewallBuiltinDelimiterAttacks,
		Keyword:     "Delimiter Attack",
		Description: "Detects markdown/XML/JSON delimiter tricks used to inject instructions.",
	},
	{
		ID:          promptFirewallBuiltinEncodingAttacks,
		Keyword:     "Encoding Attack",
		Description: "Detects suspicious Base64/Unicode/URL encoding patterns.",
	},
	{
		ID:          promptFirewallBuiltinJailbreakPatterns,
		Keyword:     "Jailbreak Pattern",
		Description: "Detects known jailbreak and unrestricted-role prompts.",
	},
	{
		ID:          promptFirewallBuiltinDataExfiltration,
		Keyword:     "Prompt Exfiltration",
		Description: "Detects attempts to reveal or repeat hidden system instructions.",
	},
	{
		ID:          promptFirewallBuiltinInputLengthGuard,
		Keyword:     "Input Length Guard",
		Description: "Blocks oversized prompts that may be used for context stuffing attacks.",
	},
}

// PromptFirewallRule represents one user-maintained prompt interception rule.
type PromptFirewallRule struct {
	ID          string    `json:"id"`
	Keyword     string    `json:"keyword"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	Type        string    `json:"type,omitempty"`
	BuiltIn     bool      `json:"built_in,omitempty"`
	Description string    `json:"description,omitempty"`
}

// PromptFirewallConfig represents prompt firewall settings.
type PromptFirewallConfig struct {
	Enabled          bool                 `json:"enabled"`
	Rules            []PromptFirewallRule `json:"rules"`
	BuiltinRuleState map[string]bool      `json:"builtin_rule_state,omitempty"`
}

type promptFirewallFile struct {
	Enabled          *bool                `json:"enabled,omitempty"`
	Rules            []PromptFirewallRule `json:"rules"`
	BuiltinRuleState map[string]bool      `json:"builtin_rule_state,omitempty"`
}

// PromptFirewallConfigResponse is the API payload for firewall settings.
type PromptFirewallConfigResponse struct {
	Enabled   bool                 `json:"enabled"`
	Rules     []PromptFirewallRule `json:"rules"`
	RuleCount int                  `json:"rule_count"`
}

type UpdatePromptFirewallRequest struct {
	Enabled *bool `json:"enabled"`
}

type AddPromptFirewallRuleRequest struct {
	Keyword string `json:"keyword"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type UpdatePromptFirewallRuleRequest struct {
	Keyword *string `json:"keyword,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// SetPromptGuard wires chat promptguard detector into security handler for firewall management.
func (h *Handler) SetPromptGuard(detector *promptguard.Detector) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.promptGuard = detector
	if h.scanner != nil && h.scanner.config != nil {
		h.scanner.config.PromptGuardEnabled = detector != nil
	}
	h.ensurePromptFirewallBuiltinStateLocked()
	h.applyPromptFirewallLocked()
}

// GetPromptFirewall handles GET /api/v1/security/firewall.
func (h *Handler) GetPromptFirewall(c echo.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return c.JSON(http.StatusOK, h.promptFirewallResponseLocked())
}

// UpdatePromptFirewall handles PUT /api/v1/security/firewall.
func (h *Handler) UpdatePromptFirewall(c echo.Context) error {
	var req UpdatePromptFirewallRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Enabled == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "enabled is required")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.ensurePromptFirewallBuiltinStateLocked()

	h.firewall.Enabled = *req.Enabled
	h.applyPromptFirewallLocked()
	if err := h.savePromptFirewallLocked(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to save firewall config: %v", err))
	}

	return c.JSON(http.StatusOK, h.promptFirewallResponseLocked())
}

// AddPromptFirewallRule handles POST /api/v1/security/firewall/rules.
func (h *Handler) AddPromptFirewallRule(c echo.Context) error {
	var req AddPromptFirewallRuleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "keyword is required")
	}
	if len(keyword) > 256 {
		return echo.NewHTTPError(http.StatusBadRequest, "keyword is too long")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.ensurePromptFirewallBuiltinStateLocked()

	if h.hasPromptFirewallKeywordLocked(keyword, "") {
		return echo.NewHTTPError(http.StatusConflict, "keyword already exists")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := PromptFirewallRule{
		ID:        generatePromptFirewallRuleID(),
		Keyword:   keyword,
		Enabled:   enabled,
		CreatedAt: timeutil.NowTime(),
	}
	h.firewall.Rules = append(h.firewall.Rules, rule)

	h.applyPromptFirewallLocked()
	if err := h.savePromptFirewallLocked(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to save firewall config: %v", err))
	}

	return c.JSON(http.StatusCreated, h.promptFirewallResponseLocked())
}

// UpdatePromptFirewallRule handles PUT /api/v1/security/firewall/rules/:id.
func (h *Handler) UpdatePromptFirewallRule(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	var req UpdatePromptFirewallRuleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Keyword == nil && req.Enabled == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "nothing to update")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.ensurePromptFirewallBuiltinStateLocked()

	if _, ok := promptFirewallBuiltinRuleByID(id); ok {
		if req.Keyword != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "built-in rule keyword cannot be modified")
		}
		if req.Enabled == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "enabled is required for built-in rule")
		}
		h.firewall.BuiltinRuleState[id] = *req.Enabled
		h.applyPromptFirewallLocked()
		if err := h.savePromptFirewallLocked(); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to save firewall config: %v", err))
		}
		return c.JSON(http.StatusOK, h.promptFirewallResponseLocked())
	}

	idx := h.promptFirewallRuleIndexLocked(id)
	if idx < 0 {
		return echo.NewHTTPError(http.StatusNotFound, "rule not found")
	}

	if req.Keyword != nil {
		keyword := strings.TrimSpace(*req.Keyword)
		if keyword == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "keyword cannot be empty")
		}
		if len(keyword) > 256 {
			return echo.NewHTTPError(http.StatusBadRequest, "keyword is too long")
		}
		if h.hasPromptFirewallKeywordLocked(keyword, id) {
			return echo.NewHTTPError(http.StatusConflict, "keyword already exists")
		}
		h.firewall.Rules[idx].Keyword = keyword
	}

	if req.Enabled != nil {
		h.firewall.Rules[idx].Enabled = *req.Enabled
	}

	h.applyPromptFirewallLocked()
	if err := h.savePromptFirewallLocked(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to save firewall config: %v", err))
	}

	return c.JSON(http.StatusOK, h.promptFirewallResponseLocked())
}

// DeletePromptFirewallRule handles DELETE /api/v1/security/firewall/rules/:id.
func (h *Handler) DeletePromptFirewallRule(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.ensurePromptFirewallBuiltinStateLocked()

	if _, ok := promptFirewallBuiltinRuleByID(id); ok {
		return echo.NewHTTPError(http.StatusBadRequest, "built-in rule cannot be deleted")
	}

	idx := h.promptFirewallRuleIndexLocked(id)
	if idx < 0 {
		return echo.NewHTTPError(http.StatusNotFound, "rule not found")
	}

	h.firewall.Rules = append(h.firewall.Rules[:idx], h.firewall.Rules[idx+1:]...)
	h.applyPromptFirewallLocked()
	if err := h.savePromptFirewallLocked(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to save firewall config: %v", err))
	}

	return c.JSON(http.StatusOK, h.promptFirewallResponseLocked())
}

func (h *Handler) promptFirewallResponseLocked() PromptFirewallConfigResponse {
	h.ensurePromptFirewallBuiltinStateLocked()

	rules := make([]PromptFirewallRule, 0, len(promptFirewallBuiltinRules)+len(h.firewall.Rules))
	for _, builtin := range promptFirewallBuiltinRules {
		rules = append(rules, PromptFirewallRule{
			ID:          builtin.ID,
			Keyword:     builtin.Keyword,
			Enabled:     h.firewall.BuiltinRuleState[builtin.ID],
			Type:        promptFirewallRuleTypeBuiltin,
			BuiltIn:     true,
			Description: builtin.Description,
		})
	}

	customRules := make([]PromptFirewallRule, len(h.firewall.Rules))
	copy(customRules, h.firewall.Rules)
	sort.SliceStable(customRules, func(i, j int) bool {
		if customRules[i].CreatedAt.IsZero() || customRules[j].CreatedAt.IsZero() {
			return false
		}
		return customRules[i].CreatedAt.After(customRules[j].CreatedAt)
	})
	for i := range customRules {
		customRules[i].Type = promptFirewallRuleTypeCustom
		customRules[i].BuiltIn = false
		customRules[i].Description = ""
	}
	rules = append(rules, customRules...)

	return PromptFirewallConfigResponse{
		Enabled:   h.firewall.Enabled,
		Rules:     rules,
		RuleCount: len(rules),
	}
}

func (h *Handler) promptFirewallRuleIndexLocked(id string) int {
	for i := range h.firewall.Rules {
		if h.firewall.Rules[i].ID == id {
			return i
		}
	}
	return -1
}

func (h *Handler) hasPromptFirewallKeywordLocked(keyword string, excludeID string) bool {
	normalized := strings.ToLower(strings.TrimSpace(keyword))
	for _, rule := range h.firewall.Rules {
		if excludeID != "" && rule.ID == excludeID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(rule.Keyword)) == normalized {
			return true
		}
	}
	return false
}

func generatePromptFirewallRuleID() string {
	return "firewall_" + strconv.FormatInt(timeutil.NowNano(), 36)
}

func (h *Handler) applyPromptFirewallLocked() {
	if h.promptGuard == nil {
		return
	}
	h.ensurePromptFirewallBuiltinStateLocked()

	config := clonePromptGuardConfig(h.promptGuard.GetConfig())
	baseCustom := make([]promptguard.PatternRule, 0, len(config.CustomPatterns))
	for _, rule := range config.CustomPatterns {
		if strings.HasPrefix(rule.Name, "firewall_rule_") {
			continue
		}
		baseCustom = append(baseCustom, rule)
	}
	h.applyPromptFirewallBuiltinConfigLocked(config)
	config.CustomPatterns = append(baseCustom, h.buildPromptFirewallPatternsLocked()...)
	h.promptGuard.UpdateConfig(config)
}

func clonePromptGuardConfig(cfg *promptguard.DetectorConfig) *promptguard.DetectorConfig {
	if cfg == nil {
		return promptguard.DefaultDetectorConfig()
	}

	customPatterns := make([]promptguard.PatternRule, len(cfg.CustomPatterns))
	copy(customPatterns, cfg.CustomPatterns)

	return &promptguard.DetectorConfig{
		EnableRoleInjection:       cfg.EnableRoleInjection,
		EnableInstructionOverride: cfg.EnableInstructionOverride,
		EnableDelimiterAttacks:    cfg.EnableDelimiterAttacks,
		EnableEncodingAttacks:     cfg.EnableEncodingAttacks,
		EnableJailbreakPatterns:   cfg.EnableJailbreakPatterns,
		EnableDataExfiltration:    cfg.EnableDataExfiltration,
		CustomPatterns:            customPatterns,
		BlockThreshold:            cfg.BlockThreshold,
		MaxInputLength:            cfg.MaxInputLength,
	}
}

func (h *Handler) buildPromptFirewallPatternsLocked() []promptguard.PatternRule {
	if !h.firewall.Enabled {
		return []promptguard.PatternRule{}
	}

	patterns := make([]promptguard.PatternRule, 0, len(h.firewall.Rules))
	for _, rule := range h.firewall.Rules {
		if !rule.Enabled {
			continue
		}
		keyword := strings.TrimSpace(rule.Keyword)
		if keyword == "" {
			continue
		}
		patterns = append(patterns, promptguard.PatternRule{
			Name:        "firewall_rule_" + rule.ID,
			Pattern:     "(?i)" + regexp.QuoteMeta(keyword),
			Severity:    promptguard.ThreatCritical,
			Description: "Prompt firewall blocked keyword",
		})
	}

	return patterns
}

func (h *Handler) applyPromptFirewallBuiltinConfigLocked(config *promptguard.DetectorConfig) {
	enabled := h.firewall.Enabled

	config.EnableRoleInjection = enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinRoleInjection]
	config.EnableInstructionOverride = enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinInstructionOverride]
	config.EnableDelimiterAttacks = enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinDelimiterAttacks]
	config.EnableEncodingAttacks = enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinEncodingAttacks]
	config.EnableJailbreakPatterns = enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinJailbreakPatterns]
	config.EnableDataExfiltration = enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinDataExfiltration]

	if enabled && h.firewall.BuiltinRuleState[promptFirewallBuiltinInputLengthGuard] {
		if config.MaxInputLength <= 0 {
			config.MaxInputLength = promptFirewallDefaultMaxInputLength
		}
	} else {
		config.MaxInputLength = 0
	}
}

func (h *Handler) promptFirewallPathLocked() string {
	base := strings.TrimSpace(h.dataDir)
	if base == "" {
		base = "./data"
	}
	return filepath.Join(base, "security", "firewall_rules.json")
}

func (h *Handler) loadPromptFirewallLocked() {
	if h.loadPromptFirewallFromKVLocked() {
		return
	}

	path := h.promptFirewallPathLocked()
	data, err := os.ReadFile(path)
	if err != nil {
		h.ensurePromptFirewallBuiltinStateLocked()
		return
	}

	var file promptFirewallFile
	if err := json.Unmarshal(data, &file); err != nil {
		return
	}
	h.applyPromptFirewallFileLocked(&file)
	changed := h.enforcePromptFirewallAllOnLocked()
	if h.kv != nil || changed {
		if err := h.savePromptFirewallLocked(); err == nil && h.kv != nil {
			archivePromptFirewallLegacyFile(path)
		}
	}
}

func (h *Handler) loadPromptFirewallFromKVLocked() bool {
	if h.kv == nil {
		return false
	}

	var file promptFirewallFile
	if err := h.kv.GetJSON(context.Background(), promptFirewallKVKey, &file); err != nil {
		return false
	}
	h.applyPromptFirewallFileLocked(&file)
	if h.enforcePromptFirewallAllOnLocked() {
		_ = h.savePromptFirewallLocked()
	}
	return true
}

func (h *Handler) applyPromptFirewallFileLocked(file *promptFirewallFile) {
	if file == nil {
		return
	}
	h.firewall.Rules = normalizePromptFirewallRules(file.Rules)
	h.firewall.BuiltinRuleState = normalizePromptFirewallBuiltinRuleState(file.BuiltinRuleState)
	if file.Enabled != nil {
		h.firewall.Enabled = *file.Enabled
	}
}

func (h *Handler) enforcePromptFirewallAllOnLocked() bool {
	changed := false

	if !h.firewall.Enabled {
		h.firewall.Enabled = true
		changed = true
	}

	for i := range h.firewall.Rules {
		if !h.firewall.Rules[i].Enabled {
			h.firewall.Rules[i].Enabled = true
			changed = true
		}
	}

	forcedBuiltin := defaultPromptFirewallBuiltinRuleState()
	for _, rule := range promptFirewallBuiltinRules {
		if enabled, ok := h.firewall.BuiltinRuleState[rule.ID]; !ok || !enabled {
			changed = true
		}
	}
	h.firewall.BuiltinRuleState = forcedBuiltin

	return changed
}

func normalizePromptFirewallRules(rules []PromptFirewallRule) []PromptFirewallRule {
	if len(rules) == 0 {
		return []PromptFirewallRule{}
	}

	normalized := make([]PromptFirewallRule, 0, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		keyword := strings.TrimSpace(rule.Keyword)
		if keyword == "" {
			continue
		}
		id := strings.TrimSpace(rule.ID)
		if id == "" {
			id = generatePromptFirewallRuleID()
		}
		if _, exists := seen[id]; exists {
			id = generatePromptFirewallRuleID()
		}
		seen[id] = struct{}{}

		normalized = append(normalized, PromptFirewallRule{
			ID:          id,
			Keyword:     keyword,
			Enabled:     rule.Enabled,
			CreatedAt:   rule.CreatedAt,
			Type:        "",
			BuiltIn:     false,
			Description: "",
		})
	}
	return normalized
}

func normalizePromptFirewallBuiltinRuleState(state map[string]bool) map[string]bool {
	normalized := defaultPromptFirewallBuiltinRuleState()
	if len(state) == 0 {
		return normalized
	}
	for _, rule := range promptFirewallBuiltinRules {
		if enabled, ok := state[rule.ID]; ok {
			normalized[rule.ID] = enabled
		}
	}
	return normalized
}

func defaultPromptFirewallBuiltinRuleState() map[string]bool {
	state := make(map[string]bool, len(promptFirewallBuiltinRules))
	for _, rule := range promptFirewallBuiltinRules {
		state[rule.ID] = true
	}
	return state
}

func promptFirewallBuiltinRuleByID(id string) (promptFirewallBuiltinRule, bool) {
	for _, rule := range promptFirewallBuiltinRules {
		if rule.ID == id {
			return rule, true
		}
	}
	return promptFirewallBuiltinRule{}, false
}

func (h *Handler) ensurePromptFirewallBuiltinStateLocked() {
	h.firewall.BuiltinRuleState = normalizePromptFirewallBuiltinRuleState(h.firewall.BuiltinRuleState)
}

func (h *Handler) savePromptFirewallLocked() error {
	h.ensurePromptFirewallBuiltinStateLocked()

	enabled := h.firewall.Enabled
	payload := promptFirewallFile{
		Enabled:          &enabled,
		Rules:            h.firewall.Rules,
		BuiltinRuleState: h.firewall.BuiltinRuleState,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	if h.kv != nil {
		return h.kv.SetJSON(context.Background(), promptFirewallKVKey, &payload, 0)
	}

	path := h.promptFirewallPathLocked()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func archivePromptFirewallLegacyFile(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	archivedPath := path + ".migrated"
	if _, err := os.Stat(archivedPath); err == nil {
		return
	}
	_ = os.Rename(path, archivedPath)
}
