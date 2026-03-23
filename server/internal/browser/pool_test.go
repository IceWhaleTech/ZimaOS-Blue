package browser

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
)

func TestPoolAcquireReturnsErrWhenChannelClosed(t *testing.T) {
	pool := &Pool{
		config:    DefaultConfig(),
		available: make(chan *browserInstance),
	}
	close(pool.available)

	browser, err := pool.Acquire(context.Background())

	require.Nil(t, browser)
	require.ErrorIs(t, err, ErrBrowserNotRunning)
}

func TestPoolAcquireReturnsErrWhenPoolClosesWhileWaiting(t *testing.T) {
	pool := &Pool{
		config:    DefaultConfig(),
		available: make(chan *browserInstance, 1),
	}

	type result struct {
		browser *rod.Browser
		err     error
	}
	done := make(chan result, 1)
	go func() {
		browser, err := pool.Acquire(context.Background())
		done <- result{browser: browser, err: err}
	}()

	// Let Acquire reach the blocking receive before we mark the pool closed.
	time.Sleep(10 * time.Millisecond)

	pool.mu.Lock()
	pool.closed = true
	pool.mu.Unlock()

	pool.available <- &browserInstance{browser: &rod.Browser{}}

	res := <-done
	require.Nil(t, res.browser)
	require.ErrorIs(t, res.err, ErrBrowserNotRunning)
}
