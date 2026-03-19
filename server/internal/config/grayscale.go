package config

import (
	"crypto/sha256"
	"encoding/binary"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"sync"
	"time"
)

// GrayscaleConfig holds grayscale/feature flag configuration.
type GrayscaleConfig struct {
	Enabled  bool            `yaml:"enabled"`
	Flags    []FeatureFlag   `yaml:"flags"`
	ABTests  []ABTest        `yaml:"ab_tests"`
	Versions []ConfigVersion `yaml:"versions"`
}

// FeatureFlag represents a single feature flag with targeting rules.
type FeatureFlag struct {
	Name         string          `yaml:"name"`
	Description  string          `yaml:"description"`
	Enabled      bool            `yaml:"enabled"`
	Percentage   float64         `yaml:"percentage"` // 0-100, for percentage rollout
	Users        []string        `yaml:"users"`      // Specific user IDs
	Groups       []string        `yaml:"groups"`     // User groups
	Variants     []FlagVariant   `yaml:"variants"`   // For multivariate flags
	Rules        []TargetingRule `yaml:"rules"`      // Advanced targeting rules
	DefaultValue interface{}     `yaml:"default_value"`
	CreatedAt    time.Time       `yaml:"created_at"`
	UpdatedAt    time.Time       `yaml:"updated_at"`
}

// FlagVariant represents a variant in a multivariate flag.
type FlagVariant struct {
	Name   string      `yaml:"name"`
	Value  interface{} `yaml:"value"`
	Weight float64     `yaml:"weight"` // Percentage weight (0-100)
}

// TargetingRule represents an advanced targeting rule.
type TargetingRule struct {
	Attribute string   `yaml:"attribute"` // e.g., "country", "version", "platform"
	Operator  string   `yaml:"operator"`  // "eq", "neq", "in", "not_in", "gt", "lt", "contains"
	Values    []string `yaml:"values"`
}

// ABTest represents an A/B test configuration.
type ABTest struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Enabled     bool        `yaml:"enabled"`
	StartTime   time.Time   `yaml:"start_time"`
	EndTime     time.Time   `yaml:"end_time"`
	Variants    []ABVariant `yaml:"variants"`
	TrafficPct  float64     `yaml:"traffic_pct"` // Percentage of traffic in test
	Metrics     []string    `yaml:"metrics"`     // Metrics to track
}

// ABVariant represents a variant in an A/B test.
type ABVariant struct {
	Name      string      `yaml:"name"`
	Value     interface{} `yaml:"value"`
	Weight    float64     `yaml:"weight"` // Percentage weight
	IsControl bool        `yaml:"is_control"`
}

// ConfigVersion represents a versioned configuration.
type ConfigVersion struct {
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	CreatedAt   time.Time              `yaml:"created_at"`
	Values      map[string]interface{} `yaml:"values"`
	Active      bool                   `yaml:"active"`
}

// EvaluationContext provides context for flag evaluation.
type EvaluationContext struct {
	UserID     string
	Groups     []string
	Attributes map[string]string
}

// FlagEvaluator evaluates feature flags for a given context.
type FlagEvaluator struct {
	config *GrayscaleConfig
	mu     sync.RWMutex
}

// NewFlagEvaluator creates a new flag evaluator.
func NewFlagEvaluator(config *GrayscaleConfig) *FlagEvaluator {
	return &FlagEvaluator{
		config: config,
	}
}

// UpdateConfig updates the grayscale configuration (thread-safe).
func (e *FlagEvaluator) UpdateConfig(config *GrayscaleConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// IsEnabled checks if a feature flag is enabled for the given context.
func (e *FlagEvaluator) IsEnabled(flagName string, ctx *EvaluationContext) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil || !e.config.Enabled {
		return false
	}

	flag := e.findFlag(flagName)
	if flag == nil || !flag.Enabled {
		return false
	}

	return e.evaluateFlag(flag, ctx)
}

// GetVariant returns the variant value for a multivariate flag.
func (e *FlagEvaluator) GetVariant(flagName string, ctx *EvaluationContext) interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil || !e.config.Enabled {
		return nil
	}

	flag := e.findFlag(flagName)
	if flag == nil {
		return nil
	}
	if !flag.Enabled {
		return flag.DefaultValue
	}

	if !e.evaluateFlag(flag, ctx) {
		return flag.DefaultValue
	}

	if len(flag.Variants) == 0 {
		return flag.DefaultValue
	}

	return e.selectVariant(flag.Variants, ctx.UserID)
}

// GetABTestVariant returns the A/B test variant for a user.
func (e *FlagEvaluator) GetABTestVariant(testName string, ctx *EvaluationContext) *ABVariant {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil || !e.config.Enabled {
		return nil
	}

	test := e.findABTest(testName)
	if test == nil || !test.Enabled {
		return nil
	}

	now := timeutil.NowTime()
	if !test.StartTime.IsZero() && now.Before(test.StartTime) {
		return nil
	}
	if !test.EndTime.IsZero() && now.After(test.EndTime) {
		return nil
	}

	// Check if user is in test traffic
	if !e.isInPercentage(ctx.UserID, testName, test.TrafficPct) {
		return nil
	}

	return e.selectABVariant(test.Variants, ctx.UserID, testName)
}

// GetConfigVersion returns the active configuration version.
func (e *FlagEvaluator) GetConfigVersion() *ConfigVersion {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil {
		return nil
	}

	for i := range e.config.Versions {
		if e.config.Versions[i].Active {
			return &e.config.Versions[i]
		}
	}
	return nil
}

// ListFlags returns all feature flags.
func (e *FlagEvaluator) ListFlags() []FeatureFlag {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil {
		return nil
	}
	return e.config.Flags
}

// ListABTests returns all A/B tests.
func (e *FlagEvaluator) ListABTests() []ABTest {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil {
		return nil
	}
	return e.config.ABTests
}

// HasFlag reports whether the named feature flag exists in the current config.
func (e *FlagEvaluator) HasFlag(flagName string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config == nil {
		return false
	}
	return e.findFlag(flagName) != nil
}

func (e *FlagEvaluator) findFlag(name string) *FeatureFlag {
	for i := range e.config.Flags {
		if e.config.Flags[i].Name == name {
			return &e.config.Flags[i]
		}
	}
	return nil
}

func (e *FlagEvaluator) findABTest(name string) *ABTest {
	for i := range e.config.ABTests {
		if e.config.ABTests[i].Name == name {
			return &e.config.ABTests[i]
		}
	}
	return nil
}

func (e *FlagEvaluator) evaluateFlag(flag *FeatureFlag, ctx *EvaluationContext) bool {
	if ctx == nil {
		// No context, use percentage-based evaluation with empty user
		return e.isInPercentage("", flag.Name, flag.Percentage)
	}

	// Check specific users first
	for _, userID := range flag.Users {
		if userID == ctx.UserID {
			return true
		}
	}

	// Check groups
	for _, group := range flag.Groups {
		for _, userGroup := range ctx.Groups {
			if group == userGroup {
				return true
			}
		}
	}

	// Check targeting rules
	if len(flag.Rules) > 0 {
		if !e.evaluateRules(flag.Rules, ctx) {
			return false
		}
	}

	// Percentage-based rollout
	if flag.Percentage > 0 {
		return e.isInPercentage(ctx.UserID, flag.Name, flag.Percentage)
	}

	return false
}

func (e *FlagEvaluator) evaluateRules(rules []TargetingRule, ctx *EvaluationContext) bool {
	for _, rule := range rules {
		if !e.evaluateRule(&rule, ctx) {
			return false
		}
	}
	return true
}

func (e *FlagEvaluator) evaluateRule(rule *TargetingRule, ctx *EvaluationContext) bool {
	if ctx.Attributes == nil {
		return false
	}

	attrValue, exists := ctx.Attributes[rule.Attribute]
	if !exists {
		return false
	}

	switch rule.Operator {
	case "eq":
		return len(rule.Values) > 0 && attrValue == rule.Values[0]
	case "neq":
		return len(rule.Values) > 0 && attrValue != rule.Values[0]
	case "in":
		for _, v := range rule.Values {
			if attrValue == v {
				return true
			}
		}
		return false
	case "not_in":
		for _, v := range rule.Values {
			if attrValue == v {
				return false
			}
		}
		return true
	case "contains":
		for _, v := range rule.Values {
			if contains(attrValue, v) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (e *FlagEvaluator) isInPercentage(userID, salt string, percentage float64) bool {
	if percentage <= 0 {
		return false
	}
	if percentage >= 100 {
		return true
	}

	hash := hashUserForPercentage(userID, salt)
	bucket := float64(hash%10000) / 100.0 // 0-99.99
	return bucket < percentage
}

func (e *FlagEvaluator) selectVariant(variants []FlagVariant, userID string) interface{} {
	if len(variants) == 0 {
		return nil
	}

	hash := hashUserForPercentage(userID, "variant")
	bucket := float64(hash%10000) / 100.0

	var cumulative float64
	for _, v := range variants {
		cumulative += v.Weight
		if bucket < cumulative {
			return v.Value
		}
	}

	return variants[len(variants)-1].Value
}

func (e *FlagEvaluator) selectABVariant(variants []ABVariant, userID, testName string) *ABVariant {
	if len(variants) == 0 {
		return nil
	}

	hash := hashUserForPercentage(userID, testName+"_variant")
	bucket := float64(hash%10000) / 100.0

	var cumulative float64
	for i := range variants {
		cumulative += variants[i].Weight
		if bucket < cumulative {
			return &variants[i]
		}
	}

	return &variants[len(variants)-1]
}

func hashUserForPercentage(userID, salt string) uint64 {
	h := sha256.New()
	h.Write([]byte(userID + ":" + salt))
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
