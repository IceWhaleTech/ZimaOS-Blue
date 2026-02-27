package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// AnalyzeExecutor is the interface for the analyze backend.
// Decouples skill/builtin from tools package to avoid import cycles.
type AnalyzeExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Analyze is a built-in skill for deep-dive content analysis and report generation.
type Analyze struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor AnalyzeExecutor
}

// NewAnalyze creates a new analyze skill.
func NewAnalyze() *Analyze {
	return &Analyze{
		manifest: &skill.Manifest{
			ID:          "analyze",
			Name:        "Analyze",
			Version:     "1.0.0",
			Description: "Deep-dive analysis tool. Gathers data from URLs and web searches, then generates a comprehensive HTML report with statistics, insights, and visualizations.",
			Category:    "system",
			Icon:        "analyze",
			Tags:        []string{"analyze", "report", "research", "insights", "data"},
			Inputs: []skill.Parameter{
				{Name: "topic", Type: "string", Description: "Analysis topic / report title", Required: true},
				{Name: "urls", Type: "array", Description: "URLs to scrape for content (max 5)"},
				{Name: "text", Type: "string", Description: "Direct text content to analyze"},
				{Name: "search_queries", Type: "array", Description: "Web search queries to gather additional data (max 3)"},
				{Name: "lang", Type: "string", Description: "Output language (default: zh-CN)"},
			},
			Outputs: []skill.Parameter{
				{Name: "report_url", Type: "string", Description: "URL to the generated HTML report"},
			},
		},
	}
}

// SetExecutor injects the analyze backend.
func (a *Analyze) SetExecutor(e AnalyzeExecutor) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.executor = e
}

func (a *Analyze) Manifest() *skill.Manifest { return a.manifest }

func (a *Analyze) Validate(input map[string]any) error {
	// Derive topic from query/url when not provided.
	if _, ok := input["topic"]; !ok {
		if q, ok := input["query"].(string); ok && q != "" {
			input["topic"] = q
		} else if u, ok := input["url"].(string); ok && u != "" {
			input["topic"] = u
		} else {
			return fmt.Errorf("topic is required")
		}
	}

	// Normalize: promote "url" to "urls" array if urls not set.
	if _, ok := input["urls"]; !ok {
		if u, ok := input["url"].(string); ok && u != "" {
			input["urls"] = []interface{}{u}
		}
	}

	// Normalize: promote "query" to "search_queries" array if not set.
	if _, ok := input["search_queries"]; !ok {
		if q, ok := input["query"].(string); ok && q != "" {
			input["search_queries"] = []interface{}{q}
		}
	}

	return nil
}

func (a *Analyze) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	// Normalize input (defaults action, derives topic from query/url, etc.)
	if err := a.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	a.mu.RLock()
	exec := a.executor
	a.mu.RUnlock()

	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("analyze backend not available")), nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
	}

	result, err := exec.Execute(ctx, args)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	switch v := result.(type) {
	case string:
		var parsed map[string]any
		if json.Unmarshal([]byte(v), &parsed) == nil {
			return skill.NewResult(parsed), nil
		}
		return skill.NewResult(map[string]any{"result": v}), nil
	case map[string]any:
		return skill.NewResult(v), nil
	default:
		b, _ := json.Marshal(v)
		return skill.NewResult(map[string]any{"result": string(b)}), nil
	}
}
