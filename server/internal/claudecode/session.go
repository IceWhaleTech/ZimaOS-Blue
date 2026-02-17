package claudecode

import (
	"context"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// CliSession represents a Claude Code CLI session.
type CliSession struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// AgentID is the agent this session belongs to.
	AgentID string `json:"agent_id,omitempty"`

	// ChannelID is the channel this session belongs to.
	ChannelID string `json:"channel_id,omitempty"`

	// WorkspaceDir is the working directory for this session.
	WorkspaceDir string `json:"workspace_dir,omitempty"`

	// Model is the model used for this session.
	Model string `json:"model,omitempty"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// LastUsedAt is when the session was last used.
	LastUsedAt time.Time `json:"last_used_at"`

	// MessageCount is the number of messages in this session.
	MessageCount int `json:"message_count"`

	// Metadata contains additional session metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SessionStore manages CLI sessions.
type SessionStore interface {
	// Create creates a new session.
	Create(ctx context.Context, session *CliSession) error

	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*CliSession, error)

	// Update updates an existing session.
	Update(ctx context.Context, session *CliSession) error

	// Delete deletes a session by ID.
	Delete(ctx context.Context, id string) error

	// List lists all sessions with optional filters.
	List(ctx context.Context, filter *SessionFilter) ([]*CliSession, error)

	// CleanupExpired removes sessions older than the TTL.
	CleanupExpired(ctx context.Context, ttl time.Duration) (int, error)
}

// SessionFilter contains filters for listing sessions.
type SessionFilter struct {
	AgentID   string
	ChannelID string
	Model     string
	Limit     int
	Offset    int
}

// InMemorySessionStore is an in-memory implementation of SessionStore.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*CliSession
}

// NewInMemorySessionStore creates a new in-memory session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*CliSession),
	}
}

// Create creates a new session.
func (s *InMemorySessionStore) Create(ctx context.Context, session *CliSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = timeutil.NowTime()
	}
	if session.LastUsedAt.IsZero() {
		session.LastUsedAt = session.CreatedAt
	}

	s.sessions[session.ID] = session
	return nil
}

// Get retrieves a session by ID.
func (s *InMemorySessionStore) Get(ctx context.Context, id string) (*CliSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound{SessionId: id}
	}

	// Return a copy to prevent mutation
	copy := *session
	return &copy, nil
}

// Update updates an existing session.
func (s *InMemorySessionStore) Update(ctx context.Context, session *CliSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[session.ID]; !ok {
		return ErrSessionNotFound{SessionId: session.ID}
	}

	session.LastUsedAt = timeutil.NowTime()
	s.sessions[session.ID] = session
	return nil
}

// Delete deletes a session by ID.
func (s *InMemorySessionStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return ErrSessionNotFound{SessionId: id}
	}

	delete(s.sessions, id)
	return nil
}

// List lists all sessions with optional filters.
func (s *InMemorySessionStore) List(ctx context.Context, filter *SessionFilter) ([]*CliSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*CliSession

	for _, session := range s.sessions {
		// Apply filters
		if filter != nil {
			if filter.AgentID != "" && session.AgentID != filter.AgentID {
				continue
			}
			if filter.ChannelID != "" && session.ChannelID != filter.ChannelID {
				continue
			}
			if filter.Model != "" && session.Model != filter.Model {
				continue
			}
		}

		// Return a copy
		copy := *session
		result = append(result, &copy)
	}

	// Apply pagination
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(result) {
			result = result[filter.Offset:]
		} else if filter.Offset >= len(result) {
			return []*CliSession{}, nil
		}

		if filter.Limit > 0 && filter.Limit < len(result) {
			result = result[:filter.Limit]
		}
	}

	return result, nil
}

// CleanupExpired removes sessions older than the TTL.
func (s *InMemorySessionStore) CleanupExpired(ctx context.Context, ttl time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := timeutil.NowTime().Add(-ttl)
	count := 0

	for id, session := range s.sessions {
		if session.LastUsedAt.Before(cutoff) {
			delete(s.sessions, id)
			count++
		}
	}

	return count, nil
}

// SessionManager manages CLI sessions with automatic cleanup.
type SessionManager struct {
	store    SessionStore
	ttl      time.Duration
	stopCh   chan struct{}
	interval time.Duration
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(store SessionStore, ttl time.Duration, cleanupInterval time.Duration) *SessionManager {
	if cleanupInterval == 0 {
		cleanupInterval = 1 * time.Hour
	}
	return &SessionManager{
		store:    store,
		ttl:      ttl,
		stopCh:   make(chan struct{}),
		interval: cleanupInterval,
	}
}

// Start begins the background cleanup routine.
func (m *SessionManager) Start() {
	go m.cleanupLoop()
}

// Stop stops the background cleanup routine.
func (m *SessionManager) Stop() {
	close(m.stopCh)
}

// cleanupLoop runs the periodic cleanup.
func (m *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			m.store.CleanupExpired(ctx, m.ttl)
			cancel()
		case <-m.stopCh:
			return
		}
	}
}

// GetOrCreate gets an existing session or creates a new one.
func (m *SessionManager) GetOrCreate(ctx context.Context, id string, defaults *CliSession) (*CliSession, error) {
	if id != "" {
		session, err := m.store.Get(ctx, id)
		if err == nil {
			return session, nil
		}
		// If not found, fall through to create
		if _, ok := err.(ErrSessionNotFound); !ok {
			return nil, err
		}
	}

	// Create new session
	session := &CliSession{
		ID:           uuid.New().String(),
		CreatedAt:    timeutil.NowTime(),
		LastUsedAt:   timeutil.NowTime(),
		MessageCount: 0,
	}

	// Apply defaults
	if defaults != nil {
		if defaults.AgentID != "" {
			session.AgentID = defaults.AgentID
		}
		if defaults.ChannelID != "" {
			session.ChannelID = defaults.ChannelID
		}
		if defaults.WorkspaceDir != "" {
			session.WorkspaceDir = defaults.WorkspaceDir
		}
		if defaults.Model != "" {
			session.Model = defaults.Model
		}
		if defaults.Metadata != nil {
			session.Metadata = defaults.Metadata
		}
	}

	if err := m.store.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// Touch updates the last used time and increments message count.
func (m *SessionManager) Touch(ctx context.Context, id string) error {
	session, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}

	session.LastUsedAt = timeutil.NowTime()
	session.MessageCount++

	return m.store.Update(ctx, session)
}

// Store returns the underlying session store.
func (m *SessionManager) Store() SessionStore {
	return m.store
}
