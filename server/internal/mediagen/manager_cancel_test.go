package mediagen

import (
	"context"
	"errors"
	"testing"
	"time"

	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type blockingTestProvider struct {
	generateStarted chan struct{}
	generateRelease chan struct{}
	generateResult  *MediaTask
	generateErr     error

	pollStarted chan struct{}
	pollRelease chan struct{}
	pollResult  *MediaTask
	pollErr     error
}

func (p *blockingTestProvider) Name() string { return "fake" }

func (p *blockingTestProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{{ID: "fake-model", Name: "Fake", Type: MediaTypeImage, Provider: "fake"}}
}

func (p *blockingTestProvider) SupportsType(MediaType) bool { return true }

func (p *blockingTestProvider) Generate(ctx context.Context, _ *MediaRequest) (*MediaTask, error) {
	if p.generateStarted != nil {
		close(p.generateStarted)
	}
	if p.generateRelease != nil {
		select {
		case <-p.generateRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.generateErr != nil {
		return nil, p.generateErr
	}
	if p.generateResult != nil {
		return p.generateResult, nil
	}
	return &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing}}, nil
}

func (p *blockingTestProvider) Poll(ctx context.Context, _ string) (*MediaTask, error) {
	if p.pollStarted != nil {
		close(p.pollStarted)
	}
	if p.pollRelease != nil {
		select {
		case <-p.pollRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.pollErr != nil {
		return nil, p.pollErr
	}
	if p.pollResult != nil {
		return p.pollResult, nil
	}
	return &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing}}, nil
}

func TestManager_WaitForTask_ReturnsCancelled(t *testing.T) {
	m := NewManager(nil, nil, "")
	task := &MediaTask{BaseTask: basetask.BaseTask{ID: "wait-cancel", Status: TaskStatusProcessing, CreatedAt: time.Now()}}
	m.tasks.Store(task.ID, task)

	go func() {
		time.Sleep(20 * time.Millisecond)
		m.CancelTask(task.ID)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := m.WaitForTask(ctx, task.ID)
	if got == nil {
		t.Fatal("expected task")
	}
	if got.Status != TaskStatusCancelled {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusCancelled)
	}
	if !errors.Is(err, ErrGenerationCancelled) {
		t.Fatalf("err=%v, want ErrGenerationCancelled", err)
	}
}

func TestManager_CancelTask_PreventsExecuteOverwrite(t *testing.T) {
	provider := &blockingTestProvider{
		generateStarted: make(chan struct{}),
		generateRelease: make(chan struct{}),
		generateResult:  &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing}, UpstreamID: "up-1"},
	}
	m := NewManager(nil, nil, "")
	task := &MediaTask{
		BaseTask: basetask.BaseTask{ID: "execute-cancel", Status: TaskStatusPending, CreatedAt: time.Now()},
		Type:     MediaTypeImage,
		Model:    "fake-model",
		Provider: "fake",
		Request:  &MediaRequest{Type: MediaTypeImage, Model: "fake-model", Prompt: "test"},
	}
	m.tasks.Store(task.ID, task)

	go m.executeTask(task, provider)

	select {
	case <-provider.generateStarted:
	case <-time.After(time.Second):
		t.Fatal("generate did not start")
	}

	if !m.CancelTask(task.ID) {
		t.Fatal("expected cancel to succeed")
	}
	close(provider.generateRelease)
	time.Sleep(100 * time.Millisecond)

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusCancelled {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusCancelled)
	}
}

func TestManager_CancelTask_PreventsPollOverwrite(t *testing.T) {
	provider := &blockingTestProvider{
		pollStarted: make(chan struct{}),
		pollRelease: make(chan struct{}),
		pollResult:  &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusFailed, Error: "provider failed"}},
	}
	m := NewManager(nil, nil, "")
	task := &MediaTask{
		BaseTask:   basetask.BaseTask{ID: "poll-cancel", Status: TaskStatusProcessing, CreatedAt: time.Now()},
		Type:       MediaTypeImage,
		Model:      "fake-model",
		Provider:   "fake",
		UpstreamID: "up-2",
		Request:    &MediaRequest{Type: MediaTypeImage, Model: "fake-model", Prompt: "test"},
	}
	m.tasks.Store(task.ID, task)

	go m.pollTask(task.ID, provider)

	select {
	case <-provider.pollStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("poll did not start")
	}

	if !m.CancelTask(task.ID) {
		t.Fatal("expected cancel to succeed")
	}
	close(provider.pollRelease)
	time.Sleep(100 * time.Millisecond)

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusCancelled {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusCancelled)
	}
}

func TestManager_WaitForTask_UsesContextUserScope(t *testing.T) {
	m := NewManager(nil, nil, "")
	task := &MediaTask{
		BaseTask: basetask.BaseTask{ID: "scoped-task", Status: TaskStatusSucceeded, CreatedAt: time.Now()},
		UserID:   "user-a",
		Type:     MediaTypeImage,
	}
	m.tasks.Store(task.ID, task)

	ctx, cancel := context.WithDeadline(tools.WithUserID(context.Background(), "user-b"), time.Now().Add(-time.Second))
	defer cancel()

	got, err := m.WaitForTask(ctx, task.ID)
	if got != nil {
		t.Fatalf("got task = %#v, want nil for cross-user lookup", got)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}
