package chatcmd

import (
	"context"
	"strings"
	"testing"
)

type fakeDeps struct {
	state        CommandState
	providers    []ProviderInfo
	models       []ModelInfo
	messages     int
	runtime      *LatestAssistantMessage
	streamActive bool
	title        string
	resetCount   int
}

func (f *fakeDeps) executor() *Executor {
	return NewExecutor(Deps{
		GetConversationTitle:    func(ctx context.Context, conversationID string) (string, error) { return f.title, nil },
		UpdateConversationTitle: func(ctx context.Context, conversationID, title string) error { f.title = title; return nil },
		CountMessages:           func(ctx context.Context, conversationID string) (int, error) { return f.messages, nil },
		GetLatestAssistant: func(ctx context.Context, conversationID string) (*LatestAssistantMessage, error) {
			return f.runtime, nil
		},
		GetCommandState:   func(ctx context.Context, conversationID string) (CommandState, error) { return f.state, nil },
		SaveCommandState:  func(ctx context.Context, state CommandState) error { f.state = state; return nil },
		ResetConversation: func(ctx context.Context, conversationID string) (int, error) { f.resetCount++; return f.messages, nil },
		HasActiveStream:   func(conversationID string) bool { return f.streamActive },
		StopActiveStream: func(conversationID string) bool {
			if !f.streamActive {
				return false
			}
			f.streamActive = false
			return true
		},
		ListProviders: func(ctx context.Context) ([]ProviderInfo, error) { return f.providers, nil },
		ListModels:    func(ctx context.Context) ([]ModelInfo, error) { return f.models, nil },
	})
}

func TestParseSupportsColonSyntaxAndAliases(t *testing.T) {
	deps := &fakeDeps{}
	parsed, ok := deps.executor().Parse("/commands: now")
	if !ok {
		t.Fatal("expected parse success")
	}
	if parsed.Name != "help" || parsed.Args != "now" {
		t.Fatalf("unexpected parsed command: %+v", parsed)
	}
}

func TestModelAmbiguityRequiresProviderModel(t *testing.T) {
	deps := &fakeDeps{models: []ModelInfo{{ID: "gpt-5", ProviderID: "openai"}, {ID: "gpt-5", ProviderID: "azure"}}}
	result, handled := deps.executor().Execute(context.Background(), "conv-1", "/model gpt-5")
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if !strings.Contains(strings.ToLower(result.Content), "ambiguous") {
		t.Fatalf("unexpected result: %s", result.Content)
	}
}

func TestModelExplicitProviderModelUpdatesState(t *testing.T) {
	deps := &fakeDeps{
		state:     CommandState{ConversationID: "conv-1", WebSearchEnabled: true},
		providers: []ProviderInfo{{ID: "openai", Enabled: true}},
		models:    []ModelInfo{{ID: "gpt-5", ProviderID: "openai"}},
	}
	result, handled := deps.executor().Execute(context.Background(), "conv-1", "/model openai/gpt-5")
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if deps.state.SelectedProviderID != "openai" || deps.state.SelectedModelID != "gpt-5" {
		t.Fatalf("unexpected state: %+v", deps.state)
	}
	if !strings.Contains(result.Content, "openai/gpt-5") {
		t.Fatalf("unexpected result: %s", result.Content)
	}
}

func TestProviderStatusAndStop(t *testing.T) {
	deps := &fakeDeps{
		state:        CommandState{ConversationID: "conv-1", SelectedProviderID: "openai", WebSearchEnabled: true},
		providers:    []ProviderInfo{{ID: "openai", Status: "active", Location: "cloud", BaseURL: "https://api.example.com", APIFormat: "openai", ModelCount: 2, Enabled: true}},
		streamActive: true,
	}
	result, _ := deps.executor().Execute(context.Background(), "conv-1", "/provider status")
	if !strings.Contains(result.Content, "openai") || !strings.Contains(result.Content, "active") {
		t.Fatalf("unexpected provider status: %s", result.Content)
	}
	result, _ = deps.executor().Execute(context.Background(), "conv-1", "/stop")
	if !strings.Contains(strings.ToLower(result.Content), "stopped") {
		t.Fatalf("unexpected stop result: %s", result.Content)
	}
}

func TestLegacyLocalToolToggleCommandsRemoved(t *testing.T) {
	deps := &fakeDeps{}

	for _, command := range []string{"/deep on", "/research status", "/web off"} {
		result, handled := deps.executor().Execute(context.Background(), "conv-1", command)
		if !handled {
			t.Fatalf("expected %q to surface as an unknown command", command)
		}
		if !strings.Contains(result.Content, "Unknown command") {
			t.Fatalf("expected unknown command result for %q, got %q", command, result.Content)
		}
	}
}

func TestHelpOmitsRemovedLocalToolToggleCommands(t *testing.T) {
	result, handled := (&fakeDeps{}).executor().Execute(context.Background(), "conv-1", "/help")
	if !handled {
		t.Fatal("expected help command to be handled")
	}
	for _, removed := range []string{"/research on|off|status", "/deep on|off|status", "/web on|off|status"} {
		if strings.Contains(result.Content, removed) {
			t.Fatalf("expected help to omit removed toggle command %q, got %s", removed, result.Content)
		}
	}
}
