package builtin

import (
	"context"
	"testing"
)

type mockContactsExecutor struct {
	lastArgs map[string]interface{}
}

func (m *mockContactsExecutor) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.lastArgs = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.lastArgs[k] = v
	}
	return map[string]any{
		"action":       args["action"],
		"display_name": args["display_name"],
		"id":           args["id"],
		"query":        args["query"],
	}, nil
}

func TestContactsSkill_ValidateInfersCreateFromAliases(t *testing.T) {
	c := NewContacts()
	input := map[string]any{
		"name":   "Alice Chen",
		"emails": []string{"alice@example.com"},
	}
	if err := c.Validate(input); err != nil {
		t.Fatalf("expected alias input to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "create" {
		t.Fatalf("action = %q, want create", got)
	}
	if got, _ := input["display_name"].(string); got != "Alice Chen" {
		t.Fatalf("display_name = %q, want Alice Chen", got)
	}
}

func TestContactsSkill_ValidateInfersGetFromContactIDAlias(t *testing.T) {
	c := NewContacts()
	input := map[string]any{
		"contact_id": "contact-1",
	}
	if err := c.Validate(input); err != nil {
		t.Fatalf("expected contact_id alias to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "get" {
		t.Fatalf("action = %q, want get", got)
	}
	if got, _ := input["id"].(string); got != "contact-1" {
		t.Fatalf("id = %q, want contact-1", got)
	}
}

func TestContactsSkill_ExecuteAcceptsSearchAlias(t *testing.T) {
	exec := &mockContactsExecutor{}
	c := NewContacts()
	c.SetExecutor(exec)

	res, err := c.Execute(context.Background(), map[string]any{
		"search": "alice",
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
	if got, _ := exec.lastArgs["query"].(string); got != "alice" {
		t.Fatalf("query = %q, want alice", got)
	}
}
