package cron

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

type captureInjector struct {
	ownerID string
	session string
	content string
	called  chan struct{}
}

func (c *captureInjector) InjectMessage(_ context.Context, ownerID, sessionID, content string) (string, error) {
	c.ownerID = ownerID
	c.session = sessionID
	c.content = content
	select {
	case c.called <- struct{}{}:
	default:
	}
	return sessionID, nil
}

type capturePublisher struct {
	userID    string
	eventType string
	data      any
	called    chan struct{}
}

func (c *capturePublisher) Publish(userID string, eventType string, data any) {
	c.userID = userID
	c.eventType = eventType
	c.data = data
	select {
	case c.called <- struct{}{}:
	default:
	}
}

func TestNewService(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	s := NewService(cfg, logger)
	if s == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCreate(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	// Register handler
	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	job, err := s.Create("test-job", "Test job", "0 * * * * *", "test-handler", nil)
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	if job.ID == "" {
		t.Error("expected non-empty ID")
	}

	if job.Name != "test-job" {
		t.Errorf("expected name 'test-job', got '%s'", job.Name)
	}

	if job.Status != StatusActive {
		t.Errorf("expected status %s, got %s", StatusActive, job.Status)
	}

	if !job.Enabled {
		t.Error("expected job to be enabled")
	}
}

func TestCreateInvalidSchedule(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	_, err := s.Create("test-job", "", "invalid", "test-handler", nil)
	if err == nil {
		t.Error("expected error for invalid schedule")
	}
}

func TestCreateHandlerNotFound(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	_, err := s.Create("test-job", "", "0 * * * * *", "non-existent", nil)
	if err == nil {
		t.Error("expected error for non-existent handler")
	}
}

func TestCreateHTTPRequiresURL(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)
	s.RegisterBuiltinHandlers()

	_, err := s.Create("http-job", "", "0 * * * * *", "http", nil)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if err.Error() != "url not specified in payload" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCommandRequiresCommandPayload(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)
	s.RegisterCommandHandler(CommandSecurityConfig{Enabled: true})

	_, err := s.Create("command-job", "", "0 * * * * *", "command", nil)
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if err.Error() != "command not specified in payload" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCommandRejectsDisallowedCommandEarly(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)
	s.RegisterCommandHandler(CommandSecurityConfig{Enabled: true})

	_, err := s.Create("command-job", "", "0 * * * * *", "command", map[string]interface{}{"command": "docker restart myapp"})
	if err == nil {
		t.Fatal("expected error for disallowed command")
	}
	if !strings.Contains(err.Error(), "not in the allowed commands list") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGet(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	job, _ := s.Create("test", "", "0 * * * * *", "test-handler", nil)

	// Get existing
	found, exists := s.Get(job.ID)
	if !exists {
		t.Fatal("job should exist")
	}

	if found.ID != job.ID {
		t.Errorf("expected ID %s, got %s", job.ID, found.ID)
	}

	// Get non-existing
	_, exists = s.Get("non-existent")
	if exists {
		t.Error("job should not exist")
	}
}

func TestList(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	s.Create("job1", "", "0 * * * * *", "test-handler", nil)
	s.Create("job2", "", "0 * * * * *", "test-handler", nil)
	s.Create("job3", "", "0 * * * * *", "test-handler", nil)

	jobs := s.List()
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestTriggerInjectsExecutionResultIntoConversation(t *testing.T) {
	logger := zap.NewNop()
	s := NewService(DefaultConfig(), logger)
	injector := &captureInjector{called: make(chan struct{}, 1)}
	publisher := &capturePublisher{called: make(chan struct{}, 1)}
	s.SetMessageInjector(injector)
	s.SetEventPublisher(publisher)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		_ = ctx
		return map[string]interface{}{"ok": true, "query": job.Payload["query"]}, nil
	})

	job, err := s.Create("notify-me", "", "0 * * * * *", "test-handler", map[string]interface{}{
		"conversation_id": "conv-123",
		"user_id":         "user-123",
		"query":           "daily report",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	if err := s.Trigger(job.ID); err != nil {
		t.Fatalf("failed to trigger job: %v", err)
	}

	select {
	case <-injector.called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for injected message")
	}

	if injector.ownerID != "user-123" {
		t.Fatalf("ownerID = %q, want user-123", injector.ownerID)
	}
	if injector.session != "conv-123" {
		t.Fatalf("session = %q, want conv-123", injector.session)
	}
	if !strings.Contains(injector.content, "\"type\":\"result\"") {
		t.Fatalf("expected result card content, got %q", injector.content)
	}
	if !strings.Contains(injector.content, "daily report") {
		t.Fatalf("expected injected result to include handler result, got %q", injector.content)
	}

	select {
	case <-publisher.called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for conversation_updated event")
	}
	if publisher.userID != "user-123" {
		t.Fatalf("publisher userID = %q, want user-123", publisher.userID)
	}
	if publisher.eventType != "conversation_updated" {
		t.Fatalf("publisher eventType = %q, want conversation_updated", publisher.eventType)
	}
}

func TestUpdate(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	job, _ := s.Create("original", "original desc", "0 * * * * *", "test-handler", nil)

	err := s.Update(job.ID, "updated", "updated desc", "30 * * * * *", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	updated, _ := s.Get(job.ID)
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got '%s'", updated.Name)
	}

	if updated.Schedule != "30 * * * * *" {
		t.Errorf("expected schedule '30 * * * * *', got '%s'", updated.Schedule)
	}
}

func TestUpdateHTTPRequiresURL(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)
	s.RegisterBuiltinHandlers()

	job, err := s.Create("http-job", "", "0 * * * * *", "http", map[string]interface{}{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("failed to create http job: %v", err)
	}

	err = s.Update(job.ID, job.Name, job.Description, job.Schedule, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when clearing http url")
	}
	if err.Error() != "url not specified in payload" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCommandRejectsDisallowedCommandEarly(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)
	s.RegisterCommandHandler(CommandSecurityConfig{Enabled: true})

	job, err := s.Create("command-job", "", "0 * * * * *", "command", map[string]interface{}{"command": "echo ok"})
	if err != nil {
		t.Fatalf("failed to create command job: %v", err)
	}

	err = s.Update(job.ID, job.Name, job.Description, job.Schedule, map[string]interface{}{"command": "docker restart myapp"})
	if err == nil {
		t.Fatal("expected error for disallowed command")
	}
	if !strings.Contains(err.Error(), "not in the allowed commands list") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	job, _ := s.Create("to-delete", "", "0 * * * * *", "test-handler", nil)

	err := s.Delete(job.ID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, exists := s.Get(job.ID)
	if exists {
		t.Error("job should have been deleted")
	}

	// Delete non-existent
	err = s.Delete("non-existent")
	if err == nil {
		t.Error("expected error when deleting non-existent job")
	}
}

func TestEnableDisable(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	job, _ := s.Create("test", "", "0 * * * * *", "test-handler", nil)

	// Disable
	err := s.Disable(job.ID)
	if err != nil {
		t.Fatalf("failed to disable: %v", err)
	}

	disabled, _ := s.Get(job.ID)
	if disabled.Enabled {
		t.Error("expected job to be disabled")
	}

	if disabled.Status != StatusPaused {
		t.Errorf("expected status %s, got %s", StatusPaused, disabled.Status)
	}

	// Enable
	err = s.Enable(job.ID)
	if err != nil {
		t.Fatalf("failed to enable: %v", err)
	}

	enabled, _ := s.Get(job.ID)
	if !enabled.Enabled {
		t.Error("expected job to be enabled")
	}

	if enabled.Status != StatusActive {
		t.Errorf("expected status %s, got %s", StatusActive, enabled.Status)
	}
}

func TestTrigger(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.JobTimeoutSeconds = 5
	s := NewService(cfg, logger)

	var executed atomic.Bool
	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		executed.Store(true)
		return map[string]string{"result": "ok"}, nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	job, _ := s.Create("test", "", "0 0 0 1 1 *", "test-handler", nil) // Far future schedule

	// Trigger manually
	err := s.Trigger(job.ID)
	if err != nil {
		t.Fatalf("failed to trigger: %v", err)
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	if !executed.Load() {
		t.Error("job was not executed")
	}

	// Check execution record
	executions, _ := s.GetExecutions(job.ID, 1)
	if len(executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(executions))
	}

	if executions[0].Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", executions[0].Status)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)
}

func TestJobExecution(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.JobTimeoutSeconds = 5
	s := NewService(cfg, logger)

	var runCount atomic.Int32
	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		runCount.Add(1)
		return nil, nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Create job that runs every second
	job, _ := s.Create("test", "", "* * * * * *", "test-handler", nil)

	// Wait for at least 2 executions
	time.Sleep(2500 * time.Millisecond)

	if runCount.Load() < 2 {
		t.Errorf("expected at least 2 runs, got %d", runCount.Load())
	}

	// Check job stats
	updated, _ := s.Get(job.ID)
	if updated.RunCount < 2 {
		t.Errorf("expected run count >= 2, got %d", updated.RunCount)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)
}

func TestJobExecutionFailure(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.JobTimeoutSeconds = 5
	s := NewService(cfg, logger)

	s.RegisterHandler("failing-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, errors.New("intentional failure")
	})

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	job, _ := s.Create("test", "", "0 0 0 1 1 *", "failing-handler", nil)

	// Trigger manually
	s.Trigger(job.ID)

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	// Check execution record
	executions, _ := s.GetExecutions(job.ID, 1)
	if len(executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(executions))
	}

	if executions[0].Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", executions[0].Status)
	}

	if executions[0].Error == "" {
		t.Error("expected error message")
	}

	// Check fail count
	updated, _ := s.Get(job.ID)
	if updated.FailCount != 1 {
		t.Errorf("expected fail count 1, got %d", updated.FailCount)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)
}

func TestGetExecutions(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.JobTimeoutSeconds = 5
	s := NewService(cfg, logger)

	s.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	job, _ := s.Create("test", "", "0 0 0 1 1 *", "test-handler", nil)

	// Trigger multiple times
	for i := 0; i < 5; i++ {
		s.Trigger(job.ID)
		time.Sleep(50 * time.Millisecond)
	}

	// Get limited executions
	executions, err := s.GetExecutions(job.ID, 3)
	if err != nil {
		t.Fatalf("failed to get executions: %v", err)
	}

	if len(executions) != 3 {
		t.Errorf("expected 3 executions, got %d", len(executions))
	}

	// Get all executions
	allExecutions, _ := s.GetExecutions(job.ID, 0)
	if len(allExecutions) != 5 {
		t.Errorf("expected 5 executions, got %d", len(allExecutions))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)
}

func TestConcurrentJobs(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxConcurrentJobs = 2
	cfg.JobTimeoutSeconds = 5
	s := NewService(cfg, logger)

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	s.RegisterHandler("slow-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		current := concurrent.Add(1)
		defer concurrent.Add(-1)

		// Track max concurrent
		for {
			max := maxConcurrent.Load()
			if current <= max || maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}

		time.Sleep(100 * time.Millisecond)
		return nil, nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Create multiple jobs
	for i := 0; i < 5; i++ {
		s.Create("job-"+string(rune('0'+i)), "", "0 0 0 1 1 *", "slow-handler", nil)
	}

	// Trigger all jobs
	for _, job := range s.List() {
		s.Trigger(job.ID)
	}

	// Wait for all to complete
	time.Sleep(500 * time.Millisecond)

	if maxConcurrent.Load() > int32(cfg.MaxConcurrentJobs) {
		t.Errorf("max concurrent %d exceeded limit %d", maxConcurrent.Load(), cfg.MaxConcurrentJobs)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Stop(ctx)
}
