package pruner

import (
	"context"
	"errors"
	"testing"
	"time"
)

type hybridStubBackend struct {
	prune func(ctx context.Context, req PruneRequest) (*PruneResponse, error)
}

func (b hybridStubBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	if b.prune != nil {
		return b.prune(ctx, req)
	}
	return &PruneResponse{PrunedContent: "ok", CompressionRate: 1.0}, nil
}

func (b hybridStubBackend) Health(context.Context) error { return nil }
func (b hybridStubBackend) Close() error                 { return nil }

func TestHybridBackend_PicksLowerCompression(t *testing.T) {
	h := NewHybridBackend(
		hybridStubBackend{prune: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{PrunedContent: "local", CompressionRate: 0.7}, nil
		}},
		hybridStubBackend{prune: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{PrunedContent: "neural", CompressionRate: 0.3}, nil
		}},
		Config{TimeoutMs: 1000},
	)

	resp, err := h.Prune(context.Background(), PruneRequest{Content: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PrunedContent != "neural" {
		t.Fatalf("expected neural result, got %q", resp.PrunedContent)
	}
}

func TestHybridBackend_FallsBackToLocalWhenNeuralFails(t *testing.T) {
	h := NewHybridBackend(
		hybridStubBackend{prune: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{PrunedContent: "local", CompressionRate: 0.8}, nil
		}},
		hybridStubBackend{prune: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return nil, errors.New("neural failed")
		}},
		Config{TimeoutMs: 1000},
	)

	resp, err := h.Prune(context.Background(), PruneRequest{Content: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PrunedContent != "local" {
		t.Fatalf("expected local result, got %q", resp.PrunedContent)
	}
}

func TestHybridBackend_TimeoutReturnsAvailableResult(t *testing.T) {
	h := NewHybridBackend(
		hybridStubBackend{prune: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			return &PruneResponse{PrunedContent: "local", CompressionRate: 0.8}, nil
		}},
		hybridStubBackend{prune: func(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
			time.Sleep(30 * time.Millisecond)
			return &PruneResponse{PrunedContent: "neural", CompressionRate: 0.2}, nil
		}},
		Config{TimeoutMs: 5},
	)

	resp, err := h.Prune(context.Background(), PruneRequest{Content: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PrunedContent != "local" {
		t.Fatalf("expected local result on timeout, got %q", resp.PrunedContent)
	}
}

func TestHybridBackend_CanceledContextReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h := NewHybridBackend(
		hybridStubBackend{},
		hybridStubBackend{},
		Config{TimeoutMs: 1000},
	)

	_, err := h.Prune(ctx, PruneRequest{Content: "x"})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
