package bootstrap

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"go.uber.org/zap"
)

type stubTool struct {
	name   string
	result interface{}
	seen   map[string]interface{}
}

func (s *stubTool) Definition() tools.ToolDefinition {
	name := s.name
	if name == "" {
		name = "web_fetch"
	}
	return tools.ToolDefinition{Name: name}
}

func (s *stubTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	s.seen = args
	return s.result, nil
}

func TestTryExecuteToolFallback_ExecutesBuiltinWebFetch(t *testing.T) {
	registry := tools.NewRegistry()
	tool := &stubTool{result: `{"url":"https://example.com","title":"Example","content":"Hello"}`}
	registry.Register(tool)

	result, handled, err := tryExecuteToolFallback(context.Background(), registry, "web_fetch", map[string]any{
		"url": "https://example.com",
	})
	if err != nil {
		t.Fatalf("fallback execute failed: %v", err)
	}
	if !handled {
		t.Fatal("expected tool fallback to handle web_fetch")
	}
	if tool.seen["url"] != "https://example.com" {
		t.Fatalf("tool saw url = %v, want https://example.com", tool.seen["url"])
	}
	if result["url"] != "https://example.com" {
		t.Fatalf("result url = %q, want https://example.com", result["url"])
	}
	if result["title"] != "Example" {
		t.Fatalf("result title = %q, want Example", result["title"])
	}
	if result["content"] != "Hello" {
		t.Fatalf("result content = %q, want Hello", result["content"])
	}
}

func TestTryExecuteToolFallback_ReturnsNotHandledWhenToolMissing(t *testing.T) {
	result, handled, err := tryExecuteToolFallback(context.Background(), tools.NewRegistry(), "missing_tool", map[string]any{"q": "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected missing tool to be unhandled, got result=%v", result)
	}
}

func TestRegisterSkillFallback_AllowsBuiltinWebFetchIPC(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&stubTool{result: `{"url":"https://example.com","title":"Example","content":"Hello from IPC"}`})

	dir, err := os.MkdirTemp("/tmp", "blue-ipc")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	sockPath := filepath.Join(dir, "s.sock")
	srv := sockipc.NewServer(sockPath, zap.NewNop())
	sockipc.RegisterSkillFallback(srv, sockipc.SkillExecutorFunc(func(ctx context.Context, skillID string, input map[string]any) (map[string]string, error) {
		if data, handled, err := tryExecuteToolFallback(ctx, registry, skillID, input); handled {
			return data, err
		}
		return nil, context.Canceled
	}), zap.NewNop())
	if err := srv.Start(); err != nil {
		t.Fatalf("start sockipc server: %v", err)
	}
	defer srv.Close()

	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("dial sockipc server: %v", err)
	}
	defer conn.Close()

	if err := sockipc.WriteJSON(conn, &sockipc.Request{Cmd: "web_fetch", Params: map[string]string{"url": "https://example.com"}}); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := sockipc.ReadJSON[sockipc.Response](conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status=%q error=%q", resp.Status, resp.Error)
	}
	if resp.Data["url"] != "https://example.com" {
		t.Fatalf("url=%q, want https://example.com", resp.Data["url"])
	}
	if resp.Data["title"] != "Example" {
		t.Fatalf("title=%q, want Example", resp.Data["title"])
	}
	if resp.Data["content"] != "Hello from IPC" {
		t.Fatalf("content=%q, want Hello from IPC", resp.Data["content"])
	}
}

func TestRuntimeIPCSkillExecutor_ComputerUseManifestFallsBackToTool(t *testing.T) {
	workspaceDir := t.TempDir()
	skillDir := filepath.Join(workspaceDir, ".agents", "skills", "computer_use")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: computer_use
version: 1.0.0
description: Computer use declarative manifest
invocation: blue computer_use
examples:
  - blue computer_use action=snapshot_interactive
capability_tags:
  - computer-use
interaction_mode: stateless
card_support: none
---
# Computer Use
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	registry := tools.NewRegistry()
	registry.Register(&stubTool{
		name:   "computer_use",
		result: `{"message":"Host action completed and submitted","window_id":"win-feishu"}`,
	})
	executor := newRuntimeIPCSkillExecutor(&Services{
		ToolRegistry:  registry,
		SkillRegistry: skillpkg.NewRegistry(),
	}, workspaceDir)
	if executor == nil {
		t.Fatal("expected runtime IPC skill executor")
	}

	out, err := executor.Execute(context.Background(), "computer_use", map[string]any{
		"action":       "message",
		"conversation": "Orca",
		"value":        "你好，Orca。",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if out["message"] != "Host action completed and submitted" {
		t.Fatalf("message=%q, want tool fallback result", out["message"])
	}
	if out["window_id"] != "win-feishu" {
		t.Fatalf("window_id=%q, want win-feishu", out["window_id"])
	}
	if out["skill"] != "" {
		t.Fatalf("expected tool fallback instead of declarative skill payload, got %#v", out)
	}
}
