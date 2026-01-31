package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// Timer is a built-in timer skill
type Timer struct {
	manifest *skill.Manifest
	timers   map[string]*time.Timer
}

// NewTimer creates a new timer skill
func NewTimer() *Timer {
	return &Timer{
		manifest: &skill.Manifest{
			ID:          "timer",
			Name:        "Timer",
			Version:     "1.0.0",
			Description: "Set timers and stopwatch functionality",
			Category:    "productivity",
			Icon:        "timer",
			Tags:        []string{"timer", "stopwatch", "countdown", "productivity"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action to perform: start, stop, status",
					Required:    true,
				},
				{
					Name:        "duration",
					Type:        "string",
					Description: "Duration for timer (e.g., '5m', '1h30m', '30s')",
					Required:    false,
				},
				{
					Name:        "name",
					Type:        "string",
					Description: "Name/ID for the timer",
					Required:    false,
					Default:     "default",
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "status",
					Type:        "string",
					Description: "Timer status",
				},
				{
					Name:        "remaining",
					Type:        "string",
					Description: "Remaining time",
				},
			},
		},
		timers: make(map[string]*time.Timer),
	}
}

// Manifest returns the skill manifest
func (t *Timer) Manifest() *skill.Manifest {
	return t.manifest
}

// Validate validates the input parameters
func (t *Timer) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{"start": true, "stop": true, "status": true}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s (must be start, stop, or status)", actionStr)
	}

	if actionStr == "start" {
		if _, ok := input["duration"]; !ok {
			return fmt.Errorf("duration is required for start action")
		}
	}

	return nil
}

// Execute executes the timer skill
func (t *Timer) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)
	name := "default"
	if n, ok := input["name"].(string); ok {
		name = n
	}

	switch action {
	case "start":
		durationStr := input["duration"].(string)
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("invalid duration format: %v", err)), nil
		}

		// Stop existing timer if any
		if existing, ok := t.timers[name]; ok {
			existing.Stop()
		}

		t.timers[name] = time.NewTimer(duration)
		return skill.NewResult(map[string]any{
			"status":   "started",
			"name":     name,
			"duration": durationStr,
			"message":  fmt.Sprintf("Timer '%s' started for %s", name, durationStr),
		}), nil

	case "stop":
		if timer, ok := t.timers[name]; ok {
			timer.Stop()
			delete(t.timers, name)
			return skill.NewResult(map[string]any{
				"status":  "stopped",
				"name":    name,
				"message": fmt.Sprintf("Timer '%s' stopped", name),
			}), nil
		}
		return skill.NewResult(map[string]any{
			"status":  "not_found",
			"name":    name,
			"message": fmt.Sprintf("Timer '%s' not found", name),
		}), nil

	case "status":
		if _, ok := t.timers[name]; ok {
			return skill.NewResult(map[string]any{
				"status":  "running",
				"name":    name,
				"message": fmt.Sprintf("Timer '%s' is running", name),
			}), nil
		}
		return skill.NewResult(map[string]any{
			"status":  "not_found",
			"name":    name,
			"message": fmt.Sprintf("Timer '%s' not found", name),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}
