package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/stretchr/testify/require"
)

type disconnectedBrowserClient struct{}

func (disconnectedBrowserClient) Call(context.Context, string, string, interface{}) ([]byte, error) {
	return nil, errors.New("websocket: close 1006 (abnormal closure)")
}

func (disconnectedBrowserClient) Event() <-chan *cdp.Event { return nil }

func TestPoolCleanupTerminatesDisconnectedManagedBrowser(t *testing.T) {
	binary := findTestBrowserBinary()
	if binary == "" {
		t.Skip("Chrome/Chromium required")
	}
	for _, operation := range []string{"replace", "close"} {
		t.Run(operation, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.BrowserPath, cfg.PoolSize, cfg.Headless = binary, 1, true
			pool, err := NewPool(cfg)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			require.NoError(t, pool.Start(ctx))
			instance := pool.browsers[0]
			oldLauncher := instance.launcher
			require.Positive(t, oldLauncher.PID())
			// The CDP close fails immediately while the managed process remains alive.
			instance.browser = rod.New().Client(disconnectedBrowserClient{})
			done := make(chan error, 1)
			go func() {
				if operation == "close" {
					done <- pool.Close()
					return
				}
				_, err := pool.replaceInstance(ctx, instance.browser)
				done <- err
			}()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(4 * time.Second):
				t.Error("launcher cleanup blocked on a still-running disconnected process")
				oldLauncher.Kill()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("pool operation remained blocked after test cleanup killed Chromium")
				}
			}
			require.NoError(t, pool.Close())
		})
	}
}
