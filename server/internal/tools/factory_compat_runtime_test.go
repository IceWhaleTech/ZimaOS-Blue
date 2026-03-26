package tools

import (
	"strings"
	"testing"
)

func TestResolveFactorySessionsSpawnAliasCommand_RuntimeAware(t *testing.T) {
	cmd, handled, invalid := resolveFactoryAliasCommand("sessions_spawn", map[string]interface{}{
		"runtime":    "acp",
		"profile_id": "codex",
		"title":      "Codex Session",
	})
	if !handled || invalid != "" {
		t.Fatalf("handled=%v invalid=%q", handled, invalid)
	}
	if !strings.Contains(cmd, "/api/v1/agent-sessions/sessions") {
		t.Fatalf("command = %q, want agent sessions endpoint", cmd)
	}
	if !strings.Contains(cmd, "codex") {
		t.Fatalf("command = %q, want profile id", cmd)
	}
}

func TestResolveFactorySessionsSendAliasCommand_RuntimeAware(t *testing.T) {
	cmd, handled, invalid := resolveFactoryAliasCommand("sessions_send", map[string]interface{}{
		"runtime": "a2a",
		"id":      "sess_123",
		"message": "hello",
	})
	if !handled || invalid != "" {
		t.Fatalf("handled=%v invalid=%q", handled, invalid)
	}
	if !strings.Contains(cmd, "/api/v1/agent-sessions/sessions/sess_123/messages") {
		t.Fatalf("command = %q, want runtime-aware messages endpoint", cmd)
	}
}

