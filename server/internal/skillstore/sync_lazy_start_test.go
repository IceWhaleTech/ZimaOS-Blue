package skillstore

import (
	"context"
	"log/slog"
	"testing"
)

func TestSyncServiceStartDoesNotEagerlyStartReadmeFetcher(t *testing.T) {
	_, store, cleanup := setupSyncTestDB(t)
	defer cleanup()
	svc := NewSyncService(store, DefaultSyncServiceConfig(), slog.Default())

	if svc.readmeFetcher != nil {
		t.Fatal("expected readme fetcher to stay uninitialized after NewSyncService")
	}

	svc.Start(context.Background())

	if svc.readmeFetcher != nil {
		t.Fatal("expected readme fetcher to remain uninitialized after Start")
	}

	svc.ensureWorkersStarted(context.Background())

	if svc.readmeFetcher == nil {
		t.Fatal("expected readme fetcher to initialize on first sync demand")
	}
	if !svc.readmeFetcher.Started() {
		t.Fatal("expected readme fetcher workers to start on first sync demand")
	}

	svc.Stop()
}
