package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type stubChatBootstrapTaskProjectionService struct {
	list func(ctx context.Context, filter harness.UserTaskProjectionFilter) ([]harness.UserTaskProjection, error)
}

func (s stubChatBootstrapTaskProjectionService) List(
	ctx context.Context,
	filter harness.UserTaskProjectionFilter,
) ([]harness.UserTaskProjection, error) {
	if s.list == nil {
		return nil, nil
	}
	return s.list(ctx, filter)
}

type stubChatBootstrapToolApprovalSource struct {
	bySession func(sessionID string) map[string]any
	byUser    func(userID string) map[string]any
}

func (s stubChatBootstrapToolApprovalSource) GetPendingBySession(sessionID string) map[string]any {
	if s.bySession == nil {
		return nil
	}
	return s.bySession(sessionID)
}

func (s stubChatBootstrapToolApprovalSource) GetPending(userID string) map[string]any {
	if s.byUser == nil {
		return nil
	}
	return s.byUser(userID)
}

type stubChatBootstrapQuestionSource struct {
	bySession func(sessionID string) *tools.QuestionRequest
	byUser    func(userID string) *tools.QuestionRequest
}

func (s stubChatBootstrapQuestionSource) GetPendingBySession(sessionID string) *tools.QuestionRequest {
	if s.bySession == nil {
		return nil
	}
	return s.bySession(sessionID)
}

func (s stubChatBootstrapQuestionSource) GetPending(userID string) *tools.QuestionRequest {
	if s.byUser == nil {
		return nil
	}
	return s.byUser(userID)
}

type stubChatBootstrapExecApprovalSource struct {
	bySession func(sessionID string) *tools.ApprovalRequest
	byUser    func(userID string) *tools.ApprovalRequest
}

func (s stubChatBootstrapExecApprovalSource) GetPendingBySession(sessionID string) *tools.ApprovalRequest {
	if s.bySession == nil {
		return nil
	}
	return s.bySession(sessionID)
}

func (s stubChatBootstrapExecApprovalSource) GetPending(userID string) *tools.ApprovalRequest {
	if s.byUser == nil {
		return nil
	}
	return s.byUser(userID)
}

func TestChatHandlerGetConversationBootstrap_AggregatesSparseConversationState(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "Owned conversation", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := store.UpsertConversationCommandState(ctx, memory.ConversationCommandState{
		ConversationID:      conv.ID,
		SelectedProviderID:  "openai",
		SelectedModelID:     "gpt-5",
		Offline:             true,
		WebSearchEnabled:    false,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	handler.streamController.Register("stream-a", func() {})
	handler.convStreamMu.Lock()
	handler.convToStream[conv.ID] = "stream-a"
	handler.convStreamMu.Unlock()

	handler.SetTaskProjectionService(stubChatBootstrapTaskProjectionService{
		list: func(_ context.Context, filter harness.UserTaskProjectionFilter) ([]harness.UserTaskProjection, error) {
			switch filter.Scope {
			case "current":
				return []harness.UserTaskProjection{{
					ID:             "task-current",
					ConversationID: conv.ID,
					Scope:          "current",
					Title:          "Current task",
					Status:         "running",
					Stage:          "working",
					Progress:       45,
				}}, nil
			case "background":
				return []harness.UserTaskProjection{{
					ID:             "task-background",
					ConversationID: "conv-2",
					Scope:          "background",
					Title:          "Background task",
					Status:         "running",
					Stage:          "verifying",
					Progress:       72,
				}}, nil
			default:
				return nil, nil
			}
		},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/bootstrap", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-a",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)
	c.Set("user", &auth.Claims{
		UserClaims: auth.UserClaims{
			UserID: "user-a",
			Role:   "user",
		},
	})

	if err := handler.GetConversationBootstrap(c); err != nil {
		t.Fatalf("GetConversationBootstrap: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		CommandState struct {
			ConversationID     string `json:"conversation_id"`
			SelectedProviderID string `json:"selected_provider_id"`
			SelectedModelID    string `json:"selected_model_id"`
			Offline            bool   `json:"offline"`
		} `json:"command_state"`
		ActiveStream struct {
			ConversationID string `json:"conversation_id"`
			Active         bool   `json:"active"`
			StreamID       string `json:"stream_id"`
		} `json:"active_stream"`
		CurrentTasks    []harness.UserTaskProjection `json:"current_tasks"`
		BackgroundTasks []harness.UserTaskProjection `json:"background_tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if body.CommandState.ConversationID != conv.ID ||
		body.CommandState.SelectedProviderID != "openai" ||
		body.CommandState.SelectedModelID != "gpt-5" ||
		!body.CommandState.Offline {
		t.Fatalf("unexpected command state payload: %+v", body.CommandState)
	}
	if !body.ActiveStream.Active || body.ActiveStream.StreamID != "stream-a" || body.ActiveStream.ConversationID != conv.ID {
		t.Fatalf("unexpected active stream payload: %+v", body.ActiveStream)
	}
	if len(body.CurrentTasks) != 1 || body.CurrentTasks[0].ID != "task-current" {
		t.Fatalf("unexpected current tasks payload: %+v", body.CurrentTasks)
	}
	if len(body.BackgroundTasks) != 1 || body.BackgroundTasks[0].ID != "task-background" {
		t.Fatalf("unexpected background tasks payload: %+v", body.BackgroundTasks)
	}
}

func TestChatHandlerGetConversationBootstrap_AggregatesPendingConfirmationState(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "Owned conversation", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	handler.SetConversationBootstrapToolApprovalSource(stubChatBootstrapToolApprovalSource{
		bySession: func(sessionID string) map[string]any {
			if sessionID != conv.ID {
				return nil
			}
			return map[string]any{
				"id":           "approval-session",
				"tool_name":    "browser",
				"tool_call_id": "tool-1",
				"arguments":    map[string]any{"url": "https://example.com"},
				"session_id":   conv.ID,
				"binding_hash": "bind-session",
			}
		},
		byUser: func(userID string) map[string]any {
			return map[string]any{
				"id":           "approval-user",
				"tool_name":    "browser",
				"tool_call_id": "tool-user",
				"arguments":    map[string]any{"url": "https://user.example.com"},
				"session_id":   "other-session",
				"binding_hash": "bind-user",
			}
		},
	})
	handler.SetConversationBootstrapQuestionSource(stubChatBootstrapQuestionSource{
		bySession: func(sessionID string) *tools.QuestionRequest {
			return nil
		},
		byUser: func(userID string) *tools.QuestionRequest {
			if userID != "user-a" {
				return nil
			}
			return &tools.QuestionRequest{
				ID:        "question-user",
				UserID:    userID,
				SessionID: conv.ID,
				ExpiresAt: 1735689600000,
				Questions: []tools.QuestionItem{{
					ID:       "q1",
					Header:   "Question",
					Question: "Need confirmation?",
				}},
			}
		},
	})
	handler.SetConversationBootstrapExecApprovalSource(stubChatBootstrapExecApprovalSource{
		bySession: func(sessionID string) *tools.ApprovalRequest {
			if sessionID != conv.ID {
				return nil
			}
			return &tools.ApprovalRequest{
				ID:        "exec-session",
				Type:      "command",
				Command:   "ls -la",
				SessionID: conv.ID,
				ExpiresAt: 1735689600000,
			}
		},
		byUser: func(userID string) *tools.ApprovalRequest {
			return &tools.ApprovalRequest{
				ID:        "exec-user",
				Type:      "directory",
				Directory: "/tmp",
				SessionID: "other-session",
				ExpiresAt: 1735689600000,
			}
		},
	})

	body, err := handler.conversationBootstrap(context.Background(), conv.ID, "user-a")
	if err != nil {
		t.Fatalf("conversationBootstrap: %v", err)
	}

	if got := body.PendingApproval["id"]; got != "approval-session" {
		t.Fatalf("pending approval id = %v, want %q", got, "approval-session")
	}
	if got := body.PendingApproval["session_id"]; got != conv.ID {
		t.Fatalf("pending approval session_id = %v, want %q", got, conv.ID)
	}
	if body.PendingQuestion == nil || body.PendingQuestion.ID != "question-user" || body.PendingQuestion.SessionID != conv.ID {
		t.Fatalf("unexpected pending question payload: %+v", body.PendingQuestion)
	}
	if body.PendingExecApproval == nil || body.PendingExecApproval.ID != "exec-session" || body.PendingExecApproval.SessionID != conv.ID {
		t.Fatalf("unexpected pending exec approval payload: %+v", body.PendingExecApproval)
	}
}
