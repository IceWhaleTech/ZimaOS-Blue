package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindTool_BareFilenameFallsBackToWorkspaceRoot(t *testing.T) {
	tmpDir := t.TempDir()
	targetName := "orca_命理完整报告.pdf"
	if err := os.WriteFile(filepath.Join(tmpDir, targetName), []byte("pdf"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	tool := NewFindTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":      targetName,
		"pattern":   "*.pdf",
		"type":      "file",
		"max_depth": 2,
	})
	if err != nil {
		t.Fatalf("find with bare filename should fall back to workspace root, got error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode find result: %v", err)
	}
	if got, _ := payload["base_path"].(string); got != "." {
		t.Fatalf("base_path = %q, want .", got)
	}
	entries, ok := payload["entries"].([]interface{})
	if !ok || len(entries) == 0 {
		t.Fatalf("expected find entries, got %#v", payload["entries"])
	}
	found := false
	for _, raw := range entries {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["path"] == targetName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("find entries = %#v, want %q present", payload["entries"], targetName)
	}
}

func TestLsTool_BareFilenameFallsBackToWorkspaceRoot(t *testing.T) {
	tmpDir := t.TempDir()
	targetName := "orca_命理完整报告.pdf"
	if err := os.WriteFile(filepath.Join(tmpDir, targetName), []byte("pdf"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	tool := NewLsTool([]string{tmpDir})
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": targetName,
	})
	if err != nil {
		t.Fatalf("ls with bare filename should fall back to workspace root, got error: %v", err)
	}

	var payload struct {
		BasePath string `json:"base_path"`
		Entries  []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode ls result: %v", err)
	}
	if payload.BasePath != "." {
		t.Fatalf("base_path = %q, want .", payload.BasePath)
	}
	found := false
	for _, entry := range payload.Entries {
		if entry.Path == targetName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ls entries = %+v, want %q present", payload.Entries, targetName)
	}
}
