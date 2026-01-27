// Package promptguard provides protection against prompt injection attacks.
package promptguard

import (
	"regexp"
	"sync"
)

// OutputFilterConfig holds configuration for output filtering.
type OutputFilterConfig struct {
	// EnableSensitiveDataFilter filters sensitive data from output.
	EnableSensitiveDataFilter bool
	// EnableSystemPromptLeakFilter prevents system prompt leakage.
	EnableSystemPromptLeakFilter bool
	// EnableCodeInjectionFilter filters potentially dangerous code.
	EnableCodeInjectionFilter bool
	// EnablePIIFilter filters personally identifiable information.
	EnablePIIFilter bool
	// CustomFilters allows adding custom output filters.
	CustomFilters []OutputFilterRule
	// SystemPromptPatterns are patterns from the system prompt to detect leakage.
	SystemPromptPatterns []string
	// MaxOutputLength is the maximum allowed output length (0 = unlimited).
	MaxOutputLength int
}

// OutputFilterRule defines a custom output filter rule.
type OutputFilterRule struct {
	// Name is the name of the rule.
	Name string
	// Pattern is the regex pattern to match.
	Pattern string
	// Replacement is the replacement text (empty = redact).
	Replacement string
	// Description describes what this filter does.
	Description string
}

// OutputFilterResult contains the result of output filtering.
type OutputFilterResult struct {
	// WasFiltered indicates if any content was filtered.
	WasFiltered bool `json:"was_filtered"`
	// FilteredOutput is the filtered output.
	FilteredOutput string `json:"filtered_output"`
	// Violations contains the list of filter violations.
	Violations []FilterViolation `json:"violations,omitempty"`
}

// FilterViolation represents a single filter violation.
type FilterViolation struct {
	// FilterName is the name of the filter that was triggered.
	FilterName string `json:"filter_name"`
	// Match is the matched content (may be redacted).
	Match string `json:"match"`
	// Position is the position in the output.
	Position int `json:"position"`
	// Action is the action taken (redacted, replaced, blocked).
	Action string `json:"action"`
}

// DefaultOutputFilterConfig returns the default output filter configuration.
func DefaultOutputFilterConfig() *OutputFilterConfig {
	return &OutputFilterConfig{
		EnableSensitiveDataFilter:    true,
		EnableSystemPromptLeakFilter: true,
		EnableCodeInjectionFilter:    true,
		EnablePIIFilter:              false, // Disabled by default for performance
		MaxOutputLength:              500000,
	}
}

// OutputFilter filters LLM output for sensitive or dangerous content.
type OutputFilter struct {
	config   *OutputFilterConfig
	patterns map[string][]*compiledOutputPattern
	mu       sync.RWMutex
}

type compiledOutputPattern struct {
	rule  OutputFilterRule
	regex *regexp.Regexp
}

// NewOutputFilter creates a new output filter.
func NewOutputFilter(config *OutputFilterConfig) *OutputFilter {
	if config == nil {
		config = DefaultOutputFilterConfig()
	}

	f := &OutputFilter{
		config:   config,
		patterns: make(map[string][]*compiledOutputPattern),
	}

	f.initPatterns()
	return f
}

// initPatterns initializes the output filter patterns.
func (f *OutputFilter) initPatterns() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Sensitive data patterns
	if f.config.EnableSensitiveDataFilter {
		f.patterns["sensitive_data"] = f.compilePatterns([]OutputFilterRule{
			{Name: "api_key", Pattern: `(?i)(api[_-]?key|secret[_-]?key|access[_-]?token)\s*[=:]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`, Replacement: "[API_KEY_REDACTED]", Description: "API key pattern"},
			{Name: "password", Pattern: `(?i)(password|passwd|pwd)\s*[=:]\s*['"]?[^\s'"]{8,}['"]?`, Replacement: "[PASSWORD_REDACTED]", Description: "Password pattern"},
			{Name: "private_key", Pattern: `-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(RSA\s+)?PRIVATE\s+KEY-----`, Replacement: "[PRIVATE_KEY_REDACTED]", Description: "Private key pattern"},
			{Name: "aws_key", Pattern: `(?i)AKIA[0-9A-Z]{16}`, Replacement: "[AWS_KEY_REDACTED]", Description: "AWS access key"},
			{Name: "jwt_token", Pattern: `eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`, Replacement: "[JWT_REDACTED]", Description: "JWT token"},
			{Name: "connection_string", Pattern: `(?i)(mongodb|mysql|postgres|redis)://[^\s]+`, Replacement: "[CONNECTION_STRING_REDACTED]", Description: "Database connection string"},
		})
	}

	// System prompt leak patterns
	if f.config.EnableSystemPromptLeakFilter {
		f.patterns["system_prompt_leak"] = f.compilePatterns([]OutputFilterRule{
			{Name: "system_instruction", Pattern: `(?i)(my\s+)?system\s+(prompt|instructions?)\s+(is|are|says?)\s*:`, Replacement: "[SYSTEM_INFO_REDACTED]", Description: "System instruction leak"},
			{Name: "initial_prompt", Pattern: `(?i)(initial|original|base)\s+prompt\s*:`, Replacement: "[PROMPT_REDACTED]", Description: "Initial prompt leak"},
			{Name: "rules_disclosure", Pattern: `(?i)(my|the)\s+rules?\s+(are|is|include)\s*:`, Replacement: "[RULES_REDACTED]", Description: "Rules disclosure"},
			{Name: "instruction_list", Pattern: `(?i)i\s+(was|am)\s+(told|instructed|programmed)\s+to\s*:`, Replacement: "[INSTRUCTION_REDACTED]", Description: "Instruction list leak"},
		})
	}

	// Code injection patterns
	if f.config.EnableCodeInjectionFilter {
		f.patterns["code_injection"] = f.compilePatterns([]OutputFilterRule{
			{Name: "shell_command", Pattern: `(?i)(rm\s+-rf|sudo\s+rm|chmod\s+777|curl\s+.*\|\s*sh|wget\s+.*\|\s*sh)`, Replacement: "[DANGEROUS_COMMAND_BLOCKED]", Description: "Dangerous shell command"},
			{Name: "sql_injection", Pattern: `(?i)(DROP\s+TABLE|DELETE\s+FROM|TRUNCATE\s+TABLE|UPDATE\s+.*SET.*WHERE\s+1\s*=\s*1)`, Replacement: "[SQL_BLOCKED]", Description: "SQL injection pattern"},
			{Name: "script_tag", Pattern: `<script[^>]*>[\s\S]*?</script>`, Replacement: "[SCRIPT_BLOCKED]", Description: "Script tag"},
			{Name: "eval_exec", Pattern: `(?i)(eval|exec|system|shell_exec|passthru)\s*\(`, Replacement: "[EXEC_BLOCKED]", Description: "Code execution function"},
		})
	}

	// PII patterns
	if f.config.EnablePIIFilter {
		f.patterns["pii"] = f.compilePatterns([]OutputFilterRule{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, Replacement: "[EMAIL_REDACTED]", Description: "Email address"},
			{Name: "phone", Pattern: `(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}`, Replacement: "[PHONE_REDACTED]", Description: "Phone number"},
			{Name: "ssn", Pattern: `\b\d{3}[-\s]?\d{2}[-\s]?\d{4}\b`, Replacement: "[SSN_REDACTED]", Description: "Social Security Number"},
			{Name: "credit_card", Pattern: `\b(?:\d{4}[-\s]?){3}\d{4}\b`, Replacement: "[CC_REDACTED]", Description: "Credit card number"},
			{Name: "ip_address", Pattern: `\b(?:\d{1,3}\.){3}\d{1,3}\b`, Replacement: "[IP_REDACTED]", Description: "IP address"},
		})
	}

	// Add custom patterns
	if len(f.config.CustomFilters) > 0 {
		f.patterns["custom"] = f.compilePatterns(f.config.CustomFilters)
	}

	// Add system prompt patterns if provided
	if len(f.config.SystemPromptPatterns) > 0 {
		var rules []OutputFilterRule
		for i, pattern := range f.config.SystemPromptPatterns {
			rules = append(rules, OutputFilterRule{
				Name:        "system_prompt_" + string(rune('0'+i)),
				Pattern:     regexp.QuoteMeta(pattern),
				Replacement: "[REDACTED]",
				Description: "System prompt content",
			})
		}
		f.patterns["system_prompt_content"] = f.compilePatterns(rules)
	}
}

// compilePatterns compiles a list of output filter rules.
func (f *OutputFilter) compilePatterns(rules []OutputFilterRule) []*compiledOutputPattern {
	patterns := make([]*compiledOutputPattern, 0, len(rules))
	for _, rule := range rules {
		if regex, err := regexp.Compile(rule.Pattern); err == nil {
			patterns = append(patterns, &compiledOutputPattern{
				rule:  rule,
				regex: regex,
			})
		}
	}
	return patterns
}

// Filter filters the output and returns the result.
func (f *OutputFilter) Filter(output string) *OutputFilterResult {
	result := &OutputFilterResult{
		FilteredOutput: output,
		Violations:     make([]FilterViolation, 0),
	}

	// Check output length
	if f.config.MaxOutputLength > 0 && len(output) > f.config.MaxOutputLength {
		result.WasFiltered = true
		result.FilteredOutput = output[:f.config.MaxOutputLength] + "\n[OUTPUT_TRUNCATED]"
		result.Violations = append(result.Violations, FilterViolation{
			FilterName: "max_length",
			Match:      "output exceeded maximum length",
			Action:     "truncated",
		})
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Apply all filters
	for category, patterns := range f.patterns {
		for _, cp := range patterns {
			matches := cp.regex.FindAllStringIndex(result.FilteredOutput, -1)
			if len(matches) > 0 {
				result.WasFiltered = true

				// Record violations
				for _, match := range matches {
					matchText := result.FilteredOutput[match[0]:match[1]]
					// Truncate match for logging
					if len(matchText) > 50 {
						matchText = matchText[:50] + "..."
					}
					result.Violations = append(result.Violations, FilterViolation{
						FilterName: category + "/" + cp.rule.Name,
						Match:      matchText,
						Position:   match[0],
						Action:     "replaced",
					})
				}

				// Apply replacement
				replacement := cp.rule.Replacement
				if replacement == "" {
					replacement = "[REDACTED]"
				}
				result.FilteredOutput = cp.regex.ReplaceAllString(result.FilteredOutput, replacement)
			}
		}
	}

	return result
}

// AddSystemPromptPattern adds a pattern from the system prompt to detect leakage.
func (f *OutputFilter) AddSystemPromptPattern(pattern string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if regex, err := regexp.Compile(regexp.QuoteMeta(pattern)); err == nil {
		f.patterns["system_prompt_content"] = append(f.patterns["system_prompt_content"], &compiledOutputPattern{
			rule: OutputFilterRule{
				Name:        "system_prompt_dynamic",
				Pattern:     pattern,
				Replacement: "[REDACTED]",
				Description: "Dynamic system prompt content",
			},
			regex: regex,
		})
	}
}

// AddFilter adds a custom output filter.
func (f *OutputFilter) AddFilter(rule OutputFilterRule) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	regex, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return err
	}

	f.patterns["custom"] = append(f.patterns["custom"], &compiledOutputPattern{
		rule:  rule,
		regex: regex,
	})
	return nil
}

// RemoveFilter removes a custom filter by name.
func (f *OutputFilter) RemoveFilter(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	customs := f.patterns["custom"]
	for i, cp := range customs {
		if cp.rule.Name == name {
			f.patterns["custom"] = append(customs[:i], customs[i+1:]...)
			return
		}
	}
}

// GetConfig returns the current filter configuration.
func (f *OutputFilter) GetConfig() *OutputFilterConfig {
	return f.config
}

// UpdateConfig updates the filter configuration and reinitializes patterns.
func (f *OutputFilter) UpdateConfig(config *OutputFilterConfig) {
	f.config = config
	f.initPatterns()
}

// ValidateOutput validates output without filtering.
func (f *OutputFilter) ValidateOutput(output string) []FilterViolation {
	violations := make([]FilterViolation, 0)

	f.mu.RLock()
	defer f.mu.RUnlock()

	for category, patterns := range f.patterns {
		for _, cp := range patterns {
			matches := cp.regex.FindAllStringIndex(output, -1)
			for _, match := range matches {
				matchText := output[match[0]:match[1]]
				if len(matchText) > 50 {
					matchText = matchText[:50] + "..."
				}
				violations = append(violations, FilterViolation{
					FilterName: category + "/" + cp.rule.Name,
					Match:      matchText,
					Position:   match[0],
					Action:     "detected",
				})
			}
		}
	}

	return violations
}
