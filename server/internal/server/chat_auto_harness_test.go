package server

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type autoHarnessSubmitterSpy struct {
	specs []AutoHarnessQuickEvalSpec
}

func (s *autoHarnessSubmitterSpy) SubmitAutoHarnessConversationQuickEval(_ context.Context, spec AutoHarnessQuickEvalSpec) (string, error) {
	s.specs = append(s.specs, spec)
	return "group-auto-1", nil
}

type autoHarnessEventSpy struct {
	events []autoHarnessPublishedEvent
}

type autoHarnessPublishedEvent struct {
	userID    string
	eventType string
	data      any
}

func (s *autoHarnessEventSpy) Publish(userID string, eventType string, data any) {
	s.events = append(s.events, autoHarnessPublishedEvent{
		userID:    userID,
		eventType: eventType,
		data:      data,
	})
}

func stringSliceFromAnyTest(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func TestAutoHarnessTurnHook_SubmitsConversationQuickEvalForToolBackedCompletedTurn(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Release candidate fix", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	userMsg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Finish and verify the fix",
	})
	if err != nil {
		t.Fatalf("AddMessage(user): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "tool",
		ToolName: "file_write",
		Content:  `{"path":"result.txt","ok":true}`,
	}); err != nil {
		t.Fatalf("AddMessage(tool): %v", err)
	}
	assistantMsg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Implemented and verified the fix.",
		Provider: "openai",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("AddMessage(assistant): %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	defer handler.Close()
	submitter := &autoHarnessSubmitterSpy{}
	hook := NewAutoHarnessTurnHook(handler, submitter)

	if err := hook.AfterAssistantPersisted(context.Background(), TurnContext{
		ConversationID: conv.ID,
		UserMessage:    "Finish and verify the fix",
		Model:          "gpt-5.4-mini",
	}, assistantMsg); err != nil {
		t.Fatalf("AfterAssistantPersisted: %v", err)
	}

	if len(submitter.specs) != 1 {
		t.Fatalf("submitted spec count = %d, want 1", len(submitter.specs))
	}
	spec := submitter.specs[0]
	if got, _ := spec.Metadata["auto_harness"].(bool); !got {
		t.Fatalf("spec auto_harness = %#v, want true", spec.Metadata["auto_harness"])
	}
	if got := spec.Metadata["quick_eval_preset"]; got != "research" {
		t.Fatalf("quick_eval_preset = %#v, want research", got)
	}
	if got := spec.Metadata["source_ref"]; got != conv.ID {
		t.Fatalf("source_ref = %#v, want %q", got, conv.ID)
	}
	if spec.Subject != "research" {
		t.Fatalf("subject = %q, want research", spec.Subject)
	}
	if len(spec.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(spec.Items))
	}
	item := spec.Items[0]
	if item.RunKind != "research" {
		t.Fatalf("item run kind = %q, want research", item.RunKind)
	}
	if item.Profile != "research" {
		t.Fatalf("item profile = %q, want research", item.Profile)
	}
	if got := item.Input["goal"]; got != "Finish and verify the fix" {
		t.Fatalf("item goal = %#v, want exact user goal", got)
	}
	if got := item.Metadata["conversation_id"]; got != conv.ID {
		t.Fatalf("item conversation_id = %#v, want %q", got, conv.ID)
	}
	if got := item.Metadata["user_message_id"]; got != userMsg.ID {
		t.Fatalf("item user_message_id = %#v, want %q", got, userMsg.ID)
	}
	if got := item.Metadata["assistant_message_id"]; got != assistantMsg.ID {
		t.Fatalf("item assistant_message_id = %#v, want %q", got, assistantMsg.ID)
	}
	if got := item.Expected["status"]; got != "completed" {
		t.Fatalf("item expected status = %#v, want completed", got)
	}
}

func TestAutoHarnessTurnHook_CapturesContextPackSnapshotInQuickEvalMetadata(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Responses docs fix", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Finish and verify the Responses API fix",
	}); err != nil {
		t.Fatalf("AddMessage(user): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "tool",
		ToolName: "file_write",
		Content:  `{"path":"result.txt","ok":true}`,
	}); err != nil {
		t.Fatalf("AddMessage(tool): %v", err)
	}
	assistantMsg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Implemented and verified the Responses API fix.",
		Provider: "openai",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("AddMessage(assistant): %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	defer handler.Close()
	submitter := &autoHarnessSubmitterSpy{}
	hook := NewAutoHarnessTurnHook(handler, submitter)

	selection := &contextpack.SelectionSet{
		SelectedSkill: "web_query",
		TotalTokens:   420,
		Files: []contextpack.SelectedFile{{
			EntryID:     "openai/docs/responses-api",
			Type:        contextpack.EntryTypeDoc,
			SourceTrust: contextpack.SourceTrustOfficial,
			Language:    "en-US",
			Version:     "latest",
			File:        "references/tools.md",
			SHA256:      "abc123",
			Tokens:      420,
			Annotated:   true,
		}},
	}

	if err := hook.AfterAssistantPersisted(context.Background(), TurnContext{
		ConversationID:       conv.ID,
		UserMessage:          "Finish and verify the Responses API fix",
		Model:                "gpt-5.4-mini",
		ContextPackSelection: selection,
	}, assistantMsg); err != nil {
		t.Fatalf("AfterAssistantPersisted: %v", err)
	}

	if len(submitter.specs) != 1 {
		t.Fatalf("submitted spec count = %d, want 1", len(submitter.specs))
	}
	spec := submitter.specs[0]
	groupSnapshot, ok := spec.Metadata["contextpack_snapshot"].(map[string]interface{})
	if !ok {
		t.Fatalf("group contextpack_snapshot type = %T, want map[string]interface{}", spec.Metadata["contextpack_snapshot"])
	}
	if got := groupSnapshot["selected_count"]; got != float64(1) && got != 1 {
		t.Fatalf("group selected_count = %#v, want 1", got)
	}
	if got := groupSnapshot["selected_skill"]; got != "web_query" {
		t.Fatalf("group selected_skill = %#v, want web_query", got)
	}
	if got := groupSnapshot["selection_digest"]; got == "" || got == nil {
		t.Fatalf("group selection_digest = %#v, want non-empty", got)
	}

	itemSnapshot, ok := spec.Items[0].Metadata["contextpack_snapshot"].(map[string]interface{})
	if !ok {
		t.Fatalf("item contextpack_snapshot type = %T, want map[string]interface{}", spec.Items[0].Metadata["contextpack_snapshot"])
	}
	files, ok := itemSnapshot["files"].([]map[string]interface{})
	if ok {
		if len(files) != 1 {
			t.Fatalf("item snapshot files len = %d, want 1", len(files))
		}
	} else {
		rawFiles, ok := itemSnapshot["files"].([]interface{})
		if !ok || len(rawFiles) != 1 {
			t.Fatalf("item snapshot files = %#v, want single file", itemSnapshot["files"])
		}
		file, ok := rawFiles[0].(map[string]interface{})
		if !ok {
			t.Fatalf("item snapshot file[0] type = %T, want map[string]interface{}", rawFiles[0])
		}
		if got := file["entry_id"]; got != "openai/docs/responses-api" {
			t.Fatalf("item snapshot file entry_id = %#v, want openai/docs/responses-api", got)
		}
		if got := file["source_trust"]; got != "official" {
			t.Fatalf("item snapshot file source_trust = %#v, want official", got)
		}
	}
}

func TestAutoHarnessTurnHook_SkipsPlainChatTurnsWithoutToolEvidence(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Small talk", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Can you verify this?",
	}); err != nil {
		t.Fatalf("AddMessage(user): %v", err)
	}
	assistantMsg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Verified.",
		Provider: "openai",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("AddMessage(assistant): %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	defer handler.Close()
	submitter := &autoHarnessSubmitterSpy{}
	hook := NewAutoHarnessTurnHook(handler, submitter)

	if err := hook.AfterAssistantPersisted(context.Background(), TurnContext{
		ConversationID: conv.ID,
		UserMessage:    "Can you verify this?",
		Model:          "gpt-5.4-mini",
	}, assistantMsg); err != nil {
		t.Fatalf("AfterAssistantPersisted: %v", err)
	}

	if len(submitter.specs) != 0 {
		t.Fatalf("submitted spec count = %d, want 0", len(submitter.specs))
	}
}

func TestAutoHarnessTurnHook_UsesResearchPresetForResearchEvidence(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Investigate release regression", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := store.UpsertConversationCommandState(context.Background(), memory.ConversationCommandState{
		ConversationID:      conv.ID,
		WebSearchEnabled:    true,
		DeepResearchEnabled: true,
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Investigate the regression and verify the sources",
	}); err != nil {
		t.Fatalf("AddMessage(user): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "tool",
		ToolName: "web_search",
		Content:  `{"query":"release regression"}`,
	}); err != nil {
		t.Fatalf("AddMessage(tool): %v", err)
	}
	assistantMsg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Finished researching the regression and verified the findings with sources.",
		Provider: "openai",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("AddMessage(assistant): %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	defer handler.Close()
	submitter := &autoHarnessSubmitterSpy{}
	hook := NewAutoHarnessTurnHook(handler, submitter)

	if err := hook.AfterAssistantPersisted(context.Background(), TurnContext{
		ConversationID: conv.ID,
		UserMessage:    "Investigate the regression and verify the sources",
		Model:          "gpt-5.4-mini",
	}, assistantMsg); err != nil {
		t.Fatalf("AfterAssistantPersisted: %v", err)
	}

	if len(submitter.specs) != 1 {
		t.Fatalf("submitted spec count = %d, want 1", len(submitter.specs))
	}
	spec := submitter.specs[0]
	if got := spec.Metadata["quick_eval_preset"]; got != "research" {
		t.Fatalf("quick_eval_preset = %#v, want research", got)
	}
	if spec.Subject != "research" {
		t.Fatalf("subject = %q, want research", spec.Subject)
	}
	if len(spec.Items) != 1 {
		t.Fatalf("item count = %d, want 1", len(spec.Items))
	}
	item := spec.Items[0]
	if item.RunKind != "research" {
		t.Fatalf("item run kind = %q, want research", item.RunKind)
	}
	if item.Profile != "research" {
		t.Fatalf("item profile = %q, want research", item.Profile)
	}
	requiredObservations := stringSliceFromAnyTest(item.Expected["required_observations"])
	if len(requiredObservations) != 1 || requiredObservations[0] != "evidence_tool_used" {
		t.Fatalf("required_observations = %#v, want [evidence_tool_used]", item.Expected["required_observations"])
	}
}

func TestAutoHarnessTurnHook_PublishesTaskCreatedEventForChatRefresh(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Ship checklist", "user-42")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Finish and verify the checklist",
	}); err != nil {
		t.Fatalf("AddMessage(user): %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "tool",
		ToolName: "file_write",
		Content:  `{"path":"checklist.md","ok":true}`,
	}); err != nil {
		t.Fatalf("AddMessage(tool): %v", err)
	}
	assistantMsg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Implemented and verified the checklist.",
		Provider: "openai",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("AddMessage(assistant): %v", err)
	}

	handler := NewChatHandler(store, nil, tools.NewRegistry())
	defer handler.Close()
	eventSpy := &autoHarnessEventSpy{}
	handler.SetSSEBroker(eventSpy)

	submitter := &autoHarnessSubmitterSpy{}
	hook := NewAutoHarnessTurnHook(handler, submitter)

	if err := hook.AfterAssistantPersisted(context.Background(), TurnContext{
		ConversationID: conv.ID,
		UserMessage:    "Finish and verify the checklist",
		Model:          "gpt-5.4-mini",
	}, assistantMsg); err != nil {
		t.Fatalf("AfterAssistantPersisted: %v", err)
	}

	if len(eventSpy.events) != 1 {
		t.Fatalf("published event count = %d, want 1", len(eventSpy.events))
	}
	event := eventSpy.events[0]
	if event.userID != "user-42" {
		t.Fatalf("event user_id = %q, want user-42", event.userID)
	}
	if event.eventType != "task_created" {
		t.Fatalf("event type = %q, want task_created", event.eventType)
	}
	payload, ok := event.data.(map[string]interface{})
	if !ok {
		t.Fatalf("event payload type = %T, want map[string]interface{}", event.data)
	}
	if got := payload["task_id"]; got != "group-auto-1" {
		t.Fatalf("event task_id = %#v, want group-auto-1", got)
	}
	if got := payload["conversation_id"]; got != conv.ID {
		t.Fatalf("event conversation_id = %#v, want %q", got, conv.ID)
	}
	if got := payload["assistant_message_id"]; got != assistantMsg.ID {
		t.Fatalf("event assistant_message_id = %#v, want %q", got, assistantMsg.ID)
	}
	if got, _ := payload["auto_harness"].(bool); !got {
		t.Fatalf("event auto_harness = %#v, want true", payload["auto_harness"])
	}
	if got := payload["quick_eval_preset"]; got != "research" {
		t.Fatalf("event quick_eval_preset = %#v, want research", got)
	}
}
