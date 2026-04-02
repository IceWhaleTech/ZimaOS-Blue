package harness

import (
	"context"
	"fmt"
)

type RunTraceProvider interface {
	Snapshot(ctx context.Context, runID string) (*RunTrace, error)
}

func (c *Controller) SetRunTraceProvider(provider RunTraceProvider) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runTrace = provider
}

func (c *Controller) RunTraceSnapshot(ctx context.Context, runID string) (*RunTrace, error) {
	if c == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	c.mu.RLock()
	provider := c.runTrace
	c.mu.RUnlock()
	if provider == nil {
		return nil, nil
	}
	return provider.Snapshot(ctx, runID)
}
