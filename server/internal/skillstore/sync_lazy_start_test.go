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

	svc.Start(context.Background())

	if svc.readmeFetcher == nil {
		t.Fatal("expected readme fetcher to be configured")
	}
	if svc.readmeFetcher.Started() {
		t.Fatal("expected readme fetcher workers to stay lazy after Start")
	}

	svc.ensureWorkersStarted(context.Background())
	if !svc.readmeFetcher.Started() {
		t.Fatal("expected readme fetcher workers to start on first sync demand")
	}

	svc.Stop()
}
