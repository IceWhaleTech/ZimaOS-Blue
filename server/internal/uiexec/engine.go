package uiexec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type EngineOptions struct {
	ScrollLines      int
	WaitPollInterval time.Duration
	WaitTimeout      time.Duration
}

type DirectIntent struct {
	Target string // target label
	Intent string // click | focus | type:... | invoke:...
}

type RunResult struct {
	Success        bool
	UsedDirect     bool
	UsedPolicy     bool
	UsedPlanner    bool
	ActionsApplied []Action
	FinalState     State
}

type Engine struct {
	rt      SemanticRuntime
	policy  *PolicyCache
	planner Planner
	options EngineOptions
}

func NewEngine(rt SemanticRuntime, policy *PolicyCache, planner Planner, options EngineOptions) *Engine {
	if options.ScrollLines <= 0 {
		options.ScrollLines = 6
	}
	if options.WaitPollInterval <= 0 {
		options.WaitPollInterval = 250 * time.Millisecond
	}
	if options.WaitTimeout <= 0 {
		options.WaitTimeout = 6 * time.Second
	}
	if planner == nil {
		planner = NoPlanner{}
	}
	return &Engine{
		rt:      rt,
		policy:  policy,
		planner: planner,
		options: options,
	}
}

func (e *Engine) Run(ctx context.Context, scope a11y.CaptureScope, task string, direct DirectIntent) (RunResult, error) {
	ui, state, err := e.sense(ctx, scope)
	if err != nil {
		return RunResult{}, err
	}

	result := RunResult{FinalState: state}

	var currentTarget *Node
	applied := make([]Action, 0, 16)
	exec := func(a Action) error {
		var applyErr error
		ui, currentTarget, applyErr = e.apply(ctx, ui, currentTarget, a)
		if applyErr == nil {
			applied = append(applied, a)
			result.FinalState = StateFromUI(ui, uiTitle(ui))
		}
		return applyErr
	}

	// 3) Opportunistic execution: try the shortest direct action first.
	if strings.TrimSpace(direct.Target) != "" && strings.TrimSpace(direct.Intent) != "" {
		if TryDirectAction(ui.Finder, exec, direct.Target, direct.Intent) {
			result.Success = true
			result.UsedDirect = true
			result.ActionsApplied = applied
			return result, nil
		}
	}

	// 4) Policy cache.
	if e.policy != nil {
		if p, ok := e.policy.Match(task, state); ok && len(p.Actions) > 0 {
			result.UsedPolicy = true
			okExec, execErr := e.executePlan(ctx, ui, &currentTarget, p.Actions, exec)
			if execErr != nil {
				result.ActionsApplied = applied
				return result, execErr
			}
			result.Success = okExec
			result.ActionsApplied = applied
			return result, nil
		}
	}

	// 5) LLM planner fallback (primitive DSL only).
	steps, planErr := e.planner.Plan(ctx, task, state)
	if planErr != nil {
		return result, planErr
	}
	result.UsedPlanner = true

	actions, convErr := e.stepsToActions(ui, steps)
	if convErr != nil {
		return result, convErr
	}

	okExec, execErr := e.executePlan(ctx, ui, &currentTarget, actions, exec)
	if execErr != nil {
		result.ActionsApplied = applied
		return result, execErr
	}
	result.Success = okExec
	result.ActionsApplied = applied

	// 8) Local learning: update cache with the executed path if it succeeded.
	if e.policy != nil && okExec {
		finalState := StateFromUI(ui, uiTitle(ui))
		e.policy.Upsert(Policy{
			TaskPattern:  TaskPatternFromTask(task),
			StatePattern: StatePatternFromState(state),
			Actions:      actions,
			Cost:         len(actions),
		}, true)
		result.FinalState = finalState
	}

	return result, nil
}

func (e *Engine) sense(ctx context.Context, scope a11y.CaptureScope) (*SemanticUI, State, error) {
	if e == nil || e.rt == nil {
		return nil, State{}, fmt.Errorf("semantic runtime is unavailable")
	}
	raw, err := e.rt.Capture(ctx, scope)
	if err != nil {
		return nil, State{}, err
	}
	snapshot, err := e.rt.Compile(raw, a11y.CompileOptions{Mode: raw.Mode})
	if err != nil {
		return nil, State{}, err
	}
	ui := BuildSemanticUI(snapshot)
	state := StateFromUI(ui, uiTitle(ui))
	return ui, state, nil
}

func uiTitle(ui *SemanticUI) string {
	if ui == nil || ui.Snapshot == nil {
		return ""
	}
	return ui.Snapshot.Title
}

func (e *Engine) executePlan(ctx context.Context, ui *SemanticUI, currentTarget **Node, actions []Action, exec func(Action) error) (bool, error) {
	for _, a := range actions {
		// "find" is a local-only action; the runtime never sees it.
		if strings.EqualFold(strings.TrimSpace(a.Type), "find") {
			target := strings.TrimSpace(a.Value)
			if target == "" {
				target = strings.TrimSpace(a.Target)
			}
			n := ui.Finder.FindNode(Query{Name: target})
			if n == nil {
				n = ui.Finder.FindNode(Query{NameApprox: target})
			}
			*currentTarget = n
			continue
		}
		if err := exec(a); err != nil {
			return false, err
		}
	}
	return true, nil
}
