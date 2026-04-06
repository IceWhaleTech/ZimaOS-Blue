package memory

import (
	"context"
	"testing"
	"time"
)

func TestConversationCommandStateRoundTrip(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conv, err := store.CreateConversation(ctx, "test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	state, err := store.GetConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(default): %v", err)
	}
	if state.SelectedProviderID != "" || state.SelectedModelID != "" || state.Offline || !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("unexpected default state: %+v", state)
	}

	err = store.UpsertConversationCommandState(ctx, ConversationCommandState{
		ConversationID:      conv.ID,
		SelectedProviderID:  "openai",
		SelectedModelID:     "gpt-5",
		Offline:             true,
		WebSearchEnabled:    false,
		DeepResearchEnabled: true,
	})
	if err != nil {
		t.Fatalf("UpsertConversationCommandState: %v", err)
	}

	state, err = store.GetConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState: %v", err)
	}
	if state.SelectedProviderID != "openai" || state.SelectedModelID != "gpt-5" || !state.Offline || !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("unexpected stored state: %+v", state)
	}

	if err := store.ClearConversationCommandState(ctx, conv.ID); err != nil {
		t.Fatalf("ClearConversationCommandState: %v", err)
	}
	state, err = store.GetConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(after clear): %v", err)
	}
	if state.SelectedProviderID != "" || state.SelectedModelID != "" || state.Offline || !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("unexpected cleared state: %+v", state)
	}
}

func TestConversationCommandStateSharedByUser(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	convA1, err := store.CreateConversation(ctx, "user-a-1", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a-1): %v", err)
	}
	convA2, err := store.CreateConversation(ctx, "user-a-2", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a-2): %v", err)
	}
	convB, err := store.CreateConversation(ctx, "user-b-1", "user-b")
	if err != nil {
		t.Fatalf("CreateConversation(user-b-1): %v", err)
	}

	if err := store.UpsertConversationCommandState(ctx, ConversationCommandState{
		ConversationID:      convA1.ID,
		SelectedProviderID:  "openai",
		SelectedModelID:     "gpt-5",
		Offline:             true,
		WebSearchEnabled:    false,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState(user-a): %v", err)
	}

	sharedA2, err := store.GetConversationCommandState(ctx, convA2.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(user-a-2): %v", err)
	}
	if sharedA2.ConversationID != convA2.ID {
		t.Fatalf("ConversationID = %q, want %q", sharedA2.ConversationID, convA2.ID)
	}
	if sharedA2.SelectedProviderID != "openai" || sharedA2.SelectedModelID != "gpt-5" || !sharedA2.Offline || !sharedA2.WebSearchEnabled || !sharedA2.DeepResearchEnabled {
		t.Fatalf("unexpected shared state for user-a: %+v", sharedA2)
	}

	isolatedB, err := store.GetConversationCommandState(ctx, convB.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(user-b): %v", err)
	}
	if isolatedB.SelectedProviderID != "" || isolatedB.SelectedModelID != "" || isolatedB.Offline || !isolatedB.WebSearchEnabled || !isolatedB.DeepResearchEnabled {
		t.Fatalf("expected isolated default for user-b, got %+v", isolatedB)
	}

	if err := store.UpsertConversationCommandState(ctx, ConversationCommandState{
		ConversationID:      convA2.ID,
		SelectedProviderID:  "anthropic",
		SelectedModelID:     "claude-3.7",
		Offline:             false,
		WebSearchEnabled:    true,
		DeepResearchEnabled: false,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState(user-a update): %v", err)
	}

	sharedA1, err := store.GetConversationCommandState(ctx, convA1.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(user-a-1): %v", err)
	}
	if sharedA1.SelectedProviderID != "anthropic" || sharedA1.SelectedModelID != "claude-3.7" || sharedA1.Offline || !sharedA1.WebSearchEnabled || !sharedA1.DeepResearchEnabled {
		t.Fatalf("unexpected updated shared state for user-a: %+v", sharedA1)
	}

	if err := store.ClearConversationCommandState(ctx, convA1.ID); err != nil {
		t.Fatalf("ClearConversationCommandState(user-a): %v", err)
	}
	clearedA2, err := store.GetConversationCommandState(ctx, convA2.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(user-a-2 after clear): %v", err)
	}
	if clearedA2.SelectedProviderID != "" || clearedA2.SelectedModelID != "" || clearedA2.Offline || !clearedA2.WebSearchEnabled || !clearedA2.DeepResearchEnabled {
		t.Fatalf("unexpected cleared shared state: %+v", clearedA2)
	}
}

func TestConversationCommandStateFallsBackToLegacyUserScopedRows(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	legacyConv, err := store.CreateConversation(ctx, "legacy", "user-legacy")
	if err != nil {
		t.Fatalf("CreateConversation(legacy): %v", err)
	}
	probeConv, err := store.CreateConversation(ctx, "probe", "user-legacy")
	if err != nil {
		t.Fatalf("CreateConversation(probe): %v", err)
	}

	if _, err := store.db.ExecContext(ctx,
		`INSERT INTO conversation_command_state (conversation_id, selected_provider_id, selected_model_id, offline, web_search_enabled, deep_research_enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		legacyConv.ID,
		"legacy-provider",
		"legacy-model",
		true,
		false,
		true,
		time.Now(),
	); err != nil {
		t.Fatalf("insert legacy conversation_command_state: %v", err)
	}

	state, err := store.GetConversationCommandState(ctx, probeConv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(legacy fallback): %v", err)
	}
	if state.ConversationID != probeConv.ID {
		t.Fatalf("ConversationID = %q, want %q", state.ConversationID, probeConv.ID)
	}
	if state.SelectedProviderID != "legacy-provider" || state.SelectedModelID != "legacy-model" || !state.Offline || !state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("unexpected legacy fallback state: %+v", state)
	}
}
