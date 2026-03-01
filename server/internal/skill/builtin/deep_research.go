package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// DeepResearchExecutor is the interface for the deep_research backend.
// Decouples skill/builtin from deepresearch package internals.
type DeepResearchExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// DeepResearch is a pinned built-in skill for multi-step research.
type DeepResearch struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor DeepResearchExecutor
}

// NewDeepResearch creates a new deep research skill.
func NewDeepResearch() *DeepResearch {
	return &DeepResearch{
		manifest: &skill.Manifest{
			ID:          "deep_research",
			Name:        "Deep Research",
			Version:     "1.0.0",
			Description: "Run multi-step deep research with planning, evidence collection, and citation-based summary.",
			Category:    "system",
			Icon:        "search",
			Tags:        []string{"deep-research", "research", "evidence", "citations"},
			Inputs: []skill.Parameter{
				{Name: "query", Type: "string", Description: "Research query", Required: true},
				{Name: "mode", Type: "string", Description: "Search depth: fast, standard, deep"},
				{Name: "lang", Type: "string", Description: "Output language (default follows system locale)"},
				{Name: "max_sources", Type: "number", Description: "Optional source cap override"},
				{Name: "max_seconds", Type: "number", Description: "Optional time budget in seconds"},
			},
			Outputs: []skill.Parameter{
				{Name: "answer", Type: "string", Description: "Citation-backed answer"},
				{Name: "confidence", Type: "number", Description: "Confidence score (0-1)"},
			},
		},
	}
}

// SetExecutor injects deep research backend.
func (d *DeepResearch) SetExecutor(e DeepResearchExecutor) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.executor = e
}

func (d *DeepResearch) Manifest() *skill.Manifest { return d.manifest }

func (d *DeepResearch) Validate(input map[string]any) error {
	q, ok := input["query"]
	if !ok {
		return fmt.Errorf("query is required")
	}
	if s, ok := q.(string); !ok || s == "" {
		return fmt.Errorf("query must be a non-empty string")
	}
	if modeV, ok := input["mode"]; ok {
		mode, ok := modeV.(string)
		if !ok {
			return fmt.Errorf("mode must be a string")
		}
		switch mode {
		case "", "fast", "standard", "deep":
		default:
			return fmt.Errorf("mode must be one of: fast, standard, deep")
		}
	}
	return nil
}

func (d *DeepResearch) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := d.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	d.mu.RLock()
	exec := d.executor
	d.mu.RUnlock()
	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("deep research backend not available")), nil
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
