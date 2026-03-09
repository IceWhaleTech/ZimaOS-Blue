package memory

import (
	"context"
	"testing"
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
	if state.SelectedProviderID != "" || state.SelectedModelID != "" || state.Offline || !state.WebSearchEnabled || state.DeepResearchEnabled {
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
	if state.SelectedProviderID != "openai" || state.SelectedModelID != "gpt-5" || !state.Offline || state.WebSearchEnabled || !state.DeepResearchEnabled {
		t.Fatalf("unexpected stored state: %+v", state)
	}

	if err := store.ClearConversationCommandState(ctx, conv.ID); err != nil {
		t.Fatalf("ClearConversationCommandState: %v", err)
	}
	state, err = store.GetConversationCommandState(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationCommandState(after clear): %v", err)
	}
	if state.SelectedProviderID != "" || state.SelectedModelID != "" || state.Offline || !state.WebSearchEnabled || state.DeepResearchEnabled {
		t.Fatalf("unexpected cleared state: %+v", state)
	}
}
