package mediagen

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/scenecompose"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/google/uuid"
)

// CostEvent contains the data needed to record a media generation cost.
type CostEvent struct {
	Provider    string
	Model       string
	Type        string // "image" or "video"
	Category    string // "t2i", "t2v", etc.
	ImageCount  int
	DurationSec float64
	CostUSD     float64
	Success     bool
	LatencyMs   int64
}

// CostRecorder is a callback for recording media generation costs.
// Set via Manager.SetCostRecorder.

// EventPublisher pushes real-time events to connected clients (e.g. SSE broker).
type EventPublisher interface {
	Publish(userID string, eventType string, data any)
}
type CostRecorder func(event CostEvent)

// TaskDoneCallback is called when a media task reaches a terminal state (succeeded/failed/cancelled).
// taskID is the task ID, status is the terminal status, model is the model name, imageURL is the first result thumbnail (empty if none).
type TaskDoneCallback func(taskID, status, model, imageURL string)

// Manager orchestrates media generation across providers.
type Manager struct {
	providers     map[string]MediaProvider // name -> provider
	providerOrder []string                 // provider names sorted by priority (ascending)
	modelMap      map[string]string        // modelID -> provider name
	storage       *MediaStorage
	fallback      *FallbackEngine
	tasks         sync.Map // taskID -> *MediaTask
	taskStore     *TaskStore
	mu            sync.RWMutex

	// Provider config management
	configs      map[string]*MediaProviderConfig
	configStore  MediaConfigStore
	locale       string // user locale for priority ordering
	costRecorder CostRecorder
	onTaskDone   TaskDoneCallback
	eventPub     EventPublisher

	runCtx    context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	closed    atomic.Bool
}

// NewManager creates a new media generation manager.
func NewManager(storage *MediaStorage, configStore MediaConfigStore, locale string) *Manager {
	runCtx, cancel := context.WithCancel(context.Background())
	return &Manager{
		providers:   make(map[string]MediaProvider),
		modelMap:    make(map[string]string),
		storage:     storage,
		configs:     make(map[string]*MediaProviderConfig),
		configStore: configStore,
		locale:      locale,
		runCtx:      runCtx,
		cancel:      cancel,
	}
}

func (m *Manager) managerContext() context.Context {
	if m == nil || m.runCtx == nil {
		return context.Background()
	}
	return m.runCtx
}

func (m *Manager) isClosed() bool {
	return m == nil || m.closed.Load()
}

// Close stops background media task execution so shutdown can proceed cleanly.
// In-flight non-terminal tasks remain recoverable via the persistent task store.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		if m.cancel != nil {
			m.cancel()
		}
	})
	return nil
}

func extractMediaTaskUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if claims, ok := ctx.Value(auth.UserContextKey).(*auth.UserClaims); ok && claims != nil {
		return strings.TrimSpace(claims.UserID)
	}
	return strings.TrimSpace(tools.GetUserID(ctx))
}

func mediaScopeFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	if claims, ok := ctx.Value(auth.UserContextKey).(*auth.UserClaims); ok && claims != nil {
		if claims.Role == "admin" {
			return nil
		}
		return []string{strings.TrimSpace(claims.UserID)}
	}
	if userID := strings.TrimSpace(tools.GetUserID(ctx)); userID != "" {
		return []string{userID}
	}
	return nil
}

func taskVisibleToScope(task *MediaTask, userID []string) bool {
	if task == nil {
		return false
	}
	scopedUserID, scoped := normalizeMediaTaskScope(userID)
	if !scoped {
		return true
	}
	if scopedUserID == "" {
		return strings.TrimSpace(task.UserID) == ""
	}
	return strings.TrimSpace(task.UserID) == scopedUserID
}

func (m *Manager) registerConfiguredProviderLocked(c *MediaProviderConfig) {
	if c == nil {
		return
	}
	p := createProvider(c)
	if p == nil {
		return
	}
	c.Models = p.SupportedModels()
	delete(m.providers, c.ID)
	if !c.Enabled || !providerHasCredential(c) {
		return
	}
	m.providers[c.ID] = p
	log.Printf("[mediagen] registered provider: %s (priority %d)", c.ID, c.Priority)
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
				c.HasAPIKey = providerHasCredential(c)
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
		c.HasAPIKey = providerHasCredential(c)
		m.registerConfiguredProviderLocked(c)
	}

	// Build priority-sorted providerOrder + modelMap (higher priority wins conflicts)
	m.rebuildProviderOrderLocked()
}

func (m *Manager) GetFallbackModelStatus(modelID string) (*scenecompose.ModelStatus, error) {
	if m == nil || m.fallback == nil {
		return nil, fmt.Errorf("fallback engine unavailable")
	}
	return m.fallback.GetFallbackModelStatus(modelID)
}

// ListConfigs returns all media provider configurations.
func (m *Manager) ListConfigs() []*MediaProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MediaProviderConfig, 0, len(m.configs))
	for _, c := range m.configs {
		EnrichModelPricing(c.Models)
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
	c.HasAPIKey = providerHasCredential(c)
	c.KeyHash = hashAPIKey(key)

	m.registerConfiguredProviderLocked(c)
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
	c.HasAPIKey = providerHasCredential(c)
	c.KeyHash = ""
	m.registerConfiguredProviderLocked(c)
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
	m.registerConfiguredProviderLocked(c)
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
	oldBaseURL := c.BaseURL
	result := TestMediaProvider(c)
	// If probe auto-switched the BaseURL (e.g. MiniMax regional fallback), persist it
	if c.BaseURL != oldBaseURL {
		_ = m.saveConfigs()
	}
	return result
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
			m.modelMap[model.ID] = entries[i].name
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
	m.rebuildProviderOrderLocked()
}

// SetFallbackEngine installs the built-in no-key fallback executor.
func (m *Manager) SetFallbackEngine(engine *FallbackEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallback = engine
}

// SetFallbackVisionBridge wires an optional VLM into the fallback engine for result reranking.
func (m *Manager) SetFallbackVisionBridge(bridge tools.VLMBridge) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fallback == nil {
		return
	}
	m.fallback.SetVisionBridge(bridge)
}

// RenderFallbackPage returns a temporary internal HTML page used by the screenshot fallback.
func (m *Manager) RenderFallbackPage(token string) (string, bool) {
	m.mu.RLock()
	fallback := m.fallback
	m.mu.RUnlock()
	if fallback == nil {
		return "", false
	}
	return fallback.RenderPage(token)
}

// ReadServedURL reads a locally cached media asset by its served URL.
func (m *Manager) ReadServedURL(servedURL string) ([]byte, error) {
	if m == nil || m.storage == nil {
		return nil, fmt.Errorf("media storage unavailable")
	}
	return m.storage.ReadServedURL(servedURL)
}

// HasActiveProviders returns true if at least one media generation provider is registered.
func (m *Manager) HasActiveProviders() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.providers) > 0
}

// HasAvailableModels reports whether either a real provider or the built-in fallback can generate media.
func (m *Manager) HasAvailableModels() bool {
	return len(m.Models()) > 0
}

// Models returns all available media models across providers, ordered by provider priority.
func (m *Manager) Models() []MediaModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models := m.realModelsLocked()
	if m.fallback != nil && m.fallback.Enabled() {
		missing := map[MediaCategory]bool{
			CategoryT2I:  true,
			CategoryI2I:  true,
			CategoryT2V:  true,
			CategoryI2V:  true,
			CategoryKF2V: true,
		}
		for _, model := range models {
			if model.Category != CategoryNone {
				missing[model.Category] = false
			}
		}
		for _, model := range m.fallback.SupportedModels() {
			if missing[model.Category] {
				models = append(models, model)
			}
		}
	}
	EnrichModelPricing(models)
	return models
}

func (m *Manager) realModelsLocked() []MediaModelInfo {
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
	if m.isClosed() {
		return nil, ErrManagerClosed
	}
	category := inferCategoryFromRequest(req)
	provider, resolvedModel, err := m.resolveExecutor(req, string(category))
	if err != nil {
		return nil, err
	}

	if !provider.SupportsType(req.Type) {
		return nil, fmt.Errorf("%w: provider %s does not support %s", ErrUnsupportedType, provider.Name(), req.Type)
	}

	effectiveReq := cloneMediaRequest(req)
	effectiveReq.Model = resolvedModel
	req.Model = resolvedModel

	task, err := provider.Generate(ctx, effectiveReq)
	if err != nil {
		return nil, err
	}

	// Assign a local task ID if the provider didn't set one
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Provider = provider.Name()
	task.Model = resolvedModel
	task.Type = req.Type
	if task.Category == "" && category != CategoryNone {
		task.Category = string(category)
	}
	if task.Request == nil {
		task.Request = cloneMediaRequest(effectiveReq)
	}
	task.CreatedAt = timeutil.NowTime()
	if task.UserID == "" {
		task.UserID = extractMediaTaskUserID(ctx)
	}

	m.tasks.Store(task.ID, task)

	// If async (pending/processing), start background polling
	if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing {
		m.tasks.Store(task.ID, task)
		if !m.isClosed() {
			go m.pollTask(task.ID, provider)
		}
	} else if task.Status == TaskStatusSucceeded {
		// Sync provider returned immediately — cache media in background.
		// Store as "processing" first so the ChannelTaskWatcher doesn't see
		// Succeeded with the original remote URL before cacheResults downloads
		// the file and re-stores with local URLs.
		task.Status = TaskStatusProcessing
		m.tasks.Store(task.ID, task)
		if !m.isClosed() {
			go func() {
				task.Status = TaskStatusSucceeded
				m.cacheResults(task)
			}()
		}
	} else {
		m.tasks.Store(task.ID, task)
	}

	return task, nil
}

// GetTask returns the current state of a task.
func (m *Manager) GetTask(taskID string, userID ...string) (*MediaTask, error) {
	v, ok := m.tasks.Load(taskID)
	if ok {
		task := v.(*MediaTask)
		if taskVisibleToScope(task, userID) {
			return task, nil
		}
		return nil, ErrTaskNotFound
	}
	// Fall back to persistent store
	if m.taskStore != nil {
		pt, err := m.taskStore.Get(taskID, userID...)
		if err == nil && pt != nil {
			return pt.ToMediaTask(), nil
		}
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}
	return nil, ErrTaskNotFound
}

// WaitForTask blocks until the task completes or context is cancelled.
func (m *Manager) WaitForTask(ctx context.Context, taskID string, userID ...string) (*MediaTask, error) {
	scopeArgs := userID
	if len(scopeArgs) == 0 {
		scopeArgs = mediaScopeFromContext(ctx)
	}

	// Exponential backoff: 500ms → 1s → 2s → 4s → ... capped at 5s
	interval := 500 * time.Millisecond
	const maxInterval = 5 * time.Second

	for {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			// Return last known task state (not an error)
			if task, err := m.GetTask(taskID, scopeArgs...); err == nil {
				return task, nil
			}
			return nil, ErrTimeout
		case <-timer.C:
		}

		task, err := m.GetTask(taskID, scopeArgs...)
		if err != nil {
			return nil, err
		}
		switch task.Status {
		case TaskStatusSucceeded:
			return task, nil
		case TaskStatusFailed:
			return task, fmt.Errorf("%w: %s", ErrGenerationFailed, task.Error)
		case TaskStatusCancelled:
			if strings.TrimSpace(task.Error) != "" {
				return task, fmt.Errorf("%w: %s", ErrGenerationCancelled, task.Error)
			}
			return task, ErrGenerationCancelled
		}

		// Exponential backoff
		interval = interval * 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}

func (m *Manager) isTaskCancelled(taskID string) bool {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return false
	}
	task, ok := v.(*MediaTask)
	return ok && task != nil && task.Status == TaskStatusCancelled
}

// findProvider looks up the provider for a given model ID.
// When modelID is empty, returns the highest-priority registered provider.
func (m *Manager) findProvider(modelID string) (MediaProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.findProviderLocked(modelID)
}

// pollTask polls an async provider until the task completes.
// Uses exponential backoff: 2s → 4s → 8s → ... capped at 15s.
// No timeout — polls indefinitely until the provider returns a terminal status or a fatal error.
func (m *Manager) pollTask(taskID string, provider MediaProvider) {
	ctx := m.managerContext()
	interval := 2 * time.Second
	const maxInterval = 15 * time.Second

	for {
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}

		v, ok := m.tasks.Load(taskID)
		if !ok {
			return
		}
		task := v.(*MediaTask)
		if task.Status == TaskStatusCancelled {
			return
		}

		updated, err := provider.Poll(ctx, task.UpstreamID)
		if err != nil {
			if m.isClosed() || m.isTaskCancelled(taskID) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			log.Printf("[mediagen] poll error for task %s: %v", taskID, err)
			// Fatal errors (missing metadata, unknown task) — fail immediately, retrying won't help
			errMsg := err.Error()
			if strings.Contains(errMsg, "no vendor/model metadata") || strings.Contains(errMsg, "unknown task") {
				m.updateTaskError(taskID, errMsg)
				return
			}
			// Transient errors — backoff and retry
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
		updated.UpstreamID = task.UpstreamID
		updated.MessageID = task.MessageID
		updated.Category = task.Category
		updated.Source = task.Source
		if updated.FallbackInfo == nil {
			updated.FallbackInfo = cloneFallbackInfo(task.FallbackInfo)
		}
		if m.isTaskCancelled(taskID) || m.isClosed() {
			return
		}

		switch updated.Status {
		case TaskStatusSucceeded:
			// cacheResults downloads media, stores the task with local URLs,
			// persists to DB, and publishes the event. Do NOT store the task
			// before cacheResults — the ChannelTaskWatcher would see Succeeded
			// with the original remote URL and fail to load binary data.
			m.cacheResults(updated)
			return
		case TaskStatusFailed:
			m.tasks.Store(taskID, updated)
			if m.taskStore != nil {
				_ = m.taskStore.UpdateStatus(taskID, TaskStatusFailed, updated.Progress, updated.Error, "")
				if updated.FallbackInfo != nil {
					_ = m.taskStore.UpdateFallbackInfo(taskID, updated.FallbackInfo)
				}
			}
			m.publishTaskEvent(updated)
			return
		default:
			m.tasks.Store(taskID, updated)
			// Persist progress
			if m.taskStore != nil {
				_ = m.taskStore.UpdateStatus(taskID, updated.Status, updated.Progress, "", "")
				if updated.FallbackInfo != nil {
					_ = m.taskStore.UpdateFallbackInfo(taskID, updated.FallbackInfo)
				}
			}
			m.publishTaskEvent(updated)
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
	if task == nil || m.isTaskCancelled(task.ID) || m.isClosed() {
		return
	}
	if task.Response == nil || m.storage == nil {
		return
	}
	ctx := m.managerContext()

	for i := range task.Response.Data {
		result := &task.Response.Data[i]

		// Download from remote URL
		if result.OriginalURL != "" && result.URL == "" {
			localURL, err := m.storage.Download(ctx, result.OriginalURL, task.Type)
			if err != nil {
				if m.isClosed() || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				log.Printf("[mediagen] cache download failed for task %s: %v", task.ID, err)
				continue
			}
			result.URL = localURL
		}

		// If no OriginalURL and no URL, log for debugging
		if result.OriginalURL == "" && result.URL == "" && result.B64JSON == "" {
			log.Printf("[mediagen] result %d for task %s has no URL, OriginalURL, or B64JSON (content_type=%s)", i, task.ID, result.ContentType)
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

	now := timeutil.NowTime()
	task.CompletedAt = &now
	if m.isTaskCancelled(task.ID) || m.isClosed() {
		return
	}
	m.tasks.Store(task.ID, task)

	// Record cost
	m.recordCost(task)

	// Persist success to DB
	if m.taskStore != nil {
		respJSON := ""
		if task.Response != nil {
			if b, err := json.Marshal(task.Response); err == nil {
				respJSON = string(b)
			}
		}
		_ = m.taskStore.UpdateStatus(task.ID, TaskStatusSucceeded, 1.0, "", respJSON)
		if task.FallbackInfo != nil {
			_ = m.taskStore.UpdateFallbackInfo(task.ID, task.FallbackInfo)
		}
	}

	if m.onTaskDone != nil {
		imageURL := ""
		if task.Response != nil && len(task.Response.Data) > 0 {
			d := task.Response.Data[0]
			if d.ThumbnailURL != "" {
				imageURL = d.ThumbnailURL
			} else if d.URL != "" {
				imageURL = d.URL
			}
		}
		m.onTaskDone(task.ID, string(TaskStatusSucceeded), task.Model, imageURL)
	}

	m.publishTaskEvent(task)
}

// updateTaskError marks a task as failed.
func (m *Manager) updateTaskError(taskID, errMsg string) {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return
	}
	task := v.(*MediaTask)
	if task.Status == TaskStatusCancelled {
		return
	}
	task.Status = TaskStatusFailed
	task.Error = errMsg
	now := timeutil.NowTime()
	task.CompletedAt = &now
	m.tasks.Store(taskID, task)

	// Persist to DB
	if m.taskStore != nil {
		_ = m.taskStore.UpdateStatus(taskID, TaskStatusFailed, task.Progress, errMsg, "")
		if task.FallbackInfo != nil {
			_ = m.taskStore.UpdateFallbackInfo(taskID, task.FallbackInfo)
		}
	}

	if m.onTaskDone != nil {
		m.onTaskDone(taskID, string(TaskStatusFailed), task.Model, "")
	}

	m.publishTaskEvent(task)
}

// CancelTask cancels a pending or processing task. Returns true if the task was cancelled.
func (m *Manager) CancelTask(taskID string, userID ...string) bool {
	v, ok := m.tasks.Load(taskID)
	if !ok {
		return false
	}
	task := v.(*MediaTask)
	if !taskVisibleToScope(task, userID) {
		return false
	}
	if task.Status != TaskStatusPending && task.Status != TaskStatusProcessing {
		return false
	}
	task.Status = TaskStatusCancelled
	now := timeutil.NowTime()
	task.CompletedAt = &now
	m.tasks.Store(taskID, task)

	if m.taskStore != nil {
		_ = m.taskStore.UpdateStatus(taskID, TaskStatusCancelled, task.Progress, "cancelled by user", "")
	}

	if m.onTaskDone != nil {
		m.onTaskDone(taskID, string(TaskStatusCancelled), task.Model, "")
	}

	m.publishTaskEvent(task)
	return true
}

// recordCost calculates and records the cost for a completed media task.
func (m *Manager) recordCost(task *MediaTask) {
	if m.costRecorder == nil {
		return
	}

	imageCount := 0
	var durationSec float64
	if task.Response != nil {
		imageCount = len(task.Response.Data)
		for _, r := range task.Response.Data {
			durationSec += float64(r.DurationSec)
		}
	}

	cost := CalculateMediaCost(task.Model, imageCount, durationSec)

	var latencyMs int64
	if task.CompletedAt != nil && !task.CreatedAt.IsZero() {
		latencyMs = task.CompletedAt.Sub(task.CreatedAt).Milliseconds()
	}

	m.costRecorder(CostEvent{
		Provider:    task.Provider,
		Model:       task.Model,
		Type:        string(task.Type),
		Category:    task.Category,
		ImageCount:  imageCount,
		DurationSec: durationSec,
		CostUSD:     cost,
		Success:     true,
		LatencyMs:   latencyMs,
	})
}

// SetTaskStore sets the persistent task store for power-failure recovery.
func (m *Manager) SetTaskStore(store *TaskStore) {
	m.taskStore = store
}

// SetCostRecorder sets the callback for recording media generation costs.
func (m *Manager) SetCostRecorder(recorder CostRecorder) {
	m.costRecorder = recorder
}

// SetOnTaskDone sets the callback invoked when a task reaches a terminal state.
func (m *Manager) SetOnTaskDone(cb TaskDoneCallback) {
	m.onTaskDone = cb
}

// SetEventPublisher sets the SSE broker for real-time task progress events.
func (m *Manager) SetEventPublisher(pub EventPublisher) {
	m.eventPub = pub
}

// publishTaskEvent sends a media_task_update event to all SSE clients.
func (m *Manager) publishTaskEvent(task *MediaTask) {
	if m.eventPub == nil {
		return
	}
	evt := map[string]interface{}{
		"id":       task.ID,
		"status":   string(task.Status),
		"progress": task.Progress,
		"type":     string(task.Type),
	}
	if task.Error != "" {
		evt["error"] = task.Error
	}
	if task.Response != nil {
		evt["response"] = task.Response
	}
	if task.FallbackInfo != nil {
		evt["fallback_info"] = task.FallbackInfo
	}
	userID := strings.TrimSpace(task.UserID)
	if userID == "" {
		userID = "default"
	}
	m.eventPub.Publish(userID, "media_task_update", evt)
}

// DefaultModelForCategory returns the first available model ID for a given category,
// respecting provider priority order. Returns "" if no model matches.
func (m *Manager) DefaultModelForCategory(category string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cat := MediaCategory(category)
	for _, name := range m.providerOrder {
		p, ok := m.providers[name]
		if !ok {
			continue
		}
		for _, model := range p.SupportedModels() {
			if model.Category == cat {
				return model.ID
			}
		}
	}
	if m.fallback != nil && m.fallback.Enabled() {
		return m.fallback.DefaultModelForCategory(cat)
	}
	return ""
}

// CreateTask creates a persistent media generation task and starts async execution.
// Returns immediately with the task ID — the caller polls for status.
func (m *Manager) CreateTask(ctx context.Context, req *MediaRequest, messageID, category, source string) (*MediaTask, error) {
	if m.isClosed() {
		return nil, ErrManagerClosed
	}
	provider, resolvedModel, err := m.resolveExecutor(req, category)
	if err != nil {
		return nil, err
	}
	if !provider.SupportsType(req.Type) {
		return nil, fmt.Errorf("%w: provider %s does not support %s", ErrUnsupportedType, provider.Name(), req.Type)
	}

	effectiveReq := cloneMediaRequest(req)
	effectiveReq.Model = resolvedModel
	req.Model = resolvedModel
	if category == "" {
		category = string(inferCategoryFromRequest(effectiveReq))
	}

	taskID := uuid.New().String()
	now := timeutil.NowTime()
	task := &MediaTask{
		BaseTask:  basetask.BaseTask{ID: taskID, Status: TaskStatusPending, CreatedAt: now},
		UserID:    extractMediaTaskUserID(ctx),
		MessageID: messageID,
		Type:      req.Type,
		Category:  category,
		Provider:  provider.Name(),
		Model:     resolvedModel,
		Request:   effectiveReq,
		Source:    source,
	}
	if m.fallback != nil {
		task.FallbackInfo = m.fallback.PendingInfoForRequest(effectiveReq, resolvedModel)
	}

	// Persist to DB first (survives power failure)
	if m.taskStore != nil {
		pt := FromMediaTask(task)
		if err := m.taskStore.Create(pt); err != nil {
			return nil, fmt.Errorf("failed to persist task: %w", err)
		}
	}

	// Store in memory
	m.tasks.Store(taskID, task)

	// Start async generation
	if !m.isClosed() {
		go m.executeTask(task, provider)
	}

	return task, nil
}

// executeTask runs the actual generation and updates task state.
func (m *Manager) executeTask(task *MediaTask, provider MediaProvider) {
	ctx := m.managerContext()
	if m.isClosed() {
		return
	}

	if task.Status != TaskStatusCancelled {
		task.Status = TaskStatusProcessing
		if task.Progress < 0.05 {
			task.Progress = 0.05
		}
		m.tasks.Store(task.ID, task)
		if m.taskStore != nil {
			_ = m.taskStore.UpdateStatus(task.ID, TaskStatusProcessing, task.Progress, "", "")
			if task.FallbackInfo != nil {
				_ = m.taskStore.UpdateFallbackInfo(task.ID, task.FallbackInfo)
			}
		}
		m.publishTaskEvent(task)
	}

	upstream, err := provider.Generate(ctx, task.Request)
	if err != nil {
		if task.Status == TaskStatusCancelled || m.isClosed() || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		m.updateTaskError(task.ID, err.Error())
		return
	}
	if task.Status == TaskStatusCancelled || m.isClosed() {
		return
	}

	// Update with upstream info
	task.UpstreamID = upstream.UpstreamID
	if upstream.FallbackInfo != nil {
		task.FallbackInfo = cloneFallbackInfo(upstream.FallbackInfo)
	}
	if upstream.Category != "" {
		task.Category = upstream.Category
	}
	if m.taskStore != nil && task.FallbackInfo != nil {
		_ = m.taskStore.UpdateFallbackInfo(task.ID, task.FallbackInfo)
	}
	if m.taskStore != nil {
		_ = m.taskStore.UpdateUpstreamID(task.ID, upstream.UpstreamID)
	}

	// If sync provider returned immediately
	if upstream.Status == TaskStatusSucceeded {
		if task.Status == TaskStatusCancelled || m.isClosed() {
			return
		}
		task.Status = TaskStatusSucceeded
		task.Response = upstream.Response
		task.Progress = 1.0
		m.tasks.Store(task.ID, task)
		m.cacheResults(task)
		return
	}

	// Async: update status to processing and start polling
	if task.Status == TaskStatusCancelled || m.isClosed() {
		return
	}
	task.Status = TaskStatusProcessing
	m.tasks.Store(task.ID, task)
	if m.taskStore != nil {
		_ = m.taskStore.UpdateStatus(task.ID, TaskStatusProcessing, 0, "", "")
	}

	m.pollTask(task.ID, provider)
}

// GetTasksByMessage returns all tasks associated with a message ID.
func (m *Manager) GetTasksByMessage(messageID string, userID ...string) ([]*MediaTask, error) {
	if m.taskStore == nil {
		return nil, nil
	}
	pts, err := m.taskStore.GetByMessageID(messageID, userID...)
	if err != nil {
		return nil, err
	}
	var tasks []*MediaTask
	for _, pt := range pts {
		tasks = append(tasks, pt.ToMediaTask())
	}
	return tasks, nil
}

// RecoverTasks resumes non-terminal tasks after a restart.
func (m *Manager) RecoverTasks() {
	if m.isClosed() {
		return
	}
	if m.taskStore == nil {
		return
	}
	pending, err := m.taskStore.ListPending()
	if err != nil {
		log.Printf("[mediagen] failed to load pending tasks for recovery: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}
	log.Printf("[mediagen] recovering %d pending media tasks", len(pending))

	for _, pt := range pending {
		if m.isClosed() {
			return
		}
		task := pt.ToMediaTask()
		m.tasks.Store(task.ID, task)

		// Find the provider or fallback executor to resume.
		provider, resolvedModel, err := m.resolveExecutor(task.Request, task.Category)
		if err != nil {
			log.Printf("[mediagen] recovery: provider not found for task %s model %s: %v", task.ID, task.Model, err)
			m.updateTaskError(task.ID, "provider not available after restart")
			continue
		}
		if task.Model == "" {
			task.Model = resolvedModel
		}

		if task.UpstreamID != "" {
			// Restore in-memory metadata for providers that need it (e.g. MuleRouter taskMeta)
			if restorer, ok := provider.(interface{ RestoreTaskMeta(string, string, string) }); ok {
				restorer.RestoreTaskMeta(task.UpstreamID, task.Model, task.Category)
			}
			// Has upstream ID — resume polling
			log.Printf("[mediagen] resuming poll for task %s (upstream: %s)", task.ID, task.UpstreamID)
			go m.pollTask(task.ID, provider)
		} else {
			// No upstream ID — re-execute from scratch
			log.Printf("[mediagen] re-executing task %s", task.ID)
			go m.executeTask(task, provider)
		}
	}
}

func (m *Manager) resolveExecutor(req *MediaRequest, category string) (MediaProvider, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cat := MediaCategory(strings.TrimSpace(category))
	if cat == CategoryNone {
		cat = inferCategoryFromRequest(req)
	}
	modelID := strings.TrimSpace(req.Model)
	if modelID != "" {
		if m.fallback != nil && m.fallback.Enabled() && m.fallback.IsFallbackModel(modelID) {
			return m.fallback, modelID, nil
		}
		provider, err := m.findProviderLocked(modelID)
		return provider, modelID, err
	}

	if cat != CategoryNone {
		if provider, model := m.firstRealProviderForCategoryLocked(cat); provider != nil {
			return provider, model, nil
		}
		if m.fallback != nil && m.fallback.Enabled() {
			if fallbackModel := m.fallback.ModelForRequest(req, cat); fallbackModel != "" {
				return m.fallback, fallbackModel, nil
			}
		}
	}

	if req.Type != "" {
		if provider, model := m.firstRealProviderForTypeLocked(req.Type); provider != nil {
			return provider, model, nil
		}
		if m.fallback != nil && m.fallback.Enabled() {
			if fallbackModel := m.fallback.ModelForRequest(req, inferCategoryFromRequest(req)); fallbackModel != "" {
				return m.fallback, fallbackModel, nil
			}
		}
	}

	return nil, "", ErrProviderNotFound
}

func (m *Manager) firstRealProviderForCategoryLocked(category MediaCategory) (MediaProvider, string) {
	for _, name := range m.providerOrder {
		provider, ok := m.providers[name]
		if !ok {
			continue
		}
		for _, model := range provider.SupportedModels() {
			if model.Category == category {
				return provider, model.ID
			}
		}
	}
	return nil, ""
}

func (m *Manager) firstRealProviderForTypeLocked(mediaType MediaType) (MediaProvider, string) {
	for _, name := range m.providerOrder {
		provider, ok := m.providers[name]
		if !ok || !provider.SupportsType(mediaType) {
			continue
		}
		for _, model := range provider.SupportedModels() {
			if model.Type == mediaType {
				return provider, model.ID
			}
		}
		return provider, ""
	}
	return nil, ""
}

func (m *Manager) findProviderLocked(modelID string) (MediaProvider, error) {
	if modelID == "" {
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

// createProvider creates a concrete MediaProvider from config.
func createProvider(c *MediaProviderConfig) MediaProvider {
	switch c.ID {
	case "gemini-image":
		return NewGeminiProvider(c.APIKey, c.BaseURL)
	case "dashscope-image":
		return NewDashScopeProvider(c.APIKey, c.BaseURL)
	case "mulerouter":
		return NewMuleRouterProvider(c.APIKey, c.BaseURL)
	case "minimax-media":
		return NewMiniMaxProvider(c.APIKey, c.BaseURL)
	case fakeMediaProviderID:
		return NewFakeMediaProvider()
	default:
		return nil
	}
}
