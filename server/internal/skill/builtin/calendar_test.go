package builtin

import (
	"context"
	"testing"
)

type mockCalendarExecutor struct {
	lastArgs map[string]interface{}
}

func (m *mockCalendarExecutor) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.lastArgs = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.lastArgs[k] = v
	}
	return map[string]any{
		"action": args["action"],
		"title":  args["title"],
		"time":   args["time"],
		"id":     args["id"],
		"query":  args["query"],
	}, nil
}

func TestCalendarSkill_ValidateInfersCreateFromAliases(t *testing.T) {
	c := NewCalendar()
	input := map[string]any{
		"name": "Sprint Review",
		"when": "tomorrow 9am",
	}
	if err := c.Validate(input); err != nil {
		t.Fatalf("expected alias input to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "create" {
		t.Fatalf("action = %q, want create", got)
	}
	if got, _ := input["title"].(string); got != "Sprint Review" {
		t.Fatalf("title = %q, want Sprint Review", got)
	}
	if got, _ := input["time"].(string); got != "tomorrow 9am" {
		t.Fatalf("time = %q, want tomorrow 9am", got)
	}
}

func TestCalendarSkill_ValidateInfersGetFromEventIDAlias(t *testing.T) {
	c := NewCalendar()
	input := map[string]any{
		"event_id": "evt_1",
	}
	if err := c.Validate(input); err != nil {
		t.Fatalf("expected event_id alias to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "get" {
		t.Fatalf("action = %q, want get", got)
	}
	if got, _ := input["id"].(string); got != "evt_1" {
		t.Fatalf("id = %q, want evt_1", got)
	}
}

func TestCalendarSkill_ExecuteAcceptsSearchAlias(t *testing.T) {
	exec := &mockCalendarExecutor{}
	c := NewCalendar()
	c.SetExecutor(exec)

	res, err := c.Execute(context.Background(), map[string]any{
		"search": "team sync",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if got, _ := exec.lastArgs["action"].(string); got != "search" {
		t.Fatalf("action = %q, want search", got)
	}
	if got, _ := exec.lastArgs["query"].(string); got != "team sync" {
		t.Fatalf("query = %q, want team sync", got)
	}
}
