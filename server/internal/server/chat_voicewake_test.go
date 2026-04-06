package server

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voicewake"
)

func TestSubmitVoiceWakeMessageTargetUnavailable(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())

	err = handler.SubmitVoiceWakeMessage(context.Background(), "missing-conversation", "hello")
	if err != voicewake.ErrTargetUnavailable {
		t.Fatalf("SubmitVoiceWakeMessage() error = %v, want %v", err, voicewake.ErrTargetUnavailable)
	}
}

func TestSubmitVoiceWakeMessageInjectsIntoActiveStream(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "VoiceWake", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.convStreamMu.Lock()
	handler.convToStream[conv.ID] = "stream-1"
	handler.convStreamMu.Unlock()

	if err := handler.SubmitVoiceWakeMessage(context.Background(), conv.ID, "continue the build"); err != nil {
		t.Fatalf("SubmitVoiceWakeMessage() error = %v", err)
	}

	handler.injectionsMu.Lock()
	ch := handler.injections[conv.ID]
	handler.injectionsMu.Unlock()
	if ch == nil {
		t.Fatal("expected injection channel")
	}
	select {
	case got := <-ch:
		if got != "continue the build" {
			t.Fatalf("injected message = %q, want %q", got, "continue the build")
		}
	default:
		t.Fatal("expected injected message")
	}
}

func TestSubmitVoiceWakeMessageUsesNormalSendPipeline(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "VoiceWake", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if err := store.UpsertConversationCommandState(context.Background(), memory.ConversationCommandState{
		ConversationID:      conv.ID,
		Offline:             true,
		WebSearchEnabled:    true,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState() error = %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	if err := handler.SubmitVoiceWakeMessage(context.Background(), conv.ID, "hello from voice wake"); err != nil {
		t.Fatalf("SubmitVoiceWakeMessage() error = %v", err)
	}

	msgs, err := store.GetMessages(context.Background(), conv.ID, 10, 0, "user-1")
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("message count = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello from voice wake" {
		t.Fatalf("user message = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Model != "offline" {
		t.Fatalf("assistant message = %+v", msgs[1])
	}
}
