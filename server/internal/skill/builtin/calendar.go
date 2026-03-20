package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// CalendarExecutor is the interface for the native calendar backend.
type CalendarExecutor interface {
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Calendar is a built-in skill wrapper over the native calendar tool.
type Calendar struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	executor CalendarExecutor
}

// NewCalendar creates a new calendar skill.
func NewCalendar() *Calendar {
	return &Calendar{
		manifest: &skill.Manifest{
			ID:          "calendar",
			Name:        "Calendar",
			Version:     "1.0.0",
			Description: "Calendar and daily planning skill. Creates events, lists agenda items, and produces a daily summary across events, reminders, jobs, and priority emails.",
			Category:    "system",
			Icon:        "calendar",
			Tags:        []string{"calendar", "agenda", "schedule", "productivity"},
			Inputs: []skill.Parameter{
				{Name: "action", Type: "string", Description: "create, list, get, search, today, daily_summary"},
				{Name: "id", Type: "string", Description: "Event ID for get"},
				{Name: "title", Type: "string", Description: "Event title for create"},
				{Name: "time", Type: "string", Description: "Natural language or RFC3339 time for create"},
				{Name: "end", Type: "string", Description: "Optional end time"},
				{Name: "duration", Type: "string", Description: "Optional duration when end is omitted"},
				{Name: "query", Type: "string", Description: "Search query"},
				{Name: "date", Type: "string", Description: "Date anchor for today/daily_summary"},
				{Name: "location", Type: "string", Description: "Event location"},
				{Name: "notes", Type: "string", Description: "Event notes"},
				{Name: "limit", Type: "integer", Description: "Maximum results to return"},
			},
			Outputs: []skill.Parameter{
				{Name: "event", Type: "object", Description: "Single calendar event"},
				{Name: "events", Type: "array", Description: "Calendar event list"},
				{Name: "summary", Type: "string", Description: "Human-readable daily summary"},
			},
		},
	}
}

// SetExecutor injects the native calendar backend.
func (c *Calendar) SetExecutor(exec CalendarExecutor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.executor = exec
}

func (c *Calendar) Manifest() *skill.Manifest { return c.manifest }

func (c *Calendar) Validate(input map[string]any) error {
	if input == nil {
		return fmt.Errorf("input is required")
	}
	action, _ := input["action"].(string)
	switch action {
	case "create":
		if _, ok := input["title"]; !ok {
			return fmt.Errorf("title is required for create")
		}
		if _, ok := input["time"]; !ok {
			if _, startOK := input["start"]; !startOK {
				return fmt.Errorf("time is required for create")
			}
		}
	case "get":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for get")
		}
	}
	return nil
}

func (c *Calendar) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := c.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	c.mu.RLock()
	exec := c.executor
	c.mu.RUnlock()
	if exec == nil {
		return skill.NewErrorResult(fmt.Errorf("calendar backend not available")), nil
	}

	args := make(map[string]interface{}, len(input))
	for k, v := range input {
		args[k] = v
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
	case map[string]interface{}:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return skill.NewResult(out), nil
	default:
		b, _ := json.Marshal(typed)
		return skill.NewResult(map[string]any{"result": string(b)}), nil
	}
}
