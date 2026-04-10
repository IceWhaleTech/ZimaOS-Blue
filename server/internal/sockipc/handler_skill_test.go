package sockipc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"go.uber.org/zap"
)

type mockSkillFallbackExecutor struct {
	lastSkillID string
	lastInput   map[string]any
	lastUserID  string
	hasDeadline bool
	timeLeft    time.Duration
	result      map[string]string
	err         error
}

func (m *mockSkillFallbackExecutor) Execute(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
	m.lastSkillID = skillID
	m.lastInput = input
	m.lastUserID = skill.GetUserID(ctx)
	if deadline, ok := ctx.Deadline(); ok {
		m.hasDeadline = true
		m.timeLeft = time.Until(deadline)
	} else {
		m.hasDeadline = false
		m.timeLeft = 0
	}
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return map[string]string{"ok": "1"}, nil
}

func (m *mockSkillFallbackExecutor) Help(_ context.Context, skillID string, input map[string]any) (map[string]string, error) {
	m.lastSkillID = "help:" + skillID
	m.lastInput = input
	return map[string]string{
		"description": "Rewrite a local text file into a more natural voice.",
		"usage":       "blue humanizer input=<path> [output=<path>]",
	}, nil
}

func TestRegisterSkillFallback_AnalyzeUsesLongerTimeout(t *testing.T) {
	exec := &mockSkillFallbackExecutor{result: map[string]string{"status": "ok"}}
	conn, cleanup := setupSkillFallbackServer(t, exec)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd: "analyze",
		Params: map[string]string{
			"topic": "homelab",
			"text":  "sample",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if !exec.hasDeadline {
		t.Fatal("expected analyze fallback to apply deadline")
	}
	if exec.timeLeft < analyzeSkillFallbackTimeout-10*time.Second {
		t.Fatalf("analyze timeout too short: %s", exec.timeLeft)
	}
}

func setupSkillFallbackServer(t *testing.T, executor SkillExecutor) (net.Conn, func()) {
	t.Helper()
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterSkillFallback(srv, executor, zap.NewNop())
	startServerOrSkip(t, srv)
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

func TestRegisterSkillFallback_InheritsBlueUserIDContext(t *testing.T) {
	exec := &mockSkillFallbackExecutor{result: map[string]string{"status": "ok"}}
	conn, cleanup := setupSkillFallbackServer(t, exec)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd: "ask",
		Params: map[string]string{
			"q":              "Pick one",
			"a":              `["A","B"]`,
			"__blue_user_id": "user-xyz",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if exec.lastUserID != "user-xyz" {
		t.Fatalf("user_id=%q, want %q", exec.lastUserID, "user-xyz")
	}
}

func TestRegisterSkillFallback_HelpFlagUsesSkillHelpProvider(t *testing.T) {
	exec := &mockSkillFallbackExecutor{
		err: context.DeadlineExceeded,
	}
	conn, cleanup := setupSkillFallbackServer(t, exec)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{
		Cmd: "humanizer",
		Params: map[string]string{
			"help": "true",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if exec.lastSkillID != "help:humanizer" {
		t.Fatalf("skillID=%q, want %q", exec.lastSkillID, "help:humanizer")
	}
	if got := resp.Data["usage"]; got != "blue humanizer input=<path> [output=<path>]" {
		t.Fatalf("usage=%q", got)
	}
}
