package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPersistentTabsReleaseCreationLease(t *testing.T) {
	binary := findTestBrowserBinary()
	if binary == "" {
		t.Skip("Chrome/Chromium required")
	}
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><head><title>Lease test</title></head><body>retained</body></html>")
	}))
	defer pageServer.Close()
	cfg := DefaultConfig()
	cfg.BrowserPath, cfg.PoolSize, cfg.Headless = binary, 1, true
	svc, err := NewService(cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, svc.Start(ctx))
	defer svc.Close()
	var tabs []*Tab
	for i := 0; i < 4; i++ {
		tab, err := svc.OpenTab(ctx, pageServer.URL)
		require.NoError(t, err, "open retained tab %d", i)
		tabs = append(tabs, tab)
	}
	listed, err := svc.Tabs(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 4)
	// Closing a retained tab must not release another caller's active lease.
	leased, err := svc.pool.Acquire(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.CloseTab(ctx, tabs[0].TargetID))
	waitCtx, stopWait := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stopWait()
	_, err = svc.pool.Acquire(waitCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	svc.pool.Release(leased)
	for _, tab := range tabs[1:] {
		require.NoError(t, svc.CloseTab(ctx, tab.TargetID))
	}
}
