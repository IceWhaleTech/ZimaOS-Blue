package tools

import (
	"context"
	"errors"
	"time"
)

// SessionSummary is the normalized lightweight session view exposed to tools.
type SessionSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UserID    string    `json:"user_id,omitempty"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionMessage is the normalized message view used by session tools.
type SessionMessage struct {
	ID         string    `json:"id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	Model      string    `json:"model,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SessionsService provides list/read/write access without importing the memory package.
type SessionsService interface {
	ListSessions(ctx context.Context, limit, offset int, userID string) ([]SessionSummary, error)
	GetSession(ctx context.Context, sessionID string) (*SessionSummary, error)
	GetSessionMessages(ctx context.Context, sessionID string, limit, offset int) ([]SessionMessage, error)
	CreateSession(ctx context.Context, title, userID string, pinned bool) (*SessionSummary, error)
	AppendSessionMessage(ctx context.Context, sessionID string, msg SessionMessage) (*SessionMessage, error)
}

// SessionsListTool lists recent sessions/conversations.
type SessionsListTool struct {
	service SessionsService
}

// SessionsHistoryTool reads session history.
type SessionsHistoryTool struct {
	service SessionsService
}

// SessionStatusTool reads current session metadata.
type SessionStatusTool struct {
	service SessionsService
}

// SessionsSpawnTool creates a new session.
type SessionsSpawnTool struct {
	service SessionsService
}

// SessionsSendTool appends a message to an existing session.
type SessionsSendTool struct {
	service SessionsService
}

// NewSessionsListTool creates a native sessions_list tool.
func NewSessionsListTool(service SessionsService) *SessionsListTool {
	return &SessionsListTool{service: service}
}

// NewSessionsHistoryTool creates a native sessions_history tool.
func NewSessionsHistoryTool(service SessionsService) *SessionsHistoryTool {
	return &SessionsHistoryTool{service: service}
}

// NewSessionStatusTool creates a native session_status tool.
func NewSessionStatusTool(service SessionsService) *SessionStatusTool {
	return &SessionStatusTool{service: service}
}

// NewSessionsSpawnTool creates a native sessions_spawn tool.
func NewSessionsSpawnTool(service SessionsService) *SessionsSpawnTool {
	return &SessionsSpawnTool{service: service}
}

// NewSessionsSendTool creates a native sessions_send tool.
func NewSessionsSendTool(service SessionsService) *SessionsSendTool {
	return &SessionsSendTool{service: service}
}

// Definition returns the tool schema.
func (t *SessionsListTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "sessions_list",
		Description: "List recent sessions and conversation metadata.",
		Icon:        "sessions",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit":       map[string]interface{}{"type": "integer", "description": "Maximum number of sessions to return (default 20, max 100)"},
				"offset":      map[string]interface{}{"type": "integer", "description": "Pagination offset (default 0)"},
				"user_id":     map[string]interface{}{"type": "string", "description": "Optional user ID filter"},
				"pinned_only": map[string]interface{}{"type": "boolean", "description": "If true, return only pinned sessions"},
			},
		},
	}
}

// Definition returns the tool schema.
func (t *SessionsHistoryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "sessions_history",
		Description: "Read recent messages from a session.",
		Icon:        "history",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":     map[string]interface{}{"type": "string", "description": "Session / conversation ID"},
				"limit":  map[string]interface{}{"type": "integer", "description": "Maximum number of messages (default 50, max 500)"},
				"offset": map[string]interface{}{"type": "integer", "description": "Pagination offset (default 0)"},
			},
			"required": []string{"id"},
		},
	}
}

// Definition returns the tool schema.
func (t *SessionStatusTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "session_status",
		Description: "Get metadata and a short recent-history preview for a session.",
		Icon:        "session-status",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{"type": "string", "description": "Session / conversation ID"},
			},
			"required": []string{"id"},
		},
	}
}

// Definition returns the tool schema.
func (t *SessionsSpawnTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "sessions_spawn",
		Description: "Create a new session and optionally seed it with an initial message.",
		Icon:        "session-add",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title":           map[string]interface{}{"type": "string", "description": "Session title"},
				"name":            map[string]interface{}{"type": "string", "description": "Alias for title"},
				"user_id":         map[string]interface{}{"type": "string", "description": "Optional session owner override"},
				"pinned":          map[string]interface{}{"type": "boolean", "description": "Whether to pin the new session"},
				"initial_message": map[string]interface{}{"type": "string", "description": "Optional first message to append after creation"},
				"role":            map[string]interface{}{"type": "string", "description": "Role for the initial message (default user)"},
				"provider":        map[string]interface{}{"type": "string", "description": "Optional provider metadata for the initial message"},
				"model":           map[string]interface{}{"type": "string", "description": "Optional model metadata for the initial message"},
			},
		},
	}
}

// Definition returns the tool schema.
func (t *SessionsSendTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "sessions_send",
		Description: "Append a message to an existing session.",
		Icon:        "session-send",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":       map[string]interface{}{"type": "string", "description": "Session / conversation ID"},
				"message":  map[string]interface{}{"type": "string", "description": "Message content"},
				"content":  map[string]interface{}{"type": "string", "description": "Alias for message"},
				"role":     map[string]interface{}{"type": "string", "description": "Message role (default user)"},
				"provider": map[string]interface{}{"type": "string", "description": "Optional provider metadata"},
				"model":    map[string]interface{}{"type": "string", "description": "Optional model metadata"},
			},
			"required": []string{"id"},
		},
	}
}

// Execute returns recent sessions.
func (t *SessionsListTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("sessions service not available")
	}
	limit, err := fsAsInt(args, "limit", 20)
	if err != nil {
		return nil, err
	}
	offset, err := fsAsInt(args, "offset", 0)
	if err != nil {
		return nil, err
	}
	limit = fsClamp(limit, 1, 100)
	if offset < 0 {
		offset = 0
	}
	pinnedOnly, _ := compatBoolArg(args, "pinned_only", "pinnedOnly")
	userID := firstCompatString(args, "user_id", "user", "owner_id", "ownerId")

	sessions, err := t.service.ListSessions(ctx, limit, offset, userID)
	if err != nil {
		return nil, err
	}
	if pinnedOnly {
		filtered := make([]SessionSummary, 0, len(sessions))
		for _, session := range sessions {
			if session.Pinned {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}
	return map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	}, nil
}

// Execute returns recent messages for a session.
func (t *SessionsHistoryTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("sessions service not available")
	}
	sessionID := firstCompatString(args, "id", "session_id", "session", "conversation_id")
	if sessionID == "" {
		return nil, errors.New("id is required")
	}
	limit, err := fsAsInt(args, "limit", 50)
	if err != nil {
		return nil, err
	}
	offset, err := fsAsInt(args, "offset", 0)
	if err != nil {
		return nil, err
	}
	limit = fsClamp(limit, 1, 500)
	if offset < 0 {
		offset = 0
	}
	session, err := getRequiredSession(ctx, t.service, sessionID)
	if err != nil {
		return nil, err
	}
	messages, err := t.service.GetSessionMessages(ctx, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session":  session,
		"messages": messages,
		"count":    len(messages),
	}, nil
}

// Execute returns summary metadata and recent messages for a session.
func (t *SessionStatusTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("sessions service not available")
	}
	sessionID := firstCompatString(args, "id", "session_id", "session", "conversation_id")
	if sessionID == "" {
		return nil, errors.New("id is required")
	}
	session, err := getRequiredSession(ctx, t.service, sessionID)
	if err != nil {
		return nil, err
	}
	messages, err := t.service.GetSessionMessages(ctx, sessionID, 5, 0)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session":         session,
		"recent_messages": messages,
		"message_count":   len(messages),
	}, nil
}

// Execute creates a new session.
func (t *SessionsSpawnTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("sessions service not available")
	}
	title := firstCompatString(args, "title", "name")
	if title == "" {
		title = trimCompatText(firstCompatString(args, "input", "prompt", "message", "content", "text"), 72)
	}
	if title == "" {
		title = "New Conversation"
	}
	userID := firstCompatString(args, "user_id", "user", "owner_id", "ownerId")
	if userID == "" {
		userID = GetUserID(ctx)
	}
	pinned, _ := compatBoolArg(args, "pinned", "isPinned")
	initialMessage := firstCompatString(args, "initial_message", "initialMessage")
	role := normalizeSessionRole(firstCompatString(args, "role"))

	session, err := t.service.CreateSession(ctx, title, userID, pinned)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"session": session,
		"created": true,
		"title":   session.Title,
		"id":      session.ID,
		"pinned":  session.Pinned,
	}
	if initialMessage != "" {
		msg, err := t.service.AppendSessionMessage(ctx, session.ID, SessionMessage{
			Role:     role,
			Content:  initialMessage,
			Provider: firstCompatString(args, "provider"),
			Model:    firstCompatString(args, "model"),
		})
		if err != nil {
			return nil, err
		}
		result["initial_message"] = msg
	}
	return result, nil
}

// Execute appends a message to an existing session.
func (t *SessionsSendTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("sessions service not available")
	}
	sessionID := firstCompatString(args, "id", "session_id", "session", "conversation_id")
	if sessionID == "" {
		return nil, errors.New("id is required")
	}
	content := firstCompatString(args, "message", "content", "text", "input", "prompt")
	if content == "" {
		return nil, errors.New("message is required")
	}
	message, err := t.service.AppendSessionMessage(ctx, sessionID, SessionMessage{
		Role:     normalizeSessionRole(firstCompatString(args, "role")),
		Content:  content,
		Provider: firstCompatString(args, "provider"),
		Model:    firstCompatString(args, "model"),
	})
	if err != nil {
		return nil, err
	}
	session, err := getRequiredSession(ctx, t.service, sessionID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"session": session,
		"message": message,
		"sent":    true,
		"id":      message.ID,
		"role":    message.Role,
		"content": message.Content,
	}, nil
}

func getRequiredSession(ctx context.Context, service SessionsService, sessionID string) (*SessionSummary, error) {
	session, err := service.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func normalizeSessionRole(role string) string {
	role = asString(role)
	if role == "" {
		return "user"
	}
	return role
}

// RegisterSessionTools registers native session tools backed by the provided service.
func RegisterSessionTools(registry *Registry, service SessionsService) {
	if registry == nil || service == nil {
		return
	}
	registry.Register(NewSessionsListTool(service))
	registry.Register(NewSessionsHistoryTool(service))
	registry.Register(NewSessionStatusTool(service))
	registry.Register(NewSessionsSpawnTool(service))
	registry.Register(NewSessionsSendTool(service))
}
