package mediagen

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPollInterval = 3 * time.Second
	defaultPollTimeout  = 5 * time.Minute
)

// Manager orchestrates media generation across providers.
type Manager struct {
	providers map[string]MediaProvider // name -> provider
	modelMap  map[string]string       // modelID -> provider name
	storage   *MediaStorage
	tasks     sync.Map // taskID -> *MediaTask
	mu        sync.RWMutex
}

// NewManager creates a new media generation manager.
func NewManager(storage *MediaStorage) *Manager {
	return &Manager{
		providers: make(map[string]MediaProvider),
		modelMap:  make(map[string]string),
		storage:   storage,
	}
}

// RegisterProvider adds a media provider and indexes its models.
func (m *Manager) RegisterProvider(p MediaProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[p.Name()] = p
	for _, model := range p.SupportedModels() {
		m.modelMap[model.ID] = p.Name()
	}
}

// Models returns all available media models across providers.
func (m *Manager) Models() []MediaModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var models []MediaModelInfo
	for _, p := range m.providers {
		models = append(models, p.SupportedModels()...)
	}
	return models
}

// Generate starts a media generation task.
func (m *Manager) Generate(ctx context.Context, req *MediaRequest) (*MediaTask, error) {
	provider, err := m.findProvider(req.Model)
	if err != nil {
		return nil, err
	}

	if !provider.SupportsType(req.Type) {
		return nil, fmt.Errorf("%w: provider %s does not support %s", ErrUnsupportedType, provider.Name(), req.Type)
	}

	task, err := provider.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	// Assign a local task ID if the provider didn't set one
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Provider = provider.Name()
	task.Model = req.Model
	task.Type = req.Type
	task.CreatedAt = time.Now()

	m.tasks.Store(task.ID, task)

	// If async (pending/processing), start background polling
	if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing {
		go m.pollTask(task.ID, provider)
	} else if task.Status == TaskStatusSucceeded {
		// Sync provider returned immediately — cache media
		go m.cacheResults(task)
	}

	return task, nil
}

// GetTask returns the current state of a task.
func (m *Manager) GetTask(taskID string) (*MediaTask, error) {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	return v.(*MediaTask), nil
}

// WaitForTask blocks until the task completes or context is cancelled.
func (m *Manager) WaitForTask(ctx context.Context, taskID string) (*MediaTask, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ErrTimeout
		case <-ticker.C:
			task, err := m.GetTask(taskID)
			if err != nil {
				return nil, err
			}
			switch task.Status {
			case TaskStatusSucceeded:
				return task, nil
			case TaskStatusFailed:
				return task, fmt.Errorf("%w: %s", ErrGenerationFailed, task.Error)
			}
		}
	}
}

// findProvider looks up the provider for a given model ID.
func (m *Manager) findProvider(modelID string) (MediaProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modelID == "" {
		// Pick first available provider
		for _, p := range m.providers {
			return p, nil
		}
		return nil, ErrProviderNotFound
	}

	providerName, ok := m.modelMap[modelID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, modelID)
	}
	p, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("%w: provider %s not registered", ErrProviderNotFound, providerName)
	}
	return p, nil
}

// pollTask polls an async provider until the task completes.
func (m *Manager) pollTask(taskID string, provider MediaProvider) {
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	timeout := time.After(defaultPollTimeout)

	for {
		select {
		case <-timeout:
			m.updateTaskError(taskID, "generation timed out")
			return
		case <-ticker.C:
			v, ok := m.tasks.Load(taskID)
			if !ok {
				return
			}
			task := v.(*MediaTask)

			updated, err := provider.Poll(context.Background(), task.UpstreamID)
			if err != nil {
				log.Printf("[mediagen] poll error for task %s: %v", taskID, err)
				continue
			}

			// Preserve local fields
			updated.ID = taskID
			updated.Provider = task.Provider
			updated.Model = task.Model
			updated.Type = task.Type
			updated.CreatedAt = task.CreatedAt
			updated.Request = task.Request

			m.tasks.Store(taskID, updated)

			switch updated.Status {
			case TaskStatusSucceeded:
				m.cacheResults(updated)
				return
			case TaskStatusFailed:
				return
			}
		}
	}
}

// cacheResults downloads remote media to local storage.
func (m *Manager) cacheResults(task *MediaTask) {
	if task.Response == nil || m.storage == nil {
		return
	}

	for i := range task.Response.Data {
		result := &task.Response.Data[i]

		// Download from remote URL
		if result.OriginalURL != "" && result.URL == "" {
			localURL, err := m.storage.Download(context.Background(), result.OriginalURL, task.Type)
			if err != nil {
				log.Printf("[mediagen] cache download failed for task %s: %v", task.ID, err)
				continue
			}
			result.URL = localURL
		}

		// Store base64 data
		if result.B64JSON != "" && result.URL == "" {
			ct := result.ContentType
			if ct == "" {
				ct = "image/png"
			}
			localURL, err := m.storage.StoreBase64(result.B64JSON, ct, task.Type)
			if err != nil {
				log.Printf("[mediagen] cache store failed for task %s: %v", task.ID, err)
				continue
			}
			result.URL = localURL
			result.B64JSON = "" // Free memory
		}
	}

	now := time.Now()
	task.CompletedAt = &now
	m.tasks.Store(task.ID, task)
}

// updateTaskError marks a task as failed.
func (m *Manager) updateTaskError(taskID, errMsg string) {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return
	}
	task := v.(*MediaTask)
	task.Status = TaskStatusFailed
	task.Error = errMsg
	now := time.Now()
	task.CompletedAt = &now
	m.tasks.Store(taskID, task)
}
