package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChatHandlerCancelStream_RejectsUnownedConversation(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Owned conversation", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	cancelled := false
	handler.streamController.Register("stream-a", func() { cancelled = true })
	handler.convStreamMu.Lock()
	handler.convToStream[conv.ID] = "stream-a"
	handler.convStreamMu.Unlock()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/cancel", strings.NewReader(`{"stream_id":"stream-a"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-b",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.CancelStream(c)
	if err == nil {
		t.Fatal("expected ownership error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusNotFound)
	}
	if cancelled {
		t.Fatal("expected stream to remain active")
	}
	if got := handler.streamController.ActiveSessions(); got != 1 {
		t.Fatalf("active streams = %d, want 1", got)
	}
}

func TestChatHandlerCancelStream_RejectsStreamFromDifferentConversation(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	convA, err := store.CreateConversation(context.Background(), "Conversation A", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(A): %v", err)
	}
	convB, err := store.CreateConversation(context.Background(), "Conversation B", "user-b")
	if err != nil {
		t.Fatalf("CreateConversation(B): %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	cancelledA := false
	cancelledB := false
	handler.streamController.Register("stream-a", func() { cancelledA = true })
	handler.streamController.Register("stream-b", func() { cancelledB = true })
	handler.convStreamMu.Lock()
	handler.convToStream[convA.ID] = "stream-a"
	handler.convToStream[convB.ID] = "stream-b"
	handler.convStreamMu.Unlock()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+convA.ID+"/messages/cancel", strings.NewReader(`{"stream_id":"stream-b"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-a",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(convA.ID)

	if err := handler.CancelStream(c); err != nil {
		t.Fatalf("CancelStream: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if cancelledA || cancelledB {
		t.Fatal("expected both streams to remain active")
	}
	if got := handler.streamController.ActiveSessions(); got != 2 {
		t.Fatalf("active streams = %d, want 2", got)
	}
}

func TestChatHandlerGetConversationBootstrap_ReportsOwnedConversationActiveStreamState(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Owned conversation", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	handler.streamController.Register("stream-a", func() {})
	handler.convStreamMu.Lock()
	handler.convToStream[conv.ID] = "stream-a"
	handler.convStreamMu.Unlock()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/bootstrap", nil)
	req = requestWithUser(req, "user-a")
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
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"active_stream":{"conversation_id":"`+conv.ID+`","active":true`) {
		t.Fatalf("expected active stream in response, body=%s", body)
	}
	if !strings.Contains(body, `"stream_id":"stream-a"`) {
		t.Fatalf("expected stream id in response, body=%s", body)
	}
}
