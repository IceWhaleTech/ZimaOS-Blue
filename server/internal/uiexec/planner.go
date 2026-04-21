package uiexec

import "context"

// LLMStep is the only UI-related output an LLM is allowed to produce in this design.
// The engine resolves "find" into stable node IDs and validates executability locally.
type LLMStep struct {
	Type  string `json:"type"`            // find | click | focus | type | invoke | scroll | wait
	Name  string `json:"name,omitempty"`  // for find
	Role  string `json:"role,omitempty"`  // for find (optional)
	Value string `json:"value,omitempty"` // for type/scroll/wait/invoke payload
}

type Planner interface {
	Plan(ctx context.Context, task string, state State) ([]LLMStep, error)
}

type NoPlanner struct{}

func (p NoPlanner) Plan(ctx context.Context, task string, state State) ([]LLMStep, error) {
	return nil, ErrPlannerUnavailable
}
