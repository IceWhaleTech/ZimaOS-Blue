package tools

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockPushService implements PushServiceInterface for testing.
type mockPushService struct {
	addCalled   bool
	addOwner    string
	addMessage  string
	addSession  string
	listResult  []PushResult
	deletedID   string
	clearedUser string
	clearCount  int64
	err         error
}

func (m *mockPushService) Add(_ context.Context, ownerID, message string, _ time.Time, _, sessionID string) (PushResult, error) {
	m.addCalled = true
	m.addOwner = ownerID
	m.addMessage = message
	m.addSession = sessionID
	if m.err != nil {
		return PushResult{}, m.err
	}
	return PushResult{ID: "push-1", Message: message, Status: "pending"}, nil
}

func (m *mockPushService) List(_ context.Context, ownerID string) ([]PushResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.listResult, nil
}

func (m *mockPushService) Delete(_ context.Context, ownerID, id string) error {
	m.deletedID = id
	return m.err
}

func (m *mockPushService) Clear(_ context.Context, ownerID string) (int64, error) {
	m.clearedUser = ownerID
	return m.clearCount, m.err
}

func TestPushToolDefinition(t *testing.T) {
	tool := NewPushTool(&mockPushService{})
	def := tool.Definition()

	if def.Name != "reminder" {
		t.Errorf("Name = %q, want %q", def.Name, "reminder")
	}
	if def.Icon != "notifications" {
		t.Errorf("Icon = %q, want %q", def.Icon, "notifications")
	}

	// session_id should NOT be in the parameter definition (auto-injected from context)
	props, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, has := props["session_id"]; has {
		t.Error("session_id should not be in tool parameters (auto-injected from context)")
	}
	// action should be present
	if _, has := props["action"]; !has {
		t.Error("expected action parameter")
	}
}

func TestPushToolExecuteAdd(t *testing.T) {
	svc := &mockPushService{}
	tool := NewPushTool(svc)

	ctx := context.Background()
	ctx = WithUserID(ctx, "user-1")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "add",
		"message": "drink water",
		"time":    "1h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled {
		t.Error("expected Add to be called")
	}
	if svc.addOwner != "user-1" {
		t.Errorf("owner = %q, want %q", svc.addOwner, "user-1")
	}
	if svc.addMessage != "drink water" {
		t.Errorf("message = %q, want %q", svc.addMessage, "drink water")
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if m["reminder"] == nil {
		t.Error("expected reminder in result")
	}
}

func TestPushToolExecuteAdd_SessionIDFromContext(t *testing.T) {
	svc := &mockPushService{}
	tool := NewPushTool(svc)

	ctx := context.Background()
	ctx = WithUserID(ctx, "user-1")
	ctx = WithSessionID(ctx, "conv-abc")

	_, err := tool.Execute(ctx, map[string]interface{}{
		"action":  "add",
		"message": "test",
		"time":    "30m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.addSession != "conv-abc" {
		t.Errorf("session_id = %q, want %q (should be auto-extracted from context)", svc.addSession, "conv-abc")
	}
}

func TestPushToolExecuteAdd_MissingMessage(t *testing.T) {
	tool := NewPushTool(&mockPushService{})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "add",
		"time":   "1h",
	})
	if err == nil {
		t.Error("expected error for missing message")
	}
}

func TestPushToolExecuteAdd_MissingTime(t *testing.T) {
	tool := NewPushTool(&mockPushService{})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "add",
		"message": "test",
	})
	if err == nil {
		t.Error("expected error for missing time")
	}
}

func TestPushToolExecuteList(t *testing.T) {
	svc := &mockPushService{
		listResult: []PushResult{
			{ID: "p1", Message: "A"},
			{ID: "p2", Message: "B"},
		},
	}
	tool := NewPushTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if m["count"] != 2 {
		t.Errorf("count = %v, want 2", m["count"])
	}
}

func TestPushToolExecuteDelete(t *testing.T) {
	svc := &mockPushService{}
	tool := NewPushTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "delete",
		"id":     "push-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.deletedID != "push-1" {
		t.Errorf("deleted ID = %q, want %q", svc.deletedID, "push-1")
	}
}

func TestPushToolExecuteDelete_MissingID(t *testing.T) {
	tool := NewPushTool(&mockPushService{})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "delete",
	})
	if err == nil {
		t.Error("expected error for missing id")
	}
}

func TestPushToolExecuteClear(t *testing.T) {
	svc := &mockPushService{clearCount: 5}
	tool := NewPushTool(svc)

	ctx := WithUserID(context.Background(), "user-1")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "clear",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if m["cleared"] != int64(5) {
		t.Errorf("cleared = %v, want 5", m["cleared"])
	}
	if svc.clearedUser != "user-1" {
		t.Errorf("cleared user = %q, want %q", svc.clearedUser, "user-1")
	}
}

func TestPushToolExecute_UnknownAction(t *testing.T) {
	tool := NewPushTool(&mockPushService{})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "explode",
	})
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestPushToolExecute_MissingAction(t *testing.T) {
	tool := NewPushTool(&mockPushService{})
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestPushToolExecute_NilService(t *testing.T) {
	tool := NewPushTool(nil)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})
	if err == nil {
		t.Error("expected error for nil service")
	}
}

func TestPushToolExecute_DefaultUserID(t *testing.T) {
	svc := &mockPushService{}
	tool := NewPushTool(svc)

	// No WithUserID — should default to "default"
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "add",
		"message": "test",
		"time":    "1h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.addOwner != "default" {
		t.Errorf("owner = %q, want %q", svc.addOwner, "default")
	}
}

func TestPushToolExecuteAdd_ServiceError(t *testing.T) {
	svc := &mockPushService{err: fmt.Errorf("db error")}
	tool := NewPushTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "add",
		"message": "test",
		"time":    "1h",
	})
	if err == nil {
		t.Error("expected error from service")
	}
}

func TestRegisterPushTool(t *testing.T) {
	registry := NewRegistry()
	svc := &mockPushService{}
	RegisterPushTool(registry, svc)

	if registry.Get("reminder") == nil {
		t.Error("expected reminder tool to be registered")
	}
}

func TestRegisterPushTool_NilService(t *testing.T) {
	registry := NewRegistry()
	RegisterPushTool(registry, nil)

	if registry.Get("reminder") != nil {
		t.Error("expected reminder tool NOT to be registered with nil service")
	}
}

func TestParsePushTime_Duration(t *testing.T) {
	before := time.Now()
	got, err := parsePushTime("1h30m")
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := before.Add(90 * time.Minute)
	if got.Before(expected.Add(-time.Second)) || got.After(after.Add(90*time.Minute+time.Second)) {
		t.Errorf("parsePushTime(1h30m) = %v, expected ~%v", got, expected)
	}
}

func TestParsePushTime_RFC3339(t *testing.T) {
	got, err := parsePushTime("2026-06-15T10:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 6 || got.Day() != 15 {
		t.Errorf("parsePushTime(RFC3339) = %v", got)
	}
}

func TestParsePushTime_DateTime(t *testing.T) {
	got, err := parsePushTime("2026-03-01 09:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 3 || got.Day() != 1 || got.Hour() != 9 {
		t.Errorf("parsePushTime(datetime) = %v", got)
	}
}

func TestParsePushTime_Invalid(t *testing.T) {
	_, err := parsePushTime("not-a-time")
	if err == nil {
		t.Error("expected error for invalid time")
	}
}
