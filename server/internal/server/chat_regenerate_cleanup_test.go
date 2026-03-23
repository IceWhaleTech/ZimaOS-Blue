package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func seedBuggyRegenerateTail(t *testing.T, store *memory.Store, conversationID, content string) string {
	t.Helper()

	originalUser, err := store.AddMessage(context.Background(), conversationID, memory.Message{
		Role:    "user",
		Content: content,
	})
	if err != nil {
		t.Fatalf("AddMessage(original user): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conversationID, memory.Message{
		Role:    "assistant",
		Content: "old answer",
	}); err != nil {
		t.Fatalf("AddMessage(old assistant): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conversationID, memory.Message{
		Role:    "user",
		Content: content,
	}); err != nil {
		t.Fatalf("AddMessage(duplicated user): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conversationID, memory.Message{
		Role:    "assistant",
		Content: "duplicated answer",
	}); err != nil {
		t.Fatalf("AddMessage(duplicated assistant): %v", err)
	}

	return originalUser.ID
}

func seedDeepResearchAssistantTail(t *testing.T, store *memory.Store, conversationID, content string) string {
	t.Helper()

	user, err := store.AddMessage(context.Background(), conversationID, memory.Message{
		Role:    "user",
		Content: content,
	})
	if err != nil {
		t.Fatalf("AddMessage(user): %v", err)
	}
	for _, assistantContent := range []string{
		"```typeless\n{\"type\":\"deep-research-progress\",\"job_id\":\"job-1\"}\n```",
		"```typeless\n{\"type\":\"deep-research\",\"query\":\"topic\"}\n```",
	} {
		if _, err := store.AddMessage(context.Background(), conversationID, memory.Message{
			Role:    "assistant",
			Content: assistantContent,
		}); err != nil {
			t.Fatalf("AddMessage(assistant tail): %v", err)
		}
	}

	return user.ID
}

func TestSendMessageRegenerateCleansDuplicatedTail(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "regenerate send cleanup")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	originalUserID := seedBuggyRegenerateTail(t, store, conv.ID, "repeat this request")

	registry := llm.NewProviderRegistry()
	provider := &requestCaptureProvider{}
	registry.Register(provider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.setPreviousResponseID(conv.ID, "resp-old-send")

	e := echo.New()
	reqBody := `{"message":"repeat this request","provider":"capture","model":"gpt-5-codex","regenerate":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if provider.lastReq.PreviousResponseID != "" {
		t.Fatalf("PreviousResponseID = %q, want empty on regenerate", provider.lastReq.PreviousResponseID)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0].ID != originalUserID {
		t.Fatalf("user id = %q, want original %q", messages[0].ID, originalUserID)
	}
	if messages[0].Role != "user" || messages[0].Content != "repeat this request" {
		t.Fatalf("user message = %+v, want original regenerated request", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != "ok" {
		t.Fatalf("assistant message = %+v, want regenerated response", messages[1])
	}
}

func TestStreamMessageRegenerateCleansDuplicatedTail(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "regenerate stream cleanup")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	originalUserID := seedBuggyRegenerateTail(t, store, conv.ID, "repeat this request")

	registry := llm.NewProviderRegistry()
	provider := NewStreamingMockProvider("fresh streamed answer")
	registry.Register(provider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.setPreviousResponseID(conv.ID, "resp-old-stream")

	e := echo.New()
	reqBody := `{"message":"repeat this request","provider":"streaming-mock","model":"gpt-5.3-codex","regenerate":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if provider.lastReq.PreviousResponseID != "" {
		t.Fatalf("PreviousResponseID = %q, want empty on regenerate", provider.lastReq.PreviousResponseID)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0].ID != originalUserID {
		t.Fatalf("user id = %q, want original %q", messages[0].ID, originalUserID)
	}
	if messages[0].Role != "user" || messages[0].Content != "repeat this request" {
		t.Fatalf("user message = %+v, want original regenerated request", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != "fresh streamed answer" {
		t.Fatalf("assistant message = %+v, want regenerated streamed response", messages[1])
	}
}

func TestSendMessageRegenerateCleansDeepResearchAssistantTail(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "regenerate research cleanup")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	originalUserID := seedDeepResearchAssistantTail(t, store, conv.ID, "deep research this topic")

	registry := llm.NewProviderRegistry()
	provider := &requestCaptureProvider{}
	registry.Register(provider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())

	e := echo.New()
	reqBody := `{"message":"deep research this topic","provider":"capture","model":"gpt-5-codex","regenerate":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0].ID != originalUserID || messages[0].Role != "user" {
		t.Fatalf("unexpected preserved user message: %+v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != "ok" {
		t.Fatalf("assistant message = %+v, want regenerated response", messages[1])
	}
}
