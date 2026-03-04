package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestExecuteSlashCommandClearClearsPreviousResponseID(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(ctx, conv.ID, memory.Message{Role: "user", Content: "hello"}); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	handler.setPreviousResponseID(conv.ID, "resp_old_clear_1")

	reply, handled := handler.executeSlashCommand(ctx, conv.ID, "/clear")
	if !handled {
		t.Fatal("executeSlashCommand should handle /clear")
	}
	if !strings.Contains(reply, "Conversation cleared") {
		t.Fatalf("unexpected /clear reply: %q", reply)
	}
	if got := handler.getPreviousResponseID(conv.ID); got != "" {
		t.Fatalf("previous_response_id should be cleared, got %q", got)
	}

	persisted, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if persisted != "" {
		t.Fatalf("persisted previous_response_id should be cleared, got %q", persisted)
	}
}

func TestDeleteMessagesClearsPreviousResponseID(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	msg, err := store.AddMessage(ctx, conv.ID, memory.Message{Role: "user", Content: "hello"})
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if _, err := store.AddMessage(ctx, conv.ID, memory.Message{Role: "assistant", Content: "world"}); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	handler.setPreviousResponseID(conv.ID, "resp_old_delete_1")

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message_ids":["`+msg.ID+`"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.DeleteMessages(c); err != nil {
		t.Fatalf("DeleteMessages: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := handler.getPreviousResponseID(conv.ID); got != "" {
		t.Fatalf("previous_response_id should be cleared, got %q", got)
	}

	persisted, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if persisted != "" {
		t.Fatalf("persisted previous_response_id should be cleared, got %q", persisted)
	}
}
