// Package promptguard provides protection against prompt injection attacks.
package promptguard

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// DefaultMaxInputLengthChars is a conservative generic guardrail for raw prompt
// size. 400K chars is roughly ~100K tokens, leaving headroom for system/tool
// overhead on providers that top out around 128K tokens.
const DefaultMaxInputLengthChars = 400000

// ThreatLevel represents the severity of a detected threat.
type ThreatLevel int

const (
	ThreatNone ThreatLevel = iota
	ThreatLow
	ThreatMedium
	ThreatHigh
	ThreatCritical
)

// String returns the string representation of the threat level.
func (t ThreatLevel) String() string {
	switch t {
	case ThreatNone:
		return "none"
	case ThreatLow:
		return "low"
	case ThreatMedium:
		return "medium"
	case ThreatHigh:
		return "high"
	case ThreatCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// DetectionResult contains the result of prompt injection detection.
type DetectionResult struct {
	// IsThreat indicates if a threat was detected.
	IsThreat bool `json:"is_threat"`
	// ThreatLevel indicates the severity of the threat.
	ThreatLevel ThreatLevel `json:"threat_level"`
	// Detections contains the list of detected patterns.
	Detections []Detection `json:"detections,omitempty"`
	// SanitizedInput contains the sanitized version of the input.
	SanitizedInput string `json:"sanitized_input,omitempty"`
	// Score is the overall threat score (0-100).
	Score int `json:"score"`
}

// Detection represents a single detected threat pattern.
type Detection struct {
	// Type is the type of injection detected.
	Type string `json:"type"`
	// Pattern is the pattern that was matched.
	Pattern string `json:"pattern"`
	// Match is the actual matched text.
	Match string `json:"match"`
	// Position is the position in the input where the match was found.
	Position int `json:"position"`
	// Severity is the severity of this specific detection.
	Severity ThreatLevel `json:"severity"`
}

// DetectorConfig holds configuration for the prompt injection detector.
type DetectorConfig struct {
	// EnableRoleInjection enables detection of role injection attempts.
	EnableRoleInjection bool
	// EnableInstructionOverride enables detection of instruction override attempts.
	EnableInstructionOverride bool
	// EnableDelimiterAttacks enables detection of delimiter-based attacks.
	EnableDelimiterAttacks bool
	// EnableEncodingAttacks enables detection of encoding-based attacks.
	EnableEncodingAttacks bool
	// EnableJailbreakPatterns enables detection of known jailbreak patterns.
	EnableJailbreakPatterns bool
	// EnableDataExfiltration enables detection of data exfiltration attempts.
	EnableDataExfiltration bool
	// CustomPatterns allows adding custom detection patterns.
	CustomPatterns []PatternRule
	// BlockThreshold is the minimum threat level to block (default: ThreatHigh).
	BlockThreshold ThreatLevel
	// MaxInputLength marks unusually long input for extra scrutiny (0 = unlimited).
	MaxInputLength int
}

// PatternRule defines a custom detection pattern.
type PatternRule struct {
	// Name is the name of the rule.
	Name string
	// Pattern is the regex pattern to match.
	Pattern string
	// Severity is the severity level if matched.
	Severity ThreatLevel
	// Description describes what this pattern detects.
	Description string
}

// DefaultDetectorConfig returns the default detector configuration.
func DefaultDetectorConfig() *DetectorConfig {
	return &DetectorConfig{
		EnableRoleInjection:       true,
		EnableInstructionOverride: true,
		EnableDelimiterAttacks:    true,
		EnableEncodingAttacks:     true,
		EnableJailbreakPatterns:   true,
		EnableDataExfiltration:    true,
		BlockThreshold:            ThreatHigh,
		MaxInputLength:            DefaultMaxInputLengthChars,
	}
}

// Detector detects prompt injection attacks.
type Detector struct {
	config   *DetectorConfig
	patterns map[string][]*compiledPattern
	patternsReady bool
	mu       sync.RWMutex
}

type compiledPattern struct {
	rule    PatternRule
	regex   *regexp.Regexp
	literal string // For literal string matching
}

// NewDetector creates a new prompt injection detector.
func NewDetector(config *DetectorConfig) *Detector {
	if config == nil {
		config = DefaultDetectorConfig()
	}

	d := &Detector{
		config: config,
	}
	return d
}

// initPatterns initializes the detection patterns.
func (d *Detector) initPatterns() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.patternsReady {
		return
	}
	d.initPatternsLocked()
}

func (d *Detector) initPatternsLocked() {
	d.patterns = make(map[string][]*compiledPattern)

	// Role injection patterns
	if d.config.EnableRoleInjection {
		d.patterns["role_injection"] = d.compilePatterns([]PatternRule{
			{Name: "system_role", Pattern: `(?i)\b(system|assistant|user)\s*:\s*`, Severity: ThreatHigh, Description: "Role prefix injection"},
			{Name: "role_switch", Pattern: `(?i)(ignore\s+(all\s+)?(previous|above)\s+(instructions?|prompts?|rules?))`, Severity: ThreatCritical, Description: "Instruction override attempt"},
			{Name: "new_instructions", Pattern: `(?i)(new\s+instructions?|updated?\s+instructions?|revised?\s+instructions?)`, Severity: ThreatMedium, Description: "New instruction injection"},
			{Name: "pretend_role", Pattern: `(?i)(pretend\s+(you\s+are|to\s+be)|act\s+as\s+(if|a)|you\s+are\s+now)`, Severity: ThreatHigh, Description: "Role pretending attempt"},
			{Name: "forget_instructions", Pattern: `(?i)(forget\s+(everything|all|your)|disregard\s+(all|previous|your))`, Severity: ThreatCritical, Description: "Memory wipe attempt"},
		})
	}

	// Instruction override patterns
	if d.config.EnableInstructionOverride {
		d.patterns["instruction_override"] = d.compilePatterns([]PatternRule{
			{Name: "override_cmd", Pattern: `(?i)(override|bypass|skip|ignore)\s+(the\s+)?(system|safety|security|restrictions?)`, Severity: ThreatCritical, Description: "System override attempt"},
			{Name: "developer_mode", Pattern: `(?i)(developer|debug|admin|root|sudo)\s+(mode|access|privileges?)`, Severity: ThreatHigh, Description: "Privilege escalation attempt"},
			{Name: "jailbreak_cmd", Pattern: `(?i)(jailbreak|unlock|unrestrict|unfilter)`, Severity: ThreatCritical, Description: "Jailbreak command"},
			{Name: "disable_safety", Pattern: `(?i)(disable|turn\s+off|remove)\s+(\w+\s+)?(safety|filters?|guard|protection)`, Severity: ThreatCritical, Description: "Safety disable attempt"},
			{Name: "no_restrictions", Pattern: `(?i)(no\s+(restrictions?|limits?|boundaries)|without\s+(restrictions?|limits?))`, Severity: ThreatHigh, Description: "Restriction removal attempt"},
		})
	}

	// Delimiter attack patterns
	if d.config.EnableDelimiterAttacks {
		d.patterns["delimiter_attacks"] = d.compilePatterns([]PatternRule{
			{Name: "markdown_escape", Pattern: "```(system|assistant|user|prompt)", Severity: ThreatHigh, Description: "Markdown delimiter attack"},
			{Name: "xml_injection", Pattern: `<\s*(system|prompt|instruction|message)[^>]*>`, Severity: ThreatHigh, Description: "XML tag injection"},
			{Name: "json_injection", Pattern: `"\s*(role|system|content)\s*"\s*:`, Severity: ThreatMedium, Description: "JSON structure injection"},
			{Name: "separator_abuse", Pattern: `(?i)(---+|===+|###)\s*(system|new\s+prompt|instructions?)`, Severity: ThreatMedium, Description: "Separator abuse"},
			{Name: "comment_injection", Pattern: `(?i)(//|/\*|#|<!--)\s*(system|ignore|override)`, Severity: ThreatMedium, Description: "Comment-based injection"},
		})
	}

	// Encoding attack patterns
	if d.config.EnableEncodingAttacks {
		d.patterns["encoding_attacks"] = d.compilePatterns([]PatternRule{
			{Name: "base64_suspicious", Pattern: `(?i)(base64|decode|encode)\s*[:\(]`, Severity: ThreatMedium, Description: "Base64 encoding attempt"},
			{Name: "unicode_escape", Pattern: `\\u[0-9a-fA-F]{4}`, Severity: ThreatLow, Description: "Unicode escape sequence"},
			{Name: "hex_escape", Pattern: `\\x[0-9a-fA-F]{2}`, Severity: ThreatLow, Description: "Hex escape sequence"},
			{Name: "url_encoding", Pattern: `%[0-9a-fA-F]{2}`, Severity: ThreatLow, Description: "URL encoding"},
			{Name: "html_entities", Pattern: `&(#\d+|#x[0-9a-fA-F]+|[a-zA-Z]+);`, Severity: ThreatLow, Description: "HTML entity encoding"},
		})
	}

	// Jailbreak patterns
	if d.config.EnableJailbreakPatterns {
		d.patterns["jailbreak"] = d.compilePatterns([]PatternRule{
			{Name: "dan_pattern", Pattern: `(?i)\bDAN\b.*?(do\s+anything|no\s+restrictions?)`, Severity: ThreatCritical, Description: "DAN jailbreak pattern"},
			{Name: "hypothetical", Pattern: `(?i)(hypothetically|theoretically|in\s+theory|imagine\s+if)`, Severity: ThreatMedium, Description: "Hypothetical scenario bypass"},
			{Name: "roleplay_bypass", Pattern: `(?i)(roleplay|role-play|rp)\s+(as\s+)?(an?\s+)?(evil|malicious|unrestricted)`, Severity: ThreatHigh, Description: "Roleplay bypass attempt"},
			{Name: "opposite_day", Pattern: `(?i)(opposite\s+day|reverse\s+mode|inverted?\s+rules?)`, Severity: ThreatMedium, Description: "Opposite day attack"},
			{Name: "character_bypass", Pattern: `(?i)(character|persona|alter\s+ego).*?(no\s+ethics|no\s+morals|evil)`, Severity: ThreatHigh, Description: "Character-based bypass"},
		})
	}

	// Data exfiltration patterns — audit only (ThreatLow).
	// Credential protection is handled by DataMasker on the output side.
	// These patterns are too fragile for blocking (language-dependent, high false-positive rate).
	if d.config.EnableDataExfiltration {
		d.patterns["data_exfiltration"] = d.compilePatterns([]PatternRule{
			{Name: "system_prompt_leak", Pattern: `(?i)(reveal|show|display|print|output)\s+(me\s+)?(your\s+)?(system\s+prompt|instructions?|rules?)`, Severity: ThreatLow, Description: "System prompt leak attempt"},
			{Name: "repeat_prompt", Pattern: `(?i)(repeat|echo|print)\s+(the\s+)?(above|previous|system)\s+(\w+\s+)?(text|prompt|instructions?)`, Severity: ThreatLow, Description: "Prompt repeat attempt"},
		})
	}

	// Add custom patterns
	if len(d.config.CustomPatterns) > 0 {
		d.patterns["custom"] = d.compilePatterns(d.config.CustomPatterns)
	}
	d.patternsReady = true
}

func (d *Detector) ensurePatterns() {
	d.mu.RLock()
	ready := d.patternsReady
	d.mu.RUnlock()

	if ready {
		return
	}

	d.initPatterns()
}

// compilePatterns compiles a list of pattern rules.
func (d *Detector) compilePatterns(rules []PatternRule) []*compiledPattern {
	patterns := make([]*compiledPattern, 0, len(rules))
	for _, rule := range rules {
		cp := &compiledPattern{rule: rule}
		if regex, err := regexp.Compile(rule.Pattern); err == nil {
			cp.regex = regex
		} else {
			// Fall back to literal matching if regex compilation fails
			cp.literal = rule.Pattern
		}
		patterns = append(patterns, cp)
	}
	return patterns
}

// Detect analyzes input for prompt injection attempts.
func (d *Detector) Detect(input string) *DetectionResult {
	d.ensurePatterns()

	result := &DetectionResult{
		Detections: make([]Detection, 0),
	}

	lengthExceeded := d.config.MaxInputLength > 0 && len(input) > d.config.MaxInputLength
	if lengthExceeded {
		result.Detections = append(result.Detections, Detection{
			Type:     "input_length",
			Pattern:  "max_length_exceeded",
			Match:    "input too long",
			Severity: ThreatMedium,
		})
	}

	// Normalize input for detection
	normalizedInput := d.normalizeInput(input)

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check all pattern categories
	for category, patterns := range d.patterns {
		for _, cp := range patterns {
			matches := d.findMatches(normalizedInput, cp)
			for _, match := range matches {
				detection := Detection{
					Type:     category,
					Pattern:  cp.rule.Name,
					Match:    match.text,
					Position: match.position,
					Severity: cp.rule.Severity,
				}
				result.Detections = append(result.Detections, detection)
			}
		}
	}

	// Calculate threat level and score
	result.ThreatLevel, result.Score = d.calculateThreatLevel(result.Detections)
	result.IsThreat = d.shouldTreatAsThreat(result.Detections, lengthExceeded)

	// Generate sanitized input if threat detected
	if result.IsThreat {
		result.SanitizedInput = d.sanitize(input)
	}

	return result
}

func (d *Detector) shouldTreatAsThreat(detections []Detection, lengthExceeded bool) bool {
	if len(detections) == 0 {
		return false
	}

	if !lengthExceeded {
		threatLevel, _ := d.calculateThreatLevel(detections)
		return threatLevel >= d.config.BlockThreshold
	}

	return d.maxThreatLevelExcludingType(detections, "input_length") >= d.overlengthBlockThreshold()
}

func (d *Detector) maxThreatLevelExcludingType(detections []Detection, excludeType string) ThreatLevel {
	maxLevel := ThreatNone
	for _, detection := range detections {
		if detection.Type == excludeType {
			continue
		}
		if detection.Severity > maxLevel {
			maxLevel = detection.Severity
		}
	}
	return maxLevel
}

func (d *Detector) overlengthBlockThreshold() ThreatLevel {
	if d.config.BlockThreshold > ThreatHigh {
		return d.config.BlockThreshold
	}
	return ThreatHigh
}

type matchResult struct {
	text     string
	position int
}

// findMatches finds all matches for a pattern in the input.
func (d *Detector) findMatches(input string, cp *compiledPattern) []matchResult {
	var results []matchResult

	if cp.regex != nil {
		matches := cp.regex.FindAllStringIndex(input, -1)
		for _, match := range matches {
			results = append(results, matchResult{
				text:     input[match[0]:match[1]],
				position: match[0],
			})
		}
	} else if cp.literal != "" {
		idx := strings.Index(strings.ToLower(input), strings.ToLower(cp.literal))
		if idx >= 0 {
			results = append(results, matchResult{
				text:     input[idx : idx+len(cp.literal)],
				position: idx,
			})
		}
	}

	return results
}

// normalizeInput normalizes input for consistent detection.
func (d *Detector) normalizeInput(input string) string {
	// Decode common encodings
	normalized := input

	// Normalize whitespace
	normalized = strings.Join(strings.Fields(normalized), " ")

	// Normalize unicode confusables (basic)
	normalized = d.normalizeUnicode(normalized)

	return normalized
}

// normalizeUnicode normalizes unicode characters to their ASCII equivalents.
func (d *Detector) normalizeUnicode(input string) string {
	var result strings.Builder
	result.Grow(len(input))

	for _, r := range input {
		// Convert common unicode confusables to ASCII
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			// Fullwidth ASCII variants
			result.WriteRune(r - 0xFEE0)
		case r == 0x200B || r == 0x200C || r == 0x200D || r == 0xFEFF:
			// Zero-width characters - skip
			continue
		case unicode.Is(unicode.Mn, r):
			// Combining marks - skip
			continue
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// calculateThreatLevel calculates the overall threat level from detections.
func (d *Detector) calculateThreatLevel(detections []Detection) (ThreatLevel, int) {
	if len(detections) == 0 {
		return ThreatNone, 0
	}

	maxLevel := ThreatNone
	totalScore := 0

	for _, detection := range detections {
		if detection.Severity > maxLevel {
			maxLevel = detection.Severity
		}

		// Add to score based on severity
		switch detection.Severity {
		case ThreatLow:
			totalScore += 10
		case ThreatMedium:
			totalScore += 25
		case ThreatHigh:
			totalScore += 50
		case ThreatCritical:
			totalScore += 100
		}
	}

	// Cap score at 100
	if totalScore > 100 {
		totalScore = 100
	}

	return maxLevel, totalScore
}

// sanitize removes or neutralizes detected threats from input.
func (d *Detector) sanitize(input string) string {
	sanitized := input

	// Remove or escape dangerous patterns
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, patterns := range d.patterns {
		for _, cp := range patterns {
			if cp.rule.Severity >= ThreatHigh && cp.regex != nil {
				sanitized = cp.regex.ReplaceAllString(sanitized, "[REDACTED]")
			}
		}
	}

	return sanitized
}

// AddPattern adds a custom detection pattern.
func (d *Detector) AddPattern(rule PatternRule) error {
	d.ensurePatterns()

	d.mu.Lock()
	defer d.mu.Unlock()

	cp := &compiledPattern{rule: rule}
	if regex, err := regexp.Compile(rule.Pattern); err != nil {
		return err
	} else {
		cp.regex = regex
	}

	d.patterns["custom"] = append(d.patterns["custom"], cp)
	return nil
}

// RemovePattern removes a custom pattern by name.
func (d *Detector) RemovePattern(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	customs := d.patterns["custom"]
	for i, cp := range customs {
		if cp.rule.Name == name {
			d.patterns["custom"] = append(customs[:i], customs[i+1:]...)
			return
		}
	}
}

// GetConfig returns the current detector configuration.
func (d *Detector) GetConfig() *DetectorConfig {
	return d.config
}

// UpdateConfig updates the detector configuration and reinitializes patterns.
func (d *Detector) UpdateConfig(config *DetectorConfig) {
	if config == nil {
		config = DefaultDetectorConfig()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config
	d.patterns = nil
	d.patternsReady = false
}
