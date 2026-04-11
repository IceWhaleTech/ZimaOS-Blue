package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// AdvisorExecutor is the interface for the advisor backend.
type AdvisorExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Advisor is a built-in skill for decision support and best-practice guidance.
type Advisor struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor AdvisorExecutor
}

// NewAdvisor creates a new advisor skill.
func NewAdvisor() *Advisor {
	return &Advisor{
		manifest: &skill.Manifest{
			ID:          "advisor",
			Name:        "Advisor",
			Version:     "1.0.0",
			Description: "Decision advisor for selection, replacement, migration, and best-practice questions.",
			Category:    "system",
			Icon:        "advisor",
			Tags:        []string{"advisor", "decision", "compare", "replace", "migration", "best-practice"},
			Inputs: []skill.Parameter{
				{Name: "action", Type: "string", Description: "run|status"},
				{Name: "question", Type: "string", Description: "Decision question", Required: true},
				{Name: "job_id", Type: "string", Description: "Async advisor job ID for action=status"},
				{Name: "category", Type: "string", Description: "architecture|language|framework|library|process|migration|ops"},
				{Name: "decision_mode", Type: "string", Description: "recommend|compare|review|replace|best_practice"},
				{Name: "candidates", Type: "array", Description: "Optional candidate list"},
				{Name: "context", Type: "object", Description: "Optional context such as stack, team_size, data_scale, deployment, current_solution"},
				{Name: "constraints", Type: "object", Description: "Optional decision constraints"},
				{Name: "grounding", Type: "string", Description: "auto (default), none, or web"},
				{Name: "depth", Type: "string", Description: "quick, standard (default), or deep"},
				{Name: "output", Type: "string", Description: "decision_memo|scorecard|decision_pack"},
				{Name: "scorecard_pack", Type: "string", Description: "auto|solution_selection_v1|migration_v1|architecture_v1|process_v1"},
				{Name: "scorecard_weights", Type: "object", Description: "Criterion weight overrides keyed by criterion id"},
				{Name: "lang", Type: "string", Description: "Preferred output language"},
				{Name: "wait", Type: "boolean", Description: "Wait for deep advisor completion"},
				{Name: "wait_timeout_seconds", Type: "number", Description: "Optional max wait time for deep advisor"},
				{Name: "poll_interval_ms", Type: "number", Description: "Polling interval for deep advisor"},
			},
			Outputs: []skill.Parameter{
				{Name: "recommendation", Type: "string", Description: "Top recommendation"},
				{Name: "confidence", Type: "number", Description: "Confidence score (0-1)"},
				{Name: "tradeoffs", Type: "array", Description: "Key tradeoffs"},
				{Name: "risks", Type: "array", Description: "Key risks"},
				{Name: "winner", Type: "string", Description: "Top ranked candidate when output=scorecard"},
				{Name: "pack_id", Type: "string", Description: "Scorecard criteria pack identifier"},
			},
		},
	}
}

// SetExecutor injects the advisor backend.
func (a *Advisor) SetExecutor(e AdvisorExecutor) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.executor = e
}

func (a *Advisor) Manifest() *skill.Manifest { return a.manifest }

func (a *Advisor) Validate(input map[string]any) error {
	normalizeAdvisorSkillInput(input)
	question, _ := input["question"].(string)
	if strings.TrimSpace(question) == "" {
		return fmt.Errorf("question is required")
	}
	return nil
}

func (a *Advisor) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := a.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	a.mu.RLock()
	exec := a.executor
	a.mu.RUnlock()
	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("advisor backend not available")), nil
	}

	args := make(map[string]interface{}, len(input))
	for key, value := range input {
		args[key] = value
	}
	result, err := exec.Execute(ctx, args)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	switch typed := result.(type) {
	case string:
		var parsed map[string]any
		if json.Unmarshal([]byte(typed), &parsed) == nil {
			return skill.NewResult(parsed), nil
		}
		return skill.NewResult(map[string]any{"result": typed}), nil
	case map[string]any:
		return skill.NewResult(typed), nil
	default:
		raw, _ := json.Marshal(typed)
		return skill.NewResult(map[string]any{"result": string(raw)}), nil
	}
}

func normalizeAdvisorSkillInput(input map[string]any) {
	normalizeStringAlias(input, "question", "query", "topic", "prompt", "message", "input")
	normalizeStringAlias(input, "category", "kind", "type")
	normalizeStringAlias(input, "decision_mode", "decisionMode")
	normalizeStringAlias(input, "grounding")
	normalizeStringAlias(input, "depth")
	normalizeStringAlias(input, "output")
	normalizeStringAlias(input, "scorecard_pack", "scorecardPack")
	normalizeStringAlias(input, "job_id", "jobId", "id")
	normalizeStringAlias(input, "lang", "language", "locale")

	if question, ok := input["question"].(string); ok {
		input["question"] = strings.TrimSpace(question)
	}
	if action, ok := input["action"].(string); !ok || strings.TrimSpace(action) == "" {
		input["action"] = "run"
	}
	if grounding, ok := input["grounding"].(string); !ok || strings.TrimSpace(grounding) == "" {
		input["grounding"] = "auto"
	}
	if depth, ok := input["depth"].(string); !ok || strings.TrimSpace(depth) == "" {
		input["depth"] = "standard"
	}
	if output, ok := input["output"].(string); !ok || strings.TrimSpace(output) == "" {
		input["output"] = "decision_memo"
	}
	if scorecardPack, ok := input["scorecard_pack"].(string); !ok || strings.TrimSpace(scorecardPack) == "" {
		input["scorecard_pack"] = "auto"
	}
}
