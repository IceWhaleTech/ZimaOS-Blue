package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
)

// Test ChatHandler creation
func TestNewChatHandler(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()

	handler := NewChatHandler(store, registry, toolRegistry)
	if handler == nil {
		t.Fatal("expected handler, got nil")
	}
}

// Test CreateConversation endpoint
func TestChatHandlerCreateConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"title": "Test Conversation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["id"] == nil {
		t.Error("expected id in response")
	}
	if resp["title"] != "Test Conversation" {
		t.Errorf("expected title 'Test Conversation', got '%v'", resp["title"])
	}
}

// Test ListConversations endpoint
func TestChatHandlerListConversations(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	// Create some conversations
	store.CreateConversation(context.Background(), "Conv 1")
	store.CreateConversation(context.Background(), "Conv 2")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListConversations(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(resp))
	}
}

// Test GetConversation endpoint
func TestChatHandlerGetConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["id"] != conv.ID {
		t.Errorf("expected id '%s', got '%v'", conv.ID, resp["id"])
	}
}

// Test GetConversation not found
func TestChatHandlerGetConversationNotFound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := handler.GetConversation(c)
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

// Test DeleteConversation endpoint
func TestChatHandlerDeleteConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "To Delete")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+conv.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.DeleteConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

// Test GetMessages endpoint
func TestChatHandlerGetMessages(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Hello"})
	store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "Hi!"})

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/messages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetMessages(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Errorf("expected 2 messages, got %d", len(resp))
	}
}

// Test SendMessage endpoint with mock provider
func TestChatHandlerSendMessage(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-123",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Hello! How can I help?"},
	})
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "mock", "model": "mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["content"] != "Hello! How can I help?" {
		t.Errorf("expected content 'Hello! How can I help?', got '%v'", resp["content"])
	}
}

// Test SendMessage with invalid provider
func TestChatHandlerSendMessageInvalidProvider(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "nonexistent", "model": "model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

// Test ListProviders endpoint
func TestChatHandlerListProviders(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	registry.Register(llm.NewMockProvider())

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListProviders(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 1 {
		t.Errorf("expected 1 provider, got %d", len(resp))
	}
}

// Test ListTools endpoint
func TestChatHandlerListTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)

	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListTools(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) < 3 {
		t.Errorf("expected at least 3 tools, got %d", len(resp))
	}
}

// Test StreamMessage with Provider Pool ID mapping
func TestChatHandlerStreamMessageWithProviderPoolID(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	// Use CustomProvider which has Name() = "custom"
	// Provider Pool IDs that don't match known mappings will map to "custom"
	customProvider := llm.NewCustomProvider("test-key", "http://localhost:8080")
	registry.Register(customProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	// Use a Provider Pool ID format (prov_<hex>)
	// This should be mapped to "custom" by mapProviderID()
	reqBody := `{"message": "Hello!", "provider": "prov_992ef6e8c8ad938a", "model": "test-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.StreamMessage(c)
	// The provider should be found (mapped to "custom")
	// We expect an error from the actual API call (since we're using a fake endpoint),
	// but NOT a "provider not found" error
	if err != nil {
		httpErr, ok := err.(*echo.HTTPError)
		if ok && httpErr.Code == http.StatusBadRequest {
			// Check if it's the "provider not found" error
			if msg, ok := httpErr.Message.(string); ok && bytes.Contains([]byte(msg), []byte("provider not found")) {
				t.Fatalf("StreamMessage should map Provider Pool ID to LLM provider name, but got error: %v", err)
			}
		}
		// Other errors are acceptable (e.g., API call failures, streaming setup issues)
		// The important thing is that the provider was found
	}
}
