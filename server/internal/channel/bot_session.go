// Package channel provides bot session management for monitoring.
package channel

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/google/uuid"
)

// CompanionSessionManager is the minimal interface needed for bot session management.
type CompanionSessionManager interface {
	CreateSession(ctx context.Context, session *companion.Session) (*companion.Session, error)
	EmitEvent(ctx context.Context, event *companion.SessionEvent) error
	EndSession(ctx context.Context, id string) error
}

// SessionInfo holds session metadata for tracking.
type SessionInfo struct {
	ID           string
	UserID       string
	CreatedAt    time.Time
	LastActiveAt time.Time
	MessageCount int64
	ReplyCount   int64
}

// BotSessionStats holds statistics for the bot session manager.
type BotSessionStats struct {
	ActiveSessions   int64 `json:"active_sessions"`
	TotalSessions    int64 `json:"total_sessions"`
	TotalMessages    int64 `json:"total_messages"`
	TotalReplies     int64 `json:"total_replies"`
	TotalErrors      int64 `json:"total_errors"`
	EventsQueued     int64 `json:"events_queued"`
	EventsProcessed  int64 `json:"events_processed"`
	EventsDropped    int64 `json:"events_dropped"`
}

// BotSessionConfig holds configuration for the bot session manager.
type BotSessionConfig struct {
	// SessionTimeout is the duration after which inactive sessions are cleaned up.
	// Default: 30 minutes
	SessionTimeout time.Duration

	// EventQueueSize is the size of the async event queue.
	// Default: 100
	EventQueueSize int

	// BatchSize is the number of events to process in a batch.
	// Default: 10
	BatchSize int

	// BatchInterval is the interval for batch processing.
	// Default: 100ms
	BatchInterval time.Duration
}

// DefaultBotSessionConfig returns the default configuration.
func DefaultBotSessionConfig() *BotSessionConfig {
	return &BotSessionConfig{
		SessionTimeout: 30 * time.Minute,
		EventQueueSize: 100,
		BatchSize:      10,
		BatchInterval:  100 * time.Millisecond,
	}
}

// BotSessionManager manages bot conversation sessions for monitoring.
// It provides a common abstraction for all bot channels to track conversations
// asynchronously with the companion monitoring system.
type BotSessionManager struct {
	companionManager CompanionSessionManager
	platform         companion.Platform
	config           *BotSessionConfig

	// Map channel+user to session info
	sessions   map[string]*SessionInfo
	sessionsMu sync.RWMutex

	// Statistics
	stats BotSessionStats

	// Async event queue
	eventQueue chan *companion.SessionEvent
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewBotSessionManager creates a new BotSessionManager with default config.
func NewBotSessionManager(companionManager CompanionSessionManager, platform companion.Platform) *BotSessionManager {
	return NewBotSessionManagerWithConfig(companionManager, platform, DefaultBotSessionConfig())
}

// NewBotSessionManagerWithConfig creates a new BotSessionManager with custom config.
func NewBotSessionManagerWithConfig(companionManager CompanionSessionManager, platform companion.Platform, config *BotSessionConfig) *BotSessionManager {
	if config == nil {
		config = DefaultBotSessionConfig()
	}

	m := &BotSessionManager{
		companionManager: companionManager,
		platform:         platform,
		config:           config,
		sessions:         make(map[string]*SessionInfo),
		eventQueue:       make(chan *companion.SessionEvent, config.EventQueueSize),
		done:             make(chan struct{}),
	}

	// Start async event processor
	m.wg.Add(1)
	go m.processEvents()

	// Start session cleanup goroutine
	m.wg.Add(1)
	go m.cleanupLoop()

	return m
}

// GetOrCreateSession gets or creates a session for a conversation.
// The key is typically "chatID" for group chats or "userID" for P2P chats.
func (m *BotSessionManager) GetOrCreateSession(ctx context.Context, key string, userID string) (string, error) {
	if m.companionManager == nil {
		return "", nil
	}

	m.sessionsMu.RLock()
	info, exists := m.sessions[key]
	m.sessionsMu.RUnlock()

	if exists {
		// Update last active time
		m.sessionsMu.Lock()
		info.LastActiveAt = time.Now()
		m.sessionsMu.Unlock()
		return info.ID, nil
	}

	// Create new session
	session := &companion.Session{
		Platform: m.platform,
		UserID:   userID,
		Metadata: companion.SessionMeta{
			ChannelID: key,
		},
	}

	created, err := m.companionManager.CreateSession(ctx, session)
	if err != nil {
		return "", err
	}

	now := time.Now()
	m.sessionsMu.Lock()
	m.sessions[key] = &SessionInfo{
		ID:           created.ID,
		UserID:       userID,
		CreatedAt:    now,
		LastActiveAt: now,
	}
	m.sessionsMu.Unlock()

	atomic.AddInt64(&m.stats.ActiveSessions, 1)
	atomic.AddInt64(&m.stats.TotalSessions, 1)

	return created.ID, nil
}

// GetSessionInfo returns session info for a key.
func (m *BotSessionManager) GetSessionInfo(key string) *SessionInfo {
	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()
	if info, exists := m.sessions[key]; exists {
		// Return a copy
		return &SessionInfo{
			ID:           info.ID,
			UserID:       info.UserID,
			CreatedAt:    info.CreatedAt,
			LastActiveAt: info.LastActiveAt,
			MessageCount: info.MessageCount,
			ReplyCount:   info.ReplyCount,
		}
	}
	return nil
}

// GetStats returns current statistics.
func (m *BotSessionManager) GetStats() BotSessionStats {
	return BotSessionStats{
		ActiveSessions:   atomic.LoadInt64(&m.stats.ActiveSessions),
		TotalSessions:    atomic.LoadInt64(&m.stats.TotalSessions),
		TotalMessages:    atomic.LoadInt64(&m.stats.TotalMessages),
		TotalReplies:     atomic.LoadInt64(&m.stats.TotalReplies),
		TotalErrors:      atomic.LoadInt64(&m.stats.TotalErrors),
		EventsQueued:     atomic.LoadInt64(&m.stats.EventsQueued),
		EventsProcessed:  atomic.LoadInt64(&m.stats.EventsProcessed),
		EventsDropped:    atomic.LoadInt64(&m.stats.EventsDropped),
	}
}

// EmitMessageReceived emits a message received event asynchronously.
func (m *BotSessionManager) EmitMessageReceived(sessionID string, userID string, content string) {
	if m.companionManager == nil || sessionID == "" {
		return
	}

	atomic.AddInt64(&m.stats.TotalMessages, 1)

	// Update session message count
	m.sessionsMu.Lock()
	for _, info := range m.sessions {
		if info.ID == sessionID {
			info.MessageCount++
			info.LastActiveAt = time.Now()
			break
		}
	}
	m.sessionsMu.Unlock()

	event := &companion.SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: companion.EventMessageReceived,
		Platform:  m.platform,
		UserID:    userID,
		Message: &companion.MessageEvent{
			Direction:   "inbound",
			Content:     truncateContent(content, 1000),
			ContentType: "text",
			Length:      len(content),
			Truncated:   len(content) > 1000,
		},
		Status: "completed",
	}

	m.queueEvent(event)
}

// EmitMessageSent emits a message sent event asynchronously.
func (m *BotSessionManager) EmitMessageSent(sessionID string, userID string, content string) {
	if m.companionManager == nil || sessionID == "" {
		return
	}

	atomic.AddInt64(&m.stats.TotalReplies, 1)

	// Update session reply count
	m.sessionsMu.Lock()
	for _, info := range m.sessions {
		if info.ID == sessionID {
			info.ReplyCount++
			info.LastActiveAt = time.Now()
			break
		}
	}
	m.sessionsMu.Unlock()

	event := &companion.SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: companion.EventMessageSent,
		Platform:  m.platform,
		UserID:    userID,
		Message: &companion.MessageEvent{
			Direction:   "outbound",
			Content:     truncateContent(content, 1000),
			ContentType: "text",
			Length:      len(content),
			Truncated:   len(content) > 1000,
		},
		Status: "completed",
	}

	m.queueEvent(event)
}

// EmitLLMRequest emits an LLM request event asynchronously.
func (m *BotSessionManager) EmitLLMRequest(sessionID string, userID string, provider string, model string, promptTokens int, completionTokens int, duration time.Duration, err error) {
	if m.companionManager == nil || sessionID == "" {
		return
	}

	status := "completed"
	errMsg := ""
	if err != nil {
		status = "error"
		errMsg = err.Error()
		atomic.AddInt64(&m.stats.TotalErrors, 1)
	}

	event := &companion.SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: companion.EventLLMRequest,
		Platform:  m.platform,
		UserID:    userID,
		LLMRequest: &companion.LLMRequestEvent{
			Provider:         provider,
			Model:            model,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			Duration:         companion.FromDuration(duration),
			Status:           status,
			Error:            errMsg,
		},
		Duration: companion.FromDuration(duration),
		Status:   status,
		Error:    errMsg,
	}

	m.queueEvent(event)
}

// EmitError emits an error event asynchronously.
func (m *BotSessionManager) EmitError(sessionID string, userID string, errMsg string) {
	if m.companionManager == nil || sessionID == "" {
		return
	}

	atomic.AddInt64(&m.stats.TotalErrors, 1)

	event := &companion.SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: companion.EventError,
		Platform:  m.platform,
		UserID:    userID,
		Status:    "error",
		Error:     errMsg,
	}

	m.queueEvent(event)
}

// EndSession ends a session.
func (m *BotSessionManager) EndSession(ctx context.Context, key string) error {
	if m.companionManager == nil {
		return nil
	}

	m.sessionsMu.Lock()
	info, exists := m.sessions[key]
	if exists {
		delete(m.sessions, key)
		atomic.AddInt64(&m.stats.ActiveSessions, -1)
	}
	m.sessionsMu.Unlock()

	if exists && info != nil {
		return m.companionManager.EndSession(ctx, info.ID)
	}
	return nil
}

// Stop stops the bot session manager.
func (m *BotSessionManager) Stop() {
	close(m.done)
	m.wg.Wait()
}

// queueEvent queues an event for async processing.
func (m *BotSessionManager) queueEvent(event *companion.SessionEvent) {
	atomic.AddInt64(&m.stats.EventsQueued, 1)

	select {
	case m.eventQueue <- event:
	default:
		// Queue full, drop event and log
		atomic.AddInt64(&m.stats.EventsDropped, 1)
	}
}

// processEvents processes events from the queue asynchronously with batching.
func (m *BotSessionManager) processEvents() {
	defer m.wg.Done()

	batch := make([]*companion.SessionEvent, 0, m.config.BatchSize)
	ticker := time.NewTicker(m.config.BatchInterval)
	defer ticker.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		for _, event := range batch {
			if m.companionManager != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = m.companionManager.EmitEvent(ctx, event)
				cancel()
				atomic.AddInt64(&m.stats.EventsProcessed, 1)
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-m.done:
			// Drain remaining events
			flushBatch()
			for {
				select {
				case event := <-m.eventQueue:
					if m.companionManager != nil {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						_ = m.companionManager.EmitEvent(ctx, event)
						cancel()
						atomic.AddInt64(&m.stats.EventsProcessed, 1)
					}
				default:
					return
				}
			}
		case event := <-m.eventQueue:
			batch = append(batch, event)
			if len(batch) >= m.config.BatchSize {
				flushBatch()
			}
		case <-ticker.C:
			flushBatch()
		}
	}
}

// cleanupLoop periodically cleans up inactive sessions.
func (m *BotSessionManager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.SessionTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.cleanupInactiveSessions()
		}
	}
}

// cleanupInactiveSessions removes sessions that have been inactive for too long.
func (m *BotSessionManager) cleanupInactiveSessions() {
	if m.companionManager == nil {
		return
	}

	now := time.Now()
	var toRemove []string

	m.sessionsMu.RLock()
	for key, info := range m.sessions {
		if now.Sub(info.LastActiveAt) > m.config.SessionTimeout {
			toRemove = append(toRemove, key)
		}
	}
	m.sessionsMu.RUnlock()

	for _, key := range toRemove {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = m.EndSession(ctx, key)
		cancel()
	}
}

// truncateContent truncates content to maxLen.
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen]
}
