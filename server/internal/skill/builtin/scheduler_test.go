package builtin

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type mockSchedulerCronService struct {
	lastName        string
	lastDescription string
	lastSchedule    string
	lastHandler     string
	lastPayload     map[string]interface{}
}

func (m *mockSchedulerCronService) Create(name, description, schedule, handler string, payload map[string]interface{}) (CronJobInfo, error) {
	m.lastName = name
	m.lastDescription = description
	m.lastSchedule = schedule
	m.lastHandler = handler
	m.lastPayload = payload
	return CronJobInfo{
		ID:          "job-1",
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Handler:     handler,
		Enabled:     true,
		Status:      "scheduled",
	}, nil
}

func (m *mockSchedulerCronService) List() []CronJobInfo            { return nil }
func (m *mockSchedulerCronService) Get(string) (CronJobInfo, bool) { return CronJobInfo{}, false }
func (m *mockSchedulerCronService) Delete(string) error            { return nil }
func (m *mockSchedulerCronService) Trigger(string) error           { return nil }
func (m *mockSchedulerCronService) Enable(string) error            { return nil }
func (m *mockSchedulerCronService) Disable(string) error           { return nil }
func (m *mockSchedulerCronService) GetExecutions(string, int) ([]CronJobExecution, error) {
	return nil, nil
}

func TestSchedulerCreateAutoGeneratesNameWhenMissing(t *testing.T) {
	scheduler := NewScheduler()
	mockSvc := &mockSchedulerCronService{}
	scheduler.SetCronService(mockSvc)

	input := map[string]any{
		"action":      "create",
		"schedule":    "*/5 * * * *",
		"command":     "echo hello",
		"description": "Health check",
	}
	if err := scheduler.Validate(input); err != nil {
		t.Fatalf("validate: %v", err)
	}

	ctx := tools.WithSessionID(tools.WithUserID(context.Background(), "user-1"), "conv-1")
	result, err := scheduler.Execute(ctx, input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if mockSvc.lastName != "Health check" {
		t.Fatalf("auto-generated name = %q, want %q", mockSvc.lastName, "Health check")
	}
	if mockSvc.lastPayload["user_id"] != "user-1" {
		t.Fatalf("payload user_id = %v, want user-1", mockSvc.lastPayload["user_id"])
	}
	if mockSvc.lastPayload["conversation_id"] != "conv-1" {
		t.Fatalf("payload conversation_id = %v, want conv-1", mockSvc.lastPayload["conversation_id"])
	}
}
