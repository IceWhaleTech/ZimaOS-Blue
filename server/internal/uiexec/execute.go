package uiexec

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func (e *Engine) stepsToActions(ui *SemanticUI, steps []LLMStep) ([]Action, error) {
	if len(steps) == 0 {
		return nil, ErrInvalidDSL
	}
	out := make([]Action, 0, len(steps))
	for _, step := range steps {
		stepType := strings.TrimSpace(strings.ToLower(step.Type))
		switch stepType {
		case "find":
			out = append(out, Action{Type: "find", Value: strings.TrimSpace(step.Name)})
		case "click", "focus":
			out = append(out, Action{Type: stepType})
		case "type":
			out = append(out, Action{Type: "type", Value: step.Value})
		case "invoke":
			out = append(out, Action{Type: "invoke", Value: step.Value})
		case "scroll":
			out = append(out, Action{Type: "scroll", Value: step.Value})
		case "wait":
			out = append(out, Action{Type: "wait", Value: step.Value})
		default:
			return nil, fmt.Errorf("%w: unsupported step type %q", ErrInvalidDSL, step.Type)
		}
	}
	return out, nil
}

func (e *Engine) apply(ctx context.Context, ui *SemanticUI, currentTarget *Node, action Action) (*SemanticUI, *Node, error) {
	if e == nil || e.rt == nil {
		return ui, currentTarget, fmt.Errorf("semantic runtime is unavailable")
	}
	if ui == nil || ui.Snapshot == nil {
		return ui, currentTarget, fmt.Errorf("semantic snapshot is unavailable")
	}

	actType := strings.TrimSpace(strings.ToLower(action.Type))
	switch actType {
	case "click", "focus":
		targetID := strings.TrimSpace(action.Target)
		if targetID == "" && currentTarget != nil {
			targetID = currentTarget.ID
		}
		if targetID == "" {
			return ui, currentTarget, fmt.Errorf("%w: %s requires a target", ErrInvalidDSL, actType)
		}
		res, err := e.rt.Execute(ctx, ui.Snapshot, a11y.Action{Op: actType, TargetID: targetID})
		if err != nil {
			return ui, currentTarget, err
		}
		nextUI := BuildSemanticUI(res.Snapshot)
		return nextUI, currentTarget, nil
	case "type":
		value := action.Value
		targetID := strings.TrimSpace(action.Target)
		if targetID == "" && currentTarget != nil {
			targetID = currentTarget.ID
		}
		if targetID == "" {
			// Try typing into the focused input if present.
			for _, n := range ui.Nodes {
				if n.Focused && hasAction(n.Actions, "type") {
					targetID = n.ID
					break
				}
			}
		}
		if targetID == "" {
			return ui, currentTarget, fmt.Errorf("%w: type requires a target or focused input", ErrInvalidDSL)
		}
		res, err := e.rt.Execute(ctx, ui.Snapshot, a11y.Action{Op: "type", TargetID: targetID, Value: value})
		if err != nil {
			return ui, currentTarget, err
		}
		nextUI := BuildSemanticUI(res.Snapshot)
		return nextUI, currentTarget, nil
	case "invoke":
		targetID := strings.TrimSpace(action.Target)
		if targetID == "" && currentTarget != nil {
			targetID = currentTarget.ID
		}
		if targetID == "" {
			return ui, currentTarget, fmt.Errorf("%w: invoke requires a target", ErrInvalidDSL)
		}
		op := strings.TrimSpace(action.Value)
		if op == "" {
			op = "click"
		}
		res, err := e.rt.Execute(ctx, ui.Snapshot, a11y.Action{Op: op, TargetID: targetID})
		if err != nil {
			return ui, currentTarget, err
		}
		nextUI := BuildSemanticUI(res.Snapshot)
		return nextUI, currentTarget, nil
	case "scroll":
		dir, lines := parseScrollValue(action.Value)
		res, err := e.rt.Execute(ctx, ui.Snapshot, a11y.Action{Op: "scroll", Direction: dir, Lines: lines})
		if err != nil {
			return ui, currentTarget, err
		}
		nextUI := BuildSemanticUI(res.Snapshot)
		return nextUI, currentTarget, nil
	case "wait":
		if err := e.wait(ctx, ui, action.Value); err != nil {
			return ui, currentTarget, err
		}
		refreshed, err := e.rt.Refresh(ctx, ui.Snapshot, a11y.RefreshHint{WindowID: ui.Snapshot.WindowID, Reason: "post_wait"})
		if err != nil {
			return ui, currentTarget, err
		}
		nextUI := BuildSemanticUI(refreshed)
		return nextUI, currentTarget, nil
	default:
		return ui, currentTarget, fmt.Errorf("%w: unsupported action type %q", ErrInvalidDSL, action.Type)
	}
}

func parseScrollValue(value string) (string, int) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "down", 6
	}
	parts := strings.Split(value, ":")
	dir := strings.TrimSpace(parts[0])
	lines := 6
	if len(parts) > 1 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && parsed > 0 {
			lines = parsed
		}
	}
	switch dir {
	case "up", "down", "left", "right":
	default:
		dir = "down"
	}
	return dir, lines
}

func (e *Engine) wait(ctx context.Context, ui *SemanticUI, condition string) error {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		// Default: small settle time.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(350 * time.Millisecond):
			return nil
		}
	}

	deadline := time.Now().Add(e.options.WaitTimeout)
	for {
		if e.waitConditionSatisfied(ui, condition) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("wait timeout: %s", condition)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(e.options.WaitPollInterval):
		}
		refreshed, err := e.rt.Refresh(ctx, ui.Snapshot, a11y.RefreshHint{WindowID: ui.Snapshot.WindowID, Reason: "wait_poll"})
		if err != nil {
			return err
		}
		ui = BuildSemanticUI(refreshed)
	}
}

func (e *Engine) waitConditionSatisfied(ui *SemanticUI, condition string) bool {
	if ui == nil || ui.Finder == nil {
		return false
	}
	cond := strings.TrimSpace(strings.ToLower(condition))
	switch {
	case strings.HasPrefix(cond, "exists:"):
		name := strings.TrimSpace(condition[len("exists:"):])
		return ui.Finder.FindNode(Query{Name: name}) != nil || ui.Finder.FindNode(Query{NameApprox: name}) != nil
	case strings.HasPrefix(cond, "focused_role:"):
		role := strings.TrimSpace(condition[len("focused_role:"):])
		s := StateFromUI(ui, uiTitle(ui))
		return strings.EqualFold(s.FocusedRole, normalizeText(role))
	default:
		// Treat as "exists:<condition>".
		name := strings.TrimSpace(condition)
		return ui.Finder.FindNode(Query{Name: name}) != nil || ui.Finder.FindNode(Query{NameApprox: name}) != nil
	}
}
