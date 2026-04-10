package drivers

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"

	_ "github.com/mattn/go-sqlite3"
)

type blockingAgentDriverLLM struct {
	started chan struct{}
	once    sync.Once
}

func (m *blockingAgentDriverLLM) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if m != nil && m.started != nil {
		m.once.Do(func() {
			close(m.started)
		})
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func testAgentDriverStore(t *testing.T) *agentpkg.Store {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "agent-driver-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := agentpkg.NewStore(db)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	return store
}

func TestComposeConversationContext(t *testing.T) {
	tests := []struct {
		name string
		meta map[string]interface{}
		want string
	}{
		{
			name: "base context only",
			meta: map[string]interface{}{"context": "recent chat context"},
			want: "recent chat context",
		},
		{
			name: "retry context only",
			meta: map[string]interface{}{"retry_context": "Harness retry guidance"},
			want: "Harness retry guidance",
		},
		{
			name: "base and retry context",
			meta: map[string]interface{}{
				"context":       "recent chat context",
				"retry_context": "Harness retry guidance",
			},
			want: "recent chat context\n\nHarness retry guidance",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeConversationContext(tc.meta); got != tc.want {
				t.Fatalf("composeConversationContext() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunConversationID_FallsBackToSessionID(t *testing.T) {
	run := &harness.Run{SessionID: "session-1"}

	if got := runConversationID(run); got != "session-1" {
		t.Fatalf("runConversationID() = %q, want %q", got, "session-1")
	}
}

func TestTaskToRun_PreservesExistingSessionIDWhenTaskConversationEmpty(t *testing.T) {
	existing := &harness.Run{
		ID:        "run-1",
		SessionID: "session-1",
	}
	task := &agentpkg.Task{
		ID:     "run-1",
		UserID: "user-1",
		Goal:   "schedule reminder",
	}

	got := taskToRun(existing, task, harness.RunKindAgentTask)
	if got.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want %q", got.SessionID, "session-1")
	}
	if got.ConversationID != "session-1" {
		t.Fatalf("ConversationID = %q, want %q", got.ConversationID, "session-1")
	}
}

func TestTaskToRun_UsesTaskConversationIDWhenPresent(t *testing.T) {
	existing := &harness.Run{
		ID:             "run-1",
		ConversationID: "conv-old",
		SessionID:      "session-old",
	}
	task := &agentpkg.Task{
		ID:             "run-1",
		UserID:         "user-1",
		ConversationID: "conv-new",
		Goal:           "schedule reminder",
	}

	got := taskToRun(existing, task, harness.RunKindAgentTask)
	if got.ConversationID != "conv-new" {
		t.Fatalf("ConversationID = %q, want %q", got.ConversationID, "conv-new")
	}
	if got.SessionID != "conv-new" {
		t.Fatalf("SessionID = %q, want %q", got.SessionID, "conv-new")
	}
}

func TestAgentDriver_StartInjectsSilentHarnessMetadataIntoTask(t *testing.T) {
	store := testAgentDriverStore(t)
	llmStub := &blockingAgentDriverLLM{started: make(chan struct{})}
	runner := agentpkg.NewRunner(store, llmStub, nil, nil, nil, agentpkg.RunnerConfig{
		TaskTimeout: 5 * time.Second,
	})
	t.Cleanup(runner.Shutdown)

	driver := NewAgentDriver(harness.RunKindAgentTask, runner, store, nil)
	run := &harness.Run{
		ID:           "run-silent-driver",
		Kind:         harness.RunKindAgentTask,
		UserID:       "user-1",
		Goal:         "ship silently",
		ApprovalMode: harness.ApprovalModeAsk,
		Metadata: map[string]interface{}{
			"context":             "existing context",
			"harness_silent_mode": false,
			"non_interactive":     false,
			"skip_hil":            false,
		},
	}

	if err := driver.Start(context.Background(), run, harness.RunEnv{}); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	task, err := store.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	for _, key := range []string{"harness_silent_mode", "non_interactive", "skip_hil"} {
		value, ok := task.Metadata[key].(bool)
		if !ok || !value {
			t.Fatalf("task metadata[%q] = %#v, want true", key, task.Metadata[key])
		}
	}
	if got := task.Metadata["approval_mode"]; got != string(harness.ApprovalModeAsk) {
		t.Fatalf("task metadata approval_mode = %#v, want %q", got, harness.ApprovalModeAsk)
	}
	if got := run.Metadata["harness_silent_mode"]; got != false {
		t.Fatalf("run metadata harness_silent_mode mutated to %#v, want false", got)
	}

	select {
	case <-llmStub.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for runner execution to start")
	}
	if !runner.Cancel(run.ID) {
		t.Fatal("expected running task to be cancellable for cleanup")
	}
}
