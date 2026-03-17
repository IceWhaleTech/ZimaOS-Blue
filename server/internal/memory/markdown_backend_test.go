package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPureMarkdownBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-backend-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	backend, err := NewPureMarkdownBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()

	t.Run("Remember", func(t *testing.T) {
		chunk, err := backend.Remember(ctx, "Test memory content about AI", []string{"test", "ai"})
		if err != nil {
			t.Fatalf("Remember failed: %v", err)
		}
		if chunk.ID == "" {
			t.Error("expected non-empty ID")
		}
		if chunk.Content != "Test memory content about AI" {
			t.Error("content mismatch")
		}
	})

	t.Run("Recall", func(t *testing.T) {
		// Add more memories
		backend.Remember(ctx, "Machine learning is a subset of AI", []string{"ml"})
		backend.Remember(ctx, "Deep learning uses neural networks", []string{"dl"})

		results, err := backend.Recall(ctx, "AI", 10)
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected at least one result")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats, err := backend.Stats(ctx)
		if err != nil {
			t.Fatalf("Stats failed: %v", err)
		}
		if stats.Backend != "markdown" {
			t.Errorf("expected backend 'markdown', got %s", stats.Backend)
		}
		if stats.TotalChunks == 0 {
			t.Error("expected non-zero chunk count")
		}
	})

	t.Run("Name", func(t *testing.T) {
		if backend.Name() != "markdown" {
			t.Errorf("expected 'markdown', got %s", backend.Name())
		}
	})
}

func TestMarkdownMemoryStore_Search(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "md-store-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "test1.md"), []byte("# Hello World\n\nThis is about AI and machine learning."), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test2.md"), []byte("# Notes\n\nDeep learning is powerful."), 0644)

	store := NewMarkdownMemoryStore(tmpDir)
	ctx := context.Background()

	results, err := store.Search(ctx, "AI", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results for 'AI'")
	}

	// Test heading boost
	results2, _ := store.Search(ctx, "Hello", 10)
	for _, r := range results2 {
		if strings.Contains(r.Content, "# Hello") && r.MatchType != "heading" {
			t.Error("heading should be detected")
		}
	}
}

func TestMarkdownMemoryStore_ReadFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "md-read-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte(content), 0644)

	store := NewMarkdownMemoryStore(tmpDir)
	ctx := context.Background()

	// Read full file
	full, err := store.ReadFile(ctx, "test.md", 0, 0)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if full != content {
		t.Error("content mismatch")
	}

	// Read partial
	partial, _ := store.ReadFile(ctx, "test.md", 2, 2)
	if partial != "Line 2\nLine 3" {
		t.Errorf("expected 'Line 2\\nLine 3', got '%s'", partial)
	}
}

func TestPureMarkdownBackendRememberReturnsReadablePath(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewPureMarkdownBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewPureMarkdownBackend returned error: %v", err)
	}
	ctx := context.Background()
	chunk, err := backend.Remember(ctx, "Path-readable memory", []string{"smoke"})
	if err != nil {
		t.Fatalf("Remember returned error: %v", err)
	}
	if chunk.ID == "" {
		t.Fatal("expected non-empty chunk ID")
	}
	got, err := backend.Get(ctx, chunk.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != chunk.ID {
		t.Fatalf("Get ID = %q, want %q", got.ID, chunk.ID)
	}
	if got.Content == "" {
		t.Fatal("expected readable content")
	}
	if err := backend.Forget(ctx, chunk.ID); err != nil {
		t.Fatalf("Forget returned error: %v", err)
	}
}

func TestMarkdownMemoryStoreReadFileRejectsTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewMarkdownMemoryStore(tmpDir)

	_, err := store.ReadFile(context.Background(), "../outside.md", 0, 0)
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("ReadFile error = %v, want ErrInvalidPath", err)
	}
}

func TestPureMarkdownBackendForgetRejectsTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewPureMarkdownBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewPureMarkdownBackend returned error: %v", err)
	}

	err = backend.Forget(context.Background(), "../memory-escape.md")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Forget error = %v, want ErrInvalidPath", err)
	}
}
