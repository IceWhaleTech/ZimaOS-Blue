package pruner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HybridBackend runs local and neural backends in parallel, picking the better result.
type HybridBackend struct {
	local  Backend
	neural Backend
	config Config
}

// NewHybridBackend creates a hybrid backend that runs both backends concurrently.
func NewHybridBackend(local, neural Backend, cfg Config) *HybridBackend {
	return &HybridBackend{local: local, neural: neural, config: cfg}
}

type pruneResult struct {
	resp *PruneResponse
	err  error
}

// Prune runs both backends in parallel and returns the result with better compression.
func (h *HybridBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	timeout := time.Duration(h.config.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	localCh := make(chan pruneResult, 1)
	neuralCh := make(chan pruneResult, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		resp, err := h.local.Prune(ctx, req)
		localCh <- pruneResult{resp, err}
	}()
	go func() {
		defer wg.Done()
		resp, err := h.neural.Prune(ctx, req)
		neuralCh <- pruneResult{resp, err}
	}()

	// Collect results
	var localRes, neuralRes pruneResult
	for i := 0; i < 2; i++ {
		select {
		case r := <-localCh:
			localRes = r
		case r := <-neuralCh:
			neuralRes = r
		case <-ctx.Done():
			// Timeout — use whatever we have
			break
		}
	}

	// Pick the better result (lower compression rate = more aggressive pruning)
	if neuralRes.err == nil && neuralRes.resp != nil {
		if localRes.err != nil || localRes.resp == nil {
			return neuralRes.resp, nil
		}
		// Both succeeded — pick the one with lower compression rate
		if neuralRes.resp.CompressionRate < localRes.resp.CompressionRate {
			return neuralRes.resp, nil
		}
		return localRes.resp, nil
	}

	// Neural failed — fall back to local
	if localRes.err == nil && localRes.resp != nil {
		return localRes.resp, nil
	}

	// Both failed
	if localRes.err != nil {
		return nil, fmt.Errorf("hybrid: both backends failed: local=%v, neural=%v", localRes.err, neuralRes.err)
	}
	return nil, fmt.Errorf("hybrid: no results")
}

// Health returns healthy if either backend is healthy.
func (h *HybridBackend) Health(ctx context.Context) error {
	localErr := h.local.Health(ctx)
	neuralErr := h.neural.Health(ctx)
	if localErr == nil || neuralErr == nil {
		return nil
	}
	return fmt.Errorf("hybrid: local=%v, neural=%v", localErr, neuralErr)
}

// Close closes both backends.
func (h *HybridBackend) Close() error {
	var errs []error
	if err := h.local.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := h.neural.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("hybrid close: %v", errs)
	}
	return nil
}
