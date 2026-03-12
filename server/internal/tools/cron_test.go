package tools

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type stubCronService struct {
	config            CronConfigInfo
	handlers          []string
	jobs              map[string]*CronJobInfo
	executions        map[string][]CronExecutionInfo
	lastCreateName    string
	lastCreateSched   string
	lastCreateHandle  string
	lastUpdateID      string
	lastUpdateName    string
	lastUpdateSched   string
	lastUpdateDesc    string
	lastUpdatePayload map[string]interface{}
	lastTriggerID     string
}

func (s *stubCronService) Config(ctx context.Context) CronConfigInfo {
	_ = ctx
	return s.config
}

func (s *stubCronService) Handlers(ctx context.Context) []string {
	_ = ctx
	return append([]string(nil), s.handlers...)
}

func (s *stubCronService) ListJobs(ctx context.Context) ([]CronJobInfo, error) {
	_ = ctx
	result := make([]CronJobInfo, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job == nil {
			continue
		}
		result = append(result, *job)
	}
	return result, nil
}

func (s *stubCronService) GetJob(ctx context.Context, id string) (*CronJobInfo, error) {
	_ = ctx
	job, ok := s.jobs[id]
	if !ok {
		return nil, nil
	}
	copy := *job
	return &copy, nil
}

func (s *stubCronService) CreateJob(ctx context.Context, name, description, schedule, handler string, payload map[string]interface{}) (*CronJobInfo, error) {
	_ = ctx
	s.lastCreateName = name
	s.lastCreateSched = schedule
	s.lastCreateHandle = handler
	if s.jobs == nil {
		s.jobs = map[string]*CronJobInfo{}
	}
	job := &CronJobInfo{
		ID:          fmt.Sprintf("job_%d", len(s.jobs)+1),
		Name:        name,
		Description: description,
		Schedule:    schedule,
		Handler:     handler,
		Payload:     payload,
		Enabled:     true,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	s.jobs[job.ID] = job
	return job, nil
}

func (s *stubCronService) UpdateJob(ctx context.Context, id, name, description, schedule string, payload map[string]interface{}) error {
	_ = ctx
	s.lastUpdateID = id
	s.lastUpdateName = name
	s.lastUpdateDesc = description
	s.lastUpdateSched = schedule
	s.lastUpdatePayload = payload
	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job not found")
	}
	job.Name = name
	job.Description = description
	job.Schedule = schedule
	job.Payload = payload
	job.UpdatedAt = time.Now()
	return nil
}

func (s *stubCronService) DeleteJob(ctx context.Context, id string) error {
	_ = ctx
	delete(s.jobs, id)
	return nil
}

func (s *stubCronService) EnableJob(ctx context.Context, id string) error {
	_ = ctx
	if job, ok := s.jobs[id]; ok {
		job.Enabled = true
		job.Status = "active"
		return nil
	}
	return fmt.Errorf("job not found")
}

func (s *stubCronService) DisableJob(ctx context.Context, id string) error {
	_ = ctx
	if job, ok := s.jobs[id]; ok {
		job.Enabled = false
		job.Status = "paused"
		return nil
	}
	return fmt.Errorf("job not found")
}

func (s *stubCronService) TriggerJob(ctx context.Context, id string) error {
	_ = ctx
	s.lastTriggerID = id
	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("job not found")
	}
	return nil
}

func (s *stubCronService) GetExecutions(ctx context.Context, id string, limit int) ([]CronExecutionInfo, error) {
	_ = ctx
	items := append([]CronExecutionInfo(nil), s.executions[id]...)
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

func TestCronToolListExecute(t *testing.T) {
	now := time.Now()
	svc := &stubCronService{
		config: CronConfigInfo{Enabled: true, MaxConcurrentJobs: 5},
		jobs: map[string]*CronJobInfo{
			"job_1": {ID: "job_1", Name: "Older", CreatedAt: now.Add(-time.Hour)},
			"job_2": {ID: "job_2", Name: "Newer", CreatedAt: now},
		},
	}
	tool := NewCronTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "list"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	jobs := payload["jobs"].([]CronJobInfo)
	if len(jobs) != 2 || jobs[0].ID != "job_2" {
		t.Fatalf("unexpected jobs: %#v", jobs)
	}
}

func TestCronToolCreateExecute(t *testing.T) {
	svc := &stubCronService{handlers: []string{"http"}}
	tool := NewCronTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"name":     "Health Check",
		"schedule": "0 * * * *",
		"handler":  "http",
		"payload":  map[string]interface{}{"url": "https://example.com"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if created, _ := payload["created"].(bool); !created {
		t.Fatalf("expected created=true, got %#v", payload)
	}
	if svc.lastCreateName != "Health Check" || svc.lastCreateHandle != "http" {
		t.Fatalf("unexpected create args: %#v", svc)
	}
}

func TestCronToolUpdateExecuteUsesExistingDefaults(t *testing.T) {
	svc := &stubCronService{
		jobs: map[string]*CronJobInfo{
			"job_1": {ID: "job_1", Name: "Health Check", Description: "old", Schedule: "0 * * * *", Payload: map[string]interface{}{"url": "https://example.com"}},
		},
	}
	tool := NewCronTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "update",
		"id":          "job_1",
		"description": "new",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if updated, _ := payload["updated"].(bool); !updated {
		t.Fatalf("expected updated=true, got %#v", payload)
	}
	if svc.lastUpdateName != "Health Check" || svc.lastUpdateSched != "0 * * * *" {
		t.Fatalf("unexpected update defaults: name=%q schedule=%q", svc.lastUpdateName, svc.lastUpdateSched)
	}
}

func TestCronToolSupportsNestedCamelCaseArgs(t *testing.T) {
	svc := &stubCronService{
		jobs: map[string]*CronJobInfo{
			"job_1": {ID: "job_1", Name: "Health Check", Description: "old", Schedule: "0 * * * *", Payload: map[string]interface{}{"url": "https://example.com"}},
		},
	}
	tool := NewCronTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":      "update",
			"jobId":       "job_1",
			"description": "new",
			"payload":     map[string]interface{}{"url": "https://example.com/new"},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if updated, _ := payload["updated"].(bool); !updated {
		t.Fatalf("expected updated=true, got %#v", payload)
	}
	if svc.lastUpdateID != "job_1" {
		t.Fatalf("lastUpdateID = %q, want job_1", svc.lastUpdateID)
	}
	if got := svc.lastUpdatePayload["url"]; got != "https://example.com/new" {
		t.Fatalf("payload url = %v, want https://example.com/new", got)
	}
}

func TestCronToolExecutionsAndHandlersExecute(t *testing.T) {
	svc := &stubCronService{
		config:   CronConfigInfo{Enabled: true},
		handlers: []string{"command", "http"},
		executions: map[string][]CronExecutionInfo{
			"job_1": {{ID: "exec_1", JobID: "job_1"}, {ID: "exec_2", JobID: "job_1"}},
		},
	}
	tool := NewCronTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "executions", "id": "job_1", "limit": 1})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	executions := payload["executions"].([]CronExecutionInfo)
	if len(executions) != 1 || executions[0].ID != "exec_1" {
		t.Fatalf("unexpected executions: %#v", executions)
	}

	handlersResult, err := tool.Execute(context.Background(), map[string]interface{}{"action": "handlers"})
	if err != nil {
		t.Fatalf("Execute handlers returned error: %v", err)
	}
	handlersPayload := handlersResult.(map[string]interface{})
	handlers := handlersPayload["handlers"].([]string)
	if len(handlers) != 2 || handlers[0] != "command" {
		t.Fatalf("unexpected handlers: %#v", handlers)
	}
}
