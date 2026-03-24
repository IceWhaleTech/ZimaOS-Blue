package builtin

import (
	"context"
	"testing"
)

type mockEmailExecutor struct {
	lastArgs map[string]interface{}
}

func (m *mockEmailExecutor) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.lastArgs = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.lastArgs[k] = v
	}
	return map[string]any{
		"action": args["action"],
		"id":     args["id"],
		"query":  args["query"],
		"from":   args["from"],
		"label":  args["label"],
	}, nil
}

func TestEmailSkill_ValidateInfersGetFromMessageIDAlias(t *testing.T) {
	e := NewEmail()
	input := map[string]any{
		"message_id": "mail_1",
	}
	if err := e.Validate(input); err != nil {
		t.Fatalf("expected message_id alias to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "get" {
		t.Fatalf("action = %q, want get", got)
	}
	if got, _ := input["id"].(string); got != "mail_1" {
		t.Fatalf("id = %q, want mail_1", got)
	}
}

func TestEmailSkill_ValidateInfersSearchFromAliases(t *testing.T) {
	e := NewEmail()
	input := map[string]any{
		"sender": "alice@example.com",
	}
	if err := e.Validate(input); err != nil {
		t.Fatalf("expected sender alias to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "search" {
		t.Fatalf("action = %q, want search", got)
	}
	if got, _ := input["from"].(string); got != "alice@example.com" {
		t.Fatalf("from = %q, want alice@example.com", got)
	}
}

func TestEmailSkill_ExecuteAcceptsActionAlias(t *testing.T) {
	exec := &mockEmailExecutor{}
	e := NewEmail()
	e.SetExecutor(exec)

	res, err := e.Execute(context.Background(), map[string]any{
		"action":  "read",
		"emailId": "mail_2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if got, _ := exec.lastArgs["action"].(string); got != "get" {
		t.Fatalf("action = %q, want get", got)
	}
	if got, _ := exec.lastArgs["id"].(string); got != "mail_2" {
		t.Fatalf("id = %q, want mail_2", got)
	}
}
