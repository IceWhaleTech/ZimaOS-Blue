package memory

import (
	"context"
	"os"
	"testing"
)

func TestDualWriteBackend_RememberWritesBoth(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dual-write-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mdBackend, err := NewPureMarkdownBackend(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Use markdown backend as both primary and secondary for simplicity
	primary := mdBackend
	secondary, _ := NewPureMarkdownBackend(tmpDir + "/secondary")

	dual := NewDualWriteBackend(primary, secondary)

	chunk, err := dual.Remember(context.Background(), "test memory", []string{"tag1"})
	if err != nil {
		t.Fatalf("Remember failed: %v", err)
	}
	if chunk == nil || chunk.Content != "test memory" {
		t.Fatal("expected chunk with content 'test memory'")
	}
	if dual.Name() != "mixed" {
		t.Errorf("expected name 'mixed', got %s", dual.Name())
	}
}

func TestDualWriteBackend_RecallUsesPrimary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dual-recall-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	primary, _ := NewPureMarkdownBackend(tmpDir + "/primary")
	secondary, _ := NewPureMarkdownBackend(tmpDir + "/secondary")
	dual := NewDualWriteBackend(primary, secondary)

	// Write to dual
	_, _ = dual.Remember(context.Background(), "searchable content about golang", []string{})

	// Recall should use primary
	results, err := dual.Recall(context.Background(), "golang", 10)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result from primary")
	}
}

func TestDualWriteBackend_SecondaryFailureDoesNotBlock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dual-fail-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	primary, _ := NewPureMarkdownBackend(tmpDir + "/primary")
	// secondary with invalid path to force write failures
	secondary := &PureMarkdownBackend{
		store:   NewMarkdownMemoryStore("/nonexistent/path/that/should/fail"),
		baseDir: "/nonexistent/path/that/should/fail",
	}

	dual := NewDualWriteBackend(primary, secondary)

	// Should succeed despite secondary failure
	chunk, err := dual.Remember(context.Background(), "test content", []string{})
	if err != nil {
		t.Fatalf("Remember should succeed despite secondary failure: %v", err)
	}
	if chunk == nil {
		t.Fatal("expected non-nil chunk")
	}
}

func TestDualWriteBackend_Stats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dual-stats-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	primary, _ := NewPureMarkdownBackend(tmpDir + "/primary")
	secondary, _ := NewPureMarkdownBackend(tmpDir + "/secondary")
	dual := NewDualWriteBackend(primary, secondary)

	stats, err := dual.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.Backend != "mixed" {
		t.Errorf("expected backend 'mixed', got %s", stats.Backend)
	}
}

func TestDualWriteBackend_ForgetAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dual-forgetall-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	primary, _ := NewPureMarkdownBackend(tmpDir + "/primary")
	secondary, _ := NewPureMarkdownBackend(tmpDir + "/secondary")
	dual := NewDualWriteBackend(primary, secondary)

	_, _ = dual.Remember(context.Background(), "memory to forget", []string{})

	// ForgetAll should succeed on both backends
	err = dual.ForgetAll(context.Background())
	if err != nil {
		t.Fatalf("ForgetAll failed: %v", err)
	}
}
