package sockipc

import (
	"context"
	"net"
	"testing"

	"go.uber.org/zap"
)

type mockSkillFallbackExecutor struct {
	lastSkillID string
	lastInput   map[string]any
	result      map[string]string
	err         error
}

func (m *mockSkillFallbackExecutor) Execute(_ context.Context, skillID string, input map[string]any) (map[string]string, error) {
	m.lastSkillID = skillID
	m.lastInput = input
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return map[string]string{"ok": "1"}, nil
}

func setupSkillFallbackServer(t *testing.T, executor SkillExecutor) (net.Conn, func()) {
	t.Helper()
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterSkillFallback(srv, executor, zap.NewNop())
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	return conn, func() { conn.Close(); srv.Close() }
}

func TestRegisterSkillFallback_DottedCommandMapsToAction(t *testing.T) {
	exec := &mockSkillFallbackExecutor{result: map[string]string{"status": "ok"}}
	conn, cleanup := setupSkillFallbackServer(t, exec)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd: "reminder.add",
		Params: map[string]string{
			"message": "喝水",
			"time":    "10s",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if exec.lastSkillID != "reminder" {
		t.Fatalf("skillID=%q, want %q", exec.lastSkillID, "reminder")
	}
	if got, _ := exec.lastInput["action"].(string); got != "add" {
		t.Fatalf("action=%q, want %q", got, "add")
	}
	if got, _ := exec.lastInput["message"].(string); got != "喝水" {
		t.Fatalf("message=%q, want %q", got, "喝水")
	}
	if got, _ := exec.lastInput["time"].(string); got != "10s" {
		t.Fatalf("time=%q, want %q", got, "10s")
	}
}

func TestRegisterSkillFallback_DottedCommandKeepsExplicitAction(t *testing.T) {
	exec := &mockSkillFallbackExecutor{result: map[string]string{"status": "ok"}}
	conn, cleanup := setupSkillFallbackServer(t, exec)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd: "reminder.add",
		Params: map[string]string{
			"action": "list",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if exec.lastSkillID != "reminder" {
		t.Fatalf("skillID=%q, want %q", exec.lastSkillID, "reminder")
	}
	if got, _ := exec.lastInput["action"].(string); got != "list" {
		t.Fatalf("action=%q, want explicit value %q", got, "list")
	}
}

func TestRegisterSkillFallback_NonDottedCommandUnchanged(t *testing.T) {
	exec := &mockSkillFallbackExecutor{result: map[string]string{"status": "ok"}}
	conn, cleanup := setupSkillFallbackServer(t, exec)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd: "reminder",
		Params: map[string]string{
			"action": "list",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if exec.lastSkillID != "reminder" {
		t.Fatalf("skillID=%q, want %q", exec.lastSkillID, "reminder")
	}
	if got, _ := exec.lastInput["action"].(string); got != "list" {
		t.Fatalf("action=%q, want %q", got, "list")
	}
}
