package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type workspaceExecGuard struct{ root string }

func (g workspaceExecGuard) CheckExecWorkdir(_ context.Context, path string) error {
	return g.check(path)
}
func (g workspaceExecGuard) CheckExecPath(_ context.Context, path string) error { return g.check(path) }
func (g workspaceExecGuard) check(path string) error {
	if path == g.root || strings.HasPrefix(path, g.root+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("exec denied: escapes run workspace_root")
}

func TestShellUsesRunWorkspaceByDefault(t *testing.T) {
	global, workspace := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("actual workspace result"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()
	cfg := DefaultExecConfig()
	cfg.AllowedDirs = []string{global}
	native := NewExecTool(cfg, sessions, nil, nil, nil)
	registry.Register(native)
	registerCanonicalShellSurface(registry)
	gateway := NewToolGateway(registry, NewExecutor(registry))
	ctx := WithFSRootOverride(context.Background(), []string{workspace}, map[string]string{"workspace": workspace})
	ctx = WithExecPathGuard(ctx, workspaceExecGuard{root: workspace})
	for _, name := range []string{"bash", "exec"} {
		t.Run(name, func(t *testing.T) {
			for _, command := range []string{"pwd", "cat " + filepath.Join(workspace, "result.txt")} {
				args, _ := json.Marshal(map[string]any{"command": command})
				got, err := gateway.Execute(ctx, ToolGatewayRequest{ToolName: name, Arguments: string(args), RouteKind: ToolRouteKindAgent})
				if err != nil {
					t.Fatalf("run workspace command rejected: %#v", err)
				}
				want := workspace
				if strings.HasPrefix(command, "cat ") {
					want = "actual workspace result"
				}
				if !strings.Contains(got.AuditContent, want) {
					t.Fatalf("missing actual run output: %s", got.AuditContent)
				}
			}
		})
	}
	for _, args := range []map[string]any{{"command": "pwd", "workdir": global}, {"command": "cat " + filepath.Join(global, "outside.txt")}} {
		raw, _ := json.Marshal(args)
		if _, err := gateway.Execute(ctx, ToolGatewayRequest{ToolName: "exec", Arguments: string(raw), RouteKind: ToolRouteKindAgent}); err == nil {
			t.Fatal("explicit outside path escaped run guard")
		}
	}
	if native.config.AllowedDirs[0] != global {
		t.Fatal("run scope mutated shared configuration")
	}
	// Supplemental document roots must not replace the shell's default workspace.
	ctx = WithFSScope(context.Background(), []string{workspace}, nil)
	raw, _ := json.Marshal(map[string]any{"command": "pwd"})
	got, err := gateway.Execute(ctx, ToolGatewayRequest{ToolName: "bash", Arguments: string(raw), RouteKind: ToolRouteKindAgent})
	if err != nil || !strings.Contains(got.AuditContent, global) {
		t.Fatalf("supplemental root changed default: %#v, %v", got, err)
	}
}
