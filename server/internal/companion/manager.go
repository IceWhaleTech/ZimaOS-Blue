package companion

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Manager handles session lifecycle and event management.
type Manager struct {
	storage  Storage
	streamer Streamer
	config   *Config
	mu       sync.RWMutex

	// Active sessions cache
	activeSessions map[string]*Session
}

// NewManager creates a new session manager.
func NewManager(storage Storage, streamer Streamer, config *Config) *Manager {
	return &Manager{
		storage:        storage,
		streamer:       streamer,
		config:         config,
		activeSessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session.
func (m *Manager) CreateSession(ctx context.Context, session *Session) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check concurrent session limit
	if len(m.activeSessions) >= m.config.Performance.MaxConcurrentSessions {
		// Find and close oldest inactive session
		m.cleanupOldestSession(ctx)
	}

	// Generate ID if not provided
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	// Set defaults
	session.Status = SessionStatusActive
	session.StartedAt = timeutil.NowTime()
	session.ThreatLevel = ThreatLevelNone
	session.ThreatScore = 0
	session.EventCount = 0

	// Save to storage
	if err := m.storage.SaveSession(ctx, session); err != nil {
		return nil, err
	}

	// Add to active sessions
	m.activeSessions[session.ID] = session

	// Emit session start event
	event := &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Timestamp: session.StartedAt,
		EventType: EventSessionStart,
		Platform:  session.Platform,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Status:    "success",
	}
	m.streamer.Emit(event)

	return session, nil
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(ctx context.Context, id string) (*Session, error) {
	m.mu.RLock()
	if session, ok := m.activeSessions[id]; ok {
		m.mu.RUnlock()
		return session, nil
	}
	m.mu.RUnlock()

	return m.storage.GetSession(ctx, id)
}

// ListSessions lists sessions with filtering.
func (m *Manager) ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, int, error) {
	return m.storage.ListSessions(ctx, opts)
}

// EndSession ends an active session.
func (m *Manager) EndSession(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.endSessionLocked(ctx, id)
}

func (m *Manager) endSessionLocked(ctx context.Context, id string) error {
	session, ok := m.activeSessions[id]
	if !ok {
		// Try to get from storage
		var err error
		session, err = m.storage.GetSession(ctx, id)
		if err != nil {
			return err
		}
	}

	if session.Status == SessionStatusEnded {
		return ErrSessionClosed
	}

	// Update session
	now := timeutil.NowTime()
	session.Status = SessionStatusEnded
	session.EndedAt = &now
	session.Duration = FromDuration(now.Sub(session.StartedAt))

	// Update storage
	if err := m.storage.UpdateSession(ctx, session); err != nil {
		return err
	}

	// Remove from active sessions
	delete(m.activeSessions, id)

	// Emit session end event
	event := &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Timestamp: now,
		EventType: EventSessionEnd,
		Platform:  session.Platform,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Duration:  session.Duration,
		Status:    "success",
	}
	m.streamer.Emit(event)

	return nil
}

// EmitEvent emits an event for a session.
func (m *Manager) EmitEvent(ctx context.Context, event *SessionEvent) error {
	// Generate ID if not provided
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = timeutil.NowTime()
	}

	// Get session
	m.mu.RLock()
	session, ok := m.activeSessions[event.SessionID]
	m.mu.RUnlock()

	if !ok {
		var err error
		session, err = m.storage.GetSession(ctx, event.SessionID)
		if err != nil {
			return err
		}
	}

	// Update session metadata based on event type
	m.updateSessionFromEvent(session, event)

	// Save event to storage
	if err := m.storage.AppendEvent(ctx, event); err != nil {
		return err
	}

	// Update session in storage
	if err := m.storage.UpdateSession(ctx, session); err != nil {
		return err
	}

	// Emit to streamer
	m.streamer.Emit(event)

	return nil
}

// GetSessionEvents retrieves events for a session.
func (m *Manager) GetSessionEvents(ctx context.Context, sessionID string, opts *ListOptions) ([]*SessionEvent, int, error) {
	return m.storage.GetSessionEvents(ctx, sessionID, opts)
}

// GetSessionFlow builds the flow graph for a session.
func (m *Manager) GetSessionFlow(ctx context.Context, sessionID string) (*FlowGraph, error) {
	events, _, err := m.storage.GetSessionEvents(ctx, sessionID, nil)
	if err != nil {
		return nil, err
	}

	return m.buildFlowGraph(sessionID, events), nil
}

// GetStats returns current statistics.
func (m *Manager) GetStats(ctx context.Context) (*Stats, error) {
	m.mu.RLock()
	activeCount := len(m.activeSessions)
	m.mu.RUnlock()

	// Get all sessions for stats
	sessions, total, err := m.storage.ListSessions(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Get alerts
	alerts, alertTotal, err := m.storage.ListAlerts(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Calculate stats
	stats := &Stats{
		ActiveSessions:     activeCount,
		TotalSessions:      total,
		TotalAlerts:        alertTotal,
		SessionsByPlatform: make(map[Platform]int),
		ThreatsByLevel:     make(map[ThreatLevel]int),
		EventsByType:       make(map[SessionEventType]int),
		LastUpdated:        timeutil.NowTime(),
	}

	var totalDuration time.Duration
	for _, session := range sessions {
		stats.SessionsByPlatform[session.Platform]++
		stats.ThreatsByLevel[session.ThreatLevel]++
		stats.TotalEvents += session.EventCount
		if session.Duration > 0 {
			totalDuration += time.Duration(session.Duration)
		}
	}

	if total > 0 {
		stats.AvgSessionDuration = FromDuration(totalDuration / time.Duration(total))
	}

	// Count unacknowledged alerts
	for _, alert := range alerts {
		if !alert.Acknowledged {
			stats.UnackedAlerts++
		}
	}

	return stats, nil
}

// GetActiveSessions returns all active sessions.
func (m *Manager) GetActiveSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.activeSessions))
	for _, session := range m.activeSessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// CleanupExpired removes expired data.
func (m *Manager) CleanupExpired(ctx context.Context) error {
	return m.storage.CleanupExpired(ctx, &m.config.Retention)
}

// Helper methods

func (m *Manager) updateSessionFromEvent(session *Session, event *SessionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session.EventCount++

	switch event.EventType {
	case EventMessageReceived, EventMessageSent:
		session.Metadata.MessageCount++
	case EventToolCall:
		session.Metadata.ToolCallCount++
	case EventLLMRequest:
		session.Metadata.LLMCallCount++
		if event.LLMRequest != nil {
			session.Metadata.TotalTokens += event.LLMRequest.TotalTokens
		}
	case EventSecurityThreat:
		if event.Security != nil {
			// Update threat level if higher
			if threatLevelPriority(event.Security.ThreatLevel) > threatLevelPriority(session.ThreatLevel) {
				session.ThreatLevel = event.Security.ThreatLevel
			}
			if event.Security.ThreatScore > session.ThreatScore {
				session.ThreatScore = event.Security.ThreatScore
			}
		}
	}
}

func (m *Manager) buildFlowGraph(sessionID string, events []*SessionEvent) *FlowGraph {
	graph := &FlowGraph{
		SessionID: sessionID,
		Nodes:     make([]FlowNode, 0, len(events)),
		Edges:     make([]FlowEdge, 0, len(events)-1),
	}

	var prevNodeID string
	yPos := 0.0

	for _, event := range events {
		node := FlowNode{
			ID:        event.ID,
			Timestamp: event.Timestamp,
			Status:    event.Status,
			Position: &Position{
				X: 100,
				Y: yPos,
			},
		}

		switch event.EventType {
		case EventMessageReceived, EventMessageSent:
			node.Type = "message"
			if event.Message != nil {
				node.Label = truncateString(event.Message.Content, 50)
				node.Data = map[string]interface{}{
					"direction":   event.Message.Direction,
					"contentType": event.Message.ContentType,
					"length":      event.Message.Length,
				}
			}
		case EventToolCall:
			node.Type = "tool_call"
			if event.ToolCall != nil {
				node.Label = event.ToolCall.ToolName
				node.Duration = event.ToolCall.Duration
				node.Data = map[string]interface{}{
					"toolId":      event.ToolCall.ToolID,
					"sandboxUsed": event.ToolCall.SandboxUsed,
					"status":      event.ToolCall.Status,
				}
			}
		case EventLLMRequest:
			node.Type = "llm_request"
			if event.LLMRequest != nil {
				node.Label = event.LLMRequest.Model
				node.Duration = event.LLMRequest.Duration
				node.Data = map[string]interface{}{
					"provider":         event.LLMRequest.Provider,
					"promptTokens":     event.LLMRequest.PromptTokens,
					"completionTokens": event.LLMRequest.CompletionTokens,
					"totalTokens":      event.LLMRequest.TotalTokens,
				}
			}
		case EventSecurityThreat:
			node.Type = "security_check"
			if event.Security != nil {
				node.Label = string(event.Security.ThreatLevel)
				node.Data = map[string]interface{}{
					"threatScore":  event.Security.ThreatScore,
					"threatTypes":  event.Security.ThreatTypes,
					"action":       event.Security.Action,
				}
			}
		default:
			node.Type = string(event.EventType)
			node.Label = string(event.EventType)
		}

		graph.Nodes = append(graph.Nodes, node)

		// Create edge from previous node
		if prevNodeID != "" {
			edge := FlowEdge{
				ID:     uuid.New().String(),
				Source: prevNodeID,
				Target: node.ID,
			}
			graph.Edges = append(graph.Edges, edge)
		}

		prevNodeID = node.ID
		yPos += 100
	}

	return graph
}

func (m *Manager) cleanupOldestSession(ctx context.Context) {
	var oldestID string
	var oldestStartedAt time.Time
	for _, session := range m.activeSessions {
		if oldestID == "" || session.StartedAt.Before(oldestStartedAt) {
			oldestID = session.ID
			oldestStartedAt = session.StartedAt
		}
	}
	if oldestID != "" {
		_ = m.endSessionLocked(ctx, oldestID)
	}
}

func threatLevelPriority(level ThreatLevel) int {
	switch level {
	case ThreatLevelNone:
		return 0
	case ThreatLevelLow:
		return 1
	case ThreatLevelMedium:
		return 2
	case ThreatLevelHigh:
		return 3
	case ThreatLevelCritical:
		return 4
	default:
		return 0
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GetRetentionConfig returns the current retention configuration.
func (m *Manager) GetRetentionConfig() *RetentionConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &m.config.Retention
}

// UpdateRetentionConfig updates the retention configuration.
func (m *Manager) UpdateRetentionConfig(config *RetentionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Retention = *config
}

// GetConfig returns the full companion configuration.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
