package proxy

import (
	"bytes"
	stdjson "encoding/json"
	"io"
	"regexp"
	"strings"
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
	config     *MaskingConfig
	compiled   map[string]*regexp.Regexp
	localeFunc func() string // returns current locale (e.g. "zh-CN", "en-US")
	mu         sync.RWMutex

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

	label := dm.maskLabel()
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
		replacement := strings.ReplaceAll(rule.Replacement, "{MASKED}", label)
		replaced := re.ReplaceAllString(result, replacement)
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

// MaskBytes masks sensitive data in byte content, avoiding []byte→string→[]byte round-trips.
// Uses regexp.ReplaceAll which operates on []byte directly.
func (dm *DataMasker) MaskBytes(content []byte, direction MaskingDirection) []byte {
	if !dm.config.Enabled {
		return content
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	label := dm.maskLabel()
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
		replacement := []byte(strings.ReplaceAll(rule.Replacement, "{MASKED}", label))
		replaced := re.ReplaceAll(result, replacement)
		if !bytes.Equal(replaced, result) {
			dm.maskCount[rule.ID]++
			if dm.config.OnMask != nil {
				dm.config.OnMask(rule.ID, string(result), string(replaced))
			}
			result = replaced
		}
	}
	return result
}

// MaskRequestBytes masks sensitive data in request body bytes.
func (dm *DataMasker) MaskRequestBytes(content []byte) []byte {
	return dm.MaskBytes(content, MaskingRequest)
}

// MaskResponseBytes masks sensitive data in response body bytes.
func (dm *DataMasker) MaskResponseBytes(content []byte) []byte {
	trimmed := bytes.TrimSpace(content)
	// Keep JSON responses structurally valid: mask only string fields.
	// Raw byte-level regex replacement can corrupt numbers/booleans/null.
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') && stdjson.Valid(trimmed) {
		if masked, ok := dm.maskJSONResponseBytes(trimmed); ok {
			return masked
		}
		// Fail-safe: preserve original valid JSON if safe masking fails unexpectedly.
		return content
	}
	return dm.MaskBytes(content, MaskingResponse)
}

func (dm *DataMasker) maskJSONResponseBytes(content []byte) ([]byte, bool) {
	dec := stdjson.NewDecoder(bytes.NewReader(content))
	dec.UseNumber()

	var payload interface{}
	if err := dec.Decode(&payload); err != nil {
		return nil, false
	}
	// Reject trailing garbage; keep behavior conservative.
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, false
	}

	maskedPayload, changed := dm.maskJSONResponseValue(payload)
	if !changed {
		return content, true
	}
	out, err := stdjson.Marshal(maskedPayload)
	if err != nil || !stdjson.Valid(out) {
		return nil, false
	}
	return out, true
}

func (dm *DataMasker) maskJSONResponseValue(v interface{}) (interface{}, bool) {
	switch tv := v.(type) {
	case map[string]interface{}:
		changed := false
		for k, child := range tv {
			maskedChild, childChanged := dm.maskJSONResponseValue(child)
			if childChanged {
				tv[k] = maskedChild
				changed = true
			}
		}
		return tv, changed
	case []interface{}:
		changed := false
		for i := range tv {
			maskedChild, childChanged := dm.maskJSONResponseValue(tv[i])
			if childChanged {
				tv[i] = maskedChild
				changed = true
			}
		}
		return tv, changed
	case string:
		masked := dm.Mask(tv, MaskingResponse)
		return masked, masked != tv
	default:
		return v, false
	}
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

// SetLocaleFunc sets the function used to resolve the current locale for i18n replacement labels.
func (dm *DataMasker) SetLocaleFunc(f func() string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.localeFunc = f
}

// maskLabel returns the localized masking label, e.g. "🛡️数据脱敏" or "🛡️Data Masked".
// Covers all 27 supported locales.
var maskLabelMap = map[string]string{
	"zh": "🛡️数据脱敏",
	"ja": "🛡️データマスク",
	"ko": "🛡️데이터 마스킹",
	"de": "🛡️Daten maskiert",
	"fr": "🛡️Données masquées",
	"es": "🛡️Datos enmascarados",
	"pt": "🛡️Dados mascarados",
	"it": "🛡️Dati mascherati",
	"nl": "🛡️Gegevens gemaskeerd",
	"ru": "🛡️Данные скрыты",
	"pl": "🛡️Dane zamaskowane",
	"cs": "🛡️Data maskována",
	"sk": "🛡️Údaje maskované",
	"da": "🛡️Data maskeret",
	"sv": "🛡️Data maskerad",
	"nb": "🛡️Data maskert",
	"hu": "🛡️Adat maszkolva",
	"ro": "🛡️Date mascate",
	"hr": "🛡️Podaci maskirani",
	"el": "🛡️Δεδομένα καλυμμένα",
	"ca": "🛡️Dades emmascarades",
	"ga": "🛡️Sonraí mascaithe",
	"ml": "🛡️ഡാറ്റ മാസ്ക് ചെയ്തു",
}

func (dm *DataMasker) maskLabel() string {
	locale := ""
	if dm.localeFunc != nil {
		locale = dm.localeFunc()
	}
	// Try full locale first (e.g. "zh-CN"), then language prefix (e.g. "zh")
	if label, ok := maskLabelMap[locale]; ok {
		return label
	}
	if idx := strings.IndexByte(locale, '-'); idx > 0 {
		if label, ok := maskLabelMap[locale[:idx]]; ok {
			return label
		}
	}
	return "🛡️Data Masked"
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
	enabledRuleCount := 0
	for _, rule := range dm.config.Rules {
		if rule != nil && rule.Enabled {
			enabledRuleCount++
		}
	}
	for _, count := range dm.maskCount {
		totalMasks += count
	}

	return map[string]interface{}{
		"enabled":     dm.config.Enabled,
		"rule_count":  enabledRuleCount,
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

// GetDefaultRules returns predefined masking rules.
// Replacement text uses {MASKED} placeholder — resolved at runtime via DataMasker.maskLabel().
func GetDefaultRules() []*MaskingRule {
	return []*MaskingRule{
		// PII Rules
		{
			ID:          "email",
			Name:        "Email Address",
			Category:    MaskingPII,
			Pattern:     `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			Replacement: "【{MASKED}】[EMAIL]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},
		{
			ID:          "phone",
			Name:        "Phone Number",
			Category:    MaskingPII,
			Pattern:     `\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`,
			Replacement: "【{MASKED}】[PHONE]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},
		{
			ID:          "ssn",
			Name:        "Social Security Number",
			Category:    MaskingPII,
			Pattern:     `\b\d{3}-\d{2}-\d{4}\b`,
			Replacement: "【{MASKED}】[SSN]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},

		// Credential Rules
		{
			ID:          "api_key_format",
			Name:        "API Key (sk-/key- prefix)",
			Category:    MaskingCredentials,
			Pattern:     `\b(sk-[a-zA-Z0-9_-]{20,}|key-[a-zA-Z0-9_-]{20,})`,
			Replacement: "【{MASKED}】[API_KEY]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},
		{
			ID:          "api_key",
			Name:        "API Key (label=value)",
			Category:    MaskingCredentials,
			Pattern:     `(?i)(api[_-]?key|apikey)["\s:=]+["']?([a-zA-Z0-9_-]{20,})["']?`,
			Replacement: "【{MASKED}】[API_KEY]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},
		{
			ID:          "bearer_token",
			Name:        "Bearer Token",
			Category:    MaskingCredentials,
			Pattern:     `(?i)bearer\s+[a-zA-Z0-9_-]{20,}`,
			Replacement: "Bearer 【{MASKED}】[TOKEN]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},

		// Financial Rules
		{
			ID:          "credit_card",
			Name:        "Credit Card Number",
			Category:    MaskingFinancial,
			Pattern:     `\b(?:\d{4}[-\s]?){3}\d{4}\b`,
			Replacement: "【{MASKED}】[CARD]",
			Direction:   MaskingResponse,
			Enabled:     true,
		},
	}
}
