package tools

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type stubSessionsService struct {
	sessions          []SessionSummary
	messages          map[string][]SessionMessage
	lastCreateTitle   string
	lastCreateUserID  string
	lastCreatePinned  bool
	lastAppendSession string
	lastAppendMessage SessionMessage
}

func (s *stubSessionsService) ListSessions(ctx context.Context, limit, offset int, userID string) ([]SessionSummary, error) {
	_ = ctx
	if offset < 0 {
		offset = 0
	}
	filtered := make([]SessionSummary, 0, len(s.sessions))
	for _, session := range s.sessions {
		if userID == "" || session.UserID == userID {
			filtered = append(filtered, session)
		}
	}
	if offset >= len(filtered) {
		return []SessionSummary{}, nil
	}
	end := len(filtered)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return append([]SessionSummary(nil), filtered[offset:end]...), nil
}

func (s *stubSessionsService) GetSession(ctx context.Context, sessionID string) (*SessionSummary, error) {
	_ = ctx
	for i := range s.sessions {
		if s.sessions[i].ID == sessionID {
			session := s.sessions[i]
			return &session, nil
		}
	}
	return nil, nil
}

func (s *stubSessionsService) GetSessionMessages(ctx context.Context, sessionID string, limit, offset int) ([]SessionMessage, error) {
	_ = ctx
	messages := append([]SessionMessage(nil), s.messages[sessionID]...)
	if offset < 0 {
		offset = 0
	}
	if offset >= len(messages) {
		return []SessionMessage{}, nil
	}
	end := len(messages)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return messages[offset:end], nil
}

func (s *stubSessionsService) CreateSession(ctx context.Context, title, userID string, pinned bool) (*SessionSummary, error) {
	_ = ctx
	s.lastCreateTitle = title
	s.lastCreateUserID = userID
	s.lastCreatePinned = pinned
	session := SessionSummary{
		ID:        fmt.Sprintf("conv_%d", len(s.sessions)+1),
		Title:     title,
		UserID:    userID,
		Pinned:    pinned,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions = append(s.sessions, session)
	if s.messages == nil {
		s.messages = make(map[string][]SessionMessage)
	}
	return &session, nil
}

func (s *stubSessionsService) AppendSessionMessage(ctx context.Context, sessionID string, msg SessionMessage) (*SessionMessage, error) {
	_ = ctx
	s.lastAppendSession = sessionID
	s.lastAppendMessage = msg
	for i := range s.sessions {
		if s.sessions[i].ID == sessionID {
			message := msg
			if message.ID == "" {
				message.ID = fmt.Sprintf("msg_%d", len(s.messages[sessionID])+1)
			}
			if message.Role == "" {
				message.Role = "user"
			}
			if message.CreatedAt.IsZero() {
				message.CreatedAt = time.Now()
			}
			s.messages[sessionID] = append(s.messages[sessionID], message)
			s.sessions[i].UpdatedAt = message.CreatedAt
			return &message, nil
		}
	}
	return nil, fmt.Errorf("session not found")
}

func TestSessionsListToolExecuteSupportsNestedCamelCaseArgs(t *testing.T) {
	svc := &stubSessionsService{
		sessions: []SessionSummary{
			{ID: "conv_1", UserID: "user_a", Pinned: true},
			{ID: "conv_2", UserID: "user_a", Pinned: false},
			{ID: "conv_3", UserID: "user_b", Pinned: true},
		},
	}
	tool := NewSessionsListTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"limit":      10,
			"offset":     0,
			"user":       "user_a",
			"pinnedOnly": true,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	sessions := payload["sessions"].([]SessionSummary)
	if len(sessions) != 1 || sessions[0].ID != "conv_1" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}
	if payload["count"].(int) != 1 {
		t.Fatalf("count = %v, want 1", payload["count"])
	}
}

func TestSessionsHistoryToolExecute(t *testing.T) {
	svc := &stubSessionsService{
		sessions: []SessionSummary{{ID: "conv_1", Title: "First"}},
		messages: map[string][]SessionMessage{
			"conv_1": {{ID: "msg_1", Role: "user", Content: "hello"}, {ID: "msg_2", Role: "assistant", Content: "hi"}},
		},
	}
	tool := NewSessionsHistoryTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"id": "conv_1", "limit": 1})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	messages := payload["messages"].([]SessionMessage)
	if len(messages) != 1 || messages[0].ID != "msg_1" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	if payload["count"].(int) != 1 {
		t.Fatalf("count = %v, want 1", payload["count"])
	}
}

func TestSessionStatusToolExecute(t *testing.T) {
	svc := &stubSessionsService{
		sessions: []SessionSummary{{ID: "conv_1", Title: "First"}},
		messages: map[string][]SessionMessage{
			"conv_1": {{ID: "msg_1", Role: "user", Content: "hello"}, {ID: "msg_2", Role: "assistant", Content: "hi"}},
		},
	}
	tool := NewSessionStatusTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"id": "conv_1"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["message_count"].(int) != 2 {
		t.Fatalf("message_count = %v, want 2", payload["message_count"])
	}
	recent := payload["recent_messages"].([]SessionMessage)
	if len(recent) != 2 || recent[1].ID != "msg_2" {
		t.Fatalf("unexpected recent messages: %#v", recent)
	}
}

func TestSessionsSpawnToolExecute(t *testing.T) {
	svc := &stubSessionsService{}
	tool := NewSessionsSpawnTool(svc)
	ctx := WithUserID(context.Background(), "user_ctx")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"input":           "Conversation from prompt",
		"pinned":          true,
		"initial_message": "seed",
		"role":            "assistant",
		"provider":        "openai",
		"model":           "gpt-test",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	session := payload["session"].(*SessionSummary)
	if session.Title != "Conversation from prompt" {
		t.Fatalf("title = %q, want Conversation from prompt", session.Title)
	}
	if svc.lastCreateUserID != "user_ctx" || !svc.lastCreatePinned {
		t.Fatalf("unexpected create args: user=%q pinned=%v", svc.lastCreateUserID, svc.lastCreatePinned)
	}
	initial := payload["initial_message"].(*SessionMessage)
	if initial.Content != "seed" || initial.Role != "assistant" {
		t.Fatalf("unexpected initial message: %#v", initial)
	}
}

func TestSessionsSpawnToolExecuteSupportsNestedCamelCaseArgs(t *testing.T) {
	svc := &stubSessionsService{}
	tool := NewSessionsSpawnTool(svc)
	ctx := WithUserID(context.Background(), "user_ctx")
	result, err := tool.Execute(ctx, map[string]interface{}{
		"input": map[string]interface{}{
			"message":        "Conversation from prompt",
			"pinned":         true,
			"initialMessage": "seed",
			"role":           "assistant",
			"provider":       "openai",
			"model":          "gpt-test",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	session := payload["session"].(*SessionSummary)
	if session.Title != "Conversation from prompt" {
		t.Fatalf("title = %q, want Conversation from prompt", session.Title)
	}
	if svc.lastCreateUserID != "user_ctx" || !svc.lastCreatePinned {
		t.Fatalf("unexpected create args: user=%q pinned=%v", svc.lastCreateUserID, svc.lastCreatePinned)
	}
	initial := payload["initial_message"].(*SessionMessage)
	if initial.Content != "seed" || initial.Role != "assistant" {
		t.Fatalf("unexpected initial message: %#v", initial)
	}
}

func TestSessionsSendToolExecute(t *testing.T) {
	svc := &stubSessionsService{
		sessions: []SessionSummary{{ID: "conv_1", Title: "First"}},
		messages: map[string][]SessionMessage{"conv_1": {}},
	}
	tool := NewSessionsSendTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"id":       "conv_1",
		"content":  "hello there",
		"provider": "anthropic",
		"model":    "claude-test",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	message := payload["message"].(*SessionMessage)
	if message.Content != "hello there" || message.Role != "user" {
		t.Fatalf("unexpected message: %#v", message)
	}
	if svc.lastAppendSession != "conv_1" || svc.lastAppendMessage.Provider != "anthropic" {
		t.Fatalf("unexpected append args: session=%q msg=%#v", svc.lastAppendSession, svc.lastAppendMessage)
	}
}
