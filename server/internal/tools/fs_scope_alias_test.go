package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileWriteTool_AllowsAliasPathFromContextScope(t *testing.T) {
	workspaceRoot := t.TempDir()
	whitelistRoot := t.TempDir()

	tool := NewFileWriteTool([]string{workspaceRoot}, 0)
	ctx := WithFSScope(context.Background(), []string{whitelistRoot}, map[string]string{
		"docs": whitelistRoot,
	})

	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":    "@docs/note.txt",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	data, readErr := os.ReadFile(filepath.Join(whitelistRoot, "note.txt"))
	if readErr != nil {
		t.Fatalf("expected file in whitelist root: %v", readErr)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestFileWriteTool_AliasPathCannotEscapeRoot(t *testing.T) {
	workspaceRoot := t.TempDir()
	whitelistRoot := t.TempDir()

	tool := NewFileWriteTool([]string{workspaceRoot}, 0)
	ctx := WithFSScope(context.Background(), []string{whitelistRoot}, map[string]string{
		"docs": whitelistRoot,
	})

	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":    "docs:../escape.txt",
		"content": "blocked",
	})
	if err == nil {
		t.Fatal("expected error for escaping alias root, got nil")
	}
	if !strings.Contains(err.Error(), "path escapes workspace root") {
		t.Fatalf("unexpected error: %v", err)
	}

	escapedPath := filepath.Join(filepath.Dir(whitelistRoot), "escape.txt")
	if _, statErr := os.Stat(escapedPath); !os.IsNotExist(statErr) {
		t.Fatalf("escaped file should not exist: %s", escapedPath)
	}
}
