package proxy

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// ModelFamily represents a model series/family
type ModelFamily struct {
	Name     string   `json:"name" yaml:"name"`         // e.g., "claude-3", "gpt-4", "gemini-pro"
	Patterns []string `json:"patterns" yaml:"patterns"` // Regex patterns to match model IDs
	Provider string   `json:"provider" yaml:"provider"` // Target provider for this family
	Fallback string   `json:"fallback" yaml:"fallback"` // Fallback model for background tasks
}

// RegexRule allows expert-level model redirection
type RegexRule struct {
	Pattern     string `json:"pattern" yaml:"pattern"`         // Regex pattern to match
	Target      string `json:"target" yaml:"target"`           // Target model ID
	Provider    string `json:"provider" yaml:"provider"`       // Target provider
	Priority    int    `json:"priority" yaml:"priority"`       // Rule priority (lower = higher)
	Description string `json:"description" yaml:"description"` // Human-readable description
}

// ModelRouterConfig configuration for model routing
type ModelRouterConfig struct {
	Enabled          bool           `json:"enabled" yaml:"enabled"`
	Families         []*ModelFamily `json:"families" yaml:"families"`
	BackgroundModels []string       `json:"background_models" yaml:"background_models"` // Models for background tasks
	DefaultFamily    string         `json:"default_family" yaml:"default_family"`
	RegexCustomRules []*RegexRule   `json:"regex_rules" yaml:"regex_rules"` // Expert-level regex rules
}

// DefaultModelRouterConfig returns default model router configuration.
// Families and BackgroundModels are intentionally empty — the TierResolver
// dynamically handles model classification based on actual pricing data.
// Users can still add custom RegexCustomRules for explicit overrides.
func DefaultModelRouterConfig() *ModelRouterConfig {
	return &ModelRouterConfig{
		Enabled:          true,
		DefaultFamily:    "",
		Families:         nil,
		BackgroundModels: nil,
		RegexCustomRules: []*RegexRule{},
	}
}

// compiledRule holds a compiled regex pattern
type compiledRule struct {
	rule    *RegexRule
	pattern *regexp.Regexp
}

// compiledFamily holds compiled patterns for a family
type compiledFamily struct {
	family   *ModelFamily
	patterns []*regexp.Regexp
}

// ModelRouter handles intelligent model-based routing
type ModelRouter struct {
	config       *ModelRouterConfig
	families     map[string]*compiledFamily
	rules        []*compiledRule
	tierResolver *TierResolver
	mu           sync.RWMutex
}

// ModelRoute represents the routing decision
type ModelRoute struct {
	OriginalModel string `json:"original_model"`
	TargetModel   string `json:"target_model"`
	Provider      string `json:"provider"`
	Family        string `json:"family,omitempty"`
	RuleApplied   string `json:"rule_applied,omitempty"`
	Downgraded    bool   `json:"downgraded,omitempty"`
}

// NewModelRouter creates a new model router
func NewModelRouter(config *ModelRouterConfig) (*ModelRouter, error) {
	if config == nil {
		config = DefaultModelRouterConfig()
	}

	mr := &ModelRouter{
		config:   config,
		families: make(map[string]*compiledFamily),
		rules:    make([]*compiledRule, 0),
	}

	// Compile regex rules
	for _, rule := range config.RegexCustomRules {
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", rule.Pattern, err)
		}
		mr.rules = append(mr.rules, &compiledRule{rule: rule, pattern: compiled})
	}

	// Sort rules by priority (lower = higher priority)
	sort.Slice(mr.rules, func(i, j int) bool {
		return mr.rules[i].rule.Priority < mr.rules[j].rule.Priority
	})

	// Compile family patterns
	for _, family := range config.Families {
		cf := &compiledFamily{
			family:   family,
			patterns: make([]*regexp.Regexp, 0, len(family.Patterns)),
		}
		for _, pattern := range family.Patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid family pattern %q for %s: %w", pattern, family.Name, err)
			}
			cf.patterns = append(cf.patterns, compiled)
		}
		mr.families[family.Name] = cf
	}

	return mr, nil
}

// RouteModel determines the target provider and model for a request
func (mr *ModelRouter) RouteModel(requestedModel string, isBackground bool) (*ModelRoute, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	if !mr.config.Enabled {
		// If disabled, return the model as-is with no routing
		return &ModelRoute{
			OriginalModel: requestedModel,
			TargetModel:   requestedModel,
		}, nil
	}

	// 1. Check custom regex rules first (highest priority)
	for _, cr := range mr.rules {
		if cr.pattern.MatchString(requestedModel) {
			return &ModelRoute{
				OriginalModel: requestedModel,
				TargetModel:   cr.rule.Target,
				Provider:      cr.rule.Provider,
				RuleApplied:   cr.rule.Description,
			}, nil
		}
	}

	// 2. Match against model families
	for _, cf := range mr.families {
		for _, pattern := range cf.patterns {
			if pattern.MatchString(requestedModel) {
				targetModel := requestedModel
				downgraded := false

				// Downgrade to fallback for background tasks
				if isBackground && cf.family.Fallback != "" {
					targetModel = cf.family.Fallback
					downgraded = true
				}

				return &ModelRoute{
					OriginalModel: requestedModel,
					TargetModel:   targetModel,
					Provider:      cf.family.Provider,
					Family:        cf.family.Name,
					Downgraded:    downgraded,
				}, nil
			}
		}
	}

	// 3. TierResolver-based background downgrade
	if isBackground && mr.tierResolver != nil && mr.tierResolver.IsEnabled() {
		economyModel := mr.tierResolver.BestModelForTier(TierEconomy)
		if economyModel != "" && economyModel != requestedModel {
			return &ModelRoute{
				OriginalModel: requestedModel,
				TargetModel:   economyModel,
				RuleApplied:   "tier-background-downgrade",
				Downgraded:    true,
			}, nil
		}
	}

	// 4. No match found - return as-is
	return &ModelRoute{
		OriginalModel: requestedModel,
		TargetModel:   requestedModel,
	}, nil
}

// SetTierResolver sets the tier resolver for dynamic background downgrade.
func (mr *ModelRouter) SetTierResolver(tr *TierResolver) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.tierResolver = tr
}

// IsBackgroundRequest detects if request is a background task (e.g., title generation)
func (mr *ModelRouter) IsBackgroundRequest(r *http.Request) bool {
	// Check for common background task indicators

	// 1. X-Background-Task header
	if r.Header.Get("X-Background-Task") == "true" {
		return true
	}

	// 2. X-Request-Type header
	requestType := strings.ToLower(r.Header.Get("X-Request-Type"))
	if requestType == "background" || requestType == "async" {
		return true
	}

	// 3. Check request path for known background endpoints
	backgroundPaths := []string{"/title", "/summarize", "/embed", "/background"}
	path := strings.ToLower(r.URL.Path)
	for _, bgPath := range backgroundPaths {
		if strings.Contains(path, bgPath) {
			return true
		}
	}

	// 4. Check query parameter
	if r.URL.Query().Get("background") == "true" {
		return true
	}

	return false
}

// GetFamilies returns all configured model families
func (mr *ModelRouter) GetFamilies() []*ModelFamily {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	families := make([]*ModelFamily, 0, len(mr.config.Families))
	families = append(families, mr.config.Families...)
	return families
}

// GetRules returns all configured regex rules
func (mr *ModelRouter) GetRules() []*RegexRule {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	rules := make([]*RegexRule, 0, len(mr.config.RegexCustomRules))
	rules = append(rules, mr.config.RegexCustomRules...)
	return rules
}

// AddRule adds a new regex rule
func (mr *ModelRouter) AddRule(rule *RegexRule) error {
	compiled, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern %q: %w", rule.Pattern, err)
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.config.RegexCustomRules = append(mr.config.RegexCustomRules, rule)
	mr.rules = append(mr.rules, &compiledRule{rule: rule, pattern: compiled})

	// Re-sort rules by priority
	sort.Slice(mr.rules, func(i, j int) bool {
		return mr.rules[i].rule.Priority < mr.rules[j].rule.Priority
	})

	return nil
}

// RemoveRule removes a regex rule by pattern
func (mr *ModelRouter) RemoveRule(pattern string) bool {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Remove from config
	for i, rule := range mr.config.RegexCustomRules {
		if rule.Pattern == pattern {
			mr.config.RegexCustomRules = append(mr.config.RegexCustomRules[:i], mr.config.RegexCustomRules[i+1:]...)
			break
		}
	}

	// Remove from compiled rules
	for i, cr := range mr.rules {
		if cr.rule.Pattern == pattern {
			mr.rules = append(mr.rules[:i], mr.rules[i+1:]...)
			return true
		}
	}

	return false
}

// Stats returns model router statistics
func (mr *ModelRouter) Stats() map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	familyStats := make([]map[string]interface{}, 0, len(mr.config.Families))
	for _, family := range mr.config.Families {
		familyStats = append(familyStats, map[string]interface{}{
			"name":     family.Name,
			"provider": family.Provider,
			"fallback": family.Fallback,
			"patterns": family.Patterns,
		})
	}

	return map[string]interface{}{
		"enabled":           mr.config.Enabled,
		"default_family":    mr.config.DefaultFamily,
		"families":          familyStats,
		"regex_rules_count": len(mr.rules),
		"background_models": mr.config.BackgroundModels,
	}
}
