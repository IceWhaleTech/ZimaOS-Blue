package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChatHandlerGetMessages_AllowsSystemScopedChannelConversationForAuthenticatedUser(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	convID := "ch:feishu:oc_test_channel_scope"
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu chat"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), convID, memory.Message{Role: "user", Content: "hello"}); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+convID+"/messages", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-a",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(convID)

	if err := handler.GetMessages(c); err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp []memory.Message
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("message count = %d, want 1", len(resp))
	}
}

func TestChatHandlerGetMessages_StillRejectsUnownedNonChannelConversationForAuthenticatedUser(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	const convID = "system-conversation"
	if _, err := store.CreateConversationWithID(context.Background(), convID, "system chat"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+convID+"/messages", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-a",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(convID)

	err = handler.GetMessages(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusNotFound)
	}
}

func TestChatHandlerListConversations_IncludesSystemScopedChannelConversationForAuthenticatedUser(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	channelConv, err := store.CreateConversationWithID(context.Background(), "ch:wechat_ilink:user-123", "wechat_ilink - user-123")
	if err != nil {
		t.Fatalf("CreateConversationWithID(channel): %v", err)
	}
	if _, err := store.CreateConversationWithID(context.Background(), "system-conversation", "system chat"); err != nil {
		t.Fatalf("CreateConversationWithID(system): %v", err)
	}
	ownedConv, err := store.CreateConversation(context.Background(), "owned chat", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(owned): %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-a",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListConversations(c); err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp []memory.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	seen := make(map[string]bool, len(resp))
	for _, conv := range resp {
		seen[conv.ID] = true
	}
	if !seen[channelConv.ID] {
		t.Fatalf("expected channel conversation %q in response, got %+v", channelConv.ID, resp)
	}
	if !seen[ownedConv.ID] {
		t.Fatalf("expected owned conversation %q in response, got %+v", ownedConv.ID, resp)
	}
	if seen["system-conversation"] {
		t.Fatalf("unexpected unowned non-channel conversation in response: %+v", resp)
	}
}
