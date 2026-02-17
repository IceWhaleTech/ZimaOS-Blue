package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SessionID uniquely identifies a session.
type SessionID struct {
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id"`
	PeerID    string `json:"peer_id"`
	ThreadID  string `json:"thread_id,omitempty"`
}

// String returns the string representation of SessionID.
func (s SessionID) String() string {
	parts := []string{s.AgentID, s.ChannelID, s.PeerID}
	if s.ThreadID != "" {
		parts = append(parts, s.ThreadID)
	}
	return strings.Join(parts, ":")
}

// ParseSessionID parses a session ID string.
func ParseSessionID(s string) (SessionID, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return SessionID{}, fmt.Errorf("invalid session ID format: %s", s)
	}
	id := SessionID{
		AgentID:   parts[0],
		ChannelID: parts[1],
		PeerID:    parts[2],
	}
	if len(parts) > 3 {
		id.ThreadID = parts[3]
	}
	return id, nil
}

// SessionState represents session lifecycle state.
type SessionState int

const (
	SessionStateActive SessionState = iota
	SessionStateIdle
	SessionStateCompacting
	SessionStateSuspended
	SessionStateArchived
)

// String returns the string representation of SessionState.
func (s SessionState) String() string {
	switch s {
	case SessionStateActive:
		return "active"
	case SessionStateIdle:
		return "idle"
	case SessionStateCompacting:
		return "compacting"
	case SessionStateSuspended:
		return "suspended"
	case SessionStateArchived:
		return "archived"
	default:
		return "unknown"
	}
}

// SessionMetadata holds session metadata.
type SessionMetadata struct {
	Title           string            `json:"title,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	CustomData      map[string]string `json:"custom_data,omitempty"`
	MessageCount    int               `json:"message_count"`
	TokenCount      int               `json:"token_count"`
	CompactionCount int               `json:"compaction_count"`
}

// Session represents a conversation session.
type Session struct {
	ID           SessionID                    `json:"id"`
	Context      *context.ConversationContext `json:"-"`
	Metadata     SessionMetadata              `json:"metadata"`
	State        SessionState                 `json:"state"`
	CreatedAt    time.Time                    `json:"created_at"`
	UpdatedAt    time.Time                    `json:"updated_at"`
	LastActiveAt time.Time                    `json:"last_active_at"`
	CompactedAt  *time.Time                   `json:"compacted_at,omitempty"`

	mu sync.RWMutex
}

// NewSession creates a new session.
func NewSession(id SessionID, maxTokens int) *Session {
	now := timeutil.NowTime()
	return &Session{
		ID:           id,
		Context:      context.NewConversationContext(maxTokens),
		Metadata:     SessionMetadata{},
		State:        SessionStateActive,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg context.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Context.AddMessage(msg)
	s.Metadata.MessageCount++
	s.Metadata.TokenCount = s.Context.TotalTokens()
	s.LastActiveAt = timeutil.NowTime()
	s.UpdatedAt = timeutil.NowTime()
	s.State = SessionStateActive
}

// GetMessages returns all messages in the session.
func (s *Session) GetMessages() []context.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Context.Messages()
}

// SetSystemPrompt sets the system prompt.
func (s *Session) SetSystemPrompt(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Context.SetSystemPrompt(prompt)
	s.UpdatedAt = timeutil.NowTime()
}

// Clear clears all messages but keeps the system prompt.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Context.ClearKeepSystem()
	s.Metadata.MessageCount = 0
	s.Metadata.TokenCount = s.Context.TotalTokens()
	s.UpdatedAt = timeutil.NowTime()
}

// SetState sets the session state.
func (s *Session) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
	s.UpdatedAt = timeutil.NowTime()
}

// GetState returns the session state.
func (s *Session) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// IsActive returns true if the session is active.
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State == SessionStateActive
}

// TokenCount returns the current token count.
func (s *Session) TokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Context.TotalTokens()
}

// MaxTokens returns the maximum token limit.
func (s *Session) MaxTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Context.MaxTokens()
}

// TokenUsageRatio returns the ratio of used tokens to max tokens.
func (s *Session) TokenUsageRatio() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	max := s.Context.MaxTokens()
	if max == 0 {
		return 0
	}
	return float64(s.Context.TotalTokens()) / float64(max)
}

// SetSummary sets the session summary (from compaction).
func (s *Session) SetSummary(summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Metadata.Summary = summary
	now := timeutil.NowTime()
	s.CompactedAt = &now
	s.Metadata.CompactionCount++
	s.UpdatedAt = now
}

// GetSummary returns the session summary.
func (s *Session) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Metadata.Summary
}

// SetTitle sets the session title.
func (s *Session) SetTitle(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Metadata.Title = title
	s.UpdatedAt = timeutil.NowTime()
}

// GetTitle returns the session title.
func (s *Session) GetTitle() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Metadata.Title
}

// AddTag adds a tag to the session.
func (s *Session) AddTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.Metadata.Tags {
		if t == tag {
			return
		}
	}
	s.Metadata.Tags = append(s.Metadata.Tags, tag)
	s.UpdatedAt = timeutil.NowTime()
}

// RemoveTag removes a tag from the session.
func (s *Session) RemoveTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.Metadata.Tags {
		if t == tag {
			s.Metadata.Tags = append(s.Metadata.Tags[:i], s.Metadata.Tags[i+1:]...)
			s.UpdatedAt = timeutil.NowTime()
			return
		}
	}
}

// GetTags returns all tags.
func (s *Session) GetTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tags := make([]string, len(s.Metadata.Tags))
	copy(tags, s.Metadata.Tags)
	return tags
}

// SetCustomData sets custom data.
func (s *Session) SetCustomData(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Metadata.CustomData == nil {
		s.Metadata.CustomData = make(map[string]string)
	}
	s.Metadata.CustomData[key] = value
	s.UpdatedAt = timeutil.NowTime()
}

// GetCustomData returns custom data.
func (s *Session) GetCustomData(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Metadata.CustomData == nil {
		return "", false
	}
	v, ok := s.Metadata.CustomData[key]
	return v, ok
}

// SessionInfo represents session information for API responses.
type SessionInfo struct {
	ID           string          `json:"id"`
	AgentID      string          `json:"agent_id"`
	ChannelID    string          `json:"channel_id"`
	PeerID       string          `json:"peer_id"`
	ThreadID     string          `json:"thread_id,omitempty"`
	State        string          `json:"state"`
	Title        string          `json:"title,omitempty"`
	Summary      string          `json:"summary,omitempty"`
	MessageCount int             `json:"message_count"`
	TokenCount   int             `json:"token_count"`
	MaxTokens    int             `json:"max_tokens"`
	TokenUsage   float64         `json:"token_usage"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	LastActiveAt time.Time       `json:"last_active_at"`
	CompactedAt  *time.Time      `json:"compacted_at,omitempty"`
}

// Info returns session information.
func (s *Session) Info() SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionInfo{
		ID:           s.ID.String(),
		AgentID:      s.ID.AgentID,
		ChannelID:    s.ID.ChannelID,
		PeerID:       s.ID.PeerID,
		ThreadID:     s.ID.ThreadID,
		State:        s.State.String(),
		Title:        s.Metadata.Title,
		Summary:      s.Metadata.Summary,
		MessageCount: s.Metadata.MessageCount,
		TokenCount:   s.Context.TotalTokens(),
		MaxTokens:    s.Context.MaxTokens(),
		TokenUsage:   s.TokenUsageRatio(),
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		LastActiveAt: s.LastActiveAt,
		CompactedAt:  s.CompactedAt,
	}
}

// SessionFilter for querying sessions.
type SessionFilter struct {
	AgentID       string
	ChannelID     string
	PeerID        string
	State         *SessionState
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Limit         int
	Offset        int
}
