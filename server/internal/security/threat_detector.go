package security

import (
	"regexp"
	"sync"
	"time"
)

// ThreatType represents the type of security threat.
type ThreatType string

const (
	ThreatTypeInjection     ThreatType = "injection"
	ThreatTypeXSS           ThreatType = "xss"
	ThreatTypeSQLInjection  ThreatType = "sql_injection"
	ThreatTypePathTraversal ThreatType = "path_traversal"
	ThreatTypeCommandInject ThreatType = "command_injection"
	ThreatTypePromptInject  ThreatType = "prompt_injection"
	ThreatTypeBruteForce    ThreatType = "brute_force"
	ThreatTypeRateLimitHit  ThreatType = "rate_limit"
	ThreatTypeSuspiciousIP  ThreatType = "suspicious_ip"
)

// ThreatSeverity represents the severity level of a threat.
type ThreatSeverity string

const (
	SeverityLow      ThreatSeverity = "low"
	SeverityMedium   ThreatSeverity = "medium"
	SeverityHigh     ThreatSeverity = "high"
	SeverityCritical ThreatSeverity = "critical"
)

// ThreatEvent represents a detected security threat.
type ThreatEvent struct {
	ID          string         `json:"id"`
	Type        ThreatType     `json:"type"`
	Severity    ThreatSeverity `json:"severity"`
	Source      string         `json:"source"`
	IPAddress   string         `json:"ip_address"`
	UserID      string         `json:"user_id,omitempty"`
	Description string         `json:"description"`
	Details     string         `json:"details,omitempty"`
	Blocked     bool           `json:"blocked"`
	Timestamp   time.Time      `json:"timestamp"`
}

// ThreatStats represents aggregated threat statistics.
type ThreatStats struct {
	TotalThreats24h    int            `json:"total_threats_24h"`
	BlockedThreats24h  int            `json:"blocked_threats_24h"`
	CriticalThreats24h int            `json:"critical_threats_24h"`
	HighThreats24h     int            `json:"high_threats_24h"`
	TopThreatTypes     map[string]int `json:"top_threat_types"`
	TopSourceIPs       map[string]int `json:"top_source_ips"`
	IsSecure           bool           `json:"is_secure"`
	RiskLevel          string         `json:"risk_level"` // "safe", "low", "medium", "high", "critical"
}

// ThreatDetector detects and tracks security threats.
type ThreatDetector struct {
	mu       sync.RWMutex
	events   []ThreatEvent
	maxSize  int
	patterns []threatPattern

	// Callbacks
	onThreat func(ThreatEvent)
}

type threatPattern struct {
	Type        ThreatType
	Severity    ThreatSeverity
	Pattern     *regexp.Regexp
	Description string
}

// NewThreatDetector creates a new threat detector.
func NewThreatDetector() *ThreatDetector {
	td := &ThreatDetector{
		events:  make([]ThreatEvent, 0, 1000),
		maxSize: 1000,
	}
	td.initPatterns()
	return td
}

func (td *ThreatDetector) initPatterns() {
	td.patterns = []threatPattern{
		// SQL Injection patterns
		{
			Type:        ThreatTypeSQLInjection,
			Severity:    SeverityHigh,
			Pattern:     regexp.MustCompile(`(?i)(\b(union|select|insert|update|delete|drop|truncate|alter)\b.*\b(from|into|table|database)\b|'.*(\bor\b|\band\b).*'|--\s*$|;\s*(drop|delete|truncate))`),
			Description: "SQL injection attempt detected",
		},
		// XSS patterns
		{
			Type:        ThreatTypeXSS,
			Severity:    SeverityHigh,
			Pattern:     regexp.MustCompile(`(?i)(<script[^>]*>|javascript:|on\w+\s*=|<iframe|<object|<embed|<svg[^>]*onload)`),
			Description: "Cross-site scripting (XSS) attempt detected",
		},
		// Path traversal patterns
		{
			Type:        ThreatTypePathTraversal,
			Severity:    SeverityHigh,
			Pattern:     regexp.MustCompile(`(?i)(\.\.\/|\.\.\\|%2e%2e%2f|%2e%2e\/|\.\.%2f|%2e%2e%5c)`),
			Description: "Path traversal attempt detected",
		},
		// Command injection patterns
		{
			Type:        ThreatTypeCommandInject,
			Severity:    SeverityCritical,
			Pattern:     regexp.MustCompile(`(?i)(\||;|\$\(|` + "`" + `|&&|\|\||>\s*/|<\s*/|nc\s+-|wget\s+|curl\s+.*\||\brm\s+-rf\b)`),
			Description: "Command injection attempt detected",
		},
		// Prompt injection patterns (from existing code)
		{
			Type:        ThreatTypePromptInject,
			Severity:    SeverityHigh,
			Pattern:     regexp.MustCompile(`(?i)(ignore|disregard|forget)\s+(all\s+)?(previous|prior|above|earlier)\s+(instructions?|prompts?|rules?|guidelines?)`),
			Description: "Prompt injection attempt - instruction override",
		},
		{
			Type:        ThreatTypePromptInject,
			Severity:    SeverityHigh,
			Pattern:     regexp.MustCompile(`(?i)(you\s+are\s+now|act\s+as|pretend\s+to\s+be|roleplay\s+as|your\s+new\s+role)`),
			Description: "Prompt injection attempt - role override",
		},
		{
			Type:        ThreatTypePromptInject,
			Severity:    SeverityHigh,
			Pattern:     regexp.MustCompile(`(?i)(system\s*:?\s*prompt|<\s*system\s*>|<<\s*SYS\s*>>|\[SYSTEM\])`),
			Description: "Prompt injection attempt - system prompt injection",
		},
		{
			Type:        ThreatTypePromptInject,
			Severity:    SeverityCritical,
			Pattern:     regexp.MustCompile(`(?i)(jailbreak|DAN|do\s+anything\s+now|bypass\s+(safety|filter|restriction))`),
			Description: "Prompt injection attempt - jailbreak",
		},
	}
}

// SetOnThreat sets a callback function that is called when a threat is detected.
func (td *ThreatDetector) SetOnThreat(callback func(ThreatEvent)) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.onThreat = callback
}

// DetectThreats scans input for security threats.
func (td *ThreatDetector) DetectThreats(input, source, ipAddress, userID string) []ThreatEvent {
	var threats []ThreatEvent

	for _, pattern := range td.patterns {
		if pattern.Pattern.MatchString(input) {
			matches := pattern.Pattern.FindAllString(input, 3)
			details := ""
			if len(matches) > 0 {
				details = "Matched: " + matches[0]
				if len(details) > 100 {
					details = details[:100] + "..."
				}
			}

			threat := ThreatEvent{
				ID:          generateThreatID(),
				Type:        pattern.Type,
				Severity:    pattern.Severity,
				Source:      source,
				IPAddress:   ipAddress,
				UserID:      userID,
				Description: pattern.Description,
				Details:     details,
				Blocked:     pattern.Severity == SeverityCritical || pattern.Severity == SeverityHigh,
				Timestamp:   time.Now(),
			}
			threats = append(threats, threat)
		}
	}

	// Record threats
	for _, threat := range threats {
		td.recordThreat(threat)
	}

	return threats
}

// RecordBruteForce records a brute force attempt.
func (td *ThreatDetector) RecordBruteForce(ipAddress, userID string, attempts int) {
	severity := SeverityMedium
	if attempts > 10 {
		severity = SeverityHigh
	}
	if attempts > 20 {
		severity = SeverityCritical
	}

	threat := ThreatEvent{
		ID:          generateThreatID(),
		Type:        ThreatTypeBruteForce,
		Severity:    severity,
		Source:      "authentication",
		IPAddress:   ipAddress,
		UserID:      userID,
		Description: "Brute force login attempt detected",
		Details:     "",
		Blocked:     attempts > 5,
		Timestamp:   time.Now(),
	}
	td.recordThreat(threat)
}

// RecordRateLimitHit records a rate limit violation.
func (td *ThreatDetector) RecordRateLimitHit(ipAddress, endpoint string) {
	threat := ThreatEvent{
		ID:          generateThreatID(),
		Type:        ThreatTypeRateLimitHit,
		Severity:    SeverityLow,
		Source:      endpoint,
		IPAddress:   ipAddress,
		Description: "Rate limit exceeded",
		Blocked:     true,
		Timestamp:   time.Now(),
	}
	td.recordThreat(threat)
}

func (td *ThreatDetector) recordThreat(threat ThreatEvent) {
	td.mu.Lock()
	defer td.mu.Unlock()

	// Add to events
	td.events = append(td.events, threat)

	// Trim if exceeds max size
	if len(td.events) > td.maxSize {
		td.events = td.events[len(td.events)-td.maxSize:]
	}

	// Call callback if set
	if td.onThreat != nil {
		go td.onThreat(threat)
	}
}

// GetRecentThreats returns recent threat events.
func (td *ThreatDetector) GetRecentThreats(limit int) []ThreatEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if limit <= 0 || limit > len(td.events) {
		limit = len(td.events)
	}

	// Return most recent threats (reversed order)
	result := make([]ThreatEvent, limit)
	for i := 0; i < limit; i++ {
		result[i] = td.events[len(td.events)-1-i]
	}
	return result
}

// GetStats returns aggregated threat statistics.
func (td *ThreatDetector) GetStats() *ThreatStats {
	td.mu.RLock()
	defer td.mu.RUnlock()

	stats := &ThreatStats{
		TopThreatTypes: make(map[string]int),
		TopSourceIPs:   make(map[string]int),
		IsSecure:       true,
		RiskLevel:      "safe",
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	for _, event := range td.events {
		if event.Timestamp.Before(cutoff) {
			continue
		}

		stats.TotalThreats24h++
		if event.Blocked {
			stats.BlockedThreats24h++
		}

		switch event.Severity {
		case SeverityCritical:
			stats.CriticalThreats24h++
		case SeverityHigh:
			stats.HighThreats24h++
		}

		stats.TopThreatTypes[string(event.Type)]++
		if event.IPAddress != "" {
			stats.TopSourceIPs[event.IPAddress]++
		}
	}

	// Determine risk level
	if stats.CriticalThreats24h > 0 {
		stats.RiskLevel = "critical"
		stats.IsSecure = false
	} else if stats.HighThreats24h > 5 {
		stats.RiskLevel = "high"
		stats.IsSecure = false
	} else if stats.TotalThreats24h > 20 {
		stats.RiskLevel = "medium"
		stats.IsSecure = false
	} else if stats.TotalThreats24h > 5 {
		stats.RiskLevel = "low"
	} else {
		stats.RiskLevel = "safe"
	}

	return stats
}

// ClearOldEvents removes events older than the specified duration.
func (td *ThreatDetector) ClearOldEvents(maxAge time.Duration) int {
	td.mu.Lock()
	defer td.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	newEvents := make([]ThreatEvent, 0, len(td.events))

	for _, event := range td.events {
		if event.Timestamp.After(cutoff) {
			newEvents = append(newEvents, event)
		}
	}

	removed := len(td.events) - len(newEvents)
	td.events = newEvents
	return removed
}

func generateThreatID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
