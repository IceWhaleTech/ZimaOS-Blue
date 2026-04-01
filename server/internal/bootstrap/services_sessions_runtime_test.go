package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
)

func TestSessionListAdapterHandleRuntimeSessionActionRequiresConfiguredAgentSessions(t *testing.T) {
	adapter := sessionListAdapter{}

	_, err := adapter.HandleRuntimeSessionAction(context.Background(), "list", nil)
	if err == nil || !strings.Contains(err.Error(), "runtime-aware sessions are not configured") {
		t.Fatalf("HandleRuntimeSessionAction(list) err = %v, want configured error", err)
	}
}

func TestSessionListAdapterHandleRuntimeSessionActionRejectsUnknownAction(t *testing.T) {
	adapter := sessionListAdapter{agentSessions: &agentsessions.Service{}}

	_, err := adapter.HandleRuntimeSessionAction(context.Background(), "mystery", nil)
	if err == nil || !strings.Contains(err.Error(), `unknown runtime-aware sessions action "mystery"`) {
		t.Fatalf("HandleRuntimeSessionAction(mystery) err = %v, want unknown action error", err)
	}
}

func TestSessionListAdapterHandleRuntimeSessionActionSpawnRequiresProfileID(t *testing.T) {
	adapter := sessionListAdapter{agentSessions: &agentsessions.Service{}}

	_, err := adapter.HandleRuntimeSessionAction(context.Background(), "spawn", map[string]interface{}{"title": "Test"})
	if err == nil || !strings.Contains(err.Error(), "profile_id is required for runtime-aware spawn") {
		t.Fatalf("HandleRuntimeSessionAction(spawn) err = %v, want profile_id error", err)
	}
}

func TestRuntimeCompatTitleTruncatesAndTrims(t *testing.T) {
	text := "   " + strings.Repeat("x", 80) + "   "

	got := runtimeCompatTitle(text)
	if len(got) != 72 {
		t.Fatalf("runtimeCompatTitle len = %d, want 72", len(got))
	}
	if got != strings.Repeat("x", 72) {
		t.Fatalf("runtimeCompatTitle = %q, want trimmed 72-char prefix", got)
	}
}
