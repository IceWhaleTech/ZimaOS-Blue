package builtin

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/remindertime"
)

type mockReminderPushService struct {
	lastOwnerID   string
	lastMessage   string
	lastRecurring string
	lastSessionID string
	lastFireAt    time.Time
	lastDeleteID  string
	listResult    []PushInfo
	clearCount    int64
}

func (m *mockReminderPushService) Add(_ context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string, untilAt *time.Time) (PushInfo, error) {
	m.lastOwnerID = ownerID
	m.lastMessage = message
	m.lastRecurring = recurring
	m.lastSessionID = sessionID
	m.lastFireAt = fireAt
	return PushInfo{
		ID:        "push_1",
		Message:   message,
		FireAt:    fireAt,
		Recurring: recurring,
		Status:    "scheduled",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockReminderPushService) List(_ context.Context, ownerID string) ([]PushInfo, error) {
	m.lastOwnerID = ownerID
	return m.listResult, nil
}

func (m *mockReminderPushService) Delete(_ context.Context, ownerID, id string) error {
	m.lastOwnerID = ownerID
	m.lastDeleteID = id
	return nil
}

func (m *mockReminderPushService) Clear(_ context.Context, ownerID string) (int64, error) {
	m.lastOwnerID = ownerID
	return m.clearCount, nil
}

func TestReminderSkill_ValidateInfersAddFromAliases(t *testing.T) {
	r := NewReminder()
	input := map[string]any{
		"content": "drink water",
		"when":    "10m",
	}
	if err := r.Validate(input); err != nil {
		t.Fatalf("expected alias input to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "add" {
		t.Fatalf("action = %q, want add", got)
	}
	if got, _ := input["message"].(string); got != "drink water" {
		t.Fatalf("message = %q, want drink water", got)
	}
	if got, _ := input["time"].(string); got != "10m" {
		t.Fatalf("time = %q, want 10m", got)
	}
}

func TestReminderSkill_ValidateInfersDeleteFromReminderIDAlias(t *testing.T) {
	r := NewReminder()
	input := map[string]any{
		"reminder_id": "push_1",
	}
	if err := r.Validate(input); err != nil {
		t.Fatalf("expected reminder_id alias to pass validation: %v", err)
	}
	if got, _ := input["action"].(string); got != "delete" {
		t.Fatalf("action = %q, want delete", got)
	}
	if got, _ := input["id"].(string); got != "push_1" {
		t.Fatalf("id = %q, want push_1", got)
	}
}

func TestReminderSkill_ExecuteAcceptsAliases(t *testing.T) {
	svc := &mockReminderPushService{}
	r := NewReminder()
	r.SetPushService(svc)

	res, err := r.Execute(context.Background(), map[string]any{
		"content": "drink water",
		"when":    "10m",
		"repeat":  "daily",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if svc.lastMessage != "drink water" {
		t.Fatalf("message = %q, want drink water", svc.lastMessage)
	}
	if svc.lastRecurring != "daily" {
		t.Fatalf("recurring = %q, want daily", svc.lastRecurring)
	}
}

func TestReminderSkill_ValidateInfersMissingTimeFromLocalizedMessage(t *testing.T) {
	r := NewReminder()
	input := map[string]any{
		"action":  "add",
		"message": "Programa un recordatori per dema a les 9 per enviar l'informe setmanal.",
	}
	if err := r.Validate(input); err != nil {
		t.Fatalf("expected localized reminder message to infer time: %v", err)
	}
	got, _ := input["time"].(string)
	if got == "" {
		t.Fatal("expected inferred time to be populated")
	}
	if _, err := remindertime.Parse(got); err != nil {
		t.Fatalf("inferred time %q should parse: %v", got, err)
	}
}

func TestReminderSkill_ExecuteInfersMissingTimeFromLocalizedMessage(t *testing.T) {
	svc := &mockReminderPushService{}
	r := NewReminder()
	r.SetPushService(svc)

	res, err := r.Execute(context.Background(), map[string]any{
		"action":  "add",
		"message": "Naplanuj pripominku na zitra v 9 pro odeslani tydenniho reportu.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error=%s", res.Error)
	}
	if svc.lastFireAt.IsZero() {
		t.Fatal("expected inferred fire time to be forwarded to service")
	}
	if svc.lastFireAt.Hour() != 9 || svc.lastFireAt.Minute() != 0 {
		t.Fatalf("fireAt = %s, want hour 9 minute 0", svc.lastFireAt.Format(time.RFC3339))
	}
}
