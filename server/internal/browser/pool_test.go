package browser

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
	launcherflags "github.com/go-rod/rod/lib/launcher/flags"
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

func TestPoolNewLauncherAddsManagedChromiumStabilityFlags(t *testing.T) {
	pool := &Pool{config: DefaultConfig()}

	l := pool.newLauncher()

	require.True(t, l.Has(launcherflags.Flag("no-default-browser-check")))
	require.True(t, l.Has(launcherflags.Flag("disable-extensions")))
	require.True(t, l.Has(launcherflags.Flag("disable-plugins")))
	require.True(t, l.Has(launcherflags.Flag("disable-plugins-discovery")))
	require.True(t, l.Has(launcherflags.Flag("disable-gpu")))

	disableFeatures, ok := l.GetFlags(launcherflags.Flag("disable-features"))
	require.True(t, ok)
	require.Contains(t, disableFeatures, "site-per-process")
	require.Contains(t, disableFeatures, "TranslateUI")
	require.Contains(t, disableFeatures, "DownloadBubble")
	require.Contains(t, disableFeatures, "DownloadBubbleV2")
}
