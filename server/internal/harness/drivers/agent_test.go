package drivers

import (
	"testing"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

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
