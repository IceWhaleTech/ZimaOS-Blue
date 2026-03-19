package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileWriteToolHonorsWritePathGuard(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewFileWriteTool([]string{tmpDir}, 0)
	guard := &stubWritePathGuard{err: errors.New("direct write denied: protected path")}
	ctx := WithWritePathGuard(context.Background(), guard)

	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":    "denied.txt",
		"content": "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Fatalf("expected protected path error, got %v", err)
	}
	if guard.lastPath == "" {
		t.Fatal("expected write guard to be called")
	}
}

func TestEditToolHonorsWritePathGuard(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(target, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	tool := NewEditTool([]string{tmpDir}, 0)
	guard := &stubWritePathGuard{err: errors.New("direct write denied: protected path")}
	ctx := WithWritePathGuard(context.Background(), guard)

	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":     "notes.txt",
		"old_text": "world",
		"new_text": "blue",
	})
	if err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Fatalf("expected protected path error, got %v", err)
	}
}

func TestWriteBeginHonorsWritePathGuard(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewFileWriteBeginTool([]string{tmpDir}, NewWriteSessionManager(0))
	guard := &stubWritePathGuard{err: errors.New("direct write denied: protected path")}
	ctx := WithWritePathGuard(context.Background(), guard)

	_, err := tool.Execute(ctx, map[string]interface{}{"path": "blocked.txt"})
	if err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Fatalf("expected protected path error, got %v", err)
	}
}

func TestWriteCommitHonorsWritePathGuard(t *testing.T) {
	tmpDir := t.TempDir()
	sessions := NewWriteSessionManager(0)
	beginTool := NewFileWriteBeginTool([]string{tmpDir}, sessions)
	chunkTool := NewFileWriteChunkTool(sessions)
	commitTool := NewFileWriteCommitTool(sessions)

	beginResult, err := beginTool.Execute(context.Background(), map[string]interface{}{"path": "blocked.txt"})
	if err != nil {
		t.Fatalf("write_begin failed: %v", err)
	}
	beginPayload := decodeWriteGuardJSONMap(t, beginResult)
	sessionID, _ := beginPayload["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("write_begin returned empty session_id: %#v", beginPayload)
	}
	if _, err := chunkTool.Execute(context.Background(), map[string]interface{}{
		"session_id": sessionID,
		"content":    "hello",
	}); err != nil {
		t.Fatalf("write_chunk failed: %v", err)
	}

	guard := &stubWritePathGuard{err: errors.New("direct write denied: protected path")}
	ctx := WithWritePathGuard(context.Background(), guard)
	_, err = commitTool.Execute(ctx, map[string]interface{}{"session_id": sessionID})
	if err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Fatalf("expected protected path error, got %v", err)
	}
}

func decodeWriteGuardJSONMap(t *testing.T, raw interface{}) map[string]interface{} {
	t.Helper()
	text, ok := raw.(string)
	if !ok {
		t.Fatalf("expected string payload, got %T", raw)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	return payload
}
