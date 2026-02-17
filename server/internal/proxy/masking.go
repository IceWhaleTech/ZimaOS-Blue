package proxy

import (
	"regexp"
	"sync"
)

// MaskingCategory reserved masking categories
type MaskingCategory string

const (
	MaskingPII         MaskingCategory = "pii"         // Personal Identifiable Information
	MaskingCredentials MaskingCategory = "credentials" // API keys, passwords, tokens
	MaskingFinancial   MaskingCategory = "financial"   // Credit card numbers, bank accounts
	MaskingCustom      MaskingCategory = "custom"      // User-defined patterns
)

// MaskingDirection specifies when to apply masking
type MaskingDirection string

const (
	MaskingRequest  MaskingDirection = "request"
	MaskingResponse MaskingDirection = "response"
	MaskingBoth     MaskingDirection = "both"
)

// MaskingRule defines what to mask
type MaskingRule struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Category    MaskingCategory  `json:"category"`
	Pattern     string           `json:"pattern"`     // Regex pattern
	Replacement string           `json:"replacement"` // e.g., "[REDACTED]", "***"
	Direction   MaskingDirection `json:"direction"`   // request, response, both
	Enabled     bool             `json:"enabled"`
}

// MaskingConfig data masking configuration
type MaskingConfig struct {
	Enabled bool           `json:"enabled"`
	Rules   []*MaskingRule `json:"rules"`
	OnMask  func(ruleID, original, masked string)
}

// DefaultMaskingConfig returns default masking configuration
func DefaultMaskingConfig() *MaskingConfig {
	return &MaskingConfig{
		Enabled: false,
		Rules:   make([]*MaskingRule, 0),
	}
}

// DataMasker handles data masking (stub implementation)
type DataMasker struct {
	config   *MaskingConfig
	compiled map[string]*regexp.Regexp
	mu       sync.RWMutex

	// Stats
	maskCount map[string]int64
}

// NewDataMasker creates a new data masker
func NewDataMasker(config *MaskingConfig) *DataMasker {
	if config == nil {
		config = DefaultMaskingConfig()
	}

	dm := &DataMasker{
		config:    config,
		compiled:  make(map[string]*regexp.Regexp),
		maskCount: make(map[string]int64),
	}

	// Pre-compile patterns
	dm.compilePatterns()

	return dm
}

// compilePatterns compiles regex patterns
func (dm *DataMasker) compilePatterns() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, rule := range dm.config.Rules {
		if rule.Enabled && rule.Pattern != "" {
			if re, err := regexp.Compile(rule.Pattern); err == nil {
				dm.compiled[rule.ID] = re
			}
		}
	}
}

// Mask masks sensitive data based on direction
func (dm *DataMasker) Mask(content string, direction MaskingDirection) string {
	if !dm.config.Enabled {
		return content
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := content
	for _, rule := range dm.config.Rules {
		if !rule.Enabled {
			continue
		}
		if rule.Direction != MaskingBoth && rule.Direction != direction {
			continue
		}
		re, ok := dm.compiled[rule.ID]
		if !ok {
			continue
		}
		replaced := re.ReplaceAllString(result, rule.Replacement)
		if replaced != result {
			dm.maskCount[rule.ID]++
			if dm.config.OnMask != nil {
				dm.config.OnMask(rule.ID, result, replaced)
			}
			result = replaced
		}
	}

	return result
}

// MaskRequest masks sensitive data in request
func (dm *DataMasker) MaskRequest(content string) string {
	return dm.Mask(content, MaskingRequest)
}

// MaskResponse masks sensitive data in response
func (dm *DataMasker) MaskResponse(content string) string {
	return dm.Mask(content, MaskingResponse)
}

// AddRule adds a masking rule
func (dm *DataMasker) AddRule(rule *MaskingRule) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Validate pattern
	if rule.Pattern != "" {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return err
		}
		dm.compiled[rule.ID] = re
	}

	dm.config.Rules = append(dm.config.Rules, rule)
	return nil
}

// RemoveRule removes a masking rule
func (dm *DataMasker) RemoveRule(id string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for i, rule := range dm.config.Rules {
		if rule.ID == id {
			dm.config.Rules = append(dm.config.Rules[:i], dm.config.Rules[i+1:]...)
			delete(dm.compiled, id)
			delete(dm.maskCount, id)
			return true
		}
	}
	return false
}

// SetRuleEnabled enables or disables a rule
func (dm *DataMasker) SetRuleEnabled(id string, enabled bool) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, rule := range dm.config.Rules {
		if rule.ID == id {
			rule.Enabled = enabled
			return true
		}
	}
	return false
}

// GetRule returns a masking rule by ID
func (dm *DataMasker) GetRule(id string) (*MaskingRule, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, rule := range dm.config.Rules {
		if rule.ID == id {
			copy := *rule
			return &copy, true
		}
	}
	return nil, false
}

// ListRules returns all masking rules
func (dm *DataMasker) ListRules() []*MaskingRule {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	rules := make([]*MaskingRule, len(dm.config.Rules))
	for i, rule := range dm.config.Rules {
		copy := *rule
		rules[i] = &copy
	}
	return rules
}

// SetEnabled enables or disables masking globally
func (dm *DataMasker) SetEnabled(enabled bool) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.config.Enabled = enabled
}

// IsEnabled returns whether masking is enabled
func (dm *DataMasker) IsEnabled() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.config.Enabled
}

// Stats returns masker statistics
func (dm *DataMasker) Stats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	totalMasks := int64(0)
	for _, count := range dm.maskCount {
		totalMasks += count
	}

	return map[string]interface{}{
		"enabled":     dm.config.Enabled,
		"rule_count":  len(dm.config.Rules),
		"total_masks": totalMasks,
		"mask_counts": dm.maskCount,
		"status":      "active",
	}
}

// ResetStats resets mask counters
func (dm *DataMasker) ResetStats() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.maskCount = make(map[string]int64)
}

// GetDefaultRules returns predefined masking rules (for future use)
func GetDefaultRules() []*MaskingRule {
	return []*MaskingRule{
		// PII Rules
		{
			ID:          "email",
			Name:        "Email Address",
			Category:    MaskingPII,
			Pattern:     `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			Replacement: "[EMAIL]",
			Direction:   MaskingBoth,
			Enabled:     false,
		},
		{
			ID:          "phone",
			Name:        "Phone Number",
			Category:    MaskingPII,
			Pattern:     `\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`,
			Replacement: "[PHONE]",
			Direction:   MaskingBoth,
			Enabled:     false,
		},
		{
			ID:          "ssn",
			Name:        "Social Security Number",
			Category:    MaskingPII,
			Pattern:     `\b\d{3}-\d{2}-\d{4}\b`,
			Replacement: "[SSN]",
			Direction:   MaskingBoth,
			Enabled:     false,
		},

		// Credential Rules
		{
			ID:          "api_key",
			Name:        "API Key",
			Category:    MaskingCredentials,
			Pattern:     `(?i)(api[_-]?key|apikey)["\s:=]+["']?([a-zA-Z0-9_-]{20,})["']?`,
			Replacement: "[API_KEY]",
			Direction:   MaskingBoth,
			Enabled:     false,
		},
		{
			ID:          "bearer_token",
			Name:        "Bearer Token",
			Category:    MaskingCredentials,
			Pattern:     `(?i)bearer\s+[a-zA-Z0-9_-]{20,}`,
			Replacement: "Bearer [TOKEN]",
			Direction:   MaskingBoth,
			Enabled:     false,
		},

		// Financial Rules
		{
			ID:          "credit_card",
			Name:        "Credit Card Number",
			Category:    MaskingFinancial,
			Pattern:     `\b(?:\d{4}[-\s]?){3}\d{4}\b`,
			Replacement: "[CARD]",
			Direction:   MaskingBoth,
			Enabled:     false,
		},
	}
}
