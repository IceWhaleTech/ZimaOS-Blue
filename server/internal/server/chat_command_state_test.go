package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestConversationCommandStateAPI(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	e := echo.New()

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/"+conv.ID+"/command-state", bytes.NewBufferString(`{"selected_provider_id":"openai","selected_model_id":"gpt-5","offline":true,"web_search_enabled":false,"deep_research_enabled":true}`))
	patchReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	patchRec := httptest.NewRecorder()
	patchCtx := e.NewContext(patchReq, patchRec)
	patchCtx.SetParamNames("id")
	patchCtx.SetParamValues(conv.ID)
	if err := handler.PatchConversationCommandState(patchCtx); err != nil {
		t.Fatalf("PatchConversationCommandState: %v", err)
	}
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200", patchRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/command-state", nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(conv.ID)
	if err := handler.GetConversationCommandState(getCtx); err != nil {
		t.Fatalf("GetConversationCommandState: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getRec.Code)
	}
	var state map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &state); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if state["selected_provider_id"] != "openai" || state["selected_model_id"] != "gpt-5" {
		t.Fatalf("unexpected command state: %v", state)
	}
	if state["web_search_enabled"] != false || state["deep_research_enabled"] != true {
		t.Fatalf("unexpected preference flags: %v", state)
	}
}

func TestApplyCommandStateToRequestLeavesFallbackFlagsNilWithoutPersistedState(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "Test Conv")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	req := &SendMessageRequest{}
	state := handler.applyCommandStateToRequest(ctx, conv.ID, req)
	if req.WebSearchEnabled != nil || req.DeepResearchEnabled != nil {
		t.Fatalf("expected nil request flags without persisted state, got web=%v deep=%v", req.WebSearchEnabled, req.DeepResearchEnabled)
	}
	if !state.WebSearchEnabled || state.DeepResearchEnabled {
		t.Fatalf("expected default response state, got %+v", state)
	}

	if err := store.UpsertConversationCommandState(ctx, memory.ConversationCommandState{
		ConversationID:      conv.ID,
		WebSearchEnabled:    false,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState: %v", err)
	}

	req = &SendMessageRequest{}
	handler.applyCommandStateToRequest(ctx, conv.ID, req)
	if req.WebSearchEnabled == nil || *req.WebSearchEnabled {
		t.Fatalf("expected persisted web flag false, got %v", req.WebSearchEnabled)
	}
	if req.DeepResearchEnabled == nil || !*req.DeepResearchEnabled {
		t.Fatalf("expected persisted deep flag true, got %v", req.DeepResearchEnabled)
	}
}

func TestChatHandlerClearResetsCommandState(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "Test Conv")
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
	if _, err := store.AddMessage(ctx, conv.ID, memory.Message{Role: "user", Content: "hello"}); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"/clear","provider":"","model":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)
	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Conversation cleared") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	state, err := store.GetConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState: %v", err)
	}
	if state.SelectedProviderID != "" || state.SelectedModelID != "" || state.Offline || !state.WebSearchEnabled || state.DeepResearchEnabled {
		t.Fatalf("expected cleared command state, got %+v", state)
	}
}
