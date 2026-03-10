package server

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestDoWarmupWithToken_DropsStaleResult(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "warmup stale")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	token := handler.armWarmupToken(conv.ID)
	handler.clearWarmupToken(conv.ID)
	handler.doWarmupWithToken(conv.ID, token)

	if got := handler.consumeWarmup(conv.ID); got != nil {
		t.Fatal("expected stale warmup result to be discarded")
	}
}

func TestAfterAssistantPersistedHooks_SchedulesNextTurnWarmup(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "warmup next turn")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	handler.afterAssistantPersistedHooks(TurnContext{ConversationID: conv.ID}, &memory.Message{
		ID:             "assistant-1",
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "done",
	})

	deadline := time.After(2 * time.Second)
	for {
		if warmup := handler.consumeWarmup(conv.ID); warmup != nil {
			return
		}
		select {
		case <-deadline:
			t.Fatal("expected next-turn warmup to be prepared asynchronously")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
