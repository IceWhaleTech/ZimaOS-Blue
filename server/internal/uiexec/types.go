package uiexec

// Action is a minimal primitive action DSL.
// Note: The engine is responsible for resolving Target selectors into stable node IDs.
type Action struct {
	Type   string `json:"type"`             // click | type | focus | invoke | scroll | wait | find
	Target string `json:"target,omitempty"` // stable node id or selector
	Value  string `json:"value,omitempty"`  // input text, scroll direction, wait condition, etc.
}

// Node is a flattened semantic UI node suitable for query + execution.
type Node struct {
	ID      string   `json:"id"`
	Role    string   `json:"role,omitempty"`
	Name    string   `json:"name,omitempty"`
	Path    string   `json:"path,omitempty"`
	Actions []string `json:"actions,omitempty"` // click | type | focus | invoke | ...

	Visible bool `json:"visible,omitempty"`
	Enabled bool `json:"enabled,omitempty"`
	Focused bool `json:"focused,omitempty"`
}

type Query struct {
	Role              string `json:"role,omitempty"` // exact match (case-insensitive)
	Name              string `json:"name,omitempty"` // fuzzy string match
	NameApprox        string `json:"name_approx,omitempty"`
	RequireCapability string `json:"require_capability,omitempty"` // click | type | focus | invoke
}

type State struct {
	PageHint        string   `json:"page_hint,omitempty"`
	VisibleEntities []string `json:"visible_entities,omitempty"`
	FocusedRole     string   `json:"focused_role,omitempty"`
}

type Policy struct {
	TaskPattern  string   `json:"task_pattern"`
	StatePattern string   `json:"state_pattern"`
	Actions      []Action `json:"actions"`
	Cost         int      `json:"cost,omitempty"`
}
