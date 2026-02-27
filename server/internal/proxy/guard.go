package proxy

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// GuardResult represents the result of a prompt guard check
type GuardResult struct {
	Blocked     bool     `json:"blocked"`
	Reason      string   `json:"reason,omitempty"`
	RiskLevel   string   `json:"risk_level"`
	Matches     []string `json:"matches,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// GuardRule represents a custom guard rule
type GuardRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	RiskLevel   string `json:"risk_level"` // low, medium, high
	Action      string `json:"action"`     // log, warn, block
}

// GuardConfig holds prompt guard configuration
type GuardConfig struct {
	Enabled           bool     `json:"enabled"`
	BlockOnDetection  bool     `json:"block_on_detection"`
	LogDetections     bool     `json:"log_detections"`
	CustomPatterns    []string `json:"custom_patterns"`
	WhitelistPatterns []string `json:"whitelist_patterns"`
	MaxPromptLength   int      `json:"max_prompt_length"`
}

// DefaultGuardConfig returns default guard configuration
func DefaultGuardConfig() *GuardConfig {
	return &GuardConfig{
		Enabled:          true,
		BlockOnDetection: false,
		LogDetections:    true,
		MaxPromptLength:  100000,
	}
}

// PromptGuard detects potential prompt injection attacks
type PromptGuard struct {
	config            *GuardConfig
	patterns          []*regexp.Regexp
	whitelistPatterns []*regexp.Regexp
	mu                sync.RWMutex
	detectionCount    int64
	blockedCount      int64
}

// NewPromptGuard creates a new prompt guard
func NewPromptGuard(config *GuardConfig) *PromptGuard {
	if config == nil {
		config = DefaultGuardConfig()
	}

	pg := &PromptGuard{
		config: config,
	}

	pg.compilePatterns()
	return pg
}

// compilePatterns compiles detection patterns
func (pg *PromptGuard) compilePatterns() {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	// Default injection patterns
	defaultPatterns := []string{
		// Instruction override attempts
		`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`,
		`(?i)disregard\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`,
		`(?i)forget\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`,
		`(?i)override\s+(system|previous|prior)\s+(prompt|instructions?)`,

		// Role manipulation (narrowed to suspicious personas/intent to reduce false positives)
		`(?i)you\s+are\s+now\s+(a|an)\s+(hacker|evil|malicious|unfiltered|unrestricted|jailbroken)\b`,
		`(?i)pretend\s+(you\s+are|to\s+be)\s+(a|an)\s+(hacker|evil|malicious|unfiltered|unrestricted|jailbroken)\b`,
		`(?i)act\s+as\s+(if\s+you\s+are\s+)?(a|an)\s+(hacker|evil|malicious|unfiltered|unrestricted|jailbroken)\b`,
		`(?i)roleplay\s+as\s+(a|an)\s+(hacker|evil|malicious|unfiltered|unrestricted|jailbroken)\b`,

		// System prompt extraction
		`(?i)reveal\s+(your\s+)?(system\s+)?prompt`,
		`(?i)show\s+(me\s+)?(your\s+)?(system\s+)?instructions?`,
		`(?i)what\s+(are|is)\s+(your\s+)?(system\s+)?(prompt|instructions?)`,
		`(?i)print\s+(your\s+)?(system\s+)?prompt`,

		// Jailbreak attempts
		`(?i)dan\s+mode`,
		`(?i)developer\s+mode`,
		`(?i)jailbreak`,
		`(?i)bypass\s+(safety|content|filter)`,

		// Code injection markers
		`(?i)\[system\]`,
		`(?i)\[assistant\]`,
		`(?i)\[user\]`,
		`(?i)<\|im_start\|>`,
		`(?i)<\|im_end\|>`,

		// Delimiter injection
		`(?i)###\s*(system|instruction|prompt)`,
		`(?i)---\s*(system|instruction|prompt)`,
	}

	// Compile default patterns
	pg.patterns = make([]*regexp.Regexp, 0)
	for _, p := range defaultPatterns {
		if re, err := regexp.Compile(p); err == nil {
			pg.patterns = append(pg.patterns, re)
		}
	}

	// Compile custom patterns
	for _, p := range pg.config.CustomPatterns {
		if re, err := regexp.Compile(p); err == nil {
			pg.patterns = append(pg.patterns, re)
		}
	}

	// Compile whitelist patterns
	pg.whitelistPatterns = make([]*regexp.Regexp, 0)
	for _, p := range pg.config.WhitelistPatterns {
		if re, err := regexp.Compile(p); err == nil {
			pg.whitelistPatterns = append(pg.whitelistPatterns, re)
		}
	}
}

// Check analyzes a prompt for potential injection attacks
func (pg *PromptGuard) Check(prompt string) *GuardResult {
	if !pg.config.Enabled {
		return &GuardResult{
			Blocked:   false,
			RiskLevel: "none",
		}
	}

	result := &GuardResult{
		Blocked:   false,
		RiskLevel: "low",
		Matches:   make([]string, 0),
	}

	// Check prompt length
	if pg.config.MaxPromptLength > 0 && len(prompt) > pg.config.MaxPromptLength {
		result.Blocked = pg.config.BlockOnDetection
		result.Reason = "prompt exceeds maximum length"
		result.RiskLevel = "high"
		result.Suggestions = append(result.Suggestions, "Reduce prompt length")
		pg.incrementDetection(result.Blocked)
		return result
	}

	// Check whitelist first
	pg.mu.RLock()
	for _, re := range pg.whitelistPatterns {
		if re.MatchString(prompt) {
			pg.mu.RUnlock()
			return &GuardResult{
				Blocked:   false,
				RiskLevel: "none",
			}
		}
	}

	// Check for injection patterns
	for _, re := range pg.patterns {
		if matches := re.FindAllString(prompt, -1); len(matches) > 0 {
			result.Matches = append(result.Matches, matches...)
		}
	}
	pg.mu.RUnlock()

	// Analyze results
	if len(result.Matches) > 0 {
		result.RiskLevel = pg.calculateRiskLevel(result.Matches)
		result.Reason = pg.generateReason(result.Matches)
		result.Suggestions = pg.generateSuggestions(result.Matches)

		if pg.config.BlockOnDetection && result.RiskLevel == "high" {
			result.Blocked = true
		}

		pg.incrementDetection(result.Blocked)
	}

	return result
}

func (pg *PromptGuard) generateReason(matches []string) string {
	for _, match := range matches {
		lower := strings.ToLower(match)
		if strings.Contains(lower, "ignore") ||
			strings.Contains(lower, "disregard") ||
			strings.Contains(lower, "override") ||
			strings.Contains(lower, "forget") {
			return "potential instruction override attempt detected"
		}
		if strings.Contains(lower, "system") && (strings.Contains(lower, "prompt") || strings.Contains(lower, "instruction")) {
			return "potential system prompt extraction attempt detected"
		}
		if strings.Contains(lower, "jailbreak") ||
			strings.Contains(lower, "dan mode") ||
			strings.Contains(lower, "bypass") {
			return "potential jailbreak attempt detected"
		}
	}
	return "potential prompt injection detected"
}

// calculateRiskLevel determines risk level based on matches
func (pg *PromptGuard) calculateRiskLevel(matches []string) string {
	if len(matches) >= 3 {
		return "high"
	}
	if len(matches) >= 2 {
		return "medium"
	}

	// Check for high-risk patterns
	highRiskKeywords := []string{
		"ignore", "disregard", "override", "jailbreak",
		"bypass", "system prompt", "dan mode",
	}

	for _, match := range matches {
		lower := strings.ToLower(match)
		for _, keyword := range highRiskKeywords {
			if strings.Contains(lower, keyword) {
				return "high"
			}
		}
	}

	return "medium"
}

// generateSuggestions provides suggestions based on detected patterns
func (pg *PromptGuard) generateSuggestions(matches []string) []string {
	suggestions := make([]string, 0)
	seen := make(map[string]bool)

	for _, match := range matches {
		lower := strings.ToLower(match)

		if strings.Contains(lower, "ignore") || strings.Contains(lower, "disregard") {
			if !seen["instruction"] {
				suggestions = append(suggestions, "Remove instruction override attempts")
				seen["instruction"] = true
			}
		}

		if strings.Contains(lower, "pretend") || strings.Contains(lower, "roleplay") {
			if !seen["role"] {
				suggestions = append(suggestions, "Remove role manipulation attempts")
				seen["role"] = true
			}
		}

		if strings.Contains(lower, "system") && strings.Contains(lower, "prompt") {
			if !seen["extraction"] {
				suggestions = append(suggestions, "Remove system prompt extraction attempts")
				seen["extraction"] = true
			}
		}
	}

	return suggestions
}

// incrementDetection updates detection counters
func (pg *PromptGuard) incrementDetection(blocked bool) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	pg.detectionCount++
	if blocked {
		pg.blockedCount++
	}
}

// AddPattern adds a custom detection pattern
func (pg *PromptGuard) AddPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	pg.mu.Lock()
	defer pg.mu.Unlock()

	pg.patterns = append(pg.patterns, re)
	pg.config.CustomPatterns = append(pg.config.CustomPatterns, pattern)
	return nil
}

// AddWhitelistPattern adds a whitelist pattern
func (pg *PromptGuard) AddWhitelistPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	pg.mu.Lock()
	defer pg.mu.Unlock()

	pg.whitelistPatterns = append(pg.whitelistPatterns, re)
	pg.config.WhitelistPatterns = append(pg.config.WhitelistPatterns, pattern)
	return nil
}

// Stats returns guard statistics
func (pg *PromptGuard) Stats() map[string]interface{} {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	return map[string]interface{}{
		"enabled":          pg.config.Enabled,
		"block_on_detect":  pg.config.BlockOnDetection,
		"pattern_count":    len(pg.patterns),
		"whitelist_count":  len(pg.whitelistPatterns),
		"detection_count":  pg.detectionCount,
		"blocked_count":    pg.blockedCount,
		"max_prompt_length": pg.config.MaxPromptLength,
	}
}

// GetRules returns all custom guard rules
func (pg *PromptGuard) GetRules() []GuardRule {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	rules := make([]GuardRule, 0, len(pg.config.CustomPatterns))
	for i, pattern := range pg.config.CustomPatterns {
		rules = append(rules, GuardRule{
			ID:        fmt.Sprintf("custom-%d", i),
			Name:      fmt.Sprintf("Custom Rule %d", i+1),
			Pattern:   pattern,
			Enabled:   true,
			RiskLevel: "medium",
			Action:    "log",
		})
	}
	return rules
}

// AddRule adds a custom guard rule
func (pg *PromptGuard) AddRule(rule GuardRule) error {
	if rule.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}

	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	pg.mu.Lock()
	defer pg.mu.Unlock()

	pg.patterns = append(pg.patterns, re)
	pg.config.CustomPatterns = append(pg.config.CustomPatterns, rule.Pattern)
	return nil
}
