package tools

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openProductivityTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "productivity_tools_test.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type mockReminderService struct {
	list []PushResult
}

func (m *mockReminderService) Add(_ context.Context, _ string, _ string, _ time.Time, _ string, _ string, _ *time.Time) (PushResult, error) {
	return PushResult{}, nil
}

func (m *mockReminderService) List(_ context.Context, _ string) ([]PushResult, error) {
	return append([]PushResult(nil), m.list...), nil
}

func (m *mockReminderService) Delete(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockReminderService) Clear(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

type mockCronService struct {
	jobs []CronJobInfo
}

func (m *mockCronService) Config(_ context.Context) CronConfigInfo { return CronConfigInfo{} }

func (m *mockCronService) Handlers(_ context.Context) []string { return nil }

func (m *mockCronService) ListJobs(_ context.Context) ([]CronJobInfo, error) {
	return append([]CronJobInfo(nil), m.jobs...), nil
}

func (m *mockCronService) GetJob(_ context.Context, _ string) (*CronJobInfo, error) { return nil, nil }

func (m *mockCronService) CreateJob(_ context.Context, _, _, _, _ string, _ map[string]interface{}) (*CronJobInfo, error) {
	return nil, nil
}

func (m *mockCronService) UpdateJob(_ context.Context, _, _, _, _ string, _ map[string]interface{}) error {
	return nil
}

func (m *mockCronService) DeleteJob(_ context.Context, _ string) error { return nil }

func (m *mockCronService) EnableJob(_ context.Context, _ string) error { return nil }

func (m *mockCronService) DisableJob(_ context.Context, _ string) error { return nil }

func (m *mockCronService) TriggerJob(_ context.Context, _ string) error { return nil }

func (m *mockCronService) GetExecutions(_ context.Context, _ string, _ int) ([]CronExecutionInfo, error) {
	return nil, nil
}

func TestEmailTool_Execute_SearchArchiveLabelAndSummarize(t *testing.T) {
	now := time.Date(2026, time.March, 20, 10, 0, 0, 0, time.UTC)
	db := openProductivityTestDB(t)

	svc, err := NewLocalEmailService(db)
	if err != nil {
		t.Fatalf("new email service: %v", err)
	}
	svc.SetNowFunc(func() time.Time { return now })
	err = svc.SeedFixtures(context.Background(), "user-1", []EmailMessage{
		{
			ID:          "mail-1",
			ThreadID:    "thread-1",
			Subject:     "Budget approval needed today",
			SenderName:  "Alice Chen",
			SenderEmail: "alice@example.com",
			Snippet:     "Please approve the budget before 5 PM.",
			Body:        "Action required: approve the budget before 5 PM today.",
			Labels:      []string{"work"},
			Priority:    "high",
			Unread:      true,
			ReceivedAt:  now.Add(-30 * time.Minute),
			UpdatedAt:   now.Add(-30 * time.Minute),
		},
		{
			ID:          "mail-2",
			ThreadID:    "thread-2",
			Subject:     "Lunch plans",
			SenderName:  "Bob",
			SenderEmail: "bob@example.com",
			Snippet:     "Want to grab lunch later?",
			Body:        "We can meet at noon.",
			Labels:      []string{"personal"},
			Priority:    "normal",
			Unread:      false,
			ReceivedAt:  now.Add(-2 * time.Hour),
			UpdatedAt:   now.Add(-2 * time.Hour),
		},
	})
	if err != nil {
		t.Fatalf("seed email fixtures: %v", err)
	}

	tool := NewEmailTool(svc)
	ctx := WithLang(WithUserID(context.Background(), "user-1"), "en-US")
	serviceMessages, err := svc.List(context.Background(), "user-1", EmailQueryOptions{Query: "approval"})
	if err != nil {
		t.Fatalf("service list email: %v", err)
	}
	if len(serviceMessages) != 1 {
		t.Fatalf("service list count = %d, want 1", len(serviceMessages))
	}

	searchResultAny, err := tool.Execute(ctx, map[string]interface{}{
		"action": "search",
		"query":  "approval",
	})
	if err != nil {
		t.Fatalf("search email: %v", err)
	}
	searchResult := searchResultAny.(map[string]interface{})
	if got := int(searchResult["count"].(int)); got != 1 {
		t.Fatalf("search count = %d, want 1", got)
	}

	archiveResultAny, err := tool.Execute(ctx, map[string]interface{}{
		"action": "archive",
		"id":     "mail-1",
	})
	if err != nil {
		t.Fatalf("archive email: %v", err)
	}
	archiveResult := archiveResultAny.(map[string]interface{})
	archivedEmail := archiveResult["email"].(map[string]interface{})
	if archived, _ := archivedEmail["archived"].(bool); !archived {
		t.Fatal("expected archived email=true after archive action")
	}

	labelResultAny, err := tool.Execute(ctx, map[string]interface{}{
		"action": "label",
		"id":     "mail-1",
		"labels": []interface{}{"follow-up"},
	})
	if err != nil {
		t.Fatalf("label email: %v", err)
	}
	labelResult := labelResultAny.(map[string]interface{})
	labels := labelResult["email"].(map[string]interface{})["labels"].([]string)
	foundFollowUp := false
	for _, label := range labels {
		if label == "follow-up" {
			foundFollowUp = true
			break
		}
	}
	if !foundFollowUp {
		t.Fatalf("expected follow-up label in %v", labels)
	}

	summaryResultAny, err := tool.Execute(ctx, map[string]interface{}{
		"action": "summarize",
	})
	if err != nil {
		t.Fatalf("summarize email: %v", err)
	}
	summaryResult := summaryResultAny.(map[string]interface{})
	if got := summaryResult["high_priority_count"].(int); got != 1 {
		t.Fatalf("high_priority_count = %d, want 1", got)
	}
	if got := summaryResult["actionable_count"].(int); got < 1 {
		t.Fatalf("actionable_count = %d, want >= 1", got)
	}
	if summary := summaryResult["summary"].(string); summary == "" {
		t.Fatal("expected human-readable summary")
	}
}

func TestCalendarTool_Execute_DailySummaryAggregatesEventsRemindersJobsAndEmails(t *testing.T) {
	now := time.Date(2026, time.March, 20, 9, 0, 0, 0, time.UTC)
	db := openProductivityTestDB(t)

	emailSvc, err := NewLocalEmailService(db)
	if err != nil {
		t.Fatalf("new email service: %v", err)
	}
	emailSvc.SetNowFunc(func() time.Time { return now })
	if err := emailSvc.SeedFixtures(context.Background(), "user-1", []EmailMessage{
		{
			ID:          "mail-urgent",
			ThreadID:    "thread-urgent",
			Subject:     "ACTION REQUIRED: approve launch plan",
			SenderName:  "Leadership",
			SenderEmail: "leadership@example.com",
			Snippet:     "Please approve before noon.",
			Body:        "Action required before noon.",
			Labels:      []string{"work"},
			Priority:    "high",
			Unread:      true,
			ReceivedAt:  now.Add(-20 * time.Minute),
			UpdatedAt:   now.Add(-20 * time.Minute),
		},
	}); err != nil {
		t.Fatalf("seed email fixtures: %v", err)
	}

	calendarSvc, err := NewLocalCalendarService(db)
	if err != nil {
		t.Fatalf("new calendar service: %v", err)
	}
	calendarSvc.SetNowFunc(func() time.Time { return now })
	if err := calendarSvc.SeedFixtures(context.Background(), "user-1", []CalendarEvent{
		{
			ID:           "event-1",
			Title:        "Project sync",
			CalendarName: "Work",
			Status:       "confirmed",
			StartAt:      now.Add(2 * time.Hour),
			EndAt:        now.Add(3 * time.Hour),
			CreatedAt:    now.Add(-24 * time.Hour),
			UpdatedAt:    now.Add(-1 * time.Hour),
		},
	}); err != nil {
		t.Fatalf("seed calendar fixtures: %v", err)
	}

	tool := NewCalendarTool(calendarSvc)
	tool.SetNowFunc(func() time.Time { return now })
	tool.SetEmailService(emailSvc)
	tool.SetReminderService(&mockReminderService{
		list: []PushResult{
			{
				ID:      "reminder-1",
				Message: "Call the vendor",
				FireAt:  now.Add(4 * time.Hour),
				Status:  "scheduled",
			},
		},
	})
	tool.SetCronService(&mockCronService{
		jobs: []CronJobInfo{
			{
				ID:        "job-1",
				Name:      "Daily export",
				Schedule:  "0 18 * * *",
				Handler:   "export.daily",
				Enabled:   true,
				NextRunAt: timePtr(now.Add(9 * time.Hour)),
				Payload: map[string]interface{}{
					"user_id": "user-1",
				},
			},
		},
	})

	ctx := WithLang(WithUserID(context.Background(), "user-1"), "en-US")
	resultAny, err := tool.Execute(ctx, map[string]interface{}{
		"action": "daily_summary",
		"date":   "2026-03-20T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("daily_summary: %v", err)
	}
	result := resultAny.(map[string]interface{})
	if got := result["focus_item_count"].(int); got != 4 {
		t.Fatalf("focus_item_count = %d, want 4", got)
	}
	if got := len(result["events"].([]map[string]interface{})); got != 1 {
		t.Fatalf("events len = %d, want 1", got)
	}
	if got := len(result["reminders"].([]map[string]interface{})); got != 1 {
		t.Fatalf("reminders len = %d, want 1", got)
	}
	if got := len(result["scheduled_jobs"].([]map[string]interface{})); got != 1 {
		t.Fatalf("scheduled_jobs len = %d, want 1", got)
	}
	if got := len(result["priority_emails"].([]map[string]interface{})); got != 1 {
		t.Fatalf("priority_emails len = %d, want 1", got)
	}
	if summary := result["summary"].(string); summary == "" {
		t.Fatal("expected human-readable daily summary")
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func TestCalendarTool_Execute_CreateSupportsNaturalLanguageTime(t *testing.T) {
	now := time.Date(2026, time.March, 20, 9, 0, 0, 0, time.UTC)
	db := openProductivityTestDB(t)

	calendarSvc, err := NewLocalCalendarService(db)
	if err != nil {
		t.Fatalf("new calendar service: %v", err)
	}
	calendarSvc.SetNowFunc(func() time.Time { return now })

	tool := NewCalendarTool(calendarSvc)
	tool.SetNowFunc(func() time.Time { return now })
	ctx := WithLang(WithUserID(context.Background(), "user-1"), "en-US")

	resultAny, err := tool.Execute(ctx, map[string]interface{}{
		"action":   "create",
		"title":    "Natural language planning review",
		"time":     "tomorrow 3pm",
		"end":      "4pm",
		"location": "Horizon Room",
	})
	if err != nil {
		t.Fatalf("create natural language event: %v", err)
	}
	result := resultAny.(map[string]interface{})
	event := result["event"].(map[string]interface{})
	if got := event["start_at"].(string); got != "2026-03-21T15:00:00Z" {
		t.Fatalf("start_at = %q, want %q", got, "2026-03-21T15:00:00Z")
	}
	if got := event["end_at"].(string); got != "2026-03-21T16:00:00Z" {
		t.Fatalf("end_at = %q, want %q", got, "2026-03-21T16:00:00Z")
	}
}
