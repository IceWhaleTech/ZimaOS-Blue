package mediagen

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPollTimeout = 5 * time.Minute
)

// Manager orchestrates media generation across providers.
type Manager struct {
	providers    map[string]MediaProvider // name -> provider
	providerOrder []string               // provider names sorted by priority (ascending)
	modelMap     map[string]string       // modelID -> provider name
	storage      *MediaStorage
	tasks        sync.Map // taskID -> *MediaTask
	mu           sync.RWMutex

	// Provider config management
	configs     map[string]*MediaProviderConfig
	configStore *ConfigStore
	locale      string // user locale for priority ordering
}

// NewManager creates a new media generation manager.
func NewManager(storage *MediaStorage, configStore *ConfigStore, locale string) *Manager {
	return &Manager{
		providers:   make(map[string]MediaProvider),
		modelMap:    make(map[string]string),
		storage:     storage,
		configs:     make(map[string]*MediaProviderConfig),
		configStore: configStore,
		locale:      locale,
	}
}

// InitConfigs merges builtin defaults with persisted configs and registers enabled providers.
// Providers are registered then ordered by priority (lower number = higher priority).
// Higher-priority providers win model ID conflicts (e.g. qwen-image-max).
func (m *Manager) InitConfigs() {
	// Start with builtin defaults (locale-aware priorities)
	for _, b := range BuiltinMediaProviders(m.locale) {
		m.configs[b.ID] = b
	}

	// Overlay persisted state (API key, enabled, base_url)
	if m.configStore != nil {
		saved, err := m.configStore.Load()
		if err != nil {
			log.Printf("[mediagen] failed to load saved configs: %v", err)
		}
		for id, s := range saved {
			if c, ok := m.configs[id]; ok {
				c.APIKey = s.APIKey
				c.HasAPIKey = s.APIKey != ""
				c.KeyHash = hashAPIKey(s.APIKey)
				c.Enabled = s.Enabled
				if s.BaseURL != "" {
					c.BaseURL = s.BaseURL
				}
			}
		}
	}

	// Register enabled providers (order doesn't matter — rebuildProviderOrderLocked sorts by priority)
	for _, c := range m.configs {
		if c.Enabled && c.APIKey != "" {
			if p := createProvider(c); p != nil {
				m.providers[p.Name()] = p
				c.Models = p.SupportedModels()
				log.Printf("[mediagen] registered provider: %s (priority %d)", c.ID, c.Priority)
			}
		}
	}

	// Build priority-sorted providerOrder + modelMap (higher priority wins conflicts)
	m.rebuildProviderOrderLocked()
}

// ListConfigs returns all media provider configurations.
func (m *Manager) ListConfigs() []*MediaProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MediaProviderConfig, 0, len(m.configs))
	for _, c := range m.configs {
		result = append(result, c)
	}
	return result
}

// GetConfig returns a single media provider configuration.
func (m *Manager) GetConfig(id string) *MediaProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs[id]
}

// SetAPIKey sets the API key for a media provider and persists.
func (m *Manager) SetAPIKey(id, key string) error {
	m.mu.Lock()
	c, ok := m.configs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown media provider: %s", id)
	}
	c.APIKey = key
	c.HasAPIKey = key != ""
	c.KeyHash = hashAPIKey(key)

	// If enabled and key set, register provider
	if c.Enabled && key != "" {
		if p := createProvider(c); p != nil {
			m.providers[p.Name()] = p
			c.Models = p.SupportedModels()
		}
	}
	m.rebuildProviderOrderLocked()
	m.mu.Unlock()

	return m.saveConfigs()
}

// RemoveAPIKey clears the API key and unregisters the provider.
func (m *Manager) RemoveAPIKey(id string) error {
	m.mu.Lock()
	c, ok := m.configs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown media provider: %s", id)
	}
	c.APIKey = ""
	c.HasAPIKey = false
	c.KeyHash = ""
	c.Models = nil
	delete(m.providers, id)
	m.rebuildProviderOrderLocked()
	m.mu.Unlock()

	return m.saveConfigs()
}

// Enable enables a media provider and registers it if API key is set.
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	c, ok := m.configs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown media provider: %s", id)
	}
	c.Enabled = true
	if c.APIKey != "" {
		if p := createProvider(c); p != nil {
			m.providers[p.Name()] = p
			c.Models = p.SupportedModels()
		}
	}
	m.rebuildProviderOrderLocked()
	m.mu.Unlock()

	return m.saveConfigs()
}

// Disable disables a media provider and unregisters it.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	c, ok := m.configs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("unknown media provider: %s", id)
	}
	c.Enabled = false
	c.Models = nil
	delete(m.providers, id)
	m.rebuildProviderOrderLocked()
	m.mu.Unlock()

	return m.saveConfigs()
}

// TestProvider tests connectivity for a media provider.
func (m *Manager) TestProvider(id string) *TestResult {
	m.mu.RLock()
	c, ok := m.configs[id]
	m.mu.RUnlock()
	if !ok {
		return &TestResult{Error: "unknown media provider: " + id}
	}
	return TestMediaProvider(c)
}

// HasImageProviders returns true if any registered provider supports image generation.
func (m *Manager) HasImageProviders() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.providers {
		if p.SupportsType(MediaTypeImage) {
			return true
		}
	}
	return false
}

// HasVideoProviders returns true if any registered provider supports video generation.
func (m *Manager) HasVideoProviders() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.providers {
		if p.SupportsType(MediaTypeVideo) {
			return true
		}
	}
	return false
}

// rebuildProviderOrderLocked rebuilds the priority-sorted provider order and model map.
// Must hold m.mu write lock.
// Model ID conflicts are resolved by priority: lower priority number wins.
func (m *Manager) rebuildProviderOrderLocked() {
	type entry struct {
		name     string
		priority int
	}
	var entries []entry
	for name := range m.providers {
		pri := 999
		if c, ok := m.configs[name]; ok {
			pri = c.Priority
		}
		entries = append(entries, entry{name, pri})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].priority < entries[j].priority
	})
	m.providerOrder = make([]string, len(entries))
	for i, e := range entries {
		m.providerOrder[i] = e.name
	}

	// Rebuild modelMap: iterate lowest-priority first, higher-priority overwrites
	m.modelMap = make(map[string]string)
	for i := len(entries) - 1; i >= 0; i-- {
		p := m.providers[entries[i].name]
		for _, model := range p.SupportedModels() {
			m.modelMap[model.ID] = p.Name()
		}
	}
}

// saveConfigs persists current configs to disk.
func (m *Manager) saveConfigs() error {
	if m.configStore == nil {
		return nil
	}
	m.mu.RLock()
	configs := make(map[string]*MediaProviderConfig, len(m.configs))
	for k, v := range m.configs {
		configs[k] = v
	}
	m.mu.RUnlock()
	return m.configStore.Save(configs)
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

// Models returns all available media models across providers, ordered by provider priority.
func (m *Manager) Models() []MediaModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var models []MediaModelInfo
	for _, name := range m.providerOrder {
		if p, ok := m.providers[name]; ok {
			models = append(models, p.SupportedModels()...)
		}
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
	// Exponential backoff: 500ms → 1s → 2s → 4s → ... capped at 5s
	interval := 500 * time.Millisecond
	const maxInterval = 5 * time.Second

	for {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			// Return last known task state (not an error)
			if task, err := m.GetTask(taskID); err == nil {
				return task, nil
			}
			return nil, ErrTimeout
		case <-timer.C:
		}

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

		// Exponential backoff
		interval = interval * 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}

// findProvider looks up the provider for a given model ID.
// When modelID is empty, returns the highest-priority registered provider.
func (m *Manager) findProvider(modelID string) (MediaProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if modelID == "" {
		// Pick highest-priority provider (providerOrder is sorted ascending)
		for _, name := range m.providerOrder {
			if p, ok := m.providers[name]; ok {
				return p, nil
			}
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
// Uses exponential backoff: 2s → 4s → 8s → ... capped at 15s.
func (m *Manager) pollTask(taskID string, provider MediaProvider) {
	interval := 2 * time.Second
	const maxInterval = 15 * time.Second
	deadline := time.After(defaultPollTimeout)

	for {
		timer := time.NewTimer(interval)
		select {
		case <-deadline:
			timer.Stop()
			m.updateTaskError(taskID, "generation timed out")
			return
		case <-timer.C:
		}

		v, ok := m.tasks.Load(taskID)
		if !ok {
			return
		}
		task := v.(*MediaTask)

		updated, err := provider.Poll(context.Background(), task.UpstreamID)
		if err != nil {
			log.Printf("[mediagen] poll error for task %s: %v", taskID, err)
			// Still backoff on error
			interval = interval * 2
			if interval > maxInterval {
				interval = maxInterval
			}
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

		// Exponential backoff
		interval = interval * 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}

// cacheResults downloads remote media to local storage and generates thumbnails.
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

		// Generate thumbnail for images
		if result.URL != "" && task.Type == MediaTypeImage {
			localPath := m.storage.localPathFromURL(result.URL)
			if isImageFile(localPath) {
				if thumbURL := m.storage.generateThumbnail(localPath); thumbURL != "" {
					result.ThumbnailURL = thumbURL
				}
			}
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

// createProvider creates a concrete MediaProvider from config.
func createProvider(c *MediaProviderConfig) MediaProvider {
	switch c.ID {
	case "gemini-image":
		return NewGeminiProvider(c.APIKey, c.BaseURL)
	case "dashscope-image":
		return NewDashScopeProvider(c.APIKey, c.BaseURL)
	case "mulerouter":
		return NewMuleRouterProvider(c.APIKey, c.BaseURL)
	default:
		return nil
	}
}
