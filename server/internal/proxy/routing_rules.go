package proxy

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ModelOrigin indicates where a model runs.
type ModelOrigin string

const (
	OriginCloud ModelOrigin = "cloud"
	OriginEdge  ModelOrigin = "edge"
	OriginLocal ModelOrigin = "local"
)

// ModelTier indicates the cost tier of a model.
type ModelTier string

const (
	TierPremium  ModelTier = "premium"  // e.g., Opus, GPT-4
	TierStandard ModelTier = "standard" // e.g., Sonnet, GPT-4o
	TierEconomy  ModelTier = "economy"  // e.g., Haiku, GPT-4o-mini
	TierFree     ModelTier = "free"     // e.g., local Ollama models
)

// RoutingRule defines a single condition-based routing rule.
// Routes requests to cheaper/smaller models based on task complexity.
type RoutingRule struct {
	Name        string         `yaml:"name" json:"name"`
	Priority    int            `yaml:"priority" json:"priority"`
	Condition   RouteCondition `yaml:"condition" json:"condition"`
	TargetModel string         `yaml:"target_model" json:"target_model"`
	Origin      ModelOrigin    `yaml:"origin" json:"origin"`
	Tier        ModelTier      `yaml:"tier" json:"tier,omitempty"`
	Fallback    string         `yaml:"fallback" json:"fallback,omitempty"`
}

// RouteCondition defines when a rule matches. All non-zero fields must match (AND logic).
type RouteCondition struct {
	Header       string `yaml:"header,omitempty" json:"header,omitempty"`
	HeaderValue  string `yaml:"header_value,omitempty" json:"header_value,omitempty"`
	MaxBodyBytes int    `yaml:"max_body_bytes,omitempty" json:"max_body_bytes,omitempty"`
	ToolPattern  string `yaml:"tool_pattern,omitempty" json:"tool_pattern,omitempty"`
	SystemTag    string `yaml:"system_tag,omitempty" json:"system_tag,omitempty"`
}

// RouteRequest is the input to rule evaluation.
type RouteRequest struct {
	Headers       http.Header
	BodySize      int
	ToolNames     []string
	SystemMessage string
}

// RouteDecision is the output of rule evaluation.
type RouteDecision struct {
	Matched  bool        `json:"-"`
	Model    string      `json:"model"`
	Origin   ModelOrigin `json:"origin"`
	Tier     ModelTier   `json:"tier,omitempty"`
	Fallback string      `json:"fallback,omitempty"`
	Rule     string      `json:"rule"`
	Reason   string      `json:"reason"`
}

// noMatch is a shared sentinel for non-matching rules (zero allocs on miss).
var noMatch = &RouteDecision{Matched: false}

// hasCondition returns true if the condition has at least one non-zero field.
func (c *RouteCondition) hasCondition() bool {
	return c.Header != "" || c.MaxBodyBytes > 0 || c.ToolPattern != "" || c.SystemTag != ""
}

// Evaluate checks if a single rule matches the request (no pre-compiled regex).
// For hot-path usage, prefer RuleEngine which pre-compiles regexes and reason strings.
func (r *RoutingRule) Evaluate(req *RouteRequest) *RouteDecision {
	if !r.Condition.hasCondition() {
		return noMatch
	}
	var toolRe *regexp.Regexp
	if r.Condition.ToolPattern != "" {
		var err error
		toolRe, err = regexp.Compile(r.Condition.ToolPattern)
		if err != nil {
			return noMatch
		}
	}
	return evaluateCondition(&r.Condition, toolRe, req, r.Name, r.TargetModel, r.Origin, r.Tier, r.Fallback, "")
}

// evaluateCondition is the shared evaluation logic.
// preReason is a pre-built reason string (used by RuleEngine to avoid allocations).
func evaluateCondition(cond *RouteCondition, toolRe *regexp.Regexp, req *RouteRequest, name, target string, origin ModelOrigin, tier ModelTier, fallback, preReason string) *RouteDecision {
	// Header check
	if cond.Header != "" {
		val := req.Headers.Get(cond.Header)
		if !strings.EqualFold(val, cond.HeaderValue) {
			return noMatch
		}
	}

	// Body size check: 0 is a valid body size (empty request)
	if cond.MaxBodyBytes > 0 {
		if req.BodySize > cond.MaxBodyBytes {
			return noMatch
		}
	}

	// Tool pattern check: requires at least one matching tool name
	if cond.ToolPattern != "" {
		if toolRe == nil {
			return noMatch
		}
		matched := false
		for _, tool := range req.ToolNames {
			if toolRe.MatchString(tool) {
				matched = true
				break
			}
		}
		if !matched {
			return noMatch
		}
	}

	// System tag check
	if cond.SystemTag != "" {
		if !strings.Contains(req.SystemMessage, cond.SystemTag) {
			return noMatch
		}
	}

	return &RouteDecision{
		Matched:  true,
		Model:    target,
		Origin:   origin,
		Tier:     tier,
		Fallback: fallback,
		Rule:     name,
		Reason:   preReason,
	}
}

// compiledRoutingRule holds a rule with pre-compiled regex and pre-built reason string.
type compiledRoutingRule struct {
	rule   RoutingRule
	toolRe *regexp.Regexp
	reason string // pre-built at init time, zero allocs at eval time
}

// RuleEngine evaluates routing rules in priority order with pre-compiled regexes.
type RuleEngine struct {
	rules      []compiledRoutingRule
	needsTools  bool // pre-computed: any rule uses ToolPattern
	needsSystem bool // pre-computed: any rule uses SystemTag
}

// buildReason pre-builds the reason string for a rule condition.
func buildReason(cond *RouteCondition) string {
	parts := make([]string, 0, 4)
	if cond.Header != "" {
		parts = append(parts, "header "+cond.Header+"="+cond.HeaderValue)
	}
	if cond.MaxBodyBytes > 0 {
		parts = append(parts, "body_size<="+strconv.Itoa(cond.MaxBodyBytes))
	}
	if cond.ToolPattern != "" {
		parts = append(parts, "tool_pattern="+cond.ToolPattern)
	}
	if cond.SystemTag != "" {
		parts = append(parts, "system_tag="+cond.SystemTag)
	}
	return strings.Join(parts, ", ")
}

// NewRuleEngine creates a rule engine, sorting rules by priority (lower = higher).
func NewRuleEngine(rules []RoutingRule) *RuleEngine {
	sorted := make([]RoutingRule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	compiled := make([]compiledRoutingRule, len(sorted))
	var needsTools, needsSystem bool
	for i, r := range sorted {
		cr := compiledRoutingRule{
			rule:   r,
			reason: buildReason(&r.Condition),
		}
		if r.Condition.ToolPattern != "" {
			cr.toolRe, _ = regexp.Compile(r.Condition.ToolPattern)
			needsTools = true
		}
		if r.Condition.SystemTag != "" {
			needsSystem = true
		}
		compiled[i] = cr
	}
	return &RuleEngine{rules: compiled, needsTools: needsTools, needsSystem: needsSystem}
}

// Evaluate returns the first matching rule's decision, or nil if none match.
func (e *RuleEngine) Evaluate(req *RouteRequest) *RouteDecision {
	for i := range e.rules {
		cr := &e.rules[i]
		if !cr.rule.Condition.hasCondition() {
			continue
		}
		d := evaluateCondition(&cr.rule.Condition, cr.toolRe, req, cr.rule.Name, cr.rule.TargetModel, cr.rule.Origin, cr.rule.Tier, cr.rule.Fallback, cr.reason)
		if d.Matched {
			return d
		}
	}
	return nil
}
