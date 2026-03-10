package pruner

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled=true by default")
	}
	if cfg.Backend != "local" {
		t.Errorf("expected Backend=local, got %s", cfg.Backend)
	}
	if cfg.Threshold != 0.5 {
		t.Errorf("expected Threshold=0.5, got %f", cfg.Threshold)
	}
	if cfg.MinLines != 50 {
		t.Errorf("expected MinLines=50, got %d", cfg.MinLines)
	}
	if cfg.TimeoutMs != 5000 {
		t.Errorf("expected TimeoutMs=5000, got %d", cfg.TimeoutMs)
	}
}

func TestNewBackend_Local(t *testing.T) {
	cfg := DefaultConfig()
	b, err := NewBackend(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil backend")
	}
	if err := b.Health(nil); err != nil {
		t.Fatalf("local backend health should always pass: %v", err)
	}
	b.Close()
}

func TestNewBackend_Remote(t *testing.T) {
	t.Skip("SWE Pruner remote backend is hidden; test temporarily disabled")

	cfg := DefaultConfig()
	cfg.Backend = "remote"
	cfg.RemoteURL = "http://localhost:9999"
	b, err := NewBackend(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil backend")
	}
	b.Close()
}

func TestNewBackend_Unknown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Backend = "unknown"
	_, err := NewBackend(cfg)
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestNewBackend_BM25(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Backend = "bm25"
	b, err := NewBackend(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil backend")
	}
	if err := b.Health(nil); err != nil {
		t.Fatalf("bm25 backend health should always pass: %v", err)
	}
	b.Close()
}
