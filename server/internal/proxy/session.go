package proxy

import (
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// SessionStatus represents the status of a session
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusTimeout   SessionStatus = "timeout"
)

// Session represents an API session
type Session struct {
	ID           string        `json:"id"`
	StartTime    time.Time     `json:"start_time"`
	LastActivity time.Time     `json:"last_activity"`
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	TokensIn     int64         `json:"tokens_in"`
	TokensOut    int64         `json:"tokens_out"`
	Status       SessionStatus `json:"status"`
	RequestCount int           `json:"request_count"`
	ErrorCount   int           `json:"error_count"`
	ClientIP     string        `json:"client_ip"`
	UserAgent    string        `json:"user_agent"`
}

// SessionMonitor manages API sessions
type SessionMonitor struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	config   *SessionConfig
}

// SessionConfig holds session monitoring configuration
type SessionConfig struct {
	Enabled        bool          `json:"enabled"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	MaxSessions    int           `json:"max_sessions"`
	CleanupPeriod  time.Duration `json:"cleanup_period"`
	RetainComplete time.Duration `json:"retain_complete"`
}

// DefaultSessionConfig returns default session configuration
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		Enabled:        true,
		IdleTimeout:    30 * time.Minute,
		MaxSessions:    10000,
		CleanupPeriod:  5 * time.Minute,
		RetainComplete: 1 * time.Hour,
	}
}

// NewSessionMonitor creates a new session monitor
func NewSessionMonitor(config *SessionConfig) *SessionMonitor {
	if config == nil {
		config = DefaultSessionConfig()
	}
	return &SessionMonitor{
		sessions: make(map[string]*Session),
		config:   config,
	}
}

// StartSession creates a new session
func (sm *SessionMonitor) StartSession(clientIP, userAgent string) *Session {
	if !sm.config.Enabled {
		return nil
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check max sessions limit
	if len(sm.sessions) >= sm.config.MaxSessions {
		sm.cleanupOldestSessions(sm.config.MaxSessions / 10)
	}

	session := &Session{
		ID:           uuid.New().String(),
		StartTime:    timeutil.NowTime(),
		LastActivity: timeutil.NowTime(),
		Status:       SessionStatusActive,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
	}

	sm.sessions[session.ID] = session
	return session
}

// UpdateSession updates session with request data
func (sm *SessionMonitor) UpdateSession(id string, provider, model string, tokensIn, tokensOut int64) {
	if !sm.config.Enabled {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return
	}

	session.LastActivity = timeutil.NowTime()
	session.Provider = provider
	session.Model = model
	session.TokensIn += tokensIn
	session.TokensOut += tokensOut
	session.RequestCount++
}

// RecordError records an error for a session
func (sm *SessionMonitor) RecordError(id string) {
	if !sm.config.Enabled {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[id]; ok {
		session.ErrorCount++
		session.LastActivity = timeutil.NowTime()
	}
}

// CompleteSession marks a session as completed
func (sm *SessionMonitor) CompleteSession(id string, status SessionStatus) {
	if !sm.config.Enabled {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[id]; ok {
		session.Status = status
		session.LastActivity = timeutil.NowTime()
	}
}

// GetSession returns a session by ID
func (sm *SessionMonitor) GetSession(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[id]
	if !ok {
		return nil, false
	}

	// Return a copy to prevent race conditions
	copy := *session
	return &copy, true
}

// ListSessions returns all sessions
func (sm *SessionMonitor) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		copy := *s
		sessions = append(sessions, &copy)
	}
	return sessions
}

// ListActiveSessions returns only active sessions
func (sm *SessionMonitor) ListActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, s := range sm.sessions {
		if s.Status == SessionStatusActive {
			copy := *s
			sessions = append(sessions, &copy)
		}
	}
	return sessions
}

// CleanupIdleSessions removes idle sessions
func (sm *SessionMonitor) CleanupIdleSessions() int {
	if !sm.config.Enabled {
		return 0
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := timeutil.NowTime()
	removed := 0

	for id, session := range sm.sessions {
		shouldRemove := false

		if session.Status == SessionStatusActive {
			// Remove active sessions that have been idle too long
			if now.Sub(session.LastActivity) > sm.config.IdleTimeout {
				session.Status = SessionStatusTimeout
				shouldRemove = true
			}
		} else {
			// Remove completed/failed sessions after retention period
			if now.Sub(session.LastActivity) > sm.config.RetainComplete {
				shouldRemove = true
			}
		}

		if shouldRemove {
			delete(sm.sessions, id)
			removed++
		}
	}

	return removed
}

// cleanupOldestSessions removes the oldest sessions (internal, no lock)
func (sm *SessionMonitor) cleanupOldestSessions(count int) {
	if count <= 0 || len(sm.sessions) == 0 {
		return
	}

	// Find oldest sessions
	type sessionAge struct {
		id   string
		time time.Time
	}

	ages := make([]sessionAge, 0, len(sm.sessions))
	for id, s := range sm.sessions {
		ages = append(ages, sessionAge{id: id, time: s.LastActivity})
	}

	// Sort by last activity (oldest first)
	for i := 0; i < len(ages)-1; i++ {
		for j := i + 1; j < len(ages); j++ {
			if ages[j].time.Before(ages[i].time) {
				ages[i], ages[j] = ages[j], ages[i]
			}
		}
	}

	// Remove oldest
	for i := 0; i < count && i < len(ages); i++ {
		delete(sm.sessions, ages[i].id)
	}
}

// Stats returns session statistics
func (sm *SessionMonitor) Stats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	active := 0
	completed := 0
	failed := 0
	totalTokensIn := int64(0)
	totalTokensOut := int64(0)
	totalRequests := 0

	for _, s := range sm.sessions {
		switch s.Status {
		case SessionStatusActive:
			active++
		case SessionStatusCompleted:
			completed++
		case SessionStatusFailed, SessionStatusTimeout:
			failed++
		}
		totalTokensIn += s.TokensIn
		totalTokensOut += s.TokensOut
		totalRequests += s.RequestCount
	}

	return map[string]interface{}{
		"enabled":          sm.config.Enabled,
		"total_sessions":   len(sm.sessions),
		"active_sessions":  active,
		"completed":        completed,
		"failed":           failed,
		"total_tokens_in":  totalTokensIn,
		"total_tokens_out": totalTokensOut,
		"total_requests":   totalRequests,
	}
}
