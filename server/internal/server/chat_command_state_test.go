package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func requestWithUser(req *http.Request, userID string) *http.Request {
	claims := &auth.UserClaims{
		UserID: userID,
		Role:   "user",
	}
	ctx := context.WithValue(req.Context(), auth.UserContextKey, claims)
	return req.WithContext(ctx)
}

func getBootstrapCommandState(
	t *testing.T,
	handler *ChatHandler,
	e *echo.Echo,
	conversationID string,
	userID string,
) conversationCommandStateResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conversationID+"/bootstrap", nil)
	if strings.TrimSpace(userID) != "" {
		req = requestWithUser(req, userID)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conversationID)
	if strings.TrimSpace(userID) != "" {
		c.Set("user", &auth.Claims{
			UserClaims: auth.UserClaims{
				UserID: userID,
				Role:   "user",
			},
		})
	}

	if err := handler.GetConversationBootstrap(c); err != nil {
		t.Fatalf("GetConversationBootstrap: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		CommandState conversationCommandStateResponse `json:"command_state"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal bootstrap: %v", err)
	}
	return body.CommandState
}

func TestConversationCommandStatePatchSurfacesInBootstrap(t *testing.T) {
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

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/"+conv.ID+"/command-state", bytes.NewBufferString(`{"selected_provider_id":"openai","selected_model_id":"gpt-5","offline":true,"agentcore_runner_ref":"release/v1"}`))
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

	state := getBootstrapCommandState(t, handler, e, conv.ID, "")
	if state.SelectedProviderID != "openai" || state.SelectedModelID != "gpt-5" {
		t.Fatalf("unexpected command state: %+v", state)
	}
	if !state.Offline {
		t.Fatalf("expected offline command state to survive bootstrap: %+v", state)
	}
	if state.AgentcoreRunnerRef != "release/v1" {
		t.Fatalf("expected runner ref to survive bootstrap: %+v", state)
	}
}

func TestConversationCommandStatePatchSharedAcrossConversationsForSameUserInBootstrap(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv1, err := store.CreateConversation(context.Background(), "User A #1", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a #1): %v", err)
	}
	conv2, err := store.CreateConversation(context.Background(), "User A #2", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a #2): %v", err)
	}
	convB, err := store.CreateConversation(context.Background(), "User B #1", "user-b")
	if err != nil {
		t.Fatalf("CreateConversation(user-b #1): %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	e := echo.New()

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/"+conv1.ID+"/command-state", bytes.NewBufferString(`{"selected_provider_id":"openai","selected_model_id":"gpt-5","offline":true}`))
	patchReq = requestWithUser(patchReq, "user-a")
	patchReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	patchRec := httptest.NewRecorder()
	patchCtx := e.NewContext(patchReq, patchRec)
	patchCtx.SetParamNames("id")
	patchCtx.SetParamValues(conv1.ID)
	if err := handler.PatchConversationCommandState(patchCtx); err != nil {
		t.Fatalf("PatchConversationCommandState(user-a #1): %v", err)
	}
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200", patchRec.Code)
	}

	userAState := getBootstrapCommandState(t, handler, e, conv2.ID, "user-a")
	if userAState.SelectedProviderID != "openai" || userAState.SelectedModelID != "gpt-5" {
		t.Fatalf("unexpected shared user-a state: %+v", userAState)
	}
	if !userAState.Offline {
		t.Fatalf("unexpected shared user-a flags: %+v", userAState)
	}

	userBState := getBootstrapCommandState(t, handler, e, convB.ID, "user-b")
	if userBState.SelectedProviderID != "" || userBState.SelectedModelID != "" || userBState.Offline {
		t.Fatalf("unexpected isolated user-b state: %v", userBState)
	}
}

func TestConversationCommandStatePatchKeepsAgentcoreRunnerRefConversationScoped(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	conv1, err := store.CreateConversation(context.Background(), "User A #1", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a #1): %v", err)
	}
	conv2, err := store.CreateConversation(context.Background(), "User A #2", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a #2): %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	e := echo.New()

	patchConv1 := httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/"+conv1.ID+"/command-state", bytes.NewBufferString(`{"selected_provider_id":"openai","selected_model_id":"gpt-5","offline":true,"agentcore_runner_ref":"release/v1"}`))
	patchConv1 = requestWithUser(patchConv1, "user-a")
	patchConv1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	patchConv1Rec := httptest.NewRecorder()
	patchConv1Ctx := e.NewContext(patchConv1, patchConv1Rec)
	patchConv1Ctx.SetParamNames("id")
	patchConv1Ctx.SetParamValues(conv1.ID)
	if err := handler.PatchConversationCommandState(patchConv1Ctx); err != nil {
		t.Fatalf("PatchConversationCommandState(conv1): %v", err)
	}
	if patchConv1Rec.Code != http.StatusOK {
		t.Fatalf("conv1 patch status = %d, want 200", patchConv1Rec.Code)
	}

	patchConv2 := httptest.NewRequest(http.MethodPatch, "/api/v1/conversations/"+conv2.ID+"/command-state", bytes.NewBufferString(`{"agentcore_runner_ref":"release/v2"}`))
	patchConv2 = requestWithUser(patchConv2, "user-a")
	patchConv2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	patchConv2Rec := httptest.NewRecorder()
	patchConv2Ctx := e.NewContext(patchConv2, patchConv2Rec)
	patchConv2Ctx.SetParamNames("id")
	patchConv2Ctx.SetParamValues(conv2.ID)
	if err := handler.PatchConversationCommandState(patchConv2Ctx); err != nil {
		t.Fatalf("PatchConversationCommandState(conv2): %v", err)
	}
	if patchConv2Rec.Code != http.StatusOK {
		t.Fatalf("conv2 patch status = %d, want 200", patchConv2Rec.Code)
	}

	state1 := getBootstrapCommandState(t, handler, e, conv1.ID, "user-a")
	if state1.SelectedProviderID != "openai" || state1.SelectedModelID != "gpt-5" || !state1.Offline {
		t.Fatalf("unexpected shared state for conv1: %+v", state1)
	}
	if state1.AgentcoreRunnerRef != "release/v1" {
		t.Fatalf("conv1 runner ref = %q, want release/v1", state1.AgentcoreRunnerRef)
	}

	state2 := getBootstrapCommandState(t, handler, e, conv2.ID, "user-a")
	if state2.SelectedProviderID != "openai" || state2.SelectedModelID != "gpt-5" || !state2.Offline {
		t.Fatalf("unexpected shared state for conv2: %+v", state2)
	}
	if state2.AgentcoreRunnerRef != "release/v2" {
		t.Fatalf("conv2 runner ref = %q, want release/v2", state2.AgentcoreRunnerRef)
	}
}

func TestApplyCommandStateToRequestLeavesLegacyToolFlagsNilWithoutPersistedState(t *testing.T) {
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
	if !state.WebSearchEnabled || !state.DeepResearchEnabled {
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
	if req.WebSearchEnabled != nil || req.DeepResearchEnabled != nil || req.ResearchModeEnabled != nil {
		t.Fatalf("expected persisted legacy tool flags to stay ignored, got web=%v deep=%v research=%v", req.WebSearchEnabled, req.DeepResearchEnabled, req.ResearchModeEnabled)
	}
}

func TestApplyCommandStateToRequestIgnoresExplicitLegacyCapabilityFlags(t *testing.T) {
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
	webSearchEnabled := false
	researchModeEnabled := true
	req := &SendMessageRequest{
		WebSearchEnabled:    &webSearchEnabled,
		ResearchModeEnabled: &researchModeEnabled,
	}

	state := handler.applyCommandStateToRequest(ctx, conv.ID, req)
	if !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("expected legacy request flags to be ignored, got %+v", state)
	}
	if req.WebSearchEnabled != nil || req.DeepResearchEnabled != nil || req.ResearchModeEnabled != nil {
		t.Fatalf("expected legacy capability flags to be cleared from request, got web=%v deep=%v research=%v", req.WebSearchEnabled, req.DeepResearchEnabled, req.ResearchModeEnabled)
	}

	persisted, err := handler.getConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("getConversationCommandState: %v", err)
	}
	if !persisted.WebSearchEnabled || !persisted.DeepResearchEnabled {
		t.Fatalf("expected normalized command state to stay fully enabled for local tools, got %+v", persisted)
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
	if state.SelectedProviderID != "" || state.SelectedModelID != "" || state.Offline || !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("expected cleared command state, got %+v", state)
	}
}

func TestRememberLastGoodRoutePersistsConversationScopedState(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv1, err := store.CreateConversation(ctx, "User A #1", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a #1): %v", err)
	}
	conv2, err := store.CreateConversation(ctx, "User A #2", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a #2): %v", err)
	}

	if err := store.UpsertConversationCommandState(ctx, memory.ConversationCommandState{
		ConversationID:      conv1.ID,
		SelectedProviderID:  "openai",
		SelectedModelID:     "gpt-5",
		AgentcoreRunnerRef:  "release/v1",
		WebSearchEnabled:    true,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState(conv1): %v", err)
	}
	if err := store.UpsertConversationCommandState(ctx, memory.ConversationCommandState{
		ConversationID:      conv2.ID,
		SelectedProviderID:  "openai",
		SelectedModelID:     "gpt-5",
		AgentcoreRunnerRef:  "release/v2",
		WebSearchEnabled:    true,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState(conv2): %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.rememberLastGoodRoute(ctx, conv1.ID, "openrouter", "gpt-4o-mini", "lite_native")

	state1, err := store.GetConversationCommandState(ctx, conv1.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(conv1): %v", err)
	}
	if state1.LastGoodProviderID != "openrouter" || state1.LastGoodModelID != "gpt-4o-mini" {
		t.Fatalf("unexpected last-good route on conv1: %+v", state1)
	}
	if state1.LastGoodNativeSurfaceMode != "lite_native" {
		t.Fatalf("conv1 native surface mode = %q, want lite_native", state1.LastGoodNativeSurfaceMode)
	}
	if state1.SelectedProviderID != "openai" || state1.SelectedModelID != "gpt-5" {
		t.Fatalf("expected selected route to stay intact on conv1, got %+v", state1)
	}
	if state1.AgentcoreRunnerRef != "release/v1" {
		t.Fatalf("expected runner ref to stay conversation-scoped on conv1, got %+v", state1)
	}

	state2, err := store.GetConversationCommandState(ctx, conv2.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(conv2): %v", err)
	}
	if state2.LastGoodProviderID != "" || state2.LastGoodModelID != "" || state2.LastGoodNativeSurfaceMode != "" {
		t.Fatalf("expected conv2 last-good route to stay untouched, got %+v", state2)
	}
}
