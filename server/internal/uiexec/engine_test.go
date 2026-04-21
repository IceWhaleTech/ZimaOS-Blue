package uiexec

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type fakeRuntime struct {
	sdk      *a11y.SemanticSDK
	raw      a11y.RawTree
	executed []a11y.Action
}

func (r *fakeRuntime) Capture(ctx context.Context, scope a11y.CaptureScope) (a11y.RawTree, error) {
	return r.raw, nil
}

func (r *fakeRuntime) Compile(raw a11y.RawTree, options a11y.CompileOptions) (*a11y.Snapshot, error) {
	if r.sdk == nil {
		r.sdk = &a11y.SemanticSDK{}
	}
	return r.sdk.Compile(raw, options)
}

func (r *fakeRuntime) Execute(ctx context.Context, snapshot *a11y.Snapshot, action a11y.Action) (a11y.ExecutionResult, error) {
	r.executed = append(r.executed, action)
	// For unit tests we keep the snapshot stable.
	return a11y.ExecutionResult{Success: true, Snapshot: snapshot}, nil
}

func (r *fakeRuntime) Refresh(ctx context.Context, prior *a11y.Snapshot, hint a11y.RefreshHint) (*a11y.Snapshot, error) {
	raw, err := r.Capture(ctx, a11y.CaptureScope{WindowID: hint.WindowID})
	if err != nil {
		return nil, err
	}
	return r.Compile(raw, a11y.CompileOptions{Mode: raw.Mode})
}

type staticPlanner struct {
	calls int
	steps []LLMStep
}

func (p *staticPlanner) Plan(ctx context.Context, task string, state State) ([]LLMStep, error) {
	p.calls++
	return append([]LLMStep(nil), p.steps...), nil
}

func TestEngine_UsesPolicyCacheBeforePlanner(t *testing.T) {
	rt := &fakeRuntime{
		raw: a11y.RawTree{
			WindowID: "w1",
			Title:    "Example",
			Mode:     "ax",
			Root: &a11y.Node{
				Role: "window",
				Name: "Example",
				Children: []*a11y.Node{
					{Role: "button", Name: "Send", Interactive: true, Actions: []string{"AXPress"}},
				},
			},
		},
	}
	snapshot, err := rt.Compile(rt.raw, a11y.CompileOptions{Mode: "ax"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	ui := BuildSemanticUI(snapshot)
	send := ui.Finder.FindNode(Query{Name: "Send"})
	if send == nil {
		t.Fatalf("expected to find Send")
	}

	cache := NewPolicyCache()
	task := "click send"
	state := StateFromUI(ui, ui.Snapshot.Title)
	cache.Upsert(Policy{
		TaskPattern:  TaskPatternFromTask(task),
		StatePattern: StatePatternFromState(state),
		Actions:      []Action{{Type: "click", Target: send.ID}},
		Cost:         1,
	}, true)

	planner := &staticPlanner{steps: []LLMStep{{Type: "find", Name: "Send"}, {Type: "click"}}}
	engine := NewEngine(rt, cache, planner, EngineOptions{})

	_, runErr := engine.Run(context.Background(), a11y.CaptureScope{WindowID: "w1"}, task, DirectIntent{})
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	if planner.calls != 0 {
		t.Fatalf("planner.calls = %d, want 0", planner.calls)
	}
	if len(rt.executed) != 1 || rt.executed[0].Op == "" {
		t.Fatalf("executed = %#v, want single action", rt.executed)
	}
}

func TestEngine_FallsBackToPlannerOnCacheMiss(t *testing.T) {
	rt := &fakeRuntime{
		raw: a11y.RawTree{
			WindowID: "w1",
			Title:    "Example",
			Mode:     "ax",
			Root: &a11y.Node{
				Role: "window",
				Name: "Example",
				Children: []*a11y.Node{
					{Role: "button", Name: "Send", Interactive: true, Actions: []string{"AXPress"}},
				},
			},
		},
	}
	cache := NewPolicyCache()
	planner := &staticPlanner{steps: []LLMStep{{Type: "find", Name: "Send"}, {Type: "click"}}}
	engine := NewEngine(rt, cache, planner, EngineOptions{})

	_, runErr := engine.Run(context.Background(), a11y.CaptureScope{WindowID: "w1"}, "click send", DirectIntent{})
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	if planner.calls != 1 {
		t.Fatalf("planner.calls = %d, want 1", planner.calls)
	}
	if len(rt.executed) == 0 {
		t.Fatalf("expected some executed actions")
	}
}
