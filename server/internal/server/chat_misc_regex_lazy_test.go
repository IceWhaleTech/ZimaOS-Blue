package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func resetChatMiscRegexesForTest(t *testing.T) {
	t.Helper()

	reSystemReminder = nil
	reThinkBlock = nil
	reAwaitingUserInputTag = nil
	reAskGateBlock = nil
	rePseudoDirectiveRecipientFunctions = nil
	rePseudoDirectiveCommandWorkdir = nil
	rePseudoDirectiveCommandPlaceholder = nil
	rePseudoDirectivePayloadJSON = nil
	rePseudoToolCallBlock = nil
	rePseudoToolCallTag = nil
	rePseudoBracketedToolCallBlock = nil
	rePseudoBracketedToolCallTag = nil
	rePseudoInlineTokenFunctions = nil
	rePseudoInlineTokenParallel = nil
	rePseudoInlineTokenRecipient = nil
	rePseudoInlineTokenToolUses = nil
	rePseudoInlineTokenJSONWord = nil
	rePseudoInlineTokenLetsDo = nil
	reExecWebSearchQuery = nil
	reTodoUnchecked = nil
	reTodoAnyItem = nil
	reTodoChecklistBlock = nil
	reTypelessBlock = nil
	reAskOptionLine = nil
	reShortAffirmativeEN = nil
	reExplicitWorkspaceCollectionCount = nil
	reShortAffirmativeIntl = nil
	reMemoryPreamble = nil
	ansiPattern = nil
	chatMiscRegexesOnce = sync.Once{}

	t.Cleanup(func() {
		chatMiscRegexesOnce = sync.Once{}
		ensureChatMiscRegexes()
	})
}

func TestChatMiscRegexes_InitializeOnDemand(t *testing.T) {
	t.Run("sanitize response content", func(t *testing.T) {
		resetChatMiscRegexesForTest(t)

		got := sanitizeResponseContentWithProvider("<system-reminder>skip</system-reminder><think>hidden</think>\nvisible", "", "", "")
		if got != "visible" {
			t.Fatalf("sanitizeResponseContentWithProvider() = %q, want %q", got, "visible")
		}
		if reSystemReminder == nil || reThinkBlock == nil {
			t.Fatal("expected sanitize response regexes to initialize on demand")
		}
	})

	t.Run("extract search query", func(t *testing.T) {
		resetChatMiscRegexesForTest(t)

		got := extractSearchQueryFromExecCommand(`web_search query="latency"`)
		if got != "latency" {
			t.Fatalf("extractSearchQueryFromExecCommand() = %q, want %q", got, "latency")
		}
		if reExecWebSearchQuery == nil {
			t.Fatal("expected web search query regex to initialize on demand")
		}
	})

	t.Run("complete todo items", func(t *testing.T) {
		resetChatMiscRegexesForTest(t)

		got, changed := completeAllTodoItems("- [ ] first item")
		if !changed {
			t.Fatal("expected todo completion change")
		}
		if got != "- [x] first item" {
			t.Fatalf("completeAllTodoItems() = %q, want %q", got, "- [x] first item")
		}
		if reTodoUnchecked == nil {
			t.Fatal("expected todo regex to initialize on demand")
		}
	})

	t.Run("extract explicit workspace collection count", func(t *testing.T) {
		resetChatMiscRegexesForTest(t)

		if got := extractExplicitWorkspaceCollectionCount("Please read 12 files and summarize them."); got != 12 {
			t.Fatalf("extractExplicitWorkspaceCollectionCount() = %d, want %d", got, 12)
		}
		if reExplicitWorkspaceCollectionCount == nil {
			t.Fatal("expected workspace collection count regex to initialize on demand")
		}
	})

	t.Run("strip ANSI", func(t *testing.T) {
		resetChatMiscRegexesForTest(t)

		got := stripANSI("\x1b[31mwarn\x1b[0m")
		if got != "warn" {
			t.Fatalf("stripANSI() = %q, want %q", got, "warn")
		}
		if ansiPattern == nil {
			t.Fatal("expected ANSI regex to initialize on demand")
		}
	})

	t.Run("extract conversation title", func(t *testing.T) {
		resetChatMiscRegexesForTest(t)

		if got := extractConversationTitleFromAIResponse("# TODO清单\n\n- [ ] first item"); got != "" {
			t.Fatalf("extractConversationTitleFromAIResponse() = %q, want empty title when checklist is present", got)
		}
		if reTodoChecklistBlock == nil {
			t.Fatal("expected checklist regex to initialize on demand")
		}
	})
}

func TestExtractMemoryAfterTurn_InitializesChatMiscRegexesOnDemand(t *testing.T) {
	resetChatMiscRegexesForTest(t)

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "lazy memory extraction")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	for i, msg := range []memory.Message{
		{Role: "user", Content: "Please remember that I prefer concise answers."},
		{Role: "assistant", Content: "I will keep that in mind."},
		{Role: "user", Content: "Also remember I want short release notes."},
	} {
		if _, err := store.AddMessage(context.Background(), conv.ID, msg); err != nil {
			t.Fatalf("AddMessage seed %d: %v", i, err)
		}
	}

	layered, _ := newLayeredMemoryServiceForTest(t)
	handler := NewChatHandler(store, nil, tools.NewRegistry())
	defer handler.Close()
	handler.SetLayeredMemory(layered)
	handler.SetProxyBridge(proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp-memory","model":"auto","choices":[{"message":{"role":"assistant","content":"<think>hidden</think>\nHere are the key facts:\n- User prefers concise answers and short release notes."},"finish_reason":"stop"}]}`))
	})))

	if !handler.extractMemoryAfterTurn(conv.ID, "web", "scripted-model") {
		t.Fatal("expected extractMemoryAfterTurn to save memory")
	}

	dates, err := layered.ListDailyLogs(context.Background())
	if err != nil {
		t.Fatalf("ListDailyLogs: %v", err)
	}
	if len(dates) != 1 {
		t.Fatalf("daily logs len = %d, want %d", len(dates), 1)
	}

	dailyLog, err := layered.GetDailyLog(context.Background(), dates[0])
	if err != nil {
		t.Fatalf("GetDailyLog: %v", err)
	}
	if !strings.Contains(dailyLog, "User prefers concise answers and short release notes.") {
		t.Fatalf("daily log = %q, want sanitized extracted memory", dailyLog)
	}
	if strings.Contains(dailyLog, "<think>") || strings.Contains(dailyLog, "Here are the key facts") {
		t.Fatalf("daily log retained internal extraction scaffolding: %q", dailyLog)
	}
	if reThinkBlock == nil || reMemoryPreamble == nil {
		t.Fatal("expected memory extraction regexes to initialize on demand")
	}
}
