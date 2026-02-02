package session

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	ctxpkg "github.com/IceWhaleTech/ZimaOS-Echo/server/internal/context"
)

// SessionManager manages sessions with isolation and persistence.
type SessionManager struct {
	sessions    map[string]*Session
	store       SessionStore
	compactor   *SessionCompactor
	hookManager *HookManager
	config      config.SessionConfig
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(cfg config.SessionConfig, store SessionStore, compactor *SessionCompactor) *SessionManager {
	mgr := &SessionManager{
		sessions:    make(map[string]*Session),
		store:       store,
		compactor:   compactor,
		hookManager: NewHookManager(),
		config:      cfg,
		stopCh:      make(chan struct{}),
	}

	// Start background tasks
	if cfg.Compaction.AutoCompact {
		mgr.wg.Add(1)
		go mgr.autoCompactLoop()
	}
	if cfg.Cleanup.Enabled {
		mgr.wg.Add(1)
		go mgr.cleanupLoop()
	}
	if cfg.Persistence.Enabled && cfg.Persistence.Interval > 0 {
		mgr.wg.Add(1)
		go mgr.persistLoop()
	}

	return mgr
}

// GetOrCreate gets an existing session or creates a new one.
func (m *SessionManager) GetOrCreate(id SessionID) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := id.String()

	// Check in-memory cache
	if session, ok := m.sessions[key]; ok {
		return session, nil
	}

	// Try to load from store
	if m.store != nil {
		session, err := m.store.Load(id)
		if err != nil {
			return nil, fmt.Errorf("failed to load session: %w", err)
		}
		if session != nil {
			m.sessions[key] = session
			return session, nil
		}
	}

	// Create new session
	session := NewSession(id, m.config.MaxTokens)
	m.sessions[key] = session

	// Persist if enabled
	if m.store != nil && m.config.Persistence.Enabled {
		if err := m.store.Save(session); err != nil {
			// Log error but don't fail
			log.Printf("[WARN] failed to persist new session: %v", err)
		}
	}

	return session, nil
}

// Get gets a session by ID.
func (m *SessionManager) Get(id SessionID) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id.String()]
	return session, ok
}

// AddMessage adds a message to a session.
func (m *SessionManager) AddMessage(id SessionID, msg ctxpkg.Message) error {
	session, err := m.GetOrCreate(id)
	if err != nil {
		return err
	}

	session.AddMessage(msg)

	// Check if compaction is needed
	if m.compactor != nil && m.compactor.ShouldCompact(session) {
		go func() {
			if _, err := m.Compact(id); err != nil {
				log.Printf("[WARN] auto-compaction failed: %v", err)
			}
		}()
	}

	// Persist if enabled
	if m.store != nil && m.config.Persistence.OnMessage {
		if err := m.store.Save(session); err != nil {
			return fmt.Errorf("failed to persist session: %w", err)
		}
	}

	return nil
}

// GetMessages gets all messages from a session.
func (m *SessionManager) GetMessages(id SessionID) ([]ctxpkg.Message, error) {
	session, err := m.GetOrCreate(id)
	if err != nil {
		return nil, err
	}
	return session.GetMessages(), nil
}

// Compact compacts a session.
func (m *SessionManager) Compact(id SessionID) (*CompactionResult, error) {
	m.mu.Lock()
	session, ok := m.sessions[id.String()]
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", id.String())
	}

	if m.compactor == nil {
		return nil, fmt.Errorf("compactor not configured")
	}

	result, err := m.compactor.Compact(session)
	if err != nil {
		return nil, err
	}

	// Persist if enabled
	if m.store != nil && m.config.Persistence.OnCompact {
		if err := m.store.Save(session); err != nil {
			return result, fmt.Errorf("failed to persist compacted session: %w", err)
		}
	}

	return result, nil
}

// NewSession starts a new session, saving the current one to memory first.
// This is triggered by the /new command.
func (m *SessionManager) NewSession(id SessionID) (*Session, error) {
	m.mu.Lock()
	oldSession, exists := m.sessions[id.String()]
	m.mu.Unlock()

	// Trigger hooks for the old session before creating new one
	if exists && m.hookManager != nil {
		m.hookManager.TriggerSessionEnd(context.Background(), oldSession, EndReasonNew)
	}

	// Clear the old session
	if exists {
		oldSession.Clear()
	}

	// Get or create the session (will be cleared if existed)
	session, err := m.GetOrCreate(id)
	if err != nil {
		return nil, err
	}

	// Persist if enabled
	if m.store != nil && m.config.Persistence.Enabled {
		if err := m.store.Save(session); err != nil {
			log.Printf("[WARN] failed to persist new session: %v", err)
		}
	}

	return session, nil
}

// Reset resets a session (clears messages but keeps system prompt).
func (m *SessionManager) Reset(id SessionID) error {
	m.mu.Lock()
	session, ok := m.sessions[id.String()]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id.String())
	}

	// Trigger hooks before clearing
	if m.hookManager != nil {
		m.hookManager.TriggerSessionEnd(context.Background(), session, EndReasonReset)
	}

	session.Clear()

	// Persist if enabled
	if m.store != nil && m.config.Persistence.Enabled {
		if err := m.store.Save(session); err != nil {
			return fmt.Errorf("failed to persist reset session: %w", err)
		}
	}

	return nil
}

// Archive archives a session.
func (m *SessionManager) Archive(id SessionID) error {
	m.mu.Lock()
	session, ok := m.sessions[id.String()]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id.String())
	}

	// Trigger hooks before archiving
	if m.hookManager != nil {
		m.hookManager.TriggerSessionEnd(context.Background(), session, EndReasonArchive)
	}

	m.mu.Lock()
	session.SetState(SessionStateArchived)
	delete(m.sessions, id.String())
	m.mu.Unlock()

	if m.store != nil {
		return m.store.Archive(id)
	}
	return nil
}

// Delete deletes a session.
func (m *SessionManager) Delete(id SessionID) error {
	m.mu.Lock()
	session, ok := m.sessions[id.String()]
	m.mu.Unlock()

	// Trigger hooks before deleting (if session exists)
	if ok && m.hookManager != nil {
		m.hookManager.TriggerSessionEnd(context.Background(), session, EndReasonDelete)
	}

	m.mu.Lock()
	delete(m.sessions, id.String())
	m.mu.Unlock()

	if m.store != nil {
		return m.store.Delete(id)
	}
	return nil
}

// RegisterHook registers a session hook.
func (m *SessionManager) RegisterHook(hook SessionHook) {
	if m.hookManager != nil {
		m.hookManager.Register(hook)
	}
}

// List lists sessions matching the filter.
func (m *SessionManager) List(filter SessionFilter) ([]*Session, error) {
	if m.store != nil {
		return m.store.List(filter)
	}

	// Fall back to in-memory list
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0)
	for _, session := range m.sessions {
		if filter.AgentID != "" && session.ID.AgentID != filter.AgentID {
			continue
		}
		if filter.ChannelID != "" && session.ID.ChannelID != filter.ChannelID {
			continue
		}
		if filter.PeerID != "" && session.ID.PeerID != filter.PeerID {
			continue
		}
		if filter.State != nil && session.State != *filter.State {
			continue
		}
		result = append(result, session)
	}

	return result, nil
}

// Stats returns session manager statistics.
func (m *SessionManager) Stats() SessionManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := SessionManagerStats{
		TotalSessions: len(m.sessions),
	}

	for _, session := range m.sessions {
		switch session.GetState() {
		case SessionStateActive:
			stats.ActiveSessions++
		case SessionStateIdle:
			stats.IdleSessions++
		case SessionStateArchived:
			stats.ArchivedSessions++
		}
		stats.TotalMessages += session.Metadata.MessageCount
		stats.TotalTokens += session.TokenCount()
	}

	return stats
}

// SessionManagerStats holds session manager statistics.
type SessionManagerStats struct {
	TotalSessions    int   `json:"total_sessions"`
	ActiveSessions   int   `json:"active_sessions"`
	IdleSessions     int   `json:"idle_sessions"`
	ArchivedSessions int   `json:"archived_sessions"`
	TotalMessages    int   `json:"total_messages"`
	TotalTokens      int   `json:"total_tokens"`
}

// Recover recovers sessions from persistence.
func (m *SessionManager) Recover() error {
	if m.store == nil {
		return nil
	}

	// Load active and idle sessions
	activeState := SessionStateActive
	sessions, err := m.store.List(SessionFilter{State: &activeState, Limit: 1000})
	if err != nil {
		return fmt.Errorf("failed to recover active sessions: %w", err)
	}

	m.mu.Lock()
	for _, session := range sessions {
		m.sessions[session.ID.String()] = session
	}
	m.mu.Unlock()

	return nil
}

// Stop stops the session manager.
func (m *SessionManager) Stop() error {
	close(m.stopCh)
	m.wg.Wait()

	// Persist all sessions
	if m.store != nil && m.config.Persistence.Enabled {
		m.mu.RLock()
		for _, session := range m.sessions {
			if err := m.store.Save(session); err != nil {
				log.Printf("[WARN] failed to persist session on stop: %v", err)
			}
		}
		m.mu.RUnlock()
	}

	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

// autoCompactLoop runs auto-compaction periodically.
func (m *SessionManager) autoCompactLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.Compaction.AutoCompactInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runAutoCompaction()
		}
	}
}

// runAutoCompaction compacts sessions that need it.
func (m *SessionManager) runAutoCompaction() {
	if m.compactor == nil {
		return
	}

	m.mu.RLock()
	sessionsToCompact := make([]*Session, 0)
	for _, session := range m.sessions {
		if m.compactor.ShouldCompact(session) {
			sessionsToCompact = append(sessionsToCompact, session)
		}
	}
	m.mu.RUnlock()

	for _, session := range sessionsToCompact {
		if _, err := m.Compact(session.ID); err != nil {
			log.Printf("[WARN] auto-compaction failed for %s: %v", session.ID.String(), err)
		}
	}
}

// cleanupLoop runs cleanup periodically.
func (m *SessionManager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.Cleanup.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runCleanup()
		}
	}
}

// runCleanup archives and deletes old sessions.
func (m *SessionManager) runCleanup() {
	now := time.Now()
	archiveThreshold := now.Add(-m.config.Cleanup.ArchiveAfter)
	deleteThreshold := now.Add(-m.config.Cleanup.DeleteAfter)

	m.mu.Lock()
	toArchive := make([]SessionID, 0)
	toDelete := make([]SessionID, 0)

	for _, session := range m.sessions {
		if session.State == SessionStateArchived && session.UpdatedAt.Before(deleteThreshold) {
			toDelete = append(toDelete, session.ID)
		} else if session.State != SessionStateArchived && session.LastActiveAt.Before(archiveThreshold) {
			toArchive = append(toArchive, session.ID)
		}
	}
	m.mu.Unlock()

	for _, id := range toArchive {
		if err := m.Archive(id); err != nil {
			log.Printf("[WARN] failed to archive session %s: %v", id.String(), err)
		}
	}

	for _, id := range toDelete {
		if err := m.Delete(id); err != nil {
			log.Printf("[WARN] failed to delete session %s: %v", id.String(), err)
		}
	}
}

// persistLoop runs persistence periodically.
func (m *SessionManager) persistLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.Persistence.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runPersist()
		}
	}
}

// runPersist persists all sessions.
func (m *SessionManager) runPersist() {
	if m.store == nil {
		return
	}

	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()

	for _, session := range sessions {
		if err := m.store.Save(session); err != nil {
			log.Printf("[WARN] failed to persist session %s: %v", session.ID.String(), err)
		}
	}
}
