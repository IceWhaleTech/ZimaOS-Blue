// Package autoreply provides automatic response functionality.
package autoreply

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// TriggerType represents the type of trigger.
type TriggerType string

const (
	// TriggerKeyword matches exact keywords.
	TriggerKeyword TriggerType = "keyword"
	// TriggerRegex matches using regular expressions.
	TriggerRegex TriggerType = "regex"
	// TriggerContains matches if message contains the text.
	TriggerContains TriggerType = "contains"
	// TriggerPrefix matches if message starts with the text.
	TriggerPrefix TriggerType = "prefix"
	// TriggerSuffix matches if message ends with the text.
	TriggerSuffix TriggerType = "suffix"
)

// Rule represents an auto-reply rule.
type Rule struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	TriggerType  TriggerType            `json:"trigger_type"`
	TriggerValue string                 `json:"trigger_value"`
	Responses    []string               `json:"responses"` // Multiple responses for random selection
	Priority     int                    `json:"priority"`  // Higher priority rules are checked first
	Enabled      bool                   `json:"enabled"`
	CaseSensitive bool                  `json:"case_sensitive"`
	Channels     []string               `json:"channels,omitempty"` // Empty = all channels
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	MatchCount   int64                  `json:"match_count"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`

	// Compiled regex (not serialized)
	compiledRegex *regexp.Regexp
}

// TemplateContext contains variables available in response templates.
type TemplateContext struct {
	User      string                 `json:"user"`
	UserID    string                 `json:"user_id"`
	Channel   string                 `json:"channel"`
	ChatID    string                 `json:"chat_id"`
	Message   string                 `json:"message"`
	Time      time.Time              `json:"time"`
	Match     string                 `json:"match"`      // The matched text
	Groups    []string               `json:"groups"`     // Regex capture groups
	Custom    map[string]interface{} `json:"custom"`     // Custom variables
}

// Config contains auto-reply service configuration.
type Config struct {
	// Enabled indicates if auto-reply is enabled.
	Enabled bool `yaml:"enabled"`
	// MaxRulesPerChannel is the maximum rules per channel.
	MaxRulesPerChannel int `yaml:"max_rules_per_channel"`
	// DefaultCooldownSeconds is the cooldown between replies to same user.
	DefaultCooldownSeconds int `yaml:"default_cooldown_seconds"`
}

// DefaultConfig returns the default auto-reply configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:                true,
		MaxRulesPerChannel:     100,
		DefaultCooldownSeconds: 0,
	}
}

// generateSecureRuleID generates a cryptographically secure rule ID.
// Security: Use crypto/rand instead of math/rand for unpredictable IDs.
func generateSecureRuleID() string {
	b := make([]byte, 8)
	if _, err := cryptorand.Read(b); err != nil {
		// Fallback to time-based ID if crypto/rand fails (should never happen)
		return fmt.Sprintf("rule_%d", timeutil.NowNano())
	}
	return fmt.Sprintf("rule_%s", hex.EncodeToString(b))
}

// Service manages auto-reply rules.
type Service struct {
	config Config
	logger *zap.Logger
	rules  map[string]*Rule
	mu     sync.RWMutex

	// Cooldown tracking
	cooldowns map[string]time.Time
	cooldownMu sync.Mutex
}

// NewService creates a new auto-reply service.
func NewService(cfg Config, logger *zap.Logger) *Service {
	return &Service{
		config:    cfg,
		logger:    logger.With(zap.String("component", "autoreply")),
		rules:     make(map[string]*Rule),
		cooldowns: make(map[string]time.Time),
	}
}

// Create creates a new auto-reply rule.
func (s *Service) Create(name string, triggerType TriggerType, triggerValue string, responses []string, priority int) (*Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate trigger
	if triggerValue == "" {
		return nil, fmt.Errorf("trigger value cannot be empty")
	}

	if len(responses) == 0 {
		return nil, fmt.Errorf("at least one response is required")
	}

	// Compile regex if needed
	var compiledRegex *regexp.Regexp
	if triggerType == TriggerRegex {
		var err error
		compiledRegex, err = regexp.Compile(triggerValue)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
	}

	// Generate ID using crypto/rand for security
	id := generateSecureRuleID()

	rule := &Rule{
		ID:            id,
		Name:          name,
		TriggerType:   triggerType,
		TriggerValue:  triggerValue,
		Responses:     responses,
		Priority:      priority,
		Enabled:       true,
		CaseSensitive: false,
		CreatedAt:     timeutil.NowTime(),
		UpdatedAt:     timeutil.NowTime(),
		Metadata:      make(map[string]interface{}),
		compiledRegex: compiledRegex,
	}

	s.rules[id] = rule

	s.logger.Info("auto-reply rule created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("trigger_type", string(triggerType)))

	return rule, nil
}

// Get returns a rule by ID.
func (s *Service) Get(id string) (*Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, exists := s.rules[id]
	if exists {
		copy := *rule
		return &copy, true
	}
	return nil, false
}

// List returns all rules sorted by priority.
func (s *Service) List() []*Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Rule, 0, len(s.rules))
	for _, rule := range s.rules {
		copy := *rule
		result = append(result, &copy)
	}

	// Sort by priority (descending)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority > result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// Update updates a rule.
func (s *Service) Update(id, name string, triggerType TriggerType, triggerValue string, responses []string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, exists := s.rules[id]
	if !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	// Compile regex if needed
	var compiledRegex *regexp.Regexp
	if triggerType == TriggerRegex {
		var err error
		compiledRegex, err = regexp.Compile(triggerValue)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}

	rule.Name = name
	rule.TriggerType = triggerType
	rule.TriggerValue = triggerValue
	rule.Responses = responses
	rule.Priority = priority
	rule.UpdatedAt = timeutil.NowTime()
	rule.compiledRegex = compiledRegex

	return nil
}

// Delete deletes a rule.
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rules[id]; !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	delete(s.rules, id)
	s.logger.Info("auto-reply rule deleted", zap.String("id", id))
	return nil
}

// Enable enables a rule.
func (s *Service) Enable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, exists := s.rules[id]
	if !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	rule.Enabled = true
	rule.UpdatedAt = timeutil.NowTime()
	return nil
}

// Disable disables a rule.
func (s *Service) Disable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, exists := s.rules[id]
	if !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	rule.Enabled = false
	rule.UpdatedAt = timeutil.NowTime()
	return nil
}

// SetChannels sets the channels for a rule.
func (s *Service) SetChannels(id string, channels []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule, exists := s.rules[id]
	if !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	rule.Channels = channels
	rule.UpdatedAt = timeutil.NowTime()
	return nil
}

// Match checks if a message matches any rule and returns the response.
func (s *Service) Match(ctx context.Context, message, channel, userID, username, chatID string) (string, *Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check cooldown
	cooldownKey := fmt.Sprintf("%s:%s", channel, userID)
	if s.config.DefaultCooldownSeconds > 0 {
		s.cooldownMu.Lock()
		if lastReply, exists := s.cooldowns[cooldownKey]; exists {
			if timeutil.SinceTime(lastReply) < time.Duration(s.config.DefaultCooldownSeconds)*time.Second {
				s.cooldownMu.Unlock()
				return "", nil, nil
			}
		}
		s.cooldownMu.Unlock()
	}

	// Get sorted rules
	rules := make([]*Rule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}

	// Sort by priority
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[j].Priority > rules[i].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	// Check each rule
	for _, rule := range rules {
		// Check channel filter
		if len(rule.Channels) > 0 {
			found := false
			for _, ch := range rule.Channels {
				if ch == channel {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check match
		matched, groups := s.matchRule(rule, message)
		if !matched {
			continue
		}

		// Update match count
		rule.MatchCount++

		// Build template context
		templateCtx := TemplateContext{
			User:    username,
			UserID:  userID,
			Channel: channel,
			ChatID:  chatID,
			Message: message,
			Time:    timeutil.NowTime(),
			Match:   groups[0],
			Groups:  groups,
			Custom:  make(map[string]interface{}),
		}

		// Select random response
		response := rule.Responses[rand.Intn(len(rule.Responses))]

		// Render template
		rendered, err := s.renderTemplate(response, templateCtx)
		if err != nil {
			s.logger.Error("failed to render template",
				zap.String("rule_id", rule.ID),
				zap.Error(err))
			rendered = response // Use raw response on error
		}

		// Update cooldown
		if s.config.DefaultCooldownSeconds > 0 {
			s.cooldownMu.Lock()
			s.cooldowns[cooldownKey] = timeutil.NowTime()
			s.cooldownMu.Unlock()
		}

		s.logger.Debug("auto-reply matched",
			zap.String("rule_id", rule.ID),
			zap.String("rule_name", rule.Name),
			zap.String("message", message))

		return rendered, rule, nil
	}

	return "", nil, nil
}

// matchRule checks if a message matches a rule.
func (s *Service) matchRule(rule *Rule, message string) (bool, []string) {
	checkMessage := message
	checkValue := rule.TriggerValue

	if !rule.CaseSensitive {
		checkMessage = strings.ToLower(message)
		checkValue = strings.ToLower(rule.TriggerValue)
	}

	switch rule.TriggerType {
	case TriggerKeyword:
		if checkMessage == checkValue {
			return true, []string{message}
		}
	case TriggerContains:
		if strings.Contains(checkMessage, checkValue) {
			return true, []string{rule.TriggerValue}
		}
	case TriggerPrefix:
		if strings.HasPrefix(checkMessage, checkValue) {
			return true, []string{rule.TriggerValue}
		}
	case TriggerSuffix:
		if strings.HasSuffix(checkMessage, checkValue) {
			return true, []string{rule.TriggerValue}
		}
	case TriggerRegex:
		if rule.compiledRegex != nil {
			matches := rule.compiledRegex.FindStringSubmatch(message)
			if len(matches) > 0 {
				return true, matches
			}
		}
	}

	return false, nil
}

// renderTemplate renders a response template with the given context.
func (s *Service) renderTemplate(templateStr string, ctx TemplateContext) (string, error) {
	// Simple variable replacement for common cases
	result := templateStr
	result = strings.ReplaceAll(result, "{{user}}", ctx.User)
	result = strings.ReplaceAll(result, "{{user_id}}", ctx.UserID)
	result = strings.ReplaceAll(result, "{{channel}}", ctx.Channel)
	result = strings.ReplaceAll(result, "{{chat_id}}", ctx.ChatID)
	result = strings.ReplaceAll(result, "{{message}}", ctx.Message)
	result = strings.ReplaceAll(result, "{{match}}", ctx.Match)
	result = strings.ReplaceAll(result, "{{time}}", ctx.Time.Format("15:04:05"))
	result = strings.ReplaceAll(result, "{{date}}", ctx.Time.Format("2006-01-02"))

	// Replace capture groups
	for i, group := range ctx.Groups {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{$%d}}", i), group)
	}

	// If there are still template markers, try full template parsing
	if strings.Contains(result, "{{") {
		tmpl, err := template.New("response").Parse(templateStr)
		if err != nil {
			return result, nil // Return partially rendered result
		}

		var buf strings.Builder
		if err := tmpl.Execute(&buf, ctx); err != nil {
			return result, nil
		}
		return buf.String(), nil
	}

	return result, nil
}

// Test tests a message against all rules without updating stats.
func (s *Service) Test(message, channel string) (string, *Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get sorted rules
	rules := make([]*Rule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}

	// Sort by priority
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[j].Priority > rules[i].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	// Check each rule
	for _, rule := range rules {
		// Check channel filter
		if len(rule.Channels) > 0 {
			found := false
			for _, ch := range rule.Channels {
				if ch == channel {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check match
		matched, groups := s.matchRule(rule, message)
		if !matched {
			continue
		}

		// Build template context
		templateCtx := TemplateContext{
			User:    "test_user",
			UserID:  "test_user_id",
			Channel: channel,
			ChatID:  "test_chat_id",
			Message: message,
			Time:    timeutil.NowTime(),
			Match:   groups[0],
			Groups:  groups,
			Custom:  make(map[string]interface{}),
		}

		// Select first response for testing
		response := rule.Responses[0]

		// Render template
		rendered, _ := s.renderTemplate(response, templateCtx)

		return rendered, rule, nil
	}

	return "", nil, nil
}
