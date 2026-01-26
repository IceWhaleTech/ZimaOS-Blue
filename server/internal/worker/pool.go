package worker

import (
	"context"
	"sync"

	"github.com/sourcegraph/conc/pool"
)

type Pool struct {
	pool     *pool.ContextPool
	size     int
	mu       sync.RWMutex
	running  int
	total    int64
}

func NewPool(ctx context.Context, size int) *Pool {
	p := pool.New().WithContext(ctx).WithMaxGoroutines(size)
	return &Pool{
		pool: p,
		size: size,
	}
}

func (p *Pool) Submit(fn func(context.Context) error) {
	p.mu.Lock()
	p.running++
	p.total++
	p.mu.Unlock()

	p.pool.Go(func(ctx context.Context) error {
		defer func() {
			p.mu.Lock()
			p.running--
			p.mu.Unlock()
		}()
		return fn(ctx)
	})
}

func (p *Pool) Wait() error {
	return p.pool.Wait()
}

func (p *Pool) Running() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

func (p *Pool) Total() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.total
}

func (p *Pool) Size() int {
	return p.size
}

type Stats struct {
	PoolSize int   `json:"pool_size"`
	Running  int   `json:"running"`
	Total    int64 `json:"total"`
}

func (p *Pool) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return Stats{
		PoolSize: p.size,
		Running:  p.running,
		Total:    p.total,
	}
}
