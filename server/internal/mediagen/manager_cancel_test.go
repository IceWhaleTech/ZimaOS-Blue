package mediagen

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
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

type sequentialPollProvider struct {
	mu        sync.Mutex
	polledIDs []string
	results   []*MediaTask
}

func (p *sequentialPollProvider) Name() string { return "fake" }

func (p *sequentialPollProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{{ID: "fake-model", Name: "Fake", Type: MediaTypeVideo, Provider: "fake"}}
}

func (p *sequentialPollProvider) SupportsType(MediaType) bool { return true }

func (p *sequentialPollProvider) Generate(context.Context, *MediaRequest) (*MediaTask, error) {
	return &MediaTask{
		BaseTask:   basetask.BaseTask{Status: TaskStatusProcessing},
		UpstreamID: "up-keep",
	}, nil
}

func (p *sequentialPollProvider) Poll(_ context.Context, taskID string) (*MediaTask, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.polledIDs = append(p.polledIDs, taskID)
	if len(p.results) == 0 {
		return &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing}}, nil
	}
	result := p.results[0]
	p.results = p.results[1:]
	if result == nil {
		return &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing}}, nil
	}
	cp := *result
	return &cp, nil
}

func (p *sequentialPollProvider) PolledIDs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.polledIDs...)
}

type stubbornGenerateProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p *stubbornGenerateProvider) Name() string { return "fake" }

func (p *stubbornGenerateProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{{ID: "fake-model", Name: "Fake", Type: MediaTypeImage, Provider: "fake"}}
}

func (p *stubbornGenerateProvider) SupportsType(MediaType) bool { return true }

func (p *stubbornGenerateProvider) Generate(context.Context, *MediaRequest) (*MediaTask, error) {
	if p.started != nil {
		close(p.started)
	}
	if p.release != nil {
		<-p.release
	}
	return &MediaTask{
		BaseTask: basetask.BaseTask{Status: TaskStatusSucceeded},
		Response: &MediaResponse{
			Data: []MediaResult{{ContentType: "image/png"}},
		},
	}, nil
}

func (p *stubbornGenerateProvider) Poll(context.Context, string) (*MediaTask, error) {
	return nil, errors.New("unexpected poll")
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

func TestManager_ExecuteTaskMarksProcessingBeforeGenerateReturns(t *testing.T) {
	provider := &blockingTestProvider{
		generateStarted: make(chan struct{}),
		generateRelease: make(chan struct{}),
		generateResult:  &MediaTask{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing}, UpstreamID: "up-1"},
	}
	m := NewManager(nil, nil, "")
	task := &MediaTask{
		BaseTask: basetask.BaseTask{ID: "execute-processing", Status: TaskStatusPending, CreatedAt: time.Now()},
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

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusProcessing {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusProcessing)
	}
	if got.Progress < 0.05 {
		t.Fatalf("progress=%v, want at least 0.05", got.Progress)
	}

	close(provider.generateRelease)
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

func TestManager_CloseStopsExecuteTaskWithoutMarkingFailure(t *testing.T) {
	provider := &blockingTestProvider{
		generateStarted: make(chan struct{}),
		generateRelease: make(chan struct{}),
	}
	m := NewManager(nil, nil, "")
	task := &MediaTask{
		BaseTask: basetask.BaseTask{ID: "execute-shutdown", Status: TaskStatusPending, CreatedAt: time.Now()},
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

	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusProcessing {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusProcessing)
	}
	if got.Error != "" {
		t.Fatalf("error=%q, want empty", got.Error)
	}
}

func TestManager_CloseStopsPollTaskWithoutMarkingFailure(t *testing.T) {
	provider := &blockingTestProvider{
		pollStarted: make(chan struct{}),
		pollRelease: make(chan struct{}),
	}
	m := NewManager(nil, nil, "")
	task := &MediaTask{
		BaseTask:   basetask.BaseTask{ID: "poll-shutdown", Status: TaskStatusProcessing, CreatedAt: time.Now()},
		Type:       MediaTypeImage,
		Model:      "fake-model",
		Provider:   "fake",
		UpstreamID: "up-shutdown",
		Request:    &MediaRequest{Type: MediaTypeImage, Model: "fake-model", Prompt: "test"},
	}
	m.tasks.Store(task.ID, task)

	go m.pollTask(task.ID, provider)

	select {
	case <-provider.pollStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("poll did not start")
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskStatusProcessing {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusProcessing)
	}
	if got.Error != "" {
		t.Fatalf("error=%q, want empty", got.Error)
	}
}

func TestManager_GenerateKeepsSyncTaskProcessingUntilCacheCompletes(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	pngBytes, err := base64.StdEncoding.DecodeString(fakeMediaImagePNGBase64)
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseResponse
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	storage := NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	provider := &blockingTestProvider{
		generateResult: &MediaTask{
			BaseTask: basetask.BaseTask{Status: TaskStatusSucceeded},
			Response: &MediaResponse{
				Data: []MediaResult{{
					OriginalURL: server.URL + "/image.png",
					ContentType: "image/png",
				}},
			},
		},
	}

	m := NewManager(storage, nil, "")
	m.RegisterProvider(provider)
	t.Cleanup(func() {
		_ = m.Close()
	})

	task, err := m.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Model:  "fake-model",
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if task == nil {
		t.Fatal("expected task")
	}

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("background cache download did not start")
	}

	got, err := m.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if got.Status != TaskStatusProcessing {
		t.Fatalf("status=%q, want %q until cache completes", got.Status, TaskStatusProcessing)
	}

	close(releaseResponse)

	waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err = m.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if got.Status != TaskStatusSucceeded {
		t.Fatalf("status=%q, want %q", got.Status, TaskStatusSucceeded)
	}
	if got.Response == nil || len(got.Response.Data) != 1 {
		t.Fatalf("response=%#v, want single result", got.Response)
	}
	if got.Response.Data[0].URL == "" {
		t.Fatal("expected cached local URL after sync cache completes")
	}
}

func TestManager_CloseWaitsForBackgroundTasks(t *testing.T) {
	provider := &stubbornGenerateProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	m := NewManager(nil, nil, "")
	m.RegisterProvider(provider)

	task, err := m.CreateTask(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Model:  "fake-model",
		Prompt: "test",
	}, "", "", "")
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if task == nil {
		t.Fatal("expected task")
	}

	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("background executeTask did not start")
	}

	closeReturned := make(chan struct{})
	go func() {
		_ = m.Close()
		close(closeReturned)
	}()

	select {
	case <-closeReturned:
		t.Fatal("Close returned before background task exited")
	case <-time.After(100 * time.Millisecond):
	}

	close(provider.release)

	select {
	case <-closeReturned:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after background task exited")
	}
}

func TestManager_CreateTaskReturnsErrWhenClosed(t *testing.T) {
	m := NewManager(nil, nil, "")
	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	task, err := m.CreateTask(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Model:  "fake-model",
		Prompt: "test",
	}, "", "", "")
	if !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("err=%v, want ErrManagerClosed", err)
	}
	if task != nil {
		t.Fatalf("task=%v, want nil", task)
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

func TestManager_PollTaskPreservesUpstreamIDAcrossNonTerminalPolls(t *testing.T) {
	provider := &sequentialPollProvider{
		results: []*MediaTask{
			{BaseTask: basetask.BaseTask{Status: TaskStatusProcessing, Progress: 0.4}},
			{
				BaseTask: basetask.BaseTask{Status: TaskStatusSucceeded, Progress: 1},
				Response: &MediaResponse{
					Data: []MediaResult{{
						B64JSON:     fakeMediaImagePNGBase64,
						ContentType: "image/png",
					}},
				},
			},
		},
	}

	storage := NewMediaStorage(t.TempDir(), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	m := NewManager(storage, nil, "")
	m.RegisterProvider(provider)
	task, err := m.Generate(context.Background(), &MediaRequest{
		Type:  MediaTypeImage,
		Model: "fake-model",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	task, err = m.WaitForTask(waitCtx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if task.Status != TaskStatusSucceeded {
		t.Fatalf("status=%q, want %q", task.Status, TaskStatusSucceeded)
	}

	got := provider.PolledIDs()
	want := []string{"up-keep", "up-keep"}
	if len(got) != len(want) {
		t.Fatalf("polled ids = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("polled ids = %#v, want %#v", got, want)
		}
	}
}
