package tools

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

func TestWithSessionID(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionID(ctx, "conv-123")

	got := GetSessionID(ctx)
	if got != "conv-123" {
		t.Errorf("GetSessionID() = %q, want %q", got, "conv-123")
	}
}

func TestGetSessionID_Empty(t *testing.T) {
	ctx := context.Background()
	got := GetSessionID(ctx)
	if got != "" {
		t.Errorf("GetSessionID() on empty ctx = %q, want empty", got)
	}
}

func TestSessionIDDoesNotInterfereWithOtherKeys(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionID(ctx, "sess-1")
	ctx = WithUserID(ctx, "user-1")
	ctx = WithLang(ctx, "zh-CN")

	if got := GetSessionID(ctx); got != "sess-1" {
		t.Errorf("GetSessionID() = %q, want %q", got, "sess-1")
	}
	if got := GetUserID(ctx); got != "user-1" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-1")
	}
	if got := GetLang(ctx); got != "zh-CN" {
		t.Errorf("GetLang() = %q, want %q", got, "zh-CN")
	}
	if got := skill.GetLang(ctx); got != "zh-CN" {
		t.Errorf("skill.GetLang() = %q, want %q", got, "zh-CN")
	}
}

func TestToolExecutionMetadataContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithProvider(ctx, "openai")
	ctx = WithProviderID(ctx, "provider-1")
	ctx = WithModel(ctx, "gpt-5")
	ctx = WithAgentID(ctx, "agent-42")
	ctx = WithRouteKind(ctx, ToolRouteKindWorkflow)

	if got := GetProvider(ctx); got != "openai" {
		t.Fatalf("GetProvider() = %q, want %q", got, "openai")
	}
	if got := GetProviderID(ctx); got != "provider-1" {
		t.Fatalf("GetProviderID() = %q, want %q", got, "provider-1")
	}
	if got := GetModel(ctx); got != "gpt-5" {
		t.Fatalf("GetModel() = %q, want %q", got, "gpt-5")
	}
	if got := GetAgentID(ctx); got != "agent-42" {
		t.Fatalf("GetAgentID() = %q, want %q", got, "agent-42")
	}
	if got := GetRouteKind(ctx); got != ToolRouteKindWorkflow {
		t.Fatalf("GetRouteKind() = %q, want %q", got, ToolRouteKindWorkflow)
	}
}

func TestSubagentExecutorContext(t *testing.T) {
	ctx := context.Background()
	executor := &stubSubagentExecutor{}
	ctx = WithSubagentExecutor(ctx, executor)

	got := GetSubagentExecutor(ctx)
	if got == nil {
		t.Fatal("expected subagent executor in context")
	}
	if got != executor {
		t.Fatalf("unexpected executor: %#v", got)
	}
}

type stubWritePathGuard struct {
	lastPath string
	err      error
}

func (g *stubWritePathGuard) CheckWritePath(_ context.Context, absPath string) error {
	g.lastPath = absPath
	return g.err
}

func TestWritePathGuardContext(t *testing.T) {
	ctx := context.Background()
	guard := &stubWritePathGuard{}
	ctx = WithWritePathGuard(ctx, guard)

	got := GetWritePathGuard(ctx)
	if got == nil {
		t.Fatal("expected write path guard in context")
	}
	if got != guard {
		t.Fatalf("unexpected guard: %#v", got)
	}
}

func TestFSRootOverrideContext(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	ctx = WithFSRootOverride(ctx, []string{tmpDir}, map[string]string{"workspace": tmpDir})

	roots, aliases := GetFSScope(ctx)
	if len(roots) != 1 {
		t.Fatalf("roots len = %d, want 1", len(roots))
	}
	if aliases["workspace"] == "" {
		t.Fatalf("expected workspace alias, got %#v", aliases)
	}
}

type stubExecPathGuard struct {
	lastWorkdir string
	lastPath    string
	workdirErr  error
	pathErr     error
}

func (g *stubExecPathGuard) CheckExecWorkdir(_ context.Context, absWorkdir string) error {
	g.lastWorkdir = absWorkdir
	return g.workdirErr
}

func (g *stubExecPathGuard) CheckExecPath(_ context.Context, absPath string) error {
	g.lastPath = absPath
	return g.pathErr
}

func TestExecPathGuardContext(t *testing.T) {
	ctx := context.Background()
	guard := &stubExecPathGuard{}
	ctx = WithExecPathGuard(ctx, guard)

	got := GetExecPathGuard(ctx)
	if got == nil {
		t.Fatal("expected exec path guard in context")
	}
	if got != guard {
		t.Fatalf("unexpected guard: %#v", got)
	}
}
